package main

import (
	"fmt"

	bkey "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	knowledge "github.com/rsdoiel/knowledge"
)

// viewState names which list is currently shown.
type viewState int

const (
	viewProjects viewState = iota
	viewObservations
	viewConcepts
	viewSearch
	viewRecords
)

// projectItem, observationItem, conceptItem, and searchResultItem each
// wrap (rather than embed) their knowledge.* value, since embedding would
// collide list.DefaultItem's required Description() method with the
// wrapped type's own Description field (Project, Observation both have
// one).
type projectItem struct{ p knowledge.Project }

func (i projectItem) Title() string { return i.p.Name }
func (i projectItem) Description() string {
	return fmt.Sprintf("[%s] %s", i.p.Status, i.p.Description)
}
func (i projectItem) FilterValue() string { return i.p.Name }

type observationItem struct{ o knowledge.Observation }

func (i observationItem) Title() string       { return fmt.Sprintf("[%s] %s", i.o.Kind, i.o.Body) }
func (i observationItem) Description() string { return i.o.CreatedAt.Format("2006-01-02 15:04") }
func (i observationItem) FilterValue() string { return i.o.Body }

// recordItem wraps a decision record for the list. The record's status, kind
// and supersession are what a reader scans for, so they lead the description
// rather than the date.
type recordItem struct{ r knowledge.Record }

func (i recordItem) Title() string { return fmt.Sprintf("DR-%s  %s", i.r.RecordID, i.r.Title) }
func (i recordItem) Description() string {
	trigger := i.r.Trigger
	if trigger == "" {
		trigger = "-"
	}
	return fmt.Sprintf("%s  %s  %s  %s", i.r.Date, i.r.Status, i.r.Kind, trigger)
}
func (i recordItem) FilterValue() string { return i.r.RecordID + " " + i.r.Title }

type conceptItem struct{ c knowledge.Concept }

func (i conceptItem) Title() string       { return i.c.Name }
func (i conceptItem) Description() string { return i.c.Description }
func (i conceptItem) FilterValue() string { return i.c.Name }

type searchResultItem struct{ r knowledge.KBSearchResult }

func (i searchResultItem) Title() string       { return fmt.Sprintf("[%s] %s", i.r.Kind, i.r.Label) }
func (i searchResultItem) Description() string { return i.r.Snippet }
func (i searchResultItem) FilterValue() string { return i.r.Label + " " + i.r.Snippet }

// tuiModel is the read-mostly browser: project list (root) -> Enter drills
// into that project's observations ('c'/'o' toggles to/from its concepts)
// -> '/' opens a search prompt from any view, showing results in their own
// list. No add/edit/link/retract in this version -- see cli-tui-design.md
// decision 4.
type tuiModel struct {
	kb    *knowledge.KnowledgeBase
	dl    *DebugLog
	state viewState

	projectList     list.Model
	observationList list.Model
	conceptList     list.Model
	searchList      list.Model
	recordList      list.Model
	searchInput     textinput.Model
	searching       bool
	searchFrom      viewState // where a search began, which q from its results returns to

	selectedProject *knowledge.Project
	err             error
	width, height   int
}

func newTUIModel(kb *knowledge.KnowledgeBase, dl *DebugLog) (*tuiModel, error) {
	projects, err := logKBCall(dl, "Projects", nil, kb.Projects)
	if err != nil {
		return nil, err
	}
	items := make([]list.Item, len(projects))
	for i, p := range projects {
		items[i] = projectItem{p}
	}
	projectList := newBrowserList(items, "Projects", 0, 0)

	ti := textinput.New()
	ti.Placeholder = "search term"

	return &tuiModel{
		kb:    kb,
		dl:    dl,
		state: viewProjects,

		projectList: projectList,
		// Constructed empty (not left as a zero-value list.Model{}) so
		// that a WindowSizeMsg arriving before the user has drilled into
		// anything can still call SetSize on these safely -- list.Model
		// has internal state a zero value doesn't populate, and calling
		// its methods before list.New has run panics.
		observationList: newBrowserList(nil, "", 0, 0),
		conceptList:     newBrowserList(nil, "", 0, 0),
		recordList:      newBrowserList(nil, "", 0, 0),
		searchList:      newBrowserList(nil, "", 0, 0),
		searchInput:     ti,
	}, nil
}

