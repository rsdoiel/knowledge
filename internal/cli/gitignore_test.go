package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func gitWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestInit_GitWorkspaceIgnoresModelsAndVectors(t *testing.T) {
	root := gitWorkspace(t)
	var out bytes.Buffer
	if err := cmdInit(nil, nil, false, []string{root}, &out); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, filepath.Join(root, ".gitignore"))
	for _, want := range []string{"agents/models/\n", "agents/knowledge.vectors.db\n"} {
		if !strings.Contains(got, want) {
			t.Errorf(".gitignore = %q, missing %q", got, want)
		}
	}
	if !strings.Contains(out.String(), "added to .gitignore: agents/models/, agents/knowledge.vectors.db") {
		t.Errorf("init said %q", out.String())
	}
	// Running init again changes nothing and says nothing about it.
	out.Reset()
	if err := cmdInit(nil, nil, false, []string{root}, &out); err != nil {
		t.Fatal(err)
	}
	if again := readFile(t, filepath.Join(root, ".gitignore")); again != got || strings.Contains(out.String(), ".gitignore") {
		t.Errorf("second init: .gitignore %q, output %q", again, out.String())
	}
}

func TestInit_JSONReportsWhatItAdded(t *testing.T) {
	root := gitWorkspace(t)
	var out bytes.Buffer
	if err := cmdInit(nil, nil, true, []string{root}, &out); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Added []string `json:"gitignore_added"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil || len(doc.Added) != 2 {
		t.Errorf("init -json = %s (%v)", out.String(), err)
	}
	plain := t.TempDir()
	out.Reset()
	cmdInit(nil, nil, true, []string{plain}, &out)
	if !strings.Contains(out.String(), `"gitignore_added": []`) {
		t.Errorf("a non-git workspace should report []: %s", out.String())
	}
}

func TestInit_NonGitWorkspaceMakesNoGitignore(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := cmdInit(nil, nil, false, []string{root}, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); err == nil {
		t.Error("init made a .gitignore in a directory that is not a git work tree")
	}
	if strings.Contains(out.String(), "gitignore") {
		t.Errorf("init mentioned git: %q", out.String())
	}
}

func TestInit_AlreadyIgnoredIsLeftByteForByte(t *testing.T) {
	for _, body := range []string{
		"agents/models/\nagents/knowledge.vectors.db\n",
		"/agents/models\n/agents/knowledge.vectors.db",
		"*.log\n  agents/models  \nagents/knowledge.vectors.db\r\n",
	} {
		root := gitWorkspace(t)
		path := filepath.Join(root, ".gitignore")
		os.WriteFile(path, []byte(body), 0o644)
		var out bytes.Buffer
		if err := cmdInit(nil, nil, false, []string{root}, &out); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, path); got != body {
			t.Errorf("%q was rewritten as %q", body, got)
		}
	}
}

func TestInit_AppendsToAnExistingGitignoreWithoutANewline(t *testing.T) {
	root := gitWorkspace(t)
	path := filepath.Join(root, ".gitignore")
	os.WriteFile(path, []byte("*.tmp\nagents/models/"), 0o644) // models present, vectors not, no final newline
	var out bytes.Buffer
	if err := cmdInit(nil, nil, false, []string{root}, &out); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	if !strings.HasPrefix(got, "*.tmp\nagents/models/\n# kb:") || strings.Count(got, "agents/models/") != 1 || !strings.HasSuffix(got, "agents/knowledge.vectors.db\n") {
		t.Errorf(".gitignore = %q", got)
	}
}

func TestInit_GitignoreThatCannotBeWrittenFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	root := gitWorkspace(t)
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("x\n"), 0o444)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"init", root}, &out, &errOut); code != classNoPermission.Code {
		t.Errorf("exit %d, want 77: %s", code, errOut.String())
	}
}

func TestModel_WorkspaceInstallIgnoresTheModelDirectory(t *testing.T) {
	work, src := modelEnv(t)
	ws := filepath.Join(work, "ws")
	os.MkdirAll(filepath.Join(ws, ".git"), 0o755)
	os.MkdirAll(filepath.Join(ws, "agents"), 0o755)
	os.WriteFile(filepath.Join(ws, "agents", "knowledge.jsonl"), nil, 0o644)
	t.Chdir(ws)
	code, out, errOut := runModel(t, "install", src, "-workspace")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	got := readFile(t, filepath.Join(ws, ".gitignore"))
	if !strings.Contains(got, "agents/models/") || !strings.Contains(out, "added to .gitignore") {
		t.Errorf(".gitignore %q, output %q", got, out)
	}
	// A user-level install needs no ignore line and touches nothing.
	before := got
	runModel(t, "install", src)
	if readFile(t, filepath.Join(ws, ".gitignore")) != before {
		t.Error("a user-level install changed .gitignore")
	}
}

func TestModel_WorkspaceInstallInANonGitWorkspaceMakesNoGitignore(t *testing.T) {
	work, src := modelEnv(t)
	ws := filepath.Join(work, "ws")
	os.MkdirAll(filepath.Join(ws, "agents"), 0o755)
	os.WriteFile(filepath.Join(ws, "agents", "knowledge.jsonl"), nil, 0o644)
	t.Chdir(ws)
	if code, _, e := runModel(t, "install", src, "-workspace"); code != 0 {
		t.Fatalf("exit %d: %s", code, e)
	}
	if _, err := os.Stat(filepath.Join(ws, ".gitignore")); err == nil {
		t.Error("made a .gitignore outside a git work tree")
	}
}

func TestWarnIfTracked_NamesAFileGitAlreadyTracks(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(git, append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	os.MkdirAll(filepath.Join(root, "agents"), 0o755)
	os.WriteFile(filepath.Join(root, "agents", "knowledge.vectors.db"), []byte("x"), 0o644)
	run("add", "-f", ".")
	run("commit", "-q", "-m", "oops")

	var errOut bytes.Buffer
	previous := adviceOut
	adviceOut = &errOut
	t.Cleanup(func() { adviceOut = previous })
	warnIfTracked(root, "agents/knowledge.vectors.db")
	warnIfTracked(root, "agents/knowledge.vectors.db") // once only
	warnIfTracked(root, "agents/models/")              // not tracked
	got := errOut.String()
	if strings.Count(got, "git tracks agents/knowledge.vectors.db") != 1 || strings.Contains(got, "agents/models/") || !strings.Contains(got, "git rm -r --cached") {
		t.Errorf("warning = %q", got)
	}
}
