package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// `kb search SCOPE/[KIND] TERM` (knowledge DR-0073, DR-0074): a first argument
// of the form SCOPE/ or SCOPE/KIND names where to look, and anything else is a
// term as before, so `kb search harvey memory` keeps its meaning.

type scopedWorld struct {
	kb          *knowledge.KnowledgeBase
	alpha, beta int64
	recAlpha    int64
}

func newScopedWorld(t *testing.T) *scopedWorld {
	t.Helper()
	kb := openTestKB(t)
	w := &scopedWorld{kb: kb}
	var err error
	if w.alpha, err = kb.AddProject("alpha", "a cache service"); err != nil {
		t.Fatal(err)
	}
	if w.beta, err = kb.AddProject("beta", "other"); err != nil {
		t.Fatal(err)
	}
	obs, _ := kb.AddObservation(w.alpha, "finding", "cache invalidation is the hard part")
	if _, err := kb.AddObservation(w.beta, "note", "cache warmup takes a minute"); err != nil {
		t.Fatal(err)
	}
	if w.recAlpha, err = kb.AddRecord(knowledge.Record{RecordID: "0001", ProjectID: w.alpha, Scope: "project", Path: "a/0001.md",
		Title: "Alpha cache layer", Date: "2026-10-01", Status: "accepted", Kind: "decision", Body: "the cache layer"}); err != nil {
		t.Fatal(err)
	}
	if _, err := kb.AddRecord(knowledge.Record{RecordID: "0002", Scope: "workspace", Path: "w/0002.md",
		Title: "Workspace cache rule", Date: "2026-10-03", Status: "accepted", Kind: "decision", Body: "a cache rule"}); err != nil {
		t.Fatal(err)
	}
	c, _ := kb.AddConcept("caching", "the cache concept")
	if err := kb.LinkObservationConcept(obs, c); err != nil {
		t.Fatal(err)
	}
	rb, _ := kb.AddRecord(knowledge.Record{RecordID: "0003", ProjectID: w.beta, Scope: "project", Path: "b/0003.md",
		Title: "Beta policy", Date: "2026-10-02", Status: "accepted", Kind: "decision", Body: "cache eviction"})
	if err := kb.LinkRecordConcept(rb, c); err != nil {
		t.Fatal(err)
	}
	return w
}

func (w *scopedWorld) search(t *testing.T, jsonOut bool, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := cmdSearch(w.kb, nil, jsonOut, args, &out)
	return out.String(), err
}

func TestSearchScope_ProjectGroupsByKindWithCountsAndQualifiedRefs(t *testing.T) {
	w := newScopedWorld(t)
	out, err := w.search(t, false, "alpha/", "cache")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"alpha: 3 matches", "records (1)", "observations (1)", "concepts (1)",
		"alpha/DR-0001", "Alpha cache layer", "[alpha, beta]"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "beta/DR-0003") || strings.Contains(out, "workspace/DR-0002") {
		t.Errorf("another scope's record is in alpha's results:\n%s", out)
	}
}

func TestSearchScope_KindNarrows(t *testing.T) {
	w := newScopedWorld(t)
	out, err := w.search(t, false, "alpha/records", "cache")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "records (1)") || strings.Contains(out, "observations") || strings.Contains(out, "concepts") {
		t.Errorf("alpha/records should show records only:\n%s", out)
	}
}

func TestSearchScope_LeadingSlashIsEverywhere(t *testing.T) {
	w := newScopedWorld(t)
	out, err := w.search(t, false, "/records", "cache")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"alpha/DR-0001", "beta/DR-0003", "workspace/DR-0002", "records (3)"} {
		if !strings.Contains(out, want) {
			t.Errorf("/records lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "observations") {
		t.Errorf("/records showed observations:\n%s", out)
	}
	all, err := w.search(t, false, "/", "cache")
	if err != nil || !strings.Contains(all, "observations (2)") {
		t.Errorf("/ is the whole database: err %v\n%s", err, all)
	}
}

func TestSearchScope_WorkspaceIsTheTierAndTheDirectoryNameIsAnAlias(t *testing.T) {
	w := newScopedWorld(t)
	out, err := w.search(t, false, "workspace/", "cache")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "workspace/DR-0002") || strings.Contains(out, "alpha/DR-0001") {
		t.Errorf("workspace/ should be the tier only:\n%s", out)
	}
	base := filepath.Base(recordRoot(w.kb, recordFlags{}))
	alias, err := w.search(t, false, base+"/", "cache")
	if err != nil {
		t.Fatalf("the directory name %q as an alias: %v", base, err)
	}
	if alias != out {
		t.Errorf("alias %q printed something else:\n%s", base, alias)
	}
}

func TestSearchScope_AProjectWinsOverTheDirectoryName(t *testing.T) {
	w := newScopedWorld(t)
	base := filepath.Base(recordRoot(w.kb, recordFlags{}))
	if _, err := w.kb.AddProject(base, "named like the directory"); err != nil {
		t.Fatal(err)
	}
	out, err := w.search(t, false, base+"/records", "cache")
	if err == nil {
		t.Fatalf("the project %q has no records matching, so the search finds nothing; got:\n%s", base, out)
	}
	if exitCodeFor(err).Code != 1 || strings.Contains(out, "workspace/DR-0002") {
		t.Errorf("project should win: code %d\n%s", exitCodeFor(err).Code, out)
	}
}

