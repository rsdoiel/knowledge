package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// DR-0057 as kb applies it: the lookup is the library's (ref_test.go in the
// knowledge package); these tests cover what kb adds, the flags and the exit
// codes.

// namedWorkspaceKB opens a database in a workspace whose directory has the
// given name, so the basename alias can be tested.
func namedWorkspaceKB(t *testing.T, name string) (*knowledge.KnowledgeBase, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	kb, err := knowledge.Open(knowledge.DefaultPath(root))
	if err != nil {
		t.Fatalf("knowledge.Open: %v", err)
	}
	t.Cleanup(func() { kb.Close() })
	return kb, root
}

// seedTiers writes and ingests DR-0001 for each named project and for the
// workspace tier, plus DR-0002 for the first project only.
func seedTiers(t *testing.T, kb *knowledge.KnowledgeBase, root string, projects ...string) {
	t.Helper()
	for i, p := range projects {
		dir := filepath.Join(root, "agents", "projects", p, "decisions")
		testRecord{ID: "0001", Project: p, Title: p + " one"}.write(t, dir)
		if i == 0 {
			testRecord{ID: "0002", Project: p, Title: p + " two"}.write(t, dir)
		}
		runIngest(t, kb, dir)
	}
	dir := filepath.Join(root, "agents", "decisions")
	testRecord{ID: "0001", Project: "", Title: "workspace one"}.write(t, dir)
	runIngest(t, kb, dir)
}

func TestResolveRef_QualifiedReachesEachTier(t *testing.T) {
	kb, root := namedWorkspaceKB(t, "Laboratory")
	seedTiers(t, kb, root, "clasm", "cold")

	for _, c := range []struct{ ref, title string }{
		{"clasm/DR-0001", "clasm one"},
		{"cold/0001", "cold one"},
		{"workspace/DR-0001", "workspace one"},
	} {
		rec, err := resolveRecord(kb, c.ref, recordFlags{root: root})
		if err != nil {
			t.Errorf("resolveRecord(%q): %v", c.ref, err)
			continue
		}
		if rec.Title != c.title {
			t.Errorf("resolveRecord(%q) = %q, want %q", c.ref, rec.Title, c.title)
		}
	}
}

func TestResolveRef_WorkspaceBasenameIsAnAlias(t *testing.T) {
	kb, root := namedWorkspaceKB(t, "Laboratory")
	seedTiers(t, kb, root, "clasm")
	for _, ref := range []string{"laboratory/DR-0001", "Laboratory/0001", "LABORATORY/1"} {
		rec, err := resolveRecord(kb, ref, recordFlags{root: root})
		if err != nil {
			t.Errorf("resolveRecord(%q): %v", ref, err)
			continue
		}
		if rec.Scope != "workspace" {
			t.Errorf("resolveRecord(%q) scope = %q, want the workspace tier", ref, rec.Scope)
		}
	}
}

func TestResolveRef_AProjectBeatsTheBasenameAlias(t *testing.T) {
	kb, root := namedWorkspaceKB(t, "Laboratory")
	seedTiers(t, kb, root, "Laboratory")
	rec, err := resolveRecord(kb, "Laboratory/DR-0001", recordFlags{root: root})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Scope != "project" || rec.Title != "Laboratory one" {
		t.Errorf("got scope %q title %q, want the project record, not the workspace tier", rec.Scope, rec.Title)
	}
	// The workspace tier stays reachable under its canonical name.
	rec, err = resolveRecord(kb, "workspace/DR-0001", recordFlags{root: root})
	if err != nil || rec.Scope != "workspace" {
		t.Errorf("workspace/DR-0001 = %+v, %v; want the workspace tier", rec, err)
	}
}

func TestResolveRef_BareIdUniqueResolves(t *testing.T) {
	kb, root := namedWorkspaceKB(t, "Laboratory")
	seedTiers(t, kb, root, "clasm", "cold")
	rec, err := resolveRecord(kb, "DR-0002", recordFlags{root: root})
	if err != nil || rec.Title != "clasm two" {
		t.Errorf("bare DR-0002 = %+v, %v; want clasm's", rec, err)
	}
}

