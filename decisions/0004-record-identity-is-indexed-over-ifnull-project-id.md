---
id: "0004"
title: "Record identity is indexed over `IFNULL(project_id, -1)`, because a NULL defeats a unique index"
date: "2026-08-25"
status: accepted
kind: correction
trigger: implementation
project: knowledge
phase: "W1"
supersedes: []
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: ["The records identity index is expressed over IFNULL(project_id, -1), not project_id, so the workspace tier is actually constrained", "Record carries IngestedAt, matching CreatedAt on Project and Observation", "records gets idx_records_uuid, so it carries the same cross-machine merge identity as the four pre-existing tables"]
tags: [schema, sqlite, workspace-tier, idempotency]
uuid: "01a03b46-e5e0-7461-a74c-ac096492f96d"
origin_host: "MACMINI-RD.local"
---

**Context.** W1 of `decision-records-plan.md` implements the `records` and `record_relations` tables. `decision-records-design.md` specifies the identity index as `CREATE UNIQUE INDEX idx_records_scope_id ON records(project_id, scope, record_id)`, which reads correctly: a record's identity is its (project, scope, id) triple, and making that unique is what lets ingest upsert rather than duplicate. Writing the W1 test for the workspace tier is what exposed the problem. Workspace-tier records carry `project: ""` in frontmatter and a NULL `project_id` in the database — that is how the tier is distinguished from the project tier at all — and SQLite treats NULLs in a unique index as distinct from one another. The index therefore places *no* constraint on any row whose `project_id` is NULL. Every workspace record is unique to itself, so `checksum`-based idempotency silently does not apply to that tier: re-ingesting `~/WorkLab/agents/decisions/` would have duplicated all six records on every run, and the tier's whole point is that it is the one both other workspaces re-ingest.

**Decision.** Index over the expression instead: `CREATE UNIQUE INDEX IF NOT EXISTS idx_records_scope_id ON records(IFNULL(project_id, -1), scope, record_id)`. `AddRecord` performs its identity lookup through the same `IFNULL(project_id, -1) = ?` predicate, with a `projectKey` helper mapping a Go-side `ProjectID` of 0 onto -1, and a matching `projectValue` helper mapping it back onto a SQL NULL on insert. `TestAddRecord_WorkspaceScopeSameIdentityUpdatesInPlace` is the regression test and asserts both that the second add returns the first row's id and that the table still holds exactly one row. Two smaller additions rode along in the same phase: `Record.IngestedAt`, for consistency with `CreatedAt` on `Project` and `Observation`, and `idx_records_uuid`, so `records` carries the same merge identity guarantee as `projects`/`observations`/`concepts`/`sources` already do.

**Rejected.** A sentinel `project_id = 0` for the workspace tier instead of NULL — this is the conventional fix and it would let the plain three-column index work, but `project_id` has a `REFERENCES projects(id)` foreign key and `PRAGMA foreign_keys = ON` is set, so 0 would need a real projects row to point at, inventing a fake project purely to satisfy an index. Enforcing identity in application code only, with a lookup before every insert — `AddRecord` does that lookup anyway, but leaving the database itself unconstrained means any future writer (`kb record new`, `merge`, a direct SQL fix-up) can reintroduce duplicates that nothing catches. Deferring the whole question to W3 on the grounds that ingest is where duplication would appear — the defect is in the schema, and W3 would have had to work around it rather than fix it.

**Consequences.** `go build`/`go vet`/`gofmt`/`go test` clean; 18 tests in `records_test.go`. The W1 migration was verified against a *copy* of the real `~/WorkLab/agents/knowledge.db` (233 observations, 12 projects, 11 concepts), counting rows per user table before and after rather than hardcoding totals, so the test does not rot as the database grows; it skips with a clear message where no such database exists, and that skip path was itself confirmed to be reachable so the pass is not vacuous. This corrects `decision-records-design.md`, which specified the three-column index; the design doc was amended the same day to show the corrected form and to explain what the original constrained, and the plan's W1 section records it too. Expression indexes are portable SQLite (3.9+) and need no special handling from the `glebarez/go-sqlite` driver.
