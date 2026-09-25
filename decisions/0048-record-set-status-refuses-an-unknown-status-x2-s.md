---
id: "0048"
title: "record set-status refuses an unknown status; X2's amendments to DR-0047"
date: "2026-09-25"
status: accepted
kind: decision
trigger: request
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0043", "0044", "0045", "0047"]
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a0d9c7-ec05-7e86-a5dc-fc03683995e2"
origin_host: "wren"
---

**Context.**

X2 of `exit-codes-plan.md` classified every error site in `kb` under DR-0047 and,
in doing so, found what the classification could not settle alone. This record
records the choices X2 made where DR-0047 was silent or said something narrower,
and one behaviour RSDOIEL decided on 2026-09-25 after being asked.

`kb record set-status ID bogus` wrote the status into the record file and the
database and exited 0. `set-status` is the promotion path (proposed to accepted),
so a typo such as `acepted` leaves a record in limbo: no `record list --status
accepted` finds it, and once one record carries the typo, DR-0043's rule stops
`record list --status acepted` from erroring. `Accepted` (capital A) was written
as-is. An empty status was already refused. The real database's records carry
only `accepted` (48), `proposed` (4) and `superseded` (1), all in the vocabulary.
The manual's reason for calling the vocabularies "reported, not enforced" is that
"a typo in a file several harnesses write should be a fixable row, not a failed
run": a reason about reading files other tools wrote, not about a command kb runs.

**Decision.**

1. `record set-status` checks its status before it looks up the record or touches
   a file, with the rule `record new` applies to `--trigger` and `--kind` (DR-0045)
   and `record list` to its filters (DR-0043): a status outside `RecordStatuses`
   is an error unless a record already in the database carries it. It is a value
   being written, so the error is a usage error (exit 2) that lists the known
   statuses. An empty status is refused the same way. The check is the shared
   `checkRecordVocabulary`. The file and database are left untouched. Ingest of a
   hand-edited file with an unknown status still only warns. A new status enters
   the vocabulary by a decision record adding it to `RecordStatuses`, as DR-0038
   did for `cancelled`.
2. The library gains a third marker, `ErrInUse`, for a refusal the current state
   forces on an item that is still referenced (a source linked to an observation).
   DR-0047 item 4 named two library markers, `ErrNotFound` and `ErrInvalid`, and
   kept `ConceptInUseError` for concepts; `RemoveSource` needed the same for
   sources without inventing a second typed error. It classifies as negative (1).
3. A zero-byte `merge` input is 65 (content that is not a knowledge base), and a
   missing input, a directory or a non-regular file is 66. DR-0044's refusals fit
   DR-0047's table only by inference; this states it.
4. Failures before a verb runs (the debug log, the database path, opening the
   database, no workspace) are printed and classified like any other error, so
   they honour `--json` and carry a class; they were plain text and always exit 1.
5. Two helpers give the file verbs their classes. A library `ErrInvalid` that came
   from reading a file (a malformed record, frontmatter, document or JSONL) is 65,
   as are JSON decode errors; the same `ErrInvalid` from an argument stays 2. A
   failure to create an output is 73, except a permission failure (77) and a failure
   part way through writing (74).
6. The library's `RemoveSource`, `RetractSource` and the `Link*` functions return
   `ErrNotFound` for an id that is not there. `source retract 99` and `source remove
   99` used to exit 0 (zero rows changed), and `source link` and `link observation`
   with a missing id failed with a raw foreign-key error (74).
7. `concept suggest --limit` refuses a negative value (usage, 2).

**Rationale.**

For `set-status`, consistency is the argument that carries it: one rule, one
function, one thing to remember for every value kb writes into a record. The
reason the vocabularies are lenient does not reach this command, and the cost of a
typo here is the highest of any field. The carried-value clause keeps a database
with a legacy status usable, and costs nothing where none exists. Items 2 to 7 are
the smallest choices that let DR-0047's table apply without exceptions, each found
by provoking a failure rather than by reading code.

**Rejected alternatives.**

- *Warn on stderr, still write, exit 0, for `set-status`.* Matches ingest, but the
  typo still lands in the file and the record stays in limbo.
- *Refuse strictly, with no carried-value clause.* Simpler to state, but different
  from the rule for trigger and kind, and it would refuse a legacy status already in
  a database.
- *Leave `set-status` as it was.* The exit 0 for a typo is the same "mistake
  answered with success" DR-0040 and DR-0047 exist to end.
- *A second typed error for a linked source.* A third marker in the library is
  smaller than a new error type per entity.

**Consequences.**

A script that sets a status outside the five and that no record carries now fails
with exit 2 where it used to succeed; using a carried legacy value still works.
`source retract` and `source remove` on a missing id, which reported success, now
fail with exit 1. `--json` callers now see an error object, not text, for
failures before a verb runs. These need an upgrade note with DR-0047's table.

No real record is affected: all 53 carry a vocabulary status. The classification
pass also confirmed, on 35 commands against the v0.0.13-equivalent binary, that
every changed exit code is a row of DR-0047's table and none is 70.

Left as they are, and worth knowing. `kb project add NAME description --status
paused` succeeds and stores "description --status paused" as the description, since
DR-0041 treats words after the fixed arguments as free text; the flag belongs before
the name. `kb project add` on an existing name prints "added" for a project that
already existed. Neither is an exit-code question. `kb ingest` still exits 0 when
records fail and `source check-retractions` does not yet use 69: both are X3.

Status `proposed`: promotion is the author's call.
