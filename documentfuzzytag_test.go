package knowledge

import (
	"strings"
	"testing"
)

// ─── kb document fuzzy-tag (v0.0.11 item 1), raw-text mechanics ───────────

func TestNearestPrecedingHeading_ReturnsLastHeadingBeforePosition(t *testing.T) {
	text := "# Title\n\n## Background\n\nsome text here\n\n## Details\n\nmore text\n"
	pos := len("# Title\n\n## Background\n\nsome text here\n\n## De")
	if got := nearestPrecedingHeading(text, pos); got != "Details" {
		t.Errorf("nearestPrecedingHeading = %q, want %q", got, "Details")
	}
}

func TestNearestPrecedingHeading_EmptyWhenPositionBeforeAnyHeading(t *testing.T) {
	text := "some lead text\n\n# Title\n\nbody\n"
	if got := nearestPrecedingHeading(text, 3); got != "" {
		t.Errorf("nearestPrecedingHeading = %q, want empty", got)
	}
}

func TestNextSectionBoundary_ReturnsNextHeadingStart(t *testing.T) {
	text := "## Background\n\nsome text here\n\n## Details\n\nmore text\n"
	pos := len("## Background\n\nsome text")
	want := len("## Background\n\nsome text here\n\n")
	if got := nextSectionBoundary(text, pos); got != want {
		t.Errorf("nextSectionBoundary = %d, want %d", got, want)
	}
}

func TestNextSectionBoundary_ReturnsTextLengthWhenNoFollowingHeading(t *testing.T) {
	text := "## Only Section\n\nsome text with no more headings\n"
	pos := len(text) - 5
	if got := nextSectionBoundary(text, pos); got != len(text) {
		t.Errorf("nextSectionBoundary = %d, want %d (len(text))", got, len(text))
	}
}

func TestNextFootnoteLabel_StartsAtOneWhenNoneExist(t *testing.T) {
	if got := nextFootnoteLabel("nothing relevant here"); got != 1 {
		t.Errorf("nextFootnoteLabel = %d, want 1", got)
	}
}

func TestNextFootnoteLabel_ContinuesPastHighestExistingLabel(t *testing.T) {
	text := "see it here[^1]\n\n[^1]: see [[Foo]]\n\nalso[^2]\n\n[^2]: see [[Bar]]\n"
	if got := nextFootnoteLabel(text); got != 3 {
		t.Errorf("nextFootnoteLabel = %d, want 3", got)
	}
}

func TestFuzzyExcludedSpans_ExcludesFootnoteDefinitionLine(t *testing.T) {
	text := "some prose here\n\n[^1]: see [[chunking]]\n"
	spans := fuzzyExcludedSpans(text)
	defLineStart := len("some prose here\n\n")
	found := false
	for _, s := range spans {
		if s.Start <= defLineStart && defLineStart < s.End {
			found = true
		}
	}
	if !found {
		t.Errorf("fuzzyExcludedSpans = %+v, want a span covering the footnote definition line at %d", spans, defLineStart)
	}
}

func TestFuzzyExcludedSpans_StillExcludesEverythingExcludedSpansDoes(t *testing.T) {
	text := "---\ntitle: x\n---\n\n# Title\n\nsaw `format` here, and [[Chunking]] there.\n"
	got := fuzzyExcludedSpans(text)
	want := excludedSpans(text)
	if len(got) < len(want) {
		t.Fatalf("fuzzyExcludedSpans returned fewer spans (%d) than excludedSpans (%d)", len(got), len(want))
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("fuzzyExcludedSpans is missing excludedSpans's span %+v", w)
		}
	}
}

func TestInsertFootnote_MarkerImmediatelyFollowsMatch(t *testing.T) {
	text := "## Background\n\nwe saw chunkibg happen.\n"
	matchStart := len("## Background\n\nwe saw ")
	matchEnd := matchStart + len("chunkibg")
	got := insertFootnote(text, matchStart, matchEnd, "chunking", 1)
	want := "## Background\n\nwe saw chunkibg[^1] happen.\n\n[^1]: see [[chunking]]\n"
	if got != want {
		t.Errorf("insertFootnote =\n%q\nwant\n%q", got, want)
	}
}

