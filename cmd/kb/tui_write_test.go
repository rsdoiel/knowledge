package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// The first write in the TUI (knowledge DR-0066, DR-0065, DR-0067; v0.0.19 T6):
// on a selected record, Enter reads it and s sets its status. Both show the record
// in the pager with the TUI suspended; s then runs the chooser and confirmation of
// T3 and writes through the same function the command line uses, and the screen
// shows the equivalent command. Esc and q cancel at every step and write nothing.

// shown records what the pager seam was asked to show.
type shown struct {
	calls   int
	argv    []string
	raw     string
	purpose string
}

// fakePagerSeam replaces execPager: it records the request and, like a pager that
// has finished, answers with a pagerDoneMsg. The test delivers that message itself.
func fakePagerSeam(t *testing.T) *shown {
	t.Helper()
	got := &shown{}
	old := execPager
	execPager = func(argv []string, raw []byte, purpose string) tea.Cmd {
		got.calls++
		got.argv, got.raw, got.purpose = argv, string(raw), purpose
		return func() tea.Msg { return pagerDoneMsg{purpose: purpose} }
	}
	t.Cleanup(func() { execPager = old })
	return got
}

// donePager delivers the pager's answer to the model, as bubbletea would.
func donePager(t *testing.T, m *tuiModel, cmd tea.Cmd) (*tuiModel, tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("no pager command was returned")
	}
	next, c := m.Update(cmd())
	return next.(*tuiModel), c
}

// writeModel is a TUI on the Records screen of a workspace with real record files:
// 0001 proposed, 0002 accepted with a superseded_by, 0003 superseded.
func writeModel(t *testing.T) (*tuiModel, string) {
	t.Helper()
	t.Setenv("KB_PAGER", "cat")
	t.Setenv("KB_PROJECT", "")
	kb, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Title: "The proposed one", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-03"},
		testRecord{ID: "0002", Title: "The accepted one", Kind: "decision", Trigger: "design", Status: "accepted", SupersededBy: []string{"0004"}, Date: "2026-08-02"},
		testRecord{ID: "0003", Title: "The final one", Kind: "decision", Trigger: "design", Status: "superseded", SupersededBy: []string{"0004"}, Date: "2026-08-01"},
		testRecord{ID: "0004", Title: "The replacement", Kind: "decision", Trigger: "design", Status: "accepted", Supersedes: []string{"0002", "0003"}, Date: "2026-08-04"},
	)
	m, err := newTUIModelAt(kb, nil, tuiStart{state: viewRecordScope, all: true})
	if err != nil {
		t.Fatalf("newTUIModelAt: %v", err)
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return next.(*tuiModel), root
}

// selectRef puts the cursor on a record in the Records screen.
func selectRef(t *testing.T, m *tuiModel, ref string) {
	t.Helper()
	for i, it := range m.scopeList.Items() {
		if r, ok := it.(scopedRecordItem); ok && r.e.Ref == ref {
			m.scopeList.Select(i)
			return
		}
	}
	t.Fatalf("no %s in the Records screen", ref)
}

// beginStatus presses s on a record and delivers the pager's answer, leaving the
// model at the chooser.
func beginStatus(t *testing.T, m *tuiModel, ref string) *tuiModel {
	t.Helper()
	selectRef(t, m, ref)
	m, cmd := press(t, m, ch('s'))
	m, _ = donePager(t, m, cmd)
	if m.state != viewReview {
		t.Fatalf("after the pager the state is %s, want the review", viewStateNames[m.state])
	}
	return m
}

// ─── reading ─────────────────────────────────────────────────────────────────

func TestWrite_EnterReadsTheRecordInThePagerAndWritesNothing(t *testing.T) {
	seam := fakePagerSeam(t)
	m, root := writeModel(t)
	selectRef(t, m, "clasm/DR-0001")
	before := readFixture(t, root, "clasm", "0001")
	m, cmd := press(t, m, keyEnter)
	if seam.calls != 1 || seam.purpose != "read" {
		t.Fatalf("pager calls %d, purpose %q; want one read", seam.calls, seam.purpose)
	}
	if !strings.Contains(seam.raw, "The proposed one") || !strings.Contains(seam.raw, "status: proposed") {
		t.Errorf("the pager was not given the record file:\n%s", seam.raw)
	}
	if len(seam.argv) == 0 || seam.argv[0] != "cat" {
		t.Errorf("pager argv = %v, want KB_PAGER's cat", seam.argv)
	}
	m, _ = donePager(t, m, cmd)
	if m.state != viewRecordScope {
		t.Errorf("after reading the state is %s, want the Records screen again", viewStateNames[m.state])
	}
	if after := readFixture(t, root, "clasm", "0001"); after != before {
		t.Error("reading changed the record file")
	}
}

