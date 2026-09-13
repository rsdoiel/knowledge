package main

import (
	"fmt"
	"io"
	"path/filepath"
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
		return fmt.Errorf("document requires a subverb: ingest, draft, review, list, or show")
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
	default:
		return fmt.Errorf("unknown document subverb %q; want ingest, draft, review, list, or show", sub)
	}
}

// cmdDocumentReview implements `kb document review list|promote`.
func cmdDocumentReview(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: document review <list|promote> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return cmdDocumentReviewList(kb, jsonOut, rest, out)
	case "promote":
		return cmdDocumentReviewPromote(kb, jsonOut, rest, out)
	default:
		return fmt.Errorf("unknown document review subverb %q; want list or promote", sub)
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
		return fmt.Errorf("usage: document draft SECTION_ID BODY --by WHO [--confidence N]")
	}
	sectionID, err := strconv.ParseInt(positional[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid section id %q", positional[0])
	}
	body := positional[1]

	var confidence *float64
	if confidenceStr != "" {
		c, err := strconv.ParseFloat(confidenceStr, 64)
		if err != nil {
			return fmt.Errorf("invalid --confidence %q", confidenceStr)
		}
		if c < 0 || c > 1 {
			return fmt.Errorf("--confidence must be between 0 and 1, got %v", c)
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
		return fmt.Errorf("usage: document review promote SECTION_ID")
	}
	sectionID, err := strconv.ParseInt(positional[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid section id %q", positional[0])
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
		return fmt.Errorf("usage: document review list [--project P] [--status S]")
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

// wholeDocumentText concatenates every section's raw body, for gist-level
// tagging and tag_density (design decisions 5 and 6): a document is
// findable by tag immediately, without waiting for anyone to draft a gist
// summary.
func wholeDocumentText(sections []knowledge.DocumentSection) string {
	var b strings.Builder
	for _, s := range sections {
		b.WriteString(s.Body)
		b.WriteString("\n")
	}
	return b.String()
}

// tagDensity counts how many known concepts MatchConceptNames finds in
// text (design decision 6) -- the mechanical triage signal, reusing
// concept-tag-retrieval's matcher verbatim rather than a new gazetteer.
func tagDensity(kb *knowledge.KnowledgeBase, text string) (int, error) {
	names, err := kb.MatchConceptNames(text)
	if err != nil {
		return 0, err
	}
	return len(names), nil
}

// tagSection resolves every [[Name]] wikilink in text, plus every entry in
// keywords, into concepts via ResolveConceptName, and links them to
// sectionID (design decision 5) -- the same mechanism and the same
// case-insensitive dedup as linkWikilinkTags (cmd/kb/ingest.go) for
// records, applied one level down. keywords is nil for an ordinary
// section; only the gist row also resolves frontmatter keywords.
func tagSection(kb *knowledge.KnowledgeBase, sectionID int64, text string, keywords []string) error {
	seen := map[string]bool{}
	var names []string
	for _, m := range wikilinkPattern.FindAllStringSubmatch(text, -1) {
		name := strings.TrimSpace(m[1])
		if name != "" && !seen[strings.ToLower(name)] {
			seen[strings.ToLower(name)] = true
			names = append(names, name)
		}
	}
	for _, kw := range keywords {
		name := strings.TrimSpace(kw)
		if name != "" && !seen[strings.ToLower(name)] {
			seen[strings.ToLower(name)] = true
			names = append(names, name)
		}
	}
	for _, name := range names {
		conceptID, err := kb.ResolveConceptName(name)
		if err != nil {
			return err
		}
		if err := kb.LinkDocumentSectionConcept(sectionID, conceptID); err != nil {
			return err
		}
	}
	return nil
}

// ingestNewDocument writes a brand-new document: the documents row, a gist
// row (seeded from parsed.GistSeed when the source frontmatter/title page
// provided one -- design decision 3), and every section, each tagged and
// density-scored as it's written.
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
	gistID, err := kb.AddDocumentSection(gist)
	if err != nil {
		return err
	}

	// Tag every section (and the gist) before computing any density.
	// Tagging can create a concept the document itself introduces for the
	// first time (an explicit [[Name]] wikilink); computing density first
	// would miss that concept entirely, since MatchConceptNames only counts
	// against concepts that already exist (design decision 6).
	sectionIDs := make([]int64, len(parsed.Sections))
	for i, s := range parsed.Sections {
		s.DocumentID = docID
		s.SourceSize = len(strings.Fields(s.Body))
		secID, err := kb.AddDocumentSection(s)
		if err != nil {
			return err
		}
		sectionIDs[i] = secID
		if err := tagSection(kb, secID, s.Body, nil); err != nil {
			return err
		}
	}
	wholeText := wholeDocumentText(parsed.Sections)
	if err := tagSection(kb, gistID, wholeText, parsed.Keywords); err != nil {
		return err
	}

	// Now that every concept this document introduces exists, density
	// reflects the complete vocabulary rather than an ingest-order artifact.
	gistDensity, err := tagDensity(kb, wholeText)
	if err != nil {
		return err
	}
	if err := kb.UpdateDocumentSectionTagDensity(gistID, gistDensity); err != nil {
		return err
	}
	for i, s := range parsed.Sections {
		density, err := tagDensity(kb, s.Body)
		if err != nil {
			return err
		}
		if err := kb.UpdateDocumentSectionTagDensity(sectionIDs[i], density); err != nil {
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
			secID, err := kb.AddDocumentSection(ns)
			if err != nil {
				return err
			}
			// Tag before computing density -- see ingestNewDocument's
			// comment: tagging can introduce a concept this section names
			// for the first time, which density must then be able to see.
			if err := tagSection(kb, secID, ns.Body, nil); err != nil {
				return err
			}
			density, err := tagDensity(kb, ns.Body)
			if err != nil {
				return err
			}
			if err := kb.UpdateDocumentSectionTagDensity(secID, density); err != nil {
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
		if err := kb.UpdateDocumentSectionBody(old.ID, ns.Body, len(strings.Fields(ns.Body)), old.TagDensity, stale); err != nil {
			return err
		}
		// Tags are mechanical and immediate (design decision 5), unlike a
		// summary -- they're refreshed to match the new body. Accretive
		// only, matching kb ingest's own established behavior for records
		// (see knowledge/TODO.md's noted relation-pruning gap): a concept
		// no longer mentioned keeps its old link rather than being pruned.
		// Tag before computing density, same reason as the new-section case
		// above.
		if err := tagSection(kb, old.ID, ns.Body, nil); err != nil {
			return err
		}
		density, err := tagDensity(kb, ns.Body)
		if err != nil {
			return err
		}
		if err := kb.UpdateDocumentSectionTagDensity(old.ID, density); err != nil {
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

	if anyChange {
		wholeText := wholeDocumentText(parsed.Sections)
		if gistRow != nil {
			// Tag before computing density, same reason as the section
			// cases above.
			if err := tagSection(kb, gistRow.ID, wholeText, parsed.Keywords); err != nil {
				return err
			}
			density, err := tagDensity(kb, wholeText)
			if err != nil {
				return err
			}
			if err := kb.UpdateDocumentSectionTagDensity(gistRow.ID, density); err != nil {
				return err
			}
			if gistRow.SummaryStatus != "unsummarized" {
				if err := kb.MarkDocumentSectionStale(gistRow.ID); err != nil {
					return err
				}
			}
		}
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
		return fmt.Errorf("usage: document list [--project P]")
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
		return fmt.Errorf("usage: document show ID")
	}
	docID, err := strconv.ParseInt(positional[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid document id %q", positional[0])
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
