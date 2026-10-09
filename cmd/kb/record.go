package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["record"] = cmdRecord
}

// recordListEntry is one row of `kb record list`, and the JSON shape callers
// script against.
type recordListEntry struct {
	Ref      string `json:"ref"`
	RecordID string `json:"record_id"`
	Project  string `json:"project"`
	Scope    string `json:"scope"`
	Date     string `json:"date"`
	Status   string `json:"status"`
	Kind     string `json:"kind"`
	Trigger  string `json:"trigger"`
	Phase    string `json:"phase"`
	Title    string `json:"title"`
	Path     string `json:"path"`
}

// recordDetail is `kb record show`, adding the body and resolved relations.
type recordDetail struct {
	recordListEntry
	Supersedes   []string `json:"supersedes"`
	SupersededBy []string `json:"superseded_by"`
	RelatesTo    []string `json:"relates_to"`
	Body         string   `json:"body"`
}

// recordFlags are the options the record subverbs share.
type recordFlags struct {
	project    string
	workspace  bool
	status     string
	kind       string
	trigger    string
	initiative string
	since      string
	root       string
	dir        string
	title      string
	partial    bool
	dryRun     bool
	concept    string
	write      bool
	all        bool
	args       []string
}

/** cmdRecord implements `kb record list|show|set-status|supersede`.
 *
 * set-status and supersede write the record files as well as the database —
 * they are the only commands that do, since ingest is forbidden from touching
 * a record file. Both sides of a supersession, in both files and in the
 * database, are written together or not at all.
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — the open knowledge base.
 *   dl      (*DebugLog)                — debug log, may be nil.
 *   jsonOut (bool)                     — emit results as JSON.
 *   args    ([]string)                 — the subverb and its arguments.
 *   out     (io.Writer)                — where results are written.
 *
 * Returns:
 *   error — on a usage error, an unknown or ambiguous record id, or a failed
 *           read or write.
 *
 * Example:
 *   err := cmdRecord(kb, nil, false, []string{"list", "--project", "clasm"}, os.Stdout)
 */
func cmdRecord(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return usageErrorf("record requires a subverb: list, pending, show, new, set-status, supersede, fmt, concepts, fuzzy-tag or delete")
	}
	flags, err := parseRecordFlags(args[1:])
	if err != nil {
		return err
	}
	dl.Log("record", map[string]any{"subverb": args[0], "args": flags.args})
	adviseDeprecatedScopeFlags(args[0], flags)
	if args[0] != "fuzzy-tag" && (flags.concept != "" || flags.write) {
		return usageErrorf("--concept and --write belong to record fuzzy-tag only")
	}

	switch args[0] {
	case "list":
		return recordList(kb, jsonOut, flags, out)
	case "pending":
		return recordPending(kb, jsonOut, flags, out)
	case "show":
		return recordShow(kb, jsonOut, flags, out)
	case "set-status":
		return recordSetStatus(kb, jsonOut, flags, out)
	case "supersede":
		return recordSupersede(kb, jsonOut, flags, out)
	case "fmt":
		return recordFmt(kb, jsonOut, flags, out)
	case "new":
		return recordNew(kb, jsonOut, flags, out)
	case "concepts":
		return recordConcepts(kb, jsonOut, flags, out)
	case "delete":
		return recordDelete(kb, jsonOut, flags, out)
	case "fuzzy-tag":
		return recordFuzzyTag(kb, jsonOut, flags, out)
	default:
		return usageErrorf("unknown record subverb %q; want list, pending, show, new, set-status, supersede, fmt, concepts, fuzzy-tag or delete", args[0])
	}
}

// parseRecordFlags separates flags from positional arguments via the shared
// splitFlags (cmd/kb/flagsplit.go).
func parseRecordFlags(args []string) (recordFlags, error) {
	var f recordFlags
	strFlags := map[string]*string{
		"--project": &f.project, "--status": &f.status, "--kind": &f.kind,
		"--trigger": &f.trigger, "--initiative": &f.initiative,
		"--since": &f.since, "--root": &f.root, "--dir": &f.dir, "--title": &f.title,
		"--concept": &f.concept,
	}
	boolFlags := map[string]*bool{
		"--workspace": &f.workspace, "--partial": &f.partial, "--dry-run": &f.dryRun,
		"--write": &f.write, "--all": &f.all,
	}
	positional, err := splitFlags(args, strFlags, boolFlags)
	if err != nil {
		return f, err
	}
	f.args = positional
	return f, nil
}

