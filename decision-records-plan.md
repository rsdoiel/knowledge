# Decision records and document ingest — Implementation Plan

## Source design

`decision-records-design.md` (decisions 1–5 confirmed 2026-08-24, decision 6
— cross-tier references — added 2026-08-25) and
`~/WorkLab/DECISION_RECORD_FORMAT.md` (the file format, amended three times
during the conversion pilot — see "What the pilot changed" below — and again
on 2026-08-25 with the "Cross-tier references" section and a revised index
line format).

TDD throughout: write the `*_test.go` file before the implementation it
covers, red confirmed first, matching this module's existing practice.

## Status (2026-08-26)

**All eight phases are implemented and green, pending review.** `records.go`,
`recordfile.go`, `cmd/kb/ingest.go`, `cmd/kb/record.go`, `cmd/kb/recordnew.go`
and `cmd/kb/index.go`; 322 tests, none skipped; released as v0.0.4.

- **W1–W3** schema, canonical frontmatter parse/render, and two-pass ingest.
- **W4–W5** the `record` verbs, plus `new`, `fmt` and the uuid collision guard.
- **W6** `kb index`, byte-identical to `decisions_index.ts` across all five
  corpora — which met DR-0003's condition, so that tool has been retired.
- **W7** man pages, vocabularies, v0.0.4, and the downstream fixes. Two of its
  premises turned out to be wrong; see DR-0009.
- **W8** the `workspace` column. A record's identity is now
  `(workspace, project, scope, record_id)`, so `~/Laboratory` can safely have a
  workspace tier — the last thing blocking it.

Decisions taken along the way are DR-0004 through DR-0011 in `decisions/`.
DR-0004..0007 are accepted; DR-0008..0011 are still `proposed`.

Five corpora, 205 records, all in canonical form and all round-tripping
byte-identically:

| Corpus | Records | Origin | Notes |
|---|---|---|---|
| `~/Laboratory/knowledge/decisions/` | 11 | 3 converted, 8 authored during this effort | this module's own log; DR-0004..0011 record the build |
| `~/WorkLab/clasm/decisions/` | 169 | `decisions_split.ts` | `kind` on all, `trigger` on 70, 2 supersessions, 3 `relates_to`, 2 `phase` |
| `~/WorkLab/CMTools/decisions/` | 13 | hand-authored | the richest corpus per record — see below |
| `~/WorkLab/cold/decisions/` | 7 | hand-authored | first corpus with `decisions[]` (3 records), `trigger: design`, `phase` on 5 |
| `~/WorkLab/agents/decisions/` | 6 | hand-authored | the workspace tier (`project: ""`); DR-0001 and DR-0002 carry the only cross-tier `relates_to` in existence |

The hand-authored corpora matter disproportionately for testing: the two
converted corpora leave `decisions[]`, `session`, `initiative`, `tags` and
`trigger: design` unpopulated, because conversion cannot invent what a
monolithic log never recorded.

**`CMTools` is the strongest single test corpus** (added 2026-08-25) and is
worth reaching for first in any table test:

- **First `session` values anywhere** — 2 records, plus workspace DR-0006. No
  other corpus exercises the field.
- **First `trigger: plan-review` anywhere** — 2 records. The format doc notes
  this value matched *zero* of `clasm`'s 169 even though such an episode was
  known to exist, so until now no corpus could test it.
- A genuine **partial supersession**: DR-0008 `supersedes: ["0003"]` while
  DR-0003 stays `accepted` with `superseded_by` set — the case the `sup` flag
  exists for, and the one `clasm:DR-0160` also covers.
- 6 records with non-empty `decisions[]`, whose entries contain **backticks,
  double quotes and `~` paths** — the YAML-quoting stress case.
- `phase` on 9, spanning `"0.0.45"` and `"0.0.46"`; 2 `correction` records;
  dates spanning 2025-01-09 to 2026-08-25.

Index generation is **already implemented outside `kb`**, by
`~/WorkLab/decisions_index.ts` (2026-08-25), which currently maintains all
five indexes. W6 supersedes it — see that phase.

Every phase below can be verified against real data rather than fixtures
alone. That is deliberate — the pilot found three format defects that only
appeared on real files, and building the index generator found two more.

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

- Parse with `gopkg.in/yaml.v3` into a flat struct. `yaml.v3` was **not** in
  `go.mod` at all, contrary to this plan's original claim that it was there as
  `// indirect`; adding it was a new direct dependency, approved 2026-08-25.
