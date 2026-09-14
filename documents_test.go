package knowledge

import (
	"os"
	"strings"
	"testing"
)

// ─── W1: documents / document_sections core ─────────────────────────────────

func TestAddDocument_CreatesRow(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	id, err := kb.AddDocument(Document{
		ProjectID: pid, Title: "A Story", Format: "markdown", Path: "stories/a-story.md",
	})
	if err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if id == 0 {
		t.Error("AddDocument returned id 0")
	}
}

func TestAddDocumentSection_CreatesRow(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "A Story", Format: "text", Path: "a.txt"})
	secID, err := kb.AddDocumentSection(DocumentSection{
		DocumentID: docID, Level: "section", Seq: 0, Body: "some text",
	})
	if err != nil {
		t.Fatalf("AddDocumentSection: %v", err)
	}
	if secID == 0 {
		t.Error("AddDocumentSection returned id 0")
	}
}

// Required for JSON-L import to preserve cross-machine identity, the same
// way AddRecord already does: a given non-empty UUID must survive, not be
// silently overwritten by a freshly generated one.
func TestAddDocument_PreservesGivenUUID(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, err := kb.AddDocument(Document{
		ProjectID: pid, Title: "A Story", Format: "text", Path: "a.txt", UUID: "given-uuid",
	})
	if err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	d, err := kb.DocumentByPath("a.txt")
	if err != nil {
		t.Fatalf("DocumentByPath: %v", err)
	}
	if d.UUID != "given-uuid" {
		t.Errorf("UUID = %q, want the given uuid preserved", d.UUID)
	}
	_ = docID
}

func TestAddDocumentSection_PreservesGivenUUID(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "A Story", Format: "text", Path: "a.txt"})
	if _, err := kb.AddDocumentSection(DocumentSection{
		DocumentID: docID, Level: "section", UUID: "given-section-uuid",
	}); err != nil {
		t.Fatalf("AddDocumentSection: %v", err)
	}
	sections, err := kb.DocumentSections(docID)
	if err != nil {
		t.Fatalf("DocumentSections: %v", err)
	}
	if len(sections) != 1 || sections[0].UUID != "given-section-uuid" {
		t.Errorf("sections = %+v, want the given uuid preserved", sections)
	}
}

func TestDocumentSections_OrdersGistFirstThenBySeq(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "A Story", Format: "text", Path: "a.txt"})
	if _, err := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section", Seq: 1, Heading: "Two"}); err != nil {
		t.Fatalf("AddDocumentSection seq1: %v", err)
	}
	if _, err := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section", Seq: 0, Heading: "One"}); err != nil {
		t.Fatalf("AddDocumentSection seq0: %v", err)
	}
	if _, err := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "gist"}); err != nil {
		t.Fatalf("AddDocumentSection gist: %v", err)
	}

	sections, err := kb.DocumentSections(docID)
	if err != nil {
		t.Fatalf("DocumentSections: %v", err)
	}
	if len(sections) != 3 {
		t.Fatalf("sections = %+v, want 3", sections)
	}
	if sections[0].Level != "gist" {
		t.Errorf("sections[0] = %+v, want the gist row first", sections[0])
	}
	if sections[1].Heading != "One" || sections[2].Heading != "Two" {
		t.Errorf("sections = %+v, want One (seq 0) before Two (seq 1)", sections)
	}
}

func TestDocumentByPath_FindsExisting(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddDocument(Document{ProjectID: pid, Title: "A Story", Format: "text", Path: "a.txt"}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	d, err := kb.DocumentByPath("a.txt")
	if err != nil {
		t.Fatalf("DocumentByPath: %v", err)
	}
	if d == nil || d.Title != "A Story" {
		t.Errorf("DocumentByPath = %+v, want the seeded document", d)
	}
}

func TestDocumentByPath_UnknownPathReturnsNotFound(t *testing.T) {
	kb := openTestKB(t)
	d, err := kb.DocumentByPath("does-not-exist.txt")
	if err != nil {
		t.Fatalf("DocumentByPath: %v", err)
	}
	if d != nil {
		t.Errorf("DocumentByPath = %+v, want nil for an unknown path", d)
	}
}

func TestDocumentByID_FindsExisting(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "A Story", Format: "text", Path: "a.txt"})
	d, err := kb.DocumentByID(docID)
	if err != nil {
		t.Fatalf("DocumentByID: %v", err)
	}
	if d == nil || d.Title != "A Story" {
		t.Errorf("DocumentByID = %+v, want the seeded document", d)
	}
}

