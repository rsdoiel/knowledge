package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// seedReviewedSummary writes a reviewed summary directly to the database,
// standing in for `kb document draft` + `kb document review promote`
// (W5, not yet implemented) so W3's re-ingest reconciliation can be tested
// against a section that already has one. Opens its own connection to
// kb.Path() rather than reaching into KnowledgeBase's unexported db field.
func seedReviewedSummary(t *testing.T, kb *knowledge.KnowledgeBase, sectionID int64, body string) {
	t.Helper()
	db, err := sql.Open("sqlite", kb.Path())
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(
		`UPDATE document_sections SET summary_body = ?, summary_status = 'reviewed' WHERE id = ?`,
		body, sectionID,
	); err != nil {
		t.Fatalf("seed reviewed summary: %v", err)
	}
}

func runDocument(t *testing.T, kb *knowledge.KnowledgeBase, jsonOut bool, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := cmdDocument(kb, nil, jsonOut, args, &out)
	return out.String(), err
}

func writeDocFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

const antennaFixture = `---
title: "A Test Post"
description: "A short summary for RSS and search engines."
pubDate: "2026-09-13"
author: "R. S. Doiel"
keywords: ["chunking"]
---
The body of the post.
`

func TestDocumentIngest_CreatesDocumentAndSections(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	if _, err := kb.AddProject("alpha", ""); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	path := writeDocFixture(t, root+"/docs", "post.md", antennaFixture)

	out, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha")
	if err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	if !strings.Contains(out, "added") {
		t.Errorf("out = %q, want it to report added", out)
	}
	d, err := kb.DocumentByPath(path)
	if err != nil || d == nil {
		t.Fatalf("DocumentByPath: %v", err)
	}
	if d.Title != "A Test Post" || d.Author != "R. S. Doiel" {
		t.Errorf("d = %+v", d)
	}
	sections, err := kb.DocumentSections(d.ID)
	if err != nil {
		t.Fatalf("DocumentSections: %v", err)
	}
	if len(sections) != 2 { // gist + one section
		t.Fatalf("sections = %+v, want 2 (gist + one)", sections)
	}
	var gist *knowledge.DocumentSection
	for i := range sections {
		if sections[i].Level == "gist" {
			gist = &sections[i]
		}
	}
	if gist == nil || gist.SummaryStatus != "drafted" || gist.SummaryBody == "" {
		t.Errorf("gist = %+v, want a drafted summary seeded from frontmatter description", gist)
	}
}

func TestDocumentIngest_TitleFromFrontmatterWhenFlagOmitted(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "post.md", antennaFixture)
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	if d.Title != "A Test Post" {
		t.Errorf("Title = %q", d.Title)
	}
}

func TestDocumentIngest_TitleFlagOverridesFrontmatter(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "post.md", antennaFixture)
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha", "--title", "Overridden"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	if d.Title != "Overridden" {
		t.Errorf("Title = %q, want the --title flag to win", d.Title)
	}
}

func TestDocumentIngest_UnchangedFileIsSkipped(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "note.txt", "some notes\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	out, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha")
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if !strings.Contains(out, "skipped") {
		t.Errorf("out = %q, want skipped for an unchanged file", out)
	}
}

func TestDocumentIngest_MatchedHeadingUnchangedBodyPreservesSummary(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	content := "# One\ntext one\n"
	path := writeDocFixture(t, root+"/docs", "a.md", content)
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	sections, _ := kb.DocumentSections(d.ID)
	var target knowledge.DocumentSection
	for _, s := range sections {
		if s.Level == "section" {
			target = s
		}
	}
	seedReviewedSummary(t, kb, target.ID, "seeded")

	// Re-ingest the identical file: nothing about this section should change.
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	sections, _ = kb.DocumentSections(d.ID)
	for _, s := range sections {
		if s.Level == "section" {
			if s.SummaryStatus != "reviewed" || s.SummaryBody != "seeded" || s.SummaryStale {
				t.Errorf("section = %+v, want the reviewed summary preserved and not stale", s)
			}
		}
	}
}

