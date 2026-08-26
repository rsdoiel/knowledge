package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	knowledge "github.com/rsdoiel/knowledge"
)

// recordFmtSummary is what one `kb record fmt` run reports.
type recordFmtSummary struct {
	Path      string   `json:"path"`
	DryRun    bool     `json:"dry_run"`
	Changed   int      `json:"changed"`
	Unchanged int      `json:"unchanged"`
	Failed    int      `json:"failed"`
	Files     []string `json:"files,omitempty"`
	Errors    []string `json:"errors,omitempty"`
}

// slugMaxLen bounds a generated filename slug, per the format's "truncated to
// roughly 50 characters" guidance. The slug is cosmetic and may be regenerated
// at any time without breaking a reference; the id is the identity.
const slugMaxLen = 50

/** recordFmt rewrites every record under a path into canonical form, and is
 * the normalisation mechanism `kb ingest` deliberately does not provide:
 * ingest never writes to a record file, so bringing a corpus into line needs
 * its own verb.
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — the open knowledge base, unused except
 *                                        for reporting; fmt touches files only.
 *   jsonOut (bool)                     — emit the summary as JSON.
 *   f       (recordFlags)              — PATH plus --dry-run.
 *   out     (io.Writer)                — where the summary is written.
 *
 * Returns:
 *   error — on a usage error or an unreadable tree. An unparsable file is
 *           reported in the summary, not returned.
 *
 * Example:
 *   err := recordFmt(kb, false, recordFlags{args: []string{"CMTools/decisions"}}, os.Stdout)
 */
func recordFmt(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer) error {
	if len(f.args) < 1 {
		return fmt.Errorf("record fmt requires a PATH")
	}
	dir, err := filepath.Abs(f.args[0])
	if err != nil {
		return err
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", f.args[0])
	}

	files, err := collectRecordFiles(dir)
	if err != nil {
		return err
	}
	summary := recordFmtSummary{Path: dir, DryRun: f.dryRun}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, err.Error())
			continue
		}
		rf, err := knowledge.ParseRecord(raw, path)
		if err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, err.Error())
			continue
		}
		rendered, err := knowledge.RenderRecordFile(rf)
		if err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, err.Error())
			continue
		}
		if string(rendered) == string(raw) {
			summary.Unchanged++
			continue
		}
		summary.Changed++
		summary.Files = append(summary.Files, filepath.Base(path))
		if f.dryRun {
			continue
		}
		if err := os.WriteFile(path, rendered, 0o644); err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, err.Error())
		}
	}

	if jsonOut {
		return printJSON(out, summary)
	}
	if summary.DryRun {
		fmt.Fprintln(out, "dry run, nothing written")
	}
	fmt.Fprintf(out, "%d changed, %d already canonical, %d failed\n",
		summary.Changed, summary.Unchanged, summary.Failed)
	for _, name := range summary.Files {
		fmt.Fprintf(out, "  changed: %s\n", name)
	}
	for _, e := range summary.Errors {
		fmt.Fprintf(out, "  error: %s\n", e)
	}
	return nil
}

/** recordNew scaffolds a new decision record: it allocates the next id for the
 * tier, fills the fields a tool owns, sets status to proposed, and prints all
 * five body headings whether or not they get filled.
 *
 * The record is left `proposed` deliberately. A model may write a record; only
 * the author accepts one. `--trigger` is required here even though a converted
 * record may carry an empty one, because on a newly authored record it is both
 * cheap and accurate to state where the need was discovered.
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — used to locate the workspace root.
 *   jsonOut (bool)                     — emit the result as JSON.
 *   f       (recordFlags)              — --project/--workspace, --title,
 *                                        --trigger, optional --kind, --root.
 *   out     (io.Writer)                — where the result is written.
 *
 * Returns:
 *   error — on a usage error or a failed write.
 *
 * Example:
 *   err := recordNew(kb, false, flags, os.Stdout) // writes decisions/0170-....md
 */
func recordNew(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer) error {
	if f.title == "" {
		return fmt.Errorf("record new requires --title")
	}
	if f.trigger == "" {
		return fmt.Errorf("record new requires --trigger; the empty-trigger concession is for converted records only")
	}
	if f.project == "" && !f.workspace {
		return fmt.Errorf("record new requires --project P or --workspace")
	}

	scope := "project"
	dir := filepath.Join(f.project, "decisions")
	if f.workspace {
		scope = "workspace"
		dir = filepath.Join("agents", "decisions")
	}
	root := recordRoot(kb, f)
	absDir := filepath.Join(root, dir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", absDir, err)
	}

	id, err := nextRecordID(absDir)
	if err != nil {
		return err
	}
	kind := f.kind
	if kind == "" {
		kind = "decision"
	}
	host, _ := os.Hostname()
	uuidValue, err := knowledge.NewUUID()
	if err != nil {
		return err
	}

	rf := &knowledge.RecordFile{
		Record: knowledge.Record{
			RecordID:   id,
			Scope:      scope,
			Title:      f.title,
			Date:       knowledge.Today(),
			Status:     "proposed",
			Kind:       kind,
			Trigger:    f.trigger,
			Body:       scaffoldBody,
			UUID:       uuidValue,
			OriginHost: host,
			Workspace:  filepath.Base(root),
		},
		ProjectName: f.project,
	}
	rendered, err := knowledge.RenderRecordFile(rf)
	if err != nil {
		return err
	}

	name := id + "-" + slugify(f.title) + ".md"
	path := filepath.Join(absDir, name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	if err := os.WriteFile(path, rendered, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	result := map[string]any{
		"record_id": id, "path": filepath.Join(dir, name),
		"status": "proposed", "scope": scope,
	}
	if jsonOut {
		return printJSON(out, result)
	}
	fmt.Fprintf(out, "DR-%s written to %s (proposed)\n", id, filepath.Join(dir, name))
	return nil
}

// scaffoldBody is the five recommended headings, printed whether or not they
// get filled: an empty heading is a cheaper prompt than discipline.
const scaffoldBody = `
**Context.**

**Decision.**

**Rationale.**

**Rejected alternatives.**

**Consequences.**
`

// nextRecordID returns the next free zero-padded id in a decisions directory,
// one past the highest already present.
func nextRecordID(dir string) (string, error) {
	files, err := collectRecordFiles(dir)
	if err != nil {
		return "", err
	}
	highest := 0
	for _, path := range files {
		name := filepath.Base(path)
		n := 0
		for _, r := range name[:4] {
			n = n*10 + int(r-'0')
		}
		if n > highest {
			highest = n
		}
	}
	return fmt.Sprintf("%04d", highest+1), nil
}

// slugify derives a filename slug from a title: lowercased, punctuation and
// backticks stripped, dashes for spaces, truncated at a word boundary.
func slugify(title string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash && b.Len() > 0:
			b.WriteRune('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) <= slugMaxLen {
		return slug
	}
	slug = slug[:slugMaxLen]
	if i := strings.LastIndex(slug, "-"); i > 0 {
		slug = slug[:i]
	}
	return strings.Trim(slug, "-")
}
