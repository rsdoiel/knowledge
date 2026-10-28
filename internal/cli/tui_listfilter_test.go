package cli

import (
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// `/` filters whatever list has focus (knowledge DR-0072 point 6, step E2 of
// design/scoped-search-design.md). On a menu there is nothing in front of the
// person to find in, so it does nothing there; S and the Search menu are the way
// to search.

func TestListFilter_SlashFiltersTheProjectList(t *testing.T) {
	m := browserAt(t, viewProjects)
	m, _ = press(t, m, ch('/'))
	if m.searching {
		t.Fatalf("/ opened the search; on a list it filters")
	}
	if v := m.View().Content; !strings.Contains(v, "Filter:") {
		t.Errorf("the legend should show the filter prompt:\n%s", v)
	}
	m, _ = press(t, m, typed("bet")...)
	m, _ = press(t, m, keyEnter)
	v := m.View().Content
	if !strings.Contains(v, "beta") || strings.Contains(v, "alpha") {
		t.Errorf("filter 'bet' should leave beta only:\n%s", v)
	}
	// Enter opens the selected, filtered project.
	m, _ = press(t, m, keyEnter)
	if m.state != viewObservations || m.selectedProject == nil || m.selectedProject.Name != "beta" {
		t.Errorf("Enter on the filtered list opened %v %v, want beta's observations", viewStateNames[m.state], m.selectedProject)
	}
}

func TestListFilter_EscClearsAKeptFilterAndAbandonsATypedOne(t *testing.T) {
	m := browserAt(t, viewProjects)
	m, _ = press(t, m, ch('/'))
	m, _ = press(t, m, typed("bet")...)
	m, _ = press(t, m, keyEsc) // abandons what is being typed
	if v := m.View().Content; !strings.Contains(v, "alpha") || !strings.Contains(v, "beta") {
		t.Errorf("Esc while typing should restore the list:\n%s", v)
	}
	m, _ = press(t, m, ch('/'))
	m, _ = press(t, m, typed("bet")...)
	m, _ = press(t, m, keyEnter)
	if v := m.View().Content; !strings.Contains(v, `filter "bet" (Esc clears)`) {
		t.Errorf("a kept filter should be named in the title:\n%s", v)
	}
	m, _ = press(t, m, keyEsc)
	if v := m.View().Content; !strings.Contains(v, "alpha") || !strings.Contains(v, "beta") {
		t.Errorf("Esc on a kept filter should clear it:\n%s", v)
	}
	if m.state != viewProjects {
		t.Errorf("Esc left the screen: %s", viewStateNames[m.state])
	}
}

func TestListFilter_TypingTakesEveryKeyAsText(t *testing.T) {
	m := browserAt(t, viewProjects)
	m, _ = press(t, m, ch('/'))
	m, cmd := press(t, m, typed("qSd:")...)
	if isQuit(cmd) || m.state != viewProjects || m.searching {
		t.Fatalf("a typed key acted as a command: state %s searching %v", viewStateNames[m.state], m.searching)
	}
	if v := m.View().Content; !strings.Contains(v, "qSd:") {
		t.Errorf("the prompt should show what was typed:\n%s", v)
	}
}

func TestListFilter_FiltersTheRecordsTab(t *testing.T) {
	m := browserAt(t, viewRecords)
	m, _ = press(t, m, ch('/'))
	m, _ = press(t, m, typed("older")...)
	m, _ = press(t, m, keyEnter)
	v := m.View().Content
	if !strings.Contains(v, "The older decision") || strings.Contains(v, "The newer decision") {
		t.Errorf("filter 'older' should leave one record:\n%s", v)
	}
	if !strings.Contains(v, "Records (2)") {
		t.Errorf("the tab count is the project's, not the filtered list's:\n%s", v)
	}
}

func TestListFilter_FiltersTheObservationsAndConceptsTabs(t *testing.T) {
	for state, want := range map[viewState]string{viewObservations: "specific observation", viewConcepts: "streaming"} {
		m := browserAt(t, state)
		m, _ = press(t, m, ch('/'))
		m, _ = press(t, m, typed("zzznomatch")...)
		m, _ = press(t, m, keyEnter)
		v := m.View().Content
		if strings.Contains(v, want) || !strings.Contains(v, `matches "zzznomatch"`) {
			t.Errorf("%s: an empty filter should say so and hide the rows:\n%s", viewStateNames[state], v)
		}
		m, _ = press(t, m, keyEsc)
		if !strings.Contains(m.View().Content, want) {
			t.Errorf("%s: Esc should bring the rows back:\n%s", viewStateNames[state], m.View().Content)
		}
	}
}

func TestListFilter_FiltersSearchResults(t *testing.T) {
	m := browserAt(t, viewSearch)
	if len(m.searchList.Items()) == 0 {
		t.Skip("no results to filter")
	}
	m, _ = press(t, m, ch('/'))
	if m.searching {
		t.Fatalf("/ in the results should filter, not start a new search")
	}
	m, _ = press(t, m, typed("zzznomatch")...)
	m, _ = press(t, m, keyEnter)
	if v := m.View().Content; !strings.Contains(v, `matches "zzznomatch"`) {
		t.Errorf("the empty filter should say so:\n%s", v)
	}
}

func TestListFilter_AMenuHasNothingToFilter(t *testing.T) {
	m := browserAt(t, viewProjects)
	m, _ = press(t, m, ch('q')) // the Projects menu
	m, _ = press(t, m, ch('/'))
	if m.searching || strings.Contains(m.View().Content, "Filter:") {
		t.Errorf("/ on a menu should do nothing:\n%s", m.View().Content)
	}
	m, _ = press(t, m, ch('q')) // the top menu
	m, _ = press(t, m, ch('/'))
	if m.searching {
		t.Errorf("/ on the top menu should do nothing")
	}
}

func TestListFilter_LegendsNameTheFilterAndSearchWhenThereIsRoom(t *testing.T) {
	for _, state := range []viewState{viewProjects, viewObservations, viewConcepts, viewRecords} {
		m := browserAt(t, state)
		if v := m.View().Content; !strings.Contains(v, "/ filter") || !strings.Contains(v, "q back") {
			t.Errorf("%s: at 80 columns the legend should name / filter and q back:\n%s", viewStateNames[state], v)
		}
		next, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
		if v := next.(*tuiModel).View().Content; !strings.Contains(v, "/ filter") || !strings.Contains(v, "S search") {
			t.Errorf("%s: at 140 columns the legend should name both keys:\n%s", viewStateNames[state], v)
		}
	}
}

func TestListFilter_AFilterSurvivesGoingIntoAnItemAndBack(t *testing.T) {
	m := browserAt(t, viewProjects)
	m, _ = press(t, m, ch('/'))
	m, _ = press(t, m, typed("bet")...)
	m, _ = press(t, m, keyEnter)
	m, _ = press(t, m, keyEnter) // beta's observations
	m, _ = press(t, m, ch('q'))  // back to the project list
	v := m.View().Content
	if strings.Contains(v, "alpha") || !strings.Contains(v, "beta") || !strings.Contains(v, "(Esc clears)") {
		t.Errorf("coming back should show the list as it was left, filtered:\n%s", v)
	}
}

// The built binary, on a pseudo-terminal: `/` filters the project list, Enter
// keeps it, and S opens the search prompt scoped to where you are.
func TestBinary_TUIFilterAndSearchOnATerminal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	kb, root := openWorkspaceKB(t)
	for _, n := range []string{"aardvark", "quokka"} {
		if _, err := kb.AddProject(n, "a "+n); err != nil {
			t.Fatal(err)
		}
	}
	master, screen, wait := startOnATerminal(t, root, "-i", "project", "list")
	screen.waitFor(t, "aardvark")
	master.WriteString("/")
	screen.waitFor(t, "Filter:")
	master.WriteString("quo")
	master.WriteString("\r")
	screen.waitFor(t, `filter "quo" (Esc clears)`)
	master.WriteString("\x1b") // Esc clears it
	time.Sleep(300 * time.Millisecond)
	master.WriteString("S")
	screen.waitFor(t, "Search:")
	master.WriteString("\x1b") // leaves the prompt
	time.Sleep(300 * time.Millisecond)
	master.WriteString("qqq")
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0:\n%s", code, screen.text())
	}
}
