package main

import (
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The changing writes (knowledge DR-0067, DR-0064 amendment; v0.0.20 U2): show the
// item, take the new value (a key for a status, typed text for a name or a
// description), then confirm old to new. Each write is the command's own function,
// so its rules and its errors are the command's. Esc cancels at every step, and
// while text is typed every key is text.

func ctrlJ() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlJ} }

// projectStatus reads a project's status from the database.
func projectStatus(t *testing.T, m *tuiModel, name string) string {
	t.Helper()
	p, err := m.kb.ProjectByName(name)
	if err != nil || p == nil {
		t.Fatalf("project %q: %v", name, err)
	}
	return p.Status
}

// ─── status ──────────────────────────────────────────────────────────────────

func TestChange_ProjectStatusIsPickedThenConfirmedOldToNew(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "empty")
	was := projectStatus(t, m, "empty")
	m, _ = press(t, m, ch('s'))
	if m.state != viewChange {
		t.Fatalf("state = %s, want the change screen", viewStateNames[m.state])
	}
	v := m.View()
	if !strings.Contains(v, "empty") || !strings.Contains(v, was) {
		t.Errorf("the screen should show the project and its status %q:\n%s", was, v)
	}
	// the statuses offered are the others, each on its own key
	for _, want := range []string{"[c]oncept", "[p]aused", "conc[l]uded"} {
		if !strings.Contains(v, want) {
			t.Errorf("the chooser lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "[a]ctive") {
		t.Errorf("the chooser offers the status the project already has:\n%s", v)
	}
	m, _ = press(t, m, ch('p'))
	v = m.View()
	for _, want := range []string{was, "paused", "y apply"} {
		if !strings.Contains(v, want) {
			t.Errorf("the confirmation lacks %q:\n%s", want, v)
		}
	}
	if projectStatus(t, m, "empty") != was {
		t.Fatal("the status changed before the confirmation")
	}
	m, _ = press(t, m, ch('y'))
	if m.state != viewProjects {
		t.Errorf("state = %s, want the project list", viewStateNames[m.state])
	}
	if got := projectStatus(t, m, "empty"); got != "paused" {
		t.Errorf("status = %q, want paused", got)
	}
	v = m.View()
	for _, want := range []string{`✓ project "empty" status set to paused`, "equivalent:  kb project set-status empty paused"} {
		if !strings.Contains(v, want) {
			t.Errorf("the screen lacks %q:\n%s", want, v)
		}
	}
}

func TestChange_EveryCancelChangesNothing(t *testing.T) {
	for name, keys := range map[string][]tea.Msg{
		"q at the choice": {ch('q')}, "Esc at the choice": {keyEsc},
		"n at the confirmation": {ch('p'), ch('n')}, "Esc at the confirmation": {ch('p'), keyEsc},
	} {
		t.Run(name, func(t *testing.T) {
			m := removeModel(t)
			selectProject(t, m, "empty")
			was := projectStatus(t, m, "empty")
			m, _ = press(t, m, ch('s'))
			var cmd tea.Cmd
			m, cmd = press(t, m, keys...)
			if isQuit(cmd) || m.state != viewProjects {
				t.Fatalf("state %s (quit %v), want the list", viewStateNames[m.state], isQuit(cmd))
			}
			if projectStatus(t, m, "empty") != was {
				t.Error("a cancel changed the status")
			}
			if v := m.View(); !strings.Contains(v, "cancelled") || strings.Contains(v, "equivalent:") {
				t.Errorf("the screen should say cancelled and show no command:\n%s", v)
			}
		})
	}
}

func TestChange_EnterDoesNotConfirmAndCtrlCQuitsWithoutWriting(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "empty")
	was := projectStatus(t, m, "empty")
	m, _ = press(t, m, ch('s'), ch('p'), keyEnter)
	if m.state != viewChange {
		t.Fatalf("Enter left the confirmation: %s", viewStateNames[m.state])
	}
	if _, cmd := press(t, m, keyCtrlC); !isQuit(cmd) {
		t.Error("Ctrl-C did not quit")
	}
	if projectStatus(t, m, "empty") != was {
		t.Error("something was written")
	}
}

