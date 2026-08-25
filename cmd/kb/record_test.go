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

// runRecord calls cmdRecord and returns its stdout.
func runRecord(t *testing.T, kb *knowledge.KnowledgeBase, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	if err := cmdRecord(kb, nil, false, args, &out); err != nil {
		t.Fatalf("cmdRecord %v: %v", args, err)
	}
	return out.String()
}

// runRecordJSON calls cmdRecord in JSON mode and decodes into v.
func runRecordJSON(t *testing.T, kb *knowledge.KnowledgeBase, v any, args ...string) {
	t.Helper()
	var out bytes.Buffer
	if err := cmdRecord(kb, nil, true, args, &out); err != nil {
		t.Fatalf("cmdRecord %v: %v", args, err)
	}
	assertValidJSON(t, out.Bytes())
	if err := json.Unmarshal(out.Bytes(), v); err != nil {
		t.Fatalf("decoding %v: %v\n%s", args, err, out.String())
	}
}

// fixtureWorkspace writes records into <root>/<project>/decisions and ingests
// them, returning the open knowledge base and the workspace root.
func fixtureWorkspace(t *testing.T, project string, records ...testRecord) (*knowledge.KnowledgeBase, string) {
	t.Helper()
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, project, "decisions")
	for _, r := range records {
		r.Project = project
		r.write(t, dir)
	}
	runIngest(t, kb, dir)
	return kb, root
}

// readFixture returns the contents of a record file in a fixture workspace.
func readFixture(t *testing.T, root, project, id string) string {
	t.Helper()
	path := filepath.Join(root, project, "decisions", id+"-fixture.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	return string(b)
}

// frontmatterLine returns the frontmatter line beginning with field+":".
func frontmatterLine(t *testing.T, src, field string) string {
	t.Helper()
	for _, line := range strings.Split(src, "\n") {
		if line == "---" && field == "" {
			break
		}
		if strings.HasPrefix(line, field+":") {
			return line
		}
	}
	t.Fatalf("no %q line in:\n%s", field, src)
	return ""
}

// ─── list ────────────────────────────────────────────────────────────────────

// Plan finding 3: ids are identity, not chronology, so ordering is by date
// then record_id.
func TestCmdRecord_ListSortsByDateThenRecordID(t *testing.T) {
	kb, _ := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0160", Date: "2026-08-18"},
		testRecord{ID: "0159", Date: "2026-08-19"},
		testRecord{ID: "0158", Date: "2026-08-19"},
	)

	var got []recordListEntry
	runRecordJSON(t, kb, &got, "list", "--project", "clasm")

	want := []string{"0160", "0158", "0159"}
	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].RecordID != want[i] {
			t.Errorf("position %d = %s, want %s", i, got[i].RecordID, want[i])
		}
	}
}

func TestCmdRecord_ListFilters(t *testing.T) {
	kb, _ := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "accepted", Date: "2026-08-01"},
		testRecord{ID: "0002", Kind: "correction", Trigger: "live-test", Status: "accepted", Date: "2026-08-05"},
		testRecord{ID: "0003", Kind: "correction", Trigger: "design", Status: "proposed", Date: "2026-08-10"},
	)

	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"kind", []string{"--kind", "correction"}, []string{"0002", "0003"}},
		{"trigger", []string{"--trigger", "design"}, []string{"0001", "0003"}},
		{"status", []string{"--status", "proposed"}, []string{"0003"}},
		{"since", []string{"--since", "2026-08-05"}, []string{"0002", "0003"}},
		{"combined", []string{"--kind", "correction", "--trigger", "design"}, []string{"0003"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []recordListEntry
			runRecordJSON(t, kb, &got, append([]string{"list", "--project", "clasm"}, tc.args...)...)
			var ids []string
			for _, r := range got {
				ids = append(ids, r.RecordID)
			}
			if strings.Join(ids, ",") != strings.Join(tc.want, ",") {
				t.Errorf("got %v, want %v", ids, tc.want)
			}
		})
	}
}

func TestCmdRecord_ListByInitiative(t *testing.T) {
	kb, _ := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Initiative: "eprints-to-rdm"},
		testRecord{ID: "0002"},
	)
	var got []recordListEntry
	runRecordJSON(t, kb, &got, "list", "--initiative", "eprints-to-rdm")
	if len(got) != 1 || got[0].RecordID != "0001" {
		t.Errorf("got %+v, want only 0001", got)
	}
}

