# `kb` — CLI and TUI — Implementation Plan

See [cli-tui-design.md](cli-tui-design.md) for the full audit and
confirmed decisions. This covers the phased build: `busy_timeout` fix,
`cmd/kb` scaffold, verb groups, the folded-in `merge` verb (retiring
`cmd/kbmerge`), JSON output, then the TUI.

Work items ordered W1 → W8. Per this workspace's TDD-first convention,
each verb group's tests are written before its handler, confirmed red,
then implemented. No CLI framework, no new dependency until W7 (`bubbletea`,
confirmed in the design).

---

## W1 — `busy_timeout` fix

### File to modify

`knowledge.go` — add `PRAGMA busy_timeout = 5000;` to the `schema` const
(alongside the existing `PRAGMA foreign_keys = ON`/`PRAGMA journal_mode =
WAL`).

### Test to add

`TestOpen_SetsBusyTimeout` — open a db, query `PRAGMA busy_timeout;`,
assert it's `5000`. Simple, no red/green ceremony needed for a one-line
PRAGMA addition, but still worth a regression test so a future schema
edit can't silently drop it.

### Acceptance criteria

- `go test ./...` still green (this must not change any existing
  behavior — WAL mode already serializes writers; this only changes how
  long a second writer waits before giving up).

---

## W2 — `cmd/kb` scaffold: dispatch, global flags, usage

### Files to create

| File | Contents |
|---|---|
| `cmd/kb/main.go` | `main()`: parse `--db`/`--json` (global, before the verb), resolve `dbPath` via `knowledge.DefaultPath(cwd)` or the `--db` override, open the KB once, dispatch to the matched verb's handler, print top-level usage on no/unknown verb or `-h`/`--help`/`help` |
| `cmd/kb/dispatch.go` | `verbs map[string]verbFunc` table + `dispatch(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out, errOut io.Writer) int` (returns the process exit code) |
| `cmd/kb/output.go` | Shared helpers: `printJSON(out io.Writer, v any) error`, `printError(errOut io.Writer, jsonOut bool, err error)` (implements decision 2's stderr/JSON-envelope/exit-code convention in one place, reused by every verb) |

`verbFunc` signature (each verb group implements one or more of these):

```go
type verbFunc func(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error
```

Handlers take an already-open `*knowledge.KnowledgeBase` (opened once in
`main`, not per-verb) and the raw remaining args (each handler owns its
own `flag.FlagSet` for verb-specific flags, e.g. `--project`,
`--identifier-type`).

### Tests to add

- `TestDispatch_UnknownVerbReturnsUsageError`
- `TestDispatch_HelpVerbPrintsUsage`
- `TestPrintError_JSONMode` / `TestPrintError_TextMode` — both write to
  stderr, never stdout; JSON mode produces `{"error": "..."}`.

### Acceptance criteria

- `go build -o bin/kb ./cmd/kb` succeeds even with zero real verbs wired
  yet (only `help`/unknown-verb paths).
- `go test ./cmd/kb/...` passes.

---

## W3 — Project, observation, concept, link verbs

### Files to create

| File | Verbs | Wraps |
|---|---|---|
| `cmd/kb/project.go` | `project add\|list\|show\|concepts` | `AddProject`, `Projects`, `ProjectByName`, `ProjectConcepts` |
| `cmd/kb/observation.go` | `observation add\|list\|show\|sources` | `AddObservation`/`AddObservationWithSource`, `Observations`, `ObservationByID`, `ObservationSources` |
| `cmd/kb/concept.go` | `concept add\|list` | `AddConcept`/`AddConceptWithIdentifier`, `Concepts` |
| `cmd/kb/link.go` | `link project\|observation` | `LinkProjectConcept`, `LinkObservationConcept` |

Per decision 7: `show`/`list` print bare rows (the struct as-is), text
mode as simple aligned columns (reuse the truncate/pad style already
established in `harvey/commands_kb.go` for familiarity, but this is new,
independent code — no shared package with `harvey`).

### Tests to add (one file each, mirroring the verb files above)

For every verb: a success-path test and at least one error-path test
(missing required flag, project not found, invalid kind, etc.), following
the existing `knowledge_test.go` pattern of a temp-dir `Open`ed database
per test. Text-mode and `--json`-mode output both asserted for at least
one verb per file (doesn't need to be exhaustive per verb — the shared
`printJSON`/`printError` helpers from W2 already carry most of that
weight).

### Acceptance criteria

- `go test ./cmd/kb/...` green for all four files.
- Manual smoke test: `bin/kb project add test-project "smoke test"`,
  `bin/kb project list`, `bin/kb project list --json` against a temp db —
  confirms the full argv → flag → handler → output path end to end, not
  just the unit-tested handler logic in isolation.

---

## W4 — Source, search, summary, format verbs

### Files to create

| File | Verbs | Wraps |
|---|---|---|
| `cmd/kb/source.go` | `source add\|list\|show\|remove\|retract\|link\|check-retractions` | `AddSource`, `ListSources`, `ShowSource`, `RemoveSource`, `RetractSource`, `LinkObservationSource`, `CheckRetractions` |
| `cmd/kb/search.go` | `search`, `summary`, `format` | `Search`, `Summary`, `FormatMarkdown` |

`kb format`'s output is Markdown text in both modes — `--json` wraps it
as `{"markdown": "..."}` rather than trying to structure prose, since
there's no natural JSON shape for a formatted document; scripts wanting
structured data should call `project show`/`observation list`/etc.
directly instead.

### Tests to add

Same pattern as W3 — one test file per source file, success + error paths
per verb.

### Acceptance criteria

- `go test ./cmd/kb/...` green.
- Manual smoke test against the real, already-migrated `~/WorkLab/agents/knowledge.db`
  copy (not the live file — copy it to a scratch path first): `kb search`,
  `kb summary`, `kb project list` all return the expected real data (8
  projects, 122 observations, 5 concepts).

---

## W5 — `merge` verb (retires `cmd/kbmerge`)

### File to create

`cmd/kb/merge.go` — the `run(aPath, bPath, outPath string, force bool, out
io.Writer) error` logic from `cmd/kbmerge/main.go` becomes the `merge`
verb's handler almost verbatim (it already only depends on
`knowledge.CollisionReport`/`ReconcileCollisions`/`MergeKnowledgeBases`,
all in-package now). Flags: `-a`, `-b`, `-out`, `-force` — unchanged names,
so any existing muscle-memory/scripts using `kbmerge`'s flags map
directly onto `kb merge`'s.