func TestInsertFootnote_DefinitionPrecedesNextHeading(t *testing.T) {
	text := "## Background\n\nchunkibg happens here.\n\n## Next\n\nmore text.\n"
	matchStart := len("## Background\n\n")
	matchEnd := matchStart + len("chunkibg")
	got := insertFootnote(text, matchStart, matchEnd, "chunking", 3)
	nextHeadingIdx := indexOf(got, "## Next")
	defIdx := indexOf(got, "[^3]: see [[chunking]]")
	if defIdx < 0 || nextHeadingIdx < 0 || defIdx >= nextHeadingIdx {
		t.Errorf("insertFootnote = %q, want the definition inserted before \"## Next\"", got)
	}
}

func TestInsertFootnote_DefinitionAtEndOfFileWhenNoFollowingHeading(t *testing.T) {
	text := "## Only Section\n\nchunkibg happens here with no more headings after.\n"
	matchStart := len("## Only Section\n\n")
	matchEnd := matchStart + len("chunkibg")
	got := insertFootnote(text, matchStart, matchEnd, "chunking", 1)
	if got[len(got)-len("[^1]: see [[chunking]]\n"):] != "[^1]: see [[chunking]]\n" {
		t.Errorf("insertFootnote = %q, want the definition appended at end of file", got)
	}
}

