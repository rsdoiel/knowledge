package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	knowledge "github.com/rsdoiel/knowledge"
)

// The document summary workflow (knowledge DR-0069, v0.0.20 U7). The Review queue
// is the triage list; Enter on a section starts a unit of two steps. A summary is
// written (an external editor with the TUI suspended, else a built-in text area)
// and then shown beside its source; y drafts it as a person and promotes it. A
// draft that is already there skips the first step, which is the review form for
// promote. The writes are the library's DraftDocumentSummary and
// PromoteDocumentSummary, which the commands call, so the two cannot disagree; the
// terminal rule for promote (DR-0070) is met by being in the interface.

// editorDoneMsg says the external editor has exited.
type editorDoneMsg struct{ err error }

// execEditor runs the editor argv on path with the TUI suspended and delivers an
// editorDoneMsg when it ends. It is a variable so tests can stand in for one.
var execEditor = func(argv []string, path string) tea.Cmd {
	cmd := exec.Command(argv[0], append(append([]string(nil), argv[1:]...), path)...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return editorDoneMsg{err: err} })
}

// reviewQueueItem is one section waiting for a person.
type reviewQueueItem struct{ it knowledge.DocumentReviewItem }

func (i reviewQueueItem) Title() string {
	conf := "-"
	if i.it.Confidence != nil {
		conf = fmt.Sprintf("%.2f", *i.it.Confidence)
	}
	stale := ""
	if i.it.SummaryStale {
		stale = " STALE"
	}
	heading := unitHeading(i.it)
	return fmt.Sprintf("%-4d %-12s %-7s %s — %s   size %d  density %d  conf %s%s",
		i.it.ID, i.it.SummaryStatus, i.it.Level, i.it.DocumentTitle, heading, i.it.SourceSize, i.it.TagDensity, conf, stale)
}
func (i reviewQueueItem) Description() string { return "" }
func (i reviewQueueItem) FilterValue() string { return i.it.DocumentTitle + " " + i.it.Heading }

// unitHeading names a section for a title: its heading, or "(whole document)".
func unitHeading(it knowledge.DocumentReviewItem) string {
	if it.Heading == "" {
		return "(whole document)"
	}
	return it.Heading
}

// summaryUnit is one section's two-step unit in progress.
type summaryUnit struct {
	item      knowledge.DocumentReviewItem
	text      string // what is summarised: the section's text, or for a gist the whole document's
	candidate string // the summary under review
	edited    bool   // a person wrote or changed it, so it is drafted by a person
	reviewing bool   // false while it is being written
	edit      *editModel
	tmp       string // the external editor's file
	source    viewport.Model
}

// loadReviewQueue loads the queue for the scope: a project's, or every project's.
func (m *tuiModel) loadReviewQueue() error {
	var projectID int64
	if name := m.recordScopeName(); name != "all scopes" {
		p, err := m.kb.ProjectByName(name)
		if err != nil || p == nil {
			return fmt.Errorf("no project %q", name)
		}
		projectID = p.ID
	}
	queue, err := m.kb.DocumentReviewQueue(projectID, "")
	if err != nil {
		return err
	}
	items := make([]list.Item, len(queue))
	for i, it := range queue {
		items[i] = reviewQueueItem{it}
	}
	l := newCompactList(items, fmt.Sprintf("Review queue — %s  (%d)", m.recordScopeName(), len(items)), m.cols()-4, m.listHeight())
	l.SetStatusBarItemName("section", "sections")
	m.queueList = l
	return nil
}

// reloadReviewQueue refreshes the queue after a write, keeping the cursor's place.
func (m *tuiModel) reloadReviewQueue() {
	at := m.queueList.Index()
	if err := m.loadReviewQueue(); err != nil {
		m.setErr(err)
		return
	}
	if n := len(m.queueList.Items()); n > 0 {
		if at >= n {
			at = n - 1
		}
		m.queueList.Select(at)
	}
}

// updateReviewQueue handles a key on the queue. Enter starts the unit; q goes back
// to the Documents menu; a widens the scope.
func (m *tuiModel) updateReviewQueue(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m.openGroup("document")
	case "esc":
		return m, nil
	case "/":
		m.startSearch()
		return m, nil
	case "a":
		m.scopeAll = !m.scopeAll
		m.reloadReviewQueue()
		return m, nil
	case "enter":
		if it, ok := m.queueList.SelectedItem().(reviewQueueItem); ok {
			return m.startSummary(it.it)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.queueList, cmd = m.queueList.Update(msg)
	return m, cmd
}

// editorArgv is $VISUAL, else $EDITOR, split into words; nil when neither is set.
func editorArgv() []string {
	for _, name := range []string{"VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			if words, err := splitCommandLine(v); err == nil && len(words) > 0 {
				return words
			}
		}
	}
	return nil
}