// projectNames maps project id to name, for rendering a record's owner.
func projectNames(kb *knowledge.KnowledgeBase) map[int64]string {
	names := map[int64]string{}
	projects, err := kb.Projects()
	if err != nil {
		return names
	}
	for _, p := range projects {
		names[p.ID] = p.Name
	}
	return names
}

// toEntry renders a record for listing.
func toEntry(r knowledge.Record, names map[int64]string) recordListEntry {
	return recordListEntry{
		Ref:      refOf(r, names).String(),
		RecordID: r.RecordID,
		Project:  names[r.ProjectID],
		Scope:    r.Scope,
		Date:     r.Date,
		Status:   r.Status,
		Kind:     r.Kind,
		Trigger:  r.Trigger,
		Phase:    r.Phase,
		Title:    r.Title,
		Path:     r.Path,
	}
}

// recordList prints the records matching the filter flags.
func recordList(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer) error {
	return recordListWith(kb, jsonOut, f, out, "no matching records")
}

/** recordPending implements `kb record pending`: the records waiting for a
 * decision, which is every record whose status is proposed, in the scope the
 * list verb would use (arguments, the working directory, KB_PROJECT, --all)
 * and oldest first. The other list filters still apply; --status does not,
 * since the status is what "pending" means.
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — the open knowledge base.
 *   jsonOut (bool)                     — print the entries as a JSON array.
 *   f       (recordFlags)              — the parsed flags.
 *   out     (io.Writer)                — where the listing goes.
 *
 * Returns:
 *   error — a usage error for --status, otherwise as record list.
 *
 * Example:
 *   err := recordPending(kb, false, recordFlags{args: []string{"harvey"}}, os.Stdout)
 */
func recordPending(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer) error {
	if f.status != "" {
		return usageErrorf("record pending lists records whose status is proposed; --status cannot change that (use record list --status)")
	}
	f.status = "proposed"
	return recordListWith(kb, jsonOut, f, out, "no pending records")
}

// recordListWith is record list with the line printed when nothing matches.
func recordListWith(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer, emptyMessage string) error {
	if err := validateRecordFilters(kb, f); err != nil {
		return err
	}
	scopes, err := recordScopes(kb, f)
	if err != nil {
		return err
	}
	base := knowledge.RecordFilter{
		Status: f.status, Kind: f.kind,
		Trigger: f.trigger, Initiative: f.initiative, Since: f.since,
	}
	records, err := listRecordsIn(kb, base, scopes)
	if err != nil {
		return err
	}
	names := projectNames(kb)
	entries := make([]recordListEntry, 0, len(records))
	for _, r := range records {
		entries = append(entries, toEntry(r, names))
	}

	if jsonOut {
		return printJSON(out, entries)
	}
	width := 0
	for _, e := range entries {
		if len(e.Ref) > width {
			width = len(e.Ref)
		}
	}
	for _, e := range entries {
		fmt.Fprintf(out, "%-*s  %s  %-11s %-11s %-16s %s\n",
			width, e.Ref, e.Date, e.Status, e.Kind, dashIfEmpty(e.Trigger), e.Title)
	}
	if len(entries) == 0 {
		fmt.Fprintln(out, emptyMessage)
	}
	return nil
}

/** validateRecordFilters refuses a `record list` filter that could only ever answer
 * "no matching records" because of a mistake, so a typo is not mistaken for an
 * empty result.
 *
 * A malformed --since, or --workspace together with --project (workspace-tier
 * records have no project), is a mistake in the command line: a usage error.
 * A --project no project has, or a --status, --kind, --trigger or --initiative
 * no record carries and the documented vocabulary does not list, names
 * something that does not exist: a plain error that lists what does. The
 * vocabularies are documented rather than enforced, so a value outside them
 * that some record really carries is a real filter and passes. A value inside
 * a vocabulary passes even when nothing carries it yet: matching nothing is a
 * legitimate answer for it.
 *
 * Parameters:
 *   kb (*knowledge.KnowledgeBase) — the open knowledge base.
 *   f  (recordFlags)              — the parsed flags.
 *
 * Returns:
 *   error — nil if every filter is usable.
 *
 * Example:
 *   if err := validateRecordFilters(kb, f); err != nil { return err }
 */
