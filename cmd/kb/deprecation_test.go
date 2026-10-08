package main

import (
	"bytes"
	"strings"
	"testing"
)

// DR-0057: --project and --workspace remain on the record verbs as aliases for
// the qualified form for a release or two, and say so on standard error, never
// standard output, so --json stays parseable.

// deprecationRoot builds a workspace with clasm/DR-0001, cold/DR-0001 and
// workspace/DR-0001, closes the handle, and makes it the working directory so
// mainRun can open it.
func deprecationRoot(t *testing.T) {
	t.Helper()
	kb, root := namedWorkspaceKB(t, "Laboratory")
	seedTiers(t, kb, root, "clasm", "cold")
	kb.Close()
	t.Chdir(root)
	t.Setenv("KB_QUIET", "")
}

func runMain(t *testing.T, args ...string) (code int, out, errOut string) {
	t.Helper()
	var o, e bytes.Buffer
	code = mainRun(args, &o, &e)
	return code, o.String(), e.String()
}

func TestDeprecation_ProjectFlagStillWorksAndSaysSo(t *testing.T) {
	deprecationRoot(t)
	for _, c := range []struct {
		alias, modern []string
		hint          string
	}{
		{[]string{"record", "list", "--project", "clasm"}, []string{"record", "list", "clasm"}, "kb record list clasm"},
		{[]string{"record", "pending", "--project", "clasm"}, []string{"record", "pending", "clasm"}, "kb record pending clasm"},
		{[]string{"record", "show", "0001", "--project", "clasm"}, []string{"record", "show", "clasm/0001"}, "clasm/DR-"},
		{[]string{"record", "concepts", "0001", "--project", "clasm"}, []string{"record", "concepts", "clasm/0001"}, "clasm/DR-"},
	} {
		code, out, errOut := runMain(t, c.alias...)
		mcode, mout, merr := runMain(t, c.modern...)
		if code != 0 || mcode != 0 {
			t.Fatalf("%v: exit %d / %d; %s %s", c.alias, code, mcode, errOut, merr)
		}
		if out != mout {
			t.Errorf("%v printed %q, the modern form printed %q; they must match", c.alias, out, mout)
		}
		if !strings.Contains(errOut, "--project clasm is deprecated") || !strings.Contains(errOut, c.hint) {
			t.Errorf("%v: stderr = %q, want a deprecation note containing %q", c.alias, errOut, c.hint)
		}
		if merr != "" {
			t.Errorf("%v: the modern form wrote %q to stderr, want nothing", c.modern, merr)
		}
		if strings.Contains(out, "deprecated") {
			t.Errorf("%v: the note leaked to standard output", c.alias)
		}
	}
}

func TestDeprecation_WorkspaceFlagStillWorksAndSaysSo(t *testing.T) {
	deprecationRoot(t)
	for _, c := range []struct{ alias, modern []string }{
		{[]string{"record", "list", "--workspace"}, []string{"record", "list", "workspace"}},
		{[]string{"record", "show", "0001", "--workspace"}, []string{"record", "show", "workspace/0001"}},
	} {
		code, out, errOut := runMain(t, c.alias...)
		_, mout, _ := runMain(t, c.modern...)
		if code != 0 || out != mout {
			t.Errorf("%v: exit %d, output %q vs modern %q", c.alias, code, out, mout)
		}
		if !strings.Contains(errOut, "--workspace is deprecated") || !strings.Contains(errOut, "workspace") {
			t.Errorf("%v: stderr = %q, want a deprecation note", c.alias, errOut)
		}
	}
}

func TestDeprecation_WriteVerbsNoteItToo(t *testing.T) {
	deprecationRoot(t)
	// Both verbs fail on purpose (no such record) after the flag is read; what
	// matters is that the note is made before the verb acts.
	_, _, errOut := runMain(t, "record", "set-status", "0099", "accepted", "--project", "clasm")
	if !strings.Contains(errOut, "--project clasm is deprecated") {
		t.Errorf("set-status: stderr = %q, want the note", errOut)
	}
	_, _, errOut = runMain(t, "record", "delete", "0099", "--project", "clasm", "--dry-run")
	if !strings.Contains(errOut, "--project clasm is deprecated") {
		t.Errorf("delete: stderr = %q, want the note", errOut)
	}
}

func TestDeprecation_JSONStaysParseable(t *testing.T) {
	deprecationRoot(t)
	code, out, errOut := runMain(t, "--json", "record", "list", "--project", "clasm")
	if code != 0 {
		t.Fatalf("exit %d; %s", code, errOut)
	}
	assertValidJSON(t, []byte(out))
	if !strings.Contains(errOut, "deprecated") {
		t.Errorf("stderr = %q, want the note under --json too", errOut)
	}
}

func TestDeprecation_NotWhereTheFlagIsTheOnlyWay(t *testing.T) {
	deprecationRoot(t)
	for _, args := range [][]string{
		{"record", "new", "--project", "clasm", "--title", "T", "--trigger", "design"},
		{"record", "new", "--workspace", "--title", "T2", "--trigger", "design"},
		{"record", "fuzzy-tag", "--project", "clasm"},
		{"observation", "add", "--project", "clasm", "note", "a note"},
		{"concept", "recall", "x", "--project", "clasm"},
	} {
		_, _, errOut := runMain(t, args...)
		if strings.Contains(errOut, "deprecated") {
			t.Errorf("%v: stderr = %q, want no deprecation note (the flag is the way to say it)", args, errOut)
		}
	}
}

func TestDeprecation_ModernFormsAreQuiet(t *testing.T) {
	deprecationRoot(t)
	for _, args := range [][]string{
		{"record", "list", "clasm", "cold"},
		{"record", "show", "clasm/DR-0001"},
		{"record", "pending", "--all"},
	} {
		if _, _, errOut := runMain(t, args...); errOut != "" {
			t.Errorf("%v: stderr = %q, want nothing", args, errOut)
		}
	}
}

func TestDeprecation_KBQuietSilencesTheNote(t *testing.T) {
	deprecationRoot(t)
	t.Setenv("KB_QUIET", "1")
	code, _, errOut := runMain(t, "record", "list", "--project", "clasm")
	if code != 0 || errOut != "" {
		t.Errorf("exit %d, stderr = %q; want 0 and nothing", code, errOut)
	}
}
