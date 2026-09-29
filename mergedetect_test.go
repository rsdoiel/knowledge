package knowledge

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DR-0053: detection is separated from correction. PrepareMergeScratch and
// DetectIdentityIssues are the read-only half of what cmd/kb's runMerge does;
// DiffDatabases is the new one-sided detection across every table.

// diffFixture builds a database holding one row of everything that travels,
// and returns its path. Each caller copies it to get an identical twin.
func diffFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ws-"+name, "agents", "knowledge.db")
	kb, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	mustID := func(id int64, err error) int64 {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	pid := mustID(kb.AddProject("alpha", "the project"))
	cid := mustID(kb.AddConcept("toast", "bread"))
	mustID(kb.AddConcept("jam", "spread")) // unlinked: link tests use it without adding a concept row
	sid := mustID(kb.AddSource(Source{Title: "A paper", IdentifierType: "doi", IdentifierValue: "10.1/x"}))
	o1 := mustID(kb.AddObservation(pid, "note", "first"))
	o2 := mustID(kb.AddObservation(pid, "note", "second"))
	rid := mustID(kb.AddRecord(Record{RecordID: "0001", ProjectID: pid, Scope: "project", Path: "d/0001.md",
		Title: "t", Date: "2026-09-01", Status: "proposed", Kind: "decision", Body: "b", Checksum: "ck1"}))
	rid2 := mustID(kb.AddRecord(Record{RecordID: "0002", ProjectID: pid, Scope: "project", Path: "d/0002.md",
		Title: "t2", Date: "2026-09-02", Status: "proposed", Kind: "decision", Body: "b2", Checksum: "ck2"}))
	did := mustID(kb.AddDocument(Document{ProjectID: pid, Title: "doc", Format: "text", Path: "a.txt", Checksum: "dk"}))
	secid := mustID(kb.AddDocumentSection(DocumentSection{DocumentID: did, Level: "section", Heading: "H", Body: "text"}))
	for _, err := range []error{
		kb.LinkObservationConcept(o1, cid), kb.LinkProjectConcept(pid, cid),
		kb.LinkRecordConcept(rid, cid), kb.LinkDocumentSectionConcept(secid, cid),
		kb.LinkObservationSource(o1, sid, "cites"), kb.AddObservationRelation(o1, o2, "relates_to"),
		kb.AddRecordRelation(rid2, rid, "supersedes"),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := kb.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// twin copies a database to a sibling workspace directory and returns the path.
func twin(t *testing.T, src string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "ws-twin", "agents", "knowledge.db")
	copyDBForTest(t, src, dst)
	return dst
}

func mutate(t *testing.T, path string, f func(kb *KnowledgeBase)) {
	t.Helper()
	kb, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	f(kb)
	if err := kb.Close(); err != nil {
		t.Fatal(err)
	}
}

func diffByTable(t *testing.T, a, b string) map[string]TableDiff {
	t.Helper()
	diffs, err := DiffDatabases(a, b)
	if err != nil {
		t.Fatalf("DiffDatabases: %v", err)
	}
	out := map[string]TableDiff{}
	for _, d := range diffs {
		out[d.Table] = d
	}
	return out
}

func TestDiffDatabases_IdenticalReportsNothingAndEveryTableIsCounted(t *testing.T) {
	a := diffFixture(t, "a")
	b := twin(t, a)
	by := diffByTable(t, a, b)
	if len(by) != 14 {
		t.Errorf("tables = %d, want all 14 that travel", len(by))
	}
	for name, d := range by {
		if len(d.OnlyA)+len(d.OnlyB)+len(d.Different) != 0 {
			t.Errorf("%s: identical databases reported %+v", name, d)
		}
		if d.CountA != d.CountB {
			t.Errorf("%s: counts %d vs %d", name, d.CountA, d.CountB)
		}
	}
	if by["observations"].CountA != 2 || by["record_relations"].CountA != 1 {
		t.Errorf("counts are not real: %+v %+v", by["observations"], by["record_relations"])
	}
}

func TestDiffDatabases_RowAddedOnOneSideIsReportedOnceOnThatSide(t *testing.T) {
	for _, tc := range []struct {
		table string
		add   func(t *testing.T, kb *KnowledgeBase)
	}{
		{"projects", func(t *testing.T, kb *KnowledgeBase) { kb.AddProject("beta", "") }},
		{"concepts", func(t *testing.T, kb *KnowledgeBase) { kb.AddConcept("marmalade", "") }},
		{"sources", func(t *testing.T, kb *KnowledgeBase) { kb.AddSource(Source{Title: "Another"}) }},
		{"observations", func(t *testing.T, kb *KnowledgeBase) {
			p, _ := kb.ProjectByName("alpha")
			kb.AddObservation(p.ID, "note", "third")
		}},
		{"records", func(t *testing.T, kb *KnowledgeBase) {
			p, _ := kb.ProjectByName("alpha")
			kb.AddRecord(Record{RecordID: "0003", ProjectID: p.ID, Scope: "project", Path: "d/0003.md",
				Title: "t3", Date: "2026-09-03", Status: "proposed", Kind: "decision", Body: "b3", Checksum: "ck3"})
		}},
		{"documents", func(t *testing.T, kb *KnowledgeBase) {
			p, _ := kb.ProjectByName("alpha")
			kb.AddDocument(Document{ProjectID: p.ID, Title: "doc2", Format: "text", Path: "b.txt", Checksum: "dk2"})
		}},
		{"document_sections", func(t *testing.T, kb *KnowledgeBase) {
			var id int64
			kb.db.QueryRow(`SELECT id FROM documents LIMIT 1`).Scan(&id)
			kb.AddDocumentSection(DocumentSection{DocumentID: id, Level: "section", Heading: "H2", Body: "more"})
		}},
		{"observation_concepts", func(t *testing.T, kb *KnowledgeBase) {
			c, _ := kb.AddConcept("toast", "")
			kb.LinkObservationConcept(2, c)
		}},
		{"project_concepts", func(t *testing.T, kb *KnowledgeBase) {
			p, _ := kb.ProjectByName("alpha")
			c, _ := kb.AddConcept("jam", "")
			kb.LinkProjectConcept(p.ID, c)
		}},
		{"record_concepts", func(t *testing.T, kb *KnowledgeBase) {
			c, _ := kb.AddConcept("toast", "")
			kb.LinkRecordConcept(2, c)
		}},
		{"document_section_concepts", func(t *testing.T, kb *KnowledgeBase) {
			var sec int64
			kb.db.QueryRow(`SELECT id FROM document_sections LIMIT 1`).Scan(&sec)
			c, _ := kb.AddConcept("jam", "")
			kb.LinkDocumentSectionConcept(sec, c)
		}},
		{"observation_sources", func(t *testing.T, kb *KnowledgeBase) {
			var sid int64
			kb.db.QueryRow(`SELECT id FROM sources LIMIT 1`).Scan(&sid)
			kb.LinkObservationSource(2, sid, "cites")
		}},
		{"observation_relations", func(t *testing.T, kb *KnowledgeBase) {
			kb.AddObservationRelation(2, 1, "relates_to")
		}},
		{"record_relations", func(t *testing.T, kb *KnowledgeBase) {
			kb.AddRecordRelation(1, 2, "relates_to")
		}},
	} {
		t.Run(tc.table, func(t *testing.T) {
			a := diffFixture(t, "a")
			b := twin(t, a)
			mutate(t, b, func(kb *KnowledgeBase) { tc.add(t, kb) })

			d := diffByTable(t, a, b)[tc.table]
			if len(d.OnlyB) != 1 || len(d.OnlyA) != 0 || len(d.Different) != 0 {
				t.Errorf("added on b: %+v", d)
			}
			if d.CountB != d.CountA+1 {
				t.Errorf("counts a=%d b=%d", d.CountA, d.CountB)
			}
			// The mirror image: the same row seen from the other side.
			r := diffByTable(t, b, a)[tc.table]
			if len(r.OnlyA) != 1 || len(r.OnlyB) != 0 {
				t.Errorf("swapped: %+v", r)
			}
			// Nothing else is disturbed.
			for name, other := range diffByTable(t, a, b) {
				if name != tc.table && len(other.OnlyA)+len(other.OnlyB)+len(other.Different) != 0 {
					t.Errorf("%s also reports %+v", name, other)
				}
			}
		})
	}
}

// There are no tombstones, so a row deleted on one side is indistinguishable
// from a row added on the other: it is reported as only on the other side.
func TestDiffDatabases_LocalDeletionIsReportedAsOnlyInTheOther(t *testing.T) {
	a := diffFixture(t, "a")
	b := twin(t, a)
	mutate(t, b, func(kb *KnowledgeBase) {
		if _, err := kb.DeleteObservation(2, true); err != nil {
			t.Fatal(err)
		}
	})
	d := diffByTable(t, a, b)["observations"]
	if len(d.OnlyA) != 1 || len(d.OnlyB) != 0 || len(d.Different) != 0 {
		t.Errorf("observation deleted from b: %+v, want it only in a", d)
	}
	if d.CountA != 2 || d.CountB != 1 {
		t.Errorf("counts a=%d b=%d", d.CountA, d.CountB)
	}
	// Its links go with it, and are reported the same way.
	if r := diffByTable(t, a, b)["observation_relations"]; len(r.OnlyA) != 1 {
		t.Errorf("relation of the deleted observation: %+v", r)
	}
	// Seen from the other side the same fact reads as an addition.
	if s := diffByTable(t, b, a)["observations"]; len(s.OnlyB) != 1 || len(s.OnlyA) != 0 {
		t.Errorf("swapped: %+v", s)
	}
}

func TestDiffDatabases_EditedRowIsDifferentNotOneSided(t *testing.T) {
	for _, tc := range []struct {
		table string
		edit  string
	}{
		{"projects", `UPDATE projects SET description = 'changed'`},
		{"projects", `UPDATE projects SET status = 'paused'`},
		{"concepts", `UPDATE concepts SET description = 'changed' WHERE name = 'toast'`},
		{"concepts", `UPDATE concepts SET identifier_type = 'doi', identifier_value = '10.9/z' WHERE name = 'toast'`},
		{"observations", `UPDATE observations SET body = 'edited' WHERE id = 1`},
		{"observations", `UPDATE observations SET kind = 'finding' WHERE id = 1`},
		{"records", `UPDATE records SET checksum = 'other' WHERE record_id = '0001'`},
		{"sources", `UPDATE sources SET title = 'Retitled'`},
		{"documents", `UPDATE documents SET checksum = 'other'`},
		{"document_sections", `UPDATE document_sections SET body = 'edited'`},
	} {
		t.Run(tc.table+" "+tc.edit, func(t *testing.T) {
			a := diffFixture(t, "a")
			b := twin(t, a)
			mutate(t, b, func(kb *KnowledgeBase) {
				if _, err := kb.db.Exec(tc.edit); err != nil {
					t.Fatal(err)
				}
			})
			d := diffByTable(t, a, b)[tc.table]
			if len(d.Different) != 1 || len(d.OnlyA)+len(d.OnlyB) != 0 {
				t.Errorf("edited: %+v", d)
			}
			if d.Different[0].Label == "" || d.Different[0].A == d.Different[0].B {
				t.Errorf("difference lacks label or sides: %+v", d.Different[0])
			}
		})
	}
}

func TestDiffDatabases_RecordIdentityIsWorkspaceProjectScopeID(t *testing.T) {
	// The same record id in two different projects is two records, not one.
	a := diffFixture(t, "a")
	b := twin(t, a)
	mutate(t, b, func(kb *KnowledgeBase) {
		p, _ := kb.AddProject("beta", "")
		kb.AddRecord(Record{RecordID: "0001", ProjectID: p, Scope: "project", Path: "e/0001.md",
			Title: "t", Date: "2026-09-01", Status: "proposed", Kind: "decision", Body: "b", Checksum: "ck1"})
	})
	d := diffByTable(t, a, b)["records"]
	if len(d.OnlyB) != 1 || len(d.Different) != 0 {
		t.Errorf("records = %+v, want DR-0001 of beta as a new row", d)
	}
	if !strings.Contains(d.OnlyB[0], "0001") {
		t.Errorf("label %q should name the record", d.OnlyB[0])
	}
}

// ─── the read-only pipeline (B1) ─────────────────────────────────────────────

func sum(t *testing.T, path string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(b)
}

func TestPrepareMergeScratch_DetectionLeavesInputsByteIdenticalAndCleansUp(t *testing.T) {
	a := diffFixture(t, "a")
	b := twin(t, a)
	mutate(t, b, func(kb *KnowledgeBase) { kb.AddConcept("jam", "") })
	beforeA, beforeB := sum(t, a), sum(t, b)

	s, err := PrepareMergeScratch(a, b)
	if err != nil {
		t.Fatalf("PrepareMergeScratch: %v", err)
	}
	dir := filepath.Dir(s.A)
	if _, _, err := DetectIdentityIssues(s.A, s.B); err != nil {
		t.Fatal(err)
	}
	if _, err := DiffDatabases(s.A, s.B); err != nil {
		t.Fatal(err)
	}
	if sum(t, a) != beforeA || sum(t, b) != beforeB {
		t.Error("an input changed; detection belongs to the scratch copies")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("scratch directory %s survives Close", dir)
	}
}

func TestPrepareMergeScratch_NormalisesAPreRecordsDatabase(t *testing.T) {
	a := diffFixture(t, "a")
	b := twin(t, a)
	mutate(t, b, func(kb *KnowledgeBase) {
		for _, s := range []string{`DROP TABLE record_relations`, `DROP TABLE record_concepts`, `DROP TABLE records`} {
			if _, err := kb.db.Exec(s); err != nil {
				t.Fatal(err)
			}
		}
	})
	s, err := PrepareMergeScratch(a, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d, err := DiffDatabases(s.A, s.B)
	if err != nil {
		t.Fatalf("diff against a normalised pre-records copy: %v", err)
	}
	if len(d) != 14 {
		t.Errorf("tables = %d", len(d))
	}
}

func TestDetectIdentityIssues_MatchesTheTwoReports(t *testing.T) {
	a := diffFixture(t, "a")
	b := twin(t, a) // same workspace, so records match on identity
	mutate(t, b, func(kb *KnowledgeBase) {
		// Two machines that each created "alpha" independently: same name, different uuid.
		kb.db.Exec(`UPDATE projects SET uuid = 'independent-uuid'`)
		kb.db.Exec(`UPDATE records SET checksum = 'other' WHERE record_id = '0001'`)
	})
	s, err := PrepareMergeScratch(a, b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	collisions, divergences, err := DetectIdentityIssues(s.A, s.B)
	if err != nil {
		t.Fatal(err)
	}
	wantC, _ := CollisionReport(s.A, s.B)
	wantD, _ := DivergenceReport(s.A, s.B)
	if len(collisions) == 0 || len(collisions) != len(wantC) || len(divergences) != len(wantD) || len(wantD) == 0 {
		t.Errorf("collisions %d/%d divergences %d/%d", len(collisions), len(wantC), len(divergences), len(wantD))
	}
}
