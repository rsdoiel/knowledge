package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

/** browseItem is one row of a Browse list: a concept, source or document, and the
 * `show` command that Enter runs on it.
 *
 * Fields:
 *   title (string)   — the row's first line.
 *   desc  (string)   — its second line.
 *   show  ([]string) — the arguments of the show command, such as {"source", "show", "3"}.
 */
type browseItem struct {
	title, desc string
	show        []string
}

func (i browseItem) Title() string       { return i.title }
func (i browseItem) Description() string { return i.desc }
func (i browseItem) FilterValue() string { return i.title + " " + i.desc }

/** openBrowse shows every concept, source or document as a list to choose from;
 * Enter on a row shows that item through the command's own show, with the command
 * that does the same. An empty list says so on the menu it came from.
 *
 * Parameters:
 *   kind (string) — "concept", "source" or "document"
 *
 * Returns:
 *   (tea.Model, tea.Cmd) — the model, on the list or on the menu with a notice
 *
 * Example:
 *   return m.openBrowse("source")
 */
func (m *tuiModel) openBrowse(kind string) (tea.Model, tea.Cmd) {
	var items []list.Item
	switch kind {
	case "concept":
		cs, err := m.kb.Concepts()
		if err != nil {
			m.notice = err.Error()
			return m, nil
		}
		for _, c := range cs {
			items = append(items, browseItem{c.Name, c.Description, []string{"concept", "show", "--", c.Name}})
		}
	case "source":
		ss, err := m.kb.ListSources()
		if err != nil {
			m.notice = err.Error()
			return m, nil
		}
		for _, s := range ss {
			desc := s.Authors
			if s.IdentifierValue != "" {
				desc = strings.TrimSpace(desc + "  " + s.IdentifierType + ":" + s.IdentifierValue)
			}
			if s.Retracted {
				desc = "RETRACTED  " + desc
			}
			items = append(items, browseItem{s.Title, desc, []string{"source", "show", strconv.FormatInt(s.ID, 10)}})
		}
	case "document":
		ds, err := m.kb.Documents(0)
		if err != nil {
			m.notice = err.Error()
			return m, nil
		}
		for _, d := range ds {
			items = append(items, browseItem{d.Title, d.Format + "  " + d.Path, []string{"document", "show", strconv.FormatInt(d.ID, 10)}})
		}
	default:
		m.notice = fmt.Sprintf("no list for %q", kind)
		return m, nil
	}
	if len(items) == 0 {
		m.notice = fmt.Sprintf("no %ss yet", kind)
		return m, nil
	}
	m.browseKind = kind
	m.browseList = newBrowserList(items, strings.ToUpper(kind[:1])+kind[1:]+"s", m.cols()-4, m.listHeight())
	m.setState(viewBrowse)
	return m, nil
}

// updateBrowse handles a key on a Browse list: Enter shows the item, q goes back to
// the group menu, / searches.
func (m *tuiModel) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.setState(viewGroup)
		return m, nil
	case "esc":
		return m, nil
	case "/":
		m.startSearch()
		return m, nil
	case "enter":
		if it, ok := m.browseList.SelectedItem().(browseItem); ok {
			return m.showReadFrom(it.show, viewBrowse)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.browseList, cmd = m.browseList.Update(msg)
	return m, cmd
}