func validateRecordFilters(kb *knowledge.KnowledgeBase, f recordFlags) error {
	if f.workspace && f.project != "" {
		return usageErrorf("--workspace and --project cannot be combined: workspace-tier records have no project")
	}
	if f.since != "" {
		if err := checkSinceDate(f.since); err != nil {
			return err
		}
	}
	if f.project != "" {
		p, err := kb.ProjectByName(f.project)
		if err != nil {
			return err
		}
		if p == nil {
			projects, err := kb.Projects()
			if err != nil {
				return err
			}
			names := make([]string, len(projects))
			for i, pr := range projects {
				names[i] = pr.Name
			}
			return notFoundf("unknown project %q; known projects: %s", f.project, joinKnown(names))
		}
	}
	for _, c := range []struct {
		field, plural, value string
		vocabulary           []string
	}{
		{"status", "statuses", f.status, knowledge.RecordStatuses},
		{"kind", "kinds", f.kind, knowledge.RecordKinds},
		{"trigger", "triggers", f.trigger, knowledge.RecordTriggers},
		{"initiative", "initiatives", f.initiative, nil},
	} {
		if err := checkRecordVocabulary(kb, c.field, c.plural, c.value, c.vocabulary); err != nil {
			return err
		}
	}
	return nil
}

/** checkRecordVocabulary applies the rule shared by `record list` filters and
 * `record new`: a value outside a field's documented vocabulary is an error
 * only if no record in the database carries it either. The vocabularies are
 * documented rather than enforced, so a value some record really carries is an
 * established convention and passes; a value in the vocabulary passes even when
 * nothing carries it yet. An empty value is not checked.
 *
 * Parameters:
 *   kb         (*knowledge.KnowledgeBase) — the open knowledge base.
 *   field      (string)   — "status", "kind", "trigger" or "initiative".
 *   plural     (string)   — the field's plural, for the message.
 *   value      (string)   — the value to check.
 *   vocabulary ([]string) — the documented values; nil for a free-text field.
 *
 * Returns:
 *   error — a plain "unknown FIELD ..." error listing what is accepted, or nil.
 *
 * Example:
 *   err := checkRecordVocabulary(kb, "trigger", "triggers", "desing", knowledge.RecordTriggers)
 */
func checkRecordVocabulary(kb *knowledge.KnowledgeBase, field, plural, value string, vocabulary []string) error {
	if value == "" {
		return nil
	}
	carried, err := kb.DistinctRecordValues(field)
	if err != nil {
		return err
	}
	known := append([]string(nil), vocabulary...)
	for _, v := range carried {
		if !containsString(known, v) {
			known = append(known, v)
		}
	}
	if !containsString(known, value) {
		return notFoundf("unknown %s %q; known %s: %s", field, value, plural, joinKnown(known))
	}
	return nil
}

/** checkSinceDate accepts the date forms a record's date column can be compared
 * against as text: YYYY, YYYY-MM or YYYY-MM-DD, and only real dates.
 *
 * Parameters:
 *   s (string) — the --since value.
 *
 * Returns:
 *   error — a usage error naming --since, or nil.
 *
 * Example:
 *   err := checkSinceDate("2026-09") // nil
 *   err = checkSinceDate("yesterday") // usage error
 */