func TestWrite_TheLegendOffersReadAndStatus(t *testing.T) {
	m, _ := writeModel(t)
	// This model starts with every scope shown, so the scope key offers to narrow.
	for _, want := range []string{"Enter read", "s status", "a this scope", "q back"} {
		if !strings.Contains(m.View(), want) {
			t.Errorf("the legend lacks %q:\n%s", want, m.View())
		}
	}
}

// ─── the status review ───────────────────────────────────────────────────────

func TestWrite_SShowsThePagerThenTheChooser(t *testing.T) {
	seam := fakePagerSeam(t)
	m, _ := writeModel(t)
	selectRef(t, m, "clasm/DR-0001")
	m, cmd := press(t, m, ch('s'))
	if seam.calls != 1 || seam.purpose != "status" {
		t.Fatalf("pager calls %d purpose %q; want one status", seam.calls, seam.purpose)
	}
	m, _ = donePager(t, m, cmd)
	v := m.View()
	for _, want := range []string{"clasm/DR-0001", "proposed", "[a]ccepted", "[r]ejected", "[c]ancelled", "[q]uit", "The proposed one"} {
		if !strings.Contains(v, want) {
			t.Errorf("the chooser lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "[s]uperseded") {
		t.Errorf("the chooser offers superseded for a record with no superseded_by:\n%s", v)
	}
}

func TestWrite_ChoosingAndConfirmingWritesThroughTheCLIsFunction(t *testing.T) {
	fakePagerSeam(t)
	m, root := writeModel(t)
	m = beginStatus(t, m, "clasm/DR-0001")
	m, _ = press(t, m, ch('r'))
	if !strings.Contains(m.View(), "from proposed to rejected") {
		t.Fatalf("the confirmation does not name the move:\n%s", m.View())
	}
	m, cmd := press(t, m, ch('y'))
	if isQuit(cmd) {
		t.Fatal("finishing a review quit the program")
	}
	if m.state != viewRecordScope {
		t.Errorf("after the write the state is %s, want the Records screen", viewStateNames[m.state])
	}
	if got := readFixture(t, root, "clasm", "0001"); !strings.Contains(got, "status: rejected") {
		t.Errorf("the file does not carry rejected:\n%s", got)
	}
	if st := storedStatus(t, m.kb, "clasm", "0001"); st != "rejected" {
		t.Errorf("the database carries %q, want rejected", st)
	}
	v := m.View()
	for _, want := range []string{"clasm/DR-0001 status set to rejected", "equivalent:  kb record set-status clasm/DR-0001 rejected"} {
		if !strings.Contains(v, want) {
			t.Errorf("the screen lacks %q:\n%s", want, v)
		}
	}
	// the row now shows the new status
	if !strings.Contains(v, "rejected") || strings.Contains(v, "clasm/DR-0001        proposed") {
		t.Errorf("the list still shows the old status:\n%s", v)
	}
	// the notice goes with the next key
	m, _ = press(t, m, ch('j'))
	if strings.Contains(m.View(), "equivalent:") {
		t.Error("the equivalent line stayed after another key")
	}
}

// The TUI is a terminal, so accepting is allowed here (DR-0061, point 6).
func TestWrite_AcceptingIsAllowedInTheTUI(t *testing.T) {
	noTerminal(t) // the seam says there is no terminal; the TUI does not ask
	fakePagerSeam(t)
	m, root := writeModel(t)
	m = beginStatus(t, m, "clasm/DR-0001")
	m, _ = press(t, m, ch('a'), ch('y'))
	if got := readFixture(t, root, "clasm", "0001"); !strings.Contains(got, "status: accepted") {
		t.Errorf("the file does not carry accepted:\n%s", got)
	}
}

func TestWrite_SupersededIsOfferedOnlyWithASupersededBy(t *testing.T) {
	fakePagerSeam(t)
	m, _ := writeModel(t)
	m = beginStatus(t, m, "clasm/DR-0002") // accepted, with superseded_by
	v := m.View()
	if !strings.Contains(v, "[s]uperseded") || !strings.Contains(v, "[c]ancelled") || strings.Contains(v, "[a]ccepted") {
		t.Errorf("accepted with a link should offer superseded and cancelled only:\n%s", v)
	}
}

func TestWrite_ARecordWithNoMovesSaysSoBeforeThePager(t *testing.T) {
	seam := fakePagerSeam(t)
	m, _ := writeModel(t)
	selectRef(t, m, "clasm/DR-0003")
	m, _ = press(t, m, ch('s'))
	if seam.calls != 0 {
		t.Error("the pager ran for a record with no moves")
	}
	if v := m.View(); !strings.Contains(v, "final") || m.state != viewRecordScope {
		t.Errorf("state %s; the screen should say the record is final:\n%s", viewStateNames[m.state], v)
	}
}

// ─── cancelling ──────────────────────────────────────────────────────────────

func TestWrite_EveryCancelWritesNothingAndSaysSo(t *testing.T) {
	for name, keys := range map[string][]tea.Msg{
		"q at the choice":         {ch('q')},
		"Esc at the choice":       {keyEsc},
		"n at the confirmation":   {ch('r'), ch('n')},
		"q at the confirmation":   {ch('r'), ch('q')},
		"Esc at the confirmation": {ch('r'), keyEsc},
	} {
		t.Run(name, func(t *testing.T) {
			fakePagerSeam(t)
			m, root := writeModel(t)
			before := readFixture(t, root, "clasm", "0001")
			m = beginStatus(t, m, "clasm/DR-0001")
			var cmd tea.Cmd
			m, cmd = press(t, m, keys...)
			if isQuit(cmd) || m.state != viewRecordScope {
				t.Fatalf("a cancel left state %s (quit %v), want the Records screen", viewStateNames[m.state], isQuit(cmd))
			}
			if v := m.View(); !strings.Contains(v, "cancelled") || !strings.Contains(v, "clasm/DR-0001") || strings.Contains(v, "equivalent:") {
				t.Errorf("the screen should say the record is unchanged and show no command:\n%s", v)
			}
			if after := readFixture(t, root, "clasm", "0001"); after != before {
				t.Errorf("a cancel changed the file:\n%s", after)
			}
		})
	}
}

func TestWrite_EnterDoesNotConfirmAndCtrlCQuitsWithoutWriting(t *testing.T) {
	fakePagerSeam(t)
	m, root := writeModel(t)
	before := readFixture(t, root, "clasm", "0001")
	m = beginStatus(t, m, "clasm/DR-0001")
	m, _ = press(t, m, ch('r'), keyEnter)
	if m.state != viewReview {
		t.Fatalf("Enter left the review: %s", viewStateNames[m.state])
	}
	if _, cmd := press(t, m, keyCtrlC); !isQuit(cmd) {
		t.Error("Ctrl-C did not quit")
	}
	if after := readFixture(t, root, "clasm", "0001"); after != before {
		t.Error("something was written")
	}
}

// ─── checked against the file as it is now ───────────────────────────────────

// The list can be stale: the move is checked against the record on disk when it is
// applied, by the same function as the command line, and a refusal is a notice.
func TestWrite_AMoveRefusedByTheTableIsANoticeAndWritesNothing(t *testing.T) {
	fakePagerSeam(t)
	m, root := writeModel(t)
	m = beginStatus(t, m, "clasm/DR-0001") // the chooser offers accepted, rejected, cancelled
	// Meanwhile the file is edited to rejected, from which accepted is not a move.
	testRecord{ID: "0001", Title: "The proposed one", Kind: "decision", Trigger: "design", Status: "rejected", Date: "2026-08-03", Project: "clasm"}.write(t, root+"/clasm/decisions")
	before := readFixture(t, root, "clasm", "0001")
	m, _ = press(t, m, ch('a'), ch('y'))
	if after := readFixture(t, root, "clasm", "0001"); after != before {
		t.Errorf("a refused move changed the file:\n%s", after)
	}
	if v := m.View(); !strings.Contains(v, "cannot be set to accepted") || m.state != viewRecordScope {
		t.Errorf("state %s; the screen should say the move is refused:\n%s", viewStateNames[m.state], v)
	}
}

// ─── keys together, and the project tab ──────────────────────────────────────

func TestWrite_ChoiceAndConfirmationInOneMessage(t *testing.T) {
	fakePagerSeam(t)
	m, root := writeModel(t)
	m = beginStatus(t, m, "clasm/DR-0001")
	m, _ = press(t, m, runeKey("ry"))
	if got := readFixture(t, root, "clasm", "0001"); !strings.Contains(got, "status: rejected") {
		t.Errorf("ry in one message did not apply:\n%s", got)
	}
}

func TestWrite_TheProjectRecordsTabHasTheSameKeys(t *testing.T) {
	seam := fakePagerSeam(t)
	m, root := writeModel(t)
	// Open the project list, a project, and its Records tab.
	pm, err := newTUIModelAt(m.kb, nil, tuiStart{state: viewProjects})
	if err != nil {
		t.Fatal(err)
	}
	pm, _ = press(t, pm, tea.WindowSizeMsg{Width: 80, Height: 24})
	pm, _ = press(t, pm, keyEnter, ch('r'))
	if pm.state != viewRecords {
		t.Fatalf("state = %s, want the Records tab", viewStateNames[pm.state])
	}
	for _, want := range []string{"Enter read", "s status"} {
		if !strings.Contains(pm.View(), want) {
			t.Errorf("the Records tab legend lacks %q:\n%s", want, pm.View())
		}
	}
	// the list is newest first: DR-0004; move to DR-0001
	for i := 0; i < 8 && !strings.Contains(selectedTitle(pm), "The proposed one"); i++ {
		pm, _ = press(t, pm, ch('j'))
	}
	pm, cmd := press(t, pm, ch('s'))
	if seam.calls != 1 {
		t.Fatalf("pager calls %d, want 1", seam.calls)
	}
	pm, _ = donePager(t, pm, cmd)
	pm, _ = press(t, pm, ch('c'), ch('y'))
	if got := readFixture(t, root, "clasm", "0001"); !strings.Contains(got, "status: cancelled") {
		t.Errorf("the tab's s did not write cancelled:\n%s", got)
	}
	if pm.state != viewRecords {
		t.Errorf("after the write the state is %s, want the Records tab", viewStateNames[pm.state])
	}
}

// selectedTitle is the selected row of the project's Records tab.
func selectedTitle(m *tuiModel) string {
	if it, ok := m.recordList.SelectedItem().(recordItem); ok {
		return it.Title()
	}
	return ""
}

// ─── no pager ────────────────────────────────────────────────────────────────

// With no pager at all the record is shown in the built-in viewer, and the review
// goes on from it.
func TestWrite_WithNoPagerTheBuiltInViewerShowsTheRecord(t *testing.T) {
	seam := fakePagerSeam(t)
	m, _ := writeModel(t)
	t.Setenv("KB_PAGER", "")
	t.Setenv("PATH", t.TempDir()) // no bat, no less
	selectRef(t, m, "clasm/DR-0001")
	m, _ = press(t, m, keyEnter)
	if seam.calls != 0 {
		t.Error("the pager seam ran with no pager available")
	}
	if m.state != viewText {
		t.Fatalf("state = %s, want the built-in viewer", viewStateNames[m.state])
	}
	v := m.View()
	for _, want := range []string{"The proposed one", "status: proposed", "q back"} {
		if !strings.Contains(v, want) {
			t.Errorf("the viewer lacks %q:\n%s", want, v)
		}
	}
	m, _ = press(t, m, ch('q'))
	if m.state != viewRecordScope {
		t.Errorf("q from the viewer gave %s, want the Records screen", viewStateNames[m.state])
	}
}

func TestWrite_WithNoPagerStatusGoesViewerThenChooser(t *testing.T) {
	fakePagerSeam(t)
	m, root := writeModel(t)
	t.Setenv("KB_PAGER", "")
	t.Setenv("PATH", t.TempDir())
	selectRef(t, m, "clasm/DR-0001")
	m, _ = press(t, m, ch('s'))
	if m.state != viewText || !strings.Contains(m.View(), "continue") {
		t.Fatalf("state %s; the viewer should offer to continue:\n%s", viewStateNames[m.state], m.View())
	}
	m, _ = press(t, m, ch('q'))
	if m.state != viewReview {
		t.Fatalf("after the viewer the state is %s, want the chooser", viewStateNames[m.state])
	}
	m, _ = press(t, m, ch('r'), ch('y'))
	if got := readFixture(t, root, "clasm", "0001"); !strings.Contains(got, "status: rejected") {
		t.Errorf("the file does not carry rejected:\n%s", got)
	}
}

// A long record scrolls in the viewer.
func TestWrite_TheViewerScrolls(t *testing.T) {
	fakePagerSeam(t)
	kb, _ := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Title: "A long record", Kind: "decision", Trigger: "design", Status: "proposed",
			Body: "\n" + strings.Repeat("a line of the body that makes the record long\n", 80)})
	t.Setenv("KB_PAGER", "")
	t.Setenv("PATH", t.TempDir())
	t.Setenv("KB_PROJECT", "")
	m, err := newTUIModelAt(kb, nil, tuiStart{state: viewRecordScope, all: true})
	if err != nil {
		t.Fatal(err)
	}
	m, _ = press(t, m, tea.WindowSizeMsg{Width: 80, Height: 24}, keyEnter)
	top := m.View()
	m, _ = press(t, m, ch('j'), ch('j'), ch('j'))
	if m.View() == top {
		t.Error("j did not scroll the viewer")
	}
	if !strings.Contains(top, "title:") {
		t.Errorf("the viewer should start at the top of the file:\n%s", top)
	}
}

