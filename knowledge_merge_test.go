package knowledge

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestCollisionReport_DetectsNameUUIDMismatch(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)

	if _, err := a.AddProject("harvey", "on a"); err != nil {
		t.Fatalf("AddProject a: %v", err)
	}
	if _, err := b.AddProject("harvey", "on b"); err != nil {
		t.Fatalf("AddProject b: %v", err)
	}

	collisions, err := CollisionReport(a.Path(), b.Path())
	if err != nil {
		t.Fatalf("CollisionReport: %v", err)
	}
	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d: %v", len(collisions), collisions)
	}
	c := collisions[0]
	if c.Table != "projects" || c.Label != "harvey" {
		t.Errorf("unexpected collision: %+v", c)
	}
	if c.UUIDA == "" || c.UUIDB == "" || c.UUIDA == c.UUIDB {
		t.Errorf("expected two distinct non-empty uuids, got a=%q b=%q", c.UUIDA, c.UUIDB)
	}
}

func TestCollisionReport_NoCollisionWhenUUIDsMatch(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)

	aid, err := a.AddProject("harvey", "on a")
	if err != nil {
		t.Fatalf("AddProject a: %v", err)
	}
	if _, err := b.AddProject("harvey", "on b"); err != nil {
		t.Fatalf("AddProject b: %v", err)
	}

	var aUUID string
	if err := a.db.QueryRow(`SELECT uuid FROM projects WHERE id = ?`, aid).Scan(&aUUID); err != nil {
		t.Fatalf("select a uuid: %v", err)
	}
	// Simulate a row already reconciled by a prior merge: b's copy shares a's uuid.
	if _, err := b.db.Exec(`UPDATE projects SET uuid = ? WHERE name = 'harvey'`, aUUID); err != nil {
		t.Fatalf("update b uuid: %v", err)
	}

	collisions, err := CollisionReport(a.Path(), b.Path())
	if err != nil {
		t.Fatalf("CollisionReport: %v", err)
	}
	if len(collisions) != 0 {
		t.Errorf("expected 0 collisions, got %d: %v", len(collisions), collisions)
	}
}

func TestCollisionReport_ConceptsAlsoChecked(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)

	if _, err := a.AddConcept("RAG", "on a"); err != nil {
		t.Fatalf("AddConcept a: %v", err)
	}
	if _, err := b.AddConcept("RAG", "on b"); err != nil {
		t.Fatalf("AddConcept b: %v", err)
	}

	collisions, err := CollisionReport(a.Path(), b.Path())
	if err != nil {
		t.Fatalf("CollisionReport: %v", err)
	}
	if len(collisions) != 1 || collisions[0].Table != "concepts" || collisions[0].Label != "RAG" {
		t.Errorf("unexpected collisions: %v", collisions)
	}
}

func TestCollisionReport_EmptyWhenNoOverlap(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)

	if _, err := a.AddProject("proj-a", ""); err != nil {
		t.Fatalf("AddProject a: %v", err)
	}
	if _, err := b.AddProject("proj-b", ""); err != nil {
		t.Fatalf("AddProject b: %v", err)
	}

	collisions, err := CollisionReport(a.Path(), b.Path())
	if err != nil {
		t.Fatalf("CollisionReport: %v", err)
	}
	if len(collisions) != 0 {
		t.Errorf("expected 0 collisions, got %d: %v", len(collisions), collisions)
	}
}

func TestMergeKnowledgeBases_RejectsExistingMergedPath(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)

	mergedPath := filepath.Join(t.TempDir(), "merged.db")
	if err := os.WriteFile(mergedPath, []byte{}, 0o644); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}

	if _, err := MergeKnowledgeBases(a.Path(), b.Path(), mergedPath); err == nil {
		t.Fatal("expected error when mergedPath already exists, got nil")
	}

	info, err := os.Stat(mergedPath)
	if err != nil {
		t.Fatalf("stat mergedPath: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("expected mergedPath to remain untouched (0 bytes), got %d", info.Size())
	}
}

func TestMergeKnowledgeBases_CreatesMigratedSchema(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)

	mergedPath := filepath.Join(t.TempDir(), "merged.db")
	if _, err := MergeKnowledgeBases(a.Path(), b.Path(), mergedPath); err != nil {
		t.Fatalf("MergeKnowledgeBases: %v", err)
	}

	merged, err := Open(mergedPath)
	if err != nil {
		t.Fatalf("open merged: %v", err)
	}
	defer merged.Close()

	projects, err := merged.Projects()
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if projects == nil {
		t.Error("expected empty (not nil) projects slice from a fully-migrated merged db")
	}
}

func openMergedTestKB(t *testing.T, a, b *KnowledgeBase) *KnowledgeBase {
	t.Helper()
	mergedPath := filepath.Join(t.TempDir(), "merged.db")
	if _, err := MergeKnowledgeBases(a.Path(), b.Path(), mergedPath); err != nil {
		t.Fatalf("MergeKnowledgeBases: %v", err)
	}
	merged, err := Open(mergedPath)
	if err != nil {
		t.Fatalf("open merged: %v", err)
	}
	t.Cleanup(func() { merged.Close() })
	return merged
}

