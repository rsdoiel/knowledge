---
name: update-knowledge-base
description: |
  Adds or updates records in agents/knowledge.db via the kb CLI (from
  github.com/rsdoiel/knowledge). Supports adding projects, observations,
  concepts, sources, decision records, and narrative documents; correcting
  an observation by superseding it; renaming a project or concept; drafting
  and promoting a document's summary; suggesting candidate new concepts
  from the corpus; inserting wikilink tags (exact and near-miss) into
  documents and decision records; showing or recalling what a concept links to; proposing a document's frontmatter; and linking
  concepts/sources to projects or observations. Use when the user asks to
  "add a project", "record an observation", "add a concept", "add a
  source", "add a decision record", "ingest a document/story/article",
  "correct an observation", "rename a project", "rename a concept", "draft
  a summary", "suggest concepts", "tag a document", "fuzzy tag a
  document", "fuzzy tag the records", "show a concept", "recall what we know about", "fix a document's frontmatter", or "link a
  concept/source to a project or observation".
compatibility: claude-code, harvey
metadata:
  author: R. S. Doiel
  version: 0.9.0
  kb_version: 0.0.24
variables:
  DB_PATH:
    type: string
    description: Path to the SQLite3 database file. Defaults to agents/knowledge.db.
    example: "agents/knowledge.db"
  ACTION:
    type: string
    description: "What to add or change: project | observation | observation-update | concept | concept-suggest | concept-show | concept-recall | source | record | record-fuzzy-tag | document | document-draft | document-promote | document-tag | document-fuzzy-tag | document-frontmatter | project-rename | concept-rename | concept-delete | link-project-concept | link-observation-concept | link-observation-source | project-delete | observation-delete | document-delete | record-delete | unlink"
    example: "observation"
---

This skill adds or updates records in `knowledge.db` by driving the `kb`
CLI (`kb VERB ARGS`, matching the git/go command model) — never raw
`sqlite3` INSERTs. `kb` assigns each row's `uuid`/`origin_host` and
maintains the FTS5 search index automatically; a hand-written `INSERT`
would silently skip both (real bug in the previous generation of this
skill: it also relied on `SELECT ... WHERE id = last_insert_rowid()` in a
*separate* `sqlite3` invocation, which never sees the first process's
autoincrement value — always confirm via `--json`'s own output, never a
follow-up `last_insert_rowid()` query).

Run `setup-knowledge-base` first if the database does not yet exist (though
`kb` creates it automatically on first use regardless).

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
- `record set-status`, `record supersede` and `record delete` **refuse a bare id
  with no scope** (exit 2). Always pass the qualified reference, or run inside the
  project. `record new` takes its project from the current directory when
  `--project` is left out; pass `--project` anyway so the command says what it does.
- A project cannot be named `workspace` (exit 2).
- **v0.0.18:** `record set-status` follows a table of moves (`proposed` to
  `accepted`, `rejected`, `cancelled` or `superseded`; `accepted` to `cancelled` or
  `superseded`; `rejected` and `cancelled` back to `proposed`; `superseded` is final).
  A move outside it, or setting the status a record already has, is exit 1 and names
  the moves; nothing is written. `record set-status REF accepted` **exits 2 without a
  terminal**, and no flag turns that off: a model cannot accept a record. Do not try
  to; name the record and tell the user to run `! kb record set-status REF`, which
  shows it and asks. `cancelled` now means pursued or explored and then abandoned,
  and need not have been accepted; `rejected` means considered and not pursued.
- **v0.0.19:** `document review promote` exits 2 without a terminal in the same
  way: a model may `draft` a summary and a person promotes it. `kb ingest` prints
  `status:` lines for a status that arrives through a file (`proposed -> accepted
  (edited in file)`, `arrived accepted`); report them to the user and never edit a
  record's status to make them go away. `kb -json verbs` prints every verb, subverb
  and flag with what it does to the database (`read`, `additive`, `changing`,
  `removing`, ...): use it to check that a form exists instead of guessing. `kb`
  with no verb, a bare group such as `kb record`, and `kb -i ...` open an
  interface that needs a terminal; a model has none and gets a usage error (exit 2),
  so always give the full command.
- **v0.0.23 (nothing here needs it; the minimum stays 0.0.19):** `kb ingest` on a
  project directory (`agents/projects/NAME`) skips `notes/`, `design/`, `plans/`,
  `feature_requests/` and `hand-off/`, and a file named `YYYY-MM-DD-...md` is a note,
  never a record, so a note needs no frontmatter. `kb document ingest` offers
  frontmatter for a file that has none only on a terminal, so a model running it
  never sees the prompt; pass `--no-prompt` anyway when a script may be given a
  terminal. A search hit carries a record's reference, status and the matching
  passage, and `kb stats` counts rows, links and checks without SQL.

---

## Variables

| Variable  | Type   | Description                                    | Default                | Prompted? |
|-----------|--------|--------------------------------------------------|--------------------------|-----------|
| `DB_PATH` | string | Path to the SQLite3 database file.               | `agents/knowledge.db`    | No        |
| `ACTION`  | string | The type of record to add (see table below).     | —                        | Yes       |

