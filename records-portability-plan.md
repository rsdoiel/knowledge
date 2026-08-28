# Records portability — Implementation Plan

## Source design

`records-portability-design.md` and **DR-0013** (`proposed` as of 2026-08-27 —
this plan assumes it is promoted unchanged; if the content-divergence or
scoping decisions move, W4 and W5 move with them).

Related records this plan must not contradict: **DR-0003** (export/import is
the no-file-access alternative to `merge`, so the two paths stay equivalent),
**DR-0004** (`IFNULL(project_id, -1)` — the NULL is real), **DR-0008**
(`source_type` in search output), **DR-0011** (a record's identity is
`(workspace, project, scope, record_id)`, and the workspace name is derived
from the path, never stored).

TDD throughout: the `*_test.go` change lands before the implementation it
covers, red confirmed first, matching this module's existing practice.

## Status (2026-08-27)

**W1 done and green.** `NormalizeForMerge` in `knowledge_merge.go`, `Open`
refactored to `openWithWorkspace` in `knowledge.go`, wired into `runMerge`
before `CollisionReport`. Five tests: three on the normalisation itself, two
at the CLI level. `go vet` and `gofmt` clean.

**W2 done and green.** `records` and `record_relations` unioned in
`MergeKnowledgeBases`, both added to the summary, which now covers nine
tables. Eight tests. `go vet` and `gofmt` clean.

**W3 done and green.** `rebuildFTSIfNeeded` indexes records as
`source_type = 'record'`, mirroring `indexRecordFTS`, and counts them towards
the "anything to index" total. `sources` stays out, now asserted rather than
merely absent. Four tests.

**W4 done and green.** `NameCollision` is now `IdentityCollision` with a
`Label`; `CollisionReport` covers records, matching their project by name
rather than by local id or uuid; `ReconcileCollisions` reconciles by uuid
alone; `DivergenceReport` is new; the CLI reports divergences on both the
success and abort paths and the JSON envelope gains `content_divergences`.
Nine tests.

W5–W6 not started. DR-0015 (`kb index` does not recurse) was taken between
W1 and W2, out of band, from two pre-existing test failures.

## What writing the plan changed, that the design did not anticipate

Two findings. Both are places where implementing DR-0013 as written would
fail, and neither is visible from the design brief.

1. **`merge` never migrates its inputs, so `b.records` may not exist.**
   `checkpointAndCopy` copies each source file with `sql.Open` and a
   `PRAGMA wal_checkpoint`, not with `knowledge.Open`, so no lazy migration
   runs on the scratch copies. Only the *output* is created through `Open()`.
   Every table `MergeKnowledgeBases` names today exists in every database old
   enough to be merged, so this has never mattered. `records` breaks that:
   wren.local's database had no `records` table at all as recently as this
   week, and `SELECT ... FROM b.records` against it is a hard error, not an
   empty result. The union cannot be written until the inputs are normalised.
   This becomes W1.

