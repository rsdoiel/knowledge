package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// R2 of removal-verbs-plan.md (DR-0050): the removal commands. Each follows
// `concept delete`: a refusal says what and how many and changes nothing (exit 1),
// --force where it makes sense, --dry-run changes nothing, and a target or link that
// is not there is exit 1, never a silent success.

func runVerb(t *testing.T, verb string, kb *knowledge.KnowledgeBase, jsonOut bool, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	fn, ok := verbs[verb]
	if !ok {
		t.Fatalf("no verb %q", verb)
	}
	err := fn(kb, nil, jsonOut, args, &out)
	return out.String(), err
}

func wantNegative(t *testing.T, err error, substrs ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("succeeded, want a refusal or not-found (exit 1)")
	}
	if got := exitCodeFor(err); got != classNegative {
		t.Errorf("class = %v, want negative (1); error: %v", got, err)
	}
	for _, s := range substrs {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("error %q should contain %q", err, s)
		}
	}
}

func wantUsage(t *testing.T, err error) {
	t.Helper()
	if err == nil || !isUsageError(err) {
		t.Errorf("error %v is not a usage error (exit 2)", err)
	}
}

// ─── project delete ──────────────────────────────────────────────────────────

func TestProjectDelete_EmptyProject(t *testing.T) {
	kb := openTestKB(t)
	kb.AddProject("stray", "")
	kb.AddProject("keep", "")
	out, err := runVerb(t, "project", kb, false, "delete", "stray")
	if err != nil {
		t.Fatalf("project delete: %v", err)
	}
	if !strings.Contains(out, "deleted") || !strings.Contains(out, "stray") {
		t.Errorf("output %q should confirm the deletion", out)
	}
	if p, _ := kb.ProjectByName("stray"); p != nil {
		t.Error("the project is still there")
	}
	if p, _ := kb.ProjectByName("keep"); p == nil {
		t.Error("another project went")
	}
}

func TestProjectDelete_DryRunChangesNothing(t *testing.T) {
	kb := openTestKB(t)
	kb.AddProject("stray", "")
	out, err := runVerb(t, "project", kb, false, "delete", "stray", "--dry-run")
	if err != nil || !strings.Contains(out, "would delete") {
		t.Fatalf("out %q err %v, want a 'would delete' preview", out, err)
	}
	if p, _ := kb.ProjectByName("stray"); p == nil {
		t.Error("--dry-run deleted the project")
	}
}

func TestProjectDelete_RefusesContentEvenWithForce(t *testing.T) {
	for _, extra := range [][]string{nil, {"--force"}} {
		kb := openTestKB(t)
		pid, _ := kb.AddProject("busy", "")
		kb.AddObservation(pid, "note", "b")
		_, err := runVerb(t, "project", kb, false, append([]string{"delete", "busy"}, extra...)...)
		wantNegative(t, err, "observation", "busy")
		if p, _ := kb.ProjectByName("busy"); p == nil {
			t.Errorf("args %v deleted a project that owns content", extra)
		}
	}
}

func TestProjectDelete_ConceptLinksNeedForce(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("linked", "")
	cid, _ := kb.AddConcept("C", "")
	kb.LinkProjectConcept(pid, cid)
	_, err := runVerb(t, "project", kb, false, "delete", "linked")
	wantNegative(t, err, "concept link", "--force")
	if p, _ := kb.ProjectByName("linked"); p == nil {
		t.Fatal("deleted without --force")
	}
	if _, err := runVerb(t, "project", kb, false, "delete", "linked", "--force"); err != nil {
		t.Fatalf("with --force: %v", err)
	}
}

func TestProjectDelete_NotFoundUsageAndDash(t *testing.T) {
	kb := openTestKB(t)
	_, err := runVerb(t, "project", kb, false, "delete", "nosuch")
	wantNegative(t, err, "nosuch")
	_, err = runVerb(t, "project", kb, false, "delete")
	wantUsage(t, err)
	_, err = runVerb(t, "project", kb, false, "delete", "a", "b")
	wantUsage(t, err)
	kb.AddProject("-odd", "")
	if _, err := runVerb(t, "project", kb, false, "delete", "--", "-odd"); err != nil {
		t.Errorf("a dash-leading name after --: %v", err)
	}
}