### ACTION values

| Value                       | What it does                                                     |
|-------------------------------|---------------------------------------------------------------------|
| `project`                    | Adds a row to `projects` (`kb project add`)                        |
| `observation`                | Adds a row to `observations` for a named project (`kb observation add`) |
| `observation-update`         | Corrects an observation by superseding it, never in place (`kb observation update`) |
| `concept`                    | Adds a row to `concepts` (`kb concept add`)                        |
| `concept-suggest`            | Proposes candidate *new* concepts from the corpus, read-only (`kb concept suggest`) |
| `concept-show`               | Shows a concept's description and what links to it, read-only (`kb concept show`, kb >= v0.0.15) |
| `concept-recall`             | Lists what the knowledge base holds about the concepts named in some text, read-only (`kb concept recall`, kb >= v0.0.15) |
| `source`                     | Adds a row to `sources` (`kb source add`)                          |
| `record`                     | Scaffolds and ingests a decision record (`kb record new` + `kb ingest`) |
| `record-fuzzy-tag`           | Reports near-miss spellings of known concepts in decision records, and with `--write` adds them to `tags:` (`kb record fuzzy-tag`, kb >= v0.0.15) |
| `document`                   | Ingests a narrative/article — Markdown, Fountain, or plain text (`kb document ingest`) |
| `document-draft`             | Writes a gist or section summary for an ingested document (`kb document draft`) |
| `document-promote`           | Promotes a drafted summary to reviewed, making it searchable (`kb document review promote`) |
| `document-tag`               | Inserts explicit `[[Name]]` wikilinks into a project's documents (`kb document tag`) |
| `document-fuzzy-tag`         | Footnotes near-miss spellings of known concepts in a project's documents (`kb document fuzzy-tag`) |
| `document-frontmatter`       | Proposes, then writes only what's accepted, one document's `title`/`author`/dates/`keywords` (`kb document frontmatter`) |
| `project-rename`             | Renames a project, rewriting its records' frontmatter (`kb project rename`) |
| `concept-rename`             | Renames a concept (`kb concept rename`)                            |
| `concept-delete`             | Deletes a concept and its links, refusing a linked one without `--force` (`kb concept delete`, kb >= v0.0.12) |
| `link-project-concept`       | Links a project to a concept (`kb link project`)                   |
| `link-observation-concept`   | Links an observation to a concept (`kb link observation`)          |
| `link-observation-source`    | Links an observation to a cited source (`kb source link`)          |
| `project-delete`             | Deletes a stray empty project (`kb project delete`, kb >= v0.0.14) |
| `observation-delete`         | Deletes an observation, refusing a linked one without `--force` (`kb observation delete`, kb >= v0.0.14) |
| `document-delete`            | Deletes a document, refusing one with a reviewed summary without `--force` (`kb document delete`, kb >= v0.0.14) |
| `record-delete`              | Drops the row of a record whose file is gone (`kb record delete`, kb >= v0.0.14) |
| `unlink`                     | Removes one project-concept, observation-concept or observation-source link (`kb unlink`, kb >= v0.0.14) |

### A note on concept tagging

A record's body and a document's section bodies don't need `link-*` calls
to get concept tags at all: writing `[[Name]]` inline, or listing names in
a record's `tags` frontmatter / a document's `keywords` frontmatter,
resolves to a concept and links it automatically at ingest time
(case-insensitive — capitalization from sentence position doesn't create a
duplicate concept). The `link-*` actions below exist for **projects** and
**observations**, which have no ingest step and no wikilink scanning of
their own.

**A wikilink inside code is an example, not a tag (kb >= v0.0.12, DR-0037).**
`[[Name]]` written inside a code span or a fenced block does not
mint or link a concept, so a document or record can show wikilink syntax without
polluting the vocabulary. In kb v0.0.11 it did, and that is where junk concepts
such as `...`, `recall: ...` and `tool: ...` came from; a database ingested with
v0.0.11 may still hold some. List them with `concept list` and, with the user's go-ahead,
remove each with the `concept-delete` action (dry run first). The same release lets a concept whose name
begins or ends in punctuation (`C++`, `F#`, `.NET`) match a mention, which it
could never do before.

---

## Step 0 — Resolve DB_PATH and locate the `kb` binary

```bash
DB_PATH="${DB_PATH:-agents/knowledge.db}"
KB="$(command -v kb || echo "$HOME/Laboratory/knowledge/bin/kb")"
```

All commands below run as `"$KB" --db "$DB_PATH" ...`. Add `--json` to any
of them for a structured, script-parseable confirmation instead of the
human-readable text line — prefer `--json` when you need the new row's id
back (e.g. to immediately link it to a concept).

---

## Step 1 — Determine ACTION

If `ACTION` is not set, ask the user:

