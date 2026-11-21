package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `record new --project NAME` builds a directory from NAME. A name that is a
// path ("../../x", "a/b", "..") wrote the file outside agents/projects, and
// outside the workspace for enough "..". The name is a project name, never a
// path: it is refused, exit 2, and nothing is written.
func TestRecordNew_ProjectNameThatIsAPathIsRefusedAndWritesNothing(t *testing.T) {
	run := kindsWorkspace(t)
	cwd, _ := os.Getwd()
	parent := filepath.Dir(cwd)
	before, _ := os.ReadDir(parent)

	for _, name := range []string{"../../../evil", "..", ".", "a/b", `a\b`, "p/../../x"} {
		for _, kind := range []string{"decision", "design", "plan"} {
			code, out, e := run("record", "new", "--project", name, "--kind", kind, "--title", "t", "--trigger", "request")
			if code != 2 || !strings.Contains(e, "project name") {
				t.Errorf("--project %q --kind %s: exit %d, out %q, err %q; want exit 2 naming a bad project name", name, kind, code, out, e)
			}
		}
	}
	after, _ := os.ReadDir(parent)
	if len(after) != len(before) {
		t.Errorf("the directory above the workspace changed: %d entries before, %d after", len(before), len(after))
	}
	var stray []string
	filepath.WalkDir(cwd, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
			stray = append(stray, p)
		}
		return nil
	})
	if len(stray) != 0 {
		t.Errorf("a Markdown file was written: %v", stray)
	}
	// A real project still works.
	if code, _, e := run("record", "new", "--project", "p", "--title", "ok", "--trigger", "request"); code != 0 {
		t.Errorf("record new for project p: exit %d: %s", code, e)
	}
}
