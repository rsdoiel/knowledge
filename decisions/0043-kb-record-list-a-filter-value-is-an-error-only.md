---
id: "0043"
title: "kb record list: a filter value is an error only when nothing carries it"
date: "2026-09-24"
status: accepted
kind: decision
trigger: live-test
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0038", "0040"]
initiative: ""
session: ""
decisions: []
tags: [cli, record, filters, vocabulary, v0.0.13]
uuid: "01a0d5ac-46eb-71b7-bda0-33f50eda9ce0"
origin_host: "wren"
---

**Context.**

Every `record list` filter answered a mistake with `no matching records` and exit
0, the same as a real filter that happened to match nothing. `--status acepted`,
`--project clasn`, `--since yesterday` (compared as text, so it matched nothing)
and `--workspace --project P` (workspace-tier records have no project) all read as
an empty result, and under `--json` as `[]`.

The obvious fix, rejecting any value outside the documented vocabulary, is wrong
here. The vocabularies for `status`, `kind` and `trigger` are documented, not
enforced: a value outside them parses with an ingest warning and is stored
(`DECISION_RECORD_FORMAT.md`, `recordfile.go`, and DR-0038, which added
`cancelled` because `set-status ... cancelled` already worked with only a warning,
and the value would otherwise have spread as an undocumented convention). So a record can legitimately carry a status the vocabulary does not list,
and a blanket rejection would make it unlistable.

**Decision.**

1. A `--status`, `--kind`, `--trigger` or `--initiative` value is accepted if it is
   in the documented vocabulary **or** carried by at least one record in the
   database. Otherwise it is an error naming what is known (exit 1, DR-0040).
   `DistinctRecordValues(field)` supplies the carried values.
2. A value inside the vocabulary that no record carries is a valid filter and
   answers `no matching records` (exit 0, `[]` under `--json`).
3. `--project` naming no project is an error listing the known ones (exit 1). An
   existing project with no records is a valid, empty answer.
4. `--since` must be a real `YYYY`, `YYYY-MM` or `YYYY-MM-DD` (exit 2). The column
   is compared as text, so the partial forms are useful and stay allowed.
5. `--workspace` together with `--project` is refused (exit 2).
6. The validation is in the CLI (`validateRecordFilters`), before the query. It is
   not in `ListRecords`, which library code and Harvey also call and which keeps
   its "zero-valued fields match anything, an unmatched value matches nothing"
   contract.
7. `DistinctRecordValues` accepts only `status`, `kind`, `trigger` and
   `initiative`, because the field name is spliced into SQL.

**Rationale.**

The rule separates the two things the old code conflated without ever refusing a
value the data really carries. A typo is almost never both outside the vocabulary
and carried by a record, and a value that is both is by definition not a typo in
this database.

Keeping the check out of `ListRecords` means the library's behaviour for its other
callers does not change, and the CLI is the layer that knows a human typed the
value.

**Rejected alternatives.**

- Enforce the vocabularies (reject anything outside them). Makes carried
  out-of-vocabulary records unlistable, and contradicts the format's "documented,
  not enforced".
- Warn on stderr and still list. A script reading stdout sees `[]` either way.
- Validate `--project` only. It is the cheapest case but leaves the four others as
  silent as before.
- Put the check in `ListRecords`. Changes what every other caller gets for an
  unmatched value.
- Match case-insensitively, or suggest the nearest value. `Accepted` for
  `accepted` is a typo the message already answers by listing the known values;
  suggestions would be a second feature.

**Consequences.**

A script that relied on a mistyped filter returning an empty list now fails. On a
copy of the real 44-record database, over 25 filter combinations, 16 valid ones
were byte-identical to v0.0.12 (including the legitimately empty `--status
rejected`, `--kind refinement` and `--trigger external`) and the 9 mistakes became
errors.

The boundary is data-dependent, and that is the price: `--status weird` is an
error on a database where no record carries `weird` and a normal listing on one
where a record does. It only affects values outside the vocabulary, which is the
population that should be rare. No record in the real database carries an
initiative, so `--initiative X` is always an error there, which is accurate.

Status `proposed`: promotion is the author's call.
