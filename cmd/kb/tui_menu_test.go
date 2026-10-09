package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The top menu, the group menus and the frame (knowledge DR-0066 and its
// 2026-10-09 amendment, v0.0.19 T5 step 2). Back is structural: q goes to a
// screen's parent, so a deep link that lands in the middle of the tree still has
// a predictable way back.

// menuModel is a model opened the way bare `kb` opens it: at the top menu.
func menuModel(t *testing.T) *tuiModel {
	t.Helper()
	m := newTestTUIModelWithRecords(t)
	top, err := newTUIModel(m.kb, nil)
	if err != nil {
		t.Fatalf("newTUIModel: %v", err)
	}
	updated, _ := top.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return updated.(*tuiModel)
}

// rowIndex finds a top-menu row by label.
func rowIndex(t *testing.T, label string) int {
	t.Helper()
	for i, it := range topMenu() {
		if it.Label == label {
			return i
		}
	}
	t.Fatalf("no top-menu row %q", label)
	return -1
}

// openRow moves to a top-menu row and presses Enter on it. The moves are bounded
// so a cursor that never moves fails the test instead of hanging it.
func openRow(t *testing.T, m *tuiModel, label string) *tuiModel {
	t.Helper()
	target := rowIndex(t, label)
	for i := 0; m.menuCursor != target; i++ {
		if i > 40 {
			t.Fatalf("the cursor is at %d and never reached %q (row %d)", m.menuCursor, label, target)
		}
		if m.menuCursor < target {
			m, _ = press(t, m, ch('j'))
		} else {
			m, _ = press(t, m, ch('k'))
		}
	}
	m, _ = press(t, m, keyEnter)
	return m
}

func TestMenu_BareKBOpensTheTopMenu(t *testing.T) {
	m := menuModel(t)
	if m.state != viewMenu {
		t.Fatalf("state = %s, want viewMenu", viewStateNames[m.state])
	}
	v := m.View()
	for _, it := range topMenu() {
		if !strings.Contains(v, it.Label) {
			t.Errorf("the top menu lacks %q:\n%s", it.Label, v)
		}
	}
	if !strings.Contains(v, "> Projects") {
		t.Errorf("the cursor is not on the first row:\n%s", v)
	}
}

// Some machines have more than one workspace, so the menu says which this is.
func TestMenu_TheHeaderNamesTheWorkspace(t *testing.T) {
	v := menuModel(t).View()
	for _, want := range []string{"agents/knowledge.db", "2 projects", "records"} {
		if !strings.Contains(v, want) {
			t.Errorf("the header lacks %q:\n%s", want, v)
		}
	}
}

// Every screen is in the same box: top border with a title, a divider above the
// legend, a bottom border, and every line the width of the window.
func TestMenu_EveryScreenIsInOneFrame(t *testing.T) {
	m := menuModel(t)
	check := func(name string, m *tuiModel) {
		t.Helper()
		lines := strings.Split(strings.TrimRight(m.View(), "\n"), "\n")
		if !strings.HasPrefix(lines[0], "┌ ") || !strings.HasPrefix(lines[len(lines)-1], "└") {
			t.Errorf("%s: not boxed:\n%s", name, m.View())
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w != 80 {
				t.Errorf("%s: line %d is %d wide, want 80: %q", name, i, w, l)
			}
		}
		if len(lines) > 24 {
			t.Errorf("%s: %d lines in a 24-line window", name, len(lines))
		}
		if !strings.Contains(m.View(), "├──") {
			t.Errorf("%s: no divider above the legend", name)
		}
	}
	check("top menu", m)
	g := openRow(t, m, "Projects")
	check("group menu", g)
	b, _ := press(t, g, keyEnter)
	check("project list", b)
}

func TestMenu_JKAndTheArrowsMoveAndStopAtTheEnds(t *testing.T) {
	m := menuModel(t)
	m, _ = press(t, m, ch('k'))
	if m.menuCursor != 0 {
		t.Errorf("k at the top moved to %d", m.menuCursor)
	}
	m, _ = press(t, m, ch('j'), tea.KeyMsg{Type: tea.KeyDown})
	if m.menuCursor != 2 {
		t.Errorf("j then Down gave %d, want 2", m.menuCursor)
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.menuCursor != 1 {
		t.Errorf("Up gave %d, want 1", m.menuCursor)
	}
	for i := 0; i < 30; i++ {
		m, _ = press(t, m, ch('j'))
	}
	if m.menuCursor != len(topMenu())-1 {
		t.Errorf("j past the end gave %d, want %d", m.menuCursor, len(topMenu())-1)
	}
}

func TestMenu_AGroupOpensItsMenuAndBrowseOpensTheProjects(t *testing.T) {
	m := openRow(t, menuModel(t), "Projects")
	if m.state != viewGroup || m.group != "project" {
		t.Fatalf("state %s group %q, want the project group menu", viewStateNames[m.state], m.group)
	}
	v := m.View()
	for _, want := range []string{"Browse", "New project…", "(v0.0.20)"} {
		if !strings.Contains(v, want) {
			t.Errorf("the Projects menu lacks %q:\n%s", want, v)
		}
	}
	m, _ = press(t, m, keyEnter) // Browse is first
	if m.state != viewProjects {
		t.Errorf("Browse opened %s, want the project list", viewStateNames[m.state])
	}
}

// A row that is not built is dimmed with its release, and choosing it explains
// itself and shows the command that does the same, without leaving the screen.
func TestMenu_AnUnbuiltRowSaysSoAndShowsTheCommand(t *testing.T) {
	m := menuModel(t)
	if v := m.View(); !strings.Contains(v, "Ingest") || !strings.Contains(v, "(v0.0.20)") {
		t.Errorf("the top menu does not mark Ingest as coming:\n%s", v)
	}
	m = openRow(t, m, "Ingest")
	if m.state != viewMenu {
		t.Errorf("choosing an unbuilt row left the menu for %s", viewStateNames[m.state])
	}
	v := m.View()
	for _, want := range []string{"not in the TUI yet", "v0.0.20", "kb ingest PATH"} {
		if !strings.Contains(v, want) {
			t.Errorf("the explanation lacks %q:\n%s", want, v)
		}
	}
	m, _ = press(t, m, ch('j'))
	if strings.Contains(m.View(), "not in the TUI yet") {
		t.Error("the explanation stayed after another key")
	}
}

// A group with nothing built opens to a menu of dimmed rows, so what is coming is visible.
func TestMenu_AGroupWithNothingBuiltStillOpens(t *testing.T) {
	m := openRow(t, menuModel(t), "Observations")
	if m.state != viewGroup || m.group != "observation" {
		t.Fatalf("state %s group %q, want the observation group menu", viewStateNames[m.state], m.group)
	}
	m, _ = press(t, m, keyEnter)
	if v := m.View(); !strings.Contains(v, "kb observation list --project P") {
		t.Errorf("choosing Browse in a dimmed group should show its command:\n%s", v)
	}
}

// ─── back is structural ──────────────────────────────────────────────────────

func TestMenu_QGoesToTheParentAtEveryLevel(t *testing.T) {
	m := menuModel(t)
	m = openRow(t, m, "Projects")
	m, _ = press(t, m, keyEnter) // project list
	m, _ = press(t, m, keyEnter) // a project: observations
	for _, step := range []viewState{viewProjects, viewGroup, viewMenu} {
		var cmd tea.Cmd
		m, cmd = press(t, m, ch('q'))
		if m.state != step || isQuit(cmd) {
			t.Fatalf("q gave %s (quit %v), want %s", viewStateNames[m.state], isQuit(cmd), viewStateNames[step])
		}
	}
	if _, cmd := press(t, m, ch('q')); !isQuit(cmd) {
		t.Error("q at the top menu did not quit")
	}
}

// A deep link lands in the middle of the tree and q still goes to the parent.
func TestMenu_ADeepLinkHasTheSameWayBack(t *testing.T) {
	kb := menuModel(t).kb
	m, err := newTUIModelAt(kb, nil, tuiStart{state: viewProjects})
	if err != nil {
		t.Fatal(err)
	}
	m, cmd := press(t, m, ch('q'))
	if m.state != viewGroup || m.group != "project" || isQuit(cmd) {
		t.Errorf("q from a deep-linked project list gave %s %q (quit %v), want the project group menu", viewStateNames[m.state], m.group, isQuit(cmd))
	}
	g, err := newTUIModelAt(kb, nil, tuiStart{state: viewGroup, group: "record"})
	if err != nil {
		t.Fatal(err)
	}
	if g.state != viewGroup || g.group != "record" || !strings.Contains(g.View(), "Pending") {
		t.Errorf("a deep link to the record group opened %s %q:\n%s", viewStateNames[g.state], g.group, g.View())
	}
}

func TestMenu_EscDoesNothingOnTheMenus(t *testing.T) {
	m := menuModel(t)
	m, cmd := press(t, m, keyEsc)
	if m.state != viewMenu || isQuit(cmd) {
		t.Errorf("Esc on the top menu: %s, quit %v", viewStateNames[m.state], isQuit(cmd))
	}
	m = openRow(t, m, "Records")
	m, cmd = press(t, m, keyEsc)
	if m.state != viewGroup || isQuit(cmd) {
		t.Errorf("Esc on a group menu: %s, quit %v", viewStateNames[m.state], isQuit(cmd))
	}
}

// ─── search from the menu, and text entry ────────────────────────────────────

func TestMenu_SearchStartsFromTheMenuAndQReturnsThere(t *testing.T) {
	m := menuModel(t)
	m = openRow(t, m, "Search")
	if !m.searching {
		t.Fatal("the Search row did not open the prompt")
	}
	m, _ = press(t, m, append(typed("observation"), keyEnter)...)
	if m.state != viewSearch {
		t.Fatalf("state = %s, want the results", viewStateNames[m.state])
	}
	m, _ = press(t, m, ch('q'))
	if m.state != viewMenu {
		t.Errorf("q from the results gave %s, want the top menu", viewStateNames[m.state])
	}
}

func TestMenu_CommandLettersAreTextInTheSearchPromptFromAMenu(t *testing.T) {
	for _, m := range []*tuiModel{menuModel(t), openRow(t, menuModel(t), "Records")} {
		m, _ = press(t, m, ch('/'))
		m, cmd := press(t, m, typed("qjkaynocr:")...)
		if isQuit(cmd) || !m.searching || m.searchInput.Value() != "qjkaynocr:" {
			t.Errorf("typing acted as a command (quit %v, searching %v, value %q)", isQuit(cmd), m.searching, m.searchInput.Value())
		}
	}
}

func TestMenu_TheLegendsNameTheKeys(t *testing.T) {
	m := menuModel(t)
	for _, w := range []string{"Enter open", "/ search", "q quit"} {
		if !strings.Contains(m.View(), w) {
			t.Errorf("the top-menu legend lacks %q:\n%s", w, m.View())
		}
	}
	g := openRow(t, m, "Projects")
	for _, w := range []string{"Enter open", "q back"} {
		if !strings.Contains(g.View(), w) {
			t.Errorf("the group legend lacks %q:\n%s", w, g.View())
		}
	}
}
