package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/term"

	tea "github.com/charmbracelet/bubbletea"
)

/** textCapturer is implemented by every model that can be taking text. A key
 * handler asks it before binding any letter (knowledge DR-0064, amendment):
 * while text is being captured, q, j, k, y, n and : are characters, and only
 * Esc, Enter and Ctrl-C are special.
 *
 * Example:
 *   if c, ok := m.(textCapturer); ok && c.capturingText() { forwardToInput(msg) }
 */
type textCapturer interface {
	capturingText() bool
}

/** keysOf splits a key message into single keys. Several characters can arrive
 * in one message (a fast typist, a paste, a test); the chooser and the
 * confirmation take them one at a time so each is decided on its own.
 *
 * Parameters:
 *   msg (tea.KeyMsg) — the message bubbletea delivered
 *
 * Returns:
 *   []tea.KeyMsg — one message per typed character, or msg itself when it is one key
 *
 * Example:
 *   keysOf(runeKey("ay")) // two messages, a then y
 */
func keysOf(msg tea.KeyMsg) []tea.KeyMsg {
	if msg.Type != tea.KeyRunes || msg.Paste || len(msg.Runes) < 2 {
		return []tea.KeyMsg{msg}
	}
	keys := make([]tea.KeyMsg, len(msg.Runes))
	for i, r := range msg.Runes {
		keys[i] = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: msg.Alt}
	}
	return keys
}

// typedRune returns the single character a key message carries, lower-cased, or
// 0 when it is not one typed character.
func typedRune(k tea.KeyMsg) rune {
	if k.Type == tea.KeyRunes && len(k.Runes) == 1 && !k.Alt {
		return unicode.ToLower(k.Runes[0])
	}
	return 0
}

// isBackOut reports whether a key leaves a choice or confirmation without
// acting: Esc and Ctrl-C always, and q where letters are commands (DR-0064).
// Alt is ignored so the Esc that a terminal glues to the next byte still counts.
func isBackOut(k tea.KeyMsg) bool {
	return k.Type == tea.KeyEsc || k.Type == tea.KeyCtrlC || typedRune(k) == 'q'
}

/** chooserModel offers the moves a record may make and takes one key to pick
 * one (knowledge DR-0065). It is a plain tea.Model so the TUI can embed it. A
 * status letter chooses at once; q, Esc and Ctrl-C cancel at once; Enter and
 * every other key are ignored. It never captures text.
 *
 * Fields:
 *   ref, from (string)    — the record and its current status, for the prompt.
 *   options   ([]string)  — the statuses on offer, in the order shown.
 *   chosen    (string)    — the status picked, "" until one is.
 *   cancelled (bool)      — true once the person backed out.
 */
type chooserModel struct {
	ref, from string
	options   []string
	keys      map[string]rune // an option's key when it is not its first letter
	prompt    string          // replaces "REF is FROM ->" when a choice is not a move
	chosen    string
	cancelled bool
}

// keyOf is the key that picks an option: its entry in keys, else its first letter.
func (m *chooserModel) keyOf(option string) rune {
	if k, ok := m.keys[option]; ok {
		return k
	}
	return statusKey(option)
}

func newChooser(ref, from string, options []string) *chooserModel {
	return &chooserModel{ref: ref, from: from, options: options}
}

// step takes one key and reports whether it decided the choice.
func (m *chooserModel) step(k tea.KeyMsg) bool {
	if isBackOut(k) {
		m.cancelled = true
		return true
	}
	if r := typedRune(k); r != 0 {
		for _, o := range m.options {
			if m.keyOf(o) == r {
				m.chosen = o
				return true
			}
		}
	}
	return false
}

func (m *chooserModel) Init() tea.Cmd { return nil }

