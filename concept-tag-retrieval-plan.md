# Concept-tag-based retrieval — Implementation Plan

See [concept-tag-retrieval-design.md](concept-tag-retrieval-design.md) for
the full rationale and confirmed decisions. Work items ordered W1 → W2.
TDD-first: each phase's tests are written and confirmed red before its
implementation.

New code lives in a new file, `retrieval.go` (tests in `retrieval_test.go`),
rather than growing `knowledge.go` further (already ~1900 lines) — matches
this package's existing convention of one file per feature area
(`records.go`, `knowledge_merge.go`, `retraction_check.go`).

---

## W1 — `RecallByConceptNames` + `ConceptMatch`

### Files to create

`retrieval.go`:

- `ConceptMatch` struct (design decision 4): `SourceType`, `ID`,
  `ProjectID`, `Title`, `Body`, `MatchCount`, `CreatedAt`.
- Unexported `conceptIDsByNames(kb *KnowledgeBase, names []string) ([]int64, error)`
  — per-name `SELECT id FROM concepts WHERE name = ?1 COLLATE NOCASE`,
  deduping resolved ids (not input names — two input spellings can resolve
  to the same id) and skipping any name with no match. Deliberately calls
  neither `AddConcept` nor `ResolveConceptName` (decision 3): a query error
  other than "no row" propagates; "no row" is a skip, not a failure.
- `RecallByConceptNames(names []string, limit int) ([]ConceptMatch, error)`:
  1. Resolve `names` via `conceptIDsByNames`; if none resolve, return `nil, nil`
     (a prompt mentioning nothing tagged isn't an error).
  2. Two queries, each `GROUP BY` the entity id with
     `COUNT(DISTINCT concept_id) AS match_count`, using a dynamic
     `IN (?,?,...)` placeholder list sized to the resolved id count:
     - Observations, joined through `observation_concepts`.
     - Records, joined through `record_concepts` (title comes along; body is
       `records.body`, timestamp is `records.ingested_at`).
  3. Merge both result slices, sort by `MatchCount` desc then `CreatedAt`
     desc, truncate to `limit` (a non-positive `limit` means "no cap" —
     confirm this default during implementation and document it in the
     doc comment either way).

### Tests to add (`retrieval_test.go`)

- `TestRecallByConceptNames_ReturnsLinkedObservation`
- `TestRecallByConceptNames_ReturnsLinkedRecord`
- `TestRecallByConceptNames_MergesAndRanksAcrossBothTypes` — seed one
  observation matching 1 concept and one record matching 2 (of the same
  queried set); assert the record sorts first.
- `TestRecallByConceptNames_TiesBreakByRecency` — two observations tied on
  match count, different `CreatedAt`; assert the newer sorts first.
- `TestRecallByConceptNames_UnmatchedNameIsSkippedNotError` — a name with no
  concept at all; assert no error, and it contributes nothing.
- `TestRecallByConceptNames_DoesNotCreateConcepts` — regression guard for
  decision 3: call with a name that matches no concept, then assert
  `Concepts()` is unchanged (count and contents) — this is the test that
  would catch an accidental `AddConcept`/`ResolveConceptName` call creeping
  back in.
- `TestRecallByConceptNames_DedupesCaseVariantInputNames` — `names` contains
  both `"Foo"` and `"foo"` resolving to the same concept; assert an
  observation linked to that concept gets `MatchCount == 1`, not 2.
- `TestRecallByConceptNames_NotProjectScoped` — link the same concept to
  observations in two different projects; assert both come back.
- `TestRecallByConceptNames_RespectsLimit` — three matching observations,
  `limit=2`; assert exactly 2 come back, the two highest-ranked.
- `TestRecallByConceptNames_EmptyNamesReturnsNilNotError`

### Acceptance criteria

- `go test ./...` green.
- `go vet ./...` clean.

---

## W2 — `MatchConceptNames`

### Files to modify

`retrieval.go`:

- `MatchConceptNames(text string) ([]string, error)` (design decision 2):
  loads `Concepts()` once, and for each concept name checks for a
  whole-word, case-insensitive match in `text` — build the check with
  `regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(name) + `\b`)` per
  concept name (compiled once per call, not cached — call frequency here is
  "once per prompt," not a hot loop, so per-call compilation is simple and
  fine; revisit only if profiling ever says otherwise). Returns matched
  concept names in `Concepts()`'s own order (by name), not input order —
  there is no input order, `text` is prose.

### Tests to add (`retrieval_test.go`)

- `TestMatchConceptNames_FindsWholeWordCaseInsensitive` — concept `"Foo"`,
  text `"... foo ..."`.
- `TestMatchConceptNames_DoesNotFalsePositiveOnSubstring` — concept
  `"RAG"`, text `"we need more storage"`; assert no match (guards decision 2
  directly — this is the case that motivated whole-word matching over
  substring).
- `TestMatchConceptNames_NoMatchesReturnsEmpty`
- `TestMatchConceptNames_MultipleConceptsInOneText`
- `TestMatchConceptNames_MatchesAtWordBoundaryPunctuation` — e.g. `"[[Foo]]"`
  or `"Foo."` still matches despite non-word characters immediately
  adjacent.

### Acceptance criteria

- `go test ./...` green.
- Manual smoke test (per `feedback_smoke_test_data_transforms` — this
  chains two real functions over real data, worth seeing it work end to
  end once, not just per-unit): in a scratch database, add a couple of
  concepts and link one to an observation and one to a record, then call
  `MatchConceptNames` on a short paragraph mentioning one of them by name,
  feed the result straight into `RecallByConceptNames`, and confirm the
  linked entity comes back. A small `go run`-able scratch program or a
  combined integration test in `retrieval_test.go` both satisfy this —
  pick whichever is faster to write at implementation time.
