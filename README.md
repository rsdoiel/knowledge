

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

## Semantic search (preview)

`kb search` finds rows that contain your words. The semantic build of `kb` also finds rows by meaning, offline, with no daemon and no network after the model is installed: `kb search -semantic "how do we keep databases in step across machines"` finds the records and notes about cross-machine sync although none of them says "in step". `-hybrid` adds the exact-word matches. It covers observations, decision records, concepts and documents' reviewed summaries; the vectors live in `agents/knowledge.vectors.db` beside the database, never in `knowledge.db` or the JSON-L export.

```shell
kb init                                  # once per workspace; keeps a workspace model and the index out of git
kb model fetch                           # downloads the model (about 87 MB); offline: kb model install PATH
kb embed                                 # builds the index; the first run takes minutes, later runs seconds
kb search -semantic "how do we keep databases in step across machines"
kb search -hybrid hunspell               # meaning, plus rows that contain the word
```

In the interactive interface, `Tab` in the search prompt switches the next search between words, meaning and both. The plain `kb` has none of this and stays at about 21 MB; the semantic build is built from the nested `semantic/` module (`make semantic`, Go 1.27) and is not yet in the release archives. See [kb-search(1)](kb-search.1.md), [kb-embed(1)](kb-embed.1.md) and [kb-model(1)](kb-model.1.md).

Measured on a Raspberry Pi 5 (4 cores, 16 GB) over a copy of this workspace's knowledge base, 893 rows in about 4,100 passages: the first index took 14 to 17 minutes and up to 875 MB of memory, a search about half a second, and a repeat `kb embed` with nothing changed a fraction of a second. The semantic `kb` is about 33 MB against 21 MB for the plain one, and the model about 87 MB. Another machine will differ.

**It is a preview.** The labelled query set it was tuned on (34 queries, `internal/evalset/testdata/labelled_queries.json`) was written by the same model that wrote the search and corrected once after the author looked over where it disagreed with the search, so read its figures as indicative: `-semantic` scored a mean reciprocal rank of 0.82 and a recall at ten of 0.88 on it, against 0.11 and 0.15 for plain search given the same queries (plain search needs every word to appear, so a question typed as a sentence finds nothing). `-hybrid` found about the same rows as `-semantic` and did not rank them better; use it to keep exact-word matches inside a search by meaning. Plain `kb search` is unchanged.

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

- version: 0.0.28
- status: active
- released: 2026-10-28

An exit status and manual page release. Every manual page now has an EXIT STATUS and a SEE ALSO section: the thirteen pages that lacked EXIT STATUS list the codes they return (workspace DR-0003), and tests keep those pages and the verb tests in step. An unreadable database file now exits 77 (no_permission) and not 74 (io); `knowledge.Open` returns an error that satisfies `errors.Is(err, fs.ErrPermission)` for it. `kb(1)` says what 73 and 77 are and that 78 is unused. The setup, update and review skills are pinned to 0.0.28.

v0.0.27 (2026-10-25): the summary editors edit only the summary; a source title is one line; `kb document tag` counts a wrapped link as present; `kb document ingest` reports a skipped link (`tag warning:`); the manual page list comes from the topic list (DR-0087).

v0.0.26 (2026-10-20): `kb story check` reports facts that differ only in a digit as `fact-number-differs` (DR-0086), and `kb story timeline` and `who-knows` are several times faster on long works.

v0.0.25 (2026-10-19):

Foul Papers can ask what a character knows, a Markdown manuscript can hold many scenes, documents and records can cite sources, and the interface starts at once on any terminal. The decisions are DR-0080 to DR-0085 (all accepted).

Story knowledge. A scene says what a character finds out with `learns` and what they already knew with `knows`, each a list of `"Character: fact"` (a YAML list in front matter; the key repeats once per fact in a Fountain boneyard or a Markdown story comment). `kb story who-knows CHARACTER [SCENE] --project P` lists what the character has learned, in story order, up to a scene; facts learned only in scenes whose order is not established are listed apart and never guessed. `kb story who-learns FACT --project P` lists who learns a fact and where. The scene is positional; DR-0075's `--at` is gone. Four advisory `kb story check` kinds: `knowledge-before-learned`, `unknown-fact` (two long facts one edit apart), `fact-number-differs` (the edit is a digit: the same figure, or a continuity slip?) and `late-source` (a source cited as `evidence`, published after the event). Story knowledge is derived from the files, like story events, and does not travel through export, import, merge or check-db: after an import, run `kb document ingest` on the same paths.

Many scenes in one Markdown file. A heading with a scene id is a scene and a story event: `## The fire {#12A}`, or `id=12A` in a story comment on the next line. The file's front matter is the default for every scene. `kb story check` reports Markdown scene problems. A `#` line inside a fenced code block no longer splits a section (this changes the sections of existing documents; re-ingest reports the removed headings once), and a heading's closing `#` marks and `{#id}` are no longer part of its title.

Provenance. Documents and decision records cite sources through an optional `sources:` front matter key (a DOI, a URL, or `source:ID`) or `kb source link --document|--record`. A search hit says which works support it, which version was read, and whether one was retracted. Sources are a search kind (`kb search /sources TERM`). The new link tables travel through merge, export, import and check-db. `observations.source_doi` is no longer written; `kb observation add --source-doi` finds or makes the source and links it.

Interface. The interactive interface moved to the Bubble Tea v2 stack (DR-0084). The old library asked the terminal for its colours before `kb` ran and waited up to five seconds for an answer; on a terminal that never replied the first screen came five seconds late. It now draws at once. Behaviour is otherwise unchanged.

Small fixes. A record's search passage is the paragraph the match is in, without its label or the title (a short record used to come back whole). `kb document ingest` offers the frontmatter editor to a file with a frontmatter block but no title, asking only about the fields it lacks, and the interface's ingest plan says when a file has no title. The viewer's find marks the match inside its line, and the pattern that strips colour before the find (it matched no real escape) is fixed, so a search for `35` no longer hits every coloured line. The review queue's `/` filters and `S` searches, as on every list, and `S` on the sources list searches sources. The `document ingest` help shows the shell loop for a directory of notes.

Concept suggestions. `kb concept suggest` ranks by a damped frequency (DR-0085): numbers and common adverbs no longer head the list, and `change` is no longer grouped with `chance`. The stoplist gains numbers, ordinals and common adverbs, and the frontmatter keyword scorer shares it. Scores are about twenty where they were several hundred.

The setup, update and review knowledge-base skills describe the story verbs, concept kinds, provenance links and `kb stats`, and are pinned to 0.0.25. `make` no longer leaves the website list empty (`HTML_PAGES`), and `publish.bash` stops on a diverged branch.

For packagers and Foul Papers: the library exports `WhoKnows`, `WhoLearns`, `KnownFact`, `WhoKnowsResult`, `Learner`, `ErrAmbiguousScene`, `ParseKnowledgeItem`, `FactKey`, `ParseStoryComment`, `HitProvenance`, `LinkDocumentSource`, `LinkRecordSource`, `ParseSourceRefs` and `SourceCitations`. `KBSearchResult` is no longer comparable with `==`. Requires fountain 1.0.3.

Checked in this release: `go vet` and the test suite pass on Linux (Raspberry Pi OS, arm64). The interface was driven on a pseudo-terminal through the menu, lists, tabs and filter and compared with the previous build, and cross-builds for Linux (amd64, arm64, 32-bit arm), macOS and Windows compile. Not checked: the race detector, a real macOS or Windows terminal, `publish.ps1`, and a manuscript of realistic length (the timeline compares every pair of events; about 14 s for 3000 scenes).


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

