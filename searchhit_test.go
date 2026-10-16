package knowledge

import (
	"strings"
	"testing"
)

// A search hit should say where it is and carry enough to act on (TODO, raised
// 2026-10-14): a record's qualified reference and status, and the passage that
// matched with the section it is in.

func searchHitFixture(t *testing.T) *KnowledgeBase {
	t.Helper()
	kb := openTestKB(t)
	p, _ := kb.AddProject("alpha", "")
	body := "\n**Context.** We needed a breakfast.\n\n**Decision.** Use toast for breakfast every day.\n\n**Rationale.** Bread keeps.\n"
	if _, err := kb.AddRecord(Record{RecordID: "0001", ProjectID: p, Scope: "project", Path: "decisions/0001-x.md",
		Title: "Breakfast policy", Date: "2026-10-01", Status: "accepted", Kind: "decision", Body: body, Checksum: "c1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := kb.AddRecord(Record{RecordID: "0002", ProjectID: 0, Scope: "workspace", Path: "decisions/0002-y.md",
		Title: "Toast standard", Date: "2026-10-02", Status: "proposed", Kind: "decision", Body: "\n**Context.** Nothing to see.\n", Checksum: "c2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := kb.AddObservation(p, "note", "A long note. "+strings.Repeat("filler words go here. ", 40)+"The marmalade is the point. "+strings.Repeat("more filler. ", 40)); err != nil {
		t.Fatal(err)
	}
	return kb
}

func findHit(t *testing.T, rs []KBSearchResult, sourceType string, pred func(KBSearchResult) bool) KBSearchResult {
	t.Helper()
	for _, r := range rs {
		if r.SourceType == sourceType && pred(r) {
			return r
		}
	}
	t.Fatalf("no %s hit in %+v", sourceType, rs)
	return KBSearchResult{}
}

func checkRecordHit(t *testing.T, rs []KBSearchResult) {
	t.Helper()
	h := findHit(t, rs, "record", func(r KBSearchResult) bool { return r.Label == "DR-0001" })
	if h.Ref != "alpha/DR-0001" || h.Status != "accepted" {
		t.Errorf("ref/status = %q/%q, want alpha/DR-0001 accepted", h.Ref, h.Status)
	}
	if !strings.Contains(h.Passage, "«toast»") && !strings.Contains(strings.ToLower(h.Passage), "«toast»") {
		t.Errorf("passage %q does not mark the match", h.Passage)
	}
	if h.Section != "Decision" {
		t.Errorf("section = %q, want Decision", h.Section)
	}
	w := findHit(t, rs, "record", func(r KBSearchResult) bool { return r.Label == "DR-0002" })
	if w.Ref != "workspace/DR-0002" || w.Status != "proposed" {
		t.Errorf("workspace hit ref/status = %q/%q", w.Ref, w.Status)
	}
	// Matched in the title only: the section is the title, not a guess at a label.
	if w.Section != "title" {
		t.Errorf("title-only match: section = %q, want title", w.Section)
	}
}

func TestSearch_HitCarriesRefStatusPassageAndSection(t *testing.T) {
	kb := searchHitFixture(t)
	rs, err := kb.Search("toast")
	if err != nil {
		t.Fatal(err)
	}
	checkRecordHit(t, rs)
}

func TestSearchScoped_HitCarriesTheSameFields(t *testing.T) {
	kb := searchHitFixture(t)
	r, err := kb.SearchScoped("toast", SearchScope{})
	if err != nil {
		t.Fatal(err)
	}
	checkRecordHit(t, r.Results)
}

func TestSearch_ObservationPassageIsTheMatchNotTheWholeBody(t *testing.T) {
	kb := searchHitFixture(t)
	rs, err := kb.Search("marmalade")
	if err != nil {
		t.Fatal(err)
	}
	h := findHit(t, rs, "observation", func(KBSearchResult) bool { return true })
	if !strings.Contains(h.Passage, "«marmalade»") {
		t.Errorf("passage = %q", h.Passage)
	}
	if len(h.Passage) > 300 {
		t.Errorf("passage is %d bytes, want a short excerpt", len(h.Passage))
	}
	if h.Ref != "" || h.Status != "" || h.Section != "" {
		t.Errorf("an observation has no ref, status or section: %+v", h)
	}
}

func TestSearch_ConceptAndProjectHitsHaveNoPassageFields(t *testing.T) {
	kb := searchHitFixture(t)
	kb.AddConcept("toastiness", "how toasted")
	rs, err := kb.Search("toastiness")
	if err != nil {
		t.Fatal(err)
	}
	h := findHit(t, rs, "concept", func(KBSearchResult) bool { return true })
	if h.Ref != "" || h.Status != "" || h.Section != "" {
		t.Errorf("concept hit: %+v", h)
	}
}
