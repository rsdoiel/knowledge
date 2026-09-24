package knowledge

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

// ─── W1: RecallByConceptNames ────────────────────────────────────────────────

func TestRecallByConceptNames_ReturnsLinkedObservation(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	oid, _ := kb.AddObservation(pid, "note", "an observation")
	cid, _ := kb.AddConcept("Foo", "")
	if err := kb.LinkObservationConcept(oid, cid); err != nil {
		t.Fatalf("LinkObservationConcept: %v", err)
	}

	matches, err := kb.RecallByConceptNames([]string{"Foo"}, 10)
	if err != nil {
		t.Fatalf("RecallByConceptNames: %v", err)
	}
	if len(matches) != 1 || matches[0].SourceType != "observation" || matches[0].ID != oid {
		t.Errorf("matches = %+v, want one observation match with id %d", matches, oid)
	}
}

func TestRecallByConceptNames_ReturnsLinkedRecord(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	recID := seedTestRecord(t, kb, pid, "0001")
	cid, _ := kb.AddConcept("Foo", "")
	if err := kb.LinkRecordConcept(recID, cid); err != nil {
		t.Fatalf("LinkRecordConcept: %v", err)
	}

	matches, err := kb.RecallByConceptNames([]string{"Foo"}, 10)
	if err != nil {
		t.Fatalf("RecallByConceptNames: %v", err)
	}
	if len(matches) != 1 || matches[0].SourceType != "record" || matches[0].ID != recID {
		t.Errorf("matches = %+v, want one record match with id %d", matches, recID)
	}
}

