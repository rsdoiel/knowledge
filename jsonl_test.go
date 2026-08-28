package knowledge

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// jsonlFixture seeds a kb with two projects (proj1 with a directly-linked
// concept and an observation-only-linked concept and a cited source; proj2
// standalone) so scoped-vs-whole-db export behavior is distinguishable.
type jsonlFixture struct {
	kb                                 *KnowledgeBase
	proj1ID, proj2ID                   int64
	directConceptID, indirectConceptID int64
	obsID                              int64
	sourceID                           int64
}

func newJSONLFixture(t *testing.T) jsonlFixture {
	t.Helper()
	kb := openTestKB(t)

	proj1ID, err := kb.AddProject("proj1", "first project")
	if err != nil {
		t.Fatalf("AddProject proj1: %v", err)
	}
	proj2ID, err := kb.AddProject("proj2", "second project")
	if err != nil {
		t.Fatalf("AddProject proj2: %v", err)
	}

	directConceptID, err := kb.AddConcept("direct-concept", "linked straight to proj1")
	if err != nil {
		t.Fatalf("AddConcept direct: %v", err)
	}
	if err := kb.LinkProjectConcept(proj1ID, directConceptID); err != nil {
		t.Fatalf("LinkProjectConcept: %v", err)
	}

	indirectConceptID, err := kb.AddConcept("indirect-concept", "reachable only via an observation")
	if err != nil {
		t.Fatalf("AddConcept indirect: %v", err)
	}

	obsID, err := kb.AddObservation(proj1ID, "finding", "a finding in proj1")
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}
	if err := kb.LinkObservationConcept(obsID, indirectConceptID); err != nil {
		t.Fatalf("LinkObservationConcept: %v", err)
	}

	sourceID, err := kb.AddSource(Source{Title: "Cited Source"})
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if err := kb.LinkObservationSource(obsID, sourceID, "cited"); err != nil {
		t.Fatalf("LinkObservationSource: %v", err)
	}

	// proj2 gets its own unrelated observation, to confirm scoped export
	// to proj1 excludes it.
	if _, err := kb.AddObservation(proj2ID, "note", "unrelated to proj1"); err != nil {
		t.Fatalf("AddObservation proj2: %v", err)
	}

	return jsonlFixture{
		kb: kb, proj1ID: proj1ID, proj2ID: proj2ID,
		directConceptID: directConceptID, indirectConceptID: indirectConceptID,
		obsID: obsID, sourceID: sourceID,
	}
}

// parseJSONLTypes decodes each line and returns the "type" field of every
// record, in file order.
func parseJSONLTypes(t *testing.T, data []byte) []string {
	t.Helper()
	var types []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			t.Fatalf("invalid JSONL line %q: %v", line, err)
		}
		types = append(types, envelope.Type)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return types
}

func countType(types []string, want string) int {
	n := 0
	for _, ty := range types {
		if ty == want {
			n++
		}
	}
	return n
}

func TestExportJSONL_WholeDatabase(t *testing.T) {
	f := newJSONLFixture(t)
	var buf bytes.Buffer
	if err := ExportJSONL(f.kb, &buf, ""); err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}
	types := parseJSONLTypes(t, buf.Bytes())

	wantCounts := map[string]int{
		"project":             2,
		"concept":             2,
		"source":              1,
		"observation":         2,
		"observation_concept": 1,
		"project_concept":     1,
		"observation_source":  1,
	}
	for ty, want := range wantCounts {
		if got := countType(types, ty); got != want {
			t.Errorf("count(%q) = %d, want %d (types=%v)", ty, got, want, types)
		}
	}

	// Dependency order: every parent type's first occurrence precedes
	// every child type's first occurrence.
	firstIdx := map[string]int{}
	for i, ty := range types {
		if _, ok := firstIdx[ty]; !ok {
			firstIdx[ty] = i
		}
	}
	order := []string{"project", "concept", "source", "observation", "observation_concept", "project_concept", "observation_source"}
	for i := 1; i < len(order); i++ {
		prev, cur := order[i-1], order[i]
		if firstIdx[cur] < firstIdx[prev] {
			t.Errorf("type %q (first at %d) appears before %q (first at %d), want dependency order",
				cur, firstIdx[cur], prev, firstIdx[prev])
		}
	}
}

