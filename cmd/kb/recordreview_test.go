package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rsdoiel/knowledge"
)

// `record set-status REF` with no status reviews the record, then asks
// (DR-0060, DR-0065): the record is shown in a pager, the person picks one of
// the moves the transition table allows with a single key, confirms with y, and
// only then is anything written. Esc and q act at once, with no Enter. Backing
// out at either step writes nothing and exits 1 (workspace DR-0003: the command
// ran correctly and the answer is no). The review needs a terminal and exits 2
// without one. Keys are fed here as a string; the built binary on a pseudo-terminal,
// with real bytes, is at the end of the file.

// fakePager installs a pager script that announces itself on standard output,
// copies the record it was given to a file, and returns that file's path.
func fakePager(t *testing.T) (copyPath string) {
	t.Helper()
	dir := t.TempDir()
	copyPath = filepath.Join(dir, "shown.md")
	script := filepath.Join(dir, "pager.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho '[pager]'\ncat > \"$KB_TEST_PAGER_COPY\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KB_PAGER", script)
	t.Setenv("KB_TEST_PAGER_COPY", copyPath)
	return copyPath
}

func reviewWith(t *testing.T, run func(args ...string) (string, error), input string, args ...string) (string, error) {
	t.Helper()
	old := reviewStdin
	reviewStdin = strings.NewReader(input)
	defer func() { reviewStdin = old }()
	return run(append(args, "--project", "clasm")...)
}

func TestReview_ShowsTheRecordThenAsksThenApplies(t *testing.T) {
	copyPath := fakePager(t)
	root, run := transitionFixture(t)
	out, err := reviewWith(t, run, "ay", "0001")
	if err != nil {
		t.Fatalf("review = %v (%q), want it applied", err, out)
	}
	if got := readFixture(t, root, "clasm", "0001"); !strings.Contains(got, "status: accepted") {
		t.Errorf("record file does not carry status accepted:\n%s", got)
	}
	shown, rerr := os.ReadFile(copyPath)
	if rerr != nil || !strings.Contains(string(shown), "title:") {
		t.Errorf("the pager was not given the record (read err %v, got %q)", rerr, shown)
	}
	// The order is: pager, the review (its last view says what was done), the write.
	order := []string{"[pager]", "clasm/DR-0001", "from proposed to accepted? yes", "status set to accepted"}
	at := 0
	for _, want := range order {
		i := strings.Index(out[at:], want)
		if i < 0 {
			t.Fatalf("output lacks %q after offset %d, in order:\n%s", want, at, out)
		}
		at += i + len(want)
	}
}

// What the review offers is what the transition table allows from the record's
// status. (The prompt itself is drawn on a pseudo-terminal in the binary test.)
func TestReviewOptions_AreTheAllowedMoves(t *testing.T) {
	for _, c := range []struct {
		from   string
		linked bool
		want   string
	}{
		{"proposed", false, "accepted rejected cancelled"},
		{"proposed", true, "accepted superseded rejected cancelled"},
		{"accepted", false, "cancelled"},
		{"accepted", true, "superseded cancelled"},
		{"rejected", false, "proposed"},
		{"cancelled", false, "proposed"},
		{"superseded", true, ""},
		{"legacy-status", false, "proposed"},
	} {
		if got := strings.Join(reviewOptions(c.from, c.linked), " "); got != c.want {
			t.Errorf("reviewOptions(%q, %v) = %q, want %q", c.from, c.linked, got, c.want)
		}
	}
}

