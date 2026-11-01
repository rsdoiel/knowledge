package knowledge

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// K3 of the draft-then-promote plan (DR-0090): drafts travel as their own
// JSONL record type, so an older kb cannot read one as a trusted observation.

func dumpLines(t *testing.T, kb *KnowledgeBase) []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	if err := ExportJSONL(kb, &buf, ""); err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("bad line %q: %v", l, err)
		}
		out = append(out, m)
	}
	return out
}

func obsUUID(t *testing.T, kb *KnowledgeBase, id int64) string {
	t.Helper()
	var u string
	if err := kb.db.QueryRow(`SELECT uuid FROM observations WHERE id = ?`, id).Scan(&u); err != nil {
		t.Fatal(err)
	}
	return u
}

func obsStatusByUUID(t *testing.T, kb *KnowledgeBase, uuid string) string {
	t.Helper()
	var s string
	if err := kb.db.QueryRow(`SELECT status FROM observations WHERE uuid = ?`, uuid).Scan(&s); err != nil {
		t.Fatalf("no observation %s: %v", uuid, err)
	}
	return s
}

func importDump(t *testing.T, kb *KnowledgeBase, dump string) []ImportTableSummary {
	t.Helper()
	s, err := ImportJSONL(kb, strings.NewReader(dump))
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}
	return s
}

