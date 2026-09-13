# `kb ingest` — inline `[[wikilink]]` concept tagging — Implementation Plan

See [wikilink-tagging-design.md](wikilink-tagging-design.md) for the full
rationale and confirmed decisions. Work items ordered W1 → W5. TDD-first:
each phase's tests are written and confirmed red before its implementation.

---

## W1 — Schema + core `knowledge` package primitives

### Files to modify

`records.go` — append to `recordsSchema` (after `record_relations`):

```sql
CREATE TABLE IF NOT EXISTS record_concepts (
    record_id  INTEGER REFERENCES records(id)  ON DELETE CASCADE,
    concept_id INTEGER REFERENCES concepts(id) ON DELETE CASCADE,
    PRIMARY KEY (record_id, concept_id)
);
```

`knowledge.go` — two new methods, placed near `LinkObservationConcept`/
`LinkProjectConcept` and `ObservationSources` respectively:

- `LinkRecordConcept(recordID, conceptID int64) error` — verbatim shape of
  `LinkProjectConcept` (`INSERT OR IGNORE INTO record_concepts ...`).
- `ResolveConceptName(name string) (int64, error)` — per design decision 4:
  `SELECT id FROM concepts WHERE name = ? COLLATE NOCASE LIMIT 1`; on
  `sql.ErrNoRows`, fall through to `kb.AddConcept(name, "")` and return its
  id; any other query error propagates.
- `RecordConcepts(recordID int64) ([]Concept, error)` — mirrors
  `ObservationSources`'s shape exactly: join `concepts` through
  `record_concepts`, `ORDER BY c.id`, scan into `Concept{ID, Name,
  Description, IdentifierType, IdentifierValue}`.

### Tests to add (`knowledge_test.go`)

- `TestLinkRecordConcept_CreatesLink`
- `TestLinkRecordConcept_DuplicateIsNoOp` — linking the same pair twice
  leaves exactly one row.
- `TestLinkRecordConcept_CascadesOnRecordDelete` and
  `TestLinkRecordConcept_CascadesOnConceptDelete` — confirms the
  `ON DELETE CASCADE` on both sides actually fires (SQLite requires
  `PRAGMA foreign_keys = ON`, already set in `schema`).
- `TestResolveConceptName_CreatesWhenMissing`
- `TestResolveConceptName_CaseInsensitiveMatchesExisting` — seed a concept
  named `Computer`, resolve `"computer"`, assert the *same* id comes back
  and no second row was created.
- `TestResolveConceptName_FirstWrittenCasingIsCanonical` — resolve
  `"computer"` first (creates it lowercase), then resolve `"Computer"`,
  assert the stored name is still `"computer"`.
- `TestRecordConcepts_ReturnsLinkedConceptsOrderedByID`
- `TestRecordConcepts_EmptyWhenNoneLinked` — returns `[]Concept{}`, not nil
  (matches `ObservationSources`'s own convention — confirm by reading it,
  match whichever it actually does).

### Acceptance criteria

- `go test ./...` (package root) green.
- `go vet ./...` clean.

---

## W2 — Ingest-side wiring: `[[wikilink]]` + `Tags` resolution

### Files to modify

`cmd/kb/ingest.go`:

- New unexported `wikilinkPattern = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)`
  package-level var, alongside the existing `recordFilePattern`.
- New method `(ing *ingester) linkWikilinkTags(rf *knowledge.RecordFile,
  recordDBID int64)`:
  - Returns immediately if `ing.dryRun || recordDBID == 0`.
  - Collects names from two sources: every `wikilinkPattern` match in
    `rf.Record.Body` (submatch 1, trimmed via `strings.TrimSpace`), plus
    every entry in `rf.Record.Tags` (also trimmed).
  - De-duplicates the combined name list before resolving (a `[[Name]]`
    appearing twice, or also listed in `Tags`, must not attempt two links —
    `LinkRecordConcept` is idempotent so this is a minor efficiency
    concern, not a correctness one, but avoids redundant `ResolveConceptName`
    round-trips).
  - For each distinct, non-empty name: `ing.kb.ResolveConceptName(name)`
    then `ing.kb.LinkRecordConcept(recordDBID, conceptID)`; on either error,
    append to `ing.summary.Warnings` (matching how other non-fatal ingest
    problems are reported) and continue rather than aborting the record.
- One new call site in `upsertAll`, immediately after
  `ing.linkInitiative(rf, projectID)`:
  ```go
  ing.linkWikilinkTags(rf, rec.dbID)
  ```

### Tests to add (`cmd/kb/ingest_test.go`, alongside
`TestCmdIngest_InitiativeBecomesAConcept`)

- `TestCmdIngest_WikilinkInBodyBecomesConcept` — a record body containing
  `[[Foo]]`; assert a concept named `Foo` exists and is linked via
  `RecordConcepts`.
- `TestCmdIngest_TagsFrontmatterBecomesConcepts` — `tags: [bar, baz]` in
  frontmatter, no wikilinks in body; assert both become linked concepts.
- `TestCmdIngest_WikilinkAndTagsAreCaseInsensitive` — body has `[[Foo]]`
  and `[[foo]]` in two different sentences (mirrors the user's own
  "`[[Computers]] are ... a [[computer]]`" example); assert exactly one
  concept, linked once.
- `TestCmdIngest_ReingestUnchangedFileDoesNotDuplicateLinks` — run ingest
  twice on the same file; assert `record_concepts` still has exactly the
  same rows (via `RecordConcepts`, not a raw duplicate count — count is the
  cheaper assertion here).
