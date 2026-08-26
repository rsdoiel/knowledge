package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
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

// W6's acceptance criterion: a search whose hit is a record reports it AS a
// record, with its DR-NNNN. The library carries both -- SourceType says which
// table the hit came from, Kind says what sort of record it is -- so the CLI
// shows the type, not the record's own kind, which would read as "decision"
// and be indistinguishable from a decision-kind observation.
func TestCmdSearch_LabelsRecordsAsRecords(t *testing.T) {
	kb := openTestKB(t)
	pid, err := kb.AddProject("clasm", "")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0160", ProjectID: pid, Scope: "project",
		Path: "clasm/decisions/0160-x.md", Title: "Associate the IAM profile",
		Date: "2026-08-26", Status: "accepted", Kind: "decision",
		Body: "distinctive zzqqmarker phrase",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}

	var out bytes.Buffer
	if err := cmdSearch(kb, nil, false, []string{"zzqqmarker"}, &out); err != nil {
		t.Fatalf("cmdSearch: %v", err)
	}
	// Check the bracketed type column specifically. Asserting that the line
	// merely contains "record" passes whenever the title happens to, which is
	// how the first version of this test certified the unfixed code.
	got := strings.TrimSpace(out.String())
	open, close := strings.Index(got, "["), strings.Index(got, "]")
	if open != 0 || close < 0 {
		t.Fatalf("unexpected output shape: %q", got)
	}
	if column := strings.TrimSpace(got[1:close]); column != "record" {
		t.Errorf("type column = %q, want \"record\"; the whole line is %q", column, got)
	}
	if !strings.Contains(got, "DR-0160") {
		t.Errorf("output does not carry the DR-NNNN:\n%s", got)
	}
}
