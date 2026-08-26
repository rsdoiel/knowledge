package knowledge

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestRecord returns a Record with every required field populated so a test
// can override only the field under examination. Dates are quoted strings
// throughout, per plan finding 4.
func newTestRecord(recordID, title, date string) Record {
	return Record{
		RecordID: recordID,
		Scope:    "project",
		Path:     "decisions/" + recordID + "-test.md",
		Title:    title,
		Date:     date,
		Status:   "accepted",
		Kind:     "decision",
		Body:     "**Context.** test record\n",
		Checksum: "sha256:" + recordID,
	}
}

// tableExists reports whether a table or index of the given name is present.
func tableExists(t *testing.T, kb *KnowledgeBase, kind, name string) bool {
	t.Helper()
	var got string
	err := kb.db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = ? AND name = ?`, kind, name,
	).Scan(&got)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("querying sqlite_master for %s %q: %v", kind, name, err)
	}
	return true
}

func TestOpen_CreatesRecordTables(t *testing.T) {
	kb := openTestKB(t)
	for _, name := range []string{"records", "record_relations"} {
		if !tableExists(t, kb, "table", name) {
			t.Errorf("Open did not create table %q", name)
		}
	}
	if !tableExists(t, kb, "index", "idx_records_scope_id") {
		t.Error("Open did not create unique index idx_records_scope_id")
	}
}

func TestOpen_RecordSchemaIsIdempotent(t *testing.T) {
	path := DefaultPath(t.TempDir())

	kb1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	id, err := kb1.AddRecord(newTestRecord("0001", "First record", "2026-08-01"))
	if err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	if err := kb1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	kb2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open on a database that already has the record tables: %v", err)
	}
	defer kb2.Close()

	got, err := kb2.RecordByID(id)
	if err != nil {
		t.Fatalf("RecordByID after reopen: %v", err)
	}
	if got.Title != "First record" {
		t.Errorf("reopen lost data: title = %q, want %q", got.Title, "First record")
	}
}

func TestAddRecord_RoundTrip(t *testing.T) {
	kb := openTestKB(t)
	pid, err := kb.AddProject("clasm", "Caltech Library AWS SSM manager")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	want := Record{
		RecordID:   "0160",
		ProjectID:  pid,
		Scope:      "project",
		Path:       "decisions/0160-associate-replace-iam-instance-profile.md",
		Title:      "Associate/Replace IAM instance profile: recoverable `EntityAlreadyExists`",
		Date:       "2026-08-18",
		Status:     "accepted",
		Kind:       "decision",
		Trigger:    "live-test",
		Phase:      "0.0.46",
		Initiative: "eprints-to-rdm",
		Session:    "2026-08-18-clasm",
		Body:       "**Context.** A re-run hits EntityAlreadyExists.\n",
		Checksum:   "sha256:deadbeef",
	}

	id, err := kb.AddRecord(want)
	if err != nil {
		t.Fatalf("AddRecord: %v", err)
	}

	got, err := kb.RecordByID(id)
	if err != nil {
		t.Fatalf("RecordByID: %v", err)
	}

	for _, tc := range []struct{ field, got, want string }{
		{"RecordID", got.RecordID, want.RecordID},
		{"Scope", got.Scope, want.Scope},
		{"Path", got.Path, want.Path},
		{"Title", got.Title, want.Title},
		{"Date", got.Date, want.Date},
		{"Status", got.Status, want.Status},
		{"Kind", got.Kind, want.Kind},
		{"Trigger", got.Trigger, want.Trigger},
		{"Phase", got.Phase, want.Phase},
		{"Initiative", got.Initiative, want.Initiative},
		{"Session", got.Session, want.Session},
		{"Body", got.Body, want.Body},
		{"Checksum", got.Checksum, want.Checksum},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.field, tc.got, tc.want)
		}
	}
	if got.ProjectID != pid {
		t.Errorf("ProjectID = %d, want %d", got.ProjectID, pid)
	}
}

// Plan finding 4: bare 0142 parses as the integer 142 and bare 2026-08-19
// resolves to a timestamp, so both are stored and returned as strings.
func TestAddRecord_PreservesZeroPaddedIDAndDateAsStrings(t *testing.T) {
	kb := openTestKB(t)
	id, err := kb.AddRecord(newTestRecord("0042", "Zero padded", "2026-08-19"))
	if err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	got, err := kb.RecordByID(id)
	if err != nil {
		t.Fatalf("RecordByID: %v", err)
	}
	if got.RecordID != "0042" {
		t.Errorf("RecordID = %q, want %q — zero padding must survive the round trip", got.RecordID, "0042")
	}
	if got.Date != "2026-08-19" {
		t.Errorf("Date = %q, want %q — must not be reformatted as a timestamp", got.Date, "2026-08-19")
	}
}

func TestAddRecord_AssignsUUIDAndOriginHost(t *testing.T) {
	kb := openTestKB(t)
	id, err := kb.AddRecord(newTestRecord("0001", "Needs identity", "2026-08-01"))
	if err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	got, err := kb.RecordByID(id)
	if err != nil {
		t.Fatalf("RecordByID: %v", err)
	}
	if got.UUID == "" {
		t.Error("UUID is empty; records must carry one so merge works across machines")
	}
	if got.OriginHost == "" {
		t.Error("OriginHost is empty; records must carry one so merge works across machines")
	}
}

func TestAddRecord_PreservesSuppliedUUID(t *testing.T) {
	kb := openTestKB(t)
	r := newTestRecord("0008", "Has its own uuid", "2026-06-18")
	r.UUID = "01a03af1-e5cb-73c6-a794-931c837a1c2e"
	r.OriginHost = "MACMINI-RD.local"

	id, err := kb.AddRecord(r)
	if err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	got, err := kb.RecordByID(id)
	if err != nil {
		t.Fatalf("RecordByID: %v", err)
	}
	if got.UUID != r.UUID {
		t.Errorf("UUID = %q, want %q — a uuid from the record file must not be regenerated", got.UUID, r.UUID)
	}
	if got.OriginHost != r.OriginHost {
		t.Errorf("OriginHost = %q, want %q", got.OriginHost, r.OriginHost)
	}
}

// Identity is (project_id, scope, record_id). Re-adding the same record must
// update the existing row, so re-ingesting a tree stays idempotent.
func TestAddRecord_SameIdentityUpdatesInPlace(t *testing.T) {
	kb := openTestKB(t)
	pid, err := kb.AddProject("clasm", "")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	r := newTestRecord("0100", "Original title", "2026-08-01")
	r.ProjectID = pid
	first, err := kb.AddRecord(r)
	if err != nil {
		t.Fatalf("first AddRecord: %v", err)
	}

	r.Title = "Corrected title"
	r.Checksum = "sha256:changed"
	second, err := kb.AddRecord(r)
	if err != nil {
		t.Fatalf("second AddRecord: %v", err)
	}
	if second != first {
		t.Fatalf("re-adding the same (project, scope, record_id) created row %d instead of updating row %d", second, first)
	}

	got, err := kb.RecordByID(first)
	if err != nil {
		t.Fatalf("RecordByID: %v", err)
	}
	if got.Title != "Corrected title" {
		t.Errorf("Title = %q, want %q — the update did not land", got.Title, "Corrected title")
	}
	if n := countRecords(t, kb); n != 1 {
		t.Errorf("records table holds %d rows, want 1", n)
	}
}

// The workspace tier stores project_id as NULL. SQLite treats NULLs in a
// UNIQUE index as distinct from one another, so a plain
// UNIQUE(project_id, scope, record_id) does not constrain workspace records at
// all and re-ingesting ~/WorkLab/agents/decisions/ would duplicate all six.
func TestAddRecord_WorkspaceScopeSameIdentityUpdatesInPlace(t *testing.T) {
	kb := openTestKB(t)

	r := newTestRecord("0001", "Make every index column field-addressable", "2026-08-25")
	r.Scope = "workspace"
	r.ProjectID = 0 // stored as NULL

	first, err := kb.AddRecord(r)
	if err != nil {
		t.Fatalf("first AddRecord: %v", err)
	}

	r.Title = "Make every index column field-addressable, revised"
	second, err := kb.AddRecord(r)
	if err != nil {
		t.Fatalf("second AddRecord: %v", err)
	}
	if second != first {
		t.Errorf("re-adding workspace record 0001 created row %d instead of updating row %d; a NULL project_id must not defeat the identity index", second, first)
	}
	if n := countRecords(t, kb); n != 1 {
		t.Errorf("records table holds %d rows, want 1 — NULL project_id defeated the unique index", n)
	}
}

// Plan finding 3: ids are identity, not chronology. clasm DR-0159 supersedes
// DR-0160, so a correction can carry a lower id than the record it corrects.
// Every ordered query sorts by date then record_id, never record_id alone.
func TestRecordsByProject_SortsByDateThenRecordID(t *testing.T) {
	kb := openTestKB(t)
	pid, err := kb.AddProject("clasm", "")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	for _, r := range []struct{ recordID, date string }{
		{"0160", "2026-08-18"},
		{"0159", "2026-08-19"},
		{"0158", "2026-08-19"},
	} {
		rec := newTestRecord(r.recordID, "Record "+r.recordID, r.date)
		rec.ProjectID = pid
		if _, err := kb.AddRecord(rec); err != nil {
			t.Fatalf("AddRecord %s: %v", r.recordID, err)
		}
	}

	got, err := kb.RecordsByProject(pid)
	if err != nil {
		t.Fatalf("RecordsByProject: %v", err)
	}
	want := []string{"0160", "0158", "0159"} // date ascending, then record_id ascending
	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].RecordID != want[i] {
			t.Errorf("position %d = %s, want %s (sort must be date then record_id)", i, got[i].RecordID, want[i])
		}
	}
}

func TestUpdateRecordStatus(t *testing.T) {
	kb := openTestKB(t)
	r := newTestRecord("0148", "Superseded later", "2026-08-01")
	r.Status = "accepted"
	id, err := kb.AddRecord(r)
	if err != nil {
		t.Fatalf("AddRecord: %v", err)
	}

	if err := kb.UpdateRecordStatus(id, "superseded"); err != nil {
		t.Fatalf("UpdateRecordStatus: %v", err)
	}
	got, err := kb.RecordByID(id)
	if err != nil {
		t.Fatalf("RecordByID: %v", err)
	}
	if got.Status != "superseded" {
		t.Errorf("Status = %q, want %q", got.Status, "superseded")
	}
}

func TestUpdateRecordStatus_UnknownRecord(t *testing.T) {
	kb := openTestKB(t)
	if err := kb.UpdateRecordStatus(9999, "accepted"); err == nil {
		t.Error("UpdateRecordStatus on a missing record returned nil, want an error")
	}
}

// Only one direction is stored. superseded_by is the inverse of supersedes,
// computed on read.
func TestRelationsFor_ReportsBothDirections(t *testing.T) {
	kb := openTestKB(t)
	newer, err := kb.AddRecord(newTestRecord("0159", "The correction", "2026-08-19"))
	if err != nil {
		t.Fatalf("AddRecord newer: %v", err)
	}
	older, err := kb.AddRecord(newTestRecord("0160", "The corrected", "2026-08-18"))
	if err != nil {
		t.Fatalf("AddRecord older: %v", err)
	}

	if err := kb.AddRecordRelation(newer, older, "supersedes"); err != nil {
		t.Fatalf("AddRecordRelation: %v", err)
	}

	fromNewer, err := kb.RelationsFor(newer)
	if err != nil {
		t.Fatalf("RelationsFor(newer): %v", err)
	}
	if len(fromNewer) != 1 {
		t.Fatalf("RelationsFor(newer) returned %d relations, want 1", len(fromNewer))
	}
	if fromNewer[0].Relationship != "supersedes" || fromNewer[0].RecordID != older {
		t.Errorf("RelationsFor(newer) = %+v, want {RecordID:%d Relationship:supersedes}", fromNewer[0], older)
	}

	fromOlder, err := kb.RelationsFor(older)
	if err != nil {
		t.Fatalf("RelationsFor(older): %v", err)
	}
	if len(fromOlder) != 1 {
		t.Fatalf("RelationsFor(older) returned %d relations, want 1", len(fromOlder))
	}
	if fromOlder[0].Relationship != "superseded_by" || fromOlder[0].RecordID != newer {
		t.Errorf("RelationsFor(older) = %+v, want {RecordID:%d Relationship:superseded_by}", fromOlder[0], newer)
	}
}

// relates_to is symmetric: one stored row is reported from both sides.
func TestRelationsFor_RelatesToIsSymmetric(t *testing.T) {
	kb := openTestKB(t)
	a, err := kb.AddRecord(newTestRecord("0003", "A", "2026-06-18"))
	if err != nil {
		t.Fatalf("AddRecord A: %v", err)
	}
	b, err := kb.AddRecord(newTestRecord("0009", "B", "2026-06-18"))
	if err != nil {
		t.Fatalf("AddRecord B: %v", err)
	}
	if err := kb.AddRecordRelation(a, b, "relates_to"); err != nil {
		t.Fatalf("AddRecordRelation: %v", err)
	}

	fromB, err := kb.RelationsFor(b)
	if err != nil {
		t.Fatalf("RelationsFor(b): %v", err)
	}
	if len(fromB) != 1 || fromB[0].Relationship != "relates_to" || fromB[0].RecordID != a {
		t.Errorf("RelationsFor(b) = %+v, want one {RecordID:%d Relationship:relates_to}", fromB, a)
	}
}

// Plan finding 1: superseded_by does not imply status: superseded. clasm
// DR-0160 is accepted and carries superseded_by: ["0159"], because a later
// record can invalidate one decision inside a multi-decision episode while the
// rest stand. Storing the relation must not touch either record's status.
func TestAddRecordRelation_DoesNotChangeStatus(t *testing.T) {
	kb := openTestKB(t)
	newer, err := kb.AddRecord(newTestRecord("0159", "The correction", "2026-08-19"))
	if err != nil {
		t.Fatalf("AddRecord newer: %v", err)
	}
	older := newTestRecord("0160", "Partially superseded", "2026-08-18")
	older.Status = "accepted"
	olderID, err := kb.AddRecord(older)
	if err != nil {
		t.Fatalf("AddRecord older: %v", err)
	}

	if err := kb.AddRecordRelation(newer, olderID, "supersedes"); err != nil {
		t.Fatalf("AddRecordRelation: %v", err)
	}

	got, err := kb.RecordByID(olderID)
	if err != nil {
		t.Fatalf("RecordByID: %v", err)
	}
	if got.Status != "accepted" {
		t.Errorf("Status = %q, want %q — superseded_by must not imply status superseded", got.Status, "accepted")
	}
}

func TestAddRecordRelation_DuplicateIsNoOp(t *testing.T) {
	kb := openTestKB(t)
	a, err := kb.AddRecord(newTestRecord("0001", "A", "2026-08-01"))
	if err != nil {
		t.Fatalf("AddRecord A: %v", err)
	}
	b, err := kb.AddRecord(newTestRecord("0002", "B", "2026-08-02"))
	if err != nil {
		t.Fatalf("AddRecord B: %v", err)
	}

	if err := kb.AddRecordRelation(a, b, "relates_to"); err != nil {
		t.Fatalf("first AddRecordRelation: %v", err)
	}
	if err := kb.AddRecordRelation(a, b, "relates_to"); err != nil {
		t.Fatalf("second AddRecordRelation should be a no-op, got: %v", err)
	}

	rels, err := kb.RelationsFor(a)
	if err != nil {
		t.Fatalf("RelationsFor: %v", err)
	}
	if len(rels) != 1 {
		t.Errorf("got %d relations after adding the same edge twice, want 1", len(rels))
	}
}

func TestAddRecordRelation_UnknownRecordFails(t *testing.T) {
	kb := openTestKB(t)
	a, err := kb.AddRecord(newTestRecord("0001", "A", "2026-08-01"))
	if err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	if err := kb.AddRecordRelation(a, 9999, "relates_to"); err == nil {
		t.Error("AddRecordRelation against a missing record returned nil, want an error")
	}
}

func TestRecordByID_NotFound(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.RecordByID(9999); err == nil {
		t.Error("RecordByID on a missing record returned nil error, want one")
	}
}

// countRecords returns the number of rows in the records table.
func countRecords(t *testing.T, kb *KnowledgeBase) int {
	t.Helper()
	var n int
	if err := kb.db.QueryRow(`SELECT COUNT(*) FROM records`).Scan(&n); err != nil {
		t.Fatalf("counting records: %v", err)
	}
	return n
}

// legacyDBPath returns the real knowledge base to migration-test against,
// overridable with KB_LEGACY_DB. The test skips when it is absent, so the
// suite still passes on a machine without ~/WorkLab checked out.
func legacyDBPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("KB_LEGACY_DB"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory: %v", err)
	}
	return filepath.Join(home, "WorkLab", "agents", "knowledge.db")
}

// copyFile copies src to dst, skipping silently when src does not exist.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatalf("copy %s to %s: %v", src, dst, err)
	}
}

// dropRecordTables removes the decision-record tables from a database file, so
// that a copy of a database which has already been migrated can be used to
// exercise the migration again. Operates on the given path only; callers pass
// a copy.
func dropRecordTables(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS record_relations`,
		`DROP TABLE IF EXISTS records`,
		`DELETE FROM kb_fts WHERE source_type = 'record'`,
	} {
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "no such table") {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

// userTableCounts returns a row count per user table, read without applying
// this package's migrations.
func userTableCounts(t *testing.T, path string) map[string]int {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer db.Close()

	rows, err := db.Query(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'kb_fts%'`,
	)
	if err != nil {
		t.Fatalf("listing tables in %s: %v", path, err)
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			t.Fatalf("scanning table name: %v", err)
		}
		names = append(names, n)
	}
	rows.Close()

	counts := make(map[string]int, len(names))
	for _, n := range names {
		var c int
		if err := db.QueryRow(`SELECT COUNT(*) FROM "` + n + `"`).Scan(&c); err != nil {
			t.Fatalf("counting %s: %v", n, err)
		}
		counts[n] = c
	}
	return counts
}

// Opening a real, populated knowledge base must add the two new tables and
// leave every existing row untouched.
func TestOpen_ExistingKnowledgeBaseIsUnharmed(t *testing.T) {
	src := legacyDBPath(t)
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no legacy knowledge base at %s; set KB_LEGACY_DB to run this test", src)
	}

	dir := t.TempDir()
	dst := filepath.Join(dir, "knowledge.db")
	copyFile(t, src, dst)
	copyFile(t, src+"-wal", dst+"-wal")
	copyFile(t, src+"-shm", dst+"-shm")

	// The real database has had records ingested into it since this test was
	// written, so the copy is returned to a genuine pre-migration state before
	// the migration is exercised. Dropping from the copy, never the source.
	dropRecordTables(t, dst)

	before := userTableCounts(t, dst)
	if before["observations"] == 0 || before["projects"] == 0 {
		t.Fatalf("legacy database at %s looks empty: %v", src, before)
	}
	if _, ok := before["records"]; ok {
		t.Fatalf("dropping the record tables from the copy did not take effect")
	}

	kb, err := Open(dst)
	if err != nil {
		t.Fatalf("Open on an existing knowledge base: %v", err)
	}
	for _, name := range []string{"records", "record_relations"} {
		if !tableExists(t, kb, "table", name) {
			t.Errorf("Open did not create table %q on an existing database", name)
		}
	}
	if err := kb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	after := userTableCounts(t, dst)
	for name, want := range before {
		got, ok := after[name]
		if !ok {
			t.Errorf("table %q disappeared", name)
			continue
		}
		if got != want {
			t.Errorf("table %q holds %d rows after Open, want %d", name, got, want)
		}
	}
	if n := after["records"]; n != 0 {
		t.Errorf("records table holds %d rows on a fresh migration, want 0", n)
	}
}
