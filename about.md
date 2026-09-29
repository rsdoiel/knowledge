---
title: knowledge
abstract: |-
  A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.
authors:
  - family_name: Doiel
    given_name: R. S.
    id: https://orcid.org/0000-0003-0900-6903



repository_code: https://github.com/rsdoiel/knowledge
version: 0.0.15
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

date_released: 2026-09-29
---

About this software
===================

## knowledge 0.0.15

Three new ways to see what the knowledge base holds, and one to check that it agrees with its JSONL dump. The decisions are DR-0051 to DR-0055. Everything was written and tested red first. The new verbs were checked by running the v0.0.14 build beside the new one, on a copy of the real workspace, over 415 commands in plain and `--json` mode: the only changed exit codes were the two new verbs' own, and nothing exited 70. That comparison was made before the two fixes described last, which change behaviour on purpose.

`kb concept show NAME` prints one concept's description and identifier, what links to it (projects, observations, records, document sections) with the full counts, and up to `--limit` items of each kind, newest first (default 10; `--limit 0` gives counts only). The name matches exactly, as `concept delete` does, and a miss offers a case variant. It replaces the raw `sqlite3` joins that judging a `concept suggest` candidate used to need.

`kb concept recall` finds the concepts named in some text (arguments, or stdin with `-`) and lists the observations, records and document sections linked to them, ranked by how many of those concepts each matches and then by recency, with the concepts each hit matched. `--concept NAME,...` names concepts directly, `--project` restricts the hits, `--limit` caps them. Text that matches no concept exits 1. Harvey's memory recall uses the same library call.

`kb record fuzzy-tag --project NAME` reports near-miss spellings of known concepts in decision records (a `detoast` for the concept `toast`), which exact matching never links, and the `tags:` line that would link them. It writes nothing by default. `--write` adds the concept to the `tags:` of `proposed` records only, changes nothing else in the file, and never touches an accepted record; `kb ingest` then links the tags. `--concept NAME,...` widens the named concepts to forms that contain the name.

`kb check-db [--jsonl FILE]` compares the database with the JSONL dump beside it by full content, never by file time, and recommends `kb import`, `kb export`, or both. It is read-only and exits 0 in sync and 1 otherwise. There are no tombstones, so rows only in the dump may be rows deleted here on purpose, and the report says so.

The detection half of `kb merge` is now a read-only library pipeline shared with `check-db`; `merge`'s output is unchanged.

Two fixes. `kb import` no longer rewrites an observation kind outside `note`, `finding`, `decision`, `question` and `hypothesis` to `note` (DR-0054): it keeps the kind and prints a `warning:` line, or a `Warnings` list under `--json`, and still exits 0. The rewrite made `kb check-db` report a false divergence, and following its advice to import and export would have written the changed kind into the authoritative dump. `kb project add` (DR-0055) no longer prints "added" for a name that already exists: it exits 1 with `already exists`, as `project rename` does, and writes nothing (it used to move `updated_at`, which merge uses to pick a winner, and drop the status and description you gave it). A `--status` typed after NAME is a usage error that says the flag goes before NAME, where it used to become part of the description; `--` before NAME keeps a flag-looking word as text.

Upgrading: two things that worked change. `kb project add` on an existing name now exits 1, where it exited 0, so a script that repeated the call must expect that; the library's `AddProject` still returns the existing id. And a record file that does not parse now exits 65 (wrong content) from `record set-status`, `record supersede` and `project rename` (the verbs that read a record file to edit it) where it exited 2, since the file's content is what is wrong, not the command line. Library additions: `ConceptDetail`, `RecallByText`, `RecallByNames`, `RecordFuzzyReport`, `AddRecordTags`, `CompareToJSONL`, `DiffDatabases`, `PrepareMergeScratch` and `DetectIdentityIssues`; `ImportTableSummary` gains a `Warnings` field.

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


