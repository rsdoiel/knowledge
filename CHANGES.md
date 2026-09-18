# Changes

Reconstructed for v0.0.1 through v0.0.3 from each tag's `codemeta.json`
release notes; maintained going forward.

## v0.0.10 — 2026-09-18

The project-rename completion this module had refused to do since v0.0.9,
an answer to the foreign-ADR question open since 2026-09-15, and two
corpus-improvement verbs that surface candidates for a human to confirm
rather than committing anything themselves.

### Added

- `kb concept suggest [--project NAME] [--limit N]` (DR-0028), a
  read-only verb that scans every record body and document section body
  and prints candidate *new* concepts, ranked by corpus-wide
  distinctiveness — a TF-IDF shape: occurring several times, confined to
  relatively few items. It closes the half of corpus improvement that
  density-linking cannot reach, since `MatchConceptNameCounts` only
  matches against concepts that already exist and can never propose one.
  Never writes to the database. Live-tested against the real workspace
  corpus it is a roughly-50%-signal mechanical pass — `rename`,
  `harvey`, `divergence`, `collision`, `uuid` genuinely useful alongside
  generic nouns like `row` and `table` — so a human still curates.
- `kb document tag --project NAME [--concept NAME,...] [--dry-run]`
  (DR-0029), a pure file operation that gives that judgment somewhere to
  land in the corpus itself: it inserts an explicit `[[Name]]` wikilink
  at the first safe occurrence of each eligible concept in every
  already-ingested document for a project, so the next
  `kb document ingest` links it through the wikilink path that already
  exists. Eligibility reuses DR-0027's density threshold verbatim with no
  `--concept` given. A minimal position-based edit on the raw bytes, no
  parse-and-rerender cycle, excluding frontmatter, fenced blocks, inline
  code spans, anything already inside `[[...]]`, and — found live,
  smoke-testing against a real document — the document's own first H1.
  Both-or-neither across a project's document set; a second run is a
  no-op.
- Density-based concept linking on document ingest (DR-0027): a known
  concept mentioned more than once in a section, outside inline code
  spans and fenced blocks, is auto-linked. `tag_density` is deliberately
  left counting the unfiltered text — it is the raw signal the threshold
  is applied to, not the threshold's output.

### Changed

- `kb project rename OLD NEW` now completes for a project that owns
  records (DR-0026), lifting v0.0.9's outright refusal. The fix is
  smaller than either shape originally sketched: `records.project_id`
  never needs to change, since it is a stable foreign key and only
  `projects.name` moves, so the corpus rewrite is a pure file operation —
  every owned record's `project:` frontmatter edited in place,
  both-or-neither, `--dry-run` and `--root` supported — and the next
  ordinary `kb ingest` takes the UPDATE path rather than the INSERT that
  collided. New `RenameProjectRow` library method does the DB-only rename
  without the records-owning guard, called only after every file is
  confirmed rewritten.
- A colleague's MADR-format ADR is modeled as a **document**, not a
  record (DR-0027). We do not own its status and can never promote or
  supersede it. Answering it that way dissolves the identity collision
  the question had been stuck on — both dialects number from 0001, but a
  document never enters the records identity tuple at all — so the
  `dialect` column, the `external` scope value and the `--format madr`
  adapter are moot rather than deferred. Accepted cost, stated rather
  than glossed: a document cannot be cited from `relates_to`, so a record
  responding to a foreign ADR links it in prose.
- `ParseDocumentFile` falls back to a document's first true H1 when
  frontmatter supplies no `title:`, instead of the caller's filename
  (DR-0027). General rather than MADR-specific, but MADR is what
  surfaced it, having no frontmatter at all.
- `user_manual.md`'s verb table is current again — it was six verbs out
  of date (`ingest`, `record`, `index`, `document`, `init`, `topics`) and
  linked a `DECISIONS.md` removed when this module became the decision-
  record format's first conversion pilot. `kb(1)`'s DESCRIPTION and SEE
  ALSO now name records and documents too.

### Fixed

