package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// pagerDoneMsg says the pager (or the built-in viewer) has finished showing a
// record. purpose is "read" or "status": a status review goes on to the chooser.
type pagerDoneMsg struct {
	purpose string
	err     error
}

// execPager shows raw in the pager argv, with the TUI suspended while it runs, and
// delivers a pagerDoneMsg when it ends. It is a variable so tests can stand in for
// a terminal program.
var execPager = func(argv []string, raw []byte, purpose string) tea.Cmd {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = bytes.NewReader(raw)
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return pagerDoneMsg{purpose: purpose, err: err} })
}

/** selectedRecordRef returns the qualified reference of the record under the
 * cursor on the Records screen or a project's Records tab.
 *
 * Returns:
 *   ref (string) — such as clasm/DR-0001
 *   ok (bool) — false when no record is selected (an empty list, another screen)
 *
 * Example:
 *   if ref, ok := m.selectedRecordRef(); ok { ... }
 */
func (m *tuiModel) selectedRecordRef() (ref string, ok bool) {
	switch m.state {
	case viewRecordScope:
		if it, found := m.scopeList.SelectedItem().(scopedRecordItem); found {
			return it.e.Ref, true
		}
	case viewRecords:
		if it, found := m.recordList.SelectedItem().(recordItem); found {
			return refOf(it.r, projectNames(m.kb)).String(), true
		}
	}
	return "", false
}

/** showSelected starts reading the selected record (purpose "read") or setting its
 * status (purpose "status"): the record file is shown in the pager with the TUI
 * suspended, or in the built-in viewer when there is no pager, and a status review
 * goes on to the chooser afterwards. A record with no move to make says so before
 * anything is shown.
 *
 * Parameters:
 *   purpose (string) — "read" or "status"
 *
 * Returns:
 *   (tea.Model, tea.Cmd) — the model and the command that runs the pager, if any
 *
 * Example:
 *   return m.showSelected("status")
 */
func (m *tuiModel) showSelected(purpose string) (tea.Model, tea.Cmd) {
	ref, ok := m.selectedRecordRef()
	if !ok {
		return m, nil
	}
	return m.showRecord(ref, purpose)
}

// showRecord shows the record with that reference for reading or for a status
// review: in the pager when there is one, else in the built-in viewer.
func (m *tuiModel) showRecord(ref, purpose string) (tea.Model, tea.Cmd) {
	rec, err := resolveRecordForWrite(m.kb, ref, recordFlags{})
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	rf, raw, err := loadRecordFile(recordRoot(m.kb, recordFlags{}), rec)
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	m.reviewRec, m.reviewRef, m.reviewBack, m.reviewRaw = rec, ref, m.state, raw
	m.reviewFrom = rf.Record.Status
	m.reviewOptions = reviewOptions(rf.Record.Status, len(rf.SupersededBy) > 0)
	if purpose == "status" && len(m.reviewOptions) == 0 {
		m.notice = fmt.Sprintf("%s is %s, which is final; there is no move to make", ref, rf.Record.Status)
		return m, nil
	}
	if argv := pagerCommand(os.Getenv, exec.LookPath); argv != nil {
		return m, execPager(argv, raw, purpose)
	}
	m.openViewer(purpose)
	return m, nil
}

// afterPager goes on from a finished pager or viewer: a status review to the
// chooser, a read back to where it began.
func (m *tuiModel) afterPager(purpose string) (tea.Model, tea.Cmd) {
	if purpose != "status" {
		m.setState(m.reviewBack)
		return m, nil
	}
	m.review = newReview(m.reviewRef, m.reviewFrom, m.reviewOptions)
	m.setState(viewReview)
	return m, nil
}

// handlePagerDone takes the pager's answer. A pager that exits non-zero (a person
// quitting it) is not a failure; one that could not start is, and the record is
// shown in the built-in viewer instead.
func (m *tuiModel) handlePagerDone(msg pagerDoneMsg) (tea.Model, tea.Cmd) {
	var exitErr *exec.ExitError
	if msg.err != nil && !errors.As(msg.err, &exitErr) {
		m.notice = fmt.Sprintf("the pager could not run (%v); showing the record here", msg.err)
		m.openViewer(msg.purpose)
		return m, nil
	}
	return m.afterPager(msg.purpose)
}

