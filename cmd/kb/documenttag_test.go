package main

import (
	"strings"
	"testing"
)

// ─── insertWikilink (kb document tag, TODO.md's programmatic-corpus-
// improvement item, wikilink-insertion half) ────────────────────────────────

func TestInsertWikilink_WrapsFirstOccurrencePreservingCasing(t *testing.T) {
	got, inserted := insertWikilink("Chunking is discussed here, and chunking again later.", "chunking")
	if !inserted {
		t.Fatal("expected an insertion")
	}
	want := "[[Chunking]] is discussed here, and chunking again later."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInsertWikilink_NoMatchReturnsUnchanged(t *testing.T) {
	got, inserted := insertWikilink("nothing relevant here", "chunking")
	if inserted {
		t.Error("expected no insertion")
	}
	if got != "nothing relevant here" {
		t.Errorf("got %q, want the text unchanged", got)
	}
}

func TestInsertWikilink_SkipsWhenAlreadyWikilinked(t *testing.T) {
	text := "See [[Chunking]] for background. Chunking comes up again here."
	got, inserted := insertWikilink(text, "chunking")
	if inserted {
		t.Error("expected no insertion -- chunking is already wikilinked")
	}
	if got != text {
		t.Errorf("got %q, want the text unchanged", got)
	}
}

func TestInsertWikilink_DoesNotMatchInsideCodeSpan(t *testing.T) {
	got, inserted := insertWikilink("saw `const format = x` here, nothing else about format", "format")
	if !inserted {
		t.Fatal("expected an insertion at the safe, non-code occurrence")
	}
	if strings.Contains(got, "[[format = x]]") || strings.Contains(got, "`[[format]]`") {
		t.Errorf("got %q, want the code span left untouched", got)
	}
	if !strings.Contains(got, "[[format]]") {
		t.Errorf("got %q, want format wrapped at its safe occurrence", got)
	}
}

func TestInsertWikilink_DoesNotMatchInsideFencedCodeBlock(t *testing.T) {
	text := "```\ngit push\ngit pull\n```\nnothing relevant out here"
	got, inserted := insertWikilink(text, "git")
	if inserted {
		t.Errorf("got %q, inserted=%v, want no insertion -- both mentions are inside the fenced block", got, inserted)
	}
}

func TestInsertWikilink_DoesNotMatchInsideExistingWikilink(t *testing.T) {
	// "chunking" is a substring of the existing "[[chunking strategy]]"
	// wikilink -- it must not be nested inside it.
	text := "See [[chunking strategy]] for details."
	got, inserted := insertWikilink(text, "chunking")
	if inserted {
		t.Errorf("got %q, inserted=%v, want no insertion -- the only occurrence is inside an existing wikilink", got, inserted)
	}
}

// Found live, smoke-testing against a real document: a concept name
// appearing in the document's own H1 title got wrapped there, mechanically
// correct but stylistically wrong -- a title should read as prose, not
// carry a wikilink.
func TestInsertWikilink_SkipsFirstH1Heading(t *testing.T) {
	text := "# A Request for comment\n\nBody text with no other mention of the word.\n"
	got, inserted := insertWikilink(text, "request")
	if inserted {
		t.Errorf("got %q, inserted=%v, want no insertion -- the only occurrence is in the document's own title", got, inserted)
	}
}

// A second-level (or deeper) heading is an ordinary section heading, not
// the document's title, and remains eligible.
func TestInsertWikilink_DoesNotSkipH2Heading(t *testing.T) {
	text := "# Title\n\n## A section about chunking\n\nmore chunking below.\n"
	got, inserted := insertWikilink(text, "chunking")
	if !inserted {
		t.Fatal("expected an insertion -- chunking appears in an H2 heading and body, not the H1 title")
	}
	if !strings.Contains(got, "## A section about [[chunking]]") && !strings.Contains(got, "[[chunking]] below") {
		t.Errorf("got %q, want chunking wrapped somewhere outside the H1 title", got)
	}
}

func TestInsertWikilink_SkipsYAMLFrontmatter(t *testing.T) {
	text := "---\ntitle: chunking overview\n---\n\nBody text with no mention of the word.\n"
	got, inserted := insertWikilink(text, "chunking")
	if inserted {
		t.Errorf("got %q, inserted=%v, want no insertion -- the only occurrence is inside frontmatter", got, inserted)
	}
}

func TestInsertWikilink_MatchesMultiWordConceptName(t *testing.T) {
	got, inserted := insertWikilink("we discussed write-ahead logging at length", "write-ahead logging")
	if !inserted {
		t.Fatal("expected an insertion for the multi-word phrase")
	}
	if !strings.Contains(got, "[[write-ahead logging]]") {
		t.Errorf("got %q, want the phrase wrapped as one unit", got)
	}
}

func TestInsertWikilink_CaseInsensitiveMatch(t *testing.T) {
	got, inserted := insertWikilink("CHUNKING is capitalized here", "chunking")
	if !inserted {
		t.Fatal("expected an insertion")
	}
	if got != "[[CHUNKING]] is capitalized here" {
		t.Errorf("got %q, want the original casing preserved inside the brackets", got)
	}
}

func TestInsertWikilink_DoesNotSubstringMatch(t *testing.T) {
	got, inserted := insertWikilink("we need more storage", "RAG")
	if inserted {
		t.Errorf("got %q, inserted=%v, want no insertion -- RAG must not substring-match inside storage", got, inserted)
	}
}

// ─── tagDocumentText ─────────────────────────────────────────────────────────

func TestTagDocumentText_InsertsEveryMatchingCandidate(t *testing.T) {
	text := "We discussed chunking and also workspace layout today."
	got, inserted := tagDocumentText(text, []string{"chunking", "workspace"})
	if got != "We discussed [[chunking]] and also [[workspace]] layout today." {
		t.Errorf("got %q", got)
	}
	if len(inserted) != 2 {
		t.Errorf("inserted = %v, want both names", inserted)
	}
}

func TestTagDocumentText_OnlyReturnsNamesActuallyInserted(t *testing.T) {
	text := "We discussed chunking today."
	got, inserted := tagDocumentText(text, []string{"chunking", "not-mentioned-anywhere"})
	if len(inserted) != 1 || inserted[0] != "chunking" {
		t.Errorf("inserted = %v, want only chunking", inserted)
	}
	if got != "We discussed [[chunking]] today." {
		t.Errorf("got %q", got)
	}
}

// A longer phrase is applied before a shorter name it contains, so the
// shorter name's own pass does not fragment the phrase (e.g. wrapping just
// "chunking" inside what should become "[[chunking strategy]]").
func TestTagDocumentText_PrefersLongerPhraseOverContainedShorterName(t *testing.T) {
	text := "Our chunking strategy changed last quarter."
	got, inserted := tagDocumentText(text, []string{"chunking", "chunking strategy"})
	if !strings.Contains(got, "[[chunking strategy]]") {
		t.Errorf("got %q, want the longer phrase wrapped as one unit", got)
	}
	if strings.Contains(got, "[[chunking]] strategy") {
		t.Errorf("got %q, want chunking not wrapped separately inside the phrase", got)
	}
	found := map[string]bool{}
	for _, n := range inserted {
		found[n] = true
	}
	if !found["chunking strategy"] {
		t.Errorf("inserted = %v, want chunking strategy present", inserted)
	}
}

func TestTagDocumentText_EmptyCandidatesReturnsUnchanged(t *testing.T) {
	text := "nothing to do here"
	got, inserted := tagDocumentText(text, nil)
	if got != text || len(inserted) != 0 {
		t.Errorf("got %q, %v, want the text unchanged and nothing inserted", got, inserted)
	}
}

// ─── eligibleConcepts ────────────────────────────────────────────────────────

func TestEligibleConcepts_RequiresMoreThanOneOccurrence(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	got, err := eligibleConcepts(kb, "chunking mentioned once here", []string{"chunking"})
	if err != nil {
		t.Fatalf("eligibleConcepts: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want none -- only one occurrence", got)
	}
}

func TestEligibleConcepts_IncludesTermMentionedTwice(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	got, err := eligibleConcepts(kb, "chunking here, chunking again", []string{"chunking"})
	if err != nil {
		t.Fatalf("eligibleConcepts: %v", err)
	}
	if len(got) != 1 || got[0] != "chunking" {
		t.Errorf("got %v, want [chunking]", got)
	}
}

func TestEligibleConcepts_ExcludesCandidatesNotInAllowedList(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept chunking: %v", err)
	}
	if _, err := kb.AddConcept("workspace", ""); err != nil {
		t.Fatalf("AddConcept workspace: %v", err)
	}
	got, err := eligibleConcepts(kb, "chunking chunking workspace workspace", []string{"chunking"})
	if err != nil {
		t.Fatalf("eligibleConcepts: %v", err)
	}
	for _, name := range got {
		if name == "workspace" {
			t.Errorf("got %v, want workspace excluded -- it was not in the allowed candidate list", got)
		}
	}
}

func TestEligibleConcepts_ExcludesCodeSpans(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("format", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	got, err := eligibleConcepts(kb, "saw `const format = x` once, then `unsupported format` again", []string{"format"})
	if err != nil {
		t.Fatalf("eligibleConcepts: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want format excluded -- both mentions are inside code spans", got)
	}
}
