---
id: "0018"
title: "A record's project is matched across databases by name, and a divergence is reported on the path that aborts as well as the one that succeeds"
date: "2026-08-27"
status: accepted
kind: decision
trigger: implementation
project: knowledge
phase: "W4"
supersedes: []
superseded_by: []
relates_to: ["0011", "0013", "0016"]
initiative: ""
session: ""
decisions: ["Across two databases a record's identity is (workspace, project name, scope, record_id), not the identity index's project_id: a local autoincrement key is not comparable between databases", "Matching the project by uuid is rejected because it is order-dependent -- record collisions would appear only after the project collision was reconciled, so the same two databases would report different collisions before and after -force", "NameCollision becomes IdentityCollision and its Name field becomes Label, since what identifies an entity is a name for projects and concepts and a four-part tuple for records", "ReconcileCollisions locates b's row by uuid alone, because every table carrying a uuid has a UNIQUE index over it; identity is CollisionReport's problem, not the update's", "DivergenceReport is a separate function rather than a second return value from CollisionReport, so each answers one question and a caller can ask either", "A divergence is reported on the abort path as well as the success path, folded into the error message, because when something else blocks the merge the divergence is still true", "A record held by two machines is essentially always an identity collision, because each machine's ingest mints its own uuid for the same file, so -force is the ordinary path for records rather than the exceptional one"]
tags: [merge, collisions, records, identity, portability]
uuid: "01a045b4-8f1a-7b90-913e-ff428ad7f1ee"
origin_host: "MACMINI-RD.local"
---

**Context.** W4 generalises the collision machinery from `projects`/`concepts` names to records, and adds the content-divergence report DR-0013 specified. DR-0013 named records' identity as `(workspace, IFNULL(project_id,-1), scope, record_id)` — the identity index DR-0011 built. Writing the query showed that identity is right within one database and wrong between two.

`project_id` is an autoincrement key. Comparing `a.records.project_id` with `b.records.project_id` compares two unrelated id sequences, so a's `harvey` and b's `antenna` are "the same project" whenever they happen to have landed on the same number, and a's `harvey` and b's `harvey` are different projects whenever they did not. The identity index is not wrong; it identifies a row within the database that holds it, which is all an index has to do. Cross-database identity is a different question that happens to have been phrased in the same words.

The obvious repair — match the project by its uuid — is worse in a way that only shows up when the two reports are run in sequence. Two machines that each created `harvey` independently hold different project uuids until `-force` reconciles them. Matching records by project uuid therefore finds no record collisions before reconciliation and finds them after, so `CollisionReport` would describe the same pair of databases differently depending on whether a previous run had already fixed something.

**Decision.** Across databases, a record is identified by `(workspace, project name, scope, record_id)`, with the workspace tier's absent project matched as the empty string. Name is what the collision machinery already treats as the stable cross-machine key for a project — that is the premise `projects.name` collisions rest on — and it does not move when uuids are reconciled.

`NameCollision` becomes `IdentityCollision` and its `Name` field becomes `Label`, holding a name for projects and concepts and `DR-0007` for a record. `CollisionReport` gains a per-table query rather than one template, because what identity means now differs by table. `ReconcileCollisions` drops its name predicate and locates b's row by `uuid` alone.

`DivergenceReport` is a separate exported function. The CLI calls both, prints divergences to `out` on a successful run, and folds them into the error message when a collision aborts. The `--json` envelope gains `content_divergences` alongside `collisions_reconciled`.

**Rationale.** Reconciling by uuid alone is what let identity generalise without the collision struct having to carry a tuple. Every table that has a uuid has a UNIQUE index over it, so the uuid already locates exactly one row in b; the old `WHERE name = ? AND uuid = ?` was belt and braces over a key that was sufficient on its own. Recognising that turned "generalise the identity into the update statement" into "delete half the predicate", and the struct stayed four fields wide.

Keeping the two reports separate follows from what they are for. A collision is actionable — `-force` resolves it. A divergence is informational — nothing resolves it but a person editing two Markdown files. Folding them into one call would produce a result whose two halves mean different things to the caller, and the CLI wants to present them differently anyway.

Reporting a divergence on the abort path took a moment's thought because the existing design is deliberate about `out`: it is reserved for a successful run's narration, and collision detail goes in the error message precisely so it survives into both the text-mode line and the JSON error envelope. A divergence is subject to the same constraint for the same reason, and it does not become untrue because a collision stopped the merge — so it goes in the message too, under its own heading.

**Rejected alternatives.** *Use the identity index verbatim, as DR-0013 said.* It compares two databases' autoincrement sequences and produces both false matches and false misses. *Match the project by uuid.* Correct in a single-shot sense and order-dependent across runs, so the same inputs report different collisions before and after `-force`. *Keep `NameCollision` and add a parallel `RecordCollision`.* Two mechanisms for one concept, and the CLI would have to print and reconcile both. *Return divergences from `CollisionReport`.* One call, one ATTACH, and a return value whose halves are actionable and informational respectively. *Block the merge on a divergence.* DR-0013 decided against it and nothing here changed that; the implementation makes the cost of the decision visible, since a divergence is now known to be the common case rather than the rare one.

**Consequences.** Implemented; `go build`, `go vet`, `gofmt` and the full suite are clean. Nine tests: records colliding on identity, identity surviving differing local project ids, two projects' DR-0007 not colliding with each other, reconciliation preserving both sides, divergence detected and not detected, divergence reported without blocking, divergence surviving a collision abort, and divergence in the JSON output.

The finding worth carrying forward is operational rather than structural. A record that exists on two machines will essentially always be an identity collision, because the same file ingested on each machine gets a fresh uuid from `AddRecord` on each. So for records, `-force` is not the exceptional path — it is the ordinary one, and the first real cross-machine merge should expect a collision line per shared record rather than treating one as a sign that something is wrong. A test fixture written on the assumption that a divergence could occur alone had to be rewritten once this became clear.

That also sharpens what the divergence report is for. Since the collision list will be long and mechanical, the divergence list is the short one worth reading: it names the records whose text actually differs between the two machines, which is the set a person has to reconcile by hand. `kb-merge.1.md` still describes only the collision behaviour and needs both, which is W6's documentation item.
