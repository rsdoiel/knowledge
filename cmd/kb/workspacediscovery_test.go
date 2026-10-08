package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// workspaceWithDB makes a workspace holding a database with one project,
// "harvey", and returns its root.
func workspaceWithDB(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	kb, err := knowledge.Open(knowledge.DefaultPath(root))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := kb.AddProject("harvey", "a project"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	kb.Close()
	return root
}

func mkdirAll(t *testing.T, parts ...string) string {
	t.Helper()
	dir := filepath.Join(parts...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return dir
}

func samePath(t *testing.T, a, b string) bool {
	t.Helper()
	ra, _ := filepath.EvalSymlinks(a)
	rb, _ := filepath.EvalSymlinks(b)
	return ra == rb
}

func TestResolveDBPath_WalksUpToTheWorkspace(t *testing.T) {
	root := workspaceWithDB(t)
	t.Chdir(mkdirAll(t, root, "harvey", "cmd"))
	got, err := resolveDBPath("")
	if err != nil {
		t.Fatalf("resolveDBPath: %v", err)
	}
	if want := filepath.Join(root, "agents", "knowledge.db"); !samePath(t, got, want) {
		t.Errorf("resolveDBPath = %q, want %q", got, want)
	}
}

func TestResolveDBPath_JSONLOnlyPointsAtTheDatabaseToBuild(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, root, "agents")
	if err := os.WriteFile(filepath.Join(root, "agents", "knowledge.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(mkdirAll(t, root, "sub"))
	got, err := resolveDBPath("")
	if err != nil {
		t.Fatalf("resolveDBPath: %v", err)
	}
	if want := filepath.Join(root, "agents", "knowledge.db"); !samePath(t, filepath.Dir(got), filepath.Dir(want)) || filepath.Base(got) != "knowledge.db" {
		t.Errorf("resolveDBPath = %q, want %q", got, want)
	}
}

func TestResolveDBPath_ExplicitPathIgnoresDiscovery(t *testing.T) {
	root := workspaceWithDB(t)
	t.Chdir(mkdirAll(t, root, "sub"))
	got, err := resolveDBPath("/elsewhere/x.db")
	if err != nil || got != "/elsewhere/x.db" {
		t.Errorf("resolveDBPath(abs) = %q, %v; want it unchanged", got, err)
	}
}

func TestMainRun_VerbWorksFromANestedDirectory(t *testing.T) {
	root := workspaceWithDB(t)
	t.Chdir(mkdirAll(t, root, "harvey", "cmd"))
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"project", "list"}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "harvey") {
		t.Errorf("project list = %q, want it to show harvey", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, "harvey", "cmd", "agents")); err == nil {
		t.Errorf("a nested agents directory was created under the working directory")
	}
}

func TestMainRun_JSONLOnlyNamesTheImportRemedy(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, root, "agents")
	if err := os.WriteFile(filepath.Join(root, "agents", "knowledge.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(mkdirAll(t, root, "sub"))
	var out, errOut bytes.Buffer
	code := mainRun([]string{"project", "list"}, &out, &errOut)
	if code != 66 {
		t.Fatalf("exit code = %d, want 66; stderr=%s", code, errOut.String())
	}
	msg := errOut.String()
	if !strings.Contains(msg, "kb import -in") || !strings.Contains(msg, "knowledge.jsonl") {
		t.Errorf("stderr = %q, want it to name kb import -in agents/knowledge.jsonl", msg)
	}
	if strings.Contains(msg, "kb init") {
		t.Errorf("stderr = %q, must not suggest kb init over an existing history", msg)
	}
	if _, err := os.Stat(filepath.Join(root, "agents", "knowledge.db")); err == nil {
		t.Errorf("the guard created the database")
	}
}

func TestMainRun_ImportFromANestedDirectoryBuildsTheWorkspaceDatabase(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, root, "agents")
	seed := filepath.Join(root, "agents", "knowledge.jsonl")
	if err := os.WriteFile(seed, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	sub := mkdirAll(t, root, "sub")
	t.Chdir(sub)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"import", "-in", seed}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(root, "agents", "knowledge.db")); err != nil {
		t.Errorf("expected the workspace database to be built: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sub, "agents")); err == nil {
		t.Errorf("import created agents/ under the working directory")
	}
}

func TestMainRun_KBDBEnvironmentOverridesDiscovery(t *testing.T) {
	root := workspaceWithDB(t)
	other := t.TempDir()
	kb, err := knowledge.Open(filepath.Join(other, "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kb.AddProject("elsewhere", "d"); err != nil {
		t.Fatal(err)
	}
	kb.Close()
	t.Chdir(mkdirAll(t, root, "sub"))
	t.Setenv("KB_DB", filepath.Join(other, "x.db"))
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"project", "list"}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "elsewhere") || strings.Contains(out.String(), "harvey") {
		t.Errorf("project list = %q, want only the KB_DB database", out.String())
	}
}