func TestDocumentByID_UnknownIDReturnsNotFound(t *testing.T) {
	kb := openTestKB(t)
	d, err := kb.DocumentByID(9999)
	if err != nil {
		t.Fatalf("DocumentByID: %v", err)
	}
	if d != nil {
		t.Errorf("DocumentByID = %+v, want nil for an unknown id", d)
	}
}

func TestDocumentSections_CascadesOnDocumentDelete(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "A Story", Format: "text", Path: "a.txt"})
	if _, err := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section"}); err != nil {
		t.Fatalf("AddDocumentSection: %v", err)
	}
	if _, err := kb.db.Exec(`DELETE FROM documents WHERE id = ?`, docID); err != nil {
		t.Fatalf("delete document: %v", err)
	}
	var count int
	if err := kb.db.QueryRow(`SELECT COUNT(*) FROM document_sections WHERE document_id = ?`, docID).Scan(&count); err != nil {
		t.Fatalf("count document_sections: %v", err)
	}
	if count != 0 {
		t.Errorf("expected document_sections to cascade-delete with its document, got %d rows", count)
	}
}

// ─── W2: segmentation and frontmatter extraction ────────────────────────────

func TestDetectFormat_FromExtension(t *testing.T) {
	cases := map[string]string{
		"a.md": "markdown", "a.markdown": "markdown",
		"a.fountain": "fountain", "a.spmd": "fountain",
		"a.txt": "text", "a": "text",
		"a.pdf": "pdf",
	}
	for path, want := range cases {
		if got := detectFormat(path); got != want {
			t.Errorf("detectFormat(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestSegmentMarkdown_SplitsOnHeadings(t *testing.T) {
	sections := segmentMarkdown("# One\ntext one\n## Two\ntext two\n")
	if len(sections) != 2 {
		t.Fatalf("sections = %+v, want 2", sections)
	}
	if sections[0].Heading != "One" || !strings.Contains(sections[0].Body, "text one") {
		t.Errorf("sections[0] = %+v", sections[0])
	}
	if sections[1].Heading != "Two" || !strings.Contains(sections[1].Body, "text two") {
		t.Errorf("sections[1] = %+v", sections[1])
	}
}

func TestSegmentMarkdown_NoHeadingsIsOneSection(t *testing.T) {
	sections := segmentMarkdown("just some plain prose, no headings at all\n")
	if len(sections) != 1 || sections[0].Heading != "" {
		t.Errorf("sections = %+v, want one section with no heading", sections)
	}
}

const fountainFixture = `Title: A Short Scene
Author: Test Author

INT. LAB - DAY

Charlie works at the bench.

CHARLIE
Bring that to me.

EXT. STREET - NIGHT

Wilma walks away.
`

func TestSegmentFountain_SplitsOnSceneHeadings(t *testing.T) {
	sections, _, err := segmentFountain([]byte(fountainFixture))
	if err != nil {
		t.Fatalf("segmentFountain: %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("sections = %+v, want 2", sections)
	}
	if sections[0].Heading != "INT. LAB - DAY" || !strings.Contains(sections[0].Body, "Bring that to me") {
		t.Errorf("sections[0] = %+v", sections[0])
	}
	if sections[1].Heading != "EXT. STREET - NIGHT" || !strings.Contains(sections[1].Body, "Wilma walks away") {
		t.Errorf("sections[1] = %+v", sections[1])
	}
}

func TestSegmentFountain_ExtractsTitlePage(t *testing.T) {
	_, titlePage, err := segmentFountain([]byte(fountainFixture))
	if err != nil {
		t.Fatalf("segmentFountain: %v", err)
	}
	if titlePage["Title"] != "A Short Scene" {
		t.Errorf("titlePage[Title] = %q, want %q", titlePage["Title"], "A Short Scene")
	}
	if titlePage["Author"] != "Test Author" {
		t.Errorf("titlePage[Author] = %q, want %q", titlePage["Author"], "Test Author")
	}
}

// Regression: standard Fountain reserves [[double-brackets]] for its own
// Notes syntax (confirmed against github.com/rsdoiel/fountain's isNoteStart/
// isNoteEnd -- a [[...]] line is classified NoteType, not Action/Dialogue).
// Harvey's own session-recording convention uses exactly this syntax for
// file events ([[write: path -- ok]], [[read: path -- ok]]). Before this
// fix, segmentFountain concatenated every non-scene-heading element's raw
// text into the section body regardless of type, so a Note's literal
// "[[...]]" text would survive into the body and then get re-matched by
// wikilink tagging as a bogus concept name. Notes/Sections/Synopses/
// Boneyard are annotations, not narrative prose, and must be excluded from
// the body the same way the fountain library's own String()/ToHTML()
// treat them as separate, opt-in content.
func TestSegmentFountain_ExcludesNotesFromBody(t *testing.T) {
	src := `Title: A Session
Author: Test

INT. AGENT MODE 2026-05-08 11:45:00

HARVEY
Harvey modified the following files during this session.

[[write: harvey/rag_support.go -- ok]]
`
	sections, _, err := segmentFountain([]byte(src))
	if err != nil {
		t.Fatalf("segmentFountain: %v", err)
	}
	if len(sections) != 1 {
		t.Fatalf("sections = %+v, want 1", sections)
	}
	if strings.Contains(sections[0].Body, "[[") || strings.Contains(sections[0].Body, "write:") {
		t.Errorf("Body = %q, want the [[write: ...]] note excluded entirely", sections[0].Body)
	}
	if !strings.Contains(sections[0].Body, "Harvey modified the following files") {
		t.Errorf("Body = %q, want the real dialogue preserved", sections[0].Body)
	}
}

func TestSegmentText_IsAlwaysOneSection(t *testing.T) {
	sections := segmentText("some plain text\nwith multiple lines\n")
	if len(sections) != 1 || sections[0].Seq != 0 || sections[0].Heading != "" {
		t.Errorf("sections = %+v, want exactly one section, seq 0, no heading", sections)
	}
}

const antennaFrontmatterFixture = `---
title: "A Test Post"
description: "A short summary for RSS and search engines."
pubDate: "2026-09-13"
author: "R. S. Doiel"
keywords: ["chunking", "small-models"]
---
The body of the post.
`

func TestExtractFrontmatter_ParsesAntennaAppFields(t *testing.T) {
	fields, keywords, body, warning := extractFrontmatter([]byte(antennaFrontmatterFixture))
	if warning != "" {
		t.Fatalf("unexpected warning: %s", warning)
	}
	if fields["title"] != "A Test Post" {
		t.Errorf("title = %q", fields["title"])
	}
	if fields["description"] != "A short summary for RSS and search engines." {
		t.Errorf("description = %q", fields["description"])
	}
	if fields["published_date"] != "2026-09-13" {
		t.Errorf("published_date = %q", fields["published_date"])
	}
	if fields["author"] != "R. S. Doiel" {
		t.Errorf("author = %q", fields["author"])
	}
	if len(keywords) != 2 || keywords[0] != "chunking" || keywords[1] != "small-models" {
		t.Errorf("keywords = %v", keywords)
	}
	if !strings.Contains(body, "The body of the post.") {
		t.Errorf("body = %q, want it to contain the post body", body)
	}
}

func TestExtractFrontmatter_MissingFrontmatterIsNotAWarning(t *testing.T) {
	fields, keywords, body, warning := extractFrontmatter([]byte("just prose, no frontmatter\n"))
	if warning != "" {
		t.Errorf("warning = %q, want none -- missing frontmatter is normal for documents", warning)
	}
	if len(fields) != 0 || len(keywords) != 0 {
		t.Errorf("fields=%v keywords=%v, want none", fields, keywords)
	}
	if !strings.Contains(body, "just prose") {
		t.Errorf("body = %q, want the whole input preserved", body)
	}
}

func TestExtractFrontmatter_UnterminatedFrontmatterWarnsNotFails(t *testing.T) {
	_, _, body, warning := extractFrontmatter([]byte("---\ntitle: \"Oops\"\nno closing fence\n"))
	if warning == "" {
		t.Error("expected a warning for an unterminated frontmatter block")
	}
	if body == "" {
		t.Error("expected the whole input to still be usable as body")
	}
}

func TestExtractFrontmatter_IgnoresUnrecognizedKeys(t *testing.T) {
	fields, _, _, warning := extractFrontmatter([]byte("---\ntitle: \"Fine\"\nsomeUnknownKey: [1, 2, 3]\n---\nbody\n"))
	if warning != "" {
		t.Errorf("unexpected warning: %s", warning)
	}
	if fields["title"] != "Fine" {
		t.Errorf("title = %q", fields["title"])
	}
}

// ─── W3: ParseDocumentFile ───────────────────────────────────────────────────

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := t.TempDir() + "/" + name
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestParseDocumentFile_Markdown_UsesFrontmatter(t *testing.T) {
	path := writeTempFile(t, "post.md", antennaFrontmatterFixture)
	pd, err := ParseDocumentFile(path, "")
	if err != nil {
		t.Fatalf("ParseDocumentFile: %v", err)
	}
	if pd.Format != "markdown" {
		t.Errorf("Format = %q", pd.Format)
	}
	if pd.Title != "A Test Post" || pd.Author != "R. S. Doiel" || pd.PublishedDate != "2026-09-13" {
		t.Errorf("pd = %+v", pd)
	}
	if pd.GistSeed == "" {
		t.Error("expected GistSeed from frontmatter description")
	}
	if len(pd.Sections) != 1 {
		t.Errorf("Sections = %+v, want one (no headings in the fixture body)", pd.Sections)
	}
}

func TestParseDocumentFile_Fountain_UsesTitlePage(t *testing.T) {
	path := writeTempFile(t, "scene.fountain", fountainFixture)
	pd, err := ParseDocumentFile(path, "")
	if err != nil {
		t.Fatalf("ParseDocumentFile: %v", err)
	}
	if pd.Format != "fountain" {
		t.Errorf("Format = %q", pd.Format)
	}
	if pd.Title != "A Short Scene" || pd.Author != "Test Author" {
		t.Errorf("pd = %+v", pd)
	}
	if len(pd.Sections) != 2 {
		t.Errorf("Sections = %+v, want 2", pd.Sections)
	}
}

func TestParseDocumentFile_Text_NoMetadata(t *testing.T) {
	path := writeTempFile(t, "note.txt", "just some plain notes\n")
	pd, err := ParseDocumentFile(path, "")
	if err != nil {
		t.Fatalf("ParseDocumentFile: %v", err)
	}
	if pd.Title != "" || pd.Author != "" {
		t.Errorf("pd = %+v, want no metadata for plain text", pd)
	}
	if len(pd.Sections) != 1 {
		t.Errorf("Sections = %+v, want 1", pd.Sections)
	}
}

func TestParseDocumentFile_RejectsPDF(t *testing.T) {
	path := writeTempFile(t, "story.pdf", "%PDF-1.4 fake content")
	_, err := ParseDocumentFile(path, "")
	if err == nil {
		t.Fatal("expected an error for a .pdf document, got none")
	}
}

func TestParseDocumentFile_ChecksumReflectsContent(t *testing.T) {
	path := writeTempFile(t, "a.txt", "version one\n")
	pd1, err := ParseDocumentFile(path, "")
	if err != nil {
		t.Fatalf("ParseDocumentFile: %v", err)
	}
	if err := os.WriteFile(path, []byte("version two\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	pd2, err := ParseDocumentFile(path, "")
	if err != nil {
		t.Fatalf("ParseDocumentFile (2nd): %v", err)
	}
	if pd1.Checksum == pd2.Checksum {
		t.Error("expected different checksums for different content")
	}
}

// ─── W3: update helpers for re-ingest reconciliation ────────────────────────

func TestUpdateDocumentMetadata_UpdatesFields(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "Old Title", Format: "text", Path: "a.txt", Checksum: "abc"})
	if err := kb.UpdateDocumentMetadata(docID, "New Title", "New Author", "2026-01-01", "def"); err != nil {
		t.Fatalf("UpdateDocumentMetadata: %v", err)
	}
	d, err := kb.DocumentByPath("a.txt")
	if err != nil {
		t.Fatalf("DocumentByPath: %v", err)
	}
	if d.Title != "New Title" || d.Author != "New Author" || d.PublishedDate != "2026-01-01" || d.Checksum != "def" {
		t.Errorf("d = %+v, want updated fields", d)
	}
}

func TestUpdateDocumentSectionBody_UpdatesBodyAndStale(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "T", Format: "text", Path: "a.txt"})
	secID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section", Body: "old body"})
	if err := kb.UpdateDocumentSectionBody(secID, "new body", 2, 3, true); err != nil {
		t.Fatalf("UpdateDocumentSectionBody: %v", err)
	}
	sections, err := kb.DocumentSections(docID)
	if err != nil {
		t.Fatalf("DocumentSections: %v", err)
	}
	if len(sections) != 1 || sections[0].Body != "new body" || sections[0].SourceSize != 2 || sections[0].TagDensity != 3 || !sections[0].SummaryStale {
		t.Errorf("sections = %+v, want updated body/size/density/stale", sections)
	}
}

func TestUpdateDocumentSectionTagDensity_UpdatesDensityOnly(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "T", Format: "text", Path: "a.txt"})
	secID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "gist", SummaryBody: "a gist", SummaryStatus: "drafted"})
	if err := kb.UpdateDocumentSectionTagDensity(secID, 5); err != nil {
		t.Fatalf("UpdateDocumentSectionTagDensity: %v", err)
	}
	sections, err := kb.DocumentSections(docID)
	if err != nil {
		t.Fatalf("DocumentSections: %v", err)
	}
	if len(sections) != 1 || sections[0].TagDensity != 5 || sections[0].SummaryBody != "a gist" {
		t.Errorf("sections = %+v, want density updated and summary untouched", sections)
	}
}