// ─── text: description and rename ────────────────────────────────────────────

func TestChange_ADescriptionIsEditedFromItsCurrentText(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "empty")
	m, _ = press(t, m, ch('e'))
	if m.state != viewChange || !m.capturingText() {
		t.Fatalf("state %s, capturing %v; want the editor, taking text", viewStateNames[m.state], m.capturingText())
	}
	if !strings.Contains(m.View(), "a stray project") {
		t.Errorf("the editor should start from the current text:\n%s", m.View())
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlU}) // clear the line
	m, _ = press(t, m, append(typed("a better description"), keyEnter)...)
	v := m.View()
	for _, want := range []string{"a stray project", "a better description", "y apply"} {
		if !strings.Contains(v, want) {
			t.Errorf("the confirmation lacks %q:\n%s", want, v)
		}
	}
	m, _ = press(t, m, ch('y'))
	p, _ := m.kb.ProjectByName("empty")
	if p.Description != "a better description" {
		t.Errorf("description = %q", p.Description)
	}
	if !strings.Contains(m.View(), `equivalent:  kb project set-description empty "a better description"`) {
		t.Errorf("the equivalent command is missing:\n%s", m.View())
	}
}

func TestChange_AnUnchangedOrEmptyEditStaysInTheEditor(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "empty")
	m, _ = press(t, m, ch('r'))  // rename
	m, _ = press(t, m, keyEnter) // unchanged
	if m.state != viewChange || !strings.Contains(m.View(), "unchanged") {
		t.Errorf("state %s; an unchanged name should say so and stay:\n%s", viewStateNames[m.state], m.View())
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlU}, keyEnter) // empty
	if m.state != viewChange || !strings.Contains(m.View(), "empty") {
		t.Errorf("state %s; an empty name should be refused:\n%s", viewStateNames[m.state], m.View())
	}
}

func TestChange_ARenameIsTheCommandsOwnAndItsErrorIsANotice(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "empty")
	m, _ = press(t, m, ch('r'), tea.KeyMsg{Type: tea.KeyCtrlU})
	m, _ = press(t, m, append(typed("alpha"), keyEnter, ch('y'))...) // a name that is taken
	if m.state != viewProjects {
		t.Fatalf("state = %s, want the list", viewStateNames[m.state])
	}
	if p, _ := m.kb.ProjectByName("empty"); p == nil {
		t.Error("the project was renamed onto a taken name")
	}
	if v := m.View(); strings.Contains(v, "✓") || !strings.Contains(v, "alpha") {
		t.Errorf("the screen should show the command's refusal, not a success:\n%s", v)
	}
}

func TestChange_ARenameThatIsFreeWorks(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "empty")
	m, _ = press(t, m, ch('r'), tea.KeyMsg{Type: tea.KeyCtrlU})
	m, _ = press(t, m, append(typed("quokka"), keyEnter, ch('y'))...) // starts with q: text
	if p, _ := m.kb.ProjectByName("quokka"); p == nil {
		t.Fatal("the project was not renamed")
	}
	if !strings.Contains(m.View(), "equivalent:  kb project rename empty quokka") {
		t.Errorf("the equivalent command is missing:\n%s", m.View())
	}
}

// Text is text: the command letters do nothing in the editor, and Esc leaves it.
func TestChange_CommandLettersAreTextInTheEditor(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "empty")
	m, _ = press(t, m, ch('e'), tea.KeyMsg{Type: tea.KeyCtrlU})
	m, cmd := press(t, m, typed("qjkdsyn:")...)
	if isQuit(cmd) || m.state != viewChange {
		t.Fatalf("typing acted as a command: state %s, quit %v", viewStateNames[m.state], isQuit(cmd))
	}
	m, cmd = press(t, m, keyEsc)
	if isQuit(cmd) || m.state != viewProjects || !strings.Contains(m.View(), "cancelled") {
		t.Errorf("Esc: state %s, quit %v", viewStateNames[m.state], isQuit(cmd))
	}
	p, _ := m.kb.ProjectByName("empty")
	if p.Description != "a stray project" {
		t.Errorf("description changed to %q", p.Description)
	}
}

