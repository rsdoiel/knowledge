package knowledge

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// recordsSchema creates the decision-record tables. Applied after
// sourcesSchema; CREATE TABLE IF NOT EXISTS is idempotent, following the same
// lazy-migration pattern as the rest of the schema. The identity index is
// created separately, in applyRecordsMigration, because on a pre-W8 database
// the workspace column it spans does not exist until the ALTER has run.
const recordsSchema = `
CREATE TABLE IF NOT EXISTS records (
    id           INTEGER  PRIMARY KEY AUTOINCREMENT,
    record_id    TEXT     NOT NULL,
    project_id   INTEGER  REFERENCES projects(id) ON DELETE SET NULL,
    scope        TEXT     NOT NULL DEFAULT 'project',
    path         TEXT     NOT NULL,
    title        TEXT     NOT NULL,
    date         TEXT     NOT NULL,
    status       TEXT     NOT NULL DEFAULT 'proposed',
    kind         TEXT     NOT NULL DEFAULT 'decision',
    "trigger"    TEXT     NOT NULL DEFAULT '',
    phase        TEXT     NOT NULL DEFAULT '',
    initiative   TEXT     NOT NULL DEFAULT '',
    session      TEXT     NOT NULL DEFAULT '',
    body         TEXT     NOT NULL,
    checksum     TEXT     NOT NULL DEFAULT '',
    ingested_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    uuid         TEXT     NOT NULL DEFAULT '',
    origin_host  TEXT     NOT NULL DEFAULT '',
    workspace    TEXT     NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_records_uuid
    ON records(uuid);

CREATE TABLE IF NOT EXISTS record_relations (
    from_id      INTEGER NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    to_id        INTEGER NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    relationship TEXT    NOT NULL,
    PRIMARY KEY (from_id, to_id, relationship)
);
`

// recordsAlterStmts are lazy-migration statements for the records table.
// SQLite reports "duplicate column name" on a second run, which the caller
// ignores, matching kbAlterStmts.
var recordsAlterStmts = []string{
	`ALTER TABLE records ADD COLUMN workspace TEXT NOT NULL DEFAULT ''`,
}

// recordsIdentityIndex is the unique index defining a record's identity.
//
// Two things about it are deliberate. It spans workspace because every
// workspace has an agents/decisions/ and both may hold a DR-0001: without the
// column, ingesting two workspaces' tiers into one database silently
// overwrote the first with the second. And it is expressed over
// IFNULL(project_id, -1) rather than project_id, because workspace-tier
// records carry a NULL project_id and SQLite treats NULLs in a unique index as
// distinct from one another, which would leave that tier unconstrained.
//
// The name differs from the pre-W8 idx_records_scope_id on purpose: CREATE
// UNIQUE INDEX IF NOT EXISTS would not replace an existing index of the same
// name, so the old one is dropped by name and the new one created beside it.
const recordsIdentityIndex = `
DROP INDEX IF EXISTS idx_records_scope_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_records_identity
    ON records(workspace, IFNULL(project_id, -1), scope, record_id);
`

// applyRecordsMigration adds the workspace column to a pre-W8 database,
// backfills it, and installs the identity index.
//
// Backfilling every existing row with this database's own workspace is
// correct precisely because of the defect that motivated the column: a
// database could only ever hold one workspace's records, since a second
// workspace's would have overwritten the first. There is therefore no
// ambiguity about which workspace the existing rows belong to.
func applyRecordsMigration(db *sql.DB, workspace string) error {
	for _, stmt := range recordsAlterStmts {
		_, _ = db.Exec(stmt)
	}
	if _, err := db.Exec(
		`UPDATE records SET workspace = ? WHERE workspace = ''`, workspace,
	); err != nil {
		return err
	}
	_, err := db.Exec(recordsIdentityIndex)
	return err
}