func TestExportJSONL_ScopedToProject(t *testing.T) {
	f := newJSONLFixture(t)
	var buf bytes.Buffer
	if err := ExportJSONL(f.kb, &buf, "proj1"); err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}
	types := parseJSONLTypes(t, buf.Bytes())

	if got := countType(types, "project"); got != 1 {
		t.Errorf("project count = %d, want 1 (proj2 must be excluded)", got)
	}
	if got := countType(types, "concept"); got != 2 {
		t.Errorf("concept count = %d, want 2 (direct + indirect-via-observation)", got)
	}
	if got := countType(types, "observation"); got != 1 {
		t.Errorf("observation count = %d, want 1 (proj2's observation excluded)", got)
	}
	if got := countType(types, "source"); got != 1 {
		t.Errorf("source count = %d, want 1", got)
	}
}

func TestExportJSONL_EmptyDatabase(t *testing.T) {
	kb := openTestKB(t)
	var buf bytes.Buffer
	if err := ExportJSONL(kb, &buf, ""); err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("export of empty db = %q, want empty output", buf.String())
	}
}

func TestExportJSONL_UnknownProjectErrors(t *testing.T) {
	f := newJSONLFixture(t)
	var buf bytes.Buffer
	if err := ExportJSONL(f.kb, &buf, "does-not-exist"); err == nil {
		t.Errorf("ExportJSONL with unknown project = nil error, want an error")
	}
}

func TestExportJSONL_ProjectRecordFields(t *testing.T) {
	f := newJSONLFixture(t)
	var buf bytes.Buffer
	if err := ExportJSONL(f.kb, &buf, "proj1"); err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}
	p, err := f.kb.ProjectByName("proj1")
	if err != nil || p == nil {
		t.Fatalf("ProjectByName: %v", err)
	}

	sc := bufio.NewScanner(bytes.NewReader(buf.Bytes()))
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, `"type":"project"`) {
			continue
		}
		var rec struct {
			Type        string `json:"type"`
			UUID        string `json:"uuid"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Status      string `json:"status"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("unmarshal project record: %v", err)
		}
		if rec.UUID == "" {
			t.Errorf("project record has empty uuid: %s", line)
		}
		if rec.Name != "proj1" || rec.Description != "first project" || rec.Status != "active" {
			t.Errorf("project record = %+v, want name=proj1 description=%q status=active", rec, "first project")
		}
		return
	}
	t.Fatal("no project record found in export")
}

// summaryByTable indexes an ImportTableSummary slice by Table for
// convenient per-type assertions.
func summaryByTable(summary []ImportTableSummary) map[string]ImportTableSummary {
	out := map[string]ImportTableSummary{}
	for _, s := range summary {
		out[s.Table] = s
	}
	return out
}

