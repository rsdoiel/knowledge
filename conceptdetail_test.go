package knowledge

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// `kb concept show` (DR-0051, GitHub issue #1): one concept's description,
// counts and the items linked to it, so curation needs no raw SQL.

func TestConceptDetail_UnknownNameIsNotFound(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.ConceptDetail("nothing", 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestConceptDetail_ExactMatchOnlyAndCaseVariantOffered(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("Toast", ""); err != nil {
		t.Fatal(err)
	}
	_, err := kb.ConceptDetail("toast", 10)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound (exact match only)", err)
	}
	if !strings.Contains(err.Error(), `"Toast"`) {
		t.Errorf("error %q does not offer the case variant Toast", err)
	}
}

func TestConceptDetail_ReachesEveryKind(t *testing.T) {
	kb := openTestKB(t)
	f := newConceptFixture(t, kb, "toast")
	d, err := kb.ConceptDetail("toast", 10)
	if err != nil {
		t.Fatalf("ConceptDetail: %v", err)
	}
	if d.Concept.Name != "toast" || d.Concept.Description != "to be deleted" {
		t.Errorf("concept = %+v", d.Concept)
	}
	if len(d.Projects) != 1 || d.Projects[0].ID != f.project || d.Projects[0].Project != "alpha" {
		t.Errorf("projects = %+v", d.Projects)
	}
	if len(d.Observations) != 1 || d.Observations[0].ID != f.observation || d.Observations[0].Project != "alpha" {
		t.Errorf("observations = %+v", d.Observations)
	}
	if len(d.Records) != 1 || d.Records[0].ID != f.record || d.Records[0].Title != "t" {
		t.Errorf("records = %+v", d.Records)
	}
	if len(d.DocumentSections) != 1 || d.DocumentSections[0].ID != f.section || d.DocumentSections[0].Project != "alpha" {
		t.Errorf("sections = %+v", d.DocumentSections)
	}
}

func TestConceptDetail_CountsEqualConceptUsage(t *testing.T) {
	kb := openTestKB(t)
	newConceptFixture(t, kb, "toast")
	d, err := kb.ConceptDetail("toast", 10)
	if err != nil {
		t.Fatal(err)
	}
	u, err := kb.ConceptUsage("toast")
	if err != nil {
		t.Fatal(err)
	}
	if d.Usage != u {
		t.Errorf("Usage = %+v, want %+v", d.Usage, u)
	}
}

func TestConceptDetail_LimitZeroGivesCountsOnly(t *testing.T) {
	kb := openTestKB(t)
	newConceptFixture(t, kb, "toast")
	d, err := kb.ConceptDetail("toast", 0)
	if err != nil {
		t.Fatal(err)
	}
	if d.Usage.Total() != 4 {
		t.Errorf("total = %d, want 4", d.Usage.Total())
	}
	if len(d.Projects)+len(d.Observations)+len(d.Records)+len(d.DocumentSections) != 0 {
		t.Errorf("limit 0 returned items: %+v", d)
	}
}

func TestConceptDetail_CapsPerKindNewestFirst(t *testing.T) {
	kb := openTestKB(t)
	cid, _ := kb.AddConcept("toast", "")
	pid, _ := kb.AddProject("alpha", "")
	for i := 1; i <= 12; i++ {
		oid, err := kb.AddObservation(pid, "note", fmt.Sprintf("obs %02d", i))
		if err != nil {
			t.Fatal(err)
		}
		// Explicit timestamps: CURRENT_TIMESTAMP only has one-second resolution.
		if _, err := kb.db.Exec(`UPDATE observations SET created_at = ? WHERE id = ?`,
			fmt.Sprintf("2026-01-%02d 10:00:00", i), oid); err != nil {
			t.Fatal(err)
		}
		if err := kb.LinkObservationConcept(oid, cid); err != nil {
			t.Fatal(err)
		}
	}
	d, err := kb.ConceptDetail("toast", 10)
	if err != nil {
		t.Fatal(err)
	}
	if d.Usage.Observations != 12 {
		t.Errorf("count = %d, want 12 (the cap limits items, not counts)", d.Usage.Observations)
	}
	if len(d.Observations) != 10 {
		t.Fatalf("items = %d, want 10", len(d.Observations))
	}
	if d.Observations[0].Body != "obs 12" || d.Observations[9].Body != "obs 03" {
		t.Errorf("order = %q ... %q, want newest first", d.Observations[0].Body, d.Observations[9].Body)
	}
}

func TestConceptDetail_NoLinksReturnsZeros(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("lonely", "just a concept"); err != nil {
		t.Fatal(err)
	}
	d, err := kb.ConceptDetail("lonely", 10)
	if err != nil {
		t.Fatalf("ConceptDetail: %v", err)
	}
	if d.Usage.Total() != 0 || len(d.Observations) != 0 {
		t.Errorf("detail = %+v, want zeros", d)
	}
	if d.Concept.Description != "just a concept" {
		t.Errorf("description = %q", d.Concept.Description)
	}
}
