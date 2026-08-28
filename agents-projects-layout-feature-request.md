# `agents/projects/<project>/` layout — Feature Request

> **2026-08-28: Filed.** Captured from a design conversation with RSDOIEL
> about reorganising where language-model process artifacts live across
> `~/WorkLab`. No design/decide/plan cycle has been run on the `kb` side yet.
> This document preserves the context for that cycle — it proposes a shape,
> not a committed design. Treat every "would"/"should" below as a starting
> point to confirm or revise.

## Motivation

Colleagues in Digital Library Development have objected to the number of
files committed at the root of the shared project repositories. The response,
agreed in discussion on 2026-08-28, is that feature requests, design briefs,
plans and decision records move **out of the project repositories** and into
the `~/WorkLab` workspace under `agents/`.

`agents/decisions/caltechauthors/` already set a partial precedent — DR-0004
in that corpus argued for keeping records outside the repository they govern,
and it works because record attribution is driven by frontmatter rather than
by filepath. What that precedent did not settle is where the *other* artifact
types go. Decisions were the only kind that had a home.

The layout chosen is **project-first**:

```
agents/projects/<project>/
    decisions/
    plans/
    feature_requests/
    design/
```

The alternative considered was type-first — `agents/decisions/<project>/`,
`agents/plans/<project>/`, and so on, which is the shape the existing
`agents/decisions/caltechauthors/` implies. It was rejected because it splits
one project's paper trail across three or four sibling trees, making "show me
everything about `cold`" a multi-directory question. Project-first keeps a
project's history in one place, which is how it is actually read.

This is a workspace-layout decision that `kb` currently contradicts.

## What `kb` assumes today

One place hardcodes the layout: `recordNew` in `cmd/kb/recordnew.go`.

```go
scope := "project"
dir := filepath.Join(f.project, "decisions")
if f.workspace {
    scope = "workspace"
    dir = filepath.Join("agents", "decisions")
}
```

So `kb record new --project cold` writes to `<root>/cold/decisions/` — inside
the repository — and `--workspace` writes to `<root>/agents/decisions/`.
Neither target survives the reorganisation.

`cmd/kb/index_test.go` encodes the same assumption in roughly fourteen places,
building fixture paths as `filepath.Join(root, "clasm", "decisions")` and
`filepath.Join(root, "agents", "decisions")`.

`helptext.go` describes the convention in prose in at least four places
("a decision record is one file under a project's `decisions/` directory").

## What is already fine

- **`kb ingest` takes an explicit path** (`kb ingest clasm/decisions`), so it
  ingests whatever directory it is pointed at. It also recurses. Nothing about
  the new layout breaks it.
- **Attribution is frontmatter-driven, not path-driven.** A record's project
  comes from its own frontmatter, which is why `agents/decisions/caltechauthors/`
  works at all. Records can move without being rewritten.
- **`RecordsUnderPath`** is already parameterised by path.

The change is therefore narrow: it is about where `kb` *writes* and what it
*documents*, not how it reads.

## Proposal

1. `recordNew` computes `agents/projects/<project>/decisions` for project
   scope. Workspace scope keeps `agents/decisions/` — a workspace-tier record
   governs the workspace itself and has no project to nest under.
2. The path becomes overridable rather than hardcoded, so the layout is a
   default and not a constraint. A `--dir` flag, a config value, or both.
3. `index_test.go` fixtures follow.
4. `helptext.go` and the `kb-record(1)` page describe the new location.
5. Consider whether `record new` should grow a `--kind` beyond decisions —
   if plans and feature requests are to live alongside records under the same
   project directory, there is a question about whether `kb` should scaffold
   those too, or whether they stay hand-authored Markdown that `kb` merely
   indexes. **Not decided.**

## Open questions for the design cycle

- **Migration of existing corpora.** `clasm/decisions/` (169 records),
  `cold/decisions/` (7), `CMTools/decisions/` (13) and
  `agents/decisions/caltechauthors/` all exist today under the old shape.
  Moving them is a `git mv` plus a re-ingest, but `clasm` and `CMTools` are
  outside the cleanup scope agreed on 2026-08-28 (clasm is not in
  `Projects.md`; CMTools is deprecated). Do their corpora move anyway for
  consistency, or does the workspace end up with two layouts?
- **`agents/decisions/caltechauthors/`** is the closest thing to a precedent
  and it is type-first. Does it move to `agents/projects/caltechauthors/decisions/`?
- **Does `kb` need to know about `plans/` and `feature_requests/` at all,**
  or is indexing decisions sufficient and the rest is just files on disk?
- **Should the default be discoverable** — i.e. should `kb` find an existing
  `decisions/` directory for a project wherever it sits, rather than assuming
  one location?

## Related

- Workspace reorganisation discussion, 2026-08-28 (WorkLab).
- `agents/decisions/caltechauthors/` DR-0004 — keeping a corpus outside the
  repository it governs.
- `~/WorkLab/PROJECT_CYCLE.md` and `DISCUSS_REVIEW_PLAN_IMPLEMENT.md`, both of
  which need revising for the same change and currently state that records
  live in the repository they govern.
