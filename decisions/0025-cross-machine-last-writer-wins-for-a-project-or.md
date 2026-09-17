---
id: "0025"
title: "Cross-machine last-writer-wins for a project or concept's mutable fields"
date: "2026-09-17"
status: accepted
kind: decision
trigger: request
project: knowledge
phase: ""
supersedes: ["0003"]
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: ["concepts gains updated_at, lazily migrated and backfilled to created_at, mirroring how projects already has it and how concepts.created_at was itself lazily added", "kb merge's projects/concepts INSERT OR IGNORE becomes INSERT ... ON CONFLICT DO UPDATE ... WHERE excluded.updated_at > existing.updated_at, so the row with the later timestamp wins regardless of which side (a or b) is applied first", "kb import's importProject/importConcept compare the incoming record's new UpdatedAt field against the local row's updated_at and adopt the incoming mutable fields (description, status, identifier_type/identifier_value) only when it is later, refreshing kb_fts either way", "RenameProject, RenameConcept, and AddConceptWithIdentifier's ON CONFLICT branch now touch updated_at, closing gaps DR-0024 and DR-0012 left open now that the column is load-bearing", "Observations are out of scope and the TODO note's framing of them is stale: DR-0023 already resolved observation correction via supersession, with full merge/JSONL coverage, so there is no edited body to reconcile and no updated_at needed on observations", "A rename crossing machines is explicitly out of scope: merge/import still dedupe projects and concepts by name, so a name changed on one machine does not reconcile against the same uuid's old name on another -- filed as a known follow-on, not solved here"]
tags: [request, projects, concepts, merge, portability, data-integrity]
uuid: "01a0b14d-1bd9-7288-808c-837c47847f78"
origin_host: "wren"
---

**Context.** Deferred out of the `set-description` work (DR-0012) as a policy
inversion rather than a column touch, and filed in `TODO.md` with three
things to do: add `updated_at` to `concepts` and `observations`, carry it
through `merge`/`import`, and reverse first-wins to last-writer-wins. Two of
those three claims don't survive rereading the current code.

**`updated_at` already travels through `merge` for `projects` — the gap is
the conflict rule, not the column.** `MergeKnowledgeBases`' `parentCols`
already lists `updated_at` for `projects`, and it is copied over exactly.
The actual defect is `INSERT OR IGNORE INTO projects (...) SELECT ... FROM
a.projects` run before the same statement for `b`: on a name/uuid conflict
SQLite silently drops whichever side runs second, in full, timestamp
unconsulted. **A wins deterministically not because A's edit is newer, but
because the loop visits `a` first.** `importProject`/`importConcept` are
the same shape at the Go level: each returns the existing local id the
moment a name match is found, before looking at anything else in the
incoming record.

**Observations need none of this — DR-0023 already answered the question
`TODO.md` was still asking when this item was filed.** The note assumed
observations would eventually need their own `updated_at` and a
last-writer-wins path once "correct an observation" was settled. It has
been: DR-0023 makes correction a new row plus a `supersedes` edge, and the
old row's body is never rewritten. There is no mutable field left on
`observations` for a timestamp to reconcile, and `observation_relations`
already has full `merge`/JSON-L coverage, shipped with that record. Treating
observations as still needing this work would be solving a problem DR-0023
retired.

**`concepts` really is missing the column, and it really is mutable in
place.** `kb concept add` on an existing name upserts (`ON CONFLICT(name) DO
UPDATE`, per DR-0012), so a concept's `description`/`identifier_type`/
`identifier_value` can be corrected the same way a project's can — but
`concepts` has no `updated_at` at all, added or not.

**Two more gaps, both freshly introduced and neither yet a problem in
practice, but both real once this record makes `updated_at` load-bearing.**
`RenameProject`/`RenameConcept` (DR-0024) update `name` without touching
`updated_at`. And renaming crosses a boundary this record does not attempt
to fix: `merge`/`import` dedupe projects and concepts by name, so a project
renamed on one machine and left untouched on another does not reconcile
against its own uuid — the merge sees two names, not one project with a
stale label. That is a harder problem (matching by uuid instead of by name
would change what "the same project" means to every table that joins
through it) and is filed as a follow-on, not solved here.

