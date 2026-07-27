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
	dbPath, jsonOut, debugOn, rest, err := parseGlobalFlags(args)
	if err != nil {
		fmt.Fprintf(errOut, "kb: %v\n", err)
		return 2
	}

	if len(rest) == 0 {
		dl, err := openDebugLogIfRequested(debugOn, errOut)
		if err != nil {
			fmt.Fprintf(errOut, "kb: opening debug log: %v\n", err)
			return 1
		}
		defer dl.Close()
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
		if err := runTUI(kb, dl); err != nil {
			fmt.Fprintf(errOut, "kb: %v\n", err)
			return 1
		}
		return 0
	}
	if rest[0] == "help" || rest[0] == "-h" || rest[0] == "-help" || rest[0] == "--help" {
		topic := ""
		if rest[0] == "help" && len(rest) > 1 {
			topic = rest[1]
		}
		if !printHelp(out, topic) {
			fmt.Fprintf(errOut, "kb: unknown help topic %q\n", topic)
			return 2
		}
		return 0
	}
	// kb VERB -h / kb VERB --help: print that verb's page. Checked here,
	// before any database is opened, for the same reason merge is
	// special-cased below -- printing help shouldn't have the side effect
	// of creating an ambient ./agents/knowledge.db.
	if len(rest) > 1 && (rest[1] == "-h" || rest[1] == "-help" || rest[1] == "--help") {
		if !printHelp(out, rest[0]) {
			fmt.Fprintf(errOut, "kb: unknown verb %q\n", rest[0])
			return 2
		}
		return 0
	}

	dl, err := openDebugLogIfRequested(debugOn, errOut)
	if err != nil {
		fmt.Fprintf(errOut, "kb: opening debug log: %v\n", err)
		return 1
	}
	defer dl.Close()

	// merge operates on explicit -a/-b/-out paths, not the ambient --db
	// database at all -- opening (and potentially auto-creating) an
	// unrelated ./agents/knowledge.db here would be a pointless, surprising
	// side effect for a verb that never touches it.
	if rest[0] == "merge" {
		return dispatch(verbs, nil, dl, jsonOut, rest, out, errOut)
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

	return dispatch(verbs, kb, dl, jsonOut, rest, out, errOut)
}

// openDebugLogIfRequested opens a new DebugLog and announces its path to
// errOut when debugOn is set; returns (nil, nil) otherwise. Skipped
// entirely for the help/no-op paths above, which return before this is
// ever called.
func openDebugLogIfRequested(debugOn bool, errOut io.Writer) (*DebugLog, error) {
	if !debugOn {
		return nil, nil
	}
	dl, err := NewDebugLog(DefaultDebugLogPath())
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(errOut, "Debug log: %s\n", dl.Path())
	return dl, nil
}

// parseGlobalFlags consumes leading --db/--json/--debug flags from args
// and returns them along with the remaining, unconsumed arguments (the
// verb and its own arguments/flags).
func parseGlobalFlags(args []string) (dbPath string, jsonOut bool, debugOn bool, rest []string, err error) {
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--db":
			if i+1 >= len(args) {
				return "", false, false, nil, fmt.Errorf("--db requires a path argument")
			}
			dbPath = args[i+1]
			i += 2
		case "--json":
			jsonOut = true
			i++
		case "--debug":
			debugOn = true
			i++
		default:
			return dbPath, jsonOut, debugOn, args[i:], nil
		}
	}
	return dbPath, jsonOut, debugOn, args[i:], nil
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
