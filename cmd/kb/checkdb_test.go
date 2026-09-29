package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	knowledge "github.com/rsdoiel/knowledge"
)

// `kb check-db` (DR-0053): decide by full content, never by file time, whether
// the database and the JSONL dump beside it agree, and say what to do.

func checkDBFixture(t *testing.T) (kb *knowledge.KnowledgeBase, root, dump string) {
	t.Helper()
	kb, root = openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "the project")
	c, _ := kb.AddConcept("toast", "")
	o, _ := kb.AddObservation(pid, "note", "first")
	kb.LinkObservationConcept(o, c)
	dump = filepath.Join(root, "agents", "knowledge.jsonl")
	writeDump(t, kb, dump)
	return
}

func writeDump(t *testing.T, kb *knowledge.KnowledgeBase, path string) {
	t.Helper()
	var buf bytes.Buffer
	if err := knowledge.ExportJSONL(kb, &buf, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runCheckDB(t *testing.T, kb *knowledge.KnowledgeBase, jsonOut bool, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := cmdCheckDB(kb, nil, jsonOut, args, &out)
	return out.String(), err
}

func TestCheckDB_InSyncIsExit0AndNamesBothPaths(t *testing.T) {
	kb, root, dump := checkDBFixture(t)
	out, err := runCheckDB(t, kb, false)
	if err != nil {
		t.Fatalf("in sync should be success: %v\n%s", err, out)
	}
	for _, want := range []string{"in sync", filepath.Join(root, "agents", "knowledge.db"), dump} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
}

func TestCheckDB_DatabaseAheadRecommendsExport(t *testing.T) {
	kb, _, _ := checkDBFixture(t)
	p, _ := kb.ProjectByName("alpha")
	kb.AddObservation(p.ID, "note", "made after the export")
	out, err := runCheckDB(t, kb, false)
	if err == nil || exitCodeFor(err) != classNegative {
		t.Fatalf("err = %v, want a negative (exit 1)", err)
	}
	if !strings.Contains(out, "kb export") || !strings.Contains(out, "made after the export") {
		t.Errorf("report:\n%s", out)
	}
}

func TestCheckDB_DumpAheadRecommendsImportAndWarnsAboutDeletions(t *testing.T) {
	kb, _, dump := checkDBFixture(t)
	p, _ := kb.ProjectByName("alpha")
	extra, _ := kb.AddObservation(p.ID, "note", "in the dump only")
	writeDump(t, kb, dump)
	kb.DeleteObservation(extra, true)
	out, err := runCheckDB(t, kb, false)
	if err == nil || exitCodeFor(err) != classNegative {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{"kb import", "never received, or deleted here", "brings back", "in the dump only"} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
}

func TestCheckDB_BothWaysIsDiverged(t *testing.T) {
	kb, _, dump := checkDBFixture(t)
	p, _ := kb.ProjectByName("alpha")
	extra, _ := kb.AddObservation(p.ID, "note", "in the dump only")
	writeDump(t, kb, dump)
	kb.DeleteObservation(extra, true)
	kb.AddObservation(p.ID, "note", "here only")
	out, err := runCheckDB(t, kb, false)
	if err == nil {
		t.Fatal("want exit 1")
	}
	if !strings.Contains(out, "diverged") || !strings.Contains(out, "kb import, then kb export") {
		t.Errorf("report:\n%s", out)
	}
}

func TestCheckDB_JSONLFlagOverridesTheDefault(t *testing.T) {
	kb, root, dump := checkDBFixture(t)
	other := filepath.Join(root, "elsewhere.jsonl")
	if err := os.Rename(dump, other); err != nil {
		t.Fatal(err)
	}
	if _, err := runCheckDB(t, kb, false); exitCodeFor(err) != classNoInput {
		t.Errorf("default path is gone: err = %v, want no_input", err)
	}
	if out, err := runCheckDB(t, kb, false, "--jsonl", other); err != nil {
		t.Errorf("--jsonl: %v\n%s", err, out)
	}
}

func TestCheckDB_MalformedDumpIsData(t *testing.T) {
	kb, _, dump := checkDBFixture(t)
	os.WriteFile(dump, []byte("{\"type\": \n"), 0o644)
	if _, err := runCheckDB(t, kb, false); exitCodeFor(err) != classData {
		t.Errorf("err = %v, want data (65)", err)
	}
}

func TestCheckDB_FileTimeIsAHintAndNeverTheVerdict(t *testing.T) {
	kb, root, dump := checkDBFixture(t)
	db := filepath.Join(root, "agents", "knowledge.db")
	old, recent := time.Now().Add(-48*time.Hour), time.Now()
	var reports []string
	for _, times := range [][2]time.Time{{old, recent}, {recent, old}} {
		os.Chtimes(dump, times[0], times[0])
		os.Chtimes(db, times[1], times[1])
		out, err := runCheckDB(t, kb, false)
		if err != nil {
			t.Fatalf("in-sync content must stay in sync whatever the file times: %v", err)
		}
		if !strings.Contains(out, "file times") {
			t.Errorf("no file-time hint:\n%s", out)
		}
		reports = append(reports, out)
	}
	if !strings.Contains(reports[0], "in sync") || !strings.Contains(reports[1], "in sync") {
		t.Error("verdict changed with file times")
	}
}

func TestCheckDB_ChangesNothing(t *testing.T) {
	kb, _, dump := checkDBFixture(t)
	before, _ := os.ReadFile(dump)
	p, _ := kb.ProjectByName("alpha")
	kb.AddObservation(p.ID, "note", "drift")
	runCheckDB(t, kb, false)
	after, _ := os.ReadFile(dump)
	if !bytes.Equal(before, after) {
		t.Error("the dump changed")
	}
	obs, _ := kb.Observations(p.ID)
	if len(obs) != 2 {
		t.Errorf("observations = %d, want the database untouched", len(obs))
	}
}

func TestCheckDB_JSONShape(t *testing.T) {
	kb, _, _ := checkDBFixture(t)
	p, _ := kb.ProjectByName("alpha")
	kb.AddObservation(p.ID, "note", "drift")
	out, err := runCheckDB(t, kb, true)
	if err == nil {
		t.Fatal("want exit 1")
	}
	var v struct {
		InSync         bool   `json:"in_sync"`
		Recommendation string `json:"recommendation"`
		Database       string `json:"database"`
		JSONL          string `json:"jsonl"`
		Tables         []struct {
			Table     string   `json:"table"`
			Database  int      `json:"database"`
			JSONL     int      `json:"jsonl"`
			OnlyInDB  []string `json:"only_in_database"`
			OnlyInDmp []string `json:"only_in_jsonl"`
		} `json:"tables"`
	}
	if e := json.Unmarshal([]byte(out), &v); e != nil {
		t.Fatalf("not one JSON object: %v\n%s", e, out)
	}
	if v.InSync || v.Recommendation != "export" || v.Database == "" || v.JSONL == "" || len(v.Tables) != 14 {
		t.Errorf("decoded = %+v", v)
	}
	for _, tb := range v.Tables {
		if tb.Table == "observations" && (tb.Database != 2 || tb.JSONL != 1 || len(tb.OnlyInDB) != 1) {
			t.Errorf("observations = %+v", tb)
		}
	}
}

func TestCheckDB_JSONErrorEnvelopeCarriesTheClass(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	db := filepath.Join(dir, "agents", "knowledge.db")
	kb, err := knowledge.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	kb.Close()
	var out, errOut bytes.Buffer
	code := mainRun([]string{"--json", "--db", db, "check-db"}, &out, &errOut)
	if code != classNoInput.Code {
		t.Fatalf("exit %d, want 66 for a missing dump; stderr %s", code, errOut.String())
	}
	var env struct {
		Class string `json:"class"`
		Code  int    `json:"code"`
	}
	if err := json.Unmarshal(errOut.Bytes(), &env); err != nil || env.Class != "no_input" {
		t.Errorf("envelope = %s (%v)", errOut.String(), err)
	}
}

func TestCheckDB_UsageErrors(t *testing.T) {
	kb, _, _ := checkDBFixture(t)
	for _, args := range [][]string{{"extra"}, {"--bogus"}, {"--jsonl"}} {
		if _, err := runCheckDB(t, kb, false, args...); exitCodeFor(err) != classUsage {
			t.Errorf("check-db %v: err = %v, want usage (2)", args, err)
		}
	}
}
