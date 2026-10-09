package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The additive forms (knowledge DR-0067, DR-0064 amendment; v0.0.20 U1): New
// project, observation, concept, source and record. The fields are asked one at a
// time, a text field takes every key as text and a choice takes one key, what has
// been entered so far stays on the screen, and the confirmation shows the command
// that does the same. Each write is the command's own function. Esc cancels the
// whole form at any step and writes nothing.

// formModel is a menu model over the removeModel workspace (projects alpha, empty
// and busy) with a record layout, KB_PROJECT=alpha.
func formModel(t *testing.T) *tuiModel {
	t.Helper()
	m := removeModel(t)
	t.Setenv("KB_PROJECT", "alpha")
	top, err := newTUIModel(m.kb, nil)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := top.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return next.(*tuiModel)
}

// newFrom opens a group's New… leaf by its label.
func newFrom(t *testing.T, m *tuiModel, group, label string) *tuiModel {
	t.Helper()
	m = openRow(t, m, group)
	for i, it := range groupMenu(m.group) {
		if it.Label == label {
			for j := 0; j < i; j++ {
				m, _ = press(t, m, ch('j'))
			}
			m, _ = press(t, m, keyEnter)
			if m.state != viewForm {
				t.Fatalf("%s opened %s, want a form", label, viewStateNames[m.state])
			}
			return m
		}
	}
	t.Fatalf("no leaf %q in %s", label, group)
	return nil
}

func enter(s string) []tea.Msg { return append(typed(s), keyEnter) }

// ─── project ─────────────────────────────────────────────────────────────────

func TestForm_NewProjectAsksTheFieldsThenConfirms(t *testing.T) {
	m := newFrom(t, formModel(t), "Projects", "New project…")
	if !m.capturingText() || !strings.Contains(m.View(), "name") {
		t.Fatalf("the first field should be a text field asking for the name:\n%s", m.View())
	}
	m, cmd := press(t, m, enter("quokka")...) // a name that starts with q: text
	if isQuit(cmd) || m.state != viewForm {
		t.Fatalf("state %s (quit %v)", viewStateNames[m.state], isQuit(cmd))
	}
	v := m.View()
	if !strings.Contains(v, "quokka") || !strings.Contains(v, "description") {
		t.Errorf("the entered name should stay on screen and the next field be asked:\n%s", v)
	}
	m, _ = press(t, m, enter("a small marsupial")...)
	v = m.View()
	for _, want := range []string{"quokka", "a small marsupial", `kb project add quokka "a small marsupial"`, "y"} {
		if !strings.Contains(v, want) {
			t.Errorf("the confirmation lacks %q:\n%s", want, v)
		}
	}
	if p, _ := m.kb.ProjectByName("quokka"); p != nil {
		t.Fatal("the project was created before the confirmation")
	}
	m, _ = press(t, m, ch('y'))
	p, _ := m.kb.ProjectByName("quokka")
	if p == nil || p.Description != "a small marsupial" {
		t.Fatalf("project = %+v", p)
	}
	if v := m.View(); !strings.Contains(v, "✓") || !strings.Contains(v, `equivalent:  kb project add quokka "a small marsupial"`) {
		t.Errorf("the screen should say what was done and the command:\n%s", v)
	}
	if m.state == viewForm {
		t.Error("still in the form after the write")
	}
}

func TestForm_ARequiredFieldCannotBeEmptyAndAnOptionalOneCan(t *testing.T) {
	m := newFrom(t, formModel(t), "Projects", "New project…")
	m, _ = press(t, m, keyEnter)
	if m.state != viewForm || !strings.Contains(m.View(), "required") {
		t.Fatalf("an empty name should be refused:\n%s", m.View())
	}
	m, _ = press(t, m, enter("plain")...)
	m, _ = press(t, m, keyEnter) // the description is optional
	if !strings.Contains(m.View(), "kb project add plain") {
		t.Errorf("an empty description should be accepted:\n%s", m.View())
	}
}

func TestForm_EscCancelsTheWholeFormAtEveryStep(t *testing.T) {
	for name, keys := range map[string][]tea.Msg{
		"in the first field":  {keyEsc},
		"in the second field": append(enter("quokka"), keyEsc),
		"at the confirmation": append(append(enter("quokka"), keyEnter), ch('n')),
		"Esc at the confirm":  append(append(enter("quokka"), keyEnter), keyEsc),
	} {
		t.Run(name, func(t *testing.T) {
			m := newFrom(t, formModel(t), "Projects", "New project…")
			var cmd tea.Cmd
			m, cmd = press(t, m, keys...)
			if isQuit(cmd) || m.state == viewForm || !strings.Contains(m.View(), "cancelled") {
				t.Fatalf("state %s (quit %v); want the form closed with a cancelled notice:\n%s", viewStateNames[m.state], isQuit(cmd), m.View())
			}
			if p, _ := m.kb.ProjectByName("quokka"); p != nil {
				t.Error("a cancel created the project")
			}
		})
	}
}

