package main

import (
	"fmt"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// `kb ingest` reports a status that enters the database through a file
// (knowledge DR-0068, v0.0.19 T2). Ingest is how a status edited by hand in a
// record file reaches the database, bypassing the transition table and the
// terminal rule; that is by design (files are the source of truth) but nothing
// said so. The report is about visibility and never blocks: the status is
// applied, the exit code is unchanged.

func ingestJSON(t *testing.T, kb *knowledge.KnowledgeBase, dir string, extra ...string) ingestSummary {
	t.Helper()
	return runIngest(t, kb, append([]string{dir}, extra...)...)
}

// storedStatus reads a record's status from the database, through record show.
func storedStatus(t *testing.T, kb *knowledge.KnowledgeBase, project, id string) string {
	t.Helper()
	var detail struct {
		Status string `json:"status"`
	}
	runRecordJSON(t, kb, &detail, "show", project+"/DR-"+id)
	return detail.Status
}

func changeFor(s ingestSummary, ref string) (statusChange, bool) {
	for _, c := range s.StatusChanges {
		if c.Ref == ref {
			return c, true
		}
	}
	return statusChange{}, false
}

func TestIngestStatus_AnUnchangedReIngestReportsNothing(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := root + "/clasm/decisions"
	testRecord{ID: "0001", Project: "clasm", Status: "proposed"}.write(t, dir)
	ingestJSON(t, kb, dir)
	if s := ingestJSON(t, kb, dir); len(s.StatusChanges) != 0 {
		t.Errorf("a re-ingest of unchanged files reported %v", s.StatusChanges)
	}
}

func TestIngestStatus_AStatusEditedInAFileIsReported(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := root + "/clasm/decisions"
	testRecord{ID: "0001", Project: "clasm", Status: "proposed"}.write(t, dir)
	ingestJSON(t, kb, dir)
	testRecord{ID: "0001", Project: "clasm", Status: "accepted"}.write(t, dir)
	s := ingestJSON(t, kb, dir)
	c, ok := changeFor(s, "clasm/DR-0001")
	if !ok {
		t.Fatalf("no status change reported for clasm/DR-0001: %v", s.StatusChanges)
	}
	if c.From != "proposed" || c.To != "accepted" || c.Kind != "edited" {
		t.Errorf("change = %+v, want proposed -> accepted, edited", c)
	}
	if s.Updated != 1 {
		t.Errorf("updated = %d, want 1: the status is applied, not blocked", s.Updated)
	}
	if st := storedStatus(t, kb, "clasm", "0001"); st != "accepted" {
		t.Errorf("stored status = %q, want accepted", st)
	}
}

func TestIngestStatus_ANewRecordThatIsAlreadyAcceptedIsReported(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := root + "/clasm/decisions"
	testRecord{ID: "0001", Project: "clasm", Status: "accepted"}.write(t, dir)
	s := ingestJSON(t, kb, dir)
	c, ok := changeFor(s, "clasm/DR-0001")
	if !ok || c.Kind != "arrived" || c.To != "accepted" || c.From != "" {
		t.Errorf("change = %+v (found %v), want an arrival at accepted", c, ok)
	}
}

func TestIngestStatus_ANewProposedRecordReportsNothing(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := root + "/clasm/decisions"
	testRecord{ID: "0001", Project: "clasm", Status: "proposed"}.write(t, dir)
	testRecord{ID: "0002", Project: "clasm", Status: "rejected"}.write(t, dir)
	if s := ingestJSON(t, kb, dir); len(s.StatusChanges) != 0 {
		t.Errorf("new proposed and rejected records reported %v", s.StatusChanges)
	}
}

// Only a status change is reported: editing the body is an ordinary update.
func TestIngestStatus_ABodyEditAloneReportsNothing(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := root + "/clasm/decisions"
	testRecord{ID: "0001", Project: "clasm", Status: "proposed"}.write(t, dir)
	ingestJSON(t, kb, dir)
	testRecord{ID: "0001", Project: "clasm", Status: "proposed", Body: "\n**Context.** Changed.\n"}.write(t, dir)
	s := ingestJSON(t, kb, dir)
	if s.Updated != 1 || len(s.StatusChanges) != 0 {
		t.Errorf("updated %d, changes %v; want one update and no status change", s.Updated, s.StatusChanges)
	}
}

// A move that set-status would refuse is reported and applied, like any other:
// ingest never rejects content for being unwise, and a hand edit may be repairing
// data.
func TestIngestStatus_AForbiddenMoveEditedInAFileIsReportedAndApplied(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := root + "/clasm/decisions"
	testRecord{ID: "0001", Project: "clasm", Status: "rejected"}.write(t, dir)
	ingestJSON(t, kb, dir)
	testRecord{ID: "0001", Project: "clasm", Status: "accepted"}.write(t, dir)
	s := ingestJSON(t, kb, dir)
	if c, ok := changeFor(s, "clasm/DR-0001"); !ok || c.From != "rejected" || c.To != "accepted" {
		t.Errorf("change = %+v (found %v), want rejected -> accepted", c, ok)
	}
	if st := storedStatus(t, kb, "clasm", "0001"); st != "accepted" {
		t.Errorf("stored status = %q, want accepted: the file is the source of truth", st)
	}
	if s.Failed != 0 {
		t.Errorf("failed = %d, a report is not a failure", s.Failed)
	}
}

// --dry-run reports the same lines and writes nothing.
func TestIngestStatus_DryRunReportsAndWritesNothing(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := root + "/clasm/decisions"
	testRecord{ID: "0001", Project: "clasm", Status: "proposed"}.write(t, dir)
	ingestJSON(t, kb, dir)
	testRecord{ID: "0001", Project: "clasm", Status: "accepted"}.write(t, dir)
	testRecord{ID: "0002", Project: "clasm", Status: "accepted"}.write(t, dir)
	s := ingestJSON(t, kb, dir, "--dry-run")
	if _, ok := changeFor(s, "clasm/DR-0001"); !ok {
		t.Errorf("the dry run did not report the edit: %v", s.StatusChanges)
	}
	if _, ok := changeFor(s, "clasm/DR-0002"); !ok {
		t.Errorf("the dry run did not report the arrival: %v", s.StatusChanges)
	}
	if st := storedStatus(t, kb, "clasm", "0001"); st != "proposed" {
		t.Errorf("the dry run changed the stored status to %q", st)
	}
	// and the real run still sees the same changes, because nothing was written
	if real := ingestJSON(t, kb, dir); len(real.StatusChanges) != 2 {
		t.Errorf("the real run reported %v, want the same two changes", real.StatusChanges)
	}
}

// A status written by `kb record set-status` is in the file and the database
// together, so the next ingest is not told it came from a file.
func TestIngestStatus_SetStatusThenIngestReportsNothing(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm", testRecord{ID: "0001", Kind: "decision", Trigger: "design", Status: "proposed"})
	runRecord(t, kb, "set-status", "clasm/DR-0001", "rejected")
	if s := ingestJSON(t, kb, root+"/clasm/decisions"); len(s.StatusChanges) != 0 {
		t.Errorf("an ingest after set-status reported %v", s.StatusChanges)
	}
}

func TestIngestStatus_TextLinesAndTheCap(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := root + "/clasm/decisions"
	for i := 1; i <= 25; i++ {
		testRecord{ID: fmt.Sprintf("%04d", i), Project: "clasm", Status: "accepted"}.write(t, dir)
	}
	var sb strings.Builder
	if err := cmdIngest(kb, nil, false, []string{dir}, &sb); err != nil {
		t.Fatalf("cmdIngest: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "status: clasm/DR-0001 arrived accepted") {
		t.Errorf("the text lacks the arrival line:\n%s", out)
	}
	if n := strings.Count(out, "status: clasm/DR-"); n != 20 {
		t.Errorf("%d status lines in the text, want the first 20", n)
	}
	if !strings.Contains(out, "5 more") {
		t.Errorf("the text should say how many it left out:\n%s", out)
	}
	// the JSON keeps every one
	kb2, root2 := openWorkspaceKB(t)
	dir2 := root2 + "/clasm/decisions"
	for i := 1; i <= 25; i++ {
		testRecord{ID: fmt.Sprintf("%04d", i), Project: "clasm", Status: "accepted"}.write(t, dir2)
	}
	if all := ingestJSON(t, kb2, dir2); len(all.StatusChanges) != 25 {
		t.Errorf("the JSON lists %d changes, want all 25", len(all.StatusChanges))
	}
}

func TestIngestStatus_EditedLineWording(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := root + "/clasm/decisions"
	testRecord{ID: "0012", Project: "clasm", Status: "proposed"}.write(t, dir)
	ingestJSON(t, kb, dir)
	testRecord{ID: "0012", Project: "clasm", Status: "accepted"}.write(t, dir)
	var sb strings.Builder
	if err := cmdIngest(kb, nil, false, []string{dir}, &sb); err != nil {
		t.Fatalf("cmdIngest: %v", err)
	}
	if want := "status: clasm/DR-0012 proposed -> accepted (edited in file)"; !strings.Contains(sb.String(), want) {
		t.Errorf("the text lacks %q:\n%s", want, sb.String())
	}
}

// The workspace tier has no project; its reference says workspace.
func TestStatusChangeRef(t *testing.T) {
	if got := statusChangeRef("", "0003"); got != "workspace/DR-0003" {
		t.Errorf("workspace tier = %q, want workspace/DR-0003", got)
	}
	if got := statusChangeRef("harvey", "0004"); got != "harvey/DR-0004" {
		t.Errorf("project tier = %q, want harvey/DR-0004", got)
	}
}