func TestImportJSONL_RoundTrip(t *testing.T) {
	src := newJSONLFixture(t)
	var buf bytes.Buffer
	if err := ExportJSONL(src.kb, &buf, ""); err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}

	dst := openTestKB(t)
	summary, err := ImportJSONL(dst, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}

	byTable := summaryByTable(summary)
	wantImported := map[string]int{
		"project":             2,
		"concept":             2,
		"source":              1,
		"observation":         2,
		"observation_concept": 1,
		"project_concept":     1,
		"observation_source":  1,
	}
	for table, want := range wantImported {
		s, ok := byTable[table]
		if !ok {
			t.Errorf("no summary entry for table %q (summary=%+v)", table, summary)
			continue
		}
		if s.Imported != want || s.Skipped != 0 || s.Read != want {
			t.Errorf("summary[%q] = %+v, want Read=Imported=%d Skipped=0", table, s, want)
		}
	}

	projects, err := dst.Projects()
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("dst.Projects() = %d rows, want 2", len(projects))
	}
	p1, err := dst.ProjectByName("proj1")
	if err != nil || p1 == nil {
		t.Fatalf("ProjectByName proj1: %v", err)
	}
	obs, err := dst.Observations(p1.ID)
	if err != nil {
		t.Fatalf("Observations: %v", err)
	}
	if len(obs) != 1 || obs[0].Body != "a finding in proj1" {
		t.Errorf("dst proj1 observations = %+v, want one matching the source observation", obs)
	}
	concepts, err := dst.ProjectConcepts(p1.ID)
	if err != nil {
		t.Fatalf("ProjectConcepts: %v", err)
	}
	if len(concepts) != 1 || concepts[0].Name != "direct-concept" {
		t.Errorf("dst proj1 concepts = %+v, want [direct-concept]", concepts)
	}
	obsSources, err := dst.ObservationSources(obs[0].ID)
	if err != nil {
		t.Fatalf("ObservationSources: %v", err)
	}
	if len(obsSources) != 1 || obsSources[0].Title != "Cited Source" {
		t.Errorf("dst observation sources = %+v, want [Cited Source]", obsSources)
	}
}

func TestImportJSONL_ReimportIsNoOp(t *testing.T) {
	src := newJSONLFixture(t)
	var buf bytes.Buffer
	if err := ExportJSONL(src.kb, &buf, ""); err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}

	dst := openTestKB(t)
	if _, err := ImportJSONL(dst, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("first ImportJSONL: %v", err)
	}
	summary, err := ImportJSONL(dst, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("second ImportJSONL: %v", err)
	}

	for _, s := range summary {
		if s.Imported != 0 || s.Skipped != s.Read {
			t.Errorf("re-import summary[%q] = %+v, want Imported=0 Skipped=Read", s.Table, s)
		}
	}

	projects, err := dst.Projects()
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("dst.Projects() after re-import = %d rows, want 2 (no duplicates)", len(projects))
	}
}

func TestImportJSONL_SameNameDifferentUUIDMergesUnderLocalProject(t *testing.T) {
	// Two independently-created databases both have a project named
	// "shared" but with different uuids (no cross-machine sync has
	// happened yet) -- importing kbB's export into kbA must attach kbB's
	// observation to kbA's existing local "shared" row, not create a
	// second "shared" project or drop the observation.
	kbA := openTestKB(t)
	if _, err := kbA.AddProject("shared", "kbA's version"); err != nil {
		t.Fatalf("AddProject on kbA: %v", err)
	}

	kbB := openTestKB(t)
	if _, err := kbB.AddProject("shared", "kbB's version"); err != nil {
		t.Fatalf("AddProject on kbB: %v", err)
	}
	pB, err := kbB.ProjectByName("shared")
	if err != nil || pB == nil {
		t.Fatalf("ProjectByName on kbB: %v", err)
	}
	if _, err := kbB.AddObservation(pB.ID, "note", "observed on kbB"); err != nil {
		t.Fatalf("AddObservation on kbB: %v", err)
	}

	var buf bytes.Buffer
	if err := ExportJSONL(kbB, &buf, ""); err != nil {
		t.Fatalf("ExportJSONL kbB: %v", err)
	}
	if _, err := ImportJSONL(kbA, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("ImportJSONL into kbA: %v", err)
	}

	projects, err := kbA.Projects()
	if err != nil {
		t.Fatalf("kbA.Projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("kbA.Projects() = %d rows, want 1 (must not duplicate 'shared')", len(projects))
	}
	pA, err := kbA.ProjectByName("shared")
	if err != nil || pA == nil {
		t.Fatalf("ProjectByName on kbA: %v", err)
	}
	if pA.Description != "kbA's version" {
		t.Errorf("kbA local project description = %q, want %q (local row must win, not be overwritten)", pA.Description, "kbA's version")
	}
	obs, err := kbA.Observations(pA.ID)
	if err != nil {
		t.Fatalf("kbA.Observations: %v", err)
	}
	if len(obs) != 1 || obs[0].Body != "observed on kbB" {
		t.Errorf("kbA 'shared' observations = %+v, want kbB's observation attached to the local project row", obs)
	}
}

func TestImportJSONL_UnresolvableJoinReferenceIsSkippedNotFatal(t *testing.T) {
	dst := openTestKB(t)
	if _, err := dst.AddProject("p", ""); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	p, err := dst.ProjectByName("p")
	if err != nil || p == nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	var projectUUID string
	if err := dst.db.QueryRow(`SELECT uuid FROM projects WHERE id = ?`, p.ID).Scan(&projectUUID); err != nil {
		t.Fatalf("query project uuid: %v", err)
	}

	input := fmt.Sprintf(`{"type":"observation","uuid":"11111111-1111-7111-8111-111111111111","origin_host":"h","project_uuid":"%s","kind":"note","body":"b","source_doi":"","created_at":"2026-01-01T00:00:00Z"}
{"type":"observation_concept","observation_uuid":"11111111-1111-7111-8111-111111111111","concept_uuid":"does-not-exist"}
`, projectUUID)
	summary, err := ImportJSONL(dst, strings.NewReader(input))
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}
	byTable := summaryByTable(summary)
	oc := byTable["observation_concept"]
	if oc.Read != 1 || oc.Imported != 0 || oc.Skipped != 1 {
		t.Errorf("observation_concept summary = %+v, want Read=1 Imported=0 Skipped=1 (unresolvable concept_uuid)", oc)
	}

	obs, err := dst.Observations(p.ID)
	if err != nil {
		t.Fatalf("Observations: %v", err)
	}
	if len(obs) != 1 {
		t.Errorf("observations after import = %d, want 1 (the observation itself must still import)", len(obs))
	}
}