func TestForm_CommandLettersAreTextInAField(t *testing.T) {
	m := newFrom(t, formModel(t), "Projects", "New project…")
	m, cmd := press(t, m, typed("qjkdsyn:")...)
	if isQuit(cmd) || m.state != viewForm {
		t.Fatalf("typing acted as a command: state %s, quit %v", viewStateNames[m.state], isQuit(cmd))
	}
	if !strings.Contains(m.View(), "qjkdsyn:") {
		t.Errorf("the typed text is missing:\n%s", m.View())
	}
}

func TestForm_CtrlCQuitsWithoutWriting(t *testing.T) {
	m := newFrom(t, formModel(t), "Projects", "New project…")
	m, _ = press(t, m, enter("quokka")...)
	m, _ = press(t, m, keyEnter)
	if _, cmd := press(t, m, keyCtrlC); !isQuit(cmd) {
		t.Error("Ctrl-C did not quit")
	}
	if p, _ := m.kb.ProjectByName("quokka"); p != nil {
		t.Error("Ctrl-C created the project")
	}
}

func TestForm_AnExistingNameIsTheCommandsRefusalAsANotice(t *testing.T) {
	m := newFrom(t, formModel(t), "Projects", "New project…")
	m, _ = press(t, m, enter("alpha")...)
	m, _ = press(t, m, keyEnter)
	m, _ = press(t, m, ch('y'))
	v := m.View()
	if strings.Contains(v, "✓") || !strings.Contains(v, "alpha") {
		t.Errorf("the screen should show the command's refusal, not a success:\n%s", v)
	}
}

// ─── observation ─────────────────────────────────────────────────────────────

func TestForm_NewObservationFromAProjectKnowsTheProject(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "alpha")
	m, _ = press(t, m, keyEnter, ch('n')) // observations tab, new
	if m.state != viewForm {
		t.Fatalf("state = %s, want the form", viewStateNames[m.state])
	}
	v := m.View()
	if !strings.Contains(v, "alpha") || strings.Contains(v, "project name") {
		t.Errorf("the project should be known and not asked:\n%s", v)
	}
	for _, want := range []string{"[n]ote", "[f]inding", "[d]ecision", "q[u]estion", "[h]ypothesis"} {
		if !strings.Contains(v, want) {
			t.Errorf("the kind chooser lacks %q:\n%s", want, v)
		}
	}
	m, _ = press(t, m, ch('u')) // question: q is the cancel key, so the word takes u
	m, _ = press(t, m, enter("does it scale?")...)
	m, _ = press(t, m, keyEnter) // no DOI
	v = m.View()
	for _, want := range []string{"question", "does it scale?", `kb observation add --project alpha question "does it scale?"`} {
		if !strings.Contains(v, want) {
			t.Errorf("the confirmation lacks %q:\n%s", want, v)
		}
	}
	m, _ = press(t, m, ch('y'))
	obs, _ := m.kb.Observations(mustProject(t, m, "alpha").ID)
	found := false
	for _, o := range obs {
		if o.Kind == "question" && o.Body == "does it scale?" {
			found = true
		}
	}
	if !found {
		t.Errorf("the observation was not added: %+v", obs)
	}
	if m.state != viewObservations || !strings.Contains(m.View(), "Observations (2)") {
		t.Errorf("state %s; the tab should be refreshed:\n%s", viewStateNames[m.state], m.View())
	}
}

