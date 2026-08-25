# Decision records and document ingest — Design

**Status (2026-08-25):** Audit complete; decisions 1–5 confirmed 2026-08-24,
decision 6 (cross-tier references) added 2026-08-25. `decision-records-plan.md`
exists; nothing implemented yet.

**References:**
- `~/WorkLab/DECISION_RECORD_FORMAT.md` — the file format this design
  ingests. Settled 2026-08-24, amended 2026-08-25 with the "Cross-tier
  references" section; frontmatter schema, vocabularies, supersession rules
  and the reference grammar are specified there, not here.
- `~/WorkLab/agents/decisions/` — the workspace tier's own records. DR-0001
  and DR-0002 both constrain this design: the generated index format, and the
  cross-tier reference grammar behind decision 6 below.
- `module-extraction-design.md` — established this module as the owner of
  the knowledge base, with `harvey` as a consumer. The same relationship
  applies to whatever writes decision records.
- `cli-tui-design.md` — the `TOOL VERB PARAMETERS` CLI model and the
  `--json` contract that new verbs below must follow.

## Motivation

The knowledge base is currently a hand-curated summary layer, not an index
of the workspace's actual artifacts. Measured against
`~/WorkLab/agents/knowledge.db` on 2026-08-24:

| Table | Rows |
|---|---|
| `observations` | 215 |
| `projects` | 11 |
| `concepts` | 8 |
| `sources` | 1 |
| `kb_fts` (total) | 234 |

Against that, the artifacts those 234 rows are meant to represent:

| Artifact | Size | In the knowledge base? |
|---|---|---|
| `clasm/DECISIONS.md` | 169 entries, 9,338 lines | No — 79 `decision` observations shadow it |
| `clasm/DESIGN.md` | 4,501 lines | No |
| `clasm/PLAN.md` | 7,146 lines | No |
| `agents/hand-off/*.spmd` | 34 files | No — 7 observations merely mention hand-offs |
| `agents/project_notes/*.md` | 13 files | No |

Two problems follow. **Drift:** 79 `decision`-kind observations are a lossy,
hand-written shadow of 169 real decisions, and nothing keeps them in step.
**Unreachability:** the highest-value content in the workspace — the
reasoning behind five years of decisions — cannot be retrieved at all,
because it lives in files the knowledge base has never read.

The `agents/` directory is explicitly shared infrastructure for both Claude
Code and Harvey's local Ollama models. Retrieval over these artifacts is
exactly the kind of grounding a small local model can do adequately, so
closing this gap is not Claude-specific work.

## What already works, and does not need changing

Worth stating, because it narrows the design considerably:

- **`kb_fts` accommodates records as-is.** Its columns are `body`, `kind`,
  `label`, `descr`, `source_type`, `source_id`, `project_id`. A decision
  record maps onto them without a schema change: `label` = title, `kind` =
  the record's kind, `source_type` = `'record'`. No FTS migration.
- **Lazy migration is an established pattern.** `Open()` applies
  `CREATE TABLE IF NOT EXISTS` plus the idempotent `kbAlterStmts`/
  `kbSourcesAlterStmts` lists. New tables follow that precedent directly.
- **The CLI contract is already right for this.** `TOOL VERB PARAMETERS`,
  global `--db`/`--json`/`--debug`, errors always to stderr. New verbs
  inherit all of it.
- **`PRAGMA busy_timeout = 5000`** is already set — necessary, since ingest
  will run concurrently with interactive `kb` use.
- **`uuid`/`origin_host`** are already on every table for cross-machine
  `merge`. Records must carry them too, for the same reason.

`gopkg.in/yaml.v3` is present in `go.mod` but currently marked
`// indirect`; the first real import plus `go mod tidy` promotes it to the
direct require block.

## What's missing (audited 2026-08-24)

| Gap | Consequence |
|---|---|
| No document/record entity | Files cannot be first-class. `sources` is for cited literature — DOIs, retraction checking — and is the wrong tool for local artifacts |
| No ingest path | Everything is hand-entered, so it drifts. This is the root cause of the 79-vs-169 gap |
| No same-type relations | `observation_concepts` and `project_concepts` exist, but there is no observation↔observation or record↔record edge, so `supersedes` cannot be expressed |
| Flat `projects` | No way to express a multi-repo, multi-year initiative such as EPrints → Invenio RDM, which spans eight repositories |
| Undocumented `kind` vocabulary | `finding`/`decision`/`note`/`release`/`question` emerged organically. A model has no way to learn the set; `kb help observation` does not list it |

## Proposed schema additions

### `records`

