---
id: "0019"
title: "A project-scoped JSON-L export excludes workspace-tier records; only an unscoped export carries them"
date: "2026-08-28"
status: accepted
kind: decision
trigger: plan-review
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a048f3-3382-70d2-9269-d1c96b9e5563"
origin_host: "MACMINI-RD.local"
---

**Context.**

`ExportJSONL` takes a `projectName` and every helper threads a `scoped`
flag through it (`records-portability-plan.md`, W5). Records gained a tier
in DR-0011: project-tier records belong to one project, workspace-tier
records belong to no project at all. A scoped export therefore has no
principled claim on a workspace-tier record — it was never that project's
in the first place. This needed settling before W5 (JSON-L export/import
parity for `records`/`record_relations`) could be written, since the export
and merge paths must agree on what "this project's records" means.

**Decision.**

A `--project`-scoped export carries only that project's records (and
relations that stay inside them). Workspace-tier records, and any relation
that crosses a project boundary, appear only in an unscoped export.

**Rationale.**

Ownership, not reachability, decides scope. A workspace-tier record was
authored to span repositories; folding it into every project's scoped
export would make one workspace-tier record travel N times over N project
exports, with no single export being the "real" owner of it. That is a
worse kind of non-portability than simply not carrying it — a project
export should be replayable into a fresh database as *that project and
nothing else*, matching how `merge` and `kb record list --project` already
treat the tiers as disjoint.

**Rejected alternatives.**

*Always include the workspace tier in every scoped export.* Rejected: makes
a scoped export non-portable in a different way, since the importing side
may not be part of the same workspace, and duplicates workspace-tier
records across every project's export stream.

**Consequences.**

- W5's `ExportJSONL`/import path treats `scoped` for records the same way
  the rest of the scoped export already treats projects: workspace-tier
  records and relations are visible only when `scoped` is false.
- Getting a workspace-tier record onto another machine requires an
  unscoped export/import, or `merge`, not a scoped export of any single
  project.
- This does not change `merge`, which is already unscoped by nature (W1-W4
  carry all tiers).
