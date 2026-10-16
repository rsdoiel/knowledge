package knowledge

import "testing"

// A database's shape without SQL (TODO, raised 2026-10-14): row counts per
// table, link counts per join table, the facts a curator checks by hand, and a
// line per project.

func statCount(t *testing.T, list []StatCount, name string) int {
	t.Helper()
	for _, c := range list {
		if c.Name == name {
			return c.Count
		}
	}
	t.Fatalf("no %q in %+v", name, list)
	return 0
}

func TestStats_CountsRowsLinksAndChecks(t *testing.T) {
	kb := openTestKB(t)
	f := newConceptFixture(t, kb, "toast") // one of everything, two concepts linked everywhere
	if _, err := kb.AddConcept("orphan", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := kb.AddSource(Source{Title: "unused"}); err != nil {
		t.Fatal(err)
	}
	other, _ := kb.AddObservation(f.project, "note", "second")
	if err := kb.AddObservationRelation(other, f.observation, "relates_to"); err != nil {
		t.Fatal(err)
	}

	s, err := kb.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	for name, want := range map[string]int{
		"projects": 1, "observations": 2, "concepts": 3, "sources": 1,
		"records": 1, "documents": 1, "document_sections": 1,
	} {
		if got := statCount(t, s.Rows, name); got != want {
			t.Errorf("rows[%s] = %d, want %d", name, got, want)
		}
	}
	for name, want := range map[string]int{
		"project_concepts": 2, "observation_concepts": 2, "record_concepts": 2,
		"document_section_concepts": 2, "observation_sources": 0,
		"observation_relations": 1, "record_relations": 0,
	} {
		if got := statCount(t, s.Links, name); got != want {
			t.Errorf("links[%s] = %d, want %d", name, got, want)
		}
	}
	for name, want := range map[string]int{
		"observations_without_project": 0,
		"concepts_without_links":       1, // orphan
		"sources_without_links":        1,
		"records_without_project":      0,
		"documents_without_project":    0,
	} {
		if got := statCount(t, s.Checks, name); got != want {
			t.Errorf("checks[%s] = %d, want %d", name, got, want)
		}
	}
	if len(s.Projects) != 1 {
		t.Fatalf("projects = %+v", s.Projects)
	}
	p := s.Projects[0]
	if p.Name != "alpha" || p.Observations != 2 || p.Records != 1 || p.Documents != 1 || p.Concepts != 2 {
		t.Errorf("project line = %+v, want alpha 2 observations, 1 record, 1 document, 2 concepts", p)
	}
}

func TestStats_EmptyDatabaseIsAllZeros(t *testing.T) {
	kb := openTestKB(t)
	s, err := kb.Stats()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range append(append([]StatCount{}, s.Rows...), append(s.Links, s.Checks...)...) {
		if c.Count != 0 {
			t.Errorf("%s = %d on an empty database", c.Name, c.Count)
		}
	}
	if len(s.Projects) != 0 {
		t.Errorf("projects = %+v", s.Projects)
	}
}

// A project's concepts are the union of the links on the project, on its
// observations and on its records (DR-0073), counted once each.
func TestStats_ProjectConceptsAreTheUnionCountedOnce(t *testing.T) {
	kb := openTestKB(t)
	p, _ := kb.AddProject("alpha", "")
	o, _ := kb.AddObservation(p, "note", "x")
	a, _ := kb.AddConcept("a", "")
	b, _ := kb.AddConcept("b", "")
	kb.LinkProjectConcept(p, a)
	kb.LinkObservationConcept(o, a) // same concept again
	kb.LinkObservationConcept(o, b)
	s, err := kb.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Projects[0].Concepts; got != 2 {
		t.Errorf("concepts = %d, want 2", got)
	}
}

// `concept show` should say which projects a concept spans, not only the ones
// it is tagged on directly (TODO, 2026-10-14).
func TestConceptDetail_SpansProjectsThroughTheirObservations(t *testing.T) {
	kb := openTestKB(t)
	f := newConceptFixture(t, kb, "toast")
	beta, _ := kb.AddProject("beta", "")
	ob, _ := kb.AddObservation(beta, "note", "mentions toast")
	if err := kb.LinkObservationConcept(ob, f.concept); err != nil {
		t.Fatal(err)
	}
	d, err := kb.ConceptDetail("toast", 10)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ConceptSpan{}
	for _, s := range d.Spans {
		got[s.Project] = s
	}
	if len(got) != 2 {
		t.Fatalf("spans = %+v, want alpha and beta", d.Spans)
	}
	if a := got["alpha"]; !a.Tagged || a.Observations != 1 || a.Records != 1 || a.DocumentSections != 1 {
		t.Errorf("alpha = %+v, want tagged with one of each", a)
	}
	if b := got["beta"]; b.Tagged || b.Observations != 1 || b.Records != 0 {
		t.Errorf("beta = %+v, want untagged, one observation", b)
	}
	// limit zero still gives the counts only, so no spans either.
	if d0, _ := kb.ConceptDetail("toast", 0); len(d0.Spans) != 0 {
		t.Errorf("limit 0 returned spans: %+v", d0.Spans)
	}
}
