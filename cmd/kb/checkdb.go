package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["check-db"] = cmdCheckDB
}

// maxListed caps how many row labels check-db prints per table and set; the
// counts are always complete.
const maxListed = 20

// jsonlPathFor is the dump kept beside a database: the same path with .jsonl
// in place of .db. Nothing else is searched for, since agents/ can hold
// several dumps and a wrong guess gives a confident wrong verdict.
func jsonlPathFor(dbPath string) string {
	return strings.TrimSuffix(dbPath, ".db") + ".jsonl"
}

// cmdCheckDB implements `kb check-db [--jsonl FILE]` (DR-0053): a read-only
// comparison of the database with a JSONL dump by full content. It never
// decides by file time; the times are printed as a hint only, since a git
// pull refreshes a file's time without changing its content. Exit 0 when the
// two are in sync, 1 when not (a check that reports drift), 66 for a missing
// dump and 65 for a malformed one. The report goes to stdout either way.
func cmdCheckDB(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	const usage = "usage: check-db [--jsonl FILE]"
	var jsonlArg string
	positional, err := splitFlags(args, map[string]*string{"--jsonl": &jsonlArg}, nil)
	if errors.Is(err, flag.ErrHelp) {
		return err
	}
	if err != nil {
		return usageErrorf("%v; %s", err, usage)
	}
	if len(positional) != 0 {
		return usageErrorf("%s", usage)
	}
	dbPath := kb.Path()
	dumpPath := jsonlArg
	if dumpPath == "" {
		dumpPath = jsonlPathFor(dbPath)
	}

	f, err := os.Open(dumpPath)
	if err != nil {
		if os.IsNotExist(err) {
			return noInputf("no JSONL dump at %s (name one with --jsonl, or kb export to create it)", dumpPath)
		}
		return err
	}
	defer f.Close()

	c, err := logKBCall(dl, "CompareToJSONL", map[string]any{"jsonl": dumpPath}, func() (knowledge.DBComparison, error) {
		return kb.CompareToJSONL(f)
	})
	if err != nil {
		if errors.Is(err, knowledge.ErrInvalid) {
			return classedAs(classData, fmt.Errorf("%s: %w", dumpPath, err))
		}
		return err
	}
	rec := c.Recommendation()
	hint := fileTimeHint(dbPath, dumpPath)

	if jsonOut {
		type table struct {
			Table    string                    `json:"table"`
			Database int                       `json:"database"`
			JSONL    int                       `json:"jsonl"`
			OnlyDB   []string                  `json:"only_in_database"`
			OnlyDump []string                  `json:"only_in_jsonl"`
			Differ   []knowledge.RowDifference `json:"different"`
		}
		var tables []table
		for _, d := range c.Tables {
			t := table{d.Table, d.CountA, d.CountB, d.OnlyA, d.OnlyB, d.Different}
			if t.OnlyDB == nil {
				t.OnlyDB = []string{}
			}
			if t.OnlyDump == nil {
				t.OnlyDump = []string{}
			}
			if t.Differ == nil {
				t.Differ = []knowledge.RowDifference{}
			}
			tables = append(tables, t)
		}
		collisions := c.Collisions
		if collisions == nil {
			collisions = []knowledge.IdentityCollision{}
		}
		if err := printJSON(out, map[string]any{
			"in_sync": c.InSync(), "recommendation": rec, "database": dbPath, "jsonl": dumpPath,
			"tables": tables, "collisions": collisions, "file_times": hint,
		}); err != nil {
			return err
		}
		return driftResult(c)
	}

	fmt.Fprintf(out, "database: %s\njsonl:    %s\n\n", dbPath, dumpPath)
	fmt.Fprintf(out, "%-26s %8s %8s\n", "table", "database", "jsonl")
	for _, d := range c.Tables {
		mark := ""
		if !d.Clean() {
			mark = "  *"
		}
		fmt.Fprintf(out, "%-26s %8d %8d%s\n", d.Table, d.CountA, d.CountB, mark)
	}
	listed := func(title string, pick func(knowledge.TableDiff) []string) {
		printed := false
		for _, d := range c.Tables {
			labels := pick(d)
			if len(labels) == 0 {
				continue
			}
			if !printed {
				fmt.Fprintf(out, "\n%s\n", title)
				printed = true
			}
			fmt.Fprintf(out, "  %s (%d)\n", d.Table, len(labels))
			for i, l := range labels {
				if i == maxListed {
					fmt.Fprintf(out, "    ... and %d more\n", len(labels)-maxListed)
					break
				}
				fmt.Fprintf(out, "    %s\n", l)
			}
		}
	}
	listed("in the JSONL, not in this database (never received, or deleted here):", func(d knowledge.TableDiff) []string { return d.OnlyB })
	listed("in this database, not in the JSONL:", func(d knowledge.TableDiff) []string { return d.OnlyA })
	printed := false
	for _, d := range c.Tables {
		for i, r := range d.Different {
			if !printed {
				fmt.Fprintln(out, "\nin both, but different:")
				printed = true
			}
			if i == 0 {
				fmt.Fprintf(out, "  %s (%d)\n", d.Table, len(d.Different))
			}
			if i < maxListed {
				fmt.Fprintf(out, "    %s: database %q, jsonl %q\n", r.Label, r.A, r.B)
			}
		}
	}
	if len(c.Collisions) > 0 {
		fmt.Fprintf(out, "\n%d entity(ies) exist on both sides under different uuids:\n", len(c.Collisions))
		for _, k := range c.Collisions {
			fmt.Fprintf(out, "  %-10s %s\n", k.Table, k.Label)
		}
	}
	fmt.Fprintln(out)
	switch rec {
	case "in-sync":
		fmt.Fprintln(out, "in sync: the database and the JSONL hold the same content.")
	case "import":
		fmt.Fprintln(out, "recommendation: the JSONL is ahead. Run kb import to bring its rows in.")
		fmt.Fprintln(out, "warning: kb import brings back anything deleted here on purpose, since there are no tombstones.")
	case "export":
		fmt.Fprintln(out, "recommendation: the database is ahead. Run kb export to update the JSONL.")
	default:
		fmt.Fprintln(out, "recommendation: diverged. Run kb import, then kb export.")
		fmt.Fprintln(out, "warning: kb import brings back anything deleted here on purpose, since there are no tombstones.")
	}
	fmt.Fprintf(out, "file times (a hint only, never the verdict; a git pull changes a file's time, not its content): %s\n", hint)
	return driftResult(c)
}

// driftResult is nil when in sync and a negative (exit 1) otherwise, printed
// after the report so the report is not lost to the error line.
func driftResult(c knowledge.DBComparison) error {
	if c.InSync() {
		return nil
	}
	return negativef("the database and the JSONL are not in sync (%s)", c.Recommendation())
}

// fileTimeHint says which file was modified last, from modification times.
func fileTimeHint(dbPath, dumpPath string) string {
	di, derr := os.Stat(dbPath)
	ji, jerr := os.Stat(dumpPath)
	if derr != nil || jerr != nil {
		return "unavailable"
	}
	rel := "the same time"
	switch {
	case di.ModTime().After(ji.ModTime()):
		rel = "the database is newer"
	case ji.ModTime().After(di.ModTime()):
		rel = "the JSONL is newer"
	}
	return fmt.Sprintf("database %s, jsonl %s; %s", di.ModTime().Format("2006-01-02 15:04"), ji.ModTime().Format("2006-01-02 15:04"), rel)
}
