package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

// sectionHeadingPattern is fuzzy-tag's own copy of markdownHeadingPattern
// (documents.go:341) -- the same "own copy, not a shared internal"
// precedent firstH1HeadingPattern (documenttag.go) already sets for reusing
// a documents.go pattern from cmd/kb.
var sectionHeadingPattern = regexp.MustCompile(`(?m)^#{1,6}[ \t]+(.*)$`)

// footnoteLabelPattern matches both a marker ([^3]) and the start of a
// definition ([^3]:).
var footnoteLabelPattern = regexp.MustCompile(`\[\^(\d+)\]`)

// footnoteDefinitionLinePattern matches a whole footnote-definition line, so
// fuzzyExcludedSpans can keep a marker from ever landing inside one.
var footnoteDefinitionLinePattern = regexp.MustCompile(`(?m)^\[\^\d+\]:.*$`)

// nearestPrecedingHeading returns the text of the last sectionHeadingPattern
// match starting before pos, or "" if pos falls before any heading (gist/
// lead text).
func nearestPrecedingHeading(text string, pos int) string {
	var heading string
	for _, m := range sectionHeadingPattern.FindAllStringSubmatchIndex(text, -1) {
		if m[0] >= pos {
			break
		}
		heading = text[m[2]:m[3]]
	}
	return heading
}

// nextSectionBoundary returns the start offset of the next
// sectionHeadingPattern match after pos, or len(text) if none -- where a
// footnote definition for a marker at pos gets inserted.
func nextSectionBoundary(text string, pos int) int {
	for _, loc := range sectionHeadingPattern.FindAllStringIndex(text, -1) {
		if loc[0] > pos {
			return loc[0]
		}
	}
	return len(text)
}

