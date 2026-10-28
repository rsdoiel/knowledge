package cli

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

/** listFilter is the filter of one list in the terminal interface: the rows
 * before it, the text that narrows them, and the prompt while it is being typed
 * (knowledge DR-0072). `/` opens it on whatever list has focus. It is the
 * model's own and not the list component's, which re-enables keys the model
 * owns whenever its state changes.
 *
 * Fields:
 *   all    ([]list.Item)     — the list's rows before the filter.
 *   text   (string)          — the kept filter; "" for none.
 *   typing (bool)            — the filter is being typed.
 *   input  (textinput.Model) — the prompt.
 */
type listFilter struct {
	all    []list.Item
	text   string
	typing bool
	input  textinput.Model
}

// listOf returns the list a screen shows, or nil for a screen that has none.
func (m *tuiModel) listOf(state viewState) *list.Model {
	switch state {
	case viewProjects:
		return &m.projectList
	case viewObservations:
		return &m.observationList
	case viewConcepts:
		return &m.conceptList
	case viewRecords:
		return &m.recordList
	case viewRecordScope:
		return &m.scopeList
	case viewReviewQueue:
		return &m.queueList
	case viewSearch:
		return &m.searchList
	case viewBrowse:
		return &m.browseList
	case viewUnlink:
		return &m.unlinkList
	}
	return nil
}

// filterNoun names what a screen's rows are, for "no concept matches ...".
func (m *tuiModel) filterNoun(state viewState) string {
	switch state {
	case viewProjects:
		return "project"
	case viewObservations:
		return "observation"
	case viewConcepts:
		return "concept"
	case viewRecords, viewRecordScope:
		return "record"
	case viewReviewQueue:
		return "section"
	case viewSearch:
		return "result"
	case viewBrowse:
		return m.browseKind
	case viewUnlink:
		return "link"
	}
	return "row"
}

// clearFilter forgets a screen's filter. A list that is loaded again starts
// whole, so its loader calls this.
func (m *tuiModel) clearFilter(state viewState) {
	delete(m.filters, state)
}

// filterOf returns the filter of the current screen, or nil when it has none in
// force.
func (m *tuiModel) filterOf(state viewState) *listFilter {
	if lf := m.filters[state]; lf != nil && (lf.typing || lf.text != "") {
		return lf
	}
	return nil
}

// filterTyping reports whether a filter is being typed on the current screen.
func (m *tuiModel) filterTyping() bool {
	lf := m.filters[m.state]
	return lf != nil && lf.typing
}

// applyFilter shows the rows of the screen's list that match text and puts the
// cursor on the first.
func (m *tuiModel) applyFilter(state viewState, lf *listFilter, text string) {
	l := m.listOf(state)
	l.SetItems(filterItems(lf.all, text))
	l.Select(0)
}

/** filterKey handles a key for the filter of the list that has focus: `/` opens
 * it, a typed key goes to it, Enter keeps it and Esc abandons it, or clears a
 * kept one. It returns true when it took the key. Where a screen has no list, or
 * a prompt of its own is open, it takes nothing.
 *
 * Parameters:
 *   msg (tea.KeyPressMsg) — the key
 *
 * Returns:
 *   (tea.Cmd, bool) — the text input's command, and whether the key was taken
 *
 * Example:
 *   if cmd, took := m.filterKey(msg); took { return m, cmd }
 */
func (m *tuiModel) filterKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	l := m.listOf(m.state)
	if l == nil || m.searching || m.err != nil {
		return nil, false
	}
	lf := m.filters[m.state]
	if lf != nil && lf.typing {
		switch {
		case isEsc(msg):
			lf.typing = false
			lf.input.Blur()
			lf.text = ""
			m.applyFilter(m.state, lf, "")
			m.clearFilter(m.state)
			return nil, true
		case msg.Code == tea.KeyEnter:
			lf.typing = false
			lf.input.Blur()
			lf.text = strings.TrimSpace(lf.input.Value())
			m.applyFilter(m.state, lf, lf.text)
			if lf.text == "" {
				m.clearFilter(m.state)
			}
			return nil, true
		}
		var cmd tea.Cmd
		lf.input, cmd = lf.input.Update(msg)
		m.applyFilter(m.state, lf, lf.input.Value())
		return cmd, true
	}
	switch msg.String() {
	case "/":
		if lf == nil {
			lf = &listFilter{all: l.Items(), input: newFindInput()}
			if m.filters == nil {
				m.filters = map[viewState]*listFilter{}
			}
			m.filters[m.state] = lf
		}
		lf.typing = true
		lf.input.SetValue(lf.text)
		lf.input.CursorEnd()
		lf.input.Focus()
		return nil, true
	case "esc":
		if lf != nil && lf.text != "" {
			m.applyFilter(m.state, lf, "")
			m.clearFilter(m.state)
			return nil, true
		}
	}
	return nil, false
}

// filterLegend is the legend while a filter is typed, or the screen's own legend
// with `/ filter` where a list has focus, and `S search` too when the window has
// the room: the room is tight at 80 columns, where the screens' own keys come
// first and S is named on the menus and in the empty-filter line.
func (m *tuiModel) filterLegend(legend string, width int) string {
	if lf := m.filters[m.state]; lf != nil && lf.typing {
		return "Filter: " + lf.input.View() + "   Enter keep   Esc cancel"
	}
	if m.listOf(m.state) == nil {
		return legend
	}
	base := legend
	if !strings.Contains(base, "/ filter") {
		base = strings.Replace(base, "S search", "/ filter", 1)
	}
	withS := strings.Replace(base, "q back", "S search   q back", 1)
	if withS != base && lipgloss.Width(strings.ReplaceAll(withS, "   ", "  ")) <= width {
		return withS
	}
	return base
}

// filteredLines is a list's body: its rows, or, when a filter hides every row, a
// line that says so and names the search.
func (m *tuiModel) filteredLines(state viewState) []string {
	l := m.listOf(state)
	if lf := m.filterOf(state); lf != nil && len(l.Items()) == 0 {
		text := lf.text
		if lf.typing {
			text = strings.TrimSpace(lf.input.Value())
		}
		if text != "" {
			return []string{fmt.Sprintf("no %s matches %q; S searches the knowledge base for it", m.filterNoun(state), text)}
		}
	}
	return viewLines(*l)
}

// totalRows is the number of rows a list has before its filter, for a tab's count.
func (m *tuiModel) totalRows(state viewState) int {
	if lf := m.filters[state]; lf != nil {
		return len(lf.all)
	}
	return len(m.listOf(state).Items())
}

// filterTitle adds a kept filter to a screen's title, with the key that clears it,
// so the legend keeps the room for the screen's own keys.
func (m *tuiModel) filterTitle(title string) string {
	if lf := m.filterOf(m.state); lf != nil && lf.text != "" && !lf.typing {
		return fmt.Sprintf("%s  ·  filter %q (Esc clears)", title, lf.text)
	}
	return title
}
