---
title: knowledge
abstract: |-
  A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.
authors:
  - family_name: Doiel
    given_name: R. S.
    id: https://orcid.org/0000-0003-0900-6903



repository_code: https://github.com/rsdoiel/knowledge
version: 0.0.23
license_url: https://www.gnu.org/licenses/agpl-3.0.txt

programming_language:
  - Go >= 1.26.4

keywords:
  - SQLite3
  - knowledge base
  - observations
  - concepts
  - projects
  - cross-machine sync
  - CLI
  - TUI
  - decision records
  - ADR
  - provenance
  - full-text search
  - wikilinks
  - retrieval-augmented generation
  - narrative documents

date_released: 2026-10-16
---

About this software
===================

## knowledge 0.0.23

Notes get frontmatter at ingest, search hits say where they are, `kb stats` shows a database's shape, and the knowledge skills and an agent document now ship in this repository. The decisions are DR-0076 and DR-0077 (both proposed).

Notes. `kb document ingest PATH --project P`, on a terminal, offers frontmatter for a file that has none: `title`, `author`, `dateCreated` and the known concepts the text mentions as `keywords`, one at a time (Enter accepts, text replaces, `-` skips, `q` stops), written only after you confirm. `--no-prompt`, `--dry-run`, `--json` and a missing terminal never prompt. A date at the start of a file name, `2026-09-29-worklog.md`, is now the first `dateCreated` signal, here and in `kb document frontmatter`.

Searching. A hit now carries a record's qualified reference and status (`harvey/DR-0004 (accepted)`) and the passage that matched, the matched words between « and », with the section it is in for a record (`Decision:`, or `title:`). This is in `kb search TERM` as well as the scoped form, and in `--json`.

Counting. `kb stats` reports rows per table, links per join table, five checks (`observations_without_project`, `concepts_without_links` and others) and a line per project, with no SQL; `--json` is one object. `kb concept show` says which projects a concept spans, tagged directly or through observations, records and document sections.

Completion. `kb completion bash -install` now also adds a line to the bash startup file (`~/.bashrc`, or on macOS the first of `~/.bash_profile`, `~/.bash_login` and `~/.profile` that exists), because macOS's bash 3.2 never reads the bash-completion directory, so completion did nothing in a new Terminal window. `-uninstall` removes what `-install` wrote, and `-check` says what is missing. Use `eval "$(kb completion bash)"` to try it in a session: `source <(...)` silently does nothing in bash 3.2.

Fixed. `kb ingest` on a project directory failed on dated notes (`notes/2026-09-29-....md`, "no frontmatter"): the year read as a record id. A record is `NNNN-slug.md` and not `YYYY-MM-DD...`, and the walk skips `design`, `plans`, `feature_requests`, `notes` and `hand-off` below the directory you give it. The closing `kb: ingest: N error(s)` line no longer repeats the first error.

Shipping. `agents/skills/` holds `setup-knowledge-base`, `update-knowledge-base` and `review-knowledge-base`, and `AGENTS.md` (with a `CLAUDE.md` that points at it) tells a harness what kb is, the rules for using it, the verbs and the exit codes. The release zips include them. `make install-skills` copies the skills to `~/.claude/skills` (`SKILLS_DIR` changes it) and `make check-skills` compares; `make install` does not install them. A test fails if a skill's `kb_version` is not the release's.

For packagers and harvey: the library gains `Stats`, `StatCount`, `ProjectStats`, `ConceptSpan` and `ConceptDetailResult.Spans`, and `KBSearchResult` carries `Ref`, `Status`, `Passage` and `Section`; `Search` and `SearchScoped` take what they took.

Checked in this release: the race detector passed on darwin/arm64 (macOS), and the test suite passes on Linux (Raspberry Pi OS, arm64). The built binary was driven on a pseudo-terminal on Linux for the frontmatter prompts, and on macOS for `completion -install`, `-check` and `-uninstall` under /bin/bash 3.2 and for the review skill. Not checked: the frontmatter prompts by hand on macOS or Windows, and `completion` on Windows.

## Authors

- [R. S. Doiel](https://orcid.org/0000-0003-0900-6903)






A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.

- [License](https://www.gnu.org/licenses/agpl-3.0.txt)
- [Code Repository](https://github.com/rsdoiel/knowledge)
  - [Issue Tracker](https://github.com/rsdoiel/knowledge/issues)

## Programming languages

- Go >= 1.26.4




## Software Requirements

- Go >= 1.26.4
- gopkg.in/yaml.v3 >= 3.0.1
- github.com/rsdoiel/fountain >= 1.0.2
- git >= 2.0 (runtime, for kb document frontmatter's provenance detection)


## Software Suggestions

- CMTools >= 0.0.45b
- Pandoc >= 3.9
- GNU Make >= 3


