package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `kb VERB -help` prints the verb's manual page. One level down it used to do
// something different for every verb: the Go flag package's "flag: help
// requested" (exit 1), a lookup of a project named "-help", an attempt to open
// a file called "-help", or "unknown flag". A verb's page documents all of its
// subverbs, so a help flag right after the subverb prints that page too.

var subverbHelpCases = []struct {
	verb string
	subs []string
}{
	{"project", []string{"add", "list", "show", "concepts", "set-status", "set-description", "rename"}},
	{"observation", []string{"add", "list", "show", "update"}},
	{"concept", []string{"add", "list", "suggest", "delete", "rename"}},
	{"source", []string{"add", "list", "show", "remove", "retract", "link", "check-retractions"}},
	{"link", []string{"project", "observation"}},
	{"record", []string{"new", "list", "show", "set-status", "supersede", "concepts"}},
	{"document", []string{"ingest", "list", "show", "tag", "fuzzy-tag", "frontmatter", "review"}},
}

func TestMainRun_SubverbHelpPrintsTheVerbPage(t *testing.T) {
	for _, tc := range subverbHelpCases {
		for _, sub := range tc.subs {
			for _, flagForm := range []string{"-help", "--help", "-h"} {
				name := tc.verb + "_" + sub + "_" + flagForm
				t.Run(name, func(t *testing.T) {
					dir := t.TempDir()
					t.Chdir(dir)
					var out, errOut bytes.Buffer
					code := mainRun([]string{tc.verb, sub, flagForm}, &out, &errOut)
					if code != 0 {
						t.Errorf("exit %d, want 0; stderr=%q", code, errOut.String())
					}
					if !strings.Contains(out.String(), "kb-"+tc.verb+"(1)") {
						t.Errorf("stdout is not the %s page: %.120q", tc.verb, out.String())
					}
					if errOut.Len() != 0 {
						t.Errorf("stderr = %q, want it empty", errOut.String())
					}
					if _, err := os.Stat(filepath.Join(dir, "agents", "knowledge.db")); err == nil {
						t.Error("a database was created while answering a help request")
					}
				})
			}
		}
	}
}

func TestMainRun_NestedSubverbHelpPrintsTheDocumentPage(t *testing.T) {
	for _, sub := range []string{"list", "promote"} {
		t.Run(sub, func(t *testing.T) {
			t.Chdir(t.TempDir())
			var out, errOut bytes.Buffer
			if code := mainRun([]string{"document", "review", sub, "-help"}, &out, &errOut); code != 0 {
				t.Errorf("exit %d, want 0; stderr=%q", code, errOut.String())
			}
			if !strings.Contains(out.String(), "kb-document(1)") {
				t.Errorf("stdout is not the document page: %.120q", out.String())
			}
		})
	}
}

// A help flag that follows other flags reaches the subverb's own flag parser,
// which reports it as flag.ErrHelp. That must become the page, not the
// package's "flag: help requested".
func TestMainRun_HelpAfterFlagsPrintsThePageNotFlagError(t *testing.T) {
	for _, args := range [][]string{
		{"project", "add", "--status", "active", "-help"},
		{"observation", "add", "--project", "p", "-help"},
		{"concept", "suggest", "--limit", "3", "-h"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "kb.db")
			var out, errOut bytes.Buffer
			code := mainRun(append([]string{"--db", dbPath}, args...), &out, &errOut)
			if code != 0 {
				t.Errorf("exit %d, want 0; stderr=%q", code, errOut.String())
			}
			if strings.Contains(errOut.String(), "help requested") {
				t.Errorf("stderr leaks the flag package's message: %q", errOut.String())
			}
			if !strings.Contains(out.String(), "kb-"+args[0]+"(1)") {
				t.Errorf("stdout is not the %s page: %.120q", args[0], out.String())
			}
		})
	}
}

// Only a help flag in the position where a subverb's first argument goes is a
// request for help. A "-h" inside free text stays text: observation bodies,
// descriptions and search terms are typed as bare words.
func TestMainRun_HelpFlagLookalikesInFreeTextStayText(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	run := func(args ...string) (int, string, string) {
		var out, errOut bytes.Buffer
		code := mainRun(append([]string{"--db", dbPath}, args...), &out, &errOut)
		return code, out.String(), errOut.String()
	}
	if code, _, e := run("project", "add", "p", "description", "-h", "words"); code != 0 {
		t.Fatalf("project add: exit %d: %s", code, e)
	}
	if code, out, e := run("observation", "add", "--project", "p", "note", "pass", "-h", "to", "it"); code != 0 {
		t.Fatalf("observation add: exit %d: %s", code, e)
	} else if strings.Contains(out, "user manual") {
		t.Errorf("observation add printed a help page instead of recording:\n%.200s", out)
	}
	code, out, e := run("observation", "list", "--project", "p")
	if code != 0 || !strings.Contains(out, "pass -h to it") {
		t.Errorf("observation list: exit %d, stdout %q, stderr %q; want the body recorded verbatim", code, out, e)
	}
	code, out, e = run("project", "list")
	if code != 0 || !strings.Contains(out, "description -h words") {
		t.Errorf("project list: exit %d, stdout %q, stderr %q; want the description recorded verbatim", code, out, e)
	}
}

func TestMainRun_UnknownVerbWithSubverbHelpStillFails(t *testing.T) {
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"bogus", "add", "-help"}, &out, &errOut); code == 0 {
		t.Errorf("exit 0 for an unknown verb; stdout=%.120q", out.String())
	}
}

// The hand-rolled parsers share splitFlags, which answered a help flag after
// other flags with `unknown flag "-help"` (exit 1). It is the same request as
// the FlagSet case above and must give the same page.
func TestMainRun_HelpAfterFlagsInHandRolledParsersPrintsThePage(t *testing.T) {
	for _, args := range [][]string{
		{"record", "list", "--status", "accepted", "-help"},
		{"record", "new", "--project", "p", "--help"},
		{"document", "list", "--project", "p", "-h"},
		{"document", "ingest", "--project", "p", "-help"},
		{"concept", "delete", "--force", "-help"},
		{"source", "add", "--doi", "10.1/x", "-help"},
		{"ingest", "--dry-run", "-help"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "kb.db")
			var out, errOut bytes.Buffer
			code := mainRun(append([]string{"--db", dbPath}, args...), &out, &errOut)
			if code != 0 {
				t.Errorf("exit %d, want 0; stderr=%q", code, errOut.String())
			}
			if !strings.Contains(out.String(), "kb-"+args[0]+"(1)") {
				t.Errorf("stdout is not the %s page: %.120q; stderr=%q", args[0], out.String(), errOut.String())
			}
		})
	}
}