func dumpString(t *testing.T, kb *KnowledgeBase) string {
	t.Helper()
	var buf bytes.Buffer
	if err := ExportJSONL(kb, &buf, ""); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestDraftTravel_ExportShapes(t *testing.T) {
	f := newDraftFixture(t)
	var rev, dr map[string]any
	for _, m := range dumpLines(t, f.kb) {
		switch {
		case m["type"] == "observation":
			rev = m
		case m["type"] == "observation_draft":
			dr = m
		}
	}
	if rev == nil || dr == nil {
		t.Fatalf("want one observation and one observation_draft line; got %v / %v", rev, dr)
	}
	for _, k := range []string{"status", "generated_by", "confidence"} {
		if _, ok := rev[k]; ok {
			t.Errorf("reviewed observation line carries %q; it must export exactly as before", k)
		}
	}
	if dr["status"] != "drafted" || dr["generated_by"] != "harvey" || dr["uuid"] != obsUUID(t, f.kb, f.drafted) {
		t.Errorf("draft line = %v", dr)
	}
	if rev["uuid"] != obsUUID(t, f.kb, f.reviewed) {
		t.Errorf("reviewed line = %v", rev)
	}
}

func TestDraftTravel_RoundTripIntoAFreshDatabase(t *testing.T) {
	f := newDraftFixture(t)
	dump := dumpString(t, f.kb)
	dst := openTestKB(t)
	importDump(t, dst, dump)

	if got := obsStatusByUUID(t, dst, obsUUID(t, f.kb, f.reviewed)); got != "reviewed" {
		t.Errorf("reviewed row imported as %q", got)
	}
	du := obsUUID(t, f.kb, f.drafted)
	if got := obsStatusByUUID(t, dst, du); got != "drafted" {
		t.Errorf("draft imported as %q", got)
	}
	var by string
	var conf any
	dst.db.QueryRow(`SELECT generated_by, confidence FROM observations WHERE uuid = ?`, du).Scan(&by, &conf)
	if by != "harvey" {
		t.Errorf("generated_by = %q", by)
	}
	if n := ftsObservationRows(t, dst); n != 1 {
		t.Errorf("kb_fts observation rows = %d, want 1 (the reviewed one)", n)
	}
	var links int
	dst.db.QueryRow(`SELECT COUNT(*) FROM observation_concepts x JOIN observations o ON o.id = x.observation_id WHERE o.status = 'drafted'`).Scan(&links)
	if links != 2 {
		t.Errorf("the draft's concept links imported = %d, want 2", links)
	}
	// The imported copy dumps back to the same text.
	if dumpString(t, dst) == "" {
		t.Error("empty re-export")
	}
	cmp, err := dst.CompareToJSONL(strings.NewReader(dump))
	if err != nil || !cmp.InSync() {
		t.Errorf("imported database is not in sync with its dump: %v %+v", err, cmp.Recommendation())
	}
}

func TestDraftTravel_ImportingTwiceAddsNothing(t *testing.T) {
	f := newDraftFixture(t)
	dump := dumpString(t, f.kb)
	dst := openTestKB(t)
	importDump(t, dst, dump)
	for _, s := range importDump(t, dst, dump) {
		if s.Table == "observation_draft" && s.Imported != 0 {
			t.Errorf("second import of drafts imported %d", s.Imported)
		}
	}
}

func TestDraftTravel_PromotionOnAReachesB(t *testing.T) {
	f := newDraftFixture(t) // machine A
	b := openTestKB(t)
	importDump(t, b, dumpString(t, f.kb)) // B now holds the draft
	du := obsUUID(t, f.kb, f.drafted)
	if err := f.kb.PromoteObservation(f.drafted); err != nil {
		t.Fatal(err)
	}
	importDump(t, b, dumpString(t, f.kb)) // A's dump now carries it as an observation
	if got := obsStatusByUUID(t, b, du); got != "reviewed" {
		t.Fatalf("B's copy is %q after A's promotion was imported", got)
	}
	if n := ftsObservationRows(t, b); n != 2 {
		t.Errorf("B kb_fts observation rows = %d, want 2", n)
	}
	if hits, _ := b.Search("drafted"); len(hits) == 0 {
		t.Error("B cannot find the promoted observation")
	}
}

func TestDraftTravel_AnOldDraftNeverDowngradesAReviewedRow(t *testing.T) {
	f := newDraftFixture(t)
	oldDump := dumpString(t, f.kb) // carries the draft
	if err := f.kb.PromoteObservation(f.drafted); err != nil {
		t.Fatal(err)
	}
	importDump(t, f.kb, oldDump)
	if got := obsStatusByUUID(t, f.kb, obsUUID(t, f.kb, f.drafted)); got != "reviewed" {
		t.Errorf("importing a stale draft downgraded the row to %q", got)
	}
	if n := ftsObservationRows(t, f.kb); n != 2 {
		t.Errorf("kb_fts observation rows = %d, want 2", n)
	}
}

func TestDraftTravel_CompareReportsAStatusDifference(t *testing.T) {
	f := newDraftFixture(t)
	dump := dumpString(t, f.kb)
	if err := f.kb.PromoteObservation(f.drafted); err != nil {
		t.Fatal(err)
	}
	c, err := f.kb.CompareToJSONL(strings.NewReader(dump))
	if err != nil {
		t.Fatal(err)
	}
	if c.InSync() {
		t.Fatal("a status difference was reported as in sync")
	}
	var obs TableDiff
	for _, d := range c.Tables {
		if d.Table == "observations" {
			obs = d
		}
	}
	if len(obs.Different) != 1 {
		t.Errorf("observations diff = %+v, want one Different row", obs)
	}
}

// Merge.

func mergePair(t *testing.T, aStatus, bStatus string) (*KnowledgeBase, string) {
	t.Helper()
	a, b := openTestKB(t), openTestKB(t)
	pa, _ := a.AddProject("shared", "")
	pb, _ := b.AddProject("shared", "")
	syncProjectUUID(t, a, b, "shared")
	mk := func(kb *KnowledgeBase, pid int64, status string) int64 {
		var id int64
		var err error
		if status == "drafted" {
			id, err = kb.AddObservationDraft(pid, "note", "same words", "h", f64(0.5))
		} else {
			id, err = kb.AddObservation(pid, "note", "same words")
		}
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	ia, ib := mk(a, pa, aStatus), mk(b, pb, bStatus)
	u := obsUUID(t, a, ia)
	if _, err := b.db.Exec(`UPDATE observations SET uuid = ? WHERE id = ?`, u, ib); err != nil {
		t.Fatal(err)
	}
	return openMergedTestKB(t, a, b), u
}

func TestDraftMerge_Status(t *testing.T) {
	for _, c := range []struct{ a, b, want string }{
		{"drafted", "drafted", "drafted"},
		{"reviewed", "reviewed", "reviewed"},
		{"drafted", "reviewed", "reviewed"},
		{"reviewed", "drafted", "reviewed"},
	} {
		m, u := mergePair(t, c.a, c.b)
		if got := obsStatusByUUID(t, m, u); got != c.want {
			t.Errorf("merge(%s, %s) = %s, want %s", c.a, c.b, got, c.want)
		}
		var n int
		m.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&n)
		if n != 1 {
			t.Errorf("merge(%s, %s) kept %d rows", c.a, c.b, n)
		}
		wantFTS := 0
		if c.want == "reviewed" {
			wantFTS = 1
		}
		if got := ftsObservationRows(t, m); got != wantFTS {
			t.Errorf("merge(%s, %s) kb_fts observation rows = %d, want %d", c.a, c.b, got, wantFTS)
		}
	}
}

func TestDraftMerge_KeepsDraftFields(t *testing.T) {
	m, u := mergePair(t, "drafted", "drafted")
	var by string
	var conf float64
	if err := m.db.QueryRow(`SELECT generated_by, confidence FROM observations WHERE uuid = ?`, u).Scan(&by, &conf); err != nil {
		t.Fatal(err)
	}
	if by != "h" || conf != 0.5 {
		t.Errorf("generated_by=%q confidence=%v after merge", by, conf)
	}
}
