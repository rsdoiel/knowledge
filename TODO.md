
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
