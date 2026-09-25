---
id: "0047"
title: "kb adopts the workspace exit-code convention and supersedes DR-0040"
date: "2026-09-25"
status: accepted
kind: decision
trigger: request
project: knowledge
phase: ""
supersedes: ["0040"]
superseded_by: []
relates_to: ["0040", "0041", "0043", "0044", "0045", "0046"]
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a0d996-d097-768f-aa63-8d4250ea6355"
origin_host: "wren"
---

**Context.**

DR-0040 made a mistake in the command line exit 2 and left everything else at 1,
and named its own open questions: whether 1 should be split further, that the
root `CLAUDE.md` puts I/O errors at 2, that a value the library rejects stays 1,
and that nothing forces a new verb to classify its errors. RSDOIEL filed them as
a TODO item and on 2026-09-25 decided to split the codes now, following the POSIX
conventions where sensible, so scripts and embedded callers can tell a usage
error from an I/O error and from the rest. The convention itself is workspace DR-0003.
This record applies it to `kb`.

A survey of the v0.0.13 binary on a scratch database found what one code for
everything costs. Not found, no results, a value the library rejects, a refused
`source remove`, a missing workspace, an unreadable database and a missing merge
input all exit 1. The workspace's skill scripts branch on success or failure and
print "not found" for every failure, including a broken database. A bulk `kb
ingest` with two bad records out of three exited 0 and printed two errors. The
code has 171 error constructions in `cmd/kb` and 215 in the library, and one
typed error (`ConceptInUseError`), so most of them are plain `fmt.Errorf`.

**Decision.**

1. `kb` returns the codes of workspace DR-0003 and no others: 0, 1, 2, 65, 66,
   69, 70, 73, 74, 75, 77 and 78 (78 is not currently reachable, as `kb` has no
   configuration file). This supersedes DR-0040, in full, once accepted.
2. What `kb` returns, by situation:

   | Situation | Code |
   |-----------|------|
   | Success, including a listing that matches nothing | 0 |
   | `search` finds nothing; `show` or a lookup of a name or id that does not exist (`project show nosuch`, `--project nosuch`); a `record list` filter value no record carries and no vocabulary lists (DR-0043); `index --check` reports a stale index; an operation the current state forbids (`concept delete` or `source remove` while linked, without `--force`) | 1 |
   | Unknown verb, subverb or flag; missing or surplus argument; a missing required flag; any bad value passed on the command line: one that does not parse (`--since yesterday`, a malformed id), one outside a closed vocabulary (observation kind, project status, `--set`, `--accept`, `record new --trigger` or `--kind` that no record carries), a blank or control-character name, a malformed `--published`, `--url` or `--doi`; the refusals of DR-0041 | 2 |
   | Content the command reads is wrong: a malformed record file, frontmatter, JSONL or document; a file that is not a database; an identity collision in `merge` without `-force` | 65 |
   | A named input or the workspace is missing (`ingest /nonexistent`, a missing `merge` input, no `agents/knowledge.db`), or is a directory where a file is needed and the reverse | 66 |
   | A network service is unreachable (`source check-retractions`) | 69 |
   | An error nothing classified | 70 |
   | An output cannot be created (`merge -out` exists, `init` cannot make its directory, a record file already exists) | 73 |
   | A read or write failed part way (database file, record file) | 74 |
   | The database is locked | 75 |
   | Permission denied | 77 |

3. The dividing line is where the bad value came from. A value passed on the
   command line that no database could make acceptable is 2: it does not parse, is
   outside a closed vocabulary, or is blank. This is DR-0040 item 2's own test, and
   it keeps every code that is 2 today, `--since`, `--set`, `--accept` and
   `--confidence` included. It also settles DR-0040's borderline case: a value the
   library used to reject with 1 (an unknown observation kind or project status, a
   blank name) becomes 2, since the CLI passes it straight through. Content the
   command reads from a file or database is 65. A value that is well formed but
   might be acceptable to a different database is a lookup: a name that is not
   there, or a `record list` filter nothing carries, is 1, as DR-0043 decided, and
   its exit code is unchanged. `record new --trigger` and `--kind` share that
   vocabulary check with the filters but are values being written, not searched
   for, so they exit 2.