> What would you like to add or change?
> 1. project
> 2. observation
> 3. correct an observation (supersede it)
> 4. concept
> 5. suggest candidate concepts from the corpus (read-only)
> 6. source
> 7. decision record
> 8. document (story, article, screenplay)
> 9. draft a document's summary
> 10. promote a document's summary (make it searchable)
> 11. tag a project's documents with `[[wikilinks]]`
> 12. fuzzy-tag a project's documents (typos, plurals, tense variants)
> 13. propose and write a document's frontmatter
> 14. rename a project
> 15. rename a concept
> 16. delete a concept (kb >= v0.0.12)
> 17. link-project-concept
> 18. link-observation-concept
> 19. link-observation-source
> 20. show a concept and what links to it (read-only, kb >= v0.0.15)
> 21. recall what the knowledge base holds about the concepts in some text (read-only, kb >= v0.0.15)
> 22. fuzzy-tag a project's decision records (kb >= v0.0.15)

---

## Step 2 — Gather fields and run the matching `kb` command

### ACTION = project

Prompt for:
- `NAME` — unique short identifier (filesystem-safe, no spaces)
- `DESCRIPTION` — one-sentence description (optional)
- `STATUS` — one of: `concept`, `active`, `paused`, `concluded` (default: `active`)

```bash
"$KB" --db "$DB_PATH" project add --status "STATUS" "NAME" "DESCRIPTION"
```

(Omit `--status` to accept the default `active`; `--status` goes before
`NAME`.) A name that already exists is refused: `kb` exits 1 with
`project "NAME" already exists (id=N)` and changes nothing (needs `kb` >=
0.0.15). Use `project set-status` and `project set-description` to change an
existing project.

---

### ACTION = observation

First show the project list so the user can pick one:

```bash
"$KB" --db "$DB_PATH" project list
```

Prompt for:
- `PROJECT_NAME` — from the list above (not a numeric id — observations
  are added by project *name*)
- `KIND` — one of: `note`, `finding`, `decision`, `question`, `hypothesis` (default: `note`)
- `BODY` — the observation text (may be multi-line; collect until the user signals done)

```bash
"$KB" --db "$DB_PATH" observation add --project "PROJECT_NAME" "KIND" "BODY"
```

---

### ACTION = observation-update

An observation is a timestamped note and is never edited in place (DR-0023).
"Updating" one means superseding it: `kb` inserts a *new* observation with
the corrected text, inheriting the original's project and kind, and links
the two with a `supersedes` edge. The old observation's body is untouched —
this is how the correction's history is kept, not a separate log.

Prompt for:
- `OBSERVATION_ID` — the numeric id of the observation being corrected (from
  `kb observation list --project NAME` or `kb observation show ID`)
- `BODY` — the corrected text (may be multi-line)

```bash
"$KB" --db "$DB_PATH" observation update OBSERVATION_ID "BODY"
```

The command reports both ids (the new observation's, and which one it
supersedes). `kb observation show OBSERVATION_ID` afterward shows
`superseded_by` on the old one; `kb observation show` on the new id shows
`supersedes` pointing back.

---

### ACTION = concept

Prompt for:
- `CONCEPT_NAME` — unique name for the concept
- `DESCRIPTION` — one-sentence description (optional)
- Optionally, if this concept represents a scholarly entity (a paper, person, institution, funder): `IDENTIFIER_TYPE` (e.g. `doi`, `orcid`, `ror`, `fundref`) and `IDENTIFIER_VALUE`

```bash
"$KB" --db "$DB_PATH" concept add "CONCEPT_NAME" "DESCRIPTION"
# or, with a scholarly identifier:
"$KB" --db "$DB_PATH" concept add --identifier-type "IDENTIFIER_TYPE" --identifier-value "IDENTIFIER_VALUE" "CONCEPT_NAME" "DESCRIPTION"
```

