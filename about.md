---
title: knowledge
abstract: |-
  A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.
authors:
  - family_name: Doiel
    given_name: R. S.
    id: https://orcid.org/0000-0003-0900-6903



repository_code: https://github.com/rsdoiel/knowledge
version: 0.0.21
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

date_released: 2026-10-09
---

About this software
===================

## knowledge 0.0.21

Records are now read in kb's own viewer, with Markdown highlighting, and the pager no longer depends on which programs happen to be installed. The decision is DR-0071 (a partial supersede of DR-0066 point 5).

Reading. Enter on a record in the terminal interface used to return straight to the list when `bat` was installed: `bat` quits at once when the text fits one screen and the interface repainted over it. kb now shows a record itself. YAML frontmatter, headings, fenced code, list markers, quotes, inline code, bold and links are coloured with the terminal's own 16 colours; `NO_COLOR` or `TERM=dumb` turns it off. In the viewer `/` finds text (case-insensitive), `n` and `N` move between matches, `g` and `G` jump to the ends. `kb record set-status REF` with no status prints the same highlighted record on a terminal. `bat`, `batcat` and `less` are no longer looked for; `$KB_PAGER` is the override and is run exactly as written (give `bat` `--paging=always`).

Fixed. `kb help ingest` showed `kb ingest clasm/decisions`, which exits 66 from the workspace root: `PATH` is a directory relative to the current directory, one only, walked recursively, and the project and tier come from each record's `project:` frontmatter. The text now says so and the examples work; the `kb index` and `kb record fmt` examples are corrected too. `kb ingest`'s "missing" line said kb had no verb to remove a row; it now names `kb record delete REF`. The `kb-index(1)` page no longer points at the developers' `TODO.md`.

For packagers: `github.com/muesli/termenv` is now a direct dependency (it was already built in). The race detector passed on darwin/arm64 at the last code commit. The viewer was driven on a pseudo-terminal against the built binary on Linux (Raspberry Pi OS), and tried by hand on macOS and Windows, where it worked.

Not in the interface yet: Documents Browse and Frontmatter (frontmatter is reachable through `:`), Concepts Recall and Suggest, Check, and the removing writes `source remove`, `unlink` and `document delete`; `kb verbs --json` does not yet carry the menu fields.

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


