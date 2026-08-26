package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// testRecord describes a decision record to write into a fixture tree.
type testRecord struct {
	ID           string
	Project      string
	Title        string
	Date         string
	Status       string
	Kind         string
	Trigger      string
	Supersedes   []string
	SupersededBy []string
	RelatesTo    []string
	Initiative   string
	Body         string
}

// flowList renders a string slice as a flow sequence of quoted items.
func flowList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = `"` + s + `"`
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// write renders the record and writes it into dir, returning its path.
func (r testRecord) write(t *testing.T, dir string) string {
	t.Helper()
	if r.Date == "" {
		r.Date = "2026-08-01"
	}
	if r.Status == "" {
		r.Status = "accepted"
	}
	if r.Kind == "" {
		r.Kind = "decision"
	}
	if r.Title == "" {
		r.Title = "Record " + r.ID
	}
	if r.Body == "" {
		r.Body = "\n**Context.** Fixture record " + r.ID + ".\n"
	}
	src := fmt.Sprintf(`---
id: "%s"
title: "%s"
date: "%s"
status: %s
kind: %s
trigger: %s
project: %s
phase: ""
supersedes: %s
superseded_by: %s
relates_to: %s
initiative: "%s"
session: ""
decisions: []
tags: []
uuid: ""
origin_host: ""
---%s`,
		r.ID, r.Title, r.Date, r.Status, r.Kind,
		quoteIfEmpty(r.Trigger), quoteIfEmpty(r.Project),
		flowList(r.Supersedes), flowList(r.SupersededBy), flowList(r.RelatesTo),
		r.Initiative, r.Body)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	path := filepath.Join(dir, r.ID+"-fixture.md")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
	return path
}

// quoteIfEmpty renders "" for an empty value and the bare value otherwise,
// matching the canonical rendering of trigger, project and session.
func quoteIfEmpty(s string) string {
	if s == "" {
		return `""`
	}
	return s
}

// openWorkspaceKB creates a workspace root with agents/knowledge.db inside it,
// so that ingest's default root (the parent of the database's directory) is
// the workspace root.
func openWorkspaceKB(t *testing.T) (*knowledge.KnowledgeBase, string) {
	t.Helper()
	root := t.TempDir()
	kb, err := knowledge.Open(knowledge.DefaultPath(root))
	if err != nil {
		t.Fatalf("knowledge.Open: %v", err)
	}
	t.Cleanup(func() { kb.Close() })
	return kb, root
}

// runIngest calls cmdIngest and decodes its JSON summary.
func runIngest(t *testing.T, kb *knowledge.KnowledgeBase, args ...string) ingestSummary {
	t.Helper()
	var out bytes.Buffer
	if err := cmdIngest(kb, nil, true, args, &out); err != nil {
		t.Fatalf("cmdIngest %v: %v", args, err)
	}
	assertValidJSON(t, out.Bytes())
	var s ingestSummary
	if err := json.Unmarshal(out.Bytes(), &s); err != nil {
		t.Fatalf("decoding summary: %v\n%s", err, out.String())
	}
	return s
}

func TestCmdIngest_AddsRecords(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	testRecord{ID: "0001", Project: "clasm", Trigger: "design"}.write(t, dir)
	testRecord{ID: "0002", Project: "clasm", Trigger: "live-test"}.write(t, dir)
	testRecord{ID: "0003", Project: "clasm"}.write(t, dir)

	s := runIngest(t, kb, dir)
	if s.Added != 3 || s.Updated != 0 || s.Skipped != 0 || s.Failed != 0 {
		t.Errorf("summary = %+v, want 3 added and nothing else", s)
	}

	p, err := kb.ProjectByName("clasm")
	if err != nil || p == nil {
		t.Fatalf("ingest did not create the clasm project: %v", err)
	}
	recs, err := kb.RecordsByProject(p.ID)
	if err != nil {
		t.Fatalf("RecordsByProject: %v", err)
	}
	if len(recs) != 3 {
		t.Errorf("got %d records in the database, want 3", len(recs))
	}
}

func TestCmdIngest_StoresPathsRelativeToRoot(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	testRecord{ID: "0001", Project: "clasm"}.write(t, dir)

	runIngest(t, kb, dir)

	p, _ := kb.ProjectByName("clasm")
	recs, _ := kb.RecordsByProject(p.ID)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	want := filepath.Join("clasm", "decisions", "0001-fixture.md")
	if recs[0].Path != want {
		t.Errorf("Path = %q, want %q — absolute paths do not survive merge between machines", recs[0].Path, want)
	}
}

