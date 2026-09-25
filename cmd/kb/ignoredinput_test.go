package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Many verbs checked only that they had *enough* arguments. A bogus flag or a
// surplus argument was dropped and the command exited 0: `project list
// -bogus` listed, `project show p q` showed p, `source add T more words`
// stored the title "T" and lost the rest, `record show 1 x` ignored x. A typo
// that changes what a command means must not succeed silently, so each of
// these is now a usage error (exit 2, see usageexit_test.go).

// seededDB returns the path of a database holding project p, concept c,
// observation 1 and source 1, built through the CLI itself.
func seededDB(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	for _, args := range [][]string{
		{"project", "add", "p", "d"},
		{"concept", "add", "c"},
		{"observation", "add", "--project", "p", "note", "hello"},
		{"source", "add", "T"},
	} {
		var out, errOut bytes.Buffer
		if code := mainRun(append([]string{"--db", dbPath}, args...), &out, &errOut); code != 0 {
			t.Fatalf("seed %v: exit %d: %s", args, code, errOut.String())
		}
	}
	return dbPath
}

func runOn(dbPath string, args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := mainRun(append([]string{"--db", dbPath}, args...), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestMainRun_BogusFlagIsAUsageError(t *testing.T) {
	db := seededDB(t)
	for _, args := range [][]string{
		{"project", "list", "--bogus-flag"},
		{"project", "show", "p", "--bogus-flag"},
		{"project", "show", "--bogus-flag", "p"},
		{"project", "concepts", "p", "--bogus-flag"},
		{"project", "set-status", "p", "active", "--bogus-flag"},
		{"project", "set-description", "--bogus-flag", "d"},
		{"observation", "show", "1", "--bogus-flag"},
		{"observation", "sources", "1", "--bogus-flag"},
		{"concept", "list", "--bogus-flag"},
		{"concept", "rename", "--bogus-flag", "x"},
		{"link", "project", "p", "c", "--bogus-flag"},
		{"link", "observation", "1", "c", "--bogus-flag"},
		{"source", "list", "--bogus-flag"},
		{"source", "show", "1", "--bogus-flag"},
		{"source", "remove", "1", "--bogus-flag"},
		{"source", "retract", "--bogus-flag", "note"},
		{"source", "link", "1", "1", "--bogus-flag"},
		{"source", "check-retractions", "--bogus-flag"},
		{"summary", "--bogus-flag"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			code, out, errOut := runOn(db, args...)
			if code != 2 {
				t.Errorf("exit %d, want 2; stdout=%.100q stderr=%q", code, out, errOut)
			}
			if !strings.Contains(errOut, "unknown flag") && !strings.Contains(errOut, "usage:") {
				t.Errorf("stderr = %q, want it to name the unknown flag or the usage", errOut)
			}
		})
	}
}

func TestMainRun_SurplusArgumentIsAUsageError(t *testing.T) {
	db := seededDB(t)
	for _, args := range [][]string{
		{"project", "list", "extra"},
		{"project", "show", "p", "extra"},
		{"project", "concepts", "p", "extra"},
		{"project", "set-status", "p", "active", "extra"},
		{"observation", "list", "--project", "p", "extra"},
		{"observation", "show", "1", "extra"},
		{"observation", "sources", "1", "extra"},
		{"concept", "list", "extra"},
		{"link", "project", "p", "c", "extra"},
		{"link", "observation", "1", "c", "extra"},
		{"source", "list", "extra"},
		{"source", "show", "1", "extra"},
		{"source", "remove", "1", "extra"},
		{"source", "add", "T2", "more", "words"},
		{"source", "link", "1", "1", "stray"},
		{"source", "check-retractions", "extra"},
		{"record", "list", "extra"},
		{"record", "show", "1", "extra"},
		{"record", "concepts", "1", "extra"},
		{"record", "set-status", "1", "accepted", "extra"},
		{"record", "supersede", "1", "2", "extra"},
		{"record", "fmt", ".", "extra"},
		{"record", "new", "--project", "p", "--title", "t", "--trigger", "design", "stray"},
		{"summary", "extra"},
		{"format", "extra"},
		{"export", "extra"},
		{"import", "extra"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			code, out, errOut := runOn(db, args...)
			if code != 2 {
				t.Errorf("exit %d, want 2; stdout=%.100q stderr=%q", code, out, errOut)
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Errorf("a refused command left files behind: %v", entries)
			}
		})
	}
}