func checkSinceDate(s string) error {
	layout := map[int]string{4: "2006", 7: "2006-01", 10: "2006-01-02"}[len(s)]
	if layout != "" {
		if _, err := time.Parse(layout, s); err == nil {
			return nil
		}
	}
	return usageErrorf("invalid --since %q; want YYYY, YYYY-MM or YYYY-MM-DD", s)
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// joinKnown renders the values an unknown filter could have meant, capped so
// a database with hundreds of initiatives does not print hundreds of them. An
// empty list says so rather than printing nothing.
func joinKnown(values []string) string {
	const limit = 20
	if len(values) == 0 {
		return "(none recorded)"
	}
	if len(values) > limit {
		return strings.Join(values[:limit], ", ") + fmt.Sprintf(", ... (%d more)", len(values)-limit)
	}
	return strings.Join(values, ", ")
}

// dashIfEmpty renders an empty column as "-", never as spaces, so every
// column stays addressable.
func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// resolveRecord finds the record a reference names (knowledge DR-0057):
// "SCOPE/DR-NNNN", where SCOPE is a project name, "workspace", or the
// workspace directory's basename as an alias for it; or a bare id, which
// resolves only where it is unambiguous. The lookup itself is the library's
// (knowledge.ResolveRef), so harvey and kb answer the same way; what is kb's
// own is --project and --workspace, which remain as aliases for the qualified
// form and must agree with it when both are given.
//
// The workspace comes from the same root the record files are read from, not
// from where the database happens to live. ingest stamps it that way, so
// resolving any other way makes every record verb fail whenever the database
// sits outside the workspace it indexes — which is what a scratch database is.
func resolveRecord(kb *knowledge.KnowledgeBase, arg string, f recordFlags) (*knowledge.Record, error) {
	return resolveRecordScoped(kb, arg, f, false)
}

// resolveRecordForWrite is resolveRecord for a verb that changes a record. A
// write never runs on a bare id with no scope (DR-0058): it needs a qualified
// reference, --project or --workspace, or a project inferred from the working
// directory or KB_PROJECT, so the record changed is the record meant. With no
// scope the error offers the qualified form of each record the id could be.
func resolveRecordForWrite(kb *knowledge.KnowledgeBase, arg string, f recordFlags) (*knowledge.Record, error) {
	return resolveRecordScoped(kb, arg, f, true)
}

func resolveRecordScoped(kb *knowledge.KnowledgeBase, arg string, f recordFlags, write bool) (*knowledge.Record, error) {
	ref, err := knowledge.ParseRef(arg)
	if err != nil {
		return nil, wrapUsage(err)
	}
	var flagProject *knowledge.Project
	if f.project != "" {
		flagProject, err = kb.ProjectByName(f.project)
		if err != nil || flagProject == nil {
			return nil, notFoundf("unknown project %q", f.project)
		}
	}
	if ref.Scope == "" {
		switch {
		case f.workspace:
			ref.Scope = "workspace"
		case flagProject != nil:
			ref.Scope = flagProject.Name
		default:
			inferred, err := inferredProject(kb, f)
			if err != nil {
				return nil, err
			}
			ref.Scope = inferred
		}
	}
	if write && ref.Scope == "" {
		return nil, writeNeedsScope(kb, ref, f)
	}
	rec, err := kb.ResolveRef(ref, filepath.Base(recordRoot(kb, f)))
	if err != nil {
		if errors.Is(err, knowledge.ErrInvalid) {
			return nil, wrapUsage(err)
		}
		return nil, err
	}
	switch {
	case f.workspace && rec.Scope != "workspace":
		return nil, usageErrorf("%s names a project record, but --workspace was also given", ref)
	case flagProject != nil && (rec.Scope != "project" || rec.ProjectID != flagProject.ID):
		return nil, usageErrorf("%s is not in project %s, but --project %s was also given", ref, flagProject.Name, flagProject.Name)
	}
	return rec, nil
}

// recordShow prints one record with its relations resolved in both
// directions. Only supersedes is stored; superseded_by is its inverse.
func recordShow(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer) error {
	if len(f.args) != 1 {
		return usageErrorf("usage: record show RECORD_ID")
	}
	rec, err := resolveRecord(kb, f.args[0], f)
	if err != nil {
		return err
	}
	names := projectNames(kb)

	relations, err := kb.RelationsFor(rec.ID)
	if err != nil {
		return err
	}
	detail := recordDetail{recordListEntry: toEntry(*rec, names), Body: rec.Body}
	for _, rel := range relations {
		other, err := kb.RecordByID(rel.RecordID)
		if err != nil {
			continue
		}
		label := refOf(*other, names).String()
		switch rel.Relationship {
		case "supersedes":
			detail.Supersedes = append(detail.Supersedes, label)
		case "superseded_by":
			detail.SupersededBy = append(detail.SupersededBy, label)
		default:
			detail.RelatesTo = append(detail.RelatesTo, label)
		}
	}

	if jsonOut {
		return printJSON(out, detail)
	}
	fmt.Fprintf(out, "%s\n", detail.Ref)
	for _, row := range [][2]string{
		{"title", detail.Title},
		{"date", detail.Date},
		{"status", detail.Status},
		{"kind", detail.Kind},
		{"trigger", detail.Trigger},
		{"phase", detail.Phase},
		{"path", detail.Path},
		{"supersedes", strings.Join(detail.Supersedes, ", ")},
		{"superseded_by", strings.Join(detail.SupersededBy, ", ")},
		{"relates_to", strings.Join(detail.RelatesTo, ", ")},
	} {
		fmt.Fprintf(out, "  %-14s %s\n", row[0]+":", dashIfEmpty(row[1]))
	}
	fmt.Fprintf(out, "%s\n", detail.Body)
	return nil
}

// refOf is the qualified reference of a record: its project, or workspace, and
// its id. It is what every listing and confirmation prints (DR-0057).
func refOf(r knowledge.Record, names map[int64]string) knowledge.Ref {
	return knowledge.Ref{Scope: qualify(r, names), ID: r.RecordID}
}

// qualify names a record's tier: its project, or the workspace tier.
func qualify(r knowledge.Record, names map[int64]string) string {
	if r.Scope == "workspace" {
		return "workspace"
	}
	return names[r.ProjectID]
}

// recordConcepts implements `kb record concepts RECORD_ID`, mirroring
// `kb observation sources`: read-only visibility into what wikilink-tagging
// (see wikilink-tagging-design.md) has linked to a record.
func recordConcepts(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer) error {
	if len(f.args) != 1 {
		return usageErrorf("usage: record concepts RECORD_ID")
	}
	rec, err := resolveRecord(kb, f.args[0], f)
	if err != nil {
		return err
	}
	concepts, err := kb.RecordConcepts(rec.ID)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, concepts)
	}
	if len(concepts) == 0 {
		fmt.Fprintln(out, "(no linked concepts)")
		return nil
	}
	for _, c := range concepts {
		fmt.Fprintf(out, "%-4d  %s\n", c.ID, c.Name)
	}
	return nil
}