// ─── observation text ────────────────────────────────────────────────────────

func TestChange_AnObservationIsUpdatedAndMayHaveSeveralLines(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "alpha")
	m, _ = press(t, m, keyEnter, ch('e')) // observations tab, edit
	if m.state != viewChange || !strings.Contains(m.View(), "a specific observation body") {
		t.Fatalf("state %s; the editor should start from the observation:\n%s", viewStateNames[m.state], m.View())
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m, _ = press(t, m, typed("first line")...)
	m, _ = press(t, m, ctrlJ()) // a newline; Enter submits
	m, _ = press(t, m, append(typed("second line"), keyEnter)...)
	if v := m.View(); !strings.Contains(v, "supersed") || !strings.Contains(v, "original wording is kept") {
		t.Errorf("the confirmation should say that this corrects by superseding and keeps the original:\n%s", v)
	}
	m, _ = press(t, m, ch('y'))
	if m.state != viewObservations {
		t.Fatalf("state = %s, want the observations tab", viewStateNames[m.state])
	}
	obs, _ := m.kb.Observations(mustProject(t, m, "alpha").ID)
	var corrected string
	for _, o := range obs {
		if o.ID == 1 && o.Body != "a specific observation body" {
			t.Errorf("the original was changed to %q", o.Body)
		}
		if o.Body == "first line\nsecond line" {
			corrected = o.Body
		}
	}
	if corrected == "" || len(obs) != 2 {
		t.Errorf("observations = %+v, want the original and a new one with the two lines", obs)
	}
	v := m.View()
	for _, want := range []string{"observation 1 corrected by observation", "supersedes it", "equivalent:  kb observation update 1", "Observations (2)"} {
		if !strings.Contains(v, want) {
			t.Errorf("the screen lacks %q:\n%s", want, v)
		}
	}
}

// ─── concepts ────────────────────────────────────────────────────────────────

func TestChange_AConceptIsRenamedFromItsTab(t *testing.T) {
	m := removeModel(t)
	selectProject(t, m, "alpha")
	m, _ = press(t, m, keyEnter, ch('c'), ch('e')) // concepts tab, edit its name
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m, _ = press(t, m, append(typed("event-streaming"), keyEnter, ch('y'))...)
	if m.state != viewConcepts {
		t.Fatalf("state = %s, want the concepts tab", viewStateNames[m.state])
	}
	found := false
	cs, _ := m.kb.Concepts()
	for _, c := range cs {
		if c.Name == "event-streaming" {
			found = true
		}
	}
	if !found {
		t.Error("the concept was not renamed")
	}
	if !strings.Contains(m.View(), "equivalent:  kb concept rename streaming event-streaming") {
		t.Errorf("the equivalent command is missing:\n%s", m.View())
	}
}

// ─── the legends ─────────────────────────────────────────────────────────────

func TestChange_TheLegendsOfferTheKeys(t *testing.T) {
	m := removeModel(t)
	for _, want := range []string{"s status", "e describe", "r rename"} {
		if !strings.Contains(m.View(), want) {
			t.Errorf("the project list legend lacks %q:\n%s", want, m.View())
		}
	}
	selectProject(t, m, "alpha")
	m, _ = press(t, m, keyEnter)
	if !strings.Contains(m.View(), "e edit") {
		t.Errorf("the observations legend lacks e edit:\n%s", m.View())
	}
	m, _ = press(t, m, ch('c'))
	if !strings.Contains(m.View(), "e rename") {
		t.Errorf("the concepts legend lacks e rename:\n%s", m.View())
	}
}