// ─── the real binary: the terminal is handed to a pager and taken back ───────

// waitingPager writes a pager script that prints a marker and the record, then
// waits for one key from the terminal, like less does, and returns its path.
func waitingPager(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pager.sh")
	script := "#!/bin/sh\necho '[PAGER]'\ncat\nhead -c1 < /dev/tty > /dev/null\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func startPendingOnATerminal(t *testing.T) (root string, master *os.File, screen *ptyScreen, wait func() int) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	_, root = fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Title: "The proposed one", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"})
	bin := builtKB(t)
	master, slave := openPtyForTest(t)
	cmd := exec.Command(bin, "-i", "record", "pending")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "KB_PAGER="+waitingPager(t))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	if err := cmd.Start(); err != nil {
		t.Fatalf("start kb: %v", err)
	}
	screen = newPtyScreen(master)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return root, master, screen, func() int {
		select {
		case err := <-done:
			return exitCode(t, err)
		case <-time.After(10 * time.Second):
			t.Fatalf("kb did not exit; it showed:\n%s", screen.text())
			return -1
		}
	}
}

func TestBinary_TUISetStatusFromAPagerToTheWrite(t *testing.T) {
	root, master, screen, wait := startPendingOnATerminal(t)
	screen.waitFor(t, "clasm/DR-0001")
	master.WriteString("s")
	screen.waitFor(t, "[PAGER]") // the terminal now belongs to the pager
	screen.waitFor(t, "status: proposed")
	master.WriteString("x") // the pager's own key
	screen.waitFor(t, "[r]ejected")
	master.WriteString("r") // no Enter
	screen.waitFor(t, "from proposed to rejected")
	master.WriteString("y")
	screen.waitFor(t, "equivalent:  kb record set-status clasm/DR-0001 rejected")
	if got := readFixture(t, root, "clasm", "0001"); !strings.Contains(got, "status: rejected") {
		t.Errorf("the file does not carry rejected:\n%s", got)
	}
	master.WriteString("qqq") // back out of the Records screen, the menu, and quit
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0:\n%s", code, screen.text())
	}
}