- `TestCmdIngest_DryRunCreatesNoConceptsOrLinks` — `--dry-run` with a
  wikilink in the body; assert the concept was never created at all
  (distinguish from "created but not linked").
- `TestCmdIngest_WikilinkWhitespaceIsTrimmed` — `[[ Foo ]]` resolves to the
  same concept as `[[Foo]]`.

### Acceptance criteria

- `go test ./cmd/kb/...` green.
- Manual smoke test against a scratch decision record (per
  `feedback_smoke_test_data_transforms` convention — this is exactly the
  kind of ingest/merge-shaped transform that needs a real binary run, not
  just unit tests): write a throwaway record with a `[[wikilink]]` and a
  `tags:` entry under a scratch project, `kb ingest` it for real, confirm
  with `kb record concepts` (once W3 lands) or a direct `sqlite3` query
  against `record_concepts`.

---

## W3 — Read path: `kb record concepts ID`

### Files to modify

`cmd/kb/record.go`:

- Add `case "concepts": return recordConcepts(kb, dl, jsonOut, flags, out)`
  to the subverb switch; update the `default` error's verb list to include
  `concepts`.
- New `recordConcepts` function, verbatim shape of `cmdObservationSources`:
  parses one positional `RECORD_ID` (note: this is the record's *display*
  id, e.g. `0042` or `DR-0042` — confirm during implementation which form
  `record show` already accepts and match it exactly, rather than requiring
  the internal database id a caller can't otherwise see), calls
  `kb.RecordConcepts(...)` through `logKBCall`, prints one `id  name` line
  per concept or `(no linked concepts)`, honors `--json`.

### Tests to add (`cmd/kb/record_test.go`)

- `TestRecordConcepts_ListsLinkedConcepts`
- `TestRecordConcepts_NoLinkedConceptsPrintsPlaceholder`
- `TestRecordConcepts_JSONOutput`
- `TestRecordConcepts_UnknownRecordIDErrors`

### Acceptance criteria

- `go test ./cmd/kb/...` green.

---

## W4 — Portability: merge + JSON-L export/import

### Files to modify

`knowledge_merge.go`:

- New uuid-joined merge block for `record_concepts`, placed with the other
  concept-link blocks (mirrors `project_concepts`'s block exactly, joining
  through `records`/`concepts` by `uuid` instead of `projects`/`concepts`).
- Add `"record_concepts"` to the `allTables` summary slice — per DR-0013,
  a table absent from that list is a table whose loss during merge goes
  unreported.

`jsonl.go`:

- New `recordConceptRecord` struct (`Type`, `RecordUUID`, `ConceptUUID`),
  mirroring `projectConceptRecord`.
- New `exportRecordConcepts` function, mirroring `exportProjectConcepts`
  (join `record_concepts` through `records`/`concepts` by internal id,
  select their uuids).
- New import branch (alongside the existing `recObservationConcept`/
  `recProjectConcept` handling) resolving both uuids via
  `resolveLocalID`/`conceptLocalID` and inserting through
  `INSERT OR IGNORE INTO record_concepts (record_id, concept_id) VALUES (?, ?)`.
- Wire the new export function into whatever calls
  `exportProjectConcepts`/`exportObservationConcepts` today (same call
  site(s), same scoping behavior).

### Tests to add

`knowledge_merge_test.go` — `TestMergeKnowledgeBases_RecordConcepts`,
mirroring `TestMergeKnowledgeBases_ProjectConcepts`'s structure (seed two
source databases, merge, assert exactly the expected row count and that it
appears in the summary).

`jsonl_test.go` — `TestExportRecordConcepts_RoundTrips` (export then
re-import into a fresh database, assert the link survives),
`TestImportRecordConcept_SkipsWhenEndpointMissing` (mirrors whatever the
existing `project_concept`/`observation_concept` import tests do for a
dangling uuid reference).

### Acceptance criteria

- `go test ./...` green, including `knowledge_merge_test.go` and
  `jsonl_test.go`.
- Re-run the W2 manual smoke-test record through an actual
  export → fresh-database import round trip (not just unit tests — this is
  the same "smoke-test data transforms on a real run" requirement as W2,
  applied to the merge/export path specifically) and confirm the
  `[[wikilink]]`-derived link survives.

---

## W5 — Documentation

### Files to modify

`cmd/kb/helptext.go`:

- `IngestHelpText` (line ~563): add a `DESCRIPTION` paragraph covering
  `[[Name]]` body scanning and `Tags` frontmatter resolution (design
  decision 3), and the case-insensitive-matching/first-write-wins-casing
  behavior (decision 4) — including a short example matching the
  `[[Computers]]`/`[[computer]]` case from the design conversation.
- `RecordHelpText` (line ~633): add `concepts RECORD_ID` to the `SYNOPSIS`
  block and a `concepts` bullet to the verb list in `DESCRIPTION`.

### Regeneration (do not hand-edit `.1.md` files — they are generated)

```bash
make kb-topics-help   # regenerates kb-ingest.1.md and kb-record.1.md
make man              # (if HTML/man output is otherwise part of the normal build)
```

### Acceptance criteria

- `git diff kb-ingest.1.md kb-record.1.md` shows only the expected
  additions, regenerated from the edited `helptext.go` constants — no
  hand-edits to either `.1.md` file.
