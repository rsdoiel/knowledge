// kb is a command-line and interactive (TUI) interface for a
// github.com/rsdoiel/knowledge knowledge base. The work is in internal/cli.
package main

import (
	"os"

	"github.com/rsdoiel/knowledge/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
