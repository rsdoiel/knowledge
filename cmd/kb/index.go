package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["index"] = cmdIndex
}

// Index column widths. Every column always holds a value: an empty one renders
// as the placeholder, never as spaces, because awk's default separator is a run
// of whitespace and a space-padded column is not a field at all — the next
// column would silently take its position, so no field number would reliably
// mean "title". With the placeholder the title always starts at $7.
const (
	indexStatusWidth  = 11
	indexKindWidth    = 11
	indexTriggerWidth = 15
	indexFlagWidth    = 4
	indexEmpty        = "-"
	indexSupersededOn = "sup"
)

/** cmdIndex implements `kb index PATH [--stdout]`: it regenerates
 * PATH/index.md, one greppable line per record, newest first.
 *
 * Newest-first and one-line-per-record preserve the affordance a single
 * top-inserted DECISIONS.md had: head, grep and awk reach the recent and the
 * relevant without reading the whole corpus. The index is what stays loadable
 * as the corpus grows; records are then read selectively.
 *
 * The file is generated and never hand-edited. It is also the only thing this
 * command writes — the format has no decisions/README.md, so one is never
 * created.
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — unused; the index is built from the
 *                                        files, so it works before any ingest.
 *   dl      (*DebugLog)                — debug log, may be nil.
 *   jsonOut (bool)                     — unused; the index has one format.
 *   args    ([]string)                 — PATH plus optional --stdout.
 *   out     (io.Writer)                — where --stdout writes, and where the
 *                                        confirmation line goes otherwise.
 *
 * Returns:
 *   error — on a usage error, an unreadable tree, or a malformed record. A
 *           record that cannot be parsed is fatal here, unlike in ingest:
 *           silently dropping one from the index would make the index lie
 *           about what the corpus contains.
 *
 * Example:
 *   err := cmdIndex(kb, nil, false, []string{"clasm/decisions"}, os.Stdout)
 */
func cmdIndex(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	var dir string
	toStdout := false
	for _, arg := range args {
		switch arg {
		case "--stdout", "-stdout":
			toStdout = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown flag %q", arg)
			}
			if dir != "" {
				return fmt.Errorf("index takes a single PATH, got %q and %q", dir, arg)
			}
			dir = arg
		}
	}
	if dir == "" {
		return fmt.Errorf("index requires a PATH; see kb help index")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	files, err := collectRecordFilesIn(abs)
	if err != nil {
		return err
	}
	records := make([]*knowledge.RecordFile, 0, len(files))
	for _, path := range files {
		rf, err := knowledge.ParseRecordFile(path)
		if err != nil {
			return fmt.Errorf("cannot index %s: %w", filepath.Base(path), err)
		}
		records = append(records, rf)
	}

	index := renderIndex(records)
	dl.Log("index", map[string]any{"path": abs, "records": len(records), "stdout": toStdout})

	if toStdout {
		_, err := io.WriteString(out, index)
		return err
	}
	target := filepath.Join(abs, "index.md")
	if err := os.WriteFile(target, []byte(index), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", target, err)
	}
	plural := "s"
	if len(records) == 1 {
		plural = ""
	}
	fmt.Fprintf(out, "%d record%s indexed to %s\n", len(records), plural, target)
	return nil
}

// renderIndex builds the whole index file: a heading, a tool-neutral
// attribution, and a fenced block of one row per record, newest first.
//
// The attribution names no tool. More than one generator has existed for this
// format, and a file naming one of them cannot be reproduced byte-for-byte by
// another without asserting something false about itself.
func renderIndex(records []*knowledge.RecordFile) string {
	sorted := make([]*knowledge.RecordFile, len(records))
	copy(sorted, records)
	// Newest first: date descending, then record id descending. Never id
	// alone — ids are identity, not chronology, so within one date a
	// correction can carry a lower id than the record it supersedes.
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i].Record, sorted[j].Record
		if a.Date != b.Date {
			return a.Date > b.Date
		}
		return a.RecordID > b.RecordID
	})

	rows := make([]string, 0, len(sorted))
	for _, rf := range sorted {
		// The flag fires on a non-empty superseded_by regardless of status.
		// That makes it redundant for a wholly superseded record, whose status
		// column already says so, but one rule with no conditional is easier
		// to rely on. Its real work is the partial case, where a record stays
		// accepted because most of its episode still stands.
		flag := ""
		if len(rf.SupersededBy) > 0 {
			flag = indexSupersededOn
		}
		rows = append(rows, strings.Join([]string{
			"DR-" + rf.Record.RecordID,
			rf.Record.Date,
			indexCol(rf.Record.Status, indexStatusWidth),
			indexCol(rf.Record.Kind, indexKindWidth),
			indexCol(rf.Record.Trigger, indexTriggerWidth),
			indexCol(flag, indexFlagWidth),
			rf.Record.Title,
		}, "  "))
	}

	return "# Decision Records — index\n\n" +
		"Generated file. Do not hand-edit.\n\n" +
		"```\n" + strings.Join(rows, "\n") + "\n```\n"
}

// indexCol pads a column to width, substituting the placeholder for an empty
// value so that every column remains an addressable awk field.
func indexCol(value string, width int) string {
	if value == "" {
		value = indexEmpty
	}
	for len(value) < width {
		value += " "
	}
	return value
}
