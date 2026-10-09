package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	bkey "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	viewMenu        // the top menu
	viewGroup       // one group's menu (m.group)
	viewRecordScope // records in a scope, and the pending ones
)

// tuiStart says where the TUI opens: the top menu by default, or deep in the
// tree for a deep link.
type tuiStart struct {
	state viewState
	group string
}

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
	group           string    // the group whose menu is shown, in viewGroup
	menuCursor      int       // the row in the top menu
	groupCursor     int       // the row in a group menu
	notice          string    // lines under the screen: a dimmed choice's explanation, or what just happened
	workspaceDir    string    // the workspace directory, for the header
	dbRel           string    // the database path inside it
	recordCount     int
	scopeList       list.Model // the Records screens: records in a scope
	scopeStatus     string     // "" to browse, "proposed" for the pending ones
	scopeAll        bool       // widened to every scope with a
	scopeLabel      string     // the scope's name, for the title

	selectedProject *knowledge.Project
	err             error
	width, height   int
}

// newTUIModel opens the TUI at the top menu.
func newTUIModel(kb *knowledge.KnowledgeBase, dl *DebugLog) (*tuiModel, error) {
	return newTUIModelAt(kb, dl, tuiStart{state: viewMenu})
}

// newTUIModelAt opens the TUI at a place in the tree (a deep link).
func newTUIModelAt(kb *knowledge.KnowledgeBase, dl *DebugLog, start tuiStart) (*tuiModel, error) {
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

	m := &tuiModel{
		kb:    kb,
		dl:    dl,
		state: start.state,
		group: start.group,

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
		scopeList:       newBrowserList(nil, "", 0, 0),
		searchInput:     ti,
	}
	m.workspaceDir, m.dbRel = workspaceLabel(kb.Path())
	if recs, err := kb.ListRecords(knowledge.RecordFilter{}); err == nil {
		m.recordCount = len(recs)
	}
	return m, nil
}

/** workspaceLabel names a workspace for the menu header: its directory, with the
 * home directory shown as ~, and the database's path inside it.
 *
 * Parameters:
 *   dbPath (string) — the database file, normally <root>/agents/knowledge.db
 *
 * Returns:
 *   dir (string) — the workspace directory, ~-abbreviated
 *   db (string) — the database path relative to it
 *
 * Example:
 *   workspaceLabel("/home/me/Lab/agents/knowledge.db") // "~/Lab", "agents/knowledge.db"
 */
func workspaceLabel(dbPath string) (dir, db string) {
	root := filepath.Dir(filepath.Dir(dbPath))
	db = filepath.Join(filepath.Base(filepath.Dir(dbPath)), filepath.Base(dbPath))
	dir = root
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(home, root); err == nil && !strings.HasPrefix(rel, "..") {
			dir = filepath.Join("~", rel)
			if rel == "." {
				dir = "~"
			}
		}
	}
	return dir, db
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
	viewMenu:         "viewMenu",
	viewGroup:        "viewGroup",
	viewRecordScope:  "viewRecordScope",
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
		for _, l := range []*list.Model{&m.projectList, &m.observationList, &m.conceptList, &m.recordList, &m.searchList, &m.scopeList} {
			l.SetSize(m.cols()-4, m.listHeight())
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
		// A notice lasts until the next key.
		m.notice = ""
		// While text is being typed every printable key is text, so this is
		// asked before any letter is bound (DR-0064, amendment).
		if m.capturingText() {
			return m.updateSearching(msg)
		}
		switch m.state {
		case viewMenu:
			return m.updateMenu(msg)
		case viewGroup:
			return m.updateGroup(msg)
		case viewObservations:
			return m.updateObservations(msg)
		case viewConcepts:
			return m.updateConcepts(msg)
		case viewSearch:
			return m.updateSearchResults(msg)
		case viewRecords:
			return m.updateRecords(msg)
		case viewRecordScope:
			return m.updateRecordScope(msg)
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
	l.SetShowTitle(false) // the frame shows the title
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	// Empty bindings, not disabled ones: the list re-enables its own quit keys
	// whenever its filter or key state changes, but a binding with no keys never
	// matches.
	l.KeyMap.Quit = bkey.NewBinding()
	l.KeyMap.ForceQuit = bkey.NewBinding()
	return l
}

// cols is the window width, or 80 before the first resize message.
func (m *tuiModel) cols() int {
	if m.width < 20 {
		return 80
	}
	return m.width
}

// bodyHeight is the number of lines inside the frame above the divider: the
// window less the top border, the divider, the legend and the bottom border.
func (m *tuiModel) bodyHeight() int {
	if m.height < 6 {
		return 0
	}
	return m.height - 4
}

// listHeight is the height given to a list: the body of the frame.
func (m *tuiModel) listHeight() int { return m.bodyHeight() }

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

// openGroup shows one group's menu.
func (m *tuiModel) openGroup(group string) (tea.Model, tea.Cmd) {
	m.group = group
	m.groupCursor = 0
	m.setState(viewGroup)
	return m, nil
}

// moveCursor applies j, k and the arrows to a cursor over n rows, stopping at the ends.
func moveCursor(cur, n int, msg tea.KeyMsg) int {
	switch msg.String() {
	case "j", "down":
		if cur < n-1 {
			cur++
		}
	case "k", "up":
		if cur > 0 {
			cur--
		}
	}
	return cur
}

// explainUnbuilt is what choosing a row that is not built says: that it is not in
// the TUI yet, when it is, and the command that does the same.
func (m *tuiModel) explainUnbuilt(it menuItem) {
	m.notice = fmt.Sprintf("%s is not in the TUI yet (%s).\nFrom the command line: %s", it.Label, it.Since, it.Equivalent)
}

// updateMenu handles a key on the top menu. q quits: there is nowhere further back.
func (m *tuiModel) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := topMenu()
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		return m, nil
	case "/":
		m.startSearch()
		return m, nil
	case "j", "down", "k", "up":
		m.menuCursor = moveCursor(m.menuCursor, len(items), msg)
		return m, nil
	case "enter":
		it := items[m.menuCursor]
		switch {
		case it.Group:
			return m.openGroup(it.Verb)
		case it.Verb == "search":
			m.startSearch()
		case !it.Built:
			m.explainUnbuilt(it)
		}
	}
	return m, nil
}