func TestMergeKnowledgeBases_ProjectsUnion(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	if _, err := a.AddProject("one", ""); err != nil {
		t.Fatalf("AddProject a: %v", err)
	}
	if _, err := b.AddProject("two", ""); err != nil {
		t.Fatalf("AddProject b: %v", err)
	}

	merged := openMergedTestKB(t, a, b)
	projects, err := merged.Projects()
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d: %v", len(projects), projects)
	}
	names := map[string]bool{}
	for _, p := range projects {
		names[p.Name] = true
		if p.ID == 0 {
			t.Errorf("expected a fresh non-zero merged id for project %q", p.Name)
		}
	}
	if !names["one"] || !names["two"] {
		t.Errorf("expected both projects present, got %v", names)
	}
}

func TestMergeKnowledgeBases_ProjectsDedupSharedUUID(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, err := a.AddProject("shared", "on a")
	if err != nil {
		t.Fatalf("AddProject a: %v", err)
	}
	var aUUID string
	if err := a.db.QueryRow(`SELECT uuid FROM projects WHERE id = ?`, aid).Scan(&aUUID); err != nil {
		t.Fatalf("select a uuid: %v", err)
	}
	if _, err := b.AddProject("shared", "on b"); err != nil {
		t.Fatalf("AddProject b: %v", err)
	}
	if _, err := b.db.Exec(`UPDATE projects SET uuid = ? WHERE name = 'shared'`, aUUID); err != nil {
		t.Fatalf("update b uuid: %v", err)
	}

	merged := openMergedTestKB(t, a, b)
	projects, err := merged.Projects()
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 deduped project, got %d: %v", len(projects), projects)
	}
}

func TestMergeKnowledgeBases_ConceptsAndSourcesUnion(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	if _, err := a.AddConcept("concept-a", ""); err != nil {
		t.Fatalf("AddConcept a: %v", err)
	}
	if _, err := b.AddConcept("concept-b", ""); err != nil {
		t.Fatalf("AddConcept b: %v", err)
	}
	if _, err := a.AddSource(Source{Title: "source-a"}); err != nil {
		t.Fatalf("AddSource a: %v", err)
	}
	if _, err := b.AddSource(Source{Title: "source-b"}); err != nil {
		t.Fatalf("AddSource b: %v", err)
	}

	merged := openMergedTestKB(t, a, b)

	concepts, err := merged.Concepts()
	if err != nil {
		t.Fatalf("Concepts: %v", err)
	}
	if len(concepts) != 2 {
		t.Fatalf("expected 2 concepts, got %d: %v", len(concepts), concepts)
	}

	sources, err := merged.ListSources()
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d: %v", len(sources), sources)
	}
}

func TestMergeKnowledgeBases_ObservationsTranslateProjectID(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	pid, err := a.AddProject("proj", "")
	if err != nil {
		t.Fatalf("AddProject a: %v", err)
	}
	if _, err := a.AddObservation(pid, "note", "obs on a"); err != nil {
		t.Fatalf("AddObservation a: %v", err)
	}

	merged := openMergedTestKB(t, a, b)

	var mergedPID int64
	if err := merged.db.QueryRow(`SELECT id FROM projects WHERE name = 'proj'`).Scan(&mergedPID); err != nil {
		t.Fatalf("select merged project id: %v", err)
	}

	var gotPID int64
	if err := merged.db.QueryRow(`SELECT project_id FROM observations WHERE body = 'obs on a'`).Scan(&gotPID); err != nil {
		t.Fatalf("select observation project_id: %v", err)
	}
	if gotPID != mergedPID {
		t.Errorf("observation.project_id = %d, want merged project id %d (source id was %d)", gotPID, mergedPID, pid)
	}
}

