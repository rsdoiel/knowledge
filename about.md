---
title: knowledge
abstract: |-
  A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.
authors:
  - family_name: Doiel
    given_name: R. S.
    id: https://orcid.org/0000-0003-0900-6903



repository_code: https://github.com/rsdoiel/knowledge
version: 0.0.22
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

date_released: 2026-10-14
---

About this software
===================

## knowledge 0.0.22

Search is scoped by a path, in the command and in the interface, `/` now filters every list, and every row of the interface's menus is built. The decisions are DR-0072 to DR-0074.

Searching. `kb search harvey/ "exit codes"` searches one project, `harvey/records` one kind in it, `workspace/` the workspace tier only and `/records` one kind everywhere; `kb search TERM` is unchanged, and a first word with no slash is still part of the term. Results are grouped by kind with counts, name a record by its qualified reference, show the projects a concept is in, and say `showing 50 of N` when they were cut. An unknown scope or kind is exit 2; `workspace/observations` and `harvey/projects` are exit 1 with the reason. In the interface, Search is a menu of scopes and a prompt that takes the same line, `S` opens it from any screen with the scope of where you are, and `kb search` on a terminal opens it.

Filtering. `/` narrows the list in front of you as you type, on every list; Enter keeps the filter and Esc clears it. `/` no longer starts the search, `S` does.

The interface. Check, the Browse lists, Recall, Suggest, Check retractions, Frontmatter (a checklist, written only on `y`), Enter on a search result, `d` to delete a concept, source or document, and `u` to unlink a concept from a project or cut one link of an observation are all on screens now, each with the typed or keyed confirmation its class needs and the command that does the same. `kb --json verbs` carries the menu fields.

Changed. `source check-retractions` is classed as a changing write, so `:` asks first; on the command line nothing changes.

For packagers and harvey: the library gains `SearchScoped`, `SearchKinds`, `ErrNoMembers` and `ObservationConcepts`, and `KBSearchResult` carries `SourceID` and `ProjectID`; `Search` is unchanged.

Not checked in this release: the race detector was not run (the machine it runs on was not available), and the new screens were not tried by hand on macOS or Windows. The test suite passes on Linux (Raspberry Pi OS, arm64), and the built binary was driven on a pseudo-terminal there for the filter and the search prompt.

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