func TestReview_BackingOutWritesNothingAndExits1(t *testing.T) {
	fakePager(t)
	for name, input := range map[string]string{
		"q at the choice":                  "q",
		"Esc at the choice":                "\x1b",
		"end of input at the choice":       "",
		"n at the confirmation":            "an",
		"q at the confirmation":            "aq",
		"Esc at the confirmation":          "a\x1b",
		"Enter does not confirm":           "a\r",
		"end of input at the confirmation": "a",
	} {
		t.Run(name, func(t *testing.T) {
			root, run := transitionFixture(t)
			before := readFixture(t, root, "clasm", "0001")
			out, err := reviewWith(t, run, input, "0001")
			if err == nil {
				t.Fatalf("a backed-out review succeeded with %q", out)
			}
			if class, ok := classify(err); !ok || class != classNegative {
				t.Errorf("error %v is class %v, want negative (exit 1)", err, class)
			}
			if !strings.Contains(err.Error(), "unchanged") {
				t.Errorf("error %q should say the record is unchanged", err)
			}
			if after := readFixture(t, root, "clasm", "0001"); after != before {
				t.Errorf("a backed-out review changed the record file:\n%s", after)
			}
		})
	}
}

func TestReview_AcceptsUpperCaseAndIgnoresKeysItDidNotOffer(t *testing.T) {
	fakePager(t)
	for _, c := range []struct{ input, want string }{
		{"RY", "rejected"},
		{"CY", "cancelled"},
		{"sxry", "rejected"}, // s is not offered (no superseded_by), x is nothing: both ignored
		{"r y", "rejected"},  // a space is nothing
	} {
		root, run := transitionFixture(t)
		if out, err := reviewWith(t, run, c.input, "0001"); err != nil {
			t.Fatalf("input %q = %v (%q)", c.input, err, out)
		}
		if got := readFixture(t, root, "clasm", "0001"); !strings.Contains(got, "status: "+c.want) {
			t.Errorf("input %q: record does not carry %q:\n%s", c.input, c.want, got)
		}
	}
}

func TestReview_AFinalRecordIsRefusedBeforeThePager(t *testing.T) {
	copyPath := fakePager(t)
	_, run := transitionFixture(t)
	_, err := reviewWith(t, run, "ay", "0005")
	if err == nil {
		t.Fatal("reviewing a superseded record succeeded")
	}
	if class, ok := classify(err); !ok || class != classNegative {
		t.Errorf("error %v is class %v, want negative (exit 1)", err, class)
	}
	if _, serr := os.Stat(copyPath); serr == nil {
		t.Error("the pager ran for a record with no moves")
	}
}

func TestReview_NeedsATerminal(t *testing.T) {
	noTerminal(t)
	copyPath := fakePager(t)
	root, run := transitionFixture(t)
	before := readFixture(t, root, "clasm", "0001")
	for _, extra := range [][]string{{}, {"--json"}} {
		_, err := reviewWith(t, run, "ry", append([]string{"0001"}, extra...)...)
		if err == nil || !isUsageError(err) {
			t.Errorf("review without a terminal %v: err = %v, want a usage error (exit 2)", extra, err)
		}
	}
	if after := readFixture(t, root, "clasm", "0001"); after != before {
		t.Errorf("a refused review changed the record file:\n%s", after)
	}
	if _, serr := os.Stat(copyPath); serr == nil {
		t.Error("the pager ran without a terminal")
	}
}

func TestReview_JSONIsRefused(t *testing.T) {
	fakePager(t)
	kb, _ := fixtureWorkspace(t, "clasm", testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"})
	old := reviewStdin
	reviewStdin = strings.NewReader("ry")
	defer func() { reviewStdin = old }()
	var out bytes.Buffer
	err := cmdRecord(kb, nil, true, []string{"set-status", "clasm/0001"}, &out)
	if err == nil || !isUsageError(err) {
		t.Errorf("review with --json: err = %v, want a usage error", err)
	}
}

func TestSetStatus_ArgumentCounts(t *testing.T) {
	_, run := transitionFixture(t)
	for _, args := range [][]string{{}, {"0001", "accepted", "extra"}} {
		if _, err := run(append(args, "--project", "clasm")...); err == nil || !isUsageError(err) {
			t.Errorf("set-status %v: err = %v, want a usage error", args, err)
		}
	}
}

// With no pager at all the record is printed straight through and the flow
// goes on; a person without bat or less is not locked out.
func TestReview_WithoutAPagerPrintsTheRecord(t *testing.T) {
	t.Setenv("KB_PAGER", "")
	t.Setenv("PATH", t.TempDir())
	root, run := transitionFixture(t)
	out, err := reviewWith(t, run, "ry", "0001")
	if err != nil {
		t.Fatalf("review = %v (%q)", err, out)
	}
	if !strings.Contains(out, "title:") {
		t.Errorf("the record was not printed:\n%s", out)
	}
	if got := readFixture(t, root, "clasm", "0001"); !strings.Contains(got, "status: rejected") {
		t.Errorf("record file does not carry status rejected:\n%s", got)
	}
}

// A pager that exits non-zero (a person quitting with q, or a broken pipe once
// the pager has had enough) does not abort the review.
func TestReview_APagerFailureDoesNotAbort(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "pager.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KB_PAGER", script)
	root, run := transitionFixture(t)
	if out, err := reviewWith(t, run, "ry", "0001"); err != nil {
		t.Fatalf("review = %v (%q)", err, out)
	}
	if got := readFixture(t, root, "clasm", "0001"); !strings.Contains(got, "status: rejected") {
		t.Errorf("record file does not carry status rejected:\n%s", got)
	}
}