func TestCmdRecord_ListSeparatesTiers(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	testRecord{ID: "0001", Project: "clasm"}.write(t, filepath.Join(root, "clasm", "decisions"))
	testRecord{ID: "0001", Project: ""}.write(t, filepath.Join(root, "agents", "decisions"))
	runIngest(t, kb, filepath.Join(root, "clasm", "decisions"))
	runIngest(t, kb, filepath.Join(root, "agents", "decisions"))

	var project, workspace []recordListEntry
	runRecordJSON(t, kb, &project, "list", "--project", "clasm")
	runRecordJSON(t, kb, &workspace, "list", "--workspace")

	if len(project) != 1 || project[0].Scope != "project" {
		t.Errorf("--project returned %+v, want one project-tier record", project)
	}
	if len(workspace) != 1 || workspace[0].Scope != "workspace" {
		t.Errorf("--workspace returned %+v, want one workspace-tier record", workspace)
	}
}

func TestCmdRecord_ListHumanReadable(t *testing.T) {
	kb, _ := fixtureWorkspace(t, "clasm", testRecord{ID: "0042", Title: "A findable title"})
	out := runRecord(t, kb, "list", "--project", "clasm")
	if !strings.Contains(out, "DR-0042") || !strings.Contains(out, "A findable title") {
		t.Errorf("output = %q, want it to name the record", out)
	}
}

// ─── show ────────────────────────────────────────────────────────────────────

func TestCmdRecord_ShowReportsRelationsBothDirections(t *testing.T) {
	kb, _ := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0159", Supersedes: []string{"0160"}},
		testRecord{ID: "0160", Status: "accepted", SupersededBy: []string{"0159"}},
	)

	out := runRecord(t, kb, "show", "0160")
	if !strings.Contains(out, "superseded_by") || !strings.Contains(out, "0159") {
		t.Errorf("show 0160 = %q, want it to report superseded_by 0159", out)
	}
	if !strings.Contains(out, "accepted") {
		t.Errorf("show 0160 = %q, want status accepted alongside the supersession", out)
	}

	out = runRecord(t, kb, "show", "0159")
	if !strings.Contains(out, "supersedes") || !strings.Contains(out, "0160") {
		t.Errorf("show 0159 = %q, want it to report supersedes 0160", out)
	}
}

func TestCmdRecord_ShowIncludesBody(t *testing.T) {
	kb, _ := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Body: "\n**Context.** A distinctive body sentence.\n"})
	out := runRecord(t, kb, "show", "0001")
	if !strings.Contains(out, "A distinctive body sentence") {
		t.Errorf("show = %q, want the body included", out)
	}
}

// A bare id is ambiguous when two projects both have it, so the command must
// say so rather than pick one.
func TestCmdRecord_ShowAmbiguousRecordID(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	testRecord{ID: "0001", Project: "clasm"}.write(t, filepath.Join(root, "clasm", "decisions"))
	testRecord{ID: "0001", Project: "cold"}.write(t, filepath.Join(root, "cold", "decisions"))
	runIngest(t, kb, filepath.Join(root, "clasm", "decisions"))
	runIngest(t, kb, filepath.Join(root, "cold", "decisions"))

	var out bytes.Buffer
	err := cmdRecord(kb, nil, false, []string{"show", "0001"}, &out)
	if err == nil {
		t.Fatal("an ambiguous record id was resolved silently, want an error")
	}
	if !strings.Contains(err.Error(), "clasm") || !strings.Contains(err.Error(), "cold") {
		t.Errorf("error = %v, want it to name both candidates", err)
	}

	// Qualifying it resolves.
	if out := runRecord(t, kb, "show", "0001", "--project", "cold"); !strings.Contains(out, "cold") {
		t.Errorf("show --project cold = %q, want the cold record", out)
	}
}

func TestCmdRecord_ShowUnknownRecord(t *testing.T) {
	kb, _ := fixtureWorkspace(t, "clasm", testRecord{ID: "0001"})
	var out bytes.Buffer
	if err := cmdRecord(kb, nil, false, []string{"show", "9999"}, &out); err == nil {
		t.Error("expected an error for an unknown record id")
	}
}

// ─── set-status ──────────────────────────────────────────────────────────────

func TestCmdRecord_SetStatusWritesFileAndDatabase(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm", testRecord{ID: "0001", Status: "proposed"})

	runRecord(t, kb, "set-status", "0001", "accepted")

	if got := frontmatterLine(t, readFixture(t, root, "clasm", "0001"), "status"); got != "status: accepted" {
		t.Errorf("file line = %q, want %q", got, "status: accepted")
	}
	p, _ := kb.ProjectByName("clasm")
	rec, err := kb.RecordByIdentity(p.ID, "project", "0001")
	if err != nil {
		t.Fatalf("RecordByIdentity: %v", err)
	}
	if rec.Status != "accepted" {
		t.Errorf("database status = %q, want accepted", rec.Status)
	}
}

