package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// The chooser, the confirmation and the typed confirmation (knowledge DR-0065,
// DR-0064 amendment). They are driven here with key messages; the built binary
// on a pseudo-terminal, with real bytes such as a lone Esc, is in
// recordreview_test.go.

// ch is a single typed character; runeKey (tui_model_test.go) is a string of them.
func ch(r rune) tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

var (
	keyEsc   tea.Msg = tea.KeyMsg{Type: tea.KeyEsc}
	keyEnter tea.Msg = tea.KeyMsg{Type: tea.KeyEnter}
	keyCtrlC tea.Msg = tea.KeyMsg{Type: tea.KeyCtrlC}
)

// isQuit reports whether a command is bubbletea's Quit.
func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

var proposedOptions = []string{"accepted", "rejected", "cancelled"}

func TestChooser_AKeyChoosesAtOnce(t *testing.T) {
	for key, want := range map[rune]string{'a': "accepted", 'r': "rejected", 'c': "cancelled"} {
		m := newChooser("clasm/DR-0001", "proposed", proposedOptions)
		_, cmd := m.Update(ch(key))
		if m.chosen != want || m.cancelled || !isQuit(cmd) {
			t.Errorf("key %q: chosen %q, cancelled %v, quit %v; want %q chosen at once", key, m.chosen, m.cancelled, isQuit(cmd), want)
		}
	}
}

func TestChooser_UppercaseChoosesToo(t *testing.T) {
	m := newChooser("clasm/DR-0001", "proposed", proposedOptions)
	if _, cmd := m.Update(ch('R')); m.chosen != "rejected" || !isQuit(cmd) {
		t.Errorf("R: chosen %q, quit %v", m.chosen, isQuit(cmd))
	}
}

func TestChooser_QEscAndCtrlCCancelAtOnce(t *testing.T) {
	for name, msg := range map[string]tea.Msg{"q": ch('q'), "esc": keyEsc, "ctrl+c": keyCtrlC} {
		m := newChooser("clasm/DR-0001", "proposed", proposedOptions)
		_, cmd := m.Update(msg)
		if !m.cancelled || m.chosen != "" || !isQuit(cmd) {
			t.Errorf("%s: cancelled %v, chosen %q, quit %v; want cancelled at once", name, m.cancelled, m.chosen, isQuit(cmd))
		}
	}
}

// Enter, a letter that is not offered and anything else do nothing: the chooser
// keeps waiting.
func TestChooser_OtherKeysAreIgnored(t *testing.T) {
	for name, msg := range map[string]tea.Msg{
		"enter": keyEnter, "s (not offered)": ch('s'), "p (not offered)": ch('p'), "x": ch('x'),
		"space": tea.KeyMsg{Type: tea.KeySpace}, "tab": tea.KeyMsg{Type: tea.KeyTab},
	} {
		m := newChooser("clasm/DR-0001", "proposed", proposedOptions)
		_, cmd := m.Update(msg)
		if m.chosen != "" || m.cancelled || isQuit(cmd) {
			t.Errorf("%s: chosen %q, cancelled %v, quit %v; want it ignored", name, m.chosen, m.cancelled, isQuit(cmd))
		}
	}
}

// Several keys can arrive in one message (a fast typist, a paste); they are
// taken one at a time and the first decisive one wins.
func TestChooser_SeveralKeysInOneMessage(t *testing.T) {
	m := newChooser("clasm/DR-0001", "proposed", proposedOptions)
	if _, cmd := m.Update(runeKey("xar")); m.chosen != "accepted" || !isQuit(cmd) {
		t.Errorf("xar: chosen %q, quit %v; want accepted, the first decisive key", m.chosen, isQuit(cmd))
	}
}

func TestChooser_ViewNamesTheRecordAndTheMoves(t *testing.T) {
	v := newChooser("clasm/DR-0001", "proposed", proposedOptions).View()
	for _, want := range []string{"clasm/DR-0001", "proposed", "[a]ccepted", "[r]ejected", "[c]ancelled", "[q]uit"} {
		if !strings.Contains(v, want) {
			t.Errorf("the view lacks %q: %q", want, v)
		}
	}
}

func TestConfirm_OnlyYApplies(t *testing.T) {
	m := newConfirm("Set clasm/DR-0001 from proposed to accepted?")
	if _, cmd := m.Update(ch('y')); !m.confirmed || m.cancelled || !isQuit(cmd) {
		t.Errorf("y: confirmed %v, cancelled %v, quit %v", m.confirmed, m.cancelled, isQuit(cmd))
	}
}

