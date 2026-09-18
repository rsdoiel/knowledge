package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

// existingWikilinkPattern finds [[...]] spans already present in text, to
// exclude them from both re-matching (no nested brackets) and from
// alreadyWikilinked's idempotency check. Deliberately not wikilinkPattern
// (cmd/kb/ingest.go): that one captures the inner name for resolution; this
// one only needs the byte span of the whole bracketed form.
var existingWikilinkPattern = regexp.MustCompile(`\[\[[^\[\]]+\]\]`)

// textSpan is a half-open byte range [Start, End) within a string.
type textSpan struct{ Start, End int }

// overlaps reports whether s and loc (a regexp FindIndex-style [start, end)
// pair) share any byte.
func (s textSpan) overlaps(loc []int) bool {
	return loc[0] < s.End && s.Start < loc[1]
}

// excludedSpans returns the byte ranges of text that insertWikilink must
// never write into: a leading YAML frontmatter block, fenced code blocks,
// inline code spans, and any wikilink already present. Reuses the same
// fenced/inline patterns document ingest's density-linking already applies
// (DR-0027's stripCodeSpans) rather than a second definition of "code," and
// the same frontmatter delimiter recordfile.go's splitFrontmatter uses,
// applied positionally instead of by string-splitting since a caller here
// needs byte offsets into the original text, not a parsed-and-rejoined copy.
func excludedSpans(text string) []textSpan {
	var spans []textSpan
	if loc := frontmatterBlockPattern.FindStringIndex(text); loc != nil {
		spans = append(spans, textSpan{loc[0], loc[1]})
	}
	if loc := firstH1HeadingPattern.FindStringIndex(text); loc != nil {
		spans = append(spans, textSpan{loc[0], loc[1]})
	}
	for _, loc := range fencedCodeBlockPattern.FindAllStringIndex(text, -1) {
		spans = append(spans, textSpan{loc[0], loc[1]})
	}
	for _, loc := range inlineCodeSpanPattern.FindAllStringIndex(text, -1) {
		spans = append(spans, textSpan{loc[0], loc[1]})
	}
	for _, loc := range existingWikilinkPattern.FindAllStringIndex(text, -1) {
		spans = append(spans, textSpan{loc[0], loc[1]})
	}
	return spans
}

// frontmatterBlockPattern matches a leading "---\n...\n---" block, the same
// shape splitFrontmatter (recordfile.go) requires to treat a file as having
// frontmatter at all. Anchored to the very start of text (Go's regexp `^`
// without (?m) matches only the string start, not every line), since
// frontmatter is only ever a document's opening block, never a mid-body
// occurrence of three dashes.
var frontmatterBlockPattern = regexp.MustCompile(`(?s)^---\n.*?\n---`)

// firstH1HeadingPattern matches only the document's very first true H1
// line (a single '#'), wherever it falls in text -- found live,
// smoke-testing against a real document: a concept name in the title got
// wikilinked, mechanically correct (a real, distinctive match) but
// stylistically wrong, since a title should read as plain prose. `(?m)`
// so `^`/`$` match at any line start/end, not just the string's; only the
// first match is ever used (FindStringIndex, not FindAll), matching
// firstH1Heading's (documents.go) own "first H1 only" semantics.
var firstH1HeadingPattern = regexp.MustCompile(`(?m)^#[ \t]+.*$`)

// alreadyWikilinked reports whether name (case-insensitively) already
// appears inside some [[...]] span in text -- the idempotency check that
// makes a second `kb document tag` run on an unchanged file a no-op for
// this concept, the same "second run skips unchanged" discipline `kb
// ingest` and `kb document ingest` already apply, just checked structurally
// here rather than by comparing a checksum.
func alreadyWikilinked(text, name string) bool {
	pattern := regexp.MustCompile(`(?i)\[\[\s*` + regexp.QuoteMeta(name) + `\s*\]\]`)
	return pattern.MatchString(text)
}