func (m *tuiModel) Init() tea.Cmd { return nil }

// viewStateNames gives a readable name per viewState for debug-log field
// values -- never shown in the actual UI.
var viewStateNames = map[viewState]string{
	viewProjects:     "viewProjects",
	viewObservations: "viewObservations",
	viewConcepts:     "viewConcepts",
	viewSearch:       "viewSearch",
	viewRecords:      "viewRecords",
}

// setState logs the transition (if it's an actual change) before applying
// it. Every m.state assignment in this file goes through this instead of
// a bare field write, so --debug sees every view change.
func (m *tuiModel) setState(newState viewState) {
	if newState != m.state {
		m.dl.Log("tui_state_change", map[string]any{"from": viewStateNames[m.state], "to": viewStateNames[newState]})
	}
	m.state = newState
}

// setErr logs the error (if non-nil) before storing it -- every m.err
// assignment in this file goes through this instead of a bare field
// write, so --debug captures the failure at the point it happened, not
// just whatever View() happens to render afterward.
func (m *tuiModel) setErr(err error) {
	if err != nil {
		m.dl.Log("tui_error", map[string]any{"error": err.Error()})
	}
	m.err = err
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	msgFields := map[string]any{"msg_type": fmt.Sprintf("%T", msg)}
	if km, ok := msg.(tea.KeyMsg); ok {
		msgFields["key"] = km.String()
	}
	m.dl.Log("tui_msg", msgFields)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		h := m.listHeight()
		for _, l := range []*list.Model{&m.projectList, &m.observationList, &m.conceptList, &m.recordList, &m.searchList} {
			l.SetSize(msg.Width, h)
		}
		return m, nil

	case tea.KeyMsg:
		// Ctrl-C quits from everywhere, typing and errors included, and writes
		// nothing (DR-0064).
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		if m.err != nil {
			return m.updateError(msg)
		}
		// While text is being typed every printable key is text, so this is
		// asked before any letter is bound (DR-0064, amendment).
		if m.capturingText() {
			return m.updateSearching(msg)
		}
		switch m.state {
		case viewObservations:
			return m.updateObservations(msg)
		case viewConcepts:
			return m.updateConcepts(msg)
		case viewSearch:
			return m.updateSearchResults(msg)
		case viewRecords:
			return m.updateRecords(msg)
		default:
			return m.updateProjects(msg)
		}
	}
	return m, nil
}

/** newBrowserList builds one of the TUI's lists. The model owns the keys, so the
 * list gets none of its own that act: no quit keys (its keymap quits on q and
 * Esc, and Esc must do nothing outside a step), no filter of its own (it would be
 * a hidden text mode where q is text), and no help line (the legend bar is the
 * screen's help).
 *
 * Parameters:
 *   items ([]list.Item) — the rows
 *   title (string) — the list title, "" for none
 *   w, h (int) — the size
 *
 * Returns:
 *   list.Model — a list with only navigation keys
 *
 * Example:
 *   l := newBrowserList(items, "Records — alpha", 80, 23)
 */
func newBrowserList(items []list.Item, title string, w, h int) list.Model {
	l := list.New(items, list.NewDefaultDelegate(), w, h)
	l.Title = title
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	// Empty bindings, not disabled ones: the list re-enables its own quit keys
	// whenever its filter or key state changes, but a binding with no keys never
	// matches.
	l.KeyMap.Quit = bkey.NewBinding()
	l.KeyMap.ForceQuit = bkey.NewBinding()
	return l
}

// listHeight is the window height less the legend line.
func (m *tuiModel) listHeight() int {
	if m.height <= 1 {
		return 0
	}
	return m.height - 1
}

