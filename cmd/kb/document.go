package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["document"] = cmdDocument
}

/** cmdDocument implements `kb document ingest`. Review/draft/promote and
 * list/show land in later phases (narrative-documents-plan.md W5/W8).
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — the open knowledge base.
 *   dl      (*DebugLog)                — debug log, may be nil.
 *   jsonOut (bool)                     — emit results as JSON.
 *   args    ([]string)                 — the subverb and its arguments.
 *   out     (io.Writer)                — where results are written.
 *
 * Returns:
 *   error — on a usage error or a failed read/write.
 *
 * Example:
 *   err := cmdDocument(kb, nil, false, []string{"ingest", "a.md", "--project", "p"}, os.Stdout)
 */
func cmdDocument(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("document requires a subverb: ingest")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "ingest":
		return cmdDocumentIngest(kb, jsonOut, rest, out)
	default:
		return fmt.Errorf("unknown document subverb %q; want ingest", sub)
	}
}

// documentIngestSummary is `kb document ingest`'s result, in both JSON and
// text form. RemovedHeadings lists old sections whose heading no longer
// matches anything in the re-ingested content -- reported, never deleted
// (design decision 10).
type documentIngestSummary struct {
	Path            string   `json:"path"`
	DryRun          bool     `json:"dry_run"`
	Action          string   `json:"action"`
	SectionsAdded   int      `json:"sections_added"`
	SectionsUpdated int      `json:"sections_updated"`
	SectionsStale   int      `json:"sections_stale"`
	RemovedHeadings []string `json:"removed_headings,omitempty"`
	Warning         string   `json:"warning,omitempty"`
}

// parseDocumentIngestFlags separates flags from positional arguments via the
// shared splitFlags (cmd/kb/flagsplit.go) rather than flag.FlagSet, which
// stops parsing at the first non-flag argument -- the wrong behavior for
// `document ingest PATH --project P`, where PATH comes first.
func parseDocumentIngestFlags(args []string) (project, title, format string, dryRun bool, positional []string, err error) {
	strFlags := map[string]*string{"--project": &project, "--title": &title, "--format": &format}
	boolFlags := map[string]*bool{"--dry-run": &dryRun}
	positional, err = splitFlags(args, strFlags, boolFlags)
	return project, title, format, dryRun, positional, err
}

func cmdDocumentIngest(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	project, title, format, dryRun, positional, err := parseDocumentIngestFlags(args)
	if err != nil {
		return err
	}
	if project == "" || len(positional) != 1 {
		return fmt.Errorf("usage: document ingest PATH --project P [--title T] [--format F] [--dry-run]")
	}
	path := positional[0]

	p, err := kb.ProjectByName(project)
	if err != nil || p == nil {
		return fmt.Errorf("unknown project %q", project)
	}

	parsed, err := knowledge.ParseDocumentFile(path, format)
	if err != nil {
		return err
	}

	docTitle := title
	if docTitle == "" {
		docTitle = parsed.Title
	}
	if docTitle == "" {
		docTitle = filepath.Base(path)
	}

	summary := documentIngestSummary{Path: path, DryRun: dryRun, Warning: parsed.Warning}

	existing, err := kb.DocumentByPath(path)
	if err != nil {
		return err
	}

	switch {
	case existing == nil:
		summary.Action = "added"
		summary.SectionsAdded = len(parsed.Sections)
		if !dryRun {
			if err := ingestNewDocument(kb, p.ID, path, docTitle, parsed); err != nil {
				return err
			}
		}

	case existing.Checksum == parsed.Checksum:
		summary.Action = "skipped"

	default:
		summary.Action = "updated"
		if !dryRun {
			if err := reingestChangedDocument(kb, existing, docTitle, parsed, &summary); err != nil {
				return err
			}
		}
	}

	if jsonOut {
		return printJSON(out, summary)
	}
	fmt.Fprintf(out, "%s: %s (%d added, %d updated, %d stale)\n",
		path, summary.Action, summary.SectionsAdded, summary.SectionsUpdated, summary.SectionsStale)
	if summary.Warning != "" {
		fmt.Fprintln(out, "warning:", summary.Warning)
	}
	for _, h := range summary.RemovedHeadings {
		fmt.Fprintf(out, "removed (not deleted, may need review): %q\n", h)
	}
	return nil
}

