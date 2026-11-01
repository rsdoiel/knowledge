package knowledge

import (
	"strings"
	"testing"
)

// K2 of the draft-then-promote plan (DR-0090): every read hides a draft.
// One test per site in the design's table, so removing the filter from any
// one site fails exactly that test.

// draftFixture holds one project with a reviewed and a drafted observation
// that share concept "caching" and the word "cache", plus a concept and a
// source that only the draft links.
type draftFixture struct {
	kb                *KnowledgeBase
	pid               int64
	reviewed, drafted int64
	both, draftOnly   int64
	draftOnlySource   int64
}

func newDraftFixture(t *testing.T) *draftFixture {
	t.Helper()
	kb := openTestKB(t)
	f := &draftFixture{kb: kb}
	var err error
	if f.pid, err = kb.AddProject("alpha", "a cache service"); err != nil {
		t.Fatal(err)
	}
	if f.reviewed, err = kb.AddObservation(f.pid, "finding", "cache reviewed finding"); err != nil {
		t.Fatal(err)
	}
	if f.drafted, err = kb.AddObservationDraft(f.pid, "finding", "cache drafted finding", "harvey", nil); err != nil {
		t.Fatal(err)
	}
	if f.both, err = kb.AddConcept("caching", "shared cache concept"); err != nil {
		t.Fatal(err)
	}
	if f.draftOnly, err = kb.AddConcept("draft-only-cache", "linked only from a draft"); err != nil {
		t.Fatal(err)
	}
	for _, l := range [][2]int64{{f.reviewed, f.both}, {f.drafted, f.both}, {f.drafted, f.draftOnly}} {
		if err = kb.LinkObservationConcept(l[0], l[1]); err != nil {
			t.Fatal(err)
		}
	}
	if f.draftOnlySource, err = kb.AddSource(Source{Title: "cache paper", IdentifierType: "doi", IdentifierValue: "10.1234/draftonly"}); err != nil {
		t.Fatal(err)
	}
	if err = kb.LinkObservationSource(f.drafted, f.draftOnlySource, "cited"); err != nil {
		t.Fatal(err)
	}
	return f
}

const draftBody = "cache drafted finding"

func TestDraftHidden_Search(t *testing.T) {
	f := newDraftFixture(t)
	hits, err := f.kb.Search("cache")
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if strings.Contains(h.Snippet, "drafted") {
			t.Errorf("Search returned the draft: %+v", h)
		}
	}
}

func TestDraftHidden_ObservationsList(t *testing.T) {
	f := newDraftFixture(t)
	obs, err := f.kb.Observations(f.pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 || obs[0].ID != f.reviewed {
		t.Errorf("Observations = %+v, want only the reviewed one", obs)
	}
}

func TestDraftHidden_ObservationsByStatus(t *testing.T) {
	f := newDraftFixture(t)
	for status, want := range map[string]int{"reviewed": 1, "drafted": 1, "all": 2} {
		obs, err := f.kb.ObservationsByStatus(f.pid, status)
		if err != nil || len(obs) != want {
			t.Errorf("ObservationsByStatus(%q) = %d rows, err %v; want %d", status, len(obs), err, want)
		}
	}
	if _, err := f.kb.ObservationsByStatus(f.pid, "bogus"); err == nil {
		t.Error("an unknown status was accepted")
	}
}

func TestDraftHidden_ObservationByIDFindsAndFlags(t *testing.T) {
	f := newDraftFixture(t)
	o, err := f.kb.ObservationByID(f.drafted)
	if err != nil || o.Status != "drafted" {
		t.Errorf("ObservationByID(draft) = %+v, %v; want found and flagged drafted", o, err)
	}
}

func TestDraftHidden_RecentObservationsInSummary(t *testing.T) {
	f := newDraftFixture(t)
	s, err := f.kb.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s, draftBody) || !strings.Contains(s, "cache reviewed finding") {
		t.Errorf("Summary = %q", s)
	}
}

func TestDraftHidden_FormatMarkdown(t *testing.T) {
	f := newDraftFixture(t)
	s, err := f.kb.FormatMarkdown(f.pid)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s, draftBody) {
		t.Errorf("FormatMarkdown shows the draft: %q", s)
	}
}

