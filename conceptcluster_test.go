package knowledge

import (
	"testing"
)

// Moved from cmd/kb/conceptcluster_test.go (library-lift-plan.md L3, DR-0035).

// ─── kb concept suggest fuzzy clustering (v0.0.11 item 5) ──────────────────
//
// fuzzyTermsClose is the shared distance check for both excludeNearExisting
// (FC3) and clusterCandidateTerms (FC4): raw distance first, falling back
// to comparing both sides' stripCommonSuffix forms. Computed by hand before
// writing this (see fuzzy-concept-clustering-design.md's corrected
// decisions 3/4): stemming *both* sides unconditionally, as the design
// first described, breaks its own chunking/chunkings pair (distance 3, not
// 1); a raw-only check breaks its own chunking/chunked pair (distance 3,
// exceeds threshold). Only the two-tier check catches both.

func TestFuzzyTermsClose_RawDistanceCatchesPluralVariant(t *testing.T) {
	d, ok := fuzzyTermsClose("chunking", "chunkings", 1)
	if !ok || d != 1 {
		t.Errorf("fuzzyTermsClose(chunking, chunkings) = %d, %v, want 1, true", d, ok)
	}
}

func TestFuzzyTermsClose_StemmedFallbackCatchesTenseVariant(t *testing.T) {
	d, ok := fuzzyTermsClose("chunking", "chunked", 1)
	if !ok || d != 0 {
		t.Errorf("fuzzyTermsClose(chunking, chunked) = %d, %v, want 0, true (both stem to \"chunk\")", d, ok)
	}
}

func TestFuzzyTermsClose_UnrelatedTermsDoNotMatch(t *testing.T) {
	if _, ok := fuzzyTermsClose("chunking", "workspace", 1); ok {
		t.Error("expected chunking/workspace not to be close")
	}
}

// ─── FC3: near-existing exclusion ──────────────────────────────────────────

func TestExcludeNearExisting_DropsTokenWithinDistanceOneOfKnownConcept(t *testing.T) {
	occurrences := map[string]int{"chunkings": 3, "unrelated": 2}
	known := map[string]bool{"chunking": true}
	survivors, nearExisting := excludeNearExisting(occurrences, known)
	if survivors["chunkings"] {
		t.Error("chunkings should have been excluded as near-existing")
	}
	if !survivors["unrelated"] {
		t.Error("unrelated should have survived")
	}
	found := false
	for _, m := range nearExisting {
		if m.Token == "chunkings" && m.Concept == "chunking" && m.Distance == 1 {
			found = true
		}
	}
	if !found {
		t.Errorf("nearExisting = %+v, want chunkings ~ chunking distance 1", nearExisting)
	}
}

func TestExcludeNearExisting_KeepsExactMatchToken(t *testing.T) {
	// An exact match is already filtered earlier by known[tok] in
	// tokenizeCandidateItems, so it would never actually appear as an
	// occurrences key here -- confirm excludeNearExisting doesn't ALSO try
	// to exclude it via distance 0 (double-handling), which would be
	// harmless but wrong to rely on.
	occurrences := map[string]int{"chunking": 3}
	known := map[string]bool{"chunking": true}
	survivors, nearExisting := excludeNearExisting(occurrences, known)
	if !survivors["chunking"] {
		t.Error("exact-match token should not be excluded by this function (distance 0 is not > 0)")
	}
	if len(nearExisting) != 0 {
		t.Errorf("nearExisting = %+v, want none", nearExisting)
	}
}

func TestExcludeNearExisting_KeepsTokenBeyondThreshold(t *testing.T) {
	occurrences := map[string]int{"somethingcompletelydifferent": 3}
	known := map[string]bool{"chunking": true}
	survivors, nearExisting := excludeNearExisting(occurrences, known)
	if !survivors["somethingcompletelydifferent"] {
		t.Error("expected the unrelated token to survive")
	}
	if len(nearExisting) != 0 {
		t.Errorf("nearExisting = %+v, want none", nearExisting)
	}
}

