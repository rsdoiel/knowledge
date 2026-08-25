# Decision records and document ingest — Implementation Plan

## Source design

`decision-records-design.md` (all five decisions confirmed 2026-08-24) and
`~/WorkLab/DECISION_RECORD_FORMAT.md` (the file format, amended three times
during the conversion pilot — see "What the pilot changed" below).

TDD throughout: write the `*_test.go` file before the implementation it
covers, red confirmed first, matching this module's existing practice.

## Status (2026-08-24)

Nothing implemented. Two real corpora already exist to test against, both
produced by `~/WorkLab/decisions_split.ts`:

| Corpus | Records | Notes |
|---|---|---|
| `~/Laboratory/knowledge/decisions/` | 3 | this module's own log |
| `~/WorkLab/clasm/decisions/` | 169 | `kind` on all, `trigger` on 70, 2 supersessions, 3 `relates_to`, 2 `phase` |

Every phase below can be verified against real data rather than fixtures
alone. That is deliberate — the pilot found three format defects that only
appeared on real files.

## What the pilot changed, that this plan must honour

Four findings from converting 172 real records. Each is a place where the
obvious implementation is wrong:

1. **`superseded_by` does not imply `status: superseded`.** Because the unit
   is an episode, a later record can invalidate one decision inside a
   multi-decision episode while the rest stand. `clasm` DR-0160 is
   `accepted` *and* carries `superseded_by: ["0159"]`. Ingest stores what
   the file says and derives neither field from the other.
2. **`trigger: ""` is valid.** Converted records legitimately carry an empty
   trigger. Validation must not reject it.
3. **Ids are identity, not chronology.** Within a single date, real logs are
   inconsistently ordered, so a correction can carry a *lower* id than the
   record it supersedes (`clasm` DR-0159 supersedes DR-0160). Every ordered
   query sorts by `date` then `record_id`, never `record_id` alone.
4. **`id`, `date` and `phase` are quoted scalars.** Bare `0142` parses as
   the integer 142; bare `2026-08-19` resolves to a timestamp. The Go
   structs use `string` for all three.

---

## W1 — Schema and types

### Files to create

- `records.go` — `Record` struct, `RecordRelation` struct, CRUD.
- `records_test.go`

### Work items

- Add `recordsSchema` (the two `CREATE TABLE IF NOT EXISTS` statements from
  the design) applied in `Open()` alongside `sourcesSchema`, following the
  existing lazy-migration precedent. No `kb_fts` migration — its existing
  columns already accommodate records.
- `Record` struct: `RecordID`, `ProjectID`, `Scope`, `Path`, `Title`,
  `Date`, `Status`, `Kind`, `Trigger`, `Phase`, `Initiative`, `Session`,
  `Body`, `Checksum`, `UUID`, `OriginHost` — `Date` a `string`, per finding 4.
- `AddRecord`, `RecordByID`, `RecordsByProject`, `UpdateRecordStatus`,
  `AddRecordRelation`, `RelationsFor`.
- `RelationsFor` returns both directions: rows where the record is `from_id`
  (`supersedes`, `relates_to`) and where it is `to_id` (rendered to the
  caller as `superseded_by`). Only one direction is stored.

### Acceptance criteria

- `go build`/`go vet`/`gofmt`/`go test` clean.
- Opening an *existing* `knowledge.db` (use a copy of
  `~/WorkLab/agents/knowledge.db`, 215 observations / 11 projects) creates
  the two new tables and leaves every existing row untouched.
- Opening a database that already has them is a no-op.

---

## W2 — Frontmatter parsing and rendering

### Files to create

- `recordfile.go` — `ParseRecordFile`, `RenderRecordFile`
- `recordfile_test.go`

### Work items

- Parse with `gopkg.in/yaml.v3` into a flat struct. First real import; run
  `go mod tidy` to promote it from `// indirect` to a direct requirement.