func TestCmdRecord_SetStatusLeavesEveryOtherLineUntouched(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Status: "proposed", Title: "Keep me exactly", Trigger: "design"})
	before := readFixture(t, root, "clasm", "0001")

	runRecord(t, kb, "set-status", "0001", "accepted")
	after := readFixture(t, root, "clasm", "0001")

	beforeLines := strings.Split(before, "\n")
	afterLines := strings.Split(after, "\n")
	if len(beforeLines) != len(afterLines) {
		t.Fatalf("line count changed: %d -> %d", len(beforeLines), len(afterLines))
	}
	for i := range beforeLines {
		if beforeLines[i] == afterLines[i] {
			continue
		}
		if strings.HasPrefix(beforeLines[i], "status:") {
			continue
		}
		t.Errorf("line %d changed unexpectedly:\n before: %q\n  after: %q", i, beforeLines[i], afterLines[i])
	}
}

func TestCmdRecord_SetStatusRejectsUnknownRecord(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm", testRecord{ID: "0001"})
	before := readFixture(t, root, "clasm", "0001")

	var out bytes.Buffer
	if err := cmdRecord(kb, nil, false, []string{"set-status", "9999", "accepted"}, &out); err == nil {
		t.Error("expected an error for an unknown record id")
	}
	if readFixture(t, root, "clasm", "0001") != before {
		t.Error("a failed set-status modified an unrelated file")
	}
}

func TestCmdRecord_SetStatusRequiresTwoArguments(t *testing.T) {
	kb, _ := fixtureWorkspace(t, "clasm", testRecord{ID: "0001"})
	var out bytes.Buffer
	if err := cmdRecord(kb, nil, false, []string{"set-status", "0001"}, &out); err == nil {
		t.Error("expected a usage error when STATUS is missing")
	}
}

// ─── supersede ───────────────────────────────────────────────────────────────

func TestCmdRecord_SupersedeWritesBothSides(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0148", Status: "accepted"},
		testRecord{ID: "0149", Status: "accepted"},
	)

	runRecord(t, kb, "supersede", "0149", "0148")

	newer := readFixture(t, root, "clasm", "0149")
	if got := frontmatterLine(t, newer, "supersedes"); got != `supersedes: ["0148"]` {
		t.Errorf("0149 supersedes line = %q, want %q", got, `supersedes: ["0148"]`)
	}
	older := readFixture(t, root, "clasm", "0148")
	if got := frontmatterLine(t, older, "superseded_by"); got != `superseded_by: ["0149"]` {
		t.Errorf("0148 superseded_by line = %q, want %q", got, `superseded_by: ["0149"]`)
	}
	if got := frontmatterLine(t, older, "status"); got != "status: superseded" {
		t.Errorf("0148 status line = %q, want %q without --partial", got, "status: superseded")
	}

	p, _ := kb.ProjectByName("clasm")
	newerRec, _ := kb.RecordByIdentity(p.ID, "project", "0149")
	olderRec, _ := kb.RecordByIdentity(p.ID, "project", "0148")
	rels, err := kb.RelationsFor(newerRec.ID)
	if err != nil {
		t.Fatalf("RelationsFor: %v", err)
	}
	if len(rels) != 1 || rels[0].Relationship != "supersedes" || rels[0].RecordID != olderRec.ID {
		t.Errorf("relations = %+v, want one supersedes pointing at 0148", rels)
	}
	if olderRec.Status != "superseded" {
		t.Errorf("database status = %q, want superseded", olderRec.Status)
	}
}

// Plan finding 1: a partial supersession leaves the older record accepted,
// because a later record can invalidate one decision inside a multi-decision
// episode while the rest stand. clasm DR-0160 is the real case.
func TestCmdRecord_SupersedePartialLeavesStatusAccepted(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0159", Status: "accepted"},
		testRecord{ID: "0160", Status: "accepted"},
	)

	runRecord(t, kb, "supersede", "0159", "0160", "--partial")

	older := readFixture(t, root, "clasm", "0160")
	if got := frontmatterLine(t, older, "superseded_by"); got != `superseded_by: ["0159"]` {
		t.Errorf("0160 superseded_by = %q, want %q", got, `superseded_by: ["0159"]`)
	}
	if got := frontmatterLine(t, older, "status"); got != "status: accepted" {
		t.Errorf("0160 status = %q, want it left accepted under --partial", got)
	}

	p, _ := kb.ProjectByName("clasm")
	olderRec, _ := kb.RecordByIdentity(p.ID, "project", "0160")
	if olderRec.Status != "accepted" {
		t.Errorf("database status = %q, want accepted under --partial", olderRec.Status)
	}
}