// Found reviewing this code before release: bestConcept is chosen by
// iterating `for name := range known`, a randomized-order Go map, keeping
// only a strictly-smaller distance -- an exact-distance tie between two
// known concepts is resolved by whichever happens to be visited first,
// nondeterministically across runs, despite the surrounding code sorting
// its own output "for deterministic output."
func TestExcludeNearExisting_TiesBrokenDeterministically(t *testing.T) {
	// "chunkx" is distance 1 from both "chunki" and "chunka" -- a genuine
	// tie (verified by hand), not just two candidates that happen to both
	// qualify at different distances.
	occurrences := map[string]int{"chunkx": 3}
	known := map[string]bool{"chunki": true, "chunka": true}
	for i := 0; i < 20; i++ {
		_, nearExisting := excludeNearExisting(occurrences, known)
		if len(nearExisting) != 1 || nearExisting[0].Concept != "chunka" {
			t.Fatalf("run %d: nearExisting = %+v, want a stable tie-break (alphabetically first concept, \"chunka\")", i, nearExisting)
		}
	}
}

func TestExcludeNearExisting_ReportSortedDeterministically(t *testing.T) {
	occurrences := map[string]int{"promts": 2, "chunkings": 2}
	known := map[string]bool{"prompt": true, "chunking": true}
	_, nearExisting := excludeNearExisting(occurrences, known)
	if len(nearExisting) != 2 {
		t.Fatalf("nearExisting = %+v, want 2 entries", nearExisting)
	}
	if nearExisting[0].Token > nearExisting[1].Token {
		t.Errorf("nearExisting = %+v, want sorted by Token ascending", nearExisting)
	}
}

// ─── FC4: star clustering ───────────────────────────────────────────────────

func idxSet(indices ...int) map[int]bool {
	m := map[int]bool{}
	for _, i := range indices {
		m[i] = true
	}
	return m
}

func TestClusterCandidateTerms_MergesPluralAndTenseVariants(t *testing.T) {
	occurrences := map[string]int{"chunking": 5, "chunkings": 2, "chunked": 2}
	itemCounts := map[string]map[int]bool{
		"chunking":  idxSet(0, 1, 2, 3, 4),
		"chunkings": idxSet(5, 6),
		"chunked":   idxSet(7, 8),
	}
	survivors := map[string]bool{"chunking": true, "chunkings": true, "chunked": true}
	clusters := clusterCandidateTerms(occurrences, itemCounts, survivors)
	if len(clusters) != 1 {
		t.Fatalf("clusters = %+v, want exactly 1 (all three merge)", clusters)
	}
	c := clusters[0]
	if c.Seed != "chunking" {
		t.Errorf("Seed = %q, want %q (highest occurrence)", c.Seed, "chunking")
	}
	if len(c.Variants) != 2 {
		t.Errorf("Variants = %+v, want chunkings and chunked", c.Variants)
	}
}

func TestClusterCandidateTerms_SeedIsHighestOccurrenceMember(t *testing.T) {
	occurrences := map[string]int{"widget": 3, "widgets": 9}
	itemCounts := map[string]map[int]bool{
		"widget":  idxSet(0, 1, 2),
		"widgets": idxSet(3, 4, 5, 6, 7, 8, 9, 10, 11),
	}
	survivors := map[string]bool{"widget": true, "widgets": true}
	clusters := clusterCandidateTerms(occurrences, itemCounts, survivors)
	if len(clusters) != 1 || clusters[0].Seed != "widgets" {
		t.Errorf("clusters = %+v, want one cluster seeded by widgets (higher occurrence)", clusters)
	}
}

