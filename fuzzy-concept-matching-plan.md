# `kb document fuzzy-tag` — Implementation Plan

See [fuzzy-concept-matching-design.md](fuzzy-concept-matching-design.md) for
the full rationale and confirmed decisions. Work items ordered F1 → F4.
TDD-first: each phase's tests are written and confirmed red before its
implementation, per this workspace's standing convention.

---

## F1 — Library primitive: `FuzzyMatchConceptNames` (`knowledge` package)

Pure, DB-read-only (`kb.Concepts()`), no file I/O. Mirrors `MatchConceptNames`
(`retrieval.go:234`) and `MatchConceptNameCounts` (`retrieval.go:272`) in
shape and location, following the same "library gives the raw signal, the
caller applies eligibility policy" split DR-0027/DR-0028 already established
for density-linking and concept suggestion.

### Files to modify

`retrieval.go`:

- New unexported `levenshteinDistance(a, b string) int` — classic
  two-row iterative DP over `[]rune`, not `[]byte` (concept names and
  document prose aren't guaranteed ASCII).
- New unexported `stripCommonSuffix(s string) string` — lowercases, then
  strips exactly one trailing suffix from `"ing"`, `"es"`, `"ed"`, `"s"`
  tried longest-first (design decision 5); returns the input unchanged if
  none match. Applied to both sides of every distance comparison, never to
  `MatchConceptNames`'s own exact matching.
- New exported type:
  ```go
  type FuzzyConceptMatch struct {
      Concept  string // canonical concept name (Concepts()'s own casing)
      Text     string // the actual text found, original casing preserved
      Start    int    // byte offset into the input text
      End      int
      Distance int    // Levenshtein distance after stripCommonSuffix normalization
  }
  ```
- New exported method:
  ```go
  func (kb *KnowledgeBase) FuzzyMatchConceptNames(text string) ([]FuzzyConceptMatch, error)
  ```
  - Fetch `kb.Concepts()`. For each concept, first check whether it's
    already an *exact* whole-word match in `text` (same per-concept regex
    `MatchConceptNames` already builds) — if so, skip it entirely (design
    decision 5's scope rule: fuzzy is additive, never a duplicate of exact).
  - Tokenize `text` once, up front, into `[]struct{ Text string; Start,
    End int }` on a simple word-boundary split (`\b[\p{L}\p{N}']+\b`), not
    per-concept — one tokenize pass shared across every remaining concept.
  - For a single-word concept name, compare each token. For a `W`-word
    concept name, slide a window of `W` consecutive tokens (only where the
    tokens are contiguous in the original text — no window spanning a gap
    wider than ordinary whitespace, so "chunking, strategy" one sentence
    apart never becomes a false multi-word candidate), joining with a
    single space for comparison.
  - Normalize both sides with `stripCommonSuffix`, compute
    `levenshteinDistance`. Record a `FuzzyConceptMatch` when
    `0 < distance <= maxFuzzyDistance` (a package constant, `3` — a
    generous *candidate-generation* ceiling, not the real eligibility
    threshold; decision 5's tighter `1`/`2`-by-length threshold is cmd/kb's
    job to apply, same split as `MatchConceptNameCounts` → `densityLinkCandidates`).
  - Sort results by `Start`, then `Concept`, for deterministic output.

### Tests to add (`retrieval_test.go`, alongside `TestMatchConceptNameCounts_*`)

- `TestLevenshteinDistance_IdenticalStringsIsZero`
- `TestLevenshteinDistance_SingleCharacterEditIsOne` — "chunking" vs
  "chunkng" (deletion), "chunking" vs "chunkings" (insertion), "chunking"
  vs "chunkibg" (substitution).
- `TestStripCommonSuffix_StripsLongestMatchingSuffix` — "chunkings" →
  "chunk" via "ings"? No — decision 5's list is `ing`/`es`/`ed`/`s`
  individually, longest-first, so "chunkings" strips only trailing `s` →
  "chunking" (one strip, not iterative) — confirm this exact behavior
  against the design, not an assumed stemmer-like multi-pass.
- `TestFuzzyMatchConceptNames_FindsPluralVariant` — concept "chunking",
  text "the chunkings here", expect one match, distance 1.
- `TestFuzzyMatchConceptNames_FindsTenseVariant` — concept "chunk", text
  "we chunked it", expect one match.
- `TestFuzzyMatchConceptNames_FindsTypo` — concept "chunking", text
  "chunkibg happens", expect one match, distance 1.
- `TestFuzzyMatchConceptNames_SkipsConceptAlreadyExactlyMatched` — text
  contains both "chunking" (exact) and "chunkings" (near-miss); expect
  zero matches, per decision 5's scope rule.
- `TestFuzzyMatchConceptNames_SkipsUnrelatedWords` — a distance-4+ word
  pair produces nothing (confirms `maxFuzzyDistance` actually bounds
  output, not just documents it).
- `TestFuzzyMatchConceptNames_MatchesMultiWordConceptName` — concept
  "context window", text "the contxt windows were", expect one match
  spanning both tokens.
- `TestFuzzyMatchConceptNames_DoesNotSpanSentenceBoundary` — tokens from
  two different, unrelated multi-word mentions never combine into a false
  window match.
- `TestFuzzyMatchConceptNames_NoMatchesReturnsEmpty`
- `TestFuzzyMatchConceptNames_ResultsSortedByPosition`

### Acceptance criteria

- `go test .` (package root) green.
- `go vet ./...` clean.

---

## F2 — Raw-text mechanics (`cmd/kb`, pure functions)

No DB, no file I/O — string-in, string-out functions operating on a single
document's raw file text, mirroring `documenttag_test.go`'s existing split
between pure-function tests and CLI integration tests.

### Files to modify

New `cmd/kb/documentfuzzytag.go`:

- `sectionHeadingPattern = regexp.MustCompile(`(?m)^#{1,6}[ \t]+(.*)$`)` —
  own copy of `markdownHeadingPattern` (`documents.go:341`), same
  "own copy, not a shared internal" precedent `firstH1HeadingPattern`
  already sets (`cmd/kb/documenttag.go:75`).
- `footnoteLabelPattern = regexp.MustCompile(`\[\^(\d+)\]`)` — matches
  both a marker (`[^3]`) and the start of a definition (`[^3]:`).
- `footnoteDefinitionLinePattern = regexp.MustCompile(`(?m)^\[\^\d+\]:.*$`)`
- `nearestPrecedingHeading(text string, pos int) string` — scans all
  `sectionHeadingPattern` matches, returns the last one whose match starts
  before `pos`, or `""` if none (gist/lead text before any heading).
- `nextSectionBoundary(text string, pos int) int` — the start offset of
  the next `sectionHeadingPattern` match after `pos`, or `len(text)` if
  none. This is where a footnote definition for a marker at `pos` gets
  inserted.
- `nextFootnoteLabel(text string) int` — every `footnoteLabelPattern`
  match's captured number, `1 + max(...)`, or `1` if none found.
- `fuzzyExcludedSpans(text string) []textSpan` — `excludedSpans(text)`
  (`cmd/kb/documenttag.go:39`, untouched) plus every
  `footnoteDefinitionLinePattern` match, so a marker can never land inside
  an existing footnote definition. A new function, not an edit to
  `excludedSpans` itself — `kb document tag` must stay exactly as it is
  (design decision 2).
- `insertFootnote(text string, matchStart, matchEnd int, concept string, label int) string`
  — splices in definition-then-marker order (definition's insertion point
  is always `>= matchEnd`, so inserting it first keeps `matchEnd` valid for
  the marker insertion that follows): appends
  `"\n[^" + label + "]: see [[" + concept + "]]\n"` immediately before
  `nextSectionBoundary(text, matchEnd)`, then inserts `"[^" + label + "]"`
  immediately after `matchEnd` in the (now longer) text.

### Tests to add (`cmd/kb/documentfuzzytag_test.go`)

- `TestNearestPrecedingHeading_ReturnsLastHeadingBeforePosition`
- `TestNearestPrecedingHeading_EmptyWhenPositionBeforeAnyHeading`
- `TestNextSectionBoundary_ReturnsNextHeadingStart`
- `TestNextSectionBoundary_ReturnsTextLengthWhenNoFollowingHeading`
- `TestNextFootnoteLabel_StartsAtOneWhenNoneExist`
- `TestNextFootnoteLabel_ContinuesPastHighestExistingLabel` — text
  containing `[^1]` and `[^2]` (in either marker or definition form)
  yields `3`.
- `TestFuzzyExcludedSpans_ExcludesFootnoteDefinitionLine`
- `TestFuzzyExcludedSpans_StillExcludesEverythingExcludedSpansDoes` — a
  spot-check that fenced code / frontmatter / existing wikilinks are still
  excluded (regression against decision 5's "reuse as-is" claim).
- `TestInsertFootnote_MarkerImmediatelyFollowsMatch`
- `TestInsertFootnote_DefinitionPrecedesNextHeading`
- `TestInsertFootnote_DefinitionAtEndOfFileWhenNoFollowingHeading`
- `TestInsertFootnote_MultipleFootnotesInOneSectionDoNotCollideLabels`

### Acceptance criteria

- `go test ./cmd/kb/...` green.

---

## F3 — `cmdDocumentFuzzyTag` wiring

### Files to modify

`cmd/kb/documentfuzzytag.go` (continued):

- `fuzzyEligible(matches []knowledge.FuzzyConceptMatch, explicit []string) []knowledge.FuzzyConceptMatch`
  — applies design decision 5's real threshold (not `F1`'s generous
  `maxFuzzyDistance` candidate ceiling): keep a match if its `Concept` is
  in `explicit` (any distance up to `maxFuzzyDistance` qualifies — the
  `--concept` bypass, decision 2), or if no `explicit` list was given and
  `Distance <= 1` for concept names of 8 characters or fewer, `<= 2`
  otherwise. Then drops any match whose `[Start, End)` overlaps a span from
  `fuzzyExcludedSpans(text)` (F2).
- `documentFuzzyTagResult` — mirrors `documentTagResult`
  (`cmd/kb/documenttag.go:217`): `Path string`, `Footnoted []fuzzyTagEntry`,
  where `fuzzyTagEntry{Section, Text, Concept string; Distance, Footnote int}`.
- `cmdDocumentFuzzyTag(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error`
  — same shape as `cmdDocumentTag` (`cmd/kb/documenttag.go:256`): parse
  `--project`/`--concept`/`--dry-run`, resolve the project, validate
  `--concept` names against known concepts up front (fail before any file
  opens, matching `tag`'s own rule), iterate `kb.Documents(p.ID)`, per file:
  `os.ReadFile` → `kb.FuzzyMatchConceptNames(text)` → `fuzzyEligible` →
  for each surviving match, in `Start` order, skip if
  `alreadyWikilinked(text, match.Concept)` is already true (decision 3's
  free idempotency), else assign the next label via `nextFootnoteLabel`
  and apply `insertFootnote`, recording a `fuzzyTagEntry` with
  `Section: nearestPrecedingHeading(text, match.Start)`. Same both-or-
  neither staged-write-with-rollback pattern `cmdDocumentTag` already uses
  (`cmd/kb/documenttag.go:307-357`) when not `--dry-run`.
- `reportDocumentFuzzyTag(jsonOut, dryRun bool, results []documentFuzzyTagResult, out io.Writer) error`
  — mirrors `reportDocumentTag` (`cmd/kb/documenttag.go:361`): "would
  footnote"/"footnoted" verb, one line per entry per design decision 4.

`cmd/kb/document.go`:

- Add `case "fuzzy-tag": return cmdDocumentFuzzyTag(kb, jsonOut, rest, out)`
  to the `document` subverb switch, alongside the existing `"tag"` case.

### Tests to add (`cmd/kb/documentfuzzytag_cmd_test.go`, mirroring `documenttag_cmd_test.go`)

- `TestCmdDocumentFuzzyTag_DryRunWritesNothing`
- `TestCmdDocumentFuzzyTag_InsertsFootnoteForPluralNearMiss`
- `TestCmdDocumentFuzzyTag_PreservesOriginalWordSpelling` — the matched
  word itself is byte-identical before/after, only a `[^n]` marker and a
  trailing definition line are new (decision 6).
- `TestCmdDocumentFuzzyTag_SkipsWhenAlreadyExactlyLinkedElsewhere`
- `TestCmdDocumentFuzzyTag_SecondRunIsIdempotent` — run twice, assert the
  second run's `Footnoted` is empty and the file is byte-identical to
  after the first run.
- `TestCmdDocumentFuzzyTag_ConceptFlagBypassesDistanceThreshold` —a
  distance-3 near-miss is skipped by default but footnoted when named via
  `--concept`.
- `TestCmdDocumentFuzzyTag_UnknownConceptFlagErrorsBeforeAnyFileOpens`
- `TestCmdDocumentFuzzyTag_RequiresProject`
- `TestCmdDocumentFuzzyTag_JSONOutputShape`
- `TestCmdDocumentFuzzyTag_RollsBackAllFilesIfOneWriteFails`
- `TestCmdDocumentFuzzyTag_NeverTouchesExactMatchInsertion` — a file with
  both an exact-match candidate and a fuzzy candidate for *different*
  concepts: run `fuzzy-tag` alone, assert the exact-match concept is
  untouched (only `tag` ever inserts a bracket).

### Acceptance criteria

- `go test ./cmd/kb/...` green.
- Manual smoke test against a scratch document (per
  `feedback_smoke_test_data_transforms` — this is exactly the kind of
  prose-rewriting transform DR-0029 itself found a real bug in only by
  running against a real file, not just unit tests): write a throwaway
  document under a scratch project containing a deliberate typo/plural of
  a known concept, run `kb document fuzzy-tag --project SCRATCH`
  for real, inspect the resulting file by eye for a correctly placed
  marker and definition, then run `kb document ingest` on it and confirm
  via `kb document show` (or a direct query) that the concept actually
  linked.

---

## F4 — Documentation

### Files to modify

`cmd/kb/helptext.go`:

- `DocumentHelpText` (`cmd/kb/helptext.go:154`): add
  `{app_name} document fuzzy-tag --project P [--concept NAME,...] [--dry-run]`
  to `SYNOPSIS`, alongside the existing `tag` line (`cmd/kb/helptext.go:176`),
  and a `DESCRIPTION` paragraph covering the footnote mechanism and why it
  differs from `tag`'s direct bracket-wrap (design decision 3, short form).

### Regeneration (do not hand-edit `kb-document.1.md` — it is generated)

```bash
make kb-topics-help   # regenerates kb-document.1.md
```

### Acceptance criteria

- `git diff kb-document.1.md` shows only the expected addition, regenerated
  from the edited `helptext.go` constant — no hand-edits to the `.1.md`
  file itself.

---

## After F4: a decision record

Once implemented and smoke-tested, author a decision record via
`kb record new --project knowledge`, following DR-0029's own precedent as
the most directly related prior work (`kb document tag`'s own original
implementation). Authored `proposed`, promoted by the user — not
self-promoted, per this workspace's standing rule that a model may write a
record but not accept one.
