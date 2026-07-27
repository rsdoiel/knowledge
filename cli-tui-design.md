# `kb` — CLI and TUI — Design

**Status (2026-07-27):** Decisions confirmed. See
[cli-tui-plan.md](cli-tui-plan.md) for the phased implementation plan.

**References:**
- `module-extraction-design.md`/`-plan.md` — this is the natural next step
  after the extraction: `henry`/`antennaApp` and other tools/harnesses can
  now depend on `github.com/rsdoiel/knowledge`, but the only executable
  interface today is `cmd/kbmerge` (a two-database merge tool only). This
  design adds the general-purpose interface.
- Confirmed in conversation (2026-07-27): binary name `kb`; human text by
  default, `--json` for machine consumption; `cmd/kbmerge` folds into `kb`
  as a verb; TUI is the same binary launched with no verb, read-mostly to
  start; TUI built on `bubbletea` rather than `termlib`.

## Motivation

`github.com/rsdoiel/knowledge` exposes a full typed Go API (projects,
observations, concepts, sources, search, linking, retraction-checking),
but the only way to drive it today is: write Go code against the package
directly, or use `cmd/kbmerge` (merge only). There's no general
command-line or interactive way to read or write a knowledge base —
`henry`/`antennaApp` are still limited to raw `sqlite3` CLI inserts for
anything beyond what a bespoke Go program provides, and no other tool or
LLM harness can drive this data store without writing Go.

`kb` closes that gap: a single binary, `<TOOL> <VERB> <PARAMETERS>`
(matching `git`/`go`'s own model, per your stated preference), general
enough for scripts, other language-model harnesses, and a human at a
terminal — plus a read-mostly interactive browser when launched bare.

## Audit — full current API surface (2026-07-27)

Every exported method/function in `github.com/rsdoiel/knowledge` today:

| Area | Methods |
|---|---|
| Lifecycle | `Open(dbPath)`, `DefaultPath(root)`, `(*KnowledgeBase).Close()`, `.Path()` |
| Projects | `AddProject`, `Projects`, `ProjectByName`, `ProjectConcepts` |
| Observations | `AddObservation`, `AddObservationWithSource`, `Observations`, `ObservationByID` |
| Concepts | `AddConcept`, `AddConceptWithIdentifier`, `Concepts` |
| Links | `LinkObservationConcept`, `LinkProjectConcept` |
| Sources | `AddSource`, `ListSources`, `ShowSource`, `RemoveSource`, `RetractSource`, `LinkObservationSource`, `ObservationSources`, `FindOrCreateSource`, `CheckRetractions` |
| Search/format | `Search`, `Summary`, `FormatMarkdown` |
| Cross-machine merge | `CollisionReport`, `ReconcileCollisions`, `MergeKnowledgeBases` (currently wrapped by `cmd/kbmerge`) |

**One real gap found while auditing for this design, not related to CLI
work directly but relevant to it:** `Open` sets `PRAGMA journal_mode = WAL`
but never sets a busy timeout. A single interactive `harvey` session
opening the db for a whole run is unlikely to hit this, but the explicit
goal here — "other language model harnesses or systems can use the
knowledge base" — means multiple independent processes (this CLI, `harvey`,
some other future tool) may now genuinely open and write to the same file
concurrently. Without a busy timeout, SQLite returns `SQLITE_BUSY`
immediately on any write contention instead of waiting briefly for the
other writer to finish — exactly the failure mode this whole feature
exists to make more likely. Proposed fix (in `knowledge.go`, not just the
CLI): add `PRAGMA busy_timeout = 5000` to the schema applied in `Open`.
Benefits every consumer (`harvey` included), not just `kb`.

## Decisions

### 1. Verb tree