func TestMarkDocumentSectionStale_SetsStaleWithoutTouchingBody(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "T", Format: "text", Path: "a.txt"})
	secID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "gist", SummaryBody: "a gist", SummaryStatus: "drafted"})
	if err := kb.MarkDocumentSectionStale(secID); err != nil {
		t.Fatalf("MarkDocumentSectionStale: %v", err)
	}
	sections, err := kb.DocumentSections(docID)
	if err != nil {
		t.Fatalf("DocumentSections: %v", err)
	}
	if len(sections) != 1 || !sections[0].SummaryStale || sections[0].SummaryBody != "a gist" {
		t.Errorf("sections = %+v, want stale=true and SummaryBody untouched", sections)
	}
}

// ─── W4: document_section_concepts ──────────────────────────────────────────

func TestLinkDocumentSectionConcept_CreatesLink(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "T", Format: "text", Path: "a.txt"})
	secID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section"})
	conceptID, _ := kb.AddConcept("Foo", "")
	if err := kb.LinkDocumentSectionConcept(secID, conceptID); err != nil {
		t.Fatalf("LinkDocumentSectionConcept: %v", err)
	}
	concepts, err := kb.DocumentSectionConcepts(secID)
	if err != nil {
		t.Fatalf("DocumentSectionConcepts: %v", err)
	}
	if len(concepts) != 1 || concepts[0].Name != "Foo" {
		t.Errorf("concepts = %+v, want one concept named Foo", concepts)
	}
}

