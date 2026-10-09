package main

import (
	"bytes"
	"strings"
	"testing"
)

// `record set-status` applies the D0 transition table (DR-0060): a move the
// table does not list is refused, as a negative answer (exit 1, workspace
// DR-0003), not a usage error. The command line was well formed; the record's
// current state forbids the move. A refused move leaves the file untouched.

func transitionFixture(t *testing.T) (root string, run func(args ...string) (string, error)) {
	t.Helper()
	kb, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01"},
		testRecord{ID: "0002", Kind: "decision", Trigger: "design", Status: "accepted", Date: "2026-08-02"},
		testRecord{ID: "0003", Kind: "decision", Trigger: "design", Status: "rejected", Date: "2026-08-03"},
		testRecord{ID: "0004", Kind: "decision", Trigger: "design", Status: "cancelled", Date: "2026-08-04"},
		testRecord{ID: "0005", Kind: "decision", Trigger: "design", Status: "superseded", SupersededBy: []string{"0006"}, Date: "2026-08-05"},
		testRecord{ID: "0006", Kind: "decision", Trigger: "design", Status: "accepted", Supersedes: []string{"0005"}, Date: "2026-08-06"},
		testRecord{ID: "0007", Kind: "decision", Trigger: "design", Status: "accepted", SupersededBy: []string{"0006"}, Date: "2026-08-07"},
		testRecord{ID: "0008", Kind: "decision", Trigger: "design", Status: "legacy-status", Date: "2026-08-08"},
	)
	return root, func(args ...string) (string, error) {
		var out bytes.Buffer
		err := cmdRecord(kb, nil, false, append([]string{"set-status"}, args...), &out)
		return out.String(), err
	}
}

func TestCmdRecord_SetStatusRefusesAForbiddenTransition(t *testing.T) {
	for _, c := range []struct{ id, to string }{
		{"0001", "proposed"},  // same status
		{"0002", "rejected"},  // adopted, so not "never adopted"
		{"0002", "proposed"},  // no way back
		{"0002", "accepted"},  // same status
		{"0003", "accepted"},  // reopen first, accept from proposed
		{"0003", "cancelled"}, //
		{"0004", "accepted"},  //
		{"0005", "proposed"},  // superseded is terminal
		{"0005", "accepted"},  //
		{"0005", "rejected"},  //
		{"0005", "cancelled"}, //
	} {
		t.Run(c.id+"->"+c.to, func(t *testing.T) {
			root, run := transitionFixture(t)
			before := readFixture(t, root, "clasm", c.id)
			out, err := run(c.id, c.to, "--project", "clasm")
			if err == nil {
				t.Fatalf("set-status %s %s succeeded with %q, want a refusal", c.id, c.to, out)
			}
			if class, ok := classify(err); !ok || class != classNegative {
				t.Errorf("error %v is class %v, want negative (exit 1)", err, class)
			}
			if after := readFixture(t, root, "clasm", c.id); after != before {
				t.Errorf("a refused transition changed the record file:\n%s", after)
			}
		})
	}
}

func TestCmdRecord_SetStatusAppliesAPermittedTransition(t *testing.T) {
	for _, c := range []struct{ id, to string }{
		{"0001", "accepted"},
		{"0001", "rejected"},
		{"0001", "cancelled"}, // pursued or explored, then abandoned; need not have been accepted
		{"0002", "cancelled"},
		{"0003", "proposed"},
		{"0004", "proposed"},
		{"0007", "superseded"}, // accepted, with superseded_by already set
	} {
		t.Run(c.id+"->"+c.to, func(t *testing.T) {
			root, run := transitionFixture(t)
			if out, err := run(c.id, c.to, "--project", "clasm"); err != nil {
				t.Fatalf("set-status %s %s = %v (%q), want it applied", c.id, c.to, err, out)
			}
			if got := readFixture(t, root, "clasm", c.id); !strings.Contains(got, "status: "+c.to) {
				t.Errorf("record file does not carry status %q:\n%s", c.to, got)
			}
		})
	}
}

// superseded needs superseded_by. Without it the refusal points at
// `record supersede`, which writes both sides.
func TestCmdRecord_SetStatusSupersededNeedsALink(t *testing.T) {
	for _, id := range []string{"0001", "0002"} {
		t.Run(id, func(t *testing.T) {
			root, run := transitionFixture(t)
			before := readFixture(t, root, "clasm", id)
			_, err := run(id, "superseded", "--project", "clasm")
			if err == nil {
				t.Fatal("set-status superseded with no superseded_by succeeded")
			}
			if class, ok := classify(err); !ok || class != classNegative {
				t.Errorf("error %v is class %v, want negative (exit 1)", err, class)
			}
			if !strings.Contains(err.Error(), "record supersede") {
				t.Errorf("error %q should point at `record supersede`", err)
			}
			if after := readFixture(t, root, "clasm", id); after != before {
				t.Errorf("a refused transition changed the record file:\n%s", after)
			}
		})
	}
}

// The message names the current status and what is allowed, so the refusal is
// enough to act on without opening the manual.
func TestCmdRecord_SetStatusRefusalNamesTheAlternatives(t *testing.T) {
	_, run := transitionFixture(t)
	_, err := run("0003", "accepted", "--project", "clasm")
	if err == nil {
		t.Fatal("rejected -> accepted succeeded")
	}
	for _, want := range []string{"rejected", "accepted", "proposed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

// A status outside the vocabulary that a record already carries is data to
// repair. It can become proposed and nothing else, so reaching accepted takes
// two moves and the second is the one DR-0061 guards. The table is not applied
// to a move onto a carried value, which stays as it was.
func TestCmdRecord_SetStatusAStrayValueBecomesProposedOnly(t *testing.T) {
	for _, to := range []string{"accepted", "rejected", "cancelled", "superseded"} {
		t.Run(to, func(t *testing.T) {
			root, run := transitionFixture(t)
			before := readFixture(t, root, "clasm", "0008")
			_, err := run("0008", to, "--project", "clasm")
			if err == nil {
				t.Fatalf("legacy-status -> %s succeeded, want a refusal", to)
			}
			if class, ok := classify(err); !ok || class != classNegative {
				t.Errorf("error %v is class %v, want negative (exit 1)", err, class)
			}
			if !strings.Contains(err.Error(), "proposed") {
				t.Errorf("error %q should say the record can become proposed", err)
			}
			if after := readFixture(t, root, "clasm", "0008"); after != before {
				t.Errorf("a refused transition changed the record file:\n%s", after)
			}
		})
	}
	root, run := transitionFixture(t)
	if out, err := run("0008", "proposed", "--project", "clasm"); err != nil {
		t.Fatalf("legacy-status -> proposed = %v (%q), want it applied", err, out)
	}
	if got := readFixture(t, root, "clasm", "0008"); !strings.Contains(got, "status: proposed") {
		t.Errorf("record file does not carry status proposed:\n%s", got)
	}
}

func TestCmdRecord_SetStatusAVocabularyStatusMayBecomeACarriedValue(t *testing.T) {
	_, run := transitionFixture(t)
	if out, err := run("0001", "legacy-status", "--project", "clasm"); err != nil {
		t.Fatalf("proposed -> legacy-status = %v (%q), want it applied", err, out)
	}
}