// capturingText reports whether the TUI is taking text, so a key handler treats
// q, j, k and the rest as characters (DR-0064, amendment). Today that is the
// search prompt; filters, forms and the command line join it later.
func (m *tuiModel) capturingText() bool { return m.searching }

// updateError handles a key while an error is on screen: q or Enter dismisses it
// and nothing else happens. Ctrl-C is handled before this is reached.
func (m *tuiModel) updateError(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEnter || (msg.Type == tea.KeyRunes && string(msg.Runes) == "q") {
		m.err = nil
	}
	return m, nil
}

// updateProjects: the top screen. q quits here (there is nowhere further back).
func (m *tuiModel) updateProjects(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		return m, nil
	case "/":
		m.startSearch()
		return m, nil
	case "enter":
		if item, ok := m.projectList.SelectedItem().(projectItem); ok {
			p := item.p
			m.selectedProject = &p
			if err := m.loadObservations(); err != nil {
				m.setErr(err)
				return m, nil
			}
			m.setState(viewObservations)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.projectList, cmd = m.projectList.Update(msg)
	return m, cmd
}

// goTo loads and shows one of a project's lists, or shows the error.
func (m *tuiModel) goTo(state viewState, load func() error) (tea.Model, tea.Cmd) {
	if load != nil {
		if err := load(); err != nil {
			m.setErr(err)
			return m, nil
		}
	}
	m.setState(state)
	return m, nil
}

// updateObservations: a project's observations. q goes back to the projects; c and
// r switch to its concepts and records.
func (m *tuiModel) updateObservations(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m.goTo(viewProjects, nil)
	case "esc":
		return m, nil
	case "/":
		m.startSearch()
		return m, nil
	case "c":
		return m.goTo(viewConcepts, m.loadConcepts)
	case "r":
		return m.goTo(viewRecords, m.loadRecords)
	}
	var cmd tea.Cmd
	m.observationList, cmd = m.observationList.Update(msg)
	return m, cmd
}

func (m *tuiModel) updateConcepts(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m.goTo(viewProjects, nil)
	case "esc":
		return m, nil
	case "o":
		return m.goTo(viewObservations, nil)
	case "r":
		return m.goTo(viewRecords, m.loadRecords)
	case "/":
		m.startSearch()
		return m, nil
	}
	var cmd tea.Cmd
	m.conceptList, cmd = m.conceptList.Update(msg)
	return m, cmd
}

// updateRecords handles the records view. No key here writes yet: the first
// write, set-status from a selected record, arrives with v0.0.19 T6.
func (m *tuiModel) updateRecords(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m.goTo(viewProjects, nil)
	case "esc":
		return m, nil
	case "o":
		return m.goTo(viewObservations, nil)
	case "c":
		return m.goTo(viewConcepts, m.loadConcepts)
	case "/":
		m.startSearch()
		return m, nil
	}
	var cmd tea.Cmd
	m.recordList, cmd = m.recordList.Update(msg)
	return m, cmd
}

// updateSearchResults: q returns to the screen the search began on.
func (m *tuiModel) updateSearchResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m.goTo(m.searchFrom, nil)
	case "esc":
		return m, nil
	case "/":
		m.startSearch()
		return m, nil
	}
	var cmd tea.Cmd
	m.searchList, cmd = m.searchList.Update(msg)
	return m, cmd
}

// updateSearching handles a key in the search prompt. Everything printable is
// text; Esc leaves the prompt and Enter runs the search.
func (m *tuiModel) updateSearching(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.searchInput.Blur()
		return m, nil
	case "enter":
		term := m.searchInput.Value()
		m.searching = false
		m.searchInput.Blur()
		if err := m.runSearch(term); err != nil {
			m.setErr(err)
			return m, nil
		}
		m.setState(viewSearch)
		return m, nil
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m *tuiModel) startSearch() {
	// A search started from its own results keeps the way back to where the first
	// one began.
	if m.state != viewSearch {
		m.searchFrom = m.state
	}
	m.searching = true
	m.searchInput.SetValue("")
	m.searchInput.Focus()
}

