package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// Characterisation of `kb merge` output (DR-0053 B1): the detection half of
// runMerge moves into the module, and merge's behaviour and output must not
// change. The golden files were captured from the code before the move.
// UPDATE_GOLDEN=1 rewrites them.

var uuidPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func goldenMergeFixture(t *testing.T) (dir, a, b string) {
	t.Helper()
	dir = t.TempDir()
	a, b = filepath.Join(dir, "a.db"), filepath.Join(dir, "b.db")
	seed := func(desc, obs, checksum string) func(kb *knowledge.KnowledgeBase) {
		return func(kb *knowledge.KnowledgeBase) {
			pid, _ := kb.AddProject("shared", desc)
			kb.AddObservation(pid, "note", obs)
			kb.AddRecord(knowledge.Record{RecordID: "0001", Scope: "workspace", Workspace: "ws", Path: "d/0001.md",
				Title: "t", Date: "2026-09-01", Status: "proposed", Kind: "decision", Body: "b", Checksum: checksum})
		}
	}
	buildTestDB(t, a, seed("a's", "on a", "ck-a"))
	buildTestDB(t, b, seed("b's", "on b", "ck-b"))
	return
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s differs:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func normaliseMerge(dir, s string) string {
	s = strings.ReplaceAll(s, dir, "DIR")
	return uuidPattern.ReplaceAllString(s, "UUID")
}

func TestMergeGolden_CollisionAbort(t *testing.T) {
	dir, a, b := goldenMergeFixture(t)
	var out bytes.Buffer
	err := cmdMerge(nil, nil, false, []string{"-a", a, "-b", b, "-out", filepath.Join(dir, "m.db")}, &out)
	if err == nil {
		t.Fatal("want a collision abort")
	}
	checkGolden(t, "merge_abort.golden", normaliseMerge(dir, "out:"+out.String()+"\nerr:"+err.Error()+"\n"))
	if _, statErr := os.Stat(filepath.Join(dir, "m.db")); statErr == nil {
		t.Error("an aborted merge left an output file")
	}
}

func TestMergeGolden_ForceText(t *testing.T) {
	dir, a, b := goldenMergeFixture(t)
	var out bytes.Buffer
	if err := cmdMerge(nil, nil, false, []string{"-a", a, "-b", b, "-out", filepath.Join(dir, "m.db"), "-force"}, &out); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "merge_force.golden", normaliseMerge(dir, out.String()))
}

func TestMergeGolden_ForceJSON(t *testing.T) {
	dir, a, b := goldenMergeFixture(t)
	var out bytes.Buffer
	if err := cmdMerge(nil, nil, true, []string{"-a", a, "-b", b, "-out", filepath.Join(dir, "m.db"), "-force"}, &out); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "merge_force_json.golden", normaliseMerge(dir, out.String()))
}