- Body is everything after the closing `---` line, stored verbatim.
- Validation: `id`, `title`, `date`, `status`, `kind`, `project` required;
  `trigger` may be empty (finding 2); `status` and `kind` must be in the
  documented vocabularies; unknown *fields* are preserved, not rejected.
- `Checksum` over the raw file bytes.

### Acceptance criteria

- Round-trips every one of the 172 real records: parse then render produces
  byte-identical output. This is the phase's real test — run it over both
  corpora in a table test, not over hand-written fixtures.
- A record with `status: accepted` and a non-empty `superseded_by` parses
  and validates (finding 1). A record with `trigger: ""` validates.
- Titles containing `"`, backticks and `<>` survive the round trip —
  `clasm` DR-0164's title embeds escaped quotes.

---

## W3 — `kb ingest`

### Files to create

- `cmd/kb/ingest.go`, `cmd/kb/ingest_test.go`

### Work items

- `kb ingest PATH [--dry-run] [--root DIR]` — walk `PATH`, parse every
  `NNNN-*.md`, upsert into `records` and `kb_fts` with
  `source_type = 'record'`.
- **Two passes.** Pass one upserts every record; pass two resolves
  `supersedes` / `relates_to` into `record_relations`. A record may
  reference one not yet read, so a single pass would fail on forward
  references. `superseded_by` in a file is *not* inserted — it is the
  stored relation's inverse.
- Paths stored **relative to the workspace root**, defaulting to the parent
  of the directory holding the database (so `--db agents/knowledge.db`
  gives a root of `.`), overridable with `--root`. Absolute paths do not
  survive `merge` between machines.
- Skip unchanged files by `Checksum` — re-ingesting a whole tree is cheap
  and safe to repeat.
- Additive only: a record whose file has vanished stays in the database
  (design decision 4). Report such records in the summary so the operator
  can see them, but do not delete.
- `--dry-run` reports the same counts and writes nothing.
- Ingest **never writes to the record files**. Only `kb record` does.

### Acceptance criteria

- Ingesting `~/WorkLab/clasm/decisions/` yields 169 records, 2
  `supersedes` relations and 3 `relates_to` relations.
- A second run reports 169 skipped, 0 updated.
- Touching one file's body and re-running reports exactly 1 updated.
- Forward reference proven: ingest a directory where `0002` supersedes
  `0003` and confirm the relation resolves.
- `--dry-run` leaves the database byte-identical.

---

## W4 — `kb record` read and status verbs

### Files to create

- `cmd/kb/record.go`, `cmd/kb/record_test.go`

### Work items

- `kb record list` with `--project`, `--status`, `--kind`, `--trigger`,
  `--initiative`, `--since`. **Sorted by `date` then `record_id`**
  (finding 3).
- `kb record show RECORD_ID` — frontmatter, body, and resolved relations in
  both directions.
- `kb record set-status RECORD_ID STATUS` — writes both the database row and
  the file's frontmatter, leaving every other line untouched.
- `kb record supersede NEW OLD [--partial]` — writes `supersedes` on `NEW`
  and `superseded_by` on `OLD` in both files, plus the relation row. Without
  `--partial`, also sets `OLD`'s status to `superseded`; with it, `OLD`
  stays `accepted` (finding 1). Both file writes and the database write
  succeed together or not at all.
- Every verb honours `--json` and routes errors to stderr, per
  `cli-tui-design.md`.

### Acceptance criteria

- `record list --kind correction --project clasm` returns 25.
- `record list --trigger live-test --project clasm` returns 59.
- `record show 0160` reports `superseded_by: 0159` while showing
  `status: accepted`.
- `supersede 0149 0148` (no flag) leaves 0148 `superseded`;
  `supersede 0159 0160 --partial` leaves 0160 `accepted`. Verified in the
  files, not only the database.
- A `supersede` against a non-existent record changes nothing on disk.

---

## W5 — `kb record new`

### Work items

- `kb record new --project P --title T [--kind K] [--trigger G]` — allocate
  the next id for the scope, fill `date`, `uuid`, `origin_host`, `project`,
  set `status: proposed`, and emit **all five body headings** even when
  empty.
