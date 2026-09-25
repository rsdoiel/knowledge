package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

// fuzzyTagEntry is one footnote insertion in documentFuzzyTagResult's report.
// It is the library's FuzzyTagInsertion (library-lift-plan.md L2), aliased so
// the JSON contract has exactly one definition.
type fuzzyTagEntry = knowledge.FuzzyTagInsertion

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
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 || *projectName == "" {
		return usageErrorf("usage: document fuzzy-tag --project NAME [--concept NAME,...] [--dry-run]")
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
		eligible := knowledge.FuzzyEligible(matches, explicit, text)
		if len(eligible) == 0 {
			continue
		}

		next, entries := knowledge.FuzzyTagDocumentText(text, eligible)
		if len(entries) == 0 {
			continue
		}
		results = append(results, documentFuzzyTagResult{Path: d.Path, Footnoted: entries})
		writes = append(writes, stagedWrite{path: d.Path, raw: raw, next: []byte(next)})
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