func TestPagerCommand(t *testing.T) {
	has := func(names ...string) func(string) (string, error) {
		return func(n string) (string, error) {
			for _, x := range names {
				if x == n {
					return "/usr/bin/" + n, nil
				}
			}
			return "", errors.New("not found")
		}
	}
	env := func(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }
	for _, c := range []struct {
		name string
		env  map[string]string
		path func(string) (string, error)
		want []string
	}{
		{"KB_PAGER wins", map[string]string{"KB_PAGER": "most -w"}, has("bat", "less"), []string{"most", "-w"}},
		{"bat when installed", nil, has("bat", "less"), []string{"bat", "-l", "markdown"}},
		{"less otherwise", nil, has("less"), []string{"less", "-R"}},
		{"nothing", nil, has(), nil},
		{"blank KB_PAGER is unset", map[string]string{"KB_PAGER": "  "}, has("less"), []string{"less", "-R"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := pagerCommand(env(c.env), c.path)
			if strings.Join(got, " ") != strings.Join(c.want, " ") {
				t.Errorf("pagerCommand = %v, want %v", got, c.want)
			}
		})
	}
}

// The prompt letters are the first letters of the statuses. They must be
// distinct across the vocabulary, and none may be q, which backs out: a
// status added later that collided would make a choice ambiguous.
func TestStatusKeys_AreDistinctAndLeaveQFree(t *testing.T) {
	seen := map[rune]string{}
	for _, s := range knowledge.RecordStatuses {
		k := statusKey(s)
		if k == 0 {
			t.Errorf("statusKey(%q) = 0", s)
		}
		if k == 'q' {
			t.Errorf("statusKey(%q) is q, which backs out of the review", s)
		}
		if prev, dup := seen[k]; dup {
			t.Errorf("statuses %q and %q share the key %q", prev, s, k)
		}
		seen[k] = s
	}
}

// ─── the real binary on a terminal ───────────────────────────────────────────

// ptyScreen collects what the program writes to a pseudo-terminal, so a test can
// wait for the prompt before it types, as a person would.
type ptyScreen struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func newPtyScreen(master *os.File) *ptyScreen {
	sc := &ptyScreen{}
	go func() {
		b := make([]byte, 4096)
		for {
			n, err := master.Read(b)
			if n > 0 {
				sc.mu.Lock()
				sc.buf.Write(b[:n])
				sc.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	return sc
}

func (sc *ptyScreen) text() string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.buf.String()
}

// waitFor fails the test if text does not appear within ten seconds.
func (sc *ptyScreen) waitFor(t *testing.T, text string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(sc.text(), text) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%q never appeared on the terminal; it showed:\n%s", text, sc.text())
}

// reviewOnATerminal starts the built kb on a pseudo-terminal with the pager set
// to cat and returns what the test needs to type at it and to see the result.
func reviewOnATerminal(t *testing.T) (root string, master *os.File, screen *ptyScreen, wait func() int) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	bin := builtKB(t)
	master, slave := openPtyForTest(t)
	_, root = fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"})
	cmd := exec.Command(bin, "record", "set-status", "clasm/DR-0001")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "KB_PAGER=cat")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	if err := cmd.Start(); err != nil {
		t.Fatalf("start kb: %v", err)
	}
	screen = newPtyScreen(master)
	return root, master, screen, func() int {
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			return exitCode(t, err)
		case <-time.After(15 * time.Second):
			_ = cmd.Process.Kill()
			t.Fatalf("kb did not exit; it showed:\n%s", screen.text())
			return -1
		}
	}
}