func TestCmdIngest_SecondRunSkipsUnchanged(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	for _, id := range []string{"0001", "0002", "0003"} {
		testRecord{ID: id, Project: "clasm"}.write(t, dir)
	}

	if s := runIngest(t, kb, dir); s.Added != 3 {
		t.Fatalf("first run added %d, want 3", s.Added)
	}
	s := runIngest(t, kb, dir)
	if s.Skipped != 3 || s.Updated != 0 || s.Added != 0 {
		t.Errorf("second run = %+v, want 3 skipped, 0 updated, 0 added", s)
	}
}

func TestCmdIngest_ChangedFileIsUpdated(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	for _, id := range []string{"0001", "0002", "0003"} {
		testRecord{ID: id, Project: "clasm"}.write(t, dir)
	}
	runIngest(t, kb, dir)

	testRecord{ID: "0002", Project: "clasm", Body: "\n**Context.** Rewritten body.\n"}.write(t, dir)

	s := runIngest(t, kb, dir)
	if s.Updated != 1 || s.Skipped != 2 || s.Added != 0 {
		t.Errorf("summary = %+v, want exactly 1 updated and 2 skipped", s)
	}
}

// Two passes: a record may reference one that has not been read yet, so
// resolution cannot happen during the walk.
func TestCmdIngest_ForwardReferenceResolves(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	// 0002 supersedes 0003, which the walk reaches afterwards.
	testRecord{ID: "0002", Project: "clasm", Supersedes: []string{"0003"}}.write(t, dir)
	testRecord{ID: "0003", Project: "clasm", SupersededBy: []string{"0002"}}.write(t, dir)

	s := runIngest(t, kb, dir)
	if s.Supersedes != 1 {
		t.Fatalf("summary = %+v, want 1 supersedes relation from a forward reference", s)
	}

	p, _ := kb.ProjectByName("clasm")
	newer, err := kb.RecordByIdentity(kb.Workspace(), p.ID, "project", "0002")
	if err != nil {
		t.Fatalf("RecordByIdentity 0002: %v", err)
	}
	older, err := kb.RecordByIdentity(kb.Workspace(), p.ID, "project", "0003")
	if err != nil {
		t.Fatalf("RecordByIdentity 0003: %v", err)
	}
	rels, err := kb.RelationsFor(older.ID)
	if err != nil {
		t.Fatalf("RelationsFor: %v", err)
	}
	if len(rels) != 1 || rels[0].Relationship != "superseded_by" || rels[0].RecordID != newer.ID {
		t.Errorf("relations for 0003 = %+v, want one superseded_by pointing at 0002", rels)
	}
}

// superseded_by in a file is the inverse of a stored supersedes, never
// inserted in its own right; otherwise the pair would be stored twice.
func TestCmdIngest_SupersededByIsNotStoredDirectly(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	testRecord{ID: "0002", Project: "clasm", Supersedes: []string{"0003"}}.write(t, dir)
	testRecord{ID: "0003", Project: "clasm", SupersededBy: []string{"0002"}}.write(t, dir)

	runIngest(t, kb, dir)

	p, _ := kb.ProjectByName("clasm")
	newer, _ := kb.RecordByIdentity(kb.Workspace(), p.ID, "project", "0002")
	rels, _ := kb.RelationsFor(newer.ID)
	if len(rels) != 1 {
		t.Errorf("relations for 0002 = %+v, want exactly one (the stored supersedes)", rels)
	}
}

// Plan finding 1: a partial supersession leaves the older record accepted.
func TestCmdIngest_DoesNotDeriveStatusFromSupersession(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	testRecord{ID: "0159", Project: "clasm", Supersedes: []string{"0160"}}.write(t, dir)
	testRecord{ID: "0160", Project: "clasm", Status: "accepted", SupersededBy: []string{"0159"}}.write(t, dir)

	runIngest(t, kb, dir)

	p, _ := kb.ProjectByName("clasm")
	older, _ := kb.RecordByIdentity(kb.Workspace(), p.ID, "project", "0160")
	if older.Status != "accepted" {
		t.Errorf("status = %q, want accepted — ingest stores what the file says", older.Status)
	}
}