```sql
CREATE TABLE IF NOT EXISTS records (
    id           INTEGER  PRIMARY KEY AUTOINCREMENT,
    record_id    TEXT     NOT NULL,                    -- "0142", zero-padded
    project_id   INTEGER  REFERENCES projects(id) ON DELETE SET NULL,
    scope        TEXT     NOT NULL DEFAULT 'project',  -- project | workspace
    path         TEXT     NOT NULL,                    -- relative to workspace root
    title        TEXT     NOT NULL,
    date         TEXT     NOT NULL,                    -- YYYY-MM-DD
    status       TEXT     NOT NULL DEFAULT 'proposed',
    kind         TEXT     NOT NULL DEFAULT 'decision',
    trigger      TEXT     NOT NULL DEFAULT '',
    phase        TEXT     NOT NULL DEFAULT '',
    initiative   TEXT     NOT NULL DEFAULT '',
    session      TEXT     NOT NULL DEFAULT '',
    body         TEXT     NOT NULL,
    checksum     TEXT     NOT NULL DEFAULT '',
    ingested_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    uuid         TEXT     NOT NULL DEFAULT '',
    origin_host  TEXT     NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_records_scope_id
    ON records(IFNULL(project_id, -1), scope, record_id);
```

The identity index is expressed over `IFNULL(project_id, -1)` rather than
`project_id` directly. This design originally specified the plain three-column
form, which places **no constraint at all on the workspace tier**: those
records carry a NULL `project_id` — that is how the tier is distinguished —
and SQLite treats NULLs in a unique index as distinct from one another. Every
workspace record would be unique to itself, so re-ingesting
`~/WorkLab/agents/decisions/` would duplicate all six on every run, defeating
the `checksum` idempotency below for the one tier both workspaces re-ingest.
Corrected during W1; see `decisions/` DR-0004.

`path` is stored **relative to the workspace root**, not absolute. Absolute
paths do not survive `merge` between machines, which is the whole point of
carrying `uuid`/`origin_host`.

`checksum` makes ingest idempotent: unchanged files are skipped, so
re-ingesting the whole tree is cheap and safe to run repeatedly.

### `record_relations`

```sql
CREATE TABLE IF NOT EXISTS record_relations (
    from_id      INTEGER NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    to_id        INTEGER NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    relationship TEXT    NOT NULL,          -- supersedes | relates_to
    PRIMARY KEY (from_id, to_id, relationship)
);
```

Only one direction is stored. `superseded_by` is the inverse of
`supersedes`, computed on read. The *file* denormalizes and stores both
sides, because a human opening a stale record must see it is stale without
running a query; the database has no such need.

Ingest is therefore **two-pass**: pass one upserts every record, pass two
resolves relations. A record may legitimately reference one that has not
been read yet.

#### Resolving a reference

A `relates_to` entry is `[<scope>:]<id>` — see `WorkLab/DECISION_RECORD_FORMAT.md`,
"Cross-tier references", settled by `WorkLab/agents/decisions` DR-0002 on
2026-08-25. Resolution needs the three values in
`idx_records_scope_id (project_id, scope, record_id)`, and the entry form
supplies them:

| Entry | `scope` | `project_id` | `record_id` |
|---|---|---|---|
| `0159` | the citing record's | the citing record's | `0159` |
| `clasm:0160` | `project` | lookup by `projects.name` | `0160` |
| `workspace:0001` | `workspace` | `NULL` | `0001` |

Split on the first `:` only, and strip an optional leading `DR-` from the id
part rather than rejecting it. A stray prefix in one cross-reference should
not fail a run, on the same reasoning as decision 5 below.

`supersedes` and `superseded_by` are **same-tier only**, so their entries are
always bare ids and inherit the citing record's scope and project. The file
format forbids the cross-tier case, because writing both sides would mean
writing into another repository. Ingest does not need to handle it, and should
report a qualified entry in either field as malformed rather than resolving it.

**An unresolvable reference is reported and skipped, not fatal.** Following
decision 4 (additive only), a reference to a project or record not present in
the database leaves the relation unwritten, adds a line to the run summary, and
lets the rest of the ingest proceed. The overwhelmingly common cause is a
record that has not been ingested yet — a tree ingested one project at a time,
or a workspace-tier record read before the project it cites. Failing the run
would make ingest order significant, which the two-pass design exists
specifically to avoid.

This does mean a relation can stay unwritten indefinitely if the target never
arrives. Re-running ingest over the full tree is cheap by design — unchanged
files are skipped by checksum — so the remedy is to re-run, and the summary
line is what tells the operator to.

## Proposed verbs

### `kb ingest PATH [--dry-run]`

Walk a directory tree, parse YAML frontmatter, upsert into `records` and
`kb_fts`, skip unchanged files by checksum. Report counts of added,
updated, skipped, and failed.

This is the highest-value single addition in this design. It is also what
makes the knowledge base genuinely model-agnostic: any harness that can run
a command and read JSON gets the whole corpus, with no Claude-specific
integration.