func TestConfirm_NQEscAndCtrlCCancelAtOnce(t *testing.T) {
	for name, msg := range map[string]tea.Msg{"n": ch('n'), "q": ch('q'), "esc": keyEsc, "ctrl+c": keyCtrlC} {
		m := newConfirm("Set?")
		_, cmd := m.Update(msg)
		if m.confirmed || !m.cancelled || !isQuit(cmd) {
			t.Errorf("%s: confirmed %v, cancelled %v, quit %v; want cancelled at once", name, m.confirmed, m.cancelled, isQuit(cmd))
		}
	}
}

// Enter must not apply a write: a habitual Enter is not consent.
func TestConfirm_EnterAndOtherKeysDoNothing(t *testing.T) {
	for name, msg := range map[string]tea.Msg{"enter": keyEnter, "space": tea.KeyMsg{Type: tea.KeySpace}, "x": ch('x'), "a": ch('a')} {
		m := newConfirm("Set?")
		_, cmd := m.Update(msg)
		if m.confirmed || m.cancelled || isQuit(cmd) {
			t.Errorf("%s: confirmed %v, cancelled %v, quit %v; want it ignored", name, m.confirmed, m.cancelled, isQuit(cmd))
		}
	}
}

func TestReview_AThenYAppliesAndNeedsNoEnter(t *testing.T) {
	m := newReview("clasm/DR-0001", "proposed", proposedOptions)
	if _, cmd := m.Update(ch('a')); isQuit(cmd) {
		t.Fatal("choosing a status ended the program; the confirmation still has to be answered")
	}
	if !strings.Contains(m.View(), "from proposed to accepted") {
		t.Errorf("the confirmation does not name the move: %q", m.View())
	}
	if _, cmd := m.Update(ch('y')); !isQuit(cmd) || !m.confirm.confirmed || m.chooser.chosen != "accepted" {
		t.Errorf("y: quit %v, confirmed %v, chosen %q", isQuit(cmd), m.confirm.confirmed, m.chooser.chosen)
	}
}

func TestReview_EveryBackOutIsAtOnce(t *testing.T) {
	for name, keys := range map[string][]tea.Msg{
		"q at the choice": {ch('q')}, "esc at the choice": {keyEsc},
		"n at the confirmation": {ch('r'), ch('n')}, "q at the confirmation": {ch('r'), ch('q')},
		"esc at the confirmation": {ch('r'), keyEsc},
	} {
		m := newReview("clasm/DR-0001", "proposed", proposedOptions)
		var cmd tea.Cmd
		for _, k := range keys {
			_, cmd = m.Update(k)
		}
		if !isQuit(cmd) || !m.cancelledOut() {
			t.Errorf("%s: quit %v, cancelled %v; want cancelled with no Enter", name, isQuit(cmd), m.cancelledOut())
		}
	}
}

func TestReview_EnterAtTheConfirmationDoesNotApply(t *testing.T) {
	m := newReview("clasm/DR-0001", "proposed", proposedOptions)
	m.Update(ch('r'))
	if _, cmd := m.Update(keyEnter); isQuit(cmd) || m.confirm.confirmed {
		t.Errorf("Enter applied or ended the review: quit %v, confirmed %v", isQuit(cmd), m.confirm.confirmed)
	}
}

func TestReview_KeysArrivingTogether(t *testing.T) {
	m := newReview("clasm/DR-0001", "proposed", proposedOptions)
	if _, cmd := m.Update(runeKey("ay")); !isQuit(cmd) || !m.confirm.confirmed || m.chooser.chosen != "accepted" {
		t.Errorf("ay in one message: quit %v, confirmed %v, chosen %q", isQuit(cmd), m.confirm.confirmed, m.chooser.chosen)
	}
}

// ─── text entry (DR-0064 amendment) ──────────────────────────────────────────

func TestTextCapture_OnlyTheTypedConfirmCapturesText(t *testing.T) {
	for name, m := range map[string]textCapturer{
		"chooser": newChooser("r", "proposed", proposedOptions), "confirm": newConfirm("?"), "review": newReview("r", "proposed", proposedOptions),
	} {
		if m.capturingText() {
			t.Errorf("%s reports capturing text", name)
		}
	}
	if !newTypedConfirm("clasm/DR-0001").capturingText() {
		t.Error("the typed confirmation does not report capturing text")
	}
}