// The classic cat/cot/cog/dog drift the design's own Motivation names --
// built here as 6-letter synthetic words (aaaaaa/baaaaa/bbaaaa/bbbaaa) so
// the pair isn't also filtered out by minFuzzyTermLength, which real
// 3-letter words like cat/cot/cog/dog would be. Each adjacent pair is
// distance 1, but w0~w2 (distance 2) and w0~w3 (distance 3) are not within
// threshold. Chain (transitive) clustering would merge all four into one;
// star clustering, bounded to each cluster's own seed, must not -- w0's
// cluster gets only its direct neighbor w1, never w2 or w3 reached only
// "through" w1.
func TestClusterCandidateTerms_DoesNotChainThroughIntermediateMembers(t *testing.T) {
	w0, w1, w2, w3 := "aaaaaa", "baaaaa", "bbaaaa", "bbbaaa"
	occurrences := map[string]int{w0: 10, w1: 5, w2: 4, w3: 3}
	itemCounts := map[string]map[int]bool{
		w0: idxSet(0), w1: idxSet(1), w2: idxSet(2), w3: idxSet(3),
	}
	survivors := map[string]bool{w0: true, w1: true, w2: true, w3: true}
	clusters := clusterCandidateTerms(occurrences, itemCounts, survivors)
	for _, c := range clusters {
		if c.Seed != w0 {
			continue
		}
		for _, v := range c.Variants {
			if v == w2 || v == w3 {
				t.Errorf("clusters = %+v, want %s's cluster to hold only its direct neighbor %s, not %s reached only through the chain", clusters, w0, w1, v)
			}
		}
	}
}

func TestClusterCandidateTerms_OccurrencesSumAcrossMembers(t *testing.T) {
	occurrences := map[string]int{"chunking": 5, "chunkings": 3}
	itemCounts := map[string]map[int]bool{
		"chunking":  idxSet(0, 1),
		"chunkings": idxSet(2),
	}
	survivors := map[string]bool{"chunking": true, "chunkings": true}
	clusters := clusterCandidateTerms(occurrences, itemCounts, survivors)
	if len(clusters) != 1 || clusters[0].Occurrences != 8 {
		t.Errorf("clusters = %+v, want Occurrences=8 (5+3)", clusters)
	}
}

func TestClusterCandidateTerms_ItemSetUnionsNotSums(t *testing.T) {
	// Item 0 contains both "chunking" and "chunkings" -- must count once
	// toward the cluster's df, not twice.
	occurrences := map[string]int{"chunking": 3, "chunkings": 2}
	itemCounts := map[string]map[int]bool{
		"chunking":  idxSet(0, 1),
		"chunkings": idxSet(0, 2),
	}
	survivors := map[string]bool{"chunking": true, "chunkings": true}
	clusters := clusterCandidateTerms(occurrences, itemCounts, survivors)
	if len(clusters) != 1 || len(clusters[0].Items) != 3 {
		t.Errorf("clusters = %+v, want a 3-item union {0,1,2}, not a 4-item sum", clusters)
	}
}

// A length gap alone (5, well beyond the threshold of 1) must prune the
// pair before any Levenshtein DP runs, per design decision 10 -- a
// behavioral check, not an instrumented call-count, since the pruning is a
// pure optimization that can never change a correct outcome, only skip
// reaching it the slow way.
func TestClusterCandidateTerms_LengthPruningExcludesBeforeLevenshteinRuns(t *testing.T) {
	// Both terms clear minFuzzyTermLength on their own; the length gap
	// between them (8) is what must rule this pair out, not the floor.
	occurrences := map[string]int{"chunking": 5, "chunkingcategory": 3}
	itemCounts := map[string]map[int]bool{"chunking": idxSet(0), "chunkingcategory": idxSet(1)}
	survivors := map[string]bool{"chunking": true, "chunkingcategory": true}
	clusters := clusterCandidateTerms(occurrences, itemCounts, survivors)
	if len(clusters) != 2 {
		t.Errorf("clusters = %+v, want 2 separate clusters -- length gap alone rules this pair out", clusters)
	}
}

