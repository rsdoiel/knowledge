package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// `kb record fuzzy-tag` (DR-0052): report near-miss concept mentions in
// decision records; --write adds the concept to `tags:` of non-accepted
// records only, changing nothing else in the file.

const detoastBody = "\n**Context.**\n\nWe detoast the value, then detoasting again. Plain toast here.\n"

func fuzzyTagFixture(t *testing.T, recs ...testRecord) (*knowledge.KnowledgeBase, string) {
	t.Helper()
	kb, root := fixtureWorkspace(t, "cold", recs...)
	if _, err := kb.AddConcept("toast", ""); err != nil {
		t.Fatal(err)
	}
	return kb, root
}

func runFuzzyTag(t *testing.T, kb *knowledge.KnowledgeBase, jsonOut bool, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := cmdRecord(kb, nil, jsonOut, append([]string{"fuzzy-tag"}, args...), &out)
	return out.String(), err
}

func proposed(id string) testRecord {
	return testRecord{ID: id, Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01", Body: detoastBody}
}

func TestRecordFuzzyTag_ReportWritesNothing(t *testing.T) {
	kb, root := fuzzyTagFixture(t, proposed("0013"))
	before := readFixture(t, root, "cold", "0013")
	recBefore, _ := kb.ListRecords(knowledge.RecordFilter{Project: "cold"})
	out, err := runFuzzyTag(t, kb, false, "--project", "cold", "--concept", "toast")
	if err != nil {
		t.Fatalf("fuzzy-tag: %v", err)
	}
	for _, want := range []string{"0013", "toast", "detoast", "tags:"} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "exact mention") {
		t.Errorf("the plain `toast` should be listed as an exact mention, not linked:\n%s", out)
	}
	if after := readFixture(t, root, "cold", "0013"); after != before {
		t.Errorf("report mode changed the file:\n%s", after)
	}
	recAfter, _ := kb.ListRecords(knowledge.RecordFilter{Project: "cold"})
	if len(recBefore) != len(recAfter) || recBefore[0].Checksum != recAfter[0].Checksum {
		t.Error("report mode changed the database")
	}
}

func TestRecordFuzzyTag_NoHintUsesTheConservativeRules(t *testing.T) {
	kb, _ := fuzzyTagFixture(t, proposed("0013"))
	out, err := runFuzzyTag(t, kb, false, "--project", "cold")
	if err != nil {
		t.Fatalf("fuzzy-tag: %v", err)
	}
	if strings.Contains(out, "detoast") {
		t.Errorf("toast is under the length floor, so nothing without --concept:\n%s", out)
	}
}

func TestRecordFuzzyTag_WriteTagsProposedOnceAndIsIdempotent(t *testing.T) {
	kb, root := fuzzyTagFixture(t, proposed("0013"))
	before := readFixture(t, root, "cold", "0013")
	if _, err := runFuzzyTag(t, kb, false, "--project", "cold", "--concept", "toast", "--write"); err != nil {
		t.Fatalf("--write: %v", err)
	}
	after := readFixture(t, root, "cold", "0013")
	if want := strings.Replace(before, `tags: []`, `tags: [toast]`, 1); after != want {
		t.Errorf("file after --write:\n%s\nwant only the tags line changed:\n%s", after, want)
	}
	// Second run: the concept is linked now, so nothing is reported or changed.
	out, err := runFuzzyTag(t, kb, false, "--project", "cold", "--concept", "toast", "--write")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "detoast") {
		t.Errorf("second run still reports a linked concept:\n%s", out)
	}
	if again := readFixture(t, root, "cold", "0013"); again != after {
		t.Error("second --write changed the file")
	}
}

func TestRecordFuzzyTag_AcceptedIsNeverTouchedAndIsCounted(t *testing.T) {
	acc := proposed("0014")
	acc.Status = "accepted"
	kb, root := fuzzyTagFixture(t, proposed("0013"), acc)
	beforeAcc := readFixture(t, root, "cold", "0014")
	out, err := runFuzzyTag(t, kb, false, "--project", "cold", "--concept", "toast", "--write")
	if err != nil {
		t.Fatalf("--write: %v", err)
	}
	if readFixture(t, root, "cold", "0014") != beforeAcc {
		t.Error("an accepted record was modified")
	}
	if !strings.Contains(readFixture(t, root, "cold", "0013"), "tags: [toast]") {
		t.Error("the proposed record was not tagged")
	}
	if !strings.Contains(out, "1 accepted") {
		t.Errorf("the skipped accepted record should be counted:\n%s", out)
	}
	if !strings.Contains(out, "0014") {
		t.Errorf("the accepted record's finding should still be reported:\n%s", out)
	}
}

