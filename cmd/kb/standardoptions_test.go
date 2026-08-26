package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// The standard options — help, license, version — are answered and exited
// before any real work, matching how clasm, scripttool, mdtools and harvey all
// handle them.
//
// This matters more for kb than for those, because kb auto-creates its
// database: resolveDBPath("") is ./agents/knowledge.db relative to the current
// directory, so any path that opens one before answering leaves a 127KB file
// behind wherever it was run.
func TestMainRun_StandardOptionsCreateNoDatabase(t *testing.T) {
	for _, args := range [][]string{
		{"-help"}, {"--help"}, {"-h"},
		{"-version"}, {"--version"},
		{"-license"}, {"--license"},
		{"help"}, {"help", "record"},
		{"project", "-help"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)

			var out, errOut bytes.Buffer
			if code := mainRun(args, &out, &errOut); code != 0 {
				t.Errorf("exit %d, want 0; stderr=%s", code, errOut.String())
			}
			if out.Len() == 0 {
				t.Errorf("nothing written to stdout; stderr=%s", errOut.String())
			}
			if _, err := os.Stat(filepath.Join(dir, "agents", "knowledge.db")); err == nil {
				t.Error("a database was created while answering an informational option")
			}
		})
	}
}

// -version and -license render from version.go, which cmt regenerates from
// codemeta.json, so what the CLI reports and what the release says cannot
// drift apart.
func TestMainRun_VersionAndLicenseComeFromVersionGo(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, tc := range []struct{ name, flag, want string }{
		{"version number", "-version", knowledge.Version},
		{"release hash", "-version", knowledge.ReleaseHash},
		{"app name", "-version", "kb"},
		{"license name", "-license", "GNU Affero General Public License"},
		{"copyright holder", "-license", "R. S. Doiel"},
	} {
		var out, errOut bytes.Buffer
		if code := mainRun([]string{tc.flag}, &out, &errOut); code != 0 {
			t.Fatalf("%s: exit %d: %s", tc.name, code, errOut.String())
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Errorf("%s: kb %s output lacks %q:\n%s", tc.name, tc.flag, tc.want, out.String())
		}
	}
}

// -h is deliberately NOT declared as a flag. Leaving it undefined makes Go's
// flag package return ErrHelp, which mainRun answers with kb's own Pandoc help
// text rather than letting the FlagSet print its terse usage over it.
func TestMainRun_DashHGivesTheRealHelpNotFlagUsage(t *testing.T) {
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"-h"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	got := out.String()
	if !strings.Contains(got, "user manual") || !strings.Contains(got, "# VERBS") {
		t.Errorf("-h did not print the real help page:\n%s", got)
	}
	if strings.Contains(got, "Usage of") {
		t.Errorf("-h printed the FlagSet's usage instead of kb's help:\n%s", got)
	}
}

// index builds the index from the record files and never queries the database
// (DR-0008), so opening one is a pointless side effect — exactly as it is for
// merge, which mainRun already special-cases for this reason.
func TestMainRun_IndexCreatesNoDatabase(t *testing.T) {
	dir := t.TempDir()
	decisions := filepath.Join(dir, "decisions")
	if err := os.MkdirAll(decisions, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	testRecord{ID: "0001", Project: "probe"}.write(t, decisions)
	t.Chdir(dir)

	var out, errOut bytes.Buffer
	if code := mainRun([]string{"index", "decisions", "--stdout"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "DR-0001") {
		t.Errorf("index output looks wrong:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "agents", "knowledge.db")); err == nil {
		t.Error("kb index created a database it never reads")
	}
}

// The global options keep working in both dash forms, and stop at the verb so
// a verb's own flags reach it untouched.
func TestMainRun_GlobalOptionsInBothDashForms(t *testing.T) {
	for _, args := range [][]string{
		{"-db", "x.db", "-json", "project", "list"},
		{"--db", "x.db", "--json", "project", "list"},
	} {
		dir := t.TempDir()
		t.Chdir(dir)
		var out, errOut bytes.Buffer
		if code := mainRun(args, &out, &errOut); code != 0 {
			t.Errorf("%v: exit %d: %s", args, code, errOut.String())
		}
		if _, err := os.Stat(filepath.Join(dir, "x.db")); err != nil {
			t.Errorf("%v: --db was not honoured (no x.db): %v", args, err)
		}
	}
}

// A verb's own flags are not consumed by the global FlagSet.
func TestMainRun_VerbFlagsReachTheVerb(t *testing.T) {
	dir := t.TempDir()
	decisions := filepath.Join(dir, "clasm", "decisions")
	if err := os.MkdirAll(decisions, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	testRecord{ID: "0001", Project: "clasm"}.write(t, decisions)
	t.Chdir(dir)

	var out, errOut bytes.Buffer
	if code := mainRun([]string{"--db", "kb.db", "ingest", "clasm/decisions", "--dry-run"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "dry run") {
		t.Errorf("--dry-run did not reach the ingest verb:\n%s", out.String())
	}
}

func TestMainRun_UnknownGlobalFlagIsAUsageError(t *testing.T) {
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"-nosuchflag"}, &out, &errOut); code != 2 {
		t.Errorf("exit %d, want 2 for an unknown global flag", code)
	}
	if errOut.Len() == 0 {
		t.Error("nothing written to stderr for an unknown global flag")
	}
}

// The drift check the whole {app_name}/{version} substitution exists to make
// possible: help text carries the version it was generated from, so a man page
// source that names a different release is a stale artifact.
//
// Version only, never ReleaseHash — cmt re-stamps the hash from HEAD on every
// build, so asserting it would fail on every commit by design.
func TestManPageSourcesCarryTheCurrentVersion(t *testing.T) {
	sources, err := filepath.Glob(filepath.Join("..", "..", "*.1.md"))
	if err != nil || len(sources) == 0 {
		t.Skipf("no man page sources to check: %v", err)
	}
	for _, path := range sources {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		first := strings.SplitN(string(data), "\n", 2)[0]
		if !strings.Contains(first, knowledge.Version) {
			t.Errorf("%s is stale: its header says %q but the current version is %s; run make kb-topics-help && make man",
				filepath.Base(path), first, knowledge.Version)
		}
	}
}
