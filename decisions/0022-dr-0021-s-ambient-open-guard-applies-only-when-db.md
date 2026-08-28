---
id: "0022"
title: "DR-0021's ambient-open guard applies only when --db is not given; explicit --db PATH keeps today's auto-create behavior"
date: "2026-08-28"
status: accepted
kind: decision
trigger: plan-review
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0021"]
initiative: ""
session: ""
decisions: []
tags: [kb, workspace-init]
uuid: "01a049b1-a114-7196-8811-78b442b38a27"
origin_host: "MACMINI-RD.local"
---

**Context.**

Writing `PLAN.md` for `DR-0021`'s guard (item 4: "`main.go`'s ambient-database
dispatch path... gains a guard") surfaced an ambiguity the record itself
didn't resolve: does "ambient" mean only the true default (no `--db` flag,
`dbPath == ""`), or does it also cover an explicit `--db PATH` that happens
to point at a file that doesn't exist yet? `TestMainRun_UnknownVerbOpensDBAndReturnsUsageError`
(`cmd/kb/main_test.go:82`) already exercises the latter: it passes
`--db <tmpdir>/agents/knowledge.db` to a path that has never existed and
asserts that `kb` opens (auto-creates) it and then fails on the unknown verb
— not on the missing database.

**Decision.**

The guard fires only when `dbPath == ""` — i.e. no `--db` flag at all, the
true ambient default resolved from cwd. An explicit `--db PATH` keeps
today's open-or-create behavior unchanged, regardless of whether that path
exists yet.

**Rationale.**

The bugs `DR-0021` exists to fix (`kb record new` and `kb observation add`
silently creating stray workspaces when run from inside `~/WorkLab/clasm`)
both happened through the ambient default — nobody passed `--db`. An
explicit `--db PATH` is the opposite situation: the caller said exactly
where to open, so there is no "wrong directory" for the guard to protect
against. Narrowing the guard to `dbPath == ""` also means
`TestMainRun_UnknownVerbOpensDBAndReturnsUsageError` needs no change, and
keeps `--db` usable for tests, scratch analysis, and scripting against an
arbitrary path without first requiring `kb init` there.

**Rejected alternatives.**

*Guard every resolved path, ambient or explicit.* Rejected: would additionally
require updating `TestMainRun_UnknownVerbOpensDBAndReturnsUsageError` and any
other test or script that relies on `--db PATH` auto-creating, for a case the
guard was never meant to protect against.

**Consequences.**

- `cmd/kb/main.go`'s guard (per `DR-0021` item 4) is implemented as
  `if dbPath == "" { /* stat + guard */ }`, not applied to the `--db`-given
  branch of `resolveDBPath`.
- `TestMainRun_UnknownVerbOpensDBAndReturnsUsageError` is unaffected by this
  work and needs no changes.