func (m *chooserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		for _, k := range keysOf(km) {
			if m.step(k) {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m *chooserModel) View() string {
	labels := make([]string, 0, len(m.options)+1)
	for _, o := range m.options {
		k := m.keyOf(o)
		// The key is shown inside the word where it falls: [c]oncept, conc[l]uded.
		if i := strings.IndexRune(strings.ToLower(o), k); i >= 0 {
			labels = append(labels, o[:i]+"["+string(k)+"]"+o[i+1:])
		} else {
			labels = append(labels, fmt.Sprintf("[%c] %s", k, o))
		}
	}
	labels = append(labels, "[q]uit")
	if m.prompt != "" {
		return m.prompt + " " + strings.Join(labels, " ")
	}
	return fmt.Sprintf("%s is %s -> %s", m.ref, m.from, strings.Join(labels, " "))
}

func (m *chooserModel) capturingText() bool { return false }

/** confirmModel asks for a single y to apply; n, q, Esc and Ctrl-C cancel at
 * once. Enter and every other key do nothing, so a habitual Enter is not
 * consent to a write (DR-0065). It never captures text.
 *
 * Fields:
 *   question  (string) — what is being confirmed.
 *   confirmed (bool)   — true once y was pressed.
 *   cancelled (bool)   — true once the person backed out.
 */
type confirmModel struct {
	question  string
	confirmed bool
	cancelled bool
}

func newConfirm(question string) *confirmModel { return &confirmModel{question: question} }

// step takes one key and reports whether it decided the confirmation.
func (m *confirmModel) step(k tea.KeyMsg) bool {
	switch {
	case typedRune(k) == 'y':
		m.confirmed = true
		return true
	case isBackOut(k) || typedRune(k) == 'n':
		m.cancelled = true
		return true
	}
	return false
}

func (m *confirmModel) Init() tea.Cmd { return nil }

func (m *confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		for _, k := range keysOf(km) {
			if m.step(k) {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m *confirmModel) View() string {
	return m.question + "  [y] apply  [n/q/Esc] cancel"
}

func (m *confirmModel) capturingText() bool { return false }

// Phases of a review.
const (
	phaseChoose = iota
	phaseConfirm
	phaseDone
)

/** reviewModel is the chooser followed by the confirmation, as one program: the
 * choice is made on one key, the move is confirmed on a second, and either can be
 * backed out of at once. It is what kb record set-status REF runs, and what the
 * TUI embeds around its own record view.
 *
 * Fields:
 *   chooser (chooserModel) — the first phase.
 *   confirm (confirmModel) — the second phase, built when a status is chosen.
 *   phase   (int)          — phaseChoose, phaseConfirm or phaseDone.
 */
type reviewModel struct {
	chooser chooserModel
	confirm confirmModel
	phase   int
}

func newReview(ref, from string, options []string) *reviewModel {
	return &reviewModel{chooser: *newChooser(ref, from, options)}
}

func (m *reviewModel) Init() tea.Cmd { return nil }

func (m *reviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	for _, k := range keysOf(km) {
		switch m.phase {
		case phaseChoose:
			if m.chooser.step(k) {
				if m.chooser.cancelled {
					m.phase = phaseDone
					return m, tea.Quit
				}
				m.confirm = *newConfirm(fmt.Sprintf("Set %s from %s to %s?", m.chooser.ref, m.chooser.from, m.chooser.chosen))
				m.phase = phaseConfirm
			}
		case phaseConfirm:
			if m.confirm.step(k) {
				m.phase = phaseDone
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m *reviewModel) View() string {
	switch m.phase {
	case phaseConfirm:
		return m.confirm.View()
	case phaseDone:
		if m.confirm.confirmed {
			return m.confirm.question + " yes"
		}
		return "cancelled"
	}
	return m.chooser.View()
}

func (m *reviewModel) capturingText() bool { return false }

// cancelledOut reports whether the review ended without a confirmed choice.
func (m *reviewModel) cancelledOut() bool { return m.chooser.cancelled || m.confirm.cancelled }

// result returns the chosen status and whether it was confirmed; both are
// zero when the person backed out or the input ended.
func (m *reviewModel) result() (chosen string, confirmed bool) {
	if m.confirm.confirmed {
		return m.chooser.chosen, true
	}
	return "", false
}

/** typedConfirmModel asks the person to type a name or reference exactly to
 * confirm a removal (knowledge DR-0067). It captures text: every printable key
 * is a character, including q, j, k, y, n and :, so a name that starts with q
 * works. Enter confirms only when the text matches; Esc and Ctrl-C cancel.
 *
 * Fields:
 *   expected  (string) — what must be typed.
 *   value     (string) — what has been typed.
 *   message   (string) — a line saying the last Enter did not match.
 *   confirmed (bool)   — true once the exact text was entered.
 *   cancelled (bool)   — true once the person backed out.
 */
type typedConfirmModel struct {
	expected  string
	value     string
	message   string
	confirmed bool
	cancelled bool
}

func newTypedConfirm(expected string) *typedConfirmModel {
	return &typedConfirmModel{expected: expected}
}

func (m *typedConfirmModel) Init() tea.Cmd { return nil }

func (m *typedConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.cancelled = true
		return m, tea.Quit
	case tea.KeyEnter:
		if m.value == m.expected {
			m.confirmed = true
			return m, tea.Quit
		}
		m.message = fmt.Sprintf("%q does not match; type %q exactly, or Esc to cancel", m.value, m.expected)
	case tea.KeyBackspace:
		if r := []rune(m.value); len(r) > 0 {
			m.value = string(r[:len(r)-1])
		}
		m.message = ""
	case tea.KeyCtrlU:
		m.value, m.message = "", ""
	case tea.KeyRunes, tea.KeySpace:
		if km.Type == tea.KeySpace {
			m.value += " "
		} else {
			m.value += string(km.Runes)
		}
		m.message = ""
	}
	return m, nil
}

func (m *typedConfirmModel) View() string {
	v := fmt.Sprintf("Type %s to confirm: %s", m.expected, m.value)
	if m.message != "" {
		v += "\n" + m.message
	}
	return v + "\n[Esc] cancel  [Enter] accept"
}

func (m *typedConfirmModel) capturingText() bool { return true }

// cancelAtEOF turns the end of a reader that is not a terminal into a Ctrl-C, so
// a review fed from a pipe or a test cancels where a terminal would wait for a
// key that is never coming.
type cancelAtEOF struct {
	r    io.Reader
	eof  bool
	sent bool
}

func (c *cancelAtEOF) Read(p []byte) (int, error) {
	if c.eof {
		if !c.sent && len(p) > 0 {
			c.sent = true
			p[0] = 0x03
			return 1, nil
		}
		return 0, io.EOF
	}
	n, err := c.r.Read(p)
	if err != nil {
		c.eof = true
		if n > 0 {
			return n, nil
		}
		return c.Read(p)
	}
	return n, nil
}

// reviewInput leaves a terminal as it is, so bubbletea can put it in raw mode, and
// wraps anything else so its end cancels.
func reviewInput(in io.Reader) io.Reader {
	if f, ok := in.(*os.File); ok && term.IsTerminal(f.Fd()) {
		return f
	}
	return &cancelAtEOF{r: in}
}

/** runReview runs the chooser and confirmation as an inline program (no
 * alternate screen) reading single keys from in and drawing on out. Esc and q
 * act at once, with no Enter (knowledge DR-0065).
 *
 * Parameters:
 *   in (io.Reader) — where the keys come from; a terminal, or any reader (its end cancels)
 *   out (io.Writer) — where the prompt is drawn
 *   ref (string) — the record's qualified reference
 *   from (string) — its current status
 *   options ([]string) — the statuses to offer
 *
 * Returns:
 *   chosen (string) — the status picked, "" when the person backed out
 *   confirmed (bool) — true only when the move was confirmed with y
 *   err (error) — the program failed to run
 *
 * Example:
 *   chosen, ok, err := runReview(os.Stdin, os.Stdout, "clasm/DR-0004", "proposed", []string{"accepted", "rejected"})
 */
func runReview(in io.Reader, out io.Writer, ref, from string, options []string) (chosen string, confirmed bool, err error) {
	m := newReview(ref, from, options)
	if _, err := tea.NewProgram(m, tea.WithInput(reviewInput(in)), tea.WithOutput(out)).Run(); err != nil {
		return "", false, err
	}
	chosen, confirmed = m.result()
	return chosen, confirmed, nil
}