// The edge clasm met: a name that starts with q. Every command letter is text
// while typing.
func TestTypedConfirm_CommandLettersAreText(t *testing.T) {
	m := newTypedConfirm("quokka/DR-0001")
	for _, r := range "qjkaynd:" {
		_, cmd := m.Update(ch(r))
		if isQuit(cmd) || m.cancelled || m.confirmed {
			t.Fatalf("typing %q acted as a command (quit %v, cancelled %v, confirmed %v)", r, isQuit(cmd), m.cancelled, m.confirmed)
		}
	}
	if m.value != "qjkaynd:" {
		t.Errorf("value = %q, want every typed key kept", m.value)
	}
}

func TestTypedConfirm_ANameStartingWithQConfirms(t *testing.T) {
	m := newTypedConfirm("quokka/DR-0001")
	for _, r := range "quokka/DR-0001" {
		m.Update(ch(r))
	}
	if _, cmd := m.Update(keyEnter); !m.confirmed || !isQuit(cmd) {
		t.Errorf("the exact name did not confirm: confirmed %v, quit %v", m.confirmed, isQuit(cmd))
	}
}

func TestTypedConfirm_EnterWithTheWrongTextDoesNotConfirm(t *testing.T) {
	m := newTypedConfirm("clasm/DR-0001")
	for _, r := range "clasm/DR-0002" {
		m.Update(ch(r))
	}
	if _, cmd := m.Update(keyEnter); m.confirmed || isQuit(cmd) {
		t.Errorf("a wrong name confirmed or ended the prompt: confirmed %v, quit %v", m.confirmed, isQuit(cmd))
	}
	if m.message == "" {
		t.Error("a wrong name should say it does not match")
	}
}

func TestTypedConfirm_EscAndCtrlCCancel(t *testing.T) {
	for name, msg := range map[string]tea.Msg{"esc": keyEsc, "ctrl+c": keyCtrlC} {
		m := newTypedConfirm("clasm/DR-0001")
		m.Update(ch('c'))
		_, cmd := m.Update(msg)
		if !m.cancelled || m.confirmed || !isQuit(cmd) {
			t.Errorf("%s: cancelled %v, confirmed %v, quit %v", name, m.cancelled, m.confirmed, isQuit(cmd))
		}
	}
}

func TestTypedConfirm_BackspaceEdits(t *testing.T) {
	m := newTypedConfirm("ab")
	m.Update(runeKey("abx"))
	m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if _, cmd := m.Update(keyEnter); !m.confirmed || !isQuit(cmd) {
		t.Errorf("after a backspace the text should match: value %q", m.value)
	}
}

// ─── the program ─────────────────────────────────────────────────────────────

// runReviewWithin runs runReview and fails the test if it does not return, which
// is what a program waiting forever for a key would do.
func runReviewWithin(t *testing.T, input string) (chosen string, confirmed bool, out string) {
	t.Helper()
	type result struct {
		chosen    string
		confirmed bool
		err       error
	}
	done := make(chan result, 1)
	var buf bytes.Buffer
	go func() {
		c, ok, err := runReview(strings.NewReader(input), &buf, "clasm/DR-0001", "proposed", proposedOptions)
		done <- result{c, ok, err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("runReview(%q) = %v", input, r.err)
		}
		return r.chosen, r.confirmed, buf.String()
	case <-time.After(5 * time.Second):
		t.Fatalf("runReview(%q) did not return: it is waiting for a key that will never come", input)
		return
	}
}

func TestRunReview_KeysFromAReader(t *testing.T) {
	chosen, confirmed, out := runReviewWithin(t, "ay")
	if chosen != "accepted" || !confirmed {
		t.Errorf("ay: chosen %q, confirmed %v", chosen, confirmed)
	}
	// Both keys arrived in one read, so the program ended before a timed frame was
	// drawn; the last view says what was done.
	if !strings.Contains(out, "from proposed to accepted? yes") {
		t.Errorf("the final view does not say what was done: %q", out)
	}
}

func TestRunReview_ALoneEscCancelsWithNoEnter(t *testing.T) {
	if chosen, confirmed, _ := runReviewWithin(t, "\x1b"); chosen != "" || confirmed {
		t.Errorf("Esc: chosen %q, confirmed %v; want a cancel", chosen, confirmed)
	}
}

// End of input must end the program as a cancel, not hang it.
func TestRunReview_EndOfInputIsACancel(t *testing.T) {
	for name, input := range map[string]string{"nothing": "", "after choosing": "r", "enter only": "r\r"} {
		if chosen, confirmed, _ := runReviewWithin(t, input); confirmed {
			t.Errorf("%s: confirmed (chosen %q); want a cancel", name, chosen)
		}
	}
}
