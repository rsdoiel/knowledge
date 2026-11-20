package knowledge

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// DR-0091 item 4 as corrected by DR-0092: ingesting a document writes a
// `document` entry in kb_fts holding the title, the front-matter description
// and the section headings, so a document is findable with nothing reviewed.
// A raw section body is never indexed (narrative-documents-design decision 8).

const docFTSFixture = `---
title: Zebrafish Notebook
description: Fieldwork on quokka habitats
---

## Marsupial Census

The wombat counts were recorded on Tuesday.

## Tide Tables

Barnacle growth followed the tides.
`

func documentHits(t *testing.T, kb *KnowledgeBase, term string) []KBSearchResult {
	t.Helper()
	res, err := kb.Search(term)
	if err != nil {
		t.Fatalf("Search(%q): %v", term, err)
	}
	var out []KBSearchResult
	for _, r := range res {
		if r.SourceType == "document" {
			out = append(out, r)
		}
	}
	return out
}

func countDocumentRows(t *testing.T, kb *KnowledgeBase) int {
	t.Helper()
	var n int
	if err := kb.db.QueryRow(`SELECT COUNT(*) FROM kb_fts WHERE source_type = 'document'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestIngestDocument_NothingReviewedIsFoundByTitleDescriptionAndHeading(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "z.md", docFTSFixture)
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	for _, term := range []string{"Zebrafish", "quokka", "Marsupial"} {
		hits := documentHits(t, kb, term)
		if len(hits) != 1 {
			t.Errorf("Search(%q): %d document hits, want 1", term, len(hits))
			continue
		}
		h := hits[0]
		if h.Label != "Zebrafish Notebook" {
			t.Errorf("Search(%q): label %q, want the document title", term, h.Label)
		}
		if h.Material == nil || h.Material.Path != path || h.Material.Checksum == "" {
			t.Errorf("Search(%q): Material = %+v, want path %q and a checksum so a consumer can cite it", term, h.Material, path)
		}
	}
	// A section body is never indexed.
	if hits := documentHits(t, kb, "wombat"); len(hits) != 0 {
		t.Errorf("a word only in a section body found %d document hits, want 0", len(hits))
	}
}

func TestSearchScoped_DocumentsKindIncludesTheDocumentEntry(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "z.md", docFTSFixture)
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatal(err)
	}
	r, err := kb.SearchScoped("quokka", SearchScope{Kinds: []string{"documents"}})
	if err != nil || len(r.Results) != 1 {
		t.Fatalf("SearchScoped documents: %+v, %v; want one result", r, err)
	}
	r, err = kb.SearchScoped("quokka", SearchScope{Project: "alpha"})
	if err != nil || len(r.Results) != 1 {
		t.Errorf("project scope: %+v, %v; want the project's document", r, err)
	}
}

func TestIngestDocument_ChangedTitleOrHeadingReplacesTheEntry(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "z.md", docFTSFixture)
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatal(err)
	}
	changed := strings.NewReplacer("Zebrafish Notebook", "Axolotl Notebook", "Tide Tables", "Moon Phases", "quokka", "numbat").Replace(docFTSFixture)
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatal(err)
	}
	if n := countDocumentRows(t, kb); n != 1 {
		t.Errorf("%d document rows after re-ingest, want 1 (no duplicate)", n)
	}
	for term, want := range map[string]int{"Axolotl": 1, "Moon": 1, "numbat": 1, "Zebrafish": 0, "quokka": 0} {
		if got := len(documentHits(t, kb, term)); got != want {
			t.Errorf("Search(%q): %d hits, want %d", term, got, want)
		}
	}
}

func TestDeleteDocument_RemovesTheEntry(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "z.md", docFTSFixture)
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatal(err)
	}
	d, _ := kb.DocumentByPath(path)
	if _, err := kb.DeleteDocument(d.ID, true); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	if n := countDocumentRows(t, kb); n != 0 {
		t.Errorf("%d document rows after delete, want 0", n)
	}
}

// A database made by merge or import has documents and no entry for them; the
// next open builds it, and a second open adds nothing.
func TestOpen_BackfillsTheDocumentEntryIdempotently(t *testing.T) {
	dir := t.TempDir()
	kb, err := Open(DefaultPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "z.md", docFTSFixture)
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := kb.db.Exec(`DELETE FROM kb_fts WHERE source_type = 'document'`); err != nil {
		t.Fatal(err)
	}
	kb.Close()
	for i := 1; i <= 2; i++ {
		kb, err = Open(DefaultPath(dir))
		if err != nil {
			t.Fatal(err)
		}
		if n := countDocumentRows(t, kb); n != 1 {
			t.Errorf("open %d: %d document rows, want 1", i, n)
		}
		if len(documentHits(t, kb, "quokka")) != 1 {
			t.Errorf("open %d: description not searchable after backfill", i)
		}
		kb.Close()
	}
}

// Documents travel (export, import, merge) with their description, and the
// index, which is derived, is rebuilt for them on the other side.
func TestDocumentEntry_SurvivesImportAndMerge(t *testing.T) {
	a := openTestKB(t)
	pid, _ := a.AddProject("alpha", "")
	path := writeTempFile(t, "z.md", docFTSFixture)
	if _, err := a.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := ExportJSONL(a, &buf, ""); err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}
	b := openTestKB(t)
	if _, err := ImportJSONL(b, &buf); err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}
	d, _ := b.DocumentByPath(path)
	if d == nil || d.Description != "Fieldwork on quokka habitats" {
		t.Fatalf("imported document = %+v, want its description", d)
	}
	for _, term := range []string{"Zebrafish", "quokka", "Marsupial"} {
		if len(documentHits(t, b, term)) != 1 {
			t.Errorf("after import, Search(%q) has no document hit", term)
		}
	}

	merged := openMergedTestKB(t, a, openTestKB(t))
	if len(documentHits(t, merged, "quokka")) != 1 {
		t.Error("after merge, the description is not searchable")
	}
	if n := countDocumentRows(t, merged); n != 1 {
		t.Errorf("after merge, %d document rows, want 1", n)
	}
}

// A document ingested before the description column existed has none stored;
// its next ingest, though its file has not changed, stores it and indexes it.
func TestIngestDocument_SkippedIngestFillsTheDescription(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "z.md", docFTSFixture)
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := kb.db.Exec(`UPDATE documents SET description = ''`); err != nil {
		t.Fatal(err)
	}
	kb.rebuildDocumentFTS()
	if len(documentHits(t, kb, "quokka")) != 0 {
		t.Fatal("setup: the description should not be searchable yet")
	}
	res, err := kb.IngestDocument(pid, path, DocumentIngestOptions{})
	if err != nil || res.Action != "skipped" {
		t.Fatalf("second ingest = %+v, %v; want skipped", res, err)
	}
	if len(documentHits(t, kb, "quokka")) != 1 {
		t.Error("a skipped ingest did not fill and index the description")
	}
}
