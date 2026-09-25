package knowledge

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Moved from cmd/kb/document.go (library-lift-plan.md L1, DR-0035). The
// bodies of ingestNewDocument, reingestChangedDocument, tagSection,
// tagDensity, densityLinkCandidates, wholeDocumentText and StripCodeSpans are
// a verbatim move: no behavior changed. Only the surrounding orchestration
// (flag parsing, project lookup by name, rendering) stays in cmd/kb.

/** DocumentIngestOptions tunes one IngestDocument call.
 *
 * Fields:
 *   Title  (string) — overrides the title parsed from frontmatter, a
 *                     Fountain title page, or a first H1. Empty accepts
 *                     the parsed title, falling back to the file's base name.
 *   Format (string) — overrides extension-based format detection
 *                     ("markdown", "fountain", "text"). Empty detects.
 *   DryRun (bool)   — report what would happen and write nothing.
 *
 * Example:
 *   opts := knowledge.DocumentIngestOptions{Title: "Field Notes", DryRun: true}
 */
type DocumentIngestOptions struct {
	Title  string
	Format string
	DryRun bool
}

/** DocumentIngestResult is what IngestDocument reports, and is also the
 * JSON shape `kb document ingest --json` prints, so its tags are a
 * compatibility contract. RemovedHeadings lists old sections whose heading
 * no longer matches anything in the re-ingested content: reported, never
 * deleted (narrative-documents-design.md decision 10).
 *
 * Fields:
 *   Path            (string)   — the path as it was passed to IngestDocument.
 *   DryRun          (bool)     — true when nothing was written.
 *   Action          (string)   — "added", "updated", or "skipped" (checksum unchanged).
 *   SectionsAdded   (int)      — sections newly inserted (all of them, for "added").
 *   SectionsUpdated (int)      — matched sections whose body changed.
 *   SectionsStale   (int)      — updated sections whose existing summary is now flagged stale.
 *   RemovedHeadings ([]string) — old headings with no match in the new content.
 *   Warning         (string)   — a non-fatal parser warning, or empty.
 *
 * Example:
 *   res, _ := kb.IngestDocument(pid, "notes.md", knowledge.DocumentIngestOptions{})
 *   fmt.Println(res.Action, res.SectionsAdded)
 */
type DocumentIngestResult struct {
	Path            string   `json:"path"`
	DryRun          bool     `json:"dry_run"`
	Action          string   `json:"action"`
	SectionsAdded   int      `json:"sections_added"`
	SectionsUpdated int      `json:"sections_updated"`
	SectionsStale   int      `json:"sections_stale"`
	RemovedHeadings []string `json:"removed_headings,omitempty"`
	Warning         string   `json:"warning,omitempty"`
}

/** IngestDocument parses the file at path and creates or reconciles its
 * documents row and sections in project projectID. A path not yet stored is
 * ingested fresh; a stored path with an unchanged checksum is skipped; a
 * stored path with a changed checksum is reconciled by matching sections on
 * heading text (an unchanged body is left alone, a changed body updates in
 * place and flags its summary stale, an unmatched old heading is reported
 * and never deleted, an unmatched new heading is inserted unsummarized).
 * Every section and the gist are concept-tagged and density-scored as they
 * are written.
 *
 * The path is stored exactly as given, and DocumentByPath is an exact string
 * match, so a second consumer must pass the same form (relative or absolute)
 * every time or it will ingest the same file twice as two documents.
 *
 * The caller is responsible for confirming projectID exists; unlike the
 * CLI this does not look a project up by name.
 *
 * Parameters:
 *   projectID (int64)                 — the owning project's id.
 *   path      (string)                — the file to ingest.
 *   opts      (DocumentIngestOptions) — title/format overrides and dry-run.
 *
 * Returns:
 *   DocumentIngestResult — what happened (zero value on error).
 *   error                — on an unreadable file, an unsupported format
 *                          (PDF), or a database failure.
 *
 * Example:
 *   res, err := kb.IngestDocument(pid, "stories/a.md", knowledge.DocumentIngestOptions{})
 *   // res.Action == "added"
 */
