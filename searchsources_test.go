package knowledge

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// ─── sources are searchable (DR-0080 items 2 and 3, amending DR-0013) ────────

func sourceHits(rs []KBSearchResult) []KBSearchResult {
	var out []KBSearchResult
	for _, r := range rs {
		if r.SourceType == "source" {
			out = append(out, r)
		}
	}
	return out
}

func TestSearch_FindsASourceByTitleAuthorPublisherAndIdentifier(t *testing.T) {
	kb := openTestKB(t)
	id, err := kb.AddSource(Source{Title: "Streaming Parsers Considered", IdentifierType: "doi", IdentifierValue: "10.5555/stream.7",
		Authors: "Ada Lovelace; Grace Hopper", Publisher: "Cambridge University Press", PublishedDate: "2024"})
	if err != nil {
		t.Fatal(err)
	}
	for _, term := range []string{"parsers", "lovelace", "cambridge", "stream.7"} {
		rs, err := kb.Search(term)
		if err != nil {
			t.Fatalf("Search(%q): %v", term, err)
		}
		hits := sourceHits(rs)
		if len(hits) != 1 || hits[0].SourceID != id || hits[0].Label != "Streaming Parsers Considered" {
			t.Errorf("Search(%q) source hits = %+v, want the one source", term, hits)
		}
		if len(hits) == 1 && (hits[0].Sources == nil || len(hits[0].Sources) != 0 || hits[0].Material != nil) {
			t.Errorf("a source hit lists no sources of its own: %+v", hits[0])
		}
	}
}

func TestSourcesIndex_StaysInStepWithTheTable(t *testing.T) {
	kb := openTestKB(t)
	id, _ := kb.AddSource(Source{Title: "A paper about marmalade"})
	if rs, _ := kb.Search("marmalade"); len(sourceHits(rs)) != 1 {
		t.Fatalf("a new source is not indexed: %+v", rs)
	}
	// Adding the same source again is a no-op and must not index it twice.
	if rs, _ := kb.Search("marmalade"); len(sourceHits(rs)) != 1 {
		t.Errorf("source indexed twice: %+v", rs)
	}
	if err := kb.RemoveSource(id); err != nil {
		t.Fatal(err)
	}
	if rs, _ := kb.Search("marmalade"); len(sourceHits(rs)) != 0 {
		t.Errorf("a removed source is still found: %+v", rs)
	}
}