4. Mechanism. `cmd/kb/usage.go`'s `usageError` becomes one case of a class-carrying
   error: a small type holding a class from DR-0003's table, with constructors
   (`usageErrorf` stays). `dispatch` maps the class to its code in one function.
   The library gains typed errors, matched with `errors.Is` and wrapped with `%w`:
   `ErrNotFound` and `ErrInvalid` for the two classes it produces itself, and it
   keeps `ConceptInUseError` (class 1). A library `ErrInvalid` that reaches
   `dispatch` from an argument is class 2; the verbs that process files (`ingest`,
   `document ingest`, `import`, `merge`, `index`) wrap the library's error as class
   65 where the bad value came from the content. Errors from the standard library
   classify
   themselves: `fs.ErrNotExist` is 66, `fs.ErrPermission` 77, `fs.ErrExist` 73,
   other `*fs.PathError` 74, a `net.Error` or `*url.Error` 69, SQLite busy or
   locked 75, SQLite "not a database" or corrupt 65, any other SQLite error 74. An
   error that matches none of these is 70, not 1, so a gap shows up as an
   "internal error" in a test instead of hiding as a negative answer.
5. `printError`'s JSON envelope keeps going to stderr with stdout empty, as in
   DR-0040, and gains `class` and `code` fields (`"class": "no_input"`,
   `"code": 66`).
6. Bulk commands process everything, then exit with the class of the first failure
   and print the counts. `kb ingest` and `kb document ingest` exit 65 when a file
   was malformed and 74 when one could not be read; `source check-retractions`
   exits 69 when the service was unreachable for any source, after checking the
   rest. They no longer exit 0 with failures. The good items are still written.
7. The manual's EXIT STATUS table lists exactly these codes in the same words, the
   upgrade notes list them for scripts, and the skill scripts that read `kb`'s
   exit status (`review-knowledge-base`, `update-knowledge-base`,
   `setup-knowledge-base`) are updated so that "project not found" means 1, not
   any failure.
8. A table-driven test runs every registered verb with a bogus flag and a surplus
   argument and asserts 2, and a second test provokes one failure of each class
   and asserts its code. A third asserts that none of the provoked failures
   produces 70. Together they close DR-0040's gap: a new verb that returns a plain
   `fmt.Errorf` for a bad argument is now a failing test.

**Rationale.**

One call to `kb` now gives a wrapper what it needs in a single comparison: 1 means
the question was answered in the negative and a retry or a fallback is sensible;
2 means fix the caller; 65 means fix the data; 66, 73, 74, 75 and 77 mean the
environment, with 75 the one worth retrying; 70 means report a bug. Classifying
by where the value came from gives the closed vocabularies an honest code without
a taxonomy and without moving any code that is 2 today: two typed errors cover
everything the library itself raises, and the standard library classifies the
rest. Defaulting an unclassified
error to 70 is deliberate: DR-0040's own consequence was that a new error path
silently exits 1, and the same silent failure would recur with any other default.

**Rejected alternatives.**

- *Edit DR-0040 in place.* DR-0040 says a later change to a code needs a new
  record, and an accepted record is history. This record supersedes it.
- *Pure `sysexits` with usage 64.* Rejected in DR-0003.
- *A dedicated partial-failure code.* Rejected in DR-0003; the class of the
  failure plus the counts carries the same information.
- *Default an unclassified error to 1.* Restores the silent failure this record
  exists to end.
- *Classify by message text.* 171 and 215 sites would depend on wording that
  changes; a class on the error does not.

**Consequences.**

Breaking for scripts, and pre-1.0 so acceptable. A script that tests for exactly 1
to mean any failure now sees 2, 65, 66, 69, 70, 73, 74, 75 or 77 for the classes
above (an unknown observation kind or project status, and a blank name, move from
1 to 2), and one that tests for "non-zero" is unaffected. `search` and `project show`
keep 1, so the skill scripts' `|| true` and "not found" branches still work, and
are corrected to stop misreporting a broken database. A release adopting this needs an upgrade note
that lists the table.

The effort is large: about 390 error sites, but the change is mostly at the edges.
The library needs its two typed errors and a handful of construction sites
(validation, the not-found returns); the CLI's 171 sites are classified by
constructor. The plan (`exit-codes-plan.md`) puts the classifier and its tests
first, so each later phase can be checked against a table.

Things this record does not do. It does not change harvey, which uses the library
and reads errors, not codes. It does not edit the root `CLAUDE.md`; that follows
DR-0003's acceptance. It leaves `kb`'s stdout and stderr contents alone apart from
the envelope fields. It does not decide the exit code of a `kb` embedded in a
pipeline that closes early (a broken pipe), which is 74 by the classification above
and untested.

Settled in review with RSDOIEL, 2026-09-25, and worth recording because the first
draft of this record got it wrong. That draft made every value outside a vocabulary
65, which contradicted its own dividing line and would have moved `--set` and
`--accept` from 2 to 65 without saying so (found by probing them: both exit 2
today). The rule above replaced it. A refusal the current state forces (a concept
still linked) is 1, RSDOIEL's choice over 65 and over a dedicated code.

Status `proposed`: promotion is the author's call. The supersession of DR-0040 is
not recorded in either file yet: on acceptance, set it with `kb record supersede`.
