

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

In the interface, `Enter` on a record reads it and `s` sets its status; both show the record file in your pager (`$KB_PAGER`, else `bat -l markdown`, else `less -R`) with the interface suspended, or in a built-in viewer when there is none. From the command line, `set-status` with no status needs a terminal and does the same: it shows the record, prompts with the moves the record's current status allows (`harvey/DR-0004 is proposed -> [a]ccepted [r]ejected [c]ancelled [q]uit`), and writes nothing until you press `y`. A status is one key, with no Enter. `Esc`, `q` or `Ctrl-C` backs out at either step, as does `n` at the confirmation; `Enter` does nothing, so a habitual Enter is never consent. Backing out changes nothing and exits 1.

The moves are a table: `proposed` to `accepted`, `rejected`, `cancelled` or `superseded`; `accepted` to `cancelled` or `superseded`; `rejected` or `cancelled` back to `proposed`; `superseded` is final and needs `superseded_by` (use `kb record supersede`). `rejected` means considered and not pursued; `cancelled` means pursued or explored and then abandoned. Scripts use the direct form, `kb record set-status REF STATUS`, which is held to the same table. `kb record set-status REF accepted` exits 2 unless standard input and output are a terminal, and nothing turns that off, so a script or a model can propose, reject, cancel and supersede but not accept. The same holds for `kb document review promote SECTION_ID`, which makes a document summary trusted and searchable. `kb ingest` reports a status that arrives through a file edit, so that route is visible too. See [kb-record(1)](kb-record.1.md).

## Release Notes

- version: 0.0.20
- status: active
- released: 2026-10-09

The terminal interface can now do everything the command line does to what you are looking at, behind the same gates. The decisions are DR-0066 to DR-0070 (the menu tree and `:`, the shared chooser, the write classes and the removing gate, plan then apply for ingest, the document summary workflow, and the terminal rule for promote), from the design brief `tui-keys-and-write-flows-design.md` and its plan. Everything was written and tested red first, including the built binary on a pseudo-terminal with real bytes. The race detector passed on darwin/arm64 at the last code commit, and the interface was tried by hand on Linux (Raspberry Pi OS) and macOS. It has not been run on Windows.

Writes. `n` on the project list, an observation or concept tab and the Records screens, and the New rows of the menus, open a form that asks the fields one at a time and ends with the command that does the same and `y`. `s`, `e` and `r` on the project list change a project's status, description and name; `e` on an observation writes a correction (a new observation that supersedes it, as `kb observation update` does) and on a concept renames it; `u` on a record makes it supersede another. `d` deletes a project, observation, concept or record behind a gate that shows what points at it and asks for its name to be typed. Each write is the command's own function, so its rules and refusals are the same, and the screen ends by showing the command that does the same.

Plans and the command line. Ingest, Records, Fuzzy-tag and Documents (Ingest a document, Tag, Fuzzy-tag) show the command's own `--dry-run` output first and apply on `y`; Index and Format files run at once. `:` opens a command line on the menus and browsing screens: any verb, run by the command line's own code, with the gate its write class calls for (reads and direct verbs run, additive and changing writes ask `y`, a delete meets the typed-name gate, plan verbs show their dry run, `merge`, `import`, `init`, `export` and `completion` are refused).

Documents. Documents, Review queue lists the sections waiting for a person; `Enter` writes the summary in `$VISUAL` or `$EDITOR` (the interface is suspended) or in a built-in text area, then shows it above its source, and `y` saves it as written by a person and promotes it. A section that already has a draft starts at that review, so a model's draft is read beside its source, and promoted as it is it keeps its author.

`kb ingest` now says which concepts it created, or with `--dry-run` would create, so a typo in a `[[wikilink]]` no longer adds a concept silently (`concepts_created` in `-json`; `(*KnowledgeBase).HasConcept` in the library).

Building: `cmt` is no longer part of `make`. `make generate` regenerates `version.go`, `about.md`, `CITATION.cff` and the installers from `codemeta.json`, and `make release` runs it, so the version and release hash change only when asked for.

Not in the interface yet: Documents Browse, Documents Frontmatter (`document frontmatter` is reachable through `:`), Concepts Recall and Suggest, Check, and the removing writes `source remove`, `unlink` and `document delete`; `kb verbs --json` does not yet carry the menu fields.


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

