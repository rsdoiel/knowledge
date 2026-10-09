package main

import (
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	knowledge "github.com/rsdoiel/knowledge"
)

// The removing gate (knowledge DR-0067, DR-0064 amendment; v0.0.20 U3). `d` on a
// selected project, observation, concept or record shows a plan of what would go
// and what points at it, and asks the person to type the thing's name or reference;
// only an exact match deletes. No flag, no y/n. A refusal (a project that owns
// content, a record whose file still exists) is a notice, not a gate. While the
// name is typed every key is text.

// removeModel is the project list of a workspace with: alpha (an observation linked
// to a concept, the concept linked to alpha, two records with no files), empty (linked
// to one concept) and busy (owns an observation).
func removeModel(t *testing.T) *tuiModel {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("KB_PROJECT", "")
	m := newTestTUIModelWithRecords(t)
	kb := m.kb
	cid, err := kb.AddConcept("shared", "linked to empty")
	if err != nil {
		t.Fatal(err)
	}
	empty, err := kb.AddProject("empty", "a stray project")
	if err != nil {
		t.Fatal(err)
	}
	if err := kb.LinkProjectConcept(empty, cid); err != nil {
		t.Fatal(err)
	}
	busy, err := kb.AddProject("busy", "owns work")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kb.AddObservation(busy, "note", "work that must not be deleted with its project"); err != nil {
		t.Fatal(err)
	}
	fresh, err := newTUIModelAt(kb, nil, tuiStart{state: viewProjects})
	if err != nil {
		t.Fatal(err)
	}
	next, _ := fresh.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return next.(*tuiModel)
}

// selectProject puts the cursor on a project by name.
func selectProject(t *testing.T, m *tuiModel, name string) {
	t.Helper()
	for i, it := range m.projectList.Items() {
		if p, ok := it.(projectItem); ok && p.p.Name == name {
			m.projectList.Select(i)
			return
		}
	}
	t.Fatalf("no project %q", name)
}

// unwrapped reads a framed screen as running text: the borders go, the lines are
// joined, and a hyphen the wrapping split (set-/status) is put back.
func unwrapped(view string) string {
	var parts []string
	for _, l := range strings.Split(view, "\n") {
		l = strings.TrimSpace(strings.Trim(strings.TrimSpace(l), "│┌└├─┐┘┤"))
		if l != "" {
			parts = append(parts, l)
		}
	}
	return regexp.MustCompile(`([A-Za-z])- ([A-Za-z])`).ReplaceAllString(strings.Join(parts, " "), "$1-$2")
}

// ─── projects ────────────────────────────────────────────────────────────────

func TestRemove_AProjectThatOwnsContentIsRefusedWithoutAGate(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "busy")
	m, cmd := press(t, m, ch('d'))
	if m.state != viewProjects || isQuit(cmd) {
		t.Fatalf("state = %s; a refusal should leave the list", viewStateNames[m.state])
	}
	v := m.View()
	for _, want := range []string{"busy", "owns", "1 observation", "nothing was deleted"} {
		if !strings.Contains(v, want) {
			t.Errorf("the refusal lacks %q:\n%s", want, v)
		}
	}
	if p, _ := m.kb.ProjectByName("busy"); p == nil {
		t.Error("the project was deleted")
	}
}

func TestRemove_TheGateShowsThePlanAndAsksForTheName(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "empty")
	m, _ = press(t, m, ch('d'))
	if m.state != viewRemove || !m.capturingText() {
		t.Fatalf("state %s, capturing %v; want the gate, taking text", viewStateNames[m.state], m.capturingText())
	}
	v := m.View()
	for _, want := range []string{"empty", "1 concept link", "Type empty", "local", "Esc cancel"} {
		if !strings.Contains(v, want) {
			t.Errorf("the gate lacks %q:\n%s", want, v)
		}
	}
}

func TestRemove_TypingTheNameDeletesAndShowsTheCommand(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "empty")
	m, _ = press(t, m, ch('d'))
	m, cmd := press(t, m, append(typed("empty"), keyEnter)...)
	if isQuit(cmd) || m.state != viewProjects {
		t.Fatalf("state %s (quit %v); want the project list", viewStateNames[m.state], isQuit(cmd))
	}
	if p, _ := m.kb.ProjectByName("empty"); p != nil {
		t.Error("the project is still there")
	}
	v := m.View()
	for _, want := range []string{`✓ deleted project "empty"`, "equivalent:  kb project delete empty --force"} {
		if !strings.Contains(v, want) {
			t.Errorf("the screen lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "stray project") {
		t.Errorf("the list still shows the project:\n%s", v)
	}
}

func TestRemove_AWrongNameDeletesNothing(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "empty")
	m, _ = press(t, m, ch('d'))
	m, cmd := press(t, m, append(typed("emptx"), keyEnter)...)
	if m.state != viewRemove || isQuit(cmd) {
		t.Fatalf("state %s; a wrong name should stay in the gate", viewStateNames[m.state])
	}
	if p, _ := m.kb.ProjectByName("empty"); p == nil {
		t.Error("a wrong name deleted the project")
	}
	if !strings.Contains(m.View(), "does not match") {
		t.Errorf("the gate should say the name does not match:\n%s", m.View())
	}
}

