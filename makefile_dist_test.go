package knowledge

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A release archive holds the program, its manual pages, the project's own
// documents and the skills, and nothing else. `cp -vR agents dist/` put whatever
// sat in agents/ into every zip: an empty agents/knowledge.db that `kb` leaves
// in the repository when it runs there. These tests keep the copy and the
// check that stands behind it honest.

// distSandbox is a scratch directory with the real Makefile and the files a
// release is made from, including the strays that must not ship.
func distSandbox(t *testing.T) string {
	t.Helper()
	for _, tool := range []string{"make", "unzip"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available", tool)
		}
	}
	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	put := func(name, content string) {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put("Makefile", string(mk))
	for _, f := range []string{"codemeta.json", "CITATION.cff", "README.md", "LICENSE", "INSTALL.md", "CHANGES.md", "kb.1.md"} {
		put(f, "x\n")
	}
	put("man/man1/kb.1", "x\n")
	put("agents/skills/setup-knowledge-base/SKILL.md", "x\n")
	// What must not ship.
	put("agents/knowledge.db", "SQLite format 3\x00")
	put("agents/knowledge.jsonl", "{}\n")
	put("agents/tmp/scratch.txt", "x\n")
	return dir
}

func listTree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func TestDistributeDocs_CopiesTheSkillsAndNothingElseFromAgents(t *testing.T) {
	dir := distSandbox(t)
	c := exec.Command("make", "distribute_docs")
	c.Dir = dir
	if b, err := c.CombinedOutput(); err != nil {
		t.Fatalf("make distribute_docs: %v\n%s", err, b)
	}
	got := listTree(t, filepath.Join(dir, "dist", "agents"))
	want := []string{"skills/setup-knowledge-base/SKILL.md"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("dist/agents holds\n  %s\nwant only\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// writeZip makes a zip holding the named files.
func writeZip(t *testing.T, path string, names ...string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for _, n := range names {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(n, "/") {
			w.Write([]byte("x"))
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

var goodZip = []string{
	"LICENSE", "codemeta.json", "CITATION.cff", "README.md", "CHANGES.md", "INSTALL.md",
	"kb.1.md", "kb-record.1.md", "bin/", "bin/kb", "bin/kb.exe",
	"man/", "man/man1/", "man/man1/kb.1", "man/man1/kb-record.1",
	"agents/", "agents/skills/", "agents/skills/setup-knowledge-base/", "agents/skills/setup-knowledge-base/SKILL.md",
}

func runCheckDist(t *testing.T, dir string) (string, error) {
	t.Helper()
	c := exec.Command("make", "-s", "check-dist")
	c.Dir = dir
	b, err := c.CombinedOutput()
	return string(b), err
}

func TestCheckDist_AcceptsAReleaseArchive(t *testing.T) {
	dir := distSandbox(t)
	if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeZip(t, filepath.Join(dir, "dist", "knowledge-v0.0.0-Linux-x86_64.zip"), goodZip...)
	if out, err := runCheckDist(t, dir); err != nil {
		t.Errorf("a good archive was refused: %v\n%s", err, out)
	}
}

func TestCheckDist_RefusesAFileThatIsNotPartOfARelease(t *testing.T) {
	for _, stray := range []string{
		"agents/knowledge.db", "agents/knowledge.jsonl", "agents/tmp/scratch.txt",
		"knowledge.db", "notes.txt", "bin/.env", "man/man1/scratch.db", "TODO.md",
	} {
		t.Run(stray, func(t *testing.T) {
			dir := distSandbox(t)
			if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o755); err != nil {
				t.Fatal(err)
			}
			writeZip(t, filepath.Join(dir, "dist", "knowledge-v0.0.0-Linux-x86_64.zip"), goodZip...)
			writeZip(t, filepath.Join(dir, "dist", "knowledge-v0.0.0-macOS-arm64.zip"), append(append([]string{}, goodZip...), stray)...)
			out, err := runCheckDist(t, dir)
			if err == nil {
				t.Fatalf("an archive holding %s was accepted:\n%s", stray, out)
			}
			if !strings.Contains(out, stray) || !strings.Contains(out, "macOS-arm64") {
				t.Errorf("the message does not name the file and the archive:\n%s", out)
			}
		})
	}
}

func TestCheckDist_NoArchiveIsAFailure(t *testing.T) {
	dir := distSandbox(t)
	if out, err := runCheckDist(t, dir); err == nil {
		t.Errorf("no archives in dist/ passed:\n%s", out)
	}
}

// release runs the check, after the archives are built and before it says it is
// ready.
func TestReleaseTargetRunsCheckDist(t *testing.T) {
	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	s := string(mk)
	i := strings.Index(s, "\nrelease: ")
	if i < 0 {
		t.Fatal("no release target")
	}
	rule := s[i:]
	if j := strings.Index(rule[1:], "\n\n"); j > 0 {
		rule = rule[:j+1]
	}
	if !strings.Contains(rule, "check-dist") {
		t.Errorf("the release target does not run check-dist:\n%s", rule)
	}
	if strings.Contains(s, "cp -vR agents dist/") {
		t.Errorf("distribute_docs still copies all of agents/")
	}
}
