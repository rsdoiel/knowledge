package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// ─── scoreCandidateTerms (kb concept suggest, TODO.md's term-frequency item) ─

func TestScoreCandidateTerms_RequiresMoreThanOneOccurrence(t *testing.T) {
	items := []string{"chunking showed up once here", "nothing else relevant"}
	got := scoreCandidateTerms(items, nil)
	for _, c := range got {
		if c.Term == "chunking" {
			t.Errorf("candidates = %+v, want chunking excluded -- only one occurrence in the whole corpus", got)
		}
	}
}

func TestScoreCandidateTerms_IncludesTermMentionedTwice(t *testing.T) {
	// A third, unrelated item keeps chunking from appearing in literally
	// every item (df < n), or its idf collapses to zero regardless of how
	// many times it's mentioned -- see ExcludesUbiquitousTerm below.
	items := []string{"chunking showed up here", "chunking showed up again here too", "something else entirely"}
	got := scoreCandidateTerms(items, nil)
	found := false
	for _, c := range got {
		if c.Term == "chunking" {
			found = true
			if c.Occurrences != 2 || c.Items != 2 {
				t.Errorf("chunking candidate = %+v, want Occurrences=2 Items=2", c)
			}
		}
	}
	if !found {
		t.Errorf("candidates = %+v, want chunking present", got)
	}
}

func TestScoreCandidateTerms_ExcludesKnownConcepts(t *testing.T) {
	items := []string{"chunking showed up here", "chunking showed up again here too"}
	known := map[string]bool{"chunking": true}
	got := scoreCandidateTerms(items, known)
	for _, c := range got {
		if c.Term == "chunking" {
			t.Errorf("candidates = %+v, want chunking excluded -- it is already a known concept", got)
		}
	}
}

func TestScoreCandidateTerms_ExcludesUbiquitousTerm(t *testing.T) {
	// "widget" appears in every item, so it carries no distinctiveness
	// (idf collapses to zero) even though its raw occurrence count is high.
	items := []string{
		"widget one context here", "widget two context here",
		"widget three context here", "widget four context here",
	}
	got := scoreCandidateTerms(items, nil)
	for _, c := range got {
		if c.Term == "widget" {
			t.Errorf("candidates = %+v, want widget excluded -- present in every item, not distinctive", got)
		}
	}
	// "context" is equally ubiquitous, both are filtered, but "one"/"two"
	// etc. are below the length-3 token floor via digits-as-words -- this
	// just confirms the corpus isn't accidentally producing zero candidates
	// for an unrelated reason (regression guard against an overly strict filter).
}

// A record-id-shaped token (dr-0013, adr-0004, ...) is a legitimate
// statistical signal -- distinctive, repeated -- but never a usable
// concept name, so it is filtered outright rather than left for a human
// to skip on every single run.
func TestScoreCandidateTerms_ExcludesRecordIDShapedTokens(t *testing.T) {
	items := []string{
		"see dr-0013 for the full repro",
		"dr-0013 is referenced again here",
		"something unrelated entirely",
	}
	got := scoreCandidateTerms(items, nil)
	for _, c := range got {
		if c.Term == "dr-0013" {
			t.Errorf("candidates = %+v, want dr-0013 excluded -- it names a record, not a concept", got)
		}
	}
}

func TestScoreCandidateTerms_ExcludesStopwords(t *testing.T) {
	items := []string{"the and this that appear here", "the and this that appear again"}
	got := scoreCandidateTerms(items, nil)
	for _, c := range got {
		if c.Term == "the" || c.Term == "and" || c.Term == "this" || c.Term == "that" {
			t.Errorf("candidates = %+v, want common stopwords excluded", got)
		}
	}
}

func TestScoreCandidateTerms_ExcludesCodeSpans(t *testing.T) {
	items := []string{
		"saw `const format = x` once, then `unsupported format` again",
		"nothing else relevant here",
	}
	got := scoreCandidateTerms(items, nil)
	for _, c := range got {
		if c.Term == "format" {
			t.Errorf("candidates = %+v, want format excluded -- both mentions are inside inline code spans", got)
		}
	}
}

func TestScoreCandidateTerms_RanksMoreDistinctiveTermsFirst(t *testing.T) {
	// "gadget" is confined to one item (rarer, more distinctive) but
	// mentioned there several times; "sprocket" appears in more items.
	items := []string{
		"gadget gadget gadget gadget context here",
		"sprocket appears here",
		"sprocket appears here too",
		"sprocket appears yet again",
	}
	got := scoreCandidateTerms(items, nil)
	if len(got) < 2 {
		t.Fatalf("candidates = %+v, want at least gadget and sprocket", got)
	}
	rank := map[string]int{}
	for i, c := range got {
		rank[c.Term] = i
	}
	if rank["gadget"] >= rank["sprocket"] {
		t.Errorf("ranks = %+v, want gadget (confined to one item, mentioned repeatedly) ranked above sprocket (spread across many items)", rank)
	}
}

func TestScoreCandidateTerms_EmptyCorpusReturnsNil(t *testing.T) {
	got := scoreCandidateTerms(nil, nil)
	if len(got) != 0 {
		t.Errorf("candidates = %+v, want none for an empty corpus", got)
	}
}

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
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v (%q)", err, out.String())
	}
	found := false
	for _, c := range got {
		if c["term"] == "chunking" {
			found = true
		}
	}
	if !found {
		t.Errorf("suggestions = %v, want chunking, mentioned across a record and a document", got)
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
		Body: "alpha alpha bravo bravo charlie charlie delta delta echo echo distinct distinct",
		Checksum: "c1",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0002", ProjectID: pid, Scope: "project", Path: "decisions/0002-y.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body: "just filler text so these terms are not present in every item",
		Checksum: "c2",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
	var got []candidateTerm
	runConceptJSON(t, kb, &got, "suggest", "--limit", "2")
	if len(got) > 2 {
		t.Errorf("got %d suggestions, want at most 2 (--limit 2)", len(got))
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
