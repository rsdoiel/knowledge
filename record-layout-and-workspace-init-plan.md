# kb record layout and workspace init — Implementation Plan

## Source design

**DR-0021** (accepted 2026-08-28) and **DR-0022** (accepted 2026-08-28, a
plan-review correction narrowing DR-0021 item 4 to `dbPath == ""`). Both are
project-tier records in `knowledge/decisions/`.

Originating feature request: `agents-projects-layout-feature-request.md`
(filed 2026-08-28, not itself a design cycle — DR-0021/DR-0022 are). Related
`TODO.md` items: "Move kb's record-writing default to the
`agents/projects/<project>/` layout" (Requested features), and "`kb record
new` run from inside a project directory silently creates a nested corpus"
(Noticed in passing) — both closed by this plan.

Out of scope, per DR-0021 Consequences: migrating existing corpora
(`clasm/decisions/`, `cold/decisions/`, `CMTools/decisions/`,
`agents/decisions/caltechauthors/`); `PROJECT_CYCLE.md` /
`DISCUSS_REVIEW_PLAN_IMPLEMENT.md` updates; any `kb` support for `plans/` or
`feature_requests/` beyond decisions.

TDD throughout: the failing test lands before the code that satisfies it,
matching this module's existing practice (see `records-portability-plan.md`).

## A finding from writing this plan

`index_test.go`'s roughly fourteen fixtures building paths like
`filepath.Join(root, "clasm", "decisions")` (flagged in the feature request
as encoding the old layout) turn out **not** to need changing. `kb index`
takes an explicit directory and walks whatever it's given — the fixtures are
realistic sample data for that directory-walking behavior, not a hardcoded
assumption in `index.go` itself. Since migrating existing corpora is out of
scope, those fixtures correctly continue to mirror the layout the real
corpora are still in. Nothing in this plan touches `index_test.go`.

## W1 — `kb init [PATH]` (DR-0021 item 3)

New verb, schema-only, idempotent, default `PATH` is cwd — same shape as
`git init`.

- `cmd/kb/init_test.go` (red first): `kb init` in an empty temp dir creates
  `agents/knowledge.db` with the schema applied and zero rows in `projects`;
  running it a second time succeeds and reports "already initialized" rather
  than erroring or truncating existing data; `kb init PATH` targets
  `PATH/agents/knowledge.db` rather than cwd.
- `cmd/kb/init.go`: `cmdInit(kb *knowledge.KnowledgeBase, dl *DebugLog,
  jsonOut bool, args []string, out io.Writer) error`. `kb` arrives `nil`
  (see below) — `init` resolves its own target path from `args[0]` (or cwd),
  computes `knowledge.DefaultPath(root)`, checks existence first (to decide
  the "already initialized" message), then calls `knowledge.Open` — which
  already does exactly the create-schema work `init` needs — and closes it
  immediately. `verbs["init"] = cmdInit` registered via the package's `init()`
  pattern, matching `jsonl.go`/`merge.go`/`index.go`.
- `cmd/kb/main.go:120`: add `"init"` to the bypass list (`rest[0] == "merge"
  || rest[0] == "index" || rest[0] == "init"`), so `init` — like `merge` and
  `index` — never triggers the ambient `--db` open before it gets to run its
  own path logic. This is also a prerequisite for W2: `init` must reach
  `cmdInit` before any existence guard could apply to it.
- Docs (see W4) and `Makefile` `KB_TOPICS` (see W4) — `TestPrintHelp_
  EveryRegisteredVerbHasATopic` and `TestHelpText_ListsEveryRegisteredVerb`
  (`help_dispatch_test.go`) fail the moment `init` is registered without
  them, so in practice this step happens inside W1, not deferred to W4.

## W2 — ambient-open guard (DR-0021 items 4–5, DR-0022)

- `cmd/kb/main_test.go` (red first): running a normal verb (e.g. `project
  list`) with no `--db` in a temp cwd that has no `agents/knowledge.db` exits
  1 with a message naming both `kb init` and `kb import -in FILE`, and does
  **not** create the file. Running the same verb with `--db
  <nonexistent-path>` still auto-creates and proceeds (unchanged — this is
  exactly `TestMainRun_UnknownVerbOpensDBAndReturnsUsageError`, which stays
  as-is per DR-0022 and needs no edit). Running `kb import -in FILE` with no
  `--db` in that same empty temp cwd still succeeds and creates the db (DR-0021
  item 5).
