package knowledge

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// RecallByText (DR-0051): match concepts in free text, then return every
// observation, record and document section linked to them, with the concepts
// each hit matched. RecallByConceptNames is deliberately left alone.

type recallCorpus struct {
	alpha, beta                  int64
	toast, jam                   int64
	obsBoth, obsToast, obsOther  int64
	recToast, recBoth            int64
	sectionToast, sectionBetaJam int64
}

func newRecallCorpus(t *testing.T, kb *KnowledgeBase) recallCorpus {
	t.Helper()
	var c recallCorpus
	c.alpha, _ = kb.AddProject("alpha", "")
	c.beta, _ = kb.AddProject("beta", "")
	c.toast, _ = kb.AddConcept("toast", "")
	c.jam, _ = kb.AddConcept("jam", "")

	obs := func(project int64, body, ts string, concepts ...int64) int64 {
		id, err := kb.AddObservation(project, "note", body)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := kb.db.Exec(`UPDATE observations SET created_at = ? WHERE id = ?`, ts, id); err != nil {
			t.Fatal(err)
		}
		for _, cid := range concepts {
			if err := kb.LinkObservationConcept(id, cid); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	rec := func(project int64, rid, ts string, concepts ...int64) int64 {
		id, err := kb.AddRecord(Record{
			RecordID: rid, ProjectID: project, Scope: "project", Path: "decisions/" + rid + ".md",
			Title: "record " + rid, Date: "2026-09-01", Status: "accepted", Kind: "decision",
			Body: "body " + rid, Checksum: "ck" + rid,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := kb.db.Exec(`UPDATE records SET ingested_at = ? WHERE id = ?`, ts, id); err != nil {
			t.Fatal(err)
		}
		for _, cid := range concepts {
			if err := kb.LinkRecordConcept(id, cid); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	c.obsBoth = obs(c.alpha, "both", "2026-03-01 10:00:00", c.toast, c.jam)
	c.obsToast = obs(c.alpha, "toast only", "2026-03-05 10:00:00", c.toast)
	c.obsOther = obs(c.beta, "jam only, beta", "2026-03-03 10:00:00", c.jam)
	c.recToast = rec(c.alpha, "0001", "2026-03-04 10:00:00", c.toast)
	c.recBoth = rec(c.beta, "0002", "2026-03-02 10:00:00", c.toast, c.jam)
	return c
}

func addRecallSections(t *testing.T, kb *KnowledgeBase, c *recallCorpus) {
	t.Helper()
	da, _ := kb.AddDocument(Document{ProjectID: c.alpha, Title: "guide", Format: "text", Path: "a.txt"})
	db2, _ := kb.AddDocument(Document{ProjectID: c.beta, Title: "notes", Format: "text", Path: "b.txt"})
	c.sectionToast, _ = kb.AddDocumentSection(DocumentSection{DocumentID: da, Level: "section", Heading: "Bread", Body: "x"})
	c.sectionBetaJam, _ = kb.AddDocumentSection(DocumentSection{DocumentID: db2, Level: "section", Heading: "Fruit", Body: "y"})
	if err := kb.LinkDocumentSectionConcept(c.sectionToast, c.toast); err != nil {
		t.Fatal(err)
	}
	if err := kb.LinkDocumentSectionConcept(c.sectionBetaJam, c.jam); err != nil {
		t.Fatal(err)
	}
}

func hitKeys(hits []TextRecallHit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, fmt.Sprintf("%s:%d", h.SourceType, h.ID))
	}
	return out
}

func TestRecallByText_NoConceptMatchIsEmptyNotError(t *testing.T) {
	kb := openTestKB(t)
	newRecallCorpus(t, kb)
	r, err := kb.RecallByText("nothing relevant here", 10, "")
	if err != nil {
		t.Fatalf("RecallByText: %v", err)
	}
	if len(r.Matched) != 0 || len(r.Hits) != 0 {
		t.Errorf("result = %+v, want empty", r)
	}
}

func TestRecallByText_ReturnsMatchedNamesInOrder(t *testing.T) {
	kb := openTestKB(t)
	newRecallCorpus(t, kb)
	r, err := kb.RecallByText("jam on toast", 10, "")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := kb.MatchConceptNames("jam on toast")
	if !reflect.DeepEqual(r.Matched, want) || len(want) != 2 {
		t.Errorf("Matched = %v, want %v", r.Matched, want)
	}
}

func TestRecallByText_HitCarriesExactlyItsConcepts(t *testing.T) {
	kb := openTestKB(t)
	c := newRecallCorpus(t, kb)
	r, err := kb.RecallByText("jam on toast", 10, "")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]TextRecallHit{}
	for _, h := range r.Hits {
		byID[fmt.Sprintf("%s:%d", h.SourceType, h.ID)] = h
	}
	if got := byID[fmt.Sprintf("observation:%d", c.obsToast)].Concepts; !reflect.DeepEqual(got, []string{"toast"}) {
		t.Errorf("toast-only observation concepts = %v", got)
	}
	both := byID[fmt.Sprintf("observation:%d", c.obsBoth)]
	if len(both.Concepts) != 2 || both.MatchCount != 2 {
		t.Errorf("both observation = %+v", both)
	}
	if h := byID[fmt.Sprintf("observation:%d", c.obsOther)]; h.Project != "beta" {
		t.Errorf("project = %q, want beta", h.Project)
	}
}

func TestRecallByText_RankingAgreesWithRecallByConceptNames(t *testing.T) {
	kb := openTestKB(t)
	newRecallCorpus(t, kb)
	old, err := kb.RecallByConceptNames([]string{"toast", "jam"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, m := range old {
		want = append(want, fmt.Sprintf("%s:%d", m.SourceType, m.ID))
	}
	r, err := kb.RecallByText("toast and jam", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := hitKeys(r.Hits); !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestRecallByText_IncludesDocumentSections(t *testing.T) {
	kb := openTestKB(t)
	c := newRecallCorpus(t, kb)
	addRecallSections(t, kb, &c)
	r, err := kb.RecallByText("toast", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("document_section:%d", c.sectionToast)
	found := false
	for _, k := range hitKeys(r.Hits) {
		if k == key {
			found = true
		}
	}
	if !found {
		t.Errorf("%s not in %v", key, hitKeys(r.Hits))
	}
}

func TestRecallByText_ProjectFilter(t *testing.T) {
	kb := openTestKB(t)
	c := newRecallCorpus(t, kb)
	addRecallSections(t, kb, &c)
	r, err := kb.RecallByText("toast and jam", 0, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Hits) == 0 {
		t.Fatal("no hits for alpha")
	}
	for _, h := range r.Hits {
		if h.Project != "alpha" {
			t.Errorf("hit %s:%d from project %q", h.SourceType, h.ID, h.Project)
		}
	}
	if _, err := kb.RecallByText("toast", 0, "nosuch"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown project: err = %v, want ErrNotFound", err)
	}
}

func TestRecallByText_LimitTruncatesAfterRanking(t *testing.T) {
	kb := openTestKB(t)
	c := newRecallCorpus(t, kb)
	r, err := kb.RecallByText("toast and jam", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Hits) != 1 || r.Hits[0].MatchCount != 2 {
		t.Fatalf("hits = %+v, want the single best (2 concepts)", r.Hits)
	}
	// Of the two-concept hits, the record (March 2nd) is newer than the observation (March 1st).
	if r.Hits[0].SourceType != "record" || r.Hits[0].ID != c.recBoth {
		t.Errorf("top hit = %s:%d, want record:%d", r.Hits[0].SourceType, r.Hits[0].ID, c.recBoth)
	}
}

func TestRecallByText_ListsLinkedProjectsSeparately(t *testing.T) {
	kb := openTestKB(t)
	c := newRecallCorpus(t, kb)
	if err := kb.LinkProjectConcept(c.beta, c.jam); err != nil {
		t.Fatal(err)
	}
	r, err := kb.RecallByText("jam", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Projects, []string{"beta"}) {
		t.Errorf("Projects = %v, want [beta]", r.Projects)
	}
	for _, h := range r.Hits {
		if h.SourceType == "project" {
			t.Errorf("a project appeared among the hits: %+v", h)
		}
	}
}

func TestRecallByText_CreatesNothing(t *testing.T) {
	kb := openTestKB(t)
	newRecallCorpus(t, kb)
	before, _ := kb.Concepts()
	if _, err := kb.RecallByText("toast, marmalade and Vegemite", 10, ""); err != nil {
		t.Fatal(err)
	}
	after, _ := kb.Concepts()
	if len(before) != len(after) {
		t.Errorf("concepts %d -> %d", len(before), len(after))
	}
}

func TestRecallByNames_SkipsMatchingAndIsCaseInsensitive(t *testing.T) {
	kb := openTestKB(t)
	newRecallCorpus(t, kb)
	r, err := kb.RecallByNames([]string{"TOAST", "nosuch"}, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Matched, []string{"toast"}) {
		t.Errorf("Matched = %v, want canonical [toast]", r.Matched)
	}
	if len(r.Hits) != 4 {
		t.Errorf("hits = %v, want 4 toast-linked items", hitKeys(r.Hits))
	}
}
