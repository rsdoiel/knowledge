package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

func buildTestDB(t *testing.T, dbPath string, seed func(kb *knowledge.KnowledgeBase)) {
	t.Helper()
	kb, err := knowledge.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	seed(kb)
	if err := kb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestMerge_TwoIdenticalCopiesProduceMatchingCounts(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.db")
	buildTestDB(t, aPath, func(kb *knowledge.KnowledgeBase) {
		pid, _ := kb.AddProject("proj", "")
		kb.AddObservation(pid, "note", "body")
	})
	bPath := filepath.Join(dir, "b.db")
	if err := copyFile(aPath, bPath); err != nil {
		t.Fatalf("copy: %v", err)
	}

	outPath := filepath.Join(dir, "merged.db")
	var out bytes.Buffer
	if err := cmdMerge(nil, nil, false, []string{"-a", aPath, "-b", bPath, "-out", outPath}, &out); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected merged output file to exist: %v", err)
	}

	merged, err := knowledge.Open(outPath)
	if err != nil {
		t.Fatalf("open merged: %v", err)
	}
	defer merged.Close()
	projects, err := merged.Projects()
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("merged projects = %+v, want exactly one (deduped, not doubled)", projects)
	}
}

func TestMerge_CollisionRequiresForce(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.db")
	buildTestDB(t, aPath, func(kb *knowledge.KnowledgeBase) {
		kb.AddProject("shared-name", "a's version")
	})
	bPath := filepath.Join(dir, "b.db")
	buildTestDB(t, bPath, func(kb *knowledge.KnowledgeBase) {
		kb.AddProject("shared-name", "b's version")
	})

	outPath := filepath.Join(dir, "merged.db")
	var out bytes.Buffer
	err := cmdMerge(nil, nil, false, []string{"-a", aPath, "-b", bPath, "-out", outPath}, &out)
	if err == nil {
		t.Fatal("expected an error when a collision exists and -force is not set")
	}
	// Collision detail belongs in the returned error (so it survives into
	// the JSON error envelope too), not written separately to out -- out is
	// for success data; see TestMerge_CollisionErrorSurvivesJSONMode.
	if !strings.Contains(err.Error(), "collision") {
		t.Errorf("error = %q, want it to mention the collision", err.Error())
	}
	if out.Len() != 0 {
		t.Errorf("out = %q, want nothing written to stdout on an aborted merge", out.String())
	}
	if _, statErr := os.Stat(outPath); statErr == nil {
		t.Error("expected no output file to be written when merge aborts on a collision")
	}
}

func TestMerge_CollisionErrorSurvivesJSONMode(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.db")
	buildTestDB(t, aPath, func(kb *knowledge.KnowledgeBase) { kb.AddProject("shared-name", "") })
	bPath := filepath.Join(dir, "b.db")
	buildTestDB(t, bPath, func(kb *knowledge.KnowledgeBase) { kb.AddProject("shared-name", "") })

	var out bytes.Buffer
	err := cmdMerge(nil, nil, true, []string{"-a", aPath, "-b", bPath, "-out", filepath.Join(dir, "merged.db")}, &out)
	if err == nil {
		t.Fatal("expected an error")
	}
	// dispatch is what actually writes the JSON error envelope; cmdMerge
	// itself just needs to return an error whose message contains the
	// collision detail, same as the text-mode assertion above.
	if !strings.Contains(err.Error(), "collision") {
		t.Errorf("error = %q, want it to mention the collision", err.Error())
	}
	if out.Len() != 0 {
		t.Errorf("out = %q, want nothing written to stdout on an aborted merge, even in JSON mode", out.String())
	}
}