// recordRoot is the workspace root that stored paths are relative to.
func recordRoot(kb *knowledge.KnowledgeBase, f recordFlags) string {
	if f.root != "" {
		return f.root
	}
	return defaultIngestRoot(kb.Path())
}

// resolveWithinRoot joins a stored, workspace-relative record path onto the
// root and refuses any result that escapes it.
//
// filepath.Join cleans a path but does not confine it — Join("/a/b",
// "../../etc/x") is "/etc/x" — so without this check a path value that reached
// the database by some route other than ingest could make set-status and
// supersede read and write anywhere the user can. Ingest itself cannot store
// such a path (relativeTo falls back to the base name), but a hand-edited
// database, a direct SQL insert, or a merge from another machine could.
func resolveWithinRoot(root, rel string) (string, error) {
	joined := filepath.Join(root, rel)
	inside, err := filepath.Rel(root, joined)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", dataErrorf(
			"record path %q resolves outside the workspace root %s; refusing to read or write it",
			rel, root)
	}
	return joined, nil
}

// loadRecordFile reads and parses the file backing a record.
func loadRecordFile(root string, rec *knowledge.Record) (*knowledge.RecordFile, []byte, error) {
	path, err := resolveWithinRoot(root, rec.Path)
	if err != nil {
		return nil, nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, asContent(fmt.Errorf("reading DR-%s at %s: %w", rec.RecordID, path, err))
	}
	rf, err := knowledge.ParseRecord(raw, rec.Path)
	if err != nil {
		// The file's content is what is wrong, so it is data (65), not the
		// usage error the library's invalid-value error means for an argument.
		if errors.Is(err, knowledge.ErrInvalid) {
			return nil, nil, classedAs(classData, err)
		}
		return nil, nil, err
	}
	return rf, raw, nil
}

// saveRecordFile renders a record file, writes it, and refreshes its database
// row so the stored checksum and body match what is now on disk. It returns
// the rendered bytes so a caller can roll the write back.
func saveRecordFile(kb *knowledge.KnowledgeBase, root string, rec *knowledge.Record, rf *knowledge.RecordFile) ([]byte, error) {
	rendered, err := knowledge.RenderRecordFile(rf)
	if err != nil {
		return nil, err
	}
	path, err := resolveWithinRoot(root, rec.Path)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, rendered, 0o644); err != nil {
		return nil, fmt.Errorf("writing %s: %w", path, err)
	}
	updated, err := knowledge.ParseRecord(rendered, rec.Path)
	if err != nil {
		return rendered, err
	}
	// Carry the identity fields the file does not hold. Workspace especially:
	// leaving it empty makes AddRecord default to the database's own
	// workspace, which is a *different* identity whenever the database sits
	// outside the tree being edited — inserting a duplicate row, or failing
	// the uuid index where the file carries one.
	updated.Record.ProjectID = rec.ProjectID
	updated.Record.Path = rec.Path
	updated.Record.Workspace = rec.Workspace
	if _, err := kb.AddRecord(updated.Record); err != nil {
		return rendered, err
	}
	return rendered, nil
}

// normalisationNote reports whether re-rendering a file unchanged would
// already alter it, so that a set-status or supersede which also brings a
// non-canonical file into canonical form says so rather than doing it
// silently.
func normalisationNote(rf *knowledge.RecordFile, raw []byte, path string) string {
	asRead, err := knowledge.RenderRecordFile(rf)
	if err != nil || string(asRead) == string(raw) {
		return ""
	}
	return fmt.Sprintf("note: %s was not in canonical form and has also been normalised", path)
}

