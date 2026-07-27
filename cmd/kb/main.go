// kb is a command-line and interactive (TUI) interface for a
// github.com/rsdoiel/knowledge knowledge base.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	knowledge "github.com/rsdoiel/knowledge"
)

func main() {
	os.Exit(mainRun(os.Args[1:], os.Stdout, os.Stderr))
}

// mainRun is main's testable body: parses global flags, handles the
// no-args/help cases (which don't need an open KnowledgeBase), opens the
// database, and dispatches to the matched verb.
func mainRun(args []string, out, errOut io.Writer) int {
	dbPath, jsonOut, rest, err := parseGlobalFlags(args)
	if err != nil {
		fmt.Fprintf(errOut, "kb: %v\n", err)
		return 2
	}

	if len(rest) == 0 {
		// TODO(W7): launch the interactive TUI here instead of printing usage.
		printUsage(out)
		return 0
	}
	if rest[0] == "help" || rest[0] == "-h" || rest[0] == "--help" {
		printUsage(out)
		return 0
	}

	// merge operates on explicit -a/-b/-out paths, not the ambient --db
	// database at all -- opening (and potentially auto-creating) an
	// unrelated ./agents/knowledge.db here would be a pointless, surprising
	// side effect for a verb that never touches it.
	if rest[0] == "merge" {
		return dispatch(verbs, nil, jsonOut, rest, out, errOut)
	}

	resolvedPath, err := resolveDBPath(dbPath)
	if err != nil {
		fmt.Fprintf(errOut, "kb: %v\n", err)
		return 1
	}
	kb, err := knowledge.Open(resolvedPath)
	if err != nil {
		fmt.Fprintf(errOut, "kb: open %s: %v\n", resolvedPath, err)
		return 1
	}
	defer kb.Close()

	return dispatch(verbs, kb, jsonOut, rest, out, errOut)
}

// parseGlobalFlags consumes leading --db/--json flags from args and
// returns them along with the remaining, unconsumed arguments (the verb
// and its own arguments/flags).
func parseGlobalFlags(args []string) (dbPath string, jsonOut bool, rest []string, err error) {
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--db":
			if i+1 >= len(args) {
				return "", false, nil, fmt.Errorf("--db requires a path argument")
			}
			dbPath = args[i+1]
			i += 2
		case "--json":
			jsonOut = true
			i++
		default:
			return dbPath, jsonOut, args[i:], nil
		}
	}
	return dbPath, jsonOut, args[i:], nil
}

// resolveDBPath returns the absolute database path: dbPath itself if
// absolute, dbPath joined onto the current directory if relative and
// non-empty, or knowledge.DefaultPath(cwd) if dbPath is "".
func resolveDBPath(dbPath string) (string, error) {
	if dbPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return knowledge.DefaultPath(cwd), nil
	}
	if filepath.IsAbs(dbPath) {
		return dbPath, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, dbPath), nil
}