func TestRecordFuzzyTag_AlreadyTaggedIsNotReported(t *testing.T) {
	r := proposed("0013")
	r.Tags = []string{"toast"}
	kb, root := fuzzyTagFixture(t, r)
	before := readFixture(t, root, "cold", "0013")
	out, err := runFuzzyTag(t, kb, false, "--project", "cold", "--concept", "toast", "--write")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "detoast") || readFixture(t, root, "cold", "0013") != before {
		t.Errorf("an already-tagged record was reported or changed:\n%s", out)
	}
}

func TestRecordFuzzyTag_DryRunWithWriteWritesNothing(t *testing.T) {
	kb, root := fuzzyTagFixture(t, proposed("0013"))
	before := readFixture(t, root, "cold", "0013")
	out, err := runFuzzyTag(t, kb, false, "--project", "cold", "--concept", "toast", "--write", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if readFixture(t, root, "cold", "0013") != before {
		t.Error("--dry-run --write changed the file")
	}
	if !strings.Contains(out, "would") {
		t.Errorf("a dry run should say what it would do:\n%s", out)
	}
}

func TestRecordFuzzyTag_IngestAfterWriteLinksTheConcept(t *testing.T) {
	kb, root := fuzzyTagFixture(t, proposed("0013"))
	if _, err := runFuzzyTag(t, kb, false, "--project", "cold", "--concept", "toast", "--write"); err != nil {
		t.Fatal(err)
	}
	runIngest(t, kb, filepath.Join(root, "cold", "decisions"))
	recs, _ := kb.ListRecords(knowledge.RecordFilter{Project: "cold"})
	linked, err := kb.RecordConcepts(recs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range linked {
		found = found || c.Name == "toast"
	}
	if !found {
		t.Errorf("record concepts after ingest = %+v, want toast", linked)
	}
}

func TestRecordFuzzyTag_JSONShape(t *testing.T) {
	acc := proposed("0014")
	acc.Status = "accepted"
	kb, _ := fuzzyTagFixture(t, proposed("0013"), acc)
	out, err := runFuzzyTag(t, kb, true, "--project", "cold", "--concept", "toast", "--write", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		DryRun          bool `json:"dry_run"`
		Write           bool `json:"write"`
		SkippedAccepted int  `json:"skipped_accepted"`
		Records         []struct {
			RecordID string `json:"record_id"`
			Status   string `json:"status"`
			Path     string `json:"path"`
			TagsLine string `json:"tags_line"`
			Action   string `json:"action"`
			Findings []struct {
				Concept       string   `json:"concept"`
				Variants      []string `json:"variants"`
				Count         int      `json:"count"`
				ExactMentions int      `json:"exact_mentions"`
			} `json:"findings"`
		} `json:"records"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("not one JSON object: %v\n%s", err, out)
	}
	if !v.DryRun || !v.Write || v.SkippedAccepted != 1 || len(v.Records) != 2 {
		t.Fatalf("decoded = %+v", v)
	}
	r := v.Records[0]
	if r.RecordID != "0013" || r.TagsLine != "tags: [toast]" || len(r.Findings) != 1 ||
		r.Findings[0].Concept != "toast" || r.Findings[0].Count != 2 || r.Findings[0].ExactMentions != 1 {
		t.Errorf("record 0013 = %+v", r)
	}
	if v.Records[1].Action != "skipped-accepted" {
		t.Errorf("accepted record action = %q", v.Records[1].Action)
	}
}

func TestRecordFuzzyTag_NothingFoundIsSuccess(t *testing.T) {
	kb, _ := fuzzyTagFixture(t, testRecord{ID: "0013", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-01", Body: "\nnothing here\n"})
	out, err := runFuzzyTag(t, kb, false, "--project", "cold", "--concept", "toast")
	if err != nil {
		t.Fatalf("a listing that matches nothing is exit 0: %v", err)
	}
	if !strings.Contains(out, "no near-miss") {
		t.Errorf("output = %q", out)
	}
}

func TestRecordFuzzyTag_UnsupportedTagsFormFailsBeforeAnyWrite(t *testing.T) {
	kb, root := fuzzyTagFixture(t, proposed("0013"), proposed("0012"))
	path := filepath.Join(root, "cold", "decisions", "0012-fixture.md")
	raw, _ := os.ReadFile(path)
	bad := strings.Replace(string(raw), "tags: []", "tags: [a,\n  b]", 1)
	os.WriteFile(path, []byte(bad), 0o644)
	before := readFixture(t, root, "cold", "0013")
	_, err := runFuzzyTag(t, kb, false, "--project", "cold", "--concept", "toast", "--write")
	if err == nil {
		t.Fatal("want an error for a tags: form it will not edit")
	}
	if readFixture(t, root, "cold", "0013") != before {
		t.Error("the run wrote one record and failed on another; it must be both or neither")
	}
}

func TestRecordFuzzyTag_Errors(t *testing.T) {
	kb, _ := fuzzyTagFixture(t, proposed("0013"))
	for _, tc := range []struct {
		name string
		args []string
		want exitClass
	}{
		{"unknown project", []string{"--project", "nosuch"}, classNegative},
		{"unknown concept", []string{"--project", "cold", "--concept", "zzz"}, classNegative},
		{"no project", []string{}, classUsage},
		{"surplus argument", []string{"--project", "cold", "extra"}, classUsage},
		{"bad flag", []string{"--project", "cold", "--bogus"}, classUsage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runFuzzyTag(t, kb, false, tc.args...)
			if err == nil {
				t.Fatal("want an error")
			}
			if got := exitCodeFor(err); got != tc.want {
				t.Errorf("class = %v, want %v (%v)", got, tc.want, err)
			}
		})
	}
}

func TestRecord_OtherSubverbsRefuseFuzzyTagFlags(t *testing.T) {
	kb, _ := fuzzyTagFixture(t, proposed("0013"))
	var out bytes.Buffer
	for _, args := range [][]string{
		{"list", "--write"},
		{"set-status", "0013", "accepted", "--concept", "toast"},
	} {
		err := cmdRecord(kb, nil, false, args, &out)
		if err == nil || !isUsageError(err) {
			t.Errorf("record %v: err = %v, want a usage error", args, err)
		}
	}
}

// One failure of each other class the verb can produce (workspace DR-0003).

func recordPath(root, id string) string {
	return filepath.Join(root, "cold", "decisions", id+"-fixture.md")
}

func TestRecordFuzzyTag_MissingRecordFileIsNoInput(t *testing.T) {
	kb, root := fuzzyTagFixture(t, proposed("0013"))
	os.Remove(recordPath(root, "0013"))
	_, err := runFuzzyTag(t, kb, false, "--project", "cold", "--concept", "toast")
	if err == nil || exitCodeFor(err) != classNoInput {
		t.Errorf("err = %v, class %v, want no_input (66)", err, exitCodeFor(err))
	}
}

func TestRecordFuzzyTag_MalformedRecordFileIsData(t *testing.T) {
	kb, root := fuzzyTagFixture(t, proposed("0013"))
	os.WriteFile(recordPath(root, "0013"), []byte("no frontmatter here\n"), 0o644)
	_, err := runFuzzyTag(t, kb, false, "--project", "cold", "--concept", "toast")
	if err == nil || exitCodeFor(err) != classData {
		t.Errorf("err = %v, class %v, want data (65)", err, exitCodeFor(err))
	}
}

func TestRecordFuzzyTag_UnsupportedTagsFormIsData(t *testing.T) {
	kb, root := fuzzyTagFixture(t, proposed("0013"))
	path := recordPath(root, "0013")
	raw, _ := os.ReadFile(path)
	os.WriteFile(path, []byte(strings.Replace(string(raw), "tags: []", "tags: [a,\n  b]", 1)), 0o644)
	_, err := runFuzzyTag(t, kb, false, "--project", "cold", "--concept", "toast", "--write")
	if err == nil || exitCodeFor(err) != classData {
		t.Errorf("err = %v, class %v, want data (65): the file's content is what is wrong", err, exitCodeFor(err))
	}
}

func TestRecordFuzzyTag_WriteFailureIsIOAndRollsBack(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	kb, root := fuzzyTagFixture(t, proposed("0013"), proposed("0012"))
	// 0012 sorts first and is written first; 0013 is made unwritable, so the
	// run fails part way and must undo the write it already made.
	before12 := readFixture(t, root, "cold", "0012")
	if err := os.Chmod(recordPath(root, "0013"), 0o444); err != nil {
		t.Fatal(err)
	}
	_, err := runFuzzyTag(t, kb, false, "--project", "cold", "--concept", "toast", "--write")
	if err == nil || exitCodeFor(err) != classIO {
		t.Fatalf("err = %v, class %v, want io (74)", err, exitCodeFor(err))
	}
	if readFixture(t, root, "cold", "0012") != before12 {
		t.Error("the write to 0012 was not rolled back")
	}
}