// joinNotes appends an advisory to an existing single-string note field,
// newline-separated, without introducing a leading blank when note is still
// empty.
func joinNotes(note, addition string) string {
	if note == "" {
		return addition
	}
	return note + "\n" + addition
}

// recordSetStatus writes a record's status to both the file and the database.
func recordSetStatus(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer) error {
	switch len(f.args) {
	case 1:
		return recordReviewStatus(kb, jsonOut, f, out)
	case 2:
	default:
		return usageErrorf("usage: record set-status RECORD_REF [STATUS]")
	}
	id, status := f.args[0], f.args[1]
	// A status is a value being written, and the promotion path at that, so a
	// typo is refused before any record or file is touched, by the rule record
	// new applies to trigger and kind: outside the vocabulary and carried by no
	// record is an error (DR-0048). An unknown one is a usage error here, where the
	// same value as a record list filter is a lookup.
	if strings.TrimSpace(status) == "" {
		return usageErrorf("record set-status needs a status; known statuses: %s", joinKnown(knowledge.RecordStatuses))
	}
	if err := checkRecordVocabulary(kb, "status", "statuses", status, knowledge.RecordStatuses); err != nil {
		return wrapUsage(err)
	}
	// Accepting a record is the author's act (DR-0061): it needs a person at a
	// terminal, and nothing on the command line or in the environment turns that
	// off. The check comes before anything is read or written, and it is a usage
	// error because the command as run cannot do what it asks.
	if status == "accepted" && !atTerminal(out) {
		return usageErrorf("a record can only be accepted by a person at a terminal: run kb record set-status %s accepted from an interactive shell (standard input and output must be a terminal)", id)
	}
	rec, err := resolveRecordForWrite(kb, id, f)
	if err != nil {
		return err
	}
	return applyRecordStatus(kb, jsonOut, f, out, rec, status)
}

/** applyRecordStatus is the write half of record set-status: it checks the
 * transition against the record as it is on disk now, then writes the file and
 * the database and refreshes index.md. The direct form and the review form both
 * end here, so they cannot disagree about what a status change is (DR-0060).
 *
 * Parameters:
 *   kb (*knowledge.KnowledgeBase) — the open knowledge base
 *   jsonOut (bool) — print the result as JSON
 *   f (recordFlags) — the parsed flags, for the corpus root
 *   out (io.Writer) — where the confirmation goes
 *   rec (*knowledge.Record) — the record to change, already resolved
 *   status (string) — the status to write
 *
 * Returns:
 *   error — nil on success; a negative-class error for a move the table forbids
 *
 * Example:
 *   err := applyRecordStatus(kb, false, f, os.Stdout, rec, "rejected")
 */
func applyRecordStatus(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer, rec *knowledge.Record, status string) error {
	root := recordRoot(kb, f)
	rf, raw, err := loadRecordFile(root, rec)
	if err != nil {
		return err
	}
	note := normalisationNote(rf, raw, rec.Path)

	if err := checkTransition(rf.Record.Status, status, len(rf.SupersededBy) > 0); err != nil {
		return err
	}
	rf.Record.Status = status
	if _, err := saveRecordFile(kb, root, rec, rf); err != nil {
		_ = os.WriteFile(filepath.Join(root, rec.Path), raw, 0o644)
		return err
	}
	if regenErr := regenerateIndexIfPresent(filepath.Dir(filepath.Join(root, rec.Path))); regenErr != nil {
		note = joinNotes(note, fmt.Sprintf("index.md could not be refreshed: %v", regenErr))
	}

	ref := refOf(*rec, projectNames(kb))
	result := map[string]any{"ref": ref.String(), "record_id": rec.RecordID, "status": status, "path": rec.Path}
	if note != "" {
		result["note"] = note
	}
	if jsonOut {
		return printJSON(out, result)
	}
	fmt.Fprintf(out, "%s status set to %s in %s\n", ref, status, rec.Path)
	if note != "" {
		fmt.Fprintln(out, note)
	}
	return nil
}

