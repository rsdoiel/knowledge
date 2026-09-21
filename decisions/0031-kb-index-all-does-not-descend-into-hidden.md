---
id: "0031"
title: "kb index --all does not descend into hidden directories"
date: "2026-09-18"
status: accepted
kind: correction
trigger: live-test
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0030"]
initiative: ""
session: ""
decisions: ["discoverIndexedCorpora prunes any directory whose name begins with a dot, returning fs.SkipDir rather than merely declining to report it, so nothing inside a hidden tree is walked at all", "The prune applies only to directories descended into, never to ROOT itself, so kb index ~/WorkLab/.claude/worktrees/wt-1 --all still finds the corpora inside a hidden root a caller named deliberately", "Only hidden directories are pruned. node_modules and vendor directories are left alone, because nothing was observed matching there and the two-signal test (generated heading plus a real record file) already makes a false positive unlikely", "The exclusion is documented in kb-index(1) rather than left as behaviour a reader has to infer, since a corpus silently not reported is the failure mode this class of bug produces"]
tags: [correction, index, discovery, worktree, live-test]
uuid: "01a0b70c-6a8b-74e5-baf8-e144f1b73855"
origin_host: "MACMINI-RD.local"
---

**Context.** Found 2026-09-18 while updating the workspace `kb` skills to
v0.0.10 and running the new index-check section of `review-knowledge-base`
against the real `~/WorkLab` tree for the first time:

```
/Users/rsdoiel/WorkLab/.claude/worktrees/competent-hertz-8b7537/agents/decisions: up to date
/Users/rsdoiel/WorkLab/.claude/worktrees/competent-hertz-8b7537/agents/decisions/caltechauthors: up to date
/Users/rsdoiel/WorkLab/CMTools/decisions: up to date
/Users/rsdoiel/WorkLab/agents/decisions: up to date
```

The first two are not corpora. They are a git worktree's copy of the tree
under `.claude/worktrees/<name>/`, which holds a full second copy of
`agents/decisions/` — record files and generated `index.md` alike.

**Why the existing guards do not catch it.** `discoverIndexedCorpora`
already requires two signals together, added last release after matching on
the filename `index.md` alone picked up a docs-site front page and a blog
index: the file must open with this format's generated heading, *and* the
directory must hold at least one record file. A worktree copy satisfies
both, correctly — it is a byte-identical copy of something that genuinely
is a corpus. The two-signal rule answers "is this a corpus?", and the
question that needed asking here is different: "is this corpus one we
should be acting on?"

**It is wrong in two distinct ways, and the read-only one is the milder.**
Under `--check` it reports a duplicate of a corpus already listed, inflating
the count (8 where 6 was right) and, if the worktree is mid-edit, reporting
a stale index that no one should act on. In write mode `--all` would
*rewrite* that copy's `index.md` — editing a throwaway worktree instead of
the real tree, a write to a file the operator did not think they were
naming. That is the same shape as the bug the two-signal rule was added to
prevent, reached by a different route.

**Decision.** `discoverIndexedCorpora` prunes hidden directories: any
directory whose name begins with a dot returns `fs.SkipDir`, so the walk
never descends into it. The prune is skipped for `root` itself, so a caller
who deliberately names a hidden directory as the root still gets the
corpora inside it. `kb-index(1)` documents the exclusion.

**Rationale.** A dot prefix is the convention for "tooling lives here, not
content", and every directory this rule excludes on a real workspace is
tooling: `.claude/worktrees`, `.git`, `.venv`. A corpus a person actually
maintains does not live inside one. Pruning at the walk rather than
filtering the results also means the worktree's subtree is never read at
all, which is the cheaper and the more honest behaviour — the alternative
leaves `--all` having examined files it has no business touching.

Skipping the prune for `root` matters more than it first appears. Without
it, `kb index ~/WorkLab/.claude/worktrees/wt-1 --all` would return nothing
and report "0 corpora processed" — a silent empty success, which is exactly
the failure mode this record is about. A caller who names a hidden path has
said what they mean.

**This is DR-0030's lesson arriving from the other side.** That record
taught a *test's* corpus discovery to skip hidden directories, so a
worktree copy would not be counted twice. The same walk existed in shipping
code with the same gap, and it went unnoticed because nothing in the test
suite walked a tree that had a worktree in it. Writing the skill that
called `--all` against a real workspace is what surfaced it — the same
"run it live before shipping" step that caught the original `index.md`
false positives last release, and the second time that step has paid for
itself on this one function.

**Rejected alternatives.**

*Filter the reported results instead of pruning the walk.* What the skills
did as a stopgap (`grep -v '/\.'`). Fine for a report, wrong for the
library: `--all`'s write mode would still have visited and rewritten the
file before anything downstream could filter it.

*Detect worktrees specifically, by looking for a `.git` file naming a
common dir.* Precise, and too narrow. The problem is not worktrees; it is
that anything under a tooling directory is not content. A vendored
dependency or an editor's backup tree would reproduce it.

*Prune `node_modules` and `vendor` as well,* as the DR-0030 test does.
Considered and declined for now: nothing was observed matching there, and
the two-signal test makes a false positive genuinely unlikely — a vendored
package would have to ship a file opening with this format's exact heading
next to `NNNN-*.md` files. Left as a one-line change if it ever does.

*Make it a flag (`--skip-hidden`, or `--include-hidden`).* Adds a decision
for the caller to get wrong in the direction that writes to the wrong file.
The safe behaviour should not be opt-in.

**Consequences.**

- `kb index ROOT --all` reports fewer corpora on any workspace containing a
  git worktree. Against `~/WorkLab` it went from 8 to 6, and both removed
  entries were duplicates — every real corpus is still found.
- The skills' `grep -v '/\.'` stopgap is now redundant. It is harmless and
  still correct for anyone running an older `kb`, so it stays until the
  workspaces are known to be on a build carrying this fix.
- A corpus that genuinely lives under a dot-directory is now invisible to a
  `--all` run rooted above it. No such corpus exists, and naming it as
  `ROOT` remains the escape hatch.
- Regression coverage is two tests, not one: that a worktree copy is not
  discovered, and that an explicitly hidden root still works. The second
  guards the over-fix, which is the more likely way a future change to this
  function goes wrong.

**Correction (2026-09-19).** This record was originally tagged
`phase: "0.0.10"`, and codemeta.json's `0.0.10` release notes described it
alongside DR-0030. Both were wrong: commit `1ba3acd` (this fix) landed at
2026-09-19T00:28:27Z, roughly 32 minutes after the `v0.0.10` tag (`bdd1cf3`,
2026-09-18T23:56:31Z) was created and the GitHub release published
(2026-09-18T23:58:30Z). The published `v0.0.10` release notes do not, in
fact, mention DR-0031 — only this record's own metadata and the working
copy of codemeta.json drifted. `phase` is cleared to `""` pending the next
release; codemeta.json's release notes need the same correction.
