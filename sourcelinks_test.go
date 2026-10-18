package knowledge

import (
	"bytes"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

// ─── documents and records cite sources (DR-0081 items 2, 3, 5) ──────────────

type citeFixture struct {
	kb        *KnowledgeBase
	pid, doc  int64
	rec, src  int64
	obs, src2 int64
}

func newCiteFixture(t *testing.T) citeFixture {
	t.Helper()
	kb := openTestKB(t)
	pid, err := kb.AddProject("work", "")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := kb.AddDocument(Document{ProjectID: pid, Title: "A Story", Format: "markdown", Path: "story.md"})
	if err != nil {
		t.Fatal(err)
	}
	rec, err := kb.AddRecord(Record{RecordID: "0001", ProjectID: pid, Path: "decisions/0001-x.md", Title: "X", Date: "2026-10-18"})
	if err != nil {
		t.Fatal(err)
	}
	src, err := kb.AddSource(Source{Title: "SPARQL 1.1", IdentifierType: "doi", IdentifierValue: "10.1234/sparql", PublishedDate: "2013"})
	if err != nil {
		t.Fatal(err)
	}
	src2, err := kb.AddSource(Source{Title: "A page", IdentifierType: "url", IdentifierValue: "https://example.org/a"})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := kb.AddObservation(pid, "note", "drawn from the page")
	if err != nil {
		t.Fatal(err)
	}
	return citeFixture{kb: kb, pid: pid, doc: doc, rec: rec, src: src, obs: obs, src2: src2}
}

func TestOpen_AddsTheCitationTablesAndColumnsToAnExistingDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := DefaultPath(dir)
	kb, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := kb.AddProject("work", "")
	sid, _ := kb.AddSource(Source{Title: "kept", IdentifierType: "doi", IdentifierValue: "10.1/kept"})
	oid, _ := kb.AddObservation(pid, "note", "cites kept")
	if err := kb.LinkObservationSource(oid, sid, "cited"); err != nil {
		t.Fatal(err)
	}
	kb.Close()

	// Put the database back as v0.0.24 left it: no citation tables, none of
	// the two new observation_sources columns.
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`DROP TABLE document_sources`, `DROP TABLE record_sources`,
		`ALTER TABLE observation_sources DROP COLUMN source_version`,
		`ALTER TABLE observation_sources DROP COLUMN source_hash`,
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	raw.Close()

	for i := 0; i < 2; i++ { // twice: the second Open must be a no-op
		kb, err = Open(dbPath)
		if err != nil {
			t.Fatalf("Open #%d: %v", i+1, err)
		}
		links, err := kb.ObservationSourceLinks(oid)
		if err != nil || len(links) != 1 || links[0].Source.ID != sid || links[0].Relationship != "cited" {
			t.Fatalf("Open #%d: the existing link did not survive: %+v, %v", i+1, links, err)
		}
		if err := kb.LinkObservationSourceAt(oid, sid, "cited", "v2", "abc"); err != nil {
			t.Fatalf("Open #%d: new columns missing: %v", i+1, err)
		}
		kb.Close()
	}
}

func TestLinkDocumentSource_LinksOnceAndListsWithTheLinkColumns(t *testing.T) {
	f := newCiteFixture(t)
	if err := f.kb.LinkDocumentSource(f.doc, f.src, "", "2nd ed", "sha256:aa"); err != nil {
		t.Fatalf("LinkDocumentSource: %v", err)
	}
	if err := f.kb.LinkDocumentSource(f.doc, f.src, "cited", "other", "other"); err != nil {
		t.Fatalf("a second link of the same pair must be a no-op, got %v", err)
	}
	if err := f.kb.LinkDocumentSource(f.doc, f.src2, "retrieved", "", ""); err != nil {
		t.Fatal(err)
	}
	links, err := f.kb.DocumentSources(f.doc)
	if err != nil || len(links) != 2 {
		t.Fatalf("DocumentSources = %+v, %v, want 2", links, err)
	}
	a, b := links[0], links[1]
	if a.Source.ID != f.src || a.Source.Title != "SPARQL 1.1" || a.Source.IdentifierValue != "10.1234/sparql" ||
		a.Source.PublishedDate != "2013" || a.Relationship != "cited" || a.SourceVersion != "2nd ed" || a.SourceHash != "sha256:aa" {
		t.Errorf("first link (empty relationship defaults to cited; the first write wins) = %+v", a)
	}
	if b.Source.ID != f.src2 || b.Relationship != "retrieved" {
		t.Errorf("second link = %+v", b)
	}
}

