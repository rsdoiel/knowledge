

# knowledge

A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and an interactive terminal interface for browsing and, behind explicit gates, changing the knowledge base. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.

## The interactive interface

`kb` with no verb, on a terminal, opens a menu with one row per verb group, so using it teaches the command line:

```
┌ kb — knowledge ────────────────────────────────────────────────────────────┐
│ ~/Laboratory   agents/knowledge.db   119 records · 23 projects             │
│                                                                            │
│ > Projects         browse projects, their notes, concepts, records         │
│   Records          decision records: browse, pending, …                    │
│   Observations     notes, findings, decisions, questions                   │
│   Concepts         named ideas that span projects                          │
│   …                                                                        │
├────────────────────────────────────────────────────────────────────────────┤
│ ↑/↓ j/k move   Enter open   S search   : command   q quit                  │
└────────────────────────────────────────────────────────────────────────────┘
```

Every row is built; one that was not would be dimmed with the release that brings it, and choosing it would print the command that does the same. A project opens to its observations, concepts and records as tabs (`o`, `c`, `r`); Records → Browse and Pending list records in the scope `kb record list` would use, and `a` widens it. `q` goes back one screen (and quits at the top menu), `Esc` cancels only something in progress and never closes a screen, `Ctrl-C` quits from anywhere, and a line at the bottom of every screen lists the keys that apply. While you type, every key is text, `q` included.

**Finding things.** `/` filters the list in front of you as you type (every word must appear; Enter keeps the filter, Esc clears it), on every list. `S` searches the knowledge base from any screen, with the scope of where you are: a project's tabs search that project, a Browse list its kind. The Search row of the top menu offers the scopes, and the prompt takes what the command takes: `harvey/records exit codes`.

```
kb search "exit codes"                 every kind, everywhere
kb search harvey/ "exit codes"         one project
kb search harvey/records "exit codes"  one kind in it
kb search workspace/ "exit codes"      the workspace tier only
kb search /records "exit codes"        records in every scope
```

A first word with no slash is part of the term, so `kb search harvey memory` is still *harvey AND memory*; text that contains a slash goes after `--`. The kinds are `records`, `observations`, `concepts`, `documents` and `projects` (`/projects` only). Results are grouped by kind with counts, and a concept shows the projects it is in. See [kb-search(1)](kb-search.1.md).