Re-running with a name that already exists updates its description (and
identifier, if one wasn't already set) in place rather than duplicating.

---

### ACTION = concept-show

**Requires kb >= v0.0.15** (DR-0051). Read-only: one concept's description and
identifier, and for each kind (projects, observations, records, document
sections) the full count and up to `--limit` of the newest items.

```bash
"$KB" --db "$DB_PATH" concept show "CONCEPT_NAME"
"$KB" --db "$DB_PATH" concept show --limit 0 "CONCEPT_NAME"   # counts only
```

The default limit is 10. The name must match exactly, as `concept delete`
does; a miss exits 1 and offers a case variant, so offer that to the user
rather than guessing. A name that looks like a flag goes after `--`. Use it
before `concept-delete` or `concept-rename` to show what would be affected.

---

### ACTION = concept-recall

**Requires kb >= v0.0.15** (DR-0051). Read-only: finds the concepts named in
some text and lists the observations, records and document sections linked to
them, ranked by how many concepts matched and then by recency, each with the
concepts it matched and an excerpt. This is what Harvey's own recall calls.

```bash
"$KB" --db "$DB_PATH" concept recall "TEXT"
echo "TEXT" | "$KB" --db "$DB_PATH" concept recall -
"$KB" --db "$DB_PATH" concept recall --concept "NAME,OTHER" --project "PROJECT_NAME" --limit 10
```

`--concept` names concepts directly and is combined with any found in the
text. It writes nothing and creates no concept. Text that matches no concept,
or an unknown project, exits 1: say so plainly. No text and no `--concept`
exits 2.

---

### ACTION = concept-suggest

Read-only: proposes candidate *new* concepts by scanning every record body
and document section body and ranking terms by corpus-wide distinctiveness
— mentioned several times, but confined to relatively few items rather than
spread evenly across nearly all of them. It is the half of concept
discovery that tagging cannot reach: `[[wikilinks]]` and density-linking
only ever match concepts that **already exist**, and can never propose one.

```bash
"$KB" --db "$DB_PATH" concept suggest --limit 20
"$KB" --db "$DB_PATH" concept suggest --project "PROJECT_NAME"
```

Optional `--project` scopes the scan; `--limit` caps the list (default 20).
Code spans and fenced blocks are excluded, an existing concept's name is
never suggested again, and a bare record reference (`dr-0013`, `adr-0004`)
is filtered out rather than scored.

**It never writes to the database**, and this is the point rather than a
limitation — a suggestion becomes a concept only when a human runs
`kb concept add`. Expect roughly half the list to be noise: run live
against a real corpus it mixed genuinely useful terms with generic nouns
like `row`, `name` and `table` (DR-0028). Present the ranked list and let
the user pick; do not loop `concept add` over the output.

**Spelling variants are clustered before scoring (kb >= v0.0.11, DR-0034).**
A line like `rename (+renamed, renaming, renames)` is one candidate: adding
it as a concept means adding the head term, and `[[rename]]` then matches
the whole-word form only. A candidate that is a near-miss of an
*already-known* concept is not listed as a candidate at all; it appears in
a trailing `near-existing (excluded from candidates):` block instead, which
is `document-fuzzy-tag`'s job, not a suggestion. That block is **noisy**:
run against the real corpus it included plausible pairs (`collision ~
collisions`, `observation ~ observations`) beside false ones (`nested ~
testing`, `around ~ grounding`, both from suffix-stemming). Present it as
"possible near-misses to check", never as findings. *Fixed in kb v0.0.12 (DR-0036):* terms must now share a first letter, which removes the three false pairs above; a `kb` older than that still shows them.

`--json` is `{"candidates": [...], "near_existing": [...]}`, with an
optional `variants` list on a candidate. Before v0.0.11 it was a bare
array, so anything parsing the old shape breaks.

---

### ACTION = document-tag

A pure file operation over every already-ingested document in a project: it
inserts an explicit `[[Name]]` wikilink at the first safe occurrence of
each eligible concept. It **never writes to the database** — the next
`kb document ingest` of the changed file links it through the ordinary
wikilink path.

Prompt for `PROJECT_NAME`, and optionally a comma-separated `CONCEPTS`
list. Always preview first — this edits the user's prose:

```bash
"$KB" --db "$DB_PATH" document tag --project "PROJECT_NAME" --dry-run
# on a yes:
"$KB" --db "$DB_PATH" document tag --project "PROJECT_NAME"
```

With no `--concept`, eligible means mentioned more than once in the file
outside code spans — the same threshold density-linking already applies.
`--concept "NAME,OTHER"` forces exactly those names regardless of
occurrence count, but each must already be a known concept, checked before
any file is touched.

What it will not write into: YAML frontmatter, fenced code blocks, inline
code spans, anything already inside `[[...]]`, and the document's own first
H1 heading — a concept named in a document's title is mechanically a match
but stylistically wrong to wrap, found live while smoke-testing (DR-0029).
A concept already wikilinked anywhere in a file is left alone, so a second
run is a no-op. Writes across the project's document set are
both-or-neither.

The natural pairing is `concept-suggest` → `kb concept add` → `document-tag`:
discover a candidate, have the human confirm it, then make the judgment
durable in the source file instead of leaving it implicit in the database.

---

### ACTION = document-fuzzy-tag

Same shape as `document-tag`, for what exact matching cannot catch: a typo,
plural or simple tense variant of a known concept's name (Levenshtein
distance, **not** paraphrase). It edits the user's files but never writes
to the database; the next `kb document ingest` picks the link up.

It does not bracket-wrap the near-miss, because that would mint a
duplicate concept from the misspelled form. It inserts a `[^n]` footnote
marker at the near-miss and a footnote definition carrying the canonical
`[[Concept]]`, so the prose is untouched. Always preview first:

```bash
"$KB" --db "$DB_PATH" document fuzzy-tag --project "PROJECT_NAME" --dry-run
# on a yes:
"$KB" --db "$DB_PATH" document fuzzy-tag --project "PROJECT_NAME"
```

`--concept "NAME,OTHER"` bypasses the distance threshold for exactly those
names; each must be a known concept, checked before any file is touched. A
concept that already has an exact match, a footnote or a wikilink anywhere
in the file is skipped, so a second run is a no-op. Run `document-tag`
first: fuzzy-tag only handles what exact tagging left behind.