2. **Normalising the scratch copies naively would corrupt the workspace
   name.** `applyRecordsMigration(db, workspace)` backfills `records.workspace`
   from the workspace it is handed, and per DR-0011 that name is
   `filepath.Base` of the database's root — *derived from the path*. A scratch
   copy lives at `/tmp/kbmerge-XXXX/a.db`, so migrating it in place would
   backfill `workspace = "kbmerge-XXXX"` for any record that predates W8, and
   those records would then collide with nothing and merge as strangers. The
   window is narrow (a database holding pre-W8 records, which neither of this
   workspace's machines has today) but the failure is silent and permanent in
   the merged file. The workspace must be derived from the *original* `-a`/`-b`
   paths and passed into the migration of the copy.

Finding 2 is the kind of thing DR-0011's "derived, never stored" buys almost
everywhere and costs exactly here, where the file is deliberately not where it
came from. Both findings, and what to do about them, are **DR-0014**
(`plan-review`, `proposed`).

---

## W1 — Normalise the inputs before ATTACH (DR-0014, DR-0013, DR-0011)

Prerequisite for everything after it. Nothing else in this plan works against
a database that predates the `records` table.

### Files

- `knowledge_merge.go` — new unexported `normalizeForMerge(scratchPath, workspace string) error`
- `cmd/kb/merge.go` — derive each side's workspace before `checkpointAndCopy`
- `knowledge_merge_test.go`, `cmd/kb/merge_test.go`

### Work items

- Derive the workspace name for each side from the **original** `aPath`/`bPath`
  the same way `kb.Workspace()` does, before the copy. Thread it to the copy.
- After `checkpointAndCopy`, open each scratch copy through the normal lazy
  migration path with that derived workspace, so a pre-records database gains
  empty `records` and `record_relations` tables and a pre-W8 one backfills
  `workspace` correctly.
- `-a` and `-b` remain untouched on disk. Migrating a throwaway copy is not a
  violation of that promise, and the man page's "reads both read-only" wording
  should be checked against what it now does.

### Acceptance criteria

- A test merging a fixture database with **no `records` table** against one
  with records succeeds instead of erroring, and yields the records side's rows.
- A test asserting a pre-W8 fixture's records land under their real workspace
  name and **not** under the temp directory's name. This is finding 2; it must
  fail against the naive implementation.
- `-a` and `-b` are byte-identical before and after a merge.

---

## W2 — Union `records` and `record_relations` (DR-0013)

### Files

- `knowledge_merge.go` — `MergeKnowledgeBases`
- `knowledge_merge_test.go`

### Work items

- Add `records` to the parent-table pass, keyed by `uuid`, moving through
  `projects` with a **LEFT JOIN** so a NULL `project_id` survives. Columns:
  `record_id, project_id, scope, path, title, date, status, kind, "trigger",
  phase, initiative, session, body, checksum, ingested_at, uuid, origin_host,
  workspace`. Note `"trigger"` is quoted — it is a SQL keyword.
- Add `record_relations`, remapping both endpoints through record uuids
  exactly as `observation_concepts` remaps its two, keeping
  unresolvable-reference-is-skipped.
- Add both to `allTables` so they appear in the summary. This is what would
  have made the original loss visible.

### Acceptance criteria

- Union counts correct for records present on one side, both sides, and
  neither.
- **A workspace-tier record (NULL `project_id`) survives.** The test asserts
  the row is present, not just that the count is right — 12 of 13 arriving
  passes a count check. It must fail against an INNER JOIN.
- A `record_relations` row whose target is absent from both sides is skipped,
  not fatal.
- The summary lists nine tables.

---

## W3 — Records reach `kb_fts` on the merge path (DR-0013, DR-0008)

Without this the merged database has the rows and `kb search` finds none of
them, which is the first thing anyone checks.

### Files

- `knowledge.go` — `rebuildFTSIfNeeded`
- `knowledge_test.go`

### Work items

- Add records to `rebuildFTSIfNeeded` as `source_type = 'record'`, matching
  the body/label/descr shape `indexRecordFTS` already writes so a merged
  record and an ingested one are indistinguishable in the index. Prefer
  reusing that shape over restating it.
- Include `records` in the "is there anything to index" total, so a database
  holding only records still triggers the rebuild.
- Leave `sources` absent — DR-0013 decided this deliberately. Add a comment
  saying so, so the next reader does not read it as an oversight.

### Acceptance criteria

- After a merge, `kb search` returns a record that arrived from each side.
- A record's search output is byte-identical whether it arrived via `ingest`
  or via `merge`.
- No `source_type = 'source'` rows appear.

---

## W4 — Collision reporting: identity, and content divergence (DR-0013)

### Files

- `knowledge_merge.go` — `NameCollision`, `CollisionReport`, `ReconcileCollisions`
- `cmd/kb/merge.go` — abort message, `-force` narration, JSON envelope
- `knowledge_merge_test.go`, `cmd/kb/merge_test.go`

### Work items

- Generalise `NameCollision` from a single `Name` to an identity that can be
  four columns. `projects`/`concepts` keep matching on `name`; `records` match
  on `(workspace, IFNULL(project_id,-1), scope, record_id)`, with a display
  label the CLI can print in the existing fixed-width table. The type name
  stops being about names.
- `ReconcileCollisions` currently does `UPDATE <table> SET uuid=? WHERE name=?
  AND uuid=?`. For records the predicate becomes the identity tuple, with the
  same `IFNULL` treatment, so a workspace-tier record is reconcilable.
- Add the **content divergence** class: same identity, different `checksum`.
  Reported separately from an identity collision, naming the record and both
  checksums; does **not** block, with or without `-force`. A-wins stands.
- CLI: divergences print in both the abort path and the `-force` path (they
  are informational either way, so they must not be swallowed when `-force`
  suppresses the abort). JSON gains a field alongside `collisions_reconciled`
  — an additive change, but it is the tool's machine-readable contract and
  should be called out in `CHANGES.md`.

### Acceptance criteria

- Two databases whose `harvey` project has different uuids still abort without
  `-force` and reconcile with it — the existing behaviour, unchanged.
- Two databases holding DR-0007 under different uuids collide on identity and
  reconcile under `-force`.
- Two databases holding DR-0007 with **the same** identity and different
  bodies produce a divergence report, a successful merge, and A's body.
- The divergence appears in `--json` output and in both text paths.

---

## W5 — JSON-L export and import (DR-0013, DR-0003)

Must reach the same end state as W2–W4 by the other route, or DR-0003's
equivalence is broken in a way only a cross-machine sync discovers.

### Files

- `jsonl.go` — type discriminators, two new line structs, `exportRecords`,
  `exportRecordRelations`, `importRecord`, `importRecordRelation`, dispatch
- `jsonl_test.go`

### Work items

- Wire values are `"record"` and `"record_relation"` — `"record"` matches
  `kb_fts.source_type` and `kb search` output. The Go identifiers avoid the
  `recordRecord` collision with `jsonl.go`'s existing use of "record" for a
  line envelope; pick a naming convention once, in this phase, and apply it to
  both structs.
- References travel by uuid, as everything else does: a record carries its
  project's uuid (empty for workspace tier), a relation carries both records'.
- Import applies records after projects and relations after records, in the
  existing buffer-then-apply-in-dependency-order structure. Reuse
  `resolveLocalID` for endpoint lookup and **`indexRecordFTS` for the FTS
  write**, rather than open-coding an insert as the other import paths do —
  that is what keeps an imported record identical to an ingested one.
- Conflict resolution is uuid-keyed and already-present-wins, matching both
  observations and W4.

### Decision point

`ExportJSONL` takes a `projectName` and every helper takes `scoped`. A
workspace-tier record has no project, so a scoped export has no principled
claim on it. **Proposal: a scoped export carries only that project's records
and no relations crossing out of them; workspace-tier records appear only in
an unscoped export.** The alternative — always including the workspace tier —
makes a scoped export non-portable in a different way, since the importing
side may not be the same workspace. This wants confirming before W5 starts,
and if it is contentious it belongs in its own record rather than in a
plan bullet.

### Acceptance criteria

- Round trip: a database with project-tier and workspace-tier records and at
  least one relation exports and re-imports into an empty database unchanged,
  including `checksum` and `workspace`.
- Re-importing the same stream twice reports everything skipped, not
  re-added — the idempotency property the whole UUID migration exists for.
- An unrecognised line type is still skipped, not fatal.
- A relation whose endpoint never arrives is skipped, not fatal.

---

## W6 — End to end, and documentation (DR-0013)

### Work items

- The acceptance test named in DR-0013, which has no coverage today: two
  databases, each holding records the other lacks, one of them workspace-tier.
  Merge, then assert the union, the surviving workspace-tier record, and that
  `kb search` reaches every record from both sides.
- The same scenario by the export/import route, asserting the same end state.
  This is DR-0003's equivalence expressed as a test rather than as a claim.
- `kb-merge.1.md`, `kb-export.1.md`, `kb-import.1.md`: records now travel; the
  divergence report; whatever W1 changed about the read-only wording.
- `CHANGES.md`, and the `--json` envelope change from W4.
- `go build`/`go vet`/`gofmt`/`go test` clean. `go test -race` is expected to
  remain blocked on the Pi by the pre-existing ThreadSanitizer/VMA issue, not
  by anything here.

### Acceptance criteria

- Both routes reach the same database state from the same inputs.
- A real rehearsal against copies of this machine's and wren.local's actual
  databases, merged into a scratch file, before anything is copied into place.

---

## Resuming from here (written 2026-08-27, end of W4)

Everything needed to finish is in this repository. Read, in order:
`records-portability-design.md` (the problem), then **DR-0013** (what was
decided), then **DR-0014, DR-0016, DR-0017, DR-0018** (what implementing
W1–W4 changed about it), then this plan. `decisions/index.md` lists all
eighteen records newest first. `kb search "portability"` reaches them once
the corpus is ingested.

**Ground first, from `~/Laboratory`:**

```shell
kb search "portability"
kb record show 0013 --project knowledge
head -20 knowledge/decisions/index.md
```

**State of the code.** W1–W4 are implemented and green; `go vet`, `gofmt`
and `go test ./...` are clean from inside `knowledge/`. Two tests skip when
`~/WorkLab` is absent, which is expected on any machine that is not
macmini-rd.

| Where | What changed |
|---|---|
| `knowledge.go` | `Open` split over `openWithWorkspace`; `rebuildFTSIfNeeded` indexes records |
| `knowledge_merge.go` | `NormalizeForMerge`; `IdentityCollision`; `DivergenceReport`; records + record_relations in `MergeKnowledgeBases` |
| `cmd/kb/merge.go` | normalisation call; divergence reporting; `content_divergences` in `--json` |
| `cmd/kb/index.go`, `cmd/kb/ingest.go` | `collectRecordFilesIn` — index stops at the directory given (DR-0015) |
| `cmd/kb/testdata/index-corpus/` | golden corpus + `index.golden` |

**What is left is W5 and W6 below.** W5 is the larger piece: `jsonl.go`
currently carries the same seven tables `merge` used to, and must reach the
same end state W2–W4 gave the merge path, or DR-0003's equivalence is broken
in a way only a cross-machine sync discovers.

**The one thing not decided.** W5's decision point — whether a
`--project`-scoped export carries workspace-tier records, which have no
project to be scoped by. The proposal below is that it does not. Settle this
before writing W5; if it turns out contentious it wants its own record.

**Three findings from W1–W4 that W5 must honour**, each already argued in the
record cited:

1. A record's identity **across databases** is `(workspace, project name,
   scope, record_id)` — not `project_id`, which is a local autoincrement key,
   and not the project's uuid, which is order-dependent on reconciliation
   (DR-0018). Import must resolve a record's project the same way.
2. A record held by two machines is **essentially always** an identity
   collision, because each machine's ingest mints its own uuid for the same
   file (DR-0018). Import's conflict handling should expect that as the
   normal case, not the exception.
3. The FTS row for a record is written in two places already
   (`indexRecordFTS` and `rebuildFTSIfNeeded`) and must not become three.
   Import should call `indexRecordFTS`, not open-code an insert the way the
   other import paths do (DR-0017).

**Verifying a change:**

```shell
cd knowledge && gofmt -l . && go vet ./... && go test -count=1 ./...
```

**Closing a phase**, per `~/Laboratory/DESIGN_REVIEW_PLAN_IMPLEMENT.md`:
write the record with `kb record new --project knowledge --trigger
implementation`, leave it `proposed` for the author to promote, then
`kb record fmt knowledge/decisions`, `kb ingest knowledge/decisions`,
`kb index knowledge/decisions`.

**Not done and deliberately so:** the real cross-machine merge with
wren.local has not been run, and neither machine's `agents/knowledge.db` has
been modified. The installed `~/bin/kb` on both machines predates all of this
and needs rebuilding before any real run.

## Sequencing note

W1 gates W2. W3 and W4 are independent of each other and both depend on W2.
W5 depends on W2 and W3 for the semantics it must match, but not on W4's CLI
work. W6 is last.

The pending cross-machine merge does not have to wait for any of this — the
`kb ingest` workaround in `records-portability-design.md` still restores the
records, and W1's finding does not affect a merge that never touches the
`records` table. But once W2 lands, a merge run against an unmigrated database
changes behaviour, so the rehearsal in W6 should happen before the real one.
