# knowledge User Manual

A standalone SQLite3-backed knowledge base for tracking projects, observations, and concepts across independent experiments. `cmd/kb` is a single `kb VERB ARGS` binary (matching the `git`/`go` command model) covering the full API, plus a read-mostly interactive TUI.

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
| `project` | [kb-project(1)](kb-project.1.md) | add, list, show projects; list a project's concepts |
| `observation` | [kb-observation(1)](kb-observation.1.md) | add, list, show observations; list an observation's sources |
| `concept` | [kb-concept(1)](kb-concept.1.md) | add, list concepts — including scholarly identifiers (DOI, ORCID, ROR, Fundref) |
| `link` | [kb-link(1)](kb-link.1.md) | link projects/observations to concepts |
| `source` | [kb-source(1)](kb-source.1.md) | manage cited sources; check DOIs against Retraction Watch |
| `search`, `summary`, `format` | [kb-search(1)](kb-search.1.md) | full-text search (FTS5) and assembled Markdown views |
| `merge` | [kb-merge(1)](kb-merge.1.md) | reconcile two `knowledge.db` files that drifted independently (e.g. across machines) |

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

- **[DECISIONS.md](DECISIONS.md)** — architecture/UX decision log: the module extraction, the CLI/TUI design, and the `--debug` tracing feature, each with rejected alternatives and real bugs found along the way
- [module-extraction-design.md](module-extraction-design.md) / [-plan.md](module-extraction-plan.md) — pulling this module out of `harvey`
- [cli-tui-design.md](cli-tui-design.md) / [-plan.md](cli-tui-plan.md) — the `kb` CLI and TUI
- [debug-logging-design.md](debug-logging-design.md) / [-plan.md](debug-logging-plan.md) — the `--debug` JSONL trace

---

## Can't Find What You Need?

- Run `kb help <verb>` or `kb <verb> -h` for the same reference at the terminal
- **[About](about.md)** — project metadata, license, requirements
- **[Getting Help, Reporting Bugs](https://github.com/rsdoiel/knowledge/issues)**
