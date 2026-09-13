package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
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
