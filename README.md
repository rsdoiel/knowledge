

# knowledge

A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.

## Release Notes

- version: 0.0.16
- status: active
- released: 2026-10-06

One new verb, `kb completion`, so that tab completion works in bash and PowerShell. The decision is DR-0056. It was written and tested red first, and checked by sourcing the script in real bash and PowerShell against the real workspace.

`kb completion bash` and `kb completion powershell` write a completion script to standard output (`pwsh` is accepted for `powershell`). It completes the verbs, the subverbs of `project`, `observation`, `concept`, `source`, `link`, `record` and `document` (and `document review`), each verb's flags once a dash is typed, help topics after `help`, paths after `-db`, `-in`, `-out`, `-a`, `-b`, `-jsonl`, `-root` and `-dir`, and project names after `--project` and after `project show`, `concepts`, `set-status`, `set-description`, `rename` and `delete`. Project names come from `kb project list` in the current directory and honour a `-db` typed earlier on the line.

`kb completion SHELL -install` installs it instead. With bash the script goes to `$XDG_DATA_HOME/bash-completion/completions/kb`, or the same path under `~/.local/share`, and a file that `kb` did not write is never overwritten (exit 73). With PowerShell it writes `kb-completion.ps1` beside the profile and adds one line that dot-sources it, once. Windows PowerShell 5.1 is not covered.

The verb list is read from the verbs `kb` registers, so a new verb is completed without anyone naming it. The subverbs and flags are tables, and the tests fail when a table disagrees with the code, so a new flag or subverb must be added to them. The verb opens no database and refuses `-db`. A bad or missing shell name, a surplus argument and an unknown flag exit 2.

Upgrading: nothing that worked changes. `kb completion` did not exist before, so a script that called it got exit 2 (unknown verb).


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