func TestSearchScope_AWordThatIsNotAScopeStaysATerm(t *testing.T) {
	w := newScopedWorld(t)
	// "alpha cache" is the words alpha and cache, as it always was: the legacy output.
	out, err := w.search(t, false, "alpha", "cache")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "matches") || !strings.HasPrefix(out, "[") {
		t.Errorf("a bare first word should be a term and print the flat list:\n%s", out)
	}
}

func TestSearchScope_UsageErrorsAreExit2(t *testing.T) {
	w := newScopedWorld(t)
	for name, args := range map[string][]string{
		"unknown project":      {"nosuch/", "cache"},
		"unknown kind":         {"alpha/sources", "cache"},
		"scope with no query":  {"alpha/records"},
		"slash alone":          {"/"},
		"a path-like term":     {"cmd/kb"},
		"empty kind after dir": {"alpha/records/extra", "cache"},
	} {
		out, err := w.search(t, false, args...)
		if err == nil {
			t.Errorf("%s: no error, printed %q", name, out)
			continue
		}
		if code := exitCodeFor(err).Code; code != 2 {
			t.Errorf("%s: exit %d (%v), want 2", name, code, err)
		}
	}
}

func TestSearchScope_DashDashMakesAPathLikeAString(t *testing.T) {
	w := newScopedWorld(t)
	_, err := w.search(t, false, "--", "cmd/kb")
	if err == nil || exitCodeFor(err).Code != 1 {
		t.Errorf("`-- cmd/kb` is a term that finds nothing, exit 1; got %v", err)
	}
}

func TestSearchScope_ImpossibleScopeIsExit1WithAReason(t *testing.T) {
	w := newScopedWorld(t)
	for _, args := range [][]string{{"workspace/observations", "cache"}, {"alpha/projects", "cache"}, {"workspace/projects", "cache"}} {
		out, err := w.search(t, false, args...)
		if err == nil || exitCodeFor(err).Code != 1 {
			t.Errorf("%v: err %v, want exit 1", args, err)
			continue
		}
		if out != "" || !strings.Contains(err.Error(), "no ") {
			t.Errorf("%v: stdout %q, error %q; want an empty stdout and a reason", args, out, err)
		}
	}
}

func TestSearchScope_NothingFoundIsExit1(t *testing.T) {
	w := newScopedWorld(t)
	_, err := w.search(t, false, "alpha/", "zzzznotthere")
	if err == nil || exitCodeFor(err).Code != 1 {
		t.Errorf("err = %v, want exit 1", err)
	}
}

func TestSearchScope_TheCapIsStated(t *testing.T) {
	w := newScopedWorld(t)
	for i := 0; i < 55; i++ {
		if _, err := w.kb.AddObservation(w.alpha, "note", fmt.Sprintf("many cache notes %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	out, err := w.search(t, false, "alpha/observations", "cache")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "showing 50 of 56") {
		t.Errorf("the cut should be stated:\n%s", out)
	}
}

func TestSearchScope_JSONIsOneObject(t *testing.T) {
	w := newScopedWorld(t)
	out, err := w.search(t, true, "alpha/", "cache")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Scope   string         `json:"scope"`
		Kinds   []string       `json:"kinds"`
		Term    string         `json:"term"`
		Total   int            `json:"total"`
		Counts  map[string]int `json:"counts"`
		Results []struct {
			Kind     string   `json:"kind"`
			Ref      string   `json:"ref"`
			Label    string   `json:"label"`
			Snippet  string   `json:"snippet"`
			Type     string   `json:"source_type"`
			ID       int64    `json:"source_id"`
			Projects []string `json:"projects"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not one JSON object: %v\n%s", err, out)
	}
	if doc.Scope != "alpha" || doc.Term != "cache" || doc.Total != 3 || doc.Counts["records"] != 1 {
		t.Errorf("doc = %+v", doc)
	}
	var sawRef, sawSpan bool
	for _, r := range doc.Results {
		if r.Type == "record" && r.Ref == "alpha/DR-0001" {
			sawRef = true
		}
		if r.Type == "concept" && strings.Join(r.Projects, ",") == "alpha,beta" {
			sawSpan = true
		}
	}
	if !sawRef || !sawSpan {
		t.Errorf("ref %v, span %v:\n%s", sawRef, sawSpan, out)
	}
}

func TestSearchScope_ErrNoMembersIsClassifiedNegative(t *testing.T) {
	_, err := openTestKB(t).SearchScoped("x", knowledge.SearchScope{Workspace: true, Kinds: []string{"observations"}})
	if err == nil {
		t.Fatal("expected ErrNoMembers")
	}
	if c := exitCodeFor(err); c.Code != 1 {
		t.Errorf("exit %d (%s), want 1: a classified negative, not 70", c.Code, c.Name)
	}
}
