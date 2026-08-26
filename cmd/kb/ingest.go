package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["ingest"] = cmdIngest
}

// recordFilePattern matches a decision record filename: a four-digit id, a
// dash, a slug. The generated index.md deliberately does not match.
var recordFilePattern = regexp.MustCompile(`^[0-9]{4}-.*\.md$`)

// ingestSummary is what one ingest run reports, in both JSON and text form.
// Unresolved, Malformed and Missing are the three ways a run can be
// incomplete without being a failure.
type ingestSummary struct {
	Root       string   `json:"root"`
	Path       string   `json:"path"`
	DryRun     bool     `json:"dry_run"`
	Added      int      `json:"added"`
	Updated    int      `json:"updated"`
	Skipped    int      `json:"skipped"`
	Failed     int      `json:"failed"`
	Supersedes int      `json:"supersedes"`
	RelatesTo  int      `json:"relates_to"`
	Warnings   []string `json:"warnings,omitempty"`
	Unresolved []string `json:"unresolved,omitempty"`
	Malformed  []string `json:"malformed,omitempty"`
	Missing    []string `json:"missing,omitempty"`
	Errors     []string `json:"errors,omitempty"`
}

// ingestedRecord is one file that made it through pass one, carrying what
// pass two needs to resolve its references.
type ingestedRecord struct {
	file      *knowledge.RecordFile
	dbID      int64 // 0 during --dry-run, where nothing is written
	projectID int64
}

// identityKey is how a reference names a record: the project name (empty at
// the workspace tier), the scope, and the record id.
type identityKey struct {
	project  string
	scope    string
	recordID string
}

// ingester carries one run's state across both passes.
type ingester struct {
	kb   *knowledge.KnowledgeBase
	root string
	// workspace is the base name of root, not of the database's location:
	// --root may name a workspace other than the one the database sits in.
	workspace string
	dryRun    bool
	summary   ingestSummary
	byIdent   map[identityKey]*ingestedRecord
	order     []*ingestedRecord
}

/** cmdIngest implements `kb ingest PATH [--dry-run] [--root DIR]`: it walks a
 * directory tree of decision records, upserts each into the records table and
 * the full-text index, and resolves their cross-references.
 *
 * Ingest is additive. A record whose file has vanished stays in the database
 * and is reported, because pruning rows whose files are missing would destroy
 * data on a partial or wrong-directory run. Ingest also never writes to a
 * record file — only `kb record` does.
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — the open knowledge base.
 *   dl      (*DebugLog)                — debug log, may be nil.
 *   jsonOut (bool)                     — emit the summary as JSON.
 *   args    ([]string)                 — PATH plus --dry-run / --root DIR.
 *   out     (io.Writer)                — where the summary is written.
 *
 * Returns:
 *   error — on a usage error or an unreadable tree. An unresolvable
 *           reference, a malformed one, or a single unparsable file are
 *           reported in the summary, not returned.
 *
 * Example:
 *   err := cmdIngest(kb, nil, false, []string{"clasm/decisions"}, os.Stdout)
 */
func cmdIngest(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	path, root, dryRun, err := parseIngestFlags(args)
	if err != nil {
		return err
	}
	if root == "" {
		root = defaultIngestRoot(kb.Path())
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolving --root %s: %w", root, err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", path, err)
	}
	if fi, err := os.Stat(absPath); err != nil || !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}

	ing := &ingester{
		kb:        kb,
		root:      absRoot,
		workspace: filepath.Base(absRoot),
		dryRun:    dryRun,
		byIdent:   map[identityKey]*ingestedRecord{},
		summary:   ingestSummary{Root: absRoot, Path: absPath, DryRun: dryRun},
	}

	files, err := collectRecordFiles(absPath)
	if err != nil {
		return err
	}
	ing.upsertAll(files)
	ing.resolveAll()
	ing.reportMissing(absPath)

	dl.Log("ingest", map[string]any{
		"path": absPath, "root": absRoot, "dry_run": dryRun,
		"added": ing.summary.Added, "updated": ing.summary.Updated,
		"skipped": ing.summary.Skipped, "failed": ing.summary.Failed,
		"supersedes": ing.summary.Supersedes, "relates_to": ing.summary.RelatesTo,
	})

	if jsonOut {
		return printJSON(out, ing.summary)
	}
	writeIngestText(out, ing.summary)
	return nil
}

