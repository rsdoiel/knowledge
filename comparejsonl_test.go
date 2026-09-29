package knowledge

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// CompareToJSONL (DR-0053): compare a live database with a JSONL dump by full
// content, read-only, without a second implementation of the identity rules.

func openFixtureKB(t *testing.T) (*KnowledgeBase, string) {
	t.Helper()
	path := diffFixture(t, "a")
	kb, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kb.Close() })
	return kb, path
}

func dumpOf(t *testing.T, kb *KnowledgeBase) string {
	t.Helper()
	var buf bytes.Buffer
	if err := ExportJSONL(kb, &buf, ""); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestCompareToJSONL_DatabaseAndItsOwnDumpAreInSync(t *testing.T) {
	kb, _ := openFixtureKB(t)
	c, err := kb.CompareToJSONL(strings.NewReader(dumpOf(t, kb)))
	if err != nil {
		t.Fatalf("CompareToJSONL: %v", err)
	}
	if !c.InSync() || c.Recommendation() != "in-sync" {
		for _, d := range c.Tables {
			if !d.Clean() {
				t.Errorf("%s: %+v", d.Table, d)
			}
		}
		t.Fatalf("own dump: verdict %q", c.Recommendation())
	}
	if len(c.Tables) != 14 {
		t.Errorf("tables = %d", len(c.Tables))
	}
}

func TestCompareToJSONL_ObservationAddedHereIsOnlyInDatabaseAndSuggestsExport(t *testing.T) {
	kb, _ := openFixtureKB(t)
	dump := dumpOf(t, kb)
	p, _ := kb.ProjectByName("alpha")
	kb.AddObservation(p.ID, "note", "made after the export")
	c, err := kb.CompareToJSONL(strings.NewReader(dump))
	if err != nil {
		t.Fatal(err)
	}
	var obs TableDiff
	for _, d := range c.Tables {
		if d.Table == "observations" {
			obs = d
		}
	}
	if len(obs.OnlyA) != 1 || len(obs.OnlyB) != 0 || c.InSync() || c.Recommendation() != "export" {
		t.Errorf("obs = %+v, verdict %q", obs, c.Recommendation())
	}
}

func TestCompareToJSONL_ObservationAddedToTheDumpIsOnlyInJSONLAndSuggestsImport(t *testing.T) {
	kb, _ := openFixtureKB(t)
	p, _ := kb.ProjectByName("alpha")
	extra, _ := kb.AddObservation(p.ID, "note", "only in the dump")
	dump := dumpOf(t, kb)
	if _, err := kb.DeleteObservation(extra, true); err != nil {
		t.Fatal(err)
	}
	c, err := kb.CompareToJSONL(strings.NewReader(dump))
	if err != nil {
		t.Fatal(err)
	}
	var obs TableDiff
	for _, d := range c.Tables {
		if d.Table == "observations" {
			obs = d
		}
	}
	if len(obs.OnlyB) != 1 || len(obs.OnlyA) != 0 || c.Recommendation() != "import" {
		t.Errorf("obs = %+v, verdict %q", obs, c.Recommendation())
	}
}

func TestCompareToJSONL_BothDirectionsIsDiverged(t *testing.T) {
	kb, _ := openFixtureKB(t)
	p, _ := kb.ProjectByName("alpha")
	extra, _ := kb.AddObservation(p.ID, "note", "only in the dump")
	dump := dumpOf(t, kb)
	if _, err := kb.DeleteObservation(extra, true); err != nil {
		t.Fatal(err)
	}
	kb.AddObservation(p.ID, "note", "only here")
	c, err := kb.CompareToJSONL(strings.NewReader(dump))
	if err != nil {
		t.Fatal(err)
	}
	if c.Recommendation() != "diverged" {
		t.Errorf("verdict %q, want diverged", c.Recommendation())
	}
}

func TestCompareToJSONL_PromotedRecordStatusIsDifferent(t *testing.T) {
	kb, _ := openFixtureKB(t)
	dump := dumpOf(t, kb)
	if _, err := kb.db.Exec(`UPDATE records SET checksum = 'promoted' WHERE record_id = '0001'`); err != nil {
		t.Fatal(err)
	}
	c, err := kb.CompareToJSONL(strings.NewReader(dump))
	if err != nil {
		t.Fatal(err)
	}
	var rec TableDiff
	for _, d := range c.Tables {
		if d.Table == "records" {
			rec = d
		}
	}
	if len(rec.Different) != 1 || c.InSync() || c.Recommendation() != "diverged" {
		t.Errorf("records = %+v, verdict %q", rec, c.Recommendation())
	}
}

func TestCompareToJSONL_EmptyDumpMeansEverythingIsOnlyInTheDatabase(t *testing.T) {
	kb, _ := openFixtureKB(t)
	c, err := kb.CompareToJSONL(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if c.Recommendation() != "export" || c.InSync() {
		t.Errorf("verdict %q", c.Recommendation())
	}
}

func TestCompareToJSONL_MalformedDumpIsInvalid(t *testing.T) {
	kb, _ := openFixtureKB(t)
	_, err := kb.CompareToJSONL(strings.NewReader("{\"type\": \n"))
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
}

func TestCompareToJSONL_LeavesTheRealDatabaseByteIdenticalAndCleansUp(t *testing.T) {
	kb, path := openFixtureKB(t)
	dump := dumpOf(t, kb)
	p, _ := kb.ProjectByName("alpha")
	kb.AddObservation(p.ID, "note", "drift, so the comparison has real work to do")
	// The database is open and has a live -wal beside it: hash both, and the
	// directory listing, before and after. -shm is left out on purpose: it is
	// SQLite's shared-memory index, rewritten by any read, and holds no data.
	files := []string{path, path + "-wal"}
	hashAll := func() map[string]string {
		out := map[string]string{}
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				out[f] = "absent"
				continue
			}
			out[f] = fmt.Sprintf("%x", sha256.Sum256(b))
		}
		return out
	}
	list := func() string {
		es, _ := os.ReadDir(filepath.Dir(path))
		var names []string
		for _, e := range es {
			names = append(names, e.Name())
		}
		return strings.Join(names, ",")
	}
	before, listBefore, rowsBefore := hashAll(), list(), diffRowsOf(t, kb)
	tmpBefore, _ := filepath.Glob(filepath.Join(os.TempDir(), "kbcheck-*"))
	if _, err := kb.CompareToJSONL(strings.NewReader(dump)); err != nil {
		t.Fatal(err)
	}
	if after := hashAll(); !reflect.DeepEqual(before, after) {
		t.Errorf("database files changed:\n%v\n%v", before, after)
	}
	if list() != listBefore {
		t.Errorf("directory listing changed: %s -> %s", listBefore, list())
	}
	if got := diffRowsOf(t, kb); got != rowsBefore {
		t.Errorf("rows changed: %q -> %q", rowsBefore, got)
	}
	if tmpAfter, _ := filepath.Glob(filepath.Join(os.TempDir(), "kbcheck-*")); len(tmpAfter) != len(tmpBefore) {
		t.Errorf("scratch directories left behind: %v -> %v", tmpBefore, tmpAfter)
	}
}

func diffRowsOf(t *testing.T, kb *KnowledgeBase) string {
	t.Helper()
	var parts []string
	for _, tbl := range []string{"projects", "concepts", "sources", "observations", "records", "documents", "document_sections"} {
		var n int
		if err := kb.db.QueryRow(`SELECT COUNT(*) FROM ` + tbl).Scan(&n); err != nil {
			t.Fatal(err)
		}
		parts = append(parts, fmt.Sprintf("%s=%d", tbl, n))
	}
	return strings.Join(parts, ",")
}
