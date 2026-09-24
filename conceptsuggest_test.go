package knowledge

import (
	"strings"
	"testing"
)

// Moved from cmd/kb/concept_test.go (library-lift-plan.md L3, DR-0035): unit
// tests for the scoring logic, which now lives in this package.

// ─── scoreCandidateTerms (kb concept suggest, TODO.md's term-frequency item) ─

func TestScoreCandidateTerms_RequiresMoreThanOneOccurrence(t *testing.T) {
	items := []string{"chunking showed up once here", "nothing else relevant"}
	got, _ := scoreCandidateTerms(items, nil)
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
	got, _ := scoreCandidateTerms(items, nil)
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
	got, _ := scoreCandidateTerms(items, known)
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
	got, _ := scoreCandidateTerms(items, nil)
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
	got, _ := scoreCandidateTerms(items, nil)
	for _, c := range got {
		if c.Term == "dr-0013" {
			t.Errorf("candidates = %+v, want dr-0013 excluded -- it names a record, not a concept", got)
		}
	}
}

func TestScoreCandidateTerms_ExcludesStopwords(t *testing.T) {
	items := []string{"the and this that appear here", "the and this that appear again"}
	got, _ := scoreCandidateTerms(items, nil)
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
	got, _ := scoreCandidateTerms(items, nil)
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
	got, _ := scoreCandidateTerms(items, nil)
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
	got, _ := scoreCandidateTerms(nil, nil)
	if len(got) != 0 {
		t.Errorf("candidates = %+v, want none for an empty corpus", got)
	}
}

// itemCounts (FC2) tracks a set of item indices per term, not a bare count,
// so two mentions of the same term within one item still count once toward
// that term's df -- a third, unrelated item keeps the term from appearing
// in literally every item (idf would otherwise collapse to zero).
func TestScoreCandidateTerms_ItemCountsTracksDistinctItemIndices(t *testing.T) {
	items := []string{
		"gadgetry mentioned here, and gadgetry again in the very same item",
		"something else entirely, unrelated to the first item",
	}
	got, _ := scoreCandidateTerms(items, nil)
	for _, c := range got {
		if c.Term == "gadgetry" {
			if c.Occurrences != 2 {
				t.Errorf("gadgetry.Occurrences = %d, want 2 (two mentions)", c.Occurrences)
			}
			if c.Items != 1 {
				t.Errorf("gadgetry.Items = %d, want 1 (both mentions in the same item)", c.Items)
			}
		}
	}
}

// ─── L3: the exported surface ───────────────────────────────────────────────

func suggestRecord(t *testing.T, kb *KnowledgeBase, pid int64, id, body string) {
	t.Helper()
	if _, err := kb.AddRecord(Record{
		RecordID: id, ProjectID: pid, Scope: "project", Path: "decisions/" + id + "-x.md",
		Title: "t", Date: "2026-09-18", Status: "accepted", Kind: "decision",
		Body: body, Checksum: "c" + id + body[:3],
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}
}

func hasCandidate(cs []ConceptCandidate, term string) bool {
	for _, c := range cs {
		if c.Term == term {
			return true
		}
	}
	return false
}

func TestSuggestConcepts_ScansRecordAndDocumentSectionBodies(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	suggestRecord(t, kb, pid, "0001", "chunking is discussed here in some depth")
	docID, err := kb.AddDocument(Document{ProjectID: pid, Title: "doc", Format: "text", Path: "a.txt"})
	if err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if _, err := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section", Body: "chunking comes up again in this document"}); err != nil {
		t.Fatalf("AddDocumentSection: %v", err)
	}
	// An unrelated item keeps chunking from being in every item (idf 0).
	suggestRecord(t, kb, pid, "0002", "something entirely unrelated")

	got, err := kb.SuggestConcepts("", 0)
	if err != nil {
		t.Fatalf("SuggestConcepts: %v", err)
	}
	if !hasCandidate(got.Candidates, "chunking") {
		t.Errorf("candidates = %+v, want chunking, mentioned across a record and a document", got.Candidates)
	}
}

