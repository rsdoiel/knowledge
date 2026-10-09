package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rsdoiel/knowledge"
)

// `record set-status REF` with no status reviews the record, then asks
// (DR-0060): the record is shown in a pager, the person chooses among the
// moves the transition table allows, confirms, and only then is anything
// written. Backing out at either step writes nothing and exits 1 (workspace
// DR-0003: the command ran correctly and the answer is no). The review needs a
// terminal and exits 2 without one.

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
	out, err := reviewWith(t, run, "a\ny\n", "0001")
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
	// The order is: pager, the choice, the confirmation, the write.
	order := []string{"[pager]", "clasm/DR-0001", "proposed", "[a]ccepted", "[y/N]", "status set to accepted"}
	at := 0
	for _, want := range order {
		i := strings.Index(out[at:], want)
		if i < 0 {
			t.Fatalf("output lacks %q after offset %d, in order:\n%s", want, at, out)
		}
		at += i + len(want)
	}
}

func TestReview_OffersExactlyTheAllowedMoves(t *testing.T) {
	fakePager(t)
	for _, c := range []struct {
		id      string
		offered []string
		absent  []string
	}{
		{"0001", []string{"[a]ccepted", "[r]ejected", "[c]ancelled", "[q]"}, []string{"[s]uperseded", "[p]roposed"}},
		{"0007", []string{"[s]uperseded", "[c]ancelled", "[q]"}, []string{"[a]ccepted", "[r]ejected", "[p]roposed"}},
		{"0003", []string{"[p]roposed", "[q]"}, []string{"[a]ccepted", "[c]ancelled", "[r]ejected"}},
		{"0008", []string{"[p]roposed", "[q]"}, []string{"[a]ccepted", "[c]ancelled", "[r]ejected", "[s]uperseded"}},
	} {
		t.Run(c.id, func(t *testing.T) {
			_, run := transitionFixture(t)
			out, _ := reviewWith(t, run, "q\n", c.id)
			for _, w := range c.offered {
				if !strings.Contains(out, w) {
					t.Errorf("prompt lacks %q:\n%s", w, out)
				}
			}
			for _, w := range c.absent {
				if strings.Contains(out, w) {
					t.Errorf("prompt offers %q, which the table forbids:\n%s", w, out)
				}
			}
		})
	}
}

func TestReview_BackingOutWritesNothingAndExits1(t *testing.T) {
	fakePager(t)
	for name, input := range map[string]string{
		"q at the choice":                  "q\n",
		"end of input at the choice":       "",
		"declined at the confirmation":     "a\nn\n",
		"empty answer at the confirmation": "a\n\n",
		"anything but yes":                 "a\nmaybe\n",
		"end of input at the confirmation": "a\n",
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

func TestReview_AcceptsWordsAndAnyCase(t *testing.T) {
	fakePager(t)
	for _, c := range []struct{ input, want string }{
		{"rejected\nyes\n", "rejected"},
		{"R\nY\n", "rejected"},
		{"  cancelled \ny\n", "cancelled"},
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

// A choice the table does not offer is asked again, not applied and not fatal.
func TestReview_AsksAgainOnAChoiceItDidNotOffer(t *testing.T) {
	fakePager(t)
	root, run := transitionFixture(t)
	out, err := reviewWith(t, run, "s\nx\n\nr\ny\n", "0001") // s: not offered (no superseded_by); x: unknown; blank
	if err != nil {
		t.Fatalf("review = %v (%q)", err, out)
	}
	if got := readFixture(t, root, "clasm", "0001"); !strings.Contains(got, "status: rejected") {
		t.Errorf("record file does not carry status rejected:\n%s", got)
	}
}

func TestReview_AFinalRecordIsRefusedBeforeThePager(t *testing.T) {
	copyPath := fakePager(t)
	_, run := transitionFixture(t)
	_, err := reviewWith(t, run, "a\ny\n", "0005")
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
		_, err := reviewWith(t, run, "r\ny\n", append([]string{"0001"}, extra...)...)
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
	reviewStdin = strings.NewReader("r\ny\n")
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
	out, err := reviewWith(t, run, "r\ny\n", "0001")
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
	if out, err := reviewWith(t, run, "r\ny\n", "0001"); err != nil {
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

func TestBinary_ReviewFromATerminal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	bin := builtKB(t)
	master, slave := openPtyForTest(t)
	_, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"})
	if _, err := master.WriteString("r\ny\n"); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "record", "set-status", "clasm/DR-0001")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "KB_PAGER=cat")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	if code := exitCode(t, cmd.Run()); code != 0 {
		t.Fatalf("exit = %d from a terminal, want 0", code)
	}
	if got := readFixture(t, root, "clasm", "0001"); !strings.Contains(got, "status: rejected") {
		t.Errorf("record file does not carry status rejected:\n%s", got)
	}
}

func TestBinary_ReviewWithRedirectedStreamsIsExit2(t *testing.T) {
	bin := builtKB(t)
	_, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"})
	before := readFixture(t, root, "clasm", "0001")
	cmd := exec.Command(bin, "record", "set-status", "clasm/DR-0001")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader("r\ny\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if code := exitCode(t, cmd.Run()); code != 2 {
		t.Errorf("exit = %d, want 2 (stderr %q)", code, stderr.String())
	}
	if after := readFixture(t, root, "clasm", "0001"); after != before {
		t.Errorf("a refused review changed the record file:\n%s", after)
	}
}