func TestDocumentIngest_MatchedHeadingChangedBodySetsStale(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.md", "# One\noriginal text\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	sections, _ := kb.DocumentSections(d.ID)
	var target knowledge.DocumentSection
	for _, s := range sections {
		if s.Level == "section" {
			target = s
		}
	}
	seedReviewedSummary(t, kb, target.ID, "seeded")

	writeDocFixture(t, root+"/docs", "a.md", "# One\nchanged text\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	sections, _ = kb.DocumentSections(d.ID)
	for _, s := range sections {
		if s.Level == "section" {
			if !s.SummaryStale {
				t.Errorf("section = %+v, want summary_stale after body changed under a reviewed summary", s)
			}
			if s.SummaryBody != "seeded" {
				t.Errorf("section = %+v, want the summary preserved, not deleted", s)
			}
			if !strings.Contains(s.Body, "changed text") {
				t.Errorf("section = %+v, want the body actually updated", s)
			}
		}
	}
}

func TestDocumentIngest_RemovedHeadingIsReportedNotDeleted(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.md", "# One\ntext one\n## Two\ntext two\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)

	writeDocFixture(t, root+"/docs", "a.md", "# One\ntext one\n")
	out, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha")
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if !strings.Contains(out, "Two") {
		t.Errorf("out = %q, want the removed heading reported", out)
	}
	sections, _ := kb.DocumentSections(d.ID)
	found := false
	for _, s := range sections {
		if s.Heading == "Two" {
			found = true
		}
	}
	if !found {
		t.Error("expected the 'Two' section to still exist in the database, not deleted")
	}
}

func TestDocumentIngest_NewHeadingInsertsUnsummarizedSection(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.md", "# One\ntext one\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)

	writeDocFixture(t, root+"/docs", "a.md", "# One\ntext one\n## Two\ntext two\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	sections, _ := kb.DocumentSections(d.ID)
	found := false
	for _, s := range sections {
		if s.Heading == "Two" && s.SummaryStatus == "unsummarized" {
			found = true
		}
	}
	if !found {
		t.Errorf("sections = %+v, want a new unsummarized section for Two", sections)
	}
}

func TestDocumentIngest_RejectsPDFExplicitly(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.pdf", "%PDF-1.4 fake")
	_, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha")
	if err == nil {
		t.Fatal("expected an error for a .pdf document")
	}
}

func TestDocumentIngest_DryRunWritesNothing(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.txt", "some notes\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha", "--dry-run"); err != nil {
		t.Fatalf("document ingest --dry-run: %v", err)
	}
	d, err := kb.DocumentByPath(path)
	if err != nil {
		t.Fatalf("DocumentByPath: %v", err)
	}
	if d != nil {
		t.Errorf("DocumentByPath = %+v, want nil after a dry run", d)
	}
}

func TestDocumentIngest_JSONOutput(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.txt", "some notes\n")
	out, err := runDocument(t, kb, true, "ingest", path, "--project", "alpha")
	if err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	var summary documentIngestSummary
	if jerr := json.Unmarshal([]byte(out), &summary); jerr != nil {
		t.Fatalf("decoding %s: %v", out, jerr)
	}
	if summary.Action != "added" {
		t.Errorf("summary = %+v, want Action=added", summary)
	}
}

// ─── W4: concept tagging ─────────────────────────────────────────────────────

func TestDocumentIngest_WikilinkInSectionBodyBecomesConcept(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.md", "# One\nSee [[Foo]] for background.\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	sections, _ := kb.DocumentSections(d.ID)
	var section knowledge.DocumentSection
	for _, s := range sections {
		if s.Level == "section" {
			section = s
		}
	}
	concepts, err := kb.DocumentSectionConcepts(section.ID)
	if err != nil {
		t.Fatalf("DocumentSectionConcepts: %v", err)
	}
	if len(concepts) != 1 || concepts[0].Name != "Foo" {
		t.Errorf("concepts = %+v, want one concept named Foo", concepts)
	}
}

// TODO.md "document_section_concepts should be checked for the same gap":
// tagSection has the same insert-only defect as linkWikilinkTags for records.
func TestDocumentIngest_RemovedWikilinkConceptIsPrunedOnReingest(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.md", "# One\nSee [[Foo]] for background.\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	sections, _ := kb.DocumentSections(d.ID)
	var section knowledge.DocumentSection
	for _, s := range sections {
		if s.Level == "section" {
			section = s
		}
	}
	if concepts, _ := kb.DocumentSectionConcepts(section.ID); len(concepts) != 1 {
		t.Fatalf("concepts before removal = %+v, want 1", concepts)
	}

	writeDocFixture(t, root+"/docs", "a.md", "# One\nNo more mentions.\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest (2nd): %v", err)
	}

	concepts, err := kb.DocumentSectionConcepts(section.ID)
	if err != nil {
		t.Fatalf("DocumentSectionConcepts: %v", err)
	}
	if len(concepts) != 0 {
		t.Errorf("concepts after removal = %+v, want none — a wikilink dropped from the section should not survive re-ingest", concepts)
	}
}