func TestRemove_EscCancelsAndSaysSo(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "empty")
	m, _ = press(t, m, ch('d'))
	m, cmd := press(t, m, typed("emp")...)
	m, cmd = press(t, m, keyEsc)
	if m.state != viewProjects || isQuit(cmd) {
		t.Fatalf("state %s (quit %v); want the list", viewStateNames[m.state], isQuit(cmd))
	}
	if v := m.View(); !strings.Contains(v, "cancelled") || strings.Contains(v, "equivalent:") {
		t.Errorf("the screen should say cancelled and show no command:\n%s", v)
	}
	if p, _ := m.kb.ProjectByName("empty"); p == nil {
		t.Error("a cancel deleted the project")
	}
}

func TestRemove_CtrlCQuitsWithoutDeleting(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "empty")
	m, _ = press(t, m, ch('d'))
	m, _ = press(t, m, typed("empty")...)
	if _, cmd := press(t, m, keyCtrlC); !isQuit(cmd) {
		t.Error("Ctrl-C did not quit")
	}
	if p, _ := m.kb.ProjectByName("empty"); p == nil {
		t.Error("Ctrl-C deleted the project")
	}
}

// The edge clasm met: a name that starts with q. In the gate every command letter
// is text, so a project called quokka can be confirmed.
func TestRemove_ANameStartingWithQCanBeTyped(t *testing.T) {
	m := removeModel(t)
	if _, err := m.kb.AddProject("quokka", "starts with q"); err != nil {
		t.Fatal(err)
	}
	m, err := newTUIModelAt(m.kb, nil, tuiStart{state: viewProjects})
	if err != nil {
		t.Fatal(err)
	}
	m, _ = press(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	selectProject(t, m, "quokka")
	m, _ = press(t, m, ch('d'))
	m, cmd := press(t, m, typed("qjk")...) // command letters: text, and not yet a match
	if isQuit(cmd) || m.state != viewRemove {
		t.Fatalf("typing qjk acted as a command: state %s, quit %v", viewStateNames[m.state], isQuit(cmd))
	}
	m, _ = press(t, m, keyCtrlU())
	m, _ = press(t, m, append(typed("quokka"), keyEnter)...)
	if p, _ := m.kb.ProjectByName("quokka"); p != nil {
		t.Error("quokka was not deleted")
	}
}

func keyCtrlU() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlU} }

// ─── observations and concepts ───────────────────────────────────────────────

func TestRemove_AnObservationFromItsTab(t *testing.T) {
	m := removeModel(t)
	obs, _ := m.kb.Observations(mustProject(t, m, "alpha").ID)
	cid, _ := m.kb.AddConcept("tagged", "")
	if err := m.kb.LinkObservationConcept(obs[0].ID, cid); err != nil {
		t.Fatal(err)
	}
	selectProject(t, m, "alpha")
	m, _ = press(t, m, keyEnter) // observations tab
	m, _ = press(t, m, ch('d'))
	v := m.View()
	for _, want := range []string{"observation", "1 concept link", "Type 1"} {
		if !strings.Contains(v, want) {
			t.Errorf("the gate lacks %q:\n%s", want, v)
		}
	}
	m, _ = press(t, m, append(typed("1"), keyEnter)...)
	if m.state != viewObservations {
		t.Fatalf("state = %s, want the observations tab", viewStateNames[m.state])
	}
	if left, _ := m.kb.Observations(mustProject(t, m, "alpha").ID); len(left) != 0 {
		t.Errorf("%d observations left", len(left))
	}
	if !strings.Contains(m.View(), "equivalent:  kb observation delete 1 --force") {
		t.Errorf("the equivalent command is missing:\n%s", m.View())
	}
	if !strings.Contains(m.View(), "Observations (0)") {
		t.Errorf("the tab count was not refreshed:\n%s", m.View())
	}
}

func TestRemove_AConceptFromItsTab(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "alpha")
	m, _ = press(t, m, keyEnter, ch('c')) // concepts tab: streaming
	m, _ = press(t, m, ch('d'))
	v := m.View()
	for _, want := range []string{"streaming", "1 project", "Type streaming"} {
		if !strings.Contains(v, want) {
			t.Errorf("the gate lacks %q:\n%s", want, v)
		}
	}
	m, _ = press(t, m, append(typed("streaming"), keyEnter)...)
	if m.state != viewConcepts || !strings.Contains(m.View(), "equivalent:  kb concept delete streaming --force") {
		t.Errorf("state %s; the command is missing:\n%s", viewStateNames[m.state], m.View())
	}
	if cs, _ := m.kb.Concepts(); len(cs) > 0 {
		for _, c := range cs {
			if c.Name == "streaming" {
				t.Error("the concept is still there")
			}
		}
	}
}

func mustProject(t *testing.T, m *tuiModel, name string) knowledge.Project {
	t.Helper()
	p, err := m.kb.ProjectByName(name)
	if err != nil || p == nil {
		t.Fatalf("project %s: %v", name, err)
	}
	return *p
}

