package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["document"] = cmdDocument
}

/** cmdDocument implements `kb document ingest|draft|review`. list/show land
 * in W8 (narrative-documents-plan.md).
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
		return usageErrorf("document requires a subverb: ingest, draft, review, list, show, tag, fuzzy-tag, or frontmatter")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "ingest":
		return cmdDocumentIngest(kb, jsonOut, rest, out)
	case "draft":
		return cmdDocumentDraft(kb, jsonOut, rest, out)
	case "review":
		return cmdDocumentReview(kb, jsonOut, rest, out)
	case "list":
		return cmdDocumentList(kb, jsonOut, rest, out)
	case "show":
		return cmdDocumentShow(kb, jsonOut, rest, out)
	case "tag":
		return cmdDocumentTag(kb, jsonOut, rest, out)
	case "fuzzy-tag":
		return cmdDocumentFuzzyTag(kb, jsonOut, rest, out)
	case "frontmatter":
		return cmdDocumentFrontmatter(kb, jsonOut, rest, out)
	default:
		return usageErrorf("unknown document subverb %q; want ingest, draft, review, list, show, tag, fuzzy-tag, or frontmatter", sub)
	}
}

// cmdDocumentReview implements `kb document review list|promote`.
func cmdDocumentReview(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return usageErrorf("usage: document review <list|promote> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return cmdDocumentReviewList(kb, jsonOut, rest, out)
	case "promote":
		return cmdDocumentReviewPromote(kb, jsonOut, rest, out)
	default:
		return usageErrorf("unknown document review subverb %q; want list or promote", sub)
	}
}

// cmdDocumentDraft implements `kb document draft SECTION_ID BODY --by WHO
// [--confidence N]` (design decision 7). Confidence range validation
// happens here, at the CLI boundary where untrusted input arrives --
// DraftDocumentSummary itself trusts its caller.
func cmdDocumentDraft(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	var by, confidenceStr string
	positional, err := splitFlags(args, map[string]*string{"--by": &by, "--confidence": &confidenceStr}, nil)
	if err != nil {
		return err
	}
	if len(positional) != 2 || by == "" {
		return usageErrorf("usage: document draft SECTION_ID BODY --by WHO [--confidence N]")
	}
	sectionID, err := strconv.ParseInt(positional[0], 10, 64)
	if err != nil {
		return usageErrorf("invalid section id %q", positional[0])
	}
	body := positional[1]

	var confidence *float64
	if confidenceStr != "" {
		c, err := strconv.ParseFloat(confidenceStr, 64)
		if err != nil {
			return usageErrorf("invalid --confidence %q", confidenceStr)
		}
		if c < 0 || c > 1 {
			return usageErrorf("--confidence must be between 0 and 1, got %v", c)
		}
		confidence = &c
	}

	if err := kb.DraftDocumentSummary(sectionID, body, by, confidence); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, map[string]any{"section_id": sectionID, "status": "drafted"})
	}
	fmt.Fprintf(out, "section %d drafted\n", sectionID)
	return nil
}

// cmdDocumentReviewPromote implements `kb document review promote
// SECTION_ID`.
func cmdDocumentReviewPromote(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	positional, err := splitFlags(args, nil, nil)
	if err != nil {
		return err
	}
	if len(positional) != 1 {
		return usageErrorf("usage: document review promote SECTION_ID")
	}
	sectionID, err := strconv.ParseInt(positional[0], 10, 64)
	if err != nil {
		return usageErrorf("invalid section id %q", positional[0])
	}
	if err := kb.PromoteDocumentSummary(sectionID); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, map[string]any{"section_id": sectionID, "status": "reviewed"})
	}
	fmt.Fprintf(out, "section %d reviewed\n", sectionID)
	return nil
}

// cmdDocumentReviewList implements `kb document review list [--project P]
// [--status S]` -- the triage queue from design decision 7.
func cmdDocumentReviewList(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	var projectName, status string
	positional, err := splitFlags(args, map[string]*string{"--project": &projectName, "--status": &status}, nil)
	if err != nil {
		return err
	}
	if len(positional) != 0 {
		return usageErrorf("usage: document review list [--project P] [--status S]")
	}
	var projectID int64
	if projectName != "" {
		p, err := kb.ProjectByName(projectName)
		if err != nil || p == nil {
			return fmt.Errorf("unknown project %q", projectName)
		}
		projectID = p.ID
	}
	items, err := kb.DocumentReviewQueue(projectID, status)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, items)
	}
	if len(items) == 0 {
		fmt.Fprintln(out, "(nothing to review)")
		return nil
	}
	for _, it := range items {
		stale := ""
		if it.SummaryStale {
			stale = " [STALE]"
		}
		conf := "-"
		if it.Confidence != nil {
			conf = fmt.Sprintf("%.2f", *it.Confidence)
		}
		fmt.Fprintf(out, "%-4d  %-12s  %-7s  %-30s  size=%-4d density=%-3d confidence=%s%s\n",
			it.ID, it.SummaryStatus, it.Level, it.DocumentTitle+" "+dashIfEmpty(it.Heading),
			it.SourceSize, it.TagDensity, conf, stale)
	}
	return nil
}

// documentIngestSummary is `kb document ingest`'s result, in both JSON and
// text form. It is the library's DocumentIngestResult (library-lift-plan.md
// L1), aliased so the JSON contract has exactly one definition.
type documentIngestSummary = knowledge.DocumentIngestResult

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
		return usageErrorf("usage: document ingest PATH --project P [--title T] [--format F] [--dry-run]")
	}
	path := positional[0]

	p, err := kb.ProjectByName(project)
	if err != nil || p == nil {
		return fmt.Errorf("unknown project %q", project)
	}

	summary, err := kb.IngestDocument(p.ID, path, knowledge.DocumentIngestOptions{
		Title: title, Format: format, DryRun: dryRun,
	})
	if err != nil {
		return err
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

// cmdDocumentList implements `kb document list [--project P]`.
func cmdDocumentList(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	var projectName string
	positional, err := splitFlags(args, map[string]*string{"--project": &projectName}, nil)
	if err != nil {
		return err
	}
	if len(positional) != 0 {
		return usageErrorf("usage: document list [--project P]")
	}
	var projectID int64
	if projectName != "" {
		p, err := kb.ProjectByName(projectName)
		if err != nil || p == nil {
			return fmt.Errorf("unknown project %q", projectName)
		}
		projectID = p.ID
	}
	docs, err := kb.Documents(projectID)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, docs)
	}
	if len(docs) == 0 {
		fmt.Fprintln(out, "no matching documents")
		return nil
	}
	for _, d := range docs {
		fmt.Fprintf(out, "%-4d  %-8s  %-30s  %s\n", d.ID, d.Format, d.Title, d.Path)
	}
	return nil
}

// cmdDocumentShow implements `kb document show ID`: metadata plus every
// section (heading, status, stale flag, linked concepts) in one view --
// unlike record, no separate concepts subverb, since this entity is new
// enough to design show to include everything from day one.
func cmdDocumentShow(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	positional, err := splitFlags(args, nil, nil)
	if err != nil {
		return err
	}
	if len(positional) != 1 {
		return usageErrorf("usage: document show ID")
	}
	docID, err := strconv.ParseInt(positional[0], 10, 64)
	if err != nil {
		return usageErrorf("invalid document id %q", positional[0])
	}
	doc, err := kb.DocumentByID(docID)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("no document with id %d", docID)
	}
	sections, err := kb.DocumentSections(docID)
	if err != nil {
		return err
	}

	type sectionView struct {
		knowledge.DocumentSection
		Concepts []knowledge.Concept `json:"concepts"`
	}
	views := make([]sectionView, 0, len(sections))
	for _, s := range sections {
		concepts, err := kb.DocumentSectionConcepts(s.ID)
		if err != nil {
			return err
		}
		views = append(views, sectionView{DocumentSection: s, Concepts: concepts})
	}

	if jsonOut {
		return printJSON(out, struct {
			knowledge.Document
			Sections []sectionView `json:"sections"`
		}{Document: *doc, Sections: views})
	}

	fmt.Fprintf(out, "%d  %s  (%s)\n", doc.ID, doc.Title, doc.Format)
	fmt.Fprintf(out, "  path:           %s\n", doc.Path)
	fmt.Fprintf(out, "  author:         %s\n", dashIfEmpty(doc.Author))
	fmt.Fprintf(out, "  published_date: %s\n", dashIfEmpty(doc.PublishedDate))
	for _, v := range views {
		stale := ""
		if v.SummaryStale {
			stale = " [STALE]"
		}
		heading := v.Heading
		if v.Level == "gist" {
			heading = "(gist)"
		}
		fmt.Fprintf(out, "  section %-4d  %-20s  %-12s%s\n", v.ID, heading, v.SummaryStatus, stale)
		if len(v.Concepts) > 0 {
			names := make([]string, len(v.Concepts))
			for i, c := range v.Concepts {
				names[i] = c.Name
			}
			fmt.Fprintf(out, "      concepts: %s\n", strings.Join(names, ", "))
		}
	}
	return nil
}
