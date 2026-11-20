package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DR-0096: designs and plans are records with their own id prefixes and
// sequences, written to design/ and plans/ and approved like a decision.
func kindsWorkspace(t *testing.T) func(args ...string) (int, string, string) {
	t.Helper()
	noTerminal(t)
	t.Chdir(t.TempDir())
	run := func(args ...string) (int, string, string) {
		var out, errOut bytes.Buffer
		code := mainRun(args, &out, &errOut)
		return code, out.String(), errOut.String()
	}
	for _, a := range [][]string{{"init", "."}, {"project", "add", "p", "a project"}} {
		if code, _, e := run(a...); code != 0 {
			t.Fatalf("%v: exit %d: %s", a, code, e)
		}
	}
	return run
}

func TestRecordNew_DesignAndPlanHaveTheirOwnPrefixAndSequence(t *testing.T) {
	run := kindsWorkspace(t)
	for _, tc := range []struct {
		kind, title, wantRef, wantDir, wantFile string
	}{
		{"decision", "Use SQLite", "p/DR-0001", "decisions", "0001-use-sqlite.md"},
		{"design", "Search design", "p/DS-0001", "design", "DS-0001-search-design.md"},
		{"design", "Second design", "p/DS-0002", "design", "DS-0002-second-design.md"},
		{"plan", "Search plan", "p/PL-0001", "plans", "PL-0001-search-plan.md"},
		{"decision", "Another choice", "p/DR-0002", "decisions", "0002-another-choice.md"},
	} {
		code, out, e := run("record", "new", "--project", "p", "--kind", tc.kind, "--title", tc.title, "--trigger", "request")
		if code != 0 || !strings.Contains(out, tc.wantRef+" written to") {
			t.Fatalf("record new %s: exit %d, out %q, err %q; want %s", tc.kind, code, out, e, tc.wantRef)
		}
		path := filepath.Join("agents", "projects", "p", tc.wantDir, tc.wantFile)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		id := strings.TrimSuffix(strings.TrimPrefix(tc.wantRef, "p/"), "")
		id = strings.TrimPrefix(id, "DR-")
		if !strings.Contains(string(raw), `id: "`+id+`"`) || !strings.Contains(string(raw), "kind: "+tc.kind) || !strings.Contains(string(raw), "status: proposed") {
			t.Errorf("%s front matter lacks id %q, kind %s or proposed:\n%.300s", path, id, tc.kind, raw)
		}
	}
}

func TestRecordNew_DesignInTheWorkspaceTierIsAUsageError(t *testing.T) {
	run := kindsWorkspace(t)
	code, _, e := run("record", "new", "--workspace", "--kind", "design", "--title", "X", "--trigger", "request")
	if code != 2 || !strings.Contains(e, "project") {
		t.Errorf("exit %d, stderr %q; want 2 and a message that designs belong to a project", code, e)
	}
}

func TestIngest_ReadsDesignsAndPlansAndIgnoresOtherFilesInTheirDirectories(t *testing.T) {
	run := kindsWorkspace(t)
	for _, k := range []string{"design", "plan", "decision"} {
		if code, _, e := run("record", "new", "--project", "p", "--kind", k, "--title", "T "+k, "--trigger", "request"); code != 0 {
			t.Fatalf("record new %s: %s", k, e)
		}
	}
	// A free-form brief and a hand-off note are not records.
	for name, content := range map[string]string{
		"design/semantic-design.md": "# A brief with no front matter\n",
		"notes/20261120-note.md":    "# a note\n",
	} {
		path := filepath.Join("agents", "projects", "p", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	code, out, e := run("ingest", filepath.Join("agents", "projects", "p"))
	if code != 0 || !strings.Contains(out, "3 added") || !strings.Contains(out, "0 failed") {
		t.Fatalf("ingest: exit %d, out %q, err %q; want 3 added, 0 failed", code, out, e)
	}
	for _, ref := range []string{"p/DR-0001", "p/DS-0001", "p/PL-0001"} {
		if code, out, e := run("record", "show", ref); code != 0 || !strings.Contains(out, ref[2:]) {
			t.Errorf("record show %s: exit %d, out %.120q, err %q", ref, code, out, e)
		}
	}
}

func TestRecord_DesignRelatesToADecisionAndNeedsATerminalToBeAccepted(t *testing.T) {
	run := kindsWorkspace(t)
	for _, k := range []string{"design", "decision"} {
		if code, _, e := run("record", "new", "--project", "p", "--kind", k, "--title", "T "+k, "--trigger", "request"); code != 0 {
			t.Fatalf("record new %s: %s", k, e)
		}
	}
	if code, _, e := run("ingest", filepath.Join("agents", "projects", "p")); code != 0 {
		t.Fatalf("ingest: %s", e)
	}
	if code, _, e := run("record", "relates-to", "p/DS-0001", "p/DR-0001"); code != 0 {
		t.Fatalf("relates-to: exit %d: %s", code, e)
	}
	if code, out, _ := run("record", "show", "p/DR-0001"); code != 0 || !strings.Contains(out, "DS-0001") {
		t.Errorf("the decision does not show the design that relates to it: exit %d\n%s", code, out)
	}
	code, _, e := run("record", "set-status", "p/DS-0001", "accepted")
	if code != 2 {
		t.Errorf("accepting a design without a terminal: exit %d, stderr %q; want 2", code, e)
	}
	raw, _ := os.ReadFile(filepath.Join("agents", "projects", "p", "design", "DS-0001-t-design.md"))
	if !strings.Contains(string(raw), "status: proposed") {
		t.Errorf("the design file changed despite the refusal:\n%.300s", raw)
	}
}
