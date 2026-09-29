package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// `kb project add` (DR-0055): a flag typed after NAME is refused rather than
// stored as description text, and adding a name that exists is exit 1, not
// "added".

func projectAddRun(t *testing.T, db string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := mainRun(append([]string{"--db", db}, args...), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestProjectAdd_TrailingOwnFlagIsAUsageError(t *testing.T) {
	for _, tail := range [][]string{
		{"--status", "paused"}, {"-status", "paused"}, {"--status=paused"}, {"-status=paused"},
	} {
		db := filepath.Join(t.TempDir(), "kb.db")
		args := append([]string{"project", "add", "demo", "a description"}, tail...)
		code, _, errOut := projectAddRun(t, db, args...)
		if code != 2 {
			t.Errorf("%v: exit %d, want 2; stderr: %s", tail, code, errOut)
		}
		if !strings.Contains(errOut, "before NAME") {
			t.Errorf("%v: stderr does not say the flag goes before NAME: %s", tail, errOut)
		}
		if _, out, _ := projectAddRun(t, db, "project", "list"); strings.Contains(out, "demo") {
			t.Errorf("%v: a project was stored despite the usage error:\n%s", tail, out)
		}
	}
}

func TestProjectAdd_FlagBeforeNameStillWorks(t *testing.T) {
	db := filepath.Join(t.TempDir(), "kb.db")
	code, _, errOut := projectAddRun(t, db, "project", "add", "--status", "paused", "demo", "a description")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	_, out, _ := projectAddRun(t, db, "project", "show", "demo")
	if !strings.Contains(out, "paused") || !strings.Contains(out, "a description") {
		t.Errorf("show does not carry status and description:\n%s", out)
	}
}

func TestProjectAdd_DoubleDashKeepsFlagLikeWordsAsDescription(t *testing.T) {
	db := filepath.Join(t.TempDir(), "kb.db")
	code, _, errOut := projectAddRun(t, db, "project", "add", "--", "demo", "uses", "--status", "words")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	_, out, _ := projectAddRun(t, db, "project", "show", "demo")
	if !strings.Contains(out, "uses --status words") {
		t.Errorf("description lost its flag-looking words:\n%s", out)
	}
}

func TestProjectAdd_ExistingNameIsExitOneAndChangesNothing(t *testing.T) {
	db := filepath.Join(t.TempDir(), "kb.db")
	if code, _, errOut := projectAddRun(t, db, "project", "add", "demo", "first"); code != 0 {
		t.Fatalf("first add: exit %d: %s", code, errOut)
	}
	_, before, _ := projectAddRun(t, db, "project", "show", "demo")

	code, out, errOut := projectAddRun(t, db, "project", "add", "--status", "paused", "demo", "second")
	if code != 1 {
		t.Errorf("exit %d, want 1; stdout: %s stderr: %s", code, out, errOut)
	}
	if strings.Contains(out, "added") {
		t.Errorf("said added for a project that already existed: %s", out)
	}
	if !strings.Contains(errOut, `"demo" already exists`) {
		t.Errorf("stderr does not name the existing project: %s", errOut)
	}
	if _, after, _ := projectAddRun(t, db, "project", "show", "demo"); after != before {
		t.Errorf("the existing project changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestProjectAdd_ExistingNameJSONErrorNamesTheClass(t *testing.T) {
	db := filepath.Join(t.TempDir(), "kb.db")
	projectAddRun(t, db, "project", "add", "demo")
	code, _, errOut := projectAddRun(t, db, "--json", "project", "add", "demo")
	if code != 1 || !strings.Contains(errOut, `"negative"`) {
		t.Errorf("exit %d, stderr %s; want 1 and class negative", code, errOut)
	}
}
