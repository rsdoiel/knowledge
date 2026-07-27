# Extracting `knowledge.go` from `harvey` — Design

**Status (2026-07-27):** Draft — audit complete, decisions below need
sign-off before a `-plan.md` is written.

**References:**
- `harvey/knowledge_db_merge_design.md`'s "Sequencing" section — the
  umbrella decision that this is step 2, after the UUID migration + merge
  tool (step 1, done) and before JSON-L export (step 3, deferred).
- `harvey/experiments-migration-design.md` — the most recent work on
  `knowledge.go`, done 2026-07-27, the day before this extraction started.
  Anything below assumes that work already landed (it has).

## Motivation

Two other experiments in this Laboratory — `henry` and `antennaApp` — want
to write structured observations into `agents/knowledge.db` today, but the
typed CRUD API for it (`OpenKnowledgeBase`, `AddProject`,
`AddObservationWithSource`, `AddConceptWithIdentifier`, `Search`, etc.) is
currently locked inside `harvey`'s own Go module, so they're limited to raw
`sqlite3` CLI inserts. Extracting the code into its own module makes
`harvey` a *consumer* of the knowledge base rather than its owner — the
same relationship `cmd/assay` already has to harvey's root package, one
layer further out.

## What's actually being moved (audited 2026-07-27)

| Source file (harvey) | Lines | Destination |
|---|---|---|
| `knowledge.go` | 1532 | `knowledge.go` (this repo, package `knowledge`) |
| `knowledge_merge.go` | 284 | `knowledge_merge.go` |
| `knowledge_test.go` | 1287 | `knowledge_test.go` |
| `knowledge_merge_test.go` | 577 | `knowledge_merge_test.go` (missed in the first pass of this audit — a separate test file from `knowledge_test.go`, covering `CollisionReport`/`ReconcileCollisions`/`MergeKnowledgeBases`) |
| `cmd/kbmerge/main.go` | 126 | `cmd/kbmerge/main.go` |

## Dependency audit — the actual coupling to `harvey` is small

Checked every reference to harvey-specific types from inside
`knowledge.go`/`knowledge_merge.go`, and every reference to the knowledge
base's own exported types from *outside* those two files, across all of
`harvey`'s other `.go` files:

- **Exactly one** harvey-specific dependency exists: `OpenKnowledgeBase`
  takes `*Workspace` and calls `ws.AbsPath(harveySubdir + "/knowledge.db")`
  to resolve the default location when `customPath == ""`. `Workspace.AbsPath`
  itself (`workspace.go:101`) is a small, harvey-specific helper: it joins a
  relative path onto `ws.Root` and guards against escaping that root — it has
  no other purpose the extracted module needs.
- `IdentifierType` and its constants (`IdentifierDOI`, `IdentifierORCID`,
  etc.) — used by `commands_kb.go` when calling
  `AddConceptWithIdentifier`/`FindOrCreateSource` — are defined in a
  *separate* harvey file, `scholarly_identifiers.go`, and are **not**
  referenced by `knowledge.go` at all: `AddConceptWithIdentifier`'s
  `identifierType`/`identifierValue` parameters are plain `string`. This
  type stays in `harvey`, untouched; callers just pass
  `string(harvey.IdentifierORCID)` into the extracted module's plain-string
  parameters, same as today.
- Every other harvey file that uses the knowledge base does so only through
  **method calls** on a `*KnowledgeBase` value obtained from `Agent.KB`
  (`harvey.go:159`) — `commands_kb.go`, `commands_memory.go`, etc. never
  spell out `Project`/`Observation`/`Concept`/`Source`/`KBSearchResult`/
  `MergeTableSummary`/`NameCollision` by name anywhere (Go's `:=` type
  inference means a `for _, p := range kb.Projects()` loop never needs to
  name the type). **This means renaming the package qualifier
  (`knowledge.Project` instead of bare `Project`) requires zero changes to
  any file except the ones that spell out `*KnowledgeBase` directly.**
