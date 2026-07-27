package main

import (
	"bytes"
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