func TestMerge_JSONModeOutputsSummaryAndSuppressesProgressText(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.db")
	buildTestDB(t, aPath, func(kb *knowledge.KnowledgeBase) { kb.AddProject("proj", "") })
	bPath := filepath.Join(dir, "b.db")
	if err := copyFile(aPath, bPath); err != nil {
		t.Fatalf("copy: %v", err)
	}
	outPath := filepath.Join(dir, "merged.db")

	var out bytes.Buffer
	if err := cmdMerge(nil, nil, true, []string{"-a", aPath, "-b", bPath, "-out", outPath}, &out); err != nil {
		t.Fatalf("merge: %v", err)
	}
	assertValidJSON(t, out.Bytes())
	var got struct {
		CollisionsReconciled int `json:"collisions_reconciled"`
		Tables               []struct {
			Table  string `json:"Table"`
			Merged int    `json:"Merged"`
		} `json:"tables"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v (%q)", err, out.String())
	}
	if len(got.Tables) == 0 {
		t.Error("expected the JSON summary to include per-table results")
	}
}

func TestMerge_CollisionReconciledWithForce(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.db")
	buildTestDB(t, aPath, func(kb *knowledge.KnowledgeBase) {
		kb.AddProject("shared-name", "a's version")
	})
	bPath := filepath.Join(dir, "b.db")
	buildTestDB(t, bPath, func(kb *knowledge.KnowledgeBase) {
		pid, _ := kb.AddProject("shared-name", "b's version")
		kb.AddObservation(pid, "note", "only on b")
	})

	outPath := filepath.Join(dir, "merged.db")
	var out bytes.Buffer
	if err := cmdMerge(nil, nil, false, []string{"-a", aPath, "-b", bPath, "-out", outPath, "-force"}, &out); err != nil {
		t.Fatalf("merge with -force: %v", err)
	}

	merged, err := knowledge.Open(outPath)
	if err != nil {
		t.Fatalf("open merged: %v", err)
	}
	defer merged.Close()
	projects, err := merged.Projects()
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("merged projects = %+v, want exactly one (reconciled)", projects)
	}
	obs, err := merged.Observations(projects[0].ID)
	if err != nil {
		t.Fatalf("Observations: %v", err)
	}
	if len(obs) != 1 {
		t.Errorf("merged observations = %+v, want b's observation to survive under the reconciled project", obs)
	}
}

func TestMerge_RequiresAllThreePaths(t *testing.T) {
	var out bytes.Buffer
	if err := cmdMerge(nil, nil, false, []string{"-a", "x.db"}, &out); err == nil {
		t.Error("expected an error when -b/-out are missing")
	}
}

func TestMainRun_MergeDoesNotOpenAmbientDB(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	aPath := filepath.Join(dir, "a.db")
	buildTestDB(t, aPath, func(kb *knowledge.KnowledgeBase) { kb.AddProject("p", "") })
	bPath := filepath.Join(dir, "b.db")
	if err := copyFile(aPath, bPath); err != nil {
		t.Fatalf("copy: %v", err)
	}
	outPath := filepath.Join(dir, "merged.db")

	var out, errOut bytes.Buffer
	code := mainRun([]string{"merge", "-a", aPath, "-b", bPath, "-out", outPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (errOut=%q)", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "agents", "knowledge.db")); err == nil {
		t.Error("expected no ./agents/knowledge.db to be created as a side effect of running merge")
	}
}

// ─── W1: input normalisation before ATTACH (DR-0014) ──────────────────────

// dropRecordsTables makes a database look like one written before decision
// records existed. wren.local's was in exactly this state as recently as
// 2026-08.
func dropRecordsTables(t *testing.T, dbPath string) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	defer db.Close()
	for _, stmt := range []string{`DROP TABLE record_relations`, `DROP TABLE records`} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

// A merge against a database that predates the records table must succeed.
// This passes trivially until W2 unions records — at that point it is the
// test that keeps `SELECT ... FROM b.records` from erroring against a real
// unmigrated machine, which is the failure DR-0014 exists to prevent.
func TestMerge_SucceedsAgainstPreRecordsDatabase(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.db")
	buildTestDB(t, aPath, func(kb *knowledge.KnowledgeBase) {
		pid, _ := kb.AddProject("proj", "")
		kb.AddObservation(pid, "note", "on a")
	})
	bPath := filepath.Join(dir, "b.db")
	buildTestDB(t, bPath, func(kb *knowledge.KnowledgeBase) {
		pid, _ := kb.AddProject("other", "")
		kb.AddObservation(pid, "note", "on b")
	})
	dropRecordsTables(t, bPath)

	outPath := filepath.Join(dir, "merged.db")
	var out bytes.Buffer
	if err := cmdMerge(nil, nil, false, []string{"-a", aPath, "-b", bPath, "-out", outPath}, &out); err != nil {
		t.Fatalf("merge against a pre-records database: %v", err)
	}

	merged, err := knowledge.Open(outPath)
	if err != nil {
		t.Fatalf("open merged: %v", err)
	}
	defer merged.Close()
	projects, err := merged.Projects()
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("merged projects = %d, want 2", len(projects))
	}
}

// DR-0014 decided merge normalises its throwaway copies, not the operator's
// files. This is the test that says so: normalising aPath or bPath directly
// would be the simpler implementation and is the one this forbids.
func TestMerge_LeavesSourceDatabasesUnmodified(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.db")
	buildTestDB(t, aPath, func(kb *knowledge.KnowledgeBase) {
		pid, _ := kb.AddProject("proj", "")
		kb.AddObservation(pid, "note", "on a")
	})
	bPath := filepath.Join(dir, "b.db")
	buildTestDB(t, bPath, func(kb *knowledge.KnowledgeBase) {
		pid, _ := kb.AddProject("other", "")
		kb.AddObservation(pid, "note", "on b")
	})
	dropRecordsTables(t, bPath)

	before := map[string][]byte{}
	for _, p := range []string{aPath, bPath} {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		before[p] = data
	}

	outPath := filepath.Join(dir, "merged.db")
	var out bytes.Buffer
	if err := cmdMerge(nil, nil, false, []string{"-a", aPath, "-b", bPath, "-out", outPath}, &out); err != nil {
		t.Fatalf("merge: %v", err)
	}

	for _, p := range []string{aPath, bPath} {
		after, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if !bytes.Equal(before[p], after) {
			t.Errorf("%s changed during merge: normalization belongs to the scratch copy, not the operator's database", filepath.Base(p))
		}
	}
	// The pre-records side must still be pre-records afterwards.
	db, err := sql.Open("sqlite", bPath)
	if err != nil {
		t.Fatalf("open b: %v", err)
	}
	defer db.Close()
	var name string
	if err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'records'`,
	).Scan(&name); err == nil {
		t.Error("merge created a records table in the source database; it must only migrate its own copy")
	}
}

