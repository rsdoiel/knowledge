---
name: review-knowledge-base
description: |
  Queries agents/knowledge.db via the kb CLI and delivers a structured
  report covering projects (by status), recent observations, concepts,
  decision records, narrative documents, and the document review queue
  (sections needing a summary drafted or promoted). Can drill into a
  single project for full detail, run a full-text search, check whether any
  corpus's index.md has gone stale, or surface candidate new concepts. Use
  when the user asks to "review the knowledge base", "show projects",
  "summarize the knowledge base", "what needs review", "is the index
  stale", "what concepts are missing", or "what's in the knowledge base".
compatibility: claude-code, harvey
metadata:
  author: R. S. Doiel
  version: 0.9.0
  kb_version: 0.0.28
variables:
  DB_PATH:
    type: string
    description: Path to the SQLite3 database file. Defaults to agents/knowledge.db.
    example: "agents/knowledge.db"
  FOCUS:
    type: string
    description: "What to report: all | projects | observations | concepts | concept-candidates | records | documents | review-queue | index-check | search:<term> | <project-name>"
    example: "all"
---

This skill reads `knowledge.db` via the `kb` CLI and delivers a structured
report. By default (`FOCUS=all`) it covers every section. Set `FOCUS` to a
specific section, a `search:<term>` query, or a project name for a focused
report.

Prefer `kb` verbs over raw `sqlite3` SELECTs wherever one exists — they're
schema-version-proof (a future `knowledge` module release could add columns
or lazy-migrate old rows without this skill needing to change) and already
match exactly what `kb`'s own TUI and `--json` scripting consumers see. One
section below (`observations`, the cross-project recent-activity view) has
no direct `kb` verb yet and falls back to a read-only `sqlite3` query —
called out explicitly where that happens.

---

## Requires kb >= v0.0.19

Run `kb -version` first. If it is older, stop and tell the user to upgrade
(`curl https://rsdoiel.github.io/knowledge/installer.sh | sh`): an older `kb`
rejects the forms below, and some of them would fail without a word under a
`2>/dev/null`. What changed since v0.0.17, for this skill:

- A decision record is named `SCOPE/DR-NNNN`, where SCOPE is a project name or
  `workspace`: `harvey/DR-0004`, `workspace/DR-0003`. Listings print it, and the
  bare id is not enough to act on.
- `--project` and `--workspace` still work on the record verbs but are
  deprecated and print a note on stderr. Prefer a scope argument
  (`kb record list harvey`) or the qualified reference.
- With no scope, `record list` and `record pending` use the project the
  **current directory** belongs to, then `KB_PROJECT`, then the whole workspace.
  A skill that wants the whole workspace passes `--all`.
- `kb` finds the workspace by walking up from the current directory, so it works
  from any subdirectory; each workspace is independent of the others.
- This skill reports on the **whole workspace**, so every `record list` below carries
  `--all`; `record pending --all` is the one-line answer to "what is waiting for a
  decision". A single project is named as the scope argument.
- **v0.0.18, v0.0.19:** accepting a record (`record set-status REF accepted`) and
  promoting a document summary (`document review promote`) **exit 2 without a
  terminal**, and no flag turns that off. This skill never does either; a `proposed`
  record or a `drafted` summary is reported, and the user decides at a terminal
  (`! kb record set-status REF`). `kb ingest` prints `status:` lines for a status that
  arrives through a file (`proposed -> accepted (edited in file)`, `arrived
  accepted`): include them in the report, as findings about how an acceptance got
  there, and do not act on them. `cancelled` now means pursued or explored and then
  abandoned, and need not have been accepted. `kb -json verbs` lists every verb,
  subverb and flag with what it does to the database; use it to check that a form
  exists. `kb` with no verb, a bare group and `kb -i ...` need a terminal; give the
  full command.
- **v0.0.23 (nothing here needs it; the minimum stays 0.0.19):** `kb ingest` on a
  project directory (`agents/projects/NAME`) skips `notes/`, `design/`, `plans/`,
  `feature_requests/` and `hand-off/`, and a file named `YYYY-MM-DD-...md` is a note,
  never a record, so a note needs no frontmatter. `kb document ingest` offers
  frontmatter for a file that has none only on a terminal, so a model running it
  never sees the prompt; pass `--no-prompt` anyway when a script may be given a
  terminal. A search hit carries a record's reference, status and the matching
  passage, and `kb stats` counts rows, links and checks without SQL.