func (kb *KnowledgeBase) IngestDocument(projectID int64, path string, opts DocumentIngestOptions) (DocumentIngestResult, error) {
	parsed, err := ParseDocumentFile(path, opts.Format)
	if err != nil {
		return DocumentIngestResult{}, err
	}

	docTitle := opts.Title
	if docTitle == "" {
		docTitle = parsed.Title
	}
	if docTitle == "" {
		docTitle = filepath.Base(path)
	}

	summary := DocumentIngestResult{Path: path, DryRun: opts.DryRun, Warning: parsed.Warning}

	existing, err := kb.DocumentByPath(path)
	if err != nil {
		return DocumentIngestResult{}, err
	}

	switch {
	case existing == nil:
		summary.Action = "added"
		summary.SectionsAdded = len(parsed.Sections)
		if !opts.DryRun {
			if err := ingestNewDocument(kb, projectID, path, docTitle, parsed); err != nil {
				return DocumentIngestResult{}, err
			}
		}

	case existing.Checksum == parsed.Checksum:
		summary.Action = "skipped"

	default:
		summary.Action = "updated"
		if !opts.DryRun {
			if err := reingestChangedDocument(kb, existing, docTitle, parsed, &summary); err != nil {
				return DocumentIngestResult{}, err
			}
		}
	}
	return summary, nil
}