func TestMergeKnowledgeBases_ObservationsDedupSharedUUID(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	apid, err := a.AddProject("proj", "")
	if err != nil {
		t.Fatalf("AddProject a: %v", err)
	}
	oid, err := a.AddObservation(apid, "note", "shared obs")
	if err != nil {
		t.Fatalf("AddObservation a: %v", err)
	}
	var oUUID string
	if err := a.db.QueryRow(`SELECT uuid FROM observations WHERE id = ?`, oid).Scan(&oUUID); err != nil {
		t.Fatalf("select observation uuid: %v", err)
	}

	bpid, err := b.AddProject("proj", "")
	if err != nil {
		t.Fatalf("AddProject b: %v", err)
	}
	// Same project by name (dedups to one merged project), and the same
	// observation (by uuid) already present on both, simulating a row
	// already reconciled by a prior merge.
	if _, err := b.db.Exec(
		`INSERT INTO observations (project_id, kind, body, uuid, origin_host) VALUES (?, 'note', 'shared obs', ?, 'unknown')`,
		bpid, oUUID,
	); err != nil {
		t.Fatalf("insert b observation: %v", err)
	}

	merged := openMergedTestKB(t, a, b)
	var count int
	if err := merged.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE body = 'shared obs'`).Scan(&count); err != nil {
		t.Fatalf("count observations: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 deduped observation, got %d", count)
	}
}

func TestMergeKnowledgeBases_ObservationConceptsSurvive(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	pid, err := a.AddProject("proj", "")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	oid, err := a.AddObservation(pid, "note", "linked obs")
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}
	cid, err := a.AddConcept("concept", "")
	if err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	if err := a.LinkObservationConcept(oid, cid); err != nil {
		t.Fatalf("LinkObservationConcept: %v", err)
	}

	merged := openMergedTestKB(t, a, b)
	var count int
	if err := merged.db.QueryRow(`
		SELECT COUNT(*) FROM observation_concepts j
		JOIN observations o ON o.id = j.observation_id
		JOIN concepts     c ON c.id = j.concept_id
		WHERE o.body = 'linked obs' AND c.name = 'concept'`,
	).Scan(&count); err != nil {
		t.Fatalf("count link: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 observation_concepts link in merged db, got %d", count)
	}
}

func TestMergeKnowledgeBases_ProjectConceptsSurvive(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	pid, err := a.AddProject("proj", "")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	cid, err := a.AddConcept("concept", "")
	if err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	if err := a.LinkProjectConcept(pid, cid); err != nil {
		t.Fatalf("LinkProjectConcept: %v", err)
	}

	merged := openMergedTestKB(t, a, b)
	var count int
	if err := merged.db.QueryRow(`
		SELECT COUNT(*) FROM project_concepts j
		JOIN projects p ON p.id = j.project_id
		JOIN concepts c ON c.id = j.concept_id
		WHERE p.name = 'proj' AND c.name = 'concept'`,
	).Scan(&count); err != nil {
		t.Fatalf("count link: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 project_concepts link in merged db, got %d", count)
	}
}

func TestMergeKnowledgeBases_ObservationSourcesSurvive(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	pid, err := a.AddProject("proj", "")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	oid, err := a.AddObservation(pid, "note", "sourced obs")
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}
	sid, err := a.AddSource(Source{Title: "src"})
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if err := a.LinkObservationSource(oid, sid, "supports"); err != nil {
		t.Fatalf("LinkObservationSource: %v", err)
	}

	merged := openMergedTestKB(t, a, b)
	var relationship string
	if err := merged.db.QueryRow(`
		SELECT j.relationship FROM observation_sources j
		JOIN observations o ON o.id = j.observation_id
		JOIN sources      s ON s.id = j.source_id
		WHERE o.body = 'sourced obs' AND s.title = 'src'`,
	).Scan(&relationship); err != nil {
		t.Fatalf("select link: %v", err)
	}
	if relationship != "supports" {
		t.Errorf("relationship = %q, want %q", relationship, "supports")
	}
}

func TestMergeKnowledgeBases_SummaryCounts(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)

	aid, err := a.AddProject("shared", "on a")
	if err != nil {
		t.Fatalf("AddProject a shared: %v", err)
	}
	if _, err := a.AddProject("only-a", ""); err != nil {
		t.Fatalf("AddProject a only-a: %v", err)
	}
	var aUUID string
	if err := a.db.QueryRow(`SELECT uuid FROM projects WHERE id = ?`, aid).Scan(&aUUID); err != nil {
		t.Fatalf("select a uuid: %v", err)
	}
	if _, err := b.AddProject("shared", "on b"); err != nil {
		t.Fatalf("AddProject b shared: %v", err)
	}
	if _, err := b.db.Exec(`UPDATE projects SET uuid = ? WHERE name = 'shared'`, aUUID); err != nil {
		t.Fatalf("update b uuid: %v", err)
	}

	mergedPath := filepath.Join(t.TempDir(), "merged.db")
	summary, err := MergeKnowledgeBases(a.Path(), b.Path(), mergedPath)
	if err != nil {
		t.Fatalf("MergeKnowledgeBases: %v", err)
	}

	var projSummary *MergeTableSummary
	for i := range summary {
		if summary[i].Table == "projects" {
			projSummary = &summary[i]
		}
	}
	if projSummary == nil {
		t.Fatalf("no summary entry for projects: %v", summary)
	}
	if projSummary.FromA != 2 || projSummary.FromB != 1 || projSummary.Merged != 2 {
		t.Errorf("projects summary = %+v, want FromA=2 FromB=1 Merged=2", projSummary)
	}
}

func TestMergeKnowledgeBases_FTSPopulatedAfterMerge(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	pid, err := a.AddProject("proj", "")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if _, err := a.AddObservation(pid, "note", "findable via fts search term"); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	merged := openMergedTestKB(t, a, b)
	results, err := merged.Search("findable")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected Search to find the merged observation via FTS, got 0 results")
	}
}

func TestReconcileCollisions_RewritesUUIDToMatchA(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	if _, err := a.AddProject("harvey", "on a"); err != nil {
		t.Fatalf("AddProject a: %v", err)
	}
	if _, err := b.AddProject("harvey", "on b"); err != nil {
		t.Fatalf("AddProject b: %v", err)
	}

	collisions, err := CollisionReport(a.Path(), b.Path())
	if err != nil {
		t.Fatalf("CollisionReport: %v", err)
	}
	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d", len(collisions))
	}

	if err := ReconcileCollisions(b.Path(), collisions); err != nil {
		t.Fatalf("ReconcileCollisions: %v", err)
	}

	var bUUID string
	if err := b.db.QueryRow(`SELECT uuid FROM projects WHERE name = 'harvey'`).Scan(&bUUID); err != nil {
		t.Fatalf("select b uuid: %v", err)
	}
	if bUUID != collisions[0].UUIDA {
		t.Errorf("b uuid = %q, want a's uuid %q", bUUID, collisions[0].UUIDA)
	}

	remaining, err := CollisionReport(a.Path(), b.Path())
	if err != nil {
		t.Fatalf("CollisionReport after reconcile: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 collisions after reconcile, got %d: %v", len(remaining), remaining)
	}
}

func TestReconcileCollisions_PreservesChildObservationsAfterMerge(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	apid, err := a.AddProject("harvey", "on a")
	if err != nil {
		t.Fatalf("AddProject a: %v", err)
	}
	if _, err := a.AddObservation(apid, "note", "obs on a"); err != nil {
		t.Fatalf("AddObservation a: %v", err)
	}
	bpid, err := b.AddProject("harvey", "on b")
	if err != nil {
		t.Fatalf("AddProject b: %v", err)
	}
	if _, err := b.AddObservation(bpid, "note", "obs on b"); err != nil {
		t.Fatalf("AddObservation b: %v", err)
	}

	collisions, err := CollisionReport(a.Path(), b.Path())
	if err != nil {
		t.Fatalf("CollisionReport: %v", err)
	}
	if err := ReconcileCollisions(b.Path(), collisions); err != nil {
		t.Fatalf("ReconcileCollisions: %v", err)
	}

	merged := openMergedTestKB(t, a, b)

	projects, err := merged.Projects()
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 merged project (reconciled), got %d: %v", len(projects), projects)
	}

	var obsCount int
	if err := merged.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE body IN ('obs on a', 'obs on b')`).Scan(&obsCount); err != nil {
		t.Fatalf("count observations: %v", err)
	}
	if obsCount != 2 {
		t.Errorf("expected both observations to survive the merge after reconciliation, got %d", obsCount)
	}
}