- **v0.0.25 (nothing here needs it; the minimum stays 0.0.19):** `kb stats` is the
  way to count; use it in place of the `sqlite3` counts below where it covers the
  question. `kb story check --project P` reports continuity problems in a writing
  project's manuscript (see `FOCUS = story:<project>`). `concept suggest` ranks by a
  damped frequency, so the candidate list holds fewer function words and fewer
  wrongly merged variants (DR-0085).
- **v0.0.26 (nothing here needs it; the minimum stays 0.0.19):** `kb story check` reports
  facts that differ only in a digit as `fact-number-differs` (the same figure, or a
  continuity slip?), apart from the `unknown-fact` typo finding.
- **v0.0.27 (nothing here needs it; the minimum stays 0.0.19):** `kb document ingest`
  prints a `tag warning:` line for a wikilink or keyword it skipped (a control
  character); list any in the report, it is not a failure.
- **v0.0.28 (nothing here needs it; the minimum stays 0.0.19):** every manual page has an
  EXIT STATUS section, and an unreadable database file now exits 77 (it was 74); a script
  that only tests for success or failure is unaffected.

---


## Variables

| Variable  | Type   | Description                                              | Default               | Prompted? |
|-----------|--------|------------------------------------------------------------|--------------------------|-----------|
| `DB_PATH` | string | Path to the SQLite3 database file.                         | `agents/knowledge.db`    | No        |
| `FOCUS`   | string | Scope of the report (see values below).                    | `all`                    | No        |

### FOCUS values

| Value                  | Report scope                                                        |
|--------------------------|-------------------------------------------------------------------------|
| `all`                   | Full report: projects, recent observations, concepts, records, documents, review queue, sync check, index check |
| `projects`              | Project summary table only (`kb project list`)                         |
| `observations`          | Last 20 observations across all projects (raw read-only query — see below) |
| `concepts`              | All concepts (`kb concept list`)                                       |
| `concept-candidates`    | Candidate *new* concepts mined from the corpus, read-only (`kb concept suggest`) |
| `stats`                 | Row, link and check counts without SQL (`kb stats`, kb >= v0.0.23) |
| `story:<project>`       | A writing project's timeline problems and continuity check, read-only (`kb story check`, kb >= v0.0.24) |
| `sync-check`            | Whether the database and its JSONL dump agree by content, with a recommendation (`kb check-db`, kb >= v0.0.15) |
| `index-check`           | Whether any corpus's `decisions/index.md` is stale (`kb index ROOT --all --check`) |
| `records`               | Decision record summary (`kb record list`)                             |
| `documents`             | Narrative/article document list (`kb document list`)                   |
| `review-queue`          | Document sections needing a summary drafted or promoted (`kb document review list`) |
| `search:<term>`         | Full-text search across projects/observations/concepts/records/reviewed document summaries (`kb search`) |
| `<project-name>`        | Full detail for one project: metadata + concepts + observations + records + documents |

---

## Step 0 — Resolve DB_PATH and locate the `kb` binary

```bash
DB_PATH="${DB_PATH:-agents/knowledge.db}"
FOCUS="${FOCUS:-all}"
KB="$(command -v kb || echo "$HOME/Laboratory/knowledge/bin/kb")"
```

If `DB_PATH` doesn't exist, report the error and suggest running
`setup-knowledge-base` (though most `kb` commands below will just report
"no projects"/"no results" against a freshly auto-created empty database
rather than erroring).

---

## Step 1 — Run queries

### Projects section (`FOCUS` = `all` or `projects`)

```bash
"$KB" --db "$DB_PATH" --json project list
```

Group the output by `status` and label each group:
- **Active** — `status = "active"`
- **Concept** — `status = "concept"`
- **Paused** — `status = "paused"`
- **Concluded** — `status = "concluded"`

(Use `--json` here specifically so status can be grouped programmatically;
plain `kb project list` prints one line per project already sorted by id,
which is fine too for a quick unstructured look.)

