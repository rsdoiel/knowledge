# `kb --debug` — Implementation Plan

See [debug-logging-design.md](debug-logging-design.md) for the full
rationale and confirmed decisions. Work items ordered W1 → W5. TDD-first:
each phase's tests are written and confirmed red before its implementation.

---

## W1 — `DebugLog` type + `logKBCall` helper

### Files to create

`cmd/kb/debuglog.go` — `DebugLog` struct, `New`, `DefaultPath`, `Log`,
`Path`, `Close` (verbatim shape from `clasm`'s `internal/debuglog`, per
design decision 1), plus the `logKBCall[T any]` generic helper (decision 4).

### Tests to add (`cmd/kb/debuglog_test.go`)

- `TestLog_WritesOneJSONObjectPerLine` — port of `clasm`'s own test.
- `TestLog_NilReceiverIsSafe`
- `TestDefaultPath_MatchesExpectedFormat`
- `TestLogKBCall_LogsMethodParamsAndResult` — fake `call func() (string, error)`
  returning a value; assert the JSONL record has `method`, `params`,
  `result`, `duration_ms`, and the call's return value is passed through
  unchanged.
- `TestLogKBCall_LogsErrorInsteadOfResult` — fake call returning an error;
  assert `error` field present, no `result` field, error passed through
  unchanged.
- `TestLogKBCall_NilDebugLogStillReturnsCallResult` — confirms the logging
  wrapper itself is transparent when `dl` is nil (no panic, correct
  passthrough), same spirit as `TestLog_NilReceiverIsSafe` one level up.

### Acceptance criteria

- `go test ./cmd/kb/...` green.

---

## W2 — Wire `--debug` through `verbFunc` and every handler

### Files to modify

| File | Change |
|---|---|
| `main.go` | `parseGlobalFlags` returns `debugPath string` (or a parsed bool + computed default) alongside `dbPath`/`jsonOut`; `mainRun` opens the `DebugLog` once (skipped for help/no-op paths), prints its path to stderr, passes `dl` into `dispatch`/`runTUI` |
| `dispatch.go` | `verbFunc` gains `dl *DebugLog`; `dispatch`'s own signature and its one call site updated |
| `project.go`, `observation.go`, `concept.go`, `link.go`, `source.go`, `search.go` | every handler function gains `dl *DebugLog`; every real `kb.*` call site (34 total, per the design's audit) becomes `logKBCall(dl, "MethodName", params, func() (T, error) { return kb.MethodName(args...) })` |
| `merge.go` | `cmdMerge` gains `dl *DebugLog`; three new log points per decision 6 (not routed through `logKBCall`, since these are package-level functions, not `*KnowledgeBase` methods) |

### Files to modify (tests)

Every existing test in `project_test.go`, `observation_test.go`,
`concept_test.go`, `link_test.go`, `source_test.go`, `search_test.go`,
`merge_test.go`, `jsonaudit_test.go`, `dispatch_test.go` that calls a
handler directly needs one more argument at the call site (`nil` for
`dl` in tests that don't care about logging — nil-safety means this is a
mechanical, behavior-preserving addition, not a new failure mode).

### Tests to add

- One representative test per verb file confirming a real `kb_call` JSONL
  record actually appears when a non-nil `dl` is passed (not exhaustive
  per call site — `logKBCall`'s own W1 tests already cover the wrapper's
  correctness; this just confirms it's actually wired at real call sites,
  not just defined).
- `TestMerge_LogsCollisionAndSummaryEvents` — real `dl`, confirm
  `merge_collision`/`merge_reconciled`/`merge_summary` events appear as
  appropriate.

### Acceptance criteria

- `go build ./...`, `go vet ./...`, `go test ./...` all clean — this is
  the phase most likely to have a missed call site or stale test
  signature, so treat any compile error here as a real signal, not noise
  to silence.

---

## W3 — TUI: log every `tea.Msg` and state transition

### Files to modify

`tui.go` (`runTUI` gains a `dl *DebugLog` parameter, passed to
`newTUIModel`), `tui_model.go` (`tuiModel` gains a `dl *DebugLog` field;
`Update` logs `tui_msg` for every message — including the key string for
`tea.KeyMsg` — before dispatching to the existing per-state handlers;
every `m.state = viewX` assignment logs `tui_state_change`; every
`m.err = err` assignment logs `tui_error`; `loadObservations`/
`loadConcepts`/`runSearch`'s `kb.*` calls route through `logKBCall`, same
as decision 4).

### Tests to add (`tui_model_test.go`)

- `TestTUIModel_LogsKeyMsgEvents` — construct a model with a real
  temp-file `DebugLog`, send a `tea.KeyMsg`, close the log, read it back,
  assert a `tui_msg` record with the right key string appears.
- `TestTUIModel_LogsStateTransitions` — send Enter (drill into a
  project), assert a `tui_state_change` record with
  `from="viewProjects"`/`to="viewObservations"` appears.
- `TestTUIModel_LogsKBCallsFromSearch` — run the existing search flow
  (`/` → type → Enter), assert a `kb_call` record for `Search` appears
  alongside the `tui_msg`/`tui_state_change` records already covered.

### Acceptance criteria

- `go test ./cmd/kb/...` green.
- Manual verification: run `bin/kb --debug` in a real terminal (same
  pseudo-tty approach used for W7's original TUI verification), then
  `tail`/`cat` the resulting `kb-debug-*.jsonl` and confirm it reads as a
  genuinely useful debugging trace of the session just run — this is the
  actual point of the feature, not just "does it compile."

---

## W4 — Final verification

```bash
go build ./...
go vet ./...
gofmt -l cmd/kb/*.go
go test ./...
go build -o bin/kb ./cmd/kb
```

Manual smoke test: `./bin/kb --debug project list` (and a couple of other
verbs) against a scratch database, confirm the announced log path exists,
contains valid JSONL, and `--debug` being *absent* produces no log file
and no behavior change (the nil-safety guarantee, checked for real, not
just asserted by the unit tests).

Log the feature in this repo's `DECISIONS.md` (append, don't create a new
file — one already exists from the CLI/TUI work).

---

## Out of scope here

- Any change to `harvey`'s own `--debug`/`DebugLog`.
- Redacting any field (see design doc's "what this does not cover").
- Extending `--debug` to log TUI *rendering* (`View()` output) — only
  `Update`'s inputs/transitions and `knowledge.KnowledgeBase` calls are in
  scope; the rendered screen itself isn't logged.