- Real cross-file references to the type itself, outside `knowledge.go`/
  `knowledge_merge.go` (checked 2026-07-27, including a second pass
  specifically over test files for direct type literals/declarations,
  which method-call-only usage wouldn't surface):

  | File | What |
  |---|---|
  | `harvey.go:159` | `Agent.KB *KnowledgeBase` field declaration |
  | `terminal.go:1549` | `OpenKnowledgeBase(a.Workspace, a.Config.Memory.KnowledgeDB)` |
  | `memory_unified.go:274` | `OpenKnowledgeBase(u.ws, u.cfg.KnowledgeDB)` |
  | `commands_kb.go:334` | `var s Source` (in `cmdKb`'s `add` subcommand, before `kb.AddSource(s)`) |
  | `commands_test.go` (4 lines: 1516, 1541, 1575, 1594) | `Source{Title: ..., IdentifierType: ..., IdentifierValue: ...}` literals |
  | `config_yaml.go:180` | **False positive** — `knowledgeBaseYAML` struct field named `KnowledgeBase` for the `knowledge_base:` YAML key; unrelated to the `*KnowledgeBase` Go type, not touched by this extraction |

  A broader sweep (grepping every bare `Project`/`Observation`/`Concept`/
  `Source`/etc. token, not just struct literals) turned up many more hits,
  all confirmed false positives on inspection: English prose in
  `helptext.go`/`commands_kb.go`'s user-facing output ("Project not
  found", "Observation %d not found"), and unrelated `Source string`
  fields on entirely different types (`SkillSearchDir.Source`,
  `RecordedMemory.Source`, `TemplateEntry.Source`) that happen to share a
  field name with the knowledge base's own `Source` type but aren't it.

This is a narrow, well-contained seam — six real reference sites total
across all of `harvey`, five of them one-line changes.

## Decisions (draft — need your confirmation)

1. **Package name: `knowledge`** (already set via `go mod init
   github.com/rsdoiel/knowledge`, matches this repo). Not in question —
   confirmed by the scaffold already committed.

2. **Open question — keep `KnowledgeBase`/`OpenKnowledgeBase` names as-is,
   or rename to drop the `knowledge.KnowledgeBase` / `knowledge.OpenKnowledgeBase`
   stutter** (e.g. `knowledge.Base` / `knowledge.Open`)? Recommendation:
   **keep the names as-is for this first extraction pass.** Renaming at
   the same time as moving repos and changing the `Open` signature
   (decision 3) combines two independent risks into one change — if
   something breaks, it's harder to tell whether the move or the rename
   caused it. A pure cosmetic rename is easy and low-risk to do later, in
   its own small follow-up, once the extracted module has already proven
   itself working end-to-end inside `harvey`.

3. **Drop the `*Workspace` parameter entirely.** New signature:
   ```go
   func Open(dbPath string) (*KnowledgeBase, error)
   ```
   No more "empty string means default location" magic inside the
   library — every caller passes a fully-resolved path. This is what
   "harvey becomes a consumer, not the owner" actually requires: the
   library shouldn't need to know what a "workspace" is.

   Also add a small convenience helper, since the motivating use case
   (`henry`, `antennaApp` — neither has a `Workspace`-shaped abstraction of
   its own) still benefits from not hand-rolling the "agents/knowledge.db
   under this project root" convention every time:
   ```go
   // DefaultPath returns the conventional knowledge.db location under root
   // (root + "agents/knowledge.db"), for callers with no path override.
   func DefaultPath(root string) string {
       return filepath.Join(root, "agents", "knowledge.db")
   }
   ```
   `harvey`'s two call sites become (illustrative, exact wiring in the
   `-plan.md`):
   ```go
   dbPath := a.Config.Memory.KnowledgeDB
   if dbPath == "" {
       dbPath = knowledge.DefaultPath(a.Workspace.Root)
   }
   kb, err := knowledge.Open(dbPath)
   ```

4. **`cmd/kbmerge` moves too**, in the same pass — it's a thin wrapper
   around `knowledge_merge.go`'s `CollisionReport`/`ReconcileCollisions`/
   `MergeKnowledgeBases`, which are moving anyway. Leaving it behind in
   `harvey` pointing at a not-yet-independent dependency doesn't make
   sense structurally, and its own tests (none exist yet — see
   `harvey/TODO.md`/`DECISIONS.md`, `cmd/kbmerge` has no test file today)
   aren't a blocker either way.

5. **`harvey`'s `go.mod` uses a `replace` directive during development**:
   ```
   replace github.com/rsdoiel/knowledge => ../knowledge
   ```
   This repo hasn't been pushed to GitHub yet (`codeRepository` in
   `codemeta.json` names the eventual URL, but nothing exists there today)
   and has no tagged version — a real `require` line needs either a public
   tag or a `replace` pointing at the local checkout. Switch to a real
   tagged `require` once this repo is pushed and versioned; document that
   as a followup, not part of this plan.

6. **`IdentifierType` stays in `harvey`.** Confirmed above it's unused by
   the extracted code — no decision needed here, just stating it so it
   doesn't get relitigated during implementation.

## What this design does *not* cover

- Actually moving the files and updating `harvey`'s call sites — that's
  the `-plan.md`, phased TDD, after decisions above are confirmed.
- Publishing/tagging `github.com/rsdoiel/knowledge` on GitHub — decision 5
  above defers this to a followup once the module is proven working via
  `replace`.
- JSON-L export/import (`knowledge_db_jsonl_export_design.md`) — explicitly
  step 3 of the sequencing, after this extraction, not part of it.
- Any change to the actual schema, migration logic, or merge logic itself
  — this is a pure move, mirroring the "no behavior changes" ground rule
  from `harvey/refactoring-plan.md`'s R0–R9 work.
