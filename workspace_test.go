package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// touchMarker creates root/agents/name, making the directories it needs.
func touchMarker(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
}

func nested(t *testing.T, root string, parts ...string) string {
	t.Helper()
	dir := filepath.Join(append([]string{root}, parts...)...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	return dir
}

func TestFindWorkspace_InTheDirectoryItself(t *testing.T) {
	root := t.TempDir()
	touchMarker(t, root, "knowledge.db")
	got, marker, ok := FindWorkspace(root)
	if !ok || got != root || marker != MarkerDB {
		t.Errorf("FindWorkspace(root) = %q, %q, %v; want %q, %q, true", got, marker, ok, root, MarkerDB)
	}
}

func TestFindWorkspace_WalksUpFromANestedDirectory(t *testing.T) {
	root := t.TempDir()
	touchMarker(t, root, "knowledge.db")
	deep := nested(t, root, "harvey", "cmd", "x")
	got, marker, ok := FindWorkspace(deep)
	if !ok || got != root || marker != MarkerDB {
		t.Errorf("FindWorkspace(deep) = %q, %q, %v; want %q, %q, true", got, marker, ok, root, MarkerDB)
	}
}

func TestFindWorkspace_JSONLAloneIsAWorkspace(t *testing.T) {
	root := t.TempDir()
	touchMarker(t, root, "knowledge.jsonl")
	got, marker, ok := FindWorkspace(nested(t, root, "a"))
	if !ok || got != root || marker != MarkerJSONL {
		t.Errorf("FindWorkspace = %q, %q, %v; want %q, %q, true", got, marker, ok, root, MarkerJSONL)
	}
}

func TestFindWorkspace_DatabaseBeatsJSONLInTheSameDirectory(t *testing.T) {
	root := t.TempDir()
	touchMarker(t, root, "knowledge.jsonl")
	touchMarker(t, root, "knowledge.db")
	_, marker, ok := FindWorkspace(root)
	if !ok || marker != MarkerDB {
		t.Errorf("marker = %q, ok = %v; want %q", marker, ok, MarkerDB)
	}
}

func TestFindWorkspace_NearestAncestorWins(t *testing.T) {
	outer := t.TempDir()
	touchMarker(t, outer, "knowledge.db")
	inner := nested(t, outer, "inner")
	touchMarker(t, inner, "knowledge.jsonl")
	got, marker, ok := FindWorkspace(nested(t, inner, "x"))
	if !ok || got != inner || marker != MarkerJSONL {
		t.Errorf("FindWorkspace = %q, %q, %v; want the inner %q, %q", got, marker, ok, inner, MarkerJSONL)
	}
}

func TestFindWorkspace_NoneFound(t *testing.T) {
	dir := t.TempDir()
	if got, marker, ok := FindWorkspace(dir); ok {
		t.Errorf("FindWorkspace(empty) = %q, %q, true; want not found", got, marker)
	}
}

func TestFindWorkspace_AnAgentsDirectoryAloneIsNotAWorkspace(t *testing.T) {
	root := t.TempDir()
	nested(t, root, "agents")
	if got, _, ok := FindWorkspace(root); ok {
		t.Errorf("found %q from an empty agents directory", got)
	}
}

func TestFindWorkspace_AMarkerThatIsADirectoryIsIgnored(t *testing.T) {
	root := t.TempDir()
	nested(t, root, "agents", "knowledge.db")
	if got, _, ok := FindWorkspace(root); ok {
		t.Errorf("found %q although knowledge.db is a directory", got)
	}
}

func TestFindWorkspace_RelativeDirectoryIsMadeAbsolute(t *testing.T) {
	root := t.TempDir()
	touchMarker(t, root, "knowledge.db")
	t.Chdir(nested(t, root, "a", "b"))
	got, _, ok := FindWorkspace(".")
	want, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if !ok || gotResolved != want {
		t.Errorf("FindWorkspace(\".\") = %q, %v; want %q", got, ok, want)
	}
}

// KB_CEILING_DIRECTORIES (DR-0058): directories the search must not examine,
// nor anything above them, as git's GIT_CEILING_DIRECTORIES does. It is what
// lets a test, or a scratch directory inside a real workspace, be certain it
// cannot reach that workspace.

func TestFindWorkspace_CeilingAtTheWorkspaceRootHidesIt(t *testing.T) {
	root := t.TempDir()
	touchMarker(t, root, "knowledge.db")
	t.Setenv(CeilingEnv, root)
	if got, _, ok := FindWorkspace(nested(t, root, "a", "b")); ok {
		t.Errorf("found %q although it is a ceiling", got)
	}
}

func TestFindWorkspace_CeilingBelowTheWorkspaceStopsTheWalk(t *testing.T) {
	root := t.TempDir()
	touchMarker(t, root, "knowledge.db")
	scratch := nested(t, root, "tmp", "scratch")
	t.Setenv(CeilingEnv, scratch)
	if got, _, ok := FindWorkspace(nested(t, scratch, "deeper")); ok {
		t.Errorf("walked past the ceiling and found %q", got)
	}
}

func TestFindWorkspace_AWorkspaceInsideTheCeilingIsStillFound(t *testing.T) {
	outer := t.TempDir()
	scratch := nested(t, outer, "tmp")
	inner := nested(t, scratch, "ws")
	touchMarker(t, inner, "knowledge.db")
	t.Setenv(CeilingEnv, scratch)
	got, _, ok := FindWorkspace(nested(t, inner, "x"))
	if !ok || got != inner {
		t.Errorf("FindWorkspace = %q, %v; want %q (below the ceiling)", got, ok, inner)
	}
}

func TestFindWorkspace_CeilingAboveTheWorkspaceChangesNothing(t *testing.T) {
	outer := t.TempDir()
	root := nested(t, outer, "ws")
	touchMarker(t, root, "knowledge.db")
	t.Setenv(CeilingEnv, outer)
	got, _, ok := FindWorkspace(nested(t, root, "x"))
	if !ok || got != root {
		t.Errorf("FindWorkspace = %q, %v; want %q", got, ok, root)
	}
}

func TestFindWorkspace_SeveralCeilingsAndBlankEntries(t *testing.T) {
	root := t.TempDir()
	touchMarker(t, root, "knowledge.db")
	other := t.TempDir()
	list := strings.Join([]string{"", other, "", root}, string(os.PathListSeparator))
	t.Setenv(CeilingEnv, list)
	if got, _, ok := FindWorkspace(nested(t, root, "a")); ok {
		t.Errorf("found %q although root is the last of several ceilings", got)
	}
}

func TestFindWorkspace_UnsetOrEmptyCeilingChangesNothing(t *testing.T) {
	root := t.TempDir()
	touchMarker(t, root, "knowledge.db")
	t.Setenv(CeilingEnv, "")
	if got, _, ok := FindWorkspace(nested(t, root, "a")); !ok || got != root {
		t.Errorf("FindWorkspace = %q, %v; want %q", got, ok, root)
	}
}

// TestTestsRunUnderACeiling guards the TestMain in testmain_test.go: without
// it, a test that creates a directory with no workspace of its own could walk
// up into a real one when TMPDIR points inside it.
func TestTestsRunUnderACeiling(t *testing.T) {
	if got := os.Getenv(CeilingEnv); got != os.TempDir() {
		t.Errorf("%s = %q, want the system temp directory %q", CeilingEnv, got, os.TempDir())
	}
}
