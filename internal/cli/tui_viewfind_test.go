package cli

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

// viewerModel is a TUI with a long record open in the built-in viewer.
func viewerModel(t *testing.T) *tuiModel { return viewerModelColour(t, false) }

// viewerModelColour is viewerModel on a terminal that does, or does not, show
// colour: the profile message Bubble Tea sends at start-up (DR-0084).
func viewerModelColour(t *testing.T, colour bool) *tuiModel {
	t.Helper()
	m, _ := writeModel(t)
	m, _ = press(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if colour {
		m, _ = press(t, m, tea.ColorProfileMsg{Profile: colorprofile.ANSI})
	}
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
	if !strings.Contains(m.View().Content, "line 001") {
		t.Fatalf("the viewer does not start at the top:\n%s", m.View().Content)
	}
	m, _ = press(t, m, ch('G'))
	if v := m.View().Content; !strings.Contains(v, "second Needle") {
		t.Errorf("G did not reach the end:\n%s", v)
	}
	m, _ = press(t, m, ch('g'))
	if v := m.View().Content; !strings.Contains(v, "line 001") {
		t.Errorf("g did not return to the top:\n%s", v)
	}
	if m.state != viewText {
		t.Errorf("g or G left the viewer: %s", viewStateNames[m.state])
	}
}

func TestViewer_SlashFindsAndNMovesToTheNextMatch(t *testing.T) {
	m := viewerModel(t)
	m, _ = press(t, m, append(append([]tea.Msg{ch('/')}, typed("needle")...), keyEnter)...)
	if v := m.View().Content; !strings.Contains(v, "the needle is here") {
		t.Fatalf("the first match is not on screen:\n%s", v)
	}
	if !strings.Contains(m.View().Content, "1/2") {
		t.Errorf("the legend does not say which match this is:\n%s", m.View().Content)
	}
	m, _ = press(t, m, ch('n'))
	if v := m.View().Content; !strings.Contains(v, "second Needle") {
		t.Errorf("n did not move to the second match (matching is case-insensitive):\n%s", v)
	}
	m, _ = press(t, m, ch('n'))
	if v := m.View().Content; !strings.Contains(v, "the needle is here") {
		t.Errorf("n did not wrap to the first match:\n%s", v)
	}
	m, _ = press(t, m, ch('N'))
	if v := m.View().Content; !strings.Contains(v, "second Needle") {
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
	if !strings.Contains(m.View().Content, "qg:n") {
		t.Errorf("the find prompt does not show what was typed:\n%s", m.View().Content)
	}
	m, _ = press(t, m, keyEsc)
	if m.state != viewText || strings.Contains(m.View().Content, "qg:n") {
		t.Errorf("Esc did not abandon the find (state %s):\n%s", viewStateNames[m.state], m.View().Content)
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
	if !strings.Contains(m.View().Content, "no match") {
		t.Errorf("a find with no match says nothing:\n%s", m.View().Content)
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

// ─── the find marks the match inside its line (TODO, v0.0.22) ────────────────

func TestAnsiPattern_StripsTheEscapesTheHighlighterWrites(t *testing.T) {
	// The pattern had an extra backslash and matched no real escape, so the find
	// searched the escape codes as well as the text.
	got := ansiPattern.ReplaceAllString("\x1b[1;35m# Title\x1b[0m and \x1b[4;34mlink\x1b[0m", "")
	if got != "# Title and link" {
		t.Errorf("stripped = %q", got)
	}
}

func TestMarkMatches(t *testing.T) {
	const on, off = "\x1b[7m", "\x1b[27m"
	for name, tc := range map[string]struct{ line, term, want string }{
		"plain":            {"the needle is here", "needle", "the " + on + "needle" + off + " is here"},
		"case-insensitive": {"a second Needle appears", "needle", "a second " + on + "Needle" + off + " appears"},
		"two in a line":    {"ab ab", "ab", on + "ab" + off + " " + on + "ab" + off},
		"inside a colour":  {"\x1b[1;35m# Needle here\x1b[0m", "needle", "\x1b[1;35m# " + on + "Needle" + off + " here\x1b[0m"},
		"across a style":   {"ne\x1b[1medle", "needle", on + "ne\x1b[1m" + on + "edle" + off},
		"no match":         {"nothing here", "needle", "nothing here"},
		"empty term":       {"nothing here", "", "nothing here"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := markMatches(tc.line, tc.term); got != tc.want {
				t.Errorf("markMatches(%q, %q)\n got %q\nwant %q", tc.line, tc.term, got, tc.want)
			}
			if plain := ansiPattern.ReplaceAllString(markMatches(tc.line, tc.term), ""); plain != ansiPattern.ReplaceAllString(tc.line, "") {
				t.Errorf("marking changed the text: %q", plain)
			}
		})
	}
}

func TestViewer_FindMarksTheMatchInsideItsLineAndANewFindClearsIt(t *testing.T) {
	m := viewerModelColour(t, true)
	m, _ = press(t, m, append(append([]tea.Msg{ch('/')}, typed("needle")...), keyEnter)...)
	if v := m.View().Content; !strings.Contains(v, "the \x1b[7mneedle\x1b[27m is here") {
		t.Errorf("the match is not marked in its line:\n%q", v)
	}
	m, _ = press(t, m, ch('n'))
	if v := m.View().Content; !strings.Contains(v, "\x1b[7mNeedle\x1b[27m appears") {
		t.Errorf("the second match is not marked:\n%q", v)
	}
	// Another term moves the marks; an empty one removes them.
	m, _ = press(t, m, append(append([]tea.Msg{ch('/')}, typed("filler")...), keyEnter)...)
	if v := m.View().Content; strings.Contains(v, "\x1b[7mneedle") || !strings.Contains(v, "\x1b[7mfiller\x1b[27m") {
		t.Errorf("the marks did not follow the new term:\n%q", v)
	}
	m, _ = press(t, m, ch('/'))
	m, _ = press(t, m, keyEnter)
	if v := m.View().Content; strings.Contains(v, "\x1b[7m") {
		t.Errorf("an empty find left the marks on:\n%q", v)
	}
}