// ─── records ─────────────────────────────────────────────────────────────────

// A record row can be dropped only when its file is gone; while it exists the TUI
// shows the CLI's refusal and the way out, and there is no gate.
func TestRemove_ARecordWhoseFileExistsIsRefused(t *testing.T) {
	m, _ := writeModel(t)
	selectRef(t, m, "clasm/DR-0001")
	m, _ = press(t, m, ch('d'))
	if m.state != viewRecordScope {
		t.Fatalf("state = %s; a refusal should stay on the Records screen", viewStateNames[m.state])
	}
	v := unwrapped(m.View())
	for _, want := range []string{"file still exists", "nothing was deleted", "set-status clasm/DR-0001 cancelled"} {
		if !strings.Contains(v, want) {
			t.Errorf("the refusal lacks %q:\n%s", want, v)
		}
	}
}

func TestRemove_ARecordWhoseFileIsGoneNamesWhatPointsAtIt(t *testing.T) {
	m, root := writeModel(t) // 0004 supersedes 0002 and 0003; 0002 and 0003 carry superseded_by 0004
	if err := os.Remove(root + "/clasm/decisions/0002-fixture.md"); err != nil {
		t.Fatal(err)
	}
	selectRef(t, m, "clasm/DR-0002")
	m, _ = press(t, m, ch('d'))
	if m.state != viewRemove {
		t.Fatalf("state = %s; want the gate (the file is gone)", viewStateNames[m.state])
	}
	v := m.View()
	for _, want := range []string{"clasm/DR-0002", "clasm/DR-0004", "Type clasm/DR-0002", "relation"} {
		if !strings.Contains(v, want) {
			t.Errorf("the gate lacks %q (it should name what points at the record):\n%s", want, v)
		}
	}
	m, _ = press(t, m, append(typed("clasm/DR-0002"), keyEnter)...)
	if m.state != viewRecordScope {
		t.Fatalf("state = %s, want the Records screen", viewStateNames[m.state])
	}
	v = m.View()
	if strings.Contains(v, "clasm/DR-0002 ") && strings.Contains(v, "The accepted one") {
		t.Errorf("the record is still listed:\n%s", v)
	}
	for _, want := range []string{"equivalent:  kb record delete clasm/DR-0002", "(3)"} {
		if !strings.Contains(v, want) {
			t.Errorf("the screen lacks %q:\n%s", want, v)
		}
	}
}

// ─── the legends ─────────────────────────────────────────────────────────────

func TestRemove_TheLegendsOfferDeleteWhereItWorks(t *testing.T) {
	m := removeModel(t)
	if !strings.Contains(m.View(), "d delete") {
		t.Errorf("the project list legend lacks d delete:\n%s", m.View())
	}
	selectProject(t, m, "alpha")
	m, _ = press(t, m, keyEnter)
	for _, state := range []viewState{viewObservations, viewConcepts, viewRecords} {
		m.setState(state)
		if !strings.Contains(m.View(), "d delete") {
			t.Errorf("%s: the legend lacks d delete:\n%s", viewStateNames[state], m.View())
		}
	}
	w, _ := writeModel(t)
	if !strings.Contains(w.View(), "d delete") {
		t.Errorf("the Records screen legend lacks d delete:\n%s", w.View())
	}
}

// ─── the real binary ─────────────────────────────────────────────────────────

// A stray project called quokka, deleted by typing its name on a real terminal:
// the first key typed is a q, which is text here and a command everywhere else.
func TestBinary_TUIDeleteAProjectByTypingItsName(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	kb, root := openWorkspaceKB(t)
	if _, err := kb.AddProject("quokka", "a stray project"); err != nil {
		t.Fatal(err)
	}
	master, screen, wait := startOnATerminal(t, root, "-i", "project", "list")
	screen.waitFor(t, "a stray project")
	master.WriteString("d")
	screen.waitFor(t, "Type quokka")
	master.WriteString("quokka") // a name that starts with q: text, not a command
	screen.waitFor(t, "quokka to confirm: quokka")
	master.WriteString("\r")
	screen.waitFor(t, "✓ deleted project")
	screen.waitFor(t, "equivalent:  kb project delete quokka")
	if p, _ := kb.ProjectByName("quokka"); p != nil {
		t.Error("the project is still there")
	}
	master.WriteString("qqq")
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0:\n%s", code, screen.text())
	}
}

func TestBinary_TUIDeleteEscCancelsWithNoEnter(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	kb, root := openWorkspaceKB(t)
	if _, err := kb.AddProject("stray", "a stray project"); err != nil {
		t.Fatal(err)
	}
	master, screen, wait := startOnATerminal(t, root, "-i", "project", "list")
	screen.waitFor(t, "a stray project")
	master.WriteString("d")
	screen.waitFor(t, "Type stray")
	master.WriteString("str")
	master.WriteString("\x1b") // a lone Esc
	screen.waitFor(t, "cancelled; project \"stray\" is unchanged")
	if p, _ := kb.ProjectByName("stray"); p == nil {
		t.Error("a cancel deleted the project")
	}
	master.WriteString("qqq")
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}
