package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	knowledge "github.com/rsdoiel/knowledge"
)

// Deep links (knowledge DR-0066 amendment, v0.0.19 T5 step 5). A complete command
// line always runs as a command and prints, on a terminal or not. Only an
// incomplete one opens the TUI, and only on a terminal: bare kb, a bare group, a
// leaf missing the argument it needs. The global -i opens the TUI at a command.

// capture replaces launchTUI for the test and returns where it was told to open.
func capture(t *testing.T) *struct {
	called bool
	start  tuiStart
} {
	t.Helper()
	got := &struct {
		called bool
		start  tuiStart
	}{}
	old := launchTUI
	launchTUI = func(_ *knowledge.KnowledgeBase, _ *DebugLog, start tuiStart) error {
		got.called, got.start = true, start
		return nil
	}
	t.Cleanup(func() { launchTUI = old })
	return got
}

// inWorkspace chdirs into a workspace holding one project with one proposed record.
func inWorkspace(t *testing.T) string {
	t.Helper()
	_, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"})
	t.Chdir(root)
	t.Setenv("KB_PROJECT", "")
	return root
}

func TestDeepLink_BareKBOpensTheTopMenuOnATerminal(t *testing.T) {
	got := capture(t)
	inWorkspace(t)
	if code, _, errOut := runMain(t); code != 0 || !got.called || got.start.state != viewMenu {
		t.Errorf("exit %d (%s), called %v, start %+v; want the top menu", code, errOut, got.called, got.start)
	}
}

// Without a terminal there is no interface to open: a usage error, never a hang.
func TestDeepLink_BareKBWithoutATerminalIsAUsageError(t *testing.T) {
	got := capture(t)
	noTerminal(t)
	inWorkspace(t)
	code, _, errOut := runMain(t)
	if code != 2 || got.called {
		t.Errorf("exit %d, called %v; want exit 2 and no interface", code, got.called)
	}
	if !strings.Contains(errOut, "terminal") {
		t.Errorf("stderr %q should say a terminal is needed", errOut)
	}
}

func TestDeepLink_ABareGroupOpensItsMenuOnATerminal(t *testing.T) {
	for _, g := range []string{"project", "record", "observation", "concept", "source", "document"} {
		got := capture(t)
		inWorkspace(t)
		if code, _, errOut := runMain(t, g); code != 0 || !got.called || got.start.state != viewGroup || got.start.group != g {
			t.Errorf("kb %s: exit %d (%s), called %v, start %+v; want the %s menu", g, code, errOut, got.called, got.start, g)
		}
	}
}

// With no terminal a bare group is the usage error it has always been.
func TestDeepLink_ABareGroupWithoutATerminalKeepsItsUsageError(t *testing.T) {
	got := capture(t)
	noTerminal(t)
	inWorkspace(t)
	if code, _, _ := runMain(t, "record"); code != 2 || got.called {
		t.Errorf("exit %d, called %v; want the usage error and no interface", code, got.called)
	}
}

// link has no menu entry, so it is not a deep link.
func TestDeepLink_AGroupWithNoMenuIsStillAUsageError(t *testing.T) {
	got := capture(t)
	inWorkspace(t)
	if code, _, _ := runMain(t, "link"); code != 2 || got.called {
		t.Errorf("kb link: exit %d, called %v; want a usage error", code, got.called)
	}
}

// The rule that matters most to scripts: a complete command prints, even on a terminal.
func TestDeepLink_ACompleteCommandPrintsOnATerminal(t *testing.T) {
	got := capture(t)
	inWorkspace(t)
	for _, args := range [][]string{{"record", "list"}, {"record", "show", "clasm/DR-0001"}, {"record", "pending"}, {"project", "list"}} {
		code, out, errOut := runMain(t, args...)
		if code != 0 || got.called || strings.TrimSpace(out) == "" {
			t.Errorf("kb %v: exit %d (%s), interface called %v, output %q; want it printed and no interface", args, code, errOut, got.called, out)
		}
	}
}

func TestDeepLink_ALeafMissingItsArgumentOpensThePicker(t *testing.T) {
	for args, want := range map[string]viewState{"record show": viewRecordScope, "project show": viewProjects} {
		got := capture(t)
		inWorkspace(t)
		if code, _, errOut := runMain(t, strings.Fields(args)...); code != 0 || !got.called || got.start.state != want {
			t.Errorf("kb %s: exit %d (%s), start %+v; want %s", args, code, errOut, got.start, viewStateNames[want])
		}
	}
}

