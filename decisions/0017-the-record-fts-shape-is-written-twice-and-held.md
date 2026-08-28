---
id: "0017"
title: "The record FTS shape is written twice and held together by a test, rather than extracted into a shared writer"
date: "2026-08-27"
status: accepted
kind: decision
trigger: implementation
project: knowledge
phase: "W3"
supersedes: []
superseded_by: []
relates_to: ["0008", "0013"]
initiative: ""
session: ""
decisions: ["rebuildFTSIfNeeded indexes records as source_type 'record', restating indexRecordFTS's column shape in SQL rather than sharing a writer with it, because the two operate on different things -- one on a Record value in Go, the other on a whole table in SQL", "The duplication is held together by a test asserting a merged record and an ingested one search identically, not by a comment", "A workspace-tier record's NULL project_id is indexed as 0, matching what Record.ProjectID already carries for that tier, so the merged row is identical to the ingested one", "records counts towards the rebuild's anything-to-index total, so a database holding only records still gets indexed", "sources staying out of the rebuild is asserted by a test rather than left as an absence, so DR-0013's deliberate omission cannot be quietly undone"]
tags: [fts, search, records, merge, portability]
uuid: "01a045ae-3926-7b1f-8a89-c7ab76882955"
origin_host: "MACMINI-RD.local"
---

**Context.** W3 makes records reachable from `kb search` after a merge. DR-0013 had already decided that they must be — a merge that unions `records` and stops there produces a database holding every record and finding none of them, because `kb_fts` has no triggers and the merge writes rows straight in. So this phase had one job with its shape already settled, and the only open question was how the rebuild should produce rows that match the ones `indexRecordFTS` writes on the ingest path.

They are not obviously the same kind of code. `indexRecordFTS` takes a `Record` value and one id, and composes the row in Go: `r.Title + "\n" + r.Body` as the body, `"DR-" + r.RecordID` as the label, `r.Title` as the descr, `r.Kind` as the kind. `rebuildFTSIfNeeded` runs one `INSERT ... SELECT` over the whole `records` table and has no `Record` values at all. Expressing one in terms of the other means either loading every record into Go to reindex, or pushing the ingest path's single-row write through SQL string building.

**Decision.** The shape is restated in SQL — `title || char(10) || body`, `'DR-' || record_id`, `title`, `kind` — rather than shared with `indexRecordFTS`. What keeps the two honest is a test, `TestMergeKnowledgeBases_MergedRecordSearchesLikeAnIngestedOne`, which searches for the same record in the database that ingested it and in a merge of that database, and requires the two `KBSearchResult` values to be equal. A comment at each site says the other exists and must match.

A workspace-tier record's NULL `project_id` is indexed as `IFNULL(project_id, 0)`, which is what `Record.ProjectID` already carries for that tier, so the merged row equals the ingested one there too. `records` counts towards the rebuild's anything-to-index total, so a database holding only records — which a merged one can be — still gets indexed. `sources` stays out, and a test now asserts that no `source_type = 'source'` row exists.

**Rationale.** Extracting a shared writer would mean one of two contortions. Reindexing through Go would have the rebuild load every record in the database to write rows it can already write in one statement, on the path that exists precisely for bulk repopulation. Pushing the single-row path through generated SQL would make the ordinary ingest write less direct in order to serve the rare one. Duplication across a Go/SQL boundary is not the same as duplication within one language, and neither collapse is an improvement.

That leaves the duplication needing a guard, and the guard is the interesting part. A comment saying "keep these in sync" is a request; the equality test is a check. It also tests the right thing: not that the two SQL fragments look alike, but that a record found by one route is indistinguishable from the same record found by the other, which is the property anyone actually depends on.

The test's reach was measured rather than assumed. `Search` returns kind, label, snippet and source type, and the snippet for a record is its title — so comparing results catches a wrong label, kind or descr but says nothing about whether the body was indexed at all. A second assertion searches for a phrase that appears only in a record's body. Breaking the shape deliberately, by dropping the `DR-` prefix and the body concatenation, fails both assertions, which is how the guard was confirmed to be one.

Asserting the sources absence is the cheapest part of this and worth naming. DR-0013 left `sources` out on purpose, in a function this phase edits; an absence with a comment beside it is one careless addition away from disappearing, and an absence with a test is not.

**Rejected alternatives.** *Extract a shared FTS writer.* The instinct is right in general and wrong across this boundary, for the reasons above. *Have the merge call `indexRecordFTS` per record.* It reuses the exact shape, and it makes the merge path depend on loading and iterating every merged record in Go, where the rebuild backstop exists to avoid exactly that. *Keep them in sync by comment alone.* What was already there for the other three source types, and what let records be missing from the rebuild in the first place. *Index `sources` while here.* A few lines in the function being edited, which is the argument that lets unrelated changes ride along; DR-0013 declined it and nothing since has changed that.

**Consequences.** Implemented; `go build`, `go vet`, `gofmt` and the full suite are clean. Four tests: both sides' records reachable after a merge, a record's body reachable and not only its title, a merged record searching identically to an ingested one, and no source rows in the index.

`kb search` now reaches a record that arrived by merge, which was DR-0013's stated bar for this work being done rather than half done. The merge path is complete as far as data goes: records travel, their relations travel, and both are findable.

What is not done is the collision handling. W2 established that an unreconciled project name collision still orphans the losing side's records, and W3 changes nothing about that — it makes the records that *do* arrive searchable, not the ones that were dropped. W4 is next and now carries both the identity-collision generalisation and the content-divergence report.
