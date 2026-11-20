package cli

import (
	"bytes"
	"strings"
	"testing"
)

// A record written by `record new` is a file until `ingest` loads it. Asking
// for it before then said only "no record DR-0001 in the workspace tier",
// which is also what a record that does not exist says. The message must say
// the file is on disk and what to run; the exit code stays 1 (not found).
func TestRecordShow_FileOnDiskButNotIngestedSaysSo(t *testing.T) {
	t.Chdir(t.TempDir())
	run := func(args ...string) (int, string, string) {
		var out, errOut bytes.Buffer
		code := mainRun(args, &out, &errOut)
		return code, out.String(), errOut.String()
	}
	if code, _, e := run("init", "."); code != 0 {
		t.Fatalf("init: exit %d: %s", code, e)
	}
	if code, _, e := run("record", "new", "--workspace", "--title", "T", "--trigger", "design"); code != 0 {
		t.Fatalf("record new: exit %d: %s", code, e)
	}
	for _, args := range [][]string{
		{"record", "show", "workspace/DR-0001"},
		{"record", "relates-to", "workspace/DR-0001", "workspace/DR-0001"},
	} {
		code, out, e := run(args...)
		if code != 1 {
			t.Errorf("%v: exit %d, want 1 (not found)", args, code)
		}
		msg := out + e
		if !strings.Contains(msg, "on disk") || !strings.Contains(msg, "kb ingest agents/decisions") {
			t.Errorf("%v: message %q should say the record is on disk and name `kb ingest agents/decisions`", args, msg)
		}
	}
	// A record that exists nowhere keeps the plain message.
	code, out, e := run("record", "show", "workspace/DR-0042")
	if code != 1 || strings.Contains(out+e, "on disk") {
		t.Errorf("missing record: exit %d, message %q; want exit 1 and no mention of disk", code, out+e)
	}
	// After ingest it works.
	if code, _, e := run("ingest", "agents/decisions"); code != 0 {
		t.Fatalf("ingest: exit %d: %s", code, e)
	}
	if code, _, e := run("record", "show", "workspace/DR-0001"); code != 0 {
		t.Errorf("show after ingest: exit %d: %s", code, e)
	}
}
