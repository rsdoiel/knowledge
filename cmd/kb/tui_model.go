package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	bkey "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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
	viewReview      // choosing and confirming a status for the selected record
	viewText        // a record shown in the built-in viewer, when there is no pager
	viewRemove      // the gate for a removing verb: a plan and a typed confirmation
	viewChange      // a changing write: pick or type the new value, then confirm old to new
	viewForm        // an additive write: the fields one at a time, then confirm
	viewPlan        // a plan-then-apply write: the dry run is shown, then y applies it
	viewCommand     // the `:` command line
	viewReviewQueue // document sections waiting for a person
	viewSummary     // one section's summary unit: write it, then review it beside its source
	viewBrowse      // a Browse list of concepts, sources or documents: Enter shows the item
	viewUnlink      // the links of one observation, to pick one to cut
	viewPick        // a checklist of proposals to accept, with nothing selected
)

// tuiStart says where the TUI opens: the top menu by default, or deep in the
// tree for a deep link.
type tuiStart struct {
	state  viewState
	group  string
	status string // for the Records screen: "" to browse, "proposed" for pending
	all    bool   // for the Records screen: every scope
	term   string // for search results: the term, which is run at once
	ref    string // for the Records screen: the record the cursor starts on
	prompt string // for the search prompt: the scope it opens on, such as "harvey/records"
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

type searchResultItem struct {
	r     knowledge.KBSearchResult
	ref   string   // a record's qualified reference, from a scoped search
	spans []string // for a concept from a scoped search, the projects it is in
}

func (i searchResultItem) Title() string {
	label := i.r.Label
	if i.ref != "" {
		label = i.ref
	}
	return fmt.Sprintf("[%s] %s", i.r.Kind, label)
}
func (i searchResultItem) Description() string {
	if len(i.spans) > 0 {
		return i.r.Snippet + "   [" + strings.Join(i.spans, ", ") + "]"
	}
	return i.r.Snippet
}
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
	searchFrom      viewState    // where a search began, which q from its results returns to
	group           string       // the group whose menu is shown, in viewGroup
	menuCursor      int          // the row in the top menu
	groupCursor     int          // the row in a group menu
	quitting        bool         // set when the program has been told to quit
	review          *reviewModel // the status review in progress, in viewReview
	reviewRec       *knowledge.Record
	reviewRef       string
	reviewFrom      string
	reviewOptions   []string
	reviewRaw       []byte
	reviewBack      viewState // the screen the review returns to
	text            viewport.Model
	removal         *removalPlan // the delete the gate is asking about, in viewRemove
	removeInput     *typedConfirmModel
	removeBack      viewState
	change          *changeFlow // the changing write in progress, in viewChange
	form            *formFlow   // the additive form in progress, in viewForm
	plan            *planFlow   // the plan-then-apply write in progress, in viewPlan
	planView        viewport.Model
	command         *commandFlow              // the `:` prompt, in viewCommand
	queueList       list.Model                // the review queue
	browseList      list.Model                // a Browse list, in viewBrowse
	unlinkList      list.Model                // an observation's links, in viewUnlink
	unlinkObs       int64                     // the observation whose links it lists
	pick            *pickFlow                 // the checklist in progress, in viewPick
	browseKind      string                    // what it lists: concept, source or document
	filters         map[viewState]*listFilter // the filter of each list that has one in force
	unit            *summaryUnit              // the summary unit in progress, in viewSummary
	textPurpose     string
	textPlain       []string        // the viewer's lines without colour, for the find
	finding         bool            // a find term is being typed in the viewer
	findInput       textinput.Model // the viewer's find prompt
	findQuery       string
	findHits        []int  // the viewer lines that match findQuery
	findCur         int    // which of findHits the viewer is at
	notice          string // lines under the screen: a dimmed choice's explanation, or what just happened
	workspaceDir    string // the workspace directory, for the header
	dbRel           string // the database path inside it
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
		findInput:       newFindInput(),
	}
	m.workspaceDir, m.dbRel = workspaceLabel(kb.Path())
	if recs, err := kb.ListRecords(knowledge.RecordFilter{}); err == nil {
		m.recordCount = len(recs)
	}
	// A deep link opens in the middle of the tree.
	switch start.state {
	case viewRecordScope:
		m.scopeStatus, m.scopeAll = start.status, start.all
		if err := m.loadRecordScope(); err != nil {
			return nil, err
		}
		for i, it := range m.scopeList.Items() {
			if r, ok := it.(scopedRecordItem); ok && start.ref != "" && r.e.Ref == start.ref {
				m.scopeList.Select(i)
			}
		}
	case viewSearch:
		m.searchFrom = viewMenu
		if err := m.runSearch(start.term); err != nil {
			return nil, err
		}
	case viewMenu:
		if start.prompt != "" {
			m.searchFrom = viewMenu
			m.startSearchWith(start.prompt)
		}
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
	viewReview:       "viewReview",
	viewText:         "viewText",
	viewRemove:       "viewRemove",
	viewChange:       "viewChange",
	viewForm:         "viewForm",
	viewPlan:         "viewPlan",
	viewCommand:      "viewCommand",
	viewReviewQueue:  "viewReviewQueue",
	viewSummary:      "viewSummary",
	viewBrowse:       "viewBrowse",
	viewUnlink:       "viewUnlink",
	viewPick:         "viewPick",
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
		for _, l := range []*list.Model{&m.projectList, &m.searchList, &m.scopeList} {
			l.SetSize(m.cols()-4, m.listHeight())
		}
		if m.browseKind != "" { // the list exists once a Browse row has been opened
			m.browseList.SetSize(m.cols()-4, m.listHeight())
		}
		if m.unlinkObs != 0 {
			m.unlinkList.SetSize(m.cols()-4, m.listHeight())
		}
		for _, l := range []*list.Model{&m.observationList, &m.conceptList, &m.recordList} {
			l.SetSize(m.cols()-4, m.tabHeight())
		}
		return m, nil

	case pagerDoneMsg:
		return m.handlePagerDone(msg)

	case editorDoneMsg:
		return m.handleEditorDone(msg)

	case tea.KeyMsg:
		// Ctrl-C quits from everywhere, typing and errors included, and writes
		// nothing (DR-0064).
		if msg.Type == tea.KeyCtrlC {
			return m.quit()
		}
		if m.err != nil {
			return m.updateError(msg)
		}
		// Keys typed ahead, or pasted, can arrive as one message ("qq"). Each is a
		// key of its own and is taken in turn, stopping if one quits. While text is
		// being typed the run is text and goes to the prompt whole.
		if keys := keysOf(msg); len(keys) > 1 && !m.capturingText() {
			var last tea.Cmd
			for _, k := range keys {
				_, last = m.Update(k)
				if m.quitting {
					break
				}
			}
			return m, last
		}
		// A notice lasts until the next key.
		m.notice = ""
		if m.state == viewChange {
			return m.updateChange(msg)
		}
		if m.state == viewForm {
			return m.updateForm(msg)
		}
		if m.state == viewCommand {
			return m.updateCommand(msg)
		}
		if m.state == viewSummary {
			return m.updateSummary(msg)
		}
		// `/` filters the list that has focus, and while a filter is typed every
		// key is text, so this is asked before any letter is bound (DR-0072).
		if cmd, took := m.filterKey(msg); took {
			return m, cmd
		}
		if msg.String() == ":" && m.canOpenCommand() {
			return m.openCommand()
		}
		// While text is being typed every printable key is text, so this is
		// asked before any letter is bound (DR-0064, amendment).
		if m.capturingText() {
			if m.state == viewText {
				return m.updateText(msg)
			}
			if m.state == viewRemove && !m.searching {
				return m.updateRemove(msg)
			}
			return m.updateSearching(msg)
		}
		if msg.String() == "S" && m.canSearchFromHere() {
			m.startSearchWith(m.contextSearchScope())
			return m, nil
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
		case viewReview:
			return m.updateReview(msg)
		case viewText:
			return m.updateText(msg)
		case viewPlan:
			return m.updatePlan(msg)
		case viewReviewQueue:
			return m.updateReviewQueue(msg)
		case viewBrowse:
			return m.updateBrowse(msg)
		case viewUnlink:
			return m.updateUnlink(msg)
		case viewPick:
			return m.updatePick(msg)
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

// tabHeight is the height of a project tab's list: the body less the two lines
// the tab strip takes.
func (m *tuiModel) tabHeight() int {
	if h := m.bodyHeight() - 2; h > 0 {
		return h
	}
	return 0
}

// capturingText reports whether the TUI is taking text, so a key handler treats
// q, j, k and the rest as characters (DR-0064, amendment). Today that is the
// search prompt; filters, forms and the command line join it later.
func (m *tuiModel) capturingText() bool {
	if m.state == viewChange && m.change != nil {
		return m.change.edit != nil && m.change.phase == changePick
	}
	if m.state == viewForm && m.form != nil {
		return m.form.edit != nil
	}
	if m.state == viewSummary && m.unit != nil {
		return !m.unit.reviewing
	}
	if m.state == viewText {
		return m.finding
	}
	return m.searching || m.state == viewRemove
}

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

// quit ends the program and remembers that it did, so a run of keys in one
// message stops there.
func (m *tuiModel) quit() (tea.Model, tea.Cmd) {
	m.quitting = true
	return m, tea.Quit
}

// updateMenu handles a key on the top menu. q quits: there is nowhere further back.
func (m *tuiModel) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := topMenu()
	switch msg.String() {
	case "q":
		return m.quit()
	case "esc":
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
		case it.Verb == "index":
			return m.runDirect("index")
		case it.Verb == "ingest":
			return m.planIngest()
		case it.Verb == "check-db":
			return m.showRead([]string{"check-db"})
		case !it.Built:
			m.explainUnbuilt(it)
		}
	}
	return m, nil
}