func TestMainRun_DBFlagBeatsKBDBEnvironment(t *testing.T) {
	root := workspaceWithDB(t)
	t.Setenv("KB_DB", filepath.Join(t.TempDir(), "nope.db"))
	var out, errOut bytes.Buffer
	args := []string{"--db", filepath.Join(root, "agents", "knowledge.db"), "project", "list"}
	if code := mainRun(args, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "harvey") {
		t.Errorf("project list = %q, want the --db database", out.String())
	}
}

func TestMainRun_KBDBEnvironmentDoesNotTripMergeIndexInitCompletion(t *testing.T) {
	t.Setenv("KB_DB", filepath.Join(t.TempDir(), "x.db"))
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"completion", "bash"}, &out, &errOut); code != 0 {
		t.Errorf("completion with KB_DB set: exit %d; stderr=%s", code, errOut.String())
	}
}

// DR-0058: a workspace found above the working directory is announced on
// standard error, so a verb run in a scratch directory that happens to sit
// inside a real workspace cannot write to it unnoticed.

func TestMainRun_NotesAWorkspaceFoundAboveTheWorkingDirectory(t *testing.T) {
	root := workspaceWithDB(t)
	t.Chdir(mkdirAll(t, root, "harvey", "cmd"))
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"project", "list"}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "using the workspace at") {
		t.Errorf("stderr = %q, want a note naming the workspace", errOut.String())
	}
	resolved, _ := filepath.EvalSymlinks(root)
	if !strings.Contains(errOut.String(), root) && !strings.Contains(errOut.String(), resolved) {
		t.Errorf("stderr = %q, want it to name %s", errOut.String(), root)
	}
	if strings.Contains(out.String(), "using the workspace") {
		t.Errorf("the note leaked to standard output: %q", out.String())
	}
}

func TestMainRun_NoNoteAtTheWorkspaceRoot(t *testing.T) {
	root := workspaceWithDB(t)
	t.Chdir(root)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"project", "list"}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, errOut.String())
	}
	if strings.Contains(errOut.String(), "using the workspace") {
		t.Errorf("stderr = %q, want no note when the workspace is the working directory", errOut.String())
	}
}

func TestMainRun_NoNoteWhenTheLocationIsExplicit(t *testing.T) {
	root := workspaceWithDB(t)
	t.Chdir(mkdirAll(t, root, "sub"))
	db := filepath.Join(root, "agents", "knowledge.db")

	var out, errOut bytes.Buffer
	if code := mainRun([]string{"--db", db, "project", "list"}, &out, &errOut); code != 0 {
		t.Fatalf("--db: exit code = %d; stderr=%s", code, errOut.String())
	}
	if strings.Contains(errOut.String(), "using the workspace") {
		t.Errorf("--db: stderr = %q, want no note", errOut.String())
	}

	t.Setenv("KB_DB", db)
	out.Reset()
	errOut.Reset()
	if code := mainRun([]string{"project", "list"}, &out, &errOut); code != 0 {
		t.Fatalf("KB_DB: exit code = %d; stderr=%s", code, errOut.String())
	}
	if strings.Contains(errOut.String(), "using the workspace") {
		t.Errorf("KB_DB: stderr = %q, want no note", errOut.String())
	}
}

func TestMainRun_NoteLeavesJSONOutputParseable(t *testing.T) {
	root := workspaceWithDB(t)
	t.Chdir(mkdirAll(t, root, "sub"))
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"--json", "project", "list"}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, errOut.String())
	}
	assertValidJSON(t, out.Bytes())
	if !strings.Contains(errOut.String(), "using the workspace at") {
		t.Errorf("stderr = %q, want the note under --json too", errOut.String())
	}
}