// wholeDocumentText concatenates every section's raw body, for gist-level
// tagging and tag_density (design decisions 5 and 6): a document is
// findable by tag immediately, without waiting for anyone to draft a gist
// summary.
func wholeDocumentText(sections []DocumentSection) string {
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
func tagDensity(kb *KnowledgeBase, text string) (int, error) {
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
//
// Clears the section's existing links before re-adding its current set, so a
// wikilink dropped from a re-ingested section is actually dropped from the
// database -- see ClearDocumentSectionConcepts and, for the same gap in
// records, ClearRecordConcepts.
func tagSection(kb *KnowledgeBase, sectionID int64, text string, keywords []string) error {
	if err := kb.ClearDocumentSectionConcepts(sectionID); err != nil {
		return err
	}
	seen := map[string]bool{}
	var names []string
	// A wikilink inside a code span or fenced block is an example of the syntax,
	// not a tag: minting a concept from it polluted the vocabulary with junk such
	// as "..." and "recall: ..." (DR-0037). `kb document tag`, density-linking and
	// fuzzy-tag already exclude code the same way.
	//
	// Each name goes through CleanName first, so a hard-wrapped [[a<newline>b]]
	// is the same concept as [[a b]] (DR-0046). One CleanName refuses, a control
	// character, is not a tag: it is skipped, not allowed to fail the document.
	for _, m := range wikilinkPattern.FindAllStringSubmatch(StripCodeSpans(text), -1) {
		if name, err := CleanName("concept", m[1]); err == nil && !seen[strings.ToLower(name)] {
			seen[strings.ToLower(name)] = true
			names = append(names, name)
		}
	}
	for _, kw := range keywords {
		if name, err := CleanName("concept", kw); err == nil && !seen[strings.ToLower(name)] {
			seen[strings.ToLower(name)] = true
			names = append(names, name)
		}
	}
	densityNames, err := densityLinkCandidates(kb, text, seen)
	if err != nil {
		return err
	}
	names = append(names, densityNames...)
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

// wikilinkPattern matches a [[Name]] inline concept tag in a section body.
// cmd/kb keeps its own copy for record ingest until L5 moves that here.
var wikilinkPattern = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)

// fencedCodeBlockPattern and inlineCodeSpanPattern isolate Markdown code,
// for densityLinkCandidates only -- TagDensity itself is left counting the
// raw text, since it is the signal a threshold is applied to, not the
// threshold's own output (TODO.md's MADR item: "density is the raw signal,
// links are what survived review").
var (
	fencedCodeBlockPattern = regexp.MustCompile("(?s)```.*?```")
	inlineCodeSpanPattern  = regexp.MustCompile("`[^`\n]*`")
)

/** StripCodeSpans removes fenced code blocks and inline code spans from
 * text, replacing each with a single space so word boundaries on either side
 * survive a removed span. It is the one definition of "code" shared by
 * density-linking at ingest and by `kb document tag`'s eligibility check
 * (DR-0027, DR-0029), exported so cmd/kb does not carry a second copy.
 *
 * Parameters:
 *   text (string) — Markdown text.
 *
 * Returns:
 *   string — text with every code block and code span replaced by a space.
 *
 * Example:
 *   knowledge.StripCodeSpans("use `rename` twice") // "use   twice"
 */
func StripCodeSpans(text string) string {
	text = fencedCodeBlockPattern.ReplaceAllString(text, " ")
	text = inlineCodeSpanPattern.ReplaceAllString(text, " ")
	return text
}

// densityLinkCandidates returns known concept names mentioned more than
// once in text, outside of code spans -- TODO.md's MADR item, "threshold +
// code-span exclusion": a document with no [[wikilinks]] or frontmatter
// keywords (an MADR-style ADR is the motivating case) would otherwise link
// no concepts at all, even when its prose clearly and repeatedly names a
// known one. A hand-run prototype found unfiltered density linking
// roughly 60% signal; the two false-positive patterns it found -- a short,
// common concept name colliding with a word used in a different sense
// inside quoted code or an error string, and a one-off incidental mention
// -- are exactly what the code-span exclusion and >1 threshold catch.
//
// A name already in seen (an explicit wikilink or frontmatter keyword) is
// skipped: that is a deliberate signal that bypasses the threshold
// entirely, not one this mechanical pass should second-guess. Unlike a
// wikilink, this can only link to a concept that already exists --
// MatchConceptNameCounts never mints one -- so it connects prose to
// established vocabulary rather than discovering new concepts, which is a
// separate, uncosted question (see TODO.md). Sorted for deterministic
// linking order, since ResolveConceptName's call order should not depend
// on Go's randomized map iteration.
func densityLinkCandidates(kb *KnowledgeBase, text string, seen map[string]bool) ([]string, error) {
	counts, err := kb.MatchConceptNameCounts(StripCodeSpans(text))
	if err != nil {
		return nil, err
	}
	var names []string
	for name, count := range counts {
		if count > 1 && !seen[strings.ToLower(name)] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// ingestNewDocument writes a brand-new document: the documents row, a gist
// row (seeded from parsed.GistSeed when the source frontmatter/title page
// provided one -- design decision 3), and every section, each tagged and
// density-scored as it's written.
func ingestNewDocument(kb *KnowledgeBase, projectID int64, path, title string, parsed *ParsedDocument) error {
	docID, err := kb.AddDocument(Document{
		ProjectID: projectID, Title: title, Format: parsed.Format, Path: path,
		Author: parsed.Author, PublishedDate: parsed.PublishedDate, Checksum: parsed.Checksum,
	})
	if err != nil {
		return err
	}

	gist := DocumentSection{DocumentID: docID, Level: "gist"}
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
func reingestChangedDocument(kb *KnowledgeBase, existing *Document, title string, parsed *ParsedDocument, summary *DocumentIngestResult) error {
	if err := kb.UpdateDocumentMetadata(existing.ID, title, parsed.Author, parsed.PublishedDate, parsed.Checksum); err != nil {
		return err
	}
	oldSections, err := kb.DocumentSections(existing.ID)
	if err != nil {
		return err
	}
	oldByHeading := map[string]DocumentSection{}
	var gistRow *DocumentSection
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
	// In the old document's own order, each heading once, so the report does not
	// depend on map iteration order (DR-0037). oldByHeading collapses a repeated
	// heading to one entry, hence the seen set.
	removedSeen := map[string]bool{}
	for _, s := range oldSections {
		if s.Level == "gist" || matchedHeadings[s.Heading] || removedSeen[s.Heading] {
			continue
		}
		removedSeen[s.Heading] = true
		summary.RemovedHeadings = append(summary.RemovedHeadings, s.Heading)
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