func TestResolveRef_BareIdAmbiguousListsQualifiedCandidates(t *testing.T) {
	kb, root := namedWorkspaceKB(t, "Laboratory")
	seedTiers(t, kb, root, "clasm", "cold")
	_, err := resolveRecord(kb, "0001", recordFlags{root: root})
	if err == nil {
		t.Fatal("an ambiguous bare id resolved silently")
	}
	if !isUsageError(err) || exitCodeFor(err).Code != 2 {
		t.Errorf("error %v: want a usage error, exit 2", err)
	}
	for _, want := range []string{"clasm/DR-0001", "cold/DR-0001", "workspace/DR-0001"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to offer %s", err, want)
		}
	}
}

func TestResolveRef_UnknownScopeIsNotFound(t *testing.T) {
	kb, root := namedWorkspaceKB(t, "Laboratory")
	seedTiers(t, kb, root, "clasm")
	_, err := resolveRecord(kb, "nosuch/DR-0001", recordFlags{root: root})
	if err == nil || exitCodeFor(err).Code != 1 {
		t.Fatalf("error = %v, want not found (exit 1)", err)
	}
	if !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("error = %q, want it to name the scope", err)
	}
}

func TestResolveRef_QualifiedButAbsentIsNotFound(t *testing.T) {
	kb, root := namedWorkspaceKB(t, "Laboratory")
	seedTiers(t, kb, root, "clasm", "cold")
	_, err := resolveRecord(kb, "cold/DR-0002", recordFlags{root: root})
	if err == nil || exitCodeFor(err).Code != 1 {
		t.Fatalf("error = %v, want not found (exit 1)", err)
	}
	if !strings.Contains(err.Error(), "cold") {
		t.Errorf("error = %q, want it to name the project", err)
	}
}

func TestResolveRef_QualifiedRefAndFlagMustAgree(t *testing.T) {
	kb, root := namedWorkspaceKB(t, "Laboratory")
	seedTiers(t, kb, root, "clasm", "cold")

	if _, err := resolveRecord(kb, "cold/DR-0001", recordFlags{root: root, project: "cold"}); err != nil {
		t.Errorf("agreeing --project: %v", err)
	}
	_, err := resolveRecord(kb, "cold/DR-0001", recordFlags{root: root, project: "clasm"})
	if err == nil || !isUsageError(err) {
		t.Errorf("conflicting --project: error = %v, want a usage error", err)
	}
	_, err = resolveRecord(kb, "cold/DR-0001", recordFlags{root: root, workspace: true})
	if err == nil || !isUsageError(err) {
		t.Errorf("conflicting --workspace: error = %v, want a usage error", err)
	}
	if rec, err := resolveRecord(kb, "workspace/DR-0001", recordFlags{root: root, workspace: true}); err != nil || rec.Scope != "workspace" {
		t.Errorf("agreeing --workspace = %+v, %v", rec, err)
	}
}

func TestResolveRef_MalformedRefIsAUsageError(t *testing.T) {
	kb, root := namedWorkspaceKB(t, "Laboratory")
	seedTiers(t, kb, root, "clasm")
	_, err := resolveRecord(kb, "clasm/DR-xyz", recordFlags{root: root})
	if err == nil || !isUsageError(err) {
		t.Errorf("error = %v, want a usage error", err)
	}
}

func TestCmdRecord_ShowTakesAQualifiedRef(t *testing.T) {
	kb, root := namedWorkspaceKB(t, "Laboratory")
	seedTiers(t, kb, root, "clasm", "cold")
	var out bytes.Buffer
	if err := cmdRecord(kb, nil, false, []string{"show", "cold/DR-0001", "--root", root}, &out); err != nil {
		t.Fatalf("show cold/DR-0001: %v", err)
	}
	if !strings.Contains(out.String(), "cold one") || strings.Contains(out.String(), "clasm one") {
		t.Errorf("show printed %q, want only the cold record", out.String())
	}
}

// A project named "workspace" could never be reached by a reference, so the
// name is refused at the command line like any other bad value (exit 2).
func TestMainRun_ProjectNamedWorkspaceIsAUsageError(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"init", root}, &out, &errOut); code != 0 {
		t.Fatalf("init: exit %d; %s", code, errOut.String())
	}
	for _, args := range [][]string{
		{"project", "add", "workspace", "d"},
		{"project", "add", "Workspace", "d"},
	} {
		out.Reset()
		errOut.Reset()
		if code := mainRun(args, &out, &errOut); code != 2 {
			t.Errorf("kb %v: exit %d, want 2; stderr=%s", args, code, errOut.String())
		}
		if !strings.Contains(errOut.String(), "reserved") {
			t.Errorf("kb %v: stderr = %q, want it to say the name is reserved", args, errOut.String())
		}
	}
}

// ─── scope (DR-0058) ──────────────────────────────────────────────────────────

