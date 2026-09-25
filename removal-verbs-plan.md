# Removal verbs: implementation plan

Implements DR-0050 (`proposed`); see `removal-verbs-design.md`. TDD: every test is written and
confirmed red before its code. One commit per phase, made only when RSDOIEL asks. Every new exported
symbol gets the `/** */` block (description, parameters, returns, example).

## R1: the library

`removal.go`, `removal_test.go`. `InUseError`; `UnlinkProjectConcept`, `UnlinkObservationConcept`,
`UnlinkObservationSource`; `ProjectUsage`/`DeleteProject`; `ObservationUsage`/`DeleteObservation`;
`DocumentUsage`/`DeleteDocument`; `RecordUsage`/`DeleteRecord`.

**Red tests first:** for each delete, a missing target is `ErrNotFound`; an in-use target is
`ErrInUse` and nothing changes (row counts identical); `force` removes the links and the row and the
`kb_fts` entry; a project that owns content is refused even with `force`; a document with a reviewed
section is refused and one without is not; each unlink removes exactly one link and a missing link is
`ErrNotFound`. Nothing outside the target is touched (other rows' counts unchanged).

## R2: the commands

`cmd/kb/unlink.go`, and `delete` subverbs in `project.go`, `observation.go`, `document.go`, `record.go`.
`--force` and `--dry-run` as `concept delete`; `--` for a dash-leading name; JSON result with the
counts. `record delete` stats the file first.

**Red tests first:** each verb's success, `--dry-run` changes nothing, refusal exits 1 with what and how
many and changes nothing, `--force` where it applies, a missing target is 1, a bad id is 2, `--json`
shape. `record delete` with the file present is refused and with it gone deletes; a project that owns
content is refused with `--force`. Cases added to `exitcodes_verbs_test.go`.

## R3: help and the registry

SYNOPSIS lines and descriptions on each verb's page; `UnlinkHelpText`, `printHelp` case, `kb-unlink.1.md`,
`KB_TOPICS` in the Makefile (`help_dispatch_test.go` enforces it, and that the verb appears in `kb(1)`),
the topics page. The SYNOPSIS-derived every-command tests reach the new commands with no edit.

## R4: verification

Full suite; the every-command tests; `scripts/compare-exit-codes.py` against the v0.0.13 build (no
existing exit code may change; the new commands appear as additions). A smoke test of each verb on a
copy of the real database, then a check that `sqlite3` row counts and `kb search` agree afterwards
(the search entry must go with the row).

## Status (2026-09-25)

R1 to R5 done, uncommitted. R1: `removal.go` + `removal_test.go` (mutation-checked: force on a project that owns
content, the search entry). R2: `cmd/kb/removal.go`, `unlink.go`, `removal_cmd_test.go` (mutation-checked: the
record file check, refusing a missing link); the delete subverbs are wired in `project.go`, `observation.go`,
`document.go`, `record.go`. R3: SYNOPSIS lines and descriptions on the four pages, `UnlinkHelpText`, `printHelp`,
`KB_TOPICS`; the registry-driven every-command tests reached the new commands with no edit and pass. R4: full
suite; `scripts/compare-exit-codes.py` (VERBS gained `unlink`): 55 changed codes, the existing 48 plus 7 that are
the new commands (2 to 1, and 2 to 0 for `observation delete 1` on the copy), no 70; smoke test on a copy of the
real database in a workspace named `Laboratory`: stray project deleted and gone from search, `harvey` refused
(163 observations, 3 records, 1 document) even with `--force`, an observation link/refuse/unlink/delete cycle,
the real document refused (8 reviewed summaries), a record refused while its file existed and deleted once it
was gone (`records` 56 to 55, `kb_fts` 527 to 526). R5: release notes, `CHANGES.md`, derived files, 17 man pages,
the four-field check, and the `update-knowledge-base` skill (new actions `project-delete`, `observation-delete`,
`document-delete`, `record-delete`, `unlink`; skill version 0.6.5).

## R5: release prep, again

`CHANGES.md`, `codemeta.json` releaseNotes and the four fields, regenerated derived files and man pages,
the `update-knowledge-base` skill's verb list, and the four-field check.