// startSummary begins a unit: a draft goes straight to the review, anything else
// to writing.
func (m *tuiModel) startSummary(it knowledge.DocumentReviewItem) (tea.Model, tea.Cmd) {
	u := &summaryUnit{item: it, text: strings.TrimSpace(it.Body)}
	if u.text == "" && it.Level == "gist" {
		// A gist row holds no text of its own: it summarises the document, whose
		// text is its sections in order.
		if secs, err := m.kb.DocumentSections(it.DocumentID); err == nil {
			var parts []string
			for _, s := range secs {
				if s.Level == "gist" || strings.TrimSpace(s.Body) == "" {
					continue
				}
				if s.Heading != "" {
					parts = append(parts, "## "+s.Heading)
				}
				parts = append(parts, strings.TrimSpace(s.Body), "")
			}
			u.text = strings.TrimSpace(strings.Join(parts, "\n"))
		}
	}
	m.unit = u
	if it.SummaryStatus == "drafted" && strings.TrimSpace(it.SummaryBody) != "" {
		u.candidate = it.SummaryBody
		return m.reviewSummary()
	}
	return m.writeSummary(u.text)
}

// writeSummary opens the editor on initial: the external one with the TUI
// suspended, or the built-in text area.
func (m *tuiModel) writeSummary(initial string) (tea.Model, tea.Cmd) {
	u := m.unit
	u.reviewing = false
	m.setState(viewSummary)
	if argv := editorArgv(); argv != nil {
		f, err := os.CreateTemp("", "kb-summary-*.md")
		if err != nil {
			return m.leaveUnit("cannot make a file for the editor: %v", err)
		}
		header := fmt.Sprintf("# Write the summary of this section below. Lines at the top that start with # are removed.\n# %s — %s (section %d)\n\n",
			u.item.DocumentTitle, unitHeading(u.item), u.item.ID)
		_, werr := f.WriteString(header + initial)
		f.Close()
		if werr != nil {
			os.Remove(f.Name())
			return m.leaveUnit("cannot write the editor's file: %v", werr)
		}
		u.tmp = f.Name()
		u.edit = nil
		return m, execEditor(argv, f.Name())
	}
	e := newEdit(initial, false, m.cols()-8)
	e.allowSame = true
	h := m.bodyHeight() - 6
	if h < 5 {
		h = 5
	}
	e.ta.SetHeight(h)
	u.edit = e
	return m, nil
}

// stripComments removes the header of # lines the editor's file started with.
func stripComments(s string) string {
	lines := strings.Split(s, "\n")
	i := 0
	for i < len(lines) && strings.HasPrefix(lines[i], "#") {
		i++
	}
	return strings.TrimSpace(strings.Join(lines[i:], "\n"))
}

// handleEditorDone takes what the external editor left in its file.
func (m *tuiModel) handleEditorDone(msg editorDoneMsg) (tea.Model, tea.Cmd) {
	u := m.unit
	if u == nil || m.state != viewSummary || u.tmp == "" {
		return m, nil
	}
	b, err := os.ReadFile(u.tmp)
	os.Remove(u.tmp)
	u.tmp = ""
	if msg.err != nil {
		return m.leaveUnit("the editor failed (%v); nothing was changed", msg.err)
	}
	if err != nil {
		return m.leaveUnit("cannot read what the editor wrote: %v", err)
	}
	return m.acceptSummary(stripComments(string(b)))
}

// acceptSummary takes written text: an empty or unchanged one is refused and the
// unit ends with a notice; anything else goes to the review.
func (m *tuiModel) acceptSummary(text string) (tea.Model, tea.Cmd) {
	u := m.unit
	if text == "" {
		return m.leaveUnit("the summary is empty; nothing was changed")
	}
	if text == strings.TrimSpace(u.candidate) || (u.candidate == "" && text == u.text) {
		return m.leaveUnit("the summary is unchanged from the %s; nothing was changed", map[bool]string{true: "draft", false: "section text"}[u.candidate != ""])
	}
	u.candidate, u.edited = text, true
	return m.reviewSummary()
}

// reviewSummary shows the candidate summary above its source.
func (m *tuiModel) reviewSummary() (tea.Model, tea.Cmd) {
	u := m.unit
	u.reviewing = true
	h := m.bodyHeight() - 9
	if m.bodyHeight() <= 0 {
		h = 11
	}
	if h < 3 {
		h = 3
	}
	vp := viewport.New(m.cols()-4, h)
	vp.SetContent(lipgloss.NewStyle().Width(m.cols() - 4).Render(u.text))
	u.source = vp
	m.setState(viewSummary)
	return m, nil
}

// leaveUnit goes back to the queue with a notice.
func (m *tuiModel) leaveUnit(format string, a ...any) (tea.Model, tea.Cmd) {
	m.unit = nil
	m.setState(viewReviewQueue)
	m.notice = fmt.Sprintf(format, a...)
	return m, nil
}

