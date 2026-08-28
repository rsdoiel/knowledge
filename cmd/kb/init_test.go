package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

func TestCmdInit_CreatesSchemaOnlyDB(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := cmdInit(nil, nil, false, []string{root}, &out); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}

	dbPath := knowledge.DefaultPath(root)
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected %s to exist: %v", dbPath, err)
	}

	kb, err := knowledge.Open(dbPath)
	if err != nil {
		t.Fatalf("reopening %s: %v", dbPath, err)
	}
	defer kb.Close()
	projects, err := kb.Projects()
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("got %d projects, want 0 (schema-only)", len(projects))
	}
}

func TestCmdInit_DefaultsToCwd(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out bytes.Buffer
	if err := cmdInit(nil, nil, false, nil, &out); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "agents", "knowledge.db")); err != nil {
		t.Fatalf("expected agents/knowledge.db under cwd: %v", err)
	}
}

func TestCmdInit_IdempotentOnExistingWorkspace(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := cmdInit(nil, nil, false, []string{root}, &out); err != nil {
		t.Fatalf("first cmdInit: %v", err)
	}

	dbPath := knowledge.DefaultPath(root)
	kb, err := knowledge.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := kb.AddProject("demo", "a project that should survive re-init"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	kb.Close()

	out.Reset()
	if err := cmdInit(nil, nil, false, []string{root}, &out); err != nil {
		t.Fatalf("second cmdInit: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("already initialized")) {
		t.Errorf("expected an already-initialized message, got %q", out.String())
	}

	kb2, err := knowledge.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kb2.Close()
	projects, err := kb2.Projects()
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "demo" {
		t.Errorf("re-init clobbered existing data: %+v", projects)
	}
}

func TestCmdInit_RejectsExtraArgs(t *testing.T) {
	var out bytes.Buffer
	if err := cmdInit(nil, nil, false, []string{"a", "b"}, &out); err == nil {
		t.Error("expected an error for more than one positional argument")
	}
}

func TestCmdInit_JSONOutput(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := cmdInit(nil, nil, true, []string{root}, &out); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out.String())
	}
	if result["path"] == "" || result["path"] == nil {
		t.Errorf("expected a non-empty path field, got %v", result)
	}
}

// TestMainRun_InitBypassesAmbientOpen exercises the full CLI path: init must
// not trigger main.go's ambient --db resolution before it gets to run its own
// path logic, the same way merge and index are already exempted. Without that
// bypass, running `kb init TARGET` from a cwd with no workspace of its own
// would silently create a second, stray ambient agents/knowledge.db at cwd in
// addition to the one actually requested at TARGET.
func TestMainRun_InitBypassesAmbientOpen(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	target := t.TempDir()

	var out, errOut bytes.Buffer
	code := mainRun([]string{"init", target}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(target, "agents", "knowledge.db")); err != nil {
		t.Errorf("expected target db to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, "agents", "knowledge.db")); err == nil {
		t.Errorf("init should not create an ambient db at cwd; %s exists", filepath.Join(cwd, "agents", "knowledge.db"))
	}
}
