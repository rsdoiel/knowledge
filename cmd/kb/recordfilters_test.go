package main

import (
	"bytes"
	"strings"
	"testing"
)

// `record list` answered every filter it could not match with "no matching
// records" and exit 0, so a typo (`--status acepted`, `--project clasn`,
// `--since yesterday`) read as an empty result. A value that matches nothing
// because nothing carries it is now told apart from a real filter that just
// happens to be empty.

func listErr(t *testing.T, jsonOut bool, args ...string) (string, error) {
	t.Helper()
	kb, _ := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "accepted", Date: "2026-08-01", Initiative: "real-init"},
		testRecord{ID: "0002", Kind: "correction", Trigger: "live-test", Status: "weird", Date: "2026-08-05"},
	)
	if _, err := kb.AddProject("emptyproj", ""); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	err := cmdRecord(kb, nil, jsonOut, append([]string{"list"}, args...), &out)
	return out.String(), err
}

func TestCmdRecord_ListUnknownFilterValueIsAnError(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--status", "acepted"}, `unknown status "acepted"`},
		{[]string{"--status", "Accepted"}, `unknown status "Accepted"`},
		{[]string{"--kind", "decison"}, `unknown kind "decison"`},
		{[]string{"--trigger", "desing"}, `unknown trigger "desing"`},
		{[]string{"--initiative", "nope"}, `unknown initiative "nope"`},
		{[]string{"--project", "clasn"}, `unknown project "clasn"`},
	} {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			out, err := listErr(t, false, tc.args...)
			if err == nil {
				t.Fatalf("succeeded with %q, want an error", out)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
			if isUsageError(err) {
				t.Errorf("error %v is a usage error; an unknown value is a lookup that found nothing (exit 1)", err)
			}
			if out != "" {
				t.Errorf("stdout = %q, want it empty on error", out)
			}
		})
	}
}

// The message helps the user fix it: it lists what is accepted.
func TestCmdRecord_ListUnknownValueNamesWhatIsKnown(t *testing.T) {
	_, err := listErr(t, false, "--status", "acepted")
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range []string{"proposed", "accepted", "superseded", "rejected", "cancelled", "weird"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should list %q among the known statuses", err, want)
		}
	}
	_, err = listErr(t, false, "--project", "clasn")
	if err == nil || !strings.Contains(err.Error(), "clasm") {
		t.Errorf("unknown project error %v should list the known project clasm", err)
	}
	_, err = listErr(t, false, "--initiative", "nope")
	if err == nil || !strings.Contains(err.Error(), "real-init") {
		t.Errorf("unknown initiative error %v should list the known initiative", err)
	}
}

// A value inside the documented vocabulary, or one the data actually carries,
// is a real filter. Matching nothing is a legitimate answer for it.
func TestCmdRecord_ListRealFilterValuesStayValid(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string // substring of the output
	}{
		{"in vocabulary, no matches", []string{"--status", "rejected"}, "no matching records"},
		{"kind in vocabulary, no matches", []string{"--kind", "refinement"}, "no matching records"},
		{"trigger in vocabulary, no matches", []string{"--trigger", "external"}, "no matching records"},
		{"outside the vocabulary but carried", []string{"--status", "weird"}, "DR-0002"},
		{"an existing project with no records", []string{"--project", "emptyproj"}, "no matching records"},
		{"a real initiative", []string{"--initiative", "real-init"}, "DR-0001"},
		{"several real filters", []string{"--project", "clasm", "--status", "accepted", "--kind", "decision"}, "DR-0001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := listErr(t, false, tc.args...)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("output = %q, want it to contain %q", out, tc.want)
			}
		})
	}
}

func TestCmdRecord_ListSinceMustBeADate(t *testing.T) {
	for _, bad := range []string{"notadate", "yesterday", "2026-13-01", "2026-08-32", "20260801", "2026/08/01", "26-08-01", "2026-8-1"} {
		t.Run("bad "+bad, func(t *testing.T) {
			_, err := listErr(t, false, "--since", bad)
			if err == nil {
				t.Fatalf("--since %q succeeded, want an error", bad)
			}
			if !isUsageError(err) {
				t.Errorf("error %v is not a usage error; a malformed date is a mistake in the command line (exit 2)", err)
			}
			if !strings.Contains(err.Error(), "--since") {
				t.Errorf("error %v should name --since", err)
			}
		})
	}
	for _, good := range []string{"2026", "2026-08", "2026-08-05"} {
		t.Run("good "+good, func(t *testing.T) {
			if _, err := listErr(t, false, "--since", good); err != nil {
				t.Errorf("--since %q: %v", good, err)
			}
		})
	}
}

// Workspace-tier records have no project, so asking for both can only ever
// answer "nothing".
func TestCmdRecord_ListWorkspaceAndProjectAreExclusive(t *testing.T) {
	_, err := listErr(t, false, "--workspace", "--project", "clasm")
	if err == nil {
		t.Fatal("--workspace with --project succeeded, want an error")
	}
	if !isUsageError(err) {
		t.Errorf("error %v is not a usage error", err)
	}
}

func TestCmdRecord_ListJSONReportsTheErrorNotAnEmptyArray(t *testing.T) {
	out, err := listErr(t, true, "--status", "acepted")
	if err == nil {
		t.Fatalf("succeeded with %q, want an error", out)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing (the error goes to stderr as an envelope)", out)
	}
}

// End to end, through mainRun, on a database with no records at all.
func TestMainRun_RecordListExitCodes(t *testing.T) {
	db := seededDB(t)
	for _, tc := range []struct {
		args []string
		want int
	}{
		{[]string{"record", "list"}, 0},
		{[]string{"record", "list", "--status", "accepted"}, 0}, // in the vocabulary, none carried
		{[]string{"record", "list", "--status", "acepted"}, 1},
		{[]string{"record", "list", "--project", "nosuch"}, 1},
		{[]string{"record", "list", "--project", "p"}, 0}, // exists, owns no records
		{[]string{"record", "list", "--since", "yesterday"}, 2},
		{[]string{"record", "list", "--workspace", "--project", "p"}, 2},
	} {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			code, out, errOut := runOn(db, tc.args...)
			if code != tc.want {
				t.Errorf("exit %d, want %d; stdout=%.100q stderr=%q", code, tc.want, out, errOut)
			}
		})
	}
}

// The list is introduced with the field's plural, spelled correctly.
func TestCmdRecord_ListUnknownValueUsesProperPlurals(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--status", "x"}, "known statuses:"},
		{[]string{"--kind", "x"}, "known kinds:"},
		{[]string{"--trigger", "x"}, "known triggers:"},
		{[]string{"--initiative", "x"}, "known initiatives:"},
	} {
		_, err := listErr(t, false, tc.args...)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: error = %v, want it to contain %q", tc.args, err, tc.want)
		}
	}
}
