package main

import (
	"bytes"
	"fmt"
	"testing"
)

func TestCmdLink_ProjectToConcept(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("proj", ""); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if _, err := kb.AddConcept("streaming", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	var out bytes.Buffer
	if err := cmdLink(kb, false, []string{"project", "proj", "streaming"}, &out); err != nil {
		t.Fatalf("link project: %v", err)
	}
	p, _ := kb.ProjectByName("proj")
	concepts, err := kb.ProjectConcepts(p.ID)
	if err != nil {
		t.Fatalf("ProjectConcepts: %v", err)
	}
	if len(concepts) != 1 || concepts[0].Name != "streaming" {
		t.Errorf("concepts = %+v, want one concept named streaming", concepts)
	}
}

func TestCmdLink_ProjectNotFound(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("streaming", "")
	var out bytes.Buffer
	if err := cmdLink(kb, false, []string{"project", "nonexistent", "streaming"}, &out); err == nil {
		t.Error("expected an error for a nonexistent project")
	}
}

func TestCmdLink_ConceptNotFound(t *testing.T) {
	kb := openTestKB(t)
	kb.AddProject("proj", "")
	var out bytes.Buffer
	if err := cmdLink(kb, false, []string{"project", "proj", "nonexistent"}, &out); err == nil {
		t.Error("expected an error for a nonexistent concept")
	}
}

func TestCmdLink_ObservationToConcept(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("proj", "")
	oid, _ := kb.AddObservation(pid, "note", "body")
	kb.AddConcept("streaming", "")

	var out bytes.Buffer
	if err := cmdLink(kb, false, []string{"observation", fmt.Sprint(oid), "streaming"}, &out); err != nil {
		t.Fatalf("link observation: %v", err)
	}
}

func TestCmdLink_UnknownSubcommand(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdLink(kb, false, []string{"bogus", "a", "b"}, &out); err == nil {
		t.Error("expected an error for an unknown link subcommand")
	}
}
