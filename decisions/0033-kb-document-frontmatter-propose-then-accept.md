---
id: "0033"
title: "kb document frontmatter: propose-then-accept provenance generator"
date: "2026-09-23"
status: accepted
kind: decision
trigger: implementation
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a0cef6-784d-7bbf-8cb3-8ef8fb8e984f"
origin_host: "wren"
---

**Context.**

Document frontmatter is optional by this module's own design
(`extractFrontmatter`'s "no frontmatter is normal, not an error"), so a real
document can be missing a title, author, creation date, or keywords, with
nothing surfacing the gap or filling it. Several of those fields are
mechanically derivable from the document's own content or from git/
filesystem provenance without a model call. v0.0.11 item 2, designed in
`frontmatter-generator-design.md` (2026-09-21) and planned in
`frontmatter-generator-plan.md` (phases FM1–FM6, 2026-09-22). Implemented
and smoke-tested 2026-09-23.

**Decision.**

1. New verb: `kb document frontmatter PATH [--accept FIELD,...]
   [--accept-keywords NAME,...] [--set FIELD=VALUE] [--dry-run]`. Not
   `--project`-scoped like `tag`/`fuzzy-tag` — a human reviews one file at
   a time. A bare invocation is read-only by construction: nothing
   selected, nothing written.

2. **Provenance shells out to `git`** — this module's first
   `exec.Command` dependency (`codemeta.json`/`README.md` updated
   accordingly). `title` comes from the document's own first H1;
   `author` is a layered signal — a byline in the document's own prose
   wins, then git's earliest-commit author, then `git config user.name` —
   shown together when signals disagree, not silently collapsed to one
   guess. `dateCreated`/`dateModified` come from git's first/most-recent
   commit touching the file, falling back to filesystem mtime (birth time
   is not portably exposed by Go's standard library without per-platform
   build tags; deliberately not implemented — mtime is used
   unconditionally).

3. `title`/`author`/`dateCreated` are absent-only — never overwritten once
   present. `dateModified` is the one exception: always recomputed and
   rewritten on every accepted run, since a stale value is worse than a
   missing one.

4. **`keywords` diffs the current list against two candidate sets**: known
   concepts already mentioned in the text (`knownKeywordProposals`, reusing
   `eligibleConcepts`'s existing `>1`-occurrence threshold), and new
   candidate terms distinctive to this document specifically
   (`scoreDocumentCandidateTerms`, a sibling of `scoreCandidateTerms` —
   not a modification of it, so that function's existing single-corpus
   contract and tests stay untouched). The design's own flagged
   idf-degeneracy bug (a naive single-document scope makes every term's
   `df` trivially equal the corpus size, collapsing `idf` to 0) is fixed by
   computing `idf` from a comparison scope — the project's other
   records/document-sections, or the whole corpus — with the target
   document explicitly excluded. Accepting a new-candidate keyword calls
   `kb.AddConcept` before writing it; `--accept-keywords` treats *any*
   already-known concept name the same way tag's own `--concept` does
   (plain write, no creation), so it works for either candidate set or an
   arbitrary already-known name, without needing to check which proposal
   list a name came from.

5. **Editing is surgical, via `yaml.Node` — not `documentFrontmatter` (the
   struct) and not a full-file rewrite.** `frontmatterNode` locates the
   frontmatter block by reusing `frontmatterBlockPattern`
   (`documenttag.go`, already used for `excludedSpans`) rather than
   `splitFrontmatter`, which discards the byte offsets a surgical splice
   needs. `setMappingField` finds-or-appends a key in place, leaving every
   other key, value, and comment untouched — verified directly by test,
   the exact data-loss regression this decision exists to prevent.
   `writeDocumentFile` replaces the block (or prepends one, if none
   existed) atomically: temp file, then `os.Rename`.

6. **`documentFrontmatter` (`documents.go`) was not given a `DateModified`
   field**, correcting the design/plan's own stated intent. It's
   unexported, so `cmd/kb` — where this verb's code lives — cannot
   reference it regardless; current field values are read directly from
   the parsed `yaml.Node` instead, which needs no cross-package struct at
   all, and nothing downstream in the `knowledge` package consumes
   `dateModified` today either. See `frontmatter-generator-design.md`
   decision 4's amended note.

7. **`--set FIELD=VALUE` (repeatable) bypasses both signal detection and
   the absent-only rule** — a human's explicit assertion, not a proposal,
   so it writes unconditionally even over an existing value. This is a
   judgment call, not explicitly spelled out in the design: the
   alternative (still refusing to overwrite under `--set`) would make the
   override useless for correcting a wrong value already present.

**Rationale.**

Reading current values from the already-parsed `yaml.Node` (decision 6)
is simpler and more robust than round-tripping through a second,
cross-package struct purely for reads — it needed no new plumbing and
avoids a schema change with no reader. Reusing `frontmatterBlockPattern`
for byte offsets (decision 5) matches this project's now-repeated
precedent (`fuzzy-concept-matching-plan.md`'s F2 makes the same call for
footnote placement) of preferring a positional regex already in `cmd/kb`
over a string-based library function that discards positions.

**Rejected alternatives.**

Adding `DateModified` to `documentFrontmatter` exactly as the plan
specified — rejected once implementation showed `cmd/kb` cannot use an
unexported struct from another package, making the addition unreachable
dead code for this feature's own purposes.

**Consequences.**

`frontmatter-generator-plan.md`'s FM1–FM6 test lists are all implemented
and green (`go test ./...`, `go vet ./...` clean), plus one regression test
added after a bug found live-smoke-testing: the plain-text report's field
switch matched "already set" before checking for a proposed value, so
`dateModified`'s always-fresh proposal was silently hidden once a stale
value already existed in the file — fixed to show both, with a test
locking it down. Live end-to-end verification: a real two-document,
git-tracked project — title/author/dateCreated/dateModified proposed and
accepted correctly (author's multi-signal report shown when byline and git
agreed and also when they didn't), one known-concept keyword and one
brand-new candidate concept both accepted, concept created before the
write for the new one, file re-ingested, and both concepts confirmed
actually linked via `kb document show`. Birth-time detection and the
distance/scoring constants (`>1` occurrence floor, `idf` smoothing form)
remain open for a future record if they prove insufficient in practice.