func TestImportJSONL_MalformedLineReturnsLineNumber(t *testing.T) {
	dst := openTestKB(t)
	input := `{"type":"project","uuid":"x","name":"ok","status":"active"}
{"type":"project","uuid":"y","name":"broken"
`
	_, err := ImportJSONL(dst, strings.NewReader(input))
	if err == nil {
		t.Fatal("ImportJSONL with malformed second line = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error = %q, want it to mention line 2", err.Error())
	}
}

// ─── W5: records / record_relations (DR-0013, DR-0018, DR-0019) ───────────

// newJSONLRecordsFixture seeds a kb under workspace "ws1" with one
// project-tier record, one workspace-tier record, and a relates_to edge
// between them, kept separate from newJSONLFixture so the record-specific
// assertions below don't have to be threaded through every other test's
// exact-count checks.
func newJSONLRecordsFixture(t *testing.T) (kb *KnowledgeBase, projectID, projRecID, wsRecID int64) {
	t.Helper()
	kb = openTestKBAt(t, "ws1")
	var err error
	projectID, err = kb.AddProject("proj", "a project")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	projRecID, err = kb.AddRecord(Record{
		RecordID: "0001", ProjectID: projectID, Scope: "project",
		Path: "decisions/0001-x.md", Title: "Project-tier record",
		Date: "2026-08-28", Status: "accepted", Kind: "decision",
		Trigger: "design", Body: "project-tier body", Checksum: "abc123",
	})
	if err != nil {
		t.Fatalf("AddRecord project-tier: %v", err)
	}
	wsRecID, err = kb.AddRecord(Record{
		RecordID: "0001", Scope: "workspace",
		Path: "agents/decisions/0001-y.md", Title: "Workspace-tier record",
		Date: "2026-08-28", Status: "accepted", Kind: "decision",
		Trigger: "design", Body: "workspace-tier body", Checksum: "def456",
	})
	if err != nil {
		t.Fatalf("AddRecord workspace-tier: %v", err)
	}
	if err := kb.AddRecordRelation(wsRecID, projRecID, "relates_to"); err != nil {
		t.Fatalf("AddRecordRelation: %v", err)
	}
	return kb, projectID, projRecID, wsRecID
}

