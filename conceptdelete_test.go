package knowledge

import (
	"errors"
	"testing"
)

// `kb concept delete` (TODO.md, requested 2026-09-23): a junk concept such as
// "..." had no way out of the database, since kb had no verb to remove one.

// conceptFixture builds one concept linked from every kind of thing that can
// link to one, and returns the ids so a test can check what survives.
type conceptFixture struct {
	concept, other                        int64
	project, observation, record, section int64
}

func newConceptFixture(t *testing.T, kb *KnowledgeBase, name string) conceptFixture {
	t.Helper()
	var f conceptFixture
	var err error
	if f.concept, err = kb.AddConcept(name, "to be deleted"); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	if f.other, err = kb.AddConcept("keeper", ""); err != nil {
		t.Fatalf("AddConcept keeper: %v", err)
	}
	f.project, _ = kb.AddProject("alpha", "")
	f.observation, _ = kb.AddObservation(f.project, "note", "an observation")
	f.record, err = kb.AddRecord(Record{
		RecordID: "0001", ProjectID: f.project, Scope: "project", Path: "decisions/0001-x.md",
		Title: "t", Date: "2026-09-23", Status: "accepted", Kind: "decision", Body: "b", Checksum: "c1",
	})
	if err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	doc, _ := kb.AddDocument(Document{ProjectID: f.project, Title: "d", Format: "text", Path: "a.txt"})
	f.section, _ = kb.AddDocumentSection(DocumentSection{DocumentID: doc, Level: "section", Body: "text"})
	for _, c := range []int64{f.concept, f.other} {
		if err := kb.LinkProjectConcept(f.project, c); err != nil {
			t.Fatalf("LinkProjectConcept: %v", err)
		}
		if err := kb.LinkObservationConcept(f.observation, c); err != nil {
			t.Fatalf("LinkObservationConcept: %v", err)
		}
		if err := kb.LinkRecordConcept(f.record, c); err != nil {
			t.Fatalf("LinkRecordConcept: %v", err)
		}
		if err := kb.LinkDocumentSectionConcept(f.section, c); err != nil {
			t.Fatalf("LinkDocumentSectionConcept: %v", err)
		}
	}
	return f
}

func conceptNames(t *testing.T, kb *KnowledgeBase) map[string]bool {
	t.Helper()
	cs, err := kb.Concepts()
	if err != nil {
		t.Fatalf("Concepts: %v", err)
	}
	out := map[string]bool{}
	for _, c := range cs {
		out[c.Name] = true
	}
	return out
}

func TestConceptUsage_CountsEveryKindOfLink(t *testing.T) {
	kb := openTestKB(t)
	newConceptFixture(t, kb, "junk")
	u, err := kb.ConceptUsage("junk")
	if err != nil {
		t.Fatalf("ConceptUsage: %v", err)
	}
	if u.Name != "junk" || u.Projects != 1 || u.Observations != 1 || u.Records != 1 || u.DocumentSections != 1 || u.Total() != 4 {
		t.Errorf("usage = %+v, want one link of each kind (total 4)", u)
	}
}

func TestConceptUsage_UnlinkedConceptHasNone(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("lonely", "")
	u, err := kb.ConceptUsage("lonely")
	if err != nil || u.Total() != 0 {
		t.Errorf("usage = %+v, %v; want zero links", u, err)
	}
}

func TestConceptUsage_UnknownConceptErrors(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.ConceptUsage("nonesuch"); err == nil {
		t.Error("ConceptUsage(unknown) = nil error, want one")
	}
}

func TestDeleteConcept_UnlinkedConceptIsRemovedIncludingItsSearchEntry(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("zzzsearchable", "findable before")
	before, _ := kb.Search("zzzsearchable")
	if len(before) == 0 {
		t.Skip("full-text search is not available in this build")
	}
	u, err := kb.DeleteConcept("zzzsearchable", false)
	if err != nil {
		t.Fatalf("DeleteConcept: %v", err)
	}
	if u.Total() != 0 {
		t.Errorf("usage = %+v, want none", u)
	}
	if conceptNames(t, kb)["zzzsearchable"] {
		t.Error("concept still listed after delete")
	}
	if after, _ := kb.Search("zzzsearchable"); len(after) != 0 {
		t.Errorf("search still finds %d result(s) after delete, want none", len(after))
	}
}

