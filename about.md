---
title: knowledge
abstract: |-
  A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.
authors:
  - family_name: Doiel
    given_name: R. S.
    id: https://orcid.org/0000-0003-0900-6903



repository_code: https://github.com/rsdoiel/knowledge
version: 0.0.20
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

## knowledge 0.0.20

The terminal interface can now do everything the command line does to what you are looking at, behind the same gates. The decisions are DR-0066 to DR-0070 (the menu tree and `:`, the shared chooser, the write classes and the removing gate, plan then apply for ingest, the document summary workflow, and the terminal rule for promote), from the design brief `tui-keys-and-write-flows-design.md` and its plan. Everything was written and tested red first, including the built binary on a pseudo-terminal with real bytes. The race detector passed on darwin/arm64 at the last code commit, and the interface was tried by hand on Linux (Raspberry Pi OS) and macOS. It has not been run on Windows.

Writes. `n` on the project list, an observation or concept tab and the Records screens, and the New rows of the menus, open a form that asks the fields one at a time and ends with the command that does the same and `y`. `s`, `e` and `r` on the project list change a project's status, description and name; `e` on an observation writes a correction (a new observation that supersedes it, as `kb observation update` does) and on a concept renames it; `u` on a record makes it supersede another. `d` deletes a project, observation, concept or record behind a gate that shows what points at it and asks for its name to be typed. Each write is the command's own function, so its rules and refusals are the same, and the screen ends by showing the command that does the same.

Plans and the command line. Ingest, Records, Fuzzy-tag and Documents (Ingest a document, Tag, Fuzzy-tag) show the command's own `--dry-run` output first and apply on `y`; Index and Format files run at once. `:` opens a command line on the menus and browsing screens: any verb, run by the command line's own code, with the gate its write class calls for (reads and direct verbs run, additive and changing writes ask `y`, a delete meets the typed-name gate, plan verbs show their dry run, `merge`, `import`, `init`, `export` and `completion` are refused).

Documents. Documents, Review queue lists the sections waiting for a person; `Enter` writes the summary in `$VISUAL` or `$EDITOR` (the interface is suspended) or in a built-in text area, then shows it above its source, and `y` saves it as written by a person and promotes it. A section that already has a draft starts at that review, so a model's draft is read beside its source, and promoted as it is it keeps its author.

`kb ingest` now says which concepts it created, or with `--dry-run` would create, so a typo in a `[[wikilink]]` no longer adds a concept silently (`concepts_created` in `-json`; `(*KnowledgeBase).HasConcept` in the library).

Building: `cmt` is no longer part of `make`. `make generate` regenerates `version.go`, `about.md`, `CITATION.cff` and the installers from `codemeta.json`, and `make release` runs it, so the version and release hash change only when asked for.

Not in the interface yet: Documents Browse, Documents Frontmatter (`document frontmatter` is reachable through `:`), Concepts Recall and Suggest, Check, and the removing writes `source remove`, `unlink` and `document delete`; `kb verbs --json` does not yet carry the menu fields.

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


