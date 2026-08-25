package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["record"] = cmdRecord
}

// recordListEntry is one row of `kb record list`, and the JSON shape callers
// script against.
type recordListEntry struct {
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
	partial    bool
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
		return fmt.Errorf("record requires a subverb: list, show, set-status or supersede")
	}
	flags, err := parseRecordFlags(args[1:])
	if err != nil {
		return err
	}
	dl.Log("record", map[string]any{"subverb": args[0], "args": flags.args})

	switch args[0] {
	case "list":
		return recordList(kb, jsonOut, flags, out)
	case "show":
		return recordShow(kb, jsonOut, flags, out)
	case "set-status":
		return recordSetStatus(kb, jsonOut, flags, out)
	case "supersede":
		return recordSupersede(kb, jsonOut, flags, out)
	default:
		return fmt.Errorf("unknown record subverb %q; want list, show, set-status or supersede", args[0])
	}
}

// parseRecordFlags separates flags from positional arguments.
func parseRecordFlags(args []string) (recordFlags, error) {
	var f recordFlags
	strFlags := map[string]*string{
		"--project": &f.project, "--status": &f.status, "--kind": &f.kind,
		"--trigger": &f.trigger, "--initiative": &f.initiative,
		"--since": &f.since, "--root": &f.root,
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if target, ok := strFlags[arg]; ok {
			if i+1 >= len(args) {
				return f, fmt.Errorf("%s requires a value", arg)
			}
			*target = args[i+1]
			i++
			continue
		}
		switch arg {
		case "--workspace":
			f.workspace = true
		case "--partial":
			f.partial = true
		default:
			if strings.HasPrefix(arg, "-") {
				return f, fmt.Errorf("unknown flag %q", arg)
			}
			f.args = append(f.args, arg)
		}
	}
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
	filter := knowledge.RecordFilter{
		Project: f.project, Status: f.status, Kind: f.kind,
		Trigger: f.trigger, Initiative: f.initiative, Since: f.since,
	}
	if f.workspace {
		filter.Scope = "workspace"
	} else if f.project != "" {
		filter.Scope = "project"
	}

	records, err := kb.ListRecords(filter)
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
	for _, e := range entries {
		fmt.Fprintf(out, "DR-%s  %s  %-11s %-11s %-16s %s\n",
			e.RecordID, e.Date, e.Status, e.Kind, dashIfEmpty(e.Trigger), e.Title)
	}
	if len(entries) == 0 {
		fmt.Fprintln(out, "no matching records")
	}
	return nil
}

// dashIfEmpty renders an empty column as "-", never as spaces, so every
// column stays addressable.
func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// resolveRecord finds the record a bare id names. An id alone is not an
// identity — two projects may each have a DR-0001 — so an ambiguous id is
// reported with its candidates rather than silently resolved.
func resolveRecord(kb *knowledge.KnowledgeBase, id string, f recordFlags) (*knowledge.Record, error) {
	switch {
	case f.workspace:
		r, err := kb.RecordByIdentity(0, "workspace", id)
		if err != nil {
			return nil, fmt.Errorf("no record DR-%s in the workspace tier", id)
		}
		return r, nil
	case f.project != "":
		p, err := kb.ProjectByName(f.project)
		if err != nil || p == nil {
			return nil, fmt.Errorf("unknown project %q", f.project)
		}
		r, err := kb.RecordByIdentity(p.ID, "project", id)
		if err != nil {
			return nil, fmt.Errorf("no record DR-%s in project %s", id, f.project)
		}
		return r, nil
	}

	matches, err := kb.RecordsByRecordID(id)
	if err != nil {
		return nil, err
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no record DR-%s", id)
	case 1:
		return &matches[0], nil
	}
	names := projectNames(kb)
	var where []string
	for _, m := range matches {
		if m.Scope == "workspace" {
			where = append(where, "--workspace")
			continue
		}
		where = append(where, "--project "+names[m.ProjectID])
	}
	return nil, fmt.Errorf("DR-%s is ambiguous across %s; qualify it with one of: %s",
		id, strings.Join(projectLabels(matches, names), ", "), strings.Join(where, ", "))
}

// projectLabels names the owners of a set of records, for an ambiguity error.
func projectLabels(records []knowledge.Record, names map[int64]string) []string {
	var out []string
	for _, r := range records {
		if r.Scope == "workspace" {
			out = append(out, "the workspace tier")
			continue
		}
		out = append(out, names[r.ProjectID])
	}
	return out
}