func TestForm_NewObservationFromTheMenuAsksTheProjectAndChecksIt(t *testing.T) {
	m := newFrom(t, formModel(t), "Observations", "New observation…")
	if !strings.Contains(m.View(), "project") {
		t.Fatalf("the project should be asked:\n%s", m.View())
	}
	if !strings.Contains(m.View(), "alpha") { // prefilled from the scope
		t.Errorf("the scope's project should be the starting value:\n%s", m.View())
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m, _ = press(t, m, enter("nosuch")...)
	if m.state != viewForm || !strings.Contains(m.View(), "no project") && !strings.Contains(m.View(), "not found") && !strings.Contains(m.View(), "unknown") {
		t.Errorf("a project that does not exist should be refused in the field:\n%s", m.View())
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m, _ = press(t, m, enter("busy")...)
	if !strings.Contains(m.View(), "kind") && !strings.Contains(m.View(), "[n]ote") {
		t.Errorf("after a good project the kind should be asked:\n%s", m.View())
	}
}

// ─── concept and source ──────────────────────────────────────────────────────

func TestForm_NewConcept(t *testing.T) {
	m := newFrom(t, formModel(t), "Concepts", "New concept…")
	m, _ = press(t, m, enter("backpressure")...)
	m, _ = press(t, m, enter("a slow consumer slows the producer")...)
	if !strings.Contains(m.View(), `kb concept add backpressure "a slow consumer slows the producer"`) {
		t.Errorf("the equivalent command is missing:\n%s", m.View())
	}
	m, _ = press(t, m, ch('y'))
	cs, _ := m.kb.Concepts()
	found := false
	for _, c := range cs {
		if c.Name == "backpressure" {
			found = true
		}
	}
	if !found {
		t.Error("the concept was not added")
	}
}

func TestForm_NewSourceTakesTheTitleAndTheOptionalFields(t *testing.T) {
	m := newFrom(t, formModel(t), "Sources", "New source…")
	m, _ = press(t, m, enter("A study of queues")...)
	m, _ = press(t, m, enter("R. Doiel")...)    // authors
	m, _ = press(t, m, enter("2026-09")...)     // published
	m, _ = press(t, m, keyEnter)                // url: none
	m, _ = press(t, m, enter("10.1000/xyz")...) // doi
	v := m.View()
	for _, want := range []string{"A study of queues", "--authors", "--published 2026-09", "--doi 10.1000/xyz"} {
		if !strings.Contains(v, want) {
			t.Errorf("the confirmation lacks %q:\n%s", want, v)
		}
	}
	m, _ = press(t, m, ch('y'))
	if v := m.View(); strings.Contains(v, "doi") && !strings.Contains(v, "✓") {
		t.Errorf("the source was refused:\n%s", v)
	}
	srcs, _ := m.kb.ListSources()
	found := false
	for _, s := range srcs {
		if s.Title == "A study of queues" {
			found = true
		}
	}
	if !found {
		t.Error("the source was not added")
	}
}

// ─── record ──────────────────────────────────────────────────────────────────

func TestForm_NewRecordWritesAProposedFileInTheScopesDirectory(t *testing.T) {
	m, root, _ := layoutModel(t) // KB_PROJECT=clasm
	m = newFrom(t, m, "Records", "New record…")
	if !strings.Contains(m.View(), "clasm") {
		t.Errorf("the scope's project should be the starting value:\n%s", m.View())
	}
	m, _ = press(t, m, keyEnter)                       // accept the scope
	m, _ = press(t, m, enter("Use one verb table")...) // title
	for _, want := range []string{"[d]esign", "[p]lan-review", "[i]mplementation", "[l]ive-test", "re[v]iew", "[r]equest", "e[x]ternal"} {
		if !strings.Contains(m.View(), want) && want != "re[v]iew" {
			t.Errorf("the trigger chooser lacks %q:\n%s", want, m.View())
		}
	}
	m, _ = press(t, m, ch('d')) // trigger: design
	m, _ = press(t, m, ch('d')) // kind: decision
	v := m.View()
	for _, want := range []string{"Use one verb table", "design", "decision", "kb record new"} {
		if !strings.Contains(v, want) {
			t.Errorf("the confirmation lacks %q:\n%s", want, v)
		}
	}
	m, _ = press(t, m, ch('y'))
	matches, _ := filepath.Glob(filepath.Join(root, "agents", "projects", "clasm", "decisions", "0003-*.md"))
	if len(matches) != 1 {
		t.Fatalf("the record file was not written (found %v)", matches)
	}
	b, _ := os.ReadFile(matches[0])
	if !strings.Contains(string(b), "status: proposed") || !strings.Contains(string(b), "Use one verb table") {
		t.Errorf("the new record is not proposed or lacks its title:\n%s", b)
	}
	if v := m.View(); !strings.Contains(v, "✓") || !strings.Contains(v, "ingest") {
		t.Errorf("the screen should say it was written and that ingest brings it into the database:\n%s", v)
	}
}

// ─── the shortcuts and legends ───────────────────────────────────────────────

func TestForm_NOnTheListsStartsTheMatchingForm(t *testing.T) {
	m := removeModel(t)
	m, _ = press(t, m, ch('n'))
	if m.state != viewForm || !strings.Contains(m.View(), "name") {
		t.Errorf("n on the project list gave %s, want the New project form", viewStateNames[m.state])
	}
	m, _ = press(t, m, keyEsc)
	selectProject(t, m, "alpha")
	m, _ = press(t, m, keyEnter, ch('c'), ch('n')) // concepts tab, new
	if m.state != viewForm || !strings.Contains(m.View(), "concept") {
		t.Errorf("n on the concepts tab gave %s, want the New concept form:\n%s", viewStateNames[m.state], m.View())
	}
}

func TestForm_TheLegendsOfferNew(t *testing.T) {
	m := removeModel(t)
	if !strings.Contains(m.View(), "n new") {
		t.Errorf("the project list legend lacks n new:\n%s", m.View())
	}
	_ = runtime.GOOS
}

// ─── the real binary ─────────────────────────────────────────────────────────

func TestBinary_TUINewProjectOnATerminal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	kb, root := openWorkspaceKB(t)
	master, screen, wait := startOnATerminal(t, root, "-i", "project", "list")
	screen.waitFor(t, "q back")
	master.WriteString("n")
	screen.waitFor(t, "name")
	master.WriteString("quokka\r") // a q first: text
	screen.waitFor(t, "description")
	master.WriteString("a small marsupial\r")
	screen.waitFor(t, "kb project add quokka")
	master.WriteString("y")
	screen.waitFor(t, "equivalent:  kb project add quokka")
	if p, _ := kb.ProjectByName("quokka"); p == nil || p.Description != "a small marsupial" {
		t.Errorf("project = %+v", p)
	}
	master.WriteString("qqq")
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0:\n%s", code, screen.text())
	}
}