func TestDeleteConcept_LinkedConceptIsRefusedWithoutForceAndNothingChanges(t *testing.T) {
	kb := openTestKB(t)
	f := newConceptFixture(t, kb, "junk")
	_, err := kb.DeleteConcept("junk", false)
	var inUse *ConceptInUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("err = %v, want a *ConceptInUseError", err)
	}
	if inUse.Usage.Total() != 4 {
		t.Errorf("error usage = %+v, want the four links reported", inUse.Usage)
	}
	if !conceptNames(t, kb)["junk"] {
		t.Error("concept was deleted despite the refusal")
	}
	if cs, _ := kb.RecordConcepts(f.record); len(cs) != 2 {
		t.Errorf("record concepts = %v, want both links intact", cs)
	}
}

func TestDeleteConcept_ForceUnlinksEverywhereAndLeavesEverythingElseAlone(t *testing.T) {
	kb := openTestKB(t)
	f := newConceptFixture(t, kb, "junk")
	u, err := kb.DeleteConcept("junk", true)
	if err != nil {
		t.Fatalf("DeleteConcept(force): %v", err)
	}
	if u.Total() != 4 {
		t.Errorf("reported usage = %+v, want the four links that were removed", u)
	}
	if conceptNames(t, kb)["junk"] {
		t.Error("concept still listed")
	}
	if !conceptNames(t, kb)["keeper"] {
		t.Error("an unrelated concept was deleted")
	}
	if cs, _ := kb.RecordConcepts(f.record); len(cs) != 1 || cs[0].Name != "keeper" {
		t.Errorf("record concepts = %v, want only keeper", cs)
	}
	if cs, _ := kb.DocumentSectionConcepts(f.section); len(cs) != 1 || cs[0].Name != "keeper" {
		t.Errorf("section concepts = %v, want only keeper", cs)
	}
	if u, err := kb.ConceptUsage("keeper"); err != nil || u.Observations != 1 || u.Projects != 1 || u.Records != 1 || u.DocumentSections != 1 {
		t.Errorf("keeper usage = %+v, %v; want every one of its links intact, including the observation", u, err)
	}
	if cs, _ := kb.ProjectConcepts(f.project); len(cs) != 1 || cs[0].Name != "keeper" {
		t.Errorf("project concepts = %v, want only keeper", cs)
	}
	if r, err := kb.RecordByID(f.record); err != nil || r == nil {
		t.Errorf("the linked record was affected: %v, %v", r, err)
	}
	if d, err := kb.DocumentByPath("a.txt"); err != nil || d == nil {
		t.Errorf("the linked document was affected: %v, %v", d, err)
	}
}

func TestDeleteConcept_UnknownConceptErrorsAndChangesNothing(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("keeper", "")
	if _, err := kb.DeleteConcept("nonesuch", true); err == nil {
		t.Error("DeleteConcept(unknown) = nil error, want one")
	}
	if !conceptNames(t, kb)["keeper"] {
		t.Error("an unrelated concept disappeared")
	}
}

func TestDeleteConcept_NameMatchIsExactCase(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("C++", "")
	if _, err := kb.DeleteConcept("c++", true); err == nil {
		t.Error("DeleteConcept(c++) deleted or matched C++: a destructive verb must not guess a case")
	}
	if !conceptNames(t, kb)["C++"] {
		t.Error("C++ was deleted by a differently-cased name")
	}
}

func TestDeleteConcept_ANameCanBeCreatedAgainAfterwards(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("again", "one")
	if _, err := kb.DeleteConcept("again", false); err != nil {
		t.Fatalf("DeleteConcept: %v", err)
	}
	second, err := kb.AddConcept("again", "two")
	if err != nil || second == 0 {
		t.Fatalf("AddConcept after delete = %d, %v", second, err)
	}
	if !conceptNames(t, kb)["again"] {
		t.Error("concept not recreated")
	}
}
