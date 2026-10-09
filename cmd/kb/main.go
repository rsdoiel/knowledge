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
	"strings"

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
			return failWith(errOut, jsonOut, asCreate(fmt.Errorf("opening debug log: %w", err)))
		}
		defer dl.Close()
		resolvedPath, err := resolveDBPath(dbPath)
		if err != nil {
			return failWith(errOut, jsonOut, err)
		}
		noteWorkspaceAbove(errOut, dbPath, resolvedPath)
		kb, err := knowledge.Open(resolvedPath)
		if err != nil {
			return failWith(errOut, jsonOut, fmt.Errorf("open %s: %w", resolvedPath, err))
		}
		defer kb.Close()
		if err := runTUI(kb, dl); err != nil {
			return failWith(errOut, jsonOut, err)
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
	if wantsVerbHelp(rest) {
		if !printHelp(out, rest[0]) {
			fmt.Fprintf(errOut, "kb: unknown verb %q\n", rest[0])
			return 2
		}
		return 0
	}

	dl, err := openDebugLogIfRequested(debugOn, errOut)
	if err != nil {
		return failWith(errOut, jsonOut, asCreate(fmt.Errorf("opening debug log: %w", err)))
	}
	defer dl.Close()

	// merge, index, init and completion never touch the ambient --db database: merge
	// operates on explicit -a/-b/-out paths, index builds from the record
	// files so it works in a checkout that has never been ingested (DR-0008),
	// and init resolves and creates its own target path (DR-0021). Opening --
	// and so auto-creating -- an unrelated ./agents/knowledge.db for any of
	// them would be a pointless, surprising side effect. It is not
	// hypothetical: kb index left a 127KB database in whatever directory it
	// ran in.
	if rest[0] == "merge" || rest[0] == "index" || rest[0] == "init" || rest[0] == "completion" {
		// They do not use --db, so an explicit one is a mistake. It used to be
		// dropped silently: `kb --db rt.db init` created ./agents/knowledge.db
		// and never rt.db. Refuse it and say where the target really goes.
		if dbPath != "" {
			refusal := dbOptionRefusal(rest[0])
			printError(errOut, jsonOut, refusal)
			return exitCodeFor(refusal).Code
		}
		return dispatch(verbs, nil, dl, jsonOut, rest, out, errOut)
	}

	// KB_DB is an explicit location, like --db, which wins over it. It is read
	// only here, after the verbs that refuse --db, so a KB_DB left in the
	// environment cannot trip them.
	if dbPath == "" {
		dbPath = os.Getenv("KB_DB")
	}
	located := dbPath // "" only when the workspace is found by discovery
	resolvedPath, err := resolveDBPath(dbPath)
	if err != nil {
		return failWith(errOut, jsonOut, err)
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
			// A fresh clone has the tracked export but not the gitignored
			// database. Say how to rebuild it; kb init would start an empty
			// history beside the real one.
			if jsonl := filepath.Join(filepath.Dir(resolvedPath), knowledge.MarkerJSONL); fileExists(jsonl) {
				return failWith(errOut, jsonOut, noInputf("no %s yet, but %s exists; rebuild it with \"kb import -in %s\"", resolvedPath, jsonl, jsonl))
			}
			return failWith(errOut, jsonOut, noInputf("no %s here; run \"kb init\" to start a new workspace, or \"kb import -in FILE\" to rebuild one from an export", resolvedPath))
		}
	}

	noteWorkspaceAbove(errOut, located, resolvedPath)
	kb, err := knowledge.Open(resolvedPath)
	if err != nil {
		return failWith(errOut, jsonOut, fmt.Errorf("open %s: %w", resolvedPath, err))
	}
	defer kb.Close()

	return dispatch(verbs, kb, dl, jsonOut, rest, out, errOut)
}

/** failWith prints err as kb reports every error, plain or as the JSON envelope,
 * and returns the exit status its class calls for. mainRun uses it for the
 * failures that happen before a verb runs: the debug log, the database path, and
 * opening the database.
 *
 * Parameters:
 *   errOut  (io.Writer) — where the error goes.
 *   jsonOut (bool)      — whether --json was given.
 *   err     (error)     — the failure.
 *
 * Returns:
 *   int — the exit status for err's class.
 *
 * Example:
 *   return failWith(errOut, jsonOut, noInputf("no workspace here"))
 */
func failWith(errOut io.Writer, jsonOut bool, err error) int {
	printError(errOut, jsonOut, err)
	return exitCodeFor(err).Code
}

/** dbOptionRefusal is the usage error for a --db given to a verb that never
 * opens the ambient database: init, index or merge.
 *
 * Parameters:
 *   verb (string) — "init", "index" or "merge".
 *
 * Returns:
 *   error — a usage error saying why --db does not apply and where the
 *           verb's target is given instead.
 *
 * Example:
 *   err := dbOptionRefusal("init") // "--db does not apply to init: ..."
 */
func dbOptionRefusal(verb string) error {
	switch verb {
	case "init":
		return usageErrorf("--db does not apply to init: it creates PATH/agents/knowledge.db, so name the target as an argument, kb init PATH")
	case "index":
		return usageErrorf("--db does not apply to index: it reads record files, not a database; see kb help index")
	case "completion":
		return usageErrorf("--db does not apply to completion: it opens no database; see kb help completion")
	}
	return usageErrorf("--db does not apply to %s: it takes its databases as -a, -b and -out; see kb help %s", verb, verb)
}

