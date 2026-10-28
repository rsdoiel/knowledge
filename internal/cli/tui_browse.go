package cli

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	knowledge "github.com/rsdoiel/knowledge"
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
	id          int64 // the row's id, for a source or document; 0 for a concept
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
	items, err := m.browseItems(kind)
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	if len(items) == 0 {
		m.notice = fmt.Sprintf("no %ss yet", kind)
		return m, nil
	}
	m.browseKind = kind
	m.clearFilter(viewBrowse)
	m.browseList = newBrowserList(m.isDark, items, strings.ToUpper(kind[:1])+kind[1:]+"s", m.cols()-4, m.listHeight())
	m.setState(viewBrowse)
	return m, nil
}

// browseItems loads the rows of a Browse list.
func (m *tuiModel) browseItems(kind string) ([]list.Item, error) {
	var items []list.Item
	switch kind {
	case "concept":
		cs, err := m.kb.Concepts()
		if err != nil {
			return nil, err
		}
		for _, c := range cs {
			items = append(items, browseItem{title: c.Name, desc: c.Description, show: []string{"concept", "show", "--", c.Name}})
		}
	case "source":
		ss, err := m.kb.ListSources()
		if err != nil {
			return nil, err
		}
		for _, s := range ss {
			desc := s.Authors
			if s.IdentifierValue != "" {
				desc = strings.TrimSpace(desc + "  " + s.IdentifierType + ":" + s.IdentifierValue)
			}
			if s.Retracted {
				desc = "RETRACTED  " + desc
			}
			items = append(items, browseItem{title: s.Title, desc: desc, show: []string{"source", "show", strconv.FormatInt(s.ID, 10)}, id: s.ID})
		}
	case "document":
		ds, err := m.kb.Documents(0)
		if err != nil {
			return nil, err
		}
		for _, d := range ds {
			items = append(items, browseItem{title: d.Title, desc: d.Format + "  " + d.Path, show: []string{"document", "show", strconv.FormatInt(d.ID, 10)}, id: d.ID})
		}
	default:
		return nil, fmt.Errorf("no list for %q", kind)
	}
	return items, nil
}

/** filterItems returns the rows whose title and description contain every word of
 * filter, ignoring case. An empty filter keeps every row. This is the filter of a
 * Browse list: it narrows the rows on screen and does not touch the index.
 *
 * Parameters:
 *   items ([]list.Item) — the rows
 *   filter (string) — the words to find, separated by spaces
 *
 * Returns:
 *   []list.Item — the rows that match, in their order
 *
 * Example:
 *   shown := filterItems(all, "cache inval")
 */
func filterItems(items []list.Item, filter string) []list.Item {
	words := strings.Fields(strings.ToLower(filter))
	if len(words) == 0 {
		return items
	}
	var out []list.Item
	for _, it := range items {
		v := strings.ToLower(it.FilterValue())
		ok := true
		for _, w := range words {
			if !strings.Contains(v, w) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, it)
		}
	}
	return out
}

// setBrowseItems replaces the rows of the Browse list and shows those that match
// the filter in force.
func (m *tuiModel) setBrowseItems(items []list.Item) {
	if lf := m.filters[viewBrowse]; lf != nil && lf.text != "" {
		lf.all = items
		m.applyFilter(viewBrowse, lf, lf.text)
		return
	}
	m.clearFilter(viewBrowse)
	m.browseList.SetItems(items)
}

// updateBrowse handles a key on a Browse list: Enter shows the item, q goes back
// to the group menu and d deletes. `/` filters the rows and S searches; both are
// taken before this is reached.
func (m *tuiModel) updateBrowse(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.setState(viewGroup)
		return m, nil
	case "esc":
		return m, nil
	case "d":
		return m.removeSelected()
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

/** showSearchResult shows the row a search result names through the command's own
 * show, with the command that does the same. A result the index holds by a type
 * with no show of its own (a document summary) says where to find it.
 *
 * Parameters:
 *   r (knowledge.KBSearchResult) — the selected result
 *
 * Returns:
 *   (tea.Model, tea.Cmd) — the model, showing the item or a notice
 *
 * Example:
 *   return m.showSearchResult(it.r)
 */
func (m *tuiModel) showSearchResult(r knowledge.KBSearchResult) (tea.Model, tea.Cmd) {
	var args []string
	switch r.SourceType {
	case "observation":
		args = []string{"observation", "show", strconv.FormatInt(r.SourceID, 10)}
	case "concept":
		args = []string{"concept", "show", "--", r.Label}
	case "project":
		args = []string{"project", "show", r.Label}
	case "record":
		rec, err := m.kb.RecordByID(r.SourceID)
		if err != nil || rec == nil {
			m.notice = fmt.Sprintf("the record this result names is no longer in the database (id %d)", r.SourceID)
			return m, nil
		}
		args = []string{"record", "show", refOf(*rec, projectNames(m.kb)).String()}
	default:
		m.notice = fmt.Sprintf("a %s has no screen of its own here; Documents → Review queue lists summaries", r.SourceType)
		return m, nil
	}
	return m.showReadFrom(args, viewSearch)
}
