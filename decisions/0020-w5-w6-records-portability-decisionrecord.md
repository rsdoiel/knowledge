---
id: "0020"
title: "W5/W6 records portability: decisionRecord/decisionRecordRelation naming, project resolved by name, relation endpoints by same-import uuid cache"
date: "2026-08-28"
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
uuid: "01a04904-858d-7645-886f-f6170e2a1fd6"
origin_host: "MACMINI-RD.local"
---

**Context.**

W5 (JSON-L export/import parity for `records`/`record_relations`) and W6
(the DR-0013 acceptance tests, man pages, CHANGES.md) were implemented
following `records-portability-plan.md`'s W5/W6 sections and the "three
findings" it carried forward from W1–W4. Three shapes weren't fully pinned
down at that level of detail and were decided during implementation.

**Decision.**

1. The JSON-L line structs are named `decisionRecord` and
   `decisionRecordRelation`, not `recordRecord`/`recordRecordRelation` — the
   naming collision the plan flagged and asked to be resolved once.
2. `decisionRecord` carries the owning project's **name** (`project_name`),
   not its uuid. Import resolves the local project id by name via
   `resolveProjectByName`, a direct analogue of `importProject`'s own
   name-keyed lookup, rather than through the uuid-cache path
   `resolveLocalID` gives observations.
3. `decisionRecordRelation` still references its endpoints by the record's
   own **uuid**, resolved via a cache (`recordLocalID`) built while
   `decisionRecord` lines are processed earlier in the same import call —
   the same pattern `observation_concept`/`project_concept` already use for
   their endpoints.

**Rationale.**

(1) is cosmetic but real: `recordRecord` doubles a word that already means
two different things in this file (the domain `Record` in records.go, and
"a JSON-L record" as this file's own unit of work), and the plan asked for
one convention applied to both structs.

(2) follows DR-0018 directly: "a record's project is matched across
databases by name... not the project's own uuid, which is order-dependent
on reconciliation." The uuid-cache path observations use resolves an
incoming uuid to a local id that importProject itself established by name
— so it already amounts to name resolution one level removed, but only
for records present in the *same* export stream. A partial/incremental
record-only export (no accompanying project line) would fall through to
`resolveLocalID`'s uuid-fallback query, which fails whenever the local
project exists under a different uuid — exactly DR-0018's target case.
Resolving by name directly closes that gap instead of inheriting it.

(3) is safe precisely because it never leaves the resolution DR-0018
warns about: `recordLocalID` is populated from this file's own uuids as
`decisionRecord` lines are imported, regardless of whether each one was a
fresh insert or matched an existing row by identity. A relation's
endpoints are looked up against that same cache, so the "essentially
always a collision" case (DR-0018 finding 2) never has to be handled at
the relation layer — it was already resolved one step earlier, at the
record layer, by (2).

**Rejected alternatives.**

*Reference relation endpoints by the four-column identity tuple
(workspace, project name, scope, record id) instead of uuid.* Rejected as
unnecessary complexity: it would work, but the cache built during (2)
already gives uuid-keyed lookup the right answer within one import call,
which is the only scope a relation reference in the same file ever needs.

*Fold decisionRecord's uuid resolution into resolveLocalID by adding a
"records" case with a name-aware fallback.* Rejected: resolveLocalID's
db-fallback is uuid-only by design (every table it serves keys naturally
on uuid); special-casing one table inside it would obscure why records are
different rather than making the difference explicit.

**Consequences.**

- `jsonl-export-design.md` should gain a "records" section describing
  `project_name` and the two-tier resolution rule, matching how it already
  documents the identity rules for the other seven line types — not done
  as part of this record.
- Any future JSON-L line type whose parent is itself workspace-scoped (no
  natural project owner) has a precedent to follow: resolve by the
  content-visible name, not by a uuid minted independently on each side.
