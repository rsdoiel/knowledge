package cli

import (
	"bytes"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// `kb record relates-to A B` does for a plain cross-reference what supersede does
// for a replacement: it writes the file and the database in one step, so nobody
// edits frontmatter with sed.

func relationsOf(t *testing.T, kb *knowledge.KnowledgeBase, project, id string) []knowledge.RelatedRecord {
	t.Helper()
	p, _ := kb.ProjectByName(project)
	rec, err := kb.RecordByIdentity(kb.Workspace(), p.ID, "project", id)
	if err != nil {
		t.Fatalf("RecordByIdentity %s: %v", id, err)
	}
	rels, err := kb.RelationsFor(rec.ID)
	if err != nil {
		t.Fatalf("RelationsFor: %v", err)
	}
	return rels
}

func TestCmdRecord_RelatesToWritesTheFileAndTheDatabase(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001", Status: "accepted"},
		testRecord{ID: "0002", Status: "accepted"},
	)
	runRecord(t, kb, "relates-to", "clasm/0002", "clasm/0001")

	if got := frontmatterLine(t, readFixture(t, root, "clasm", "0002"), "relates_to"); got != `relates_to: ["0001"]` {
		t.Errorf("0002 relates_to line = %q, want %q", got, `relates_to: ["0001"]`)
	}
	if got := frontmatterLine(t, readFixture(t, root, "clasm", "0001"), "status"); got != "status: accepted" {
		t.Errorf("the target's status changed: %q", got)
	}
	rels := relationsOf(t, kb, "clasm", "0002")
	if len(rels) != 1 || rels[0].Relationship != "relates_to" {
		t.Errorf("relations = %+v, want one relates_to", rels)
	}
	// Repeating it lists nothing twice.
	runRecord(t, kb, "relates-to", "clasm/0002", "clasm/0001")
	if got := frontmatterLine(t, readFixture(t, root, "clasm", "0002"), "relates_to"); got != `relates_to: ["0001"]` {
		t.Errorf("after a repeat, relates_to line = %q", got)
	}
	if rels := relationsOf(t, kb, "clasm", "0002"); len(rels) != 1 {
		t.Errorf("after a repeat, %d relations, want 1", len(rels))
	}
}

func TestCmdRecord_RelatesToAcceptsSeveralTargetsAndKeepsTheExisting(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm",
		testRecord{ID: "0001"}, testRecord{ID: "0002"}, testRecord{ID: "0003"},
	)
	runRecord(t, kb, "relates-to", "clasm/0003", "clasm/0001")
	runRecord(t, kb, "relates-to", "clasm/0003", "clasm/0002")
	if got := frontmatterLine(t, readFixture(t, root, "clasm", "0003"), "relates_to"); got != `relates_to: ["0001", "0002"]` {
		t.Errorf("relates_to line = %q", got)
	}
}

func TestCmdRecord_RelatesToRemoveDropsTheEntryAndTheRow(t *testing.T) {
	kb, root := fixtureWorkspace(t, "clasm", testRecord{ID: "0001"}, testRecord{ID: "0002"})
	runRecord(t, kb, "relates-to", "clasm/0002", "clasm/0001")
	runRecord(t, kb, "relates-to", "clasm/0002", "clasm/0001", "--remove")

	if got := frontmatterLine(t, readFixture(t, root, "clasm", "0002"), "relates_to"); got != "relates_to: []" {
		t.Errorf("relates_to line = %q, want an empty list", got)
	}
	if rels := relationsOf(t, kb, "clasm", "0002"); len(rels) != 0 {
		t.Errorf("relations = %+v, want none", rels)
	}
}

func TestCmdRecord_RelatesToRemoveOfAnAbsentLinkIsNegative(t *testing.T) {
	kb, _ := fixtureWorkspace(t, "clasm", testRecord{ID: "0001"}, testRecord{ID: "0002"})
	err := cmdRecord(kb, nil, false, []string{"relates-to", "clasm/0002", "clasm/0001", "--remove"}, &bytes.Buffer{})
	if err == nil || exitCodeFor(err).Code != 1 {
		t.Errorf("err = %v (exit %d), want exit 1: the answer is no", err, exitCodeFor(err).Code)
	}
}

func TestCmdRecord_RelatesToRefusals(t *testing.T) {
	kb, _ := fixtureWorkspace(t, "clasm", testRecord{ID: "0001"}, testRecord{ID: "0002"})
	for _, tc := range []struct {
		name string
		args []string
		code int
	}{
		{"itself", []string{"relates-to", "clasm/0001", "clasm/0001"}, 2},
		{"one argument", []string{"relates-to", "clasm/0001"}, 2},
		{"no such target", []string{"relates-to", "clasm/0001", "clasm/0099"}, exitCodeFor(notFoundf("x")).Code},
		{"bare id with no scope", []string{"relates-to", "0001", "0002"}, 2},
	} {
		err := cmdRecord(kb, nil, false, tc.args, &bytes.Buffer{})
		var got int
		if err != nil {
			got = exitCodeFor(err).Code
		}
		if err == nil || got != tc.code {
			t.Errorf("%s: err %v exit %d, want exit %d", tc.name, err, got, tc.code)
		}
	}
}

// A target in another tier is written as a qualified entry, which ingest reads
// back; a target in the same tier is a bare id.
func TestRelatesEntry(t *testing.T) {
	names := map[int64]string{1: "clasm", 2: "other"}
	same := knowledge.Record{ProjectID: 1, Scope: "project", RecordID: "0001"}
	for _, tc := range []struct {
		from, to knowledge.Record
		want     string
	}{
		{same, knowledge.Record{ProjectID: 1, Scope: "project", RecordID: "0002"}, "0002"},
		{same, knowledge.Record{ProjectID: 2, Scope: "project", RecordID: "0007"}, "other:0007"},
		{same, knowledge.Record{Scope: "workspace", RecordID: "0011"}, "workspace:0011"},
	} {
		if got := relatesEntry(tc.from, tc.to, names); got != tc.want {
			t.Errorf("relatesEntry(%v) = %q, want %q", tc.to.RecordID, got, tc.want)
		}
	}
}