func TestProjectDelete_JSON(t *testing.T) {
	kb := openTestKB(t)
	kb.AddProject("stray", "")
	out, err := runVerb(t, "project", kb, true, "delete", "stray")
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Project string `json:"project"`
		Deleted bool   `json:"deleted"`
		DryRun  bool   `json:"dry_run"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.Project != "stray" || !res.Deleted || res.DryRun {
		t.Errorf("result %q (%v) = %+v", out, err, res)
	}
}

// ─── observation delete ──────────────────────────────────────────────────────

func TestObservationDelete(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("p", "")
	plain, _ := kb.AddObservation(pid, "note", "plain")
	linked, _ := kb.AddObservation(pid, "note", "linked")
	cid, _ := kb.AddConcept("C", "")
	kb.LinkObservationConcept(linked, cid)

	if out, err := runVerb(t, "observation", kb, false, "delete", idStr(plain), "--dry-run"); err != nil || !strings.Contains(out, "would delete") {
		t.Fatalf("dry run: %q %v", out, err)
	}
	if obs, _ := kb.Observations(pid); len(obs) != 2 {
		t.Fatal("--dry-run deleted something")
	}
	if _, err := runVerb(t, "observation", kb, false, "delete", idStr(plain)); err != nil {
		t.Fatalf("delete an unlinked observation: %v", err)
	}
	_, err := runVerb(t, "observation", kb, false, "delete", idStr(linked))
	wantNegative(t, err, "concept link", "--force")
	if obs, _ := kb.Observations(pid); len(obs) != 1 {
		t.Fatalf("observations = %d, want the linked one kept after the refusal", len(obs))
	}
	if _, err := runVerb(t, "observation", kb, false, "delete", idStr(linked), "--force"); err != nil {
		t.Fatalf("--force: %v", err)
	}
	if obs, _ := kb.Observations(pid); len(obs) != 0 {
		t.Error("the forced delete left the observation")
	}
	_, err = runVerb(t, "observation", kb, false, "delete", "99999")
	wantNegative(t, err, "99999")
	_, err = runVerb(t, "observation", kb, false, "delete", "abc")
	wantUsage(t, err)
	_, err = runVerb(t, "observation", kb, false, "delete")
	wantUsage(t, err)
}

func idStr(n int64) string { return strconv.FormatInt(n, 10) }

// ─── document delete ─────────────────────────────────────────────────────────

func TestDocumentDelete(t *testing.T) {
	kb := openTestKB(t)
	kb.AddProject("docs", "")
	path := filepath.Join(t.TempDir(), "a.md")
	os.WriteFile(path, []byte("## S\n\nbody\n"), 0o644)
	if _, err := runVerb(t, "document", kb, false, "ingest", path, "--project", "docs"); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if out, err := runVerb(t, "document", kb, false, "delete", "1", "--dry-run"); err != nil || !strings.Contains(out, "would delete") {
		t.Fatalf("dry run: %q %v", out, err)
	}
	d, _ := kb.DocumentByPath(path)
	if d == nil {
		t.Fatal("--dry-run deleted the document")
	}
	out, err := runVerb(t, "document", kb, false, "delete", idStr(d.ID))
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !strings.Contains(out, "deleted") || !strings.Contains(out, "re-ingest") {
		t.Errorf("output %q should confirm and say a re-ingest recreates it", out)
	}
	if d, _ := kb.DocumentByPath(path); d != nil {
		t.Error("the document is still there")
	}
	_, err = runVerb(t, "document", kb, false, "delete", "99999")
	wantNegative(t, err, "99999")
	_, err = runVerb(t, "document", kb, false, "delete", "abc")
	wantUsage(t, err)
}

func TestDocumentDelete_ReviewedSummaryNeedsForce(t *testing.T) {
	kb := openTestKB(t)
	kb.AddProject("docs", "")
	path := filepath.Join(t.TempDir(), "a.md")
	os.WriteFile(path, []byte("## S\n\nbody\n"), 0o644)
	runVerb(t, "document", kb, false, "ingest", path, "--project", "docs")
	d, _ := kb.DocumentByPath(path)
	secs, _ := kb.DocumentSections(d.ID)
	var secID int64
	for _, s := range secs {
		if s.Level == "section" {
			secID = s.ID
		}
	}
	if err := kb.DraftDocumentSummary(secID, "summary", "test", nil); err != nil {
		t.Fatal(err)
	}
	if err := kb.PromoteDocumentSummary(secID); err != nil {
		t.Fatal(err)
	}
	_, err := runVerb(t, "document", kb, false, "delete", idStr(d.ID))
	wantNegative(t, err, "reviewed", "--force")
	if got, _ := kb.DocumentByPath(path); got == nil {
		t.Fatal("deleted a document with a reviewed summary without --force")
	}
	if _, err := runVerb(t, "document", kb, false, "delete", idStr(d.ID), "--force"); err != nil {
		t.Fatalf("--force: %v", err)
	}
}

// ─── record delete ───────────────────────────────────────────────────────────

func TestRecordDelete_OnlyWhenTheFileIsGone(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "accepted", Date: "2026-08-01"},
		testRecord{ID: "0002", Kind: "decision", Trigger: "design", Status: "proposed", Date: "2026-08-02"},
	)
	// The file still exists: refused, pointing at the alternatives, nothing changed.
	_, err := runVerb(t, "record", kb, false, "delete", "0001", "--project", "clasm")
	wantNegative(t, err, "file", "set-status")
	if _, err := runVerb(t, "record", kb, false, "show", "0001", "--project", "clasm"); err != nil {
		t.Fatalf("the refused delete removed the row: %v", err)
	}

	// The file is gone: --dry-run previews, then the row goes.
	if err := os.Remove(filepath.Join(root, "clasm", "decisions", "0001-fixture.md")); err != nil {
		t.Fatal(err)
	}
	out, err := runVerb(t, "record", kb, false, "delete", "0001", "--project", "clasm", "--dry-run")
	if err != nil || !strings.Contains(out, "would delete") {
		t.Fatalf("dry run: %q %v", out, err)
	}
	if _, err := runVerb(t, "record", kb, false, "show", "0001", "--project", "clasm"); err != nil {
		t.Fatalf("--dry-run removed the row: %v", err)
	}
	out, err = runVerb(t, "record", kb, false, "delete", "0001", "--project", "clasm")
	if err != nil || !strings.Contains(out, "deleted") {
		t.Fatalf("delete: %q %v", out, err)
	}
	_, err = runVerb(t, "record", kb, false, "show", "0001", "--project", "clasm")
	wantNegative(t, err, "0001")
	if _, err := runVerb(t, "record", kb, false, "show", "0002", "--project", "clasm"); err != nil {
		t.Errorf("another record was affected: %v", err)
	}
	// A second delete: not found, not success.
	_, err = runVerb(t, "record", kb, false, "delete", "0001", "--project", "clasm")
	wantNegative(t, err)
	_, err = runVerb(t, "record", kb, false, "delete", "--project", "clasm")
	wantUsage(t, err)
}

// ─── unlink ──────────────────────────────────────────────────────────────────

func TestUnlink(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("p", "")
	oid, _ := kb.AddObservation(pid, "note", "o")
	cid, _ := kb.AddConcept("C", "")
	sid, _ := kb.AddSource(knowledge.Source{Title: "S"})
	kb.LinkProjectConcept(pid, cid)
	kb.LinkObservationConcept(oid, cid)
	kb.LinkObservationSource(oid, sid, "cited")

	for _, args := range [][]string{
		{"project", "p", "C"}, {"observation", idStr(oid), "C"}, {"source", idStr(oid), idStr(sid)},
	} {
		out, err := runVerb(t, "unlink", kb, false, args...)
		if err != nil || !strings.Contains(out, "unlinked") {
			t.Errorf("unlink %v: %q %v", args, out, err)
		}
		// The second time there is no link: exit 1, not a silent success.
		_, err = runVerb(t, "unlink", kb, false, args...)
		wantNegative(t, err)
	}
	if cs, _ := kb.ProjectConcepts(pid); len(cs) != 0 {
		t.Error("the project link is still there")
	}
	if u, _ := kb.ObservationUsage(oid); u.Concepts != 0 || u.Sources != 0 {
		t.Errorf("observation links are still there: %+v", u)
	}
	if cs, _ := kb.Concepts(); len(cs) != 1 {
		t.Error("unlinking must not delete the concept")
	}
}

func TestUnlink_MissingTargetsAndUsage(t *testing.T) {
	kb := openTestKB(t)
	for _, args := range [][]string{
		{"project", "nosuch", "C"}, {"observation", "99999", "C"}, {"source", "99999", "1"},
	} {
		_, err := runVerb(t, "unlink", kb, false, args...)
		wantNegative(t, err)
	}
	for _, args := range [][]string{
		{}, {"bogus"}, {"project", "p"}, {"project", "p", "C", "extra"}, {"observation", "abc", "C"}, {"source", "1"},
	} {
		_, err := runVerb(t, "unlink", kb, false, args...)
		wantUsage(t, err)
	}
}