// ingestNewDocument writes a brand-new document: the documents row, a gist
// row (seeded from parsed.GistSeed when the source frontmatter/title page
// provided one -- design decision 3), and every section.
func ingestNewDocument(kb *knowledge.KnowledgeBase, projectID int64, path, title string, parsed *knowledge.ParsedDocument) error {
	docID, err := kb.AddDocument(knowledge.Document{
		ProjectID: projectID, Title: title, Format: parsed.Format, Path: path,
		Author: parsed.Author, PublishedDate: parsed.PublishedDate, Checksum: parsed.Checksum,
	})
	if err != nil {
		return err
	}
	gist := knowledge.DocumentSection{DocumentID: docID, Level: "gist"}
	if parsed.GistSeed != "" {
		gist.SummaryBody = parsed.GistSeed
		gist.SummaryStatus = "drafted"
		gist.GeneratedBy = "human"
	}
	if _, err := kb.AddDocumentSection(gist); err != nil {
		return err
	}
	for _, s := range parsed.Sections {
		s.DocumentID = docID
		s.SourceSize = len(strings.Fields(s.Body))
		if _, err := kb.AddDocumentSection(s); err != nil {
			return err
		}
	}
	return nil
}

// reingestChangedDocument reconciles a changed file against its existing
// rows by matching sections on heading text (design decision 10): an
// unchanged body under a matched heading is left untouched; a changed body
// updates in place and flags summary_stale rather than discarding the
// summary; an old heading with no match is reported, never deleted; a new
// heading with no match is inserted fresh, unsummarized. If anything
// changed at all, an existing gist summary is flagged stale too, since a
// whole-document gist describes the document as a whole.
func reingestChangedDocument(kb *knowledge.KnowledgeBase, existing *knowledge.Document, title string, parsed *knowledge.ParsedDocument, summary *documentIngestSummary) error {
	if err := kb.UpdateDocumentMetadata(existing.ID, title, parsed.Author, parsed.PublishedDate, parsed.Checksum); err != nil {
		return err
	}
	oldSections, err := kb.DocumentSections(existing.ID)
	if err != nil {
		return err
	}
	oldByHeading := map[string]knowledge.DocumentSection{}
	var gistRow *knowledge.DocumentSection
	for _, s := range oldSections {
		if s.Level == "gist" {
			g := s
			gistRow = &g
			continue
		}
		oldByHeading[s.Heading] = s
	}

	matchedHeadings := map[string]bool{}
	anyChange := false
	for _, ns := range parsed.Sections {
		matchedHeadings[ns.Heading] = true
		old, ok := oldByHeading[ns.Heading]
		if !ok {
			ns.DocumentID = existing.ID
			ns.SourceSize = len(strings.Fields(ns.Body))
			if _, err := kb.AddDocumentSection(ns); err != nil {
				return err
			}
			summary.SectionsAdded++
			anyChange = true
			continue
		}
		if old.Body == ns.Body {
			continue
		}
		stale := old.SummaryStatus != "unsummarized"
		if err := kb.UpdateDocumentSectionBody(old.ID, ns.Body, len(strings.Fields(ns.Body)), stale); err != nil {
			return err
		}
		summary.SectionsUpdated++
		anyChange = true
		if stale {
			summary.SectionsStale++
		}
	}
	for heading := range oldByHeading {
		if !matchedHeadings[heading] {
			summary.RemovedHeadings = append(summary.RemovedHeadings, heading)
		}
	}

	if gistRow != nil && gistRow.SummaryStatus != "unsummarized" && anyChange {
		if err := kb.MarkDocumentSectionStale(gistRow.ID); err != nil {
			return err
		}
	}
	return nil
}