Handling of a file that disappears between runs is an open question, below.

### `kb record list|show|new|supersede|set-status`

- `list` — filter on `--project`, `--status`, `--kind`, `--trigger`,
  `--initiative`, `--since`
- `show RECORD_ID` — full body
- `new --project P --title T` — scaffold a file: allocate the next `id`,
  fill `date`/`uuid`/`origin_host`/`project`, set `status: proposed`, and
  print all five recommended body headings. Writes the file; does not
  ingest it
- `supersede NEW OLD` — write both sides of the link, in both files, and
  set `OLD`'s status to `superseded`. Atomic, so the two sides cannot drift
- `set-status RECORD_ID STATUS` — the promotion path from `proposed` to
  `accepted`

### `kb index PATH`

Generate `decisions/index.md` — one greppable line per record, newest
first, per `DECISION_RECORD_FORMAT.md`. Generated, never hand-edited.

Placing index generation here rather than in a WorkLab shell script keeps
one implementation for both `~/WorkLab` and `~/Laboratory`.

## Decisions (confirmed 2026-08-24)

**1. Frontmatter `type:` is renamed `kind:`.** Every existing table in this
schema uses `kind` (`observations.kind`), and `type` is awkward in both SQL
and Go. Renaming the frontmatter field cost nothing — nothing was
implemented — and avoids a permanent file-to-column mapping. *Confirmed
2026-08-24; `DECISION_RECORD_FORMAT.md` updated the same day.*

**2. Records become authoritative for decisions; existing `decision`
observations are left alone.** The 79 `decision`-kind observations stay as
historical data. New decisions go to `records` only, so the shadow stops
growing. Ingest never deletes or rewrites an observation. *Confirmed
2026-08-24 — silent deletion of hand-written history is not worth the
tidiness.*

**3. Initiatives use `concepts`, not a new table.** `project_concepts`
already links many projects to one concept, so an `eprints-to-rdm` concept
linked to all eight repositories expresses the grouping today at zero
schema cost. The `initiative` frontmatter field is materialised as a
concept link at ingest. *Confirmed 2026-08-24; revisit only if the pilot
shows concepts cannot carry it.*

**4. Ingest is additive-only in v1.** A record file deleted from disk stays
in the database until explicitly removed. The alternative — pruning rows
whose files vanished — risks destroying data on a partial or
wrong-directory ingest run. *Confirmed 2026-08-24: additive-only, with
`kb record remove` as the explicit path; reconsider after real use.*

**5. The `kind` vocabulary is documented, not enforced.** List the known
values in `kb help observation` and `kb help record`; do not reject unknown
ones. Enforcement in a tool several harnesses write to would turn a typo
into a failed run rather than a fixable row. *Confirmed 2026-08-24:
documented, not enforced.*

**6. A cross-tier reference is a qualified `relates_to` entry, and an
unresolvable one is skipped rather than fatal.** The entry grammar is
`[<scope>:]<id>`, resolved as tabulated under `record_relations` above;
supersession stays same-tier, so ingest never has to resolve a cross-repo
supersession. A reference whose target is absent leaves the relation unwritten
and adds a line to the run summary. *Confirmed 2026-08-25, deciding the
question this design had left implicit: `record_relations` could already store
a cross-tier relation, while the file format had no syntax to express one, so
ingest could never have populated it. Settled in `WorkLab/agents/decisions`
DR-0002.*

## Consequences outside this repo

- `~/WorkLab/CLAUDE.md` documents a grounding query against `kb_fts` whose
  `source_type` values are listed as `project`, `observation`, `concept`.
  Adding `record` means that text and the `rag-query` skill both need
  updating, or grounding queries will silently keep missing the richest
  source in the database.
- `~/WorkLab/agents/skills/` has `kb-schema`, whose stated job is to emit
  the DDL so a model has the schema in context without querying. It must be
  regenerated after these tables land.

## What this design does *not* cover

- **Ingesting Fountain hand-offs, project notes, DESIGN.md or PLAN.md.**
  All 34 hand-off files and 13 project notes are outside the knowledge base
  today and would benefit from the same treatment, but they have no
  frontmatter and no stable identity. The `records` table is deliberately
  shaped generally enough to hold them later; doing so is its own effort.
- **Embeddings or vector search.** FTS5 is what exists and what the
  `rag-query` skill uses. Semantic retrieval is a separate question.
- **Writing decision records from within `kb`'s TUI.** The TUI is
  read-mostly by design; `record new` is a CLI verb.
- **The conversion of `clasm/DECISIONS.md` itself.** That is a one-off
  script in the WorkLab pilot, not a `kb` feature. Its acceptance test —
  regenerate and diff byte-for-byte — is specified in
  `DECISION_RECORD_FORMAT.md`.