func TestExportJSONL_Records_WholeDatabaseIncludesBothTiers(t *testing.T) {
	kb, _, _, _ := newJSONLRecordsFixture(t)
	var buf bytes.Buffer
	if err := ExportJSONL(kb, &buf, ""); err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}
	types := parseJSONLTypes(t, buf.Bytes())
	if got := countType(types, "record"); got != 2 {
		t.Errorf("record count = %d, want 2 (project-tier and workspace-tier)", got)
	}
	if got := countType(types, "record_relation"); got != 1 {
		t.Errorf("record_relation count = %d, want 1", got)
	}
}

func TestExportJSONL_Records_ScopedExportExcludesWorkspaceTier(t *testing.T) {
	// DR-0019: a --project-scoped export carries only that project's
	// records; workspace-tier records, and relations crossing out of the
	// scoped set, appear only in an unscoped export.
	kb, _, _, _ := newJSONLRecordsFixture(t)
	var buf bytes.Buffer
	if err := ExportJSONL(kb, &buf, "proj"); err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}
	types := parseJSONLTypes(t, buf.Bytes())
	if got := countType(types, "record"); got != 1 {
		t.Errorf("scoped record count = %d, want 1 (workspace-tier excluded)", got)
	}
	if got := countType(types, "record_relation"); got != 0 {
		t.Errorf("scoped record_relation count = %d, want 0 (relation crosses the scope boundary)", got)
	}
}

func TestImportJSONL_Records_RoundTrip(t *testing.T) {
	src, _, _, _ := newJSONLRecordsFixture(t)
	var buf bytes.Buffer
	if err := ExportJSONL(src, &buf, ""); err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}

	dst := openTestKB(t)
	summary, err := ImportJSONL(dst, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}
	byTable := summaryByTable(summary)
	if s := byTable["record"]; s.Imported != 2 || s.Skipped != 0 {
		t.Errorf("record summary = %+v, want Imported=2 Skipped=0", s)
	}
	if s := byTable["record_relation"]; s.Imported != 1 || s.Skipped != 0 {
		t.Errorf("record_relation summary = %+v, want Imported=1 Skipped=0", s)
	}

	p, err := dst.ProjectByName("proj")
	if err != nil || p == nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	projRec, err := dst.RecordByIdentity("ws1", p.ID, "project", "0001")
	if err != nil {
		t.Fatalf("RecordByIdentity project-tier: %v", err)
	}
	if projRec.Title != "Project-tier record" || projRec.Body != "project-tier body" || projRec.Checksum != "abc123" {
		t.Errorf("imported project-tier record = %+v, want fields preserved", projRec)
	}
	wsRec, err := dst.RecordByIdentity("ws1", 0, "workspace", "0001")
	if err != nil {
		t.Fatalf("RecordByIdentity workspace-tier: %v", err)
	}
	rels, err := dst.RelationsFor(wsRec.ID)
	if err != nil {
		t.Fatalf("RelationsFor: %v", err)
	}
	if len(rels) != 1 || rels[0].RecordID != projRec.ID || rels[0].Relationship != "relates_to" {
		t.Errorf("relations for workspace-tier record = %+v, want one relates_to edge to the project-tier record", rels)
	}
}

func TestImportJSONL_Records_ReimportIsNoOp(t *testing.T) {
	src, _, _, _ := newJSONLRecordsFixture(t)
	var buf bytes.Buffer
	if err := ExportJSONL(src, &buf, ""); err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}
	dst := openTestKB(t)
	if _, err := ImportJSONL(dst, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("first ImportJSONL: %v", err)
	}
	summary, err := ImportJSONL(dst, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("second ImportJSONL: %v", err)
	}
	byTable := summaryByTable(summary)
	if s := byTable["record"]; s.Imported != 0 || s.Skipped != s.Read {
		t.Errorf("re-import record summary = %+v, want Imported=0 Skipped=Read", s)
	}
	if s := byTable["record_relation"]; s.Imported != 0 || s.Skipped != s.Read {
		t.Errorf("re-import record_relation summary = %+v, want Imported=0 Skipped=Read", s)
	}
}

