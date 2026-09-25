package main

import (
	"bytes"
	"encoding/json"
	knowledge "github.com/rsdoiel/knowledge"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `kb ingest` with two bad records out of three printed two errors and exited 0.
// A bulk command still does everything it can (the good records are ingested),
// but it now says it did not fully succeed: it exits with the class of the first
// failure, in path order so the answer does not depend on directory listing
// order, and still prints the counts (workspace DR-0003 rule 4, DR-0047 item 6).

const badRecord = "---\nid: \"0002\"\ntitle: \"\"\n---\nbroken\n"

func ingestDir(t *testing.T) (dir string, run func(jsonOut bool, extra ...string) (int, string, string)) {
	t.Helper()
	kb, root := openWorkspaceKB(t)
	dir = filepath.Join(root, "clasm", "decisions")
	return dir, func(jsonOut bool, extra ...string) (int, string, string) {
		var out, errOut bytes.Buffer
		code := dispatch(verbs, kb, nil, jsonOut, append([]string{"ingest", dir}, extra...), &out, &errOut)
		return code, out.String(), errOut.String()
	}
}

func TestCmdIngest_MalformedRecordsExit65AfterIngestingTheGoodOnes(t *testing.T) {
	dir, run := ingestDir(t)
	testRecord{ID: "0001", Project: "clasm"}.write(t, dir)
	writeRaw(t, dir, "0002-bad.md", badRecord)
	writeRaw(t, dir, "0003-none.md", "no frontmatter at all\n")

	code, out, errOut := run(false)
	if code != 65 {
		t.Errorf("exit %d, want 65 (data); stderr %q", code, errOut)
	}
	if !strings.Contains(out, "1 added") || !strings.Contains(out, "2 failed") {
		t.Errorf("stdout %q should still print the counts: 1 added, 2 failed", out)
	}
	if !strings.Contains(out, "0002-bad.md") || !strings.Contains(out, "0003-none.md") {
		t.Errorf("stdout %q should still list each failure", out)
	}
	if !strings.Contains(errOut, "2") {
		t.Errorf("stderr %q should say how many failed", errOut)
	}
	// The good record was ingested despite the failures.
	code, out, _ = run(false)
	if !strings.Contains(out, "1 skipped") {
		t.Errorf("second run stdout %q: the good record should now be skipped (already ingested)", out)
	}
}

func TestCmdIngest_AllGoodExitsZero(t *testing.T) {
	dir, run := ingestDir(t)
	testRecord{ID: "0001", Project: "clasm"}.write(t, dir)
	if code, out, errOut := run(false); code != 0 || errOut != "" {
		t.Errorf("exit %d stderr %q stdout %q, want a clean 0", code, errOut, out)
	}
}

// Warnings and unresolved references are information, not failures: the record
// was ingested. Only a file that could not be ingested is an error.
func TestCmdIngest_WarningsAndUnresolvedReferencesStayExitZero(t *testing.T) {
	dir, run := ingestDir(t)
	testRecord{ID: "0001", Project: "clasm", Trigger: "weird"}.write(t, dir) // vocabulary warning
	code, out, errOut := run(false)
	if code != 0 {
		t.Errorf("exit %d, want 0 for a warning; stderr %q stdout %q", code, errOut, out)
	}
}

func TestCmdIngest_JSONPrintsTheSummaryAndTheClass(t *testing.T) {
	dir, run := ingestDir(t)
	testRecord{ID: "0001", Project: "clasm"}.write(t, dir)
	writeRaw(t, dir, "0002-bad.md", badRecord)
	code, out, errOut := run(true)
	if code != 65 {
		t.Errorf("exit %d, want 65", code)
	}
	var sum struct{ Added, Failed int }
	if err := json.Unmarshal([]byte(out), &sum); err != nil || sum.Added != 1 || sum.Failed != 1 {
		t.Errorf("stdout = %q (%v), want the summary with added 1, failed 1", out, err)
	}
	var env struct {
		Class string
		Code  int
	}
	if err := json.Unmarshal([]byte(errOut), &env); err != nil || env.Class != "data" || env.Code != 65 {
		t.Errorf("stderr = %q (%v), want a data envelope", errOut, err)
	}
}

func TestCmdIngest_DryRunReportsFailuresToo(t *testing.T) {
	dir, run := ingestDir(t)
	writeRaw(t, dir, "0002-bad.md", badRecord)
	if code, _, _ := run(false, "--dry-run"); code != 65 {
		t.Errorf("--dry-run exit %d, want 65: a failure it would hit is still a failure", code)
	}
}

// An identity collision, two files claiming one record id under different uuids,
// is wrong content: 65.
func TestCmdIngest_IdentityCollisionExits65(t *testing.T) {
	dir, run := ingestDir(t)
	writeRaw(t, dir, "0001-a.md", workspaceRecord("0001", "First", "11111111-1111-7111-8111-111111111111"))
	writeRaw(t, dir, "0001-b.md", workspaceRecord("0001", "Second", "22222222-2222-7222-8222-222222222222"))
	code, out, _ := run(false)
	if code != 65 {
		t.Errorf("exit %d, want 65; stdout %q", code, out)
	}
	if !strings.Contains(out, "1 failed") {
		t.Errorf("stdout %q should count one failure", out)
	}
}

// The class is the first failure's, in path order: an unreadable 0001 (permission,
// 77) comes before a malformed 0002 (data, 65), so the exit is 77, and both are
// still reported.
func TestCmdIngest_ExitsWithTheFirstFailuresClassInPathOrder(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: permissions are not enforced")
	}
	dir, run := ingestDir(t)
	locked := writeRaw(t, dir, "0001-locked.md", "x")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o644) })
	writeRaw(t, dir, "0002-bad.md", badRecord)
	testRecord{ID: "0003", Project: "clasm"}.write(t, dir)

	code, out, _ := run(false)
	if code != 77 {
		t.Errorf("exit %d, want 77: the first failure in path order is the unreadable file", code)
	}
	if !strings.Contains(out, "2 failed") || !strings.Contains(out, "1 added") {
		t.Errorf("stdout %q should report 1 added, 2 failed", out)
	}
}

// The merge identity collision is wrong content too (65), and nothing is written.
func TestMerge_CollisionIsExit65(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.db")
	buildTestDB(t, aPath, func(kb *knowledge.KnowledgeBase) { kb.AddProject("shared-name", "a") })
	bPath := filepath.Join(dir, "b.db")
	buildTestDB(t, bPath, func(kb *knowledge.KnowledgeBase) { kb.AddProject("shared-name", "b") })
	var out bytes.Buffer
	err := cmdMerge(nil, nil, false, []string{"-a", aPath, "-b", bPath, "-out", filepath.Join(dir, "m.db")}, &out)
	if err == nil {
		t.Fatal("want a collision error")
	}
	if got := exitCodeFor(err); got != classData {
		t.Errorf("class = %v, want data (65)", got)
	}
}
