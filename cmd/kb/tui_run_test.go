package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	knowledge "github.com/rsdoiel/knowledge"
)

// `index` and `record fmt` from the menus (knowledge DR-0067, v0.0.20 U5). They
// run directly, with no confirmation, on the decisions directory of the Records
// scope (agents/projects/NAME/decisions, or agents/ for every scope), and the
// output of the command is shown on a result screen with the command that does the
// same.

// layoutModel is a menu model over a workspace laid out as the real one is:
// records in agents/projects/clasm/decisions, one of them not in canonical form.
func layoutModel(t *testing.T) (*tuiModel, string, *knowledge.KnowledgeBase) {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("KB_PROJECT", "")
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "agents", "projects", "clasm", "decisions")
	testRecord{ID: "0001", Project: "clasm", Title: "The first", Status: "accepted", Trigger: "design"}.write(t, dir)
	// not canonical: the required fields only, so fmt fills in the rest
	rough := "---\nid: \"0002\"\ntitle: \"A rough one\"\ndate: \"2026-08-02\"\nstatus: proposed\nkind: decision\ntrigger: design\nproject: clasm\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "0002-rough.md"), []byte(rough), 0o644); err != nil {
		t.Fatal(err)
	}
	runIngest(t, kb, dir)
	t.Setenv("KB_PROJECT", "clasm")
	m, err := newTUIModel(kb, nil)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return next.(*tuiModel), root, kb
}

// openLeaf goes into a group menu and presses Enter on the leaf with that label.
func openLeaf(t *testing.T, m *tuiModel, group, label string) *tuiModel {
	t.Helper()
	m = openRow(t, m, group)
	for i, it := range groupMenu(m.group) {
		if it.Label == label {
			for j := 0; j < i; j++ {
				m, _ = press(t, m, ch('j'))
			}
			m, _ = press(t, m, keyEnter)
			return m
		}
	}
	t.Fatalf("no leaf %q in %s", label, group)
	return m
}

func TestRun_FormatFilesRunsAtOnceOnTheScopesDirectory(t *testing.T) {
	m, root, _ := layoutModel(t)
	rough := filepath.Join(root, "agents", "projects", "clasm", "decisions", "0002-rough.md")
	before, _ := os.ReadFile(rough)
	m = openLeaf(t, m, "Records", "Format files…")
	if m.state != viewText {
		t.Fatalf("state = %s; fmt should run at once and show its result", viewStateNames[m.state])
	}
	after, _ := os.ReadFile(rough)
	if string(after) == string(before) || !strings.Contains(string(after), "supersedes:") {
		t.Errorf("the rough record was not rewritten:\n%s", after)
	}
	v := m.View()
	for _, want := range []string{"1 changed", "1 already canonical", "equivalent:", "kb record fmt"} {
		if !strings.Contains(v, want) {
			t.Errorf("the result lacks %q:\n%s", want, v)
		}
	}
	if !strings.Contains(v, "agents/projects/clasm/decisions") {
		t.Errorf("the result should name the directory it ran on:\n%s", v)
	}
	m, _ = press(t, m, ch('q'))
	if m.state != viewGroup || m.group != "record" {
		t.Errorf("q gave %s %q, want the Records menu", viewStateNames[m.state], m.group)
	}
}

func TestRun_IndexWritesTheIndexForTheScope(t *testing.T) {
	m, root, _ := layoutModel(t)
	index := filepath.Join(root, "agents", "projects", "clasm", "decisions", "index.md")
	if _, err := os.Stat(index); err == nil {
		t.Fatal("the index existed before")
	}
	m = openRow(t, m, "Index")
	if m.state != viewText {
		t.Fatalf("state = %s; index should run at once and show its result", viewStateNames[m.state])
	}
	b, err := os.ReadFile(index)
	if err != nil || !strings.Contains(string(b), "DR-0001") {
		t.Fatalf("index.md = %q, %v; want a line for DR-0001", b, err)
	}
	v := m.View()
	for _, want := range []string{"index.md", "equivalent:", "kb index"} {
		if !strings.Contains(v, want) {
			t.Errorf("the result lacks %q:\n%s", want, v)
		}
	}
}

func TestRun_WidenedToEveryScopeItRunsOverTheWorkspaceTree(t *testing.T) {
	m, root, _ := layoutModel(t)
	m = openRow(t, m, "Records")
	m, _ = press(t, m, ch('a')) // every scope
	for i := 0; i < 3; i++ {
		m, _ = press(t, m, ch('j'))
	}
	m, _ = press(t, m, keyEnter) // Format files…
	if m.state != viewText {
		t.Fatalf("state = %s", viewStateNames[m.state])
	}
	if v := m.View(); !strings.Contains(v, filepath.Join(root, "agents")) && !strings.Contains(v, "agents") {
		t.Errorf("the result should name the agents tree:\n%s", v)
	}
	if !strings.Contains(m.View(), "kb record fmt") {
		t.Errorf("the equivalent command is missing:\n%s", m.View())
	}
}

