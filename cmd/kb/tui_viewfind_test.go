package main

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// viewerModel is a TUI with a long record open in the built-in viewer.
func viewerModel(t *testing.T) *tuiModel {
	t.Helper()
	m, _ := writeModel(t)
	m, _ = press(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&b, "line %03d filler text\n", i)
	}
	b.WriteString("the needle is here\n")
	for i := 102; i <= 200; i++ {
		fmt.Fprintf(&b, "line %03d filler text\n", i)
	}
	b.WriteString("a second Needle appears\n")
	m.reviewRaw = []byte(b.String())
	m.reviewBack = viewRecordScope
	m.openViewer("read")
	if m.state != viewText {
		t.Fatalf("state = %s, want the built-in viewer", viewStateNames[m.state])
	}
	return m
}

func TestViewer_GAndGJumpToTheEnds(t *testing.T) {
	m := viewerModel(t)
	if !strings.Contains(m.View(), "line 001") {
		t.Fatalf("the viewer does not start at the top:\n%s", m.View())
	}
	m, _ = press(t, m, ch('G'))
	if v := m.View(); !strings.Contains(v, "second Needle") {
		t.Errorf("G did not reach the end:\n%s", v)
	}
	m, _ = press(t, m, ch('g'))
	if v := m.View(); !strings.Contains(v, "line 001") {
		t.Errorf("g did not return to the top:\n%s", v)
	}
	if m.state != viewText {
		t.Errorf("g or G left the viewer: %s", viewStateNames[m.state])
	}
}

func TestViewer_SlashFindsAndNMovesToTheNextMatch(t *testing.T) {
	m := viewerModel(t)
	m, _ = press(t, m, append(append([]tea.Msg{ch('/')}, typed("needle")...), keyEnter)...)
	if v := m.View(); !strings.Contains(v, "the needle is here") {
		t.Fatalf("the first match is not on screen:\n%s", v)
	}
	if !strings.Contains(m.View(), "1/2") {
		t.Errorf("the legend does not say which match this is:\n%s", m.View())
	}
	m, _ = press(t, m, ch('n'))
	if v := m.View(); !strings.Contains(v, "second Needle") {
		t.Errorf("n did not move to the second match (matching is case-insensitive):\n%s", v)
	}
	m, _ = press(t, m, ch('n'))
	if v := m.View(); !strings.Contains(v, "the needle is here") {
		t.Errorf("n did not wrap to the first match:\n%s", v)
	}
	m, _ = press(t, m, ch('N'))
	if v := m.View(); !strings.Contains(v, "second Needle") {
		t.Errorf("N did not go back to the previous match:\n%s", v)
	}
}

// While a term is being typed every key is text: q, g, n and : are letters,
// and Esc abandons the find without leaving the viewer.
func TestViewer_TypingATermTakesEveryKeyAsText(t *testing.T) {
	m := viewerModel(t)
	m, _ = press(t, m, append([]tea.Msg{ch('/')}, typed("qg:n")...)...)
	if m.state != viewText {
		t.Fatalf("a typed q left the viewer: %s", viewStateNames[m.state])
	}
	if !strings.Contains(m.View(), "qg:n") {
		t.Errorf("the find prompt does not show what was typed:\n%s", m.View())
	}
	m, _ = press(t, m, keyEsc)
	if m.state != viewText || strings.Contains(m.View(), "qg:n") {
		t.Errorf("Esc did not abandon the find (state %s):\n%s", viewStateNames[m.state], m.View())
	}
	m, _ = press(t, m, ch('q'))
	if m.state != viewRecordScope {
		t.Errorf("q after the find gave %s, want the screen the record was opened from", viewStateNames[m.state])
	}
}

func TestViewer_ATermWithNoMatchSaysSo(t *testing.T) {
	m := viewerModel(t)
	m, _ = press(t, m, append(append([]tea.Msg{ch('/')}, typed("zebra")...), keyEnter)...)
	if m.state != viewText {
		t.Fatalf("state = %s", viewStateNames[m.state])
	}
	if !strings.Contains(m.View(), "no match") {
		t.Errorf("a find with no match says nothing:\n%s", m.View())
	}
}

// A code span longer than a line wraps, and the colour carries onto the next
// line, so wrapping does not undo the highlighting.
func TestViewer_ColourCarriesAcrossAWrappedCodeSpan(t *testing.T) {
	long := "`" + strings.Repeat("word ", 40) + "`"
	got := wrapHighlighted(long, 40)
	lines := strings.Split(got, "\n")
	if len(lines) < 3 {
		t.Fatalf("the span did not wrap: %q", got)
	}
	for i, l := range lines {
		if strings.TrimSpace(ansiSeq.ReplaceAllString(l, "")) == "" {
			continue
		}
		if !strings.Contains(l, "\x1b[33m") {
			t.Errorf("line %d of the wrapped span has lost its colour: %q", i, l)
		}
	}
}
