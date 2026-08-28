// kb is a command-line and interactive (TUI) interface for a
// github.com/rsdoiel/knowledge knowledge base.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	knowledge "github.com/rsdoiel/knowledge"
)

// appName is the name the help text and version line report themselves under.
const appName = "kb"

func main() {
	os.Exit(mainRun(os.Args[1:], os.Stdout, os.Stderr))
}

// mainRun is main's testable body: parses the standard and global options,
// answers the informational ones without opening anything, then dispatches to
// the matched verb. It returns an exit code rather than calling os.Exit so
// that tests can drive the whole path.
func mainRun(args []string, out, errOut io.Writer) int {
	opts, rest, err := parseGlobalFlags(args)
	switch {
	case errors.Is(err, flag.ErrHelp):
		// -h is deliberately not declared, so flag reports ErrHelp for it.
		// Answer with the real help page rather than the FlagSet's usage.
		printHelp(out, "")
		return 0
	case err != nil:
		fmt.Fprintf(errOut, "kb: %v\n", err)
		return 2
	}

	// The standard options every CLI here supports, answered before any real
	// work and without touching a database. Their content comes from
	// version.go, which cmt regenerates from codemeta.json, so what the CLI
	// reports and what the release says cannot drift apart.
	if opts.showHelp {
		printHelp(out, "")
		return 0
	}
	if opts.showLicense {
		fmt.Fprintf(out, "%s\n", knowledge.LicenseText)
		return 0
	}
	if opts.showVersion {
		fmt.Fprintf(out, "%s %s %s\n", appName, knowledge.Version, knowledge.ReleaseHash)
		return 0
	}
	dbPath, jsonOut, debugOn := opts.dbPath, opts.jsonOut, opts.debugOn

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
	// help is also a verb, per the git/go convention, and takes an optional
	// topic. kb -help TOPIC reaches the same text by way of the option.
	if rest[0] == "help" {
		topic := ""
		if len(rest) > 1 {
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

	// merge, index and init never touch the ambient --db database: merge
	// operates on explicit -a/-b/-out paths, index builds from the record
	// files so it works in a checkout that has never been ingested (DR-0008),
	// and init resolves and creates its own target path (DR-0021). Opening --
	// and so auto-creating -- an unrelated ./agents/knowledge.db for any of
	// them would be a pointless, surprising side effect. It is not
	// hypothetical: kb index left a 127KB database in whatever directory it
	// ran in.
	if rest[0] == "merge" || rest[0] == "index" || rest[0] == "init" {
		return dispatch(verbs, nil, dl, jsonOut, rest, out, errOut)
	}

	resolvedPath, err := resolveDBPath(dbPath)
	if err != nil {
		fmt.Fprintf(errOut, "kb: %v\n", err)
		return 1
	}

	// The ambient-open guard (DR-0021 item 4, narrowed by DR-0022): a verb
	// resolved through the true ambient default -- dbPath == "", no --db was
	// given at all -- must not silently create a workspace wherever it
	// happens to be run from. An explicit --db PATH is the opposite case, the
	// caller said exactly where to open, so it keeps today's open-or-create
	// behavior unconditionally. import is carved out here too: unlike
	// merge/index/init it has no path handling of its own and depends
	// entirely on this branch for its create-capability, which the
	// workspace:DR-0002 rebuild recipe (rm agents/knowledge.db && kb import
	// -in agents/knowledge.jsonl) relies on.
	if dbPath == "" && rest[0] != "import" {
		if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
			fmt.Fprintf(errOut, "kb: no %s here; run \"kb init\" to start a new workspace, or \"kb import -in FILE\" to rebuild one from an export\n", resolvedPath)
			return 1
		}
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

/** globalOptions holds the options that apply before any verb: the three
 * standard ones every CLI in this workspace supports, plus kb's own.
 *
 * Fields:
 *   showHelp    (bool)   — print the help page and exit.
 *   showLicense (bool)   — print the license and exit.
 *   showVersion (bool)   — print name, version and release hash, and exit.
 *   dbPath      (string) — knowledge base to open; "" means the ambient one.
 *   jsonOut     (bool)   — emit JSON rather than human-readable text.
 *   debugOn     (bool)   — write a JSONL trace of every call.
 */
type globalOptions struct {
	showHelp    bool
	showLicense bool
	showVersion bool
	dbPath      string
	jsonOut     bool
	debugOn     bool
}

/** parseGlobalFlags reads the options preceding the verb and returns them
 * along with the unconsumed remainder — the verb and its own arguments.
 *
 * It uses a flag.FlagSet rather than a hand-rolled loop so that each option is
 * accepted in both dash forms (-help and --help) without writing the variants
 * out, which is how the other Go CLIs here declare theirs. flag stops at the
 * first non-flag argument, so a verb's own flags pass through untouched: in
 * `kb --json ingest DIR --dry-run`, --json is consumed here and --dry-run
 * reaches ingest.
 *
 * -h is deliberately not declared. Leaving it undefined makes flag return
 * ErrHelp, which the caller answers with the real help page; declaring it
 * would instead print the FlagSet's terse usage over the Pandoc help text.
 *
 * Parameters:
 *   args ([]string) — the arguments after the program name.
 *
 * Returns:
 *   globalOptions — the parsed options.
 *   []string      — the verb and its arguments.
 *   error         — flag.ErrHelp for -h, or a usage error.
 *
 * Example:
 *   opts, rest, err := parseGlobalFlags([]string{"--json", "project", "list"})
 *   // opts.jsonOut == true, rest == []string{"project", "list"}
 */
func parseGlobalFlags(args []string) (globalOptions, []string, error) {
	var opts globalOptions
	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	// Silence the FlagSet's own usage: kb answers with its own help text.
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	// Standard Options
	fs.BoolVar(&opts.showHelp, "help", false, "display help")
	fs.BoolVar(&opts.showLicense, "license", false, "display license")
	fs.BoolVar(&opts.showVersion, "version", false, "display version")

	fs.StringVar(&opts.dbPath, "db", "", "path to the knowledge base to open")
	fs.BoolVar(&opts.jsonOut, "json", false, "emit JSON instead of human-readable text")
	fs.BoolVar(&opts.debugOn, "debug", false, "write a JSONL debug trace")

	if err := fs.Parse(args); err != nil {
		return opts, nil, err
	}
	return opts, fs.Args(), nil
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
