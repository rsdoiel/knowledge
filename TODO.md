
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

- [ ] **A mechanism for knowing when `index.md` needs regenerating.** `kb
  index` regenerates it correctly; nothing says when to run it, and nothing
  notices when it was not. The index has no checksum, no timestamp compared
  against the records it summarises, and no `--check` mode. It is generated
  and never hand-edited, so it can only ever be stale — never wrong in a way
  someone would spot while reading it.

  It has now drifted twice in WorkLab, both times silently and both times the
  same way. In August 2026 promoting DR-0004..0006 desynced it. On 2026-09-15
  `agents/decisions/index.md` **did not list DR-0010 at all**, and that was
  noticed only because someone thought to look after accepting the record.
  **The trigger is not "a record was added" — it is any change to a field the
  index renders**, which is `status`, `kind`, `trigger`, `superseded_by` and
  the title. So `kb record set-status` and `kb record supersede` both
  invalidate it, and those are exactly the commands a person runs without
  thinking about an index.

  **This should cover project-tier corpora too, not just
  `agents/decisions/`.** WorkLab has four corpora today — the workspace tier
  plus `agents/projects/{clasm,cold,caltechauthors}/decisions/` — and the
  project ones are larger. Nothing regenerates those either, and a per-corpus
  manual habit will not survive four directories.

  Options, roughly in increasing ambition:

  - `kb index --check`, exiting non-zero when the file on disk differs from
    what would be generated. Cheap, scriptable, fits the workspace's
    `pre-commit` hook, which already re-exports `knowledge.jsonl` on drift and
    could do the same here. This is the smallest thing that would have caught
    both incidents. **Shipped 2026-09-15**: `kb index PATH --check` compares
    a fresh render against `PATH/index.md` byte-for-byte, never writes, and
    fails with "does not exist" or "is stale" naming the remedy (`kb index
    PATH`) rather than doing it — same exit-code shape as `kb search`'s fix
    this release. This covers one corpus per invocation, so multi-corpus
    discovery (below) is still open, but the other gap — nothing running on
    `set-status`/`supersede` — is closed by the next item.
  - Have `set-status`/`supersede` regenerate the index for the corpus they
    just wrote to, since both already know the record's `path` and therefore
    its directory. Removes the habit entirely; the cost is a write to a file
    the caller did not name. **Shipped 2026-09-16**: the new
    `regenerateIndexIfPresent(dir)` runs after either command's file+database
    write succeeds, but only refreshes an `index.md` that already exists —
    never creates one, so the "cost" above does not apply to a corpus that
    has not opted in. `supersede` covers both NEW's and OLD's directories,
    deduped when they're the same tier's corpus. A regen failure is reported
    as a note rather than failing the whole command, since the record write
    it is downstream of already succeeded and the index is always
    re-derivable. Multi-corpus discovery (below) and the on-disk-at-all
    question are still open.
  - Have `ingest` regenerate it, which is wrong on its own — ingest is the one
    command that does *not* write record files, and the drift happens on
    status changes rather than on ingest.

  Worth deciding alongside it: whether the index belongs on disk at all, given
  that everything in it is a `records` query. It exists so `head`, `grep` and
  `awk` reach the corpus without kb installed — that is a real affordance and
  probably decides it — but the staleness only exists because the data is
  duplicated, and that should be stated rather than assumed.

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

  **Evidence, 2026-09-15: density-based linking was run by hand against those
  four ADRs, and it is useful but lossy — roughly 60% signal.** The script
  mirrored kb's own semantics rather than inventing new ones: the
  `(?i)\b<name>\b` whole-word match from `MatchConceptNames`, computed over
  `wholeDocumentText`, which is what `cmd/kb/document.go` already feeds a
  gist's density. Confirmation that the mirror was faithful: the match counts
  came out 3, 2, 4, 4 — identical to the `tag_density` already stored on each
  gist.

  It produced 13 links from 95 known concepts, and the effect was real. A
  query naming `cmtools`, `documentation` and `tooling` returned ADR-0004
  first, above a workspace decision record, where before it returned nothing
  from either retrieval pass — FTS5 ANDs its tokens, so a conversational
  question misses a document that has no concept links at all.

  Eight of the thirteen were topical. **Five were coincidental, and the way
  they fail is the useful part**, because all five are the same failure — a
  concept name colliding with a word used in a different sense:

  - `format` matched `unsupported format` in a quoted *error message*, and
    separately matched the variable in `const format = outputName`.
  - `index` matched "a committed search index", a different sense from the
    `index` concept (a generated `decisions/index.md`).
  - `git` matched an incidental `git push` in a sentence about publishing.
  - `validation` matched a phrase describing a *third* project's domain, not
    the document's own subject.

  So the lesson is narrower than "density is too noisy." Whole-word matching
  is not the problem; **short, common, single-word concept names are**, and
  prose quoting code and error strings makes them worse. Three refinements
  worth costing, in increasing order of ambition: require more than one
  occurrence before linking; exclude matches inside code spans and fenced
  blocks, which would have killed `const format = outputName` and probably
  `git push`; or write density matches as *suggestions* a human confirms,
  reusing the `unsummarized → drafted → reviewed` gate one level down, which
  is the option that fits what the documents entity already does.

  **The five coincidental links were pruned by hand the same day**, on the
  author's call, leaving 8. An earlier draft of this item said they had been
  kept deliberately so the data would not drift from a future kb
  implementation; that is no longer true, and the trade was made knowingly —
  a curated corpus now, against having to re-derive the filter later.

  Two things the prune itself showed. **It cost no coverage**: all four ADRs
  stayed reachable from concept-tag recall, because each retained at least
  one topical link, so the noisy matches were redundant rather than
  load-bearing. And `tag_density` is deliberately *not* updated to match —
  it counts mentions, not links — so gists 19 and 30 now read density 4
  against 2 links each. That divergence is meaningful and worth preserving if
  suggestions ever land: density is the raw signal, links are what survived
  review, and the gap between them is exactly the quantity a suggestion
  workflow would surface.

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