---

### Observations section (`FOCUS` = `all` or `observations`)

No `kb` verb currently returns "last N observations across every project"
in one call (`kb observation list` is per-project; `kb summary` shows only
the 3 most recent per project). Use a read-only `sqlite3` query for this
one section — it only *reads*, so none of the uuid/FTS-consistency
concerns that rule out raw SQL for writes apply here:

```bash
sqlite3 -header -column "${DB_PATH}" \
  "SELECT o.id,
          p.name AS project,
          o.kind,
          o.created_at,
          substr(o.body, 1, 80) AS body_preview
   FROM observations o
   JOIN projects p ON p.id = o.project_id
   ORDER BY o.created_at DESC
   LIMIT 20;"
```

(Note the column names: `project_id`/`created_at`, not the old schema's
`experiment_id`/`recorded_at`.) For each observation where `body` was
truncated, note that the user can ask to see the full text — via
`kb observation show ID`, which also resolves and prints `supersedes`/
`superseded_by` if the observation has been corrected or has a correction
(DR-0023): an observation is never edited in place, so a superseded one's
`body` is the original wording, not stale data — say so if you surface one,
rather than presenting it as current.

---

### Concepts section (`FOCUS` = `all` or `concepts`)

```bash
"$KB" --db "$DB_PATH" concept list
```

`kb concept show NAME` (kb >= v0.0.15) prints one concept's linkage counts
per kind (projects, observations, records, document sections) with the newest
items; use `--limit 0` for counts only. For the whole list, `kb concept list`
doesn't print linkage counts (how many
projects/observations reference each concept). If the user specifically
wants those counts, either drill into a project with `kb project concepts
NAME`, or fall back to a read-only query:

```bash
sqlite3 -header -column "${DB_PATH}" \
  "SELECT c.name,
          c.description,
          count(DISTINCT pc.project_id) AS projects_linked,
          count(DISTINCT oc.observation_id) AS observations_linked
   FROM concepts c
   LEFT JOIN project_concepts pc ON pc.concept_id = c.id
   LEFT JOIN observation_concepts oc ON oc.concept_id = c.id
   GROUP BY c.id
   ORDER BY c.name;"
```

---

### Candidate concepts (`FOCUS` = `all` or `concept-candidates`)

```bash
"$KB" --db "$DB_PATH" concept suggest --limit 20
```

Read-only, and the complement to the section above: `concept list` shows
what the corpus already knows, this shows terms it arguably *should* know.
Ranked by corpus-wide distinctiveness — mentioned several times, confined to
relatively few items — over every record body and document section body.
Scope with `--project NAME`.

From kb v0.0.25 the score is `(1 + ln occurrences) x idf` and numbers and common
adverbs are not candidates; variants are merged only when they are forms of one
word. Still present the list as suggestions, not findings.

Two things to say plainly when reporting it, rather than presenting the
list as findings. It is a mechanical pass, **roughly half signal**: run
live against the real corpus it returned genuinely useful terms alongside
generic nouns like `row`, `name` and `table` (DR-0028). And it **never
writes** — a candidate becomes a concept only when a human runs
`kb concept add`. Offer the list for the user to pick from; never add them
on their behalf.

In a `FOCUS=all` report keep this short — the top 10 is plenty — since it
is a prompt for curation, not a description of the corpus's state.

**Output shape (kb >= v0.0.11, DR-0034).** Spelling variants of one term
are merged before scoring and shown as one line, `rename (+renamed,
renaming, renames)`, so count that as a single candidate. Below the
candidates the command prints a `near-existing (excluded from candidates):`
block: terms that are close spellings of an *already-known* concept
(`collision ~ collisions`). Those are not new-concept candidates; they are
what `kb document fuzzy-tag` exists to footnote. Report the count, and
treat the block as **noisy**, not as findings: on the real corpus it held
plausible pairs beside false ones (`nested ~ testing`, `around ~
grounding`). With `--json` the shape is `{"candidates": [...],
"near_existing": [...]}`, where before v0.0.11 it was a bare array. Fixed in kb v0.0.12 (DR-0036): a first-letter rule removes those three false pairs, and stops `preference` clustering into `reference`.

---

