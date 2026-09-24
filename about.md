---
title: knowledge
abstract: |-
  A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.
authors:
  - family_name: Doiel
    given_name: R. S.
    id: https://orcid.org/0000-0003-0900-6903



repository_code: https://github.com/rsdoiel/knowledge
version: 0.0.12
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

date_released: 2026-09-18
---

About this software
===================

## knowledge 0.0.12

The corpus-improvement toolchain moves out of `cmd/kb` and into the importable `knowledge` package, so another program can drive it in-process instead of shelling out to `kb`, plus a concept delete verb, a `cancelled` record status, and a sweep of bugs found by running a build from before each change against the new one on real data.

The workflow logic is now library API (DR-0035): `IngestDocument`, `TagDocumentText`/`FuzzyTagDocumentText`/`FuzzyEligible`, `SuggestConcepts`/`CandidateTerms`, and `ProposeFrontmatter`/`ApplyFrontmatter`. Each takes text or bytes and returns a result, never touching a file, so a consumer's own permission layer stays in charge of its writes. Git access for frontmatter sits behind a `Provenance` interface (`GitProvenance` by default), so a caller can supply its own authorship and dates. The `kb` verbs are now flag parsing, one library call and rendering; their output and `--json` shapes are unchanged. Record ingest (`kb ingest`) stays in `cmd/kb`.

`kb concept delete NAME [--force] [--dry-run]` (DR-0039) removes a concept, its links and its search entry. A concept still linked to a project, observation, record or document section is refused, with the counts, unless `--force`, which removes only the links. Deletion is local to one database, with no tombstone: a `merge` or `import` from a database that still has the concept brings it back, and a file still naming it recreates it when next ingested after it changes. The command and `kb-concept(1)` say so. `cancelled` joins the record status vocabulary (DR-0038): adopted, then abandoned, distinct from `rejected` and `superseded`, with the reason in the record's body.

Concept names that begin or end in punctuation (`C++`, `F#`, `.NET`) now match a mention; before, `\b` could never fire after the `+`, and `.NET` wrongly matched inside `ASP.NET`. A name with no letter or digit never matches. A wikilink inside a code span or fenced block no longer mints or links a concept, in document or record ingest: over 159 real documents ingest went from 213 concepts to 147, and all 66 that stopped were examples inside code (DR-0037).

`document fuzzy-tag` proposes far fewer false matches (DR-0036): a concept shorter than 6 letters is no longer fuzzy-matched without `--concept`, and a distance-2 match needs a 6-letter shared prefix, which took 614 proposals over the real corpus to 224 and removed every measured false positive (`fts` for `its`, `retirement` for `requirement`). `concept suggest` requires a shared first letter before treating two terms as variants. `document frontmatter --accept-keywords` accepts only a known concept or a proposed candidate, checked before anything is written, and no longer creates concepts under `--dry-run`.

Output that changed between identical runs is now deterministic (DR-0037): the order of frontmatter keys written by `--accept`/`--set`, known-concept proposals, removed headings after a re-ingest, and `project rename`'s notes. A pre-fix build gave 12 different outputs in 12 runs of one script; this one gives one.

Upgrading: a database ingested with v0.0.11 may hold junk concepts minted from wikilink examples in code (`...`, `recall: ...`), and some of them can now match; find them with `kb concept list` and remove them with `kb concept delete`. A short concept's plural (`merge` for `merges`) now needs `--concept` to be footnoted by `fuzzy-tag`.

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