- [x] `kb observation update` didn't exist — reached for in real usage
  2026-09-16, the same amend-vs-supersede question DR-0012 deliberately left
  open for observations. See DR-0023 (`knowledge/decisions/`): an observation
  is corrected by superseding it, never by mutating its body. Shipped
  2026-09-16: `kb observation update ID BODY...` inserts a new observation
  (inheriting `ID`'s project and kind) and links it to `ID` via a new
  `observation_relations` table — `record_relations`' own shape, reused —
  with `AddObservationRelation`/`ObservationRelationsFor` mirroring
  `AddRecordRelation`/`RelationsFor`. The old observation's body is never
  touched, so history retention is free; no `updated_at` column was added,
  since the new observation's own `created_at` is the correction timestamp.
  `kb observation show ID` now resolves and prints `supersedes`/
  `superseded_by`, mirroring `kb record show`. `kb merge` and JSON-L
  export/import carry `observation_relations` from this release, not as a
  follow-on, per the standing rule that a table missing from the merge
  summary is a table whose loss goes unreported.

- [x] `kb project rename` and `kb concept rename` didn't exist. **Correcting
  this item's own earlier diagnosis**: `set-description`'s reindex was
  blamed for the `kb_fts` duplicate, but `refreshProjectFTS` deletes by
  `source_id`, not by name — a live repro confirmed `set-description`
  correctly repairs a stale label after a raw rename, one row, not two. The
  real duplicate came from a different natural mistake, `project add
  NEWNAME` used to "rename" (`ON CONFLICT(name)` doesn't fire for a new
  name, so it inserts a second, fully independent project). The harder
  failure was worse than described, too: re-ingesting a renamed project's
  corpus without rewriting its files' `project:` frontmatter silently mints
  a *phantom* project under the old name and *duplicates* the record under
  it — reproduced live, not merely asserted. See DR-0024
  (`knowledge/decisions/`) for the full repros and the design. Shipped
  2026-09-17: `kb project rename OLD NEW` refuses if `NEW` exists or if the
  project owns any records, otherwise renames and reuses the existing
  `refreshProjectFTS`. `kb concept rename OLD NEW` ships in the same pass
  with no corpus-refuse condition, since every concept link is a foreign
  key to `concepts.id`, never a name matched from a file — via a new
  `refreshConceptFTS` factored out of `AddConceptWithIdentifier`'s own
  inline version. Rewriting a corpus's frontmatter automatically to lift the
  refusal remains open, filed as its own future record in DR-0024's
  Rejected alternatives.

- [x] Cross-machine reconciliation of an edited project or concept
  description. Deferred out of the `set-description` work (DR-0012) as a
  policy inversion. **Two of this item's own claims were stale by the time
  it was picked up**: `updated_at` already travelled through `merge` for
  `projects` — the bug was `INSERT OR IGNORE` never consulting it, insertion
  order alone deciding the winner, not a missing column; and observations
  need none of this at all, since DR-0023 already resolved observation
  correction via supersession, retiring the question this item was still
  asking. See DR-0025 (`knowledge/decisions/`) for both corrections in full,
  and its repros. Shipped 2026-09-17: `concepts` gained `updated_at`
  (lazily migrated, backfilled to `created_at`, mirroring how
  `concepts.created_at` itself was added). `kb merge`'s `projects`/`concepts`
  passes changed from `INSERT OR IGNORE` to `INSERT OR IGNORE` (new rows)
  plus a guarded `UPDATE ... FROM ... WHERE incoming.updated_at >
  existing.updated_at` (conflicts) — two statements rather than one
  `INSERT ... SELECT ... ON CONFLICT DO UPDATE`, because the pure-Go SQLite
  driver this project uses rejects an UPSERT clause after a SELECT-form
  INSERT ("near DO: syntax error"), confirmed live, even though the same
  UPSERT works after a VALUES-form INSERT elsewhere in this codebase.
  `importProject`/`importConcept` gained the matching comparison at the Go
  level, plus a new `UpdatedAt` field on `projectRecord`/`conceptRecord`.
  `RenameProject`, `RenameConcept`, and `AddConceptWithIdentifier`'s
  `ON CONFLICT` branch now touch `updated_at`, closing two gaps DR-0024 and
  DR-0012 left open now that the column is compared for real.
  `kb-project(1)`/`kb-concept(1)`'s CAVEATS sections now describe what
  actually happens: descriptions/status reconcile by timestamp; a *rename*
  crossing machines still does not, since `merge`/`import` dedupe by name —
  filed as its own follow-on in DR-0025, not solved here.
