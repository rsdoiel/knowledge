# `kb --debug` — Design

**Status (2026-07-27):** Decisions confirmed. See
[debug-logging-plan.md](debug-logging-plan.md) for the phased plan.

**References:**
- `harvey/debuglog.go` — the original nil-safe JSONL `DebugLog` pattern.
- `github.com/caltechlibrary/clasm`'s `internal/debuglog` (macmini-rd.local,
  `~/WorkLab/clasm`) — the same pattern applied to AWS SDK calls, explicitly
  modeled on harvey's per its own `DESIGN.md`.

## Motivation, and a correction

You asked for a `--debug` option that emits enough detail to debug the TUI,
"more detail since we're using huh and bubbletea," citing `clasm` as a
precedent that already worked. Worth stating plainly since it changes the
starting point: **`clasm` does not actually log bubbletea/huh event-loop
detail anywhere** — checked `internal/tui`, `internal/ui`, and every `.go`
file under `internal/workflow` for any logging call. `clasm`'s `-debug` is
scoped entirely to `internal/awsclient`'s `Wrap*` decorators, which log
every AWS SDK call (method, region, params, duration, output/error) — the
"successful precedent" is the *architecture* (nil-safe JSONL `DebugLog`,
`-debug` flag, path announced once at startup), not an existing example of
TUI-level tracing to copy. This design applies that same architecture to
two things `clasm` doesn't cover: our own `knowledge.KnowledgeBase` calls,
and bubbletea's `Update` loop itself.

Confirmed in conversation: `--debug` applies to **both** the TUI and every
CLI verb (not TUI-only); the log file is `./kb-debug-<timestamp>.jsonl` in
the current directory, matching `clasm`'s convention exactly (not
`<db-dir>/`).

## Decisions

1. **New file `cmd/kb/debuglog.go`**, same shape as `clasm`'s
   `internal/debuglog` (itself modeled on `harvey/debuglog.go`): a nil-safe
   `DebugLog` struct, `New(path string) (*DebugLog, error)`,
   `DefaultPath() string` (`"kb-debug-" + timestamp + ".jsonl"`),
   `(*DebugLog) Log(event string, fields map[string]any)`, `Path()`,
   `Close()`. Every method nil-safe (a nil `*DebugLog` is exactly what
   `--debug=false` produces), so no `if debug` conditional appears anywhere
   outside the one place `--debug` is parsed. Scoped to `cmd/kb`, not the
   `knowledge` package itself — this is a CLI/TUI concern, not something
   other consumers of the package need.

2. **`--debug` is a third global flag**, alongside `--db`/`--json`, parsed
   in `parseGlobalFlags` the same way. Opened once in `mainRun` (skipped
   entirely for `help`/no-op paths, same as opening the database is);
   its path is printed to stderr once at startup, matching `clasm`'s "so
   the operator knows where to `tail -f` it."

3. **`verbFunc` gains a `dl *DebugLog` parameter**, threaded the same way
   `jsonOut bool` already is — every verb handler (`cmdProject`,
   `cmdProjectAdd`, `cmdObservation`, ... all ~20 handler functions across
   7 files) picks up one more parameter. Rejected: a package-level
   `var currentDebugLog *DebugLog` — avoids touching every signature, but
   introduces mutable global state for something test code also needs to
   control per-test; an explicit parameter is more work to wire but stays
   consistent with how `jsonOut` already flows through the same functions,
   and keeps every handler's dependencies visible in its own signature.

4. **Knowledge-base calls are logged via an explicit generic helper, not
   an interface decorator.** `clasm`'s `Wrap*` pattern works because
   `EC2API`/`SSMAPI`/etc. are already narrow, fake-able *interfaces* by
   design (for testing against fakes, unrelated to debug logging).
   `*knowledge.KnowledgeBase` is a concrete struct with ~24 public
   methods — defining a parallel interface just to wrap it in a decorator
   would duplicate nearly the entire public API as a second abstraction,
   for a single first-party consumer, for one purpose (logging). Instead:
   ```go
   // logKBCall runs call, logs one "kb_call" record to dl (method, params,
   // duration_ms, and either result or error), and returns call's result
   // unchanged. A nil dl makes this call's own overhead one nil check.
   func logKBCall[T any](dl *DebugLog, method string, params any, call func() (T, error)) (T, error) {
       start := time.Now()
       result, err := call()
       fields := map[string]any{
           "method":      method,
           "params":      params,
           "duration_ms": time.Since(start).Milliseconds(),
       }
       if err != nil {
           fields["error"] = err.Error()
       } else {
           fields["result"] = result
       }
       dl.Log("kb_call", fields)
       return result, err
   }
   ```
   Applied at each of the ~34 real call sites across `project.go`,
   `observation.go`, `concept.go`, `link.go`, `source.go`, `search.go`,
   and `tui_model.go` (`merge.go`'s calls are to package-level functions,
   not `*KnowledgeBase` methods — covered separately, decision 6).

5. **The TUI additionally logs every `tea.Msg` and every view-state
   transition** — the actual "more detail... since we're using bubbletea"
   ask, and the piece with no existing precedent anywhere to follow.
   - Every message `Update` receives: `event="tui_msg"`, `msg_type` (Go
     type name via `%T`), and for `tea.KeyMsg` specifically, the key
     string (`msg.String()`) — this is the concrete, high-value case for
     "why didn't my keypress do what I expected."
   - Every view-state change (`viewProjects` → `viewObservations`, etc.):
     `event="tui_state_change"`, `from`, `to`.
   - Errors already stored in `m.err` also get an explicit `event="tui_error"`
     record at the point they're set, not just silently shown in `View()`.

6. **`merge` gets `dl` too, logging its own already-distinct events**
   (`event="merge_collision"`, `event="merge_reconciled"`,
   `event="merge_summary"`) at the same points its existing progress text
   is written — not routed through `logKBCall`, since `CollisionReport`/
   `ReconcileCollisions`/`MergeKnowledgeBases` are package-level functions
   operating on file paths, not `*KnowledgeBase` methods.

## Testing

Mirrors `clasm`'s own `debuglog_test.go` almost exactly:
`TestLog_WritesOneJSONObjectPerLine` (open a temp-file-backed `DebugLog`,
log two events, close, re-read the file, assert line count and field
values), `TestLog_NilReceiverIsSafe` (every method on a nil `*DebugLog` is
a no-op). Plus: `TestLogKBCall_LogsMethodParamsAndResult`,
`TestLogKBCall_LogsErrorInsteadOfResult`, and TUI-side tests sending a
`tea.KeyMsg`/state-changing key to a model constructed with a real
temp-file `DebugLog`, then reading the file back to confirm the expected
`tui_msg`/`tui_state_change` records appear — following the same "real
file, real JSONL, read it back" pattern rather than mocking `Log` itself.

## What this does not cover

- Any change to `harvey`'s own `--debug`/`DebugLog` — untouched, separate
  repo, separate concern.
- Redacting sensitive fields (`clasm`'s one exception: `CreateKeyPair`'s
  private key material never reaches its log) — nothing in
  `knowledge.KnowledgeBase`'s API returns comparably sensitive data today,
  so no redaction logic is being added preemptively; revisit if that
  changes.
