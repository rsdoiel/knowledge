---
title: knowledge
abstract: |-
  A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.
authors:
  - family_name: Doiel
    given_name: R. S.
    id: https://orcid.org/0000-0003-0900-6903



repository_code: https://github.com/rsdoiel/knowledge
version: 0.0.13
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

date_released: 2026-09-24
---

About this software
===================

## knowledge 0.0.13

A sweep for commands that answer a mistake with success. Every verb was run with empty, malformed, unknown and surplus input against a copy of the real database, and each one that exited 0 for input it did not understand, or lost part of it, is fixed, plus a `kb search` bug on hyphenated terms. Several changes are visible to scripts, so read the upgrading notes below. The decisions behind them are DR-0040 to DR-0044.

Usage errors now exit 2 (they exited 1; DR-0040), as `kb -help` always documented: an unknown verb, subverb or flag, a missing or surplus argument, a malformed id or flag value. Exit 1 stays for a command line that was fine and a result that was not: not found, no results, a value the knowledge base rejects, a database error. Unknown flags and surplus arguments are refused instead of dropped (DR-0041): of 25 verbs probed, 15 accepted a bogus flag and 19 ignored an extra argument, so `source add T more words` stored the title `T`, `record new --title A decision` kept only `A`, and `project list --json` printed plain text. A name that begins with a dash now follows `--` (`kb project show -- -name`).

`init`, `index` and `merge` never open the ambient database, and a `--db` given to them was dropped silently (`kb --db rt.db init` created `./agents/knowledge.db`); it is now refused (DR-0041). `record list` validates its filters (DR-0043), where every mistake used to read as `no matching records`: a `--status`, `--kind`, `--trigger`, `--initiative` or `--project` that nothing carries is an error naming what is known, `--since` must be a real date, and `--workspace` with `--project` is refused. The vocabularies are documented, not enforced, so a value outside them that a record really carries still filters.

Project and concept names are trimmed, and a blank name or observation body is refused, in the library as well as the CLI (DR-0042): `kb project add '  '` used to create a project named two spaces. That also closes a corruption: `kb project rename demo ''` on a project that owns records wrote `project: ""` into every record, renamed the row to an empty string, and left the next ingest failing on `UNIQUE constraint failed: records.uuid`. `kb merge` now checks its inputs before opening them (DR-0044). A mistyped `-a` used to merge an empty database, exit 0 and leave a zero-byte file at the typo; a missing, empty or directory input, or `-a` and `-b` naming the same file, is now refused, and a refused merge changes nothing on disk.

`kb VERB SUBVERB -help` prints the verb's page and exits 0 (DR-0041; it printed `flag: help requested`, or looked up a project named `-help`). `kb search` no longer fails on `map-reduce`, `JSON-L` or other hyphenated terms, which FTS5 read as a column filter. A flaky import test, failing about one run in 40 on a one-second clock race in the test rather than in the product, is fixed.

Upgrading: a script that tested for exit 1 to mean a mistyped command should expect 2; one that tested for 1 to mean not found still gets 1. `--json`, `--db` and `--debug` go before the verb and are refused after it. `kb --db PATH init` becomes `kb init PATH`. A typo'd `record list` filter now fails where it printed an empty list. Library callers: `AddProject`, `AddProjectWithStatus`, `AddConcept`, `AddConceptWithIdentifier`, `ResolveConceptName`, `RenameProject`, `RenameProjectRow` and `RenameConcept` trim the name and return an error for a blank one, `AddObservation` and `AddObservationWithSource` return an error for a blank body, and `CleanName` and `DistinctRecordValues` are new. A zero-byte `.db` left by an earlier failed merge can be deleted.

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


