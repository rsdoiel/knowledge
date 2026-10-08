package main

import (
	"os"
	"testing"
)

// TestMain sets KB_CEILING_DIRECTORIES to the system temporary directory, so
// no test can walk up out of its own t.TempDir() into a real workspace, even
// when TMPDIR points inside one (DR-0058). It also clears KB_DB and KB_QUIET,
// which a developer may have set in their shell and which change what kb does.
func TestMain(m *testing.M) {
	os.Setenv("KB_CEILING_DIRECTORIES", os.TempDir())
	os.Unsetenv("KB_DB")
	os.Unsetenv("KB_QUIET")
	os.Exit(m.Run())
}
