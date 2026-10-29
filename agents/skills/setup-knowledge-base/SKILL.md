---
name: setup-knowledge-base
description: |
  Ensures agents/knowledge.db exists and is on the current schema, via the
  github.com/rsdoiel/knowledge module's kb CLI (kb VERB ARGS, matching the
  git/go command model). Use when the user asks to "setup the knowledge
  base", "initialize the knowledge base", or "create knowledge.db".
  Safe to run multiple times — kb's Open() applies the schema and any
  pending lazy migrations idempotently; it never overwrites existing data.
compatibility: claude-code, harvey
metadata:
  author: R. S. Doiel
  version: 0.8.0
  kb_version: 0.0.29
variables:
  DB_PATH:
    type: string
    description: Path to the SQLite3 database file. Defaults to agents/knowledge.db.
    example: "agents/knowledge.db"
---

This skill ensures `knowledge.db` exists and is on the current schema. As of
the `github.com/rsdoiel/knowledge` module extraction (2026-07-27), there is
no hand-rolled schema SQL to run: `kb` (any verb) calls `knowledge.Open()`
internally, which creates the file if it doesn't exist and applies the
schema — and any lazy migration a pre-existing file needs — automatically,
every time it opens. "Setup" is therefore just: confirm `kb` is available,
then run one harmless read-only command against `DB_PATH` and report what's
there.

If you are looking for the *previous* generation of this skill (hand-rolled
`experiments`/`experiment_concepts`/`definition` schema, raw `sqlite3` DDL):
that schema predates even harvey's original `knowledge.go` and no longer
matches any live database in this workspace. This rewrite supersedes it.

---

## Variables

| Variable  | Type   | Description                                  | Default               | Prompted? |
|-----------|--------|-----------------------------------------------|------------------------|-----------|
| `DB_PATH` | string | Path to the SQLite3 database file.            | `agents/knowledge.db`  | No        |

---

## Schema reference (current, as of `knowledge` v0.0.25)

| Object                        | Kind  | Purpose                                                       |
|--------------------------------|-------|----------------------------------------------------------------|
| `projects`                     | table | One row per experiment/project (name, description, status, `updated_at`, uuid, origin_host) |
| `observations`                 | table | Timestamped notes/findings/decisions/questions/hypotheses per project. Immutable — corrected by superseding, never edited in place |
| `observation_relations`        | table | `supersedes` edges between observations (DR-0023) — how a correction is expressed |
| `concepts`                     | table | Named ideas/terms (optionally scholarly identifiers: DOI, ORCID, ROR, FundRef), with `updated_at` and a `kind` (`character`, `place`, `theme`, `other`; v0.0.24) |
| `sources`                      | table | Citable documents/resources, with retraction tracking          |
| `records`                      | table | Decision records — one row per `decisions/NNNN-slug.md` file, indexed by `kb ingest` |
| `record_relations`             | table | `supersedes`/`relates_to` edges between records                |
| `record_concepts`              | table | Links records → concepts, from `[[wikilink]]` tags in the body and the frontmatter `tags` list |
| `documents`                    | table | Narrative/article documents (Markdown, Fountain, plain text) ingested via `kb document ingest` |
| `document_sections`            | table | One row per document's gist plus one per structural section (scene/heading), each with its own summary lifecycle |
| `document_section_concepts`    | table | Links document sections → concepts, from `[[wikilink]]` tags, frontmatter `keywords`, and (v0.0.10) density linking — an already-known concept mentioned more than once outside code spans |
| `document_sources`, `record_sources` | table | Link a document or a decision record to a cited source, with a relationship label (v0.0.25; `kb source link --document|--record`, or a `sources:` front-matter key) |
| `story_events`                 | table | Derived: one row per scene or event in a writing project's manuscripts, rebuilt by every `kb document ingest`; does **not** travel through export, import, merge or check-db (v0.0.24) |
| `story_knowledge`              | table | Derived: what each character learns or already knows in each event (`learns`/`knows` keys), rebuilt with `story_events` and likewise local (v0.0.25) |
| `observation_concepts`         | table | Links observations → concepts (many-to-many)                   |
| `project_concepts`             | table | Links projects → concepts (many-to-many)                       |
| `observation_sources`          | table | Links observations → sources, with a relationship label        |
| `project_summary`              | view  | Projects with their concept tags (comma-separated)              |
| `kb_fts`                       | FTS5  | Full-text index over projects/observations/concepts/records and *reviewed* document summaries (if compiled in) |