### File to delete

`cmd/kbmerge/` (the whole directory) — this is the "retire" part of
decision 1. Its existing tests, if any get added before this point, move
with it; per `DECISIONS.md`, `cmd/kbmerge` currently has none.

### Tests to add

Port the manual smoke-test pattern already used twice this project
(2026-07-27 entries in `harvey/DECISIONS.md`) into an actual automated
test this time: `TestMerge_TwoIdenticalCopiesProduceMatchingCounts`,
`TestMerge_CollisionRequiresForce`, `TestMerge_CollisionReconciledWithForce`.

### Acceptance criteria

- `go build -o bin/kb ./cmd/kb` succeeds with `cmd/kbmerge/` deleted.
- `go test ./cmd/kb/...` green including the new merge tests.
- Manual smoke test: `kb merge -a copy1.db -b copy2.db -out merged.db`
  against two copies of the real production db, same as the prior manual
  verifications — confirms behavior-preserving, not just compiling.

---

## W6 — JSON mode audit pass

Not new functionality — a dedicated review pass across every verb added
in W3–W5, since JSON support was supposed to be built in per-verb but is
easy to under-test one at a time.

### Acceptance criteria

- Every verb from the tree in `cli-tui-design.md` has been exercised at
  least once with `--json` and produces valid, parseable JSON (a
  table-driven test that runs every verb's handler in both modes against
  a shared fixture database and runs `json.Valid()` / round-trips through
  `json.Unmarshal` on the `--json` output is the cleanest way to cover
  this without one bespoke test per verb).
- Every error path returns exit code 1 and writes to stderr, never stdout,
  in both modes.

---

## W7 — TUI (bubbletea)

### Files to create

| File | Contents |
|---|---|
| `cmd/kb/tui.go` | Entry point: `runTUI(kb *knowledge.KnowledgeBase) error`, called from `main()` when no verb is given |
| `cmd/kb/tui_model.go` | The `bubbletea.Model` — project list (via `bubbles/list`) as the root view, `Enter` drills into that project's observations + concepts (a second `bubbles/list`), `/` opens a search prompt (via `bubbles/textinput`) that calls `Search` and shows results in a third list, `Esc`/`q` navigates back/quits |

Read-mostly per decision 4 — no add/edit/link/retract key bindings in
this version.