// updateGroup handles a key on a group's menu. q goes back to the top menu.
func (m *tuiModel) updateGroup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.groupItems()
	switch msg.String() {
	case "q":
		m.setState(viewMenu)
		return m, nil
	case "esc":
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
	m.clearFilter(viewRecordScope)
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
	case "a":
		m.scopeAll = !m.scopeAll
		if err := m.loadRecordScope(); err != nil {
			m.setErr(err)
		}
		return m, nil
	case "enter":
		return m.showSelected("read")
	case "s":
		return m.showSelected("status")
	case "u":
		return m.supersedeSelected()
	case "n":
		return m.beginForm(m.newRecordForm(m.scopeForForm()))
	case "d":
		return m.removeSelected()
	}
	var cmd tea.Cmd
	m.scopeList, cmd = m.scopeList.Update(msg)
	return m, cmd
}

// runLeaf runs a built group leaf: it opens the screen that does what the verb does.
func (m *tuiModel) runLeaf(it menuItem) (tea.Model, tea.Cmd) {
	if it.Verb == "search" {
		m.startSearchWith(it.Sub)
		return m, nil
	}
	switch it.Verb + " " + it.Sub {
	case "project list":
		m.setState(viewProjects)
	case "record list":
		return m.openRecordScope("")
	case "record pending":
		return m.openRecordScope("proposed")
	case "record fmt":
		return m.runDirect("fmt")
	case "concept list", "document list", "source list":
		return m.openBrowse(it.Verb)
	case "source check-retractions":
		// It sends every registered DOI to a service and marks hits as retracted, so
		// it asks first (DR-0067: a write that reaches the network is not a read).
		args := []string{"source", "check-retractions"}
		m.beginPlan(&planFlow{
			title:      "source check-retractions",
			note:       "this command has not been run yet",
			text:       "It looks up every registered, non-retracted DOI source in the Retraction Watch service, which needs network access, and marks hits as retracted. It changes the database. y runs it.",
			equivalent: "kb source check-retractions",
			apply:      func() (string, error) { return m.runArgs(args) },
		}, m.state)
	case "concept recall":
		return m.beginForm(m.newRecallForm())
	case "concept suggest":
		return m.beginForm(m.newSuggestForm())
	case "observation list":
		// Observations belong to a project, so Browse is the project list, where
		// Enter opens a project and o its observations.
		m.setState(viewProjects)
		m.notice = "Observations belong to a project: press Enter on one, then o for its observations."
	case "document review list":
		m.scopeAll = false
		return m.goTo(viewReviewQueue, m.loadReviewQueue)
	case "record fuzzy-tag":
		return m.planRecordFuzzyTag()
	case "document frontmatter":
		return m.beginFrontmatter()
	case "document ingest":
		return m.planDocumentIngest()
	case "document tag":
		return m.planDocumentEdit("tag")
	case "document fuzzy-tag":
		return m.planDocumentEdit("fuzzy-tag")
	case "project add":
		return m.beginForm(m.newProjectForm())
	case "observation add":
		return m.beginForm(m.newObservationForm(""))
	case "concept add":
		return m.beginForm(m.newConceptForm())
	case "source add":
		return m.beginForm(m.newSourceForm())
	case "record new":
		return m.beginForm(m.newRecordForm(m.scopeForForm()))
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
	case "d":
		return m.removeSelected()
	case "n":
		return m.beginForm(m.newProjectForm())
	case "s":
		return m.changeSelected("status")
	case "e":
		return m.changeSelected("describe")
	case "r":
		return m.changeSelected("rename")
	case "enter":
		if item, ok := m.projectList.SelectedItem().(projectItem); ok {
			p := item.p
			m.selectedProject = &p
			// All three tabs are loaded at once, so the counts in the strip are
			// true before any of them is visited.
			for _, load := range []func() error{m.loadObservations, m.loadConcepts, m.loadRecords} {
				if err := load(); err != nil {
					m.setErr(err)
					return m, nil
				}
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
	case "enter":
		if it, ok := m.observationList.SelectedItem().(observationItem); ok {
			return m.showReadFrom([]string{"observation", "show", strconv.FormatInt(it.o.ID, 10)}, viewObservations)
		}
		return m, nil
	case "d":
		return m.removeSelected()
	case "u":
		return m.openUnlink()
	case "n":
		return m.beginForm(m.newObservationForm(m.projectName()))
	case "e":
		return m.changeSelected("edit")
	case "q":
		return m.goTo(viewProjects, nil)
	case "esc":
		return m, nil
	case "c":
		return m.goTo(viewConcepts, nil)
	case "r":
		return m.goTo(viewRecords, nil)
	}
	var cmd tea.Cmd
	m.observationList, cmd = m.observationList.Update(msg)
	return m, cmd
}

func (m *tuiModel) updateConcepts(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "d":
		return m.removeSelected()
	case "u":
		return m.unlinkConceptFromProject()
	case "n":
		return m.beginForm(m.newConceptForm())
	case "e":
		return m.changeSelected("edit")
	case "q":
		return m.goTo(viewProjects, nil)
	case "esc":
		return m, nil
	case "o":
		return m.goTo(viewObservations, nil)
	case "r":
		return m.goTo(viewRecords, nil)
	}
	var cmd tea.Cmd
	m.conceptList, cmd = m.conceptList.Update(msg)
	return m, cmd
}

// updateRecords handles a project's Records tab: Enter reads the selected record
// and s sets its status, as on the Records screen.
func (m *tuiModel) updateRecords(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.showSelected("read")
	case "s":
		return m.showSelected("status")
	case "u":
		return m.supersedeSelected()
	case "n":
		return m.beginForm(m.newRecordForm(m.projectName()))
	case "d":
		return m.removeSelected()
	case "q":
		return m.goTo(viewProjects, nil)
	case "esc":
		return m, nil
	case "o":
		return m.goTo(viewObservations, nil)
	case "c":
		return m.goTo(viewConcepts, nil)
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
	case "enter":
		if it, ok := m.searchList.SelectedItem().(searchResultItem); ok {
			return m.showSearchResult(it.r)
		}
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
		line := strings.TrimSpace(m.searchInput.Value())
		if hint, bad := m.searchLineProblem(line); bad {
			// The prompt stays open, with what is wrong, so the line can be corrected.
			m.notice = hint
			return m, nil
		}
		m.searching = false
		m.searchInput.Blur()
		if err := m.runSearchLine(line); err != nil {
			m.searching = true
			m.searchInput.Focus()
			m.notice = err.Error()
			return m, nil
		}
		m.setState(viewSearch)
		return m, nil
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

// searchLineProblem says what is wrong with a search line that cannot be run: a
// scope that does not resolve, a kind that does not exist, or no term.
func (m *tuiModel) searchLineProblem(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "type a term to search for, or a scope and a term: harvey/records exit codes", true
	}
	if fields[0] != "--" && looksLikeSearchScope(fields[0]) {
		if _, _, err := parseSearchScope(m.kb, fields[0]); err != nil {
			return err.Error(), true
		}
		if len(fields) == 1 {
			return "type a term after the scope: " + fields[0] + " exit codes", true
		}
	}
	return "", false
}

// canSearchFromHere says whether S starts a search on this screen: not while
// text is being typed (a filter, a form, a note), and not on a screen whose keys
// are a chooser or a confirmation.
func (m *tuiModel) canSearchFromHere() bool {
	if m.err != nil || m.searching || m.filterTyping() {
		return false
	}
	switch m.state {
	case viewText, viewReview, viewPlan, viewRemove, viewForm, viewChange, viewCommand, viewSummary, viewPick:
		return false
	}
	return true
}

// contextSearchScope is the scope S opens the prompt on: the project of a
// project's tabs, the kind of a Browse list, records on the Records screens, and
// nothing elsewhere. The prompt shows it, so it is never implicit.
func (m *tuiModel) contextSearchScope() string {
	switch m.state {
	case viewObservations, viewConcepts, viewRecords:
		if m.selectedProject != nil {
			return m.selectedProject.Name + "/"
		}
	case viewBrowse:
		switch m.browseKind {
		case "concept":
			return "/concepts"
		case "document":
			return "/documents"
		}
	case viewRecordScope:
		return "/records"
	}
	return ""
}

// groupItems is the rows of the current group menu. The Search menu has a row for
// the project the person is in, when there is one, first.
func (m *tuiModel) groupItems() []menuItem {
	items := groupMenu(m.group)
	if m.group == "search" && m.selectedProject != nil {
		name := m.selectedProject.Name
		row := menuItem{Verb: "search", Sub: name + "/", Label: "This project (" + name + ")…", Desc: "every kind, in " + name,
			Built: true, Equivalent: "kb search " + name + "/ TERM"}
		items = append([]menuItem{row}, items...)
	}
	return items
}

// startSearchWith opens the search prompt with a scope already typed, so the line
// is the command's first argument and the term follows it.
func (m *tuiModel) startSearchWith(scope string) {
	if m.state != viewSearch {
		m.searchFrom = m.state
	}
	m.searching = true
	text := ""
	if scope != "" && scope != "/" {
		text = scope + " "
	}
	m.searchInput.SetValue(text)
	m.searchInput.CursorEnd()
	m.searchInput.Focus()
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
	l := newBrowserList(items, fmt.Sprintf("Observations — %s", m.selectedProject.Name), m.cols()-4, m.tabHeight())
	m.observationList = l
	m.clearFilter(viewObservations)
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
	l := newBrowserList(items, fmt.Sprintf("Records — %s", m.selectedProject.Name), m.cols()-4, m.tabHeight())
	m.recordList = l
	m.clearFilter(viewRecords)
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
	l := newBrowserList(items, fmt.Sprintf("Concepts — %s", m.selectedProject.Name), m.cols()-4, m.tabHeight())
	m.conceptList = l
	m.clearFilter(viewConcepts)
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
		items[i] = searchResultItem{r: r}
	}
	l := newBrowserList(items, fmt.Sprintf("Search: %s", term), m.cols()-4, m.listHeight())
	m.searchList = l
	m.clearFilter(viewSearch)
	return nil
}