func TestDocumentIngest_GistTaggedFromWholeDocumentText(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.md", "# One\n[[Foo]]\n# Two\n[[Bar]]\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	sections, _ := kb.DocumentSections(d.ID)
	var gist knowledge.DocumentSection
	for _, s := range sections {
		if s.Level == "gist" {
			gist = s
		}
	}
	concepts, err := kb.DocumentSectionConcepts(gist.ID)
	if err != nil {
		t.Fatalf("DocumentSectionConcepts: %v", err)
	}
	names := map[string]bool{}
	for _, c := range concepts {
		names[c.Name] = true
	}
	if len(concepts) != 2 || !names["Foo"] || !names["Bar"] {
		t.Errorf("gist concepts = %+v, want Foo and Bar from both sections", concepts)
	}
}

func TestDocumentIngest_FrontmatterKeywordsBecomeGistConcepts(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "post.md", antennaFixture)
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	sections, _ := kb.DocumentSections(d.ID)
	var gist knowledge.DocumentSection
	for _, s := range sections {
		if s.Level == "gist" {
			gist = s
		}
	}
	concepts, err := kb.DocumentSectionConcepts(gist.ID)
	if err != nil {
		t.Fatalf("DocumentSectionConcepts: %v", err)
	}
	found := false
	for _, c := range concepts {
		if c.Name == "chunking" {
			found = true
		}
	}
	if !found {
		t.Errorf("gist concepts = %+v, want the frontmatter keyword chunking", concepts)
	}
}

