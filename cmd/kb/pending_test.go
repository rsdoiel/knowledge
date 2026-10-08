package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// pendingFixture holds clasm 0001 and 0003 proposed (0002 accepted), cold 0001
// proposed, and workspace 0001 proposed (0002 accepted), dated so the order is
// not the order of insertion.
func pendingFixture(t *testing.T) (*knowledge.KnowledgeBase, string) {
	t.Helper()
	kb, root := namedWorkspaceKB(t, "Laboratory")
	add := func(project, dir string, r testRecord) {
		r.Project = project
		r.write(t, filepath.Join(root, dir))
	}
	cl := filepath.Join("agents", "projects", "clasm", "decisions")
	co := filepath.Join("agents", "projects", "cold", "decisions")
	ws := filepath.Join("agents", "decisions")
	add("clasm", cl, testRecord{ID: "0001", Title: "clasm one", Status: "proposed", Date: "2026-09-03", Kind: "decision"})
	add("clasm", cl, testRecord{ID: "0002", Title: "clasm two", Status: "accepted", Date: "2026-09-01"})
	add("clasm", cl, testRecord{ID: "0003", Title: "clasm three", Status: "proposed", Date: "2026-09-05", Kind: "correction"})
	add("cold", co, testRecord{ID: "0001", Title: "cold one", Status: "proposed", Date: "2026-09-02", Kind: "decision"})
	add("", ws, testRecord{ID: "0001", Title: "workspace one", Status: "proposed", Date: "2026-09-04", Kind: "decision"})
	add("", ws, testRecord{ID: "0002", Title: "workspace two", Status: "accepted", Date: "2026-09-06"})
	for _, d := range []string{cl, co, ws} {
		runIngest(t, kb, filepath.Join(root, d))
	}
	for _, d := range []string{"clasm/cmd", "tmp/scratch"} {
		mkdirAll(t, root, d)
	}
	t.Setenv("KB_PROJECT", "")
	return kb, root
}

func pendingTitles(t *testing.T, kb *knowledge.KnowledgeBase, args ...string) []string {
	t.Helper()
	var entries []recordListEntry
	runRecordJSON(t, kb, &entries, append([]string{"pending"}, args...)...)
	var out []string
	for _, e := range entries {
		out = append(out, e.Title)
	}
	return out
}

func TestRecordPending_ListsOnlyProposedAcrossTheWorkspace(t *testing.T) {
	kb, root := pendingFixture(t)
	t.Chdir(root)
	got := pendingTitles(t, kb)
	// Oldest first by date, whatever the scope.
	want := []string{"cold one", "clasm one", "workspace one", "clasm three"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("pending = %v, want %v", got, want)
	}
}

func TestRecordPending_HumanOutputUsesTheListColumns(t *testing.T) {
	kb, root := pendingFixture(t)
	t.Chdir(root)
	lines := listLines(t, runRecord(t, kb, "pending"))
	if len(lines) != 4 {
		t.Fatalf("pending = %q, want 4 lines", lines)
	}
	if first := strings.Fields(lines[0])[0]; first != "cold/DR-0001" {
		t.Errorf("first line %q, want it to start with cold/DR-0001", lines[0])
	}
	for _, l := range lines {
		if !strings.Contains(l, "proposed") {
			t.Errorf("line %q is not a proposed record", l)
		}
	}
}

func TestRecordPending_TakesTheSameScopesAsList(t *testing.T) {
	kb, root := pendingFixture(t)
	t.Chdir(root)
	for _, c := range []struct {
		args []string
		want []string
	}{
		{[]string{"clasm"}, []string{"clasm one", "clasm three"}},
		{[]string{"cold", "workspace"}, []string{"cold one", "workspace one"}},
		{[]string{"laboratory"}, []string{"workspace one"}},
	} {
		if got := pendingTitles(t, kb, c.args...); !sameSet(got, c.want...) {
			t.Errorf("pending %v = %v, want %v", c.args, got, c.want)
		}
	}
	t.Chdir(filepath.Join(root, "clasm", "cmd"))
	if got := pendingTitles(t, kb); !sameSet(got, "clasm one", "clasm three") {
		t.Errorf("pending from clasm/ = %v, want clasm's", got)
	}
	if got := pendingTitles(t, kb, "--all"); len(got) != 4 {
		t.Errorf("pending --all = %v, want all four", got)
	}
}

func TestRecordPending_OtherFiltersStillApply(t *testing.T) {
	kb, root := pendingFixture(t)
	t.Chdir(root)
	if got := pendingTitles(t, kb, "--kind", "correction"); !sameSet(got, "clasm three") {
		t.Errorf("pending --kind correction = %v, want [clasm three]", got)
	}
	if got := pendingTitles(t, kb, "--since", "2026-09-04"); !sameSet(got, "workspace one", "clasm three") {
		t.Errorf("pending --since 2026-09-04 = %v", got)
	}
}

func TestRecordPending_NothingPendingIsSuccess(t *testing.T) {
	kb, root := pendingFixture(t)
	t.Chdir(root)
	// workspace two is accepted, and so is clasm two; nothing else is in scope here.
	var entries []recordListEntry
	runRecordJSON(t, kb, &entries, "pending", "workspace", "--kind", "correction")
	if entries == nil || len(entries) != 0 {
		t.Errorf("json = %#v, want an empty array", entries)
	}
	out := runRecord(t, kb, "pending", "workspace", "--kind", "correction")
	if !strings.Contains(out, "no pending records") {
		t.Errorf("output = %q, want it to say nothing is pending", out)
	}
}

func TestRecordPending_StatusFlagIsAUsageError(t *testing.T) {
	kb, root := pendingFixture(t)
	t.Chdir(root)
	var out bytes.Buffer
	err := cmdRecord(kb, nil, false, []string{"pending", "--status", "accepted"}, &out)
	if err == nil || !isUsageError(err) {
		t.Errorf("error = %v, want a usage error: pending is proposed by definition", err)
	}
}

func TestRecordPending_UnknownScopeIsNotFound(t *testing.T) {
	kb, root := pendingFixture(t)
	t.Chdir(root)
	var out bytes.Buffer
	err := cmdRecord(kb, nil, false, []string{"pending", "nosuch"}, &out)
	if err == nil || exitCodeFor(err).Code != 1 {
		t.Errorf("error = %v, want not found (exit 1)", err)
	}
}