/** insertWikilink wraps the first safe occurrence of name in text with
 * [[ ]], preserving the matched text's original casing, for `kb document
 * tag` (TODO.md's programmatic-corpus-improvement item). "Safe" excludes a
 * leading YAML frontmatter block, fenced code blocks, inline code spans,
 * and any span already inside an existing [[...]] wikilink -- inserting
 * into any of those would corrupt metadata, quoted code, or nest brackets.
 * A name already wikilinked anywhere in text is left alone rather than
 * wrapped a second time.
 *
 * Parameters:
 *   text (string) — the document's raw content (or the portion being
 *                    scanned).
 *   name (string) — the concept name to find, matched case-insensitively
 *                    as a whole word/phrase (word-boundary anchored, like
 *                    MatchConceptNames).
 *
 * Returns:
 *   string — text with the first safe occurrence of name wrapped in
 *            [[ ]], or text unchanged if inserted is false.
 *   bool   — whether an insertion was made.
 *
 * Example:
 *   out, inserted := insertWikilink("we discussed chunking today", "chunking")
 *   // out == "we discussed [[chunking]] today", inserted == true
 */
func insertWikilink(text, name string) (string, bool) {
	if alreadyWikilinked(text, name) {
		return text, false
	}
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(name) + `\b`)
	excluded := excludedSpans(text)
	for _, loc := range pattern.FindAllStringIndex(text, -1) {
		safe := true
		for _, span := range excluded {
			if span.overlaps(loc) {
				safe = false
				break
			}
		}
		if !safe {
			continue
		}
		matched := text[loc[0]:loc[1]]
		return text[:loc[0]] + "[[" + matched + "]]" + text[loc[1]:], true
	}
	return text, false
}

/** tagDocumentText applies insertWikilink for every name in names against
 * text, returning the fully-updated text and the subset of names actually
 * inserted. Names are applied longest-first (ties broken alphabetically,
 * for determinism), so a longer phrase is wrapped as one unit before a
 * shorter name it contains gets a chance to fragment it -- e.g. "chunking
 * strategy" wraps whole rather than "chunking" wrapping first and leaving
 * "strategy" dangling outside the brackets.
 *
 * Parameters:
 *   text  (string)   — the document's raw content to tag.
 *   names ([]string) — candidate concept names to look for.
 *
 * Returns:
 *   string   — text with every safely-found candidate wrapped in [[ ]].
 *   []string — the names actually inserted, in application order; nil if
 *              none matched.
 *
 * Example:
 *   out, inserted := tagDocumentText(body, []string{"chunking", "workspace"})
 */
func tagDocumentText(text string, names []string) (string, []string) {
	sorted := append([]string(nil), names...)
	sort.Slice(sorted, func(i, j int) bool {
		if len(sorted[i]) != len(sorted[j]) {
			return len(sorted[i]) > len(sorted[j])
		}
		return sorted[i] < sorted[j]
	})
	var inserted []string
	for _, name := range sorted {
		next, ok := insertWikilink(text, name)
		if ok {
			text = next
			inserted = append(inserted, name)
		}
	}
	return text, inserted
}

/** eligibleConcepts returns the subset of candidates mentioned more than
 * once in text, outside code spans -- the same >1-occurrence,
 * code-span-excluded threshold DR-0027's density-linking already applies
 * to decide a concept is genuinely relevant to a piece of text, reused
 * here (via MatchConceptNameCounts) so a concept `kb document tag` would
 * insert explicitly is exactly one density-linking would already have
 * linked implicitly on the next ingest -- this tool just makes that link
 * visible in the prose instead of leaving it inferred.
 *
 * Parameters:
 *   kb         (*knowledge.KnowledgeBase) — the open knowledge base.
 *   text       (string)                   — the document's raw content.
 *   candidates ([]string)                 — concept names eligibility is
 *                                            restricted to.
 *
 * Returns:
 *   []string — candidates mentioned more than once outside code spans; nil
 *              when none qualify.
 *   error    — on database failure.
 *
 * Example:
 *   names, err := eligibleConcepts(kb, body, []string{"chunking", "workspace"})
 */