func TestDocumentIngest_TagDensityReflectsMatchConceptNamesCount(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("Foo", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	path := writeDocFixture(t, root+"/docs", "a.md", "# One\nMentions Foo here.\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	sections, _ := kb.DocumentSections(d.ID)
	for _, s := range sections {
		if s.Level == "section" && s.TagDensity != 1 {
			t.Errorf("section TagDensity = %d, want 1 (matches existing concept Foo)", s.TagDensity)
		}
	}
}

// Regression: found via manual smoke test, not a unit test. TagDensity must
// be computed after tagging, not before -- otherwise a concept the document
// introduces for the first time via its own [[wikilink]] doesn't exist yet
// when density is computed, and the gist (or a section whose own wikilink
// names the concept) undercounts its own tag.
func TestDocumentIngest_GistDensityCountsSelfIntroducedConcept(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	// No concept named Foo exists anywhere before this ingest -- the
	// [[Foo]] wikilink is what creates it.
	path := writeDocFixture(t, root+"/docs", "a.md", "# One\nSee [[Foo]] for background.\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	sections, _ := kb.DocumentSections(d.ID)
	for _, s := range sections {
		if s.TagDensity != 1 {
			t.Errorf("section (level=%s) TagDensity = %d, want 1 -- density must see the concept this document just introduced", s.Level, s.TagDensity)
		}
	}
}

// ─── density-based concept linking (TODO.md's MADR item) ───────────────────

func TestDocumentIngest_DensityLinkingRequiresMoreThanOneOccurrence(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("Bar", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	path := writeDocFixture(t, root+"/docs", "a.md", "# One\nBar showed up here, then bar came up again later.\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	sections, _ := kb.DocumentSections(d.ID)
	var sec knowledge.DocumentSection
	for _, s := range sections {
		if s.Level == "section" {
			sec = s
		}
	}
	concepts, err := kb.DocumentSectionConcepts(sec.ID)
	if err != nil {
		t.Fatalf("DocumentSectionConcepts: %v", err)
	}
	found := false
	for _, c := range concepts {
		if c.Name == "Bar" {
			found = true
		}
	}
	if !found {
		t.Errorf("section concepts = %+v, want Bar auto-linked from two mentions", concepts)
	}
}

func TestDocumentIngest_DensityLinkingDoesNotLinkOnSingleMention(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("Foo", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	path := writeDocFixture(t, root+"/docs", "a.md", "# One\nMentions Foo here, just once.\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	sections, _ := kb.DocumentSections(d.ID)
	var sec knowledge.DocumentSection
	for _, s := range sections {
		if s.Level == "section" {
			sec = s
		}
	}
	concepts, err := kb.DocumentSectionConcepts(sec.ID)
	if err != nil {
		t.Fatalf("DocumentSectionConcepts: %v", err)
	}
	for _, c := range concepts {
		if c.Name == "Foo" {
			t.Errorf("section concepts = %+v, want Foo NOT auto-linked from a single mention", concepts)
		}
	}
	// TagDensity itself is unaffected by the threshold -- it's the raw
	// signal, links are what survived the filter (TODO.md's own framing).
	if sec.TagDensity != 1 {
		t.Errorf("TagDensity = %d, want 1 (the threshold gates linking, not the density metric)", sec.TagDensity)
	}
}

func TestDocumentIngest_DensityLinkingExcludesInlineCodeSpans(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("format", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	path := writeDocFixture(t, root+"/docs", "a.md", "# One\nSaw `unsupported format` once, then `const format = x` again.\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	sections, _ := kb.DocumentSections(d.ID)
	var sec knowledge.DocumentSection
	for _, s := range sections {
		if s.Level == "section" {
			sec = s
		}
	}
	concepts, err := kb.DocumentSectionConcepts(sec.ID)
	if err != nil {
		t.Fatalf("DocumentSectionConcepts: %v", err)
	}
	for _, c := range concepts {
		if c.Name == "format" {
			t.Errorf("section concepts = %+v, want format NOT linked -- both mentions are inside inline code spans", concepts)
		}
	}
}

func TestDocumentIngest_DensityLinkingExcludesFencedCodeBlocks(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("git", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	body := "# One\n```\ngit push\ngit pull\n```\n"
	path := writeDocFixture(t, root+"/docs", "a.md", body)
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	sections, _ := kb.DocumentSections(d.ID)
	var sec knowledge.DocumentSection
	for _, s := range sections {
		if s.Level == "section" {
			sec = s
		}
	}
	concepts, err := kb.DocumentSectionConcepts(sec.ID)
	if err != nil {
		t.Fatalf("DocumentSectionConcepts: %v", err)
	}
	for _, c := range concepts {
		if c.Name == "git" {
			t.Errorf("section concepts = %+v, want git NOT linked -- both mentions are inside a fenced code block", concepts)
		}
	}
}

// An explicit [[wikilink]] is a deliberate signal, so it bypasses the
// density threshold entirely -- unlike density-inferred links, it also
// mints a brand-new concept, which MatchConceptNameCounts could never do.
func TestDocumentIngest_ExplicitWikilinkBypassesDensityThreshold(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.md", "# One\nSee [[Widget]] once.\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	sections, _ := kb.DocumentSections(d.ID)
	var sec knowledge.DocumentSection
	for _, s := range sections {
		if s.Level == "section" {
			sec = s
		}
	}
	concepts, err := kb.DocumentSectionConcepts(sec.ID)
	if err != nil {
		t.Fatalf("DocumentSectionConcepts: %v", err)
	}
	found := false
	for _, c := range concepts {
		if c.Name == "Widget" {
			found = true
		}
	}
	if !found {
		t.Errorf("section concepts = %+v, want Widget linked from a single explicit wikilink mention", concepts)
	}
}

func TestDocumentIngest_ReingestUnchangedDoesNotDuplicateLinks(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.md", "# One\nSee [[Foo]].\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	d, _ := kb.DocumentByPath(path)
	sections, _ := kb.DocumentSections(d.ID)
	for _, s := range sections {
		if s.Level == "section" {
			concepts, err := kb.DocumentSectionConcepts(s.ID)
			if err != nil {
				t.Fatalf("DocumentSectionConcepts: %v", err)
			}
			if len(concepts) != 1 {
				t.Errorf("concepts = %+v, want exactly 1 after re-ingesting an unchanged file", concepts)
			}
		}
	}
}

// ─── W5: review workflow CLI ─────────────────────────────────────────────────

func seedIngestedSection(t *testing.T, kb *knowledge.KnowledgeBase, root string) int64 {
	t.Helper()
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.txt", "some notes\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, err := kb.DocumentByPath(path)
	if err != nil || d == nil {
		t.Fatalf("DocumentByPath: %v", err)
	}
	sections, err := kb.DocumentSections(d.ID)
	if err != nil || len(sections) == 0 {
		t.Fatalf("DocumentSections: %v", err)
	}
	return sections[0].ID
}

func TestDocumentDraft_SetsStatusAndClearsStale(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	secID := seedIngestedSection(t, kb, root)
	out, err := runDocument(t, kb, false, "draft", fmt.Sprint(secID), "a drafted summary", "--by", "human", "--confidence", "0.9")
	if err != nil {
		t.Fatalf("document draft: %v", err)
	}
	if !strings.Contains(out, "drafted") {
		t.Errorf("out = %q, want confirmation", out)
	}
}

func TestDocumentDraft_RejectsConfidenceOutOfRange(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	secID := seedIngestedSection(t, kb, root)
	_, err := runDocument(t, kb, false, "draft", fmt.Sprint(secID), "a summary", "--by", "human", "--confidence", "1.5")
	if err == nil {
		t.Fatal("expected an error for confidence out of [0,1]")
	}
}

func TestDocumentReviewPromote_IndexesInFTS(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	secID := seedIngestedSection(t, kb, root)
	if _, err := runDocument(t, kb, false, "draft", fmt.Sprint(secID), "a distinctive promoted summary", "--by", "human"); err != nil {
		t.Fatalf("document draft: %v", err)
	}
	out, err := runDocument(t, kb, false, "review", "promote", fmt.Sprint(secID))
	if err != nil {
		t.Fatalf("document review promote: %v", err)
	}
	if !strings.Contains(out, "reviewed") {
		t.Errorf("out = %q, want confirmation", out)
	}
	results, err := kb.Search("distinctive promoted summary")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected the promoted summary to be searchable")
	}
}

func TestDocumentReviewPromote_RequiresDraftedFirst(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	secID := seedIngestedSection(t, kb, root)
	_, err := runDocument(t, kb, false, "review", "promote", fmt.Sprint(secID))
	if err == nil {
		t.Fatal("expected an error promoting an unsummarized section")
	}
}

func TestDocumentReviewList_ShowsUnsummarizedAndDrafted(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	secID := seedIngestedSection(t, kb, root)
	out, err := runDocument(t, kb, false, "review", "list", "--project", "alpha")
	if err != nil {
		t.Fatalf("document review list: %v", err)
	}
	if !strings.Contains(out, fmt.Sprint(secID)) {
		t.Errorf("out = %q, want the unsummarized section listed", out)
	}
}

func TestDocumentReviewList_JSONOutput(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	seedIngestedSection(t, kb, root)
	out, err := runDocument(t, kb, true, "review", "list", "--project", "alpha")
	if err != nil {
		t.Fatalf("document review list: %v", err)
	}
	var items []knowledge.DocumentReviewItem
	if jerr := json.Unmarshal([]byte(out), &items); jerr != nil {
		t.Fatalf("decoding %s: %v", out, jerr)
	}
	if len(items) == 0 {
		t.Error("expected at least one review item")
	}
}

// ─── W8: kb document list / show ─────────────────────────────────────────────

func TestDocumentList_ShowsIngestedDocument(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	seedIngestedSection(t, kb, root)
	out, err := runDocument(t, kb, false, "list", "--project", "alpha")
	if err != nil {
		t.Fatalf("document list: %v", err)
	}
	if !strings.Contains(out, "a.txt") {
		t.Errorf("out = %q, want the ingested document listed", out)
	}
}

func TestDocumentList_JSONOutput(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	seedIngestedSection(t, kb, root)
	out, err := runDocument(t, kb, true, "list", "--project", "alpha")
	if err != nil {
		t.Fatalf("document list: %v", err)
	}
	var docs []knowledge.Document
	if jerr := json.Unmarshal([]byte(out), &docs); jerr != nil {
		t.Fatalf("decoding %s: %v", out, jerr)
	}
	if len(docs) != 1 {
		t.Errorf("docs = %+v, want 1", docs)
	}
}

func TestDocumentShow_IncludesSectionsAndConcepts(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.md", "# One\nSee [[Foo]].\n")
	if _, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha"); err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	d, err := kb.DocumentByPath(path)
	if err != nil || d == nil {
		t.Fatalf("DocumentByPath: %v", err)
	}
	out, err := runDocument(t, kb, false, "show", fmt.Sprint(d.ID))
	if err != nil {
		t.Fatalf("document show: %v", err)
	}
	if !strings.Contains(out, "One") {
		t.Errorf("out = %q, want the section heading", out)
	}
	if !strings.Contains(out, "Foo") {
		t.Errorf("out = %q, want the linked concept", out)
	}
}

func TestDocumentShow_UnknownIDErrors(t *testing.T) {
	kb, _ := openWorkspaceKB(t)
	_, err := runDocument(t, kb, false, "show", "9999")
	if err == nil {
		t.Fatal("expected an error for an unknown document id")
	}
}
