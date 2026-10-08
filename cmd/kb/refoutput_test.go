package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// DR-0057: every listing and every confirmation names a record by its
// qualified reference, so "DR-0001" is never left to be guessed at.

func listLines(t *testing.T, out string) []string {
	t.Helper()
	var lines []string
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func TestRecordList_HumanLinesStartWithTheQualifiedRef(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(root)
	lines := listLines(t, runRecord(t, kb, "list"))
	if len(lines) != 4 {
		t.Fatalf("list = %q, want 4 lines", lines)
	}
	want := map[string]bool{"harvey/DR-0001": false, "harvey/DR-0002": false, "clasm/DR-0001": false, "workspace/DR-0001": false}
	for _, l := range lines {
		first := strings.Fields(l)[0]
		if _, ok := want[first]; !ok {
			t.Errorf("line %q starts with %q, want a qualified ref", l, first)
		}
		want[first] = true
	}
	for ref, seen := range want {
		if !seen {
			t.Errorf("no line for %s in %q", ref, lines)
		}
	}
}

func TestRecordList_ColumnsAlignAcrossScopes(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(root)
	lines := listLines(t, runRecord(t, kb, "list"))
	col := -1
	for _, l := range lines {
		i := strings.Index(l, "2026-")
		if i < 0 {
			t.Fatalf("no date in %q", l)
		}
		if col == -1 {
			col = i
		} else if i != col {
			t.Errorf("the date column moves between lines (%d vs %d):\n%s", col, i, strings.Join(lines, "\n"))
		}
	}
}

func TestRecordList_JSONCarriesRef(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(root)
	var entries []struct {
		Ref      string `json:"ref"`
		RecordID string `json:"record_id"`
		Project  string `json:"project"`
		Scope    string `json:"scope"`
	}
	runRecordJSON(t, kb, &entries, "list")
	got := map[string]string{}
	for _, e := range entries {
		got[e.Ref] = e.Scope
		if e.RecordID == "" {
			t.Errorf("record_id lost for %s", e.Ref)
		}
	}
	for ref, scope := range map[string]string{
		"harvey/DR-0001": "project", "clasm/DR-0001": "project", "workspace/DR-0001": "workspace",
	} {
		if got[ref] != scope {
			t.Errorf("refs = %v, want %s with scope %s", got, ref, scope)
		}
	}
}

func TestRecordShow_NamesTheQualifiedRef(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(root)
	out := runRecord(t, kb, "show", "clasm/0001")
	if first := listLines(t, out)[0]; first != "clasm/DR-0001" {
		t.Errorf("first line = %q, want clasm/DR-0001", first)
	}
	var detail struct {
		Ref string `json:"ref"`
	}
	runRecordJSON(t, kb, &detail, "show", "workspace/0001")
	if detail.Ref != "workspace/DR-0001" {
		t.Errorf("show --json ref = %q, want workspace/DR-0001", detail.Ref)
	}
}

func TestRecordShow_RelationsAreQualified(t *testing.T) {
	kb, root := namedWorkspaceKB(t, "Laboratory")
	dir := filepath.Join(root, "agents", "projects", "clasm", "decisions")
	testRecord{ID: "0148", Project: "clasm", Title: "old", Status: "accepted"}.write(t, dir)
	testRecord{ID: "0149", Project: "clasm", Title: "new", Status: "proposed", Supersedes: []string{"0148"}}.write(t, dir)
	runIngest(t, kb, dir)
	t.Chdir(root)
	var detail struct {
		Supersedes   []string `json:"supersedes"`
		SupersededBy []string `json:"superseded_by"`
	}
	runRecordJSON(t, kb, &detail, "show", "clasm/0149")
	if len(detail.Supersedes) != 1 || detail.Supersedes[0] != "clasm/DR-0148" {
		t.Errorf("supersedes = %v, want [clasm/DR-0148]", detail.Supersedes)
	}
	runRecordJSON(t, kb, &detail, "show", "clasm/0148")
	if len(detail.SupersededBy) != 1 || detail.SupersededBy[0] != "clasm/DR-0149" {
		t.Errorf("superseded_by = %v, want [clasm/DR-0149]", detail.SupersededBy)
	}
}

func TestRecordSetStatus_ConfirmsWithTheQualifiedRef(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm", testRecord{ID: "0001", Status: "proposed"})
	t.Chdir(root)
	out := runRecord(t, kb, "set-status", "clasm/0001", "accepted")
	if !strings.HasPrefix(out, "clasm/DR-0001 status set to accepted") {
		t.Errorf("output = %q, want it to begin with the qualified ref", out)
	}
	var result map[string]any
	runRecordJSON(t, kb, &result, "set-status", "clasm/0001", "rejected")
	if result["ref"] != "clasm/DR-0001" || result["record_id"] != "0001" {
		t.Errorf("json = %v, want ref clasm/DR-0001 and record_id kept", result)
	}
}

func TestRecordSupersede_ConfirmsWithQualifiedRefs(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0148", Status: "accepted"}, testRecord{ID: "0149", Status: "proposed"})
	t.Chdir(root)
	out := runRecord(t, kb, "supersede", "clasm/0149", "clasm/0148")
	want := "clasm/DR-0149 supersedes clasm/DR-0148; clasm/DR-0148 is now superseded"
	if !strings.Contains(out, want) {
		t.Errorf("output = %q, want %q", out, want)
	}
	var result map[string]any
	runRecordJSON(t, kb, &result, "supersede", "clasm/0149", "clasm/0148")
	if result["new_ref"] != "clasm/DR-0149" || result["old_ref"] != "clasm/DR-0148" {
		t.Errorf("json = %v, want new_ref and old_ref qualified", result)
	}
}

func TestRecordDelete_ConfirmsWithTheQualifiedRef(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm", testRecord{ID: "0001", Status: "accepted"})
	if err := os.Remove(filepath.Join(root, "clasm", "decisions", "0001-fixture.md")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	out := runRecord(t, kb, "delete", "clasm/0001", "--dry-run")
	if !strings.Contains(out, "the row for clasm/DR-0001") {
		t.Errorf("output = %q, want the qualified ref", out)
	}
	var result struct {
		Ref    string `json:"ref"`
		Record string `json:"record"`
	}
	runRecordJSON(t, kb, &result, "delete", "clasm/0001", "--dry-run")
	if result.Ref != "clasm/DR-0001" || result.Record != "0001" {
		t.Errorf("json = %+v, want ref clasm/DR-0001 and record kept", result)
	}
}

func TestRecordNew_ReportsTheQualifiedRef(t *testing.T) {
	kb, root := namedWorkspaceKB(t, "Laboratory")
	t.Chdir(root)
	out := runRecord(t, kb, "new", "--project", "clasm", "--title", "One", "--trigger", "design")
	if !regexp.MustCompile(`^clasm/DR-0001 written to `).MatchString(out) {
		t.Errorf("project output = %q, want it to begin with clasm/DR-0001", out)
	}
	out = runRecord(t, kb, "new", "--workspace", "--title", "Two", "--trigger", "design")
	if !strings.HasPrefix(out, "workspace/DR-0001 written to ") {
		t.Errorf("workspace output = %q, want it to begin with workspace/DR-0001", out)
	}
	var result map[string]any
	runRecordJSON(t, kb, &result, "new", "--project", "clasm", "--title", "Three", "--trigger", "design")
	if result["ref"] != "clasm/DR-0002" {
		t.Errorf("json = %v, want ref clasm/DR-0002", result)
	}
	if _, err := json.Marshal(result); err != nil {
		t.Fatal(err)
	}
}