func TestClusterCandidateTerms_SingletonClusterUnchangedFromToday(t *testing.T) {
	occurrences := map[string]int{"onlyone": 4}
	itemCounts := map[string]map[int]bool{"onlyone": idxSet(0, 1)}
	survivors := map[string]bool{"onlyone": true}
	clusters := clusterCandidateTerms(occurrences, itemCounts, survivors)
	if len(clusters) != 1 || clusters[0].Seed != "onlyone" || len(clusters[0].Variants) != 0 {
		t.Errorf("clusters = %+v, want one singleton cluster with no variants", clusters)
	}
}

// ─── FC5: wired into scoreCandidateTerms + cmdConceptSuggest ───────────────

func TestScoreCandidateTerms_ReturnsNearExistingAlongsideCandidates(t *testing.T) {
	items := []string{
		"chunkings appears here, chunkings again",
		"something entirely unrelated in a second item",
	}
	known := map[string]bool{"chunking": true}
	candidates, nearExisting := scoreCandidateTerms(items, known)
	for _, c := range candidates {
		if c.Term == "chunkings" {
			t.Errorf("candidates = %+v, want chunkings excluded as near-existing, not scored as a candidate", candidates)
		}
	}
	found := false
	for _, m := range nearExisting {
		if m.Token == "chunkings" && m.Concept == "chunking" {
			found = true
		}
	}
	if !found {
		t.Errorf("nearExisting = %+v, want chunkings ~ chunking", nearExisting)
	}
}

func TestScoreCandidateTerms_MergesVariantsBelowIndividualThreshold(t *testing.T) {
	// Each variant alone would fail the occ<2 floor or look unremarkable,
	// but merged they're a clear, distinctive signal -- the design's own
	// motivating case.
	items := []string{
		"chunking discussed at length here, chunking again, chunking a third time, and chunking a fourth time",
		"chunkings mentioned here",
		"we chunked things in this one",
		"nothing at all related appears in this filler item",
	}
	candidates, _ := scoreCandidateTerms(items, nil)
	var merged *ConceptCandidate
	for i := range candidates {
		if candidates[i].Term == "chunking" {
			merged = &candidates[i]
		}
	}
	if merged == nil {
		t.Fatalf("candidates = %+v, want a merged \"chunking\" cluster", candidates)
	}
	if len(merged.Variants) != 2 {
		t.Errorf("merged = %+v, want 2 variants (chunkings, chunked)", merged)
	}
}

// ─── false-positive gate: first letters must agree (found live 2026-09-23) ───
//
// The real corpus's near-existing block held plausible pairs beside three
// false ones, all produced by the stemmed fallback comparing stems that
// disagree at the very start, or a raw substitution of the first letter:
// nested/testing (nest~test), nesting/testing (raw), around/grounding.
// A real spelling variant keeps its first letter.

func TestFuzzyTermsClose_RejectsTermsWithDifferentFirstLetters(t *testing.T) {
	for _, p := range [][2]string{{"nested", "testing"}, {"nesting", "testing"}, {"around", "grounding"}} {
		if d, ok := fuzzyTermsClose(p[0], p[1], 1); ok {
			t.Errorf("fuzzyTermsClose(%s, %s) = %d, true, want false: different first letters", p[0], p[1], d)
		}
	}
}

func TestFuzzyTermsClose_StemmedTierStillCatchesRealInflections(t *testing.T) {
	// Real near-existing pairs from the same corpus that must survive.
	pairs := [][2]string{
		{"discovered", "discovery"}, {"identities", "identity"},
		{"formatting", "formatted"}, {"chunking", "chunked"},
		{"dependency", "dependencies"}, {"idempotence", "idempotency"},
	}
	for _, p := range pairs {
		if _, ok := fuzzyTermsClose(p[0], p[1], 1); !ok {
			t.Errorf("fuzzyTermsClose(%s, %s) = false, want true", p[0], p[1])
		}
	}
}
