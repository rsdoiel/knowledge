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
	if err := cmdProject(kb, false, []string{"add", "alpha", "first", "project"}, &out); err != nil {
		t.Fatalf("project add: %v", err)
	}
	if !strings.Contains(out.String(), "alpha") {
		t.Errorf("add output = %q, want it to mention alpha", out.String())
	}

	out.Reset()
	if err := cmdProject(kb, false, []string{"list"}, &out); err != nil {
		t.Fatalf("project list: %v", err)
	}
	if !strings.Contains(out.String(), "alpha") {
		t.Errorf("list output = %q, want it to mention alpha", out.String())
	}
}

func TestCmdProject_AddJSON(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, true, []string{"add", "beta"}, &out); err != nil {
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
	err := cmdProject(kb, false, []string{"show", "nonexistent"}, &out)
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
	if err := cmdProject(kb, false, []string{"show", "gamma"}, &out); err != nil {
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
	if err := cmdProject(kb, false, []string{"concepts", "delta"}, &out); err != nil {
		t.Fatalf("project concepts: %v", err)
	}
	if !strings.Contains(out.String(), "streaming") {
		t.Errorf("concepts output = %q, want it to mention streaming", out.String())
	}
}

func TestCmdProject_UnknownSubcommand(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, false, []string{"bogus"}, &out); err == nil {
		t.Error("expected an error for an unknown project subcommand")
	}
}