/** checkTransition applies the status transition table (DR-0060) to one move.
 * A record whose current status is outside the record vocabulary is data to
 * repair, not a state the table describes: it may become proposed and nothing
 * else, so reaching accepted takes two moves. A move onto a value outside the
 * vocabulary is not checked here (the caller has already required that another
 * record carries it). A refused move is a negative answer (exit 1): the command
 * line was well formed and the record's current state forbids it.
 *
 * Parameters:
 *   from (string) — the record's current status
 *   to (string) — the requested status
 *   hasSupersededBy (bool) — whether the record already carries superseded_by
 *
 * Returns:
 *   error — nil when the move is allowed, else a negative-class error naming
 *           the current status and the statuses it may move to
 *
 * Example:
 *   err := checkTransition("rejected", "accepted", false) // negative: reopen first
 */
func checkTransition(from, to string, hasSupersededBy bool) error {
	if !containsString(knowledge.RecordStatuses, from) {
		if to == "proposed" {
			return nil
		}
		return negativef("a record whose status %q is outside the vocabulary can only be set to proposed; it can then follow the table", from)
	}
	if !containsString(knowledge.RecordStatuses, to) {
		return nil
	}
	if knowledge.CanTransition(from, to, hasSupersededBy) {
		return nil
	}
	switch {
	case from == to:
		return negativef("the record is already %s", from)
	case to == "superseded" && !hasSupersededBy && containsString(knowledge.AllowedTransitions(from, true), to):
		return negativef("a record that is %s cannot be set to superseded without a superseded_by; use `kb record supersede NEW OLD`, which writes both sides", from)
	}
	allowed := knowledge.AllowedTransitions(from, hasSupersededBy)
	if len(allowed) == 0 {
		return negativef("a record that is %s cannot be set to %s; %s is final", from, to, from)
	}
	return negativef("a record that is %s cannot be set to %s; it may become: %s", from, to, strings.Join(allowed, ", "))
}

/** resolveSupersession finds the two records of a supersession and applies the
 * rules that do not need a write: both must resolve to a qualified record, they
 * must be in the same tier (writing both sides must not mean writing into another
 * repository), and a record cannot supersede itself. The command and the TUI both
 * call it, so the field refuses exactly what the command refuses.
 *
 * Parameters:
 *   kb (*knowledge.KnowledgeBase) — the open knowledge base
 *   newArg (string) — the record that supersedes, as typed
 *   oldArg (string) — the record that is superseded, as typed
 *   f (recordFlags) — the flags, for scope inference
 *
 * Returns:
 *   newer (*knowledge.Record) — the superseding record
 *   older (*knowledge.Record) — the superseded record
 *   err (error) — a not-found or usage error naming what is wrong
 *
 * Example:
 *   newer, older, err := resolveSupersession(kb, "clasm/DR-0004", "clasm/DR-0002", recordFlags{})
 */
func resolveSupersession(kb *knowledge.KnowledgeBase, newArg, oldArg string, f recordFlags) (newer, older *knowledge.Record, err error) {
	newer, err = resolveRecordForWrite(kb, newArg, f)
	if err != nil {
		return nil, nil, err
	}
	older, err = resolveRecordForWrite(kb, oldArg, f)
	if err != nil {
		return nil, nil, err
	}
	if newer.ProjectID != older.ProjectID || newer.Scope != older.Scope {
		return nil, nil, usageErrorf(
			"DR-%s and DR-%s are in different tiers; supersession is same-tier only, because writing both sides would mean writing into another repository",
			newer.RecordID, older.RecordID)
	}
	if newer.ID == older.ID {
		return nil, nil, usageErrorf("DR-%s cannot supersede itself", newer.RecordID)
	}
	return newer, older, nil
}