func TestLinkRecordSource_LinksAndLists(t *testing.T) {
	f := newCiteFixture(t)
	if err := f.kb.LinkRecordSource(f.rec, f.src, "cited", "", ""); err != nil {
		t.Fatalf("LinkRecordSource: %v", err)
	}
	links, err := f.kb.RecordSources(f.rec)
	if err != nil || len(links) != 1 || links[0].Source.ID != f.src {
		t.Fatalf("RecordSources = %+v, %v", links, err)
	}
	if links, _ := f.kb.RecordSources(f.rec + 99); len(links) != 0 {
		t.Errorf("an unknown record lists %+v, want none", links)
	}
}

func TestLinkSources_UnknownRowsAreNotFound(t *testing.T) {
	f := newCiteFixture(t)
	for name, err := range map[string]error{
		"document": f.kb.LinkDocumentSource(f.doc+99, f.src, "cited", "", ""),
		"record":   f.kb.LinkRecordSource(f.rec+99, f.src, "cited", "", ""),
		"source":   f.kb.LinkDocumentSource(f.doc, f.src+99, "cited", "", ""),
		"source2":  f.kb.LinkRecordSource(f.rec, f.src+99, "cited", "", ""),
		"obs":      f.kb.LinkObservationSourceAt(f.obs+99, f.src, "cited", "", ""),
	} {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
}

func TestObservationSourceLinks_CarryVersionAndHash(t *testing.T) {
	f := newCiteFixture(t)
	if err := f.kb.LinkObservationSourceAt(f.obs, f.src2, "cited", "2026-10", "sha256:bb"); err != nil {
		t.Fatal(err)
	}
	links, err := f.kb.ObservationSourceLinks(f.obs)
	if err != nil || len(links) != 1 || links[0].SourceVersion != "2026-10" || links[0].SourceHash != "sha256:bb" {
		t.Fatalf("links = %+v, %v", links, err)
	}
	// The older call and the older list are unchanged.
	if err := f.kb.LinkObservationSource(f.obs, f.src, "retrieved"); err != nil {
		t.Fatal(err)
	}
	old, err := f.kb.ObservationSources(f.obs)
	if err != nil || len(old) != 2 {
		t.Errorf("ObservationSources = %+v, %v, want 2", old, err)
	}
}

func TestSourceCitations_CountsEveryKindAndStopsRemoval(t *testing.T) {
	f := newCiteFixture(t)
	f.kb.LinkObservationSource(f.obs, f.src, "cited")
	f.kb.LinkDocumentSource(f.doc, f.src, "cited", "", "")
	f.kb.LinkRecordSource(f.rec, f.src, "cited", "", "")
	c, err := f.kb.SourceCitations(f.src)
	if err != nil || c.Observations != 1 || c.Documents != 1 || c.Records != 1 || c.Total() != 3 {
		t.Fatalf("SourceCitations = %+v, %v", c, err)
	}
	if n, _ := f.kb.SourceObservationCount(f.src); n != 1 {
		t.Errorf("SourceObservationCount = %d, want the older observation-only count 1", n)
	}
	err = f.kb.RemoveSource(f.src)
	if !errors.Is(err, ErrInUse) {
		t.Fatalf("RemoveSource of a cited source = %v, want ErrInUse", err)
	}
	// A source cited only by a document or a record is refused too.
	f.kb.UnlinkObservationSource(f.obs, f.src)
	f.kb.kbExec(t, `DELETE FROM record_sources WHERE source_id = ?`, f.src)
	if err := f.kb.RemoveSource(f.src); !errors.Is(err, ErrInUse) {
		t.Errorf("a document still cites it: RemoveSource = %v, want ErrInUse", err)
	}
	f.kb.kbExec(t, `DELETE FROM document_sources WHERE source_id = ?`, f.src)
	if err := f.kb.RemoveSource(f.src); err != nil {
		t.Errorf("uncited source: RemoveSource = %v", err)
	}
}

func TestDeletingADocumentOrRecordDeletesItsCitations(t *testing.T) {
	f := newCiteFixture(t)
	f.kb.LinkDocumentSource(f.doc, f.src, "cited", "", "")
	f.kb.LinkRecordSource(f.rec, f.src, "cited", "", "")
	if _, err := f.kb.DeleteDocument(f.doc, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.kb.DeleteRecord(f.rec); err != nil {
		t.Fatal(err)
	}
	c, err := f.kb.SourceCitations(f.src)
	if err != nil || c.Total() != 0 {
		t.Errorf("SourceCitations = %+v, %v, want none left", c, err)
	}
	if err := f.kb.RemoveSource(f.src); err != nil {
		t.Errorf("RemoveSource after the citing rows are gone: %v", err)
	}
}

func TestAddObservationWithSource_LinksTheSourceAndLeavesTheColumnEmpty(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	id, err := kb.AddObservationWithSource(pid, "finding", "this paper found X", "10.1234/abcd.5678")
	if err != nil {
		t.Fatal(err)
	}
	links, err := kb.ObservationSourceLinks(id)
	if err != nil || len(links) != 1 || links[0].Source.IdentifierType != "doi" ||
		links[0].Source.IdentifierValue != "10.1234/abcd.5678" || links[0].Relationship != "cited" {
		t.Fatalf("links = %+v, %v", links, err)
	}
	var col string
	if err := kb.db.QueryRow(`SELECT source_doi FROM observations WHERE id = ?`, id).Scan(&col); err != nil || col != "" {
		t.Errorf("source_doi = %q, %v, want it no longer written", col, err)
	}
	// The same DOI again finds the source, it does not make a second.
	id2, _ := kb.AddObservationWithSource(pid, "note", "again", "10.1234/abcd.5678")
	links2, _ := kb.ObservationSourceLinks(id2)
	if len(links2) != 1 || links2[0].Source.ID != links[0].Source.ID {
		t.Errorf("second link = %+v, want the same source", links2)
	}
	// A pasted prefix is normalised, as the old doc comment's example implies.
	id3, err := kb.AddObservationWithSource(pid, "note", "prefixed", "https://doi.org/10.1234/abcd.5678")
	if err != nil {
		t.Fatalf("a doi.org URL: %v", err)
	}
	if l3, _ := kb.ObservationSourceLinks(id3); len(l3) != 1 || l3[0].Source.ID != links[0].Source.ID {
		t.Errorf("prefixed DOI = %+v, want the same source", l3)
	}
}

func TestAddObservationWithSource_NoDOIIsAsBefore(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	id, err := kb.AddObservationWithSource(pid, "note", "no source", "")
	if err != nil {
		t.Fatal(err)
	}
	if links, _ := kb.ObservationSourceLinks(id); len(links) != 0 {
		t.Errorf("links = %+v, want none", links)
	}
}

func TestAddObservationWithSource_ABadDOIWritesNothing(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	_, err := kb.AddObservationWithSource(pid, "note", "bad doi", "not a doi")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	obs, _ := kb.Observations(pid)
	if len(obs) != 0 {
		t.Errorf("observations = %+v, want none written", obs)
	}
}

// An older database whose observations still hold a source_doi keeps reading it.
func TestObservationsWithAnOldSourceDOIStillExport(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	oid, _ := kb.AddObservation(pid, "note", "old row")
	if _, err := kb.db.Exec(`UPDATE observations SET source_doi = '10.9/old' WHERE id = ?`, oid); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := ExportJSONL(kb, &buf, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"source_doi":"10.9/old"`) {
		t.Errorf("export dropped source_doi: %s", buf.String())
	}
}

func (kb *KnowledgeBase) kbExec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := kb.db.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

func TestUnlinkDocumentAndRecordSource(t *testing.T) {
	f := newCiteFixture(t)
	f.kb.LinkDocumentSource(f.doc, f.src, "cited", "", "")
	f.kb.LinkRecordSource(f.rec, f.src, "cited", "", "")
	if err := f.kb.UnlinkDocumentSource(f.doc, f.src); err != nil {
		t.Fatal(err)
	}
	if err := f.kb.UnlinkRecordSource(f.rec, f.src); err != nil {
		t.Fatal(err)
	}
	if c, _ := f.kb.SourceCitations(f.src); c.Total() != 0 {
		t.Errorf("citations = %+v, want none", c)
	}
	for name, err := range map[string]error{
		"no link":     f.kb.UnlinkDocumentSource(f.doc, f.src),
		"no link rec": f.kb.UnlinkRecordSource(f.rec, f.src),
		"no document": f.kb.UnlinkDocumentSource(f.doc+99, f.src),
		"no record":   f.kb.UnlinkRecordSource(f.rec+99, f.src),
		"no source":   f.kb.UnlinkDocumentSource(f.doc, f.src+99),
	} {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
}