func TestDeepLink_ALeafMissingItsArgumentWithoutATerminalIsAUsageError(t *testing.T) {
	got := capture(t)
	noTerminal(t)
	inWorkspace(t)
	if code, _, _ := runMain(t, "record", "show"); code != 2 || got.called {
		t.Errorf("exit %d, called %v; want the usage error", code, got.called)
	}
}

// ─── -i ──────────────────────────────────────────────────────────────────────

func TestDeepLink_IOpensTheInterfaceAtACommand(t *testing.T) {
	for _, c := range []struct {
		args  []string
		check func(tuiStart) bool
		want  string
	}{
		{[]string{"-i"}, func(s tuiStart) bool { return s.state == viewMenu }, "the top menu"},
		{[]string{"-i", "record"}, func(s tuiStart) bool { return s.state == viewGroup && s.group == "record" }, "the Records menu"},
		{[]string{"-i", "record", "list"}, func(s tuiStart) bool { return s.state == viewRecordScope && s.status == "" }, "Records Browse"},
		{[]string{"-i", "record", "pending"}, func(s tuiStart) bool { return s.state == viewRecordScope && s.status == "proposed" }, "Records Pending"},
		{[]string{"-i", "project", "list"}, func(s tuiStart) bool { return s.state == viewProjects }, "the project list"},
		{[]string{"-i", "search", "decision", "text"}, func(s tuiStart) bool { return s.state == viewSearch && s.term == "decision text" }, "search results for the whole term"},
		{[]string{"-i", "record", "show", "clasm/DR-0001"}, func(s tuiStart) bool {
			return s.state == viewRecordScope && s.all && s.ref == "clasm/DR-0001"
		}, "the Records screen with the cursor on the record"},
	} {
		got := capture(t)
		inWorkspace(t)
		code, _, errOut := runMain(t, c.args...)
		if code != 0 || !got.called || !c.check(got.start) {
			t.Errorf("kb %v: exit %d (%s), start %+v; want %s", c.args, code, errOut, got.start, c.want)
		}
	}
}

func TestDeepLink_IWithoutAScreenForTheCommandIsAUsageError(t *testing.T) {
	got := capture(t)
	inWorkspace(t)
	for _, args := range [][]string{{"-i", "record", "new"}, {"-i", "merge", "-a", "x"}, {"-i", "ingest", "."}, {"-i", "record", "list", "clasm"}} {
		code, _, errOut := runMain(t, args...)
		if code != 2 || got.called {
			t.Errorf("kb %v: exit %d, called %v; want a usage error", args, code, got.called)
		}
		if !strings.Contains(errOut, "-i") {
			t.Errorf("kb %v: stderr %q should explain -i", args, errOut)
		}
	}
}

func TestDeepLink_IToAMissingRecordIsNotFound(t *testing.T) {
	got := capture(t)
	inWorkspace(t)
	if code, _, _ := runMain(t, "-i", "record", "show", "clasm/DR-9999"); code != 1 || got.called {
		t.Errorf("exit %d, called %v; want exit 1 and no interface", code, got.called)
	}
}

func TestDeepLink_INeedsATerminalAndCannotBeJSON(t *testing.T) {
	got := capture(t)
	inWorkspace(t)
	if code, _, _ := runMain(t, "-json", "-i", "record", "list"); code != 2 || got.called {
		t.Errorf("-json -i: exit %d, called %v; want a usage error", code, got.called)
	}
	noTerminal(t)
	if code, _, _ := runMain(t, "-i", "record", "list"); code != 2 || got.called {
		t.Errorf("-i without a terminal: exit %d, called %v; want a usage error", code, got.called)
	}
}

// -i is a global option and goes before the verb; after it it is refused like any
// other flag the verb does not have.
func TestDeepLink_IAfterTheVerbIsRefused(t *testing.T) {
	got := capture(t)
	inWorkspace(t)
	if code, _, _ := runMain(t, "record", "list", "-i"); code != 2 || got.called {
		t.Errorf("exit %d, called %v; want a usage error", code, got.called)
	}
}

// ─── no workspace ────────────────────────────────────────────────────────────

// Opening the interface must not create a database where there is none.
func TestDeepLink_NoWorkspaceIsNoInputAndCreatesNothing(t *testing.T) {
	got := capture(t)
	dir := t.TempDir()
	t.Chdir(dir)
	code, _, errOut := runMain(t)
	if code != 66 || got.called {
		t.Errorf("exit %d (%s), called %v; want 66 and no interface", code, errOut, got.called)
	}
	if _, err := os.Stat(filepath.Join(dir, "agents")); err == nil {
		t.Error("kb created agents/ in a directory with no workspace")
	}
}

