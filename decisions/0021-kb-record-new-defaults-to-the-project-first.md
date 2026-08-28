---
id: "0021"
title: "kb record new defaults to the project-first agents/projects/<project>/ layout; kb requires an initialized workspace rather than silently creating one"
date: "2026-08-28"
status: accepted
kind: decision
trigger: request
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["workspace:0002"]
initiative: ""
session: ""
decisions: ["recordNew defaults --project scope to agents/projects/<project>/decisions, overridable via --dir", "New kb init [PATH] verb creates a schema-only agents/knowledge.db, idempotent", "main.go's ambient-db dispatch requires an existing db and errors toward init/import rather than auto-creating", "import (and merge/index) stay exempt from the guard; import keeps its create-on-bootstrap capability"]
tags: [kb, layout, workspace-init, records]
uuid: "01a049ab-42e0-7700-a3e4-733e66fc9a13"
origin_host: "MACMINI-RD.local"
---

**Context.**

The feature request `knowledge/agents-projects-layout-feature-request.md`
(filed 2026-08-28, dropped off by another session, no design cycle run on it
yet) proposes moving process artifacts — decisions, plans, feature requests,
design briefs — out of project repositories and into a project-first tree at
`agents/projects/<project>/` in the WorkLab-style workspace, replacing
today's split between `<project>/decisions/` (project tier) and
`agents/decisions/` (workspace tier). `TODO.md`'s "Requested features"
section tracks the same item. `recordNew` in `cmd/kb/recordnew.go` hardcodes
the old split; `cmd/kb/index_test.go` bakes roughly fourteen fixture paths to
it; `helptext.go` describes it in prose. `kb ingest`, `RecordsUnderPath`, and
record attribution (frontmatter-driven, not path-driven) are unaffected — the
change is scoped to where `kb` writes and documents, not how it reads.

Separately, `TODO.md`'s "Noticed in passing" section records a live bug:
`kb record new` run from inside a project directory (e.g. `~/WorkLab/clasm`)
resolves both its ambient database and its target path relative to the cwd,
silently creating a nested `clasm/clasm/decisions/` corpus with a colliding
`DR-0001` rather than finding the 169 records already at
`~/WorkLab/clasm/decisions/`. The same session hit the analogous failure from
`kb observation add`, which resolved `agents/knowledge.db` from the wrong cwd
and left a stray empty `clasm/agents/` behind on the way out. The root cause
in both cases is `knowledge.Open` (`knowledge.go:326`): it unconditionally
`MkdirAll`s and applies schema to whatever path it is given, so a
wrong-directory invocation silently manufactures a new, empty ambient
workspace instead of failing.

Discussed together on 2026-08-28 because both problems run through the same
code: `recordNew`'s path computation and `main.go`'s ambient-database
resolution.

**Decision.**

1. `recordNew` (`cmd/kb/recordnew.go`) computes `agents/projects/<project>/decisions`
   as the default target for `--project` scope. `--workspace` scope is
   unchanged: `agents/decisions/`.
2. The default is overridable via a new `--dir` flag on `kb record new`, not
   a config value.
3. A new `kb init [PATH]` verb creates a schema-only `agents/knowledge.db` at
   `PATH/agents/knowledge.db` (default: cwd), matching `git init`'s shape. It
   writes no data and is idempotent — rerunning it against an
   already-initialized workspace is a no-op, not an error.
4. `cmd/kb/main.go`'s ambient-database dispatch path (immediately before its
   existing `knowledge.Open` call, ~line 124) gains a guard: `os.Stat` the
   resolved path first, and if `agents/knowledge.db` is absent, fail with an
   error naming both `kb init` (new workspace) and `kb import -in FILE`
   (rebuild from an existing export), instead of calling `Open` and letting
   it silently create the file.
5. `init`, `import`, `merge` and `index` are exempt from the new guard, the
   same way `merge`/`index` already bypass ambient opening today
   (`main.go:114-122`). `import` keeps its existing create-capability: it is
   both the normal ingest path and the rebuild/bootstrap path
   (`rm agents/knowledge.db && kb import -in agents/knowledge.jsonl`,
   documented in `workspace:DR-0002`).
