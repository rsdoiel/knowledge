

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

- version: 0.0.24
- status: active
- released: 2026-10-17

Foul Papers can build on kb: story front matter and Fountain scenes become story events, concepts have a kind, and `kb story timeline` and `kb story check` list and check them. The decisions are DR-0075, DR-0078 and DR-0079 (all accepted).

Story events. A scene file with a narrative key in its front matter (`event`, `story_time`, `pov`, `characters`, `foreshadows`, `pays_off`, `precedes`; `status` and `place` ride along), or a Fountain scene with a scene number (`INT. KITCHEN - DAY #12A#`) whose metadata is a one-line boneyard after the heading (`/* story: story_time=1969-10-29; pov=Anna */`), is a story event. `story_time` is a date, a month, a year, `circa 1890`, `ordinal: 12`, or `after:`/`before:` another event's label with an optional offset (`after: the-fire +3d`). Events are derived from the files by every `kb document ingest`, an unchanged file included, and do not travel through export, import, merge or check-db: after an import, run `kb document ingest` on the same paths.

Timeline and check. `kb story timeline --project P` lists the events in story order, or `--narrative` in reading order; where the order after the row above is not established it says so (`~`) and never guesses. `kb story check --project P` reports unnumbered scenes, duplicate and dangling labels, times it could not read or resolve, orders the dates contradict, cycles, a character in two places on one day, and foreshadowing out of order. It changes nothing and exits 0 unless `--strict`.

Concepts have a kind. `character`, `place`, `theme` or `other` (the default): `kb concept add --kind`, `kb concept set-kind`, `kb concept list --kind`. The kind travels through export, import, merge and check-db.

Wikilinks accept a display alias. `[[the-long-fire/Anna|Anna]]` links the concept before the bar and shows the text after it.

Requires fountain 1.0.3, which parses boneyards and scene numbers. For packagers and Foul Papers: the library exports `StoryEvents`, `StoryTimeline`, `StoryCheck`, `ParseStoryTime`, `ResolveStoryTime`, `SetConceptKind`, `ConceptsOfKind`, `ConceptKinds`, `WikilinkTarget` and the story fields of `ParsedDocument`.

Checked in this release: `go vet` and the test suite pass on Linux (Raspberry Pi OS, arm64) under Go 1.27.2, including a test written from outside the package that drives the whole flow through exported calls. The built binary was run by hand on a small manuscript (prose scene files and a Fountain screenplay) for `kb story timeline` and `check`, and on a copy of the live database, where all 179 existing concepts migrated to kind `other`. The fountain 1.0.3 parser was run against the tests of fdx, osf and scripttool. Not checked: the race detector for this release, macOS, Windows, and a manuscript of realistic length (the timeline compares every pair of events, and was only tried on a handful).


## Software Requirements

- Go >= 1.26.4
- gopkg.in/yaml.v3 >= 3.0.1
- github.com/rsdoiel/fountain >= 1.0.3
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

