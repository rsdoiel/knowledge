package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// writeRaw writes literal file contents into dir, returning the path.
func writeRaw(t *testing.T, dir, name, contents string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
	return path
}

// workspaceRecord renders a workspace-tier record with an explicit uuid.
func workspaceRecord(id, title, uuid string) string {
	return `---
id: "` + id + `"
title: "` + title + `"
date: "2026-08-25"
status: accepted
kind: decision
trigger: design
project: ""
phase: ""
supersedes: []
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: []
tags: []
uuid: "` + uuid + `"
origin_host: "test"
---

**Context.** ` + title + `
`
}

// ─── identity-collision guard ────────────────────────────────────────────────

// Two workspaces each have an agents/decisions/, and a workspace-tier record's
// identity is unique only within one workspace. Ingesting both into one
// database must not silently overwrite the first.
func TestCmdIngest_DifferentUUIDSameIdentityIsRefused(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	wl := filepath.Join(root, "wl", "agents", "decisions")
	lab := filepath.Join(root, "lab", "agents", "decisions")
	writeRaw(t, wl, "0001-a.md", workspaceRecord("0001", "WorkLab side", "11111111-1111-7111-8111-111111111111"))
	writeRaw(t, lab, "0001-b.md", workspaceRecord("0001", "Laboratory side", "22222222-2222-7222-8222-222222222222"))

	if s := runIngest(t, kb, wl); s.Added != 1 {
		t.Fatalf("first workspace = %+v, want 1 added", s)
	}
	s := runIngest(t, kb, lab)

	if s.Added != 0 || s.Updated != 0 {
		t.Errorf("second workspace = %+v, want nothing written", s)
	}
	if s.Failed != 1 {
		t.Errorf("second workspace = %+v, want the collision counted as a failure", s)
	}
	if len(s.Errors) != 1 || !strings.Contains(s.Errors[0], "uuid") {
		t.Errorf("Errors = %v, want one naming the uuid mismatch", s.Errors)
	}

	// The first workspace's record must survive intact.
	rec, err := kb.RecordByIdentity(kb.Workspace(), 0, "workspace", "0001")
	if err != nil {
		t.Fatalf("RecordByIdentity: %v", err)
	}
	if rec.Title != "WorkLab side" {
		t.Errorf("title = %q, want the first record left intact", rec.Title)
	}
}

// A record matching itself is an ordinary update, not a collision.
func TestCmdIngest_SameUUIDIsNotACollision(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "agents", "decisions")
	writeRaw(t, dir, "0001-a.md", workspaceRecord("0001", "Original", "11111111-1111-7111-8111-111111111111"))
	runIngest(t, kb, dir)

	writeRaw(t, dir, "0001-a.md", workspaceRecord("0001", "Revised", "11111111-1111-7111-8111-111111111111"))
	s := runIngest(t, kb, dir)

	if s.Updated != 1 || s.Failed != 0 {
		t.Errorf("summary = %+v, want 1 updated and no failure", s)
	}
	rec, _ := kb.RecordByIdentity(kb.Workspace(), 0, "workspace", "0001")
	if rec.Title != "Revised" {
		t.Errorf("title = %q, want the update applied", rec.Title)
	}
}

// A slug is cosmetic and may be regenerated, so a changed path alone is a
// warning rather than proof of a collision.
func TestCmdIngest_ChangedPathWithoutUUIDsWarns(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "agents", "decisions")
	writeRaw(t, dir, "0001-old-slug.md", workspaceRecord("0001", "Same record", ""))
	runIngest(t, kb, dir)

	if err := os.Remove(filepath.Join(dir, "0001-old-slug.md")); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	writeRaw(t, dir, "0001-new-slug.md", workspaceRecord("0001", "Same record", ""))
	s := runIngest(t, kb, dir)

	if s.Failed != 0 {
		t.Errorf("summary = %+v, want a renamed slug not to fail", s)
	}
	if len(s.Warnings) == 0 {
		t.Error("want a warning that the stored path changed")
	}
}

// ─── record fmt ──────────────────────────────────────────────────────────────

// nonCanonical is a valid record written with a block-sequence decisions[],
// which the format's "inline lists only" rule forbids.
const nonCanonical = `---
id: "0001"
title: "Block sequence decisions"
date: "2026-08-25"
status: accepted
kind: decision
trigger: design
project: clasm
phase: ""
supersedes: []
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions:
  - "First decision"
  - "Second decision"
tags: []
uuid: "33333333-3333-7333-8333-333333333333"
origin_host: "test"
---

**Context.** Written by hand with a block sequence.
`

