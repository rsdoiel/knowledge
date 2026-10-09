package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// The accepted transition requires an interactive terminal on standard input
// and output, and has no bypass (DR-0061). A model driving kb can propose,
// reject, cancel and supersede, but cannot accept. Without a terminal the
// refusal is exit 2 and a message that a person must accept it; every other
// transition is unaffected.

func noTerminal(t *testing.T) {
	t.Helper()
	old := atTerminal
	atTerminal = func(io.Writer) bool { return false }
	t.Cleanup(func() { atTerminal = old })
}

func TestIsInteractive_NotForNonTerminals(t *testing.T) {
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	file, err := os.CreateTemp(t.TempDir(), "x")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	cases := []struct {
		name string
		in   io.Reader
		out  io.Writer
	}{
		{"pipes", r, w},
		{"/dev/null is a character device but not a terminal", devnull, devnull},
		{"regular file", file, file},
		{"buffers", &bytes.Buffer{}, &bytes.Buffer{}},
		{"nil", nil, nil},
	}
	for _, c := range cases {
		if isInteractive(c.in, c.out) {
			t.Errorf("%s: isInteractive = true, want false", c.name)
		}
	}
}

func TestIsInteractive_Pty(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	_, slave := openPtyForTest(t)
	if !isInteractive(slave, slave) {
		t.Error("a pseudo-terminal on both ends: isInteractive = false, want true")
	}
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()
	if isInteractive(slave, w) {
		t.Error("terminal in, pipe out: isInteractive = true, want false")
	}
	if isInteractive(r, slave) {
		t.Error("pipe in, terminal out: isInteractive = true, want false")
	}
}

func TestCmdRecord_AcceptedNeedsATerminal(t *testing.T) {
	noTerminal(t)
	root, run := transitionFixture(t)
	before := readFixture(t, root, "clasm", "0001")
	out, err := run("0001", "accepted", "--project", "clasm")
	if err == nil {
		t.Fatalf("set-status accepted without a terminal succeeded with %q", out)
	}
	if !isUsageError(err) {
		t.Errorf("error %v is not a usage error; the refusal is exit 2", err)
	}
	if !strings.Contains(err.Error(), "person") {
		t.Errorf("error %q should say that a person must accept it", err)
	}
	if after := readFixture(t, root, "clasm", "0001"); after != before {
		t.Errorf("a refused acceptance changed the record file:\n%s", after)
	}
}

// The database is untouched too, not only the file.
func TestCmdRecord_RefusedAcceptanceLeavesTheDatabaseAlone(t *testing.T) {
	noTerminal(t)
	kb, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"})
	var out bytes.Buffer
	if err := cmdRecord(kb, nil, false, []string{"set-status", "clasm/0001", "accepted"}, &out); err == nil {
		t.Fatal("set-status accepted succeeded without a terminal")
	}
	_ = root
	out.Reset()
	if err := cmdRecord(kb, nil, false, []string{"list", "clasm", "--status", "accepted"}, &out); err != nil {
		// a listing that matches nothing is 0, but a status filter with no
		// carrier is a lookup: either way the record must not be accepted.
		out.Reset()
	}
	if strings.Contains(out.String(), "DR-0001") {
		t.Errorf("the database shows DR-0001 accepted after a refusal:\n%s", out.String())
	}
}

func TestCmdRecord_OtherTransitionsNeedNoTerminal(t *testing.T) {
	noTerminal(t)
	for _, c := range []struct{ id, to string }{
		{"0001", "rejected"},
		{"0001", "cancelled"},
		{"0002", "cancelled"},
		{"0003", "proposed"},
		{"0004", "proposed"},
		{"0007", "superseded"},
	} {
		t.Run(c.id+"->"+c.to, func(t *testing.T) {
			root, run := transitionFixture(t)
			if out, err := run(c.id, c.to, "--project", "clasm"); err != nil {
				t.Fatalf("set-status %s %s without a terminal = %v (%q), want it applied", c.id, c.to, err, out)
			}
			if got := readFixture(t, root, "clasm", c.id); !strings.Contains(got, "status: "+c.to) {
				t.Errorf("record file does not carry status %q:\n%s", c.to, got)
			}
		})
	}
}