// A relates_to entry is [<scope>:]<id>. The workspace tier is scope
// "workspace" with a null project; a bare id inherits the citing record's own
// scope and project.
func TestCmdIngest_CrossTierRelatesToResolves(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	clasmDir := filepath.Join(root, "clasm", "decisions")
	wsDir := filepath.Join(root, "agents", "decisions")
	testRecord{ID: "0160", Project: "clasm"}.write(t, clasmDir)
	testRecord{ID: "0001", Project: "", RelatesTo: []string{"clasm:0160"}}.write(t, wsDir)

	runIngest(t, kb, clasmDir)
	s := runIngest(t, kb, wsDir)
	if s.RelatesTo != 1 {
		t.Fatalf("summary = %+v, want 1 relates_to resolved across tiers", s)
	}

	ws, err := kb.RecordByIdentity(kb.Workspace(), 0, "workspace", "0001")
	if err != nil {
		t.Fatalf("RecordByIdentity workspace 0001: %v", err)
	}
	if ws.Scope != "workspace" {
		t.Errorf("Scope = %q, want workspace for a record with an empty project", ws.Scope)
	}
	rels, _ := kb.RelationsFor(ws.ID)
	if len(rels) != 1 || rels[0].Relationship != "relates_to" {
		t.Errorf("relations = %+v, want one relates_to", rels)
	}
}

func TestCmdIngest_WorkspaceQualifiedRelatesToResolves(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	wsDir := filepath.Join(root, "agents", "decisions")
	clasmDir := filepath.Join(root, "clasm", "decisions")
	testRecord{ID: "0001", Project: ""}.write(t, wsDir)
	testRecord{ID: "0160", Project: "clasm", RelatesTo: []string{"workspace:0001"}}.write(t, clasmDir)

	runIngest(t, kb, wsDir)
	if s := runIngest(t, kb, clasmDir); s.RelatesTo != 1 {
		t.Errorf("summary = %+v, want 1 relates_to resolved to the workspace tier", s)
	}
}

// A stray DR- prefix in a reference should not fail a run.
func TestCmdIngest_StripsDRPrefixFromReferences(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	testRecord{ID: "0001", Project: "clasm"}.write(t, dir)
	testRecord{ID: "0002", Project: "clasm", RelatesTo: []string{"DR-0001"}}.write(t, dir)

	if s := runIngest(t, kb, dir); s.RelatesTo != 1 {
		t.Errorf("summary = %+v, want the DR- prefixed reference to resolve", s)
	}
}

// An unresolvable reference is reported and skipped, never fatal: failing
// would make ingest order significant, which two passes exist to avoid.
func TestCmdIngest_UnresolvedReferenceIsReportedNotFatal(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	wsDir := filepath.Join(root, "agents", "decisions")
	clasmDir := filepath.Join(root, "clasm", "decisions")
	testRecord{ID: "0160", Project: "clasm"}.write(t, clasmDir)
	testRecord{ID: "0001", Project: "", RelatesTo: []string{"clasm:0160"}}.write(t, wsDir)

	// Workspace tier first: clasm has not been ingested, so the target is absent.
	s := runIngest(t, kb, wsDir)
	if s.Added != 1 {
		t.Errorf("summary = %+v, want the record itself to be ingested", s)
	}
	if s.RelatesTo != 0 {
		t.Errorf("summary = %+v, want no relation written yet", s)
	}
	if len(s.Unresolved) != 1 || !strings.Contains(s.Unresolved[0], "clasm:0160") {
		t.Errorf("Unresolved = %v, want one entry naming clasm:0160", s.Unresolved)
	}

	// Re-running after clasm arrives resolves it, which is the documented remedy.
	runIngest(t, kb, clasmDir)
	s = runIngest(t, kb, wsDir)
	if s.RelatesTo != 1 {
		t.Errorf("summary = %+v, want the relation to resolve on the later run", s)
	}
	if len(s.Unresolved) != 0 {
		t.Errorf("Unresolved = %v, want none once the target exists", s.Unresolved)
	}
}

// Supersession is same-tier only, so a qualified entry cannot be honoured:
// writing both sides would mean writing into another repository.
func TestCmdIngest_QualifiedSupersedesIsMalformed(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	clasmDir := filepath.Join(root, "clasm", "decisions")
	wsDir := filepath.Join(root, "agents", "decisions")
	testRecord{ID: "0160", Project: "clasm"}.write(t, clasmDir)
	testRecord{ID: "0001", Project: "", Supersedes: []string{"clasm:0160"}}.write(t, wsDir)

	runIngest(t, kb, clasmDir)
	s := runIngest(t, kb, wsDir)

	if s.Supersedes != 0 {
		t.Errorf("summary = %+v, want no cross-tier supersession written", s)
	}
	if len(s.Malformed) != 1 || !strings.Contains(s.Malformed[0], "clasm:0160") {
		t.Errorf("Malformed = %v, want one entry naming the qualified supersedes", s.Malformed)
	}
}