// scopeFixture builds a workspace "Laboratory" with DR-0001 in harvey, clasm
// and the workspace tier, and DR-0002 in harvey, with the directories a
// working directory can sit in.
func scopeFixture(t *testing.T) (*knowledge.KnowledgeBase, string) {
	t.Helper()
	kb, root := namedWorkspaceKB(t, "Laboratory")
	seedTiers(t, kb, root, "harvey", "clasm")
	for _, d := range []string{"harvey/cmd", "clasm", "tmp/scratch"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("KB_PROJECT", "")
	return kb, root
}

func listTitles(t *testing.T, kb *knowledge.KnowledgeBase, args ...string) []string {
	t.Helper()
	var entries []recordListEntry
	runRecordJSON(t, kb, &entries, append([]string{"list"}, args...)...)
	var out []string
	for _, e := range entries {
		out = append(out, e.Title)
	}
	return out
}

func sameSet(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]int{}
	for _, g := range got {
		seen[g]++
	}
	for _, w := range want {
		seen[w]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

func TestRecordList_FromTheRootIsTheWholeWorkspace(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(root)
	got := listTitles(t, kb)
	if !sameSet(got, "harvey one", "harvey two", "clasm one", "workspace one") {
		t.Errorf("list = %v, want every record", got)
	}
}

func TestRecordList_PositionalScopes(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(root)
	for _, c := range []struct {
		args []string
		want []string
	}{
		{[]string{"harvey"}, []string{"harvey one", "harvey two"}},
		{[]string{"clasm"}, []string{"clasm one"}},
		{[]string{"harvey", "clasm"}, []string{"harvey one", "harvey two", "clasm one"}},
		{[]string{"workspace"}, []string{"workspace one"}},
		{[]string{"laboratory"}, []string{"workspace one"}},
		{[]string{"harvey", "workspace"}, []string{"harvey one", "harvey two", "workspace one"}},
		{[]string{"harvey", "harvey"}, []string{"harvey one", "harvey two"}},
	} {
		if got := listTitles(t, kb, c.args...); !sameSet(got, c.want...) {
			t.Errorf("list %v = %v, want %v", c.args, got, c.want)
		}
	}
}

func TestRecordList_StaysOldestFirstAcrossScopes(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(root)
	var entries []recordListEntry
	runRecordJSON(t, kb, &entries, "list", "clasm", "harvey")
	for i := 1; i < len(entries); i++ {
		a, b := entries[i-1], entries[i]
		if a.Date > b.Date || (a.Date == b.Date && a.RecordID > b.RecordID) {
			t.Errorf("entries out of order: %s %s before %s %s", a.Date, a.RecordID, b.Date, b.RecordID)
		}
	}
}

func TestRecordList_UnknownScopeIsNotFound(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(root)
	var out bytes.Buffer
	err := cmdRecord(kb, nil, false, []string{"list", "harvey", "nosuch"}, &out)
	if err == nil || exitCodeFor(err).Code != 1 || !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("error = %v, want not found (exit 1) naming nosuch", err)
	}
}

func TestRecordList_ConflictingScopeSourcesAreUsageErrors(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(root)
	for _, args := range [][]string{
		{"list", "harvey", "--project", "clasm"},
		{"list", "harvey", "--workspace"},
		{"list", "harvey", "--all"},
		{"list", "--all", "--project", "harvey"},
		{"list", "--all", "--workspace"},
	} {
		var out bytes.Buffer
		err := cmdRecord(kb, nil, false, args, &out)
		if err == nil || !isUsageError(err) {
			t.Errorf("record %v: error = %v, want a usage error", args, err)
		}
	}
}

func TestRecordList_InferredFromTheWorkingDirectory(t *testing.T) {
	kb, root := scopeFixture(t)
	for _, dir := range []string{"agents/projects/harvey", "agents/projects/harvey/decisions", "harvey", "harvey/cmd"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(filepath.Join(root, dir))
		if got := listTitles(t, kb); !sameSet(got, "harvey one", "harvey two") {
			t.Errorf("list from %s = %v, want harvey's records", dir, got)
		}
	}
}

func TestRecordList_AllWidensAProjectDirectory(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(filepath.Join(root, "harvey", "cmd"))
	if got := listTitles(t, kb, "--all"); len(got) != 4 {
		t.Errorf("list --all = %v, want every record", got)
	}
}

func TestRecordList_ExplicitScopeBeatsTheDirectory(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(filepath.Join(root, "harvey", "cmd"))
	if got := listTitles(t, kb, "clasm"); !sameSet(got, "clasm one") {
		t.Errorf("list clasm from harvey/ = %v, want clasm's", got)
	}
}

func TestRecordList_DirectoryWithoutAProjectIsTheWholeWorkspace(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(filepath.Join(root, "tmp", "scratch"))
	if got := listTitles(t, kb); len(got) != 4 {
		t.Errorf("list from a non-project directory = %v, want every record", got)
	}
}

func TestRecordList_KBProjectIsTheFallback(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(filepath.Join(root, "tmp", "scratch"))
	t.Setenv("KB_PROJECT", "clasm")
	if got := listTitles(t, kb); !sameSet(got, "clasm one") {
		t.Errorf("list with KB_PROJECT=clasm = %v, want clasm's", got)
	}
	// The directory is the closer evidence, so it beats the environment.
	t.Chdir(filepath.Join(root, "harvey"))
	if got := listTitles(t, kb); !sameSet(got, "harvey one", "harvey two") {
		t.Errorf("list from harvey/ with KB_PROJECT=clasm = %v, want harvey's", got)
	}
	// --all and an explicit scope beat both.
	if got := listTitles(t, kb, "--all"); len(got) != 4 {
		t.Errorf("list --all = %v, want every record", got)
	}
}

func TestRecordList_UnknownKBProjectIsNotFound(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(filepath.Join(root, "tmp", "scratch"))
	t.Setenv("KB_PROJECT", "nosuch")
	var out bytes.Buffer
	err := cmdRecord(kb, nil, false, []string{"list"}, &out)
	if err == nil || exitCodeFor(err).Code != 1 || !strings.Contains(err.Error(), "KB_PROJECT") {
		t.Errorf("error = %v, want not found (exit 1) naming KB_PROJECT", err)
	}
}

func TestRecordList_FlagsStillWorkAsAliases(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(root)
	if got := listTitles(t, kb, "--project", "clasm"); !sameSet(got, "clasm one") {
		t.Errorf("list --project clasm = %v", got)
	}
	if got := listTitles(t, kb, "--workspace"); !sameSet(got, "workspace one") {
		t.Errorf("list --workspace = %v", got)
	}
}

func TestResolveRecord_BareIdUsesTheInferredProject(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(filepath.Join(root, "harvey", "cmd"))
	rec, err := resolveRecord(kb, "0001", recordFlags{})
	if err != nil || rec.Title != "harvey one" {
		t.Errorf("bare 0001 from harvey/ = %+v, %v; want harvey's, though 0001 is ambiguous workspace-wide", rec, err)
	}
	// An id the project lacks is not found there; it does not fall through.
	_, err = resolveRecord(kb, "0099", recordFlags{})
	if err == nil || exitCodeFor(err).Code != 1 {
		t.Errorf("bare 0099 error = %v, want not found", err)
	}
}

func TestResolveRecord_QualifiedRefIgnoresTheDirectory(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(filepath.Join(root, "harvey", "cmd"))
	rec, err := resolveRecord(kb, "clasm/0001", recordFlags{})
	if err != nil || rec.Title != "clasm one" {
		t.Errorf("clasm/0001 from harvey/ = %+v, %v", rec, err)
	}
}

func TestResolveRecord_WritesNeedAScope(t *testing.T) {
	kb, root := scopeFixture(t)
	t.Chdir(root)
	// Unambiguous, but a write never runs on a bare id with no scope.
	_, err := resolveRecordForWrite(kb, "0002", recordFlags{})
	if err == nil || !isUsageError(err) || !strings.Contains(err.Error(), "harvey/DR-0002") {
		t.Errorf("error = %v, want a usage error offering harvey/DR-0002", err)
	}
	if rec, err := resolveRecordForWrite(kb, "harvey/0002", recordFlags{}); err != nil || rec.Title != "harvey two" {
		t.Errorf("qualified write = %+v, %v", rec, err)
	}
	if rec, err := resolveRecordForWrite(kb, "0002", recordFlags{project: "harvey"}); err != nil || rec.Title != "harvey two" {
		t.Errorf("write with --project = %+v, %v", rec, err)
	}
	t.Chdir(filepath.Join(root, "harvey", "cmd"))
	if rec, err := resolveRecordForWrite(kb, "0002", recordFlags{}); err != nil || rec.Title != "harvey two" {
		t.Errorf("bare write inside the project = %+v, %v", rec, err)
	}
}