- `kb merge` could silently keep a stale project or concept name, or drop
  a side outright, depending on merge order, because `INSERT OR IGNORE`
  cannot distinguish a uuid collision from a name collision; `kb import`
  hard-failed the *entire* import rather than one row on the same
  collision (DR-0026). Conflict resolution for both moves from name-keyed
  to uuid-primary, reconciling `name` by `updated_at` the same way
  `description` and `status` already did, with a name-keyed fallback
  retained for two genuinely independent, never-synced entities that
  happen to share a name. Both were traced live, not merely suspected,
  and shipping the rename completion without them would have made the
  corruption risk worse by making the trigger routine.
- `TestParseRecordFile_RoundTripsEveryLiveRecord` (DR-0030) named five
  fixed corpus paths, three of which went stale when DR-0021's
  `agents/projects/<project>/decisions/` layout was rolled out. It had
  quietly stopped exercising four fifths of the live corpus — 54 records
  of 267 — and reported that as a count shortfall rather than a discovery
  failure. Corpora are now discovered rather than listed, using the same
  two-signal rule `kb index --all` settled on (a record-shaped filename
  *and* real frontmatter in the file), which also keeps a foreign MADR
  corpus out of a round-trip test it would fail by construction. The
  remembered count floor is gone, in favour of the per-file property the
  test was always really asserting — the same lesson DR-0015 drew.

## v0.0.9 — 2026-09-17

Rename verbs for projects and concepts, cross-machine reconciliation for
their mutable fields, and a multi-corpus mode for `index.md` — three items
closed out of `TODO.md`, two of them correcting their own original
diagnosis along the way.

### Added

- `kb project rename OLD NEW` and `kb concept rename OLD NEW` (DR-0024)
  replace the only prior route, raw SQL against `projects.name`/
  `concepts.name`. `project rename` refuses if `NEW` already exists or if
  the project owns any records — a live repro showed that renaming a
  project with a corpus and then re-ingesting it silently mints a phantom
  project under the old name and duplicates the record under it, data
  corruption rather than just a stale search label. `concept rename` has
  no such corpus to desync and ships unconditionally beyond the name
  collision every rename needs. Both reuse `refreshProjectFTS`/a new
  `refreshConceptFTS` so search never lags a rename.
- Cross-machine last-writer-wins for a project or concept's mutable
  fields (DR-0025). `concepts` gains `updated_at`; `kb merge`'s conflict
  path becomes `INSERT OR IGNORE` for new rows plus a guarded
  `UPDATE ... FROM ... WHERE incoming.updated_at > existing.updated_at`
  for existing ones, so whichever side is actually newer wins regardless
  of which side is applied first. `kb import` gained the matching
  comparison. Observations needed none of this — DR-0023 already resolved
  observation correction via supersession.
- `kb index ROOT --all [--check]` discovers every corpus under `ROOT`
  that already has an `index.md`, refreshes or checks each one, keeps
  going past an individual corpus's failure, and exits non-zero if any
  needed attention — closing the index-staleness item's multi-corpus
  half. Running it live against the real workspace caught a bug before it
  shipped: matching on the filename alone picked up unrelated `index.md`
  files (a docs site, a blog) and would have silently overwritten them in
  write mode. Fixed by requiring both a content-heading match and an
  actual record file in the directory.

## v0.0.8 — 2026-09-16

A bug-fix-and-small-features release: closing the `index.md` staleness gap
at its source, and giving observations a correction path.

### Added

- `kb index PATH --check` compares a fresh render against `PATH/index.md`
  byte-for-byte and fails with "does not exist" or "is stale" instead of
  writing, naming `kb index PATH` as the remedy — same exit-code convention
  as `kb search`'s fix last release.
