# Changes

Reconstructed for v0.0.1 through v0.0.3 from each tag's `codemeta.json`
release notes; maintained going forward.

## Unreleased

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

### Notes

- DR-0013 through DR-0019 cover this effort's design and implementation
  decisions; see `knowledge/decisions/index.md`.

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
