package knowledge

import (
	"os"
	"testing"
)

// TestMain sets KB_CEILING_DIRECTORIES to the system temporary directory, so
// no test can walk up out of its own t.TempDir() into a real workspace, even
// when TMPDIR points inside one (DR-0058).
func TestMain(m *testing.M) {
	os.Setenv(CeilingEnv, os.TempDir())
	os.Exit(m.Run())
}