// updateGroup handles a key on a group's menu. q goes back to the top menu.
func (m *tuiModel) updateGroup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := groupMenu(m.group)
	switch msg.String() {
	case "q":
		m.setState(viewMenu)
		return m, nil
	case "esc":
		return m, nil
	case "/":
		m.startSearch()
		return m, nil
	case "j", "down", "k", "up":
		m.groupCursor = moveCursor(m.groupCursor, len(items), msg)
		return m, nil
	case "a":
		if m.group == "record" {
			m.scopeAll = !m.scopeAll
		}
		return m, nil
	case "enter":
		if m.groupCursor >= len(items) {
			return m, nil
		}
		it := items[m.groupCursor]
		if !it.Built {
			m.explainUnbuilt(it)
			return m, nil
		}
		return m.runLeaf(it)
	}
	return m, nil
}

/** scopedRecord is a record in the Records screens' list: one line with its
 * qualified reference, its status and its title.
 */
type scopedRecordItem struct{ e recordListEntry }

func (i scopedRecordItem) Title() string {
	return fmt.Sprintf("%-20s %-11s %s", i.e.Ref, i.e.Status, i.e.Title)
}
func (i scopedRecordItem) Description() string { return "" }
func (i scopedRecordItem) FilterValue() string { return i.e.Ref + " " + i.e.Title }

// newCompactList is a browser list with one line a row and no description.
func newCompactList(items []list.Item, title string, w, h int) list.Model {
	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetHeight(1)
	d.SetSpacing(0)
	l := newBrowserList(items, title, w, h)
	l.SetDelegate(d)
	return l
}

// recordScopeName is the scope's name for the title and the Records menu: the
// project the working directory (or KB_PROJECT) names, else every scope. It
// is chosen by the code the CLI uses.
func (m *tuiModel) recordScopeName() string {
	if m.scopeAll {
		return "all scopes"
	}
	if name, err := inferredProject(m.kb, recordFlags{}); err == nil && name != "" {
		return name
	}
	return "all scopes"
}

// scopeKey is the legend text for the key that widens or narrows the scope.
func (m *tuiModel) scopeKey() string {
	if m.scopeAll {
		return "a this scope"
	}
	return "a all scopes"
}