// parseIngestFlags pulls PATH, --root and --dry-run out of args.
func parseIngestFlags(args []string) (path, root string, dryRun bool, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run", "-dry-run":
			dryRun = true
		case "--root", "-root":
			if i+1 >= len(args) {
				return "", "", false, fmt.Errorf("--root requires a directory argument")
			}
			root = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return "", "", false, fmt.Errorf("unknown flag %q", args[i])
			}
			if path != "" {
				return "", "", false, fmt.Errorf("ingest takes a single PATH, got %q and %q", path, args[i])
			}
			path = args[i]
		}
	}
	if path == "" {
		return "", "", false, fmt.Errorf("ingest requires a PATH; see kb help ingest")
	}
	return path, root, dryRun, nil
}

// defaultIngestRoot returns the workspace root implied by the database's
// location: the parent of the directory holding it, so that
// --db agents/knowledge.db gives a root of the workspace itself. Paths are
// stored relative to it, since absolute paths do not survive merge between
// machines.
func defaultIngestRoot(dbPath string) string {
	return filepath.Dir(filepath.Dir(dbPath))
}

// collectRecordFiles walks dir and returns every record file, sorted, so that
// a run is deterministic.
func collectRecordFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !recordFilePattern.MatchString(d.Name()) {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}

// upsertAll is pass one: parse and store every record, without resolving any
// reference. A record may cite one the walk has not reached yet.
func (ing *ingester) upsertAll(files []string) {
	for _, file := range files {
		rf, err := knowledge.ParseRecordFile(file)
		if err != nil {
			ing.summary.Failed++
			ing.summary.Errors = append(ing.summary.Errors, err.Error())
			continue
		}
		name := ing.relativeTo(file)
		for _, w := range rf.Warnings {
			ing.summary.Warnings = append(ing.summary.Warnings, name+": "+w)
		}
		rf.Record.Path = name

		projectID, err := ing.projectID(rf.ProjectName)
		if err != nil {
			ing.summary.Failed++
			ing.summary.Errors = append(ing.summary.Errors, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		rf.Record.ProjectID = projectID
		rf.Record.Workspace = ing.workspace

		rec := &ingestedRecord{file: rf, projectID: projectID}
		existing, err := ing.kb.RecordByIdentity(ing.workspace, projectID, rf.Record.Scope, rf.Record.RecordID)
		if err == nil {
			if problem := identityCollision(existing, &rf.Record, name); problem != "" {
				ing.summary.Failed++
				ing.summary.Errors = append(ing.summary.Errors, problem)
				continue
			}
			if existing.Path != rf.Record.Path {
				ing.summary.Warnings = append(ing.summary.Warnings, fmt.Sprintf(
					"%s: DR-%s was stored at %s; a slug may have been regenerated, or two files may claim one id",
					name, rf.Record.RecordID, existing.Path))
			}
		}
		switch {
		case err == nil && existing.Checksum == rf.Record.Checksum:
			ing.summary.Skipped++
			rec.dbID = existing.ID
		case err == nil:
			ing.summary.Updated++
		default:
			ing.summary.Added++
		}

		if !ing.dryRun && rec.dbID == 0 {
			id, err := ing.kb.AddRecord(rf.Record)
			if err != nil {
				ing.summary.Failed++
				ing.summary.Errors = append(ing.summary.Errors, fmt.Sprintf("%s: %v", name, err))
				continue
			}
			rec.dbID = id
		}
		ing.linkInitiative(rf, projectID)

		ing.byIdent[identityKey{rf.ProjectName, rf.Record.Scope, rf.Record.RecordID}] = rec
		ing.order = append(ing.order, rec)
	}
}

// identityCollision reports whether an incoming record is a *different*
// record wearing an identity that is already taken, in which case storing it
// would destroy the existing one.
//
// The case this exists for is the workspace tier. Every workspace has an
// agents/decisions/, and a workspace-tier record's identity is
// (NULL project, "workspace", record_id) — unique only within one workspace.
// Ingesting two workspaces' tiers into one database would otherwise overwrite
// the first with the second and report it as a routine update.
//
// uuid is what settles it: it is a record's stable identity across machines,
// so two non-empty and differing uuids mean two records. Where either uuid is
// empty the check cannot conclude anything and stays silent; the caller warns
// on a changed path instead, since a slug is cosmetic and may be regenerated.
//
// This is interim. The real fix is to carry the workspace's own name so that a
// workspace-tier identity is complete, which would also retire the NULL
// project_id this works around.
func identityCollision(existing, incoming *knowledge.Record, name string) string {
	if existing.UUID == "" || incoming.UUID == "" || existing.UUID == incoming.UUID {
		return ""
	}
	return fmt.Sprintf(
		"%s: DR-%s already exists with a different uuid (%s stored at %s, incoming %s); "+
			"refusing to overwrite it, since two records cannot share one identity",
		name, incoming.RecordID, existing.UUID, existing.Path, incoming.UUID)
}

// projectID resolves a frontmatter project name to a row, creating the
// project when it is not yet known. The workspace tier has no project and
// resolves to 0, stored as a NULL project_id.
func (ing *ingester) projectID(name string) (int64, error) {
	if name == "" {
		return 0, nil
	}
	// ProjectByName reports "not found" as (nil, nil), not as an error.
	if p, err := ing.kb.ProjectByName(name); err == nil && p != nil {
		return p.ID, nil
	}
	if ing.dryRun {
		return 0, nil
	}
	return ing.kb.AddProject(name, "")
}

// linkInitiative materialises the initiative field as a concept linked to the
// project, per design decision 3: project_concepts already expresses a
// many-projects-to-one-effort grouping, so no new table is needed.
func (ing *ingester) linkInitiative(rf *knowledge.RecordFile, projectID int64) {
	if ing.dryRun || rf.Record.Initiative == "" || projectID == 0 {
		return
	}
	conceptID, err := ing.kb.AddConcept(rf.Record.Initiative, "")
	if err != nil {
		return
	}
	_ = ing.kb.LinkProjectConcept(projectID, conceptID)
}

// relativeTo renders a file path relative to the workspace root, falling back
// to the base name if the file lies outside the root entirely.
func (ing *ingester) relativeTo(file string) string {
	rel, err := filepath.Rel(ing.root, file)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.Base(file)
	}
	return rel
}

// resolveAll is pass two: every record is now stored, so forward references
// resolve. Skipped records are resolved too — re-running is the documented
// remedy for a reference whose target had not yet been ingested.
func (ing *ingester) resolveAll() {
	for _, rec := range ing.order {
		name := rec.file.Record.Path

		// supersedes and superseded_by are same-tier only, so their entries
		// are always bare. A qualified one is reported rather than resolved:
		// writing both sides would mean writing into another repository.
		for _, entry := range rec.file.Supersedes {
			ing.relate(rec, entry, "supersedes", name, false)
		}
		for _, entry := range rec.file.SupersededBy {
			if strings.Contains(entry, ":") {
				ing.summary.Malformed = append(ing.summary.Malformed, fmt.Sprintf(
					"%s: superseded_by entry %q is qualified, but supersession is same-tier only", name, entry))
			}
		}
		for _, entry := range rec.file.RelatesTo {
			ing.relate(rec, entry, "relates_to", name, true)
		}
	}
}

// relate resolves one reference and stores the edge. Nothing here is fatal:
// an unresolvable target leaves the relation unwritten and adds a line to the
// summary, because failing would make ingest order significant — which is
// exactly what two passes exist to avoid.
func (ing *ingester) relate(rec *ingestedRecord, entry, relationship, name string, allowQualified bool) {
	key, ok := parseReference(entry, rec.file.ProjectName, rec.file.Record.Scope)
	if !ok {
		ing.summary.Malformed = append(ing.summary.Malformed, fmt.Sprintf(
			"%s: %s entry %q is malformed", name, relationship, entry))
		return
	}
	if !allowQualified && key.qualified {
		ing.summary.Malformed = append(ing.summary.Malformed, fmt.Sprintf(
			"%s: %s entry %q is qualified, but supersession is same-tier only", name, relationship, entry))
		return
	}

	target, ok := ing.lookup(key.identityKey)
	if !ok {
		ing.summary.Unresolved = append(ing.summary.Unresolved, fmt.Sprintf(
			"%s: %s entry %q has no matching record yet; re-run once it is ingested", name, relationship, entry))
		return
	}

	switch relationship {
	case "supersedes":
		ing.summary.Supersedes++
	case "relates_to":
		ing.summary.RelatesTo++
	}
	if ing.dryRun || rec.dbID == 0 || target == 0 {
		return
	}
	if err := ing.kb.AddRecordRelation(rec.dbID, target, relationship); err != nil {
		ing.summary.Errors = append(ing.summary.Errors, fmt.Sprintf("%s: %v", name, err))
	}
}

// reference is a parsed cross-reference plus whether it named a scope.
type reference struct {
	identityKey
	qualified bool
}

// parseReference reads a `[<scope>:]<id>` entry. It splits on the first colon
// only, and strips an optional leading DR- from the id rather than rejecting
// it: a stray prefix in one cross-reference should not fail a run.
func parseReference(entry, citingProject, citingScope string) (reference, bool) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return reference{}, false
	}

	ref := reference{identityKey: identityKey{project: citingProject, scope: citingScope}}
	id := entry
	if i := strings.Index(entry, ":"); i >= 0 {
		qualifier, rest := entry[:i], entry[i+1:]
		if qualifier == "" || rest == "" {
			return reference{}, false
		}
		ref.qualified = true
		id = rest
		if qualifier == "workspace" {
			ref.project, ref.scope = "", "workspace"
		} else {
			ref.project, ref.scope = qualifier, "project"
		}
	}

	id = strings.TrimPrefix(id, "DR-")
	if id == "" {
		return reference{}, false
	}
	ref.recordID = id
	return ref, true
}

