---
title: knowledge
abstract: |-
  A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.
authors:
  - family_name: Doiel
    given_name: R. S.
    id: https://orcid.org/0000-0003-0900-6903



repository_code: https://github.com/rsdoiel/knowledge
version: 0.0.11
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

## knowledge 0.0.11

Three new corpus-improvement features, plus a search bug fix -- fuzzy near-miss tagging, a frontmatter provenance generator, and fuzzy candidate clustering, each of which surfaced and fixed a real self-contradiction in its own design doc before shipping, verified by hand rather than trusting the design's own worked examples.

`kb document fuzzy-tag --project NAME [--concept NAME,...] [--dry-run]` (DR-0032) catches what `tag`'s exact whole-word matching can't: a typo, plural, or simple tense variant of a known concept's name. Inserts a footnote marker at the near-miss plus a footnote definition carrying the canonical `[[Concept]]`, never bracket-wrapping the near-miss text itself, so `ResolveConceptName` can't mint a duplicate concept from a misspelled or inflected form. A follow-up review caught and fixed a real corruption bug before release: two near-misses in the same section could splice a later footnote marker inside an earlier one's own definition text.

`kb document frontmatter PATH [--accept FIELD,...] [--accept-keywords NAME,...] [--set FIELD=VALUE] [--dry-run]` (DR-0033) proposes `title`/`author`/`dateCreated`/`dateModified`/`keywords` from a document's own prose and git/filesystem provenance -- this module's first `exec.Command` dependency. `author` is a layered signal (a byline in the prose, then git's earliest-commit author, then `git config user.name`), shown together when they disagree rather than collapsed to one guess. A follow-up review caught and fixed two bugs: an explicitly-empty frontmatter value locked a field from ever being filled in, and `--set FIELD=` silently no-op'd instead of writing an explicit empty value.

Fuzzy candidate clustering for `kb concept suggest` (DR-0034) merges spelling variants of the same underlying term (`chunking`/`chunkings`/`chunked`) into one candidate before scoring, not after, so a signal split across variants no longer falls individually below the occurrence/distinctiveness floor. A candidate fuzzy-close to an already-known concept is excluded from candidacy entirely and reported separately. `--json`'s shape changes from a bare array to `{"candidates": [...], "near_existing": [...]}` -- a breaking change for any existing consumer. Live-smoke-tested against the real `agents/knowledge.db`, which found a flat distance-1 threshold with no length floor produced heavy false-positive clustering on short common words, fixed with a length gate.

`kb search TERM` threw a raw SQLite error instead of a normal no-results outcome for any term containing bare punctuation FTS5's own query grammar treats as significant (a `.` in a version string, the case found). `Search` now retries once, quoting the term as a phrase, but only when the first attempt fails with an FTS5 syntax error specifically, so documented raw FTS5 query syntax (multi-word AND, quoted phrases, `prefix*`) keeps working unchanged.

Also includes `kb index ROOT --all`'s hidden-directory pruning fix (DR-0031), shipped just after v0.0.10.

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


