
# Action items

## Requested features

- [x] `[[double-bracket]]` inline concept tagging when ingesting Markdown
  record bodies, beyond formal frontmatter. Filed as
  `wikilink-tagging-feature-request.md` (2026-09-08) — inspired by
  [Build a digitally sovereign second brain](https://www.raspberrypi.com/news/build-a-digitally-sovereign-second-brain/)
  (Raspberry Pi magazine). Implemented 2026-09-13: see
  `wikilink-tagging-design.md` and `wikilink-tagging-plan.md` (W1-W5). Both
  `[[Name]]` body scanning and the previously-unused frontmatter `tags` list
  resolve into `record_concepts`, case-insensitively via a new
  `ResolveConceptName` (not `AddConcept`/`kb concept add`, which stay
  exact-match); visible via `kb record concepts RECORD_ID`; carried through
  `knowledge_merge.go` and `jsonl.go` export/import for portability.
  Observation bodies (`kb observation add`) remain out of scope, as decided
  at filing.

- [x] Concept-tag-based retrieval query (concept names → linked
  observations/records). Filed as `concept-tag-retrieval-feature-request.md`
  (2026-09-08). Implemented 2026-09-13: see `concept-tag-retrieval-design.md`
  and `concept-tag-retrieval-plan.md` (W1-W2). New `retrieval.go`:
  `MatchConceptNames(text)` (whole-word, case-insensitive match against
  known concepts) and `RecallByConceptNames(names, limit)` (merged,
  match-count-then-recency-ranked results across both `observation_concepts`
  and `record_concepts`, read-only — never creates a concept, unlike
  `ResolveConceptName`). Not project-scoped, per the original motivation.
  Consuming this from harvey's `UnifiedMemory.Recall` is separate,
  harvey-side work, not done here.

- [x] Ingest narratives/stories (Markdown, Fountain) and articles/posts
  (Markdown, text) as a new `documents` entity, stored/indexed at graduated
  abstraction levels (gist, section/scene, one row each) rather than as one
  flat body. Filed as `narrative-documents-feature-request.md`
  (2026-09-13); implemented 2026-09-13, see `narrative-documents-design.md`
  and `narrative-documents-plan.md` (W1-W8). PDF is a documented, reserved
  format value but explicitly not ingestible yet — no extraction tooling
  exists anywhere in the workspace. Fountain segmentation uses
  `github.com/rsdoiel/fountain` directly (the same module harvey already
  depends on), not a second parser. Tagging/tag_density reuse
  wikilink-tagging and concept-tag-retrieval verbatim — the first real
  validation that those two features' shapes generalize to a second
  consumer. Review workflow (`unsummarized` → `drafted` → `reviewed`)
  mirrors decision records' propose/accept split; only a reviewed summary
  is ever FTS-indexed or returned as trustworthy content from
  `RecallByConceptNames`, which widened to a third merged source. Re-ingest
  matches sections by heading and flags a changed one `summary_stale`
  rather than discarding its summary. Portability (merge + JSON-L) covers
  all three new tables. Consuming any of this from harvey's
  `UnifiedMemory.Recall`, and building an eventual interactive/dialogic
  re-ingest mode in harvey, remain explicitly out of scope here.

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

- [ ] `kb ingest` does not prune a concept link removed from a record — the
  same defect as the relation item above, in code that shipped in v0.0.6.
  Found 2026-09-15 authoring WorkLab's workspace-tier DR-0010. The body cited
  the record it partially supersedes as `[[0007]]`, which resolved to a
  concept named `0007` and linked it. Rewriting the citation to plain
  `DR-0007` — leaving no `[[...]]` in the file at all — and re-running
  `kb ingest agents/decisions` reported `1 updated`, and
  `kb record concepts 0010 --workspace` still listed `0007`. Deleted the
  `record_concepts` row and the concept by hand to finish. `linkWikilinkTags`
  inserts the links it currently sees and never deletes the ones it no longer
  sees, so `record_concepts` only ever grows, exactly as `record_relations`
  does. The fix is the same shape — `DELETE FROM record_concepts WHERE
  record_id = ?` before re-inserting — and both should probably land together,
  since a caller who trusts one to be authoritative will trust the other.
  Note the two differ in blast radius: a stale relation is a wrong edge
  between two real records, while a stale concept link keeps an entire concept
  alive that nothing references any more. `document_section_concepts` is
  written by the same mechanism on re-ingest and should be checked for the
  same gap while fixing this.

- [ ] `[[NNNN]]` in a record body mints a junk concept instead of citing a
  record. Found 2026-09-15, same session as the item above, and worth
  separating from it: the prune gap is why the mess persisted, but this is why
  it existed. A decision record cites another record through `supersedes` and
  `relates_to`, which are ids; concepts are tagged inline with `[[Name]]`.
  The two syntaxes look interchangeable to anyone writing a body, and
  `[[0007]]` is the natural way to write "see DR-0007" — it is valid input,
  reports no warning, and silently creates a concept whose name is a record
  id. Bare numeric ids and the `DR-NNNN` form are both unambiguous enough to
  detect. Options, roughly in increasing order of cost: document the
  distinction in kb-record(1) and kb-ingest(1); warn on a wikilink whose name
  matches `^(DR-)?\d{4}$`; or resolve such a link to a record reference rather
  than a concept. The warning is probably the right first step, since the
  third option quietly invents a second citation syntax for something
  `relates_to` already does. A warning would also have caught this at ingest
  time rather than at `kb record concepts`.

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
  versioned artifact.

  **WorkLab's data was repaired on 2026-09-15; the bug is untouched.** By
  then the damage had spread past `clasm` — 184 of 223 rows were stale
  (`clasm` 172, `cold` 7, `caltechauthors` 5), and only 39 stored paths
  resolved on disk. Repaired out-of-band by matching each row's identity
  (scope, project, record id) against the frontmatter of the files actually
  present and rewriting `path` alone: 184 rows, nothing ambiguous, nothing
  unmatched, `path` the only field that differed anywhere in the re-export.
  `CMTools`' 13 rows were correctly left alone — that corpus has not moved.
  Re-ingesting every corpus afterwards is silent. So the numbers above are
  the historical finding, not current state, and a fresh reproduction needs a
  new move rather than a look at WorkLab. The repair also removes the only
  standing instance of the dangerous advice below, which is worth knowing
  before testing that half.

  Fix is presumably to treat a changed path as its own
  reason to update — either widen the skip test to require
  `existing.Path == rf.Record.Path` as well as a matching checksum, or write
  the path on the skip branch. Worth deciding at the same time whether the
  warning should still fire once a move is handled as an ordinary update; as
  worded ("a slug may have been regenerated, or two files may claim one id")
  it reads like a possible id collision, which a plain move is not. Related
  to the relation-pruning item above: both are cases where ingest treats the
  record files as authoritative for content but lets the database keep
  something the files no longer say.

  **The downstream remediation advice is actively dangerous, and that is the
  more urgent half.** Found later the same day, ingesting `agents/decisions`
  after moving the `caltechauthors` corpus to
  `agents/projects/caltechauthors/decisions/`. Because the five rows still
  carried their pre-move paths, `reportMissing` concluded the files were gone
  and printed, once per record:

  ```
  missing: DR-0004 (agents/decisions/caltechauthors/0004-...md) is in the
  database but its file is gone; use kb record remove to drop it
  ```

  The files were not gone — they were one directory away, and the database
  held exactly five correctly-scoped rows for them. Following the instruction
  would have deleted five live records, and `record_relations` cascades on
  delete, so the edges would go with them. A move is the *expected* operation
  under the `agents/projects/<project>/` layout, which means the tool now
  recommends a destructive fix for a problem that only exists because of the
  bug above. Fixing the path update removes most of this, but `reportMissing`
  should probably also stop asserting a cause it cannot distinguish: a record
  whose file is genuinely deleted and one whose file moved look identical from
  a stale row. Checking whether the record's `uuid` or `(project, scope, id)`
  turns up elsewhere in the ingested tree would tell them apart, and the
  message could then say "moved" rather than proposing deletion.

- [ ] `kb search` exits 0 when it finds nothing. The workspace convention for
  search-style tools is exit 1 on no match (see `~/Laboratory/CLAUDE.md`),
  though that section is written for the Deno tools and may not be intended
  to bind `kb`. Worth a decision either way, since scripts branch on it.

## To explore

- [ ] **Two decision-record dialects now exist in one organisation, and `kb`
  can only read one of them.** Raised 2026-09-15 after pulling
  `caltechlibrary/CL-Web-Components`, where a colleague has been recording
  architecture decisions independently. This is a design question, not a bug
  report: the immediate behaviour is correct, but the situation it produces
  is not workable.

  **The two shapes.** Ours: `agents/projects/<project>/decisions/NNNN-slug.md`
  in the *workspace*, YAML frontmatter carrying id/title/date/status/kind/
  trigger/project/supersedes/superseded_by/relates_to/tags/uuid/origin_host,
  body in bold-lead sections, three closed vocabularies, deliberately called
  a Decision Record and not an ADR. Theirs: `docs/decisions/NNNN-slug.md`
  *inside the project repository*, essentially MADR — an `# N. Title` H1, a
  two-bullet `- Status:` / `- Date:` block, then `## Context and Problem
  Statement`, `## Decision`, `## Considered Options`, `## Decision Outcome`,
  `## Consequences`, `## More Information`, with cross-references as relative
  Markdown links rather than ids.

  **What happens today.** `kb ingest docs/decisions` reports
  `0 added, 0 updated, 0 skipped, 4 failed`, one
  `no frontmatter: file does not start with ---` per file. That is the right
  refusal — guessing at a foreign format would be worse — but it means there
  is no partial path and no signal beyond a hard failure.

  **The identity problem is the harder half.** Both dialects number from 0001
  per project, and our identity is
  `(workspace, IFNULL(project_id,-1), scope, record_id)`. If their ADR-0001
  for `CL-Web-Components` were ingested as a record, and we later opened our
  own corpus for the same project, both would claim
  `(WorkLab, CL-Web-Components, project, 0001)`. So a format adapter alone is
  not enough — the scheme needs somewhere to put "whose record is this."

  **The real question to answer first, before any parsing work:** is a
  colleague's ADR a *record* — a peer, citable from `relates_to`, carrying a
  status we do not own and must never promote — or a *document*, source
  material we read and summarise? The answer determines everything else, and
  the two readings are genuinely different. It is their repository and their
  decision; we do not get to mark it superseded. That argues for document.
  But a decision that changes what we do is exactly what `relates_to` exists
  to express, and a document cannot be cited that way. That argues for record.

  **Options, none costed yet.** A `--format madr` flag or sniffing adapter on
  `ingest`. A `dialect` column, so a record knows which vocabulary its
  `status` belongs to. A third `scope` value (`external`?) alongside
  `project`/`workspace`, which would also fix the identity collision. An
  `origin_repo`/`upstream_url` field, since a foreign record has a canonical
  home that is not a path in our tree. Or decline the whole thing and treat
  foreign ADRs as documents, accepting that they cannot be cited by id.

  **What works today, as a stopgap.** `kb document ingest` reads them without
  complaint — no frontmatter is required, and it segments on the MADR
  headings, 5-14 sections each. Two limitations found while testing: with no
  frontmatter `title:` the document is titled with its *filename* (the `# N.`
  H1 is not consulted — arguably a small bug worth fixing on its own), and
  **no concepts are linked**, because linking comes from `[[wikilinks]]` and
  frontmatter `keywords`, neither of which MADR has. `tag_density` is
  non-zero, so the mentions are detected and simply have nowhere to go.
  Whether density alone should be able to produce a link — or at least a
  suggestion — is worth considering as part of this.

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
