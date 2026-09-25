---
id: "0049"
title: "State conflicts and constraint violations in kb's exit codes (X5 findings)"
date: "2026-09-25"
status: accepted
kind: correction
trigger: live-test
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0047", "0048"]
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a0d9e7-ba47-7840-9538-ffdee50b9232"
origin_host: "wren"
---

**Context.**

X5 of `exit-codes-plan.md` ran the DR-0040 comparison for the exit-code work: the
v0.0.13 tree (`4c6fc3e`) against the current one, 387 commands in plain and `--json`
mode, each on a fresh copy of a workspace holding the real `knowledge.db`, copies of
the real decision records and document, and some bad inputs. 331 were identical, and
the 49 changed exit codes were each a row of DR-0047 or DR-0048, apart from what
follows. The real database and repositories were checked unchanged afterwards.

*Regressions from X2.* Four commands exited 1 in v0.0.13 and 70 ("internal") after
X2, because the library raised a plain `fmt.Errorf` for a refusal, and X2's own
matrix had no case for a state refusal to reveal it: `concept rename RSS Markdown`
when `Markdown` exists; a `project rename` of a project that owns records;
`document review promote` on a section that is not drafted; and `document frontmatter
--accept-keywords zzz` with a keyword that is neither a known concept nor a proposed
one. The first three are the operation being well formed and the current state
forbidding it. The fourth is a bad value on the command line.

*A wrong classification.* `kb ingest --root nosuchroot` (a workspace name that does
not match the one the records were ingested under, so no record is recognised by
identity) tries to insert every record, and every insert fails with `UNIQUE
constraint failed: records.uuid`. v0.0.13 printed "3 failed" and exited 0; X3 made
it exit non-zero, and DR-0047 item 4 ("any other SQLite error is 74") made that 74,
"a read or write failed part way". A constraint violation is not an I/O failure. It
is data that conflicts with what the schema or the stored rows allow, and a script
told "74" would look for a disk problem.

*A counting error the same run exposed.* The summary for that run read `2 added, 0
updated, 0 skipped, 2 failed`: a record that failed to insert was counted as added
before the insert was tried, and again as a failure. v0.0.13 did the same (`48
added ... 48 failed`).

**Decision.**

1. The library gains a marker, `ErrConflict`, for an error raised because the
   current state of the knowledge base forbids an operation that was otherwise well
   formed: a rename onto a name already taken (`RenameConcept`, `RenameProject`), a
   project that still owns records, a document section not in the state a promotion
   needs. `kb` exits 1 for it, the class "negative" of workspace DR-0003: the command
   ran correctly and the answer is no. `ErrInUse` (DR-0048) stays for an item that is
   still referenced, and is also exit 1. This is a fourth library marker beyond the
   two DR-0047 named and the one DR-0048 added; each marks a distinct condition and
   all four are matched with `errors.Is` without changing a message.
2. An unknown `--accept-keywords` keyword is an `ErrInvalid` (a bad value on the
   command line, exit 2), not an internal error.
3. A SQLite constraint violation (primary result code 19: `UNIQUE`, `PRIMARY KEY`,
   `NOT NULL`, `CHECK` or `FOREIGN KEY`) is class data, exit 65. This amends DR-0047
   item 4: busy or locked is still 75, a corrupt file or one that is not a database
   is still 65, and any other SQLite error is still 74. `ingest --root nosuchroot`
   now exits 65.
4. A record that fails to insert during `ingest` is counted as a failure only, not as
   added as well.
5. A plain error that nothing classified is still 70, and stays so for programmer
   errors: an unsupported Go value type in the frontmatter writer, a bad field name
   passed to `DistinctRecordValues`, and "full-text search is not available". Those
   are bugs or a broken build, not conditions a caller can act on.

**Rationale.**

Refusals by state are what DR-0047 already calls negative answers, and 70 for them
was a regression from v0.0.13, found only because X5 compared against it on real
data. Marking them at the source, as `ErrNotFound` and `ErrInUse` do, keeps the
classification out of message text. For constraints, the rule "the caller supplied
data that conflicts with what is stored or with the schema" is what 65 already means
for a malformed record or a colliding identity, so grouping every constraint code
under it is simpler and more honest than naming `UNIQUE` alone. A `NOT NULL` failure
on ingest is a record missing a field; a `FOREIGN KEY` failure is a reference to a row
that is not there. The double count is a plain arithmetic error that a wrong-root run
made visible.

**Rejected alternatives.**

- *Keep constraint violations at 74.* Sends a caller looking for a disk fault.
- *Map only `UNIQUE` and `PRIMARY KEY` to 65.* Leaves `NOT NULL` and `CHECK`, which
  are also bad data, at 74, and the rule harder to state.
- *Map constraints to 70.* A constraint violation from ingesting the wrong root is
  the caller's to fix, not a bug to report.
- *Reuse `ErrInUse` for conflicts.* The name says "still referenced". A rename onto a
  taken name and a wrong-state promotion are not that, and a reader meeting
  `ErrInUse` there would be misled.
- *Leave the four regressions at 70.* They exited 1 in v0.0.13.

**Consequences.**

A wrong `--root` now exits 65 with the constraint named, and the summary counts the
records as failed only. A script that treated any non-zero from `ingest` as failure is
unaffected. The four X2 regressions return to exit 1 (or 2 for the keyword), as in
v0.0.13. `kb`'s EXIT STATUS help gains the conflicts and constraint wording. Tests:
`errors_test.go` (each conflict), `exitcode_test.go` (constraint codes and real
`UNIQUE` and `NOT NULL` errors from the driver, mutation-checked),
`exitcodes_verbs_test.go` (the three state refusals and the keyword), and
`ingestexit_test.go` (wrong root, and the count).

The method is worth keeping: X2's own matrix missed a class of failure, and only a
comparison against the last release on real data with a wide failure set found it. The
comparison scripts live outside the repository today (in a scratch directory); one
workspace-specific detail matters to anyone rerunning them, which is that the run
directory must be named as the real workspace is, or `ingest` reports spurious `UNIQUE`
failures for a reason unrelated to the code under test.

Status `proposed`: promotion is the author's call.