**Decision.** `concepts` gains `updated_at DATETIME`, added the same way
`concepts.created_at` was — a lazy `ALTER TABLE` with no default (SQLite
rejects a non-constant default on a table with existing rows) and a
one-time backfill, `UPDATE concepts SET updated_at = created_at WHERE
updated_at IS NULL`, so a never-touched row reads as "last touched at
creation" rather than null. `AddConceptWithIdentifier`'s `ON CONFLICT`
branch, `RenameProject`, and `RenameConcept` all now set `updated_at =
CURRENT_TIMESTAMP` alongside whatever else they write.

`kb merge`'s `projects`/`concepts` passes change from `INSERT OR IGNORE`
to `INSERT ... ON CONFLICT(name) DO UPDATE SET <mutable columns>, updated_at
= excluded.updated_at WHERE excluded.updated_at > <table>.updated_at`. Run
once for `a` then once for `b` as today, but order stops mattering: whichever
side's row is newer wins the conflict regardless of which pass reaches it
first, because the `WHERE` guard on the `DO UPDATE` refuses to overwrite a
newer local row with an older incoming one. Mutable columns: `description`
and `status` for `projects`; `description`, `identifier_type`,
`identifier_value` for `concepts`. `name` and `uuid` are identity, never
touched by conflict resolution; `origin_host` is treated as metadata about
the edit and follows whichever side wins.

`importProject`/`importConcept` gain the same rule at the Go level.
`projectRecord`/`conceptRecord` gain an `UpdatedAt string` field, exported
from the column that already exists (`projects`) or now exists
(`concepts`). On a name match, compare the incoming `UpdatedAt` against the
local row's; adopt the incoming mutable fields and refresh `kb_fts` only
when the incoming timestamp is strictly later, otherwise leave the local
row exactly as `importProject` does today.

**Rationale.** A single `ON CONFLICT ... DO UPDATE ... WHERE` guard is the
right shape for `merge` because it makes the two-pass loop's order stop
mattering — today's "a always wins" is an artifact of loop order, not a
policy anyone chose, and a conditional upsert removes the artifact instead
of papering over it with a second pass that re-checks timestamps by hand.
Mirroring `concepts.created_at`'s own lazy-migration shape for
`concepts.updated_at` costs nothing to a reader who already understands the
first one. Touching `updated_at` in `RenameProject`/`RenameConcept` now,
rather than waiting for evidence it is missed, is warranted here
specifically because this record is what turns `updated_at` from a
recorded-but-unread column into one three different code paths actively
compare — leaving two of the four mutation paths silently exempt would
undermine the guarantee the other two are being built to provide.

**Rejected alternatives.** Solving cross-machine rename reconciliation in
the same pass — a real gap, newly visible because of DR-0024, but a
different-shaped problem (identity by uuid instead of by name) that would
touch every table joining through `projects`/`concepts`, not a
`last-writer-wins` policy question; filed as its own follow-on. Adding
`updated_at` to `observations` anyway, for symmetry — there is no mutable
field left on that table to timestamp; a column nothing ever compares is
not groundwork, it is a migration to explain later, the same objection
DR-0012 raised against pre-adding it to `concepts` before there was a
consumer. Doing the conflict comparison as a `DELETE` of the losing row
followed by a plain `INSERT` — the `WHERE`-guarded `DO UPDATE` is one
statement, needs no explicit losing-side detection, and cannot leave the
table briefly without a row if the transaction is interrupted between the
two steps. Extending `MergeTableSummary` to report how many rows were
updated by conflict resolution versus inserted fresh — a real
improvement to the summary's honesty, but a separate, additive change to
its shape that does not block shipping the reconciliation itself.

**Consequences.** Implementation: the `concepts.updated_at` migration and
backfill in `knowledge.go`; `updated_at = CURRENT_TIMESTAMP` added to
`AddConceptWithIdentifier`'s `ON CONFLICT` `SET` clause, `RenameProject`,
and `RenameConcept`; the `projects`/`concepts` passes in
`MergeKnowledgeBases` rewritten from `INSERT OR IGNORE` to the guarded
`ON CONFLICT DO UPDATE`, refreshing `kb_fts` when a conflict update actually
changes a row; `projectRecord`/`conceptRecord` gain `UpdatedAt`, exported by
`exportProjects`/`exportConcepts`; `importProject`/`importConcept` gain the
timestamp comparison and the matching `kb_fts` refresh.
`kb-project(1)`/`kb-concept(1)`'s CAVEATS sections (added by DR-0012) are
corrected or removed, since the thing they warn does not happen stays true.
`TODO.md`'s item is corrected the way DR-0024 corrected its own predecessor
note: the observations half was already resolved by DR-0023, and the
renamed-across-machines gap is carried forward as new, explicitly separate,
future work.
