package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdProject_AddThenList(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"add", "alpha", "first", "project"}, &out); err != nil {
		t.Fatalf("project add: %v", err)
	}
	if !strings.Contains(out.String(), "alpha") {
		t.Errorf("add output = %q, want it to mention alpha", out.String())
	}

	out.Reset()
	if err := cmdProject(kb, nil, false, []string{"list"}, &out); err != nil {
		t.Fatalf("project list: %v", err)
	}
	if !strings.Contains(out.String(), "alpha") {
		t.Errorf("list output = %q, want it to mention alpha", out.String())
	}
}

func TestCmdProject_AddJSON(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, true, []string{"add", "beta"}, &out); err != nil {
		t.Fatalf("project add: %v", err)
	}
	var got struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v (%q)", err, out.String())
	}
	if got.Name != "beta" || got.ID == 0 {
		t.Errorf("got = %+v, want Name=beta and non-zero ID", got)
	}
}

func TestCmdProject_ShowNotFound(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	err := cmdProject(kb, nil, false, []string{"show", "nonexistent"}, &out)
	if err == nil {
		t.Error("expected an error for a nonexistent project")
	}
}

func TestCmdProject_ShowFound(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("gamma", "a description"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"show", "gamma"}, &out); err != nil {
		t.Fatalf("project show: %v", err)
	}
	if !strings.Contains(out.String(), "gamma") || !strings.Contains(out.String(), "a description") {
		t.Errorf("show output = %q, want it to mention name and description", out.String())
	}
}

func TestCmdProject_Concepts(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("delta", "")
	cid, _ := kb.AddConcept("streaming", "")
	if err := kb.LinkProjectConcept(pid, cid); err != nil {
		t.Fatalf("LinkProjectConcept: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"concepts", "delta"}, &out); err != nil {
		t.Fatalf("project concepts: %v", err)
	}
	if !strings.Contains(out.String(), "streaming") {
		t.Errorf("concepts output = %q, want it to mention streaming", out.String())
	}
}

func TestCmdProject_AddWithStatus(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"add", "--status", "concept", "epsilon", "desc"}, &out); err != nil {
		t.Fatalf("project add --status: %v", err)
	}
	p, err := kb.ProjectByName("epsilon")
	if err != nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	if p.Status != "concept" {
		t.Errorf("Status = %q, want %q", p.Status, "concept")
	}
}

func TestCmdProject_AddInvalidStatus(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"add", "--status", "bogus", "zeta", "desc"}, &out); err == nil {
		t.Error("expected an error for an invalid status")
	}
}

func TestCmdProject_SetStatus(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("eta", ""); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"set-status", "eta", "paused"}, &out); err != nil {
		t.Fatalf("project set-status: %v", err)
	}
	p, err := kb.ProjectByName("eta")
	if err != nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	if p.Status != "paused" {
		t.Errorf("Status = %q, want %q", p.Status, "paused")
	}
}

func TestCmdProject_SetStatusNotFound(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"set-status", "nonexistent", "active"}, &out); err == nil {
		t.Error("expected an error for a nonexistent project")
	}
}

func TestCmdProject_SetStatusMissingArgs(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"set-status", "eta"}, &out); err == nil {
		t.Error("expected an error for missing STATUS argument")
	}
}

func TestCmdProject_SetDescription(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("theta", "names DESIGN_DECIDE_PLAN.md"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{
		"set-description", "theta", "names", "DESIGN_REVIEW_PLAN_IMPLEMENT.md",
	}, &out); err != nil {
		t.Fatalf("project set-description: %v", err)
	}
	p, err := kb.ProjectByName("theta")
	if err != nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	// Trailing words join with a space, as project add already does.
	if p.Description != "names DESIGN_REVIEW_PLAN_IMPLEMENT.md" {
		t.Errorf("Description = %q, want the joined corrected text", p.Description)
	}
}

func TestCmdProject_SetDescriptionJSON(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("iota", "old"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, true, []string{"set-description", "iota", "new"}, &out); err != nil {
		t.Fatalf("project set-description: %v", err)
	}
	assertValidJSON(t, out.Bytes())
	var got struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v (%q)", err, out.String())
	}
	if got.Name != "iota" || got.Description != "new" {
		t.Errorf("got = %+v, want Name=iota Description=new", got)
	}
}

// `set-description NAME ""` is how a description gets cleared; bare
// `set-description NAME` is a usage error, so the clear stays deliberate.
func TestCmdProject_SetDescriptionEmptyString(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("kappa", "something"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"set-description", "kappa", ""}, &out); err != nil {
		t.Fatalf("project set-description: %v", err)
	}
	p, err := kb.ProjectByName("kappa")
	if err != nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	if p.Description != "" {
		t.Errorf("Description = %q, want it cleared", p.Description)
	}
}