func TestSourcesIndex_ImportIndexesAndOpenBackfillsAnOlderDatabase(t *testing.T) {
	src := openTestKB(t)
	src.AddSource(Source{Title: "Imported marmalade study", IdentifierType: "doi", IdentifierValue: "10.7/imp"})
	var buf bytes.Buffer
	if err := ExportJSONL(src, &buf, ""); err != nil {
		t.Fatal(err)
	}
	dst := openTestKB(t)
	if _, err := ImportJSONL(dst, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	if rs, _ := dst.Search("marmalade"); len(sourceHits(rs)) != 1 {
		t.Errorf("an imported source is not searchable: %+v", rs)
	}

	// A database made by v0.0.24 has sources and no source rows in the index.
	path := dst.Path()
	if _, err := dst.db.Exec(`DELETE FROM kb_fts WHERE source_type = 'source'`); err != nil {
		t.Fatal(err)
	}
	dst.Close()
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if rs, _ := again.Search("marmalade"); len(sourceHits(rs)) != 1 {
		t.Errorf("Open did not index the sources of an older database: %+v", rs)
	}
	// And a second Open adds nothing.
	again.Close()
	third, _ := Open(path)
	defer third.Close()
	if rs, _ := third.Search("marmalade"); len(sourceHits(rs)) != 1 {
		t.Errorf("sources indexed twice after a second Open: %+v", rs)
	}
}

func TestSourcesIndex_ADatabaseHoldingOnlySourcesIsRebuilt(t *testing.T) {
	kb := openTestKB(t)
	kb.AddSource(Source{Title: "Lonely marmalade source"})
	path := kb.Path()
	kb.db.Exec(`DELETE FROM kb_fts`)
	kb.Close()
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if rs, _ := again.Search("lonely"); len(sourceHits(rs)) != 1 {
		t.Errorf("a database holding only sources is not rebuilt: %+v", rs)
	}
}

func sourceScopeFixture(t *testing.T) (kb *KnowledgeBase, ids map[string]int64) {
	t.Helper()
	kb = openTestKB(t)
	ids = map[string]int64{}
	a, _ := kb.AddProject("alpha", "")
	b, _ := kb.AddProject("beta", "")
	ids["alphaSrc"], _ = kb.AddSource(Source{Title: "Cited by alpha observation about parsing"})
	ids["betaSrc"], _ = kb.AddSource(Source{Title: "Cited by a beta document about parsing"})
	ids["wsSrc"], _ = kb.AddSource(Source{Title: "Cited by a workspace record about parsing"})
	ids["bothSrc"], _ = kb.AddSource(Source{Title: "Cited by both projects about parsing"})
	ids["loneSrc"], _ = kb.AddSource(Source{Title: "Cited by nothing about parsing"})
	o, _ := kb.AddObservation(a, "note", "an observation")
	d, _ := kb.AddDocument(Document{ProjectID: b, Title: "D", Format: "text", Path: "d.txt", Checksum: "1"})
	wr := addTestRecord(t, kb, Record{RecordID: "0009", Scope: "workspace", Title: "ws", Body: "b", Path: "decisions/0009-ws.md"})
	ar := addTestRecord(t, kb, Record{RecordID: "0001", ProjectID: a, Title: "alpha rec", Body: "b", Path: "decisions/0001-a.md"})
	for _, err := range []error{
		kb.LinkObservationSource(o, ids["alphaSrc"], "cited"),
		kb.LinkDocumentSource(d, ids["betaSrc"], "cited", "", ""),
		kb.LinkRecordSource(wr, ids["wsSrc"], "cited", "", ""),
		kb.LinkRecordSource(ar, ids["bothSrc"], "cited", "", ""),
		kb.LinkDocumentSource(d, ids["bothSrc"], "cited", "", ""),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	return kb, ids
}

func scopedSourceIDs(r *ScopedSearchResult) map[int64]bool {
	out := map[int64]bool{}
	for _, h := range r.Results {
		if h.SourceType == "source" {
			out[h.SourceID] = true
		}
	}
	return out
}

func TestSearchScoped_SourcesKindAtEveryScope(t *testing.T) {
	kb, ids := sourceScopeFixture(t)
	if !containsString(SearchKinds, "sources") {
		t.Fatalf("SearchKinds = %v, want sources in it", SearchKinds)
	}

	global, err := kb.SearchScoped("parsing", SearchScope{Kinds: []string{"sources"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := scopedSourceIDs(global); len(got) != 5 || global.Counts["sources"] != 5 || global.Total != 5 {
		t.Errorf("global: %v counts %v total %d, want all five", got, global.Counts, global.Total)
	}

	alpha, _ := kb.SearchScoped("parsing", SearchScope{Project: "alpha", Kinds: []string{"sources"}})
	if got := scopedSourceIDs(alpha); len(got) != 2 || !got[ids["alphaSrc"]] || !got[ids["bothSrc"]] {
		t.Errorf("alpha: %v, want the observation's and the record's sources", got)
	}
	beta, _ := kb.SearchScoped("parsing", SearchScope{Project: "beta", Kinds: []string{"sources"}})
	if got := scopedSourceIDs(beta); len(got) != 2 || !got[ids["betaSrc"]] || !got[ids["bothSrc"]] {
		t.Errorf("beta: %v, want the document's sources", got)
	}
	ws, _ := kb.SearchScoped("parsing", SearchScope{Workspace: true, Kinds: []string{"sources"}})
	if got := scopedSourceIDs(ws); len(got) != 1 || !got[ids["wsSrc"]] {
		t.Errorf("workspace: %v, want the workspace record's source", got)
	}
}

func TestSearchScoped_ASourceHitShowsTheProjectsItSpans(t *testing.T) {
	kb, ids := sourceScopeFixture(t)
	r, err := kb.SearchScoped("parsing", SearchScope{Kinds: []string{"sources"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.SourceSpans[ids["bothSrc"]], ","); got != "alpha,beta" {
		t.Errorf("bothSrc spans %q, want alpha,beta", got)
	}
	if got := strings.Join(r.SourceSpans[ids["alphaSrc"]], ","); got != "alpha" {
		t.Errorf("alphaSrc spans %q, want alpha", got)
	}
	if got := r.SourceSpans[ids["loneSrc"]]; len(got) != 0 {
		t.Errorf("a source cited by nothing spans %v, want none", got)
	}
}

func TestSearchScoped_UnknownKindStaysInvalidAndSourcesJoinTheDefaultSet(t *testing.T) {
	kb, _ := sourceScopeFixture(t)
	_, err := kb.SearchScoped("parsing", SearchScope{Kinds: []string{"sauces"}})
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown kind: err = %v, want ErrInvalid", err)
	}
	all, err := kb.SearchScoped("parsing", SearchScope{})
	if err != nil || scopedSourceIDs(all) == nil || len(scopedSourceIDs(all)) != 5 {
		t.Errorf("an all-kinds search leaves out sources: %v, %v", scopedSourceIDs(all), err)
	}
}
