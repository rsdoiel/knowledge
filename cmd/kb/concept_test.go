package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

func TestCmdConcept_AddThenList(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"add", "WAL", "write-ahead", "logging"}, &out); err != nil {
		t.Fatalf("concept add: %v", err)
	}
	if !strings.Contains(out.String(), "WAL") {
		t.Errorf("add output = %q, want it to mention WAL", out.String())
	}

	out.Reset()
	if err := cmdConcept(kb, nil, false, []string{"list"}, &out); err != nil {
		t.Fatalf("concept list: %v", err)
	}
	if !strings.Contains(out.String(), "WAL") {
		t.Errorf("list output = %q, want it to mention WAL", out.String())
	}
}

func TestCmdConcept_AddWithIdentifier(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	err := cmdConcept(kb, nil, false, []string{"add", "--identifier-type", "orcid", "--identifier-value", "0000-0003-0900-6903", "Jane Doe"}, &out)
	if err != nil {
		t.Fatalf("concept add: %v", err)
	}
	concepts, err := kb.Concepts()
	if err != nil {
		t.Fatalf("Concepts: %v", err)
	}
	if len(concepts) != 1 || concepts[0].IdentifierType != "orcid" {
		t.Errorf("concepts = %+v, want one concept with IdentifierType=orcid", concepts)
	}
}

func TestCmdConcept_AddRequiresName(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"add"}, &out); err == nil {
		t.Error("expected an error when NAME is missing")
	}
}

// ─── rename (DR-0024) ────────────────────────────────────────────────────────

func TestCmdConcept_Rename(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("oldname", "a concept"); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"rename", "oldname", "newname"}, &out); err != nil {
		t.Fatalf("concept rename: %v", err)
	}
	concepts, err := kb.Concepts()
	if err != nil {
		t.Fatalf("Concepts: %v", err)
	}
	if len(concepts) != 1 || concepts[0].Name != "newname" {
		t.Errorf("concepts = %+v, want one concept named newname", concepts)
	}
}

func TestCmdConcept_RenameJSON(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("oldname", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, true, []string{"rename", "oldname", "newname"}, &out); err != nil {
		t.Fatalf("concept rename: %v", err)
	}
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

func TestCmdConcept_RenameRequiresTwoArguments(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("oldname", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"rename", "oldname"}, &out); err == nil {
		t.Error("expected an error when NEW is missing")
	}
}

func TestCmdConcept_RenameNotFound(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"rename", "nonexistent", "newname"}, &out); err == nil {
		t.Error("expected an error for a nonexistent concept")
	}
}

// kb-concept(1) is generated from ConceptHelpText, so a subcommand missing
// from it never reaches the man page.
func TestConceptHelpText_DocumentsRename(t *testing.T) {
	if !strings.Contains(ConceptHelpText, "rename") {
		t.Error("ConceptHelpText does not document rename")
	}
}

func TestCmdConcept_UnknownSubcommand(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"bogus"}, &out); err == nil {
		t.Error("expected an error for an unknown concept subcommand")
	}
}

// ─── suggest (TODO.md's term-frequency item) ────────────────────────────────

