
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

- [x] `kb ingest` pruned neither a relation nor a concept link dropped from a
  record's frontmatter/body, so `record_relations` and `record_concepts` only
  ever grew (found 2026-08-27 and 2026-09-15 respectively, authoring `clasm`
  DR-0170/DR-0171 and WorkLab's DR-0010). Fixed 2026-09-15: `resolveAll`
  now calls the new `ClearRecordRelationsFrom(fromID)` before re-adding a
  record's own supersedes/relates_to edges each run, scoped to forward edges
  so the other side's own declarations survive; `linkWikilinkTags` calls the
  new `ClearRecordConcepts(recordID)` the same way before re-linking. The
  same gap existed in `document_section_concepts` via `tagSection`, fixed
  with the new `ClearDocumentSectionConcepts(sectionID)`.

- [x] `[[NNNN]]` (e.g. `[[0007]]`, `[[DR-0007]]`) in a record body minted a
  junk concept instead of citing a record — found 2026-09-15, same session as
  the concept-prune item above. `linkWikilinkTags` (`cmd/kb/ingest.go`) now
  matches such a name against `recordIDLikeWikilink`
  (`^(?i:dr-)?[0-9]{4}$`), skips linking it, and adds a warning pointing at
  `supersedes`/`relates_to` instead.

- [x] `kb ingest` never updated `records.path` on a pure file move (found
  2026-08-28 migrating `clasm`'s 173-record corpus under the
  `agents/projects/<project>/` layout; WorkLab's data separately repaired
  out-of-band on 2026-09-15, see the git history for that record). Fixed
  2026-09-15: `upsertAll`'s checksum-match (skip) branch now calls the new
  `UpdateRecordPath` when the incoming path differs. The
  "DR-%s was stored at %s" warning now fires only when the checksum *also*
  differs — a path change alone is an ordinary move, not a possible slug
  collision. `reportMissing`'s message no longer asserts deletion as the only
  explanation (it used to say "use kb record remove to drop it" for a record
  that had only moved out of the ingested prefix, which would have deleted
  five live WorkLab records); it now names a move as the other possibility
  and asks for a re-ingest of the new location first.

- [x] `kb search` exited 0 when it found nothing, against the workspace's
  search-tool convention (`~/Laboratory/CLAUDE.md`: exit 1 on no match).
  Fixed 2026-09-15: `cmdSearch` returns an error instead of printing "no
  results" and returning nil, so it now exits 1 in both text and `--json`
  mode.