// recordShow prints one record with its relations resolved in both
// directions. Only supersedes is stored; superseded_by is its inverse.
func recordShow(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer) error {
	if len(f.args) < 1 {
		return fmt.Errorf("record show requires a RECORD_ID")
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
		label := "DR-" + other.RecordID
		if other.ProjectID != rec.ProjectID || other.Scope != rec.Scope {
			label = qualify(*other, names) + ":" + label
		}
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
	fmt.Fprintf(out, "DR-%s  %s\n", detail.RecordID, qualify(*rec, names))
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

// qualify names a record's tier: its project, or the workspace tier.
func qualify(r knowledge.Record, names map[int64]string) string {
	if r.Scope == "workspace" {
		return "workspace"
	}
	return names[r.ProjectID]
}

// recordRoot is the workspace root that stored paths are relative to.
func recordRoot(kb *knowledge.KnowledgeBase, f recordFlags) string {
	if f.root != "" {
		return f.root
	}
	return defaultIngestRoot(kb.Path())
}

// loadRecordFile reads and parses the file backing a record.
func loadRecordFile(root string, rec *knowledge.Record) (*knowledge.RecordFile, []byte, error) {
	path := filepath.Join(root, rec.Path)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading DR-%s at %s: %w", rec.RecordID, path, err)
	}
	rf, err := knowledge.ParseRecord(raw, rec.Path)
	if err != nil {
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
	path := filepath.Join(root, rec.Path)
	if err := os.WriteFile(path, rendered, 0o644); err != nil {
		return nil, fmt.Errorf("writing %s: %w", path, err)
	}
	updated, err := knowledge.ParseRecord(rendered, rec.Path)
	if err != nil {
		return rendered, err
	}
	updated.Record.ProjectID = rec.ProjectID
	updated.Record.Path = rec.Path
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

// recordSetStatus writes a record's status to both the file and the database.
func recordSetStatus(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer) error {
	if len(f.args) < 2 {
		return fmt.Errorf("record set-status requires a RECORD_ID and a STATUS")
	}
	id, status := f.args[0], f.args[1]
	rec, err := resolveRecord(kb, id, f)
	if err != nil {
		return err
	}
	root := recordRoot(kb, f)
	rf, raw, err := loadRecordFile(root, rec)
	if err != nil {
		return err
	}
	note := normalisationNote(rf, raw, rec.Path)

	rf.Record.Status = status
	if _, err := saveRecordFile(kb, root, rec, rf); err != nil {
		_ = os.WriteFile(filepath.Join(root, rec.Path), raw, 0o644)
		return err
	}

	result := map[string]any{"record_id": rec.RecordID, "status": status, "path": rec.Path}
	if note != "" {
		result["note"] = note
	}
	if jsonOut {
		return printJSON(out, result)
	}
	fmt.Fprintf(out, "DR-%s status set to %s in %s\n", rec.RecordID, status, rec.Path)
	if note != "" {
		fmt.Fprintln(out, note)
	}
	return nil
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
	if len(f.args) < 2 {
		return fmt.Errorf("record supersede requires a NEW and an OLD record id")
	}
	newer, err := resolveRecord(kb, f.args[0], f)
	if err != nil {
		return err
	}
	older, err := resolveRecord(kb, f.args[1], f)
	if err != nil {
		return err
	}
	if newer.ProjectID != older.ProjectID || newer.Scope != older.Scope {
		return fmt.Errorf(
			"DR-%s and DR-%s are in different tiers; supersession is same-tier only, because writing both sides would mean writing into another repository",
			newer.RecordID, older.RecordID)
	}
	if newer.ID == older.ID {
		return fmt.Errorf("DR-%s cannot supersede itself", newer.RecordID)
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

	result := map[string]any{
		"new": newer.RecordID, "old": older.RecordID,
		"partial": f.partial, "old_status": olderRF.Record.Status,
	}
	if len(notes) > 0 {
		result["notes"] = notes
	}
	if jsonOut {
		return printJSON(out, result)
	}
	fmt.Fprintf(out, "DR-%s supersedes DR-%s; DR-%s is now %s\n",
		newer.RecordID, older.RecordID, older.RecordID, olderRF.Record.Status)
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