// runSearchLine runs what the search prompt holds: a scope and a term as `kb
// search` takes them, or just a term. The results list is titled with the
// command that does the same.
func (m *tuiModel) runSearchLine(line string) error {
	fields := strings.Fields(line)
	if len(fields) == 0 || fields[0] == "--" || !looksLikeSearchScope(fields[0]) {
		term := strings.Join(fields, " ")
		if len(fields) > 0 && fields[0] == "--" {
			term = strings.Join(fields[1:], " ")
		}
		return m.runSearch(term)
	}
	scope, name, err := parseSearchScope(m.kb, fields[0])
	if err != nil {
		return err
	}
	term := strings.Join(fields[1:], " ")
	res, err := logKBCall(m.dl, "SearchScoped", map[string]any{"term": term, "scope": name}, func() (*knowledge.ScopedSearchResult, error) {
		return m.kb.SearchScoped(term, scope)
	})
	if err != nil {
		return err
	}
	names := projectNames(m.kb)
	items := make([]list.Item, len(res.Results))
	for i, r := range res.Results {
		it := searchResultItem{r: r}
		if r.SourceType == "record" {
			if rec, rerr := m.kb.RecordByID(r.SourceID); rerr == nil && rec != nil {
				it.ref = refOf(*rec, names).String()
			}
		}
		if r.SourceType == "concept" {
			it.spans = res.Spans[r.SourceID]
		}
		if r.SourceType == "source" {
			it.spans = res.SourceSpans[r.SourceID]
		}
		items[i] = it
	}
	noun := "matches"
	if res.Total == 1 {
		noun = "match"
	}
	title := fmt.Sprintf("kb search %s %s — %d %s", fields[0], term, res.Total, noun)
	if res.Total > len(items) {
		title += fmt.Sprintf(", showing %d", len(items))
	}
	m.searchList = newBrowserList(items, title, m.cols()-4, m.listHeight())
	m.clearFilter(viewSearch)
	return nil
}

