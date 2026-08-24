---
id: "0002"
title: "`kb --debug`: nil-safe JSONL trace of every knowledge-base call and TUI event"
date: "2026-07-27"
status: accepted
kind: decision
trigger: ""
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: []
tags: []
uuid: "b0c3c25c-9309-44f5-99a0-1febf533dcb6"
origin_host: "MACMINI-RD.local"
---

**Context.** Asked for a `--debug` option to help debug the TUI, citing `github.com/caltechlibrary/clasm` (macmini-rd.local, `~/WorkLab/clasm`) as a working precedent. Auditing it first (see `debug-logging-design.md`) found a correction worth recording: `clasm` doesn't actually log any bubbletea/huh event-loop detail anywhere — its `-debug` is scoped entirely to `internal/awsclient`'s AWS SDK call decorators. The reusable part of the precedent was the *architecture* (a nil-safe JSONL `DebugLog`, flag-gated, path announced once at startup — itself modeled on `harvey/debuglog.go`), not an existing TUI-tracing example to copy. Confirmed decisions: `--debug` applies to both the TUI and every CLI verb; log file is `./kb-debug-<timestamp>.jsonl` in the current directory.

**Decision.** `cmd/kb/debuglog.go`: a `DebugLog` type mirroring `clasm`'s `internal/debuglog` (every method nil-safe, so `--debug=false` costs one nil check per call site, no `if debug` conditionals anywhere else), plus a generic `logKBCall`/`logKBCallErr` helper pair wrapping every real `*knowledge.KnowledgeBase` call — an explicit wrapper at each of the ~34 real call sites, not an interface decorator (`clasm`'s `Wrap*` pattern works because its AWS clients are already narrow, fake-able interfaces by design; `*knowledge.KnowledgeBase` is a concrete struct with ~24 methods, and defining a parallel interface just to wrap it in a decorator would duplicate nearly the entire public API for one first-party consumer, for one purpose). `verbFunc` gained a `dl *DebugLog` parameter, threaded the same way `jsonOut bool` already was, across all ~20 handler functions in `project.go`/`observation.go`/`concept.go`/`link.go`/`source.go`/`search.go`/`merge.go`. The TUI additionally logs every `tea.Msg` `Update` receives (`tui_msg`, with the key string for `tea.KeyMsg`) and every view-state transition (`tui_state_change`) and error (`tui_error`), via `setState`/`setErr` helpers that replaced every bare field assignment — so any future state/error the TUI grows is automatically covered, not just today's.

**Two real bugs found, neither anticipated by the design doc:**
- `merge` and `source check-retractions` are the only two verbs whose underlying package-level logic writes directly to `out`/`progressOut` rather than returning a clean result — `CheckRetractions(int, int, error)` doesn't fit `logKBCall`'s single-result generic shape, so it's logged manually via `dl.Log("kb_call", ...)` instead of forced through an awkward wrapper type.
- **A genuine data-loss bug in the precedent itself, caught by manual verification, not anticipated by the design**: two `kb --debug` invocations within the same second silently overwrote each other's log file, because `DefaultDebugLogPath`'s original second-granularity timestamp (copied from `clasm`) collided, and `NewDebugLog` opens with `O_TRUNC`. This matters far more for `kb` than it ever did for `clasm` — `clasm` is one long interactive session per invocation, while `kb`'s verbs are typically many short-lived separate processes, exactly the pattern that triggers the collision. Worse: even switching to microsecond-precision timestamps still collided under a tight-loop regression test on this hardware's clock resolution. Fixed with a per-process atomic counter (the only unconditionally-unique component regardless of clock behavior) combined with the PID and timestamp for readability — verified both by the regression test and a real two-invocation smoke test.

**Rejected.** A package-level `var currentDebugLog *DebugLog` instead of threading `dl` through every signature — avoids the mechanical signature change across ~20 functions, but introduces mutable global state that test code would also need to fight; explicit-parameter threading stays consistent with how `jsonOut` already flows through the same functions. Redacting any logged field — nothing in `knowledge.KnowledgeBase`'s API returns comparably sensitive data to `clasm`'s one exception (`ec2:CreateKeyPair`'s private key material), so no redaction logic was added preemptively.

**Consequences.** `go build`/`go vet`/`gofmt`/`go test` clean throughout (TDD-first every phase, red confirmed before each fix); 90+ tests total. Manually verified end-to-end in a real pseudo-terminal: drove the TUI through drilling into a project, toggling to concepts and back, searching, and quitting, then read the resulting JSONL back as a genuinely useful, chronological trace — including catching bubbletea's own internal `cursor.BlinkMsg`, confirming every message is logged, not a curated subset. Also confirmed directly (not just asserted by unit tests): `--debug`'s absence produces zero log files and byte-identical command output.

