package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The `:` command line (knowledge DR-0066 point 2, DR-0067, v0.0.20 U6). Any verb
// in the table can be typed; what happens next follows its write class: a read or
// a direct verb runs and its output is shown, additive and changing writes show
// the typed command and ask y, removing writes meet the typed-name gate, plan
// verbs show their dry run first, and the command-line-only verbs are refused.

func runCommand(t *testing.T, m *tuiModel, line string) *tuiModel {
	t.Helper()
	m, _ = press(t, m, ch(':'))
	if m.state != viewCommand {
		t.Fatalf("':' left the screen at %s, want the command prompt", viewStateNames[m.state])
	}
	m, _ = press(t, m, append(typed(line), keyEnter)...)
	return m
}

func TestCommand_SplitsAsAShellWould(t *testing.T) {
	for line, want := range map[string][]string{
		`record list`:                                  {"record", "list"},
		`  search   "a b"  c `:                         {"search", "a b", "c"},
		`concept add 'It''s' "x \"y\""`:                {"concept", "add", "Its", `x "y"`},
		`observation add --project p note "two words"`: {"observation", "add", "--project", "p", "note", "two words"},
		`path\ with\ space`:                            {"path with space"},
	} {
		got, err := splitCommandLine(line)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("split(%q) = %q, %v; want %q", line, got, err, want)
		}
	}
	if _, err := splitCommandLine(`search "open`); err == nil {
		t.Error("an unterminated quote was accepted")
	}
}

func TestCommand_AReadRunsAndItsOutputIsShown(t *testing.T) {
	m, _, _ := planModel(t)
	m = runCommand(t, m, "kb record list") // a leading kb is allowed
	if m.state != viewText {
		t.Fatalf("state = %s, want the result", viewStateNames[m.state])
	}
	v := m.View()
	for _, want := range []string{"kb record list", "clasm/DR-0001"} {
		if !strings.Contains(v, want) {
			t.Errorf("the result lacks %q:\n%s", want, v)
		}
	}
	m, _ = press(t, m, ch('q'))
	if m.state != viewMenu {
		t.Errorf("q gave %s, want back where ':' was typed", viewStateNames[m.state])
	}
}

func TestCommand_QIsTextInThePromptAndEscCancels(t *testing.T) {
	m, _, _ := planModel(t)
	m, _ = press(t, m, ch(':'))
	m, _ = press(t, m, typed("quokka")...)
	if m.state != viewCommand || !strings.Contains(m.View(), "quokka") {
		t.Fatalf("typing q quit or lost the text (%s):\n%s", viewStateNames[m.state], m.View())
	}
	m, cmd := press(t, m, keyEsc)
	if isQuit(cmd) || m.state != viewMenu {
		t.Errorf("Esc gave %s (quit %v), want the menu", viewStateNames[m.state], isQuit(cmd))
	}
	if _, cmd := press(t, m, ch(':'), keyCtrlC); !isQuit(cmd) {
		t.Error("Ctrl-C in the prompt did not quit")
	}
}

func TestCommand_AFailureShowsTheErrorAndTheExitStatus(t *testing.T) {
	m, _, _ := planModel(t)
	m = runCommand(t, m, "record show nosuch/DR-0009")
	if m.state != viewText || !strings.Contains(m.View(), "exit status") {
		t.Errorf("state %s; the result should say the exit status:\n%s", viewStateNames[m.state], m.View())
	}
}

func TestCommand_UnknownAndCommandLineOnlyVerbsAreRefused(t *testing.T) {
	m, _, _ := planModel(t)
	m = runCommand(t, m, "frobnicate now")
	if m.state != viewMenu || !strings.Contains(m.View(), "unknown verb") {
		t.Errorf("unknown verb: state %s:\n%s", viewStateNames[m.state], m.View())
	}
	m = runCommand(t, m, "merge -a x.db -b y.db")
	if m.state != viewMenu || !strings.Contains(m.View(), "command line only") {
		t.Errorf("merge: state %s:\n%s", viewStateNames[m.state], m.View())
	}
	m = runCommand(t, m, "-json record list")
	if m.state != viewMenu || !strings.Contains(m.View(), "option") {
		t.Errorf("a global option should be refused:\n%s", m.View())
	}
}

func TestCommand_AnAdditiveWriteShowsTheCommandAndAsksFirst(t *testing.T) {
	m, _, kb := planModel(t)
	m = runCommand(t, m, `concept add Fresh "a new idea"`)
	if m.state != viewPlan {
		t.Fatalf("state = %s, want the confirmation", viewStateNames[m.state])
	}
	v := m.View()
	for _, want := range []string{"not been run", `kb concept add Fresh "a new idea"`, "y apply"} {
		if !strings.Contains(v, want) {
			t.Errorf("the screen lacks %q:\n%s", want, v)
		}
	}
	if ok, _ := kb.HasConcept("Fresh"); ok {
		t.Fatal("the concept existed before y")
	}
	m, _ = press(t, m, ch('y'))
	if ok, _ := kb.HasConcept("Fresh"); !ok {
		t.Error("y did not add the concept")
	}
	if m.state != viewText {
		t.Errorf("state = %s, want the result", viewStateNames[m.state])
	}
}

