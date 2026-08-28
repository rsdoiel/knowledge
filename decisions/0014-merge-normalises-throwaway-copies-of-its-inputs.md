---
id: "0014"
title: "merge normalises throwaway copies of its inputs, and the workspace name travels from the original path rather than the temp directory"
date: "2026-08-27"
status: accepted
kind: decision
trigger: plan-review
project: knowledge
phase: "W1"
supersedes: []
superseded_by: []
relates_to: ["0011", "0013"]
initiative: ""
session: ""
decisions: ["merge runs the lazy migration on its scratch copies before ATTACH, because it copies files with sql.Open rather than knowledge.Open and so has never migrated anything it reads", "Migrating a throwaway copy does not break the promise that -a and -b are untouched; the promise is about the operator's files, and a test asserts they are byte-identical after a run", "The workspace name is derived from the original -a/-b path and passed into the copy's migration, never derived from the copy's own temp path", "DR-0011's derived-not-stored workspace name is correct and stays; the copy is the one place where the path stops being evidence of provenance, and merge compensates rather than the format changing", "This becomes W1 of the records-portability plan and gates every later phase, rather than riding along inside the union work"]
tags: [merge, migration, workspace-tier, portability, plan-review]
uuid: "01a04597-daa1-7962-99f3-6e9efe24d5f4"
origin_host: "MACMINI-RD.local"
---

**Context.** DR-0013 decided that `records` and `record_relations` travel on all three portability paths, and described the merge half as unioning two more tables and adding them to the summary. Phasing that into a plan against the actual code found the union cannot be written yet, for two reasons the design brief did not reach.

`merge` never migrates the databases it reads. `checkpointAndCopy` opens each source with `sql.Open` and a `PRAGMA wal_checkpoint(FULL)`, copies the file and any `-wal`/`-shm` sidecars, and hands the copy to `MergeKnowledgeBases`, which `ATTACH`es it directly. Only the *output* is created through `Open()`. This has never mattered because every table merge names exists in any database old enough to be merged at all. `records` is the first table for which that is false: wren.local's `agents/knowledge.db` had no `records` table as recently as this week, and against such a database `SELECT ... FROM b.records` is a hard error, not an empty result.

The obvious fix — migrate the scratch copies — has a second defect underneath it. Per DR-0011 a record's workspace is `filepath.Base` of the database's root, *derived from the path and never stored*. A scratch copy lives at `/tmp/kbmerge-XXXX/a.db`, so migrating it in place hands `applyRecordsMigration` the name `kbmerge-XXXX`. For a database whose records predate W8 and therefore have an empty `workspace`, the backfill would write that temp name. Those records would then fail to collide with their real counterparts and merge as strangers — two copies of DR-0007 under different workspaces, in a file that reports a clean merge. The window is narrow, since it needs a database holding pre-W8 records and neither machine in this workspace has one, but the result is silent and permanent in the merged file.

**Decision.** `merge` normalises each scratch copy through the lazy migration before `ATTACH`, so a pre-records database gains empty `records` and `record_relations` tables and a pre-W8 one backfills correctly. The workspace name for that migration is derived from the **original** `-a`/`-b` path, computed before the copy is made and threaded through to it — never from the copy's own location. `-a` and `-b` themselves stay untouched on disk, and a test asserts they are byte-identical before and after a run. This is W1 of the records-portability plan and gates W2 through W6.

**Rationale.** Normalising the copy rather than the original keeps the guarantee the man page makes and the operator relies on: `merge` reads two databases and writes a third, and nothing it does to its own temporary files is visible. Migrating the *originals* would be the other way to get schema parity, and it is a materially different promise — a read-only reconciliation tool that quietly upgrades both machines' live databases as a side effect of being asked to compare them.

Threading the workspace from the original path is a two-line change that only looks arbitrary until the alternative is written down. DR-0011 chose derivation over storage because the path is always available and a stored name can drift out of sync with nothing to check it against. That reasoning holds everywhere the database is where it came from. The scratch copy is the single place it is not, and the honest reading is not that DR-0011 was wrong but that provenance has to be carried explicitly across the one hop that discards it. Storing the name in the record file to avoid this would rewrite 205 records to fix a temp-directory artifact.

Making this W1 rather than a bullet inside the union work is a sequencing judgement: the failure it prevents is invisible in a passing test suite built from fixtures created by the current code, since every such fixture already has a `records` table and a correct workspace. It needs its own phase with its own fixtures — one database with no `records` table, one with pre-W8 records — or it will be discovered against wren.local instead.

**Rejected alternatives.** *Migrate `-a` and `-b` in place before copying.* Simplest, and it makes the schema-parity problem disappear for every table at once, but it converts a read-only tool into one that writes to both machines' live databases. *Guard each table's presence at query time* — `SELECT ... WHERE EXISTS (SELECT 1 FROM sqlite_master ...)` or a probe before each union. It avoids the migration entirely, but it spreads schema knowledge across every table's merge statement and silently produces a partial merge where W1 produces a complete one; the operator gets no signal that a table was skipped, which is the exact failure mode that produced DR-0013. *Refuse to merge databases whose schemas differ.* Defensible, and close to the schema-coverage guard DR-0013 deferred, but it makes a first cross-machine merge impossible precisely when the two machines have drifted — which is when a merge is wanted. *Store `workspace` in the record frontmatter*, removing the derivation and with it the temp-path hazard. It rewrites every record in five corpora to solve a problem that exists in one function, and it reintroduces exactly the field DR-0011 rejected as authorable and therefore wrong-able.

**Consequences.** Not yet implemented; this record precedes W1. The plan's phase count goes from the four pieces DR-0013 forecast to six, and W1 gates the rest. Two fixture databases are needed that no test has wanted before — one predating `records`, one predating W8's `workspace` column — and the second test must be written so it fails against the naive in-place migration, since a workspace name that is merely wrong still produces a passing count.

The `kb-merge.1.md` wording that both sources are read "read-only" should be checked against what W1 makes true. It remains accurate about the operator's files, which is what it is telling them, but the sentence was written when nothing was opened for writing at all.

One question from the plan stays open and is not decided here: whether a `--project`-scoped JSON-L export should carry workspace-tier records, which have no project to be scoped by. It belongs to W5 and to its own record if it turns out to be contentious.