// ─── W4: content divergence (DR-0013) ─────────────────────────────────────

// addWorkspaceRecord writes a workspace-tier record. The tier is deliberate:
// with no project involved, two databases' records match on identity without
// a project name collision confounding what is being tested.
func addWorkspaceRecord(t *testing.T, kb *knowledge.KnowledgeBase, recordID, title, body string) {
	t.Helper()
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: recordID, Scope: "workspace",
		Path:  "agents/decisions/" + recordID + "-x.md",
		Title: title, Date: "2026-08-27", Body: body,
		Checksum: "sum-" + body,
	}); err != nil {
		t.Fatalf("AddRecord %s: %v", recordID, err)
	}
}

// A record whose text differs between the two machines is reported, and the
// merge still runs. In practice this always accompanies an identity collision
// -- the same record file ingested on two machines gets a fresh uuid on each
// -- so -force is the realistic path. The conflict rule is unchanged: a's row
// wins, and the operator is told which record's prose was dropped, because a
// decision record's text is its artifact (DR-0013).
func TestMerge_ContentDivergenceIsReportedWithoutBlocking(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.db")
	buildTestDB(t, aPath, func(kb *knowledge.KnowledgeBase) {
		addWorkspaceRecord(t, kb, "0007", "same title", "a's body")
	})
	bPath := filepath.Join(dir, "b.db")
	buildTestDB(t, bPath, func(kb *knowledge.KnowledgeBase) {
		addWorkspaceRecord(t, kb, "0007", "same title", "b's body")
	})

	outPath := filepath.Join(dir, "merged.db")
	var out bytes.Buffer
	if err := cmdMerge(nil, nil, false, []string{"-a", aPath, "-b", bPath, "-out", outPath, "-force"}, &out); err != nil {
		t.Fatalf("merge should not be blocked by a divergence: %v", err)
	}
	if !strings.Contains(out.String(), "divergence") || !strings.Contains(out.String(), "sum-a's body") {
		t.Errorf("output does not report the divergence and its checksums:\n%s", out.String())
	}

	merged, err := knowledge.Open(outPath)
	if err != nil {
		t.Fatalf("open merged: %v", err)
	}
	defer merged.Close()
	recs, err := merged.ListRecords(knowledge.RecordFilter{})
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("merged records = %d, want 1 after reconciliation", len(recs))
	}
	if recs[0].Body != "a's body" {
		t.Errorf("body = %q, want a's: the row already present still wins", recs[0].Body)
	}
}

