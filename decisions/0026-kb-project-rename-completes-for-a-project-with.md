---
id: "0026"
title: "kb project rename completes for a project with records; rename reconciles across merge and import"
date: "2026-09-18"
status: accepted
kind: decision
trigger: live-test
project: knowledge
phase: ""
supersedes: ["0024", "0025"]
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: ["kb project rename OLD NEW, when the project owns records, rewrites every owned record's project: frontmatter in place before renaming the project row -- both-or-neither, --dry-run supported. It never touches records.project_id or does any per-record database write: that FK is stable across a rename, so the corpus rewrite is a pure file operation and the next ordinary kb ingest refreshes cached checksums through its existing identity-matched UPDATE path, never the INSERT path that collided", "New library method RenameProjectRow(old, new) does the DB-only rename without the records-owning guard, called only after the CLI has confirmed every file was rewritten; RenameProject keeps its guard as the safe default for a caller that has not rewritten the corpus", "kb merge and kb import's projects/concepts conflict resolution moves from name-keyed to uuid-primary: a uuid match means the same entity, and its mutable fields -- now including name -- reconcile by updated_at, generalizing DR-0025. A uuid miss falls back to name, DR-0003's original policy for two genuinely different, never-synced entities that happen to collide, left unchanged", "This closes two bugs traced live, not merely suspected: kb merge can silently keep the stale name (or drop a side entirely) depending on merge order, because INSERT OR IGNORE swallows a uuid collision it cannot distinguish from a name collision; kb import hard-fails the entire import, not just one row, on the same collision, since importProject's INSERT (unguarded) is not OR IGNORE"]
tags: [request, projects, concepts, merge, portability, data-integrity, live-test]
uuid: "01a0b67a-38f6-7711-8cde-ef36e62cac11"
origin_host: "wren"
---

**Context.** Filed as a `TODO.md` finding 2026-09-17, executing WorkLab's
accepted `codemeta` → `codemetatools` rename (DR-0006 there, accepted and
until now unexecuted): `kb project rename` correctly refuses a project that
owns records (DR-0024 — a real corruption risk, proven live), but the manual
path its own refusal message recommends — rewrite the corpus's `project:`
frontmatter, re-ingest — is itself blocked. Re-ingesting a record whose
`project:` field no longer matches its database row makes `upsertAll` treat
the file as a *new* record under `(workspace, project_id, scope,
record_id)` identity, but the file still carries its old, stable `uuid`, and
`records.uuid` is `UNIQUE` — the `INSERT` collides. A first attempt is worse
than a no-op: the failed ingest still creates an empty project under the new
name (`projectID` resolves a frontmatter name to a row, creating one when
absent), which then trips rename's *other* guard (`NEW` already exists),
leaving the database stuck until the stray row is removed by hand — for
which `kb` has no verb.

**The fix is simpler than either shape sketched in the finding, because of
one fact neither considered: `records.project_id` never needs to change on
a rename.** It is a stable foreign key to `projects.id`, and renaming only
changes `projects.name`. The file's `project:` field is the only thing that
goes stale — a *name*, read fresh from disk each `ingest` run, not an
identity. So the corpus rewrite the manual path needs never has to go
through `ingest`'s identity resolution at all: it can edit the file in place
and leave the database record untouched, and the very next ordinary
`kb ingest` of that corpus will see a changed checksum against an unchanged
identity — the ordinary UPDATE path, not the INSERT path that collides.

**Tracing the merge/import code while designing this turned up two further
bugs, not yet in `TODO.md`, that a rename made routine would make routine to
hit.** Both `MergeKnowledgeBases` and `ImportJSONL` treat a project's or
concept's *name* as its identity for conflict resolution — reasonable under
DR-0003, before any rename verb existed. `projects.uuid` and `concepts.uuid`
each carry their own `UNIQUE` index, separate from `UNIQUE(name)`. Traced
live in both directions:

- **`kb merge` can silently keep the stale name, or drop a side outright,
  depending on merge order.** DR-0025's `INSERT OR IGNORE` for new rows does
  not distinguish *which* constraint it is ignoring. If project X (one
  `uuid`) is renamed on machine A but not B, merging A-then-B lets A's insert
  land first and B's second insert — same `uuid`, different `name` — is
  silently swallowed by the same `OR IGNORE`, with no error and no summary
  line naming it. Merging B-then-A produces the *opposite*, wrong, and
  non-deterministic result: the stale name wins.
- **`kb import` hard-fails the entire import, not one row.** `importProject`
  matches by name only; a renamed project's `name` does not match locally, so
  it falls into a plain, unguarded `INSERT` — which collides on the `uuid`
  index and returns a real SQL error. `ImportJSONL`'s per-project loop
  returns that error immediately, aborting the whole call, including every
  unrelated project, observation, and record later in the same file.

