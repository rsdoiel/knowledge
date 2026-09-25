package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

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
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 || *projectName == "" {
		return usageErrorf("usage: document tag --project NAME [--concept NAME,...] [--dry-run]")
	}

	p, err := kb.ProjectByName(*projectName)
	if err != nil {
		return err
	}
	if p == nil {
		return notFoundf("unknown project %q", *projectName)
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
				return notFoundf("unknown concept %q", name)
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
			candidates, err = kb.EligibleTagConcepts(text, allNames)
			if err != nil {
				return err
			}
		}
		if len(candidates) == 0 {
			continue
		}
		next, inserted := knowledge.TagDocumentText(text, candidates)
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
