package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCmdProject_AddThenList(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"add", "alpha", "first", "project"}, &out); err != nil {
		t.Fatalf("project add: %v", err)
	}
	if !strings.Contains(out.String(), "alpha") {
		t.Errorf("add output = %q, want it to mention alpha", out.String())
	}

	out.Reset()
	if err := cmdProject(kb, nil, false, []string{"list"}, &out); err != nil {
		t.Fatalf("project list: %v", err)
	}
	if !strings.Contains(out.String(), "alpha") {
		t.Errorf("list output = %q, want it to mention alpha", out.String())
	}
}

func TestCmdProject_AddJSON(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, true, []string{"add", "beta"}, &out); err != nil {
		t.Fatalf("project add: %v", err)
	}
	var got struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v (%q)", err, out.String())
	}
	if got.Name != "beta" || got.ID == 0 {
		t.Errorf("got = %+v, want Name=beta and non-zero ID", got)
	}
}

func TestCmdProject_ShowNotFound(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	err := cmdProject(kb, nil, false, []string{"show", "nonexistent"}, &out)
	if err == nil {
		t.Error("expected an error for a nonexistent project")
	}
}

func TestCmdProject_ShowFound(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("gamma", "a description"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"show", "gamma"}, &out); err != nil {
		t.Fatalf("project show: %v", err)
	}
	if !strings.Contains(out.String(), "gamma") || !strings.Contains(out.String(), "a description") {
		t.Errorf("show output = %q, want it to mention name and description", out.String())
	}
}

func TestCmdProject_Concepts(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("delta", "")
	cid, _ := kb.AddConcept("streaming", "")
	if err := kb.LinkProjectConcept(pid, cid); err != nil {
		t.Fatalf("LinkProjectConcept: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"concepts", "delta"}, &out); err != nil {
		t.Fatalf("project concepts: %v", err)
	}
	if !strings.Contains(out.String(), "streaming") {
		t.Errorf("concepts output = %q, want it to mention streaming", out.String())
	}
}

func TestCmdProject_AddWithStatus(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"add", "--status", "concept", "epsilon", "desc"}, &out); err != nil {
		t.Fatalf("project add --status: %v", err)
	}
	p, err := kb.ProjectByName("epsilon")
	if err != nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	if p.Status != "concept" {
		t.Errorf("Status = %q, want %q", p.Status, "concept")
	}
}

func TestCmdProject_AddInvalidStatus(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"add", "--status", "bogus", "zeta", "desc"}, &out); err == nil {
		t.Error("expected an error for an invalid status")
	}
}

func TestCmdProject_SetStatus(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("eta", ""); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"set-status", "eta", "paused"}, &out); err != nil {
		t.Fatalf("project set-status: %v", err)
	}
	p, err := kb.ProjectByName("eta")
	if err != nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	if p.Status != "paused" {
		t.Errorf("Status = %q, want %q", p.Status, "paused")
	}
}

func TestCmdProject_SetStatusNotFound(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"set-status", "nonexistent", "active"}, &out); err == nil {
		t.Error("expected an error for a nonexistent project")
	}
}

func TestCmdProject_SetStatusMissingArgs(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"set-status", "eta"}, &out); err == nil {
		t.Error("expected an error for missing STATUS argument")
	}
}

func TestCmdProject_UnknownSubcommand(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"bogus"}, &out); err == nil {
		t.Error("expected an error for an unknown project subcommand")
	}
}
