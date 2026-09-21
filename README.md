

# knowledge

A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.

## Release Notes

- version: 0.0.11
- status: concept
- released: 2026-09-18

Not yet released -- in progress since v0.0.10.

`kb index ROOT --all` descended into hidden directories, so a git worktree -- which keeps a whole second copy of the tree under `.claude/worktrees/<name>/`, corpus and generated `index.md` included -- was reported as a corpus in its own right (DR-0031). Under `--check` that is a duplicate of a corpus already listed; in write mode `--all` would have rewritten the worktree's copy, editing a throwaway tree instead of the real one. The two-signal rule added in v0.0.9 could not catch it, and correctly so: a worktree copy satisfies both signals, being a byte-identical copy of something that genuinely is a corpus. The walk now prunes any dot-prefixed directory, but never `ROOT` itself, so naming a hidden directory as `ROOT` still finds the corpora inside it. Found by running `--all` live against the real `~/WorkLab`, where it reported 8 corpora where 6 was right -- the same run-it-against-real-data step that caught the original `index.md` false positives in v0.0.9, and the second time it has paid for itself on this one function.


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

