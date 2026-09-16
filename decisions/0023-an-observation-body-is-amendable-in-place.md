---
id: "0023"
title: "An observation is corrected by superseding it, not by mutating its body"
date: "2026-09-16"
status: accepted
kind: decision
trigger: request
project: knowledge
phase: ""
supersedes: ["0012"]
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: ["kb observation update ID BODY... inserts a new observation and links it to the old one via a new observation_relations(from_id, to_id, relationship) table, supersedes only, mirroring record_relations exactly", "The old observation's body is never rewritten -- history retention is free because nothing is destroyed, rather than a bolt-on revision/history table", "No new updated_at column on observations: the new observation's own created_at is the correction timestamp, the same way a record's correction timestamp is the newer record's own date rather than a mutation of the old one", "kb observation show resolves and prints supersedes/superseded_by in both directions, mirroring kb record show; kb observation list is unchanged, also mirroring records, where only show resolves relations", "kb merge and JSON-L export/import carry observation_relations from the start, per the standing rule that a table missing from the merge summary is a table whose loss goes unreported"]
tags: [request, observations, supersession, merge, portability]
uuid: "01a0ab23-bbf7-73f7-b78b-0047ece48b5a"
origin_host: "wren"
---

**Context.** `TODO.md` recorded a concrete case on 2026-09-16: working a real
project with `kb`, the natural move to correct or extend an existing
observation was `kb observation update` — it does not exist. `kb observation`
has only `add`/`list`/`show`/`sources`. This record's own first draft filled
that gap the way `set-description` fills it for projects: overwrite `body` in
place. Review caught what that loses — an observation is supposed to be a
*timestamped* note, and DR-0012 excluded it from `set-description`'s pattern
on exactly that ground: "correct an observation" is the amend-versus-
supersession question the `records` table already answers with an explicit
`supersedes` edge, and "mutating `body` in place would decide it by
accident." An in-place `update` decides it by accident too, and destroys the
original wording doing so, with no way to see afterward that anything had
changed. That is a worse outcome than the missing verb.

**Decision.** `kb observation update ID BODY...` does not touch row `ID`. It
inserts a *new* observation — same `project_id` and `kind` as the original,
`BODY` as its own body — and links the two with a new
`observation_relations(from_id, to_id, relationship)` table, storing
`(new, old, "supersedes")`. This is `record_relations`' own shape, reused
rather than reinvented: same three columns, same composite primary key, same
`AddRecordRelation`-style `INSERT OR IGNORE` semantics, an
`AddObservationRelation` beside it. The old observation's body is never
rewritten. No `updated_at` column is added to `observations`: the correction
timestamp is simply the new observation's own `created_at`, exactly as a
record's correction timestamp is the newer record's own `date` rather than a
mutation of the old one. `kb observation show ID` resolves relations in both
directions and prints them — `supersedes: #OLD` on the new row,
`superseded_by: #NEW` on the old one — mirroring `kb record show` exactly.
`kb observation list` is unchanged; records themselves don't surface
relations in `list` either, only in `show`. `kb merge` and JSON-L
export/import carry `observation_relations` from the point this ships, not as
a follow-on: the project's standing rule is that a table missing from the
merge summary is a table whose loss goes unreported, and this table would
hold exactly the kind of correction a cross-machine workflow most needs not
to lose silently. This decision supersedes DR-0012's observation clause only;
its other five decisions stand as written.

**Rationale.** DR-0012 was right that this is fundamentally the
records-and-`supersedes` question, not the projects-and-`set-description`
one — the first draft answered it as the latter because that shape was
already at hand, not because it fit. Reusing `record_relations`' exact shape
for `observation_relations` rather than inventing a differently-shaped table
keeps the two symmetric for anyone who already understands one of them, and
keeps `RelationsFor`'s read pattern (UNION the forward edge with the inverted
reverse edge, `supersedes` becomes `superseded_by` from the other side)
copyable almost verbatim. History retention falls out for free from
immutability: there is no snapshot to take, no separate history table to
design retrieval semantics for, because the thing being asked for — "what did
this used to say" — is just the old row, sitting there unchanged. A bolt-on
revision-history table would have had to answer its own smaller version of
the same amend-vs-supersede question (does editing again amend the last
revision or add a new one?) instead of reusing an answer the schema already
has. Skipping a new `updated_at` column is the same reasoning applied to the
timestamp specifically: it would be a second, weaker way to answer "when was
this corrected" that the relation plus the newer row's `created_at` already
answers exactly.

**Rejected alternatives.** In-place `UPDATE observations SET body = ?` (this
record's own first draft) — loses the original wording and answers the
amend-vs-supersede question by accident, which is precisely what DR-0012
warned against. A bolt-on `observation_history` snapshot table, keeping the
live row mutable — solves visibility but not the deeper issue, and duplicates
a mechanism (immutable rows plus a relation) the schema already has under a
different name. An `updated_at` column touched on correction — redundant with
the new observation's own `created_at` once supersession is the mechanism;
worth adding later only if a concrete case needs "was this touched" without
following the relation, which nothing does yet. Allowing `--kind` on
`update`, to reclassify while correcting — no motivating case; the evidence
is about wrong or incomplete content, not miscategorized notes. Deferring
`kb merge`/JSON-L coverage of `observation_relations` as a follow-on — the
standing rule treats a silently-lost table as a bug, not a phase boundary,
and this table is specifically corrections, the data least safe to drop.

**Consequences.** Implementation: a new `observation_relations` table
(schema identical in shape to `record_relations`); `AddObservationRelation`
and an `ObservationRelationsFor` mirroring `AddRecordRelation`/`RelationsFor`;
`cmdObservationUpdate` in `cmd/kb/observation.go`, dispatched alongside
`add`/`list`/`show`/`sources`; `cmdObservationShow` extended to resolve and
print relations; `MergeKnowledgeBases` and `jsonl.go` extended to carry the
new table, following the existing per-table pattern in each; `kb-observation(1)`
and its help text documented for the new subcommand and `show`'s relation
output. `ID` stays the observation's internal database id, matching
`show`/`sources` today — observations have never had a display identity the
way records have `DR-NNNN`.