`concepts.kind` values: `character`, `place`, `theme`, `other` (default).
`projects.status` values: `concept` → `active` → `paused` / `concluded`.
`observations.kind` values: `note`, `finding`, `decision`, `question`, `hypothesis`.
`records.status` values: `proposed` → `accepted` / `superseded` / `rejected` / `cancelled` (`cancelled` = pursued or explored, then abandoned, and need not have been accepted; `rejected` = considered and not pursued).
`document_sections.summary_status` values: `unsummarized` → `drafted` →
`reviewed` — promotion to `reviewed` is a human-only action (`kb document
review promote`); only a reviewed summary is ever indexed for search or
returned as trustworthy content by a concept-tag query.

Concept tagging is largely automatic: an `[[Name]]` wikilink written inside
a record's or document section's body, or an entry in a record's `tags` /
a document's frontmatter `keywords`, resolves to a concept and links it at
ingest time — case-insensitively, so capitalization driven by sentence
position doesn't fragment the concept graph. This does **not** apply to
`kb observation add` bodies, which are plain text with no tag scanning.

`kb project rename OLD NEW` and `kb concept rename OLD NEW` are the only
correct way to rename either — never `UPDATE projects/concepts SET name =
...` directly. As of v0.0.10 (DR-0026, superseding DR-0024's outright
refusal) `project rename` also works for a project that owns records: it
rewrites every owned record's `project:` frontmatter first, both-or-neither,
then renames the project row, touching no record's database row. Use
`--dry-run` to see which files would change; note the flags come before the
names (`project rename [--root PATH] [--dry-run] OLD NEW`). `concept rename`
has no corpus to keep in step and needs no such pass.

Cross-machine sync (`kb merge`/`kb import`) reconciles a project's or
concept's `description`/`status`/identifier fields by `updated_at` —
whichever side was edited more recently wins, regardless of which side is
applied first (DR-0025). **As of v0.0.10 this covers a rename too**
(DR-0026): conflict resolution is now uuid-primary, so a project or concept
renamed on one machine and left untouched on another arrives as one renamed
row rather than two. Name-keyed matching survives only as a fallback, for
two genuinely independent entities that were never synced and happen to
share a name. The same release fixed two live bugs on that path — `merge`
could keep the stale name or drop a side outright depending on merge order,
and `import` hard-failed the entire import rather than one row.

Every row also carries a `uuid` (assigned automatically) used for
cross-machine identity — see `kb-merge(1)` / `kb-export(1)` /
`kb-import(1)` if you need to sync this database with another machine.

---

## Step 1 — Resolve DB_PATH and locate the `kb` binary

```bash
DB_PATH="${DB_PATH:-agents/knowledge.db}"
KB="$(command -v kb || echo "$HOME/Laboratory/knowledge/bin/kb")"
```

If neither `kb` on `PATH` nor `~/Laboratory/knowledge/bin/kb` exists, report
that the `knowledge` module needs building first: `cd ~/Laboratory/knowledge
&& make build` (or `go build -o bin/kb cmd/kb/*.go`).

Then run `kb -version`. The other `kb` skills need **kb >= v0.0.19**; if it is
older, say so and give the upgrade command
(`curl https://rsdoiel.github.io/knowledge/installer.sh | sh`) before going on.

---

## Step 2 — Open (and thereby create/migrate) the database

```bash
"$KB" --db "$DB_PATH" project list
```

This alone is enough: if `DB_PATH` didn't exist, it now does, with the
current schema applied. If it already existed on an older schema, `Open()`
already migrated it in place as a side effect of this call.

---

## Step 3 — Report

- If the command printed `(no projects)`, report that the database was
  created fresh (or already existed and is empty).
- Otherwise, report the project list as confirmation the database is up and
  reachable.

---

