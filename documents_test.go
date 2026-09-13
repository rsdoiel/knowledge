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
	if err := kb.UpdateDocumentSectionBody(secID, "new body", 2, true); err != nil {
		t.Fatalf("UpdateDocumentSectionBody: %v", err)
	}
	sections, err := kb.DocumentSections(docID)
	if err != nil {
		t.Fatalf("DocumentSections: %v", err)
	}
	if len(sections) != 1 || sections[0].Body != "new body" || sections[0].SourceSize != 2 || !sections[0].SummaryStale {
		t.Errorf("sections = %+v, want updated body/size/stale", sections)
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
