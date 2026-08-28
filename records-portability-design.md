# Records portability: teaching `merge`, `export` and `import` about decision records

Design brief. Nothing here is agreed — this is input to a design review.

## Problem

`agents/knowledge.db` has nine real tables. All three portability paths
carry seven of them:

| Table | `merge` | `export` / `import` |
|---|---|---|
| `projects` | yes | yes |
| `concepts` | yes | yes |
| `sources` | yes | yes |
| `observations` | yes | yes |
| `observation_concepts` | yes | yes |
| `project_concepts` | yes | yes |
| `observation_sources` | yes | yes |
| `records` | **no** | **no** |
| `record_relations` | **no** | **no** |

The cause is ordering, not disagreement. `merge` landed 2026-07-26..28 and
JSON-L export/import was decided in DR-0003 on 2026-08-08; decision records
arrived afterwards, DR-0004 through DR-0011 on 2026-08-25..26. Nothing went
back to close the loop.

Two consequences, both live today:

**`kb merge` loses decision records while reporting success.** The seven
merged tables are hardcoded in `knowledge_merge.go:246`, and `records` is not
among them — nor is it in the per-table summary, so the output says nothing
about the table it dropped. The merged file is created by `Open()`, so it has
a `records` table; that table is simply empty. Merging this machine's database
today would silently discard all 13 records.

**Export is not the thing DR-0003 says it is.** That record positions JSON-L
export/import as "the no-file-access alternative to `merge`". An export that
omits `records` cannot round-trip a database that has them, so the alternative
is not equivalent to the thing it is an alternative to.

The workaround — re-run `kb ingest` against each `decisions/` directory after
a merge — happens to work, because records derive from Markdown files under
git. It is not a fix. It requires file access, which is exactly what DR-0003
says one of these paths must not require, and it depends on the operator
knowing about a data loss the tool did not report.

## Constraints

**DR-0003 parity.** `merge` and `export`/`import` are two routes to one
outcome. Whatever identity, conflict and ordering rules records get, both
routes must implement the same ones, or the two paths diverge in a way the
next cross-machine sync discovers the hard way.

**The existing conflict policy is already recorded and should not be
re-litigated here.** A knowledge-base observation notes that `kb merge` and
`kb import` both resolve conflicts in favour of the row already present and
never consult `updated_at` — deterministic A-wins, not arbitrary. Records
should follow that policy by default. Changing it is a separate decision
affecting two code paths and a migration.

**The ingest/format write boundary holds.** `kb ingest` never writes to a
record file and `kb record fmt` never writes to the database. A portability
path that carries records becomes a third writer of the `records` table; it
must write only the database, never a `decisions/*.md` file, even when it
holds a record body that disagrees with the file on disk.

**Records must travel without their files.** The `records` row carries `body`
and `checksum`, so the database is already self-sufficient. Nothing in this
work should reintroduce a file dependency.

## Technical findings that shape the approach

**Records have two unique keys, not one.** `idx_records_uuid` on `uuid`, and
`idx_records_identity` on `(workspace, IFNULL(project_id,-1), scope,
record_id)`. That is structurally the same situation as `projects.name` and
`concepts.name` — two rows that are the same record logically but arrived at
different uuids on different machines. `merge` already has machinery for
exactly this shape: `CollisionReport` plus `ReconcileCollisions` behind
`-force`. Extending that machinery to a third table looks cheaper than
inventing a records-specific path, but the identity tuple is four columns
rather than a single `name`, so `NameCollision` would need to generalise.

**A naive copy of the `observations` pattern would drop workspace-tier
records.** `merge` moves observations with an inner join through
`projects`, which is correct there because `observations.project_id` is
always populated. `records.project_id` is nullable by design — a workspace-tier
record has no project — and this machine's database has one such record today
(`agents/decisions/`, DR-0001). An inner join silently drops it. Records need
a left join plus explicit NULL handling, and a test that fails on the
inner-join version specifically.

