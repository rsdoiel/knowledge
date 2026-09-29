

# knowledge

A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.

## Release Notes

- version: 0.0.15
- status: concept
- released: 2026-09-29

Three new ways to see what the knowledge base holds, and one to check that it agrees with its JSONL dump. The decisions are DR-0051 to DR-0053. Everything was written and tested red first, and checked by running the v0.0.14 build beside the new one, on a copy of the real workspace, over 415 commands in plain and `--json` mode: the only changed exit codes are the two new verbs' own, and nothing exits 70.

`kb concept show NAME` prints one concept's description and identifier, what links to it (projects, observations, records, document sections) with the full counts, and up to `--limit` items of each kind, newest first (default 10; `--limit 0` gives counts only). The name matches exactly, as `concept delete` does, and a miss offers a case variant. It replaces the raw `sqlite3` joins that judging a `concept suggest` candidate used to need.

`kb concept recall` finds the concepts named in some text (arguments, or stdin with `-`) and lists the observations, records and document sections linked to them, ranked by how many of those concepts each matches and then by recency, with the concepts each hit matched. `--concept NAME,...` names concepts directly, `--project` restricts the hits, `--limit` caps them. Text that matches no concept exits 1. Harvey's memory recall uses the same library call.

`kb record fuzzy-tag --project NAME` reports near-miss spellings of known concepts in decision records (a `detoast` for the concept `toast`), which exact matching never links, and the `tags:` line that would link them. It writes nothing by default. `--write` adds the concept to the `tags:` of `proposed` records only, changes nothing else in the file, and never touches an accepted record; `kb ingest` then links the tags. `--concept NAME,...` widens the named concepts to forms that contain the name.

`kb check-db [--jsonl FILE]` compares the database with the JSONL dump beside it by full content, never by file time, and recommends `kb import`, `kb export`, or both. It is read-only and exits 0 in sync and 1 otherwise. There are no tombstones, so rows only in the dump may be rows deleted here on purpose, and the report says so.

The detection half of `kb merge` is now a read-only library pipeline shared with `check-db`; `merge`'s output is unchanged.

Upgrading: nothing that worked changes, with one exception. A record file that does not parse now exits 65 (wrong content) from `record set-status`, `record supersede` and `project rename` (the verbs that read a record file to edit it) where it exited 2, since the file's content is what is wrong, not the command line. Library additions: `ConceptDetail`, `RecallByText`, `RecallByNames`, `RecordFuzzyReport`, `AddRecordTags`, `CompareToJSONL`, `DiffDatabases`, `PrepareMergeScratch` and `DetectIdentityIssues`.


### Authors

- Doiel, R. S.



## Software Requirements

- Go >= 1.26.4
- gopkg.in/yaml.v3 >= 3.0.1
- github.com/rsdoiel/fountain >= 1.0.2
- git >= 2.0 (runtime, for kb document frontmatter's provenance detection)

### Software Suggestions

- CMTools >= 0.0.45b
- Pandoc >= 3.9
- GNU Make >= 3



## Related resources



- [Getting Help, Reporting bugs](https://github.com/rsdoiel/knowledge/issues)
- [LICENSE](https://www.gnu.org/licenses/agpl-3.0.txt)
- [Installation](INSTALL.md)
- [About](about.md)

