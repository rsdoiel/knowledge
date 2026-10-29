package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

/** releaseSandbox is a scratch checkout of a two-module repository with a bare
 * origin, a root tag already published, and stub gh, go and make programs on PATH
 * that log what they were asked to do.
 */
type releaseSandbox struct {
	t      *testing.T
	work   string // the checkout release.bash runs in
	origin string // the bare repository it pushes to
	log    string // file the stubs append to
	path   string // PATH with the stubs first
}

func (sb *releaseSandbox) git(dir string, args ...string) string {
	sb.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		sb.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

/** newReleaseSandbox builds the sandbox. When rootTag is true the root tag v0.0.9
 * is already on origin, as it is once the draft has been finalized.
 *
 * Parameters:
 *   t (*testing.T) — the test
 *   rootTag (bool) — whether v0.0.9 exists on origin
 *
 * Returns:
 *   *releaseSandbox — the sandbox
 *
 * Example:
 *   sb := newReleaseSandbox(t, true)
 *   out, code := sb.run("y\n", "--nested")
 */
func newReleaseSandbox(t *testing.T, rootTag bool) *releaseSandbox {
	t.Helper()
	for _, tool := range []string{"bash", "git", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	script, err := os.ReadFile(filepath.Join("..", "..", "release.bash"))
	if err != nil {
		t.Fatalf("read release.bash: %v", err)
	}
	root := t.TempDir()
	sb := &releaseSandbox{t: t, origin: filepath.Join(root, "origin.git"), work: filepath.Join(root, "knowledge"),
		log: filepath.Join(root, "stub.log")}
	bin := filepath.Join(root, "bin")
	sb.path = bin + string(os.PathListSeparator) + os.Getenv("PATH")
	put := func(p, body string, mode os.FileMode) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	// gh: a published release. go: list succeeds at once, get and tidy change the
	// nested go.mod and go.sum as the real ones would. make: only logs.
	put(filepath.Join(bin, "gh"), "#!/bin/sh\necho \"gh $*\" >> \""+sb.log+"\"\n[ \"$1 $2\" = \"release view\" ] && echo false\nexit 0\n", 0o755)
	put(filepath.Join(bin, "go"), `#!/bin/sh
echo "go $* (GOWORK=$GOWORK in $(basename "$PWD"))" >> "`+sb.log+`"
case "$1 $2" in
"get example.com/k@v0.0.9") printf 'require example.com/k v0.0.9\n' >> go.mod ;;
"mod tidy") printf 'example.com/k v0.0.9 h1:x=\n' > go.sum ;;
esac
exit 0
`, 0o755)
	put(filepath.Join(bin, "make"), "#!/bin/sh\necho \"make $*\" >> \""+sb.log+"\"\nexit ${STUB_MAKE_EXIT:-0}\n", 0o755)

	if out, err := exec.Command("git", "init", "-q", "--bare", "-b", "main", sb.origin).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	if err := os.MkdirAll(sb.work, 0o755); err != nil {
		t.Fatal(err)
	}
	sb.git(sb.work, "init", "-q", "-b", "main")
	sb.git(sb.work, "remote", "add", "origin", sb.origin)
	put(filepath.Join(sb.work, "release.bash"), string(script), 0o755)
	put(filepath.Join(sb.work, "codemeta.json"), `{"version": "0.0.9", "releaseNotes": "notes"}`, 0o644)
	put(filepath.Join(sb.work, "go.mod"), "module example.com/k\n\ngo 1.27.3\n", 0o644)
	put(filepath.Join(sb.work, "semantic", "go.mod"), "module example.com/k/semantic\n\ngo 1.27.3\n", 0o644)
	sb.git(sb.work, "add", ".")
	sb.git(sb.work, "commit", "-q", "-m", "release commit")
	sb.git(sb.work, "push", "-q", "origin", "main")
	if rootTag {
		sb.git(sb.work, "tag", "v0.0.9")
		sb.git(sb.work, "push", "-q", "origin", "v0.0.9")
	}
	return sb
}

/** run starts release.bash with args, feeding stdin, and returns its combined
 * output and exit code.
 */
func (sb *releaseSandbox) run(stdin string, args ...string) (string, int) {
	sb.t.Helper()
	cmd := exec.Command("bash", append([]string{"./release.bash"}, args...)...)
	cmd.Dir = sb.work
	cmd.Env = append(os.Environ(), "PATH="+sb.path, "POLL_SECONDS=0", "PROXY_SECONDS=0", "PROXY_TRIES=2",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		sb.t.Fatalf("run release.bash: %v", err)
	}
	return string(out), code
}

func (sb *releaseSandbox) stubLog() string {
	b, _ := os.ReadFile(sb.log)
	return string(b)
}

func (sb *releaseSandbox) originHas(ref string) bool {
	cmd := exec.Command("git", "-C", sb.origin, "rev-parse", "-q", "--verify", ref)
	return cmd.Run() == nil
}

// The nested module is tagged by the same script, after the root tag is public:
// semantic/vX.Y.Z, same number as the root.
func TestReleaseNested_TagsTheNestedModule(t *testing.T) {
	sb := newReleaseSandbox(t, true)
	out, code := sb.run("y\n", "--nested")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if !sb.originHas("refs/tags/semantic/v0.0.9") {
		t.Errorf("origin has no semantic/v0.0.9 tag\n%s", out)
	}
	log := sb.stubLog()
	for _, want := range []string{"go get example.com/k@v0.0.9 (GOWORK=off in semantic)", "go mod tidy (GOWORK=off in semantic)",
		"make check-go-version", "make check-semantic-tag"} {
		if !strings.Contains(log, want) {
			t.Errorf("stub log lacks %q:\n%s", want, log)
		}
	}
	// The go.mod bump was committed and pushed before the tag, and the tag is on it.
	head := sb.git(sb.work, "rev-parse", "HEAD")
	if got := sb.git(sb.work, "rev-parse", "semantic/v0.0.9^{commit}"); got != head {
		t.Errorf("tag is on %s, HEAD is %s", got, head)
	}
	if got := sb.git(sb.origin, "rev-parse", "main"); got != head {
		t.Errorf("origin main is %s, HEAD is %s: the bump was not pushed", got, head)
	}
	if msg := sb.git(sb.work, "log", "-1", "--format=%s"); msg != "semantic: require example.com/k v0.0.9" {
		t.Errorf("last commit is %q", msg)
	}
	if !strings.Contains(out, "go install example.com/k/semantic/cmd/kb@v0.0.9") {
		t.Errorf("output does not say how to install:\n%s", out)
	}
}

func TestReleaseNested_AnsweringNoLeavesNoTag(t *testing.T) {
	sb := newReleaseSandbox(t, true)
	out, code := sb.run("n\n", "--nested")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if sb.originHas("refs/tags/semantic/v0.0.9") {
		t.Errorf("tag was pushed after n\n%s", out)
	}
	if strings.Contains(sb.git(sb.work, "tag", "--list"), "semantic/") {
		t.Errorf("tag was made locally after n")
	}
}

func TestReleaseNested_Refusals(t *testing.T) {
	t.Run("root tag not public yet", func(t *testing.T) {
		sb := newReleaseSandbox(t, false)
		out, code := sb.run("y\n", "--nested")
		if code == 0 || !strings.Contains(out, "finalize the draft first") {
			t.Fatalf("exit %d, want a refusal\n%s", code, out)
		}
		if strings.Contains(sb.stubLog(), "go get") {
			t.Errorf("went ahead without the root tag")
		}
	})
	t.Run("nested tag already on origin", func(t *testing.T) {
		sb := newReleaseSandbox(t, true)
		sb.git(sb.work, "tag", "semantic/v0.0.9")
		sb.git(sb.work, "push", "-q", "origin", "semantic/v0.0.9")
		out, code := sb.run("y\n", "--nested")
		if code == 0 || !strings.Contains(out, "already on origin") {
			t.Fatalf("exit %d, want a refusal\n%s", code, out)
		}
	})
	t.Run("tracked file has changes", func(t *testing.T) {
		sb := newReleaseSandbox(t, true)
		if err := os.WriteFile(filepath.Join(sb.work, "codemeta.json"), []byte(`{"version": "0.0.9", "releaseNotes": "edited"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		out, code := sb.run("y\n", "--nested")
		if code == 0 || !strings.Contains(out, "tracked files have changes") {
			t.Fatalf("exit %d, want a refusal\n%s", code, out)
		}
	})
	t.Run("a failed check stops before the tag", func(t *testing.T) {
		sb := newReleaseSandbox(t, true)
		t.Setenv("STUB_MAKE_EXIT", "1")
		out, code := sb.run("y\n", "--nested")
		if code == 0 {
			t.Fatalf("exit 0 after a failing check\n%s", out)
		}
		if sb.originHas("refs/tags/semantic/v0.0.9") {
			t.Errorf("tagged although the check failed")
		}
	})
}

func TestReleaseNested_DryRunChangesNothing(t *testing.T) {
	sb := newReleaseSandbox(t, true)
	before := sb.git(sb.work, "rev-parse", "HEAD")
	out, code := sb.run("", "--nested", "--dry-run")
	if code != 0 || !strings.Contains(out, "dry run") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if sb.git(sb.work, "rev-parse", "HEAD") != before || sb.originHas("refs/tags/semantic/v0.0.9") || strings.Contains(sb.stubLog(), "go get") {
		t.Errorf("a dry run changed something\n%s", sb.stubLog())
	}
}

func TestRelease_UsageIsExit2(t *testing.T) {
	sb := newReleaseSandbox(t, true)
	if out, code := sb.run("", "--bogus"); code != 2 {
		t.Errorf("exit %d for an unknown flag, want 2\n%s", code, out)
	}
}
