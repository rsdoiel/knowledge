---
title: knowledge
abstract: |-
  A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.
authors:
  - family_name: Doiel
    given_name: R. S.
    id: https://orcid.org/0000-0003-0900-6903



repository_code: https://github.com/rsdoiel/knowledge
version: 0.0.10
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

## knowledge 0.0.10

Four decision records' worth of work: the project-rename completion this module had refused to do since v0.0.9, an answer to the foreign-ADR question that had been open since 2026-09-15, and two corpus-improvement verbs that surface candidates for a human to confirm rather than committing anything themselves.

`kb project rename OLD NEW` now completes for a project that owns records (DR-0026), lifting v0.0.9's outright refusal. The fix is smaller than either shape originally sketched: `records.project_id` never needs to change on a rename, since it is a stable foreign key and only `projects.name` moves, so the corpus rewrite is a pure file operation -- every owned record's `project:` frontmatter edited in place, both-or-neither, `--dry-run` supported -- and the next ordinary `kb ingest` sees a changed checksum against an unchanged identity, the ordinary UPDATE path rather than the INSERT that collided. Tracing merge and import while designing this turned up two further bugs neither anticipated nor merely suspected: `kb merge` could silently keep a stale name or drop a side outright depending on merge order, because `INSERT OR IGNORE` cannot distinguish a uuid collision from a name collision, and `kb import` hard-failed the *entire* import rather than one row on the same collision. Both are fixed in the same pass -- shipping the rename without them would have made the corruption risk worse by making the trigger routine. Conflict resolution for projects and concepts moves from name-keyed to uuid-primary, reconciling `name` by `updated_at` the same way `description` and `status` already did.

A colleague's MADR-format ADR, pulled from another repository, is modeled as a **document**, not a record (DR-0027). We do not own its status and can never promote or supersede it, and a document is exactly the shape for source material we read and summarise. Answering it that way dissolves the identity collision the question had been stuck on -- both dialects number from 0001, but a document never enters the records identity tuple at all -- so the `dialect` column, the `external` scope value and the `--format madr` adapter are moot rather than deferred. Two real gaps in the stopgap closed with it: `ParseDocumentFile` falls back to a document's first true H1 when frontmatter supplies no `title:` (general, not MADR-specific, but MADR is what surfaced it, having no frontmatter at all), where the document used to be titled with its filename; and document ingest gains density-based concept linking, auto-linking a known concept mentioned more than once outside code spans and fenced blocks. The threshold and the code-span exclusion are not guesses -- a by-hand run against four real foreign ADRs scored roughly 60% signal, and all five of its bad links were the same failure, a short common concept name matching a word used in a different sense inside quoted code or an error string. The accepted cost, stated rather than glossed: a document cannot be cited from `relates_to`, so a record responding to a foreign ADR links it in prose.

`kb concept suggest [--project NAME] [--limit N]` (DR-0028) scans every record body and document section body and prints candidate *new* concepts ranked by corpus-wide distinctiveness -- a TF-IDF shape: occurring several times, confined to relatively few items. It closes the half of corpus improvement that density-linking cannot reach, since `MatchConceptNameCounts` only matches text against concepts that already exist and can never propose one. It never writes to the database. Live-tested against the real workspace corpus it is a roughly-50%-signal mechanical pass -- `rename`, `harvey`, `divergence`, `collision`, `uuid` genuinely useful alongside generic nouns like `row` and `table` -- the same character as DR-0027's own prototype. A human still curates; this does not close the loop on its own.

`kb document tag --project NAME [--concept NAME,...] [--dry-run]` (DR-0029) gives that judgment somewhere to land in the corpus itself. It inserts an explicit `[[Name]]` wikilink at the first safe occurrence of each eligible concept in every already-ingested document for a project, so the next `kb document ingest` links it through the wikilink path that already exists. Eligibility reuses DR-0027's density threshold verbatim with no `--concept` given, or is bypassed for explicitly named ones. It is a minimal position-based edit on the raw file bytes with no parse-and-rerender cycle, excluding frontmatter, fenced blocks, inline code spans, anything already inside `[[...]]`, and -- found live, smoke-testing against a real document -- the document's own first H1, so a concept mention in the title is never wrapped. Writes across a project's document set are both-or-neither, and a second run is a no-op.

One test repair worth naming, because of what it was hiding (DR-0030). `TestParseRecordFile_RoundTripsEveryLiveRecord` named five fixed corpus paths, three of which went stale when DR-0021's `agents/projects/<project>/decisions/` layout was rolled out. It had quietly stopped exercising four fifths of the live corpus -- 54 records of 267 -- and reported that as a count shortfall rather than a discovery failure. Corpora are now discovered rather than listed, using the same two-signal rule `kb index --all` settled on (a record-shaped filename *and* real frontmatter in the file), which also keeps a foreign MADR corpus out of a round-trip test it would fail by construction, and the remembered count floor is gone in favour of the per-file property the test was always really asserting. `user_manual.md`'s verb table, six verbs out of date and pointing at a `DECISIONS.md` removed when this module became the decision-record format's first conversion pilot, is current again.

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