- **The struct declaration is the format specification.** Field order is the
  emitted key order, `,flow` on the five sequence fields satisfies the format's
  "inline lists only" rule, and a `qstr` string type whose `MarshalYAML`
  returns a `DoubleQuotedStyle` node pins the seven fields the format requires
  quoted. Plain `yaml.Marshal` is not sufficient on its own: it emits `title`,
  `uuid` and `origin_host` bare, and quotes `phase` only when the value happens
  to resolve as a number — `"20.51"` quoted, `0.0.46` not — which would make
  the rendering value-dependent.
- Decoding takes each scalar's node value verbatim, so a bare `id: 0142` still
  yields `"0142"` with its padding and a bare `date: 2026-08-19` never becomes
  a timestamp. Rendering is a fixed point.
- Body is everything after the closing `---` line, stored verbatim.
- Validation: `id`, `title`, `date`, `status`, `kind`, `project` required;
  `trigger` may be empty (finding 2); unknown *fields* are preserved, not
  rejected.
- **Vocabularies are reported, not enforced**, resolving a contradiction
  between this plan (which said `status` and `kind` "must be in the documented
  vocabularies") and design decision 5 (documented, not enforced). An
  out-of-vocabulary value parses and is carried with a warning, as does a
  scalar written without the quoting the format requires. Same temperament as
  decision 6's unresolvable references: a typo in a file several harnesses
  write should be a fixable row, not a failed run.
- `Checksum` over the raw file bytes.

### Acceptance criteria

- Round-trips the 198 real records across all five corpora in one table test,
  not over hand-written fixtures. The 26 hand-authored records carry the
  optional fields the 172 converted ones leave empty, so a table test over
  `clasm` alone would pass while rendering `decisions[]`, `session` and `phase`
  wrongly.
- **192 of the 198 render byte-identically.** The 6 exceptions are the CMTools
  records whose `decisions[]` uses a block sequence — a conformance defect
  against "inline lists only", normalised by `kb record fmt` in W5. The test
  asserts that set exactly: a record that changes unexpectedly fails, and so
  does one of the six that does *not* change. Divergent files must still render
  to a fixed point, and normalisation must leave the body untouched.
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
- **A `relates_to` entry is `[<scope>:]<id>`** (design decision 6). Split on
  the first `:` only; strip an optional leading `DR-` from the id rather than
  rejecting it. A bare id inherits the citing record's `scope` and
  `project_id`; `clasm:` looks the project up by `projects.name`;
  `workspace:` is `scope='workspace'` with a null `project_id`. See the
  resolution table under `record_relations` in the design.
- `supersedes` / `superseded_by` are **same-tier only**, so their entries are
  always bare. Treat a qualified entry in either field as malformed: report it
  and leave the relation unwritten rather than resolving it.
- **An unresolvable reference is reported and skipped, never fatal.** A
  reference to a project or record not in the database leaves the relation
  unwritten and adds a line to the run summary. Failing would make ingest
  order significant, which the two-pass design exists to avoid.
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
- **A project named in frontmatter but absent from the database is created.**
  The plan did not say, and the alternative — reporting and skipping — would
  make "ingesting `clasm/decisions/` yields 169 records" unsatisfiable against
  a database that has never seen `clasm`. Creation is additive and reported.
- **`initiative` is materialised as a concept link during pass one**, per
  design decision 3, which the W3 work items had omitted entirely. No live
  record populates the field (0 of 198), so it is covered by a fixture test
  rather than a corpus one.
- **Relations are resolved for skipped records too.** A file unchanged by
  checksum still has its references re-resolved, because re-running is the
  documented remedy for a reference whose target had not yet been ingested —
  if a skip also skipped resolution, that remedy would never work.
- FTS indexing lives in `AddRecord`, not in ingest, so every writer maintains
  the index. `records` rows are indexed with `source_type = 'record'`,
  `label = DR-NNNN` and the title prepended to the body so titles are
  searchable.

### Acceptance criteria

- Ingesting `~/WorkLab/clasm/decisions/` yields 169 records, 2
  `supersedes` relations and 3 `relates_to` relations.
- A second run reports 169 skipped, 0 updated.
- Touching one file's body and re-running reports exactly 1 updated.
- Forward reference proven: ingest a directory where `0002` supersedes
  `0003` and confirm the relation resolves.
- Ingesting `~/WorkLab/agents/decisions/` resolves DR-0001's
  `relates_to: ["clasm:0160"]` to the `clasm` record — the first real
  cross-tier reference, and the only one that exists at the time of writing.
- Ingesting `~/WorkLab/agents/decisions/` *before* `clasm/decisions/` leaves
  that relation unwritten, reports it in the summary, exits 0, and resolves it
  on a later run once `clasm` has been ingested.
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
- **A bare RECORD_ID is not an identity.** Two projects may each have a
  DR-0001, so `show`/`set-status`/`supersede` resolve a bare id across every
  tier and report the candidates when more than one matches, rather than
  picking. `--project P` and `--workspace` qualify it.
- **The writers re-render canonically rather than editing the status line in
  place.** One writer, one rule — `RenderRecordFile` — so `record`, `fmt` and
  anything later cannot drift. In practice this satisfies "leaving every other
  line untouched" anyway, because 192 of 198 records are already canonical: a
  fresh whole supersession changes exactly the `status` and `superseded_by`
  lines and nothing else. Where a file is *not* canonical, the command says so
  rather than normalising it silently.
- **`supersede` is idempotent.** Re-running it against an already-superseded
  pair rewrites nothing, verified against `clasm` DR-0149/DR-0148, which are
  already in that state in the live corpus.

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

## W5 — `kb record new`, `kb record fmt`, and the identity-collision guard

### Work items

- **Refuse to overwrite a record that is not the same record.** Two workspaces
  each have an `agents/decisions/`, and a workspace-tier record's identity is
  `(NULL project, "workspace", record_id)` — unique only *within one
  workspace*, which nothing states and nothing enforces. Ingesting both
  tiers into one database silently overwrites the first with the second,
  reported as a routine `1 updated`. Demonstrated, not theorised.

  The guard is `uuid`-based rather than workspace-aware, because it needs no
  registry and generalises beyond this case: when an incoming record matches an
  existing identity but both carry a **non-empty and differing `uuid`**, it is a
  different record wearing the same id. Refuse the write, count it as failed,
  report it, and let the rest of the run proceed — a run should survive a
  collision, but data must not be destroyed by one. Where the uuids cannot
  settle it (either is empty) but the stored `path` differs, warn instead:
  a slug is cosmetic and may be regenerated, so a changed path alone is not
  proof of a collision.

  **This is interim.** The real fix is to carry the workspace's actual name, so
  that a workspace-tier record's identity is complete and `workspace:` in a
  reference says which workspace it means. That change would also remove the
  NULL `project_id` and retire DR-0004's `IFNULL` index, so it is its own
  effort. The guard exists so the Laboratory workspace tier can be created
  safely before that lands.

- `kb record fmt PATH [--dry-run]` — parse each record under `PATH` with W2's
  `ParseRecordFile` and write back `RenderRecordFile`'s output, bringing a
  corpus into canonical form. Nearly free once W2 exists, being parse plus
  render, but it needs to exist as its own verb: W3's rule is that **ingest
  never writes to record files**, so normalisation cannot be a write-back flag
  on `ingest`. Report per-file whether anything changed, and honour `--dry-run`.
  Its first real use is bringing `~/WorkLab/CMTools/decisions/` into line — the
  six records whose `decisions[]` uses a block sequence, which the format's
  "inline lists only" rule forbids and which W2 confirmed are the *only* six
  divergences in the whole 198-record corpus.
- `kb record new --project P --title T [--kind K] [--trigger G]` — allocate
  the next id for the scope, fill `date`, `uuid`, `origin_host`, `project`,
  set `status: proposed`, and emit **all five body headings** even when
  empty.
- `trigger` is **required here**, unlike on converted records. The empty-
  trigger concession is for conversion only.
- Writes the file; does not ingest it.

### Acceptance criteria

- Ingesting two different workspaces' `agents/decisions/`, each holding a
  DR-0001 with its own `uuid`, into one database leaves the first intact,
  reports the second as a collision, and exits 0.
- Re-ingesting all five live corpora still reports 0 failed: the guard fires on
  differing uuids, never on a record matching itself.
- `record fmt` over `CMTools/decisions/` rewrites exactly 6 files and reports
  the other 7 unchanged; a second run reports 13 unchanged.
- `record fmt --dry-run` leaves every file byte-identical.
- Into `clasm/decisions/` allocates `0170`.
- Emitted file parses cleanly under W2 and ingests under W3.
- Omitting `--trigger` is a usage error (exit 2).
- Scaffolds `Context`/`Decision`/`Rationale`/`Rejected alternatives`/
  `Consequences` headings. In `clasm` the latter two appear in only 98 and
  99 of 169 entries; an empty heading is a cheaper prompt than discipline.

---

## W6 — `kb index` and search surfacing

**`kb index` supersedes `~/WorkLab/decisions_index.ts`.** That Deno tool was
written 2026-08-25 because no index generator existed and `cold` needed one;
it currently maintains all five indexes (`WorkLab/agents/decisions`,
`WorkLab/CMTools/decisions`, `WorkLab/cold/decisions`,
`WorkLab/clasm/decisions`, and this module's own). One implementation
serving both workspaces was this design's original argument for putting
`index` in `kb`, so the Deno tool is a stopgap and retires when this phase
ships. Until then it is the live generator and its output is the reference
behaviour.

### Work items

- `kb index PATH` — regenerate `decisions/index.md`: one greppable line per
  record, **newest first**, never hand-edited. This preserves the
  `head`/`grep` affordance that top-insertion in a single `DECISIONS.md`
  originally provided.
- **Match the line format in `DECISION_RECORD_FORMAT.md`, "The index"**, as
  settled by `WorkLab/agents/decisions` DR-0001: columns are `DR-<id>`,
  `date`, `status`, `kind`, `trigger`, supersession flag, title. Every column
  holds a value — an empty one renders as `-`, never as spaces, so the title
  is always at `$7`. The flag reads `sup` when `superseded_by` is non-empty,
  `-` otherwise. Sort by `date` then `record_id`, both descending, never by
  `record_id` alone.
- **The attribution line names no tool.** `decisions_index.ts` writes
  `Generated by decisions_index.ts. Do not hand-edit.`, which `kb index`
  cannot reproduce without lying about itself. Change it to a tool-neutral
  line — `Generated file. Do not hand-edit.` — in `decisions_index.ts` first,
  regenerate the five live indexes, and have `kb index` emit the same.
  Otherwise every index churns on the changeover and the byte-comparison
  below is impossible to satisfy honestly. (Done: `decisions_index.ts` already
  emits the neutral line and all five indexes are regenerated.)
- Do not write a preamble into `index.md`. There is nowhere else for directory
  prose to go either: `decisions/README.md` was **removed from the format** on
  2026-08-25 by workspace DR-0006, and a `decisions/` directory now holds
  records and the generated `index.md` and nothing else. `kb index` must not
  create one.
- `kb search` returns `record` rows alongside projects, observations and
  concepts, labelled by `source_type`.
- TUI: records browsable under a project. Read-only, consistent with the
  TUI's existing scope.

### Acceptance criteria

- `kb index` output is **byte-identical** to `decisions_index.ts` output for
  all five live corpora — 169 records for `clasm`, 13 for `CMTools`, 7 for
  `cold`, 6 for `agents/decisions`, 3 for `knowledge` — both emitting the
  tool-neutral attribution line. This is the real acceptance test for the
  phase: two independent implementations agreeing on every byte of a 169-row
  file, and on four smaller files that between them exercise every optional
  field `clasm` leaves empty.
- Title at `$7` on every row under `awk` default field splitting, including
  rows with an empty `trigger`. `clasm` has 99 such rows.
- `clasm:DR-0160` (accepted, partially superseded) renders with the `sup`
  flag; `DR-0148` (wholly superseded) renders `superseded` *and* `sup`.
- Running twice produces an identical file.
- `kb index` writes `index.md` and nothing else — in particular it does not
  create a `README.md` (workspace DR-0006).
- A search whose top hit is a record body reports it as `record`, with its
  `DR-NNNN`.

### On retiring the Deno tool

Delete `decisions_index.ts`, `decisions_index_test.ts`, its `deno.json` test
and build entries, and the `bin/decisions_index` build artifact — but only
after the byte-identical criterion above passes, since that comparison is
what proves `kb index` is a faithful replacement. `decisions_split.ts` is
unaffected and stays: it converts monolithic `DECISIONS.md` files, which is
explicitly out of scope for `kb` (design, "What this design does not cover").

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