Show the `--dry-run` output to the user before writing, and expect to
reject some of it. Dry-run against the real corpus (kb v0.0.11) proposed
good matches (`wikilink ~ wikilinks`, `merged ~ merge`) beside plainly wrong
ones, mostly against short concept names: `its ~ fts`, `drafting ~ drift`,
`UI ~ tui`. Do not run it unfiltered; use `--concept` to name exactly the
concepts the user approves. *Fixed in kb v0.0.12 (DR-0036):*
a concept shorter than 6 letters is no longer fuzzy-matched without
`--concept`, and a distance-2 match needs a 6-letter shared prefix. On the
same corpus that took 614 proposals to 224 and removed every false pair
above, at the cost of short-concept plurals (`merge ~ merges`). A v0.0.11
binary still shows the noise. After a real run, re-ingest
the changed files (`document` action) so the links reach the database.

---

### ACTION = document-frontmatter

Propose-then-accept for **one** file (not scoped by `--project`):
`title`, `author`, `dateCreated`, `dateModified` and `keywords`. A bare
invocation only reports; nothing is written without an explicit selection.

Prompt for `PATH`. Always start with the report:

```bash
"$KB" --db "$DB_PATH" document frontmatter "PATH"
```

Show it to the user, then write only the fields they pick:

```bash
"$KB" --db "$DB_PATH" document frontmatter "PATH" --accept title,author,dateCreated,dateModified
"$KB" --db "$DB_PATH" document frontmatter "PATH" --accept-keywords "NAME,OTHER"
"$KB" --db "$DB_PATH" document frontmatter "PATH" --set "author=Jane Doe"
```

Rules worth stating to the user: `title`/`author`/`dateCreated` are
**absent-only**, never overwritten once present; `dateModified` is the one
field always refreshed on an accepted run. `author` comes from the
document's own byline, then git's first-commit author, then `git config
user.name`, and when those disagree the report shows all of them, so ask
rather than assuming the first. `--set FIELD=VALUE` (repeatable) is a
human's explicit assertion that bypasses both signal detection and the
absent-only rule, and `--set FIELD=` writes an explicit empty value.
Accepting a *new candidate* keyword creates that concept first, and
`--accept-keywords` (in v0.0.11; fixed in v0.0.12, DR-0036) did not check that a name was actually proposed (see
Error handling), so a typo mints a real concept. Only the
frontmatter block changes; the body is never touched. It shells out to
`git` for provenance and falls back to filesystem times when the file is
untracked. Candidate keywords are a mechanical score (a probe on a
design document proposed `dec` and `items` beside `miner` and `curation`),
so present them for picking, never accept the whole list.

---

### ACTION = source

Prompt for:
- `TITLE` — human-readable title or file path
- Optionally: `--doi`, `--url`, `--authors`, `--published DATE`, `--publisher`, `--rights`, `--version`

```bash
"$KB" --db "$DB_PATH" source add "TITLE" --doi "DOI_VALUE"
```

---

### ACTION = record

A decision record is authored as a file, then indexed — there is no
single "add" call the way there is for a project or observation.

Prompt for:
- `TITLE` — one-line summary of the decision
- `TRIGGER` — one of: `design`, `plan-review`, `implementation`,
  `live-test`, `release-review`, `request` (what prompted this record)
- `PROJECT_NAME` (or `--workspace` for a workspace-tier record spanning
  repos)
- `KIND` — one of: `decision`, `correction`, `refinement` (default:
  `decision`)

```bash
"$KB" --db "$DB_PATH" record new --title "TITLE" --trigger "TRIGGER" --project "PROJECT_NAME" --kind "KIND"
```

This scaffolds the file (status `proposed`) and prints its path. Tell the
user to fill in the body sections, then ingest it:

```bash
"$KB" --db "$DB_PATH" ingest PATH_TO_PROJECTS_DECISIONS_DIR
```

A record is written `proposed` and stays that way until its author
explicitly promotes it — never do this on the user's behalf; a model may write a
record but may not accept one. From kb v0.0.18 the tool enforces this (accepting
exits 2 without a terminal), so do not try to work around it: name the record and
tell the user to run `kb record set-status PROJECT/DR-NNNN` themselves (in Claude
Code, `! kb record set-status PROJECT/DR-NNNN`), which shows the record and asks.

---

### ACTION = record-fuzzy-tag

**Requires kb >= v0.0.15** (DR-0052). The decision-record counterpart of
`document-fuzzy-tag`: it finds typos, plurals and simple tense variants of a
known concept's name in a project's record bodies. It reports, per record, the
variants, their count, the exact mentions that link nothing, and the `tags:`
line that would link the concept. **By default it writes nothing.**

```bash
"$KB" --db "$DB_PATH" record fuzzy-tag --project "PROJECT_NAME" --dry-run
"$KB" --db "$DB_PATH" record fuzzy-tag --project "PROJECT_NAME" --concept "NAME,OTHER"
# on a yes:
"$KB" --db "$DB_PATH" record fuzzy-tag --project "PROJECT_NAME" --concept "NAME,OTHER" --write
"$KB" --db "$DB_PATH" ingest PATH_TO_PROJECTS_DECISIONS_DIR
```

`--write` adds the concept to the `tags:` of **`proposed` records only**,
changes nothing else in the file, never touches an accepted record (a model
may not edit what its author has accepted), and is both-or-neither across the
run. `kb ingest` then links the new tags; it is not automatic. Only the body
is searched, and a concept is skipped for a record that already links it.