func TestImportJSONL_Records_ProjectMatchedByNameNotUUID(t *testing.T) {
	// DR-0018: a record's project is matched across databases by name, not
	// by the project's own uuid, which two independently-created "same
	// project" rows need not share before reconciliation.
	kbA := openTestKB(t)
	if _, err := kbA.AddProject("shared", "kbA's version"); err != nil {
		t.Fatalf("AddProject on kbA: %v", err)
	}

	kbB := openTestKBAt(t, "wsB")
	pB, err := kbB.AddProject("shared", "kbB's version")
	if err != nil {
		t.Fatalf("AddProject on kbB: %v", err)
	}
	if _, err := kbB.AddRecord(Record{
		RecordID: "0001", ProjectID: pB, Scope: "project",
		Path: "decisions/0001-x.md", Title: "kbB's record",
		Date: "2026-08-28", Status: "accepted", Kind: "decision",
		Body: "body", Checksum: "chk",
	}); err != nil {
		t.Fatalf("AddRecord on kbB: %v", err)
	}

	var buf bytes.Buffer
	if err := ExportJSONL(kbB, &buf, ""); err != nil {
		t.Fatalf("ExportJSONL kbB: %v", err)
	}
	if _, err := ImportJSONL(kbA, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("ImportJSONL into kbA: %v", err)
	}

	pA, err := kbA.ProjectByName("shared")
	if err != nil || pA == nil {
		t.Fatalf("ProjectByName on kbA: %v", err)
	}
	rec, err := kbA.RecordByIdentity("wsB", pA.ID, "project", "0001")
	if err != nil {
		t.Fatalf("RecordByIdentity on kbA: %v (record must attach to kbA's local 'shared' project)", err)
	}
	if rec.Title != "kbB's record" {
		t.Errorf("imported record = %+v, want Title=%q", rec, "kbB's record")
	}
}

func TestImportJSONL_Records_UnresolvableRelationEndpointSkippedNotFatal(t *testing.T) {
	dst := openTestKB(t)
	input := `{"type":"record","uuid":"11111111-1111-7111-8111-111111111111","origin_host":"h","workspace":"ws","project_name":"","record_id":"0001","scope":"workspace","path":"agents/decisions/0001-x.md","title":"t","date":"2026-08-28","status":"accepted","kind":"decision","trigger":"","phase":"","initiative":"","session":"","body":"b","checksum":"c","ingested_at":""}
{"type":"record_relation","from_uuid":"11111111-1111-7111-8111-111111111111","to_uuid":"does-not-exist","relationship":"relates_to"}
`
	summary, err := ImportJSONL(dst, strings.NewReader(input))
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}
	byTable := summaryByTable(summary)
	rr := byTable["record_relation"]
	if rr.Read != 1 || rr.Imported != 0 || rr.Skipped != 1 {
		t.Errorf("record_relation summary = %+v, want Read=1 Imported=0 Skipped=1", rr)
	}
	if _, err := dst.RecordByIdentity("ws", 0, "workspace", "0001"); err != nil {
		t.Errorf("RecordByIdentity: %v, want the record itself to still import", err)
	}
}

func TestImportJSONL_Records_UnresolvableProjectNameSkippedNotFatal(t *testing.T) {
	dst := openTestKB(t)
	input := `{"type":"record","uuid":"11111111-1111-7111-8111-111111111111","origin_host":"h","workspace":"ws","project_name":"does-not-exist","record_id":"0001","scope":"project","path":"decisions/0001-x.md","title":"t","date":"2026-08-28","status":"accepted","kind":"decision","trigger":"","phase":"","initiative":"","session":"","body":"b","checksum":"c","ingested_at":""}
`
	summary, err := ImportJSONL(dst, strings.NewReader(input))
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}
	byTable := summaryByTable(summary)
	r := byTable["record"]
	if r.Read != 1 || r.Imported != 0 || r.Skipped != 1 {
		t.Errorf("record summary = %+v, want Read=1 Imported=0 Skipped=1 (unresolvable project_name)", r)
	}
}

