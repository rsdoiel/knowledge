

# knowledge

A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.

## Release Notes

- version: 0.0.14
- status: concept
- released: 2026-09-25

`kb` now says what kind of failure it was from the exit number alone, and a batch of inputs it used to accept without a word are refused. Read the upgrading notes below before updating a script. The exit codes follow one convention for every command-line tool in the workspace (workspace DR-0003); the decisions for `kb` are DR-0045 to DR-0050. The whole change was checked by running the v0.0.13 build beside the new one, on a copy of the real database, over 362 commands in plain and `--json` mode.

Exit 1 is now the normal "no" (not found, no results, a stale index, an operation the current state forbids), as it is for `grep` and `diff`; 2 is a usage error; and the `sysexits(3)` numbers above that say what went wrong: 65 wrong content (a malformed record or JSONL, a file that is not a knowledge base, a merge collision), 66 a missing input or workspace, 69 an unreachable service, 70 an internal error, 73 an output that cannot be created, 74 an I/O error, 75 a locked database, 77 permission denied. An error nothing classified is 70, never 1. The `--json` error object carries the class and the number.

Commands that work through many items do all they can and then exit with the class of the first failure, instead of 0: `kb ingest` (a record that does not parse, two files claiming one identity), `index --all`, and `source check-retractions` (69; it also no longer marks an unreachable source as checked).

Mistakes that used to succeed are refused: `source retract` or `remove` on an id that does not exist; `source add` with a blank title, a bad `--published`, `--url` or `--doi` (a mistyped DOI read as "not retracted"); `record new` or `record set-status` with a trigger, kind or status outside the vocabulary that no record carries; `kb init --bogus`, which created a directory named `--bogus`; `kb search --json foo`, which searched for that text; a negative `concept suggest --limit`.

Project and concept names are one line: interior whitespace collapses and control characters are refused, so a hard-wrapped `[[wikilink]]` is the ordinary concept instead of a second one with a newline in its name (DR-0046). `--` now works on every verb that takes a name, title or path, and `kb merge` refuses a zero-byte input even beside a `-wal`.

New commands remove what could only be removed with raw SQL, which skips the search index (DR-0050): `kb project delete` (a stray empty project; it never cascades into content), `kb observation delete`, `kb document delete` (refused while a reviewed summary would be lost) and `kb record delete` (only once the record's file is gone), and `kb unlink` removes one project-concept, observation-concept or observation-source link. Each refuses while something depends on it, says what and how many, takes `--force` where that makes sense and `--dry-run` to preview. A delete is local to one database: `kb merge` and `kb import` from a database that still has the row bring it back.

Upgrading: a script that treated any non-zero as failure is unaffected. One that treated 1 as "the command failed" must also handle 2 and the numbers above it; the table in CHANGES.md lists every changed status. The three knowledge skills in `agents/skills` branch on the new codes.


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

