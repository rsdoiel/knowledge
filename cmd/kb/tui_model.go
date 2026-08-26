package main

import (
	"fmt"

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
	projectList := list.New(items, list.NewDefaultDelegate(), 0, 0)
	projectList.Title = "Projects"

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
		observationList: list.New(nil, list.NewDefaultDelegate(), 0, 0),
		conceptList:     list.New(nil, list.NewDefaultDelegate(), 0, 0),
		recordList:      list.New(nil, list.NewDefaultDelegate(), 0, 0),
		searchList:      list.New(nil, list.NewDefaultDelegate(), 0, 0),
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
		m.projectList.SetSize(msg.Width, msg.Height)
		m.observationList.SetSize(msg.Width, msg.Height)
		m.conceptList.SetSize(msg.Width, msg.Height)
		m.searchList.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if m.searching {
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

func (m *tuiModel) updateProjects(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
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

func (m *tuiModel) updateObservations(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.setState(viewProjects)
		return m, nil
	case "/":
		m.startSearch()
		return m, nil
	case "c":
		if err := m.loadConcepts(); err != nil {
			m.setErr(err)
			return m, nil
		}
		m.setState(viewConcepts)
		return m, nil
	case "r":
		if err := m.loadRecords(); err != nil {
			m.setErr(err)
			return m, nil
		}
		m.setState(viewRecords)
		return m, nil
	}
	var cmd tea.Cmd
	m.observationList, cmd = m.observationList.Update(msg)
	return m, cmd
}

func (m *tuiModel) updateConcepts(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.setState(viewProjects)
		return m, nil
	case "o":
		m.setState(viewObservations)
		return m, nil
	case "r":
		if err := m.loadRecords(); err != nil {
			m.setErr(err)
			return m, nil
		}
		m.setState(viewRecords)
		return m, nil
	case "/":
		m.startSearch()
		return m, nil
	}
	var cmd tea.Cmd
	m.conceptList, cmd = m.conceptList.Update(msg)
	return m, cmd
}

// updateRecords handles the records view. Read-only: no key here writes,
// consistent with the TUI's existing scope. record new, set-status and
// supersede are CLI verbs.
func (m *tuiModel) updateRecords(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.setState(viewProjects)
		return m, nil
	case "o":
		m.setState(viewObservations)
		return m, nil
	case "c":
		if err := m.loadConcepts(); err != nil {
			m.setErr(err)
			return m, nil
		}
		m.setState(viewConcepts)
		return m, nil
	case "/":
		m.startSearch()
		return m, nil
	}
	var cmd tea.Cmd
	m.recordList, cmd = m.recordList.Update(msg)
	return m, cmd
}

func (m *tuiModel) updateSearchResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.setState(viewProjects)
		return m, nil
	case "/":
		m.startSearch()
		return m, nil
	}
	var cmd tea.Cmd
	m.searchList, cmd = m.searchList.Update(msg)
	return m, cmd
}

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
	l := list.New(items, list.NewDefaultDelegate(), m.width, m.height)
	l.Title = fmt.Sprintf("Observations — %s", m.selectedProject.Name)
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
	l := list.New(items, list.NewDefaultDelegate(), m.width, m.height)
	l.Title = fmt.Sprintf("Records — %s", m.selectedProject.Name)
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
	l := list.New(items, list.NewDefaultDelegate(), m.width, m.height)
	l.Title = fmt.Sprintf("Concepts — %s", m.selectedProject.Name)
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
	l := list.New(items, list.NewDefaultDelegate(), m.width, m.height)
	l.Title = fmt.Sprintf("Search: %s", term)
	m.searchList = l
	return nil
}

func (m *tuiModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("error: %v\n\npress q to quit\n", m.err)
	}
	if m.searching {
		return fmt.Sprintf("Search: %s\n\n(enter to search, esc to cancel)\n", m.searchInput.View())
	}
	switch m.state {
	case viewObservations:
		return m.observationList.View()
	case viewConcepts:
		return m.conceptList.View()
	case viewSearch:
		return m.searchList.View()
	case viewRecords:
		return m.recordList.View()
	default:
		return m.projectList.View()
	}
}
