
# Action items

## Requested features

- [ ] Cross-machine reconciliation of an edited description. Deferred out of
  the `set-description` work (see DR-0012) because it is a policy inversion
  rather than a column touch. Today `MergeKnowledgeBases` is `INSERT OR
  IGNORE` set-union deduped by `uuid`/`name`, inserting `a` before `b` for
  every table, so **A wins deterministically** and `updated_at` is never
  consulted; `importProject` is explicitly the same policy, argued for in
  DR-0003 ("an existing local row wins as-is"). Beyond the policy, the
  columns are not there: only `projects` has `updated_at` — `concepts` and
  `observations` have no such column, and the JSONL export selects
  `created_at` without it on both tables. Doing it means (a) a lazy `ALTER
  TABLE concepts ADD COLUMN updated_at`, (b) adding it to `parentCols` in
  `MergeKnowledgeBases` and to the JSONL record structs, and (c) reversing
  first-wins to last-writer-wins in both `merge` and `import` for the mutable
  columns. Needs its own decision record, since it supersedes part of
  DR-0003. `kb project set-description` already records `updated_at`
  faithfully, so the groundwork is in place; nothing reads it across machines
  yet, and `kb-project(1)` says so under CAVEATS.

- [ ] Whether an observation body should be correctable at all, and if so
  whether by amendment or by supersession. Deliberately left out of DR-0012:
  an observation is a *timestamped* note, append-only by construction, and
  the `records` table already answers the same question with an explicit
  `supersedes` edge. Mutating `body` in place would decide it by accident.

## Noticed in passing

- [ ] `kb ingest` does not prune a relation removed from a record's
  frontmatter. Found 2026-08-27 authoring `clasm` DR-0170/DR-0171: both files
  initially declared each other in `relates_to`, so `kb record show 0170`
  printed `DR-0171` twice (the record's own forward edge plus the inverse of
  0171's). Removing `"0171"` from DR-0170's `relates_to` and re-running
  `kb ingest clasm/decisions` reported `1 updated` and `8 relates_to`
  (down from 9), but the row survived and the doubled display persisted —
  confirmed directly against `record_relations`, which still held both
  `0170 → 0171` and `0171 → 0170` while only the second was still declared on
  disk. Deleted the stale row by hand to finish the session. Ingest updates a
  record's own columns and inserts its current edges, but never deletes edges
  it no longer sees, so `record_relations` only ever grows: any relation ever
  ingested is permanent, and the database silently diverges from the record
  files that are supposed to be authoritative. The files were never wrong.
  Fix is presumably a delete-then-insert of the edges owned by the record
  being updated (`DELETE FROM record_relations WHERE from_id = ?` before
  re-inserting), scoped to forward edges only so the other side's own
  declarations survive. Worth checking whether `supersedes` has the same
  problem — it is written through a different path (`record supersede` writes
  both sides together), so it may not.

- [ ] `kb ingest` never updates `records.path` when a record file moves but
  its body is unchanged. Found 2026-08-28 migrating `clasm`'s 173-record
  corpus from `clasm/decisions/` to `agents/projects/clasm/decisions/` under
  the new workspace layout. `kb ingest agents/projects/clasm/decisions`
  reported `0 added, 0 updated, 172 skipped, 0 failed` and every one of the
  172 rows kept its old `clasm/decisions/...` path; a second run behaved
  identically. The cause is in `upsertAll` (`cmd/kb/ingest.go`): the
  checksum-match branch does `ing.summary.Skipped++` and
  `rec.dbID = existing.ID`, and the only write is guarded by
  `if !ing.dryRun && rec.dbID == 0`, so a skip can never write. A pure move
  doesn't touch the body, so the checksum always matches and the path is
  never revisited. Ingest already *detects* the move — the
  `existing.Path != rf.Record.Path` warning fires for all 172, naming both
  the old and new path — and then does nothing about it. This matters more
  now that `kb export` covers the records tables: `agents/knowledge.jsonl`
  was left holding 172 stale paths and zero current ones, and that is the
  versioned artifact. Fix is presumably to treat a changed path as its own
  reason to update — either widen the skip test to require
  `existing.Path == rf.Record.Path` as well as a matching checksum, or write
  the path on the skip branch. Worth deciding at the same time whether the
  warning should still fire once a move is handled as an ordinary update; as
  worded ("a slug may have been regenerated, or two files may claim one id")
  it reads like a possible id collision, which a plain move is not. Related
  to the relation-pruning item above: both are cases where ingest treats the
  record files as authoritative for content but lets the database keep
  something the files no longer say.

- [ ] `kb search` exits 0 when it finds nothing. The workspace convention for
  search-style tools is exit 1 on no match (see `~/Laboratory/CLAUDE.md`),
  though that section is written for the Deno tools and may not be intended
  to bind `kb`. Worth a decision either way, since scripts branch on it.

## Done

- [x] `kb record new`'s default write path moved to `agents/projects/<project>/decisions/`
  for project scope (`agents/decisions/` unchanged for `--workspace`), plus a
  `--dir` override. See `agents-projects-layout-feature-request.md` (filed
  2026-08-28), `DR-0021` and `DR-0022`, and
  `record-layout-and-workspace-init-plan.md`. Migrating existing corpora
  under the old shape (`clasm/decisions/`, `cold/decisions/`,
  `agents/decisions/caltechauthors/`) and whether `kb` should index `plans/`
  or `feature_requests/` at all are both explicitly out of scope, not
  overlooked — see `DR-0021`'s Consequences.

- [x] `kb record new` (and `kb observation add`) run from *inside* a project
  directory no longer silently creates a stray nested corpus or an empty
  ambient `agents/`. Fixed at the path-resolution layer, as this item itself
  suggested: `cmd/kb/main.go`'s ambient `--db` resolution now refuses to open
  (and so auto-create) a database that doesn't exist, naming `kb init` and
  `kb import -in FILE` instead — a wrong-cwd invocation fails loudly rather
  than fabricating a workspace where it happens to stand. See `DR-0021` item 4
  and `DR-0022` (the guard applies only to the true ambient default, not an
  explicit `--db PATH`).

- [x] `kb project set-description NAME DESCRIPTION` — projects had no way to
  correct a description after `add`, `set-status` being the only in-place
  mutation. Raised 2026-08-26 from the `dev-process` project, whose
  description named `DESIGN_DECIDE_PLAN.md` after that file was renamed to
  `DESIGN_REVIEW_PLAN_IMPLEMENT.md`. Refreshes the FTS row and touches
  `updated_at`. See DR-0012.

- [x] `kb concept add` no longer wipes a description. The request had listed
  `concept` among the add-only verbs; it was the opposite —
  `AddConceptWithIdentifier` carried `description = excluded.description`
  unconditionally while both identifier columns beside it were guarded, so
  `kb concept add NAME` with no description silently cleared the stored one
  in the row and in the FTS index. The guard now covers `description` too.
  See DR-0012.