func TestCmdProject_SetDescriptionMissingArgs(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("lambda", "keep me"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"set-description", "lambda"}, &out); err == nil {
		t.Error("expected an error for a missing DESCRIPTION argument")
	}
	p, err := kb.ProjectByName("lambda")
	if err != nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	if p.Description != "keep me" {
		t.Errorf("Description = %q, want the usage error to leave it alone", p.Description)
	}
}

func TestCmdProject_SetDescriptionNotFound(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"set-description", "nonexistent", "x"}, &out); err == nil {
		t.Error("expected an error for a nonexistent project")
	}
}

// ─── rename (DR-0024) ────────────────────────────────────────────────────────

func TestCmdProject_Rename(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("oldname", "a project"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"rename", "oldname", "newname"}, &out); err != nil {
		t.Fatalf("project rename: %v", err)
	}
	p, err := kb.ProjectByName("newname")
	if err != nil || p == nil {
		t.Fatalf("ProjectByName(newname): %v", err)
	}
}

func TestCmdProject_RenameJSON(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("oldname", ""); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, true, []string{"rename", "oldname", "newname"}, &out); err != nil {
		t.Fatalf("project rename: %v", err)
	}
	assertValidJSON(t, out.Bytes())
	var got struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v (%q)", err, out.String())
	}
	if got.Old != "oldname" || got.New != "newname" {
		t.Errorf("got = %+v, want Old=oldname New=newname", got)
	}
}

func TestCmdProject_RenameRequiresTwoArguments(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("oldname", ""); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"rename", "oldname"}, &out); err == nil {
		t.Error("expected an error when NEW is missing")
	}
}

// ─── rename with records (DR-0026 Phase 1) ──────────────────────────────────

// DR-0026 supersedes DR-0024's outright refusal: rename now completes for a
// project that owns records, rewriting every owned record's project:
// frontmatter in place before renaming the project row.
func TestCmdProject_RenameRewritesCorpusWhenProjectOwnsRecords(t *testing.T) {
	kb, root := fixtureWorkspace(t, "hasrecords", testRecord{ID: "0001"}, testRecord{ID: "0002"})
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"rename", "hasrecords", "newname"}, &out); err != nil {
		t.Fatalf("project rename: %v", err)
	}
	p, err := kb.ProjectByName("newname")
	if err != nil || p == nil {
		t.Fatalf("ProjectByName(newname): %v", err)
	}
	if old, err := kb.ProjectByName("hasrecords"); err != nil || old != nil {
		t.Errorf("old name still resolves to %+v, %v, want it gone", old, err)
	}
	for _, id := range []string{"0001", "0002"} {
		src := readFixture(t, root, "hasrecords", id)
		if !strings.Contains(frontmatterLine(t, src, "project"), "newname") {
			t.Errorf("DR-%s frontmatter %q, want it to name the new project", id, frontmatterLine(t, src, "project"))
		}
	}
}

// The record row itself is untouched by rename -- only the very next
// ingest, seeing a changed checksum against an unchanged identity, updates
// it -- so the record still resolves under its original identity right
// after the rename.
func TestCmdProject_RenameLeavesRecordRowUntouched(t *testing.T) {
	kb, _ := fixtureWorkspace(t, "hasrecords", testRecord{ID: "0001"})
	before, err := kb.RecordsByRecordID("0001")
	if err != nil || len(before) != 1 {
		t.Fatalf("RecordsByRecordID before rename: %v, %d matches", err, len(before))
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"rename", "hasrecords", "newname"}, &out); err != nil {
		t.Fatalf("project rename: %v", err)
	}
	after, err := kb.RecordsByRecordID("0001")
	if err != nil || len(after) != 1 {
		t.Fatalf("RecordsByRecordID after rename: %v, %d matches", err, len(after))
	}
	if after[0].ID != before[0].ID || after[0].Checksum != before[0].Checksum {
		t.Errorf("record row changed by rename: before=%+v after=%+v", before[0], after[0])
	}
}