// A scope with no decisions directory says so; nothing is created.
func TestRun_ANoDirectoryIsANoticeAndCreatesNothing(t *testing.T) {
	m, root, _ := layoutModel(t)
	t.Setenv("KB_PROJECT", "") // every scope would run on agents/, so narrow to a project with no directory
	if _, err := m.kb.AddProject("ghost", "no files"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KB_PROJECT", "ghost")
	m = openLeaf(t, m, "Records", "Format files…")
	if m.state != viewGroup {
		t.Fatalf("state = %s; a missing directory should leave the menu", viewStateNames[m.state])
	}
	if v := m.View(); !strings.Contains(v, "ghost") || !strings.Contains(v, "no decisions directory") {
		t.Errorf("the notice should name the project and the missing directory:\n%s", v)
	}
	if _, err := os.Stat(filepath.Join(root, "agents", "projects", "ghost")); err == nil {
		t.Error("a directory was created")
	}
}

// A malformed record is the command's error, shown as a notice; nothing crashes.
func TestRun_AnIndexErrorIsANotice(t *testing.T) {
	m, root, _ := layoutModel(t)
	bad := filepath.Join(root, "agents", "projects", "clasm", "decisions", "0003-bad.md")
	if err := os.WriteFile(bad, []byte("---\nid: [unclosed\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = openRow(t, m, "Index")
	if m.state != viewMenu {
		t.Fatalf("state = %s; a failed index should stay on the menu with a notice", viewStateNames[m.state])
	}
	if v := m.View(); !strings.Contains(v, "0003-bad.md") && !strings.Contains(v, "parse") && !strings.Contains(v, "malformed") {
		t.Errorf("the notice should carry the command's error:\n%s", v)
	}
}

func TestRun_TheRowsAreBuiltAndEscDoesNothingOnTheResult(t *testing.T) {
	m, _, _ := layoutModel(t)
	for _, it := range topMenu() {
		if it.Label == "Index" && !it.Built {
			t.Error("Index is still marked as not built")
		}
	}
	for _, it := range groupMenu("record") {
		if it.Label == "Format files…" && !it.Built {
			t.Error("Format files… is still marked as not built")
		}
	}
	m = openRow(t, m, "Index")
	m, cmd := press(t, m, keyEsc)
	if m.state != viewText || isQuit(cmd) {
		t.Errorf("Esc on the result: %s, quit %v", viewStateNames[m.state], isQuit(cmd))
	}
	m, _ = press(t, m, ch('q'))
	if m.state != viewMenu {
		t.Errorf("q from the result gave %s, want the top menu", viewStateNames[m.state])
	}
}

// ─── the real binary ─────────────────────────────────────────────────────────

func TestBinary_TUIFormatFilesAndIndexOnATerminal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	builtKB(t)                   // build from the package directory, before the fixture changes it
	_, root, _ := layoutModel(t) // KB_PROJECT=clasm is set for the child too
	rough := filepath.Join(root, "agents", "projects", "clasm", "decisions", "0002-rough.md")

	master, screen, wait := startOnATerminal(t, root, "-i", "record")
	screen.waitFor(t, "Format files…")
	master.WriteString("jjj\r") // Browse, Pending, New record, then Format files…
	screen.waitFor(t, "already canonical")
	screen.waitFor(t, "equivalent:  kb record fmt agents/projects/clasm/decisions")
	if b, _ := os.ReadFile(rough); !strings.Contains(string(b), "supersedes:") {
		t.Errorf("the rough record was not rewritten:\n%s", b)
	}
	master.WriteString("qqq") // result, Records menu, top menu: then quit
	master.WriteString("q")
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0:\n%s", code, screen.text())
	}

	master, screen, wait = startOnATerminal(t, root, "-i")
	screen.waitFor(t, "regenerate a decisions/index.md")
	master.WriteString("jjjjjjjj\r") // Index is the ninth row
	screen.waitFor(t, "equivalent:  kb index agents/projects/clasm/decisions")
	if _, err := os.Stat(filepath.Join(root, "agents", "projects", "clasm", "decisions", "index.md")); err != nil {
		t.Errorf("index.md was not written: %v", err)
	}
	master.WriteString("qq")
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0:\n%s", code, screen.text())
	}
}
