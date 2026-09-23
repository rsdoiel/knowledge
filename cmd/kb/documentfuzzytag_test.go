package main

import "testing"

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