// recordSupersede writes both sides of a supersession: supersedes on the new
// record, superseded_by on the old one, the relation row, and — unless
// --partial — the old record's superseded status.
//
// Without --partial the old record is wholly replaced. With it, the old record
// stays accepted: a later record can invalidate one decision inside a
// multi-decision episode while the rest stand, which is why superseded_by
// never implies status superseded.
func recordSupersede(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer) error {
	if len(f.args) != 2 {
		return usageErrorf("usage: record supersede NEW OLD")
	}
	newer, older, err := resolveSupersession(kb, f.args[0], f.args[1], f)
	if err != nil {
		return err
	}

	root := recordRoot(kb, f)
	newerRF, newerRaw, err := loadRecordFile(root, newer)
	if err != nil {
		return err
	}
	olderRF, olderRaw, err := loadRecordFile(root, older)
	if err != nil {
		return err
	}
	notes := []string{}
	for _, n := range []string{
		normalisationNote(newerRF, newerRaw, newer.Path),
		normalisationNote(olderRF, olderRaw, older.Path),
	} {
		if n != "" {
			notes = append(notes, n)
		}
	}

	newerRF.Supersedes = appendUnique(newerRF.Supersedes, older.RecordID)
	olderRF.SupersededBy = appendUnique(olderRF.SupersededBy, newer.RecordID)
	if !f.partial {
		olderRF.Record.Status = "superseded"
	}

	restore := func() {
		_ = os.WriteFile(filepath.Join(root, newer.Path), newerRaw, 0o644)
		_ = os.WriteFile(filepath.Join(root, older.Path), olderRaw, 0o644)
	}
	if _, err := saveRecordFile(kb, root, newer, newerRF); err != nil {
		restore()
		return err
	}
	if _, err := saveRecordFile(kb, root, older, olderRF); err != nil {
		restore()
		return err
	}
	if err := kb.AddRecordRelation(newer.ID, older.ID, "supersedes"); err != nil {
		restore()
		return err
	}
	dirs := []string{filepath.Dir(filepath.Join(root, newer.Path))}
	if olderDir := filepath.Dir(filepath.Join(root, older.Path)); olderDir != dirs[0] {
		dirs = append(dirs, olderDir)
	}
	for _, dir := range dirs {
		if regenErr := regenerateIndexIfPresent(dir); regenErr != nil {
			notes = append(notes, fmt.Sprintf("index.md could not be refreshed in %s: %v", dir, regenErr))
		}
	}

	names := projectNames(kb)
	newRef, oldRef := refOf(*newer, names), refOf(*older, names)
	result := map[string]any{
		"new_ref": newRef.String(), "old_ref": oldRef.String(),
		"new": newer.RecordID, "old": older.RecordID,
		"partial": f.partial, "old_status": olderRF.Record.Status,
	}
	if len(notes) > 0 {
		result["notes"] = notes
	}
	if jsonOut {
		return printJSON(out, result)
	}
	fmt.Fprintf(out, "%s supersedes %s; %s is now %s\n",
		newRef, oldRef, oldRef, olderRF.Record.Status)
	for _, n := range notes {
		fmt.Fprintln(out, n)
	}
	return nil
}

// appendUnique adds value to list unless it is already present, so repeating a
// supersede does not list the same id twice.
func appendUnique(list []string, value string) []string {
	for _, v := range list {
		if v == value {
			return list
		}
	}
	return append(list, value)
}

// writeNeedsScope explains why a write was refused for want of a scope, and
// offers the qualified form of every record the bare id could mean.
func writeNeedsScope(kb *knowledge.KnowledgeBase, ref knowledge.Ref, f recordFlags) error {
	_, err := kb.ResolveRef(ref, filepath.Base(recordRoot(kb, f)))
	var amb *knowledge.AmbiguousRefError
	if errors.As(err, &amb) {
		return usageErrorf("%s needs a scope to be changed; say which: %s", ref, joinRefs(amb.Candidates))
	}
	if rec, rerr := kb.ResolveRef(ref, filepath.Base(recordRoot(kb, f))); rerr == nil {
		names := projectNames(kb)
		return usageErrorf("%s needs a scope to be changed; say which: %s", ref,
			knowledge.Ref{Scope: qualify(*rec, names), ID: rec.RecordID})
	}
	return err
}

func joinRefs(refs []knowledge.Ref) string {
	var parts []string
	for _, r := range refs {
		parts = append(parts, r.String())
	}
	return strings.Join(parts, ", ")
}

// deprecatedScopeFlag lists the record subverbs on which --project and
// --workspace are only aliases for a form that says the same thing without
// them: a scope argument for the listings, a qualified reference for the rest.
// new and fuzzy-tag are not here: the flag is the way to name the scope.
var deprecatedScopeFlag = recordScopeForms()

// adviseDeprecatedScopeFlags says on standard error that --project or
// --workspace has a better spelling (knowledge DR-0057). The flag still works.
func adviseDeprecatedScopeFlags(subverb string, f recordFlags) {
	form, ok := deprecatedScopeFlag[subverb]
	if !ok {
		return
	}
	if f.project != "" {
		if form == "list" {
			advise("--project %s is deprecated; give the scope as an argument: kb record %s %s", f.project, subverb, f.project)
		} else {
			advise("--project %s is deprecated; qualify the record instead: %s/DR-NNNN", f.project, f.project)
		}
	}
	if f.workspace {
		if form == "list" {
			advise("--workspace is deprecated; give the scope as an argument: kb record %s workspace", subverb)
		} else {
			advise("--workspace is deprecated; qualify the record instead: workspace/DR-NNNN")
		}
	}
}
