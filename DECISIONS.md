# knowledge — Architecture & UX Decision Log

This file records significant architectural and UX decisions, their rationale, and known trade-offs. New decisions are added at the top. Each entry names the decision, the context that prompted it, the chosen approach, the rejected alternatives, and the consequences.

---

## 2026-08-08 — JSON-L export/import: the no-file-access alternative to `merge`

**Context.** Step 4 of the original cross-machine-sync sequencing
(`knowledge_db_merge_design.md` in the Laboratory root), deferred when the
merge tool shipped 2026-07-27 (see that date's entries below). `merge`
requires `ATTACH`-ing two live `.db` files; JSON-L export/import instead
produces a plain-text snapshot that can move over any channel that only
carries text — paste, email, a git commit. Full audit and confirmed
decisions in `jsonl-export-design.md`; phased TDD plan in
`jsonl-export-plan.md`.

**Decision.** `ExportJSONL(kb, w, projectName string) error` /
`ImportJSONL(kb, r io.Reader) ([]ImportTableSummary, error)` in the
`knowledge` package (`jsonl.go`), plus `kb export [-project NAME] [-out
PATH]` / `kb import [-in PATH]` verbs (`cmd/kb/jsonl.go`). One
self-describing JSON stream — each line a `"type"`-tagged record
(`project`, `concept`, `source`, `observation`, and the three join types),
identity carried by `uuid` (already `UNIQUE`-indexed on every table since
the 2026-07-26 UUID migration), never by the local autoincrement `id`.
Export writes parents before children; import buffers the whole stream by
type first and applies it in that same phase order regardless of the
file's actual line order, so a hand-edited or `cat`-concatenated file
doesn't need to get the interleaving right.

Import's identity rule differs by table, matching how each already
resolves identity elsewhere in this package: projects/concepts are
name-keyed (an existing local row wins as-is, matching `merge`'s own
"first write wins" model — a genuinely new row keeps its origin `uuid`, so
a later real `merge` between two machines that both imported the same
export can still recognize it as the same entity); sources are
identifier-keyed, matching `AddSource`; observations and joins are
uuid-keyed, since observations have no natural content key. Unresolvable
join references and unrecognized `"type"` values are skipped and counted,
not fatal — only malformed JSON aborts the import (with the offending
line number in the error).

**Two real bugs found by the TDD round-trip/re-import tests, neither
anticipated by the design doc:**
- **Sources with no identifier broke idempotent re-import**:
  `importSource` only deduped by `(identifier_type, identifier_value)`
  when both were set (matching `AddSource`'s existing "always insert new"
  behavior for identifier-less sources) — fine for a single call, but
  since `importSource` always carries the source's own `uuid` through,
  re-importing the same file a second time hit the `sources.uuid` UNIQUE
  index. Fixed by also checking `uuid` as a fallback before inserting.
- **Partial/incremental imports silently dropped observations**: a
  JSON-L file containing only `observation`/join records (project already
  synced by an earlier import, so this file doesn't re-include it) left
  `projectLocalID` empty for that project, since the map was only
  populated from `project` records actually present in *this* file — every
  observation referencing it was silently counted as `Skipped`
  (unresolvable), losing real data with no error surfaced. Fixed with
  `resolveLocalID`: check the in-memory map first, fall through to a
  direct `SELECT id FROM <table> WHERE uuid = ?` on a miss (correct because
  a row found this way already exists locally under that exact uuid — no
  identity ambiguity, unlike the name-collision case the map exists to
  handle), caching the result for reuse by later records in the same
  import.

Also manually verified end-to-end with the real `bin/kb` binary against
real data (not just the unit tests): whole-db and `-project`-scoped
export; import into a fresh db, including that FTS5 search actually finds
the imported rows (confirms the `kb_fts` maintenance added to the import
path, mirrored from `AddProject`/`AddConceptWithIdentifier`/
`AddObservationWithSource`, works — an easy thing to get right in unit
tests and still silently miss in the real index); a full re-import
reporting 100% `Skipped`; the intentionally-adversarial case — two
independently-created databases both with a project named `"harvey"`
under different `uuid`s — importing one's export into the other correctly
attached the observation to the *local* `"harvey"` row without touching
its description or creating a duplicate project; stdin as the default
`import` source; and both error paths (unknown `-project`, malformed
JSON).

**Rejected.** Update-on-conflict for observations (re-importing an edited
export overwrites the local row) — would let importing an old backup
silently revert a local edit; matches `MergeKnowledgeBases`'s existing
first-write-wins semantics instead. Per-table files
(`projects.jsonl`, `observations.jsonl`, ...) — one file is simpler to
hand around than several, and the `"type"` field costs one field per line.
Streaming import (apply each line as read, no buffering) — would require
the file to already be in dependency order. A `-format` flag for future
non-JSONL formats — no second format exists yet; `search`/`summary`/
`format` already establishes the precedent of a narrow, format-specific
verb name here.

**Consequences.** `go build`/`go vet`/`gofmt`/`go test` clean throughout
(TDD-first at every phase, red confirmed before each fix — both bugs above
were caught by tests written *before* the fix, not discovered by manual
poking afterward). `codemeta.json` bumped 0.0.2 → 0.0.3;
`version.go`/`CITATION.cff`/`about.md`/`README.md` regenerated via `cmt`.
`Makefile`'s `KB_TOPICS` hand-edited to add `export import` (not via `cmt
codemeta.json Makefile` — that target is hand-customized, see the
2026-07-27 Makefile-regen gotcha below). `kb-export.1.md`/`kb-import.1.md`
generated via `kb-topics-help` and `make man` (pandoc available on this
machine). `user_manual.md` gained a table row and a design-doc link. This
closes out step 4 of the original cross-machine-sync sequencing —
`knowledge_db_merge_design.md`'s deferred item — leaving no open items
from that plan.

---

## 2026-07-27 — `kb --debug`: nil-safe JSONL trace of every knowledge-base call and TUI event

**Context.** Asked for a `--debug` option to help debug the TUI, citing `github.com/caltechlibrary/clasm` (macmini-rd.local, `~/WorkLab/clasm`) as a working precedent. Auditing it first (see `debug-logging-design.md`) found a correction worth recording: `clasm` doesn't actually log any bubbletea/huh event-loop detail anywhere — its `-debug` is scoped entirely to `internal/awsclient`'s AWS SDK call decorators. The reusable part of the precedent was the *architecture* (a nil-safe JSONL `DebugLog`, flag-gated, path announced once at startup — itself modeled on `harvey/debuglog.go`), not an existing TUI-tracing example to copy. Confirmed decisions: `--debug` applies to both the TUI and every CLI verb; log file is `./kb-debug-<timestamp>.jsonl` in the current directory.

**Decision.** `cmd/kb/debuglog.go`: a `DebugLog` type mirroring `clasm`'s `internal/debuglog` (every method nil-safe, so `--debug=false` costs one nil check per call site, no `if debug` conditionals anywhere else), plus a generic `logKBCall`/`logKBCallErr` helper pair wrapping every real `*knowledge.KnowledgeBase` call — an explicit wrapper at each of the ~34 real call sites, not an interface decorator (`clasm`'s `Wrap*` pattern works because its AWS clients are already narrow, fake-able interfaces by design; `*knowledge.KnowledgeBase` is a concrete struct with ~24 methods, and defining a parallel interface just to wrap it in a decorator would duplicate nearly the entire public API for one first-party consumer, for one purpose). `verbFunc` gained a `dl *DebugLog` parameter, threaded the same way `jsonOut bool` already was, across all ~20 handler functions in `project.go`/`observation.go`/`concept.go`/`link.go`/`source.go`/`search.go`/`merge.go`. The TUI additionally logs every `tea.Msg` `Update` receives (`tui_msg`, with the key string for `tea.KeyMsg`) and every view-state transition (`tui_state_change`) and error (`tui_error`), via `setState`/`setErr` helpers that replaced every bare field assignment — so any future state/error the TUI grows is automatically covered, not just today's.

**Two real bugs found, neither anticipated by the design doc:**
- `merge` and `source check-retractions` are the only two verbs whose underlying package-level logic writes directly to `out`/`progressOut` rather than returning a clean result — `CheckRetractions(int, int, error)` doesn't fit `logKBCall`'s single-result generic shape, so it's logged manually via `dl.Log("kb_call", ...)` instead of forced through an awkward wrapper type.
- **A genuine data-loss bug in the precedent itself, caught by manual verification, not anticipated by the design**: two `kb --debug` invocations within the same second silently overwrote each other's log file, because `DefaultDebugLogPath`'s original second-granularity timestamp (copied from `clasm`) collided, and `NewDebugLog` opens with `O_TRUNC`. This matters far more for `kb` than it ever did for `clasm` — `clasm` is one long interactive session per invocation, while `kb`'s verbs are typically many short-lived separate processes, exactly the pattern that triggers the collision. Worse: even switching to microsecond-precision timestamps still collided under a tight-loop regression test on this hardware's clock resolution. Fixed with a per-process atomic counter (the only unconditionally-unique component regardless of clock behavior) combined with the PID and timestamp for readability — verified both by the regression test and a real two-invocation smoke test.

**Rejected.** A package-level `var currentDebugLog *DebugLog` instead of threading `dl` through every signature — avoids the mechanical signature change across ~20 functions, but introduces mutable global state that test code would also need to fight; explicit-parameter threading stays consistent with how `jsonOut` already flows through the same functions. Redacting any logged field — nothing in `knowledge.KnowledgeBase`'s API returns comparably sensitive data to `clasm`'s one exception (`ec2:CreateKeyPair`'s private key material), so no redaction logic was added preemptively.

**Consequences.** `go build`/`go vet`/`gofmt`/`go test` clean throughout (TDD-first every phase, red confirmed before each fix); 90+ tests total. Manually verified end-to-end in a real pseudo-terminal: drove the TUI through drilling into a project, toggling to concepts and back, searching, and quitting, then read the resulting JSONL back as a genuinely useful, chronological trace — including catching bubbletea's own internal `cursor.BlinkMsg`, confirming every message is logged, not a curated subset. Also confirmed directly (not just asserted by unit tests): `--debug`'s absence produces zero log files and byte-identical command output.

## 2026-07-27 — `cmd/kb`: a `<TOOL> <VERB> <PARAMETERS>` CLI, and a read-mostly TUI

**Context.** After the module extraction (see `module-extraction-design.md`/`-plan.md`), the only executable interface to this package was `cmd/kbmerge` — a two-database merge tool only. There was no general way to read or write a knowledge base from the command line, a script, or another language-model harness without writing Go against the package directly. Full audit and confirmed decisions in `cli-tui-design.md`; phased plan in `cli-tui-plan.md`.

**Decision.** `cmd/kb`, a single binary covering the full package API as verbs (`project`, `observation`, `concept`, `link`, `source`, `search`/`summary`/`format`, `merge`), matching the `git`/`go` command model per an explicit user preference. Global `--db PATH` / `--json` flags; human text by default, structured JSON on `--json`, with errors always routed to stderr in both modes so stdout stays parseable for scripts. `cmd/kbmerge` was retired — its logic became the `merge` verb verbatim (same flags), since the design's whole point was one binary, not several. Bare `kb` (no verb) launches a read-mostly interactive TUI (`bubbletea` + `bubbles/list`/`textinput` — the first `charmbracelet` dependency anywhere in this Laboratory) for browsing projects → observations/concepts → search results, with editing deferred to a later increment. Help text follows harvey's own `helptext.go` pattern: Pandoc-Markdown man-page constants per verb group (`kb(1)`, `kb-project(1)`, etc.), formatted through the module's own `FmtHelp`/`Version`/`ReleaseDate`/`ReleaseHash`, with a `Makefile` target generating the `.1.md` sources and converting them to real troff pages via `pandoc --to man`.

**Real bugs found along the way, not anticipated by the design doc:**
- **`CreatedAt` was always the zero value** on every `Project`/`Observation` — `time.Parse("2006-01-02 15:04:05", ts)` (six call sites, inherited from harvey's original code) assumed a format the `glebarez/go-sqlite` driver never actually returns; it returns RFC3339. Fixed with a `parseTimestamp` helper (RFC3339 first, legacy format as fallback), confirmed via a raw `database/sql` probe before touching any code. This also silently affects `harvey` today (same inherited code) — not fixed there, out of scope for this repo.
- **`Open` had no `busy_timeout`** — fine for a single long-lived `harvey` session, but the entire point of this CLI is multiple independent processes touching the same file concurrently. Added `PRAGMA busy_timeout = 5000`.
- **Four `source` verbs silently ignored `--json`** (`remove`, `retract`, `link`, `check-retractions`), and **`merge` accepted but completely ignored `jsonOut`** — found during a dedicated JSON-mode audit pass (W6), not the initial per-verb implementation. All fixed.
- **`merge`'s collision detail only ever went to stdout**, even on the abort path — meaning JSON mode would either leak non-JSON text onto stdout or lose the detail entirely if suppressed. Fixed by moving collision detail into the returned error's message, so it survives correctly into both the text-mode line and the JSON error envelope.
- **`commands_kb.go`'s `/kb show`** (in `harvey`, found while wiring the module extraction, logged here for continuity) was reaching into `KnowledgeBase`'s unexported `db` field directly — invisible while both lived in one package. Fixed with a new `ObservationByID` method instead of exposing internals.
- **`list.Model` zero-value panic** — the TUI's three non-root lists (`observationList`/`conceptList`/`searchList`) were left uninitialized until first populated; the very first `WindowSizeMsg` calling `SetSize` on one paniced. Fixed by constructing all four via `list.New()` up front.

**Rejected.** A CLI framework (`cobra`/`urfave-cli`) for verb dispatch — a small hand-rolled dispatch table matches `cmd/kbmerge`'s existing stdlib-`flag` precedent and needs no new dependency. Renaming `KnowledgeBase`/`Open` to drop the `knowledge.KnowledgeBase` stutter — deferred to its own low-risk follow-up (see the module-extraction entry). A persistent "current project" state file (unlike harvey's interactive `/kb project use`) — this CLI is explicitly stateless across invocations so concurrent/scripted callers never depend on another process's last selection.

**Consequences.** `go build`/`go vet`/`gofmt`/`go test` clean throughout (TDD-first at every phase, red confirmed before each fix). Manual smoke tests against real production data at every phase: `search`/`summary`/`project list` against a copy of `~/WorkLab/agents/knowledge.db` (8 real projects), `merge` against real copies of the merged Laboratory database, and the TUI itself verified in a real pseudo-terminal (via a scripted `pty` session) rather than skipped, since unit tests alone can't confirm terminal rendering. `codemeta.json`'s description updated; `README.md`/`about.md`/`CITATION.cff` regenerated via `cmt`. Not yet done: publishing a new tagged version of this module and updating `harvey/go.mod` to point at it — a deliberate, separate step, same as every version bump so far.
