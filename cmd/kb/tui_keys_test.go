package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// The TUI's key conventions (knowledge DR-0064, v0.0.19 T4): q goes back and
// quits only at the top; Esc cancels an in-progress step and never closes a
// screen; Ctrl-C quits from anywhere; every screen shows a legend of the keys
// that apply; and while text is being typed every printable key is text.

// press sends keys one at a time and returns the model and the last command.
func press(t *testing.T, m *tuiModel, msgs ...tea.Msg) (*tuiModel, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, msg := range msgs {
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(*tuiModel)
	}
	return m, cmd
}

// typed is a string of typed characters as one message each.
func typed(s string) []tea.Msg {
	var out []tea.Msg
	for _, r := range s {
		out = append(out, ch(r))
	}
	return out
}

// browserAt returns a model on the given state, reached the way a person gets there.
func browserAt(t *testing.T, state viewState) *tuiModel {
	t.Helper()
	m := newTestTUIModelWithRecords(t)
	m.projectList.Select(0)
	switch state {
	case viewProjects:
	case viewObservations:
		m, _ = press(t, m, keyEnter)
	case viewConcepts:
		m, _ = press(t, m, keyEnter, ch('c'))
	case viewRecords:
		m, _ = press(t, m, keyEnter, ch('r'))
	case viewSearch:
		m, _ = press(t, m, append(append([]tea.Msg{ch('/')}, typed("observation")...), keyEnter)...)
	}
	if m.state != state {
		t.Fatalf("could not reach %v, ended on %v", viewStateNames[state], viewStateNames[m.state])
	}
	return m
}

var allStates = []viewState{viewProjects, viewObservations, viewConcepts, viewRecords, viewSearch}

// q goes back to the parent from every sub-screen, and does not quit.
func TestTUIKeys_QGoesBackFromEverySubScreen(t *testing.T) {
	for _, state := range []viewState{viewObservations, viewConcepts, viewRecords} {
		m := browserAt(t, state)
		m, cmd := press(t, m, ch('q'))
		if m.state != viewProjects || isQuit(cmd) {
			t.Errorf("%s: q gave state %s, quit %v; want back to the projects", viewStateNames[state], viewStateNames[m.state], isQuit(cmd))
		}
	}
}

// q walks up the tree one level at a time and quits only at the top menu.
func TestTUIKeys_QWalksUpToTheTopMenuAndQuitsThere(t *testing.T) {
	m := browserAt(t, viewProjects)
	for _, want := range []viewState{viewGroup, viewMenu} {
		var cmd tea.Cmd
		m, cmd = press(t, m, ch('q'))
		if m.state != want || isQuit(cmd) {
			t.Fatalf("q gave %s (quit %v), want %s", viewStateNames[m.state], isQuit(cmd), viewStateNames[want])
		}
	}
	if _, cmd := press(t, m, ch('q')); !isQuit(cmd) {
		t.Error("q at the top menu did not quit")
	}
}

// A search is entered from wherever you were, and q returns there, not to the top.
func TestTUIKeys_QFromSearchResultsGoesBackToWhereTheSearchStarted(t *testing.T) {
	for _, from := range []viewState{viewProjects, viewObservations, viewConcepts, viewRecords} {
		m := browserAt(t, from)
		m, _ = press(t, m, append(append([]tea.Msg{ch('/')}, typed("observation")...), keyEnter)...)
		if m.state != viewSearch {
			t.Fatalf("from %s: search did not show results", viewStateNames[from])
		}
		m, cmd := press(t, m, ch('q'))
		if m.state != from || isQuit(cmd) {
			t.Errorf("from %s: q from the results gave %s, quit %v; want back to %s", viewStateNames[from], viewStateNames[m.state], isQuit(cmd), viewStateNames[from])
		}
	}
}

// A second search from the results still returns to where the first began.
func TestTUIKeys_ASecondSearchKeepsTheWayBack(t *testing.T) {
	m := browserAt(t, viewObservations)
	m, _ = press(t, m, append(append([]tea.Msg{ch('/')}, typed("observation")...), keyEnter)...)
	m, _ = press(t, m, append(append([]tea.Msg{ch('/')}, typed("streaming")...), keyEnter)...)
	if m, _ = press(t, m, ch('q')); m.state != viewObservations {
		t.Errorf("q after two searches gave %s, want viewObservations", viewStateNames[m.state])
	}
}

// Esc cancels an in-progress step. Outside one it does nothing: never closes a
// screen, never quits, and is not handed to the list, whose own keymap quits on it.
func TestTUIKeys_EscDoesNothingOutsideAStep(t *testing.T) {
	for _, state := range allStates {
		m := browserAt(t, state)
		m, cmd := press(t, m, keyEsc)
		if m.state != state || isQuit(cmd) {
			t.Errorf("%s: Esc gave state %s, quit %v; want nothing", viewStateNames[state], viewStateNames[m.state], isQuit(cmd))
		}
		if m.searching {
			t.Errorf("%s: Esc started something", viewStateNames[state])
		}
	}
}