// nextFootnoteLabel scans every footnoteLabelPattern match's captured number
// and returns one past the highest found, or 1 if none exist. Label
// numbering is whole-file, not per-section: CommonMark footnote labels are
// scoped to the whole rendered document regardless of source structure.
func nextFootnoteLabel(text string) int {
	max := 0
	for _, m := range footnoteLabelPattern.FindAllStringSubmatch(text, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

// fuzzyExcludedSpans is excludedSpans (documenttag.go), untouched, plus
// every footnoteDefinitionLinePattern match, so a marker can never land
// inside an existing footnote definition. A new function, not an edit to
// excludedSpans itself -- kb document tag must stay exactly as it is.
func fuzzyExcludedSpans(text string) []textSpan {
	spans := excludedSpans(text)
	for _, loc := range footnoteDefinitionLinePattern.FindAllStringIndex(text, -1) {
		spans = append(spans, textSpan{loc[0], loc[1]})
	}
	return spans
}

// insertFootnote splices a footnote marker and definition into text for a
// near-miss match at [matchStart, matchEnd). The definition is inserted
// first, immediately before nextSectionBoundary(text, matchEnd) (or at end
// of file) -- its insertion point is always >= matchEnd, so doing this
// first keeps matchEnd valid for the marker insertion that follows. The
// marker ([^label]) is then inserted immediately after matchEnd. matchStart
// is unused by the splice itself; it's part of the signature for callers
// that already have both offsets from a FuzzyConceptMatch.
func insertFootnote(text string, matchStart, matchEnd int, concept string, label int) string {
	boundary := nextSectionBoundary(text, matchEnd)
	def := fmt.Sprintf("\n[^%d]: see [[%s]]\n", label, concept)
	text = text[:boundary] + def + text[boundary:]
	marker := fmt.Sprintf("[^%d]", label)
	text = text[:matchEnd] + marker + text[matchEnd:]
	return text
}

// fuzzyThreshold is design decision 5's real, length-based eligibility
// threshold -- distinct from FuzzyMatchConceptNames's own generous
// maxFuzzyDistance candidate-generation ceiling.
func fuzzyThreshold(concept string) int {
	if len(concept) <= 8 {
		return 1
	}
	return 2
}

// fuzzyEligible applies design decision 5's real threshold to matches
// already produced by FuzzyMatchConceptNames: with no explicit concepts
// given, keep a match only if its Distance is within fuzzyThreshold for its
// Concept's length; with explicit concepts given, keep only matches whose
// Concept is named there (any distance up to FuzzyMatchConceptNames's own
// ceiling qualifies -- the --concept bypass, decision 2), dropping every
// other concept's matches entirely. Either way, drop any match whose
// [Start, End) overlaps a span from fuzzyExcludedSpans(text).
//
// Takes text (not just matches) to compute exclusion spans -- the plan's
// original signature omitted it, an oversight caught while implementing:
// fuzzyExcludedSpans needs the raw text to scan for frontmatter/code/
// existing wikilinks/footnote-definition lines.
func fuzzyEligible(matches []knowledge.FuzzyConceptMatch, explicit []string, text string) []knowledge.FuzzyConceptMatch {
	explicitSet := map[string]bool{}
	for _, name := range explicit {
		explicitSet[strings.ToLower(name)] = true
	}
	excluded := fuzzyExcludedSpans(text)

	var out []knowledge.FuzzyConceptMatch
	for _, m := range matches {
		if len(explicit) > 0 {
			if !explicitSet[strings.ToLower(m.Concept)] {
				continue
			}
		} else if m.Distance > fuzzyThreshold(m.Concept) {
			continue
		}
		safe := true
		for _, span := range excluded {
			if span.overlaps([]int{m.Start, m.End}) {
				safe = false
				break
			}
		}
		if safe {
			out = append(out, m)
		}
	}
	return out
}

// fuzzyTagEntry is one footnote insertion in documentFuzzyTagResult's report.
type fuzzyTagEntry struct {
	Section  string `json:"section"`
	Text     string `json:"text"`
	Concept  string `json:"concept"`
	Distance int    `json:"distance"`
	Footnote int    `json:"footnote"`
}

// documentFuzzyTagResult is one file's outcome in `kb document fuzzy-tag`'s
// report.
type documentFuzzyTagResult struct {
	Path      string          `json:"path"`
	Footnoted []fuzzyTagEntry `json:"footnoted"`
}

/** cmdDocumentFuzzyTag implements `kb document fuzzy-tag --project NAME
 * [--concept NAME,...] [--dry-run]` (v0.0.11 item 1,
 * fuzzy-concept-matching-plan.md): mirrors `kb document tag`'s shape and
 * write behavior, substituting fuzzy (Levenshtein) matching for exact and a
 * footnote for a direct bracket-wrap. A footnote, not a bracket, because
 * bracketing a near-miss verbatim (e.g. `[[chunkings]]`) would let
 * ResolveConceptName mint a duplicate concept rather than link the real one
 * -- the footnote definition always carries the canonical concept name.
 *
 * With no --concept, eligibility is fuzzyThreshold's real, length-based
 * distance threshold. --concept NAME,NAME,... bypasses that threshold for
 * exactly the names given -- each must already be a known concept, checked
 * before any file is read, so a typo fails the whole call rather than
 * silently footnoting nothing.
 *
 * Writes are both-or-neither across the whole run, matching cmdDocumentTag.
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — the open knowledge base.
 *   jsonOut (bool)                     — emit results as JSON.
 *   args    ([]string)                 — --project, --concept, --dry-run.
 *   out     (io.Writer)                — where results are written.
 *
 * Returns:
 *   error — on a usage error, an unknown project or --concept name, or a
 *           failed read/write.
 *
 * Example:
 *   err := cmdDocumentFuzzyTag(kb, false, []string{"--project", "harvey"}, os.Stdout)
 */
func cmdDocumentFuzzyTag(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("document fuzzy-tag", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	projectName := fs.String("project", "", "scope to one project (required)")
	conceptList := fs.String("concept", "", "comma-separated concept names to force, bypassing the distance threshold")
	dryRun := fs.Bool("dry-run", false, "report what would be footnoted without writing anything")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 || *projectName == "" {
		return fmt.Errorf("usage: document fuzzy-tag --project NAME [--concept NAME,...] [--dry-run]")
	}

	p, err := kb.ProjectByName(*projectName)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("unknown project %q", *projectName)
	}

	concepts, err := kb.Concepts()
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, c := range concepts {
		known[strings.ToLower(c.Name)] = true
	}

	var explicit []string
	if *conceptList != "" {
		for _, raw := range strings.Split(*conceptList, ",") {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			if !known[strings.ToLower(name)] {
				return fmt.Errorf("unknown concept %q", name)
			}
			explicit = append(explicit, name)
		}
	}

	docs, err := kb.Documents(p.ID)
	if err != nil {
		return err
	}

	type stagedWrite struct {
		path string
		raw  []byte
		next []byte
	}
	var results []documentFuzzyTagResult
	var writes []stagedWrite
	for _, d := range docs {
		raw, err := os.ReadFile(d.Path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", d.Path, err)
		}
		text := string(raw)

		matches, err := kb.FuzzyMatchConceptNames(text)
		if err != nil {
			return err
		}
		eligible := fuzzyEligible(matches, explicit, text)
		if len(eligible) == 0 {
			continue
		}

		var entries []fuzzyTagEntry
		delta := 0
		for _, m := range eligible {
			start, end := m.Start+delta, m.End+delta
			if alreadyWikilinked(text, m.Concept) {
				continue
			}
			label := nextFootnoteLabel(text)
			section := nearestPrecedingHeading(text, start)
			before := len(text)
			text = insertFootnote(text, start, end, m.Concept, label)
			delta += len(text) - before
			entries = append(entries, fuzzyTagEntry{
				Section: section, Text: m.Text, Concept: m.Concept, Distance: m.Distance, Footnote: label,
			})
		}
		if len(entries) == 0 {
			continue
		}
		results = append(results, documentFuzzyTagResult{Path: d.Path, Footnoted: entries})
		writes = append(writes, stagedWrite{path: d.Path, raw: raw, next: []byte(text)})
	}

	if *dryRun {
		return reportDocumentFuzzyTag(jsonOut, true, results, out)
	}

	var written []stagedWrite
	rollback := func() {
		for _, w := range written {
			_ = os.WriteFile(w.path, w.raw, 0o644)
		}
	}
	for _, w := range writes {
		if err := os.WriteFile(w.path, w.next, 0o644); err != nil {
			rollback()
			return fmt.Errorf("writing %s: %w", w.path, err)
		}
		written = append(written, w)
	}

	return reportDocumentFuzzyTag(jsonOut, false, results, out)
}

// reportDocumentFuzzyTag renders cmdDocumentFuzzyTag's outcome, plain text
// or --json, mirroring reportDocumentTag: "would footnote"/"footnoted" in
// place of "would tag"/"tagged".
func reportDocumentFuzzyTag(jsonOut, dryRun bool, results []documentFuzzyTagResult, out io.Writer) error {
	if jsonOut {
		return printJSON(out, map[string]any{"dry_run": dryRun, "files": results})
	}
	if len(results) == 0 {
		fmt.Fprintln(out, "no changes -- nothing to footnote")
		return nil
	}
	verb := "footnoted"
	if dryRun {
		verb = "would footnote"
	}
	for _, r := range results {
		for _, e := range r.Footnoted {
			section := e.Section
			if section == "" {
				section = "(lead)"
			}
			fmt.Fprintf(out, "%s %s [%s]: %q ~ %s (distance %d)\n", verb, r.Path, section, e.Text, e.Concept, e.Distance)
		}
	}
	return nil
}