- `kb observation update ID BODY...` gives observations a correction path
  they never had. It does not mutate the old observation — it inserts a
  new one (inheriting the original's project and kind) and links the two
  with a new `observation_relations` table, `record_relations`' own shape
  reused rather than reinvented (see DR-0023, `knowledge/decisions/`,
  which reverses DR-0012's original observations-stay-immutable stance
  after review found that an in-place edit destroys history and answers
  the amend-vs-supersede question by accident). The old observation's
  body is never rewritten, so history retention is free; no `updated_at`
  column was added, since the new observation's own `created_at` is the
  correction timestamp. `kb observation show ID` now resolves and prints
  `supersedes`/`superseded_by`, mirroring `kb record show`. `kb merge`
  and JSON-L export/import carry `observation_relations` from this
  release, not as a follow-on, per the standing rule that a table missing
  from the merge summary is a table whose loss goes unreported.

### Fixed

- `kb record set-status` and `kb record supersede` now refresh a corpus's
  `index.md` themselves after a successful write, via the new
  `regenerateIndexIfPresent`, closing the gap `--check` only detects. Only
  when one is already present — never creating one where a corpus hasn't
  opted in. Together with `--check`, this fixes `index.md` silently
  drifting from `status`/`kind`/`trigger`/`superseded_by`/title changes,
  which bit WorkLab twice.

## v0.0.7 — 2026-09-15

A bug-fix release: four defects found running v0.0.6 against real corpora
(clasm, WorkLab, caltechauthors), all in `kb ingest`'s re-run behavior plus
`kb search`'s exit code.

### Fixed

- `kb ingest` no longer leaves stale edges behind on re-ingest. A
  `relates_to`/`supersedes` entry removed from a record's frontmatter, or a
  `[[wikilink]]` concept tag removed from its body, is now actually removed
  from `record_relations`/`record_concepts` — previously both only ever
  grew, since re-ingest inserted what a file currently declared but never
  deleted what it no longer declared. The new `ClearRecordRelationsFrom`/
  `ClearRecordConcepts` (and `ClearDocumentSectionConcepts` for
  `kb document ingest`) run before every re-insert.
- `[[0007]]`/`[[DR-0007]]` in a record body — the natural way to write "see
  DR-0007" — no longer silently mints a junk concept named after the record
  id. It is now skipped with a warning pointing at `supersedes`/
  `relates_to`, the actual way to cite another record.
- `kb ingest` now updates a record's stored `path` when its file moves but
  its content is unchanged, rather than leaving `records.path` (and so
  `kb export`'s `agents/knowledge.jsonl`) silently stale. The "DR-%s was
  stored at %s" warning now fires only when content changes alongside the
  path, since a path change alone is an ordinary move, not a possible id
  collision. The message for a record with no file at its stored path no
  longer asserts deletion as the only explanation and recommends
  `kb record remove` outright — a moved file looks identical to a deleted
  one, and the old wording would have walked a user into deleting live
  records.
- `kb search` now exits 1, in both text and `--json` mode, when it finds
  nothing, matching the workspace's search-tool convention instead of
  exiting 0 with an empty result.

## v0.0.6 — 2026-09-13

Three related features, each building on the last, extend `knowledge`
toward a small-model-friendly knowledge base: inline concept tagging,
embedder-free concept-based retrieval, and narrative/article ingestion at
graduated abstraction levels.

### Added

- `kb ingest` resolves `[[Name]]` wikilinks in a record's body, and its
  previously-unused `tags` frontmatter field, into concepts linked via a
  new `record_concepts` table — case-insensitive via a new
  `ResolveConceptName` (kept separate from `AddConcept`/`kb concept add`,
  which stays exact-match, since a human typing a concept name at the CLI
  is a deliberate act, unlike capitalization in prose). `kb record concepts
  RECORD_ID` shows the result.
- New `retrieval.go`: `MatchConceptNames` finds known concepts mentioned
  (whole-word, case-insensitive) in arbitrary text, and
  `RecallByConceptNames` returns observations, records, and documents
  linked to a given set of concepts, merged and ranked by match count then
  recency — a cheap, embedder-free first pass for small/CPU-only models,
  not project-scoped.
- New `documents`/`document_sections` entity ingests narratives and
  articles (Markdown, Fountain, plain text — PDF is a reserved format
  value, not yet supported, since no text-extraction path exists anywhere
  in this workspace) at graduated abstraction levels: a document-level
  gist plus one row per structural section (Fountain scene headings via
  `github.com/rsdoiel/fountain`, the same module harvey already depends
  on; Markdown headings; a whole file for plain text), each with its own
  summary lifecycle (`unsummarized` → `drafted` → `reviewed`, mirroring
  decision records' own author/promote split) via new
  `kb document ingest/list/show/draft/review` verbs. Frontmatter
  (title/description/pubDate/author/keywords, matching antennaApp's own
  documented vocabulary) seeds metadata and an initial gist when present,
  without ever being required. Re-ingesting a changed file matches
  sections by heading text and flags a changed one `summary_stale` rather
  than discarding its summary; a heading with no match is reported, never
  deleted. Only a reviewed summary is ever indexed for search or returned
  as trustworthy content by `RecallByConceptNames`.
- All three features carry through `kb merge` and `kb export`/`kb import`
  — `record_concepts`, `document_section_concepts`, and
  `documents`/`document_sections` all appear in every portability path,
  per the project's standing rule that a table missing from the merge
  summary is a table whose loss goes unreported.

### Changed

- The independent hand-rolled CLI flag/positional parsers in `ingest`,
  `record`, `source`, and the new `document ingest` were consolidated into
  one shared `splitFlags` helper; `source add` now errors on an
  unrecognized flag instead of silently dropping it.

## v0.0.5 — 2026-08-28

Decision records now travel on every portability path — `merge`, `export`,
`import` — not just `ingest`. Before this, `records` and `record_relations`
were invisible to all three, so running `merge` (or an export/import round
trip) discarded every decision record while reporting success (DR-0013).

### Added

- `kb merge` carries `records`/`record_relations` in its union, reports a
  record collision keyed by identity (workspace, project, scope, record id)
  rather than by `project_id` or a project's own uuid, and reports a
  **content divergence** — same record, different text — without blocking
  the merge on it. `--json` gains `content_divergences` alongside
  `collisions_reconciled` and the per-table `tables` summary.
- `kb export`/`kb import` carry `record`/`record_relation` JSON-L lines. A
  `-project`-scoped export carries only that project's records; a
  workspace-tier record, having no project, appears only in an unscoped
  export (DR-0019). Import matches a record by identity, not uuid — two
  machines' ingest of the same file mint different uuids for it, so
  identity is the normal case for "already present," not the exception
  (DR-0018) — and resolves its project by name for the same reason.
- `CollisionReport`/`ReconcileCollisions`/`DivergenceReport`,
  `NormalizeForMerge` (migrates a merge scratch copy to the current schema
  before ATTACHing, so a database predating a table still merges).
- Every table `merge` carries now appears in its per-table summary, so a
  table that would lose rows says so.

`kb record new`'s default write location for project-scoped records moves to
`agents/projects/<project>/decisions/` (was `<project>/decisions/`), part of
a workspace-wide reorganisation moving process artifacts out of project
repositories (DR-0021). `kb` also stops silently creating an ambient
`agents/knowledge.db` in whatever directory it happens to be run from — a new
`kb init` verb is now the explicit way to start a workspace (DR-0021,
DR-0022).

### Added

- `kb init [PATH]`: creates a schema-only, idempotent `agents/knowledge.db`,
  the same shape as `git init`. Documented at `kb-init(1)`.
- `kb record new --dir DIR`: overrides the default write location for a new
  record, relative to `--root` like every other stored record path.

### Changed

- `kb record new --project P` (no `--dir`) now writes to
  `agents/projects/P/decisions/` instead of `P/decisions/`. `--workspace`
  scope is unchanged (`agents/decisions/`). Existing corpora under the old
  layout are not migrated by this change.
- Any verb resolved through the ambient default (no `--db` given) now fails
  with a message pointing at `kb init` or `kb import -in FILE` if
  `agents/knowledge.db` doesn't already exist, instead of silently creating
  one — fixes `kb record new`/`kb observation add`, run from inside a
  project directory, building a stray nested workspace. An explicit `--db
  PATH` and `kb import` keep today's open-or-create behavior unchanged.

### Notes

- DR-0013 through DR-0019 cover the records-portability effort's design and
  implementation decisions; DR-0021 and DR-0022 cover the record layout and
  workspace-init change. See `knowledge/decisions/index.md`.

## v0.0.4 — 2026-08-26

Adds Decision Record support: episode-scoped Markdown files with YAML
frontmatter, kept in a project's `decisions/` directory and indexed as
first-class rows, so the reasoning behind a decision is retrievable rather than
living only in files the knowledge base never reads. The format is specified in
`~/WorkLab/DECISION_RECORD_FORMAT.md`; this release is its reference
implementation.

### Added

- **Schema**: `records` and `record_relations`. A record's identity is
  `(workspace, project, scope, id)`. The workspace name is the directory name
  of the workspace root, derived from the path rather than written in a file,
  because every workspace has an `agents/decisions/` and two of them may each
  hold a DR-0001. Records are indexed into `kb_fts` with
  `source_type = 'record'`. Existing databases migrate lazily on open.
- **`kb ingest PATH`** — walks a tree of `NNNN-slug.md` records and resolves
  `supersedes`/`relates_to` in two passes, so forward references work. Additive:
  a record whose file has vanished is reported, never deleted. Never writes to
  a record file.
- **`kb record list|show|new|set-status|supersede|fmt`** — `new` scaffolds with
  `status: proposed`; `supersede` writes both sides and the relation together
  or not at all; `fmt` normalises a tree and never writes to the database.
- **`kb index PATH`** — generates `decisions/index.md`, one greppable line per
  record, newest first. Byte-identical to the Deno generator it replaces.
- **Standard options** `-help`, `-license` and `-version`, which `kb` had
  lacked entirely, declared through a `flag.FlagSet` so each is accepted in
  either dash form.
- **`kb help topics`** — the topic index. Named `topics` rather than the
  conventional `index` because `index` is a verb here.
- **Library**: `ParseRecordFile`/`RenderRecordFile`, `ListRecords`,
  `RecordByIdentity`, `RecordsByRecordID`, `RecordsUnderPath`, `NewUUID`,
  `Today`, and a `SourceType` field on `KBSearchResult`.
- **TUI**: browse a project's records read-only, with `r`.
- Man pages for `kb-ingest(1)`, `kb-record(1)`, `kb-index(1)`.

### Changed

- `kb search` labels a hit by its source table rather than its own kind, so a
  decision record reads `[record]` rather than `[decision]`.
- Informational commands no longer open or create a database. `kb index` joins
  `kb merge` in this, since it builds from the record files and never queries
  one — it had been leaving a database in whatever directory it ran in.
- The frontmatter struct declaration is the canonical format specification:
  field order, flow-styled sequences, and a double-quoted string type for the
  seven fields the format requires quoted, so every writer produces
  byte-identical output.

### Fixed

- A record path read from the database is confined to the workspace root before
  it is read or written. `filepath.Join` cleans a path without confining it, so
  a path reaching the database by some route other than ingest could otherwise
  have made `set-status` rewrite an arbitrary file.

### Notes

- Vocabularies for record `status`/`kind`/`trigger` are documented and reported
  against, **not** enforced: in a format several tools write to, a typo should
  be a fixable row and not a failed run. Observation kinds remain enforced,
  which is a deliberate asymmetry.
- Adds `gopkg.in/yaml.v3` as a direct dependency.
- Verified against 205 real records across five corpora, all round-tripping
  byte-for-byte.

## v0.0.3 — 2026-08-08

Adds JSON-L export/import: `ExportJSONL`/`ImportJSONL` in the knowledge
package, plus `kb export [-project NAME] [-out PATH]` and `kb import [-in
PATH]`. A portable, no-file-access alternative to `merge` — the resulting file
can be pasted, emailed or committed to git, then applied elsewhere with
`import`. Projects and concepts are matched by name (existing local rows win),
sources by identifier, observations and links by uuid, so re-importing the same
file is a no-op.

## v0.0.2 — 2026-07-28

Adds project status management: `AddProjectWithStatus` and `SetProjectStatus`,
plus a `--status` flag on `kb project add` and a `kb project set-status NAME
STATUS` verb, all validated against `concept`/`active`/`paused`/`concluded`.
`AddProject`'s default behaviour is unchanged.

## v0.0.1 — 2026-07-27

Proof-of-concept pre-release. Full CRUD API (projects, observations, concepts,
sources) with FTS5 search and cross-machine merge; `cmd/kb` ships a git/go-style
CLI with `--json` output and a read-mostly bubbletea TUI; `--debug` emits a
JSONL trace of every knowledge-base call and TUI event. Extracted from harvey's
`knowledge.go`/`knowledge_merge.go`.
