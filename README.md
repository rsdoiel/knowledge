

# knowledge

A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.

## Release Notes

- version: 0.0.7
- status: concept
- released: 2026-09-15

A bug-fix release: four defects found running v0.0.6 against real corpora (clasm, WorkLab, caltechauthors), all in `kb ingest`'s re-run behavior plus `kb search`'s exit code.

`kb ingest` no longer leaves stale edges behind on re-ingest. A `relates_to`/`supersedes` entry removed from a record's frontmatter, or a `[[wikilink]]` concept tag removed from its body, is now actually removed from `record_relations`/`record_concepts` -- previously both only ever grew, since re-ingest inserted what a file currently declared but never deleted what it no longer declared. The new `ClearRecordRelationsFrom`/`ClearRecordConcepts` (and `ClearDocumentSectionConcepts` for `kb document ingest`) run before every re-insert.

`[[0007]]`/`[[DR-0007]]` in a record body -- the natural way to write "see DR-0007" -- no longer silently mints a junk concept named after the record id. It is now skipped with a warning pointing at `supersedes`/`relates_to`, the actual way to cite another record.

`kb ingest` now updates a record's stored `path` when its file moves but its content is unchanged, rather than leaving `records.path` (and so `kb export`'s `agents/knowledge.jsonl`) silently stale. The "DR-%s was stored at %s" warning now fires only when content changes alongside the path, since a path change alone is an ordinary move, not a possible id collision. The message for a record with no file at its stored path no longer asserts deletion as the only explanation and recommends `kb record remove` outright -- a moved file looks identical to a deleted one, and the old wording would have walked a user into deleting live records.

`kb search` now exits 1, in both text and `--json` mode, when it finds nothing, matching the workspace's search-tool convention instead of exiting 0 with an empty result.


### Authors

- Doiel, R. S.



## Software Requirements

- Go >= 1.26.4
- gopkg.in/yaml.v3 >= 3.0.1
- github.com/rsdoiel/fountain >= 1.0.2

### Software Suggestions

- CMTools >= 0.0.45b
- Pandoc >= 3.9
- GNU Make >= 3



## Related resources



- [Getting Help, Reporting bugs](https://github.com/rsdoiel/knowledge/issues)
- [LICENSE](https://www.gnu.org/licenses/agpl-3.0.txt)
- [Installation](INSTALL.md)
- [About](about.md)