```
kb project add NAME [DESCRIPTION]
kb project list
kb project show NAME
kb project concepts NAME

kb observation add --project NAME KIND BODY [--source-doi DOI]
kb observation list --project NAME
kb observation show ID
kb observation sources ID

kb concept add NAME [DESCRIPTION] [--identifier-type T --identifier-value V]
kb concept list

kb link project PROJECT_NAME CONCEPT_NAME
kb link observation OBS_ID CONCEPT_NAME

kb source add TITLE [--doi D] [--url U] [--authors A] [--published DATE] [--publisher P] [--rights R] [--version V]
kb source list
kb source show ID
kb source remove ID
kb source retract ID NOTE
kb source link OBS_ID SOURCE_ID [--relationship R]
kb source check-retractions

kb search TERM
kb summary
kb format --project NAME        # was FormatMarkdown; "inject" (harvey's name)
                                 # is harvey-specific framing (injects into a
                                 # chat), not appropriate for a general CLI

kb merge -a PATH -b PATH -out PATH [-force]   # cmd/kbmerge folded in verbatim
```

`kb` with no verb at all launches the TUI (decision 4). `kb help`/`kb -h`
prints usage; `kb VERB -h` prints that verb's flags (standard `flag`
package behavior).

`FindOrCreateSource` and `LinkProjectConcept`/`LinkObservationConcept`
(as a single unified verb vs. two) — covered by `kb link project`/
`kb link observation` above; `FindOrCreateSource` is not exposed as its
own verb since `kb source add` already does find-or-create semantics via
`AddSource`'s existing conflict handling — no separate CLI surface needed
for it.

`kb concept link` was considered as an alternative spelling to `kb link
project`/`kb link observation`, but concepts are the object being linked
in both cases (a project-to-concept link and an observation-to-concept
link), so grouping under `link SUBJECT_TYPE` reads more like `git remote
add`/`git branch delete` (verb-first, unambiguous) than
`kb concept link --project|--observation`, which would need a flag to
disambiguate what's on the other end of the link.

### 2. Global flags: `--db PATH`, `--json`

```
kb [--db PATH] [--json] VERB [verb-specific flags/args...]
```

- `--db PATH`: overrides the database location. Default (no flag):
  `DefaultPath(cwd)`, i.e. `./agents/knowledge.db` relative to the current
  directory — **no parent-directory walking** like `git`'s repo discovery.
  Simpler and fully predictable for scripts/harnesses (which should pass
  `--db` explicitly rather than rely on directory context anyway); an
  interactive human in a workspace root gets the convenient default for
  free. Rejected: walking up parent directories looking for `agents/` —
  adds ambiguity (which `agents/knowledge.db` wins if nested?) for a
  benefit that mostly serves interactive use, which already has the cwd
  default.
- `--json`: switches every verb's success output to structured JSON on
  stdout (the relevant Go struct(s), `encoding/json`, no custom envelope
  beyond what's needed — e.g. `kb project list --json` prints a JSON array
  of `Project`). Errors always go to **stderr**, never stdout, in both
  modes — `{"error": "message"}` as JSON when `--json` is set, plain text
  otherwise — so stdout stays cleanly parseable even when a script/harness
  doesn't perfectly separate the two streams. Exit code `1` on any error,
  `0` on success, matching Unix/`git`/`go` convention.

### 3. No persistent "current project" state

Unlike `harvey`'s interactive `/kb project use ID` (which sets
`Config.Memory.CurrentProjectID` for the rest of the session), `kb` is
stateless across invocations — every verb that needs a project takes an
explicit `--project NAME` flag. No hidden state file recording "the
current project" between runs. This is deliberate for the stated
"other harnesses" use case: concurrent or scripted callers must not have
their behavior depend on some other process's last `use` call.

### 4. TUI: same binary, bare `kb`, read-mostly to start, built on `bubbletea`

`kb` with no arguments launches an interactive terminal browser: list
projects → drill into a project's observations/concepts → search. No
add/edit/link/retract in this first version — that's an explicit,
separate future increment, not part of this plan. Built on
`github.com/charmbracelet/bubbletea` (+ `bubbles` for its ready-made list
component) rather than `termlib` — confirmed in conversation: `termlib`
only provides primitives (box-drawing, raw mode, line editing), not a
scrollable/selectable list widget, and building that from scratch on top
of `termlib` is real, avoidable work for a browser UI whose whole point is
lists. This is the **first use of a `charmbracelet` dependency anywhere in
this Laboratory** (checked — no other experiment currently imports
`bubbletea`/`bubbles`) — worth knowing since it sets a precedent, not
because it's a problem.

### 5a. Help text follows harvey's `helptext.go` pattern, generates real man pages

