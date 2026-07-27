package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCmdSearch_FindsObservation(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("proj", "")
	if _, err := kb.AddObservation(pid, "note", "a very specific searchable phrase"); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}
	var out bytes.Buffer
	if err := cmdSearch(kb, nil, false, []string{"searchable"}, &out); err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(out.String(), "searchable") {
		t.Errorf("search output = %q, want it to contain a match", out.String())
	}
}

func TestCmdSearch_RequiresTerm(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdSearch(kb, nil, false, nil, &out); err == nil {
		t.Error("expected an error when no search term is given")
	}
}

func TestCmdSummary_Empty(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdSummary(kb, nil, false, nil, &out); err != nil {
		t.Fatalf("summary: %v", err)
	}
	if out.Len() == 0 {
		t.Error("expected non-empty summary output")
	}
}

func TestCmdSummary_JSON(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdSummary(kb, nil, true, nil, &out); err != nil {
		t.Fatalf("summary: %v", err)
	}
	var got struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v (%q)", err, out.String())
	}
}

func TestCmdFormat_AllProjects(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("proj", "a project")
	kb.AddObservation(pid, "note", "some body")
	var out bytes.Buffer
	if err := cmdFormat(kb, nil, false, nil, &out); err != nil {
		t.Fatalf("format: %v", err)
	}
	if !strings.Contains(out.String(), "proj") {
		t.Errorf("format output = %q, want it to mention the project", out.String())
	}
}

func TestCmdFormat_ProjectNotFound(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdFormat(kb, nil, false, []string{"--project", "nonexistent"}, &out); err == nil {
		t.Error("expected an error for a nonexistent project")
	}
}