func TestBinary_TUIEscCancelsTheReviewWithNoEnter(t *testing.T) {
	root, master, screen, wait := startPendingOnATerminal(t)
	before := readFixture(t, root, "clasm", "0001")
	screen.waitFor(t, "clasm/DR-0001")
	master.WriteString("s")
	screen.waitFor(t, "[PAGER]")
	master.WriteString("x")
	screen.waitFor(t, "[r]ejected")
	master.WriteString("r")
	screen.waitFor(t, "from proposed to rejected")
	master.WriteString("\x1b") // a lone Esc
	screen.waitFor(t, "cancelled; clasm/DR-0001 is unchanged")
	if after := readFixture(t, root, "clasm", "0001"); after != before {
		t.Errorf("a cancel changed the file:\n%s", after)
	}
	if strings.Contains(screen.text(), "equivalent:") {
		t.Error("a cancelled review showed a command")
	}
	master.WriteString("qqq")
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}

func TestBinary_TUIEnterReadsInThePagerAndComesBack(t *testing.T) {
	root, master, screen, wait := startPendingOnATerminal(t)
	before := readFixture(t, root, "clasm", "0001")
	screen.waitFor(t, "clasm/DR-0001")
	master.WriteString("\r")
	screen.waitFor(t, "[PAGER]")
	master.WriteString("x")
	screen.waitForCount(t, "clasm/DR-0001", 2) // the list is drawn again
	if after := readFixture(t, root, "clasm", "0001"); after != before {
		t.Error("reading changed the file")
	}
	master.WriteString("qqq")
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0:\n%s", code, screen.text())
	}
}