func eligibleConcepts(kb *knowledge.KnowledgeBase, text string, candidates []string) ([]string, error) {
	allowed := map[string]bool{}
	for _, c := range candidates {
		allowed[strings.ToLower(c)] = true
	}
	counts, err := kb.MatchConceptNameCounts(stripCodeSpans(text))
	if err != nil {
		return nil, err
	}
	var out []string
	for name, count := range counts {
		if count > 1 && allowed[strings.ToLower(name)] {
			out = append(out, name)
		}
	}
	return out, nil
}

// documentTagResult is one file's outcome in `kb document tag`'s report.
type documentTagResult struct {
	Path     string   `json:"path"`
	Inserted []string `json:"inserted"`
}

/** cmdDocumentTag implements `kb document tag --project NAME [--concept
 * NAME,...] [--dry-run]` (TODO.md's programmatic-corpus-improvement item):
 * a pure file operation over every document already ingested for a
 * project, inserting an explicit [[Name]] wikilink for each eligible
 * concept at its first safe occurrence. It never touches the database --
 * the next `kb document ingest` of a changed file sees a new checksum and
 * links it through the ordinary wikilink-resolution path that already
 * exists, the same "corpus rewrite, not a database write" split DR-0026
 * uses for `kb project rename`.
 *
 * With no --concept, eligibility is DR-0027's density-linking threshold:
 * a concept must occur more than once in the file, outside code spans.
 * --concept NAME,NAME,... bypasses that threshold for exactly the names
 * given -- each must already be a known concept, checked before any file
 * is read, so a typo fails the whole call rather than silently tagging
 * nothing.
 *
 * Writes are both-or-neither across the whole run: if any file fails to
 * write, every file already written in this call is restored from its
 * original bytes.
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
 *   err := cmdDocumentTag(kb, false, []string{"--project", "harvey"}, os.Stdout)
 */
func cmdDocumentTag(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("document tag", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	projectName := fs.String("project", "", "scope to one project (required)")
	conceptList := fs.String("concept", "", "comma-separated concept names to force, bypassing the occurrence threshold")
	dryRun := fs.Bool("dry-run", false, "report what would be tagged without writing anything")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 || *projectName == "" {
		return fmt.Errorf("usage: document tag --project NAME [--concept NAME,...] [--dry-run]")
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
	var allNames []string
	for _, c := range concepts {
		known[strings.ToLower(c.Name)] = true
		allNames = append(allNames, c.Name)
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
	var results []documentTagResult
	var writes []stagedWrite
	for _, d := range docs {
		raw, err := os.ReadFile(d.Path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", d.Path, err)
		}
		text := string(raw)

		candidates := explicit
		if len(candidates) == 0 {
			candidates, err = eligibleConcepts(kb, text, allNames)
			if err != nil {
				return err
			}
		}
		if len(candidates) == 0 {
			continue
		}
		next, inserted := tagDocumentText(text, candidates)
		if len(inserted) == 0 {
			continue
		}
		results = append(results, documentTagResult{Path: d.Path, Inserted: inserted})
		writes = append(writes, stagedWrite{path: d.Path, raw: raw, next: []byte(next)})
	}

	if *dryRun {
		return reportDocumentTag(jsonOut, true, results, out)
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

	return reportDocumentTag(jsonOut, false, results, out)
}

// reportDocumentTag renders cmdDocumentTag's outcome, plain text or --json.
func reportDocumentTag(jsonOut, dryRun bool, results []documentTagResult, out io.Writer) error {
	if jsonOut {
		return printJSON(out, map[string]any{"dry_run": dryRun, "files": results})
	}
	if len(results) == 0 {
		fmt.Fprintln(out, "no changes -- nothing to tag")
		return nil
	}
	verb := "tagged"
	if dryRun {
		verb = "would tag"
	}
	for _, r := range results {
		fmt.Fprintf(out, "%s %s: %s\n", verb, r.Path, strings.Join(r.Inserted, ", "))
	}
	return nil
}