// legend is the line of keys that apply on the current screen. It changes with
// the mode: while typing, q is text and is not offered.
func (m *tuiModel) legend() string {
	return fitLegend(m.filterLegend(m.legendFull(), m.cols()-4), m.cols()-4)
}

/** fitLegend shortens a legend that is wider than the room it has, so the frame
 * does not cut the last key off: it tries two spaces between the keys, then drops
 * the movement hint, which every screen shares and a person already knows.
 *
 * Parameters:
 *   legend (string) — the keys, separated by three spaces
 *   width (int) — the room inside the frame
 *
 * Returns:
 *   string — a legend no wider than width when that is possible
 *
 * Example:
 *   fitLegend("↑/↓ j/k move   Enter open   q back", 20) // "Enter open  q back"
 */
func fitLegend(legend string, width int) string {
	tight := strings.ReplaceAll(legend, "   ", "  ")
	noMove := strings.TrimPrefix(tight, "↑/↓ j/k move  ")
	short := noMove
	for _, r := range [][2]string{{"e describe", "e desc"}, {"u supersede", "u super"}} {
		short = strings.Replace(short, r[0], r[1], 1)
	}
	// In order of how little is lost: the spacing, then the movement hint (every
	// screen shares it and a person already knows it), then long words shortened.
	shorter := short
	for _, r := range [][2]string{{"d delete", "d del"}, {"r rename", "r ren"}, {"s status", "s stat"}} {
		shorter = strings.Replace(shorter, r[0], r[1], 1)
	}
	for _, candidate := range []string{legend, tight, noMove, short, shorter} {
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
	}
	return shorter
}