// loadRecordScope fills the Records list for the current scope and status. Browse
// is newest first; Pending is oldest first, as the CLI lists them.
func (m *tuiModel) loadRecordScope() error {
	f := recordFlags{all: m.scopeAll}
	scopes, err := recordScopes(m.kb, f)
	if err != nil {
		return err
	}
	records, err := listRecordsIn(m.kb, knowledge.RecordFilter{Status: m.scopeStatus}, scopes)
	if err != nil {
		return err
	}
	names := projectNames(m.kb)
	items := make([]list.Item, len(records))
	for i, r := range records {
		e := toEntry(r, names)
		if m.scopeStatus == "" {
			items[len(records)-1-i] = scopedRecordItem{e}
		} else {
			items[i] = scopedRecordItem{e}
		}
	}
	name, noun := "Records", "record"
	if m.scopeStatus == "proposed" {
		name, noun = "Pending", "pending record"
	}
	l := newCompactList(items, fmt.Sprintf("%s — %s  (%d)", name, m.recordScopeName(), len(items)), m.cols()-4, m.listHeight())
	l.SetStatusBarItemName(noun, noun+"s")
	m.scopeList = l
	return nil
}

// openRecordScope opens the Records screen: Browse for status "", Pending for "proposed".
func (m *tuiModel) openRecordScope(status string) (tea.Model, tea.Cmd) {
	m.scopeStatus = status
	return m.goTo(viewRecordScope, m.loadRecordScope)
}

// updateRecordScope handles a key on the Records screen. q goes back to the
// Records menu; a widens the scope to every scope and back.
func (m *tuiModel) updateRecordScope(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m.openGroup("record")
	case "esc":
		return m, nil
	case "/":
		m.startSearch()
		return m, nil
	case "a":
		m.scopeAll = !m.scopeAll
		if err := m.loadRecordScope(); err != nil {
			m.setErr(err)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.scopeList, cmd = m.scopeList.Update(msg)
	return m, cmd
}

// runLeaf runs a built group leaf: it opens the screen that does what the verb does.
func (m *tuiModel) runLeaf(it menuItem) (tea.Model, tea.Cmd) {
	switch it.Verb + " " + it.Sub {
	case "project list":
		m.setState(viewProjects)
	case "record list":
		return m.openRecordScope("")
	case "record pending":
		return m.openRecordScope("proposed")
	default:
		m.notice = fmt.Sprintf("%s has no screen yet.\nFrom the command line: %s", it.Label, it.Equivalent)
	}
	return m, nil
}

// updateProjects: the project list. q goes back to the Projects menu.
func (m *tuiModel) updateProjects(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m.openGroup("project")
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
	l := newBrowserList(items, fmt.Sprintf("Observations — %s", m.selectedProject.Name), m.cols()-4, m.listHeight())
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
	l := newBrowserList(items, fmt.Sprintf("Records — %s", m.selectedProject.Name), m.cols()-4, m.listHeight())
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
	l := newBrowserList(items, fmt.Sprintf("Concepts — %s", m.selectedProject.Name), m.cols()-4, m.listHeight())
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
	l := newBrowserList(items, fmt.Sprintf("Search: %s", term), m.cols()-4, m.listHeight())
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
	case viewMenu:
		return move + "   Enter open   / search   q quit"
	case viewGroup:
		if m.group == "record" {
			return move + "   Enter open   " + m.scopeKey() + "   / search   q back"
		}
		return move + "   Enter open   / search   q back"
	case viewRecordScope:
		return move + "   " + m.scopeKey() + "   / search   q back"
	case viewObservations:
		return move + "   c concepts   r records   / search   q back"
	case viewConcepts:
		return move + "   o observations   r records   / search   q back"
	case viewRecords:
		return move + "   o observations   c concepts   / search   q back"
	case viewSearch:
		return move + "   / search   q back"
	}
	return move + "   Enter open   / search   q back"
}

// menuRows renders a menu's rows: the cursor, the label, what it does, and for a
// row that is not built the release that brings it, in a faint style.
func (m *tuiModel) menuRows(items []menuItem, cursor int) []string {
	faint := lipgloss.NewStyle().Faint(true)
	rows := make([]string, len(items))
	for i, it := range items {
		pointer := "  "
		if i == cursor {
			pointer = "> "
		}
		row := fmt.Sprintf("%s%-16s %s", pointer, it.Label, it.Desc)
		if !it.Built {
			row = faint.Render(fmt.Sprintf("%s  (%s)", row, it.Since))
		}
		rows[i] = row
	}
	return rows
}

// listLines is a list's rendered lines.
func viewLines(l list.Model) []string {
	return strings.Split(strings.TrimRight(l.View(), "\n"), "\n")
}