Without `--concept` the length and distance rules of `document-fuzzy-tag`
apply. With it, the named concept is widened to tokens that contain its name
(`detoast`, `toasting`), and a smoke test on real records showed that is still
noisy for a short concept: always show the report and let the user pick the
concepts. `--root DIR` sets the workspace root the record paths are relative to.

---

### ACTION = document

Prompt for:
- `PATH` — path to the story, article, or screenplay file (`.md`,
  `.fountain`/`.spmd`, or plain text)
- `PROJECT_NAME`
- Optionally: `TITLE` (overrides frontmatter/title-page), `FORMAT`
  (overrides extension-based detection)

```bash
"$KB" --db "$DB_PATH" document ingest "PATH" --project "PROJECT_NAME"
```

Frontmatter (Markdown: `title`/`description`/`pubDate`/`author`/`keywords`)
or a Fountain title page (`Title:`/`Author:`) is picked up automatically —
no need to re-supply `TITLE` unless overriding it. A Markdown `description`
field seeds an initial gist summary automatically, already `drafted` (see
`document-promote` below to make it searchable). PDF is not yet supported
and reports an explicit error, not silent mishandling.

Re-running on a changed file is safe: unchanged sections are left alone,
a changed section is flagged `summary_stale` rather than losing its
summary, and a removed heading is reported, never deleted.

---

### ACTION = document-draft

Show the document's sections first so the user can pick one:

```bash
"$KB" --db "$DB_PATH" document show DOCUMENT_ID
```