### Counts (`FOCUS` = `all` or `stats`)

```bash
"$KB" --db "$DB_PATH" stats
```

Rows per table, links per join table, and checks (observations without a project,
concepts and sources nothing links to, records and documents without a project).
`concepts_without_links` and `sources_without_links` are not errors; report them as
counts. Prefer this to hand-written `count(*)` queries.

### Story check (`FOCUS` = `story:<project>`)

```bash
"$KB" --db "$DB_PATH" story check --project "PROJECT"
"$KB" --db "$DB_PATH" story timeline --project "PROJECT"
```

Read-only, and only for a project whose manuscript files were ingested with
`kb document ingest`. List the findings as printed, grouped by kind; they are
advisory (`two-places`, `unknown-fact`, `fact-number-differs` and
`knowledge-before-learned` may be deliberate), so do not call them errors. A `~` row in the timeline is an order that
is not established; never present it as ordered. An unknown project is exit 1.

### Sync check (`FOCUS` = `all` or `sync-check`)

**Requires kb >= v0.0.15** (DR-0053).

```bash
"$KB" --db "$DB_PATH" check-db
```

Compares the database with the JSONL dump beside it (the database path with
`.jsonl` for `.db`; both resolved paths are printed) by full content, never by
file time, and prints per-table counts, rows only in the dump, rows only in the
database, and rows in both that differ, then a recommendation: `kb import`,
`kb export`, or both. Read-only. File times appear as a hint only.

Exit 0 is in sync, and exit 1 is **not in sync, with the report still
printed**: that is the finding, not a failure. Exit 66 means there is no dump
(say so; nothing to compare), 65 a malformed dump. Report the recommendation
and do not run it: an import brings back anything that was deleted here on
purpose, because there are no tombstones. In a `FOCUS=all` report this is one
line ("database and dump agree") when in sync.

---

### Index check (`FOCUS` = `all` or `index-check`)

```bash
"$KB" index "$HOME/Laboratory" --all --check
```

Walks every decision-record corpus under the given root and reports whether
each one's `index.md` matches a freshly rendered copy, writing nothing.
Exits non-zero if any corpus needs attention, so it doubles as a check in a
script. `--check` is what makes this safe to run in a review: without it,
`--all` *rewrites* every index it finds.

A stale index is a real finding worth reporting — nothing detects it
automatically, and `kb record set-status`/`supersede` only auto-refresh the
one corpus they touched. The fix is the same command without `--check`.

**Requires kb >= v0.0.10 with DR-0031.** Before that fix `--all` descended
into hidden directories, so a git worktree under `.claude/worktrees/`
reported its own copy of a corpus as a separate one. If you see a path with
a `.`-prefixed component in the output, the `kb` on this machine predates
the fix: those lines are duplicates of a corpus already listed, so don't
report them as findings, and never run the write form to "fix" one — it
would edit a throwaway worktree rather than the real tree. Rebuild `kb`
rather than filtering the output.

In a `FOCUS=all` report this is one line when everything is current
("N corpora, all up to date"); expand it only when something is stale.

---

### Records section (`FOCUS` = `all` or `records`)

```bash
"$KB" --db "$DB_PATH" record list --all
"$KB" --db "$DB_PATH" record pending --all
```

One line per decision record: its reference (`project/DR-NNNN`, `workspace/DR-NNNN`),
date, status, kind, trigger, title. `pending` is the `proposed` ones, oldest first.
Records are written `proposed` and only their own author promotes them
(`kb record set-status`) — report the status breakdown (how many
`proposed` vs. `accepted`, and any `superseded`, `rejected` or `cancelled`: a
`cancelled` record was pursued or explored and then abandoned (it need not have been accepted), so its reasoning still stands but the
work is not live) rather than treating every record as settled.

For one record's concept tags — what `kb ingest` resolved from its
`[[wikilinks]]` and frontmatter `tags:` — drill in with:

```bash
"$KB" --db "$DB_PATH" record concepts SCOPE/DR-NNNN
```

---

### Documents section (`FOCUS` = `all` or `documents`)

```bash
"$KB" --db "$DB_PATH" document list
```

