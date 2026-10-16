# AGENTS.md — using `kb`, the knowledge base tool

This file is for a language-model harness (Claude Code, harvey, or another) that has been
pointed at a workspace with a `knowledge.db`. It says what `kb` is and how to use it safely.
`kb help` and `kb help TOPIC` are the full manuals; `kb verbs --json` lists every verb, subverb
and flag with what it does to the database.

## What `kb` is

`kb` reads and writes `agents/knowledge.db`, a SQLite3 database kept at the root of a workspace.
It holds **projects**, **observations** (dated notes: `note`, `finding`, `decision`, `question`,
`hypothesis`), **concepts** (named ideas that span projects), **sources** (cited works),
**decision records** (one Markdown file each, indexed), and narrative **documents** with
summaries. `agents/knowledge.jsonl` beside it is the authoritative copy that travels through git.
`kb` finds the workspace by walking up from the current directory; `--db PATH` names one.

## Rules

1. **Write through `kb`, never raw SQL.** `kb` keeps the full-text index in step with every
   write; a hand-written `INSERT` does not. Raw `sqlite3` is for reading what no verb covers.
2. **Use `--json` when a program reads the output.** It goes before the verb: `kb --json search
   toast`. Errors go to standard error, so standard output stays parseable.
3. **Never accept a decision record, and never promote a document summary.** A model may write a
   record (it is created `proposed`) and may draft a summary. `kb record set-status REF accepted`
   and `kb document review promote ID` exit 2 without a terminal, with no flag to change that. Tell
   the person which record to review; they run `! kb record set-status REF` themselves.
4. **Preview before you apply.** `--dry-run` is on `ingest`, `document ingest`, `document tag`,
   `document fuzzy-tag`, `document frontmatter`, `record fuzzy-tag`, `concept delete`, `project
   delete` and others. The verb table calls these `plan_apply`.
5. **Do not open the interface.** `kb` with no verb, a bare group (`kb record`) and `kb -i` need a
   terminal and exit 2 without one. Give the whole command.
6. **Ingest one corpus at a time.** `kb ingest agents/projects/NAME/decisions`, or
   `agents/decisions`. The path is relative to the current directory. A file named
   `YYYY-MM-DD-...md` is a note, not a record, and notes need no frontmatter.

## Verbs

Each is `kb VERB`; the class is what it does to the database.

| Verb | Class | Use |
|---|---|---|
| `project` (`add list show concepts set-status set-description rename delete`) | read, additive, changing, removing | one row per git-repository experiment |
| `observation` (`add list show update sources delete`) | read, additive, changing, removing | `kb observation add --project P KIND BODY` |
| `concept` (`add list show recall rename delete suggest`) | read, additive, changing, removing | `show` lists links and the projects a concept spans |
| `link`, `unlink` | additive, removing | link a project or observation to a concept, or an observation to a source |
| `source` (`add list show remove retract link check-retractions`) | read, additive, changing, removing | cited works; `check-retractions` calls the network |
| `search` | read | `kb search TERM`, or scoped: `kb search harvey/records TERM`, `kb search /concepts TERM` |
| `record` (`list pending show set-status supersede fmt new concepts delete fuzzy-tag`) | read, additive, changing, removing | decision records; refer to one as `SCOPE/DR-NNNN`, `harvey/DR-0004` |
| `ingest` | plan_apply | index a directory of record files |
| `index` | direct | regenerate a `decisions/index.md` |
| `document` (`ingest draft review list review promote show tag fuzzy-tag frontmatter delete`) | guided, plan_apply | narrative documents; summaries need a person to promote |
| `summary`, `format` | read | formatted overviews |
| `stats` | read | counts of rows, links and checks, and a line per project |
| `check-db` | read | compare the database with its JSON-L dump |
| `export`, `import`, `merge` | cli_only | portable snapshots and reconciling two databases |
| `init`, `completion`, `verbs` | cli_only | create a workspace, shell completion, the verb table |

## Exit codes

The same table in every command-line tool in the workspace (workspace DR-0003). Read the number
before the message.

| Code | Class | Meaning |
|---|---|---|
| 0 | ok | Success. |
| 1 | negative | The command ran and the answer is no: nothing found, a stale index, a state that forbids it. Not a failure. |
| 2 | usage | The command line is wrong and nothing was attempted. Also: refused without a terminal. |
| 65 | data | Content `kb` read is wrong: a malformed record or file. |
| 66 | no_input | A named input or the workspace is missing. |
| 69 | unavailable | A service cannot be reached. |
| 70 | internal | A bug. Report it. |
| 73 | cant_create | An output cannot be created, or a file kb did not write is in the way. |
| 74 | io | A read or write failed part way. |
| 75 | temp_fail | Retry later (a locked database). |
| 77 | no_permission | The operating system refused access. |
| 78 | config | A configuration file is present but wrong. |

A script treats 1 as an answer and anything above it as a failure to pass on.

## Decision records

A record is `agents/projects/PROJECT/decisions/NNNN-slug.md` (or `agents/decisions/` for the
workspace tier), with YAML frontmatter, specified in `DECISION_RECORD_FORMAT.md` at the workspace
root. Scaffold one with `kb record new --project P --title T --trigger design`, fill in the body,
then `kb ingest` the directory. Records are `proposed` until a person accepts them.

## Skills

`agents/skills/` in this repository holds three skills that wrap the verbs above:
`setup-knowledge-base`, `update-knowledge-base` and `review-knowledge-base`. Install them for
Claude Code with `make install-skills`. Each needs `kb` of the version in its `kb_version`
metadata or later, and says so at the top.