func (m *tuiModel) loadObservations() error {
	pid := m.selectedProject.ID
	obs, err := logKBCall(m.dl, "Observations", map[string]any{"project_id": pid}, func() ([]knowledge.Observation, error) {
		return m.kb.Observations(pid)
	})
	if err != nil {
		return err
	}
	items := make([]list.Item, len(obs))
	for i, o := range obs {
		items[i] = observationItem{o}
	}
	l := newBrowserList(items, fmt.Sprintf("Observations — %s", m.selectedProject.Name), m.width, m.listHeight())
	m.observationList = l
	return nil
}

// loadRecords fills the records list for the selected project, newest first.
// RecordsByProject returns them oldest first, sorted by date then id, so the
// slice is reversed rather than re-sorted — the ordering rule lives in one
// place, and ids are identity rather than chronology, so re-sorting here on
// id alone would be wrong.
func (m *tuiModel) loadRecords() error {
	pid := m.selectedProject.ID
	records, err := logKBCall(m.dl, "RecordsByProject", map[string]any{"project_id": pid}, func() ([]knowledge.Record, error) {
		return m.kb.RecordsByProject(pid)
	})
	if err != nil {
		return err
	}
	items := make([]list.Item, len(records))
	for i, r := range records {
		items[len(records)-1-i] = recordItem{r}
	}
	l := newBrowserList(items, fmt.Sprintf("Records — %s", m.selectedProject.Name), m.width, m.listHeight())
	m.recordList = l
	return nil
}

func (m *tuiModel) loadConcepts() error {
	pid := m.selectedProject.ID
	concepts, err := logKBCall(m.dl, "ProjectConcepts", map[string]any{"project_id": pid}, func() ([]knowledge.Concept, error) {
		return m.kb.ProjectConcepts(pid)
	})
	if err != nil {
		return err
	}
	items := make([]list.Item, len(concepts))
	for i, c := range concepts {
		items[i] = conceptItem{c}
	}
	l := newBrowserList(items, fmt.Sprintf("Concepts — %s", m.selectedProject.Name), m.width, m.listHeight())
	m.conceptList = l
	return nil
}

func (m *tuiModel) runSearch(term string) error {
	results, err := logKBCall(m.dl, "Search", map[string]any{"term": term}, func() ([]knowledge.KBSearchResult, error) {
		return m.kb.Search(term)
	})
	if err != nil {
		return err
	}
	items := make([]list.Item, len(results))
	for i, r := range results {
		items[i] = searchResultItem{r}
	}
	l := newBrowserList(items, fmt.Sprintf("Search: %s", term), m.width, m.listHeight())
	m.searchList = l
	return nil
}

// legend is the line of keys that apply on the current screen. It changes with
// the mode: while typing, q is text and is not offered.
func (m *tuiModel) legend() string {
	switch {
	case m.err != nil:
		return "q dismiss   Ctrl-C quit"
	case m.capturingText():
		return "Esc cancel   Enter search   Ctrl-C quit"
	}
	move := "↑/↓ j/k move"
	switch m.state {
	case viewObservations:
		return move + "   c concepts   r records   / search   q back"
	case viewConcepts:
		return move + "   o observations   r records   / search   q back"
	case viewRecords:
		return move + "   o observations   c concepts   / search   q back"
	case viewSearch:
		return move + "   / search   q back"
	}
	return move + "   Enter open   / search   q quit"
}

func (m *tuiModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("error: %v\n\n%s\n", m.err, m.legend())
	}
	if m.searching {
		return fmt.Sprintf("Search: %s\n\n%s\n", m.searchInput.View(), m.legend())
	}
	var body string
	switch m.state {
	case viewObservations:
		body = m.observationList.View()
	case viewConcepts:
		body = m.conceptList.View()
	case viewSearch:
		body = m.searchList.View()
	case viewRecords:
		body = m.recordList.View()
	default:
		body = m.projectList.View()
	}
	return body + "\n" + m.legend()
}