// openViewer shows the record in the built-in scrolling viewer: the fallback when
// there is no pager.
func (m *tuiModel) openViewer(purpose string) {
	// With no window size yet (a terminal that has not reported one) the viewer
	// still shows a screenful instead of nothing.
	h := m.bodyHeight()
	if h <= 0 {
		h = 20
	}
	vp := viewport.New(m.cols()-4, h)
	wrapped := lipgloss.NewStyle().Width(m.cols() - 4).Render(string(m.reviewRaw))
	vp.SetContent(wrapped)
	m.text = vp
	m.textPurpose = purpose
	m.setState(viewText)
}

// updateText handles a key in the built-in viewer. The arrows, j and k, and the
// page keys scroll; q finishes (and continues to the chooser for a status review).
func (m *tuiModel) updateText(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m.afterPager(m.textPurpose)
	case "esc":
		return m, nil
	}
	var cmd tea.Cmd
	m.text, cmd = m.text.Update(msg)
	return m, cmd
}

// updateReview hands a key to the embedded chooser and confirmation (T3) and
// finishes when they have an answer. The embedded model asks the program to quit
// when it is done; that is ignored, since this is only one screen of it.
func (m *tuiModel) updateReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.review.Update(msg)
	if m.review.phase == phaseDone {
		return m.finishReview()
	}
	return m, nil
}

// finishReview applies a confirmed choice through applyRecordStatus, the function
// the command line uses, so the two cannot disagree about what a status change is,
// and says what was done and the command that does the same. A cancel, and a move
// the record's file now refuses, are notices and write nothing.
func (m *tuiModel) finishReview() (tea.Model, tea.Cmd) {
	chosen, confirmed := m.review.result()
	m.setState(m.reviewBack)
	if !confirmed {
		m.notice = fmt.Sprintf("cancelled; %s is unchanged", m.reviewRef)
		return m, nil
	}
	var sink bytes.Buffer
	if err := applyRecordStatus(m.kb, false, recordFlags{}, &sink, m.reviewRec, chosen); err != nil {
		m.notice = err.Error()
		return m, nil
	}
	m.reloadRecords()
	m.notice = fmt.Sprintf("✓ %s status set to %s\nequivalent:  kb record set-status %s %s", m.reviewRef, chosen, m.reviewRef, chosen)
	return m, nil
}

// reloadRecords refreshes the list the record is shown in after a write, keeping
// the cursor on the same record.
func (m *tuiModel) reloadRecords() {
	switch m.state {
	case viewRecordScope:
		if err := m.loadRecordScope(); err != nil {
			m.setErr(err)
			return
		}
		for i, it := range m.scopeList.Items() {
			if r, ok := it.(scopedRecordItem); ok && r.e.Ref == m.reviewRef {
				m.scopeList.Select(i)
			}
		}
	case viewRecords:
		if err := m.loadRecords(); err != nil {
			m.setErr(err)
			return
		}
		for i, it := range m.recordList.Items() {
			if r, ok := it.(recordItem); ok && r.r.RecordID == m.reviewRec.RecordID {
				m.recordList.Select(i)
			}
		}
	}
}

// reviewScreen is the status review's screen: the record, then the chooser or the
// confirmation.
func (m *tuiModel) reviewScreen() (string, []string) {
	return "Set status — " + m.reviewRef, []string{m.reviewRef + "  " + m.reviewRec.Title, "", m.review.View()}
}

// reviewLegend names the keys of the current phase of the review.
func (m *tuiModel) reviewLegend() string {
	if m.review != nil && m.review.phase == phaseConfirm {
		return "y apply   n q Esc cancel   Ctrl-C quit"
	}
	return "press a status key   q Esc cancel   Ctrl-C quit"
}

// textLegend names the keys of the built-in viewer.
func (m *tuiModel) textLegend() string {
	if m.textPurpose == "status" {
		return "↑/↓ j/k scroll   space page   q continue to choose a status"
	}
	return "↑/↓ j/k scroll   space page   q back"
}

// textScreen is the built-in viewer's screen.
func (m *tuiModel) textScreen() (string, []string) {
	return m.reviewRef, strings.Split(strings.TrimRight(m.text.View(), "\n"), "\n")
}
