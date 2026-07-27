# Extracting `knowledge.go` from `harvey` — Implementation Plan

See [module-extraction-design.md](module-extraction-design.md) for the
full audit and confirmed decisions. This covers the actual move: files
copied verbatim (minus the `*Workspace` seam), `harvey` rewired to consume
this module via a `replace` directive, old files deleted from `harvey`.

No behavior changes, no renames beyond what decision 3 requires (dropping
`*Workspace`) — same "pure move" ground rule as `harvey/refactoring-plan.md`'s
R0–R9. Work items ordered W1 → W6; commit after each, not after the whole
thing.

---

## W1 — Copy the four files into this repo, adapt the `Workspace` seam

### Files to create (copied from `harvey/`, then edited)

| New file (this repo) | Copied from | Edits needed |
|---|---|---|
| `knowledge.go` | `harvey/knowledge.go` | `package harvey` → `package knowledge`; `OpenKnowledgeBase(ws *Workspace, customPath string)` → `Open(dbPath string)`; add `DefaultPath(root string) string`; remove the `ws.AbsPath(...)` branch entirely (see below) |
| `knowledge_merge.go` | `harvey/knowledge_merge.go` | `package harvey` → `package knowledge` only — this file already takes plain path strings everywhere, no `Workspace` reference |
| `knowledge_test.go` | `harvey/knowledge_test.go` | `package harvey` → `package knowledge`; `openTestKB`/`reopenTestKB` helpers: replace `NewWorkspace(t.TempDir())` + `OpenKnowledgeBase(ws, "")` with a plain `t.TempDir() + "/knowledge.db"` path + `Open(path)` |
| `knowledge_merge_test.go` | `harvey/knowledge_merge_test.go` | Same package rename; two call sites (`TestMergeKnowledgeBases_CreatesMigratedSchema` and one more, both reopening a merged file) drop the `NewWorkspace`/`ws` argument the same way |

### Exact `OpenKnowledgeBase` → `Open`/`DefaultPath` edit

Old (`knowledge.go:282-298`, in `harvey`):
```go
func OpenKnowledgeBase(ws *Workspace, customPath string) (*KnowledgeBase, error) {
	var dbPath string
	if customPath != "" {
		if filepath.IsAbs(customPath) {
			dbPath = customPath
		} else {
			var err error
			dbPath, err = ws.AbsPath(customPath)
			if err != nil {
				return nil, err
			}
		}
	} else {
		var err error
		dbPath, err = ws.AbsPath(harveySubdir + "/knowledge.db")
		if err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", dbPath)
	...
```

New, in this repo's `knowledge.go`:
```go
// DefaultPath returns the conventional knowledge.db location under root
// (root + "agents/knowledge.db"), for callers with no path override.
//
// Example:
//   dbPath := DefaultPath(workspaceRoot)
//   kb, err := Open(dbPath)
func DefaultPath(root string) string {
	return filepath.Join(root, "agents", "knowledge.db")
}

// Open opens (or creates) the SQLite knowledge base at dbPath. The schema
// is applied on every open so tables are created on first use without
// manual migration.
//
// Example:
//   kb, err := Open(DefaultPath(workspaceRoot))
//   if err != nil {
//       log.Fatal(err)
//   }
//   defer kb.Close()
func Open(dbPath string) (*KnowledgeBase, error) {
	db, err := sql.Open("sqlite", dbPath)
	...
```

`harveySubdir` (the `"agents"` constant, `workspace.go:66` in harvey) is
now inlined into `DefaultPath` and not otherwise needed by this module.
Everything after the `db, err := sql.Open(...)` line is unchanged —
schema application, `kbAlterStmts`, UUID backfill, `migrateExperimentsToProjects`,
FTS setup, all copy verbatim.

### Acceptance criteria

- `go build ./...` succeeds inside `~/Laboratory/knowledge` (once `go.mod`
  has the right dependencies — run `go mod tidy` after copying; the two
  external deps, `github.com/google/uuid` and `github.com/glebarez/go-sqlite`,
  need adding).
- `go test ./...` passes inside `~/Laboratory/knowledge` — this is the
  same test suite that was green in `harvey` moments before the copy, so a
  failure here means the `Workspace`-removal edit broke something, not a
  new bug.

---

## W2 — Move `cmd/kbmerge`

### File to create

`cmd/kbmerge/main.go`, copied from `harvey/cmd/kbmerge/main.go`. Single
edit: the import changes from
```go
harvey "github.com/rsdoiel/harvey"
```
to
```go
knowledge "github.com/rsdoiel/knowledge"
```
and every `harvey.CollisionReport`/`harvey.ReconcileCollisions`/
`harvey.MergeKnowledgeBases` call becomes `knowledge.CollisionReport`/etc.
— same pattern `cmd/assay` already uses to import harvey's root package
from inside the same repo (per `harvey/CLAUDE.md`'s "Two executables"
section), just one module over now instead of the same one.

### Acceptance criteria

- `go build -o bin/kbmerge ./cmd/kbmerge` succeeds inside
  `~/Laboratory/knowledge`.
- Manual smoke test: run it against two temp copies of a real
  `knowledge.db`, same as the verification already done in
  `harvey/DECISIONS.md`'s 2026-07-27 entries — confirms the moved binary
  behaves identically, not just that it compiles.

---