func TestRecallByConceptNames_MergesAndRanksAcrossBothTypes(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	oid, _ := kb.AddObservation(pid, "note", "single-match observation")
	recID := seedTestRecord(t, kb, pid, "0001")
	c1, _ := kb.AddConcept("Foo", "")
	c2, _ := kb.AddConcept("Bar", "")
	if err := kb.LinkObservationConcept(oid, c1); err != nil {
		t.Fatalf("LinkObservationConcept: %v", err)
	}
	if err := kb.LinkRecordConcept(recID, c1); err != nil {
		t.Fatalf("LinkRecordConcept c1: %v", err)
	}
	if err := kb.LinkRecordConcept(recID, c2); err != nil {
		t.Fatalf("LinkRecordConcept c2: %v", err)
	}

	matches, err := kb.RecallByConceptNames([]string{"Foo", "Bar"}, 10)
	if err != nil {
		t.Fatalf("RecallByConceptNames: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("matches = %+v, want 2", matches)
	}
	if matches[0].SourceType != "record" || matches[0].MatchCount != 2 {
		t.Errorf("matches[0] = %+v, want the 2-concept record ranked first", matches[0])
	}
	if matches[1].SourceType != "observation" || matches[1].MatchCount != 1 {
		t.Errorf("matches[1] = %+v, want the 1-concept observation ranked second", matches[1])
	}
}

func TestRecallByConceptNames_TiesBreakByRecency(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	older, _ := kb.AddObservation(pid, "note", "older")
	newer, _ := kb.AddObservation(pid, "note", "newer")
	cid, _ := kb.AddConcept("Foo", "")
	if err := kb.LinkObservationConcept(older, cid); err != nil {
		t.Fatalf("LinkObservationConcept older: %v", err)
	}
	if err := kb.LinkObservationConcept(newer, cid); err != nil {
		t.Fatalf("LinkObservationConcept newer: %v", err)
	}
	if _, err := kb.db.Exec(`UPDATE observations SET created_at = ? WHERE id = ?`, "2020-01-01 00:00:00", older); err != nil {
		t.Fatalf("backdate older: %v", err)
	}
	if _, err := kb.db.Exec(`UPDATE observations SET created_at = ? WHERE id = ?`, "2025-01-01 00:00:00", newer); err != nil {
		t.Fatalf("backdate newer: %v", err)
	}

	matches, err := kb.RecallByConceptNames([]string{"Foo"}, 10)
	if err != nil {
		t.Fatalf("RecallByConceptNames: %v", err)
	}
	if len(matches) != 2 || matches[0].ID != newer || matches[1].ID != older {
		t.Errorf("matches = %+v, want newer (%d) before older (%d)", matches, newer, older)
	}
}

func TestRecallByConceptNames_UnmatchedNameIsSkippedNotError(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	oid, _ := kb.AddObservation(pid, "note", "an observation")
	cid, _ := kb.AddConcept("Foo", "")
	if err := kb.LinkObservationConcept(oid, cid); err != nil {
		t.Fatalf("LinkObservationConcept: %v", err)
	}

	matches, err := kb.RecallByConceptNames([]string{"Foo", "DoesNotExist"}, 10)
	if err != nil {
		t.Fatalf("RecallByConceptNames: %v", err)
	}
	if len(matches) != 1 || matches[0].ID != oid {
		t.Errorf("matches = %+v, want the one match on Foo, unmatched name skipped", matches)
	}
}

func TestRecallByConceptNames_DoesNotCreateConcepts(t *testing.T) {
	kb := openTestKB(t)
	before, err := kb.Concepts()
	if err != nil {
		t.Fatalf("Concepts (before): %v", err)
	}

	if _, err := kb.RecallByConceptNames([]string{"NeverSeenBefore"}, 10); err != nil {
		t.Fatalf("RecallByConceptNames: %v", err)
	}

	after, err := kb.Concepts()
	if err != nil {
		t.Fatalf("Concepts (after): %v", err)
	}
	if len(after) != len(before) {
		t.Errorf("Concepts() changed from %+v to %+v -- RecallByConceptNames must never create a concept", before, after)
	}
}

func TestRecallByConceptNames_DedupesCaseVariantInputNames(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	oid, _ := kb.AddObservation(pid, "note", "an observation")
	cid, _ := kb.AddConcept("Foo", "")
	if err := kb.LinkObservationConcept(oid, cid); err != nil {
		t.Fatalf("LinkObservationConcept: %v", err)
	}

	matches, err := kb.RecallByConceptNames([]string{"Foo", "foo", "FOO"}, 10)
	if err != nil {
		t.Fatalf("RecallByConceptNames: %v", err)
	}
	if len(matches) != 1 || matches[0].MatchCount != 1 {
		t.Errorf("matches = %+v, want one match with MatchCount=1 (case-variant names deduped to one concept)", matches)
	}
}

func TestRecallByConceptNames_NotProjectScoped(t *testing.T) {
	kb := openTestKB(t)
	p1, _ := kb.AddProject("alpha", "")
	p2, _ := kb.AddProject("beta", "")
	o1, _ := kb.AddObservation(p1, "note", "in alpha")
	o2, _ := kb.AddObservation(p2, "note", "in beta")
	cid, _ := kb.AddConcept("Foo", "")
	if err := kb.LinkObservationConcept(o1, cid); err != nil {
		t.Fatalf("LinkObservationConcept o1: %v", err)
	}
	if err := kb.LinkObservationConcept(o2, cid); err != nil {
		t.Fatalf("LinkObservationConcept o2: %v", err)
	}

	matches, err := kb.RecallByConceptNames([]string{"Foo"}, 10)
	if err != nil {
		t.Fatalf("RecallByConceptNames: %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("matches = %+v, want both cross-project observations returned", matches)
	}
}

func TestRecallByConceptNames_RespectsLimit(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	cid, _ := kb.AddConcept("Foo", "")
	for i := 0; i < 3; i++ {
		oid, _ := kb.AddObservation(pid, "note", "obs")
		if err := kb.LinkObservationConcept(oid, cid); err != nil {
			t.Fatalf("LinkObservationConcept: %v", err)
		}
	}

	matches, err := kb.RecallByConceptNames([]string{"Foo"}, 2)
	if err != nil {
		t.Fatalf("RecallByConceptNames: %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("matches = %+v, want exactly 2 (limit respected)", matches)
	}
}

func TestRecallByConceptNames_EmptyNamesReturnsNilNotError(t *testing.T) {
	kb := openTestKB(t)
	matches, err := kb.RecallByConceptNames(nil, 10)
	if err != nil {
		t.Fatalf("RecallByConceptNames: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("matches = %+v, want none for empty input", matches)
	}
}

// ─── W2: MatchConceptNames ───────────────────────────────────────────────────

func TestMatchConceptNames_FindsWholeWordCaseInsensitive(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("Foo", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	names, err := kb.MatchConceptNames("we talked about foo yesterday")
	if err != nil {
		t.Fatalf("MatchConceptNames: %v", err)
	}
	if len(names) != 1 || names[0] != "Foo" {
		t.Errorf("names = %v, want [Foo]", names)
	}
}

func TestMatchConceptNames_DoesNotFalsePositiveOnSubstring(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("RAG", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	names, err := kb.MatchConceptNames("we need more storage")
	if err != nil {
		t.Fatalf("MatchConceptNames: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("names = %v, want none -- RAG must not substring-match inside storage", names)
	}
}

func TestMatchConceptNames_NoMatchesReturnsEmpty(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("Foo", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	names, err := kb.MatchConceptNames("nothing relevant here")
	if err != nil {
		t.Fatalf("MatchConceptNames: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("names = %v, want none", names)
	}
}

func TestMatchConceptNames_MultipleConceptsInOneText(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("Foo", ""); err != nil {
		t.Fatalf("AddConcept Foo: %v", err)
	}
	if _, err := kb.AddConcept("Bar", ""); err != nil {
		t.Fatalf("AddConcept Bar: %v", err)
	}
	names, err := kb.MatchConceptNames("foo and bar both came up")
	if err != nil {
		t.Fatalf("MatchConceptNames: %v", err)
	}
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if len(names) != 2 || !found["Foo"] || !found["Bar"] {
		t.Errorf("names = %v, want Foo and Bar", names)
	}
}

func TestMatchConceptNames_MatchesAtWordBoundaryPunctuation(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("Foo", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	names, err := kb.MatchConceptNames("see [[Foo]] for background.")
	if err != nil {
		t.Fatalf("MatchConceptNames: %v", err)
	}
	if len(names) != 1 || names[0] != "Foo" {
		t.Errorf("names = %v, want [Foo] despite surrounding [[ ]] and a trailing period", names)
	}
}

// ─── MatchConceptNameCounts (MADR density-linking, TODO.md) ────────────────

func TestMatchConceptNameCounts_CountsOccurrences(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("Foo", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	counts, err := kb.MatchConceptNameCounts("foo showed up, then foo again, and foo a third time")
	if err != nil {
		t.Fatalf("MatchConceptNameCounts: %v", err)
	}
	if counts["Foo"] != 3 {
		t.Errorf("counts[Foo] = %d, want 3", counts["Foo"])
	}
}

func TestMatchConceptNameCounts_AbsentConceptIsNotInMap(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("Foo", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	counts, err := kb.MatchConceptNameCounts("nothing relevant here")
	if err != nil {
		t.Fatalf("MatchConceptNameCounts: %v", err)
	}
	if _, ok := counts["Foo"]; ok {
		t.Errorf("counts = %v, want Foo absent, not zero", counts)
	}
}

func TestMatchConceptNameCounts_DoesNotFalsePositiveOnSubstring(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("RAG", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	counts, err := kb.MatchConceptNameCounts("we need more storage, then even more storage")
	if err != nil {
		t.Fatalf("MatchConceptNameCounts: %v", err)
	}
	if _, ok := counts["RAG"]; ok {
		t.Errorf("counts = %v, want RAG absent -- must not substring-match inside storage", counts)
	}
}

// ─── W6 (narrative-documents-plan.md): widen RecallByConceptNames to documents ──

func TestRecallByConceptNames_ReturnsLinkedDocumentSection(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "A Story", Format: "text", Path: "a.txt"})
	secID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section", Heading: "One"})
	cid, _ := kb.AddConcept("Foo", "")
	if err := kb.LinkDocumentSectionConcept(secID, cid); err != nil {
		t.Fatalf("LinkDocumentSectionConcept: %v", err)
	}

	matches, err := kb.RecallByConceptNames([]string{"Foo"}, 10)
	if err != nil {
		t.Fatalf("RecallByConceptNames: %v", err)
	}
	if len(matches) != 1 || matches[0].SourceType != "document_section" || matches[0].ID != secID {
		t.Errorf("matches = %+v, want one document_section match with id %d", matches, secID)
	}
}

func TestRecallByConceptNames_UnreviewedDocumentMatchHasEmptyBodyButSetStatus(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "A Story", Format: "text", Path: "a.txt"})
	secID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "gist"})
	if err := kb.DraftDocumentSummary(secID, "a draft nobody has reviewed yet", "human", nil); err != nil {
		t.Fatalf("DraftDocumentSummary: %v", err)
	}
	cid, _ := kb.AddConcept("Foo", "")
	if err := kb.LinkDocumentSectionConcept(secID, cid); err != nil {
		t.Fatalf("LinkDocumentSectionConcept: %v", err)
	}

	matches, err := kb.RecallByConceptNames([]string{"Foo"}, 10)
	if err != nil {
		t.Fatalf("RecallByConceptNames: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %+v, want 1", matches)
	}
	if matches[0].SourceType != "document_gist" {
		t.Errorf("SourceType = %q, want document_gist", matches[0].SourceType)
	}
	if matches[0].SummaryStatus != "drafted" {
		t.Errorf("SummaryStatus = %q, want drafted", matches[0].SummaryStatus)
	}
	if matches[0].Body != "" {
		t.Errorf("Body = %q, want empty -- a drafted (unreviewed) summary must not be surfaced as trustworthy content", matches[0].Body)
	}
}

func TestRecallByConceptNames_ReviewedDocumentMatchIncludesBody(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "A Story", Format: "text", Path: "a.txt"})
	secID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "gist"})
	if err := kb.DraftDocumentSummary(secID, "a reviewed summary", "human", nil); err != nil {
		t.Fatalf("DraftDocumentSummary: %v", err)
	}
	if err := kb.PromoteDocumentSummary(secID); err != nil {
		t.Fatalf("PromoteDocumentSummary: %v", err)
	}
	cid, _ := kb.AddConcept("Foo", "")
	if err := kb.LinkDocumentSectionConcept(secID, cid); err != nil {
		t.Fatalf("LinkDocumentSectionConcept: %v", err)
	}

	matches, err := kb.RecallByConceptNames([]string{"Foo"}, 10)
	if err != nil {
		t.Fatalf("RecallByConceptNames: %v", err)
	}
	if len(matches) != 1 || matches[0].Body != "a reviewed summary" || matches[0].SummaryStatus != "reviewed" {
		t.Errorf("matches = %+v, want the reviewed summary body included", matches)
	}
}

func TestRecallByConceptNames_MergesAllThreeSourceTypes(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	oid, _ := kb.AddObservation(pid, "note", "an observation")
	recID := seedTestRecord(t, kb, pid, "0001")
	docID, _ := kb.AddDocument(Document{ProjectID: pid, Title: "A Story", Format: "text", Path: "a.txt"})
	secID, _ := kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section"})
	cid, _ := kb.AddConcept("Foo", "")
	if err := kb.LinkObservationConcept(oid, cid); err != nil {
		t.Fatalf("LinkObservationConcept: %v", err)
	}
	if err := kb.LinkRecordConcept(recID, cid); err != nil {
		t.Fatalf("LinkRecordConcept: %v", err)
	}
	if err := kb.LinkDocumentSectionConcept(secID, cid); err != nil {
		t.Fatalf("LinkDocumentSectionConcept: %v", err)
	}

	matches, err := kb.RecallByConceptNames([]string{"Foo"}, 10)
	if err != nil {
		t.Fatalf("RecallByConceptNames: %v", err)
	}
	types := map[string]bool{}
	for _, m := range matches {
		types[m.SourceType] = true
	}
	if len(matches) != 3 || !types["observation"] || !types["record"] || !types["document_section"] {
		t.Errorf("matches = %+v, want one of each: observation, record, document_section", matches)
	}
}

// ─── FuzzyMatchConceptNames (v0.0.11 item 1, `kb document fuzzy-tag`) ──────
//
// Distance is raw Levenshtein distance by default -- this alone already
// satisfies fuzzy-concept-matching-plan.md's own worked examples (plural
// "chunkings" and typo "chunkibg" both land on distance 1 against
// "chunking" with no stemming at all). stripCommonSuffix only kicks in as a
// fallback when raw distance overshoots maxFuzzyDistance, e.g. a short
// concept name against a longer suffixed token -- confirmed by computing
// all three of the design doc's worked pairs before writing this: stemming
// *both* sides (as the design first described) breaks the plural and typo
// cases outright (distance 3, not 1), so only the token side is ever
// stemmed, and only as a fallback.

func TestLevenshteinDistance_IdenticalStringsIsZero(t *testing.T) {
	if d := LevenshteinDistance("chunking", "chunking"); d != 0 {
		t.Errorf("LevenshteinDistance = %d, want 0", d)
	}
}

func TestLevenshteinDistance_SingleCharacterEditIsOne(t *testing.T) {
	cases := []struct{ a, b string }{
		{"chunking", "chunkng"},   // deletion
		{"chunking", "chunkings"}, // insertion
		{"chunking", "chunkibg"},  // substitution
	}
	for _, c := range cases {
		if d := LevenshteinDistance(c.a, c.b); d != 1 {
			t.Errorf("LevenshteinDistance(%q, %q) = %d, want 1", c.a, c.b, d)
		}
	}
}

func TestStripCommonSuffix_StripsLongestMatchingSuffix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"chunkings", "chunking"}, // strips only trailing "s", one strip not iterative
		{"chunked", "chunk"},
		{"chunks", "chunk"},
		{"boxes", "box"},
		{"chunking", "chunk"},
		{"nosuffixhere", "nosuffixhere"},
	}
	for _, c := range cases {
		if got := StripCommonSuffix(c.in); got != c.want {
			t.Errorf("StripCommonSuffix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFuzzyMatchConceptNames_FindsPluralVariant(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	matches, err := kb.FuzzyMatchConceptNames("the chunkings here")
	if err != nil {
		t.Fatalf("FuzzyMatchConceptNames: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %+v, want 1", matches)
	}
	if matches[0].Concept != "chunking" || matches[0].Distance != 1 {
		t.Errorf("matches[0] = %+v, want Concept=chunking Distance=1", matches[0])
	}
}

func TestFuzzyMatchConceptNames_FindsTenseVariant(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunk", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	matches, err := kb.FuzzyMatchConceptNames("we chunked it")
	if err != nil {
		t.Fatalf("FuzzyMatchConceptNames: %v", err)
	}
	if len(matches) != 1 || matches[0].Concept != "chunk" {
		t.Errorf("matches = %+v, want one match on chunk", matches)
	}
}

func TestFuzzyMatchConceptNames_FindsTypo(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	matches, err := kb.FuzzyMatchConceptNames("chunkibg happens")
	if err != nil {
		t.Fatalf("FuzzyMatchConceptNames: %v", err)
	}
	if len(matches) != 1 || matches[0].Distance != 1 {
		t.Errorf("matches = %+v, want one match at distance 1", matches)
	}
}

func TestFuzzyMatchConceptNames_SkipsConceptAlreadyExactlyMatched(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	matches, err := kb.FuzzyMatchConceptNames("chunking is here, chunkings too")
	if err != nil {
		t.Fatalf("FuzzyMatchConceptNames: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("matches = %+v, want none -- the concept is already an exact match elsewhere", matches)
	}
}

func TestFuzzyMatchConceptNames_SkipsUnrelatedWords(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	matches, err := kb.FuzzyMatchConceptNames("something entirely unrelated appears here")
	if err != nil {
		t.Fatalf("FuzzyMatchConceptNames: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("matches = %+v, want none", matches)
	}
}

func TestFuzzyMatchConceptNames_MatchesMultiWordConceptName(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("context window", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	matches, err := kb.FuzzyMatchConceptNames("the contxt windows were large")
	if err != nil {
		t.Fatalf("FuzzyMatchConceptNames: %v", err)
	}
	if len(matches) != 1 || matches[0].Concept != "context window" {
		t.Errorf("matches = %+v, want one match on \"context window\"", matches)
	}
}

func TestFuzzyMatchConceptNames_DoesNotSpanSentenceBoundary(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("context window", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	matches, err := kb.FuzzyMatchConceptNames("we discussed the contxt. windows were opened later.")
	if err != nil {
		t.Fatalf("FuzzyMatchConceptNames: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("matches = %+v, want none -- \"contxt\" and \"windows\" are in different sentences", matches)
	}
}

func TestFuzzyMatchConceptNames_NoMatchesReturnsEmpty(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	matches, err := kb.FuzzyMatchConceptNames("")
	if err != nil {
		t.Fatalf("FuzzyMatchConceptNames: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("matches = %+v, want none for empty text", matches)
	}
}

func TestFuzzyMatchConceptNames_ResultsSortedByPosition(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	if _, err := kb.AddConcept("prompt", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	matches, err := kb.FuzzyMatchConceptNames("we saw promt first, then chunkings later")
	if err != nil {
		t.Fatalf("FuzzyMatchConceptNames: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("matches = %+v, want 2", matches)
	}
	if matches[0].Start > matches[1].Start {
		t.Errorf("matches = %+v, want sorted by Start ascending", matches)
	}
}

// ─── concept names that begin or end in punctuation (TODO.md, found 2026-09-23) ─
//
// The matcher was `(?i)\b` + name + `\b`. \b only fires between a word
// character and a non-word one, so a name like C++ (ends in '+') could never
// match "using C++ for this": the space after the '+' is not a word boundary.
// The rule is now "not glued to a word character on either side", which is
// exactly what \b meant for names that start and end with a letter.

func matchKB(t *testing.T, names ...string) *KnowledgeBase {
	t.Helper()
	kb := openTestKB(t)
	for _, n := range names {
		if _, err := kb.AddConcept(n, ""); err != nil {
			t.Fatalf("AddConcept(%q): %v", n, err)
		}
	}
	return kb
}

func TestMatchConceptNames_FindsNamesThatBeginOrEndInPunctuation(t *testing.T) {
	cases := []struct{ name, text string }{
		{"C++", "we are using C++ for this"},
		{"C++", "C++ at the very start"},
		{"C++", "and at the very end, C++"},
		{"C++", "in parentheses (C++) and punctuation C++."},
		{"C++", "a list: C++, Go"},
		{"F#", "written in F# today"},
		{"C#", "the C# compiler"},
		{".NET", "a .NET application"},
		{".NET", "targets .NET, not Mono"},
		{"c++", "Upper-case C++ still matches a lower-case name"},
	}
	for _, c := range cases {
		kb := matchKB(t, c.name)
		got, err := kb.MatchConceptNames(c.text)
		if err != nil || len(got) != 1 || got[0] != c.name {
			t.Errorf("MatchConceptNames(%q) with concept %q = %v, %v; want [%s]", c.text, c.name, got, err, c.name)
		}
	}
}

func TestMatchConceptNames_PunctuationNamesAreNotMatchedInsideALongerWord(t *testing.T) {
	cases := []struct{ name, text string }{
		{"C++", "ABC++ is not the same word"},
		{"C++", "C++x is not either"},
		{"F#", "MyF# is not F sharp"},
		{".NET", "ASP.NET is a different thing"},
		{".NET", "the .NETwork"},
	}
	for _, c := range cases {
		kb := matchKB(t, c.name)
		if got, _ := kb.MatchConceptNames(c.text); len(got) != 0 {
			t.Errorf("MatchConceptNames(%q) with concept %q = %v, want no match: glued to a word character", c.text, c.name, got)
		}
	}
}

func TestMatchConceptNames_LetterNamesBehaveExactlyAsBefore(t *testing.T) {
	kb := matchKB(t, "foo")
	yes := []string{"foo", "a foo b", "(foo)", "foo.", "foo-bar", "FOO", "x, foo!", "foo\nbar"}
	no := []string{"xfoo", "foox", "foo_bar", "foo9", "9foo", "a_foo"}
	for _, s := range yes {
		if got, _ := kb.MatchConceptNames(s); len(got) != 1 {
			t.Errorf("MatchConceptNames(%q) = %v, want a match", s, got)
		}
	}
	for _, s := range no {
		if got, _ := kb.MatchConceptNames(s); len(got) != 0 {
			t.Errorf("MatchConceptNames(%q) = %v, want no match", s, got)
		}
	}
}

func TestMatchConceptNames_MultiWordAndInteriorPunctuationStillWork(t *testing.T) {
	kb := matchKB(t, "chunking strategy", "Node.js", "llama.cpp")
	got, _ := kb.MatchConceptNames("a chunking strategy for Node.js and llama.cpp")
	if len(got) != 3 {
		t.Errorf("got %v, want all three", got)
	}
}

func TestMatchConceptNameCounts_CountsEveryPunctuationNameMention(t *testing.T) {
	kb := matchKB(t, "C++", "F#")
	got, err := kb.MatchConceptNameCounts("C++ and C++, then C++. Also F# and ABC++ and F#.")
	if err != nil {
		t.Fatalf("MatchConceptNameCounts: %v", err)
	}
	if got["C++"] != 3 || got["F#"] != 2 {
		t.Errorf("counts = %v, want C++:3 (ABC++ excluded) and F#:2", got)
	}
}

func TestMatchConceptNameCounts_AdjacentMentionsAreEachCounted(t *testing.T) {
	// A boundary check that consumed the separator would count "C++ C++" once.
	kb := matchKB(t, "C++")
	got, _ := kb.MatchConceptNameCounts("C++ C++ C++")
	if got["C++"] != 3 {
		t.Errorf("counts = %v, want 3", got)
	}
}

func TestMatchConceptNameCounts_AMatchRejectedForItsNeighbourDoesNotHideALaterOne(t *testing.T) {
	// "a.a" first matches inside "xa.a.a" glued to the x and is rejected; the
	// overlapping "a.a" starting one character later is a valid mention.
	kb := matchKB(t, "a.a")
	got, _ := kb.MatchConceptNameCounts("xa.a.a")
	if got["a.a"] != 1 {
		t.Errorf("counts = %v, want the mention at the later offset found", got)
	}
}

// A differential test against the matcher this replaced: for any name that
// starts and ends with an ASCII word character, conceptNameMatches must find
// exactly what `(?i)\b` + name + `\b` found, over arbitrary text (including
// underscores, digits, hyphens, punctuation, newlines and non-ASCII letters).
func TestConceptNameMatches_AgreesWithTheOldBoundaryRegexForLetterEdgedNames(t *testing.T) {
	rng := rand.New(rand.NewSource(20260923))
	alphabet := []rune("abAB_ 19-.+#(),\né世")
	names := []string{"a", "ab", "a b", "b-a", "a.b", "AB", "a_b", "1a", "b9", "a b a"}
	for i := 0; i < 4000; i++ {
		n := rng.Intn(14)
		var sb strings.Builder
		for j := 0; j < n; j++ {
			sb.WriteRune(alphabet[rng.Intn(len(alphabet))])
		}
		text := sb.String()
		for _, name := range names {
			want := regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(name)+`\b`).FindAllStringIndex(text, -1)
			got := conceptNameMatches(text, name)
			if len(want) != len(got) {
				t.Fatalf("name %q, text %q: old regex found %v, conceptNameMatches found %v", name, text, want, got)
			}
			for k := range want {
				if want[k][0] != got[k][0] || want[k][1] != got[k][1] {
					t.Fatalf("name %q, text %q: old regex found %v, conceptNameMatches found %v", name, text, want, got)
				}
			}
		}
	}
}

// A concept whose name has no letter or digit (a junk concept such as "...",
// minted from a documentation example like [[...]]) must never match. Before
// punctuation-edged names were supported this held by accident, since \b could
// not fire on it; now it has to hold on purpose, or "..." would match every
// ellipsis in every document.
func TestMatchConceptNames_ANameWithNoLetterOrDigitNeverMatches(t *testing.T) {
	for _, name := range []string{"...", "+", "#", "---", "()", "!?"} {
		kb := matchKB(t, name)
		text := "wait... what? a + b # c --- (d) really!?"
		if got, _ := kb.MatchConceptNames(text); len(got) != 0 {
			t.Errorf("concept %q matched %v, want no match: it has no letter or digit", name, got)
		}
		if got, _ := kb.MatchConceptNameCounts(text); len(got) != 0 {
			t.Errorf("concept %q counted %v, want no count", name, got)
		}
	}
}

func TestInsertWikilink_NeverWrapsANameWithNoLetterOrDigit(t *testing.T) {
	if got, ok := insertWikilink("wait ... what", "..."); ok || got != "wait ... what" {
		t.Errorf("got %q, %v; want the text untouched", got, ok)
	}
}
