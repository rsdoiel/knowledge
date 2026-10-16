package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Reported 2026-10-16 on macOS: `kb completion bash -install` left a new
// Terminal session without completion. The script went to the bash-completion
// user directory, which only a bash-completion v2 loader reads, and macOS's
// /bin/bash 3.2 has none. The install must leave a line in the bash startup
// file that sources the script, so a plain bash finds it.
func TestCompletion_InstallBashIsFoundByAPlainNewShell(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"completion", "bash", "-install"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	// A new interactive shell with no bash-completion package: just the startup file.
	cmd := exec.Command(bash, "--noprofile", "--norc", "-c", `. "$HOME/.bashrc"; complete -p kb`)
	cmd.Env = append(os.Environ(), "HOME="+home)
	b, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(b), "-F _kb kb") {
		t.Errorf("a new shell has no completion for kb: %v\n%s", err, b)
	}
}

func TestCompletion_BashStartupFileByPlatform(t *testing.T) {
	home := t.TempDir()
	if got := bashStartupFile(home, "linux"); got != filepath.Join(home, ".bashrc") {
		t.Errorf("linux: %s", got)
	}
	if got := bashStartupFile(home, "darwin"); got != filepath.Join(home, ".bash_profile") {
		t.Errorf("darwin, nothing exists: %s", got)
	}
	// Creating .bash_profile would hide an existing .profile from a login shell.
	os.WriteFile(filepath.Join(home, ".profile"), []byte("x\n"), 0o644)
	if got := bashStartupFile(home, "darwin"); got != filepath.Join(home, ".profile") {
		t.Errorf("darwin, .profile exists: %s", got)
	}
	os.WriteFile(filepath.Join(home, ".bash_profile"), []byte("x\n"), 0o644)
	if got := bashStartupFile(home, "darwin"); got != filepath.Join(home, ".bash_profile") {
		t.Errorf("darwin, .bash_profile exists: %s", got)
	}
}

func TestCompletion_InstallBashAddsOneStartupLine(t *testing.T) {
	home := t.TempDir()
	rc := filepath.Join(home, ".bash_profile")
	os.WriteFile(rc, []byte("export A=1"), 0o644) // no trailing newline
	for i := 0; i < 2; i++ {
		if _, err := InstallCompletion("kb", "bash", home, "", "darwin"); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(rc)
	script := filepath.Join(home, ".local", "share", "bash-completion", "completions", "kb")
	if n := strings.Count(string(b), "# kb completion"); n != 1 || !strings.Contains(string(b), script) {
		t.Errorf("startup file has the line %d times, want 1:\n%s", n, b)
	}
	if !strings.HasPrefix(string(b), "export A=1\n") {
		t.Errorf("existing content not preserved: %q", b)
	}
}

func TestCompletion_UninstallBashRemovesScriptAndLine(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	rc := filepath.Join(home, ".bashrc")
	os.WriteFile(rc, []byte("export A=1\nexport B=2\n"), 0o644)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"completion", "bash", "-install"}, &out, &errOut); code != 0 {
		t.Fatalf("install: exit %d, %q", code, errOut.String())
	}
	out.Reset()
	if code := mainRun([]string{"completion", "bash", "-uninstall"}, &out, &errOut); code != 0 {
		t.Fatalf("uninstall: exit %d, stderr %q", code, errOut.String())
	}
	script := filepath.Join(home, ".local", "share", "bash-completion", "completions", "kb")
	if _, err := os.Stat(script); !os.IsNotExist(err) {
		t.Errorf("script still there: %v", err)
	}
	if b, _ := os.ReadFile(rc); string(b) != "export A=1\nexport B=2\n" {
		t.Errorf("startup file not restored: %q", b)
	}
	if !strings.Contains(out.String(), script) {
		t.Errorf("stdout %q does not say what was removed", out.String())
	}
	// Nothing left to remove is the answer no (exit 1), not a failure.
	if code := mainRun([]string{"completion", "bash", "-uninstall"}, &out, &errOut); code != 1 {
		t.Errorf("second uninstall: exit %d, want 1", code)
	}
}

func TestCompletion_UninstallBashLeavesAForeignFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	path := filepath.Join(home, ".local", "share", "bash-completion", "completions", "kb")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("# someone else's\n"), 0o644)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"completion", "bash", "-uninstall"}, &out, &errOut); code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if b, _ := os.ReadFile(path); string(b) != "# someone else's\n" {
		t.Errorf("foreign file touched: %q", b)
	}
}

func TestCompletion_UninstallPowerShellRemovesFileAndProfileLine(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "powershell")
	profile := filepath.Join(dir, "Microsoft.PowerShell_profile.ps1")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(profile, []byte("# my profile\n"), 0o644)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"completion", "powershell", "-install"}, &out, &errOut); code != 0 {
		t.Fatalf("install: exit %d", code)
	}
	if code := mainRun([]string{"completion", "powershell", "-uninstall"}, &out, &errOut); code != 0 {
		t.Fatalf("uninstall: exit %d, %q", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "kb-completion.ps1")); !os.IsNotExist(err) {
		t.Errorf("script still there: %v", err)
	}
	if b, _ := os.ReadFile(profile); string(b) != "# my profile\n" {
		t.Errorf("profile not restored: %q", b)
	}
}

func TestCompletion_CheckReportsWhatIsMissingAndWhatIsFine(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"completion", "bash", "-check"}, &out, &errOut); code != 1 {
		t.Errorf("nothing installed: exit %d, want 1", code)
	}
	if !strings.Contains(out.String(), "not installed") {
		t.Errorf("report does not say so:\n%s", out.String())
	}
	if code := mainRun([]string{"completion", "bash", "-install"}, &out, &errOut); code != 0 {
		t.Fatalf("install: exit %d", code)
	}
	out.Reset()
	if code := mainRun([]string{"completion", "bash", "-check"}, &out, &errOut); code != 0 {
		t.Errorf("after install: exit %d, want 0\n%s", code, out.String())
	}
	// A script from an older kb is out of date, which is a problem to report.
	script := filepath.Join(home, ".local", "share", "bash-completion", "completions", "kb")
	os.WriteFile(script, []byte("# bash completion for kb\nold\n"), 0o644)
	out.Reset()
	if code := mainRun([]string{"completion", "bash", "-check"}, &out, &errOut); code != 1 || !strings.Contains(out.String(), "out of date") {
		t.Errorf("stale script: exit %d\n%s", code, out.String())
	}
}

func TestCompletion_BashHeaderDoesNotRecommendSourceProcessSubstitution(t *testing.T) {
	s := completionScript(t, "bash")
	if strings.Contains(s, "source <(") {
		t.Error("`source <(...)` silently does nothing in bash 3.2 (macOS /bin/bash); recommend eval")
	}
	if !strings.Contains(s, `eval "$(kb completion bash)"`) {
		t.Error("the header does not give the eval form")
	}
}

func TestCompletion_InstallUninstallCheckAreExclusive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"completion", "bash", "-install", "-uninstall"}, &out, &errOut); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}