func TestLinkDocumentSectionConcept_DuplicateIsNoOp(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "T", Format: "text", Path: "a.txt"})
	secID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section"})
	conceptID, _ := kb.AddConcept("Foo", "")
	if err := kb.LinkDocumentSectionConcept(secID, conceptID); err != nil {
		t.Fatalf("LinkDocumentSectionConcept (1st): %v", err)
	}
	if err := kb.LinkDocumentSectionConcept(secID, conceptID); err != nil {
		t.Fatalf("LinkDocumentSectionConcept (2nd): %v", err)
	}
	concepts, err := kb.DocumentSectionConcepts(secID)
	if err != nil {
		t.Fatalf("DocumentSectionConcepts: %v", err)
	}
	if len(concepts) != 1 {
		t.Errorf("expected exactly 1 concept after duplicate link, got %d", len(concepts))
	}
}

func TestLinkDocumentSectionConcept_CascadesOnSectionDelete(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "T", Format: "text", Path: "a.txt"})
	secID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section"})
	conceptID, _ := kb.AddConcept("Foo", "")
	if err := kb.LinkDocumentSectionConcept(secID, conceptID); err != nil {
		t.Fatalf("LinkDocumentSectionConcept: %v", err)
	}
	if _, err := kb.db.Exec(`DELETE FROM document_sections WHERE id = ?`, secID); err != nil {
		t.Fatalf("delete section: %v", err)
	}
	var count int
	if err := kb.db.QueryRow(`SELECT COUNT(*) FROM document_section_concepts WHERE section_id = ?`, secID).Scan(&count); err != nil {
		t.Fatalf("count document_section_concepts: %v", err)
	}
	if count != 0 {
		t.Errorf("expected document_section_concepts to cascade-delete with its section, got %d rows", count)
	}
}