/** Record represents one decision record: a single file under a project's
 * decisions/ directory, parsed into its frontmatter fields plus its body.
 *
 * RecordID, Date and Phase are strings rather than numbers or times on
 * purpose. A bare 0142 in YAML parses as the integer 142, losing the zero
 * padding that makes ids sort and grep predictably, and a bare 2026-08-19
 * resolves to a timestamp. The file format quotes all three, and so does this
 * struct.
 *
 * ProjectID is 0 for a workspace-tier record, which is stored as a NULL
 * project_id. Scope distinguishes the two tiers explicitly: "project" or
 * "workspace".
 *
 * Fields:
 *   ID          (int64)     — primary key, assigned on insert.
 *   RecordID    (string)    — zero-padded record number, e.g. "0142".
 *   ProjectID   (int64)     — owning project; 0 for the workspace tier.
 *   Scope       (string)    — "project" or "workspace".
 *   Path        (string)    — file path relative to the workspace root.
 *   Title       (string)    — the record's title.
 *   Date        (string)    — YYYY-MM-DD, as written in the file.
 *   Status      (string)    — proposed | accepted | superseded | rejected.
 *   Kind        (string)    — decision | correction | note, documented not enforced.
 *   Trigger     (string)    — what prompted the episode; may be empty.
 *   Phase       (string)    — release or phase label, e.g. "0.0.46".
 *   Initiative  (string)    — cross-project initiative name.
 *   Session     (string)    — originating session identifier.
 *   Body        (string)    — everything after the closing frontmatter fence.
 *   Checksum    (string)    — over the raw file bytes; makes ingest idempotent.
 *   IngestedAt  (time.Time) — when the row was last written.
 *   UUID        (string)    — stable identity for cross-machine merge.
 *   OriginHost  (string)    — host that first wrote the record.
 *   Workspace   (string)    — the workspace the record belongs to, the base
 *                             name of the workspace root. Part of the record's
 *                             identity: two workspaces may each have a DR-0001.
 *
 * Example:
 *   recs, err := kb.RecordsByProject(projectID)
 *   for _, r := range recs {
 *       fmt.Printf("DR-%s  %s  %s\n", r.RecordID, r.Date, r.Title)
 *   }
 */
type Record struct {
	ID         int64
	RecordID   string
	ProjectID  int64
	Scope      string
	Path       string
	Title      string
	Date       string
	Status     string
	Kind       string
	Trigger    string
	Phase      string
	Initiative string
	Session    string
	Body       string
	Checksum   string
	IngestedAt time.Time
	UUID       string
	OriginHost string
	Workspace  string
}

/** RelatedRecord is one edge of the record graph, described from the point of
 * view of the record that was asked about. Only one direction of a
 * supersession is stored; RelationsFor synthesises the other, so a record that
 * is the target of a "supersedes" edge reports it as "superseded_by".
 *
 * Fields:
 *   RecordID     (int64)  — primary key of the record at the other end.
 *   Relationship (string) — supersedes | superseded_by | relates_to.
 *
 * Example:
 *   rels, err := kb.RelationsFor(id)
 *   for _, rel := range rels {
 *       fmt.Printf("%s -> %d\n", rel.Relationship, rel.RecordID)
 *   }
 */
type RelatedRecord struct {
	RecordID     int64
	Relationship string
}

// recordColumns is the SELECT list shared by every record read, in the order
// scanRecord expects.
const recordColumns = `id, record_id, IFNULL(project_id, 0), scope, path, title,
	date, status, kind, "trigger", phase, initiative, session, body, checksum,
	ingested_at, uuid, origin_host, workspace`

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanRecord reads one row selected with recordColumns into a Record.
func scanRecord(s rowScanner) (Record, error) {
	var r Record
	var ingestedAt sql.NullString
	err := s.Scan(&r.ID, &r.RecordID, &r.ProjectID, &r.Scope, &r.Path, &r.Title,
		&r.Date, &r.Status, &r.Kind, &r.Trigger, &r.Phase, &r.Initiative,
		&r.Session, &r.Body, &r.Checksum, &ingestedAt, &r.UUID, &r.OriginHost,
		&r.Workspace)
	if err != nil {
		return Record{}, err
	}
	if ingestedAt.Valid {
		r.IngestedAt = parseTimestamp(ingestedAt.String)
	}
	return r, nil
}

// projectKey maps a Record.ProjectID onto the value the identity index is
// built over, so that a workspace-tier record (project_id NULL, ProjectID 0)
// compares equal to itself.
func projectKey(projectID int64) int64 {
	if projectID <= 0 {
		return -1
	}
	return projectID
}

