---
id: "0001"
title: "`cmd/kb`: a `<TOOL> <VERB> <PARAMETERS>` CLI, and a read-mostly TUI"
date: "2026-07-27"
status: accepted
kind: decision
trigger: request
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: []
tags: []
uuid: "5a08ed5a-6e10-4289-8f24-90282d7f4032"
origin_host: "MACMINI-RD.local"
---

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
