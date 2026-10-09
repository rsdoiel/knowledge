

# knowledge

A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records, and narrative documents across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts. It also indexes Decision Records -- episode-scoped Markdown files with YAML frontmatter, kept in a project's decisions/ directory -- as first-class rows, so the reasoning behind a decision is retrievable rather than living only in files the knowledge base never reads. Inline [[wikilink]] tagging and frontmatter tags/keywords resolve to concepts at ingest time (records and documents alike), an embedder-free RecallByConceptNames/MatchConceptNames pair gives small/CPU-only models a cheap concept-based retrieval path ahead of any vector RAG, and a documents/document_sections schema ingests narratives and articles (Markdown, Fountain, plain text) at graduated abstraction levels -- a document-level gist plus per-section summaries, each going through an explicit human review step before being trusted as searchable content.

## Deciding on a record

A decision record is written `proposed` and a person decides it. `kb` enforces that from v0.0.18:

```shell
kb record pending harvey              # what is waiting for a decision
kb record set-status harvey/DR-0004   # shows the record, offers the moves, asks before it writes
```

`set-status` with no status needs a terminal. It shows the record through `$KB_PAGER` (else `bat -l markdown`, else `less -R`), then prompts with the moves the record's current status allows, for example `harvey/DR-0004 is proposed -> [a]ccepted [r]ejected [c]ancelled [q]uit`. A status is chosen by pressing its first letter, at once, and `y` confirms before anything is written. `Esc`, `q` or `Ctrl-C` backs out at either step, as does `n` at the confirmation, with no Enter needed; `Enter` does nothing. Backing out changes nothing and exits 1.

The moves are a table: `proposed` to `accepted`, `rejected`, `cancelled` or `superseded`; `accepted` to `cancelled` or `superseded`; `rejected` or `cancelled` back to `proposed`; `superseded` is final and needs `superseded_by` (use `kb record supersede`). `rejected` means considered and not pursued; `cancelled` means pursued or explored and then abandoned. Scripts use the direct form, `kb record set-status REF STATUS`, which is held to the same table. `kb record set-status REF accepted` exits 2 unless standard input and output are a terminal, and nothing turns that off, so a script or a model can propose, reject, cancel and supersede but not accept. See [kb-record(1)](kb-record.1.md).

## Release Notes

- version: 0.0.18
- status: active
- released: 2026-10-09

Accepting a record is now the author's act, and the tool enforces it. The decisions are DR-0060 (set-status reviews the record, then asks), DR-0061 (accepting needs a terminal) and DR-0063 (cancelled covers explored work), all from the review-then-decide design. Everything was written and tested red first, including the built binary on a pseudo-terminal and with redirected streams.

`kb record set-status` follows a table of allowed moves. A record that is `proposed` may become `accepted`, `rejected`, `cancelled` or `superseded`; `accepted` may become `cancelled` or `superseded`; `rejected` and `cancelled` may go back to `proposed`; `superseded` is final, and needs `superseded_by` already set (use `kb record supersede`). A move outside the table, or setting the status a record already has, exits 1 and names the moves it may make; nothing is written. A record whose status is outside the vocabulary can only become `proposed`. `cancelled` now means pursued or explored and then abandoned, and need not have been accepted; `rejected` means considered and not pursued.

`kb record set-status REF accepted` exits 2 unless standard input and standard output are both a terminal, with a message that a person must accept it. No flag or environment variable turns that off, and every other move works without a terminal, so a script can propose, reject, cancel and supersede but not accept. The test asks the terminal driver, so `< /dev/null` does not count. It guards the accidental path, not a determined one: a process can allocate a pseudo-terminal, and a record file can be edited by hand.

`kb record set-status REF` with no status is a review for a person at a terminal. It shows the record through `$KB_PAGER` (else `bat -l markdown`, else `less -R`, else prints it), prompts with the current status and the moves allowed (`harvey/DR-0004 is proposed -> [a]ccepted [r]ejected [c]ancelled [q]uit`), and asks `[y/N]` before it writes. `q`, an empty or negative answer, or end of input leaves the record unchanged and exits 1. `KB_PAGER` is new. The library exports `AllowedTransitions` and `CanTransition`, so harvey and the TUI read the same table.

Upgrading: a script that ran `set-status ... accepted` must hand that step to the author, and one that relied on arbitrary moves (for example `accepted` to `rejected`; use `cancelled`) now gets exit 1. Skills that tell a model to promote a record need updating; the ones in the Laboratory do not any longer. `github.com/charmbracelet/x/term` and `golang.org/x/sys` are direct requirements, both already built into kb through bubbletea. The generated `kb-*.1.md` pages are now checked against their help text, so an edit made only to a page fails the tests.


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

