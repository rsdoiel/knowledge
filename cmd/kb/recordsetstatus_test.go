package main

import (
	"bytes"
	"strings"
	"testing"
)

// `record set-status ID bogus` wrote the status and exited 0. It is the
// promotion path and the most consequential field: a typo such as "acepted" left
// the record in limbo, invisible to `record list --status accepted`, and once one
// record carried it the filter stopped erroring for it. The rule `record new`
// uses for trigger and kind now applies to status (DR-0048): a value outside the
// vocabulary is refused unless a record already carries it. It is a value being
// written, so a usage error (exit 2).

func setStatusFixture(t *testing.T) (root string, run func(args ...string) (string, error)) {
	t.Helper()
	kb, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"},
		testRecord{ID: "0002", Kind: "decision", Trigger: "design", Status: "legacy-status", Date: "2026-08-02"},
	)
	return root, func(args ...string) (string, error) {
		var out bytes.Buffer
		err := cmdRecord(kb, nil, false, append([]string{"set-status"}, args...), &out)
		return out.String(), err
	}
}

func TestCmdRecord_SetStatusRefusesAnUnknownStatus(t *testing.T) {
	for _, status := range []string{"bogus", "acepted", "Accepted", "ACCEPTED", "accepted ", "done"} {
		t.Run(status, func(t *testing.T) {
			root, run := setStatusFixture(t)
			before := readFixture(t, root, "clasm", "0001")
			out, err := run("0001", status, "--project", "clasm")
			if err == nil {
				t.Fatalf("set-status %q succeeded with %q, want an error", status, out)
			}
			if !isUsageError(err) {
				t.Errorf("error %v is not a usage error; a status being written is exit 2", err)
			}
			if !strings.Contains(err.Error(), "unknown status") || !strings.Contains(err.Error(), "accepted") {
				t.Errorf("error %q should say the status is unknown and list the known ones", err)
			}
			if after := readFixture(t, root, "clasm", "0001"); after != before {
				t.Errorf("a refused set-status changed the record file:\n%s", after)
			}
		})
	}
}

func TestCmdRecord_SetStatusAcceptsTheVocabularyAndCarriedValues(t *testing.T) {
	for _, status := range []string{"proposed", "accepted", "superseded", "rejected", "cancelled", "legacy-status"} {
		t.Run(status, func(t *testing.T) {
			root, run := setStatusFixture(t)
			if _, err := run("0001", status, "--project", "clasm"); err != nil {
				t.Fatalf("set-status %q = %v, want it accepted", status, err)
			}
			got := readFixture(t, root, "clasm", "0001")
			if !strings.Contains(got, "status: "+status) {
				t.Errorf("record file does not carry status %q:\n%s", status, got)
			}
		})
	}
}

func TestCmdRecord_SetStatusRefusesAnEmptyStatus(t *testing.T) {
	_, run := setStatusFixture(t)
	if _, err := run("0001", "", "--project", "clasm"); err == nil || !isUsageError(err) {
		t.Errorf("an empty status: err = %v, want a usage error", err)
	}
}

// The check is on the value alone, before a record is even looked up, so a bad
// status on a missing record is the usage error, not a not-found.
func TestCmdRecord_SetStatusChecksTheValueFirst(t *testing.T) {
	_, run := setStatusFixture(t)
	_, err := run("9999", "bogus", "--project", "clasm")
	if err == nil || !isUsageError(err) {
		t.Errorf("err = %v, want the usage error for the bad status", err)
	}
}