func TestCommand_ACancelledWriteWritesNothing(t *testing.T) {
	m, _, kb := planModel(t)
	m = runCommand(t, m, `concept add Fresh "a new idea"`)
	m, _ = press(t, m, ch('n'))
	if ok, _ := kb.HasConcept("Fresh"); ok || m.state != viewMenu {
		t.Errorf("a cancel wrote or stayed (%s)", viewStateNames[m.state])
	}
}

func TestCommand_SetStatusIsTheTUIsOwnChange(t *testing.T) {
	m, root, _ := planModel(t)
	file := filepath.Join(root, "agents", "projects", "clasm", "decisions", "0001-fixture.md")
	m = runCommand(t, m, "record set-status clasm/DR-0001 accepted")
	if m.state != viewPlan {
		t.Fatalf("state = %s, want the confirmation:\n%s", viewStateNames[m.state], m.View())
	}
	if v := m.View(); !strings.Contains(v, "proposed") || !strings.Contains(v, "accepted") {
		t.Errorf("the confirmation should show old and new:\n%s", v)
	}
	if b, _ := os.ReadFile(file); strings.Contains(string(b), "status: accepted") {
		t.Fatal("changed before y")
	}
	m, _ = press(t, m, ch('y'))
	if b, _ := os.ReadFile(file); !strings.Contains(string(b), "status: accepted") {
		t.Errorf("the file was not changed:\n%s", b)
	}
}

func TestCommand_ARefusedMoveIsRefusedBeforeAnyConfirmation(t *testing.T) {
	m, _, _ := planModel(t)
	m = runCommand(t, m, "record set-status clasm/DR-0001 proposed")
	if m.state == viewPlan {
		t.Errorf("a move the table forbids reached the confirmation:\n%s", m.View())
	}
}

func TestCommand_ARemovingWriteMeetsTheTypedGate(t *testing.T) {
	m, _, kb := planModel(t)
	m = runCommand(t, m, "concept delete streaming")
	if m.state != viewRemove {
		t.Fatalf("state = %s, want the removing gate:\n%s", viewStateNames[m.state], m.View())
	}
	m, _ = press(t, m, keyEsc)
	if ok, _ := kb.HasConcept("streaming"); !ok || m.state != viewMenu {
		t.Fatal("a cancel removed it or stayed")
	}
	m = runCommand(t, m, "concept delete streaming")
	m, _ = press(t, m, append(typed("streaming"), keyEnter)...)
	if ok, _ := kb.HasConcept("streaming"); ok {
		t.Error("the typed name did not delete")
	}
	m = runCommand(t, m, "source remove 10.1000/x")
	if m.state != viewMenu || !strings.Contains(m.View(), "command line") {
		t.Errorf("a removing verb with no gate here should say so:\n%s", m.View())
	}
}

func TestCommand_APlanVerbShowsItsDryRunFirst(t *testing.T) {
	m, root, _ := planModel(t)
	doc := filepath.Join(root, "docs", "note.md")
	m = runCommand(t, m, "document tag --project clasm")
	if m.state != viewPlan || !strings.Contains(m.View(), "would tag") {
		t.Fatalf("state %s; want the dry run:\n%s", viewStateNames[m.state], m.View())
	}
	if b, _ := os.ReadFile(doc); strings.Contains(string(b), "[[streaming]]") {
		t.Fatal("the dry run wrote")
	}
	m, _ = press(t, m, ch('y'))
	if b, _ := os.ReadFile(doc); !strings.Contains(string(b), "[[streaming]]") {
		t.Errorf("y did not apply:\n%s", b)
	}
}

func TestCommand_ADirectVerbRunsAtOnce(t *testing.T) {
	m, root, _ := planModel(t)
	m = runCommand(t, m, "index "+shellWord(filepath.Join(root, "agents", "projects", "clasm", "decisions")))
	if m.state != viewText {
		t.Fatalf("state = %s, want the result:\n%s", viewStateNames[m.state], m.View())
	}
	if _, err := os.Stat(filepath.Join(root, "agents", "projects", "clasm", "decisions", "index.md")); err != nil {
		t.Errorf("index.md was not written: %v", err)
	}
}

// A colon typed while a form field has the keyboard is text.
func TestCommand_AColonInATextFieldIsText(t *testing.T) {
	m, _, _ := planModel(t)
	m = openLeaf(t, m, "Concepts", "New concept…")
	m, _ = press(t, m, typed("a:b")...)
	if m.state != viewForm || !strings.Contains(m.View(), "a:b") {
		t.Errorf("the colon was not text (%s):\n%s", viewStateNames[m.state], m.View())
	}
}

var _ = tea.KeyMsg{}

func TestBinary_TUICommandLineOnATerminal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	builtKB(t)
	_, root, _ := planModel(t)
	master, screen, wait := startOnATerminal(t, root, "-i")
	screen.waitFor(t, ": command")
	master.WriteString(":")
	screen.waitFor(t, "Type a command as on the command line")
	master.WriteString("search streaming\r")
	screen.waitFor(t, "kb search streaming")
	master.WriteString("qq")
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0:\n%s", code, screen.text())
	}
}