- `trigger` is **required here**, unlike on converted records. The empty-
  trigger concession is for conversion only.
- Writes the file; does not ingest it.

### Acceptance criteria

- Into `clasm/decisions/` allocates `0170`.
- Emitted file parses cleanly under W2 and ingests under W3.
- Omitting `--trigger` is a usage error (exit 2).
- Scaffolds `Context`/`Decision`/`Rationale`/`Rejected alternatives`/
  `Consequences` headings. In `clasm` the latter two appear in only 98 and
  99 of 169 entries; an empty heading is a cheaper prompt than discipline.

---

## W6 — `kb index` and search surfacing

### Work items

- `kb index PATH` — regenerate `decisions/index.md`: one greppable line per
  record, **newest first**, never hand-edited. This preserves the
  `head`/`grep` affordance that top-insertion in a single `DECISIONS.md`
  originally provided.
- Do not write a preamble into `index.md`; directory prose lives in
  `decisions/README.md`, which is never regenerated.
- `kb search` returns `record` rows alongside projects, observations and
  concepts, labelled by `source_type`.
- TUI: records browsable under a project. Read-only, consistent with the
  TUI's existing scope.

### Acceptance criteria

- Regenerating `clasm/decisions/index.md` reproduces the existing file.
- `README.md` untouched by `kb index`.
- A search whose top hit is a record body reports it as `record`, with its
  `DR-NNNN`.

---

## W7 — Documentation and downstream

### Work items

- `helptext.go`: new `kb-record(1)` and `kb-ingest(1)` man pages, `kb(1)`
  updated to list them. Per `~/WorkLab/RELEASE_REVIEW.md` this is the
  release-review item most likely to go stale.
- Document the `kind` vocabulary in `kb help record` and the `observation`
  vocabulary (`finding`, `decision`, `note`, `release`, `question`) in
  `kb help observation` — documented, not enforced (design decision 5).
- `make man`; regenerate `README.md`/`about.md`/`CITATION.cff` via `cmt`;
  update `codemeta.json` description, `version`, `releaseNotes`,
  `dateModified`, `datePublished`.
- Log the work in `decisions/` — using `kb record new`, so the module's
  first forward-practice records are written by the tool this plan builds.

### Acceptance criteria

- `make build`/`make man` clean; every new verb has a man page.
- Security review and full `go test` pass before the version bump.

### Out of this repo, but blocked on it

Both tracked in `~/WorkLab/TODO.md` and `~/Laboratory/TODO.md`:

- `CLAUDE.md`'s grounding query lists `source_type` values as `project`,
  `observation`, `concept`. It needs `record`, or every automatic grounding
  query keeps missing the richest source in the database.
- The `kb-schema` skill emits the DDL as Step 0 for other skills and must be
  regenerated; `rag-query`, `review-knowledge-base`, `update-knowledge-base`
  and `setup-knowledge-base` all need review. Their
  `scripts/compiled.bash`/`.ps1` must be regenerated too, or Harvey and the
  Ollama models keep running the old logic.

---

## Out of scope here

- **Ingesting Fountain hand-offs, project notes, `DESIGN.md` or `PLAN.md`.**
  34 hand-off files and 13 project notes remain outside the knowledge base.
  The pilot strengthened the case: `plan-review` was undetectable in
  `clasm`'s decision log because that evidence lives only in the session
  hand-off. `records` is shaped generally enough to hold them later.
- **Converting `cold`.** It has no `DECISIONS.md` at all, so it needs
  authoring, not conversion.
- **Embeddings or vector search.** FTS5 is what exists.
- **Editing records in the TUI.** Read-mostly by design.
- **`~/WorkLab/decisions_split.ts`.** The one-off converter stays in
  WorkLab. Note that it re-derives `kind` and `trigger` on every run, so a
  `kind` corrected through `kb` would be overwritten if the converter were
  re-run against the same directory. Once a corpus is converted and
  ingested, the converter should not be run against it again.