// updateSummary takes a key of the unit: text while writing in the built-in area,
// otherwise y, e, the cancels and scrolling.
func (m *tuiModel) updateSummary(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	u := m.unit
	if u == nil {
		m.setState(viewReviewQueue)
		return m, nil
	}
	if !u.reviewing {
		if u.edit == nil {
			return m, nil // the external editor has the terminal
		}
		u.edit.Update(msg)
		switch {
		case u.edit.cancelled:
			if u.candidate != "" {
				return m.reviewSummary()
			}
			return m.leaveUnit("cancelled; section %d is unchanged", u.item.ID)
		case u.edit.submitted:
			text := strings.TrimSpace(u.edit.value())
			u.edit.submitted = false
			if text == "" || text == u.text && u.candidate == "" || text == strings.TrimSpace(u.candidate) && u.candidate != "" {
				u.edit.message = "empty or unchanged: write the summary, or Esc to cancel"
				return m, nil
			}
			return m.acceptSummary(text)
		}
		return m, nil
	}
	for _, k := range keysOf(msg) {
		switch k.String() {
		case "y":
			return m.approveSummary()
		case "e":
			return m.writeSummary(u.candidate)
		case "n", "q", "esc":
			return m.leaveUnit("cancelled; section %d is unchanged (summaries approved earlier are kept)", u.item.ID)
		case "j", "k", "up", "down", " ", "pgup", "pgdown":
			u.source, _ = u.source.Update(k)
		}
	}
	return m, nil
}

// approveSummary drafts the summary as a person when one wrote it, and promotes it.
// A draft left as it was is only promoted, so its author stays.
func (m *tuiModel) approveSummary() (tea.Model, tea.Cmd) {
	u := m.unit
	id := u.item.ID
	if u.edited || u.item.SummaryStatus != "drafted" {
		if err := m.kb.DraftDocumentSummary(id, u.candidate, "human", nil); err != nil {
			return m.leaveUnit("%v", err)
		}
	}
	if err := m.kb.PromoteDocumentSummary(id); err != nil {
		return m.leaveUnit("drafted, but not promoted: %v\nequivalent:  kb document review promote %d", err, id)
	}
	m.unit = nil
	m.reloadReviewQueue()
	m.setState(viewReviewQueue)
	sid := strconv.FormatInt(id, 10)
	m.notice = "✓ section " + sid + " summarised and reviewed"
	if u.edited || u.item.SummaryStatus != "drafted" {
		m.notice += "\nequivalent:  kb document draft " + sid + " BODY --by human"
	}
	m.notice += "\nequivalent:  kb document review promote " + sid
	return m, nil
}

// summaryScreen is the unit's screen: the text area while writing, else the
// summary above its source.
func (m *tuiModel) summaryScreen() (string, []string) {
	u := m.unit
	title := "Summary — " + u.item.DocumentTitle + " — " + unitHeading(u.item) + " (section " + strconv.FormatInt(u.item.ID, 10) + ")"
	wrap := func(s string) []string {
		return strings.Split(lipgloss.NewStyle().Width(m.cols()-4).Render(s), "\n")
	}
	if !u.reviewing {
		if u.edit == nil {
			return title, []string{"the editor is open"}
		}
		body := wrap("Write the summary; the section's text is below to start from. Enter accepts, Ctrl-J starts a new line, Esc cancels.")
		return title, append(append(body, ""), strings.Split(u.edit.View(), "\n")...)
	}
	body := []string{"summary — what will be saved:"}
	sum := wrap(u.candidate)
	if len(sum) > 5 {
		sum = append(sum[:4], "…")
	}
	body = append(body, sum...)
	body = append(body, "", "source:")
	body = append(body, strings.Split(strings.TrimRight(u.source.View(), "\n"), "\n")...)
	return title, body
}

// summaryLegend names the keys of the unit's current step.
func (m *tuiModel) summaryLegend() string {
	if m.unit != nil && m.unit.reviewing {
		return "↑/↓ j/k scroll source   y approve   e edit   n q Esc cancel   Ctrl-C quit"
	}
	return "Enter accept   Ctrl-J newline   Esc cancel   Ctrl-C quit"
}

// openFromCommand opens the review form for a section named at the `:` prompt
// (`document review promote ID`): it must have a draft to show.
func (m *tuiModel) openFromCommand(id int64) (tea.Model, tea.Cmd) {
	queue, err := m.kb.DocumentReviewQueue(0, "")
	if err != nil {
		return m.refuse("%v", err)
	}
	for _, it := range queue {
		if it.ID != id {
			continue
		}
		if it.SummaryStatus != "drafted" {
			return m.refuse("section %d has no draft to promote; summarise it from Documents → Review queue, or draft it first: kb document draft %d BODY --by WHO", id, id)
		}
		if err := m.loadReviewQueue(); err != nil {
			return m.refuse("%v", err)
		}
		m.setState(viewReviewQueue)
		return m.startSummary(it)
	}
	return m.refuse("section %d is not waiting for review (it does not exist, or is already reviewed)", id)
}