Confirmed by checking `harvey/helptext.go`, `harvey/help_dispatch.go`, and
`harvey/Makefile`: harvey's help text isn't ad-hoc `fmt.Errorf` strings —
it's a set of Go string constants written as Pandoc-Markdown man pages
(`%{app_name}(1) user manual | version {version} {release_hash}` title
block, then `# NAME`/`# SYNOPSIS`/`# DESCRIPTION`/`# OPTIONS` sections),
with `{app_name}`/`{version}`/`{release_date}`/`{release_hash}` tokens
substituted via `FmtHelp` (already generated into this module's own
`version.go` by `cmt`, unused until now). The Makefile then does
`./bin/TOOL -help > TOOL.1.md`, i.e. the binary's own `-help` output *is*
the checked-in Markdown man page source, and `pandoc TOOL.1.md --from
markdown --to man -s > man/man1/TOOL.1` produces the real troff man page.

`kb` adopts the same pattern, one level deeper — matching `git`'s own
convention of one man page per subcommand (`git-commit(1)`,
`git-log(1)`), which also matches the `<TOOL> <VERB>` model this whole
CLI is built around:

- `cmd/kb/helptext.go` (replacing the flat `usageText` string from the W2
  scaffold): a top-level `kb(1)` constant (`kb -h`/`kb help`) plus one
  constant per verb group (`kb-project(1)`, `kb-observation(1)`,
  `kb-concept(1)`, `kb-link(1)`, `kb-source(1)`, `kb-search(1)`,
  `kb-merge(1)`), each real Pandoc-Markdown man-page content, following
  harvey's exact section structure.
- `kb VERB -h`/`kb help VERB` prints that verb's constant (formatted via
  `knowledge.FmtHelp`, reusing the module's own `Version`/`ReleaseDate`/
  `ReleaseHash`).
- A `Makefile` target (new — this repo doesn't have one yet outside
  CMTools' generated one) generates `.1.md` files the same way harvey's
  does (`./bin/kb -help > kb.1.md`, `./bin/kb project -h > kb-project.1.md`,
  etc.) and converts them via `pandoc --to man`.
- Short, inline `fmt.Errorf("usage: ...")` messages (as sketched in W3's
  early implementation) are fine for *argument-validation* errors (wrong
  number of args, missing required flag) — those aren't man pages, they're
  the same kind of one-line usage reminder `git commit` prints on a bad
  invocation before you'd reach for `git help commit`. The two coexist,
  same as they do in `git` and in harvey today.

### 5. `busy_timeout` fix lands in `knowledge.go`, not just the CLI

Per the audit finding above: add `PRAGMA busy_timeout = 5000` to the
schema applied in `Open`. Small, low-risk, benefits every existing
consumer.

### 6. Command dispatch: small hand-rolled dispatch table, no CLI framework

`cmd/kb/main.go` gets an unexported `dispatch(args []string) error` that
maps verb name to handler function; each verb owns its own `flag.FlagSet`.
Global `--db`/`--json` parsed once, up front, before dispatching. No
`cobra`/`urfave-cli`/etc. dependency — matches `cmd/kbmerge`'s existing
stdlib-`flag` precedent; this is just enough shared plumbing for one
binary's `main.go`, not a reusable package.

### 7. `show`/`list` verbs return bare rows; `kb format` is the assembled view

`kb project show`/`kb project list` (and the equivalent for observations/
concepts/sources) return exactly what the underlying `Projects()`/
`ProjectByName()`/etc. methods already return — no inline assembly of
linked concepts or recent observations. `kb format --project NAME`
(→ `FormatMarkdown`) is the existing, already-built place to get the
fully-assembled Markdown view. Fast, predictable, and avoids two
different "shapes" of project data depending on which verb produced it.

## What this does not cover

- Actual implementation — the `-plan.md`, phased TDD, after the open
  questions above are resolved.
- Full CRUD in the TUI (add/edit/link/retract) — explicit, separate,
  later increment.
- Any change to `harvey`'s own `/kb` command surface (`commands_kb.go`) —
  independent, unaffected; `harvey` keeps its interactive slash-commands
  as they are today, this doesn't replace or wrap them.
- Authentication/authorization — out of scope; this is a local-file
  SQLite tool, same trust model as the file's own permissions today.