// projectValue maps a Record.ProjectID onto the value to store, translating 0
// into a SQL NULL.
func projectValue(projectID int64) any {
	if projectID <= 0 {
		return nil
	}
	return projectID
}

/** AddRecord inserts a decision record, or updates the existing row with the
 * same identity. A record's identity is (project_id, scope, record_id), which
 * is what makes re-ingesting a whole tree idempotent rather than duplicating
 * it.
 *
 * UUID and OriginHost are filled in when empty and preserved when the caller
 * supplies them, since a record file carries its own uuid: regenerating one on
 * every ingest would break cross-machine merge.
 *
 * Parameters:
 *   r (Record) — the record to store; the ID field is ignored.
 *
 * Returns:
 *   int64 — the ID of the inserted or updated row.
 *   error — on database failure.
 *
 * Example:
 *   id, err := kb.AddRecord(Record{
 *       RecordID: "0160", ProjectID: pid, Scope: "project",
 *       Path: "decisions/0160-iam-instance-profile.md",
 *       Title: "Associate/Replace IAM instance profile",
 *       Date: "2026-08-18", Status: "accepted", Kind: "decision",
 *   })
 */
func (kb *KnowledgeBase) AddRecord(r Record) (int64, error) {
	if r.Scope == "" {
		r.Scope = "project"
	}
	if r.Status == "" {
		r.Status = "proposed"
	}
	if r.Kind == "" {
		r.Kind = "decision"
	}
	// A record with no workspace belongs to the one this database sits in.
	if r.Workspace == "" {
		r.Workspace = kb.workspace
	}

	var existing int64
	err := kb.db.QueryRow(
		`SELECT id FROM records
		 WHERE workspace = ? AND IFNULL(project_id, -1) = ? AND scope = ? AND record_id = ?`,
		r.Workspace, projectKey(r.ProjectID), r.Scope, r.RecordID,
	).Scan(&existing)
	switch {
	case err == nil:
		if _, err := kb.db.Exec(
			`UPDATE records SET path = ?, title = ?, date = ?, status = ?, kind = ?,
			        "trigger" = ?, phase = ?, initiative = ?, session = ?, body = ?,
			        checksum = ?, ingested_at = CURRENT_TIMESTAMP
			 WHERE id = ?`,
			r.Path, r.Title, r.Date, r.Status, r.Kind, r.Trigger, r.Phase,
			r.Initiative, r.Session, r.Body, r.Checksum, existing,
		); err != nil {
			return 0, err
		}
		kb.indexRecordFTS(existing, r)
		return existing, nil
	case err != sql.ErrNoRows:
		return 0, err
	}

	if r.UUID == "" {
		u, err := uuid.NewV7()
		if err != nil {
			return 0, fmt.Errorf("knowledge: generate uuid: %w", err)
		}
		r.UUID = u.String()
	}
	if r.OriginHost == "" {
		r.OriginHost, _ = os.Hostname()
	}

	res, err := kb.db.Exec(
		`INSERT INTO records (record_id, project_id, scope, path, title, date,
		        status, kind, "trigger", phase, initiative, session, body,
		        checksum, uuid, origin_host, workspace)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.RecordID, projectValue(r.ProjectID), r.Scope, r.Path, r.Title, r.Date,
		r.Status, r.Kind, r.Trigger, r.Phase, r.Initiative, r.Session, r.Body,
		r.Checksum, r.UUID, r.OriginHost, r.Workspace,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	kb.indexRecordFTS(id, r)
	return id, nil
}

// indexRecordFTS refreshes the full-text row for a record, so that a record's
// title and body are reachable from Search alongside projects, observations
// and concepts. Delete-then-insert because FTS5 has no upsert, matching how
// projects and concepts are reindexed on update.
func (kb *KnowledgeBase) indexRecordFTS(id int64, r Record) {
	if !kb.ftsAvailable {
		return
	}
	_, _ = kb.db.Exec(
		`DELETE FROM kb_fts WHERE source_type = 'record' AND source_id = ?`, id)
	_, _ = kb.db.Exec(
		`INSERT INTO kb_fts(body, kind, label, descr, source_type, source_id, project_id)
		 VALUES (?, ?, ?, ?, 'record', ?, ?)`,
		r.Title+"\n"+r.Body, r.Kind, "DR-"+r.RecordID, r.Title, id, r.ProjectID)
}

/** RecordByIdentity returns the record with the given identity — the
 * (workspace, project, scope, record_id) tuple the identity index is built
 * over. This is how a reference in a record file is resolved to a row, since a
 * file cites "0160" or "clasm:0160", never a primary key.
 *
 * The workspace is explicit rather than taken from the database, because
 * ingest's --root may name a workspace other than the one the database sits
 * in. Pass kb.Workspace() for the database's own.
 *
 * Parameters:
 *   workspace (string) — the workspace name, e.g. "WorkLab".
 *   projectID (int64)  — the owning project; pass 0 for the workspace tier.
 *   scope     (string) — "project" or "workspace".
 *   recordID  (string) — zero-padded record number, e.g. "0160".
 *
 * Returns:
 *   *Record — the record.
 *   error   — sql.ErrNoRows when no such record exists.
 *
 * Example:
 *   r, err := kb.RecordByIdentity(kb.Workspace(), projectID, "project", "0160")
 *   ws, err := kb.RecordByIdentity("WorkLab", 0, "workspace", "0001")
 */
func (kb *KnowledgeBase) RecordByIdentity(workspace string, projectID int64, scope, recordID string) (*Record, error) {
	r, err := scanRecord(kb.db.QueryRow(
		`SELECT `+recordColumns+`
		 FROM records
		 WHERE workspace = ? AND IFNULL(project_id, -1) = ? AND scope = ? AND record_id = ?`,
		workspace, projectKey(projectID), scope, recordID,
	))
	if err != nil {
		return nil, err
	}
	return &r, nil
}

/** RecordFilter narrows a ListRecords query. A zero-valued field means "any";
 * Scope selects a tier, since a project name cannot express the workspace
 * tier, whose records have no project at all.
 *
 * Fields:
 *   Project    (string) — projects.name; "" matches any project.
 *   Scope      (string) — "project" or "workspace"; "" matches both.
 *   Status     (string) — exact status match.
 *   Kind       (string) — exact kind match.
 *   Trigger    (string) — exact trigger match.
 *   Initiative (string) — exact initiative match.
 *   Since      (string) — YYYY-MM-DD; keeps records dated on or after it.
 *
 * Example:
 *   recs, err := kb.ListRecords(RecordFilter{Project: "clasm", Kind: "correction"})
 */
type RecordFilter struct {
	Project    string
	Scope      string
	Status     string
	Kind       string
	Trigger    string
	Initiative string
	Since      string
}

// recordColumnsQ is recordColumns qualified for a join against projects.
const recordColumnsQ = `r.id, r.record_id, IFNULL(r.project_id, 0), r.scope, r.path,
	r.title, r.date, r.status, r.kind, r."trigger", r.phase, r.initiative,
	r.session, r.body, r.checksum, r.ingested_at, r.uuid, r.origin_host,
	r.workspace`

/** ListRecords returns the records matching a filter, oldest first, sorted by
 * date and then record_id — never by record_id alone, since a correction can
 * carry a lower id than the record it supersedes.
 *
 * Parameters:
 *   f (RecordFilter) — the filter; zero-valued fields match anything.
 *
 * Returns:
 *   []Record — matching records; empty when none match.
 *   error    — on database failure.
 *
 * Example:
 *   recs, err := kb.ListRecords(RecordFilter{Project: "clasm", Kind: "correction"})
 *   ws, err := kb.ListRecords(RecordFilter{Scope: "workspace"})
 */
func (kb *KnowledgeBase) ListRecords(f RecordFilter) ([]Record, error) {
	query := `SELECT ` + recordColumnsQ + `
		 FROM records r LEFT JOIN projects p ON p.id = r.project_id
		 WHERE 1 = 1`
	var args []any
	for _, c := range []struct {
		clause string
		value  string
	}{
		{` AND p.name = ?`, f.Project},
		{` AND r.scope = ?`, f.Scope},
		{` AND r.status = ?`, f.Status},
		{` AND r.kind = ?`, f.Kind},
		{` AND r."trigger" = ?`, f.Trigger},
		{` AND r.initiative = ?`, f.Initiative},
		{` AND r.date >= ?`, f.Since},
	} {
		if c.value != "" {
			query += c.clause
			args = append(args, c.value)
		}
	}
	query += ` ORDER BY r.date, r.record_id`

	rows, err := kb.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

/** RecordsByRecordID returns every record carrying the given id, across all
 * projects and both tiers. An id alone is not an identity — two projects may
 * each have a DR-0001 — so callers resolving a bare id use this to detect the
 * ambiguity rather than silently picking one.
 *
 * Parameters:
 *   recordID (string) — zero-padded record number, e.g. "0001".
 *
 * Returns:
 *   []Record — every match; empty when the id is unknown.
 *   error    — on database failure.
 *
 * Example:
 *   matches, err := kb.RecordsByRecordID("0001")
 *   if len(matches) > 1 { // ambiguous, ask the caller to qualify it }
 */
func (kb *KnowledgeBase) RecordsByRecordID(recordID string) ([]Record, error) {
	rows, err := kb.db.Query(
		`SELECT `+recordColumns+` FROM records WHERE record_id = ? ORDER BY scope, project_id`,
		recordID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

/** RecordsUnderPath returns every record whose stored path begins with
 * prefix, oldest first. Ingest uses it to report records whose file has
 * vanished from disk without deleting them.
 *
 * Parameters:
 *   prefix (string) — a path prefix, relative to the workspace root.
 *
 * Returns:
 *   []Record — matching records; empty when none match.
 *   error    — on database failure.
 *
 * Example:
 *   recs, err := kb.RecordsUnderPath("clasm/decisions")
 */
func (kb *KnowledgeBase) RecordsUnderPath(prefix string) ([]Record, error) {
	rows, err := kb.db.Query(
		`SELECT `+recordColumns+`
		 FROM records WHERE path LIKE ? ESCAPE '\'
		 ORDER BY date, record_id`,
		escapeLike(prefix)+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// escapeLike escapes the LIKE wildcards in a literal prefix.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

/** RecordByID returns the record with the given primary key.
 *
 * Parameters:
 *   id (int64) — the records table primary key.
 *
 * Returns:
 *   *Record — the record.
 *   error   — sql.ErrNoRows when not found; other errors on database failure.
 *
 * Example:
 *   r, err := kb.RecordByID(id)
 *   fmt.Println(r.Title)
 */
func (kb *KnowledgeBase) RecordByID(id int64) (*Record, error) {
	r, err := scanRecord(kb.db.QueryRow(
		`SELECT `+recordColumns+` FROM records WHERE id = ?`, id,
	))
	if err != nil {
		return nil, err
	}
	return &r, nil
}

/** RecordsByProject returns every record belonging to a project, oldest
 * first, sorted by date and then record_id.
 *
 * The sort matters. Ids are identity, not chronology: within a single date
 * real logs are inconsistently ordered, so a correction can carry a lower id
 * than the record it supersedes — clasm DR-0159 supersedes DR-0160. Sorting on
 * record_id alone would put them in the wrong order.
 *
 * Parameters:
 *   projectID (int64) — the owning project; pass 0 for workspace-tier records.
 *
 * Returns:
 *   []Record — matching records; empty when none exist.
 *   error    — on database failure.
 *
 * Example:
 *   recs, err := kb.RecordsByProject(pid)
 *   workspace, err := kb.RecordsByProject(0)
 */
func (kb *KnowledgeBase) RecordsByProject(projectID int64) ([]Record, error) {
	rows, err := kb.db.Query(
		`SELECT `+recordColumns+`
		 FROM records WHERE IFNULL(project_id, -1) = ?
		 ORDER BY date, record_id`,
		projectKey(projectID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

/** UpdateRecordStatus sets a record's status, leaving every other field
 * alone. It is the promotion path from proposed to accepted, and the
 * demotion path to superseded.
 *
 * Parameters:
 *   id     (int64)  — the records table primary key.
 *   status (string) — the new status value.
 *
 * Returns:
 *   error — when no such record exists, or on database failure.
 *
 * Example:
 *   err := kb.UpdateRecordStatus(id, "accepted")
 */
func (kb *KnowledgeBase) UpdateRecordStatus(id int64, status string) error {
	res, err := kb.db.Exec(
		`UPDATE records SET status = ? WHERE id = ?`, status, id,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("knowledge: no record with id %d", id)
	}
	return nil
}

/** AddRecordRelation stores one directed edge between two records. Adding the
 * same edge twice is a no-op.
 *
 * Only the stated direction is stored: superseded_by is the inverse of
 * supersedes and is computed by RelationsFor on read. Storing a supersession
 * deliberately does not change either record's status, because superseded_by
 * does not imply status: superseded — a later record can invalidate one
 * decision inside a multi-decision episode while the rest stand. Use
 * UpdateRecordStatus when the whole record really is superseded.
 *
 * Parameters:
 *   fromID       (int64)  — the citing record's primary key.
 *   toID         (int64)  — the cited record's primary key.
 *   relationship (string) — "supersedes" or "relates_to".
 *
 * Returns:
 *   error — when either record is missing, or on database failure.
 *
 * Example:
 *   err := kb.AddRecordRelation(newerID, olderID, "supersedes")
 */
func (kb *KnowledgeBase) AddRecordRelation(fromID, toID int64, relationship string) error {
	for _, id := range []int64{fromID, toID} {
		var n int
		if err := kb.db.QueryRow(
			`SELECT COUNT(*) FROM records WHERE id = ?`, id,
		).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("knowledge: no record with id %d", id)
		}
	}
	_, err := kb.db.Exec(
		`INSERT OR IGNORE INTO record_relations (from_id, to_id, relationship)
		 VALUES (?, ?, ?)`,
		fromID, toID, relationship,
	)
	return err
}

/** RelationsFor returns every relation touching a record, described from that
 * record's point of view. Edges where it is the source are reported as stored;
 * edges where it is the target are inverted, so "supersedes" becomes
 * "superseded_by". A relates_to edge is symmetric and reads the same from
 * either end.
 *
 * Parameters:
 *   id (int64) — the records table primary key.
 *
 * Returns:
 *   []RelatedRecord — matching relations; empty when the record has none.
 *   error           — on database failure.
 *
 * Example:
 *   rels, err := kb.RelationsFor(id)
 *   for _, rel := range rels {
 *       fmt.Printf("%s %d\n", rel.Relationship, rel.RecordID)
 *   }
 */
func (kb *KnowledgeBase) RelationsFor(id int64) ([]RelatedRecord, error) {
	rows, err := kb.db.Query(
		`SELECT other, rel FROM (
		     SELECT to_id AS other, relationship AS rel
		     FROM record_relations WHERE from_id = ?
		     UNION ALL
		     SELECT from_id AS other,
		            CASE relationship WHEN 'supersedes' THEN 'superseded_by'
		                              ELSE relationship END AS rel
		     FROM record_relations WHERE to_id = ?
		 ) ORDER BY other, rel`,
		id, id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RelatedRecord
	for rows.Next() {
		var rel RelatedRecord
		if err := rows.Scan(&rel.RecordID, &rel.Relationship); err != nil {
			return nil, err
		}
		out = append(out, rel)
	}
	return out, rows.Err()
}

/** NewUUID returns a fresh UUID v7, the identity every record and row in this
 * schema carries so that `merge` can reconcile writes made on different
 * machines.
 *
 * Returns:
 *   string — the UUID in its canonical hyphenated form.
 *   error  — if the system entropy source fails.
 *
 * Example:
 *   id, err := NewUUID() // "01a03b46-e5e0-7461-a74c-ac096492f96d"
 */
func NewUUID() (string, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("knowledge: generate uuid: %w", err)
	}
	return u.String(), nil
}

/** Today returns the current date as YYYY-MM-DD, the form the record format
 * requires. It is a string rather than a time.Time because a bare date in YAML
 * resolves to a timestamp, and the format quotes it to prevent exactly that.
 *
 * Returns:
 *   string — today's date, e.g. "2026-08-25".
 *
 * Example:
 *   date := Today() // "2026-08-25"
 */
func Today() string {
	return time.Now().Format("2006-01-02")
}
