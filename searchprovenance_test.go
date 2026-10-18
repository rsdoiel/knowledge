package knowledge

import (
	"fmt"
	"strings"
	"testing"
)

// ─── a hit says what supports it (DR-0080 item 1) ────────────────────────────

type provFixture struct {
	kb                 *KnowledgeBase
	pid                int64
	srcA, srcB         int64
	obs, rec, doc, sec int64
}

func newProvFixture(t *testing.T) provFixture {
	t.Helper()
	kb := openTestKB(t)
	f := provFixture{kb: kb}
	f.pid, _ = kb.AddProject("alpha", "")
	f.srcA, _ = kb.AddSource(Source{Title: "SPARQL 1.1", IdentifierType: "doi", IdentifierValue: "10.1234/sparql",
		PublishedDate: "2013", Rights: "CC-BY", Version: "1.1", Publisher: "W3C"})
	f.srcB, _ = kb.AddSource(Source{Title: "A page", IdentifierType: "url", IdentifierValue: "https://example.org/a"})
	f.obs, _ = kb.AddObservation(f.pid, "finding", "The marmalade result is reproducible.")
	f.rec = addTestRecord(t, kb, Record{RecordID: "0001", ProjectID: f.pid, Title: "Breakfast policy",
		Body: "\n**Decision.** Use toast for breakfast every day.\n", Checksum: "c1", Path: "decisions/0001-x.md"})
	f.doc, _ = kb.AddDocument(Document{ProjectID: f.pid, Title: "A Story", Format: "markdown", Path: "story.md", Checksum: "dk"})
	f.sec, _ = kb.AddDocumentSection(DocumentSection{DocumentID: f.doc, Level: "gist"})
	if err := kb.DraftDocumentSummary(f.sec, "a distinctive summary about hedgehogs", "human", nil); err != nil {
		t.Fatal(err)
	}
	if err := kb.PromoteDocumentSummary(f.sec); err != nil {
		t.Fatal(err)
	}
	for _, err := range []error{
		kb.LinkObservationSourceAt(f.obs, f.srcA, "cited", "v3", "sha256:cc"),
		kb.LinkObservationSource(f.obs, f.srcB, "retrieved"),
		kb.LinkRecordSource(f.rec, f.srcA, "cited", "", ""),
		kb.LinkDocumentSource(f.doc, f.srcB, "cited", "", ""),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func hitFor(t *testing.T, rs []KBSearchResult, sourceType string) KBSearchResult {
	t.Helper()
	return findHit(t, rs, sourceType, func(KBSearchResult) bool { return true })
}

func TestSearch_AnObservationHitListsTheSourcesItCites(t *testing.T) {
	f := newProvFixture(t)
	rs, err := f.kb.Search("marmalade")
	if err != nil {
		t.Fatal(err)
	}
	h := hitFor(t, rs, "observation")
	if len(h.Sources) != 2 || h.Retracted || h.Material != nil {
		t.Fatalf("hit = %+v, want two sources, not retracted, no material", h)
	}
	a := h.Sources[0]
	if a.ID != f.srcA || a.Title != "SPARQL 1.1" || a.IdentifierType != "doi" || a.IdentifierValue != "10.1234/sparql" ||
		a.PublishedDate != "2013" || a.Rights != "CC-BY" || a.Version != "1.1" || a.Relationship != "cited" ||
		a.SourceVersion != "v3" || a.SourceHash != "sha256:cc" || a.Retracted {
		t.Errorf("first source = %+v", a)
	}
	if b := h.Sources[1]; b.ID != f.srcB || b.Relationship != "retrieved" {
		t.Errorf("second source = %+v", b)
	}
}

func TestSearch_ARetractedCitationMarksTheHit(t *testing.T) {
	f := newProvFixture(t)
	if err := f.kb.RetractSource(f.srcA, "withdrawn by the publisher"); err != nil {
		t.Fatal(err)
	}
	rs, _ := f.kb.Search("marmalade")
	h := hitFor(t, rs, "observation")
	if !h.Retracted || !h.Sources[0].Retracted || h.Sources[0].RetractionNote != "withdrawn by the publisher" || h.Sources[1].Retracted {
		t.Errorf("hit = %+v, want it retracted through the first source only", h)
	}
	// An unrelated hit stays clean.
	other, _ := f.kb.Search("hedgehogs")
	if hh := hitFor(t, other, "document_summary"); hh.Retracted {
		t.Errorf("the document cites only srcB: %+v", hh)
	}
}

func TestSearch_ARecordHitCarriesItsSourcesAndMaterial(t *testing.T) {
	f := newProvFixture(t)
	rs, _ := f.kb.Search("toast")
	h := hitFor(t, rs, "record")
	if len(h.Sources) != 1 || h.Sources[0].ID != f.srcA {
		t.Errorf("sources = %+v", h.Sources)
	}
	if h.Material == nil || h.Material.Path != "decisions/0001-x.md" || h.Material.Checksum != "c1" ||
		h.Material.Format != "markdown" || h.Material.IngestedAt.IsZero() {
		t.Errorf("material = %+v", h.Material)
	}
}

func TestSearch_ADocumentSummaryHitCarriesTheDocumentsSourcesAndMaterial(t *testing.T) {
	f := newProvFixture(t)
	rs, _ := f.kb.Search("hedgehogs")
	h := hitFor(t, rs, "document_summary")
	if len(h.Sources) != 1 || h.Sources[0].ID != f.srcB {
		t.Errorf("sources = %+v", h.Sources)
	}
	if h.Material == nil || h.Material.Path != "story.md" || h.Material.Checksum != "dk" || h.Material.Format != "markdown" {
		t.Errorf("material = %+v", h.Material)
	}
}

func TestSearch_AHitWithNoSourcesHasAnEmptyListNotNil(t *testing.T) {
	f := newProvFixture(t)
	f.kb.AddObservation(f.pid, "note", "The kiwi is unrelated.")
	f.kb.AddConcept("kiwi-fruit", "a fruit")
	rs, _ := f.kb.Search("kiwi")
	for _, r := range rs {
		if r.Sources == nil || len(r.Sources) != 0 || r.Retracted || r.Material != nil {
			t.Errorf("%s hit %q = %+v, want an empty non-nil Sources and no material", r.SourceType, r.Label, r)
		}
	}
	if len(rs) == 0 {
		t.Fatal("no kiwi hits")
	}
}

func TestSearchScoped_HitsCarryTheSameProvenance(t *testing.T) {
	f := newProvFixture(t)
	got, err := f.kb.SearchScoped("marmalade", SearchScope{Project: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	h := hitFor(t, got.Results, "observation")
	if len(h.Sources) != 2 {
		t.Errorf("scoped hit sources = %+v, want 2", h.Sources)
	}
}

func TestRecall_HitsCarryProvenance(t *testing.T) {
	f := newProvFixture(t)
	cid, _ := f.kb.AddConcept("hedgehog-study", "")
	for _, err := range []error{
		f.kb.LinkObservationConcept(f.obs, cid), f.kb.LinkRecordConcept(f.rec, cid), f.kb.LinkDocumentSectionConcept(f.sec, cid),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	f.kb.RetractSource(f.srcA, "gone")
	rec, err := f.kb.RecallByNames([]string{"hedgehog-study"}, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]TextRecallHit{}
	for _, h := range rec.Hits {
		by[h.SourceType] = h
	}
	if o := by["observation"]; len(o.Sources) != 2 || !o.Retracted {
		t.Errorf("observation hit = %+v", o)
	}
	if r := by["record"]; len(r.Sources) != 1 || r.Material == nil || r.Material.Path != "decisions/0001-x.md" || !r.Retracted {
		t.Errorf("record hit = %+v", r)
	}
	if d := by["document_gist"]; len(d.Sources) != 1 || d.Material == nil || d.Material.Path != "story.md" || d.Retracted {
		t.Errorf("document hit = %+v", d)
	}
}

// A page of results costs a fixed handful of queries, not one per hit.
func TestSearch_ProvenanceIsFetchedInBatches(t *testing.T) {
	f := newProvFixture(t)
	for i := 0; i < 30; i++ {
		id, _ := f.kb.AddObservation(f.pid, "note", fmt.Sprintf("zebra sighting number %d", i))
		f.kb.LinkObservationSource(id, f.srcA, "cited")
	}
	before := provenanceQueryCount.Load()
	rs, err := f.kb.Search("zebra")
	if err != nil || len(rs) != 30 {
		t.Fatalf("%d hits, %v", len(rs), err)
	}
	if n := provenanceQueryCount.Load() - before; n < 1 || n > 4 {
		t.Errorf("30 hits used %d provenance queries, want a handful", n)
	}
	for _, r := range rs {
		if len(r.Sources) != 1 {
			t.Fatalf("hit %+v lost its source", r)
		}
	}
	if !strings.Contains(rs[0].Snippet, "zebra") {
		t.Errorf("unexpected hit %+v", rs[0])
	}
}
