package knowledge

import (
	"bytes"
	"strings"
	"testing"
)

// ─── the citation tables travel on every path (DR-0081 item 6, DR-0013) ──────

// citingKB builds a database in which a document, a record and an observation
// of project "work" all cite one source, and returns the pieces.
func citingKB(t *testing.T) (kb *KnowledgeBase, srcID int64) {
	t.Helper()
	kb = openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	srcID, _ = kb.AddSource(Source{Title: "SPARQL 1.1", IdentifierType: "doi", IdentifierValue: "10.1234/sparql"})
	doc, _ := kb.AddDocument(Document{ProjectID: pid, Title: "A Story", Format: "markdown", Path: "story.md", Checksum: "dk"})
	rec := addTestRecord(t, kb, Record{RecordID: "0001", ProjectID: pid, Title: "x", Body: "b", Checksum: "ck"})
	obs, _ := kb.AddObservation(pid, "note", "drawn from the paper")
	for _, err := range []error{
		kb.LinkDocumentSource(doc, srcID, "cited", "2nd ed", "sha256:aa"),
		kb.LinkRecordSource(rec, srcID, "retrieved", "v1", "sha256:bb"),
		kb.LinkObservationSourceAt(obs, srcID, "cited", "v3", "sha256:cc"),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	return kb, srcID
}

func linksOfWork(t *testing.T, kb *KnowledgeBase) (doc, rec, obs []SourceLink) {
	t.Helper()
	p, err := kb.ProjectByName("work")
	if err != nil || p == nil {
		t.Fatalf("project work: %v", err)
	}
	docs, _ := kb.Documents(p.ID)
	recs, _ := kb.RecordsByProject(p.ID)
	observations, _ := kb.Observations(p.ID)
	if len(docs) != 1 || len(recs) != 1 || len(observations) != 1 {
		t.Fatalf("rows: %d documents, %d records, %d observations", len(docs), len(recs), len(observations))
	}
	doc, _ = kb.DocumentSources(docs[0].ID)
	rec, _ = kb.RecordSources(recs[0].ID)
	obs, _ = kb.ObservationSourceLinks(observations[0].ID)
	return doc, rec, obs
}

func TestJSONL_CitationsRoundTrip(t *testing.T) {
	src, _ := citingKB(t)
	var buf bytes.Buffer
	if err := ExportJSONL(src, &buf, ""); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"type":"document_source"`, `"type":"record_source"`, `"source_version":"2nd ed"`, `"source_hash":"sha256:cc"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("export lacks %s:\n%s", want, buf.String())
		}
	}
	dst := openTestKB(t)
	if _, err := ImportJSONL(dst, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	doc, rec, obs := linksOfWork(t, dst)
	if len(doc) != 1 || doc[0].Relationship != "cited" || doc[0].SourceVersion != "2nd ed" || doc[0].SourceHash != "sha256:aa" ||
		doc[0].Source.IdentifierValue != "10.1234/sparql" {
		t.Errorf("document link = %+v", doc)
	}
	if len(rec) != 1 || rec[0].Relationship != "retrieved" || rec[0].SourceVersion != "v1" {
		t.Errorf("record link = %+v", rec)
	}
	if len(obs) != 1 || obs[0].SourceVersion != "v3" || obs[0].SourceHash != "sha256:cc" {
		t.Errorf("observation link = %+v", obs)
	}
	// Importing the same file again changes nothing.
	again, err := ImportJSONL(dst, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range again {
		if (s.Table == "document_source" || s.Table == "record_source") && (s.Imported != 0 || s.Skipped != 1) {
			t.Errorf("%s on a second import = %+v, want 1 skipped", s.Table, s)
		}
	}
}

func TestJSONL_ScopedExportCarriesOnlyThatProjectsCitations(t *testing.T) {
	kb, _ := citingKB(t)
	other, _ := kb.AddProject("other", "")
	s2, _ := kb.AddSource(Source{Title: "Elsewhere", IdentifierType: "doi", IdentifierValue: "10.9/else"})
	d2, _ := kb.AddDocument(Document{ProjectID: other, Title: "Other", Format: "text", Path: "o.txt", Checksum: "ok"})
	if err := kb.LinkDocumentSource(d2, s2, "cited", "", ""); err != nil {
		t.Fatal(err)
	}
	var work, oth bytes.Buffer
	if err := ExportJSONL(kb, &work, "work"); err != nil {
		t.Fatal(err)
	}
	if err := ExportJSONL(kb, &oth, "other"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(work.String(), "10.1234/sparql") || strings.Contains(work.String(), "10.9/else") ||
		!strings.Contains(work.String(), `"type":"record_source"`) {
		t.Errorf("scoped export of work:\n%s", work.String())
	}
	// A source cited only by a document or a record of the project is exported
	// with it, or the link would point at nothing.
	if !strings.Contains(oth.String(), "10.9/else") || !strings.Contains(oth.String(), `"type":"document_source"`) ||
		strings.Contains(oth.String(), "10.1234/sparql") {
		t.Errorf("scoped export of other:\n%s", oth.String())
	}
}

func TestJSONL_AnOlderExportWithoutCitationRowsImportsWithEmptyColumns(t *testing.T) {
	old := `{"type":"project","uuid":"p1","origin_host":"h","name":"work","description":"","status":"active"}` + "\n" +
		`{"type":"source","uuid":"s1","origin_host":"h","title":"T","identifier_type":"doi","identifier_value":"10.1/x"}` + "\n" +
		`{"type":"observation","uuid":"o1","origin_host":"h","project_uuid":"p1","kind":"note","body":"b","source_doi":"","created_at":"2026-01-01 00:00:00"}` + "\n" +
		`{"type":"observation_source","observation_uuid":"o1","source_uuid":"s1","relationship":"cited"}` + "\n"
	kb := openTestKB(t)
	if _, err := ImportJSONL(kb, strings.NewReader(old)); err != nil {
		t.Fatal(err)
	}
	p, _ := kb.ProjectByName("work")
	obs, _ := kb.Observations(p.ID)
	links, err := kb.ObservationSourceLinks(obs[0].ID)
	if err != nil || len(links) != 1 || links[0].SourceVersion != "" || links[0].SourceHash != "" || links[0].Relationship != "cited" {
		t.Errorf("links = %+v, %v, want the link with empty version and hash", links, err)
	}
}

func TestJSONL_ACitationWhoseOwnerOrSourceIsMissingIsSkippedNotAnError(t *testing.T) {
	in := `{"type":"document_source","document_uuid":"nope","source_uuid":"nope","relationship":"cited"}` + "\n" +
		`{"type":"record_source","record_uuid":"nope","source_uuid":"nope","relationship":"cited"}` + "\n"
	kb := openTestKB(t)
	sums, err := ImportJSONL(kb, strings.NewReader(in))
	if err != nil {
		t.Fatalf("import = %v, want it to skip with exit 0", err)
	}
	n := 0
	for _, s := range sums {
		if s.Table == "document_source" || s.Table == "record_source" {
			n++
			if s.Imported != 0 || s.Skipped != 1 {
				t.Errorf("%s = %+v, want 1 skipped", s.Table, s)
			}
		}
	}
	if n != 2 {
		t.Errorf("summaries for %d citation tables, want 2", n)
	}
}

// shareSourceUUID makes b's source carry a's uuid, as a pair of machines that
// have already reconciled the source would.
func shareSourceUUID(t *testing.T, a *KnowledgeBase, aid int64, b *KnowledgeBase, bid int64) {
	t.Helper()
	var u string
	if err := a.db.QueryRow(`SELECT uuid FROM sources WHERE id = ?`, aid).Scan(&u); err != nil {
		t.Fatal(err)
	}
	if _, err := b.db.Exec(`UPDATE sources SET uuid = ? WHERE id = ?`, u, bid); err != nil {
		t.Fatal(err)
	}
}

func TestMergeKnowledgeBases_CitationsUnion(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	apid, _ := a.AddProject("work", "")
	bpid, _ := b.AddProject("work", "")
	shareProjectUUID(t, a, apid, b, bpid)
	as, _ := a.AddSource(Source{Title: "Paper", IdentifierType: "doi", IdentifierValue: "10.1/p"})
	bs, _ := b.AddSource(Source{Title: "Paper", IdentifierType: "doi", IdentifierValue: "10.1/p"})
	shareSourceUUID(t, a, as, b, bs)
	ad, _ := a.AddDocument(Document{ProjectID: apid, Title: "on a", Format: "text", Path: "a.txt", Checksum: "1"})
	bd, _ := b.AddDocument(Document{ProjectID: bpid, Title: "on b", Format: "text", Path: "b.txt", Checksum: "2"})
	ar := addTestRecord(t, a, Record{RecordID: "0001", ProjectID: apid, Title: "on-a"})
	br := addTestRecord(t, b, Record{RecordID: "0002", ProjectID: bpid, Title: "on-b"})
	ao, _ := a.AddObservation(apid, "note", "on a")
	for _, err := range []error{
		a.LinkDocumentSource(ad, as, "cited", "v1", "h1"), b.LinkDocumentSource(bd, bs, "cited", "", ""),
		a.LinkRecordSource(ar, as, "cited", "", ""), b.LinkRecordSource(br, bs, "retrieved", "", ""),
		a.LinkObservationSourceAt(ao, as, "cited", "v9", "h9"),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	merged := openMergedTestKB(t, a, b)
	for table, want := range map[string]int{"document_sources": 2, "record_sources": 2, "observation_sources": 1} {
		var n int
		merged.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n)
		if n != want {
			t.Errorf("merged %s = %d, want %d", table, n, want)
		}
	}
	p, _ := merged.ProjectByName("work")
	obs, _ := merged.Observations(p.ID)
	if l, _ := merged.ObservationSourceLinks(obs[0].ID); len(l) != 1 || l[0].SourceVersion != "v9" || l[0].SourceHash != "h9" {
		t.Errorf("the merged observation link lost its version and hash: %+v", l)
	}
	docs, _ := merged.Documents(p.ID)
	for _, d := range docs {
		if d.Title == "on a" {
			if l, _ := merged.DocumentSources(d.ID); len(l) != 1 || l[0].SourceVersion != "v1" {
				t.Errorf("the merged document link lost its version: %+v", l)
			}
		}
	}
}

func TestMergeKnowledgeBases_SummaryCoversTheCitationTables(t *testing.T) {
	a, _ := citingKB(t)
	b := openTestKB(t)
	summary, err := MergeKnowledgeBases(a.Path(), b.Path(), t.TempDir()+"/merged.db")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]MergeTableSummary{}
	for _, s := range summary {
		seen[s.Table] = s
	}
	for _, table := range []string{"document_sources", "record_sources"} {
		if s, ok := seen[table]; !ok || s.FromA != 1 || s.Merged != 1 {
			t.Errorf("summary[%s] = %+v, %v, want 1 from a and 1 merged", table, s, ok)
		}
	}
}

func TestDiffDatabases_ACitationAddedOnOneSideIsReported(t *testing.T) {
	a := diffFixture(t, "a")
	b := twin(t, a)
	mutate(t, b, func(kb *KnowledgeBase) {
		docs, _ := kb.Documents(1)
		sid, _ := kb.AddSource(Source{Title: "A paper", IdentifierType: "doi", IdentifierValue: "10.1/x"})
		kb.LinkDocumentSource(docs[0].ID, sid, "cited", "", "")
		recs, _ := kb.RecordsByProject(1)
		kb.LinkRecordSource(recs[0].ID, sid, "cited", "", "")
	})
	by := diffByTable(t, a, b)
	if d := by["document_sources"]; len(d.OnlyB) != 1 || len(d.OnlyA) != 0 {
		t.Errorf("document_sources = %+v, want one row only in b", d)
	}
	if d := by["record_sources"]; len(d.OnlyB) != 1 {
		t.Errorf("record_sources = %+v, want one row only in b", d)
	}
}

func TestStats_CountsCitationsAndASourceCitedByADocumentIsLinked(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	sid, _ := kb.AddSource(Source{Title: "only a document cites me"})
	doc, _ := kb.AddDocument(Document{ProjectID: pid, Title: "d", Format: "text", Path: "d.txt", Checksum: "1"})
	kb.LinkDocumentSource(doc, sid, "cited", "", "")
	st, err := kb.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if got := statCount(t, st.Links, "document_sources"); got != 1 {
		t.Errorf("links document_sources = %d, want 1", got)
	}
	if got := statCount(t, st.Links, "record_sources"); got != 0 {
		t.Errorf("links record_sources = %d, want 0", got)
	}
	if got := statCount(t, st.Checks, "sources_without_links"); got != 0 {
		t.Errorf("sources_without_links = %d, want 0: a document cites it", got)
	}
}