// Ctrl-C quits from everywhere, including while typing and over an error.
func TestTUIKeys_CtrlCQuitsFromEverywhere(t *testing.T) {
	for _, state := range allStates {
		m := browserAt(t, state)
		if _, cmd := press(t, m, keyCtrlC); !isQuit(cmd) {
			t.Errorf("%s: Ctrl-C did not quit", viewStateNames[state])
		}
		m = browserAt(t, state)
		m, _ = press(t, m, ch('/'))
		if _, cmd := press(t, m, keyCtrlC); !isQuit(cmd) {
			t.Errorf("%s while typing a search: Ctrl-C did not quit", viewStateNames[state])
		}
	}
	m := browserAt(t, viewProjects)
	m.setErr(fmt.Errorf("boom"))
	if _, cmd := press(t, m, keyCtrlC); !isQuit(cmd) {
		t.Error("Ctrl-C did not quit over an error")
	}
}

// ─── text entry (DR-0064 amendment) ──────────────────────────────────────────

func TestTUIKeys_OnlyTheSearchPromptCapturesText(t *testing.T) {
	for _, state := range allStates {
		m := browserAt(t, state)
		if m.capturingText() {
			t.Errorf("%s: capturing text while browsing", viewStateNames[state])
		}
		m, _ = press(t, m, ch('/'))
		if !m.capturingText() {
			t.Errorf("%s: not capturing text in the search prompt", viewStateNames[state])
		}
	}
}

// The edge clasm met: a name that starts with q. In the search prompt every
// command letter is text, from every screen; Esc leaves the mode; then q is a
// command again.
func TestTUIKeys_CommandLettersAreTextInTheSearchPrompt(t *testing.T) {
	for _, state := range allStates {
		m := browserAt(t, state)
		m, _ = press(t, m, ch('/'))
		m, cmd := press(t, m, typed("qjkaynocr:")...)
		if isQuit(cmd) || !m.searching || m.state != state {
			t.Fatalf("%s: typing acted as a command (quit %v, searching %v, state %s)", viewStateNames[state], isQuit(cmd), m.searching, viewStateNames[m.state])
		}
		if got := m.searchInput.Value(); got != "qjkaynocr:" {
			t.Errorf("%s: the prompt holds %q, want every typed key", viewStateNames[state], got)
		}
		m, cmd = press(t, m, keyEsc)
		if m.searching || m.state != state || isQuit(cmd) {
			t.Fatalf("%s: Esc gave searching %v, state %s, quit %v; want the mode ended and nothing else", viewStateNames[state], m.searching, viewStateNames[m.state], isQuit(cmd))
		}
		m, cmd = press(t, m, ch('q'))
		if isQuit(cmd) || m.state == state {
			t.Errorf("%s: q after the prompt should go back (state %s, quit %v)", viewStateNames[state], viewStateNames[m.state], isQuit(cmd))
		}
	}
}

// A search term that starts with q reaches the search.
func TestTUIKeys_ASearchTermStartingWithQIsSearched(t *testing.T) {
	m := browserAt(t, viewProjects)
	m, _ = press(t, m, append(append([]tea.Msg{ch('/')}, typed("quux")...), keyEnter)...)
	if m.state != viewSearch || !strings.Contains(m.searchList.Title, "quux") {
		t.Errorf("state %s, title %q; want a search for quux", viewStateNames[m.state], m.searchList.Title)
	}
}

// ─── the legend bar ──────────────────────────────────────────────────────────

func TestTUIKeys_EveryScreenShowsItsKeys(t *testing.T) {
	for state, want := range map[viewState][]string{
		viewProjects:     {"Enter open", "/ search", "q back"},
		viewObservations: {"o c r tabs", "/ search", "q back"},
		viewConcepts:     {"o c r tabs", "/ search", "q back"},
		viewRecords:      {"o c r tabs", "/ search", "q back"},
		viewSearch:       {"/ search", "q back"},
	} {
		view := browserAt(t, state).View()
		for _, w := range want {
			if !strings.Contains(view, w) {
				t.Errorf("%s: the legend lacks %q:\n%s", viewStateNames[state], w, view)
			}
		}
	}
}

// While typing, the legend says what Esc and Enter do, and does not offer q.
func TestTUIKeys_TheLegendChangesWithTheMode(t *testing.T) {
	m, _ := press(t, browserAt(t, viewObservations), ch('/'))
	view := m.View()
	for _, want := range []string{"Esc cancel", "Enter search"} {
		if !strings.Contains(view, want) {
			t.Errorf("the typing legend lacks %q:\n%s", want, view)
		}
	}
	for _, bad := range []string{"q back", "q quit"} {
		if strings.Contains(view, bad) {
			t.Errorf("the typing legend offers %q, which is text there:\n%s", bad, view)
		}
	}
}

// ─── errors ──────────────────────────────────────────────────────────────────

