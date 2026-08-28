---
id: "0016"
title: "A record with a broken project reference is skipped rather than silently retiered, and a name collision still orphans the losing side's records"
date: "2026-08-27"
status: accepted
kind: decision
trigger: implementation
project: knowledge
phase: "W2"
supersedes: []
superseded_by: []
relates_to: ["0013", "0014"]
initiative: ""
session: ""
decisions: ["The records pass joins through projects with a LEFT JOIN so a NULL project_id survives, guarded by WHERE r.project_id IS NULL OR mp.id IS NOT NULL so the leniency applies only to the workspace tier's genuine NULL", "A record whose project_id is set but unresolvable is skipped, as an observation would be, rather than arriving as a workspace-tier record", "record_relations joins both endpoints INNER, so an edge whose record did not travel is skipped rather than fatal, matching observation_concepts", "ingested_at travels with the record, so a merged record keeps when it was ingested rather than claiming to have arrived at merge time", "An unreconciled project name collision drops the losing side's records exactly as it already drops its observations; this is left to -force and W4 rather than special-cased for records", "The merge summary covers all nine tables that travel, and the list carries a comment saying that a table missing from it is a table whose loss goes unreported"]
tags: [merge, records, workspace-tier, portability]
uuid: "01a045ab-41ba-7802-8775-330a051b40a7"
origin_host: "MACMINI-RD.local"
---

**Context.** W2 of the records-portability plan unions `records` and `record_relations` in `MergeKnowledgeBases`. DR-0013 settled the shape — move records by LEFT JOIN through `projects`, remap `record_relations` through record uuids, add both to the summary — and implementing it raised three questions it had not reached.

The first came out of the LEFT JOIN itself. DR-0013 chose it so a workspace-tier record's NULL `project_id` survives, where the observations pass's INNER JOIN would drop it. But a LEFT JOIN is lenient about more than the case it was chosen for: a record whose `project_id` is set to a project that does not resolve also yields a NULL, and would arrive in the merged database as a workspace-tier record. That is a worse outcome than dropping it, because it is silent and it changes what the record *is*.

The second came out of a test that failed for the right reason. `TestMergeKnowledgeBases_RecordsUnion` originally gave `a` and `b` each their own `harvey` project, which is what two machines actually have before any reconciliation, and only `a`'s record survived. The cause is the pre-existing one `ReconcileCollisions` documents: `b`'s project loses the `name` UNIQUE race, so nothing in the merged database carries its uuid, so every child row that pointed at it is orphaned. Records inherit this from the day they start travelling.

The third is smaller. `records` carries `ingested_at`, which has a `CURRENT_TIMESTAMP` default, so a merge that omits the column silently restamps every record with the time of the merge.

**Decision.** The records pass keeps DR-0013's LEFT JOIN and adds `WHERE r.project_id IS NULL OR mp.id IS NOT NULL`. The leniency then applies only to the workspace tier's genuine NULL: a record with a broken project reference is skipped, as an observation with one is. `record_relations` joins both endpoints INNER, so an edge whose record did not travel is skipped and not fatal, matching `observation_concepts`. `ingested_at` travels with the record.

An unreconciled name collision continues to orphan the losing side's records, exactly as it orphans its observations. This is not special-cased. `-force` and the collision report are the answer, and widening them to records is W4.

The summary covers all nine tables, and the `allTables` list carries a comment saying that a table missing from it is a table whose loss goes unreported — which is what happened to records.

**Rationale.** The `WHERE` clause is the whole of the first decision and it is two conditions long, but without it the LEFT JOIN quietly does something DR-0013 never asked for. DR-0013's reasoning was specifically about the workspace tier, where the NULL is *real* and means "this record belongs to no project". A NULL produced by a failed join means "this record's project is missing", which is a different fact wearing the same shape. Skipping is the conservative reading and it matches what the neighbouring observations pass already does with an unresolvable parent, so the two behave alike for the same reason rather than by coincidence.

Leaving the collision case alone is a sequencing judgement rather than an endorsement. It is genuinely pre-existing — the same merge drops `b`'s observations today, and `ReconcileCollisions` exists precisely for it — so a records-specific mitigation here would be a second, narrower answer to a problem that already has one. What changed is the stakes: an orphaned observation is a note, and an orphaned record is a decision with its rationale. That raises the priority of W4 without changing what W2 should do.

Carrying `ingested_at` costs one column and buys a true answer to "when did this record enter a knowledge base". The default would have made every merged record look freshly ingested, which is the kind of detail that is wrong quietly and forever.

**Rejected alternatives.** *INNER JOIN, as the observations pass uses.* The house pattern for moving a child table, and the reason DR-0013 called this out: it drops every workspace-tier record while leaving a plausible count. A test asserts the workspace-tier record survives and fails against this — verified by making the change and watching only that test go red. *LEFT JOIN with no `WHERE` guard.* Simpler, and silently converts a record with a broken project reference into a workspace-tier record. *Fail the merge on a broken project reference.* Defensible for a decision record, where losing one is worse than losing an observation, but it makes one corrupt row block an entire sync, and the corruption it guards against cannot arise through `kb` — `records.project_id` is `ON DELETE SET NULL`, so producing the test fixture needed `PRAGMA foreign_keys = OFF`, the same way the 24 dangling `observation_concepts` rows came to exist. *Special-case records in the collision path now.* It would mean two collision mechanisms during W4's work to generalise the one that exists.

**Consequences.** Implemented; `go build`, `go vet`, `gofmt` and the full suite are clean. Eight tests cover the union, dedup by shared uuid, the workspace tier surviving with its NULL intact, a project-tier record following its project's merged id, relations remapping through record uuids, an edge with a missing endpoint being skipped, a broken project reference not being retiered, and the summary reporting nine tables.

Two of those are guards rather than descriptions, and were checked by breaking the implementation to confirm they fail: the workspace-tier test against an INNER JOIN, and — from W1 — the workspace-name test against deriving from the scratch path.

The orphaning finding raises what W4 is worth. Until it lands, a cross-machine merge of two databases whose shared projects were created independently silently drops the losing side's records, and the summary will now show it as a records count lower than `FromA + FromB` — visible, but only to someone reading for it. Nothing here makes records searchable: `kb_fts` is still populated by `rebuildFTSIfNeeded`, which does not index records, so a merged database has the rows and finds none of them. That is W3, and it is the next thing.
