# knowledge User Manual

A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records and narrative documents across independent experiments. `cmd/kb` is a single `kb VERB ARGS` binary (matching the `git`/`go` command model) covering the full API, plus a read-mostly interactive TUI.

---

## Quick Start

1. **[Installation](INSTALL.md)** — build from source or install a release binary
2. **[Command Reference](kb.1.md)** — the primary `kb(1)` man page: global flags, every verb, exit status

Run `kb` with no verb to launch the interactive TUI, or `kb help` / `kb -h` to see this same reference at the terminal.

---

## Command Reference

`kb` follows the `TOOL VERB PARAMETERS` model — every verb below has its own man page, also reachable via `kb help VERB` or `kb VERB -h`.

| Verb | Man page | Purpose |
|---|---|---|
| `init` | [kb-init(1)](kb-init.1.md) | create a new, empty workspace |
| `project` | [kb-project(1)](kb-project.1.md) | add, list, show, rename projects; set status/description; list a project's concepts |
| `observation` | [kb-observation(1)](kb-observation.1.md) | add, list, show, update observations; list an observation's sources |
| `concept` | [kb-concept(1)](kb-concept.1.md) | add, list, rename concepts — including scholarly identifiers (DOI, ORCID, ROR, Fundref) — and `suggest` candidate new ones from the corpus |
| `link` | [kb-link(1)](kb-link.1.md) | link projects/observations to concepts |
| `source` | [kb-source(1)](kb-source.1.md) | manage cited sources; check DOIs against Retraction Watch |
| `search`, `summary`, `format` | [kb-search(1)](kb-search.1.md) | full-text search (FTS5) and assembled Markdown views |
| `ingest` | [kb-ingest(1)](kb-ingest.1.md) | index a tree of decision records into the database |
| `record` | [kb-record(1)](kb-record.1.md) | author and maintain decision records — new, list, show, set-status, supersede, fmt, concepts |
| `index` | [kb-index(1)](kb-index.1.md) | generate a corpus's `decisions/index.md`; `--check` for staleness, `--all` for a whole tree |
| `document` | [kb-document(1)](kb-document.1.md) | ingest, draft, review and tag narrative documents (Markdown, Fountain, text) at graduated abstraction levels |
| `verbs` | [kb-verbs(1)](kb-verbs.1.md) | print the verb table — every verb, subverb, flag and what it does to the database; `kb -json verbs` for models and scripts |
| `merge` | [kb-merge(1)](kb-merge.1.md) | reconcile two `knowledge.db` files that drifted independently (e.g. across machines) |
| `export` | [kb-export(1)](kb-export.1.md) | write a portable JSON-L snapshot — the no-file-access alternative to `merge` |
| `import` | [kb-import(1)](kb-import.1.md) | apply a JSON-L snapshot (from `export`) to the database |
| `topics` | [kb-topics(1)](kb-topics.1.md) | list every help topic, one man page per verb |

### Global flags

- `--db PATH` — path to `knowledge.db`. Without it, `kb` walks up from the current directory to the nearest `agents/knowledge.db` (or `agents/knowledge.jsonl`, in a fresh clone), so it works from any subdirectory of a workspace
- `--json` — machine-readable output on stdout; errors always go to stderr in both modes, so scripts and other language-model harnesses can drive `kb` directly
- `--debug` — write a JSONL trace of every knowledge-base call (and, in the TUI, every input event and view change) to `./kb-debug-<timestamp>.jsonl`

Full detail on all three: [kb(1)](kb.1.md), § GLOBAL OPTIONS.

### Workspace and environment

`kb` finds its workspace by walking up, never sideways: another workspace on the same machine is never reached. These variables are read, and an option on the command line wins over each:

- `KB_DB` — the database path, as `--db`
- `KB_PROJECT` — the project to act on when none is given and the current directory belongs to none
- `KB_CEILING_DIRECTORIES` — directories the walk up never enters or passes (like git's `GIT_CEILING_DIRECTORIES`); set it to a scratch directory that sits inside a real workspace
- `KB_QUIET` — silence advisory notes on standard error; never an error
- `KB_PAGER` — the program that shows a record for `kb record set-status REF` (no status): `bat -l markdown`, else `less -R`, else the record is printed

See [kb(1)](kb.1.md), § WORKSPACE AND ENVIRONMENT.

### Naming records

A decision record is `SCOPE/DR-NNNN`, where SCOPE is a project name or `workspace`: `harvey/DR-0004`, `workspace/DR-0003`. Listings print that form, and a change to a record (`set-status`, `supersede`, `delete`) needs it, or a project inferred from the working directory. `kb record list harvey clasm` takes scopes as arguments, `kb record pending` lists what is waiting for a decision, and `--all` widens a project directory to the whole workspace. See [kb-record(1)](kb-record.1.md).

### Interactive TUI

Bare `kb` on a terminal opens a menu with one entry per verb group (Projects, Records, Observations, Concepts, Sources, Documents, Search, Ingest, Index, Check), so using it teaches the command line. A header names the workspace and what it holds. Rows that are not built yet are dimmed with the release that brings them, and choosing one prints the command that does the same. Projects → Browse opens a project's observations, concepts and records as tabs (`o`, `c`, `r`); Records → Browse and Pending list records in the scope `kb record list` would use, and `a` widens it. `q` goes back one screen (and quits at the top menu), `Esc` cancels only something in progress and never closes a screen, `Ctrl-C` quits from anywhere, and a legend line at the bottom of every screen lists the keys that apply. Typing in the search prompt takes every key as text, `q` included.

On a record (the Records screens and a project's Records tab) `Enter` reads it and `s` sets its status. Both show the record file in your pager (`$KB_PAGER`, else `bat`, else `less -R`) with the interface suspended, or in a built-in viewer when there is no pager. `s` then offers the moves the record's status allows, each on one key, asks for `y`, and writes by the same function as `kb record set-status REF`. `Esc` or `q` backs out at either step with nothing written. When it writes, the screen shows the equivalent command line.

`d` deletes the selected project, observation, concept or record. It is not a y/n: a screen shows what will be removed and what points at it, and you type the thing's name (a record's reference, an observation's number) exactly; only an exact match and `Enter` delete, and `Esc` cancels. While you type every key is text, `q` included. A delete the command line would refuse (a project that owns content, a record whose file still exists) is refused here too, with the reason, and the screen shows the equivalent command after a delete.

Changes work the same way: the item is shown with its current value, the new one is picked (a project's status, one key each) or typed (a description, a name, an observation's correction), and `y` confirms old to new before anything is written. On the project list `s` sets the status, `e` edits the description and `r` renames; on an observation `e` writes a correction (a new observation that supersedes it, as `kb observation update` does); on a concept `e` renames it; on a record `u` makes it supersede another (you type which one, and the field refuses what `kb record supersede` refuses). In a text field `Enter` accepts, `Ctrl-J` starts a new line, `Ctrl-U` clears the line, and `Esc` cancels. Each change is the command's own function, so its refusals are shown with nothing changed.

New things are made with a form (the New… rows of the menus, and `n` on the project list, an observations tab, a concepts tab and the Records screens): the fields are asked one at a time, a line of text or a choice made with one key, what has been entered stays on screen, and the last screen shows the command that does the same and asks for `y`. `Esc` cancels the whole form. A new decision record is written `proposed` and reaches the database at the next `kb ingest`.

Index (top menu) and Records → Format files… run `kb index` and `kb record fmt` at once, with no confirmation, on the Records scope's decisions directory (a project's, or the whole `agents` tree with every scope); the command's own output is shown, with the equivalent command.

A complete command line always runs and prints, on a terminal or not. Only an incomplete one opens the interface, and only on a terminal: bare `kb`, a bare group such as `kb record`, and `kb record show` or `kb project show` with no argument. `kb -i record pending` (or `-i record list`, `-i record show REF`, `-i project list`, `-i search TERM`) opens the interface at that command. See [kb(1)](kb.1.md) for the full description.

---

## Background & Design

These documents record why `kb` is shaped the way it is — useful if you're extending it, not required to use it:

- **[decisions/index.md](decisions/index.md)** — the decision-record corpus: every architecture/UX decision this module has made, each with its rejected alternatives and the real bugs found along the way. It replaces the old flat `DECISIONS.md`, which was split into records when this module became the format's first conversion pilot; `kb search` reaches the reasoning inside them, not just their titles
- [module-extraction-design.md](module-extraction-design.md) / [-plan.md](module-extraction-plan.md) — pulling this module out of `harvey`
- [cli-tui-design.md](cli-tui-design.md) / [-plan.md](cli-tui-plan.md) — the `kb` CLI and TUI
- [debug-logging-design.md](debug-logging-design.md) / [-plan.md](debug-logging-plan.md) — the `--debug` JSONL trace
- [jsonl-export-design.md](jsonl-export-design.md) / [-plan.md](jsonl-export-plan.md) — the `export`/`import` JSON-L format and identity/conflict rules
- [decision-records-design.md](decision-records-design.md) / [-plan.md](decision-records-plan.md) — `ingest`, `record` and `index`: decision records as first-class rows
- [records-portability-design.md](records-portability-design.md) / [-plan.md](records-portability-plan.md) — carrying records and their relations through `merge`, `export` and `import`
- [wikilink-tagging-design.md](wikilink-tagging-design.md) / [-plan.md](wikilink-tagging-plan.md) — `[[Name]]` inline tagging and frontmatter tags resolving to concepts at ingest time
- [concept-tag-retrieval-design.md](concept-tag-retrieval-design.md) / [-plan.md](concept-tag-retrieval-plan.md) — `MatchConceptNames`/`RecallByConceptNames`, an embedder-free retrieval path for small models
- [narrative-documents-design.md](narrative-documents-design.md) / [-plan.md](narrative-documents-plan.md) — the `documents` entity, graduated abstraction levels, and the summary review workflow

---

## Can't Find What You Need?

- Run `kb help <verb>` or `kb <verb> -h` for the same reference at the terminal
- **[About](about.md)** — project metadata, license, requirements
- **[Getting Help, Reporting Bugs](https://github.com/rsdoiel/knowledge/issues)**