**`record_relations` is a pure id-keyed join table** — `(from_id, to_id,
relationship)`, both endpoints referencing `records(id)`. It remaps through
record uuids exactly the way `observation_concepts` remaps through observation
and concept uuids. No new pattern needed; it does need the same
unresolvable-reference-is-skipped semantics the other join tables have.

**Rows landing in the table is not enough — they must land in `kb_fts`.**
`kb_fts` has no triggers; it is maintained by explicit inserts at each write
site, with `rebuildFTSIfNeeded` as a backstop that fires only when the index
is empty. `merge` depends entirely on that backstop, and the backstop indexes
observations, projects and concepts only. So a merge that correctly unions
`records` would still produce a database where `kb search` cannot see them —
`source_type = 'record'` rows would be missing. `import` inserts FTS rows
explicitly per type and would need the same addition. This is not a
follow-up; without it the feature does not do what a user would check.

**Naming hazard in the JSON-L layer.** `jsonl.go` already uses "record" to
mean "one JSON-L line" — `projectRecord`, `observationRecord`, `recProject`.
Decision records need a type discriminator too, and the mechanical name
(`recordRecord`, `recRecord`) is unreadable. Worth settling deliberately
rather than by whatever the first patch types.

## Proposed approach

Four pieces of work, TDD throughout, each with a failing test written first:

1. **Generalise the collision machinery** from `projects`/`concepts` name
   collisions to include records under their four-column identity, so
   `CollisionReport` and `-force` reconciliation cover the new table.
2. **Union `records` and `record_relations` in `MergeKnowledgeBases`**, with
   the left join for nullable `project_id`, and add both tables to the summary
   so the operator sees the counts.
3. **Add both tables to JSON-L export and import**, matching the merge
   semantics exactly, with a round-trip test that a database containing
   project-tier and workspace-tier records exports and re-imports unchanged.
4. **Index records into `kb_fts` on both paths**, including in
   `rebuildFTSIfNeeded`, with a test that asserts `kb search` finds a record
   that arrived via merge and one that arrived via import.

An end-to-end test worth having regardless of the above shape: merge two
databases where each side holds records the other lacks, then assert the
merged database has the union and `kb search` reaches all of them. That is
the scenario that motivated this work and it currently has no coverage.

## Open questions

1. **Does the `-force` semantics change meaning for records?** For projects
   and concepts, `-force` reconciles b's identity to a's so both sides' child
   rows survive under one parent. Records have a `checksum` and a `body`, so
   the two sides can be *known* to differ in content rather than merely
   suspected. Should a checksum mismatch on an otherwise-identical identity
   be reported differently from a plain uuid collision — or even block the
   merge — given that a decision record's text is the artifact?

2. **What is the right JSON-L type discriminator name**, given `jsonl.go`
   already spends the word "record" on its line envelopes? `"record"` as the
   wire value matches `kb_fts.source_type` and `kb search` output, which
   argues for keeping it on the wire and solving the collision only in the Go
   identifier.

3. **Should `merge` refuse to run against a database whose schema it does not
   fully cover**, rather than relying on this fix being complete? A version or
   table-set guard would have turned this silent loss into an error message.
   That generalises beyond records and may deserve its own record.

4. **Does `sources` belong in `rebuildFTSIfNeeded` too?** It is absent today
   and no `source_type = 'source'` rows exist in this machine's index, so
   sources are unsearchable. Pre-existing and unrelated to records — flagged
   here only because the fix touches the same function, and the decision to
   leave it alone should be deliberate.

5. **Is there a schema-coverage test worth adding** that enumerates
   `sqlite_master` and fails when a table is added without appearing in the
   merge and export paths? That would prevent the next feature from silently
   repeating this. It would also need an explicit opt-out list for tables that
   genuinely should not travel, such as the `kb_fts` shadow tables.
