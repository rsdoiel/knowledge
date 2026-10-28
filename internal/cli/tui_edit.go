package cli

import (
	bkey "charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

/** editModel is a text field that starts from a current value and takes a new one
 * (knowledge DR-0067, DR-0064 amendment). It captures text: every printable key is
 * a character, including q, j, k, y and n. Enter submits and Esc cancels; Ctrl-J
 * inserts a newline, so a value of several lines can be typed and Enter stays the
 * key that accepts. An unchanged value, and an empty one where that is not allowed,
 * are not submitted and say why.
 *
 * Fields:
 *   ta         (textarea.Model) — the field.
 *   old        (string)         — the value it started from.
 *   allowEmpty (bool)           — whether clearing the value is allowed.
 *   allowSame  (bool)           — whether the starting value may be accepted unchanged.
 *   message    (string)         — why the last Enter did nothing.
 *   submitted  (bool)           — true once a new value was accepted.
 *   cancelled  (bool)           — true once the person backed out.
 */
type editModel struct {
	ta         textarea.Model
	old        string
	allowEmpty bool
	allowSame  bool // accept the starting value as it is (a field being filled in, not changed)
	message    string
	submitted  bool
	cancelled  bool
}

func newEdit(old string, allowEmpty bool, width int) *editModel {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetWidth(max(width, 20))
	ta.SetHeight(4)
	ta.KeyMap.InsertNewline = bkey.NewBinding(bkey.WithKeys("ctrl+j"))
	ta.SetValue(old)
	ta.Focus()
	return &editModel{ta: ta, old: old, allowEmpty: allowEmpty}
}

// value is what has been typed.
func (m *editModel) value() string { return m.ta.Value() }

func (m *editModel) Init() tea.Cmd { return nil }

func (m *editModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case isEsc(km) || isCtrlC(km):
		m.cancelled = true
		return m, nil
	case km.Code == tea.KeyEnter:
		v := m.ta.Value()
		switch {
		case v == "" && !m.allowEmpty:
			m.message = "empty (required): type something, or Esc to cancel"
		case v == m.old && !m.allowSame:
			m.message = "unchanged: edit it, or Esc to cancel"
		default:
			m.submitted = true
		}
		return m, nil
	}
	m.message = ""
	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	return m, cmd
}

// View wraps render for Bubble Tea v2, which draws a tea.View.
func (m *editModel) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = false
	return v
}

func (m *editModel) render() string {
	v := m.ta.View()
	if m.message != "" {
		v += "\n" + m.message
	}
	return v
}

func (m *editModel) capturingText() bool { return true }
