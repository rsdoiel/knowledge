package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// A project's observations, concepts and records are tabs (knowledge DR-0066
// amendment, v0.0.19 T5 step 4): a strip with counts shows them, the active one is
// underlined, and o, c and r switch between them as they always did.

func projectTabs(t *testing.T) *tuiModel {
	t.Helper()
	m := newTestTUIModelWithRecords(t) // alpha: 1 observation, 1 concept, 2 records
	m.projectList.Select(0)
	m, _ = press(t, m, keyEnter)
	return m
}

// column is the column (not the byte offset: the strip has multi-byte separators)
// at which sub first appears in line, or -1.
func column(line, sub string) int {
	i := strings.Index(line, sub)
	if i < 0 {
		return -1
	}
	return utf8.RuneCountInString(line[:i])
}

// stripAndRule returns the tab strip line and the line under it.
func stripAndRule(t *testing.T, view string) (string, string) {
	t.Helper()
	lines := strings.Split(view, "\n")
	for i, l := range lines {
		if strings.Contains(l, "Observations (") && strings.Contains(l, "Concepts (") {
			if i+1 >= len(lines) {
				break
			}
			return l, lines[i+1]
		}
	}
	t.Fatalf("no tab strip in:\n%s", view)
	return "", ""
}

func TestTabs_AProjectShowsTheStripWithCounts(t *testing.T) {
	v := projectTabs(t).View()
	strip, _ := stripAndRule(t, v)
	for _, want := range []string{"Observations (1)", "Concepts (1)", "Records (2)"} {
		if !strings.Contains(strip, want) {
			t.Errorf("the strip lacks %q: %q", want, strip)
		}
	}
	if !strings.HasPrefix(v, "┌ alpha ") {
		t.Errorf("the frame title should be the project's name:\n%s", v)
	}
}

// The rule under the strip sits under the active tab and nowhere else.
func TestTabs_TheActiveTabIsUnderlined(t *testing.T) {
	m := projectTabs(t)
	for _, tc := range []struct {
		key   rune
		label string
		state viewState
	}{{'o', "Observations", viewObservations}, {'c', "Concepts", viewConcepts}, {'r', "Records", viewRecords}} {
		m, _ = press(t, m, ch(tc.key))
		if m.state != tc.state {
			t.Fatalf("%c gave %s, want %s", tc.key, viewStateNames[m.state], viewStateNames[tc.state])
		}
		strip, rule := stripAndRule(t, m.View())
		col := column(strip, tc.label)
		bar := column(rule, "━")
		if col < 0 || bar != col {
			t.Errorf("%s: rule starts at %d, label at %d:\n%s\n%s", tc.label, bar, col, strip, rule)
		}
		if n := strings.Count(rule, "━"); n != len(tc.label)+len(" (1)") && n != len(tc.label)+len(" (2)") {
			t.Errorf("%s: rule is %d wide, want the label and its count", tc.label, n)
		}
	}
}

// The three lists are loaded on opening the project, so every count is true at once.
func TestTabs_TheCountsAreTrueBeforeAnyTabIsVisited(t *testing.T) {
	m := projectTabs(t)
	if len(m.conceptList.Items()) != 1 || len(m.recordList.Items()) != 2 || len(m.observationList.Items()) != 1 {
		t.Errorf("lists hold %d/%d/%d items, want 1/1/2 loaded on entering the project",
			len(m.observationList.Items()), len(m.conceptList.Items()), len(m.recordList.Items()))
	}
}

func TestTabs_TheLegendNamesTheTabKeys(t *testing.T) {
	m := projectTabs(t)
	for _, state := range []viewState{viewObservations, viewConcepts, viewRecords} {
		m.setState(state)
		if v := m.View(); !strings.Contains(v, "o c r tabs") || !strings.Contains(v, "q back") {
			t.Errorf("%s: the legend lacks the tab keys:\n%s", viewStateNames[state], v)
		}
	}
}

// The strip takes two lines of the body; the list is that much shorter, and the
// whole still fits the window.
func TestTabs_TheViewStillFitsTheWindow(t *testing.T) {
	m := projectTabs(t)
	for _, state := range []viewState{viewObservations, viewConcepts, viewRecords} {
		m.setState(state)
		if n := len(strings.Split(m.View(), "\n")); n != 24 {
			t.Errorf("%s: %d lines in a 24-line window", viewStateNames[state], n)
		}
	}
}