// ─── the real binary ─────────────────────────────────────────────────────────

func TestBinary_TUIProjectStatusIsPickedWithOneKey(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	kb, root := openWorkspaceKB(t)
	if _, err := kb.AddProject("stray", "a stray project"); err != nil {
		t.Fatal(err)
	}
	master, screen, wait := startOnATerminal(t, root, "-i", "project", "list")
	screen.waitFor(t, "a stray project")
	master.WriteString("s")
	screen.waitFor(t, "conc[l]uded")
	master.WriteString("p") // no Enter
	screen.waitFor(t, "y apply")
	master.WriteString("y")
	screen.waitFor(t, "equivalent:  kb project set-status stray paused")
	if p, _ := kb.ProjectByName("stray"); p == nil || p.Status != "paused" {
		t.Errorf("project = %+v, want paused", p)
	}
	master.WriteString("qqq")
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0:\n%s", code, screen.text())
	}
}

// Editing text in raw mode: the field starts from the old text, Ctrl-U clears the
// line, typed words (q and y among them) are text, and Enter then y writes.
func TestBinary_TUIEditADescriptionOnATerminal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	kb, root := openWorkspaceKB(t)
	if _, err := kb.AddProject("stray", "a stray project"); err != nil {
		t.Fatal(err)
	}
	master, screen, wait := startOnATerminal(t, root, "-i", "project", "list")
	screen.waitFor(t, "a stray project")
	master.WriteString("e")
	screen.waitFor(t, "Enter accept")
	master.WriteString("\x15") // Ctrl-U
	master.WriteString("quiet, yes: a kept name")
	master.WriteString("\r")
	screen.waitFor(t, "y apply")
	master.WriteString("y")
	screen.waitFor(t, "equivalent:  kb project set-description stray")
	if p, _ := kb.ProjectByName("stray"); p == nil || p.Description != "quiet, yes: a kept name" {
		t.Errorf("project = %+v", p)
	}
	master.WriteString("qqq")
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0:\n%s", code, screen.text())
	}
}

func TestBinary_TUIEditEscCancelsWithNoEnter(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	kb, root := openWorkspaceKB(t)
	if _, err := kb.AddProject("stray", "a stray project"); err != nil {
		t.Fatal(err)
	}
	master, screen, wait := startOnATerminal(t, root, "-i", "project", "list")
	screen.waitFor(t, "a stray project")
	master.WriteString("r")
	screen.waitFor(t, "Enter accept")
	master.WriteString("xyz")
	master.WriteString("\x1b") // a lone Esc
	screen.waitFor(t, "cancelled; project \"stray\" is unchanged")
	if p, _ := kb.ProjectByName("stray"); p == nil {
		t.Error("a cancel renamed the project")
	}
	master.WriteString("qqq")
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}

// ─── supersede ───────────────────────────────────────────────────────────────

// fileHas reports whether a record's file contains text.
func fileHas(t *testing.T, root, id, text string) bool {
	t.Helper()
	return strings.Contains(readFixture(t, root, "clasm", id), text)
}