**What it can write** (the gate depends on the kind of write, and every write is the command's own function, so the rules and refusals are the same as on the command line; the screen ends by showing the command that does the same):

- *Adding* (`n`, or the New rows): a form asks the fields one at a time and ends with `y`.
- *Changing* (`s`, `e`, `r` on a project; `e` on an observation or concept; `u` on a record to supersede another): the old and new values are shown and `y` confirms.
- *Removing* (`d` on a project, observation, concept, record, or a source or document in a Browse list; `u` to cut a link, from a project's Concepts tab or from an observation): a screen shows what points at it and asks for its name or id to be typed exactly.
- *Plans* (Ingest, Records → Fuzzy-tag, Documents → Ingest, Tag, Fuzzy-tag, Frontmatter): the command's `--dry-run` output is shown first and `y` applies it. Index and Format files run at once. Frontmatter lists what it proposes as a checklist with nothing selected, and plans only what you select.
- *Summaries* (Documents → Review queue): write the summary in `$VISUAL` or `$EDITOR`, or a built-in text area; it is shown beside its source and `y` saves it as written by a person and promotes it.
- *Anything else*: `:` opens a command line; type any verb, and it meets the gate its class calls for.

A complete command line always runs and prints, on a terminal or not. Only an incomplete one opens the interface, and only on a terminal: bare `kb`, a bare group (`kb record`), and `kb record show` or `kb project show` with no argument, and `kb search` with no term or with only a scope (`kb search harvey/records`). `kb -i record pending` (also `-i record list`, `-i record show REF`, `-i project list`, `-i search TERM`) opens the interface at that command. `kb verbs` prints the table every menu, completion script and help flag is built from; `kb -json verbs` gives it to a script or a model.

## Deciding on a record

A decision record is written `proposed` and a person decides it. `kb` enforces that: accepting needs a terminal.

```shell
kb record pending harvey              # what is waiting for a decision
kb record set-status harvey/DR-0004   # shows the record, offers the moves, asks before it writes
```

In the interface, `Enter` on a record reads it and `s` sets its status; both show the record file in a built-in viewer with Markdown highlighting (`/` finds text, `n` and `N` move between matches, `g` and `G` jump to the ends), or in `$KB_PAGER` with the interface suspended when you set it. From the command line, `set-status` with no status needs a terminal and does the same: it shows the record, prompts with the moves the record's current status allows (`harvey/DR-0004 is proposed -> [a]ccepted [r]ejected [c]ancelled [q]uit`), and writes nothing until you press `y`. A status is one key, with no Enter. `Esc`, `q` or `Ctrl-C` backs out at either step, as does `n` at the confirmation; `Enter` does nothing, so a habitual Enter is never consent. Backing out changes nothing and exits 1.

The moves are a table: `proposed` to `accepted`, `rejected`, `cancelled` or `superseded`; `accepted` to `cancelled` or `superseded`; `rejected` or `cancelled` back to `proposed`; `superseded` is final and needs `superseded_by` (use `kb record supersede`). `rejected` means considered and not pursued; `cancelled` means pursued or explored and then abandoned. Scripts use the direct form, `kb record set-status REF STATUS`, which is held to the same table. `kb record set-status REF accepted` exits 2 unless standard input and output are a terminal, and nothing turns that off, so a script or a model can propose, reject, cancel and supersede but not accept. The same holds for `kb document review promote SECTION_ID`, which makes a document summary trusted and searchable. `kb ingest` reports a status that arrives through a file edit, so that route is visible too. See [kb-record(1)](kb-record.1.md).

## Counting, and notes

```shell
kb stats                          # rows, links and checks, and a line per project
kb concept show toast             # what links to it, and the projects it spans
kb document ingest notes/2026-09-29-worklog.md --project harvey
```

`kb stats` answers "how many concept links are there" and "does any observation have no project" without SQL. On a terminal, `kb document ingest` offers frontmatter for a note that has none (title, author, date and known concepts, one at a time, written only after you confirm); `--no-prompt` turns that off, and nothing prompts without a terminal. A file named `YYYY-MM-DD-...md` is a note: it needs no frontmatter and `kb ingest` never reads it as a decision record.

## Using kb from an agent

[AGENTS.md](AGENTS.md) tells a language-model harness what `kb` is and the rules for using it safely: write through `kb`, use `--json`, never accept a record, and what each exit code means. `agents/skills/` holds three skills that wrap the verbs (`setup-knowledge-base`, `update-knowledge-base`, `review-knowledge-base`); `make install-skills` copies them to `~/.claude/skills`.

## Release Notes

- version: 0.0.23
- status: active
- released: 2026-10-16

Notes get frontmatter at ingest, search hits say where they are, `kb stats` shows a database's shape, and the knowledge skills and an agent document now ship in this repository. The decisions are DR-0076 and DR-0077 (both proposed).

Notes. `kb document ingest PATH --project P`, on a terminal, offers frontmatter for a file that has none: `title`, `author`, `dateCreated` and the known concepts the text mentions as `keywords`, one at a time (Enter accepts, text replaces, `-` skips, `q` stops), written only after you confirm. `--no-prompt`, `--dry-run`, `--json` and a missing terminal never prompt. A date at the start of a file name, `2026-09-29-worklog.md`, is now the first `dateCreated` signal, here and in `kb document frontmatter`.

Searching. A hit now carries a record's qualified reference and status (`harvey/DR-0004 (accepted)`) and the passage that matched, the matched words between « and », with the section it is in for a record (`Decision:`, or `title:`). This is in `kb search TERM` as well as the scoped form, and in `--json`.

Counting. `kb stats` reports rows per table, links per join table, five checks (`observations_without_project`, `concepts_without_links` and others) and a line per project, with no SQL; `--json` is one object. `kb concept show` says which projects a concept spans, tagged directly or through observations, records and document sections.

Completion. `kb completion bash -install` now also adds a line to the bash startup file (`~/.bashrc`, or on macOS the first of `~/.bash_profile`, `~/.bash_login` and `~/.profile` that exists), because macOS's bash 3.2 never reads the bash-completion directory, so completion did nothing in a new Terminal window. `-uninstall` removes what `-install` wrote, and `-check` says what is missing. Use `eval "$(kb completion bash)"` to try it in a session: `source <(...)` silently does nothing in bash 3.2.

Fixed. `kb ingest` on a project directory failed on dated notes (`notes/2026-09-29-....md`, "no frontmatter"): the year read as a record id. A record is `NNNN-slug.md` and not `YYYY-MM-DD...`, and the walk skips `design`, `plans`, `feature_requests`, `notes` and `hand-off` below the directory you give it. The closing `kb: ingest: N error(s)` line no longer repeats the first error.

Shipping. `agents/skills/` holds `setup-knowledge-base`, `update-knowledge-base` and `review-knowledge-base`, and `AGENTS.md` (with a `CLAUDE.md` that points at it) tells a harness what kb is, the rules for using it, the verbs and the exit codes. The release zips include them. `make install-skills` copies the skills to `~/.claude/skills` (`SKILLS_DIR` changes it) and `make check-skills` compares; `make install` does not install them. A test fails if a skill's `kb_version` is not the release's.

For packagers and harvey: the library gains `Stats`, `StatCount`, `ProjectStats`, `ConceptSpan` and `ConceptDetailResult.Spans`, and `KBSearchResult` carries `Ref`, `Status`, `Passage` and `Section`; `Search` and `SearchScoped` take what they took.

Checked in this release: the race detector passed on darwin/arm64 (macOS), and the test suite passes on Linux (Raspberry Pi OS, arm64). The built binary was driven on a pseudo-terminal on Linux for the frontmatter prompts, and on macOS for `completion -install`, `-check` and `-uninstall` under /bin/bash 3.2 and for the review skill. Not checked: the frontmatter prompts by hand on macOS or Windows, and `completion` on Windows.


## Software Requirements

- Go >= 1.26.4
- gopkg.in/yaml.v3 >= 3.0.1
- github.com/rsdoiel/fountain >= 1.0.2
- git >= 2.0 (runtime, for kb document frontmatter's provenance detection)

### Software Suggestions

- CMTools >= 0.0.45b
- Pandoc >= 3.9
- GNU Make >= 3



## Related resources



- [Getting Help, Reporting bugs](https://github.com/rsdoiel/knowledge/issues)
- [LICENSE](https://www.gnu.org/licenses/agpl-3.0.txt)
- [Installation](INSTALL.md)
- [About](about.md)