## Finding the workspace, and the environment (kb >= v0.0.17)

Without `--db`, `kb` walks up from the current directory to the nearest ancestor
with `agents/knowledge.db` or `agents/knowledge.jsonl`, as git finds `.git`, so a
verb works from any subdirectory. It never looks sideways: another workspace on the
machine (`~/WorkLab` from `~/Laboratory`, say) is a separate knowledge base and is
never reached. When the workspace it found is not the current directory it says so
once on stderr (`kb: using the workspace at DIR (found above the current directory)`).

A fresh clone has `agents/knowledge.jsonl` but no database, because databases are
not tracked. `kb` then names the remedy: run `kb import -in agents/knowledge.jsonl`
from anywhere inside the workspace, which builds `agents/knowledge.db` there. Do not
run `kb init` for that: it starts an empty history beside the real one.

Environment variables, each beaten by the command-line option:
`KB_DB` (the database, as `--db`), `KB_PROJECT` (the project to act on when the
directory gives none), `KB_CEILING_DIRECTORIES` (directories the walk up never enters
or passes, to keep a scratch directory inside a workspace from reaching it) and
`KB_QUIET` (silence advisory notes; never an error).

A decision record is named `SCOPE/DR-NNNN` (`harvey/DR-0004`, `workspace/DR-0003`),
and a project cannot be called `workspace`.

---

## A new workspace, and keeping two machines in step

`kb init [PATH]` creates `PATH/agents/knowledge.db` (schema only) and is the
verb for a genuinely new workspace; it leaves an existing database alone. The
`--db DB_PATH` form above also creates the file, but without `--db` every
other verb now refuses to create one and exits 66, pointing at `kb init` or
`kb import -in FILE`. `--db` does not apply to `init`: name the target as the
argument.

When `agents/knowledge.jsonl` travels by git between machines, `kb check-db`
(kb >= v0.0.15) compares the database with the dump beside it by content, not
by file time, and recommends `kb import`, `kb export` or both. Exit 0 means in
sync; 1 means not in sync and the report is still printed; 66 means there is
no dump. Run it after a `git pull` rather than guessing which file is newer.
It writes nothing; acting on the recommendation is a separate step the user
should approve, because importing brings back anything deleted on purpose.

---

## Tab completion

`kb completion bash|powershell` (kb >= v0.0.16) writes a completion script to
standard output; `pwsh` is accepted for `powershell`. `-install` installs it
instead and prints where it went. It is offered, never done unasked: it
changes the user's shell setup, not the workspace.

```bash
kb completion bash -install        # $XDG_DATA_HOME/bash-completion/completions/kb, else ~/.local/share/...
kb completion powershell -install  # kb-completion.ps1 beside the profile, plus one dot-source line, once
```

It opens no database and refuses `--db`, so it works anywhere, with or without
a workspace. Project names complete from `kb project list` in the current
directory, so they appear only inside a workspace. Exit 2 is no shell, an
unsupported shell or a surplus argument; exit 73 is a bash completion file
`kb` did not write, which it will not overwrite: report the path and let the
user decide. Windows PowerShell 5.1 is not covered.

---

## Error handling

| Condition                        | Action                                                              |
|-----------------------------------|-----------------------------------------------------------------------|
| `kb` not found anywhere            | Report; suggest building it (`cd ~/Laboratory/knowledge && make build`) |
| `DB_PATH`'s parent directory missing | `kb`/`Open()` creates it automatically (`os.MkdirAll`) — not an error |
| Command exits non-zero             | Report the stderr message verbatim; `kb` always routes errors there, never stdout; then act on the exit code (below) |

### Exit codes

`kb` follows the workspace exit-code convention (workspace DR-0003; `kb` v0.0.14 and later). For setup the ones that matter: **66** means the workspace or a path is
missing (run this skill, or `kb init`); **65** means the file at `DB_PATH` is not a knowledge
base (report it; never overwrite it); **75** means the database is locked, so retry later;
**77** is a permission problem; **70** is a bug in `kb`. Exit 1 is a normal negative answer,
2 means the command line was wrong. Earlier `kb` exited 1 for every failure.