// legendFull is the legend before it is fitted to the window.
func (m *tuiModel) legendFull() string {
	switch {
	case m.err != nil:
		return "q dismiss   Ctrl-C quit"
	case m.searching:
		return "Esc cancel   Enter search   Ctrl-C quit"
	}
	move := "↑/↓ j/k move"
	switch m.state {
	case viewMenu:
		return move + "   Enter open   S search   : command   q quit"
	case viewGroup:
		if m.group == "record" {
			return move + "   Enter open   " + m.scopeKey() + "   S search   : command   q back"
		}
		return move + "   Enter open   S search   : command   q back"
	case viewRecordScope:
		return move + "   Enter read   s status   u supersede   d delete   " + m.scopeKey() + "   S search   q back"
	case viewRemove:
		return "Esc cancel   Enter delete (only when the name matches)   Ctrl-C quit"
	case viewChange:
		return m.changeLegend()
	case viewForm:
		return m.formLegend()
	case viewCommand:
		return "Enter run   Esc cancel   Ctrl-C quit"
	case viewSummary:
		return m.summaryLegend()
	case viewBrowse:
		return move + "   Enter show   d delete   S search   q back"
	case viewUnlink:
		return move + "   Enter choose   S search   q back"
	case viewPick:
		return m.pickLegend()
	case viewReviewQueue:
		return move + "   Enter summarise   " + m.scopeKey() + "   S search   : command   q back"
	case viewPlan:
		return "↑/↓ j/k scroll   space page   y apply   n q Esc cancel   Ctrl-C quit"
	case viewReview:
		return m.reviewLegend()
	case viewText:
		return m.textLegend()
	case viewRecords:
		return move + "   Enter read   s status   u supersede   d delete   o c r tabs   S search   q back"
	case viewObservations, viewConcepts:
		if m.state == viewObservations {
			return move + "   Enter show   n new   e edit   u unlink   d delete   o c r tabs   S search   q back"
		}
		return move + "   n new   e rename   u unlink   d delete   o c r tabs   S search   q back"
	case viewSearch:
		return move + "   Enter show   S search   q back"
	}
	return move + "   Enter open   n new   s status   e describe   r rename   d delete   S search   q back"
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

// screen returns the title and body lines of the current screen, with a kept
// list filter named in the title.
func (m *tuiModel) screen() (string, []string) {
	title, lines := m.screenBody()
	return m.filterTitle(title), lines
}

// screenBody is screen before the filter is added to the title.
func (m *tuiModel) screenBody() (string, []string) {
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
		rows := m.menuRows(m.groupItems(), m.groupCursor)
		if m.group == "record" {
			hint := "a widens it"
			if m.scopeAll {
				hint = "a narrows it"
			}
			rows = append(rows, "", fmt.Sprintf("scope: %s   (%s)", m.recordScopeName(), hint))
		}
		return groupTitle(m.group), rows
	case viewRecordScope:
		return m.scopeList.Title, m.filteredLines(viewRecordScope)
	case viewReview:
		return m.reviewScreen()
	case viewText:
		return m.textScreen()
	case viewRemove:
		return m.removeScreen()
	case viewChange:
		return m.changeScreen()
	case viewForm:
		return m.formScreen()
	case viewPlan:
		return m.planScreen()
	case viewCommand:
		return m.commandScreen()
	case viewSummary:
		return m.summaryScreen()
	case viewBrowse:
		return m.browseList.Title, m.filteredLines(viewBrowse)
	case viewUnlink:
		return m.unlinkList.Title, m.filteredLines(viewUnlink)
	case viewPick:
		return m.pickScreen()
	case viewReviewQueue:
		return m.queueList.Title, m.filteredLines(viewReviewQueue)
	case viewObservations:
		return m.projectTitle(), append(m.tabStrip(), m.filteredLines(viewObservations)...)
	case viewConcepts:
		return m.projectTitle(), append(m.tabStrip(), m.filteredLines(viewConcepts)...)
	case viewRecords:
		return m.projectTitle(), append(m.tabStrip(), m.filteredLines(viewRecords)...)
	case viewSearch:
		if len(m.searchList.Items()) == 0 && m.filterOf(viewSearch) == nil {
			return m.searchList.Title, []string{"no results"}
		}
		return m.searchList.Title, m.filteredLines(viewSearch)
	}
	return "Projects", m.filteredLines(viewProjects)
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

// projectTitle is the frame title of a project's tabs: the project's name.
func (m *tuiModel) projectTitle() string {
	if m.selectedProject != nil {
		return m.selectedProject.Name
	}
	return "Project"
}

// tabStrip is the two lines above a project's list: the three tabs with their
// counts, and under the active one a rule. o, c and r switch between them.
func (m *tuiModel) tabStrip() []string {
	tabs := []struct {
		label  string
		active bool
	}{
		{fmt.Sprintf("Observations (%d)", m.totalRows(viewObservations)), m.state == viewObservations},
		{fmt.Sprintf("Concepts (%d)", m.totalRows(viewConcepts)), m.state == viewConcepts},
		{fmt.Sprintf("Records (%d)", m.totalRows(viewRecords)), m.state == viewRecords},
	}
	const sep = "  │  "
	strip, rule := "  ", "  "
	for i, t := range tabs {
		if i > 0 {
			strip += sep
			rule += strings.Repeat(" ", lipgloss.Width(sep))
		}
		strip += t.label
		if t.active {
			rule += strings.Repeat("━", lipgloss.Width(t.label))
		} else {
			rule += strings.Repeat(" ", lipgloss.Width(t.label))
		}
	}
	return []string{strip, strings.TrimRight(rule, " ")}
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

	// A notice can be a long sentence (a refusal says what blocked it and what to do),
	// so it is wrapped to the window instead of cut at the edge.
	var notice []string
	if m.notice != "" {
		notice = []string{""}
		wrap := lipgloss.NewStyle().Width(inner)
		for _, l := range strings.Split(m.notice, "\n") {
			notice = append(notice, strings.Split(wrap.Render(l), "\n")...)
		}
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

// scopeForForm is the Records scope's project, to start the record form with; empty
// when the scope is every scope.
func (m *tuiModel) scopeForForm() string {
	if name := m.recordScopeName(); name != "all scopes" {
		return name
	}
	return ""
}

// projectName is the project whose tabs are open, or "".
func (m *tuiModel) projectName() string {
	if m.selectedProject != nil {
		return m.selectedProject.Name
	}
	return ""
}

// newFindInput is the prompt of the viewer's find.
func newFindInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 200
	return ti
}
