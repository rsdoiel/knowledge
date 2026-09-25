package knowledge

import (
	"errors"
	"strings"
	"testing"
)

// R1 of removal-verbs-plan.md (DR-0050): the library half of the removal verbs.
// Every delete refuses while something depends on the target and changes nothing;
// force removes the links and the row; the search entry goes with the row; a target
// that is not there is ErrNotFound, never a silent success.

func ftsRows(t *testing.T, kb *KnowledgeBase, sourceType string, id int64) int {
	t.Helper()
	if !kb.ftsAvailable {
		return 0
	}
	var n int
	if err := kb.db.QueryRow(`SELECT COUNT(*) FROM kb_fts WHERE source_type = ? AND source_id = ?`, sourceType, id).Scan(&n); err != nil {
		t.Fatalf("count kb_fts: %v", err)
	}
	return n
}

// snapshot counts every table the removal verbs touch, so "nothing changed" is checked.
func snapshot(t *testing.T, kb *KnowledgeBase) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, table := range []string{"projects", "observations", "concepts", "sources", "documents", "document_sections",
		"records", "project_concepts", "observation_concepts", "observation_sources", "observation_relations",
		"record_concepts", "record_relations", "document_section_concepts"} {
		out[table] = countRows(t, kb, table)
	}
	if kb.ftsAvailable {
		out["kb_fts"] = countRows(t, kb, "kb_fts")
	}
	return out
}

func sameSnapshot(t *testing.T, before, after map[string]int) {
	t.Helper()
	for k, v := range before {
		if after[k] != v {
			t.Errorf("table %s changed from %d to %d rows; a refusal must change nothing", k, v, after[k])
		}
	}
}

// ─── missing targets ─────────────────────────────────────────────────────────

func TestRemoval_MissingTargetsAreNotFound(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("p", "")
	oid, _ := kb.AddObservation(pid, "note", "b")
	kb.AddConcept("C", "")
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"DeleteProject", func() error { _, err := kb.DeleteProject("nosuch", false); return err }},
		{"ProjectUsage", func() error { _, err := kb.ProjectUsage("nosuch"); return err }},
		{"DeleteObservation", func() error { _, err := kb.DeleteObservation(9999, false); return err }},
		{"DeleteDocument", func() error { _, err := kb.DeleteDocument(9999, false); return err }},
		{"DeleteRecord", func() error { _, err := kb.DeleteRecord(9999); return err }},
		{"UnlinkProjectConcept missing project", func() error { return kb.UnlinkProjectConcept("nosuch", "C") }},
		{"UnlinkProjectConcept missing concept", func() error { return kb.UnlinkProjectConcept("p", "nosuch") }},
		{"UnlinkProjectConcept no such link", func() error { return kb.UnlinkProjectConcept("p", "C") }},
		{"UnlinkObservationConcept missing observation", func() error { return kb.UnlinkObservationConcept(9999, "C") }},
		{"UnlinkObservationConcept missing concept", func() error { return kb.UnlinkObservationConcept(oid, "nosuch") }},
		{"UnlinkObservationConcept no such link", func() error { return kb.UnlinkObservationConcept(oid, "C") }},
		{"UnlinkObservationSource missing observation", func() error { return kb.UnlinkObservationSource(9999, 1) }},
		{"UnlinkObservationSource no such link", func() error {
			sid, _ := kb.AddSource(Source{Title: "S"})
			return kb.UnlinkObservationSource(oid, sid)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if !errors.Is(err, ErrNotFound) {
				t.Errorf("error %v is not ErrNotFound", err)
			}
		})
	}
}

// ─── projects ────────────────────────────────────────────────────────────────