// verbsWithSubverbs are the verbs whose first argument names a subverb, read
// from the verb table. Their manual page documents every subverb, so a help
// flag one level down prints the same page.
var verbsWithSubverbs = verbsTakingSubverbs()

/** isHelpFlag reports whether arg is one of the three spellings of a help
 * request: -h, -help or --help.
 *
 * Parameters:
 *   arg (string) — a single command-line argument.
 *
 * Returns:
 *   bool — true for "-h", "-help" and "--help".
 *
 * Example:
 *   isHelpFlag("--help") // true
 */
func isHelpFlag(arg string) bool {
	return arg == "-h" || arg == "-help" || arg == "--help"
}

/** wantsVerbHelp reports whether rest (the verb and its arguments) asks for the
 * verb's manual page. That is a help flag directly after the verb, or, for a
 * verb that has subverbs, directly after the subverb (`kb project add -help`),
 * or after `document review`'s own subverb. Only those positions count: past
 * them the words are a verb's free text (an observation body, a description, a
 * search term), and a "-h" there is text. A help flag that comes after other
 * flags is left to the subverb's own flag parser, whose flag.ErrHelp dispatch
 * turns into the same page.
 *
 * Parameters:
 *   rest ([]string) — the verb followed by its arguments; must be non-empty.
 *
 * Returns:
 *   bool — true when the verb's page should be printed.
 *
 * Example:
 *   wantsVerbHelp([]string{"project", "add", "-help"}) // true
 *   wantsVerbHelp([]string{"observation", "add", "--project", "p", "note", "-h"}) // false
 */
func wantsVerbHelp(rest []string) bool {
	if len(rest) > 1 && isHelpFlag(rest[1]) {
		return true
	}
	if !verbsWithSubverbs[rest[0]] {
		return false
	}
	if len(rest) > 2 && isHelpFlag(rest[2]) {
		return true
	}
	return rest[0] == "document" && len(rest) > 3 && rest[1] == "review" && isHelpFlag(rest[3])
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
// non-empty. If dbPath is "", the workspace is found by walking up from the
// current directory (knowledge.FindWorkspace, DR-0058), and the answer is
// that workspace's knowledge.DefaultPath; with no workspace above, it is
// knowledge.DefaultPath(cwd).
func resolveDBPath(dbPath string) (string, error) {
	if dbPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		if root, _, ok := knowledge.FindWorkspace(cwd); ok {
			return knowledge.DefaultPath(root), nil
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

// fileExists reports whether path names a regular file.
func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

/** noteWorkspaceAbove tells the user, on standard error, when discovery chose a
 * workspace other than the working directory (DR-0058). Walking up means a
 * verb run in a scratch directory inside a real workspace acts on that
 * workspace; the note makes that visible before anything is written. It says
 * nothing when the location was given explicitly (--db or KB_DB), when the
 * workspace is the working directory, or when the working directory cannot be
 * read, or when KB_QUIET asks for no advisory notes.
 *
 * Parameters:
 *   errOut       (io.Writer) — where the note goes.
 *   explicit     (string)    — the --db or KB_DB value, "" when discovery chose.
 *   resolvedPath (string)    — the database path resolveDBPath returned.
 *
 * Example:
 *   noteWorkspaceAbove(os.Stderr, "", "/home/me/Laboratory/agents/knowledge.db")
 *   // kb: using the workspace at /home/me/Laboratory (found above the current directory)
 */
func noteWorkspaceAbove(errOut io.Writer, explicit, resolvedPath string) {
	if explicit != "" || quietRequested() {
		return
	}
	cwd, err := os.Getwd()
	if err != nil || resolvedPath == knowledge.DefaultPath(cwd) {
		return
	}
	fmt.Fprintf(errOut, "kb: using the workspace at %s (found above the current directory)\n",
		filepath.Dir(filepath.Dir(resolvedPath)))
}

/** quietRequested reports whether KB_QUIET asks for advisory notes to be left
 * out. Any value except empty, "0" and "false" (any case) turns it on. It
 * never silences an error; it exists for tests and scripts that run in a
 * nested directory on purpose.
 *
 * Returns:
 *   bool — true when KB_QUIET is set to a "on" value.
 *
 * Example:
 *   os.Setenv("KB_QUIET", "1")
 *   quietRequested() // true
 */
func quietRequested() bool {
	switch strings.ToLower(os.Getenv("KB_QUIET")) {
	case "", "0", "false":
		return false
	}
	return true
}

// adviceOut is where advisory notes go: the standard error of the command being
// run. dispatch points it at the real errOut for the length of a verb, so a verb
// that has only an out writer can still speak on standard error; outside a
// dispatch it discards, so a test that calls a verb directly sees no stray text.
var adviceOut io.Writer = io.Discard

/** advise writes one advisory line, prefixed "kb: ", to standard error unless
 * KB_QUIET asks for none. An advisory note is never an error and never changes
 * the exit status; it goes to standard error so --json output stays parseable.
 *
 * Parameters:
 *   format (string) — the fmt format of the message, without the prefix.
 *   a      (...any) — its arguments.
 *
 * Example:
 *   advise("--project %s is deprecated; use the qualified form", "harvey")
 */
func advise(format string, a ...any) {
	if quietRequested() {
		return
	}
	fmt.Fprintf(adviceOut, "kb: "+format+"\n", a...)
}