6. `index_test.go`'s fixture paths and `helptext.go`'s prose move to the new
   layout.

**Rationale.**

Project-first (`agents/projects/<project>/{decisions,plans,feature_requests,design}`)
over type-first (`agents/decisions/<project>/`, `agents/plans/<project>/`,
...) keeps one project's paper trail in one place — "show me everything
about `cold`" stays a single-directory question, which is how it is actually
read. The type-first shape is what `agents/decisions/caltechauthors/` already
implies, but that precedent is not migrated by this decision (see
Consequences).

A flag over a config value for the override: cheap, explicit per-invocation,
and defers a workspace-level config file to whenever (if ever) a bare flag
turns out to be annoying in practice.

Requiring an existing workspace rather than silently creating one removes an
entire class of bug — stray nested corpora, colliding record ids, empty
ambient directories left behind — at its actual source, wrong-directory
invocation, rather than trying to detect or repair the damage afterward.
`kb`'s own working assumption is that it always runs at a workspace root;
this makes that assumption enforced rather than silently tolerated when
violated.

The guard belongs in `main.go`, not `knowledge.Open`, because "must already
exist" is a CLI-level policy about ambient path resolution, not a property of
the library. Other callers of `Open` — tests building throwaway databases,
`merge`'s scratch-copy handling — legitimately want open-or-create and would
break if the library itself refused.

`import` staying create-capable, rather than also being gated behind `init`,
avoids two verbs doing the same "build me a database from this file" job, and
keeps the already-accepted `workspace:DR-0002` rebuild recipe working
unmodified.

**Rejected alternatives.**

*Type-first layout*, mirroring the existing `caltechauthors` precedent.
Rejected: splits one project's history across three or four sibling trees.

*Discoverable path* — `kb` searches for an existing `decisions/` directory
for a project wherever it sits, rather than assuming a fixed default.
Rejected as unnecessary complexity (search order, ambiguity when a project
has decisions in two places) for a problem a fixed default plus an override
flag already solves.

*Config value for the override*, instead of or alongside a flag. Deferred
rather than rejected outright — no config file exists yet for this purpose,
and a flag is sufficient for the cases seen so far.

*`kb init` also accepts an `--import PATH` convenience option*, collapsing
"new empty workspace" and "rebuild/bootstrap from an existing jsonl" into one
verb. Rejected: `import` already covers the bootstrap case, since a missing
target database is exactly what `import` already handles by creating one —
`init` growing its own copy of that logic would be two code paths doing the
same thing. `init`'s job stays narrow: a schema-only workspace with no data.

*The existence guard applied inside `knowledge.Open` itself*, gating every
caller. Rejected in favor of a `main.go`-only guard (see Rationale).

**Consequences.**

- Migration of existing corpora under the old shape — `clasm/decisions/`
  (169), `cold/decisions/` (7), `CMTools/decisions/` (13),
  `agents/decisions/caltechauthors/` — is explicitly out of scope for this
  record. Both layouts coexist until a separate decision addresses migration.
- `PROJECT_CYCLE.md` and `DISCUSS_REVIEW_PLAN_IMPLEMENT.md` still describe
  the old in-repo convention and are now stale; updating them is out of
  scope for this record and tracked separately.
- Whether `kb` should ever scaffold or index `plans/` and `feature_requests/`
  content (as opposed to `decisions/`) was raised and closed without action
  this session: nothing in this decision extends `kb`'s scope beyond decision
  records.
- Any script or workflow relying on `kb` auto-creating an ambient
  `agents/knowledge.db` on first use, outside of `import`, needs to call
  `kb init` explicitly instead.
- `cmd/kb/recordnew.go`, `cmd/kb/main.go`, `cmd/kb/index_test.go`, and
  `helptext.go` (plus the `kb-record(1)` / `kb.1` man pages it feeds) all
  need code changes; see the implementation plan.