// ─── W5: review workflow ─────────────────────────────────────────────────────

func TestDraftDocumentSummary_SetsStatusAndClearsStale(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "T", Format: "text", Path: "a.txt"})
	secID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section"})
	if err := kb.MarkDocumentSectionStale(secID); err != nil {
		t.Fatalf("MarkDocumentSectionStale: %v", err)
	}
	confidence := 0.8
	if err := kb.DraftDocumentSummary(secID, "a draft", "human", &confidence); err != nil {
		t.Fatalf("DraftDocumentSummary: %v", err)
	}
	sections, err := kb.DocumentSections(docID)
	if err != nil {
		t.Fatalf("DocumentSections: %v", err)
	}
	s := sections[0]
	if s.SummaryBody != "a draft" || s.SummaryStatus != "drafted" || s.GeneratedBy != "human" {
		t.Errorf("section = %+v, want drafted fields set", s)
	}
	if s.SummaryStale {
		t.Error("expected summary_stale cleared by a fresh draft")
	}
	if s.Confidence == nil || *s.Confidence != 0.8 {
		t.Errorf("Confidence = %v, want 0.8", s.Confidence)
	}
}

func TestPromoteDocumentSummary_RequiresDraftedFirst(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "T", Format: "text", Path: "a.txt"})
	secID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section"})
	if err := kb.PromoteDocumentSummary(secID); err == nil {
		t.Fatal("expected an error promoting an unsummarized section")
	}
}