// ─── the model opens where it was told ───────────────────────────────────────

func TestDeepLink_TheModelStartsWhereItIsTold(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("KB_PROJECT", "")
	kb := recordsModel(t).kb
	for _, c := range []struct {
		start tuiStart
		check func(*tuiModel) bool
		want  string
	}{
		{tuiStart{state: viewRecordScope, status: "proposed"}, func(m *tuiModel) bool {
			return m.state == viewRecordScope && len(m.scopeList.Items()) == 1 && strings.Contains(m.View(), "Pending")
		}, "the Pending screen with its one record"},
		{tuiStart{state: viewRecordScope, all: true, ref: "alpha/DR-0001"}, func(m *tuiModel) bool {
			it, ok := m.scopeList.SelectedItem().(scopedRecordItem)
			return ok && it.e.Ref == "alpha/DR-0001"
		}, "the cursor on alpha/DR-0001"},
		{tuiStart{state: viewSearch, term: "decision"}, func(m *tuiModel) bool {
			return m.state == viewSearch && strings.Contains(m.searchList.Title, "decision")
		}, "search results for decision"},
	} {
		m, err := newTUIModelAt(kb, nil, c.start)
		if err != nil {
			t.Fatalf("%+v: %v", c.start, err)
		}
		if !c.check(m) {
			t.Errorf("%+v: want %s; state %s\n%s", c.start, c.want, viewStateNames[m.state], m.View())
		}
	}
}

// ─── the real binary ─────────────────────────────────────────────────────────

// startOnATerminal runs the built kb with args on a pseudo-terminal in a workspace
// and returns the screen and a function that waits for the exit code.
func startOnATerminal(t *testing.T, root string, args ...string) (*os.File, *ptyScreen, func() int) {
	t.Helper()
	bin := builtKB(t)
	master, slave := openPtyForTest(t)
	cmd := exec.Command(bin, args...)
	cmd.Dir = root
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	if err := cmd.Start(); err != nil {
		t.Fatalf("start kb: %v", err)
	}
	screen := newPtyScreen(master)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return master, screen, func() int {
		select {
		case err := <-done:
			return exitCode(t, err)
		case <-time.After(10 * time.Second):
			t.Fatalf("kb did not exit; it showed:\n%s", screen.text())
			return -1
		}
	}
}

func TestBinary_DeepLinksOnATerminal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	_, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"})

	master, screen, wait := startOnATerminal(t, root, "record") // a bare group
	screen.waitFor(t, "New record…")
	master.WriteString("q") // back to the top menu
	screen.waitFor(t, "kb — knowledge")
	master.WriteString("q") // quit
	if code := wait(); code != 0 {
		t.Errorf("kb record: exit %d, want 0:\n%s", code, screen.text())
	}

	master, screen, wait = startOnATerminal(t, root, "-i", "record", "pending")
	screen.waitFor(t, "Pending — ")
	screen.waitFor(t, "clasm/DR-0001")
	master.WriteString("q") // back to the Records menu
	screen.waitFor(t, "New record…")
	master.WriteString("q")
	master.WriteString("q")
	if code := wait(); code != 0 {
		t.Errorf("kb -i record pending: exit %d, want 0:\n%s", code, screen.text())
	}
}

// Scripts: with no terminal nothing opens, whatever the command line.
func TestBinary_NothingOpensWithoutATerminal(t *testing.T) {
	bin := builtKB(t)
	_, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"})
	for _, tc := range []struct {
		args []string
		code int
	}{
		{nil, 2}, {[]string{"record"}, 2}, {[]string{"record", "show"}, 2},
		{[]string{"-i", "record", "list"}, 2},
		{[]string{"record", "list"}, 0}, {[]string{"record", "show", "clasm/DR-0001"}, 0},
	} {
		cmd := exec.Command(bin, tc.args...)
		cmd.Dir = root
		cmd.Stdin = strings.NewReader("")
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		done := make(chan error, 1)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			if code := exitCode(t, err); code != tc.code {
				t.Errorf("kb %v: exit %d, want %d (stderr %q)", tc.args, code, tc.code, errOut.String())
			}
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			t.Errorf("kb %v hung with no terminal", tc.args)
		}
	}
}