// lookup finds a referenced record, checking this run's parsed set first so
// that --dry-run can resolve references against records it has not written.
func (ing *ingester) lookup(key identityKey) (int64, bool) {
	if rec, ok := ing.byIdent[key]; ok {
		return rec.dbID, true
	}
	projectID := int64(0)
	if key.scope != "workspace" {
		p, err := ing.kb.ProjectByName(key.project)
		if err != nil || p == nil {
			return 0, false
		}
		projectID = p.ID
	}
	// A reference resolves within the citing record's own workspace;
	// cross-workspace references are deliberately not expressible, for the
	// same reason supersession is same-tier only.
	r, err := ing.kb.RecordByIdentity(ing.workspace, projectID, key.scope, key.recordID)
	if err != nil {
		return 0, false
	}
	return r.ID, true
}

// reportMissing lists records whose file is no longer on disk. They are
// reported, never deleted: pruning on a partial or wrong-directory run would
// destroy data.
func (ing *ingester) reportMissing(dir string) {
	prefix := ing.relativeTo(dir)
	records, err := ing.kb.RecordsUnderPath(prefix)
	if err != nil {
		return
	}
	for _, r := range records {
		if _, err := os.Stat(filepath.Join(ing.root, r.Path)); err == nil {
			continue
		}
		ing.summary.Missing = append(ing.summary.Missing, fmt.Sprintf(
			"DR-%s (%s) is in the database but its file is gone; use kb record remove to drop it",
			r.RecordID, r.Path))
	}
}

// writeIngestText prints the summary in human-readable form.
func writeIngestText(out io.Writer, s ingestSummary) {
	if s.DryRun {
		fmt.Fprintf(out, "dry run, nothing written\n")
	}
	fmt.Fprintf(out, "%d added, %d updated, %d skipped, %d failed\n",
		s.Added, s.Updated, s.Skipped, s.Failed)
	fmt.Fprintf(out, "%d supersedes, %d relates_to\n", s.Supersedes, s.RelatesTo)
	for _, group := range []struct {
		label string
		lines []string
	}{
		{"unresolved", s.Unresolved},
		{"malformed", s.Malformed},
		{"missing", s.Missing},
		{"warning", s.Warnings},
		{"error", s.Errors},
	} {
		for _, line := range group.lines {
			fmt.Fprintf(out, "%s: %s\n", group.label, line)
		}
	}
}