func TestPromoteDocumentSummary_SetsReviewedAndIndexesFTS(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "A Story", Format: "text", Path: "a.txt"})
	secID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "gist"})
	if err := kb.DraftDocumentSummary(secID, "a distinctive drafted summary", "human", nil); err != nil {
		t.Fatalf("DraftDocumentSummary: %v", err)
	}
	if err := kb.PromoteDocumentSummary(secID); err != nil {
		t.Fatalf("PromoteDocumentSummary: %v", err)
	}
	sections, err := kb.DocumentSections(docID)
	if err != nil {
		t.Fatalf("DocumentSections: %v", err)
	}
	if sections[0].SummaryStatus != "reviewed" {
		t.Errorf("SummaryStatus = %q, want reviewed", sections[0].SummaryStatus)
	}
	results, err := kb.Search("distinctive drafted summary")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	found := false
	for _, r := range results {
		if r.SourceType == "document_summary" {
			found = true
		}
	}
	if !found {
		t.Errorf("Search results = %+v, want the reviewed summary indexed", results)
	}
}

func TestPromoteDocumentSummary_RawSectionBodyNeverIndexed(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "A Story", Format: "text", Path: "a.txt"})
	secID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section", Body: "a very distinctive raw body sentence"})
	if err := kb.DraftDocumentSummary(secID, "an unrelated summary", "human", nil); err != nil {
		t.Fatalf("DraftDocumentSummary: %v", err)
	}
	if err := kb.PromoteDocumentSummary(secID); err != nil {
		t.Fatalf("PromoteDocumentSummary: %v", err)
	}
	results, err := kb.Search("distinctive raw body sentence")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search results = %+v, want the raw body never indexed even after promotion", results)
	}
}