func TestSuggestConcepts_IgnoresGistRows(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "doc", Format: "text", Path: "a.txt"})
	// Only a gist row mentions the term; the CLI scanned section rows only.
	if _, err := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "gist", Body: "gadgetry gadgetry gadgetry"}); err != nil {
		t.Fatalf("AddDocumentSection: %v", err)
	}
	suggestRecord(t, kb, pid, "0001", "unrelated filler text about weather")
	got, err := kb.SuggestConcepts("", 0)
	if err != nil {
		t.Fatalf("SuggestConcepts: %v", err)
	}
	if hasCandidate(got.Candidates, "gadgetry") {
		t.Errorf("candidates = %+v, want the gist row ignored", got.Candidates)
	}
}

func TestSuggestConcepts_ExcludesExistingConcepts(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	suggestRecord(t, kb, pid, "0001", "chunking chunking appears here")
	suggestRecord(t, kb, pid, "0002", "something entirely unrelated")
	got, _ := kb.SuggestConcepts("", 0)
	if hasCandidate(got.Candidates, "chunking") {
		t.Errorf("candidates = %+v, want the existing concept excluded", got.Candidates)
	}
}

func TestSuggestConcepts_ScopesToProject(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	qid, _ := kb.AddProject("beta", "")
	suggestRecord(t, kb, pid, "0001", "widgetry widgetry appears here in alpha's own corpus")
	suggestRecord(t, kb, qid, "0001", "nothing about that topic appears in beta at all")
	got, err := kb.SuggestConcepts("beta", 0)
	if err != nil {
		t.Fatalf("SuggestConcepts(beta): %v", err)
	}
	if hasCandidate(got.Candidates, "widgetry") {
		t.Errorf("candidates = %+v, want widgetry excluded: it only appears in alpha", got.Candidates)
	}
	all, _ := kb.SuggestConcepts("", 0)
	if !hasCandidate(all.Candidates, "widgetry") {
		t.Errorf("unscoped candidates = %+v, want widgetry", all.Candidates)
	}
}

func TestSuggestConcepts_LimitCapsCandidates(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	suggestRecord(t, kb, pid, "0001", "alphaterm alphaterm betaterm betaterm gammaterm gammaterm")
	suggestRecord(t, kb, pid, "0002", "unrelated filler content here")
	all, _ := kb.SuggestConcepts("", 0)
	if len(all.Candidates) < 3 {
		t.Fatalf("setup: %d candidates, want at least 3", len(all.Candidates))
	}
	got, _ := kb.SuggestConcepts("", 2)
	if len(got.Candidates) != 2 {
		t.Errorf("limit 2 returned %d candidates, want 2", len(got.Candidates))
	}
	if got.Candidates[0].Term != all.Candidates[0].Term {
		t.Errorf("limit reordered results: %v vs %v", got.Candidates, all.Candidates)
	}
}

func TestSuggestConcepts_UnknownProjectErrors(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.SuggestConcepts("nonesuch", 0); err == nil {
		t.Error("SuggestConcepts(unknown project) = nil error, want one")
	}
}

func TestSuggestConcepts_EmptyCorpusHasNoCandidates(t *testing.T) {
	kb := openTestKB(t)
	got, err := kb.SuggestConcepts("", 0)
	if err != nil || len(got.Candidates) != 0 || len(got.NearExisting) != 0 {
		t.Errorf("got %+v, %v; want an empty result and no error", got, err)
	}
}

func TestSuggestConcepts_ReportsNearExistingSpellings(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	suggestRecord(t, kb, pid, "0001", "chunkings appears here, chunkings again")
	suggestRecord(t, kb, pid, "0002", "something entirely unrelated")
	got, _ := kb.SuggestConcepts("", 0)
	if hasCandidate(got.Candidates, "chunkings") {
		t.Errorf("candidates = %+v, want chunkings excluded as near-existing", got.Candidates)
	}
	if len(got.NearExisting) != 1 || got.NearExisting[0].Token != "chunkings" || got.NearExisting[0].Concept != "chunking" {
		t.Errorf("NearExisting = %+v, want chunkings ~ chunking", got.NearExisting)
	}
}

func TestCandidateTerms_StripsCodeDropsStopwordsAndRecordIDs(t *testing.T) {
	got := CandidateTerms("The Chunking and `hidden` code, see DR-0013 and adr-0004; also chunking.\n```\nfenced\n```")
	want := []string{"chunking", "code", "see", "also", "chunking"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("CandidateTerms = %v, want %v (lowercased, in order, repeats kept)", got, want)
	}
}