- `cmd/kb/main.go` (~line 124, the ambient-open branch): before the existing
  `resolveDBPath` + `Open` call, when `dbPath == ""` **and** `rest[0] !=
  "import"`, `os.Stat` the resolved path; on `os.IsNotExist`, print the guard
  error and return 1 without calling `Open`. `import` is carved out here
  specifically because — unlike `merge`/`index`/`init` — it has no path
  handling of its own; it relies entirely on this same ambient-open branch to
  get an open (and, for the rebuild recipe, freshly-created) `*KnowledgeBase`
  passed in. `dbPath != ""` (explicit `--db PATH`, any verb) is untouched.

## W3 — `recordNew` default path + `--dir` override (DR-0021 items 1–2)

- `cmd/kb/recordnew_test.go` (red first): `kb record new --project foo
  --title T --trigger request` writes under `agents/projects/foo/decisions/`;
  `--workspace` still writes under `agents/decisions/` (unchanged); `--dir
  SOME/PATH --project foo ...` writes under `SOME/PATH` instead of the
  default, ignoring `--project` for path purposes (project attribution stays
  frontmatter-driven, per the feature request's own "what's already fine").
- `cmd/kb/recordnew.go:151`: `dir := filepath.Join(f.project, "decisions")`
  becomes `dir := filepath.Join("agents", "projects", f.project,
  "decisions")`; then `if f.dir != "" { dir = f.dir }` before `absDir :=
  filepath.Join(root, dir)`. The `f.workspace` branch (line 152–155) is
  unchanged.
- `cmd/kb/record.go`: add `dir string` to `recordFlags` (line ~50, alongside
  `root`) and `"--dir": &f.dir` to the `strFlags` map in `parseRecordFlags`
  (line ~112).

## W4 — docs (DR-0021 item 6)

- `cmd/kb/helptext.go`:
  - `RecordHelpText` SYNOPSIS (line 647): add `[--dir DIR]` to the `record
    new` line.
  - `RecordHelpText` DESCRIPTION (line 653): "A decision record is one file
    under a project's `decisions/` directory" → describe the new default
    (`agents/projects/<project>/decisions/`, workspace tier unchanged at
    `agents/decisions/`).
  - `RecordHelpText` OPTIONS (~line 735, alongside `--root DIR`): new `--dir
    DIR` entry — overrides the default `--project` target directory.
  - New `InitHelpText` const, same shape as `IndexHelpText`/`MergeHelpText`:
    NAME/SYNOPSIS (`{app_name} init [PATH]`)/DESCRIPTION (idempotent,
    schema-only, defaults to cwd)/EXAMPLES.
  - `TopicsHelpText` (~line 886): add an `init` entry, alphabetically
    consistent with the existing list's verb-introduction order.
  - `HelpText` (kb(1), the main page): mention `init` somewhere in prose —
    required by `TestHelpText_ListsEveryRegisteredVerb`, which asserts every
    registered verb's name appears on this page.
  - `cmd/kb/help_dispatch.go`: `case "init": f(InitHelpText)`.
- `Makefile`: add `init` to `KB_TOPICS` (line 13) — otherwise
  `TestMakefile_KBTopicsCoversEveryVerb` fails and `make man` silently skips
  `kb-init.1.md`.
- `TODO.md`: remove the "Move kb's record-writing default..." item from
  Requested features (done, modulo the migration question it already called
  out as open, which stays out of scope per DR-0021); strike the "`kb record
  new` run from inside a project directory..." item from Noticed in passing,
  noting it's closed by W2's guard.

## Verification

- `go test ./...`, `go vet ./...` clean.
- `make kb-topics-help man` regenerates `kb-init.1.md`,
  `kb-record.1.md`, and `kb.1.md` from the updated `helptext.go`.
- Manual smoke test: `kb init /tmp/kb-smoke && cd /tmp/kb-smoke && kb record
  new --project demo --title "smoke test" --trigger request` lands at
  `/tmp/kb-smoke/agents/projects/demo/decisions/0001-....md`; `kb project
  list` in a fresh empty dir with no `agents/knowledge.db` fails with the
  guard message; `rm agents/knowledge.db && kb import -in
  agents/knowledge.jsonl` (the `workspace:DR-0002` recipe) still works
  unmodified in this workspace itself.
