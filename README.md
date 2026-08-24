

# knowledge

A standalone SQLite3-backed knowledge base for tracking projects, observations, and concepts across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts.

## Release Notes

- version: 0.0.3
- status: concept
- released: 2026-08-08

Adds JSON-L export/import: ExportJSONL/ImportJSONL in the knowledge package, plus `kb export [-project NAME] [-out PATH]` and `kb import [-in PATH]` verbs. A portable, no-file-access alternative to merge -- the resulting file can be pasted, emailed, or committed to git, then applied elsewhere with import. Projects/concepts are matched by name (existing local rows win), sources by identifier, observations and links by uuid, so re-importing the same file is a no-op.


### Authors

- Doiel, R. S.



## Software Requirements

- Go >= 1.26.3

### Software Suggestions

- CMTools >= 0.0.45b
- Pandoc >= 3.9
- GNU Make >= 3



## Related resources



- [Getting Help, Reporting bugs](https://github.com/rsdoiel/knowledge/issues)
- [LICENSE](https://www.gnu.org/licenses/agpl-3.0.txt)
- [Installation](INSTALL.md)
- [About](about.md)

