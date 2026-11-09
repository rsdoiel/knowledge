package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// kb-completion(1) EXIT STATUS: 66 is "no home directory found" and 74 is "a write
// failed part way". The older tests accept 73 or 74 for an unwritable directory
// and none gives 66, so the page named codes no test proved.

func TestCompletion_NoHomeDirectoryIsNoInput(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		t.Skip("os.UserHomeDir reads other variables here")
	}
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"completion", "bash", "-install"}, &out, &errOut); code != 66 {
		t.Errorf("exit %d, want 66 (no_input); stderr %q", code, errOut.String())
	}
}

func TestCompletion_ScriptDirectoryThatCannotBeMadeIsCantCreate(t *testing.T) {
	data := t.TempDir()
	os.WriteFile(filepath.Join(data, "bash-completion"), []byte("a file where a directory must go"), 0o644)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", data)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"completion", "bash", "-install"}, &out, &errOut); code != 73 {
		t.Errorf("exit %d, want 73 (cant_create); stderr %q", code, errOut.String())
	}
}

// A startup file that cannot be read or written is a failed read or write, 74,
// after the script itself was written.
func TestCompletion_StartupFileThatCannotBeWrittenIsIO(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	_, startup, _ := completionTargets("kb", "bash", home, "", runtime.GOOS)
	if err := os.MkdirAll(startup, 0o755); err != nil { // a directory where the file must be
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"completion", "bash", "-install"}, &out, &errOut); code != 74 {
		t.Errorf("exit %d, want 74 (io); stderr %q", code, errOut.String())
	}
}
