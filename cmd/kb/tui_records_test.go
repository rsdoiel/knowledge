package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	knowledge "github.com/rsdoiel/knowledge"
)

// The Records screens: Browse and Pending (knowledge DR-0066, v0.0.19 T5 step 3).
// They show the records in a scope, chosen the way `kb record list` chooses it
// (the project the working directory belongs to, then KB_PROJECT, then everything),
// by the same code, and `a` widens the scope.

// recordsModel is a menu model whose knowledge base also has a pending record in
// a second project, with the working directory and KB_PROJECT neutral.
func recordsModel(t *testing.T) *tuiModel {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("KB_PROJECT", "")
	m := menuModel(t)
	beta, err := m.kb.ProjectByName("beta")
	if err != nil || beta == nil {
		t.Fatalf("ProjectByName(beta): %v", err)
	}
	if _, err := m.kb.AddRecord(knowledge.Record{RecordID: "0003", ProjectID: beta.ID, Scope: "project", Date: "2026-07-01",
		Path: "beta/decisions/0003-x.md", Title: "A pending decision", Status: "proposed", Kind: "decision", Body: "body"}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	return m
}

// openRecords gets to a Records leaf: row Records, then the leaf by position.
func openRecords(t *testing.T, m *tuiModel, leaf int) *tuiModel {
	t.Helper()
	m = openRow(t, m, "Records")
	for i := 0; i < leaf; i++ {
		m, _ = press(t, m, ch('j'))
	}
	m, _ = press(t, m, keyEnter)
	return m
}

func TestRecords_BrowseShowsTheRecordsInScopeNewestFirst(t *testing.T) {
	m := openRecords(t, recordsModel(t), 0)
	if m.state != viewRecordScope {
		t.Fatalf("state = %s, want the Records screen", viewStateNames[m.state])
	}
	v := m.View()
	i2, i1, i3 := strings.Index(v, "alpha/DR-0002"), strings.Index(v, "alpha/DR-0001"), strings.Index(v, "beta/DR-0003")
	if i2 < 0 || i1 < 0 || i3 < 0 {
		t.Fatalf("the screen lacks a qualified reference (alpha/DR-0002 %d, alpha/DR-0001 %d, beta/DR-0003 %d):\n%s", i2, i1, i3, v)
	}
	// By date, as the CLI orders them: DR-0002 is 2026-08-19, DR-0001 2026-08-01 and
	// DR-0003 2026-07-01. Ids are identity, not chronology.
	if !(i2 < i1 && i1 < i3) {
		t.Errorf("not newest first by date (DR-0002 at %d, DR-0001 at %d, DR-0003 at %d):\n%s", i2, i1, i3, v)
	}
	for _, want := range []string{"accepted", "superseded", "proposed", "The newer decision", "(3)"} {
		if !strings.Contains(v, want) {
			t.Errorf("the screen lacks %q:\n%s", want, v)
		}
	}
}

// One line a record: the list's two-line rows would halve what fits.
func TestRecords_EachRecordIsOneLine(t *testing.T) {
	v := openRecords(t, recordsModel(t), 0).View()
	for _, ref := range []string{"alpha/DR-0002", "alpha/DR-0001", "beta/DR-0003"} {
		if n := strings.Count(v, ref); n != 1 {
			t.Errorf("%s appears %d times, want once", ref, n)
		}
	}
}

func TestRecords_PendingIsTheProposedOnesOldestFirst(t *testing.T) {
	m := recordsModel(t)
	alpha, _ := m.kb.ProjectByName("alpha")
	if _, err := m.kb.AddRecord(knowledge.Record{RecordID: "0004", ProjectID: alpha.ID, Scope: "project", Date: "2026-09-01",
		Path: "alpha/decisions/0004-x.md", Title: "A later pending decision", Status: "proposed", Kind: "decision", Body: "body"}); err != nil {
		t.Fatal(err)
	}
	m = openRecords(t, m, 1)
	v := m.View()
	if m.state != viewRecordScope || !strings.Contains(v, "Pending") {
		t.Fatalf("state %s, view:\n%s", viewStateNames[m.state], v)
	}
	i3, i4 := strings.Index(v, "beta/DR-0003"), strings.Index(v, "alpha/DR-0004")
	if i3 < 0 || i4 < 0 || i3 > i4 {
		t.Errorf("want the two proposed records, oldest first (DR-0003 at %d, DR-0004 at %d):\n%s", i3, i4, v)
	}
	if strings.Contains(v, "alpha/DR-0002") {
		t.Errorf("an accepted record is on the Pending screen:\n%s", v)
	}
}

// The scope is chosen as the CLI chooses it, and a widens it.
func TestRecords_TheScopeFollowsTheProjectAndAWidensIt(t *testing.T) {
	t.Setenv("KB_PROJECT", "alpha")
	m := recordsModel(t)
	t.Setenv("KB_PROJECT", "alpha") // recordsModel clears it for its own set-up
	m = openRecords(t, m, 0)
	v := m.View()
	if !strings.Contains(v, "alpha") || strings.Contains(v, "beta/DR-0003") {
		t.Fatalf("the scope should be alpha only:\n%s", v)
	}
	m, _ = press(t, m, ch('a'))
	if v = m.View(); !strings.Contains(v, "beta/DR-0003") {
		t.Errorf("a did not widen the scope:\n%s", v)
	}
	m, _ = press(t, m, ch('a'))
	if v = m.View(); strings.Contains(v, "beta/DR-0003") {
		t.Errorf("a again did not narrow it:\n%s", v)
	}
}

func TestRecords_TheGroupMenuNamesTheScope(t *testing.T) {
	t.Setenv("KB_PROJECT", "alpha")
	m := recordsModel(t)
	t.Setenv("KB_PROJECT", "alpha")
	g := openRow(t, m, "Records")
	for _, want := range []string{"scope: alpha", "a all scopes"} {
		if !strings.Contains(g.View(), want) {
			t.Errorf("the Records menu lacks %q:\n%s", want, g.View())
		}
	}
}

func TestRecords_QGoesBackToTheRecordsMenuAndEscDoesNothing(t *testing.T) {
	m := openRecords(t, recordsModel(t), 0)
	m, cmd := press(t, m, keyEsc)
	if m.state != viewRecordScope || isQuit(cmd) {
		t.Fatalf("Esc: %s, quit %v", viewStateNames[m.state], isQuit(cmd))
	}
	m, cmd = press(t, m, ch('q'))
	if m.state != viewGroup || m.group != "record" || isQuit(cmd) {
		t.Errorf("q gave %s %q (quit %v), want the Records menu", viewStateNames[m.state], m.group, isQuit(cmd))
	}
}

func TestRecords_SearchFromHereReturnsHere(t *testing.T) {
	m := openRecords(t, recordsModel(t), 0)
	m, _ = press(t, m, append(append([]tea.Msg{ch('/')}, typed("decision")...), keyEnter)...)
	if m.state != viewSearch {
		t.Fatalf("state = %s", viewStateNames[m.state])
	}
	if m, _ = press(t, m, ch('q')); m.state != viewRecordScope {
		t.Errorf("q from the results gave %s, want the Records screen", viewStateNames[m.state])
	}
}

func TestRecords_ThereIsNoReadKeyYet(t *testing.T) {
	// Enter and s arrive with T6; until then the legend does not offer them.
	v := openRecords(t, recordsModel(t), 0).View()
	for _, bad := range []string{"Enter read", "s status"} {
		if strings.Contains(v, bad) {
			t.Errorf("the legend offers %q, which does nothing yet:\n%s", bad, v)
		}
	}
	for _, want := range []string{"a all scopes", "/ search", "q back"} {
		if !strings.Contains(v, want) {
			t.Errorf("the legend lacks %q:\n%s", want, v)
		}
	}
}

func TestRecords_AnEmptyScopeSaysSo(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("KB_PROJECT", "")
	m := openRecords(t, menuModel(t), 1) // Pending; the fixture has nothing proposed
	if v := m.View(); !strings.Contains(v, "No pending records") && !strings.Contains(v, "no pending records") {
		t.Errorf("an empty Pending screen should say so:\n%s", v)
	}
}