// `source link OBS SRC --relationship` with its value missing used to be
// dropped silently and link with the default relationship.
func TestMainRun_SourceLinkRelationshipNeedsAValue(t *testing.T) {
	db := seededDB(t)
	if code, _, e := runOn(db, "source", "link", "1", "1", "--relationship"); code != 2 {
		t.Errorf("exit %d, want 2; stderr=%q", code, e)
	}
	if code, _, e := runOn(db, "source", "link", "1", "1", "--relationship", "cited"); code != 0 {
		t.Errorf("valid --relationship: exit %d: %s", code, e)
	}
}

func TestMainRun_MergeRefusesSurplusArgumentBeforeCreatingFiles(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out, errOut bytes.Buffer
	code := mainRun([]string{"merge", "-a", "a.db", "-b", "b.db", "-out", "c.db", "stray"}, &out, &errOut)
	if code != 2 {
		t.Errorf("exit %d, want 2; stderr=%q", code, errOut.String())
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("merge left files behind: %v", entries)
	}
}

// A blank search term matches nothing by construction, so "no results" (exit
// 1) is the wrong answer to it: the command line itself is wrong.
func TestMainRun_BlankSearchTermIsAUsageError(t *testing.T) {
	db := seededDB(t)
	for _, args := range [][]string{
		{"search", ""}, {"search", " "}, {"search", "\t"}, {"search", "", ""}, {"search", " ", "\n"},
	} {
		t.Run(strings.Join(args, "|"), func(t *testing.T) {
			code, out, errOut := runOn(db, args...)
			if code != 2 {
				t.Errorf("exit %d, want 2; stdout=%.100q stderr=%q", code, out, errOut)
			}
			if !strings.Contains(errOut, "usage: search TERM") {
				t.Errorf("stderr = %q, want the usage line", errOut)
			}
		})
	}
	if code, _, _ := runOn(db, "search", "hello"); code != 0 {
		t.Errorf("a real term found in the seed data: exit %d, want 0", code)
	}
}

// What must keep working: free text after the fixed arguments, a name that
// looks like a flag when it follows `--`, quoted multi-word values, and every
// runtime failure still exiting 1.
func TestMainRun_FreeTextAndDashDashStayAccepted(t *testing.T) {
	db := seededDB(t)
	for _, args := range [][]string{
		{"project", "set-description", "p", "keeps", "-x", "text"},
		{"source", "retract", "1", "note", "with", "-x", "in", "it"},
		{"project", "add", "q", "desc", "-x", "words"},
		{"project", "add", "--", "-weird"},
		{"project", "show", "--", "-weird"},
		{"project", "concepts", "--", "-weird"},
		{"concept", "add", "--", "-old"},
		{"concept", "rename", "--", "-old", "new"},
		{"source", "add", "Two words"},
		{"observation", "add", "--project", "p", "note", "pass", "-h", "to", "it"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			if code, out, errOut := runOn(db, args...); code != 0 {
				t.Errorf("exit %d, want 0; stdout=%.100q stderr=%q", code, out, errOut)
			}
		})
	}
	if code, out, _ := runOn(db, "project", "show", "--", "-weird"); code != 0 || !strings.Contains(out, "-weird") {
		t.Errorf("project show -- -weird: exit %d, stdout %q", code, out)
	}
	if code, out, _ := runOn(db, "source", "show", "1"); code != 0 || !strings.Contains(out, "T") {
		t.Errorf("source show 1: exit %d, stdout %q", code, out)
	}
	for _, args := range [][]string{
		{"project", "show", "nosuch"}, {"source", "show", "999"}, {"observation", "show", "999"},
	} {
		if code, _, _ := runOn(db, args...); code != 1 {
			t.Errorf("%v: exit %d, want 1 (a lookup that found nothing)", args, code)
		}
	}
}

