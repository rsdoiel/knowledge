# knowledge User Manual

A standalone SQLite3-backed knowledge base for tracking projects, observations, concepts, decision records and narrative documents across independent experiments. `cmd/kb` is a single `kb VERB ARGS` binary (matching the `git`/`go` command model) covering the full API, plus a read-mostly interactive TUI.

---

## Quick Start

1. **[Installation](INSTALL.md)** — build from source or install a release binary
2. **[Command Reference](kb.1.md)** — the primary `kb(1)` man page: global flags, every verb, exit status

Run `kb` with no verb to launch the interactive TUI, or `kb help` / `kb -h` to see this same reference at the terminal.

---

## Command Reference

`kb` follows the `TOOL VERB PARAMETERS` model — every verb below has its own man page, also reachable via `kb help VERB` or `kb VERB -h`.

| Verb | Man page | Purpose |
|---|---|---|
| `init` | [kb-init(1)](kb-init.1.md) | create a new, empty workspace |
| `project` | [kb-project(1)](kb-project.1.md) | add, list, show, rename projects; set status/description; list a project's concepts |
| `observation` | [kb-observation(1)](kb-observation.1.md) | add, list, show, update observations; list an observation's sources |
| `concept` | [kb-concept(1)](kb-concept.1.md) | add, list, rename concepts — including scholarly identifiers (DOI, ORCID, ROR, Fundref) — and `suggest` candidate new ones from the corpus |
| `link` | [kb-link(1)](kb-link.1.md) | link projects/observations to concepts |
| `source` | [kb-source(1)](kb-source.1.md) | manage cited sources; check DOIs against Retraction Watch |
| `search`, `summary`, `format` | [kb-search(1)](kb-search.1.md) | full-text search (FTS5) and assembled Markdown views |
| `ingest` | [kb-ingest(1)](kb-ingest.1.md) | index a tree of decision records into the database |
| `record` | [kb-record(1)](kb-record.1.md) | author and maintain decision records — new, list, show, set-status, supersede, fmt, concepts |
| `index` | [kb-index(1)](kb-index.1.md) | generate a corpus's `decisions/index.md`; `--check` for staleness, `--all` for a whole tree |
| `document` | [kb-document(1)](kb-document.1.md) | ingest, draft, review and tag narrative documents (Markdown, Fountain, text) at graduated abstraction levels |
| `merge` | [kb-merge(1)](kb-merge.1.md) | reconcile two `knowledge.db` files that drifted independently (e.g. across machines) |
| `export` | [kb-export(1)](kb-export.1.md) | write a portable JSON-L snapshot — the no-file-access alternative to `merge` |
| `import` | [kb-import(1)](kb-import.1.md) | apply a JSON-L snapshot (from `export`) to the database |
| `topics` | [kb-topics(1)](kb-topics.1.md) | list every help topic, one man page per verb |

### Global flags

- `--db PATH` — path to `knowledge.db` (default `./agents/knowledge.db`)
- `--json` — machine-readable output on stdout; errors always go to stderr in both modes, so scripts and other language-model harnesses can drive `kb` directly
- `--debug` — write a JSONL trace of every knowledge-base call (and, in the TUI, every input event and view change) to `./kb-debug-<timestamp>.jsonl`

Full detail on all three: [kb(1)](kb.1.md), § GLOBAL FLAGS.

### Interactive TUI

Bare `kb` (no verb) launches a read-mostly browser: project list → Enter drills into observations → `c`/`o` toggles to/from concepts → `/` opens a search prompt from any view → `esc` backs out, `q` quits. See [kb(1)](kb.1.md) for the full description.

---

## Background & Design

These documents record why `kb` is shaped the way it is — useful if you're extending it, not required to use it:

- **[decisions/index.md](decisions/index.md)** — the decision-record corpus: every architecture/UX decision this module has made, each with its rejected alternatives and the real bugs found along the way. It replaces the old flat `DECISIONS.md`, which was split into records when this module became the format's first conversion pilot; `kb search` reaches the reasoning inside them, not just their titles
- [module-extraction-design.md](module-extraction-design.md) / [-plan.md](module-extraction-plan.md) — pulling this module out of `harvey`
- [cli-tui-design.md](cli-tui-design.md) / [-plan.md](cli-tui-plan.md) — the `kb` CLI and TUI
- [debug-logging-design.md](debug-logging-design.md) / [-plan.md](debug-logging-plan.md) — the `--debug` JSONL trace
- [jsonl-export-design.md](jsonl-export-design.md) / [-plan.md](jsonl-export-plan.md) — the `export`/`import` JSON-L format and identity/conflict rules
- [decision-records-design.md](decision-records-design.md) / [-plan.md](decision-records-plan.md) — `ingest`, `record` and `index`: decision records as first-class rows
- [records-portability-design.md](records-portability-design.md) / [-plan.md](records-portability-plan.md) — carrying records and their relations through `merge`, `export` and `import`
- [wikilink-tagging-design.md](wikilink-tagging-design.md) / [-plan.md](wikilink-tagging-plan.md) — `[[Name]]` inline tagging and frontmatter tags resolving to concepts at ingest time
- [concept-tag-retrieval-design.md](concept-tag-retrieval-design.md) / [-plan.md](concept-tag-retrieval-plan.md) — `MatchConceptNames`/`RecallByConceptNames`, an embedder-free retrieval path for small models
- [narrative-documents-design.md](narrative-documents-design.md) / [-plan.md](narrative-documents-plan.md) — the `documents` entity, graduated abstraction levels, and the summary review workflow

---

## Can't Find What You Need?

- Run `kb help <verb>` or `kb <verb> -h` for the same reference at the terminal
- **[About](about.md)** — project metadata, license, requirements
- **[Getting Help, Reporting Bugs](https://github.com/rsdoiel/knowledge/issues)**