func TestCmdRecord_SupersedeUnknownRecordChangesNothingOnDisk(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0148", Status: "accepted"},
		testRecord{ID: "0149", Status: "accepted"},
	)
	before148 := readFixture(t, root, "clasm", "0148")
	before149 := readFixture(t, root, "clasm", "0149")

	var out bytes.Buffer
	if err := cmdRecord(kb, nil, false, []string{"supersede", "0149", "9999"}, &out); err == nil {
		t.Fatal("expected an error superseding a record that does not exist")
	}
	if readFixture(t, root, "clasm", "0148") != before148 {
		t.Error("0148 changed despite the failure")
	}
	if readFixture(t, root, "clasm", "0149") != before149 {
		t.Error("0149 changed despite the failure — both sides must succeed together or not at all")
	}
}

// Supersession is same-tier only: writing both sides across tiers would mean
// writing into another repository.
func TestCmdRecord_SupersedeAcrossTiersIsRejected(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	testRecord{ID: "0160", Project: "clasm"}.write(t, filepath.Join(root, "clasm", "decisions"))
	testRecord{ID: "0001", Project: ""}.write(t, filepath.Join(root, "agents", "decisions"))
	runIngest(t, kb, filepath.Join(root, "clasm", "decisions"))
	runIngest(t, kb, filepath.Join(root, "agents", "decisions"))

	var out bytes.Buffer
	// Each id is unique on its own tier, so both resolve without qualifying;
	// the tier mismatch is what must be caught.
	err := cmdRecord(kb, nil, false, []string{"supersede", "0001", "0160"}, &out)
	if err == nil {
		t.Fatal("a cross-tier supersession was accepted, want an error")
	}
	if !strings.Contains(err.Error(), "tier") {
		t.Errorf("error = %v, want it to explain that supersession is same-tier only", err)
	}
}

func TestCmdRecord_SupersedeIsIdempotent(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0148", Status: "accepted"},
		testRecord{ID: "0149", Status: "accepted"},
	)
	runRecord(t, kb, "supersede", "0149", "0148")
	first := readFixture(t, root, "clasm", "0149")

	runRecord(t, kb, "supersede", "0149", "0148")
	if got := readFixture(t, root, "clasm", "0149"); got != first {
		t.Errorf("a repeated supersede changed the file again:\n%s", got)
	}
	if got := frontmatterLine(t, first, "supersedes"); got != `supersedes: ["0148"]` {
		t.Errorf("supersedes = %q, want the id listed once", got)
	}
}

func TestCmdRecord_SupersedeRequiresTwoArguments(t *testing.T) {
	kb, _ := fixtureWorkspace(t, "clasm", testRecord{ID: "0001"})
	var out bytes.Buffer
	if err := cmdRecord(kb, nil, false, []string{"supersede", "0001"}, &out); err == nil {
		t.Error("expected a usage error when OLD is missing")
	}
}

func TestCmdRecord_UnknownSubverb(t *testing.T) {
	kb, _ := fixtureWorkspace(t, "clasm", testRecord{ID: "0001"})
	var out bytes.Buffer
	if err := cmdRecord(kb, nil, false, []string{"frobnicate"}, &out); err == nil {
		t.Error("expected an error for an unknown subverb")
	}
}

// ─── live corpus ─────────────────────────────────────────────────────────────

func TestCmdRecord_LiveClasmFilters(t *testing.T) {
	dir := liveCorpus(t, "clasm", "decisions")
	kb, _ := openWorkspaceKB(t)
	runIngest(t, kb, dir)

	for _, tc := range []struct {
		args []string
		want int
	}{
		{[]string{"--kind", "correction"}, 25},
		{[]string{"--trigger", "live-test"}, 59},
	} {
		var got []recordListEntry
		runRecordJSON(t, kb, &got, append([]string{"list", "--project", "clasm"}, tc.args...)...)
		if len(got) != tc.want {
			t.Errorf("list %v returned %d records, want %d", tc.args, len(got), tc.want)
		}
	}
}

// clasm DR-0160 is accepted and partially superseded by DR-0159 — the real
// case the --partial flag exists for.
func TestCmdRecord_LiveShowPartialSupersession(t *testing.T) {
	dir := liveCorpus(t, "clasm", "decisions")
	kb, _ := openWorkspaceKB(t)
	runIngest(t, kb, dir)

	out := runRecord(t, kb, "show", "0160", "--project", "clasm")
	if !strings.Contains(out, "superseded_by") || !strings.Contains(out, "0159") {
		t.Errorf("show 0160 = %q, want superseded_by 0159", out)
	}
	if !strings.Contains(out, "accepted") {
		t.Errorf("show 0160 = %q, want status accepted reported alongside it", out)
	}
}