func TestDeleteProject_EmptyProjectIsRemovedWithItsSearchEntry(t *testing.T) {
	kb := openTestKB(t)
	keep, _ := kb.AddProject("keep", "")
	kb.AddProject("stray", "")
	stray, _ := kb.ProjectByName("stray")
	if ftsRows(t, kb, "project", stray.ID) == 0 && kb.ftsAvailable {
		t.Fatal("setup: the stray project should have a search entry")
	}
	u, err := kb.DeleteProject("stray", false)
	if err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if u.Name != "stray" || u.Total() != 0 {
		t.Errorf("usage = %+v, want an empty stray", u)
	}
	if p, _ := kb.ProjectByName("stray"); p != nil {
		t.Error("the project is still there")
	}
	if p, _ := kb.ProjectByName("keep"); p == nil || p.ID != keep {
		t.Error("another project was affected")
	}
	if n := ftsRows(t, kb, "project", stray.ID); n != 0 {
		t.Errorf("%d search entries left for the deleted project", n)
	}
}

func TestDeleteProject_OwningContentIsAlwaysRefused(t *testing.T) {
	for _, force := range []bool{false, true} {
		kb := openTestKB(t)
		pid, _ := kb.AddProject("busy", "")
		kb.AddObservation(pid, "note", "b")
		before := snapshot(t, kb)
		u, err := kb.DeleteProject("busy", force)
		var inUse *InUseError
		if !errors.As(err, &inUse) || !errors.Is(err, ErrInUse) {
			t.Fatalf("force=%v: error %v is not an *InUseError matching ErrInUse", force, err)
		}
		if u.Observations != 1 {
			t.Errorf("force=%v: usage %+v should count the observation", force, u)
		}
		if !strings.Contains(err.Error(), "observation") {
			t.Errorf("force=%v: message %q should say what it owns", force, err)
		}
		sameSnapshot(t, before, snapshot(t, kb))
	}
}

func TestDeleteProject_ConceptLinksNeedForceAndOnlyTheLinksGo(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("linked", "")
	cid, _ := kb.AddConcept("C", "")
	kb.LinkProjectConcept(pid, cid)
	before := snapshot(t, kb)
	if _, err := kb.DeleteProject("linked", false); !errors.Is(err, ErrInUse) {
		t.Fatalf("without force: %v, want ErrInUse", err)
	}
	sameSnapshot(t, before, snapshot(t, kb))

	u, err := kb.DeleteProject("linked", true)
	if err != nil {
		t.Fatalf("with force: %v", err)
	}
	if u.Concepts != 1 {
		t.Errorf("usage %+v should report the one concept link removed", u)
	}
	if p, _ := kb.ProjectByName("linked"); p != nil {
		t.Error("the project is still there")
	}
	if countRows(t, kb, "project_concepts") != 0 || countRows(t, kb, "concepts") != 1 {
		t.Error("the link should be gone and the concept itself kept")
	}
}

// ─── observations ────────────────────────────────────────────────────────────

func TestDeleteObservation_UnlinkedIsRemovedWithItsSearchEntry(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("p", "")
	keep, _ := kb.AddObservation(pid, "note", "keep me")
	gone, _ := kb.AddObservation(pid, "note", "delete me")
	if _, err := kb.DeleteObservation(gone, false); err != nil {
		t.Fatalf("DeleteObservation: %v", err)
	}
	obs, _ := kb.Observations(pid)
	if len(obs) != 1 || obs[0].ID != keep {
		t.Errorf("observations = %+v, want only the one to keep", obs)
	}
	if n := ftsRows(t, kb, "observation", gone); n != 0 {
		t.Errorf("%d search entries left", n)
	}
	if n := ftsRows(t, kb, "observation", keep); kb.ftsAvailable && n == 0 {
		t.Error("the other observation lost its search entry")
	}
}

