package knowledge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DR-0057: a record is identified by SCOPE/DR-NNNN, where SCOPE is a project
// name or "workspace".

func TestParseRef_Forms(t *testing.T) {
	cases := []struct {
		in   string
		want Ref
	}{
		{"harvey/DR-0004", Ref{"harvey", "0004"}},
		{"harvey/0004", Ref{"harvey", "0004"}},
		{"harvey/dr-0004", Ref{"harvey", "0004"}},
		{"harvey/4", Ref{"harvey", "0004"}},
		{"workspace/DR-0003", Ref{"workspace", "0003"}},
		{"CL-js/DR-0002", Ref{"CL-js", "0002"}},
		{"DR-0004", Ref{"", "0004"}},
		{"dr-0004", Ref{"", "0004"}},
		{"0004", Ref{"", "0004"}},
		{"4", Ref{"", "0004"}},
		{"12345", Ref{"", "12345"}},
		{"  harvey/0004  ", Ref{"harvey", "0004"}},
	}
	for _, c := range cases {
		got, err := ParseRef(c.in)
		if err != nil {
			t.Errorf("ParseRef(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseRef(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestParseRef_BadInputIsErrInvalid(t *testing.T) {
	for _, in := range []string{"", "   ", "harvey/", "/0004", "harvey/DR-", "harvey/abc", "DR-abc", "a/b/0004", "harvey/DR-0004/x", "DR-00 04"} {
		_, err := ParseRef(in)
		if err == nil {
			t.Errorf("ParseRef(%q) succeeded, want an error", in)
			continue
		}
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseRef(%q) error %v does not match ErrInvalid", in, err)
		}
	}
}

func TestRefString(t *testing.T) {
	if got := (Ref{"harvey", "0004"}).String(); got != "harvey/DR-0004" {
		t.Errorf("String = %q, want harvey/DR-0004", got)
	}
	if got := (Ref{"", "0004"}).String(); got != "DR-0004" {
		t.Errorf("String = %q, want DR-0004", got)
	}
}

// refKB opens a knowledge base in a workspace named "Laboratory" holding
// DR-0001 for each named project and for the workspace tier, plus DR-0002 for
// the first project only.
func refKB(t *testing.T, projects ...string) *KnowledgeBase {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Laboratory")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	kb, err := Open(DefaultPath(root))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { kb.Close() })
	for i, p := range projects {
		pid, err := kb.AddProject(p, p+" project")
		if err != nil {
			t.Fatalf("AddProject %s: %v", p, err)
		}
		r := newTestRecord("0001", p+" one", "2026-08-01")
		r.ProjectID = pid
		r.Path = p + "/0001.md"
		if _, err := kb.AddRecord(r); err != nil {
			t.Fatalf("AddRecord: %v", err)
		}
		if i == 0 {
			r2 := newTestRecord("0002", p+" two", "2026-08-02")
			r2.ProjectID = pid
			r2.Path = p + "/0002.md"
			if _, err := kb.AddRecord(r2); err != nil {
				t.Fatalf("AddRecord: %v", err)
			}
		}
	}
	w := newTestRecord("0001", "workspace one", "2026-08-03")
	w.Scope = "workspace"
	w.Path = "agents/decisions/0001.md"
	if _, err := kb.AddRecord(w); err != nil {
		t.Fatalf("AddRecord workspace: %v", err)
	}
	return kb
}

func TestResolveRef_QualifiedReachesEachTier(t *testing.T) {
	kb := refKB(t, "clasm", "cold")
	for _, c := range []struct{ ref, title string }{
		{"clasm/DR-0001", "clasm one"},
		{"cold/0001", "cold one"},
		{"workspace/DR-0001", "workspace one"},
	} {
		ref, _ := ParseRef(c.ref)
		rec, err := kb.ResolveRef(ref, "")
		if err != nil {
			t.Errorf("ResolveRef(%q): %v", c.ref, err)
			continue
		}
		if rec.Title != c.title {
			t.Errorf("ResolveRef(%q) = %q, want %q", c.ref, rec.Title, c.title)
		}
	}
}

func TestResolveRef_WorkspaceBasenameIsAnAlias(t *testing.T) {
	kb := refKB(t, "clasm")
	for _, in := range []string{"laboratory/DR-0001", "Laboratory/0001", "LABORATORY/1"} {
		ref, _ := ParseRef(in)
		rec, err := kb.ResolveRef(ref, "")
		if err != nil {
			t.Errorf("ResolveRef(%q): %v", in, err)
			continue
		}
		if rec.Scope != "workspace" {
			t.Errorf("ResolveRef(%q) scope = %q, want the workspace tier", in, rec.Scope)
		}
	}
}

func TestResolveRef_AProjectBeatsTheBasenameAlias(t *testing.T) {
	kb := refKB(t, "Laboratory")
	rec, err := kb.ResolveRef(Ref{"Laboratory", "0001"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Scope != "project" || rec.Title != "Laboratory one" {
		t.Errorf("got scope %q title %q, want the project record, not the workspace tier", rec.Scope, rec.Title)
	}
	rec, err = kb.ResolveRef(Ref{"workspace", "0001"}, "")
	if err != nil || rec.Scope != "workspace" {
		t.Errorf("workspace/DR-0001 = %+v, %v; want the workspace tier", rec, err)
	}
}

func TestResolveRef_BareIdUniqueResolves(t *testing.T) {
	kb := refKB(t, "clasm", "cold")
	rec, err := kb.ResolveRef(Ref{"", "0002"}, "")
	if err != nil || rec.Title != "clasm two" {
		t.Errorf("bare DR-0002 = %+v, %v; want clasm's", rec, err)
	}
}

func TestResolveRef_BareIdAmbiguousIsATypedErrorWithCandidates(t *testing.T) {
	kb := refKB(t, "clasm", "cold")
	_, err := kb.ResolveRef(Ref{"", "0001"}, "")
	var amb *AmbiguousRefError
	if !errors.As(err, &amb) {
		t.Fatalf("error = %v, want *AmbiguousRefError", err)
	}
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("an ambiguous reference should match ErrInvalid: %v", err)
	}
	got := map[string]bool{}
	for _, c := range amb.Candidates {
		got[c.String()] = true
	}
	for _, want := range []string{"clasm/DR-0001", "cold/DR-0001", "workspace/DR-0001"} {
		if !got[want] {
			t.Errorf("candidates = %v, want %s among them", amb.Candidates, want)
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to offer %s", err, want)
		}
	}
}

func TestResolveRef_UnknownScopeIsNotFound(t *testing.T) {
	kb := refKB(t, "clasm")
	_, err := kb.ResolveRef(Ref{"nosuch", "0001"}, "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("error = %q, want it to name the scope", err)
	}
}

func TestResolveRef_QualifiedButAbsentIsNotFound(t *testing.T) {
	kb := refKB(t, "clasm", "cold")
	_, err := kb.ResolveRef(Ref{"cold", "0002"}, "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "cold") {
		t.Errorf("error = %q, want it to name the project", err)
	}
}

func TestResolveRef_ExplicitWorkspaceNameOverridesTheDatabasesOwn(t *testing.T) {
	kb := refKB(t, "clasm")
	// A caller indexing a workspace other than the one the database sits in
	// passes that workspace's name; nothing is stamped with it here.
	if _, err := kb.ResolveRef(Ref{"clasm", "0001"}, "Elsewhere"); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound for a workspace holding no such record", err)
	}
}

func TestProjectNamedWorkspaceIsRefused(t *testing.T) {
	kb := openTestKB(t)
	for _, name := range []string{"workspace", "Workspace", "  WORKSPACE "} {
		if _, err := kb.AddProject(name, "d"); !errors.Is(err, ErrInvalid) {
			t.Errorf("AddProject(%q) error = %v, want ErrInvalid", name, err)
		}
	}
	if _, err := kb.AddProject("harvey", "d"); err != nil {
		t.Fatal(err)
	}
	if err := kb.RenameProject("harvey", "workspace"); !errors.Is(err, ErrInvalid) {
		t.Errorf("RenameProject to workspace error = %v, want ErrInvalid", err)
	}
	if _, err := kb.AddProject("workspace-tools", "d"); err != nil {
		t.Errorf("a name that merely starts with workspace is fine: %v", err)
	}
}