func TestDraftHidden_RecallByText(t *testing.T) {
	f := newDraftFixture(t)
	r, err := f.kb.RecallByText("we talked about caching today", 10, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range r.Hits {
		if h.ID == f.drafted && h.SourceType == "observation" {
			t.Errorf("RecallByText returned the draft: %+v", h)
		}
	}
	if len(r.Hits) != 1 {
		t.Errorf("hits = %d, want the reviewed observation only", len(r.Hits))
	}
	r, _ = f.kb.RecallByText("draft-only-cache", 10, "")
	if len(r.Hits) != 0 {
		t.Errorf("a concept only a draft links recalled %d hits", len(r.Hits))
	}
}

func TestDraftHidden_RecallByConceptNames(t *testing.T) {
	f := newDraftFixture(t)
	ms, err := f.kb.RecallByConceptNames([]string{"caching", "draft-only-cache"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range ms {
		if m.SourceType == "observation" && m.ID == f.drafted {
			t.Errorf("RecallByConceptNames returned the draft: %+v", m)
		}
	}
}

func TestDraftHidden_ConceptDetail(t *testing.T) {
	f := newDraftFixture(t)
	d, err := f.kb.ConceptDetail("caching", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Observations) != 1 {
		t.Errorf("detail lists %d observations, want 1", len(d.Observations))
	}
	d, err = f.kb.ConceptDetail("draft-only-cache", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Observations) != 0 {
		t.Errorf("detail of a draft-only concept lists %d observations", len(d.Observations))
	}
}

func TestDraftHidden_ConceptDetailSpans(t *testing.T) {
	f := newDraftFixture(t)
	d, err := f.kb.ConceptDetail("caching", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Spans) != 1 || d.Spans[0].Observations != 1 {
		t.Errorf("spans = %+v, want alpha with 1 observation", d.Spans)
	}
	d, _ = f.kb.ConceptDetail("draft-only-cache", 10)
	if len(d.Spans) != 0 {
		t.Errorf("a draft-only concept spans %+v", d.Spans)
	}
}

func TestDraftHidden_ScopedProjectConceptSet(t *testing.T) {
	f := newDraftFixture(t)
	got, err := f.kb.SearchScoped("cache", SearchScope{Project: "alpha", Kinds: []string{"concepts"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got.Results {
		if r.SourceID == f.draftOnly {
			t.Error("a concept only a draft links is in the project's concept set")
		}
	}
}

func TestDraftHidden_ScopedProjectSourceSet(t *testing.T) {
	f := newDraftFixture(t)
	got, err := f.kb.SearchScoped("cache", SearchScope{Project: "alpha", Kinds: []string{"sources"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got.Results {
		if r.SourceID == f.draftOnlySource {
			t.Error("a source only a draft cites is in the project's source set")
		}
	}
}

func TestDraftHidden_ScopedConceptSpans(t *testing.T) {
	f := newDraftFixture(t)
	got, err := f.kb.SearchScoped("cache", SearchScope{Kinds: []string{"concepts"}})
	if err != nil {
		t.Fatal(err)
	}
	if s := got.Spans[f.draftOnly]; len(s) != 0 {
		t.Errorf("a draft-only concept spans %v", s)
	}
}

func TestDraftHidden_ScopedSourceSpans(t *testing.T) {
	f := newDraftFixture(t)
	got, err := f.kb.SearchScoped("cache", SearchScope{Kinds: []string{"sources"}})
	if err != nil {
		t.Fatal(err)
	}
	if s := got.SourceSpans[f.draftOnlySource]; len(s) != 0 {
		t.Errorf("a draft-only source spans %v", s)
	}
}

func statRow(t *testing.T, rows []StatCount, name string) int {
	t.Helper()
	for _, r := range rows {
		if r.Name == name {
			return r.Count
		}
	}
	t.Fatalf("no stat %q in %+v", name, rows)
	return 0
}

func TestDraftHidden_StatsCountDraftsSeparately(t *testing.T) {
	f := newDraftFixture(t)
	s, err := f.kb.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if n := statRow(t, s.Rows, "observations"); n != 1 {
		t.Errorf("observations = %d, want 1 reviewed", n)
	}
	if n := statRow(t, s.Rows, "observation_drafts"); n != 1 {
		t.Errorf("observation_drafts = %d, want 1", n)
	}
}

func TestDraftHidden_StatsPerProject(t *testing.T) {
	f := newDraftFixture(t)
	s, _ := f.kb.Stats()
	if len(s.Projects) != 1 || s.Projects[0].Observations != 1 {
		t.Errorf("project stats = %+v, want 1 observation", s.Projects)
	}
	if s.Projects[0].Concepts != 1 {
		t.Errorf("project concepts = %d, want 1 (the draft-only concept is not the project's)", s.Projects[0].Concepts)
	}
}

func TestDraftHidden_ProjectDeleteCountsADraftAsContent(t *testing.T) {
	f := newDraftFixture(t)
	u, err := f.kb.ProjectUsage("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if u.Observations != 2 {
		t.Errorf("usage observations = %d, want 2 (a draft is content)", u.Observations)
	}
}

func TestDraftHidden_PromotedObservationAppears(t *testing.T) {
	f := newDraftFixture(t)
	if err := f.kb.PromoteObservation(f.drafted); err != nil {
		t.Fatal(err)
	}
	obs, _ := f.kb.Observations(f.pid)
	if len(obs) != 2 {
		t.Errorf("after promote Observations = %d, want 2", len(obs))
	}
	r, _ := f.kb.RecallByText("draft-only-cache", 10, "")
	if len(r.Hits) != 1 {
		t.Errorf("after promote recall hits = %d, want 1", len(r.Hits))
	}
}

func TestDraftHidden_StatsOrphanCheckIgnoresDrafts(t *testing.T) {
	f := newDraftFixture(t)
	if _, err := f.kb.db.Exec(`UPDATE observations SET project_id = NULL WHERE id = ?`, f.drafted); err != nil {
		t.Fatal(err)
	}
	s, err := f.kb.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if n := statRow(t, s.Checks, "observations_without_project"); n != 0 {
		t.Errorf("observations_without_project = %d, want 0 (the orphan is a draft)", n)
	}
}

func TestDraftHidden_StatsLinkCountsIgnoreDraftLinks(t *testing.T) {
	f := newDraftFixture(t)
	s, err := f.kb.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if n := statRow(t, s.Links, "observation_concepts"); n != 1 {
		t.Errorf("observation_concepts = %d, want 1 (the reviewed observation's link)", n)
	}
	if n := statRow(t, s.Links, "observation_sources"); n != 0 {
		t.Errorf("observation_sources = %d, want 0 (the only cited link is a draft's)", n)
	}
}
