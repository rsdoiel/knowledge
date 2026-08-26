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

func TestCmdProject_SetDescription(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("theta", "names DESIGN_DECIDE_PLAN.md"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{
		"set-description", "theta", "names", "DESIGN_REVIEW_PLAN_IMPLEMENT.md",
	}, &out); err != nil {
		t.Fatalf("project set-description: %v", err)
	}
	p, err := kb.ProjectByName("theta")
	if err != nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	// Trailing words join with a space, as project add already does.
	if p.Description != "names DESIGN_REVIEW_PLAN_IMPLEMENT.md" {
		t.Errorf("Description = %q, want the joined corrected text", p.Description)
	}
}

func TestCmdProject_SetDescriptionJSON(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("iota", "old"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, true, []string{"set-description", "iota", "new"}, &out); err != nil {
		t.Fatalf("project set-description: %v", err)
	}
	assertValidJSON(t, out.Bytes())
	var got struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v (%q)", err, out.String())
	}
	if got.Name != "iota" || got.Description != "new" {
		t.Errorf("got = %+v, want Name=iota Description=new", got)
	}
}

// `set-description NAME ""` is how a description gets cleared; bare
// `set-description NAME` is a usage error, so the clear stays deliberate.
func TestCmdProject_SetDescriptionEmptyString(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("kappa", "something"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"set-description", "kappa", ""}, &out); err != nil {
		t.Fatalf("project set-description: %v", err)
	}
	p, err := kb.ProjectByName("kappa")
	if err != nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	if p.Description != "" {
		t.Errorf("Description = %q, want it cleared", p.Description)
	}
}

func TestCmdProject_SetDescriptionMissingArgs(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("lambda", "keep me"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"set-description", "lambda"}, &out); err == nil {
		t.Error("expected an error for a missing DESCRIPTION argument")
	}
	p, err := kb.ProjectByName("lambda")
	if err != nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	if p.Description != "keep me" {
		t.Errorf("Description = %q, want the usage error to leave it alone", p.Description)
	}
}

func TestCmdProject_SetDescriptionNotFound(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"set-description", "nonexistent", "x"}, &out); err == nil {
		t.Error("expected an error for a nonexistent project")
	}
}

// Every project subcommand has to appear in the verb's own usage line, or it
// is undiscoverable from the error you get by running the verb bare.
func TestCmdProject_UsageListsEverySubcommand(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	err := cmdProject(kb, nil, false, nil, &out)
	if err == nil {
		t.Fatal("expected a usage error for a bare project verb")
	}
	for _, sub := range []string{"add", "list", "show", "concepts", "set-status", "set-description"} {
		if !strings.Contains(err.Error(), sub) {
			t.Errorf("usage %q does not mention subcommand %q", err.Error(), sub)
		}
	}
}

// kb-project(1) is generated from ProjectHelpText, so a subcommand missing
// from it never reaches the man page.
func TestProjectHelpText_DocumentsSetDescription(t *testing.T) {
	if !strings.Contains(ProjectHelpText, "set-description") {
		t.Error("ProjectHelpText does not document set-description")
	}
}

func TestCmdProject_UnknownSubcommand(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"bogus"}, &out); err == nil {
		t.Error("expected an error for an unknown project subcommand")
	}
}