One line per ingested narrative/article: id, format, title, path. For a
specific document's structure (sections, summary status per section,
linked concepts), drill in with:

```bash
"$KB" --db "$DB_PATH" document show DOCUMENT_ID
```

---

### Review queue section (`FOCUS` = `all` or `review-queue`)

```bash
"$KB" --db "$DB_PATH" document review list
```

Defaults to sections that are `unsummarized` or `drafted` — the actual
triage queue, not every section. Each row carries the signals a human uses
to decide whether a section is small/simple enough to hand to a model or
needs a person to write it directly: word count (`size`), how many known
concepts it already mentions (`density`), and — once drafted — a
self-reported `confidence`. A `[STALE]` marker means the underlying text
changed since this summary was written (from a re-ingest); flag those
first, since their existing summary may no longer match the content.

If this section is empty, say so plainly — an empty review queue is a
normal, good state, not something to pad the report with.

---

### Full-text search (`FOCUS` = `search:<term>`)

```bash
"$KB" --db "$DB_PATH" search "TERM"
```

Ranked results across projects, observations, concepts, records, and
*reviewed* document summaries — a `drafted` or `unsummarized` document
section is findable by concept tag (see `review-queue` above) but does not
appear in full-text search results, since it isn't yet trusted content.
Reports an error if FTS5 wasn't compiled into the SQLite driver — that's an
environment issue, not a data issue; note it and fall back to the
`observations`/`concepts` sections' plain listings instead.

---

### Single-project detail (`FOCUS` = `<project-name>`)

```bash
"$KB" --db "$DB_PATH" project show "FOCUS"
```

If not found, list all project names (`kb project list`) and ask the user
to pick one. Then:

```bash
"$KB" --db "$DB_PATH" project concepts "FOCUS"
"$KB" --db "$DB_PATH" observation list --project "FOCUS"
"$KB" --db "$DB_PATH" record list "FOCUS"
"$KB" --db "$DB_PATH" document list --project "FOCUS"
```

Or, for a single assembled Markdown view of the concepts/observations
part instead of those two separate calls (`format` predates records and
documents, so it doesn't include them — still run the `record list`/
`document list` calls above alongside it):

```bash
"$KB" --db "$DB_PATH" format --project "FOCUS"
```

---

## Step 2 — Deliver the report

Structure the output as:

```
## Knowledge Base Report  (<date>)

### Projects  (N total)

**Active (N)**
  …

**Concept (N)**
  …

**Paused / Concluded (N)**
  …

---

### Recent Observations  (last 20)

| id | project | kind | created_at | body_preview |
|----|---------|------|------------|--------------|
…

---

### Concepts  (N total)

| name | description | projects_linked | observations_linked |
|------|-------------|------------------|----------------------|
…

---

### Decision Records  (N total, N proposed / N accepted)

| id | date | status | kind | trigger | title |
|----|------|--------|------|---------|-------|
…

---

### Documents  (N total)

| id | format | title | path |
|----|--------|-------|------|
…

---

### Review Queue  (N sections need attention: N unsummarized, N drafted, N stale)

| id | status | level | document / heading | size | density | confidence | stale |
|----|--------|-------|---------------------|------|---------|------------|-------|
…

---

### Sync Check
<in sync, or the differences and the recommendation `kb check-db` printed>

### Index Check

N corpora, all up to date.   ← or list only the stale ones

---

### Candidate Concepts  (top 10 of N — suggestions, not findings)

| term (+variants) | score | occurrences | items |
|------------------|-------|-------------|-------|
…

Near-existing spellings: N pairs (possible `document fuzzy-tag` targets; noisy)
```

For a single-project report use:

```
## Project: <name>

Status: … | Description: …
Concepts: …

### Observations  (N total)

[chronological list, or the kb format output verbatim]

### Records  (N total)

[kb record list FOCUS output]

### Documents  (N total)

[kb document list --project output]
```

---

## Step 3 — Offer follow-up actions

After the report, offer:

> Would you like to:
> - Add an observation, project, record, or document? → run `update-knowledge-base`
> - Correct an observation, or rename a project/concept? → run `update-knowledge-base`
> - See the full text of a specific observation? `kb observation show ID`
> - See a document's sections and concepts? `kb document show ID`
> - See a record's concept tags? `kb record concepts SCOPE/DR-NNNN`
> - Draft or promote a document's summary? → run `update-knowledge-base`
> - Add one of the candidate concepts, or tag a project's documents with it? → run `update-knowledge-base`
> - Footnote near-miss spellings of known concepts (`fuzzy-tag`), or fill in a document's frontmatter? → run `update-knowledge-base`
> - See what is linked to a concept? `kb concept show NAME`; or what the knowledge base holds about some text? `kb concept recall TEXT` (kb >= v0.0.15)
> - Footnote near-miss spellings in decision records? `record-fuzzy-tag` → run `update-knowledge-base`
> - Compare the database with its JSONL dump? Ask me to review `sync-check`.
> - Regenerate a stale `index.md`? `kb index <root> --all` (without `--check`)
> - Drill into a specific project? Ask me to review `<name>`.
> - Search for something specific? Ask me to search `<term>`.
> - See what needs review? Ask me to review `review-queue`.

---

## Error handling

| Condition                  | Action                                                            |
|-------------------------------|------------------------------------------------------------------|
| `kb` not found anywhere      | Report; suggest `cd ~/Laboratory/knowledge && make build`         |
| `DB_PATH` does not exist     | Report; suggest running `setup-knowledge-base` first               |
| Database is empty            | Report "No records yet" for each section; offer to add data       |
| Project name not found (`project show` exits 1) | List all project names and ask the user to choose. Any other exit (65, 66, ...) is not "not found": report the stderr message |
| FTS5 not available           | Report the limitation; fall back to `concept list`/raw observation query instead of `search` |
| `search` finds nothing       | `kb search` exits 1 on no match — that is "no results", not a failure; report it plainly. Exit 2 or more is a failure (a flag-shaped first word, a broken database): report the message, not "no results" |
| `check-db` exits 1 | Expected when the database and dump differ — that is the finding. Print the report and recommendation; do not import or export without the user's go-ahead |
| `check-db` exits 66 | No JSONL dump beside the database. Report that there is nothing to compare; it is not a failure |
| `index --all --check` exits 1 | Expected when a corpus is stale — that is the finding. Name the stale corpora; don't retry, and don't drop `--check` to "fix" it without asking |
| `index --all --check` exits 2 or more | The check itself failed (65 a malformed record, 66 no such root, 77 unreadable): not a finding. Report the message and the corpus it names. Never pass `--db` to `index`; it refuses one |
| `concept suggest` returns nothing | The corpus is too small or every distinctive term is already a concept. Say so; it is a normal state, not an error |

### Exit codes

`kb` follows the workspace exit-code convention (workspace DR-0003; `kb` v0.0.14 and later). Branch on the number, not only on "non-zero":

| Exit | Meaning | What to do |
|------|---------|------------|
| 0 | Success (a listing that matches nothing is still 0) | Continue |
| 1 | A normal negative answer: no such project, concept, record or observation; `search` found nothing; a stale index; a database and dump that differ (`check-db`); the current state forbids the operation (a concept or source still linked, a rename onto a taken name) | Say so plainly and offer the next step; it is not a fault |
| 2 | The command line was wrong (bad flag, missing or surplus argument, a value outside its vocabulary, a blank name) | Fix the arguments; never retry the same command |
| 65 | Content is wrong: a malformed record, JSONL or document, a file that is not a knowledge base, a merge collision, an ingest under the wrong workspace root | Show the file and message; ask the user |
| 66 | A named input or the workspace is missing (`no agents/knowledge.db here`, no such file or directory) | Check the path; run `setup-knowledge-base` if there is no workspace |
| 69, 75 | A service was unreachable, or the database is locked | Temporary: retry later |
| 73, 74, 77 | An output cannot be created, an I/O error, permission denied | Environment: report it; do not retry blindly |
| 70 | An internal error: a bug in `kb` | Report the stderr message verbatim |

A bulk command (`ingest`, `index --all`, `source check-retractions`) does everything it
can, prints its counts, then exits with the class of the first failure: read the summary
it printed as well as the code. Earlier `kb` exited 1 for every failure; against one of
those, treat 1 as "failed, read the message" and never as "not found" on its own.