func TestBinary_ReviewFromATerminal(t *testing.T) {
	root, master, screen, wait := reviewOnATerminal(t)
	// The prompt names the record, where it is, and the moves on offer.
	screen.waitFor(t, "[q]uit")
	for _, want := range []string{"clasm/DR-0001", "proposed", "[a]ccepted", "[r]ejected", "[c]ancelled"} {
		if !strings.Contains(screen.text(), want) {
			t.Errorf("the prompt lacks %q:\n%s", want, screen.text())
		}
	}
	if strings.Contains(screen.text(), "[s]uperseded") {
		t.Error("the prompt offers superseded, which needs a superseded_by")
	}
	master.WriteString("r") // no Enter
	screen.waitFor(t, "from proposed to rejected")
	master.WriteString("y") // no Enter
	if code := wait(); code != 0 {
		t.Fatalf("exit = %d from a terminal, want 0:\n%s", code, screen.text())
	}
	if got := readFixture(t, root, "clasm", "0001"); !strings.Contains(got, "status: rejected") {
		t.Errorf("record file does not carry status rejected:\n%s", got)
	}
}

// A lone Esc is a cancel by itself: no Enter after it, at either step.
func TestBinary_ReviewEscCancelsWithNoEnter(t *testing.T) {
	for name, keys := range map[string][]string{
		"at the choice":         {"\x1b"},
		"at the confirmation":   {"r", "\x1b"},
		"q at the choice":       {"q"},
		"n at the confirmation": {"a", "n"},
	} {
		t.Run(name, func(t *testing.T) {
			root, master, screen, wait := reviewOnATerminal(t)
			before := readFixture(t, root, "clasm", "0001")
			screen.waitFor(t, "[q]uit")
			for i, k := range keys {
				master.WriteString(k)
				if i < len(keys)-1 {
					screen.waitFor(t, "from proposed to")
				}
			}
			if code := wait(); code != 1 {
				t.Fatalf("exit = %d, want 1 for a cancel:\n%s", code, screen.text())
			}
			if after := readFixture(t, root, "clasm", "0001"); after != before {
				t.Errorf("a cancelled review changed the record file:\n%s", after)
			}
		})
	}
}

// Enter at the confirmation is not consent: it leaves the review waiting for a
// real answer.
func TestBinary_ReviewEnterDoesNotConfirm(t *testing.T) {
	root, master, screen, wait := reviewOnATerminal(t)
	screen.waitFor(t, "[q]uit")
	master.WriteString("r")
	screen.waitFor(t, "from proposed to rejected")
	master.WriteString("\r")
	time.Sleep(300 * time.Millisecond)
	if got := readFixture(t, root, "clasm", "0001"); strings.Contains(got, "status: rejected") {
		t.Fatal("Enter applied the write")
	}
	master.WriteString("n")
	if code := wait(); code != 1 {
		t.Errorf("exit = %d after n, want 1", code)
	}
}

func TestBinary_ReviewWithRedirectedStreamsIsExit2(t *testing.T) {
	bin := builtKB(t)
	_, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"})
	before := readFixture(t, root, "clasm", "0001")
	cmd := exec.Command(bin, "record", "set-status", "clasm/DR-0001")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader("ry")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if code := exitCode(t, cmd.Run()); code != 2 {
		t.Errorf("exit = %d, want 2 (stderr %q)", code, stderr.String())
	}
	if after := readFixture(t, root, "clasm", "0001"); after != before {
		t.Errorf("a refused review changed the record file:\n%s", after)
	}
}