func TestChange_ARecordSupersedesAnotherAndBothSidesAreWritten(t *testing.T) {
	fakePagerSeam(t)
	m, root := writeModel(t) // 0001 proposed; 0002 accepted
	selectRef(t, m, "clasm/DR-0001")
	m, _ = press(t, m, ch('u'))
	if m.state != viewChange || !m.capturingText() {
		t.Fatalf("state %s, capturing %v; want the field, taking text", viewStateNames[m.state], m.capturingText())
	}
	v := m.View()
	if !strings.Contains(v, "clasm/DR-0001") || !strings.Contains(v, "clasm/") {
		t.Errorf("the screen should name the record and start the reference with its scope:\n%s", v)
	}
	m, _ = press(t, m, append(typed("0002"), keyEnter)...)
	v = m.View()
	for _, want := range []string{"clasm/DR-0001", "clasm/DR-0002", "supersedes", "becomes superseded", "y apply"} {
		if !strings.Contains(v, want) {
			t.Errorf("the confirmation lacks %q:\n%s", want, v)
		}
	}
	if fileHas(t, root, "0002", "status: superseded") {
		t.Fatal("something was written before the confirmation")
	}
	m, _ = press(t, m, ch('y'))
	if m.state != viewRecordScope {
		t.Fatalf("state = %s, want the Records screen", viewStateNames[m.state])
	}
	if !fileHas(t, root, "0002", "status: superseded") || !fileHas(t, root, "0002", "0001") {
		t.Errorf("the old record was not marked superseded by DR-0001:\n%s", readFixture(t, root, "clasm", "0002"))
	}
	if !fileHas(t, root, "0001", "supersedes: [") || !fileHas(t, root, "0001", "0002") {
		t.Errorf("the new record does not carry supersedes DR-0002:\n%s", readFixture(t, root, "clasm", "0001"))
	}
	v = m.View()
	for _, want := range []string{"✓", "equivalent:  kb record supersede clasm/DR-0001 clasm/DR-0002"} {
		if !strings.Contains(v, want) {
			t.Errorf("the screen lacks %q:\n%s", want, v)
		}
	}
}

// What the command refuses is refused in the field, before any confirmation, and
// the person stays there to correct it.
func TestChange_ASupersessionTheCommandRefusesStaysInTheField(t *testing.T) {
	for name, typedRef := range map[string]string{"itself": "0001", "a record that is not there": "9999"} {
		t.Run(name, func(t *testing.T) {
			fakePagerSeam(t)
			m, root := writeModel(t)
			before := readFixture(t, root, "clasm", "0001")
			selectRef(t, m, "clasm/DR-0001")
			m, _ = press(t, m, ch('u'))
			m, cmd := press(t, m, append(typed(typedRef), keyEnter)...)
			if m.state != viewChange || isQuit(cmd) {
				t.Fatalf("state %s; a refusal should stay in the field", viewStateNames[m.state])
			}
			if strings.Contains(m.View(), "y apply") {
				t.Errorf("a refused supersession reached the confirmation:\n%s", m.View())
			}
			if readFixture(t, root, "clasm", "0001") != before {
				t.Error("a refused supersession wrote")
			}
		})
	}
}

func TestChange_ASupersessionCanBeCancelledAtEveryStep(t *testing.T) {
	for name, keys := range map[string][]tea.Msg{
		"Esc in the field":        {keyEsc},
		"n at the confirmation":   append(typed("0002"), keyEnter, ch('n')),
		"Esc at the confirmation": append(typed("0002"), keyEnter, keyEsc),
	} {
		t.Run(name, func(t *testing.T) {
			fakePagerSeam(t)
			m, root := writeModel(t)
			b1, b2 := readFixture(t, root, "clasm", "0001"), readFixture(t, root, "clasm", "0002")
			selectRef(t, m, "clasm/DR-0001")
			m, _ = press(t, m, ch('u'))
			m, cmd := press(t, m, keys...)
			if isQuit(cmd) || m.state != viewRecordScope || !strings.Contains(m.View(), "cancelled") {
				t.Fatalf("state %s (quit %v); want the Records screen and a cancelled notice:\n%s", viewStateNames[m.state], isQuit(cmd), m.View())
			}
			if readFixture(t, root, "clasm", "0001") != b1 || readFixture(t, root, "clasm", "0002") != b2 {
				t.Error("a cancel wrote")
			}
		})
	}
}

func TestChange_TheLegendsOfferSupersede(t *testing.T) {
	fakePagerSeam(t)
	m, _ := writeModel(t)
	if !strings.Contains(m.View(), "u supersede") {
		t.Errorf("the Records screen legend lacks u supersede:\n%s", m.View())
	}
}