func TestTUIKeys_AnErrorIsDismissedWithQ(t *testing.T) {
	m := browserAt(t, viewObservations)
	m.setErr(fmt.Errorf("boom"))
	if v := m.View(); !strings.Contains(v, "boom") || !strings.Contains(v, "q dismiss") {
		t.Errorf("the error view lacks the message or its key:\n%s", v)
	}
	m, cmd := press(t, m, ch('q'))
	if m.err != nil || isQuit(cmd) || m.state != viewObservations {
		t.Errorf("q over an error: err %v, quit %v, state %s; want it dismissed and nothing else", m.err, isQuit(cmd), viewStateNames[m.state])
	}
}

// ─── the lists ───────────────────────────────────────────────────────────────

// A list must not be able to quit or start a hidden filter of its own: the model
// owns the keys. Its keymap quits on q and Esc, and Esc is a no-op here.
func TestTUIKeys_ListsHaveNoQuitKeysOrHiddenFilter(t *testing.T) {
	for _, state := range allStates {
		m := browserAt(t, state)
		for name, l := range map[string]interface {
			FilteringEnabled() bool
			ShowHelp() bool
		}{"projects": &m.projectList, "observations": &m.observationList, "concepts": &m.conceptList, "records": &m.recordList, "search": &m.searchList} {
			if l.FilteringEnabled() {
				t.Errorf("%s: the %s list can start a filter of its own", viewStateNames[state], name)
			}
			if l.ShowHelp() {
				t.Errorf("%s: the %s list draws its own help, which is not this screen's legend", viewStateNames[state], name)
			}
		}
		for name, quit := range map[string]bool{
			"projects": m.projectList.KeyMap.Quit.Enabled(), "observations": m.observationList.KeyMap.Quit.Enabled(),
			"concepts": m.conceptList.KeyMap.Quit.Enabled(), "records": m.recordList.KeyMap.Quit.Enabled(),
			"search": m.searchList.KeyMap.Quit.Enabled(),
		} {
			if quit {
				t.Errorf("%s: the %s list still has its own quit keys", viewStateNames[state], name)
			}
		}
	}
}

// The legend takes a line; the lists are one line shorter than the window so it
// is not pushed off the bottom.
func TestTUIKeys_TheLegendFitsInTheWindow(t *testing.T) {
	for _, state := range allStates {
		m := browserAt(t, state)
		lines := strings.Count(m.View(), "\n") + 1
		if lines > 24 {
			t.Errorf("%s: the view is %d lines in a 24-line window", viewStateNames[state], lines)
		}
	}
	m := browserAt(t, viewRecords)
	m, _ = press(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if got := m.recordList.Height(); got != 24 {
		t.Errorf("the records list is %d lines after a resize to 30, want 24 (the frame takes four and the tab strip two): it was missing from the resize", got)
	}
}

// ─── the real binary on a terminal ───────────────────────────────────────────

// waitForCount waits until text has appeared at least n times on the screen.
func (sc *ptyScreen) waitForCount(t *testing.T, text string, n int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Count(sc.text(), text) >= n {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%q never appeared %d times on the terminal; it showed:\n%s", text, n, sc.text())
}

// Real bytes through the built kb: bare kb opens the top menu; a lone Esc must not
// quit (the list's own keymap quits on it); Enter opens; q goes up one level at a
// time; and q at the top menu quits.
func TestBinary_TUIWalksDownAndUpWithEscDoingNothing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	bin := builtKB(t)
	master, slave := openPtyForTest(t)
	_, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"})
	cmd := exec.Command(bin)
	cmd.Dir = root
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	if err := cmd.Start(); err != nil {
		t.Fatalf("start kb: %v", err)
	}
	screen := newPtyScreen(master)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	exited := func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}
	alive := func(what string) {
		t.Helper()
		time.Sleep(400 * time.Millisecond)
		if exited() {
			t.Fatalf("%s quit the program:\n%s", what, screen.text())
		}
	}

	screen.waitFor(t, "kb — knowledge") // the top menu
	for _, want := range []string{"agents/knowledge.db", "q quit", "(v0.0.20)"} {
		if !strings.Contains(screen.text(), want) {
			t.Errorf("the top menu lacks %q:\n%s", want, screen.text())
		}
	}
	master.WriteString("\x1b")
	alive("a lone Esc on the top menu")
	master.WriteString("\r") // Projects
	screen.waitFor(t, "New project…")
	master.WriteString("\r") // Browse
	screen.waitFor(t, "1 item")
	master.WriteString("\r") // the project
	screen.waitFor(t, "Observations (")
	master.WriteString("\x1b")
	alive("Esc on a project")
	master.WriteString("q") // back to the project list
	screen.waitForCount(t, "1 item", 2)
	master.WriteString("q") // back to the Projects menu
	screen.waitForCount(t, "New project…", 2)
	master.WriteString("q") // back to the top menu
	screen.waitForCount(t, "kb — knowledge", 2)
	alive("walking back up")
	master.WriteString("q") // quit
	select {
	case err := <-done:
		if code := exitCode(t, err); code != 0 {
			t.Errorf("exit = %d after q at the top menu, want 0", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("q at the top menu did not quit:\n%s", screen.text())
	}
}
