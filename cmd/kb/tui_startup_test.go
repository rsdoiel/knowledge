package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// startupDriver runs a binary on a pseudo-terminal that never answers a
// terminal query. It prints "queried=<bool> first=<seconds>" where first is
// the time to the first menu text, or "none".
const startupDriver = `
import os, pty, select, sys, time
binary, cwd = sys.argv[1], sys.argv[2]
pid, fd = pty.fork()
if pid == 0:
    os.chdir(cwd)
    os.environ["TERM"] = "xterm-256color"
    os.execv(binary, [binary])
start = time.time()
buf = b""
first = None
queried = False
while time.time() - start < 12:
    r, _, _ = select.select([fd], [], [], 0.05)
    if r:
        try:
            data = os.read(fd, 65536)
        except OSError:
            break
        if not data:
            break
        buf += data
        if b"\x1b]11;?" in buf:
            queried = True
        if first is None and b"Projects" in buf:
            first = time.time() - start
            break
try:
    os.write(fd, b"q")
except OSError:
    pass
print("queried=%s first=%s" % (queried, ("%.2f" % first) if first else "none"))
`

/** TestStartup_NoWaitOnATerminalThatDoesNotAnswer pins DR-0084: the first
 * screen does not wait on a terminal colour query. Today (Bubble Tea v1) the
 * library asks before any of our code runs and waits five seconds.
 */
func TestStartup_NoWaitOnATerminalThatDoesNotAnswer(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not available")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "kb")
	build := exec.Command(goBin, "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	ws := filepath.Join(dir, "ws")
	if err := os.Mkdir(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "init", ws).CombinedOutput(); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	out, err := exec.Command(py, "-I", "-c", startupDriver, bin, ws).CombinedOutput()
	if err != nil {
		t.Fatalf("driver: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	t.Logf("%s", got)
	if strings.Contains(got, "queried=True") {
		t.Errorf("the terminal was asked for its colours before the first screen: %s", got)
	}
	_, field, _ := strings.Cut(got, "first=")
	if secs, err := strconv.ParseFloat(field, 64); err != nil || secs >= 2.0 {
		t.Errorf("first screen took too long or never drew: %s", got)
	}
}
