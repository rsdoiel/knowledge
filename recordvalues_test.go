package knowledge

import (
	"reflect"
	"testing"
)

// `kb record list --status bogus` used to answer "no matching records", the
// same as a real status nobody happens to have used. Telling a typo from a
// legitimate empty result needs to know which values actually occur, because
// the vocabularies are documented rather than enforced: a record may carry a
// status outside them, and it must stay listable.

func TestDistinctRecordValues(t *testing.T) {
	kb := openTestKB(t)
	add := func(id, status, kind, trigger, initiative string) {
		t.Helper()
		r := newTestRecord(id, "t"+id, "2026-08-01")
		r.Status, r.Kind, r.Trigger, r.Initiative = status, kind, trigger, initiative
		if _, err := kb.AddRecord(r); err != nil {
			t.Fatalf("AddRecord %s: %v", id, err)
		}
	}
	add("0001", "accepted", "decision", "design", "init-b")
	add("0002", "proposed", "decision", "", "init-a")
	add("0003", "weird", "correction", "design", "init-b")

	for field, want := range map[string][]string{
		"status":     {"accepted", "proposed", "weird"},
		"kind":       {"correction", "decision"},
		"trigger":    {"design"}, // the empty trigger is not a value
		"initiative": {"init-a", "init-b"},
	} {
		got, err := kb.DistinctRecordValues(field)
		if err != nil {
			t.Errorf("DistinctRecordValues(%q): %v", field, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("DistinctRecordValues(%q) = %q, want %q (sorted, distinct, non-empty)", field, got, want)
		}
	}
}

func TestDistinctRecordValues_EmptyDatabaseGivesNoValues(t *testing.T) {
	kb := openTestKB(t)
	got, err := kb.DistinctRecordValues("status")
	if err != nil || len(got) != 0 {
		t.Errorf("DistinctRecordValues on an empty database = %q, %v; want none", got, err)
	}
}

// The field name is spliced into SQL, so anything but the four filterable
// columns must be refused before it gets there.
func TestDistinctRecordValues_RefusesOtherFields(t *testing.T) {
	kb := openTestKB(t)
	for _, field := range []string{"", "title", "body", "STATUS", "status; DROP TABLE records", "status --", "r.status"} {
		if _, err := kb.DistinctRecordValues(field); err == nil {
			t.Errorf("DistinctRecordValues(%q) succeeded, want an error", field)
		}
	}
}