// ─── W1: input normalisation before ATTACH (DR-0014) ──────────────────────

// openTestKBAt opens a knowledge base under a *named* workspace root, so a
// test can control what workspaceFromDBPath derives from it. openTestKB's
// random temp directory is fine wherever the workspace name does not matter;
// in these tests it is the thing under test.
func openTestKBAt(t *testing.T, workspaceName string) *KnowledgeBase {
	t.Helper()
	kb, err := Open(DefaultPath(filepath.Join(t.TempDir(), workspaceName)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { kb.Close() })
	return kb
}

// copyDBForTest stands in for cmd/kb's checkpointAndCopy: it puts a snapshot
// of a database somewhere that is deliberately not where it came from, which
// is the whole reason the workspace name has to travel separately.
func copyDBForTest(t *testing.T, srcPath, dstPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(dstPath), err)
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read %s: %v", srcPath, err)
	}
	if err := os.WriteFile(dstPath, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", dstPath, err)
	}
}

func TestNormalizeForMerge_AddsRecordsTablesToPreRecordsDatabase(t *testing.T) {
	kb := openTestKB(t)
	// A database written before decision records existed. wren.local's was in
	// exactly this state as recently as 2026-08, and SELECT ... FROM b.records
	// against it is a hard error rather than an empty result.
	for _, stmt := range []string{`DROP TABLE record_relations`, `DROP TABLE records`} {
		if _, err := kb.db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	path := kb.Path()
	kb.Close()

	if err := NormalizeForMerge(path, path); err != nil {
		t.Fatalf("NormalizeForMerge: %v", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open normalized: %v", err)
	}
	defer db.Close()
	for _, table := range []string{"records", "record_relations"} {
		var name string
		if err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table,
		).Scan(&name); err != nil {
			t.Errorf("table %s missing after normalization: %v", table, err)
		}
	}
}

func TestNormalizeForMerge_BackfillsWorkspaceFromOriginalPath(t *testing.T) {
	kb := openTestKBAt(t, "Laboratory")
	if _, err := kb.AddRecord(Record{
		RecordID: "0001", Scope: "workspace",
		Path:  "agents/decisions/0001-example.md",
		Title: "example", Date: "2026-08-27", Body: "body",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	// A record written before W8 added the workspace column: the migration's
	// backfill is what fills this in, and what it fills in is the question.
	if _, err := kb.db.Exec(`UPDATE records SET workspace = ''`); err != nil {
		t.Fatalf("clear workspace: %v", err)
	}
	origPath := kb.Path()
	kb.Close()

	scratch := filepath.Join(t.TempDir(), "kbmerge-XXXX", "a.db")
	copyDBForTest(t, origPath, scratch)

	if err := NormalizeForMerge(scratch, origPath); err != nil {
		t.Fatalf("NormalizeForMerge: %v", err)
	}

	db, err := sql.Open("sqlite", scratch)
	if err != nil {
		t.Fatalf("open normalized: %v", err)
	}
	defer db.Close()
	var ws string
	if err := db.QueryRow(
		`SELECT workspace FROM records WHERE record_id = '0001'`,
	).Scan(&ws); err != nil {
		t.Fatalf("select workspace: %v", err)
	}
	if ws != "Laboratory" {
		t.Errorf("workspace = %q, want %q: the name must be derived from the original path, not from the copy's location", ws, "Laboratory")
	}
}

func TestNormalizeForMerge_LeavesPopulatedWorkspaceAlone(t *testing.T) {
	kb := openTestKBAt(t, "Laboratory")
	// Another workspace's record, legitimately present because ingest's
	// --root may name a workspace other than the one the database sits in
	// (DR-0011). Normalizing must not relabel it.
	if _, err := kb.AddRecord(Record{
		RecordID: "0001", Scope: "workspace", Workspace: "WorkLab",
		Path:  "agents/decisions/0001-example.md",
		Title: "example", Date: "2026-08-27", Body: "body",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	origPath := kb.Path()
	kb.Close()

	scratch := filepath.Join(t.TempDir(), "kbmerge-XXXX", "a.db")
	copyDBForTest(t, origPath, scratch)

	if err := NormalizeForMerge(scratch, origPath); err != nil {
		t.Fatalf("NormalizeForMerge: %v", err)
	}

	db, err := sql.Open("sqlite", scratch)
	if err != nil {
		t.Fatalf("open normalized: %v", err)
	}
	defer db.Close()
	var ws string
	if err := db.QueryRow(
		`SELECT workspace FROM records WHERE record_id = '0001'`,
	).Scan(&ws); err != nil {
		t.Fatalf("select workspace: %v", err)
	}
	if ws != "WorkLab" {
		t.Errorf("workspace = %q, want %q: a populated workspace is not the migration's to rewrite", ws, "WorkLab")
	}
}

// ─── W2: records and record_relations union (DR-0013) ─────────────────────

// addTestRecord inserts a record with just enough fields to be identifiable
// after a merge. ProjectID 0 means the workspace tier, stored as NULL.
func addTestRecord(t *testing.T, kb *KnowledgeBase, r Record) int64 {
	t.Helper()
	if r.Date == "" {
		r.Date = "2026-08-27"
	}
	if r.Path == "" {
		r.Path = "decisions/" + r.RecordID + "-" + r.Title + ".md"
	}
	id, err := kb.AddRecord(r)
	if err != nil {
		t.Fatalf("AddRecord %s: %v", r.RecordID, err)
	}
	return id
}

// recordTitles returns the merged database's record titles, sorted by the
// query's own date/id ordering, for comparison in tests.
func recordTitles(t *testing.T, kb *KnowledgeBase) []string {
	t.Helper()
	recs, err := kb.ListRecords(RecordFilter{})
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	var out []string
	for _, r := range recs {
		out = append(out, r.Title)
	}
	return out
}

// The motivating case: one project, known to both machines, each holding
// records the other has never seen. The two sides' projects must share a uuid
// for this to be a union rather than a collision -- an unreconciled name
// collision drops b's project and orphans its records, exactly as it already
// does to b's observations, and reconciling it is -force's job (DR-0013, W4).
func TestMergeKnowledgeBases_RecordsUnion(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, _ := a.AddProject("harvey", "")
	bid, _ := b.AddProject("harvey", "")
	shareProjectUUID(t, a, aid, b, bid)
	addTestRecord(t, a, Record{RecordID: "0001", ProjectID: aid, Title: "only-on-a"})
	addTestRecord(t, b, Record{RecordID: "0002", ProjectID: bid, Title: "only-on-b"})

	merged := openMergedTestKB(t, a, b)
	got := recordTitles(t, merged)
	if len(got) != 2 {
		t.Fatalf("merged records = %v, want both sides' records", got)
	}
}

// shareProjectUUID rewrites b's project to carry a's uuid, standing in for a
// pair of machines whose shared project has already been reconciled.
func shareProjectUUID(t *testing.T, a *KnowledgeBase, aid int64, b *KnowledgeBase, bid int64) {
	t.Helper()
	var aUUID string
	if err := a.db.QueryRow(`SELECT uuid FROM projects WHERE id = ?`, aid).Scan(&aUUID); err != nil {
		t.Fatalf("select a uuid: %v", err)
	}
	if _, err := b.db.Exec(`UPDATE projects SET uuid = ? WHERE id = ?`, aUUID, bid); err != nil {
		t.Fatalf("update b uuid: %v", err)
	}
}

func TestMergeKnowledgeBases_RecordsDedupSharedUUID(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, _ := a.AddProject("harvey", "")
	bid, _ := b.AddProject("harvey", "")
	shared := "01a00000-0000-7000-8000-00000000dead"
	addTestRecord(t, a, Record{RecordID: "0001", ProjectID: aid, Title: "same-record", UUID: shared})
	addTestRecord(t, b, Record{RecordID: "0001", ProjectID: bid, Title: "same-record", UUID: shared})

	merged := openMergedTestKB(t, a, b)
	if got := recordTitles(t, merged); len(got) != 1 {
		t.Errorf("merged records = %v, want one -- the same uuid is the same record", got)
	}
}

// The workspace tier has no project, so records.project_id is NULL. Copying
// the observations path's INNER JOIN through projects would drop every such
// record while leaving the count plausible (DR-0013). This test fails against
// that implementation and is the reason the join is a LEFT JOIN.
func TestMergeKnowledgeBases_WorkspaceTierRecordSurvives(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, _ := a.AddProject("harvey", "")
	addTestRecord(t, a, Record{RecordID: "0001", ProjectID: aid, Scope: "project", Title: "project-tier"})
	addTestRecord(t, a, Record{RecordID: "0001", ProjectID: 0, Scope: "workspace", Title: "workspace-tier"})

	merged := openMergedTestKB(t, a, b)
	recs, err := merged.ListRecords(RecordFilter{Scope: "workspace"})
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("workspace-tier records = %d, want 1: a NULL project_id must survive the merge", len(recs))
	}
	if recs[0].Title != "workspace-tier" {
		t.Errorf("title = %q, want %q", recs[0].Title, "workspace-tier")
	}
	if recs[0].ProjectID != 0 {
		t.Errorf("ProjectID = %d, want 0: the workspace tier must not acquire a project", recs[0].ProjectID)
	}
}

func TestMergeKnowledgeBases_RecordsTranslateProjectID(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, _ := a.AddProject("harvey", "")
	addTestRecord(t, a, Record{RecordID: "0001", ProjectID: aid, Scope: "project", Title: "attached"})

	merged := openMergedTestKB(t, a, b)
	recs, err := merged.ListRecords(RecordFilter{Project: "harvey"})
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("records for harvey = %d, want 1: the record must follow its project's merged id", len(recs))
	}
}

func TestMergeKnowledgeBases_RecordRelationsSurvive(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, _ := a.AddProject("harvey", "")
	from := addTestRecord(t, a, Record{RecordID: "0002", ProjectID: aid, Title: "superseding"})
	to := addTestRecord(t, a, Record{RecordID: "0001", ProjectID: aid, Title: "superseded"})
	if err := a.AddRecordRelation(from, to, "supersedes"); err != nil {
		t.Fatalf("AddRecordRelation: %v", err)
	}

	merged := openMergedTestKB(t, a, b)
	recs, err := merged.ListRecords(RecordFilter{})
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	var mergedFrom int64
	for _, r := range recs {
		if r.Title == "superseding" {
			mergedFrom = r.ID
		}
	}
	if mergedFrom == 0 {
		t.Fatal("superseding record did not survive the merge")
	}
	rels, err := merged.RelationsFor(mergedFrom)
	if err != nil {
		t.Fatalf("RelationsFor: %v", err)
	}
	if len(rels) != 1 || rels[0].Relationship != "supersedes" {
		t.Errorf("relations = %+v, want one supersedes edge remapped through record uuids", rels)
	}
}

func TestMergeKnowledgeBases_SummaryIncludesRecordTables(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	mergedPath := filepath.Join(t.TempDir(), "merged.db")
	summary, err := MergeKnowledgeBases(a.Path(), b.Path(), mergedPath)
	if err != nil {
		t.Fatalf("MergeKnowledgeBases: %v", err)
	}
	seen := map[string]bool{}
	for _, s := range summary {
		seen[s.Table] = true
	}
	// A table absent from the summary is a table whose loss goes unreported,
	// which is how records came to be dropped silently in the first place.
	for _, want := range []string{"records", "record_relations"} {
		if !seen[want] {
			t.Errorf("summary omits %s: %v", want, seen)
		}
	}
	if len(summary) != 9 {
		t.Errorf("summary covers %d tables, want 9 (every table that travels)", len(summary))
	}
}

// A relation whose endpoint never made it into the merged database is
// skipped, not fatal -- the same leniency observation_concepts has, and the
// reason the endpoint joins here are INNER rather than LEFT.
func TestMergeKnowledgeBases_RecordRelationUnresolvableEndpointSkipped(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, _ := a.AddProject("harvey", "")
	from := addTestRecord(t, a, Record{RecordID: "0002", ProjectID: aid, Title: "surviving"})
	to := addTestRecord(t, a, Record{RecordID: "0001", ProjectID: aid, Title: "doomed"})
	if err := a.AddRecordRelation(from, to, "supersedes"); err != nil {
		t.Fatalf("AddRecordRelation: %v", err)
	}
	// Drop the target's row but leave the edge behind, standing in for a
	// database whose foreign keys were not enforced on the deleting
	// connection -- the same way 24 observation_concepts rows went dangling.
	if _, err := a.db.Exec(`DELETE FROM records WHERE id = ?`, to); err != nil {
		t.Fatalf("delete target: %v", err)
	}

	merged := openMergedTestKB(t, a, b)
	titles := recordTitles(t, merged)
	if len(titles) != 1 || titles[0] != "surviving" {
		t.Fatalf("merged records = %v, want only the surviving record", titles)
	}
	var edges int
	if err := merged.db.QueryRow(`SELECT COUNT(*) FROM record_relations`).Scan(&edges); err != nil {
		t.Fatalf("count relations: %v", err)
	}
	if edges != 0 {
		t.Errorf("record_relations = %d, want 0: an edge with no target does not travel", edges)
	}
}

// A record naming a project that does not resolve is skipped, as an
// observation would be. The LEFT JOIN exists for the workspace tier's genuine
// NULL, and must not quietly turn a broken project reference into one.
func TestMergeKnowledgeBases_RecordWithUnresolvableProjectIsNotRetiered(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, _ := a.AddProject("harvey", "")
	addTestRecord(t, a, Record{RecordID: "0001", ProjectID: aid, Scope: "project", Title: "dangling"})
	// SQLite enforces foreign keys per connection, not persistently in the
	// file, so ON DELETE SET NULL only fires if the deleting connection had
	// them on. Dropping the project with them off leaves the record pointing
	// at a project that no longer exists -- the same way 24 dangling
	// observation_concepts rows came to be in the live database.
	for _, stmt := range []string{
		`PRAGMA foreign_keys = OFF`,
		`DELETE FROM projects WHERE id = ` + strconv.FormatInt(aid, 10),
		`PRAGMA foreign_keys = ON`,
	} {
		if _, err := a.db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	merged := openMergedTestKB(t, a, b)
	recs, err := merged.ListRecords(RecordFilter{Scope: "workspace"})
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("workspace-tier records = %d, want 0: a broken project reference must not become the workspace tier", len(recs))
	}
}

// ─── W3: records reach kb_fts on the merge path (DR-0013, DR-0008) ────────

// A merge that unions records but leaves kb_fts alone produces a database
// holding every record and finding none of them. kb_fts has no triggers: it
// is written at each call site, with rebuildFTSIfNeeded as the backstop, and
// the merge path depends on that backstop entirely.
func TestMergeKnowledgeBases_RecordsAreSearchableAfterMerge(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, _ := a.AddProject("harvey", "")
	bid, _ := b.AddProject("harvey", "")
	shareProjectUUID(t, a, aid, b, bid)
	addTestRecord(t, a, Record{RecordID: "0001", ProjectID: aid,
		Title: "Streaming responses are chunked", Body: "the body from a"})
	addTestRecord(t, b, Record{RecordID: "0002", ProjectID: bid,
		Title: "Streaming responses are buffered", Body: "the body from b"})

	merged := openMergedTestKB(t, a, b)
	results, err := merged.Search("Streaming")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	labels := map[string]bool{}
	for _, r := range results {
		if r.SourceType == "record" {
			labels[r.Label] = true
		}
	}
	if !labels["DR-0001"] || !labels["DR-0002"] {
		t.Errorf("search found records %v, want both sides' records reachable after a merge", labels)
	}

	// Search returns the title as the snippet, so comparing results alone
	// cannot tell whether the body was indexed at all. Reach for a term that
	// appears only in the body.
	bodyHits, err := merged.Search("the body from b")
	if err != nil {
		t.Fatalf("Search body: %v", err)
	}
	if len(bodyHits) == 0 {
		t.Error("a record's body is not reachable after a merge; the rebuild indexes title and body together")
	}
}

// A record that arrived by merge and one that arrived by ingest must be
// indistinguishable to search. The rebuild path and indexRecordFTS write the
// same row or they do not, and a difference here would show up as a record
// that is findable by one route and not the other.
func TestMergeKnowledgeBases_MergedRecordSearchesLikeAnIngestedOne(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, _ := a.AddProject("harvey", "")
	addTestRecord(t, a, Record{RecordID: "0007", ProjectID: aid,
		Title: "Identity is four columns", Body: "workspace, project, scope, id"})

	ingested, err := a.Search("Identity")
	if err != nil {
		t.Fatalf("Search on source: %v", err)
	}
	merged := openMergedTestKB(t, a, b)
	afterMerge, err := merged.Search("Identity")
	if err != nil {
		t.Fatalf("Search on merged: %v", err)
	}

	if len(ingested) != 1 || len(afterMerge) != 1 {
		t.Fatalf("hits: ingested %d, merged %d, want exactly 1 each", len(ingested), len(afterMerge))
	}
	if ingested[0] != afterMerge[0] {
		t.Errorf("merged record searches differently from the ingested one:\n ingested: %+v\n   merged: %+v",
			ingested[0], afterMerge[0])
	}
}

// DR-0013 left sources out of the rebuild deliberately. Asserting the absence
// keeps that a decision rather than something the next change quietly undoes.
func TestMergeKnowledgeBases_SourcesStayOutOfFTS(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	if _, err := a.AddSource(Source{Title: "A paper about streaming"}); err != nil {
		t.Fatalf("AddSource: %v", err)
	}

	merged := openMergedTestKB(t, a, b)
	var n int
	if err := merged.db.QueryRow(
		`SELECT COUNT(*) FROM kb_fts WHERE source_type = 'source'`,
	).Scan(&n); err != nil {
		t.Fatalf("count source rows: %v", err)
	}
	if n != 0 {
		t.Errorf("kb_fts holds %d source rows; sources are deliberately not indexed (DR-0013)", n)
	}
}

// ─── W4: identity collisions and content divergence (DR-0013) ─────────────

// addSharedRecord writes a record under an explicit workspace, so both sides
// of a test can hold the same record identity. openTestKB derives a different
// workspace per temp directory, which would otherwise make every cross-side
// record identity distinct.
func addSharedRecord(t *testing.T, kb *KnowledgeBase, projectID int64, recordID, title, body string) int64 {
	t.Helper()
	return addTestRecord(t, kb, Record{
		RecordID: recordID, ProjectID: projectID, Scope: "project",
		Workspace: "Laboratory", Title: title, Body: body,
		Checksum: "sum-" + body,
	})
}

func TestCollisionReport_RecordsCollideOnIdentity(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, _ := a.AddProject("harvey", "")
	bid, _ := b.AddProject("harvey", "")
	addSharedRecord(t, a, aid, "0007", "same record", "same")
	addSharedRecord(t, b, bid, "0007", "same record", "same")

	collisions, err := CollisionReport(a.Path(), b.Path())
	if err != nil {
		t.Fatalf("CollisionReport: %v", err)
	}
	var recordHits []IdentityCollision
	for _, c := range collisions {
		if c.Table == "records" {
			recordHits = append(recordHits, c)
		}
	}
	if len(recordHits) != 1 {
		t.Fatalf("record collisions = %+v, want exactly one", recordHits)
	}
	if recordHits[0].Label != "DR-0007" {
		t.Errorf("label = %q, want %q", recordHits[0].Label, "DR-0007")
	}
	if recordHits[0].UUIDA == recordHits[0].UUIDB {
		t.Error("expected two distinct uuids for one identity")
	}
}

// A record's project is matched across databases by the project's name, not
// by its local id and not by its uuid. The id is meaningless across two
// autoincrement sequences; the uuid would make record collisions depend on
// whether the project collision had already been reconciled, so the same two
// databases would report different record collisions before and after -force.
func TestCollisionReport_RecordIdentityUsesProjectNameNotLocalID(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	// Give b's projects a different id sequence, so a matching local id
	// cannot be what makes these two records the same record.
	other, _ := b.AddProject("decoy", "")
	aid, _ := a.AddProject("harvey", "")
	bid, _ := b.AddProject("harvey", "")
	if aid == bid {
		t.Fatalf("fixture needs differing local ids, got %d and %d (decoy=%d)", aid, bid, other)
	}
	addSharedRecord(t, a, aid, "0007", "same record", "same")
	addSharedRecord(t, b, bid, "0007", "same record", "same")

	collisions, err := CollisionReport(a.Path(), b.Path())
	if err != nil {
		t.Fatalf("CollisionReport: %v", err)
	}
	var n int
	for _, c := range collisions {
		if c.Table == "records" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("record collisions = %d, want 1 despite differing local project ids", n)
	}
}

func TestCollisionReport_RecordsDoNotCollideAcrossProjects(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, _ := a.AddProject("harvey", "")
	bid, _ := b.AddProject("knowledge", "")
	addSharedRecord(t, a, aid, "0007", "harvey's seventh", "x")
	addSharedRecord(t, b, bid, "0007", "knowledge's seventh", "y")

	collisions, err := CollisionReport(a.Path(), b.Path())
	if err != nil {
		t.Fatalf("CollisionReport: %v", err)
	}
	for _, c := range collisions {
		if c.Table == "records" {
			t.Errorf("two projects' DR-0007 are different records, got collision %+v", c)
		}
	}
}

func TestReconcileCollisions_RecordsPreservesBothSidesAfterMerge(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, _ := a.AddProject("harvey", "")
	bid, _ := b.AddProject("harvey", "")
	shareProjectUUID(t, a, aid, b, bid)
	addSharedRecord(t, a, aid, "0007", "a's copy", "a")
	addSharedRecord(t, b, bid, "0007", "b's copy", "b")

	collisions, err := CollisionReport(a.Path(), b.Path())
	if err != nil {
		t.Fatalf("CollisionReport: %v", err)
	}
	if err := ReconcileCollisions(b.Path(), collisions); err != nil {
		t.Fatalf("ReconcileCollisions: %v", err)
	}

	merged := openMergedTestKB(t, a, b)
	titles := recordTitles(t, merged)
	if len(titles) != 1 {
		t.Fatalf("merged records = %v, want one: reconciling makes them the same record", titles)
	}
	if titles[0] != "a's copy" {
		t.Errorf("title = %q, want a's: the row already present wins", titles[0])
	}
}

func TestDivergenceReport_DetectsDifferingChecksums(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, _ := a.AddProject("harvey", "")
	bid, _ := b.AddProject("harvey", "")
	addSharedRecord(t, a, aid, "0007", "same title", "a's body")
	addSharedRecord(t, b, bid, "0007", "same title", "b's body")

	divergences, err := DivergenceReport(a.Path(), b.Path())
	if err != nil {
		t.Fatalf("DivergenceReport: %v", err)
	}
	if len(divergences) != 1 {
		t.Fatalf("divergences = %+v, want one", divergences)
	}
	d := divergences[0]
	if d.Label != "DR-0007" || d.ChecksumA == d.ChecksumB {
		t.Errorf("divergence = %+v, want DR-0007 with two differing checksums", d)
	}
}

func TestDivergenceReport_EmptyWhenChecksumsMatch(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	aid, _ := a.AddProject("harvey", "")
	bid, _ := b.AddProject("harvey", "")
	addSharedRecord(t, a, aid, "0007", "same title", "identical")
	addSharedRecord(t, b, bid, "0007", "same title", "identical")

	divergences, err := DivergenceReport(a.Path(), b.Path())
	if err != nil {
		t.Fatalf("DivergenceReport: %v", err)
	}
	if len(divergences) != 0 {
		t.Errorf("divergences = %+v, want none", divergences)
	}
}
