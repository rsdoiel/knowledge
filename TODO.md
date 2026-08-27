
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

- [ ] `kb record new` run from *inside* a project directory silently creates a
  nested corpus instead of finding the existing one. Found 2026-08-27, same
  session: run from `~/WorkLab/clasm`, `kb record new --title "…" --trigger
  request --kind decision --project clasm` reported `DR-0001 written to
  clasm/decisions/0001-….md` — having created `~/WorkLab/clasm/clasm/decisions/`
  and allocated id 0001, rather than seeing the 169 records already sitting in
  `~/WorkLab/clasm/decisions/`. `--root` defaults to the cwd and the path is
  built as `<root>/<project>/decisions`, so the command is behaving as
  specified; the problem is that the wrong invocation is indistinguishable
  from the right one in its output, and the failure mode is a duplicate
  corpus with colliding ids rather than an error. Re-running from
  `~/WorkLab` allocated DR-0170 correctly. Options: resolve the root by
  walking up for a directory whose name matches `--project` (or that contains
  a `<project>/decisions`), or refuse to allocate id 0001 into a
  `decisions/` directory that did not already exist without an explicit
  `--root`. The stray directory was removed by hand. Same root cause seen
  again the same day from a different verb: `kb observation add --project
  clasm ...` run from inside `~/WorkLab/clasm` failed with `project "clasm"
  not found` (it resolves `agents/knowledge.db` from the cwd, which inside a
  project is the wrong workspace) *and* left an empty `clasm/agents/`
  directory behind on the way out. Whatever the fix is, it wants to be at the
  path-resolution layer rather than per-verb: either walk up to the workspace
  root, or fail before creating anything.

- [ ] `kb search` exits 0 when it finds nothing. The workspace convention for
  search-style tools is exit 1 on no match (see `~/Laboratory/CLAUDE.md`),
  though that section is written for the Deno tools and may not be intended
  to bind `kb`. Worth a decision either way, since scripts branch on it.

## Done

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