## W3 — Wire `harvey` to consume this module

### `harvey/go.mod`

```
require github.com/rsdoiel/knowledge v0.0.0-00010101000000-000000000000

replace github.com/rsdoiel/knowledge => ../knowledge
```

Run `go mod tidy` inside `harvey/` after adding the `replace` line — it
resolves the pseudo-version automatically once the replace target exists
locally. Per design decision 5, switch the `require` to a real tagged
version (and drop the `replace`) only once this repo is pushed to GitHub
and tagged — not part of this plan.

### Files to edit in `harvey`

| File | Change |
|---|---|
| `harvey.go:159` | `KB *KnowledgeBase` → `KB *knowledge.KnowledgeBase`; add import |
| `terminal.go:1549` (`initKnowledgeBase`) | Resolve the path first (`dbPath := a.Config.Memory.KnowledgeDB; if dbPath == "" { dbPath = knowledge.DefaultPath(a.Workspace.Root) }`), then `kb, err := knowledge.Open(dbPath)` |
| `memory_unified.go:274` | Same pattern, using `u.cfg.KnowledgeDB` / `u.ws.Root` |
| `commands_kb.go:334` | `var s Source` → `var s knowledge.Source` |
| `commands_test.go` (4 lines: 1516, 1541, 1575, 1594) | `Source{...}` → `knowledge.Source{...}` |

Add `knowledge "github.com/rsdoiel/knowledge"` to the import block of
every file touched above that doesn't already have it.

### Files to delete from `harvey`

```
knowledge.go
knowledge_merge.go
knowledge_test.go
knowledge_merge_test.go
cmd/kbmerge/
```

### Acceptance criteria

- `go build ./...` succeeds inside `harvey/`.
- `go vet ./...` clean.
- No remaining references to the bare (unqualified) `KnowledgeBase`,
  `Project`, `Observation`, `Concept`, `Source`, `KBSearchResult`,
  `MergeTableSummary`, or `NameCollision` identifiers anywhere in `harvey`
  — a `grep -rn` for each should show zero hits outside comments/strings
  once this step is done (confirms the design doc's "six real reference
  sites" audit was complete, not just mostly complete).

---

## W4 — Full verification, both repos

```bash
# in ~/Laboratory/knowledge
go vet ./...
go test ./...

# in ~/Laboratory/harvey
go vet ./...
go test ./...
go build -o bin/harvey cmd/harvey/*.go
```

(`go test -race` remains blocked on this Pi by the pre-existing
ThreadSanitizer VMA-width issue noted throughout `harvey/DECISIONS.md` —
not a new gap from this change.)

Manual smoke test against the **real, already-merged** `agents/knowledge.db`
(the one placed on both `wren` and `macmini-rd.local` in the 2026-07-27
cross-machine merge — back it up first, same caution as every other real-data
step this session):

```bash
cp ~/Laboratory/agents/knowledge.db /tmp/knowledge.db.pre-extraction-backup
cd ~/Laboratory/harvey && go run ./cmd/harvey   # or bin/harvey once built
# run a /kb command (e.g. /kb project list) — confirm the 5 real projects
# (harvey, henry, antennaApp, sparqlset, audiobox) still show up correctly
```

### Acceptance criteria

- Both repos' test suites pass.
- `harvey`'s real knowledge base opens correctly through the new module
  and shows the expected, already-verified project/observation counts —
  proof this isn't just "compiles," the actual live data round-trips
  correctly through the moved code.

---

## W5 — Update `harvey`'s docs to reflect the new architecture

### File to edit

`harvey/CLAUDE.md`'s "Three-silo memory architecture" table — the
**Knowledge base** row currently says:
```
| **Knowledge base** | `knowledge.go`, `agents/knowledge.db` | ... |
```
Update to:
```
| **Knowledge base** | `github.com/rsdoiel/knowledge` (external module, via `replace` in `go.mod` until published), `agents/knowledge.db` | ... |
```

Also update the "Repository overview" section of the *root* Laboratory
`CLAUDE.md` if it still names `harvey/knowledge.go` anywhere for the
shared `agents/knowledge.db` — check before editing; don't assume without
verifying current text first (this file already has one known-stale
section, per `knowledge_db_schema_stale.md` memory — don't compound it
with a second one from this change).

### Acceptance criteria

- Docs match the actual code layout after W1–W4.

---

## W6 — Log the decision

`harvey/DECISIONS.md`, new entry following the same convention as every
other 2026-07-27 entry this session: what moved, why, what was
rejected (the stutter rename, decision 2), what's still open (publishing
the new repo, dropping the `replace` directive). Cross-reference this
repo's `module-extraction-design.md`/`-plan.md`.

---

## Out of scope here

- Renaming `KnowledgeBase`/`OpenKnowledgeBase` (design decision 2 — a
  separate, later, low-risk follow-up).
- Publishing `github.com/rsdoiel/knowledge` to GitHub and tagging a real
  version (design decision 5).
- JSON-L export/import (`knowledge_db_jsonl_export_design.md`) — step 3 of
  `harvey/knowledge_db_merge_design.md`'s sequencing, after this.
- Registering the `knowledge` experiment in the shared `agents/knowledge.db`
  itself (per the Laboratory root `CLAUDE.md` convention) — worth doing,
  but a one-line addition that doesn't need its own plan phase; can happen
  any time after W1.