Prompt for:
- `SECTION_ID` (from the list above — note this is the section's own id,
  not the document's)
- `BODY` — the summary text
- `WHO` — who wrote it: `human`, or a model identifier. `kb` never calls a
  model itself; if the user wants a model-drafted summary, that generation
  happens outside `kb` and this action just records the result
- Optionally `CONFIDENCE` — a number in `[0, 1]`

```bash
"$KB" --db "$DB_PATH" document draft SECTION_ID "BODY" --by "WHO" --confidence "CONFIDENCE"
```

---

### ACTION = document-promote

Promotion is what makes a summary searchable and returnable as trustworthy
content — an explicit human action, never automatic. From kb v0.0.19 the tool
enforces this: `document review promote` exits 2 unless standard input and output
are a terminal, with no flag or environment variable to turn that off. Do not try
to work around it. Name the section and tell the user to run the command
themselves (in Claude Code, `! kb document review promote SECTION_ID`); a person
at a terminal, such as the compiled script run by hand, can run it directly.

```bash
kb document review promote SECTION_ID      # the user runs this
```

Fails if the section hasn't been drafted yet (run `document-draft` first).

---

### ACTION = project-rename

Prompt for `OLD` and `NEW` names. Show the project list first if the user
isn't sure of the exact current name:

```bash
"$KB" --db "$DB_PATH" project list
"$KB" --db "$DB_PATH" project rename --dry-run "OLD" "NEW"
# on a yes:
"$KB" --db "$DB_PATH" project rename "OLD" "NEW"
```

Note the argument order: `project rename [--root PATH] [--dry-run] OLD NEW`.
This verb's own flags come **before** the two names. Verified against
v0.0.10: `project rename OLD NEW --dry-run` prints the usage line and exits
1 without renaming, so a misordered dry run fails safe rather than renaming
for real — but it also means a script that ignores the exit code will
report a preview that never ran.

Refuses only if `NEW` already names another project. A project that owns
decision records renames fine as of kb v0.0.10 (DR-0026, which supersedes
DR-0024's outright refusal) — `kb` first rewrites every owned record's
`project:` frontmatter to `NEW`, both-or-neither, rolling back every file
already written if any write fails, then renames the project row. No
record's database row is touched: `records.project_id` is a stable foreign
key and only `projects.name` moves, so the next ordinary `kb ingest` of
that corpus sees a changed checksum against an unchanged identity and
updates in place.

Because this rewrites files, run `--dry-run` first and show the user which
files would change — it is the only preview, and the rename is not a
database-only operation the way `concept rename` is. `--root` sets the
workspace root those record paths are relative to; it defaults to a value
inferred from the database path, which is right whenever `DB_PATH` is the
workspace's own `agents/knowledge.db`.

Never hand-edit a corpus's frontmatter or reach for raw SQL as a
workaround — that route is what DR-0026 exists to replace, and doing it by
hand is what minted a phantom project and a duplicated record when it was
tried.

---

### ACTION = concept-rename

Prompt for `OLD` and `NEW` names:

```bash
"$KB" --db "$DB_PATH" concept rename "OLD" "NEW"
```

Refuses only if `NEW` already names another concept — a concept has no
corpus of files to desync, unlike a project.

---

### ACTION = concept-delete

**Requires kb >= v0.0.12** (DR-0039). Earlier binaries report
`unknown concept subcommand "delete"`. There was no way to remove a concept before it, so
never reach for raw `sqlite3` `DELETE`s: they skip the search index.

Prompt for `NAME`. It matches **exactly, including case**. Always look first:

```bash
"$KB" --db "$DB_PATH" concept delete "NAME" --dry-run
```

It reports how many projects, observations, records and document sections link to the
concept, and changes nothing. Then, with the user's go-ahead:

```bash
"$KB" --db "$DB_PATH" concept delete "NAME"           # only if it has no links
"$KB" --db "$DB_PATH" concept delete "NAME" --force   # unlinks it everywhere, then deletes
```

A concept that is still linked is **refused**, with the counts, and nothing changes; `--force`
removes the links (not the projects, records or documents themselves). A name that looks like
a flag (`---`, `-x`) goes after `--`: `concept delete --force -- "---"`. Say plainly what it
does not do, since the command prints both notes:

- **Files are not touched.** A record or document that still contains `[[NAME]]`, or lists it in
  `tags`/`keywords`, recreates the concept when that file is next ingested *after it changes*
  (an unchanged file is skipped) or when the database is rebuilt from files. Offer to find and
  remove the mention rather than deleting again and again.
- **Other machines are not told.** There is no tombstone: `kb merge`/`kb import` from a database
  that still has the concept brings it back, so delete it there as well.

The typical use is a junk concept minted from a documentation example (`...`, `recall: ...`);
since DR-0037 a wikilink inside a code span or fenced block no longer mints one. Never delete a
concept in bulk or without the dry run first.

---

### ACTION = project-delete | observation-delete | document-delete | record-delete

**Require kb >= v0.0.14** (DR-0050). Earlier binaries report `unknown ... subcommand "delete"`.
Until then there was no supported way to remove any of these, so never reach for raw `sqlite3`
`DELETE`s: they skip the search index. Every one follows `concept-delete`: look first with
`--dry-run`, then delete only with the user's go-ahead, and use `--force` only when the user
has read what it removes.

```bash
"$KB" --db "$DB_PATH" project delete     "NAME" --dry-run       # a stray empty project
"$KB" --db "$DB_PATH" observation delete ID     --dry-run
"$KB" --db "$DB_PATH" document delete    ID     --dry-run
"$KB" --db "$DB_PATH" record delete      SCOPE/DR-NNNN --dry-run   # e.g. clasm/DR-0149 or workspace/DR-0003
```

What each refuses (exit 1, with the counts, and nothing changes):

| Action | Refused while | `--force` |
|--------|---------------|-----------|
| `project-delete` | it owns any observation, record or document — **always**, `--force` or not | removes only its concept links |
| `observation-delete` | it has concept links, source links or supersessions | removes those links and deletes it |
| `document-delete` | any section's summary is `reviewed` (human-gated data) | deletes it anyway, losing the summary |
| `record-delete` | the record's **file still exists** | not accepted: delete the file first, or use `record set-status SCOPE/DR-NNNN cancelled` |

Say plainly what a delete does not do: **it is local.** There is no tombstone, so `kb merge` and
`kb import` from a database that still has the row bring it back; delete it there as well. A
document deleted here is recreated, without its reviewed summaries, by the next ingest of its
file. `kb` never deletes a decision-record file from disk; a record row is only dropped once the
file is gone, and an accepted record is retired with `superseded`, `rejected` or `cancelled`
rather than deleted. Never delete in bulk, and never without the dry run first.

---

### ACTION = unlink

**Requires kb >= v0.0.14** (DR-0050). The inverse of `link-project-concept`,
`link-observation-concept` and `link-observation-source`: it removes one link and nothing else.
Names match exactly, including case.

```bash
"$KB" --db "$DB_PATH" unlink project     "PROJECT" "CONCEPT"
"$KB" --db "$DB_PATH" unlink observation OBS_ID    "CONCEPT"
"$KB" --db "$DB_PATH" unlink source      OBS_ID    SOURCE_ID
```

Removing a link that does not exist exits 1 ("not linked"), it is not a silent success, so report
that plainly rather than retrying. To remove a concept from everything at once, use
`concept-delete`.

---

### ACTION = link-project-concept

Show projects and concepts so the user can pick:

```bash
"$KB" --db "$DB_PATH" project list
"$KB" --db "$DB_PATH" concept list
```

Prompt for `PROJECT_NAME` and `CONCEPT_NAME` (both by name, not id):

```bash
"$KB" --db "$DB_PATH" link project "PROJECT_NAME" "CONCEPT_NAME"
```

---

### ACTION = link-observation-concept

Show recent observations and concepts:

```bash
"$KB" --db "$DB_PATH" observation list --project "PROJECT_NAME"
"$KB" --db "$DB_PATH" concept list
```

Prompt for `OBSERVATION_ID` (numeric, from the list above) and `CONCEPT_NAME`:

```bash
"$KB" --db "$DB_PATH" link observation OBSERVATION_ID "CONCEPT_NAME"
```

---

### ACTION = link-observation-source

Prompt for `OBSERVATION_ID`, `SOURCE_ID` (from `kb source list`), and
optionally `RELATIONSHIP` (default `cited`):

```bash
"$KB" --db "$DB_PATH" source link OBSERVATION_ID SOURCE_ID --relationship "RELATIONSHIP"
```

---

## Step 3 — Confirm success

Every `kb ... add`/`link` command already prints a confirmation line (or,
with `--json`, a structured object including the new row's `id`) — there is
no separate "look it back up" step needed. If you do want to display the
full row afterward:

```bash
"$KB" --db "$DB_PATH" project show "NAME"
"$KB" --db "$DB_PATH" observation show OBSERVATION_ID
```

---

## Error handling

| Condition                        | Action                                                              |
|------------------------------------|-------------------------------------------------------------------------|
| `kb` not found anywhere            | Report; suggest `cd ~/Laboratory/knowledge && make build`             |
| `DB_PATH` does not exist           | Not an error — `kb` creates it automatically on first use             |
| Invalid `STATUS` or `KIND` value   | `kb` reports the valid values in its error message; prompt again      |
| Project/concept name not found     | Re-run the relevant `list` command and prompt for a valid name        |
| `document ingest` given a `.pdf`   | Reports "not yet supported" — no text-extraction path exists; not a bug to work around |
| `document-promote` on an unsummarized section | `kb` reports it needs drafting first; run `document-draft` before retrying |
| `project-rename` reports a file it could not rewrite | Nothing was renamed — the rewrite is both-or-neither and rolls back. Fix the file permission/path it names and re-run; do not hand-edit frontmatter or SQL as a workaround |
| `project-rename` rewrote nothing but the project has records | `--root` is probably pointing somewhere the corpus isn't. Pass `--root` explicitly rather than re-running and hoping |
| `concept-delete` refused: still linked | Expected. Show the counts, offer `--dry-run`, and use `--force` only with the user's go-ahead |
| `project-rename`/`concept-rename` refused (`NEW` already exists) | Report it; ask for a different `NEW` name or confirm merging under the existing one is out of scope here |
| `document-tag --concept` names an unknown concept | Checked before any file is touched, so nothing was written. Run `kb concept list` (or `concept suggest`) and correct the name |
| `document-fuzzy-tag` dry run proposes a wrong match | Don't write. Re-run with `--concept` naming only the correct concepts, or skip that document; the distance gate is mechanical |
| `document-frontmatter` reports several author signals | They disagree; show all and ask which is right, or use `--set author=...` |
| `document-frontmatter --accept-keywords` given a term the report never proposed | **v0.0.11 does not reject it** (fixed in v0.0.12, DR-0036: it now errors before writing anything, and `--dry-run` no longer creates concepts). Verified against v0.0.11: it writes the term into the file's `keywords` and creates a new concept for it in the database (a probe with a nonsense term did both). Copy names exactly from the report's candidate lists, and read the command back to the user before running it |
| `project` reports `already exists` (exit 1) | Expected from kb >= v0.0.15: nothing was written. Use `project set-status` or `project set-description`, or ask whether a different name was meant. Against an older `kb` the same command exited 0 and silently dropped the new status and description |
| `concept-show` / `concept-recall` exit 1 | A normal answer: no such concept (show; offer the case variant it prints), or no concept found in the text (recall). Not a fault |
| `record-fuzzy-tag` proposes common short words | Expected for a short concept name. Do not `--write`; re-run with `--concept` naming only the intended concepts |
| `concept-suggest` output looks like noise | Expected in part — it is a mechanical pass, roughly half signal. Present it for the user to pick from; never auto-`concept add` the list |
| Command exits non-zero             | Report the stderr message verbatim (never printed to stdout, so it's safe to surface directly even in `--json` scripting contexts), and act on the exit code (below) |

### Exit codes

`kb` follows the workspace exit-code convention (workspace DR-0003; `kb` v0.0.14 and later). Branch on the number, not only on "non-zero":

| Exit | Meaning | What to do |
|------|---------|------------|
| 0 | Success (a listing that matches nothing is still 0) | Continue |
| 1 | A normal negative answer: no such project, concept, record or observation; `search` found nothing; a stale index; a database and dump that differ (`check-db`); an existing project name given to `project add`; the current state forbids the operation (a concept or source still linked, a rename onto a taken name) | Say so plainly and offer the next step; it is not a fault |
| 2 | The command line was wrong (bad flag, missing or surplus argument, a value outside its vocabulary, a blank name) | Fix the arguments; never retry the same command |
| 65 | Content is wrong: a malformed record, JSONL or document, a file that is not a knowledge base, a merge collision, an ingest under the wrong workspace root; also (kb >= v0.0.15) a record file that does not parse in `record set-status`, `supersede`, `fuzzy-tag` or `project rename`, which exited 2 before | Show the file and message; ask the user |
| 66 | A named input or the workspace is missing (`no agents/knowledge.db here`, no such file or directory) | Check the path; run `setup-knowledge-base` if there is no workspace |
| 69, 75 | A service was unreachable, or the database is locked | Temporary: retry later |
| 73, 74, 77 | An output cannot be created, an I/O error, permission denied | Environment: report it; do not retry blindly |
| 70 | An internal error: a bug in `kb` | Report the stderr message verbatim |

A bulk command (`ingest`, `index --all`, `source check-retractions`) does everything it
can, prints its counts, then exits with the class of the first failure: read the summary
it printed as well as the code. Earlier `kb` exited 1 for every failure; against one of
those, treat 1 as "failed, read the message" and never as "not found" on its own.