func TestMainRun_NoNoteWhenNoWorkspaceIsFound(t *testing.T) {
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"project", "list"}, &out, &errOut); code != 66 {
		t.Fatalf("exit code = %d, want 66", code)
	}
	if strings.Contains(errOut.String(), "using the workspace") {
		t.Errorf("stderr = %q, want only the error", errOut.String())
	}
}

func TestMainRun_CeilingKeepsAScratchDirectoryOutOfTheWorkspace(t *testing.T) {
	root := workspaceWithDB(t)
	scratch := mkdirAll(t, root, "tmp", "scratch")
	t.Chdir(scratch)
	t.Setenv("KB_CEILING_DIRECTORIES", scratch)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"project", "list"}, &out, &errOut); code != 66 {
		t.Fatalf("exit code = %d, want 66 (no workspace reachable); stderr=%s", code, errOut.String())
	}
	if strings.Contains(out.String(), "harvey") {
		t.Errorf("the scratch directory reached the real workspace: %q", out.String())
	}
}

func TestResolveDBPath_CeilingFallsBackToTheWorkingDirectory(t *testing.T) {
	root := workspaceWithDB(t)
	scratch := mkdirAll(t, root, "tmp", "scratch")
	t.Chdir(scratch)
	t.Setenv("KB_CEILING_DIRECTORIES", scratch)
	got, err := resolveDBPath("")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(scratch, "agents", "knowledge.db"); !samePath(t, filepath.Dir(got), filepath.Dir(want)) {
		t.Errorf("resolveDBPath = %q, want %q", got, want)
	}
}

// KB_QUIET silences advisory notes on standard error. It never silences an
// error, and it is for tests and scripts that run in a nested directory on
// purpose.

func TestMainRun_KBQuietSilencesTheWorkspaceNote(t *testing.T) {
	root := workspaceWithDB(t)
	t.Chdir(mkdirAll(t, root, "sub"))
	for _, value := range []string{"1", "true", "yes"} {
		t.Setenv("KB_QUIET", value)
		var out, errOut bytes.Buffer
		if code := mainRun([]string{"project", "list"}, &out, &errOut); code != 0 {
			t.Fatalf("KB_QUIET=%s: exit code = %d; stderr=%s", value, code, errOut.String())
		}
		if errOut.Len() != 0 {
			t.Errorf("KB_QUIET=%s: stderr = %q, want it empty", value, errOut.String())
		}
	}
}

func TestMainRun_KBQuietOffValuesKeepTheNote(t *testing.T) {
	root := workspaceWithDB(t)
	t.Chdir(mkdirAll(t, root, "sub"))
	for _, value := range []string{"", "0", "false"} {
		t.Setenv("KB_QUIET", value)
		var out, errOut bytes.Buffer
		if code := mainRun([]string{"project", "list"}, &out, &errOut); code != 0 {
			t.Fatalf("KB_QUIET=%q: exit code = %d", value, code)
		}
		if !strings.Contains(errOut.String(), "using the workspace at") {
			t.Errorf("KB_QUIET=%q: stderr = %q, want the note", value, errOut.String())
		}
	}
}

func TestMainRun_KBQuietNeverSilencesAnError(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("KB_QUIET", "1")
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"project", "list"}, &out, &errOut); code != 66 {
		t.Fatalf("exit code = %d, want 66", code)
	}
	if !strings.Contains(errOut.String(), "kb init") {
		t.Errorf("stderr = %q, want the error even under KB_QUIET", errOut.String())
	}
}

// TestTestsRunUnderACeiling guards the TestMain in testmain_test.go: without
// it, a test in a directory with no workspace of its own could walk up into a
// real one when TMPDIR points inside it.
func TestTestsRunUnderACeiling(t *testing.T) {
	if got := os.Getenv("KB_CEILING_DIRECTORIES"); got != os.TempDir() {
		t.Errorf("KB_CEILING_DIRECTORIES = %q, want the system temp directory %q", got, os.TempDir())
	}
}