func TestPlainArgs(t *testing.T) {
	const usage = "usage: x A B"
	for _, tc := range []struct {
		name            string
		args            []string
		min, max, fixed int
		want            []string
		wantErr         string
	}{
		{"exact", []string{"a", "b"}, 2, 2, 2, []string{"a", "b"}, ""},
		{"too few", []string{"a"}, 2, 2, 2, nil, usage},
		{"too many", []string{"a", "b", "c"}, 2, 2, 2, nil, usage},
		{"unbounded tail", []string{"a", "b", "c", "d"}, 2, -1, 1, []string{"a", "b", "c", "d"}, ""},
		{"flag in the fixed part", []string{"--x", "b"}, 2, 2, 2, nil, "unknown flag \"--x\""},
		{"flag in a free tail is text", []string{"a", "-x", "y"}, 2, -1, 1, []string{"a", "-x", "y"}, ""},
		{"dash dash lets a flag-shaped name through", []string{"--", "-x", "b"}, 2, 2, 2, []string{"-x", "b"}, ""},
		{"a lone dash is not a flag", []string{"-", "b"}, 2, 2, 2, []string{"-", "b"}, ""},
		{"nothing wanted, nothing given", nil, 0, 0, 0, []string{}, ""},
		{"nothing wanted, something given", []string{"a"}, 0, 0, 0, nil, usage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := plainArgs(tc.args, tc.min, tc.max, tc.fixed, usage)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tc.wantErr)
				}
				if !isUsageError(err) {
					t.Errorf("error %v is not a usage error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// --json, --db and --debug are global options and go before the verb. After
// it they used to be swallowed (so `project list --json` printed plain text
// and exited 0); now they are refused, and the message says where they go.
func TestMainRun_GlobalOptionAfterTheVerbSaysWhereItGoes(t *testing.T) {
	db := seededDB(t)
	for _, args := range [][]string{
		{"project", "list", "--json"},
		{"project", "list", "-json"},
		{"project", "show", "p", "--json"},
		{"record", "list", "--json"},
		{"observation", "list", "--project", "p", "--json"},
		{"concept", "suggest", "--json"},
		{"source", "list", "--debug"},
		{"project", "list", "--db", "x.db"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			code, _, errOut := runOn(db, args...)
			if code != 2 {
				t.Errorf("exit %d, want 2; stderr=%q", code, errOut)
			}
			if !strings.Contains(errOut, "before the verb") {
				t.Errorf("stderr = %q, want a hint that global options go before the verb", errOut)
			}
		})
	}
	// An ordinary unknown flag gets no such hint.
	if _, _, errOut := runOn(db, "project", "list", "--bogus"); strings.Contains(errOut, "before the verb") {
		t.Errorf("hint given for a flag that is not a global option: %q", errOut)
	}
	// And the documented position still works.
	if code, out, e := runOn(db, "--json", "project", "list"); code != 0 || !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Errorf("--json before the verb: exit %d, stdout %.60q, stderr %q", code, out, e)
	}
}

func TestGlobalOptionHint(t *testing.T) {
	for _, arg := range []string{"--json", "-json", "--db", "-db", "--debug", "-debug"} {
		if globalOptionHint(arg) == "" {
			t.Errorf("globalOptionHint(%q) is empty", arg)
		}
	}
	for _, arg := range []string{"--bogus", "-x", "json", "--jsonl", ""} {
		if h := globalOptionHint(arg); h != "" {
			t.Errorf("globalOptionHint(%q) = %q, want empty", arg, h)
		}
	}
}