// TestImportJSONL_Records_UnionAcceptance is the export/import equivalent
// of the DR-0013 (W6) acceptance scenario in
// TestMergeKnowledgeBases_RecordsAcceptance: two databases, each holding
// records the other lacks, one of them workspace-tier. Exporting b and
// importing into a must leave a holding the union, with the workspace-tier
// record surviving and every record reachable by Search -- DR-0003's claimed
// equivalence between the merge and export/import routes, as a test rather
// than a claim.
func TestImportJSONL_Records_UnionAcceptance(t *testing.T) {
	a := openTestKBAt(t, "wsA")
	aid, err := a.AddProject("harvey", "")
	if err != nil {
		t.Fatalf("AddProject a: %v", err)
	}
	if _, err := a.AddRecord(Record{RecordID: "0001", ProjectID: aid, Scope: "project",
		Path: "decisions/0001-a.md", Title: "a only project record", Date: "2026-08-28",
		Status: "accepted", Kind: "decision", Body: "body a1", Checksum: "cka1"}); err != nil {
		t.Fatalf("AddRecord a project-tier: %v", err)
	}
	if _, err := a.AddRecord(Record{RecordID: "0001", Scope: "workspace",
		Path: "agents/decisions/0001-a.md", Title: "a only workspace record", Date: "2026-08-28",
		Status: "accepted", Kind: "decision", Body: "body a-ws", Checksum: "ckaws"}); err != nil {
		t.Fatalf("AddRecord a workspace-tier: %v", err)
	}

	b := openTestKBAt(t, "wsB")
	bid, err := b.AddProject("harvey", "")
	if err != nil {
		t.Fatalf("AddProject b: %v", err)
	}
	if _, err := b.AddRecord(Record{RecordID: "0002", ProjectID: bid, Scope: "project",
		Path: "decisions/0002-b.md", Title: "b only project record", Date: "2026-08-28",
		Status: "accepted", Kind: "decision", Body: "body b2", Checksum: "ckb2"}); err != nil {
		t.Fatalf("AddRecord b: %v", err)
	}

	var buf bytes.Buffer
	if err := ExportJSONL(b, &buf, ""); err != nil {
		t.Fatalf("ExportJSONL b: %v", err)
	}
	if _, err := ImportJSONL(a, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("ImportJSONL into a: %v", err)
	}

	recs, err := a.ListRecords(RecordFilter{})
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("a's records after import = %d, want 3 (union of both sides)", len(recs))
	}

	ws, err := a.RecordByIdentity("wsA", 0, "workspace", "0001")
	if err != nil {
		t.Fatalf("RecordByIdentity workspace-tier: %v", err)
	}
	if ws.Title != "a only workspace record" {
		t.Errorf("a's workspace-tier record = %+v, want it to have survived unchanged", ws)
	}

	for _, term := range []string{"a only project record", "a only workspace record", "b only project record"} {
		results, err := a.Search(term)
		if err != nil {
			t.Fatalf("Search(%q): %v", term, err)
		}
		found := false
		for _, r := range results {
			if r.SourceType == "record" {
				found = true
			}
		}
		if !found {
			t.Errorf("Search(%q) found no record hit; every record from both sides must be reachable after import", term)
		}
	}
}

func TestImportJSONL_UnknownTypeIsSkippedNotFatal(t *testing.T) {
	dst := openTestKB(t)
	input := `{"type":"future_record_kind","some_field":"value"}
`
	summary, err := ImportJSONL(dst, strings.NewReader(input))
	if err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}
	for _, s := range summary {
		if s.Table == "future_record_kind" {
			return
		}
	}
	t.Errorf("summary = %+v, want an entry for the unknown type future_record_kind", summary)
}
