

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
│ ↑/↓ j/k move   Enter open   / search   : command   q quit                  │
└────────────────────────────────────────────────────────────────────────────┘
```

Rows that are not built yet are dimmed with the release that brings them, and choosing one prints the command that does the same. A project opens to its observations, concepts and records as tabs (`o`, `c`, `r`); Records → Browse and Pending list records in the scope `kb record list` would use, and `a` widens it. `q` goes back one screen (and quits at the top menu), `Esc` cancels only something in progress and never closes a screen, `Ctrl-C` quits from anywhere, and a line at the bottom of every screen lists the keys that apply. While you type, every key is text, `q` included.

**What it can write** (the gate depends on the kind of write, and every write is the command's own function, so the rules and refusals are the same as on the command line; the screen ends by showing the command that does the same):

- *Adding* (`n`, or the New rows): a form asks the fields one at a time and ends with `y`.
- *Changing* (`s`, `e`, `r` on a project; `e` on an observation or concept; `u` on a record to supersede another): the old and new values are shown and `y` confirms.
- *Removing* (`d` on a project, observation, concept or record): a screen shows what points at it and asks for its name to be typed exactly.
- *Plans* (Ingest, Records → Fuzzy-tag, Documents → Ingest, Tag, Fuzzy-tag): the command's `--dry-run` output is shown first and `y` applies it. Index and Format files run at once.
- *Summaries* (Documents → Review queue): write the summary in `$VISUAL` or `$EDITOR`, or a built-in text area; it is shown beside its source and `y` saves it as written by a person and promotes it.
- *Anything else*: `:` opens a command line; type any verb, and it meets the gate its class calls for.

A complete command line always runs and prints, on a terminal or not. Only an incomplete one opens the interface, and only on a terminal: bare `kb`, a bare group (`kb record`), and `kb record show` or `kb project show` with no argument. `kb -i record pending` (also `-i record list`, `-i record show REF`, `-i project list`, `-i search TERM`) opens the interface at that command. `kb verbs` prints the table every menu, completion script and help flag is built from; `kb -json verbs` gives it to a script or a model.

## Deciding on a record

A decision record is written `proposed` and a person decides it. `kb` enforces that: accepting needs a terminal.

```shell
kb record pending harvey              # what is waiting for a decision
kb record set-status harvey/DR-0004   # shows the record, offers the moves, asks before it writes
```

In the interface, `Enter` on a record reads it and `s` sets its status; both show the record file in a built-in viewer with Markdown highlighting (`/` finds text, `n` and `N` move between matches, `g` and `G` jump to the ends), or in `$KB_PAGER` with the interface suspended when you set it. From the command line, `set-status` with no status needs a terminal and does the same: it shows the record, prompts with the moves the record's current status allows (`harvey/DR-0004 is proposed -> [a]ccepted [r]ejected [c]ancelled [q]uit`), and writes nothing until you press `y`. A status is one key, with no Enter. `Esc`, `q` or `Ctrl-C` backs out at either step, as does `n` at the confirmation; `Enter` does nothing, so a habitual Enter is never consent. Backing out changes nothing and exits 1.

The moves are a table: `proposed` to `accepted`, `rejected`, `cancelled` or `superseded`; `accepted` to `cancelled` or `superseded`; `rejected` or `cancelled` back to `proposed`; `superseded` is final and needs `superseded_by` (use `kb record supersede`). `rejected` means considered and not pursued; `cancelled` means pursued or explored and then abandoned. Scripts use the direct form, `kb record set-status REF STATUS`, which is held to the same table. `kb record set-status REF accepted` exits 2 unless standard input and output are a terminal, and nothing turns that off, so a script or a model can propose, reject, cancel and supersede but not accept. The same holds for `kb document review promote SECTION_ID`, which makes a document summary trusted and searchable. `kb ingest` reports a status that arrives through a file edit, so that route is visible too. See [kb-record(1)](kb-record.1.md).

## Release Notes

- version: 0.0.21
- status: active
- released: 2026-10-13

Records are now read in kb's own viewer, with Markdown highlighting, and the pager no longer depends on which programs happen to be installed. The decision is DR-0071 (a partial supersede of DR-0066 point 5).

Reading. Enter on a record in the terminal interface used to return straight to the list when `bat` was installed: `bat` quits at once when the text fits one screen and the interface repainted over it. kb now shows a record itself. YAML frontmatter, headings, fenced code, list markers, quotes, inline code, bold and links are coloured with the terminal's own 16 colours; `NO_COLOR` or `TERM=dumb` turns it off. In the viewer `/` finds text (case-insensitive), `n` and `N` move between matches, `g` and `G` jump to the ends. `kb record set-status REF` with no status prints the same highlighted record on a terminal. `bat`, `batcat` and `less` are no longer looked for; `$KB_PAGER` is the override and is run exactly as written (give `bat` `--paging=always`).

Fixed. `kb help ingest` showed `kb ingest clasm/decisions`, which exits 66 from the workspace root: `PATH` is a directory relative to the current directory, one only, walked recursively, and the project and tier come from each record's `project:` frontmatter. The text now says so and the examples work; the `kb index` and `kb record fmt` examples are corrected too. `kb ingest`'s "missing" line said kb had no verb to remove a row; it now names `kb record delete REF`. The `kb-index(1)` page no longer points at the developers' `TODO.md`.

For packagers: `github.com/muesli/termenv` is now a direct dependency (it was already built in). The race detector passed on darwin/arm64 at the last code commit. The viewer was driven on a pseudo-terminal against the built binary on Linux (Raspberry Pi OS), and tried by hand on macOS and Windows, where it worked.

Not in the interface yet: Documents Browse and Frontmatter (frontmatter is reachable through `:`), Concepts Recall and Suggest, Check, and the removing writes `source remove`, `unlink` and `document delete`; `kb verbs --json` does not yet carry the menu fields.

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