func TestInsertFootnote_MultipleFootnotesInOneSectionDoNotCollideLabels(t *testing.T) {
	text := "## Background\n\nchunkibg and promt both appear here.\n"
	firstEnd := len("## Background\n\nchunkibg")
	afterFirst := insertFootnote(text, len("## Background\n\n"), firstEnd, "chunking", 1)

	secondStart := indexOf(afterFirst, "promt")
	secondEnd := secondStart + len("promt")
	got := insertFootnote(afterFirst, secondStart, secondEnd, "prompt", 2)

	if indexOf(got, "[^1]: see [[chunking]]") < 0 || indexOf(got, "[^2]: see [[prompt]]") < 0 {
		t.Errorf("insertFootnote = %q, want both footnote definitions present without colliding", got)
	}
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ─── L2 (library-lift-plan.md): the exported, text-in/text-out surface ──────

func fuzzyKB(t *testing.T, concepts ...string) *KnowledgeBase {
	t.Helper()
	kb := openTestKB(t)
	for _, c := range concepts {
		if _, err := kb.AddConcept(c, ""); err != nil {
			t.Fatalf("AddConcept(%q): %v", c, err)
		}
	}
	return kb
}

func TestFuzzyEligible_KeepsMatchWithinLengthBasedThreshold(t *testing.T) {
	kb := fuzzyKB(t, "chunking")
	text := "the chunkings were odd\n"
	matches, err := kb.FuzzyMatchConceptNames(text)
	if err != nil {
		t.Fatalf("FuzzyMatchConceptNames: %v", err)
	}
	got := FuzzyEligible(matches, nil, text)
	if len(got) != 1 || got[0].Concept != "chunking" {
		t.Errorf("FuzzyEligible = %+v, want the chunking near-miss kept", got)
	}
}

func TestFuzzyEligible_ExplicitConceptsFilterToNamedOnly(t *testing.T) {
	kb := fuzzyKB(t, "chunking", "prompt")
	text := "the chunkibg and the promt\n"
	matches, _ := kb.FuzzyMatchConceptNames(text)
	got := FuzzyEligible(matches, []string{"prompt"}, text)
	if len(got) != 1 || got[0].Concept != "prompt" {
		t.Errorf("FuzzyEligible = %+v, want only the explicitly named concept", got)
	}
}

func TestFuzzyEligible_DropsMatchInsideCodeSpan(t *testing.T) {
	kb := fuzzyKB(t, "chunking")
	text := "see `chunkings` in code\n"
	matches, _ := kb.FuzzyMatchConceptNames(text)
	if got := FuzzyEligible(matches, nil, text); len(got) != 0 {
		t.Errorf("FuzzyEligible = %+v, want the code-span match dropped", got)
	}
}

func TestFuzzyTagDocumentText_InsertsMarkerAndDefinition(t *testing.T) {
	kb := fuzzyKB(t, "chunking")
	text := "## Background\n\nthe chunkings here.\n\n## Next\n\nmore.\n"
	matches, _ := kb.FuzzyMatchConceptNames(text)
	got, ins := FuzzyTagDocumentText(text, FuzzyEligible(matches, nil, text))
	if len(ins) != 1 || ins[0].Concept != "chunking" || ins[0].Footnote != 1 || ins[0].Section != "Background" || ins[0].Text != "chunkings" {
		t.Fatalf("insertions = %+v, want one chunking footnote 1 in Background", ins)
	}
	if !strings.Contains(got, "chunkings[^1]") {
		t.Errorf("text = %q, want a marker right after the near-miss", got)
	}
	if strings.Index(got, "[^1]: see [[chunking]]") > strings.Index(got, "## Next") {
		t.Errorf("text = %q, want the definition before the next heading", got)
	}
}

func TestFuzzyTagDocumentText_NoMatchesReturnsTextUnchanged(t *testing.T) {
	got, ins := FuzzyTagDocumentText("nothing here\n", nil)
	if got != "nothing here\n" || len(ins) != 0 {
		t.Errorf("got %q, %+v; want unchanged and no insertions", got, ins)
	}
}

func TestFuzzyTagDocumentText_SkipsConceptAlreadyWikilinked(t *testing.T) {
	kb := fuzzyKB(t, "chunking")
	text := "exact [[chunking]] here, and later the chunkings.\n"
	matches, _ := kb.FuzzyMatchConceptNames(text)
	got, ins := FuzzyTagDocumentText(text, FuzzyEligible(matches, nil, text))
	if got != text || len(ins) != 0 {
		t.Errorf("got %q, %+v; want unchanged: fuzzy matching never duplicates an exact link", got, ins)
	}
}

// Regression (v0.0.11 post-review): two near-misses in the same section must
// each land at their own byte offset. A single running delta spliced the
// second marker inside the first footnote's own definition text.
func TestFuzzyTagDocumentText_TwoNearMissesInOneSectionDoNotCorruptEachOther(t *testing.T) {
	kb := fuzzyKB(t, "chunking", "prompt")
	text := "## Background\n\nthe chunkibg happens here and the promt appears there.\n"
	matches, _ := kb.FuzzyMatchConceptNames(text)
	got, ins := FuzzyTagDocumentText(text, FuzzyEligible(matches, nil, text))
	if len(ins) != 2 {
		t.Fatalf("insertions = %+v, want 2", ins)
	}
	for _, want := range []string{"chunkibg[^1]", "promt[^2]", "[^1]: see [[chunking]]", "[^2]: see [[prompt]]"} {
		if !strings.Contains(got, want) {
			t.Errorf("text = %q, missing %q", got, want)
		}
	}
}

func TestFuzzyTagDocumentText_IsIdempotentOnItsOwnOutput(t *testing.T) {
	kb := fuzzyKB(t, "chunking")
	text := "## A\n\nthe chunkings.\n"
	matches, _ := kb.FuzzyMatchConceptNames(text)
	once, _ := FuzzyTagDocumentText(text, FuzzyEligible(matches, nil, text))
	matches2, _ := kb.FuzzyMatchConceptNames(once)
	twice, ins := FuzzyTagDocumentText(once, FuzzyEligible(matches2, nil, once))
	if twice != once || len(ins) != 0 {
		t.Errorf("second pass changed text or inserted: %q, %+v", twice, ins)
	}
}

// ─── false-positive gates (found live 2026-09-23, see DR-0036) ───────────────
//
// Measured over 159 real documents against the real 121-concept vocabulary,
// 614 proposals came back and most were noise: short concept names colliding
// with common words (fts<-its, madr<-made, kb<-db, cmt<-cmd, Go<-to, awk<-ask,
// tui<-ui) and distance-2 substitutions on long words (retirement<-requirement,
// correction<-connection, supersession<-suppression). The true positives were
// inflections and spelling variants (documents<-document, merge<-merged,
// idempotency<-idempotent, normalisation<-normalization).

func TestFuzzyEligible_ShortConceptNamesNeverFuzzyMatchOnTheirOwn(t *testing.T) {
	cases := []struct{ concept, text string }{
		{"fts", "its"}, {"madr", "made"}, {"cmt", "cmd"}, {"awk", "ask"},
		{"tui", "ui"}, {"drift", "draft"}, // 5 letters: still below the floor
	}
	for _, c := range cases {
		kb := fuzzyKB(t, c.concept)
		text := "a sentence with " + c.text + " in it\n"
		matches, err := kb.FuzzyMatchConceptNames(text)
		if err != nil {
			t.Fatalf("FuzzyMatchConceptNames: %v", err)
		}
		if got := FuzzyEligible(matches, nil, text); len(got) != 0 {
			t.Errorf("%s<-%s: FuzzyEligible = %+v, want none: concept is shorter than %d letters",
				c.concept, c.text, got, minFuzzyConceptLength)
		}
	}
}

func TestFuzzyEligible_ExplicitConceptBypassesTheShortNameFloor(t *testing.T) {
	kb := fuzzyKB(t, "fts")
	text := "a sentence with its in it\n"
	matches, _ := kb.FuzzyMatchConceptNames(text)
	got := FuzzyEligible(matches, []string{"fts"}, text)
	// An explicit --concept accepts anything within the matcher's own ceiling,
	// so noise words match too; what matters is that "its" is not filtered.
	found := false
	for _, m := range got {
		if m.Text == "its" {
			found = true
		}
	}
	if !found {
		t.Errorf("FuzzyEligible = %+v, want the explicit --concept override honoured", got)
	}
}

func TestFuzzyEligible_LongConceptSingleEditStillMatches(t *testing.T) {
	cases := []struct{ concept, text string }{
		{"documents", "document"}, {"normalisation", "normalization"},
		{"wikilinks", "wikilink"}, {"collisions", "collision"},
	}
	for _, c := range cases {
		kb := fuzzyKB(t, c.concept)
		text := "a sentence with " + c.text + " in it\n"
		matches, _ := kb.FuzzyMatchConceptNames(text)
		if got := FuzzyEligible(matches, nil, text); len(got) != 1 {
			t.Errorf("%s<-%s: FuzzyEligible = %+v, want the match kept", c.concept, c.text, got)
		}
	}
}

func TestFuzzyEligible_DistanceTwoNeedsALongSharedPrefix(t *testing.T) {
	// Real false positives: distance 2, but the words only share a couple of
	// leading letters.
	for _, c := range []struct{ concept, text string }{
		{"retirement", "requirement"}, {"correction", "connection"},
		{"correction", "collection"}, {"supersession", "suppression"},
		{"portability", "parsability"},
	} {
		kb := fuzzyKB(t, c.concept)
		text := "a sentence with " + c.text + " in it\n"
		matches, _ := kb.FuzzyMatchConceptNames(text)
		if got := FuzzyEligible(matches, nil, text); len(got) != 0 {
			t.Errorf("%s<-%s: FuzzyEligible = %+v, want dropped: distance 2 with a short shared prefix", c.concept, c.text, got)
		}
	}
	// Real true positives at distance 2: inflections sharing a long stem.
	for _, c := range []struct{ concept, text string }{
		{"idempotency", "idempotent"}, {"documents", "documented"},
		{"discovery", "discovered"}, {"migration", "migrating"},
	} {
		kb := fuzzyKB(t, c.concept)
		text := "a sentence with " + c.text + " in it\n"
		matches, _ := kb.FuzzyMatchConceptNames(text)
		if got := FuzzyEligible(matches, nil, text); len(got) != 1 {
			t.Errorf("%s<-%s: FuzzyEligible = %+v, want kept: distance 2 sharing a long prefix", c.concept, c.text, got)
		}
	}
}
