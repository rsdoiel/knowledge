package main

import (
	"bytes"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// The unit tests for the scoring, near-existing exclusion and clustering logic
// moved to the library with it (library-lift-plan.md L3); these are the
// CLI-level tests that remain.

func TestCmdConceptSuggest_RendersVariantsInline(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0001", ProjectID: pid, Scope: "project", Path: "decisions/0001-x.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body:     "chunking discussed here, chunking again, chunking a third time, chunking a fourth",
		Checksum: "c1",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0002", ProjectID: pid, Scope: "project", Path: "decisions/0002-y.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body: "chunkings mentioned here", Checksum: "c2",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0003", ProjectID: pid, Scope: "project", Path: "decisions/0003-z.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body: "nothing at all related in this filler item", Checksum: "c3",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"suggest"}, &out); err != nil {
		t.Fatalf("concept suggest: %v", err)
	}
	if !strings.Contains(out.String(), "chunking (+chunkings)") {
		t.Errorf("output = %q, want \"chunking (+chunkings)\" rendered inline", out.String())
	}
}

func TestCmdConceptSuggest_RendersNearExistingSectionOnlyWhenNonEmpty(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0001", ProjectID: pid, Scope: "project", Path: "decisions/0001-x.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body: "chunkings mentioned here, chunkings again", Checksum: "c1",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0002", ProjectID: pid, Scope: "project", Path: "decisions/0002-y.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body: "nothing related in this filler item at all", Checksum: "c2",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}

	var withNearExisting bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"suggest"}, &withNearExisting); err != nil {
		t.Fatalf("concept suggest: %v", err)
	}
	if !strings.Contains(withNearExisting.String(), "near-existing") {
		t.Errorf("output = %q, want a near-existing section", withNearExisting.String())
	}

	kb.AddProject("beta", "")
	var noNearExisting bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"suggest", "--project", "beta"}, &noNearExisting); err != nil {
		t.Fatalf("concept suggest --project beta: %v", err)
	}
	if strings.Contains(noNearExisting.String(), "near-existing") {
		t.Errorf("output = %q, want no near-existing section for an empty project", noNearExisting.String())
	}
}

func TestCmdConceptSuggest_JSONIncludesNearExistingArray(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0001", ProjectID: pid, Scope: "project", Path: "decisions/0001-x.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body: "chunkings mentioned here, chunkings again", Checksum: "c1",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	var got struct {
		Candidates   []knowledge.ConceptCandidate  `json:"candidates"`
		NearExisting []knowledge.NearExistingMatch `json:"near_existing"`
	}
	runConceptJSON(t, kb, &got, "suggest")
	found := false
	for _, m := range got.NearExisting {
		if m.Token == "chunkings" && m.Concept == "chunking" {
			found = true
		}
	}
	if !found {
		t.Errorf("got.NearExisting = %+v, want chunkings ~ chunking", got.NearExisting)
	}
}

// Decision 9's "always on, no flag" must not break today's plain output for
// the ordinary case (no variants, nothing near-existing).
func TestCmdConceptSuggest_ExistingFixturesUnchangedWhenNoVariantsOrNearExisting(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0001", ProjectID: pid, Scope: "project", Path: "decisions/0001-x.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body: "widgetry widgetry appears here plainly", Checksum: "c1",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0002", ProjectID: pid, Scope: "project", Path: "decisions/0002-y.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body: "nothing related in this filler item", Checksum: "c2",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"suggest"}, &out); err != nil {
		t.Fatalf("concept suggest: %v", err)
	}
	want := " 1. widgetry                  score="
	if !strings.Contains(out.String(), want) {
		t.Errorf("output = %q, want it to still contain the plain, unvaried rendering %q", out.String(), want)
	}
	if strings.Contains(out.String(), "(+") || strings.Contains(out.String(), "near-existing") {
		t.Errorf("output = %q, want no variants/near-existing decoration for this plain case", out.String())
	}
}
