package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Found by X4's every-command check. `kb init --bogus-flag` took the flag as
// the PATH and created a workspace in a directory literally named "--bogus-flag"
// (exit 0). `kb search --json foo` took the option as search text and answered
// "no results" (exit 1), the answer a script reads as "nothing found" for what
// was really a mistyped command. Both now refuse a flag-shaped first argument, and
// accept `--` to give one on purpose, as every other verb does (DR-0041).

func TestInit_FlagShapedPathIsUsageAndCreatesNothing(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out, errOut bytes.Buffer
	code := mainRun([]string{"init", "--bogus-flag"}, &out, &errOut)
	if code != 2 {
		t.Errorf("exit %d, want 2; stderr %q", code, errOut.String())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("a refused init created %d entries in the directory", len(entries))
	}
}

func TestInit_DoubleDashGivesADashLeadingPath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"init", "--", "-odd-name"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d; stderr %q", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "-odd-name", "agents", "knowledge.db")); err != nil {
		t.Errorf("the workspace was not created at ./-odd-name: %v", err)
	}
}

func TestInit_PlainPathStillWorks(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"init", "ws"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d; stderr %q", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "ws", "agents", "knowledge.db")); err != nil {
		t.Error(err)
	}
}

func TestSearch_FlagShapedFirstWordIsUsageWithAGlobalOptionHint(t *testing.T) {
	code, _, errOut := runKB(t, "search", "--json", "foo")
	if code != 2 {
		t.Errorf("exit %d, want 2 (not 1, 'no results'); stderr %q", code, errOut)
	}
	if !strings.Contains(errOut, "--json") {
		t.Errorf("stderr %q should name the flag it refused", errOut)
	}
}

func TestSearch_DoubleDashSearchesADashLeadingTerm(t *testing.T) {
	code, _, errOut := runKB(t, "search", "--", "-x")
	// An empty database has no result for it: exit 1, and the term is reported
	// without the "--", so it was consumed rather than searched for.
	if code != 1 {
		t.Errorf("exit %d, want 1 (no results); stderr %q", code, errOut)
	}
	if strings.Contains(errOut, `"-- -x"`) || !strings.Contains(errOut, `-x`) {
		t.Errorf("stderr %q: want the term -x, without the --", errOut)
	}
}

func TestSearch_LaterWordsStayFreeText(t *testing.T) {
	code, _, errOut := runKB(t, "search", "map", "-reduce")
	if code != 1 {
		t.Errorf("exit %d, want 1 (no results): later words are text, dashes included; stderr %q", code, errOut)
	}
}