**Decision.** `kb project rename OLD NEW`, when `OLD` owns records: loads
every owned record's file, rewrites its `project:` field to `NEW`, and
writes all of them back — both-or-neither, restoring any already-written
file if a later one fails, and touching no database row for any record in
the process. Only once every file is confirmed written does it rename the
project row. `--dry-run` reports the file count and paths without writing
anything. The zero-records case is unchanged. A new library method,
`RenameProjectRow(old, new string) error`, does the bare database rename
without the records-owning guard — the CLI calls it only after the corpus
rewrite has already succeeded; `RenameProject` keeps its guard exactly as
DR-0024 shipped it, the safe default for any caller (library consumer,
script) that has not rewritten a corpus itself.

`kb merge`'s and `kb import`'s conflict resolution for `projects` and
`concepts` moves from name-keyed to uuid-primary. In `merge`: for each side,
first `UPDATE ... FROM src WHERE local.uuid = src.uuid AND src.updated_at >
local.updated_at` — reconciling every mutable field, `name` now included,
exactly as DR-0025 already does for `description`/`status` — then
`INSERT OR IGNORE ... WHERE NOT EXISTS (uuid already present)` for rows
genuinely new to the target. Running both steps for `a` then `b` is
order-independent: whichever side's edit is actually newer wins the `name`
regardless of which pass reaches the row first. In `import`:
`importProject`/`importConcept` look up by `uuid` first; a match reconciles
by `updated_at` the same way, `name` included; a miss falls back to today's
name lookup, preserving DR-0003's "an existing local row wins as-is" for two
genuinely different, never-synced entities that happen to share a name —
that path is untouched.

**Rationale.** The two candidate shapes the finding sketched — route through
`ingest` with a `uuid`-aware reparenting step, or invent a re-home-on-typo
fallback — both assumed the database side needed to change. It does not:
`project_id` survives a rename unmodified, so there is nothing for `ingest`
to reparent and nothing for a rename to get subtly wrong by re-deriving
identity. Treating the corpus rewrite as a pure file operation is not just
simpler, it is *correct* in a way routing through `ingest` cannot be,
because it never asks the identity-resolution code to do something it was
never designed to do (recognize the same record under a changed name).
Fixing `merge`/`import` in the same pass, rather than shipping the deadlock
fix alone and filing cross-machine reconciliation as its own future item
again, is what the tracing changed: those two bugs are not hypothetical, and
a rename that actually completes turns them from an edge case into
something a two-machine workflow will hit the first time anyone renames a
project with records on one side and merges or exports before the other side
catches up. Shipping the completion without the reconciliation would make
the corruption risk *worse*, not smaller, by making the trigger routine.

**Rejected alternatives.** Having `ingest` match on `uuid` first and
`UPDATE project_id` when it differs from what the file's current `project:`
resolves to — the finding's own alternative shape, rejected there and here:
it would quietly re-home a record on any frontmatter typo, a worse default
than refusing. Shipping the deadlock fix alone and leaving cross-machine
reconciliation deferred, as DR-0025 originally left it — the plan going in,
revised once tracing the actual merge/import code turned up two live bugs
rather than a merely theoretical gap. A general "rename event" log, letting
any future rename-like operation reconcile the same way — solves a bigger
problem than there is evidence for; `uuid`-primary identity resolves the
concrete bug without a new primitive. Changing the rare same-name,
different-`uuid` collision's own resolution (today: whichever side's insert
lands first keeps the name) to also be timestamp-based — a real asymmetry
with the `uuid`-match case, but a separate, much rarer scenario (two
independently created, never-synced entities) with no evidence of causing
a problem, unlike renaming.

**Consequences.** Implementation: `RenameProjectRow` factored out of
`RenameProject`'s existing rename+FTS-refresh logic in `knowledge.go`;
`cmdProjectRename` in `cmd/kb/project.go` gains the load-rewrite-write-then
rename-row orchestration and `--dry-run`; `knowledge_merge.go`'s
`projects`/`concepts` passes restructured to the UPDATE-by-uuid-then-
INSERT-if-absent shape; `importProject`/`importConcept` in `jsonl.go`
restructured to look up by `uuid` before falling back to `name`.
`reportMissing`'s message, found still naming the nonexistent `kb record
remove` while reproducing this, no longer does. New tests across
`knowledge_test.go`, `knowledge_merge_test.go`, `jsonl_test.go`, and
`cmd/kb`'s own suite, per this project's TDD discipline. `TODO.md`'s
rename-deadlock item is corrected and closed once this ships.