func TestDeleteObservation_LinksNeedForceAndAreAllRemoved(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("p", "")
	target, _ := kb.AddObservation(pid, "note", "target")
	newer, _ := kb.AddObservation(pid, "note", "newer")
	older, _ := kb.AddObservation(pid, "note", "older")
	cid, _ := kb.AddConcept("C", "")
	sid, _ := kb.AddSource(Source{Title: "S"})
	kb.LinkObservationConcept(target, cid)
	kb.LinkObservationSource(target, sid, "cited")
	kb.AddObservationRelation(newer, target, "supersedes")
	kb.AddObservationRelation(target, older, "supersedes")
	before := snapshot(t, kb)

	u, err := kb.DeleteObservation(target, false)
	var inUse *InUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("without force: %v, want an *InUseError", err)
	}
	if u.Concepts != 1 || u.Sources != 1 || u.RelationsFrom != 1 || u.RelationsTo != 1 {
		t.Errorf("usage = %+v, want 1 of each", u)
	}
	sameSnapshot(t, before, snapshot(t, kb))

	if _, err := kb.DeleteObservation(target, true); err != nil {
		t.Fatalf("with force: %v", err)
	}
	for _, table := range []string{"observation_concepts", "observation_sources", "observation_relations"} {
		if n := countRows(t, kb, table); n != 0 {
			t.Errorf("%s has %d rows left", table, n)
		}
	}
	if countRows(t, kb, "observations") != 2 || countRows(t, kb, "concepts") != 1 || countRows(t, kb, "sources") != 1 {
		t.Error("only the target may go: the other observations, the concept and the source stay")
	}
}

// ─── documents ───────────────────────────────────────────────────────────────

func ingestedDocument(t *testing.T, kb *KnowledgeBase) (docID, sectionID int64) {
	t.Helper()
	pid, _ := kb.AddProject("docs", "")
	path := writeTempFile(t, "a.md", "## S\n\nbody about [[Chunking]].\n")
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatal(err)
	}
	d, secs, _ := ingestSections(t, kb, path)
	return d.ID, secs["S"].ID
}

func TestDeleteDocument_NoReviewedSummaryIsDeletedWithItsSections(t *testing.T) {
	kb := openTestKB(t)
	docID, secID := ingestedDocument(t, kb)
	if err := kb.DraftDocumentSummary(secID, "a summary", "test", nil); err != nil {
		t.Fatal(err)
	}
	u, err := kb.DeleteDocument(docID, false)
	if err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	if u.Sections == 0 || u.ReviewedSections != 0 {
		t.Errorf("usage = %+v", u)
	}
	for _, table := range []string{"documents", "document_sections", "document_section_concepts"} {
		if n := countRows(t, kb, table); n != 0 {
			t.Errorf("%s has %d rows left", table, n)
		}
	}
	if countRows(t, kb, "concepts") == 0 {
		t.Error("the concept the document named must stay")
	}
}

func TestDeleteDocument_ReviewedSummaryNeedsForceAndItsSearchEntryGoes(t *testing.T) {
	kb := openTestKB(t)
	docID, secID := ingestedDocument(t, kb)
	if err := kb.DraftDocumentSummary(secID, "a summary", "test", nil); err != nil {
		t.Fatal(err)
	}
	if err := kb.PromoteDocumentSummary(secID); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, kb)
	u, err := kb.DeleteDocument(docID, false)
	var inUse *InUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("without force: %v, want an *InUseError", err)
	}
	if u.ReviewedSections != 1 || !strings.Contains(err.Error(), "reviewed") {
		t.Errorf("usage %+v, message %q should name the reviewed summary", u, err)
	}
	sameSnapshot(t, before, snapshot(t, kb))

	if _, err := kb.DeleteDocument(docID, true); err != nil {
		t.Fatalf("with force: %v", err)
	}
	if countRows(t, kb, "documents") != 0 || countRows(t, kb, "document_sections") != 0 {
		t.Error("the document and its sections should be gone")
	}
	if n := ftsRows(t, kb, "document_summary", secID); n != 0 {
		t.Errorf("%d search entries left for the reviewed summary", n)
	}
}

// ─── records ─────────────────────────────────────────────────────────────────

func addRec(t *testing.T, kb *KnowledgeBase, pid int64, id, uuid string) int64 {
	t.Helper()
	rid, err := kb.AddRecord(Record{
		ProjectID: pid, Scope: "project", RecordID: id, Title: "t" + id, Date: "2026-09-25", Status: "proposed",
		Kind: "decision", Path: "x/" + id + "-t.md", UUID: uuid, Workspace: "w",
	})
	if err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	return rid
}