func TestCmdIngest_DryRunWritesNothing(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	testRecord{ID: "0001", Project: "clasm"}.write(t, dir)
	testRecord{ID: "0002", Project: "clasm", Supersedes: []string{"0001"}}.write(t, dir)

	s := runIngest(t, kb, dir, "--dry-run")
	if s.Added != 2 {
		t.Errorf("summary = %+v, want --dry-run to report the same counts", s)
	}
	if !s.DryRun {
		t.Error("summary does not report DryRun")
	}
	// ProjectByName reports "not found" as (nil, nil), not as an error.
	if p, _ := kb.ProjectByName("clasm"); p != nil {
		t.Error("--dry-run created the clasm project")
	}

	// And a real run afterwards still reports them as additions.
	if s := runIngest(t, kb, dir); s.Added != 2 {
		t.Errorf("summary after a dry run = %+v, want 2 added", s)
	}
}

// Ingest never writes to record files; only kb record does.
func TestCmdIngest_NeverWritesToRecordFiles(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	paths := []string{
		testRecord{ID: "0001", Project: "clasm"}.write(t, dir),
		testRecord{ID: "0002", Project: "clasm", Supersedes: []string{"0001"}}.write(t, dir),
	}
	before := make(map[string][]byte, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", p, err)
		}
		before[p] = b
	}

	runIngest(t, kb, dir)

	for _, p := range paths {
		after, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", p, err)
		}
		if !bytes.Equal(before[p], after) {
			t.Errorf("ingest modified %s", filepath.Base(p))
		}
	}
}

func TestCmdIngest_ParseFailureIsReportedNotFatal(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	testRecord{ID: "0001", Project: "clasm"}.write(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "0002-broken.md"), []byte("no frontmatter here\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s := runIngest(t, kb, dir)
	if s.Added != 1 {
		t.Errorf("summary = %+v, want the good record still ingested", s)
	}
	if s.Failed != 1 || len(s.Errors) != 1 {
		t.Errorf("summary = %+v, want 1 failure reported", s)
	}
}

// Additive only: a record whose file has vanished stays in the database and is
// reported, never deleted.
func TestCmdIngest_VanishedFileIsReportedNotDeleted(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	testRecord{ID: "0001", Project: "clasm"}.write(t, dir)
	path := testRecord{ID: "0002", Project: "clasm"}.write(t, dir)
	runIngest(t, kb, dir)

	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	s := runIngest(t, kb, dir)

	if len(s.Missing) != 1 || !strings.Contains(s.Missing[0], "0002") {
		t.Errorf("Missing = %v, want one entry naming the removed record", s.Missing)
	}
	p, _ := kb.ProjectByName("clasm")
	recs, _ := kb.RecordsByProject(p.ID)
	if len(recs) != 2 {
		t.Errorf("got %d records, want 2 — a vanished file must not delete its row", len(recs))
	}
}

// Design decision 3: the initiative field is materialised as a concept link.
func TestCmdIngest_InitiativeBecomesAConcept(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	testRecord{ID: "0001", Project: "clasm", Initiative: "eprints-to-rdm"}.write(t, dir)

	runIngest(t, kb, dir)

	p, err := kb.ProjectByName("clasm")
	if err != nil || p == nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	concepts, err := kb.ProjectConcepts(p.ID)
	if err != nil {
		t.Fatalf("ProjectConcepts: %v", err)
	}
	for _, c := range concepts {
		if c.Name == "eprints-to-rdm" {
			return
		}
	}
	t.Errorf("concepts = %+v, want one named eprints-to-rdm", concepts)
}

func TestCmdIngest_RecordsAreSearchable(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	testRecord{
		ID: "0001", Project: "clasm",
		Title: "Associate the IAM instance profile",
		Body:  "\n**Context.** A distinctive phrase: xyzzykeyword.\n",
	}.write(t, dir)

	runIngest(t, kb, dir)

	results, err := kb.Search("xyzzykeyword")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("record body is not searchable after ingest")
	}
}

func TestCmdIngest_RequiresAPath(t *testing.T) {
	kb, _ := openWorkspaceKB(t)
	var out bytes.Buffer
	if err := cmdIngest(kb, nil, false, nil, &out); err == nil {
		t.Error("expected an error when PATH is missing")
	}
}

func TestCmdIngest_RootOverride(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	testRecord{ID: "0001", Project: "clasm"}.write(t, dir)

	runIngest(t, kb, dir, "--root", filepath.Join(root, "clasm"))

	p, _ := kb.ProjectByName("clasm")
	recs, _ := kb.RecordsByProject(p.ID)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	want := filepath.Join("decisions", "0001-fixture.md")
	if recs[0].Path != want {
		t.Errorf("Path = %q, want %q with --root", recs[0].Path, want)
	}
}