// screen returns the title and body lines of the current screen.
func (m *tuiModel) screen() (string, []string) {
	if m.err != nil {
		return "Error", []string{fmt.Sprintf("error: %v", m.err)}
	}
	if m.searching {
		return "Search", []string{"Search: " + m.searchInput.View()}
	}
	switch m.state {
	case viewMenu:
		return "kb — knowledge", append([]string{m.header(), ""}, m.menuRows(topMenu(), m.menuCursor)...)
	case viewGroup:
		rows := m.menuRows(groupMenu(m.group), m.groupCursor)
		if m.group == "record" {
			hint := "a widens it"
			if m.scopeAll {
				hint = "a narrows it"
			}
			rows = append(rows, "", fmt.Sprintf("scope: %s   (%s)", m.recordScopeName(), hint))
		}
		return groupTitle(m.group), rows
	case viewRecordScope:
		return m.scopeList.Title, viewLines(m.scopeList)
	case viewObservations:
		return m.observationList.Title, viewLines(m.observationList)
	case viewConcepts:
		return m.conceptList.Title, viewLines(m.conceptList)
	case viewSearch:
		return m.searchList.Title, viewLines(m.searchList)
	case viewRecords:
		return m.recordList.Title, viewLines(m.recordList)
	}
	return "Projects", viewLines(m.projectList)
}

// header names the workspace on the top menu: its directory, its database and
// what it holds. A long directory is shortened from the left, never the counts,
// so the part that tells two workspaces apart (its end) and the numbers both show.
func (m *tuiModel) header() string {
	counts := fmt.Sprintf("%d records · %d projects", m.recordCount, len(m.projectList.Items()))
	room := m.cols() - 4 - lipgloss.Width(m.dbRel) - lipgloss.Width(counts) - 6
	dir := m.workspaceDir
	if r := []rune(dir); room > 3 && len(r) > room {
		dir = "…" + string(r[len(r)-(room-1):])
	}
	return fmt.Sprintf("%s   %s   %s", dir, m.dbRel, counts)
}

// groupTitle is a group menu's title: its top-menu label.
func groupTitle(group string) string {
	for _, it := range topMenu() {
		if it.Verb == group {
			return it.Label
		}
	}
	return group
}

/** frame draws a screen in a box: a title in the top border, the body, an
 * optional notice under it, a divider, the legend, and the bottom border. Every
 * line is padded or cut to the window's width and the body is padded to fill the
 * window, so the legend stays at the bottom.
 *
 * Parameters:
 *   title (string) — the text in the top border
 *   body ([]string) — the lines of the screen
 *   legend (string) — the line of keys under the divider
 *
 * Returns:
 *   string — the screen, one line per row, ending in a newline
 *
 * Example:
 *   fmt.Print(m.frame("Records", lines, m.legend()))
 */
func (m *tuiModel) frame(title string, body []string, legend string) string {
	w := m.cols()
	inner := w - 4
	fit := func(s string) string {
		s = lipgloss.NewStyle().Inline(true).MaxWidth(inner).Render(s)
		if pad := inner - lipgloss.Width(s); pad > 0 {
			s += strings.Repeat(" ", pad)
		}
		return "│ " + s + " │"
	}
	var out []string
	top := "┌ " + title + " "
	if fill := w - 1 - lipgloss.Width(top); fill > 0 {
		top += strings.Repeat("─", fill)
	}
	out = append(out, lipgloss.NewStyle().Inline(true).MaxWidth(w-1).Render(top)+"┐")

	var notice []string
	if m.notice != "" {
		notice = append([]string{""}, strings.Split(m.notice, "\n")...)
	}
	room := m.bodyHeight()
	if room > 0 {
		avail := room - len(notice)
		if avail < 0 {
			avail = 0
		}
		if len(body) > avail {
			body = body[:avail]
		}
		for len(body) < avail {
			body = append(body, "")
		}
	}
	for _, l := range body {
		out = append(out, fit(l))
	}
	for _, l := range notice {
		out = append(out, fit(l))
	}
	out = append(out, "├"+strings.Repeat("─", w-2)+"┤", fit(legend), "└"+strings.Repeat("─", w-2)+"┘")
	// No trailing newline: it would make the view one line taller than the window,
	// and bubbletea would cut the top border to fit.
	return strings.Join(out, "\n")
}

func (m *tuiModel) View() string {
	title, body := m.screen()
	return m.frame(title, body, m.legend())
}
