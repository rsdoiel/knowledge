package knowledge

import "testing"

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
