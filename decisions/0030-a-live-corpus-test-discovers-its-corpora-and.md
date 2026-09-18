---
id: "0030"
title: "A live-corpus test discovers its corpora and excludes foreign dialects by frontmatter"
date: "2026-09-18"
status: accepted
kind: correction
trigger: implementation
project: knowledge
phase: "0.0.10"
supersedes: []
superseded_by: []
relates_to: ["0015", "0021", "0027"]
initiative: ""
session: ""
decisions: ["corpusDirs discovers decision-record corpora by walking ~/WorkLab and ~/Laboratory rather than naming fixed paths, anchored by the in-tree decisions/ directory that travels with the checkout, and deduplicated by resolved absolute path since the anchor and the walk both reach it", "A directory qualifies as one of our corpora only when it holds a record-shaped file (NNNN-*.md) whose first three bytes are ---, the same two-signal rule kb index --all settled on after matching a filename alone picked up unrelated files. This is what keeps a colleague's MADR corpus out of a round-trip test it would fail by construction, and it is DR-0027's policy applied to the test suite rather than a convenience", "TestParseRecordFile_RoundTripsEveryLiveRecord's remembered count floor (total < 190, expected ~198) is removed outright rather than raised, leaving the per-file parse-render-compare property the test was always really asserting, plus a total > 0 guard that a discovery returning nothing is a failure and not a pass", "Hidden directories and node_modules are skipped during the walk, which is what keeps a git worktree's copy of a corpus from being counted a second time under a different path"]
tags: [correction, testing, records, documents, madr, live-corpus]
uuid: "01a0b6ed-69ad-738a-9d11-77f4a5fbea39"
origin_host: "MACMINI-RD.local"
---

**Context.** Found 2026-09-18 reviewing what remained for the v0.0.10
release: `go test ./...` failed with

```
only found 54 records across 3 corpora; expected ~198
```

`corpusDirs` named five fixed paths, three of which — `WorkLab/clasm/
decisions`, `WorkLab/cold/decisions`, and by implication every corpus added
since — went stale when DR-0021's `agents/projects/<project>/decisions/`
layout was rolled out and the corpora were migrated out of band. The
directories still exist and are still empty, so `os.Stat` kept accepting
them.

**What the failure was actually reporting is the part worth recording.** It
reads as a shortfall in the corpus, and it is nothing of the kind: 265
records were sitting in the migrated layout, untouched by this test, and
what failed was *discovery*. Worse, the failure was late. While the
remaining three corpora still summed past the floor, the test passed while
already exercising a shrinking fraction of the live records, and only
started failing once enough had moved. A test that silently stops covering
four fifths of its subject and reports that as a count is a worse failure
mode than one that never ran at all.

This is the same lesson DR-0015 drew about `TestCmdIngest_ClasmCorpus`
pinning `Added == 169`, one layer up: DR-0015 fixed the *count*, and the
same test kept a hardcoded *path list*, which is the same snapshot-of-a-
moment mistake wearing different clothes.

**Decision.** `corpusDirs` discovers corpora instead of listing them. It
walks `~/WorkLab` and `~/Laboratory`, skipping hidden directories and
`node_modules`, and collects every directory named `decisions` that holds
this format's records. The in-tree `decisions/` directory is added first as
an anchor that cannot drift, since it travels with the checkout whatever
machine this runs on; results are deduplicated by resolved absolute path,
because the anchor and the walk of `~/Laboratory` both reach it and
counting its records twice would silently inflate the very total the test
reports.

A directory holds *this format's* records when it contains at least one
record-shaped file (`NNNN-*.md`) whose first three bytes are `---`. That is
the two-signal rule `kb index --all` arrived at last release after matching
on a filename alone picked up a docs-site front page and a blog index: a
name plus real content, never a name alone.

The count floor is removed rather than raised. What remains is the property
the test was always really asserting — every record file found parses,
renders back byte-identically, and any divergence is a declared
normalization case — plus a `total > 0` guard so that a discovery finding
nothing fails loudly instead of passing vacuously.

**Rationale.** The frontmatter signal is not a convenience that happens to
make the test pass; it is DR-0027 applied to the test suite. Under DR-0027 a
colleague's MADR-format ADR is a document, not a record, and MADR has no
frontmatter at all — an `# N. Title` H1 and a two-bullet status block. Those
files match `NNNN-*.md` exactly, so a discovery walk reaches them
immediately: `WorkLab/CL-Web-Components/docs/decisions` and
`WorkLab/workflows/docs/decisions`, twelve files between them as of today.
Feeding them to `ParseRecord` would fail every one with
`no frontmatter: file does not start with ---`, which is the *correct*
refusal producing a wrong test result. Excluding them by the same signal
that defines the boundary keeps the test and the policy saying one thing.

Discovery also widened coverage beyond simply repairing the three stale
entries: the walk found `~/Laboratory/agents/decisions`, a live workspace-
tier corpus the fixed list had never included at all. Coverage went from 54
records across 3 corpora to 267 across 8, all round-tripping byte-identically
with zero declared normalization cases.

**Rejected alternatives.**

*Repoint the five paths at the new layout.* The minimal fix, and it fails
the same way again the next time a corpus is added or moved — which, on the
evidence of DR-0021 and this record, is routine rather than exceptional. It
also would not have found `~/Laboratory/agents/decisions`.

*Raise the floor to 260.* Preserves exactly the property DR-0015 retired:
it fails for the wrong reason when a corpus grows, and it says nothing about
whether the records it counted are the ones it should have counted.

*Skip a corpus whose files fail to parse, rather than excluding foreign
dialects up front.* Tempting because it needs no format knowledge, but it
would also silently skip a corpus of *ours* that had genuinely broken — the
exact regression this test exists to catch. Deciding membership before
parsing keeps a parse failure meaning what it should mean.

*Drive discovery from the knowledge base's own `records.path` rows.* Real
appeal — it is the database's own idea of where the corpora are — but it
makes a file-format round-trip test depend on ingest state, so an un-ingested
corpus would silently drop out and a test failure could mean either a
format regression or a stale database.

**Consequences.**

- The test now depends on directory *shape* (`.../decisions/NNNN-*.md` with
  frontmatter) rather than on remembered locations. A corpus added anywhere
  under either workspace root is picked up on the next run with no edit.
- Coverage varies by machine, and that is accepted. It was already true of
  the fixed list, which skipped entirely when no corpus was present; a
  checkout with no workspace still exercises the in-tree corpus, which is
  never zero.
- The walk costs roughly six seconds against these two trees. Acceptable
  for what it buys, and bounded by the hidden-directory and `node_modules`
  pruning.
- A future corpus of ours that used a different dialect would be invisible
  to this test rather than failing it. No such thing exists, and
  `DECISION_RECORD_FORMAT.md` is what makes it unlikely; if one is ever
  added, this rule is where it has to be taught about it.