// A divergence must survive the abort path too. When a collision stops the
// merge the divergence is still true and still worth knowing, and out is
// reserved for a successful run -- so it belongs in the error message.
func TestMerge_ContentDivergenceSurvivesCollisionAbort(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.db")
	buildTestDB(t, aPath, func(kb *knowledge.KnowledgeBase) {
		addWorkspaceRecord(t, kb, "0007", "same title", "a's body")
	})
	bPath := filepath.Join(dir, "b.db")
	buildTestDB(t, bPath, func(kb *knowledge.KnowledgeBase) {
		addWorkspaceRecord(t, kb, "0007", "same title", "b's body")
	})

	outPath := filepath.Join(dir, "merged.db")
	var out bytes.Buffer
	err := cmdMerge(nil, nil, false, []string{"-a", aPath, "-b", bPath, "-out", outPath}, &out)
	if err == nil {
		t.Fatal("expected the identity collision to abort the merge")
	}
	if !strings.Contains(err.Error(), "divergence") || !strings.Contains(err.Error(), "sum-b's body") {
		t.Errorf("abort message does not report the divergence:\n%s", err.Error())
	}
}

func TestMerge_ContentDivergenceInJSONOutput(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.db")
	buildTestDB(t, aPath, func(kb *knowledge.KnowledgeBase) {
		addWorkspaceRecord(t, kb, "0007", "same title", "a's body")
	})
	bPath := filepath.Join(dir, "b.db")
	buildTestDB(t, bPath, func(kb *knowledge.KnowledgeBase) {
		addWorkspaceRecord(t, kb, "0007", "same title", "b's body")
	})

	outPath := filepath.Join(dir, "merged.db")
	var out bytes.Buffer
	if err := cmdMerge(nil, nil, true, []string{"-a", aPath, "-b", bPath, "-out", outPath, "-force"}, &out); err != nil {
		t.Fatalf("merge: %v", err)
	}
	var got struct {
		ContentDivergences []struct {
			Label     string `json:"Label"`
			ChecksumA string `json:"ChecksumA"`
			ChecksumB string `json:"ChecksumB"`
		} `json:"content_divergences"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out.String(), err)
	}
	if len(got.ContentDivergences) != 1 || got.ContentDivergences[0].Label != "DR-0007" {
		t.Errorf("content_divergences = %+v, want one DR-0007 entry", got.ContentDivergences)
	}
	if got.ContentDivergences[0].ChecksumA == got.ContentDivergences[0].ChecksumB {
		t.Error("expected two differing checksums in the JSON output")
	}
}
