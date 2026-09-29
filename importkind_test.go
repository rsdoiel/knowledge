package knowledge

import (
	"bytes"
	"strings"
	"testing"
)

// An observation kind outside the current vocabulary (a legacy value such as
// "release") must survive import verbatim and be reported, not rewritten to
// "note". Silent coercion made CompareToJSONL report a false divergence and
// would have corrupted the authoritative dump on the recommended fix.

func legacyKindFixture(t *testing.T) (*KnowledgeBase, string) {
	t.Helper()
	kb := openTestKB(t)
	pid, err := kb.AddProject("legacy", "holds a legacy-kind observation")
	if err != nil {
		t.Fatal(err)
	}
	id, err := kb.AddObservation(pid, "note", "cut release 1.0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kb.db.Exec(`UPDATE observations SET kind = 'release' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	var uuid string
	if err := kb.db.QueryRow(`SELECT uuid FROM observations WHERE id = ?`, id).Scan(&uuid); err != nil {
		t.Fatal(err)
	}
	return kb, uuid
}

func TestImportJSONL_UnknownObservationKindIsPreserved(t *testing.T) {
	src, uuid := legacyKindFixture(t)
	dst := openTestKB(t)
	if _, err := ImportJSONL(dst, strings.NewReader(dumpOf(t, src))); err != nil {
		t.Fatal(err)
	}
	var kind string
	if err := dst.db.QueryRow(`SELECT kind FROM observations WHERE uuid = ?`, uuid).Scan(&kind); err != nil {
		t.Fatal(err)
	}
	if kind != "release" {
		t.Errorf("imported kind = %q, want %q", kind, "release")
	}
}

func TestImportJSONL_UnknownObservationKindIsReported(t *testing.T) {
	src, uuid := legacyKindFixture(t)
	dst := openTestKB(t)
	summary, err := ImportJSONL(dst, strings.NewReader(dumpOf(t, src)))
	if err != nil {
		t.Fatal(err)
	}
	s := summaryByTable(summary)[recObservation]
	if s.Imported != 1 {
		t.Errorf("imported = %d, want 1 (a warning is not a failure)", s.Imported)
	}
	if len(s.Warnings) != 1 || !strings.Contains(s.Warnings[0], "release") || !strings.Contains(s.Warnings[0], uuid) {
		t.Errorf("warnings = %q, want one naming kind %q and uuid %s", s.Warnings, "release", uuid)
	}
}

func TestImportJSONL_KnownObservationKindsGiveNoWarning(t *testing.T) {
	src := newJSONLFixture(t)
	dst := openTestKB(t)
	summary, err := ImportJSONL(dst, strings.NewReader(dumpOf(t, src.kb)))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range summary {
		if len(s.Warnings) != 0 {
			t.Errorf("%s: unexpected warnings %q", s.Table, s.Warnings)
		}
	}
}

func TestCompareToJSONL_LegacyObservationKindIsNotADivergence(t *testing.T) {
	kb, _ := legacyKindFixture(t)
	c, err := kb.CompareToJSONL(bytes.NewReader([]byte(dumpOf(t, kb))))
	if err != nil {
		t.Fatal(err)
	}
	if !c.InSync() {
		t.Errorf("recommendation = %q (tables %+v), want in-sync", c.Recommendation(), c.Tables)
	}
}