func TestCmdIngest_HumanReadableOutput(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "clasm", "decisions")
	testRecord{ID: "0001", Project: "clasm"}.write(t, dir)

	var out bytes.Buffer
	if err := cmdIngest(kb, nil, false, []string{dir}, &out); err != nil {
		t.Fatalf("cmdIngest: %v", err)
	}
	if !strings.Contains(out.String(), "added") {
		t.Errorf("output = %q, want it to report counts", out.String())
	}
}

// ─── Live corpus ─────────────────────────────────────────────────────────────

// liveCorpus returns the path to a real decisions directory, skipping when
// ~/WorkLab is not on this machine.
func liveCorpus(t *testing.T, parts ...string) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory: %v", err)
	}
	dir := filepath.Join(append([]string{home, "WorkLab"}, parts...)...)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Skipf("no corpus at %s", dir)
	}
	return dir
}

func TestCmdIngest_ClasmCorpus(t *testing.T) {
	dir := liveCorpus(t, "clasm", "decisions")
	kb, _ := openWorkspaceKB(t)

	s := runIngest(t, kb, dir)
	if s.Added != 169 {
		t.Errorf("Added = %d, want 169", s.Added)
	}
	if s.Failed != 0 {
		t.Errorf("Failed = %d (%v), want 0", s.Failed, s.Errors)
	}
	if s.Supersedes != 2 {
		t.Errorf("Supersedes = %d, want 2", s.Supersedes)
	}
	if s.RelatesTo != 3 {
		t.Errorf("RelatesTo = %d, want 3", s.RelatesTo)
	}

	again := runIngest(t, kb, dir)
	if again.Skipped != 169 || again.Updated != 0 {
		t.Errorf("second run = %+v, want 169 skipped and 0 updated", again)
	}
}

// The first real cross-tier reference: agents/decisions DR-0001 cites
// clasm:0160.
func TestCmdIngest_LiveCrossTierReference(t *testing.T) {
	clasmDir := liveCorpus(t, "clasm", "decisions")
	wsDir := liveCorpus(t, "agents", "decisions")
	kb, _ := openWorkspaceKB(t)

	runIngest(t, kb, clasmDir)
	s := runIngest(t, kb, wsDir)

	if s.Failed != 0 {
		t.Errorf("Failed = %d (%v), want 0", s.Failed, s.Errors)
	}
	if s.RelatesTo < 1 {
		t.Errorf("RelatesTo = %d, want at least the clasm:0160 reference", s.RelatesTo)
	}

	ws, err := kb.RecordByIdentity(kb.Workspace(), 0, "workspace", "0001")
	if err != nil {
		t.Fatalf("RecordByIdentity workspace 0001: %v", err)
	}
	rels, err := kb.RelationsFor(ws.ID)
	if err != nil {
		t.Fatalf("RelationsFor: %v", err)
	}
	clasm, err := kb.ProjectByName("clasm")
	if err != nil || clasm == nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	target, err := kb.RecordByIdentity(kb.Workspace(), clasm.ID, "project", "0160")
	if err != nil {
		t.Fatalf("RecordByIdentity clasm 0160: %v", err)
	}
	for _, rel := range rels {
		if rel.RecordID == target.ID && rel.Relationship == "relates_to" {
			return
		}
	}
	t.Errorf("relations for workspace DR-0001 = %+v, want one relates_to pointing at clasm DR-0160", rels)
}

// Ingesting the workspace tier before clasm leaves the cross-tier reference
// unwritten, reports it, and succeeds; a later run resolves it.
func TestCmdIngest_LiveCrossTierOrderIndependence(t *testing.T) {
	clasmDir := liveCorpus(t, "clasm", "decisions")
	wsDir := liveCorpus(t, "agents", "decisions")
	kb, _ := openWorkspaceKB(t)

	first := runIngest(t, kb, wsDir)
	if first.Failed != 0 {
		t.Errorf("Failed = %d (%v), want 0 — an absent target must not fail the run", first.Failed, first.Errors)
	}
	if len(first.Unresolved) == 0 {
		t.Error("Unresolved is empty, want the clasm:0160 reference reported")
	}

	runIngest(t, kb, clasmDir)
	second := runIngest(t, kb, wsDir)
	if len(second.Unresolved) != 0 {
		t.Errorf("Unresolved = %v, want none once clasm has been ingested", second.Unresolved)
	}
}