func TestCmdRecord_FmtNormalisesAndReports(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	writeRaw(t, dir, "0001-block.md", nonCanonical)
	testRecord{ID: "0002", Project: "clasm"}.write(t, dir)

	var summary recordFmtSummary
	runRecordJSON(t, kb, &summary, "fmt", dir)

	if summary.Changed != 1 || summary.Unchanged != 1 {
		t.Errorf("summary = %+v, want 1 changed and 1 unchanged", summary)
	}
	after, err := os.ReadFile(filepath.Join(dir, "0001-block.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(after), `decisions: ["First decision", "Second decision"]`) {
		t.Errorf("decisions was not inlined:\n%s", after)
	}
	if !strings.Contains(string(after), "**Context.** Written by hand with a block sequence.") {
		t.Error("fmt altered the body")
	}
}

func TestCmdRecord_FmtIsIdempotent(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	writeRaw(t, dir, "0001-block.md", nonCanonical)

	var first recordFmtSummary
	runRecordJSON(t, kb, &first, "fmt", dir)
	var second recordFmtSummary
	runRecordJSON(t, kb, &second, "fmt", dir)

	if second.Changed != 0 || second.Unchanged != 1 {
		t.Errorf("second run = %+v, want nothing left to change", second)
	}
}

func TestCmdRecord_FmtDryRunWritesNothing(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	path := writeRaw(t, dir, "0001-block.md", nonCanonical)

	var summary recordFmtSummary
	runRecordJSON(t, kb, &summary, "fmt", dir, "--dry-run")

	if summary.Changed != 1 {
		t.Errorf("summary = %+v, want --dry-run to report the same count", summary)
	}
	after, _ := os.ReadFile(path)
	if string(after) != nonCanonical {
		t.Error("--dry-run modified the file")
	}
}

func TestCmdRecord_FmtRequiresAPath(t *testing.T) {
	kb, _ := openWorkspaceKB(t)
	var out bytes.Buffer
	if err := cmdRecord(kb, nil, false, []string{"fmt"}, &out); err == nil {
		t.Error("expected an error when PATH is missing")
	}
}

func TestCmdRecord_FmtReportsUnparsableFile(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	writeRaw(t, dir, "0001-broken.md", "no frontmatter\n")

	var summary recordFmtSummary
	runRecordJSON(t, kb, &summary, "fmt", dir)
	if summary.Failed != 1 || len(summary.Errors) != 1 {
		t.Errorf("summary = %+v, want the unparsable file reported", summary)
	}
}

// ─── record new ──────────────────────────────────────────────────────────────

// recordNewDefaultDir is where `record new --project P` writes by default:
// agents/projects/P/decisions (DR-0021 item 1).
func recordNewDefaultDir(root, project string) string {
	return filepath.Join(root, "agents", "projects", project, "decisions")
}

func TestCmdRecord_NewDefaultsToProjectFirstLayout(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	runRecord(t, kb, "new", "--project", "clasm", "--title", "Default layout",
		"--trigger", "design", "--root", root)

	dir := recordNewDefaultDir(root, "clasm")
	matches, _ := filepath.Glob(filepath.Join(dir, "0001-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected the default target to be %s, found %v", dir, matches)
	}
}

func TestCmdRecord_NewDirOverridesDefaultPath(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	// --dir, like the default it replaces, is relative to --root -- the same
	// convention every other stored record path already follows.
	runRecord(t, kb, "new", "--project", "clasm", "--title", "Overridden location",
		"--trigger", "design", "--root", root, "--dir", filepath.Join("somewhere", "else"))

	overrideDir := filepath.Join(root, "somewhere", "else")
	matches, _ := filepath.Glob(filepath.Join(overrideDir, "0001-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected --dir to override the default target, found %v", matches)
	}
	defaultMatches, _ := filepath.Glob(filepath.Join(recordNewDefaultDir(root, "clasm"), "*.md"))
	if len(defaultMatches) != 0 {
		t.Errorf("--dir should replace the default target, but found files there too: %v", defaultMatches)
	}

	rf, err := knowledge.ParseRecordFile(matches[0])
	if err != nil {
		t.Fatalf("ParseRecordFile: %v", err)
	}
	if rf.ProjectName != "clasm" {
		t.Errorf("project attribution = %q, want clasm — attribution is frontmatter-driven, not path-driven", rf.ProjectName)
	}
}

func TestCmdRecord_NewAllocatesNextID(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := recordNewDefaultDir(root, "clasm")
	testRecord{ID: "0001"}.write(t, dir)
	testRecord{ID: "0002"}.write(t, dir)

	runRecord(t, kb, "new", "--project", "clasm", "--title", "A new decision",
		"--trigger", "design", "--root", root)

	matches, _ := filepath.Glob(filepath.Join(dir, "0003-*.md"))
	if len(matches) != 1 {
		names, _ := filepath.Glob(filepath.Join(dir, "*.md"))
		t.Fatalf("expected one 0003-*.md, found %v in %v", matches, names)
	}
}

func TestCmdRecord_NewScaffoldsAllFiveHeadings(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := recordNewDefaultDir(root, "clasm")
	testRecord{ID: "0001"}.write(t, dir)
	runRecord(t, kb, "new", "--project", "clasm", "--title", "Scaffolded",
		"--trigger", "design", "--root", root)

	matches, _ := filepath.Glob(filepath.Join(dir, "0002-*.md"))
	body, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	for _, heading := range []string{"**Context.**", "**Decision.**", "**Rationale.**",
		"**Rejected alternatives.**", "**Consequences.**"} {
		if !strings.Contains(string(body), heading) {
			t.Errorf("scaffold is missing %s:\n%s", heading, body)
		}
	}
}

// A model may write a record but may not accept one.
func TestCmdRecord_NewIsProposedAndParses(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := recordNewDefaultDir(root, "clasm")
	testRecord{ID: "0001"}.write(t, dir)
	runRecord(t, kb, "new", "--project", "clasm", "--title", "Proposed by default",
		"--trigger", "plan-review", "--root", root)

	matches, _ := filepath.Glob(filepath.Join(dir, "0002-*.md"))
	rf, err := knowledge.ParseRecordFile(matches[0])
	if err != nil {
		t.Fatalf("the scaffold does not parse: %v", err)
	}
	if rf.Record.Status != "proposed" {
		t.Errorf("status = %q, want proposed", rf.Record.Status)
	}
	if rf.Record.UUID == "" || rf.Record.OriginHost == "" {
		t.Error("scaffold is missing uuid or origin_host")
	}
	if rf.Record.Trigger != "plan-review" {
		t.Errorf("trigger = %q, want plan-review", rf.Record.Trigger)
	}
	if len(rf.Warnings) != 0 {
		t.Errorf("scaffold produced warnings: %v", rf.Warnings)
	}

	// And it must render byte-identically — the scaffold is canonical.
	raw, _ := os.ReadFile(matches[0])
	out, err := knowledge.RenderRecordFile(rf)
	if err != nil {
		t.Fatalf("RenderRecordFile: %v", err)
	}
	if string(out) != string(raw) {
		t.Errorf("scaffold is not in canonical form:\n%s", out)
	}
}

func TestCmdRecord_NewThenIngests(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := recordNewDefaultDir(root, "clasm")
	testRecord{ID: "0001"}.write(t, dir)
	runIngest(t, kb, dir)

	runRecord(t, kb, "new", "--project", "clasm", "--title", "Ingest me",
		"--trigger", "design", "--root", root)

	s := runIngest(t, kb, dir)
	if s.Added != 1 || s.Failed != 0 {
		t.Errorf("summary = %+v, want the scaffold ingested cleanly", s)
	}
}

// trigger is required on a newly authored record; the empty-trigger
// concession is for conversion only.
func TestCmdRecord_NewRequiresTrigger(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm", testRecord{ID: "0001"})
	var out bytes.Buffer
	err := cmdRecord(kb, nil, false, []string{"new", "--project", "clasm",
		"--title", "No trigger", "--root", root}, &out)
	if err == nil {
		t.Fatal("expected an error when --trigger is missing")
	}
	if !strings.Contains(err.Error(), "trigger") {
		t.Errorf("error = %v, want it to name --trigger", err)
	}
}

func TestCmdRecord_NewRequiresTitle(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm", testRecord{ID: "0001"})
	var out bytes.Buffer
	if err := cmdRecord(kb, nil, false, []string{"new", "--project", "clasm",
		"--trigger", "design", "--root", root}, &out); err == nil {
		t.Error("expected an error when --title is missing")
	}
}

func TestCmdRecord_NewWorkspaceTier(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	runRecord(t, kb, "new", "--workspace", "--title", "A workspace decision",
		"--trigger", "design", "--root", root)

	matches, _ := filepath.Glob(filepath.Join(root, "agents", "decisions", "0001-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected one workspace-tier scaffold, found %v", matches)
	}
	rf, err := knowledge.ParseRecordFile(matches[0])
	if err != nil {
		t.Fatalf("ParseRecordFile: %v", err)
	}
	if rf.ProjectName != "" || rf.Record.Scope != "workspace" {
		t.Errorf("project=%q scope=%q, want an empty project and workspace scope", rf.ProjectName, rf.Record.Scope)
	}
}

func TestCmdRecord_NewSlugIsDerivedFromTitle(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := recordNewDefaultDir(root, "clasm")
	testRecord{ID: "0001"}.write(t, dir)
	runRecord(t, kb, "new", "--project", "clasm",
		"--title", "`kb index`: a Generated Index, Never Hand-Edited!",
		"--trigger", "design", "--root", root)

	matches, _ := filepath.Glob(filepath.Join(dir, "0002-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected one scaffold, found %v", matches)
	}
	name := filepath.Base(matches[0])
	if strings.ContainsAny(name, "`:!,") || strings.ToLower(name) != name {
		t.Errorf("slug %q should be lowercased with punctuation stripped", name)
	}
	if len(name) > 60 {
		t.Errorf("slug %q is longer than the format's ~50 character guidance", name)
	}
}

func TestCmdRecord_NewDoesNotIngest(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm", testRecord{ID: "0001"})
	runRecord(t, kb, "new", "--project", "clasm", "--title", "Not yet indexed",
		"--trigger", "design", "--root", root)

	p, _ := kb.ProjectByName("clasm")
	recs, _ := kb.RecordsByProject(p.ID)
	if len(recs) != 1 {
		t.Errorf("got %d records in the database, want 1 — new writes the file only", len(recs))
	}
}
