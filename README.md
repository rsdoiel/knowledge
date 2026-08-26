

# knowledge

A standalone SQLite3-backed knowledge base for tracking projects, observations, and concepts across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. As of v0.0.4 it also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Adds a records/record_relations schema, a canonical frontmatter parser and renderer, and the ingest, record and index verbs.

## Release Notes

- version: 0.0.4
- status: concept
- released: 2026-08-26

Adds Decision Record support: episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory, indexed as first-class rows so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads.

Schema: records and record_relations. A record's identity is (workspace, project, scope, id) -- the workspace name is the directory name of the workspace root, derived from the path rather than written in a file, because every workspace has an agents/decisions/ and two of them may each hold a DR-0001. Records are indexed into kb_fts with source_type 'record'. Existing databases migrate lazily on open.

New verbs: `kb ingest PATH` walks a tree of NNNN-slug.md records and resolves supersedes/relates_to in two passes so forward references work; `kb record list|show|new|set-status|supersede|fmt`; and `kb index PATH` generates a decisions/index.md, one greppable line per record, newest first. Ingest never writes to a record file and `record fmt` never writes to the database. The TUI browses a project's records read-only.

New library API: ParseRecordFile/RenderRecordFile, ListRecords, RecordByIdentity, RecordsByRecordID, RecordsUnderPath, NewUUID, Today, and a SourceType field on KBSearchResult so a hit reports which table it came from as well as its own kind.

The frontmatter struct declaration is the canonical format specification -- field order, flow-styled sequences, and a double-quoted string type for the seven fields the format requires quoted -- so every writer produces byte-identical output. Verified against 205 real records across five corpora, all round-tripping byte-for-byte, and `kb index` output is byte-identical to the Deno generator it replaces.

CLI: adds the standard options -help, -license and -version, which kb previously lacked, declared through a flag.FlagSet so each is accepted in either dash form. They are answered before any database is opened, as are ingest's help paths and `kb index`, which builds from the record files and never queries a database.

Vocabularies for record status/kind/trigger are documented and reported against, not enforced: in a format several tools write to, a typo should be a fixable row and not a failed run. Observation kinds remain enforced, which is a deliberate asymmetry.

Security: a record path read from the database is confined to the workspace root before it is read or written, since filepath.Join cleans a path without confining it.

Adds gopkg.in/yaml.v3 as a direct dependency. Man pages for kb-ingest(1), kb-record(1) and kb-index(1).


### Authors

- Doiel, R. S.



## Software Requirements

- Go >= 1.26.3
- gopkg.in/yaml.v3 >= 3.0.1

### Software Suggestions

- CMTools >= 0.0.45b
- Pandoc >= 3.9
- GNU Make >= 3



## Related resources



- [Getting Help, Reporting bugs](https://github.com/rsdoiel/knowledge/issues)
- [LICENSE](https://www.gnu.org/licenses/agpl-3.0.txt)
- [Installation](INSTALL.md)
- [About](about.md)