func TestCmdProject_RenameDryRunLeavesFilesAndProjectUnchanged(t *testing.T) {
	kb, root := fixtureWorkspace(t, "hasrecords", testRecord{ID: "0001"})
	before := readFixture(t, root, "hasrecords", "0001")
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"rename", "--dry-run", "hasrecords", "newname"}, &out); err != nil {
		t.Fatalf("project rename --dry-run: %v", err)
	}
	if !strings.Contains(out.String(), "0001") {
		t.Errorf("dry-run output = %q, want it to mention the affected record", out.String())
	}
	after := readFixture(t, root, "hasrecords", "0001")
	if after != before {
		t.Error("dry-run modified the record file")
	}
	if p, err := kb.ProjectByName("hasrecords"); err != nil || p == nil {
		t.Errorf("dry-run renamed the project row: ProjectByName(hasrecords) = %+v, %v", p, err)
	}
	if p, err := kb.ProjectByName("newname"); err != nil || p != nil {
		t.Errorf("dry-run created the new project row: ProjectByName(newname) = %+v, %v", p, err)
	}
}

// The DR-0026 repro, end to end: the deadlock DR-0024 refused to risk was
// re-ingesting a renamed corpus minting a phantom project and duplicating
// the record. After `project rename` rewrites the corpus, re-ingesting it
// must hit the ordinary identity-matched UPDATE path -- not the INSERT path
// that used to collide on records.uuid -- with no phantom project and no
// duplicate row.
func TestCmdProject_RenameThenReingestUpdatesInPlace(t *testing.T) {
	kb, root := fixtureWorkspace(t, "hasrecords", testRecord{ID: "0001"})
	before, err := kb.RecordsByRecordID("0001")
	if err != nil || len(before) != 1 {
		t.Fatalf("RecordsByRecordID before rename: %v, %d matches", err, len(before))
	}

	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"rename", "hasrecords", "newname"}, &out); err != nil {
		t.Fatalf("project rename: %v", err)
	}

	s := runIngest(t, kb, filepath.Join(root, "hasrecords", "decisions"))
	if s.Added != 0 || s.Updated != 1 {
		t.Errorf("re-ingest summary = %+v, want 0 added and 1 updated", s)
	}

	after, err := kb.RecordsByRecordID("0001")
	if err != nil || len(after) != 1 {
		t.Fatalf("RecordsByRecordID after re-ingest: %v, %d matches (want exactly 1, no duplicate)", err, len(after))
	}
	if after[0].ID != before[0].ID {
		t.Errorf("record row id changed from %d to %d, want the same identity reused", before[0].ID, after[0].ID)
	}
	if after[0].ProjectID != before[0].ProjectID {
		t.Errorf("record's project_id changed from %d to %d, want it untouched by rename", before[0].ProjectID, after[0].ProjectID)
	}

	if phantom, err := kb.ProjectByName("hasrecords"); err != nil || phantom != nil {
		t.Errorf("re-ingest minted a phantom project under the old name: %+v, %v", phantom, err)
	}
	projects, err := kb.Projects()
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("got %d projects, want exactly 1 (no phantom)", len(projects))
	}
}

// Refusing when NEW already exists must still happen before any file is
// touched, even when OLD owns records.
func TestCmdProject_RenameRefusesWhenNewNameExistsAndProjectOwnsRecords(t *testing.T) {
	kb, root := fixtureWorkspace(t, "hasrecords", testRecord{ID: "0001"})
	if _, err := kb.AddProject("newname", ""); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	before := readFixture(t, root, "hasrecords", "0001")
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"rename", "hasrecords", "newname"}, &out); err == nil {
		t.Error("expected an error when NEW already names another project")
	}
	after := readFixture(t, root, "hasrecords", "0001")
	if after != before {
		t.Error("refused rename modified the record file")
	}
}

// Every project subcommand has to appear in the verb's own usage line, or it
// is undiscoverable from the error you get by running the verb bare.
func TestCmdProject_UsageListsEverySubcommand(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	err := cmdProject(kb, nil, false, nil, &out)
	if err == nil {
		t.Fatal("expected a usage error for a bare project verb")
	}
	for _, sub := range []string{"add", "list", "show", "concepts", "set-status", "set-description", "rename"} {
		if !strings.Contains(err.Error(), sub) {
			t.Errorf("usage %q does not mention subcommand %q", err.Error(), sub)
		}
	}
}

// kb-project(1) is generated from ProjectHelpText, so a subcommand missing
// from it never reaches the man page.
func TestProjectHelpText_DocumentsSetDescription(t *testing.T) {
	if !strings.Contains(ProjectHelpText, "set-description") {
		t.Error("ProjectHelpText does not document set-description")
	}
}

func TestProjectHelpText_DocumentsRename(t *testing.T) {
	if !strings.Contains(ProjectHelpText, "rename") {
		t.Error("ProjectHelpText does not document rename")
	}
}

func TestCmdProject_UnknownSubcommand(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"bogus"}, &out); err == nil {
		t.Error("expected an error for an unknown project subcommand")
	}
}
