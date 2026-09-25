---
id: "0040"
title: "kb exits 2 for a mistake in the command line and 1 for everything else"
date: "2026-09-24"
status: accepted
kind: correction
trigger: live-test
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0010", "0041"]
initiative: ""
session: ""
decisions: []
tags: [cli, exit-status, v0.0.13]
uuid: "01a0d5ac-318c-7219-a42d-0ad36f3ec7c4"
origin_host: "wren"
---

**Context.**

`kb -help` has always documented two failure codes: 1, "the verb ran but failed
(database error, not found, etc.)", and 2, "usage error (bad flags, unknown
verb)". Only the last of those ever happened: an unknown top-level verb exited 2,
and every other mistake in the command line (a missing argument, an unknown
subverb, an unknown flag, a malformed id, a flag with no value) exited 1, the
same as "not found". A script therefore could not tell "fix the command" from
"handle the result".

Found in the v0.0.13 bug sweep, which ran every verb with empty, malformed,
unknown and surplus input against a copy of the real database. Of 67 commands
run through the v0.0.12 binary and the new one, 35 changed from 1 to 2, every
one a real mistake in the command line, and the other 32 kept their codes.

**Decision.**

1. A mistake in the command line is a `usageError` (`cmd/kb/usage.go`:
   `usageErrorf`, `wrapUsage`, `isUsageError`, and `parseFlags`, which marks a
   FlagSet parse failure). `dispatch` exits 2 for one and 1 for any other error.
   `flag.ErrHelp` is never a mistake: it is answered with the page and exits 0.
2. The dividing line is whether the command line is wrong on its face, without
   opening the knowledge base. That covers an unknown verb, subverb or flag; a
   missing or surplus argument; a flag without its value or with one that does
   not parse; a malformed id; a missing required flag; a value the command
   checks itself (`--set`, `--accept`, `--confidence`, `--since`); and the
   refusals in DR-0041 and DR-0044. Nothing was attempted.
3. Exit 1 stays for a command line that was fine and a result that was not: not
   found, no results, a database error, and a lookup of a name (`--project
   nosuch`, `project show nosuch`). It also stays for **a value the library
   rejects** after the command line parsed, such as an unknown observation kind
   or project status. That is the borderline case, and this record chooses 1.
4. `printError` is unchanged: the JSON envelope goes to stderr for both codes,
   and stdout stays empty.
5. `kb(1)` says exactly this under EXIT STATUS, in the same words, and the
   v0.0.13 upgrade notes list it for scripts.

**Rationale.**

The documented contract already promised 2; the code did not deliver it, so this
is a correction, not a new policy. A test on the dividing line is cheap to apply:
if a different command line would have succeeded whatever the database held, the
code is 2; if the answer depends on the data, it is 1. That is why an unknown
`--project` is 1 (a different database might have it) and a malformed `--since`
is 2 (no database could make `yesterday` a date).

The enum case goes to 1 because the check lives in the library, which returns an
ordinary error. Making it 2 would mean either duplicating the vocabulary in the
CLI or giving the library a typed invalid-value error, an error taxonomy for one
class of input.

**Rejected alternatives.**

- Everything the user got wrong exits 2, enum values included, via a typed
  `ErrInvalidValue` from the library. More uniform, and the honest answer for a
  closed vocabulary such as observation kind. It costs a taxonomy in the library
  and a decision about which of the library's errors qualify. Revisit if scripts
  need it.
- Change the documentation to say usage errors exit 1. Removes the
  inconsistency by deleting the distinction a script wants.
- `sysexits.h` codes (64 for usage, 66 for a missing input, and so on). Finer,
  but the published contract is 0, 1 and 2, and every other tool in this
  workspace uses small numbers.

**Consequences.**

A script that tested for exactly 1 to mean a mistyped command now sees 2; one
that tested for 1 to mean "not found" still sees 1. A script treating any
non-zero as failure is unaffected. About 75 error sites were converted, and a
table-driven test (`usageexit_test.go`) pins the classes, but nothing forces a
new verb to use `usageErrorf`: a verb added later that returns a plain
`fmt.Errorf` for a bad argument will exit 1 again until someone notices.

A tension worth reviewing: the workspace convention in the root `CLAUDE.md` (for
the Deno tools) is "1 when a search-style tool finds nothing, 2 on a usage or I/O
error". `kb` has always put I/O and database errors under 1 and reserved 2 for
usage, and this record keeps that, because it is what `kb -help` published in
v0.0.1. If the workspace convention should win, database and file errors would
move to 2, a larger change than this record makes.

Status `proposed`: promotion is the author's call.
