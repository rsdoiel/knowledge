package knowledge

import "testing"

// HasConcept answers whether a name is a concept, as ResolveConceptName would find
// it (case-insensitively) and without creating anything: the plan of an ingest
// needs to say which concepts it would create.

func TestHasConcept_FindsCaseInsensitivelyAndCreatesNothing(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("Streaming", "SSE"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Streaming", "streaming", "STREAMING"} {
		ok, err := kb.HasConcept(name)
		if err != nil || !ok {
			t.Errorf("HasConcept(%q) = %v, %v; want true", name, ok, err)
		}
	}
	ok, err := kb.HasConcept("backpressure")
	if err != nil || ok {
		t.Errorf("HasConcept(backpressure) = %v, %v; want false", ok, err)
	}
	cs, _ := kb.Concepts()
	if len(cs) != 1 {
		t.Errorf("asking created a concept: %d concepts", len(cs))
	}
}

func TestHasConcept_ABlankNameIsAnError(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.HasConcept("   "); err == nil {
		t.Error("a blank name should be an error")
	}
}
