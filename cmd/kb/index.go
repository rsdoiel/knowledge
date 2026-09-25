package main

import (
	"fmt"
	"io"
	"io/fs"
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

/** cmdIndex implements `kb index PATH [--stdout|--check]`: it regenerates
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
 * --check never writes. It compares PATH/index.md against a fresh render and
 * reports drift as an error instead — see TODO.md "a mechanism for knowing
 * when index.md needs regenerating", where nothing noticed index.md silently
 * falling out of sync with `status`/`kind`/`trigger`/`superseded_by`/title
 * changes made through `kb record set-status`/`supersede`. Exit code follows
 * kb search's convention: 0 when there is nothing to report, 1 when there is.
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — unused; the index is built from the
 *                                        files, so it works before any ingest.
 *   dl      (*DebugLog)                — debug log, may be nil.
 *   jsonOut (bool)                     — unused; the index has one format.
 *   args    ([]string)                 — PATH plus optional --stdout or --check.
 *   out     (io.Writer)                — where --stdout writes, and where the
 *                                        confirmation line goes otherwise.
 *
 * Returns:
 *   error — on a usage error, an unreadable tree, a malformed record, or
 *           (under --check) a missing or stale index.md. A record that
 *           cannot be parsed is fatal here, unlike in ingest: silently
 *           dropping one from the index would make the index lie about what
 *           the corpus contains.
 *
 * Example:
 *   err := cmdIndex(kb, nil, false, []string{"clasm/decisions"}, os.Stdout)
 */
func cmdIndex(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	var dir string
	toStdout := false
	check := false
	all := false
	for _, arg := range args {
		switch arg {
		case "--stdout", "-stdout":
			toStdout = true
		case "--check", "-check":
			check = true
		case "--all", "-all":
			all = true
		default:
			if strings.HasPrefix(arg, "-") {
				return usageErrorf("unknown flag %q", arg)
			}
			if dir != "" {
				return usageErrorf("index takes a single PATH, got %q and %q", dir, arg)
			}
			dir = arg
		}
	}
	if all {
		if dir == "" {
			return usageErrorf("index --all requires a ROOT; see kb help index")
		}
		if toStdout {
			return usageErrorf("--all and --stdout cannot be combined")
		}
		return cmdIndexAll(dl, jsonOut, dir, check, out)
	}
	if dir == "" {
		return usageErrorf("index requires a PATH; see kb help index")
	}
	if check && toStdout {
		return usageErrorf("--check and --stdout cannot be combined")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return noInputf("%s is not a directory", dir)
	}

	index, count, err := renderCorpusIndex(abs)
	if err != nil {
		return err
	}
	target := filepath.Join(abs, "index.md")
	dl.Log("index", map[string]any{"path": abs, "records": count, "stdout": toStdout, "check": check})

	if check {
		return checkIndex(target, index, out)
	}
	if toStdout {
		_, err := io.WriteString(out, index)
		return err
	}
	if err := os.WriteFile(target, []byte(index), 0o644); err != nil {
		return asCreate(fmt.Errorf("writing %s: %w", target, err))
	}
	plural := "s"
	if count == 1 {
		plural = ""
	}
	fmt.Fprintf(out, "%d record%s indexed to %s\n", count, plural, target)
	return nil
}

// renderCorpusIndex reads every record file directly in dir and renders
// what its index.md should contain, alongside the record count -- the
// single-corpus rendering step shared by cmdIndex, regenerateIndexIfPresent,
// and cmdIndexAll, so all three agree on what "current" means.
func renderCorpusIndex(dir string) (index string, count int, err error) {
	files, err := collectRecordFilesIn(dir)
	if err != nil {
		return "", 0, err
	}
	records := make([]*knowledge.RecordFile, 0, len(files))
	for _, path := range files {
		rf, err := knowledge.ParseRecordFile(path)
		if err != nil {
			return "", 0, asContent(fmt.Errorf("cannot index %s: %w", filepath.Base(path), err))
		}
		records = append(records, rf)
	}
	return renderIndex(records), len(records), nil
}

// indexAllSummary is what `kb index ROOT --all` reports, in both JSON and
// text form.
type indexAllSummary struct {
	Root    string              `json:"root"`
	Check   bool                `json:"check"`
	Corpora []indexCorpusResult `json:"corpora"`
}

// indexCorpusResult is one discovered corpus's outcome. Status is one of
// "written", "up_to_date", "stale", or "error" -- never "missing", since
// discoverIndexedCorpora only ever returns directories that already have an
// index.md.
type indexCorpusResult struct {
	Dir     string `json:"dir"`
	Records int    `json:"records,omitempty"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

/** cmdIndexAll implements `kb index ROOT --all [--check]`: it discovers
 * every corpus under ROOT that already has an index.md and refreshes or
 * checks each one, continuing past an individual corpus's failure so one
 * bad corpus does not hide the rest — matching `kb record fmt`'s own
 * keep-going, summarize-at-the-end shape rather than aborting on the first
 * problem. A corpus that has never had an index.md is silently left alone,
 * the same rule regenerateIndexIfPresent follows for a single corpus.
 *
 * Parameters:
 *   dl      (*DebugLog) — debug log, may be nil.
 *   jsonOut (bool)      — emit the summary as JSON instead of one line per
 *                         corpus.
 *   root    (string)    — directory to search under for indexed corpora.
 *   check   (bool)      — report drift instead of writing.
 *   out     (io.Writer) — where the summary is written.
 *
 * Returns:
 *   error — on a usage error or an unreadable root, or when any discovered
 *           corpus errored or (under --check) drifted — so a pre-commit
 *           hook can gate on the whole workspace in one call.
 *
 * Example:
 *   err := cmdIndexAll(nil, false, "agents", true, os.Stdout)
 */
func cmdIndexAll(dl *DebugLog, jsonOut bool, root string, check bool, out io.Writer) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return noInputf("%s is not a directory", root)
	}
	dirs, err := discoverIndexedCorpora(abs)
	if err != nil {
		return err
	}

	summary := indexAllSummary{Root: abs, Check: check}
	needsAttention := 0
	// firstFail is the first corpus that could not be indexed at all. A stale
	// index is a normal negative answer; a failure outranks it (X3, DR-0047).
	var firstFail error
	noteFailure := func(err error) {
		if firstFail == nil {
			firstFail = err
		}
	}
	for _, dir := range dirs {
		r := indexCorpusResult{Dir: dir}
		index, count, err := renderCorpusIndex(dir)
		if err != nil {
			r.Status, r.Detail = "error", err.Error()
			summary.Corpora = append(summary.Corpora, r)
			needsAttention++
			noteFailure(asContent(err))
			continue
		}
		r.Records = count
		target := filepath.Join(dir, "index.md")
		if check {
			got, rerr := os.ReadFile(target)
			switch {
			case rerr != nil:
				r.Status, r.Detail = "error", rerr.Error()
				needsAttention++
				noteFailure(rerr)
			case string(got) != index:
				r.Status = "stale"
				needsAttention++
			default:
				r.Status = "up_to_date"
			}
			summary.Corpora = append(summary.Corpora, r)
			continue
		}
		if err := os.WriteFile(target, []byte(index), 0o644); err != nil {
			r.Status, r.Detail = "error", err.Error()
			summary.Corpora = append(summary.Corpora, r)
			needsAttention++
			noteFailure(asCreate(err))
			continue
		}
		r.Status = "written"
		summary.Corpora = append(summary.Corpora, r)
	}

	dl.Log("index_all", map[string]any{"root": abs, "check": check, "corpora": len(summary.Corpora), "needs_attention": needsAttention})

	if jsonOut {
		if err := printJSON(out, summary); err != nil {
			return err
		}
	} else {
		writeIndexAllText(out, summary)
	}
	if firstFail != nil {
		return fmt.Errorf("%d of %d corpora need attention; see above: %w", needsAttention, len(summary.Corpora), firstFail)
	}
	if needsAttention > 0 {
		return negativef("%d of %d corpora need attention; see above", needsAttention, len(summary.Corpora))
	}
	return nil
}

// discoverIndexedCorpora walks root and returns every directory that
// already contains an index.md, sorted. Keying off the file's actual
// presence, rather than grouping record files by directory, is what keeps a
// nested corpus from ever being folded into its parent's: each directory
// with its own index.md is found and reported independently, exactly
// mirroring how kb index itself treats one directory as one corpus
// (collectRecordFilesIn does not recurse, for the same reason).
//
// Hidden directories are not descended into. A git worktree keeps a whole
// second copy of the tree under .claude/worktrees/<name>/ — corpus and
// generated index.md included — so it satisfies both signals below and was
// reported as a corpus in its own right: a duplicate under --check, and in
// write mode an edit to a throwaway worktree rather than the real tree.
// Found running this live against a real workspace. The prune applies only
// to directories descended into, never to root itself, so naming a hidden
// directory as the root still works.
func discoverIndexedCorpora(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if p != root && strings.HasPrefix(d.Name(), ".") {
			return fs.SkipDir
		}
		if !looksLikeGeneratedIndex(filepath.Join(p, "index.md")) {
			return nil
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !e.IsDir() && recordFilePattern.MatchString(e.Name()) {
				dirs = append(dirs, p)
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", root, err)
	}
	sort.Strings(dirs)
	return dirs, nil
}

// looksLikeGeneratedIndex reports whether path exists and opens with this
// format's own heading -- the cheapest signal that it is a kb-generated
// index.md rather than some other file that happens to share the name.
// Required alongside the record-file check in discoverIndexedCorpora, not
// instead of it: an index.md can exist for a reason that has nothing to do
// with kb (a docs site, a blog front page) in a directory --all has no
// business touching, found running it live against a real, larger tree —
// filename alone reported several such files as corpora, and a naive
// content check alone would still touch a directory whose real corpus
// files had all been deleted out from under a stale, coincidentally
// matching index.md.
func looksLikeGeneratedIndex(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.HasPrefix(string(data), "# Decision Records — index")
}

// writeIndexAllText prints the --all summary in human-readable form, one
// line per corpus plus a final count.
func writeIndexAllText(out io.Writer, s indexAllSummary) {
	for _, c := range s.Corpora {
		switch c.Status {
		case "written":
			plural := "s"
			if c.Records == 1 {
				plural = ""
			}
			fmt.Fprintf(out, "%s: %d record%s indexed\n", c.Dir, c.Records, plural)
		case "up_to_date":
			fmt.Fprintf(out, "%s: up to date\n", c.Dir)
		case "stale":
			fmt.Fprintf(out, "%s: stale; run kb index %s to regenerate it\n", c.Dir, c.Dir)
		case "error":
			fmt.Fprintf(out, "%s: error: %s\n", c.Dir, c.Detail)
		}
	}
	word := "corpora"
	if len(s.Corpora) == 1 {
		word = "corpus"
	}
	fmt.Fprintf(out, "%d %s processed\n", len(s.Corpora), word)
}

// checkIndex compares a freshly rendered index against what is on disk at
// target, without ever writing to it. Drift is reported as an error rather
// than fixed, matching kb search's convention: --check is meant to gate a
// pre-commit hook or CI step, where a non-zero exit is the signal and
// "kb index PATH" is the documented remedy.
func checkIndex(target, want string, out io.Writer) error {
	got, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return negativef("%s does not exist; run kb index to generate it", target)
		}
		return err
	}
	if string(got) != want {
		return negativef("%s is stale; run kb index to regenerate it", target)
	}
	fmt.Fprintf(out, "%s is up to date\n", target)
	return nil
}

// regenerateIndexIfPresent refreshes dir/index.md in place if one already
// exists there, and does nothing otherwise — it never creates one, so a
// corpus that has not opted into the generated-index convention does not
// suddenly get a file it never asked for.
//
// record set-status and supersede call this after a successful write, to
// close the staleness TODO.md's index-regeneration item describes at its
// source rather than leaving --check as the only way to notice it: status,
// kind, trigger, superseded_by and title are exactly the fields those two
// commands change, and exactly what the index renders.
//
// A failure here is reported but not fatal — the record file and database
// write already succeeded, and index.md is a derived, always-regenerable
// artifact; refusing the whole operation over it would be a worse failure
// mode than a stale index the next `kb index` or `--check` will still catch.
func regenerateIndexIfPresent(dir string) error {
	target := filepath.Join(dir, "index.md")
	if _, err := os.Stat(target); err != nil {
		return nil
	}
	index, _, err := renderCorpusIndex(dir)
	if err != nil {
		return asContent(fmt.Errorf("cannot regenerate index: %w", err))
	}
	if err := os.WriteFile(target, []byte(index), 0o644); err != nil {
		return asCreate(fmt.Errorf("writing %s: %w", target, err))
	}
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