### Dependency to add

`github.com/charmbracelet/bubbletea` + `github.com/charmbracelet/bubbles`
to `go.mod` — the first use of a `charmbracelet` package anywhere in this
Laboratory (noted in the design doc; not a blocker, just worth the
`go get`/`go mod tidy` pass being deliberate rather than incidental).

### Tests to add

`bubbletea` models are testable via `tea.Model.Update` directly without a
real terminal — `TestTUIModel_EnterDrillsIntoProject`,
`TestTUIModel_SearchFiltersResults`, `TestTUIModel_EscNavigatesBack`,
following `bubbletea`'s own recommended testing pattern (construct the
model, send `tea.KeyMsg`s to `Update`, assert on the resulting model
state) rather than anything terminal-emulation-based.

### Acceptance criteria

- `go test ./cmd/kb/...` green including the new TUI model tests.
- Manual verification: run `bin/kb` bare (no verb) against a real/copy
  database in an actual terminal (this one genuinely needs eyes-on — a
  TUI's rendering can't be fully confirmed by unit tests alone) — confirm
  the project list renders, drilling in and searching both work, and
  quitting cleanly restores the terminal (raw-mode cleanup on exit,
  including on Ctrl-C).

---

## W8 — Real help text (`helptext.go` pattern) + docs + final verification

Per design decision 5a: replace W2's flat `usageText` constant with the
same pattern `harvey/helptext.go` uses.

### Files to create/modify

| File | Contents |
|---|---|
| `cmd/kb/helptext.go` | Replaces `usage.go`. One Pandoc-Markdown man-page constant per topic: `HelpText` (`kb(1)`, shown by bare `kb -h`/`kb help`), `ProjectHelpText` (`kb-project(1)`), `ObservationHelpText`, `ConceptHelpText`, `LinkHelpText`, `SourceHelpText`, `SearchHelpText` (covers `search`/`summary`/`format`), `MergeHelpText`. Each follows harvey's section structure (title block with `{app_name}(1) user manual \| version {version} {release_hash}`, `# NAME`/`# SYNOPSIS`/`# DESCRIPTION`/`# OPTIONS`) and is formatted through `knowledge.FmtHelp` before printing. |
| `cmd/kb/help_dispatch.go` | `printHelp(out io.Writer, topic string)` — looks up the right constant (empty topic = `HelpText`), formats via `knowledge.FmtHelp(text, "kb", knowledge.Version, knowledge.ReleaseDate, knowledge.ReleaseHash)`, writes it. Replaces the current bare `printUsage` call sites in `main.go`/`dispatch.go`. |
| `Makefile` (new in this repo — doesn't exist yet outside what `cmt` generates) | Targets mirroring `harvey/Makefile`'s pattern: `./bin/kb -help > kb.1.md`, one per verb group (`./bin/kb project -h > kb-project.1.md`, etc.), then `pandoc $@.md --from markdown --to man -s > man/man1/$@` for each. |

### Acceptance criteria

- `kb -h`, `kb help`, `kb project -h`, etc. all print well-formed
  Pandoc-Markdown (verify with `pandoc --from markdown --to man -s` not
  erroring on the output — that's the real test that the man-page
  formatting is actually valid, not just "looks like text").
- Generated `.1.md` files checked in (same convention as harvey's
  `harvey.1.md`, `harvey-*.7.md` files being tracked in git).

### Remaining W8 items (unchanged from the original draft)

- `README.md`/`about.md` (cmt-generated — re-run `cmt codemeta.json
  README.md about.md` after updating `codemeta.json`'s description to
  mention the CLI/TUI, rather than hand-editing the generated files).
- Full verification: `go build ./...`, `go vet ./...`, `go test ./...`,
  `go build -o bin/kb ./cmd/kb`, manual smoke tests from W3–W5 re-run
  once more end to end after all pieces are in place together.
- Log the whole feature in a new `DECISIONS.md` entry (in this repo, not
  `harvey/DECISIONS.md` — this is a `knowledge`-repo-scoped change per
  [[feedback_repo_scoped_docs]]).

---

## Out of scope here

- Full CRUD in the TUI (add/edit/link/retract) — explicit later increment.
- Any change to `harvey`'s own `/kb` slash-commands.
- Publishing a new tagged version of `github.com/rsdoiel/knowledge` once
  this lands, and updating `harvey/go.mod` to point at it — a deliberate,
  separate step, same as every version bump so far this session.