func TestDocumentReviewQueue_DefaultShowsUnsummarizedAndDrafted(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "T", Format: "text", Path: "a.txt"})
	unsummarizedID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section", Heading: "Un"})
	draftedID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section", Heading: "Draft"})
	if err := kb.DraftDocumentSummary(draftedID, "d", "human", nil); err != nil {
		t.Fatalf("DraftDocumentSummary: %v", err)
	}
	reviewedID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section", Heading: "Rev"})
	if err := kb.DraftDocumentSummary(reviewedID, "r", "human", nil); err != nil {
		t.Fatalf("DraftDocumentSummary: %v", err)
	}
	if err := kb.PromoteDocumentSummary(reviewedID); err != nil {
		t.Fatalf("PromoteDocumentSummary: %v", err)
	}

	items, err := kb.DocumentReviewQueue(0, "")
	if err != nil {
		t.Fatalf("DocumentReviewQueue: %v", err)
	}
	ids := map[int64]bool{}
	for _, it := range items {
		ids[it.ID] = true
	}
	if !ids[unsummarizedID] || !ids[draftedID] {
		t.Errorf("items = %+v, want unsummarized and drafted included", items)
	}
	if ids[reviewedID] {
		t.Errorf("items = %+v, want reviewed excluded by default", items)
	}
}

func TestDocumentReviewQueue_StatusFilterOverridesDefault(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "T", Format: "text", Path: "a.txt"})
	reviewedID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section"})
	if err := kb.DraftDocumentSummary(reviewedID, "r", "human", nil); err != nil {
		t.Fatalf("DraftDocumentSummary: %v", err)
	}
	if err := kb.PromoteDocumentSummary(reviewedID); err != nil {
		t.Fatalf("PromoteDocumentSummary: %v", err)
	}

	items, err := kb.DocumentReviewQueue(0, "reviewed")
	if err != nil {
		t.Fatalf("DocumentReviewQueue: %v", err)
	}
	if len(items) != 1 || items[0].ID != reviewedID {
		t.Errorf("items = %+v, want the one reviewed section", items)
	}
}

func TestDocumentReviewQueue_ScopedToProject(t *testing.T) {
	kb := openTestKB(t)
	p1, _ := kb.AddProject("alpha", "")
	p2, _ := kb.AddProject("beta", "")
	doc1, _ := kb.AddDocument(Document{ProjectID: p1, Title: "T1", Format: "text", Path: "a.txt"})
	doc2, _ := kb.AddDocument(Document{ProjectID: p2, Title: "T2", Format: "text", Path: "b.txt"})
	kb.AddDocumentSection(DocumentSection{DocumentID: doc1, Level: "section"})
	kb.AddDocumentSection(DocumentSection{DocumentID: doc2, Level: "section"})

	items, err := kb.DocumentReviewQueue(p1, "")
	if err != nil {
		t.Fatalf("DocumentReviewQueue: %v", err)
	}
	if len(items) != 1 || items[0].DocumentTitle != "T1" {
		t.Errorf("items = %+v, want only project alpha's section", items)
	}
}

// ─── W8: Documents query (list/show) ─────────────────────────────────────────

func TestDocuments_ListsAllWhenProjectIDZero(t *testing.T) {
	kb := openTestKB(t)
	p1, _ := kb.AddProject("alpha", "")
	p2, _ := kb.AddProject("beta", "")
	if _, err := kb.AddDocument(Document{ProjectID: p1, Title: "T1", Format: "text", Path: "a.txt"}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if _, err := kb.AddDocument(Document{ProjectID: p2, Title: "T2", Format: "text", Path: "b.txt"}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	docs, err := kb.Documents(0)
	if err != nil {
		t.Fatalf("Documents: %v", err)
	}
	if len(docs) != 2 {
		t.Errorf("docs = %+v, want 2", docs)
	}
}

func TestDocuments_ScopedToProject(t *testing.T) {
	kb := openTestKB(t)
	p1, _ := kb.AddProject("alpha", "")
	p2, _ := kb.AddProject("beta", "")
	if _, err := kb.AddDocument(Document{ProjectID: p1, Title: "T1", Format: "text", Path: "a.txt"}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if _, err := kb.AddDocument(Document{ProjectID: p2, Title: "T2", Format: "text", Path: "b.txt"}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	docs, err := kb.Documents(p1)
	if err != nil {
		t.Fatalf("Documents: %v", err)
	}
	if len(docs) != 1 || docs[0].Title != "T1" {
		t.Errorf("docs = %+v, want only T1", docs)
	}
}