func TestDeleteRecord_RemovesTheRowItsLinksAndItsSearchEntryOnly(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("p", "")
	target := addRec(t, kb, pid, "0002", "22222222-2222-7222-8222-222222222222")
	other := addRec(t, kb, pid, "0001", "11111111-1111-7111-8111-111111111111")
	newer := addRec(t, kb, pid, "0003", "33333333-3333-7333-8333-333333333333")
	cid, _ := kb.AddConcept("C", "")
	kb.LinkRecordConcept(target, cid)
	kb.AddRecordRelation(target, other, "supersedes")
	kb.AddRecordRelation(newer, target, "relates_to")
	if kb.ftsAvailable && ftsRows(t, kb, "record", target) == 0 {
		t.Fatal("setup: the record should have a search entry")
	}

	u, err := kb.DeleteRecord(target)
	if err != nil {
		t.Fatalf("DeleteRecord: %v", err)
	}
	if u.Concepts != 1 || u.RelationsFrom != 1 || u.RelationsTo != 1 {
		t.Errorf("usage = %+v, want 1 of each", u)
	}
	if r, _ := kb.RecordByID(target); r != nil {
		t.Error("the record is still there")
	}
	if r, _ := kb.RecordByID(other); r == nil {
		t.Error("another record was removed")
	}
	for _, table := range []string{"record_concepts", "record_relations"} {
		if n := countRows(t, kb, table); n != 0 {
			t.Errorf("%s has %d rows left", table, n)
		}
	}
	if countRows(t, kb, "records") != 2 || countRows(t, kb, "concepts") != 1 {
		t.Error("only the target may go")
	}
	if n := ftsRows(t, kb, "record", target); n != 0 {
		t.Errorf("%d search entries left", n)
	}
}

// ─── unlinking ───────────────────────────────────────────────────────────────

func TestUnlink_RemovesExactlyOneLink(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("p", "")
	pid2, _ := kb.AddProject("p2", "")
	c1, _ := kb.AddConcept("C1", "")
	c2, _ := kb.AddConcept("C2", "")
	o1, _ := kb.AddObservation(pid, "note", "one")
	o2, _ := kb.AddObservation(pid, "note", "two")
	s1, _ := kb.AddSource(Source{Title: "S1"})
	s2, _ := kb.AddSource(Source{Title: "S2"})
	kb.LinkProjectConcept(pid, c1)
	kb.LinkProjectConcept(pid, c2)
	kb.LinkProjectConcept(pid2, c1)
	kb.LinkObservationConcept(o1, c1)
	kb.LinkObservationConcept(o1, c2)
	kb.LinkObservationConcept(o2, c1)
	kb.LinkObservationSource(o1, s1, "cited")
	kb.LinkObservationSource(o1, s2, "cited")
	kb.LinkObservationSource(o2, s1, "cited")

	if err := kb.UnlinkProjectConcept("p", "C1"); err != nil {
		t.Fatalf("UnlinkProjectConcept: %v", err)
	}
	if err := kb.UnlinkObservationConcept(o1, "C1"); err != nil {
		t.Fatalf("UnlinkObservationConcept: %v", err)
	}
	if err := kb.UnlinkObservationSource(o1, s1); err != nil {
		t.Fatalf("UnlinkObservationSource: %v", err)
	}
	if got := countRows(t, kb, "project_concepts"); got != 2 {
		t.Errorf("project_concepts = %d, want 2 (three minus one)", got)
	}
	if got := countRows(t, kb, "observation_concepts"); got != 2 {
		t.Errorf("observation_concepts = %d, want 2", got)
	}
	if got := countRows(t, kb, "observation_sources"); got != 2 {
		t.Errorf("observation_sources = %d, want 2", got)
	}
	// A second unlink of the same link is not a silent success.
	if err := kb.UnlinkProjectConcept("p", "C1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unlinking twice: %v, want ErrNotFound", err)
	}
}

func TestInUseError_MessageNamesTheEntityAndTheCounts(t *testing.T) {
	err := &InUseError{Entity: "observation", Name: "7", Counts: []UsageCount{{"concept links", 2}, {"source links", 1}}}
	for _, want := range []string{"observation", "7", "2 concept links", "1 source links"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q should contain %q", err, want)
		}
	}
	if !errors.Is(err, ErrInUse) {
		t.Error("an *InUseError must match ErrInUse")
	}
}
