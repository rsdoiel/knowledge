

# knowledge

A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.

## The interactive interface

`kb` with no verb, on a terminal, opens a menu with one row per verb group, so using it teaches the command line:

```
┌ kb — knowledge ────────────────────────────────────────────────────────────┐
│ ~/Laboratory   agents/knowledge.db   119 records · 23 projects             │
│                                                                            │
│ > Projects         browse projects, their notes, concepts, records         │
│   Records          decision records: browse, pending, …                    │
│   Observations     notes, findings, decisions, questions         (v0.0.20) │
│   …                                                                        │
├────────────────────────────────────────────────────────────────────────────┤
│ ↑/↓ j/k move   Enter open   / search   q quit                              │
└────────────────────────────────────────────────────────────────────────────┘
```

Rows that are not built yet are dimmed with the release that brings them, and choosing one prints the command that does the same. A project opens to its observations, concepts and records as tabs (`o`, `c`, `r`); Records → Browse and Pending list records in the scope `kb record list` would use, and `a` widens it. `q` goes back one screen (and quits at the top menu), `Esc` cancels only something in progress and never closes a screen, `Ctrl-C` quits from anywhere, and a line at the bottom of every screen lists the keys that apply. While you type in the search prompt every key is text, `q` included.

A complete command line always runs and prints, on a terminal or not. Only an incomplete one opens the interface, and only on a terminal: bare `kb`, a bare group (`kb record`), and `kb record show` or `kb project show` with no argument. `kb -i record pending` (also `-i record list`, `-i record show REF`, `-i project list`, `-i search TERM`) opens the interface at that command. `kb verbs` prints the table every menu, completion script and help flag is built from; `kb -json verbs` gives it to a script or a model.

## Deciding on a record

A decision record is written `proposed` and a person decides it. `kb` enforces that: accepting needs a terminal.

```shell
kb record pending harvey              # what is waiting for a decision
kb record set-status harvey/DR-0004   # shows the record, offers the moves, asks before it writes
```

In the interface, `Enter` on a record reads it and `s` sets its status; both show the record file in your pager (`$KB_PAGER`, else `bat -l markdown`, else `less -R`) with the interface suspended, or in a built-in viewer when there is none. From the command line, `set-status` with no status needs a terminal and does the same: it shows the record, prompts with the moves the record's current status allows (`harvey/DR-0004 is proposed -> [a]ccepted [r]ejected [c]ancelled [q]uit`), and writes nothing until you press `y`. A status is one key, with no Enter. `Esc`, `q` or `Ctrl-C` backs out at either step, as does `n` at the confirmation; `Enter` does nothing, so a habitual Enter is never consent. Backing out changes nothing and exits 1.

The moves are a table: `proposed` to `accepted`, `rejected`, `cancelled` or `superseded`; `accepted` to `cancelled` or `superseded`; `rejected` or `cancelled` back to `proposed`; `superseded` is final and needs `superseded_by` (use `kb record supersede`). `rejected` means considered and not pursued; `cancelled` means pursued or explored and then abandoned. Scripts use the direct form, `kb record set-status REF STATUS`, which is held to the same table. `kb record set-status REF accepted` exits 2 unless standard input and output are a terminal, and nothing turns that off, so a script or a model can propose, reject, cancel and supersede but not accept. The same holds for `kb document review promote SECTION_ID`, which makes a document summary trusted and searchable. `kb ingest` reports a status that arrives through a file edit, so that route is visible too. See [kb-record(1)](kb-record.1.md).

## Release Notes

- version: 0.0.19
- status: active
- released: 2026-10-09

The terminal interface can now change something, and `kb` describes its own command language in one table. The decisions are DR-0059 (one verb table, in part), DR-0064 to DR-0067 (keys, a shared chooser, the menu tree, and which writes get which gate), DR-0068 (ingest) and DR-0069 and DR-0070 (documents and promotion), from the design brief `tui-keys-and-write-flows-design.md`. Everything was written and tested red first, including the built binary on a pseudo-terminal with real bytes. The race detector passed on darwin/arm64 at the last code commit, and the interface was tried by hand in a real terminal on Linux.

Bare `kb`, on a terminal, opens a menu with one row per verb group (Projects, Records, Observations, Concepts, Sources, Documents, Search, Ingest, Index, Check), under a header naming the workspace, its database and what it holds. A group opens its own menu; rows that are not built yet are dimmed with the release that brings them, and choosing one shows the command-line equivalent. A project's observations, concepts and records are tabs with counts (`o`, `c`, `r`). Records Browse and Pending list records in the scope `kb record list` would use, one line each with its qualified reference, and `a` widens the scope. On a record, `Enter` reads it and `s` sets its status: the record file is shown in your pager (`$KB_PAGER`, else `bat`, else `less -R`) with the interface suspended, or in a built-in viewer when there is none, then each status is one key, `y` confirms, and `Esc` or `q` backs out at either step with nothing written. The write is the same function `kb record set-status` uses, and the screen shows the equivalent command.

The keys follow clasm's. `q` goes back one screen and quits only at the top menu, `Esc` cancels something in progress and never closes a screen, `Ctrl-C` quits from anywhere, and a legend line on every screen lists the keys that apply. While you type in the search prompt every key is text, so a term that starts with `q` works. A complete command line always runs and prints, on a terminal or not; only an incomplete one opens the interface, and only on a terminal (bare `kb`, a bare group such as `kb record`, `kb record show` or `kb project show` with no argument). The new global option `-i` opens the interface at a command: `kb -i record pending`, `kb -i record show REF`, `kb -i search TERM`.

`kb verbs` (and `kb -json verbs`) prints the verb table: every verb, subverb and flag, with what it does to the database. `kb record set-status REF` with no status is now a review that takes single keys, with no Enter, and `Enter` on the confirmation does nothing. `kb document review promote` needs a person at a terminal, as `accepted` does: a model can draft a summary but cannot make it trusted. `kb ingest` reports a status that arrives through a file, such as `clasm/DR-0012 proposed -> accepted (edited in file)` or a record first ingested already accepted; it is a report, not a block.

Upgrading: the interface's keys changed (`q` goes back instead of quitting, `Esc` no longer leaves a screen), and a script that promoted summaries without a person at the keyboard must hand that step over. Bare `kb` with no terminal is now a usage error (exit 2) and no longer tries to start a terminal program, and opening the interface no longer creates a database where there is no workspace. `lipgloss` is a direct dependency; it was already built into `kb` through bubbles. Skills that tell a model to run `set-status ... accepted` or `document review promote` need `kb >= 0.0.19`.


### Authors

- Doiel, R. S.



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

