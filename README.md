

# knowledge

A standalone SQLite3-backed knowledge base for tracking projects, observations, and concepts across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (OpenKnowledgeBase, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool (cmd/kbmerge), so other experiments can write structured observations directly through a typed API instead of raw sqlite3 CLI inserts, and harvey becomes a consumer of this module rather than its owner.

## Release Notes

- version: 0.0.0
- status: concept


concept


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

