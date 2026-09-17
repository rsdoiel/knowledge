---
title: knowledge
abstract: |-
  A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.
authors:
  - family_name: Doiel
    given_name: R. S.
    id: https://orcid.org/0000-0003-0900-6903



repository_code: https://github.com/rsdoiel/knowledge
version: 0.0.9
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

date_released: 2026-09-17
---

About this software
===================

## knowledge 0.0.9

Rename verbs for projects and concepts, cross-machine reconciliation for their mutable fields, and a multi-corpus mode for index.md -- three items closed out of TODO.md, two of them correcting their own original diagnosis along the way.

`kb project rename OLD NEW` and `kb concept rename OLD NEW` (DR-0024) replace the only prior route, raw SQL against `projects.name`/`concepts.name`. `project rename` refuses if `NEW` already exists or if the project owns any records: a decision record's `project:` frontmatter has to match the project's name, and a live repro showed that renaming a project with a corpus and then re-ingesting it silently mints a phantom project under the old name and duplicates the record under it -- data corruption, not just a stale search label. `concept rename` has no such corpus to desync, since every link to a concept is a foreign key, and ships unconditionally beyond the name collision every rename needs. Both reuse the existing `refreshProjectFTS`/a new `refreshConceptFTS` so search never lags a rename.

Cross-machine reconciliation (DR-0025) gives `kb merge` and `kb import` a real last-writer-wins policy for a project or concept's mutable fields, replacing a conflict rule that was really just loop order: `INSERT OR IGNORE` silently kept whichever side was applied first regardless of which edit was newer. `concepts` gains `updated_at` (lazily migrated, mirroring how `created_at` was itself added); `merge`'s conflict path becomes `INSERT OR IGNORE` for new rows plus a guarded `UPDATE ... FROM ... WHERE incoming.updated_at > existing.updated_at` for existing ones -- two statements rather than one upsert, because this project's pure-Go SQLite driver rejects an UPSERT clause after a SELECT-form INSERT, confirmed live. `importProject`/`importConcept` gained the matching comparison, and `RenameProject`/`RenameConcept`/`AddConceptWithIdentifier`'s `ON CONFLICT` branch now touch `updated_at`, closing gaps DR-0024 and DR-0012 left open now that the column is compared for real. Observations needed none of this: DR-0023 already resolved observation correction via supersession, retiring the question before this record was even picked up.

`kb index ROOT --all [--check]` closes the index-staleness item's multi-corpus half -- `--check` and the `set-status`/`supersede` auto-refresh, shipped last release, only ever covered one corpus per invocation. `--all` discovers every corpus under `ROOT` that already has an `index.md`, refreshes or checks each one, keeps going past an individual corpus's failure, and exits non-zero if any needed attention. Running it live against the real workspace caught a real bug before it shipped: matching on the filename `index.md` alone picked up several files with nothing to do with `kb` (a docs-site front page, a blog index) and would have silently overwritten them in write mode. Fixed by requiring both a content-heading match and an actual record file in the directory.

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


## Software Suggestions

- CMTools >= 0.0.45b
- Pandoc >= 3.9
- GNU Make >= 3


