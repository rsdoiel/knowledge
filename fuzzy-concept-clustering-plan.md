# `kb concept suggest` — Fuzzy Candidate Clustering — Implementation Plan

See [fuzzy-concept-clustering-design.md](fuzzy-concept-clustering-design.md)
for the full rationale and confirmed decisions. Work items ordered
FC1 → FC5. TDD-first: each phase's tests are written and confirmed red
before its implementation, per this workspace's standing convention.

---

## FC1 — Shared fuzzy primitives (`retrieval.go`)

**Only needed if [fuzzy-concept-matching-plan.md](fuzzy-concept-matching-plan.md)'s
F1 hasn't landed yet** — that phase already specifies
`levenshteinDistance(a, b string) int` and `stripCommonSuffix(s string)
string` in `retrieval.go`, which this feature reuses verbatim (design
decision 1). If F1 ships first, this phase is just an import; if this plan
ships first, build them here exactly as F1 specifies, so whichever lands
second only adds a caller.

### Acceptance criteria

- `go test .` green (package root).

---

## FC2 — `scoreCandidateTerms` restructured for item-index sets

`itemCounts` changes from `map[string]int` to `map[string]map[int]bool` (a
set of item indices, decision 7) — needed so a cluster's `df` can be
computed as the size of a *union* across members, not a sum. This is an
internal-shape change to the existing function (`cmd/kb/concept.go:184-
224`), not a new one; its external behavior (same candidates, same scores,
same `df` values) must be provably unchanged for the non-clustering case —
`df := len(itemCounts[term])` replaces the old direct int read, same
numeric result.

### Tests (existing `concept_test.go` suite)

- Re-run/confirm all current `TestScoreCandidateTerms_*` tests pass
  unmodified in assertion, only internals differ.
- `TestScoreCandidateTerms_ItemCountsTracksDistinctItemIndices` — new,
  asserting the set shape directly (two mentions in the same item count
  once).

### Acceptance criteria

- `go test ./cmd/kb/...` green, no change to `TestCmdConceptSuggest_*`
  golden output for existing fixtures.

---

## FC3 — Near-existing exclusion (decision 3)

- `nearExistingMatch struct { Token, Concept string; Distance int }`.
- `excludeNearExisting(occurrences map[string]int, known map[string]bool)
  (survivors map[string]bool, nearExisting []nearExistingMatch)` — for each
  raw token (keys of `occurrences`, pre-clustering per decision 2),
  normalize via `stripCommonSuffix`, compare against every normalized
  known concept name via `levenshteinDistance`; if `0 < distance <= 1`,
  exclude from `survivors` and record in `nearExisting` (sorted by token
  for determinism). Runs *before* clustering (FC4) — an excluded token
  never becomes a cluster seed or member.

### Tests

- `TestExcludeNearExisting_DropsTokenWithinDistanceOneOfKnownConcept`
- `TestExcludeNearExisting_KeepsExactMatchToken` — exact matches are
  already filtered earlier by `known[tok]`, not this function; confirm no
  double-handling.
- `TestExcludeNearExisting_KeepsTokenBeyondThreshold`
- `TestExcludeNearExisting_ReportSortedDeterministically`

### Acceptance criteria

- `go test ./cmd/kb/...` green.

---

## FC4 — Star clustering (decisions 2, 4, 5, 6, 7, 10)

- `termCluster struct { Seed string; Variants []string; Occurrences int;
  Items map[int]bool }`.
