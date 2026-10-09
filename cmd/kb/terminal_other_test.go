//go:build !linux

package main

import (
	"os"
	"testing"
)

// openPtyForTest is not available off Linux; the tests that need it skip first.
func openPtyForTest(t *testing.T) (master, slave *os.File) {
	t.Helper()
	t.Skip("pseudo-terminal tests are Linux only")
	return nil, nil
}
