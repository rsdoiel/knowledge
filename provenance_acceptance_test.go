package knowledge

import (
	"bytes"
	"strings"
	"testing"
)

// The acceptance test DR-0080 asks for, in the shape DR-0013 gave it: two
// databases, each holding sources and citations the other lacks, merged,
// exported and imported; then every source is found by search and every hit
// names its sources and says whether any is retracted.

type accWorld struct{ srcShared, srcA, srcB int64 }

func buildAcceptanceSide(t *testing.T, kb *KnowledgeBase, side string) {
	t.Helper()
	pid, _ := kb.AddProject("work", "")
	shared, _ := kb.AddSource(Source{Title: "Shared SPARQL paper", IdentifierType: "doi", IdentifierValue: "10.1/shared", Authors: "Ada"})
	switch side {
	case "a":
		srcA, _ := kb.AddSource(Source{Title: "Alpha parsing page", IdentifierType: "url", IdentifierValue: "https://example.org/alpha"})
		doc, _ := kb.AddDocument(Document{ProjectID: pid, Title: "Alpha story", Format: "markdown", Path: "alpha.md", Checksum: "ak"})
		sec, _ := kb.AddDocumentSection(DocumentSection{DocumentID: doc, Level: "gist"})
		kb.DraftDocumentSummary(sec, "a summary about hedgehogs and parsing", "human", nil)
		kb.PromoteDocumentSummary(sec)
		obs, _ := kb.AddObservation(pid, "finding", "parsing result from the alpha machine")
		for _, err := range []error{
			kb.LinkDocumentSource(doc, shared, "cited", "2nd ed", "sha256:aa"),
			kb.LinkDocumentSource(doc, srcA, "cited", "", ""),
			kb.LinkObservationSourceAt(obs, srcA, "retrieved", "v1", "sha256:a1"),
		} {
			if err != nil {
				t.Fatal(err)
			}
		}
	case "b":
		srcB, _ := kb.AddSource(Source{Title: "Beta tokenizer study", IdentifierType: "doi", IdentifierValue: "10.2/beta"})
		rec := addTestRecord(t, kb, Record{RecordID: "0001", ProjectID: pid, Title: "Tokenizer policy", Path: "decisions/0001-t.md",
			Body: "\n**Decision.** Use the tokenizer from the beta study.\n", Checksum: "bk"})
		obs, _ := kb.AddObservation(pid, "finding", "tokenizer result from the beta machine")
		for _, err := range []error{
			kb.LinkRecordSource(rec, shared, "cited", "", ""),
			kb.LinkRecordSource(rec, srcB, "cited", "v3", "sha256:b3"),
			kb.LinkObservationSource(obs, srcB, "cited"),
			kb.RetractSource(srcB, "withdrawn by the publisher"),
		} {
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func checkAcceptance(t *testing.T, name string, kb *KnowledgeBase, docSummaries bool) {
	t.Helper()
	// Every source is found by its title or author.
	for term, title := range map[string]string{
		"sparql": "Shared SPARQL paper", "alpha": "Alpha parsing page", "tokenizer": "Beta tokenizer study",
	} {
		rs, err := kb.Search(term)
		if err != nil {
			t.Fatalf("%s: Search(%q): %v", name, term, err)
		}
		found := false
		for _, h := range sourceHits(rs) {
			found = found || h.Label == title
		}
		if !found {
			t.Errorf("%s: Search(%q) does not find the source %q: %+v", name, term, title, rs)
		}
	}
	// Each citing hit names its sources, and Retracted is right.
	rs, _ := kb.Search("tokenizer")
	obs := findHit(t, rs, "observation", func(r KBSearchResult) bool { return strings.Contains(r.Snippet, "tokenizer result") })
	if len(obs.Sources) != 1 || obs.Sources[0].Title != "Beta tokenizer study" || !obs.Retracted ||
		obs.Sources[0].RetractionNote != "withdrawn by the publisher" {
		t.Errorf("%s: observation hit = %+v, want the retracted beta source", name, obs.HitProvenance)
	}
	rec := findHit(t, rs, "record", func(KBSearchResult) bool { return true })
	if len(rec.Sources) != 2 || !rec.Retracted || rec.Material == nil || rec.Material.Path != "decisions/0001-t.md" ||
		rec.Sources[1].SourceVersion != "v3" || rec.Sources[1].SourceHash != "sha256:b3" {
		t.Errorf("%s: record hit = %+v", name, rec.HitProvenance)
	}
	rs, _ = kb.Search("alpha")
	ao := findHit(t, rs, "observation", func(r KBSearchResult) bool { return strings.Contains(r.Snippet, "alpha machine") })
	if len(ao.Sources) != 1 || ao.Retracted || ao.Sources[0].Relationship != "retrieved" || ao.Sources[0].SourceVersion != "v1" {
		t.Errorf("%s: alpha observation hit = %+v", name, ao.HitProvenance)
	}
	if docSummaries {
		rs, _ = kb.Search("hedgehogs")
		d := findHit(t, rs, "document_summary", func(KBSearchResult) bool { return true })
		if len(d.Sources) != 2 || d.Retracted || d.Material == nil || d.Material.Path != "alpha.md" ||
			d.Sources[0].SourceVersion != "2nd ed" || d.Sources[0].SourceHash != "sha256:aa" {
			t.Errorf("%s: document summary hit = %+v", name, d.HitProvenance)
		}
	}
	// The shared source is one row, cited by a document and a record.
	var n int
	kb.db.QueryRow(`SELECT COUNT(*) FROM sources WHERE identifier_value = '10.1/shared'`).Scan(&n)
	if n != 1 {
		t.Errorf("%s: the shared source is %d rows, want 1", name, n)
	}
	var sid int64
	kb.db.QueryRow(`SELECT id FROM sources WHERE identifier_value = '10.1/shared'`).Scan(&sid)
	if c, _ := kb.SourceCitations(sid); c.Documents != 1 || c.Records != 1 {
		t.Errorf("%s: the shared source is cited by %+v, want one document and one record", name, c)
	}
	// Scoped: the sources of the project, with the project they span.
	sr, err := kb.SearchScoped("tokenizer", SearchScope{Project: "work", Kinds: []string{"sources"}})
	if err != nil || len(sr.Results) != 1 || strings.Join(sr.SourceSpans[sr.Results[0].SourceID], ",") != "work" {
		t.Errorf("%s: scoped source search = %+v, %v", name, sr, err)
	}
}

func TestProvenance_TwoDatabasesMergedExportedImportedThenSearched(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	buildAcceptanceSide(t, a, "a")
	buildAcceptanceSide(t, b, "b")
	// The same machines' project and shared source, already reconciled.
	var apid, bpid, asrc, bsrc int64
	a.db.QueryRow(`SELECT id FROM projects WHERE name = 'work'`).Scan(&apid)
	b.db.QueryRow(`SELECT id FROM projects WHERE name = 'work'`).Scan(&bpid)
	a.db.QueryRow(`SELECT id FROM sources WHERE identifier_value = '10.1/shared'`).Scan(&asrc)
	b.db.QueryRow(`SELECT id FROM sources WHERE identifier_value = '10.1/shared'`).Scan(&bsrc)
	shareProjectUUID(t, a, apid, b, bpid)
	shareSourceUUID(t, a, asrc, b, bsrc)

	merged := openMergedTestKB(t, a, b)
	checkAcceptance(t, "merged", merged, true)

	var buf bytes.Buffer
	if err := ExportJSONL(merged, &buf, ""); err != nil {
		t.Fatal(err)
	}
	imported := openTestKB(t)
	if _, err := ImportJSONL(imported, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	checkAcceptance(t, "imported", imported, true)

	// The merged database and its import agree table by table.
	diffs, err := DiffDatabases(merged.Path(), imported.Path())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diffs {
		if (d.Table == "document_sources" || d.Table == "record_sources" || d.Table == "observation_sources" || d.Table == "sources") &&
			len(d.OnlyA)+len(d.OnlyB)+len(d.Different) != 0 {
			t.Errorf("merged vs imported differ in %s: %+v", d.Table, d)
		}
	}
}
