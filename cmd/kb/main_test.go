package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGlobalFlags_DBAndJSON(t *testing.T) {
	dbPath, jsonOut, debugOn, rest, err := parseGlobalFlags([]string{"--db", "/tmp/x.db", "--json", "project", "list"})
	if err != nil {
		t.Fatalf("parseGlobalFlags: %v", err)
	}
	if dbPath != "/tmp/x.db" {
		t.Errorf("dbPath = %q, want /tmp/x.db", dbPath)
	}
	if !jsonOut {
		t.Error("expected jsonOut=true")
	}
	if debugOn {
		t.Error("expected debugOn=false")
	}
	if len(rest) != 2 || rest[0] != "project" || rest[1] != "list" {
		t.Errorf("rest = %v, want [project list]", rest)
	}
}

func TestParseGlobalFlags_Debug(t *testing.T) {
	_, _, debugOn, rest, err := parseGlobalFlags([]string{"--debug", "project", "list"})
	if err != nil {
		t.Fatalf("parseGlobalFlags: %v", err)
	}
	if !debugOn {
		t.Error("expected debugOn=true")
	}
	if len(rest) != 2 || rest[0] != "project" {
		t.Errorf("rest = %v, want [project list]", rest)
	}
}

func TestParseGlobalFlags_NoGlobalFlags(t *testing.T) {
	dbPath, jsonOut, debugOn, rest, err := parseGlobalFlags([]string{"project", "list"})
	if err != nil {
		t.Fatalf("parseGlobalFlags: %v", err)
	}
	if dbPath != "" || jsonOut || debugOn {
		t.Errorf("dbPath=%q jsonOut=%v debugOn=%v, want zero values", dbPath, jsonOut, debugOn)
	}
	if len(rest) != 2 {
		t.Errorf("rest = %v, want [project list]", rest)
	}
}

func TestParseGlobalFlags_MissingDBValue(t *testing.T) {
	_, _, _, _, err := parseGlobalFlags([]string{"--db"})
	if err == nil {
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
