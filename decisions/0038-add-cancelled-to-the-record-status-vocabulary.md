---
id: "0038"
title: "Add cancelled to the record status vocabulary"
date: "2026-09-23"
status: accepted
kind: decision
trigger: request
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a0d0b6-b5d4-72e8-8bb3-5a35c20bcad4"
origin_host: "wren"
---

**Context.**

Raised 2026-09-21 from a real case in `~/WorkLab`: a record of eleven decisions
was reviewed and accepted, then the issue was cancelled the same day because the
need was already met. None of the four statuses told that story. `rejected`
means the decisions were never adopted, and these were. `superseded` needs a
replacement record, and writing one just to get the status inflates the corpus
with a record whose only content is "this did not happen". `accepted` leaves a
reader six months out believing the work is live. `proposed` is false. The
workaround was a short cancellation record superseding the original. Vocabularies
are documented, not enforced, so `set-status ... cancelled` already worked, but
it parsed with an out-of-vocabulary warning, and the value would have spread as
an undocumented convention.

**Decision.**

1. `cancelled` joins `RecordStatuses`: proposed, accepted, superseded, rejected,
   cancelled. It means adopted, then abandoned. The distinction from `rejected`
   is temporal, and it matters because a cancelled record's reasoning stays
   valuable: it is where someone revisiting the question should start.
2. The reason a record was cancelled goes in its body, by convention, with no
   frontmatter field: "the need was met another way" and "deprioritised" tell a
   later reader different things, but a field would be overkill.
3. `index.md` needs no change: it already renders the status column, and a test
   pins that.
4. It is documented in `kb help record`'s VOCABULARIES section, which a test
   already requires to list exactly what the code accepts.

**Rationale.**

The cheapest change that stops an undocumented value spreading. The lifecycle
code has no status-specific behavior beyond `superseded`, so nothing else moves.

**Rejected alternatives.**

- Leave it as an undocumented convention: the parse warning would have told
  every author their record was wrong.
- A `cancelled_reason` field: overkill, and it would need its own vocabulary.
- Write a superseding record: the interim workaround, and the second bullet of
  the context above.

**Consequences.**

A corpus can now say adopted-then-abandoned without a replacement record.
`DECISION_RECORD_FORMAT.md` (cited by the workspace `CLAUDE.md` at
`~/WorkLab/`, absent on this machine) still needs the same line wherever it
lives. Status `proposed`: promotion is the author's call.
