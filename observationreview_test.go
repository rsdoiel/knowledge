package knowledge

import (
	"database/sql"
	"errors"
	"os"
	"testing"
)

// K1 of the draft-then-promote plan (DR-0090): the library. Written red first.

func ftsObservationRows(t *testing.T, kb *KnowledgeBase) int {
	t.Helper()
	var n int
	if err := kb.db.QueryRow(`SELECT COUNT(*) FROM kb_fts WHERE source_type = 'observation'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func f64(v float64) *float64 { return &v }

func TestOpen_MigratedDatabaseKeepsEveryObservationReviewedAndSearchable(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/agents/knowledge.db"
	os.MkdirAll(dir+"/agents", 0o755)
	// A v0.0.29 database: observations without status, generated_by or confidence.
	kb, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := kb.AddProject("p", "")
	for _, b := range []string{"one", "two", "three"} {
		if _, err := kb.AddObservation(pid, "note", b); err != nil {
			t.Fatal(err)
		}
	}
	rowsBefore, ftsBefore := 3, ftsObservationRows(t, kb)
	kb.Close()
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"status", "generated_by", "confidence"} {
		raw.Exec(`ALTER TABLE observations DROP COLUMN ` + c)
	}
	raw.Close()

	kb, err = Open(dbPath)
	if err != nil {
		t.Fatalf("Open migrated: %v", err)
	}
	defer kb.Close()
	var rows, reviewed int
	kb.db.QueryRow(`SELECT COUNT(*), SUM(status = 'reviewed') FROM observations`).Scan(&rows, &reviewed)
	if rows != rowsBefore || reviewed != rows {
		t.Errorf("rows=%d reviewed=%d, want %d reviewed", rows, reviewed, rowsBefore)
	}
	if got := ftsObservationRows(t, kb); got != ftsBefore {
		t.Errorf("kb_fts observation rows = %d, want %d", got, ftsBefore)
	}
}

func TestAddObservationDraft_IsNotIndexed(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("p", "")
	id, err := kb.AddObservationDraft(pid, "finding", "a drafted thought", "harvey qwen2.5:3b", f64(0.7))
	if err != nil {
		t.Fatalf("AddObservationDraft: %v", err)
	}
	if n := ftsObservationRows(t, kb); n != 0 {
		t.Errorf("kb_fts observation rows = %d, want 0 for a draft", n)
	}
	o, err := kb.ObservationByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != "drafted" || o.GeneratedBy != "harvey qwen2.5:3b" || o.Confidence == nil || *o.Confidence != 0.7 {
		t.Errorf("observation = %+v, want drafted by harvey with confidence 0.7", o)
	}
}

func TestAddObservationDraft_RefusesBadValues(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("p", "")
	cases := []struct {
		name string
		by   string
		conf *float64
	}{
		{"blank by", "  ", nil},
		{"multi-line by", "a\nb", nil},
		{"confidence above 1", "h", f64(1.5)},
		{"confidence below 0", "h", f64(-0.1)},
	}
	for _, c := range cases {
		if _, err := kb.AddObservationDraft(pid, "note", "body", c.by, c.conf); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", c.name, err)
		}
	}
	if _, err := kb.AddObservationDraft(pid, "bogus", "body", "h", nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad kind: err = %v, want ErrInvalid", err)
	}
	var n int
	kb.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&n)
	if n != 0 {
		t.Errorf("%d rows written by refused drafts", n)
	}
}

func TestPromoteObservation(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("p", "")
	id, _ := kb.AddObservationDraft(pid, "note", "promote me", "h", nil)

	if err := kb.PromoteObservation(id); err != nil {
		t.Fatalf("PromoteObservation: %v", err)
	}
	if n := ftsObservationRows(t, kb); n != 1 {
		t.Errorf("kb_fts observation rows = %d after promote, want 1", n)
	}
	o, _ := kb.ObservationByID(id)
	if o.Status != "reviewed" {
		t.Errorf("status = %q, want reviewed", o.Status)
	}
	if err := kb.PromoteObservation(id); !errors.Is(err, ErrConflict) {
		t.Errorf("second promote: err = %v, want ErrConflict", err)
	}
	if n := ftsObservationRows(t, kb); n != 1 {
		t.Errorf("second promote changed the index: %d rows", n)
	}
	if err := kb.PromoteObservation(9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id: err = %v, want ErrNotFound", err)
	}
}

func TestObservationReviewQueue(t *testing.T) {
	kb := openTestKB(t)
	p1, _ := kb.AddProject("one", "")
	p2, _ := kb.AddProject("two", "")
	kb.AddObservation(p1, "note", "reviewed one")
	d1, _ := kb.AddObservationDraft(p1, "note", "draft one", "h", nil)
	d2, _ := kb.AddObservationDraft(p2, "note", "draft two", "h", nil)
	promoted, _ := kb.AddObservationDraft(p1, "note", "draft promoted", "h", nil)
	kb.PromoteObservation(promoted)

	all, err := kb.ObservationReviewQueue(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("queue (all) = %d items, want 2: %+v", len(all), all)
	}
	one, _ := kb.ObservationReviewQueue(p1)
	if len(one) != 1 || one[0].ID != d1 {
		t.Errorf("queue (project one) = %+v, want only draft %d", one, d1)
	}
	_ = d2
}

func TestRebuildFTS_IndexesReviewedObservationsOnly(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("p", "")
	kb.AddObservation(pid, "note", "reviewed")
	kb.AddObservationDraft(pid, "note", "drafted", "h", nil)
	promoted, _ := kb.AddObservationDraft(pid, "note", "promoted", "h", nil)
	kb.PromoteObservation(promoted)
	incremental := ftsObservationRows(t, kb)

	kb.db.Exec(`DELETE FROM kb_fts`)
	if err := kb.rebuildFTSIfNeeded(); err != nil {
		t.Fatal(err)
	}
	if got := ftsObservationRows(t, kb); got != incremental || got != 2 {
		t.Errorf("rebuilt observation rows = %d, incremental = %d, want both 2", got, incremental)
	}
	var drafted int
	kb.db.QueryRow(`SELECT COUNT(*) FROM kb_fts WHERE source_type='observation' AND body = 'drafted'`).Scan(&drafted)
	if drafted != 0 {
		t.Error("rebuild indexed a draft")
	}
}

func TestObservationReviewQueue_ASupersededDraftIsNotQueued(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("p", "")
	old, _ := kb.AddObservationDraft(pid, "note", "old wording", "h", nil)
	newer, _ := kb.AddObservationDraft(pid, "note", "new wording", "h", nil)
	if err := kb.AddObservationRelation(newer, old, "supersedes"); err != nil {
		t.Fatal(err)
	}
	q, err := kb.ObservationReviewQueue(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(q) != 1 || q[0].ID != newer {
		t.Errorf("queue = %+v, want only draft %d", q, newer)
	}
}