func TestCmdConcept_SuggestScansRecordAndDocumentBodies(t *testing.T) {
	kb := openTestKB(t)
	pid, err := kb.AddProject("alpha", "")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0001", ProjectID: pid, Scope: "project", Path: "decisions/0001-x.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body: "chunking is discussed here in some depth", Checksum: "c1",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	docID, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "doc", Format: "text", Path: "a.txt"})
	if err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if _, err := kb.AddDocumentSection(knowledge.DocumentSection{
		DocumentID: docID, Level: "section", Body: "chunking comes up again in this document",
	}); err != nil {
		t.Fatalf("AddDocumentSection: %v", err)
	}
	// A third, unrelated record keeps chunking from appearing in literally
	// every scanned item (df < n) -- otherwise its idf collapses to zero
	// regardless of how many times it's mentioned.
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0002", ProjectID: pid, Scope: "project", Path: "decisions/0002-y.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body: "something entirely unrelated", Checksum: "c2",
	}); err != nil {
		t.Fatalf("AddRecord unrelated: %v", err)
	}

	var out bytes.Buffer
	if err := cmdConcept(kb, nil, true, []string{"suggest"}, &out); err != nil {
		t.Fatalf("concept suggest: %v", err)
	}
	var got struct {
		Candidates []map[string]any `json:"candidates"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v (%q)", err, out.String())
	}
	found := false
	for _, c := range got.Candidates {
		if c["term"] == "chunking" {
			found = true
		}
	}
	if !found {
		t.Errorf("suggestions = %v, want chunking, mentioned across a record and a document", got.Candidates)
	}
}

func TestCmdConcept_SuggestExcludesExistingConcepts(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0001", ProjectID: pid, Scope: "project", Path: "decisions/0001-x.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body: "chunking chunking chunking", Checksum: "c1",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, true, []string{"suggest"}, &out); err != nil {
		t.Fatalf("concept suggest: %v", err)
	}
	if strings.Contains(out.String(), "chunking") {
		t.Errorf("output = %q, want chunking excluded -- it is already a known concept", out.String())
	}
}

func TestCmdConcept_SuggestScopesToProject(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	qid, _ := kb.AddProject("beta", "")
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0001", ProjectID: pid, Scope: "project", Path: "decisions/0001-x.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body: "widgetry widgetry appears here in alpha's own corpus", Checksum: "c1",
	}); err != nil {
		t.Fatalf("AddRecord alpha: %v", err)
	}
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0001", ProjectID: qid, Scope: "project", Path: "decisions/0001-x.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body: "nothing about that topic appears in beta at all", Checksum: "c2",
	}); err != nil {
		t.Fatalf("AddRecord beta: %v", err)
	}
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"suggest", "--project", "beta"}, &out); err != nil {
		t.Fatalf("concept suggest --project beta: %v", err)
	}
	if strings.Contains(out.String(), "widgetry") {
		t.Errorf("output = %q, want widgetry excluded -- it only appears in alpha, not beta", out.String())
	}
}

func TestCmdConcept_SuggestRespectsLimit(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0001", ProjectID: pid, Scope: "project", Path: "decisions/0001-x.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body:     "alpha alpha bravo bravo charlie charlie delta delta echo echo distinct distinct",
		Checksum: "c1",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0002", ProjectID: pid, Scope: "project", Path: "decisions/0002-y.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body:     "just filler text so these terms are not present in every item",
		Checksum: "c2",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	var got struct {
		Candidates []knowledge.ConceptCandidate `json:"candidates"`
	}
	runConceptJSON(t, kb, &got, "suggest", "--limit", "2")
	if len(got.Candidates) > 2 {
		t.Errorf("got %d suggestions, want at most 2 (--limit 2)", len(got.Candidates))
	}
}

func TestCmdConcept_SuggestUnknownProject(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"suggest", "--project", "nonexistent"}, &out); err == nil {
		t.Error("expected an error for an unknown project")
	}
}

func TestCmdConcept_SuggestEmptyCorpus(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"suggest"}, &out); err != nil {
		t.Fatalf("concept suggest: %v", err)
	}
	if !strings.Contains(out.String(), "no candidate") {
		t.Errorf("output = %q, want a no-candidates message for an empty corpus", out.String())
	}
}

// runConceptJSON calls cmdConcept in JSON mode and decodes into v.
func runConceptJSON(t *testing.T, kb *knowledge.KnowledgeBase, v any, args ...string) {
	t.Helper()
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, true, args, &out); err != nil {
		t.Fatalf("cmdConcept %v: %v", args, err)
	}
	if err := json.Unmarshal(out.Bytes(), v); err != nil {
		t.Fatalf("decoding %v: %v\n%s", args, err, out.String())
	}
}

func TestConceptHelpText_DocumentsSuggest(t *testing.T) {
	if !strings.Contains(ConceptHelpText, "suggest") {
		t.Error("ConceptHelpText does not document suggest")
	}
}

func TestConceptHelpText_DocumentsFuzzyClustering(t *testing.T) {
	if !strings.Contains(ConceptHelpText, "near-existing (excluded from candidates):") {
		t.Error("ConceptHelpText does not document the near-existing exclusion section")
	}
}

// ─── concept delete (requested 2026-09-23, DR-0038) ─────────────────────────

func runConcept(t *testing.T, kb *knowledge.KnowledgeBase, jsonOut bool, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := cmdConcept(kb, nil, jsonOut, args, &out)
	return out.String(), err
}

func conceptExists(t *testing.T, kb *knowledge.KnowledgeBase, name string) bool {
	t.Helper()
	cs, err := kb.Concepts()
	if err != nil {
		t.Fatalf("Concepts: %v", err)
	}
	for _, c := range cs {
		if c.Name == name {
			return true
		}
	}
	return false
}

// linkedConcept makes concept name linked to a project and, when withRecord, a
// decision record too (a record link is what re-ingesting would recreate).
func linkedConcept(t *testing.T, kb *knowledge.KnowledgeBase, name string, withRecord bool) {
	t.Helper()
	cid, err := kb.AddConcept(name, "")
	if err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	pid, _ := kb.AddProject("alpha", "")
	if err := kb.LinkProjectConcept(pid, cid); err != nil {
		t.Fatalf("LinkProjectConcept: %v", err)
	}
	if withRecord {
		rid, err := kb.AddRecord(knowledge.Record{
			RecordID: "0001", ProjectID: pid, Scope: "project", Path: "decisions/0001-x.md",
			Title: "t", Date: "2026-09-23", Status: "accepted", Kind: "decision", Body: "b", Checksum: "c1",
		})
		if err != nil {
			t.Fatalf("AddRecord: %v", err)
		}
		if err := kb.LinkRecordConcept(rid, cid); err != nil {
			t.Fatalf("LinkRecordConcept: %v", err)
		}
	}
}

func TestCmdConcept_DeleteRemovesAnUnlinkedConceptAndSaysTheSyncCaveat(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("junk", "")
	out, err := runConcept(t, kb, false, "delete", "junk")
	if err != nil {
		t.Fatalf("concept delete: %v", err)
	}
	if conceptExists(t, kb, "junk") {
		t.Error("concept still exists")
	}
	if !strings.Contains(out, `concept "junk" deleted`) {
		t.Errorf("output = %q, want the deletion confirmed", out)
	}
	if !strings.Contains(out, "merge") || !strings.Contains(out, "import") {
		t.Errorf("output = %q, want the note that merge/import from another database brings it back", out)
	}
}

func TestCmdConcept_DeleteRefusesALinkedConceptWithoutForce(t *testing.T) {
	kb := openTestKB(t)
	linkedConcept(t, kb, "junk", true)
	_, err := runConcept(t, kb, false, "delete", "junk")
	if err == nil {
		t.Fatal("concept delete of a linked concept = nil error, want a refusal")
	}
	for _, want := range []string{"1 project(s)", "1 record(s)", "--force", "nothing was deleted"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
	}
	if !conceptExists(t, kb, "junk") {
		t.Error("the concept was deleted despite the refusal")
	}
}

func TestCmdConcept_DeleteForceUnlinksAndWarnsThatReingestRecreatesIt(t *testing.T) {
	kb := openTestKB(t)
	linkedConcept(t, kb, "junk", true)
	out, err := runConcept(t, kb, false, "delete", "junk", "--force")
	if err != nil {
		t.Fatalf("concept delete --force: %v", err)
	}
	if conceptExists(t, kb, "junk") {
		t.Error("concept still exists")
	}
	for _, want := range []string{"deleted", "1 project(s)", "1 record(s)", "ingest"} {
		if !strings.Contains(out, want) {
			t.Errorf("output = %q, want it to contain %q", out, want)
		}
	}
}

func TestCmdConcept_DeleteForceOnAProjectOnlyConceptDoesNotWarnAboutReingest(t *testing.T) {
	kb := openTestKB(t)
	linkedConcept(t, kb, "junk", false)
	out, err := runConcept(t, kb, false, "delete", "--force", "junk")
	if err != nil {
		t.Fatalf("concept delete --force: %v", err)
	}
	if strings.Contains(out, "recreates") {
		t.Errorf("output = %q, want no re-ingest warning: only a project links it, and nothing re-links that", out)
	}
}

func TestCmdConcept_DeleteDryRunChangesNothingEvenForALinkedConcept(t *testing.T) {
	kb := openTestKB(t)
	linkedConcept(t, kb, "junk", true)
	out, err := runConcept(t, kb, false, "delete", "junk", "--dry-run")
	if err != nil {
		t.Fatalf("concept delete --dry-run: %v", err)
	}
	if !conceptExists(t, kb, "junk") {
		t.Error("a dry run deleted the concept")
	}
	if !strings.Contains(out, "would delete") || !strings.Contains(out, "1 record(s)") {
		t.Errorf("output = %q, want a preview naming the links", out)
	}
	if strings.Contains(out, `deleted`) && !strings.Contains(out, "would delete") {
		t.Errorf("output = %q, must not claim a deletion", out)
	}
}

func TestCmdConcept_DeleteJSONShape(t *testing.T) {
	kb := openTestKB(t)
	linkedConcept(t, kb, "junk", true)
	out, err := runConcept(t, kb, true, "delete", "junk", "--force")
	if err != nil {
		t.Fatalf("concept delete --force --json: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("output not valid JSON: %v (%q)", err, out)
	}
	links, _ := raw["links"].(map[string]any)
	if raw["concept"] != "junk" || raw["deleted"] != true || raw["dry_run"] != false {
		t.Errorf("json = %v, want concept junk, deleted true, dry_run false", raw)
	}
	if links["projects"] != float64(1) || links["records"] != float64(1) || links["observations"] != float64(0) || links["document_sections"] != float64(0) {
		t.Errorf("links = %v, want projects 1, records 1, observations 0, document_sections 0", links)
	}
	if notes, _ := raw["notes"].([]any); len(notes) == 0 {
		t.Errorf("json = %v, want the caveats carried as notes", raw)
	}
}

func TestCmdConcept_DeleteJSONDryRunSaysNothingWasDeleted(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("junk", "")
	out, _ := runConcept(t, kb, true, "delete", "junk", "--dry-run")
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("output not valid JSON: %v (%q)", err, out)
	}
	if raw["deleted"] != false || raw["dry_run"] != true {
		t.Errorf("json = %v, want deleted false and dry_run true", raw)
	}
	if !conceptExists(t, kb, "junk") {
		t.Error("a dry run deleted the concept")
	}
}

func TestCmdConcept_DeleteUsageAndUnknownConceptErrors(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("keeper", "")
	if _, err := runConcept(t, kb, false, "delete"); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Errorf("no NAME: err = %v, want a usage error", err)
	}
	if _, err := runConcept(t, kb, false, "delete", "a", "b"); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Errorf("two names: err = %v, want a usage error", err)
	}
	if _, err := runConcept(t, kb, false, "delete", "--nonesuchflag", "x"); err == nil {
		t.Error("unknown flag: nil error, want one")
	}
	if _, err := runConcept(t, kb, false, "delete", "nonesuch"); err == nil || !strings.Contains(err.Error(), "nonesuch") {
		t.Errorf("unknown concept: err = %v, want it to name the concept", err)
	}
	if !conceptExists(t, kb, "keeper") {
		t.Error("an unrelated concept was deleted")
	}
}

func TestCmdConcept_DeleteAcceptsANameThatLooksLikeAFlagAfterDoubleDash(t *testing.T) {
	// The concepts most worth deleting are junk ones, and junk names look like
	// flags: "---", "-x". `--` ends flag parsing, as everywhere else.
	kb := openTestKB(t)
	kb.AddConcept("---", "")
	kb.AddConcept("--odd", "")
	if _, err := runConcept(t, kb, false, "delete", "--", "---"); err != nil {
		t.Fatalf("delete -- ---: %v", err)
	}
	if _, err := runConcept(t, kb, false, "delete", "--dry-run", "--", "--odd"); err != nil {
		t.Fatalf("delete --dry-run -- --odd: %v", err)
	}
	if conceptExists(t, kb, "---") {
		t.Error("--- still exists")
	}
	if !conceptExists(t, kb, "--odd") {
		t.Error("--odd was deleted by a dry run")
	}
}

func TestConceptHelpText_DocumentsDelete(t *testing.T) {
	var out bytes.Buffer
	printHelp(&out, "concept")
	page := out.String()
	for _, want := range []string{"concept delete NAME", "--force", "--dry-run", "merge", "import", "ingest"} {
		if !strings.Contains(page, want) {
			t.Errorf("kb help concept does not mention %q", want)
		}
	}
}

func TestCmdConcept_UsageListsDelete(t *testing.T) {
	kb := openTestKB(t)
	if _, err := runConcept(t, kb, false); err == nil || !strings.Contains(err.Error(), "delete") {
		t.Errorf("err = %v, want the usage line to list delete", err)
	}
}