- `clusterCandidateTerms(occurrences map[string]int, itemCounts
  map[string]map[int]bool, survivors map[string]bool) []termCluster`:
  1. Build the survivor list (decision 3's exclusions applied), sort by
     `occurrences` descending, term ascending on ties (mirrors
     `cmd/kb/concept.go:217-221`'s tie-break).
  2. Walk the sorted list; first unassigned token seeds a new cluster.
  3. Scan remaining unassigned tokens; a token joins the current cluster
     only if `|len(stripCommonSuffix(seed)) - len(stripCommonSuffix(tok))|
     <= 1` (decision 10's pruning) **and**
     `levenshteinDistance(stripCommonSuffix(seed), stripCommonSuffix(tok))
     <= 1` (decision 5) — both checked against the seed specifically, never
     a non-seed member (decision 4's anti-drift rule).
  4. Roll up: `Occurrences` = sum across members; `Items` = union of each
     member's `itemCounts[term]` set (decision 7).
  5. Repeat until no unassigned tokens remain. Singleton clusters (no
     variants) are the common case.

### Tests

- `TestClusterCandidateTerms_MergesPluralAndTenseVariants` — the
  chunking/chunkings/chunked case from the design's motivation.
- `TestClusterCandidateTerms_SeedIsHighestOccurrenceMember`
- `TestClusterCandidateTerms_DoesNotChainThroughIntermediateMembers` —
  A-B-C-D chain case, confirms star (not transitive) shape: D not within
  threshold of A directly must not join A's cluster even if D is within
  threshold of C.
- `TestClusterCandidateTerms_LengthPruningExcludesBeforeLevenshteinRuns` —
  a spy/counter on distance calls, or a length-differing pair that must
  never reach the DP.
- `TestClusterCandidateTerms_OccurrencesSumAcrossMembers`
- `TestClusterCandidateTerms_ItemSetUnionsNotSums` — an item containing
  both variants counts once toward `df`.
- `TestClusterCandidateTerms_SingletonClusterUnchangedFromToday`

### Acceptance criteria

- `go test ./cmd/kb/...` green.

---

## FC5 — Wire into `scoreCandidateTerms` output + `cmdConceptSuggest` rendering (decision 8)

- `candidateTerm` gains `Variants []string` (`json:"variants,omitempty"`)
  (`cmd/kb/concept.go:121-126`).
- `scoreCandidateTerms` signature changes: `func scoreCandidateTerms(items
  []string, known map[string]bool) (candidates []candidateTerm,
  nearExisting []nearExistingMatch)` — internally: build
  `occurrences`/`itemCounts` (FC2's shape) → `excludeNearExisting` (FC3) →
  `clusterCandidateTerms` (FC4) on survivors → existing `occ < 2` /
  `idf <= 0` filter loop now iterates clusters, `df := len(cluster.Items)`,
  building `candidateTerm{Term: cluster.Seed, Variants: cluster.Variants,
  Occurrences: cluster.Occurrences, Items: df, Score: ...}`.
- Update the one existing caller, `cmdConceptSuggest`
  (`cmd/kb/concept.go:293`), for the new two-value return.
- Text rendering: append `(+variant1, variant2)` after a clustered term's
  name (decision 8's exact example format); trailing
  `near-existing (excluded from candidates):` section, printed only when
  non-empty.
- `--json`: `{"candidates": [...], "near_existing": [...]}`.

### Tests

- `TestScoreCandidateTerms_ReturnsNearExistingAlongsideCandidates`
- `TestCmdConceptSuggest_RendersVariantsInline`
- `TestCmdConceptSuggest_RendersNearExistingSectionOnlyWhenNonEmpty`
- `TestCmdConceptSuggest_JSONIncludesNearExistingArray`
- `TestCmdConceptSuggest_ExistingFixturesUnchangedWhenNoVariantsOrNearExisting`
  — regression guard for decision 9's "always on, no flag" not breaking
  today's plain output.

### Acceptance criteria

- `go test ./cmd/kb/...` green.
- Manual smoke test against `agents/knowledge.db` (per
  `feedback_smoke_test_data_transforms`, matching DR-0028/DR-0029's own
  precedent): run `kb concept suggest` for real, eyeball whether any real
  variant cluster or near-existing exclusion appears and looks right —
  decision 10 explicitly defers further indexing until this shows it's
  needed.

`ConceptHelpText` (`cmd/kb/helptext.go:401+`) likely needs no synopsis
change (no new flag, decision 9) — confirm during implementation whether
the DESCRIPTION prose should mention clustering/near-existing behavior.

---

## After FC5: a decision record

Once implemented and smoke-tested, author a decision record via
`kb record new --project knowledge`, authored `proposed`, promoted by the
user — not self-promoted, per this workspace's standing rule.
