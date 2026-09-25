package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `record new --trigger bogus` and `--kind bogus` wrote a record and said
// nothing; ingest only warned later. A typo also legitimised itself: once a
// record carried "live-tets", `record list --trigger live-tets` stopped
// erroring, because the filter rule accepts any value a record carries. The
// rule `record list` already uses now guards authoring too: a value outside
// the documented vocabulary is refused unless a record in the database
// already carries it, so an established local convention still works.

func recordNewErr(t *testing.T, args ...string) (root string, out string, err error) {
	t.Helper()
	kb, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "accepted", Date: "2026-08-01"},
		testRecord{ID: "0002", Kind: "refinement", Trigger: "house-style", Status: "accepted", Date: "2026-08-05"},
	)
	full := append([]string{"new", "--project", "clasm", "--title", "Probe", "--dir", "newdir"}, args...)
	var buf bytes.Buffer
	err = cmdRecord(kb, nil, false, full, &buf)
	return root, buf.String(), err
}

func newDirEntries(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "newdir"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestCmdRecord_NewRefusesUnknownTriggerAndKind(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"trigger", []string{"--trigger", "live-tets"}, `unknown trigger "live-tets"`},
		{"trigger case", []string{"--trigger", "Design"}, `unknown trigger "Design"`},
		{"kind", []string{"--trigger", "design", "--kind", "decison"}, `unknown kind "decison"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, out, err := recordNewErr(t, tc.args...)
			if err == nil {
				t.Fatalf("succeeded with %q, want an error", out)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
			if isUsageError(err) {
				t.Errorf("error %v is a usage error; an unknown value is a lookup that found nothing (exit 1), as for record list", err)
			}
			if names := newDirEntries(t, root); len(names) != 0 {
				t.Errorf("a refused record left files behind: %v", names)
			}
		})
	}
}

// The message lists what is accepted, vocabulary plus what records carry.
func TestCmdRecord_NewUnknownValueNamesWhatIsKnown(t *testing.T) {
	_, _, err := recordNewErr(t, "--trigger", "live-tets")
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"design", "live-test", "release-review", "house-style"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should list %q", err, want)
		}
	}
}

func TestCmdRecord_NewAcceptsVocabularyAndCarriedValues(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"vocabulary trigger nothing carries yet", []string{"--trigger", "external"}},
		{"trigger a record carries", []string{"--trigger", "house-style"}},
		{"vocabulary kind", []string{"--trigger", "design", "--kind", "correction"}},
		{"kind a record carries", []string{"--trigger", "design", "--kind", "refinement"}},
		{"default kind", []string{"--trigger", "design"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, out, err := recordNewErr(t, tc.args...)
			if err != nil {
				t.Fatalf("record new %v = %v, want it accepted", tc.args, err)
			}
			if names := newDirEntries(t, root); len(names) != 1 {
				t.Errorf("want exactly one record written, got %v (output %q)", names, out)
			}
		})
	}
}