// A stray status goes to proposed first, and that move needs no terminal; the
// acceptance after it does.
func TestCmdRecord_TwoStepAcceptanceStillNeedsATerminal(t *testing.T) {
	noTerminal(t)
	_, run := transitionFixture(t)
	if _, err := run("0008", "proposed", "--project", "clasm"); err != nil {
		t.Fatalf("legacy-status -> proposed = %v", err)
	}
	if _, err := run("0008", "accepted", "--project", "clasm"); err == nil || !isUsageError(err) {
		t.Errorf("proposed -> accepted without a terminal: err = %v, want a usage error", err)
	}
}

// Nothing on the command line turns the check off. A flag that is not defined
// is a usage error, so the refusal cannot be argued with.
func TestCmdRecord_AcceptedHasNoBypassFlag(t *testing.T) {
	noTerminal(t)
	for _, flag := range []string{"--yes", "-y", "--force", "--accept", "--tty", "--no-tty", "--interactive", "--assume-yes"} {
		root, run := transitionFixture(t)
		before := readFixture(t, root, "clasm", "0001")
		if _, err := run("0001", "accepted", "--project", "clasm", flag); err == nil || !isUsageError(err) {
			t.Errorf("%s: err = %v, want a usage error", flag, err)
		}
		if after := readFixture(t, root, "clasm", "0001"); after != before {
			t.Errorf("%s changed the record file", flag)
		}
	}
}

// ─── the real binary ─────────────────────────────────────────────────────────
//
// The seam above stands in for a terminal. These run the built kb, so what is
// tested is the path production takes: the process's own standard streams.

var (
	kbBinaryOnce sync.Once
	kbBinaryPath string
	kbBinaryErr  error
)

func builtKB(t *testing.T) string {
	t.Helper()
	kbBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "kbtest")
		if err != nil {
			kbBinaryErr = err
			return
		}
		kbBinaryPath = filepath.Join(dir, "kb")
		if out, err := exec.Command("go", "build", "-o", kbBinaryPath, ".").CombinedOutput(); err != nil {
			kbBinaryErr = &buildError{string(out), err}
		}
	})
	if kbBinaryErr != nil {
		t.Fatalf("building kb: %v", kbBinaryErr)
	}
	return kbBinaryPath
}

type buildError struct {
	out string
	err error
}

func (e *buildError) Error() string { return e.err.Error() + "\n" + e.out }

// exitCode returns the process's exit status, or fails the test if it did not
// exit normally.
func exitCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	t.Fatalf("running kb: %v", err)
	return -1
}

func TestBinary_AcceptedWithRedirectedStreamsIsExit2(t *testing.T) {
	bin := builtKB(t)
	for _, c := range []struct {
		name  string
		setup func(cmd *exec.Cmd, t *testing.T)
	}{
		{"stdin from /dev/null, stdout a pipe", func(cmd *exec.Cmd, t *testing.T) {
			f, err := os.Open(os.DevNull)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { f.Close() })
			cmd.Stdin = f
		}},
		{"stdin a pipe, stdout a pipe", func(cmd *exec.Cmd, t *testing.T) {
			cmd.Stdin = strings.NewReader("y\n")
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, root := fixtureWorkspace(t, "clasm",
				testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"})
			before := readFixture(t, root, "clasm", "0001")
			cmd := exec.Command(bin, "record", "set-status", "clasm/DR-0001", "accepted")
			cmd.Dir = root
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			c.setup(cmd, t)
			code := exitCode(t, cmd.Run())
			if code != 2 {
				t.Errorf("exit = %d, want 2 (stderr %q)", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), "person") {
				t.Errorf("stderr %q should say that a person must accept it", stderr.String())
			}
			if after := readFixture(t, root, "clasm", "0001"); after != before {
				t.Errorf("a refused acceptance changed the record file:\n%s", after)
			}
		})
	}
}

func TestBinary_AcceptedIsReachableFromATerminal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	bin := builtKB(t)
	_, slave := openPtyForTest(t)
	_, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"})
	cmd := exec.Command(bin, "record", "set-status", "clasm/DR-0001", "accepted")
	cmd.Dir = root
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	if code := exitCode(t, cmd.Run()); code != 0 {
		t.Fatalf("exit = %d from a terminal, want 0", code)
	}
	if got := readFixture(t, root, "clasm", "0001"); !strings.Contains(got, "status: accepted") {
		t.Errorf("record file does not carry status accepted:\n%s", got)
	}
}
