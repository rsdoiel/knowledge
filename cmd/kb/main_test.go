package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGlobalFlags_DBAndJSON(t *testing.T) {
	opts, rest, err := parseGlobalFlags([]string{"--db", "/tmp/x.db", "--json", "project", "list"})
	if err != nil {
		t.Fatalf("parseGlobalFlags: %v", err)
	}
	if opts.dbPath != "/tmp/x.db" {
		t.Errorf("dbPath = %q, want /tmp/x.db", opts.dbPath)
	}
	if !opts.jsonOut {
		t.Error("expected jsonOut=true")
	}
	if opts.debugOn {
		t.Error("expected debugOn=false")
	}
	if len(rest) != 2 || rest[0] != "project" || rest[1] != "list" {
		t.Errorf("rest = %v, want [project list]", rest)
	}
}

func TestParseGlobalFlags_Debug(t *testing.T) {
	opts, rest, err := parseGlobalFlags([]string{"--debug", "project", "list"})
	if err != nil {
		t.Fatalf("parseGlobalFlags: %v", err)
	}
	if !opts.debugOn {
		t.Error("expected debugOn=true")
	}
	if len(rest) != 2 || rest[0] != "project" {
		t.Errorf("rest = %v, want [project list]", rest)
	}
}

func TestParseGlobalFlags_NoGlobalFlags(t *testing.T) {
	opts, rest, err := parseGlobalFlags([]string{"project", "list"})
	if err != nil {
		t.Fatalf("parseGlobalFlags: %v", err)
	}
	if opts.dbPath != "" || opts.jsonOut || opts.debugOn {
		t.Errorf("dbPath=%q jsonOut=%v debugOn=%v, want zero values", opts.dbPath, opts.jsonOut, opts.debugOn)
	}
	if len(rest) != 2 {
		t.Errorf("rest = %v, want [project list]", rest)
	}
}

func TestParseGlobalFlags_MissingDBValue(t *testing.T) {
	if _, _, err := parseGlobalFlags([]string{"--db"}); err == nil {
		t.Error("expected an error for --db with no value")
	}
}

// No-args behavior (launch the TUI, per cli-tui-design.md decision 4) is
// intentionally not exercised through mainRun here: doing so would try to
// start a real tea.Program against this test process's stdin/stdout,
// which isn't a TTY. The TUI's actual logic (newTUIModel, Update, View) is
// covered directly in tui_model_test.go without going through a real
// terminal; running the real thing is a manual, eyes-on verification step
// instead (see cli-tui-plan.md W7).

func TestMainRun_HelpFlagPrintsUsage(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"-h"}, {"-help"}, {"--help"}} {
		var out, errOut bytes.Buffer
		code := mainRun(args, &out, &errOut)
		if code != 0 {
			t.Errorf("args=%v: exit code = %d, want 0", args, code)
		}
		if out.Len() == 0 {
			t.Errorf("args=%v: expected usage text on stdout", args)
		}
	}
}

func TestMainRun_UnknownVerbOpensDBAndReturnsUsageError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "agents", "knowledge.db")
	var out, errOut bytes.Buffer
	code := mainRun([]string{"--db", dbPath, "bogus"}, &out, &errOut)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "bogus") {
		t.Errorf("errOut = %q, want it to mention the unknown verb", errOut.String())
	}
}

// TestMainRun_AmbientOpenGuardsMissingWorkspace is DR-0021 item 4 / DR-0022:
// a verb resolved through the ambient default (no --db) must not silently
// create agents/knowledge.db in whatever directory it happens to run from --
// it fails and names both kb init and kb import as the ways to get one.
func TestMainRun_AmbientOpenGuardsMissingWorkspace(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var out, errOut bytes.Buffer
	code := mainRun([]string{"project", "list"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "kb init") {
		t.Errorf("errOut = %q, want it to mention kb init", errOut.String())
	}
	if !strings.Contains(errOut.String(), "kb import") {
		t.Errorf("errOut = %q, want it to mention kb import", errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "agents", "knowledge.db")); err == nil {
		t.Errorf("the guard should not have created agents/knowledge.db")
	}
}

// TestMainRun_ExplicitDBPathStillAutoCreates is DR-0022: an explicit --db
// PATH is not "ambient" -- the caller said exactly where to open -- so it
// keeps today's open-or-create behavior even when the guard is active.
func TestMainRun_ExplicitDBPathStillAutoCreates(t *testing.T) {
	t.Chdir(t.TempDir()) // cwd has no workspace of its own
	dbPath := filepath.Join(t.TempDir(), "agents", "knowledge.db")

	var out, errOut bytes.Buffer
	code := mainRun([]string{"--db", dbPath, "project", "list"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, errOut.String())
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("expected explicit --db path to be auto-created: %v", err)
	}
}

// TestMainRun_ImportBypassesGuardOnAmbientPath is DR-0021 item 5: import has
// no path handling of its own (unlike merge/index/init), so it depends on the
// ambient-open branch for its create-capability -- the guard must not block
// it, or the documented rebuild recipe (workspace:DR-0002) breaks.
func TestMainRun_ImportBypassesGuardOnAmbientPath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	seed := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(seed, nil, 0o644); err != nil {
		t.Fatalf("writing seed file: %v", err)
	}

	var out, errOut bytes.Buffer
	code := mainRun([]string{"import", "-in", seed}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "agents", "knowledge.db")); err != nil {
		t.Errorf("expected import to create the ambient db: %v", err)
	}
}

func TestResolveDBPath_DefaultsUnderCwd(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	got, err := resolveDBPath("")
	if err != nil {
		t.Fatalf("resolveDBPath: %v", err)
	}
	want := filepath.Join(dir, "agents", "knowledge.db")
	if got != want {
		t.Errorf("resolveDBPath(\"\") = %q, want %q", got, want)
	}
}

func TestResolveDBPath_RelativeOverrideJoinsCwd(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	got, err := resolveDBPath("mydb.db")
	if err != nil {
		t.Fatalf("resolveDBPath: %v", err)
	}
	want := filepath.Join(dir, "mydb.db")
	if got != want {
		t.Errorf("resolveDBPath(\"mydb.db\") = %q, want %q", got, want)
	}
}

func TestResolveDBPath_AbsoluteOverrideUnchanged(t *testing.T) {
	got, err := resolveDBPath("/tmp/somewhere/x.db")
	if err != nil {
		t.Fatalf("resolveDBPath: %v", err)
	}
	if got != "/tmp/somewhere/x.db" {
		t.Errorf("resolveDBPath(abs) = %q, want unchanged", got)
	}
}
