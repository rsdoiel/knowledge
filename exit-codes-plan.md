# Splitting `kb`'s Exit Codes — Implementation Plan

Implements knowledge DR-0047 (`accepted` 2026-09-25), which applies workspace DR-0003
(the convention, `accepted` 2026-09-25) and supersedes DR-0040 (`superseded`). Work items X0 → X6. TDD-first:
the test for each item is written and confirmed red before the code changes.
One commit per item, made only when RSDOIEL asks. Every new exported symbol gets
the `/** … */` block (description, parameters, returns, example).

Facts this plan rests on, measured 2026-09-25 on a scratch database with the
v0.0.13 binary: 171 error constructions in `cmd/kb`, 215 in the library, one typed
error (`ConceptInUseError`). Today every failure but a usage mistake exits 1;
`kb ingest` with two bad records of three exits 0. The skill scripts read only
success or failure, and treat any failure of `project show` as "not found".

The codes (workspace DR-0003): 0 ok, 1 negative, 2 usage, 65 data, 66 no_input,
69 unavailable, 70 internal, 73 cant_create, 74 io, 75 temp_fail, 77
no_permission, 78 config.

---

## X0 — The classifier

**Add:** in `cmd/kb`, a class type and one function `exitCodeFor(err) (class,
code)`. `usageError` (`cmd/kb/usage.go`) becomes one case of it; `usageErrorf`,
`wrapUsage` and `isUsageError` keep working, so nothing that uses them changes.
New constructors: `notFoundf` (1), `negativef` (1, for a refusal the current state
forces), `dataErrorf` (65), `noInputf` (66),
`cantCreatef` (73), `unavailablef` (69). `dispatch` and `printError` call
`exitCodeFor`; `printError`'s JSON envelope gains `class` and `code`.

**Red tests first (`exitcodes_test.go`):** a table of (error → class, code) for
each constructor; `errors.Is` through `%w` wrapping; `fs.ErrNotExist` → 66,
`fs.ErrPermission` → 77, `fs.ErrExist` → 73, a `*fs.PathError` → 74, a
`net.Error` and a `*url.Error` → 69, an unclassified `errors.New` → **70** (not 1);
`flag.ErrHelp` still exits 0; the JSON envelope carries `class` and `code`.

**Done when:** the table passes, and the existing `usageexit_test.go` passes
unchanged (2 stays 2).

**DONE 2026-09-25** (`cmd/kb/exitcode.go`, `exitcode_test.go`; `usage.go`, `dispatch.go`,
`output.go`, `main.go` wired). An unclassified error keeps the legacy exit 1 through
`unclassifiedFallback`, which X2 flips to `classInternal`. Contrary to what this plan
first said, X0 is not free of observable change: errors that wrap `fs.ErrNotExist` now
exit 66 (`document ingest` and `import` of a missing file, correctly; `init /proc/x` and
`export` into a missing directory too, which X2 refines to 73/77 with `cantCreatef`).
The JSON envelope gained `class` and `code`.

---

## X1 — Library typed errors and SQLite classification

**Add (root package):** `ErrNotFound` and `ErrInvalid` sentinels, wrapped with
`%w` by the library's own validation and not-found returns; `ConceptInUseError`
is kept and classed 1. The library sites: `IsValidKind` and project-status
checks, `CleanName`, `AddSource`'s checks (DR-0045), record vocabulary
warnings stay warnings, `RenameProject` and `RenameConcept` on a missing name,
`ImportJSONL` and `MergeKnowledgeBases` refusals. SQLite errors are classified in
the CLI's classifier by driver error code: busy or locked → 75, "not a database"
or corrupt → 65, any other → 74.

**Red tests first:** for each library function that rejects a value, `errors.Is(err,
ErrInvalid)` (the CLI maps it to 2 for an argument and to 65 for content a file
verb read: DR-0047 item 3); each that misses a name, `errors.Is(err, ErrNotFound)`; a database
held locked by a second connection gives 75; a text file opened as a database
gives 65. A test that the message text of each error is unchanged (the change
adds a class, not new wording).

**Done when:** all pass and `go test ./...` at the root is green with no message
changes.

**DONE 2026-09-25** (`errors.go`; `errors_test.go`; `sqliteClass` and the sentinel cases
in `classify`). Sites converted: the value checks (`CleanName`, project status,
observation kind and body, `AddSource`, frontmatter `--set`/`--accept` fields, record and
frontmatter parse errors) to `invalidf`; the missing-name returns (project, concept,
export, suggest) to `notFoundf`. `ImportJSONL`'s 15 line-error wrappers and the merge
refusals are left plain: the file verbs classify them at the call site in X2. Mutation
checks on `sqliteClass` are caught by the real-error tests (`SQLITE_BUSY` and `NOTADB`).
The two sentinels were enough; no third one for content was needed because the CLI's file
verbs wrap. **Known interim misclassification, to be closed in X2:** a library `ErrInvalid`
from file *content* exits 2 (usage) until the file verbs wrap it as data. Seen:
`kb index --check` on a directory holding a malformed record exits 2, not 65 (was 1).
The probe matrix (`scratchpad/probes.sh`, 36 commands) against the pre-X0 binary shows
only: `observation add bogus`, `project set-status bogus`, `project add ''` 1 to 2 (DR-0047);
`document ingest`/`import` of a missing file 1 to 66; `init /proc/x` and `export` into a
missing directory 1 to 66 (X2 makes them 73/77); and the `index --check` case above.

---

## X2 — The CLI's own sites, verb by verb

**Change:** the 171 `cmd/kb` sites, by constructor, in this order: (a) not found
(`project show`, `concept`, `source show`, `record show`, `observation`, `--project`
lookups) → `notFoundf`; (b) `search` finds nothing and `index --check` stale →
class 1; (c) bad values on the command line (observation kind, project status, blank or
control-character names, `--published`, `--url`, `--doi`, `record new --trigger` and
`--kind`) → usage, 2, alongside the `--set`, `--accept`, `--confidence` and
`--since` checks that are 2 already; content the file verbs read (`ingest`,
`document ingest`, `import`, `merge`, `index`) → `dataErrorf`, 65; (d) missing inputs (`ingest`,
`document ingest`, `merge` inputs, no workspace) → `noInputf`; (e) `-out` exists,
`init` mkdir, a record file exists → `cantCreatef`. `project`, `concept` and
`observation` are done first: they have the most callers.

**Red tests first:** extend `usageexit_test.go` into a per-class table:
one provoked failure per class per verb that can produce it, asserting the code
and the JSON `class`. DR-0043's case does not change: `record list --status acepted` and `--project
clasn` stay 1, and the existing test that pins "not a usage error" stays. What does
change, and is pinned deliberately: `observation add bogus`, `project set-status
bogus` and a blank name move from 1 to 2.

**Done when:** the table passes and no provoked failure yields 70.

**DONE 2026-09-25** (uncommitted). `cmd/kb/exitcodes_verbs_test.go` provokes one failure per
class per verb (about 70 cases) and asserts the exit code, the JSON `class`, and that nothing
is 70; `unclassifiedFallback` is now `classInternal`. Sites classified: not found (project,
concept, observation, source, record, document, link targets), `search` and `index --check`
negative results, refusals the state forces (`concept delete` and `source remove` while
linked, rename onto an existing name), bad command-line values (usage), missing inputs (66),
outputs that cannot be created (`asCreate`: 73, but permission stays 77 and a write that fails
part way stays 74), content errors (`asContent`: a library `ErrInvalid` from a file, plus JSON
decode errors, 65), and `mainRun`'s own failures (`failWith`: no workspace 66, open errors by
cause; they now honour `--json`). Deviations from DR-0047's wording, all small: a third library
sentinel `ErrInUse` (a source still linked) because the library needs to mark a refusal the way
`ConceptInUseError` does; `record new`'s vocabulary check is wrapped as usage while `record
list`'s stays a lookup; a zero-byte merge input is 65 (content that is not a knowledge base)
and a directory 66.

**Bugs found by the classification pass, fixed with tests:** `source retract 99` and `source
remove 99` on a missing id exited 0 (zero rows updated or deleted); `source link 99 99`
exited 74 with a raw foreign-key error and `link observation 99 C` likewise; `document
review promote 99` exited 70 (a plain "no document section" error); `concept suggest
--limit -1` was accepted. The library's `Link*` functions now check the ids and return
`ErrNotFound`; five plain "no X with id" errors became `notFoundf`.

**Decided by the user 2026-09-25 (DR-0048, `proposed`):** `record set-status ID bogus` is refused
with exit 2 by the shared vocabulary rule (was: written, exit 0); done, with tests
(`recordsetstatus_test.go`). DR-0048 also records X2's other choices (`ErrInUse`, zero-byte merge
input 65, `--json` for pre-verb failures) as amendments to DR-0047. The main EXIT STATUS help
section and `kb.1.md` were rewritten at X2. **Left for X3:** `kb ingest` still exits 0 when records
fail; `source check-retractions` network failures.

Matrix (35 commands, `scratchpad/probes.sh`), v0.0.13-equivalent binary against this one:
14 change, every one a row of DR-0047's table, none 70: no workspace 66; bad kind, bad status,
blank name 2; missing ingest, document ingest, import, merge and index inputs 66; `--db` on a
text file 65; `--db` in a directory the user cannot create 77; `init` and `export` cannot create
73; `index --check` over a malformed record 65.

---

## X3 — Bulk commands

**Change:** `kb ingest` and `kb document ingest` keep going after a bad file, then
exit with the class of the first failure (65 for malformed content, 74 for a read
failure) and print the counts; `source check-retractions` finishes every source,
then exits 69 if the service was unreachable for any; `merge` without `-force` on
an identity collision exits 65.

**Red tests first:** three records, one malformed: the two good ones are ingested,
exit is 65, and the summary says `1 failed`. A directory with an unreadable file:
exit 74 (skip if the test process runs as root). A stub retraction checker that
fails: exit 69 and the other sources checked. A collision: 65 and nothing written.

**Done when:** the good items are written in every case and no bulk command exits
0 with a failure.

---

**DONE 2026-09-25** (uncommitted). Bulk commands now exit with the class of the first failure,
in path order, after doing everything they can: `kb ingest` (65 for content, 77 for an unreadable
file, 65 for an identity collision; warnings and unresolved references stay exit 0; the summary
is still printed, in `--json` too, with the error on stderr; `--dry-run` reports the same),
`index --all` (a failed corpus outranks a stale one; stale alone is still 1), and `source
check-retractions` (a new `knowledge.RetractionCheckError` carries the count and the first
cause; every source is tried; the command wraps checker errors as unavailable, so 69; the JSON
result gains `failed`; a source that could not be looked up is no longer stamped
`last_checked_at`, which used to make an unreachable service read as "checked, clear"). The merge
identity collision is 65, tested. `document ingest` takes one file, so it was never bulk.
Tests: `ingestexit_test.go`, `indexallexit_test.go`, `checkretractions_test.go`,
`retractionfailures_test.go`; two old ingest tests updated to the new contract (the summary is
still decoded, the error must now be data). Matrix (35 commands): the only change from X2 is
`ingest recs` 0 to 65; the real decision records of all three tiers still ingest with exit 0.
Docs: EXIT STATUS gained the 69 row and the bulk rule; `ingest` and `check-retractions` pages
updated. `document tag`/`fuzzy-tag` stop at the first failure rather than continuing, which is
also non-zero; not changed.

## X4 — Enforcement

**Add:** a test that runs every registered verb and subverb with a bogus flag and
with a surplus argument and asserts 2, and one that asserts no provoked failure in
X2 or X3 exits 70. Together they replace the guarantee DR-0040 admitted was only a
survey.

**Red-first check:** temporarily change one verb's argument check to a plain
`fmt.Errorf`, confirm the test names that verb, and revert.

---

**DONE 2026-09-25** (uncommitted): `cmd/kb/verbcoverage_test.go`. The registry of command paths is
derived from every verb's SYNOPSIS (52 paths, subverbs included; 44 get the trailing checks), so a newly documented
command is covered with no edit to the test, and it is cross-checked against the `verbs` map (a
registered verb with no documented path fails). Every path is run with a bogus flag straight after
the path, with a bogus flag after a complete valid invocation, with two surplus words after one,
and in minimal form (never 70). Paths that end in free text (observation body, project
description, retraction note, search term) are exempt from the two trailing checks, per DR-0041
item 2, and still get the flag-first check. Parser unit-tested on ten synopsis shapes. Mutation
checks: dropping surplus handling in `plainArgs` names 15 verbs; dropping flag detection names
`init`, `project set-description` and `search` (plus the others that reach it). **Found and fixed
by the test:** `kb init --bogus` created a workspace in a directory named `--bogus` and exited 0;
`kb search --json foo` searched for the text "--json foo" and exited 1 "no results", the answer a
script reads as "nothing found". Both now refuse a flag-shaped first argument and accept `--`
(`initsearchargs_test.go`); the `search` page says so.

## X5 — Old versus new on real data

**Run:** the DR-0040 method. Build the pre-change binary from `HEAD`, run the same
~70 commands (every verb with empty, malformed, unknown and surplus input, plus one
provoked failure per class) against a copy of the real database from a scratch
working directory, and diff exit codes and output. Every change of code must be one
the DR-0047 table calls for; output must differ only by the envelope fields.

**Done when:** each difference is explained by a row of DR-0047's table. Read the
real `agents/knowledge.db` only through the copy.

---

**DONE 2026-09-25** (fixes uncommitted). Method as DR-0040: the v0.0.13 tree (`4c6fc3e`) built as
the baseline and the current tree, 387 commands in plain and `--json` mode, each on a fresh copy of
a template workspace (the real `knowledge.db`, copies of the real decision records of all three
tiers, the real document, a copy of `knowledge.jsonl`, and a few bad inputs), run from a scratch
directory named `Laboratory` (the workspace name is the root directory's name, and the real records
were ingested under it). Groups: synopsis-derived (path only, bogus flag first and last, minimal,
surplus), 55 read-only commands on real data, writes on real data, and about 80 provoked failures
(one or more per class, with real names). The real database and all three repositories were
verified unchanged afterwards. Result: 331 of 387 identical; 49 exit codes changed, each a row of
DR-0047 or DR-0048; the ~55 read-only commands on real data are byte-identical; stdout differs only
in the version and hash lines; stderr only where a message improved (raw `FOREIGN KEY constraint
failed` became "no source with id 99"); the `--json` output differs only by the added `failed` field
of `source check-retractions`. Transitions: 0 to 1 (2: `source remove`/`retract` on a missing id),
0 to 2 (11: bad status, trigger, source date or doi, blank source title, negative limit, flag-shaped
init and search), 0 to 65 (`ingest` of malformed records), 1 to 2 (13), 1 to 65 (5), 1 to 66 (11),
1 to 73 (4), 1 to 74 (`import` of a directory), 0 to 74 (`ingest --root nosuchroot`, see below).

**Regressions X5 found in the X2 work, fixed with tests (uncommitted):** four commands that exited 1
in v0.0.13 exited 70 after X2, because the library raised a plain error for a refusal the current
state forces: `concept rename X Y` when Y exists, a project rename blocked by the records it owns,
`document review promote` on a section that is not drafted, and `document frontmatter
--accept-keywords` with an unknown keyword (a bad value, so usage 2). New library marker
`ErrConflict` (negative, 1) for the first three; the keyword error is `ErrInvalid`. Cases added to
`exitcodes_verbs_test.go` and `errors_test.go`. Left as 70 on purpose: an unsupported Go value type in
the frontmatter writer and a bad field name to `DistinctRecordValues` (programmer errors), and "FTS5
not compiled in".

**Decided (draft DR-0049, `proposed`, for the user's review):** `ingest --root nosuchroot` (a wrong
workspace name, so every record hits `UNIQUE constraint failed: records.uuid`) went from exit 0 with
"3 failed" to 74, because DR-0047 item 4 sent any other SQLite error to io. Every SQLite constraint
violation (primary code 19) is now class data, 65; busy/locked stays 75, corrupt/not-a-database 65,
the rest 74. Also fixed: a record that failed to insert was counted as added AND failed (`2 added ...
2 failed`, as in v0.0.13); it is now a failure only. DR-0049 records `ErrConflict`, the keyword fix, the
constraint class and the count. Rerunning X5 with the fix changes only that one exit code (0 to 65);
no 70 anywhere.

## X6 — Documentation, scripts, and the workspace

**Change:** the EXIT STATUS section of the `kb` help text and `kb.1.md`, and each
verb's page where it documents a code; CHANGES.md and `TODO.md` (the exit-code
item is closed by DR-0047); an upgrade note for scripts listing the table; the
three skill scripts that read `kb`'s exit status, so that `project show`'s "not
found" branch tests for 1 and reports anything else as an error.

**DONE 2026-09-25, on acceptance:** the exit-code paragraph of the root `CLAUDE.md` was
rewritten (draft `exit-codes-CLAUDE-md-draft.md`, applied). What follows is what was
proposed: the exit-code paragraph of the
root `CLAUDE.md` to state the convention for both the Go and the Deno tools, in
place of "1 when a search-style tool finds nothing, 2 on a usage or I/O error".
The Deno guidance also needs a note that `Deno.exit` takes the same numbers and
that a Deno tool documents its subset. This is RSDOIEL's guidance file; the
edit is proposed here, not made.

**DONE 2026-09-25, on acceptance of DR-0047:** the supersession of DR-0040 is recorded
and both indexes are current.

---

**DONE 2026-09-25** (uncommitted). (1) The main EXIT STATUS help section and `kb.1.md` (X2, X3), and the
verb pages where a status is notable: `index` (1 for drift, the failure's class otherwise, `--all`
ranking), `merge` (66, 65, 2, 73, 65), `record` (lookup 1 versus write 2, ambiguous id 2), `concept
delete` (1), `source remove` (1, including a missing id), `export` (73, 77, 74, 1) and `import` (65,
66, 77, 74); their man page sources regenerated. (2) `CHANGES.md` gains an `## Unreleased` section with
Added, Changed, Fixed and Upgrade notes (a table of every changed status, v0.0.13 against now, then the
script and library notes); it has no date or version, which release prep sets, and the four
`codemeta.json` fields are still the user's release-prep check. (3) The three skills in `agents/skills/`
(root copies only): `review-knowledge-base`'s bash and PowerShell scripts now branch on the codes
(project show 1 is "not found", anything else is passed on; search 1 is "no results", 2 or more is a
failure; the index check reports a failure that is not drift), tested with `pwsh` and bash against both
the new and the v0.0.13 `kb`; all three `SKILL.md` gained an Exit codes table and a row per condition;
skill versions bumped (`kb_version` unchanged, the codes ship with an unreleased `kb`). The update and
setup scripts run under `set -e` and pass `kb`'s status straight through, so they needed no logic
change. **Found on the way:** the review script passed `--db` to `kb index`, which has refused it since
v0.0.13, and its `|| true` had hidden the failure, so the index check never ran; fixed in both scripts
and the SKILL.md example. `harvey/agents/skills/` holds older, different copies (every file differs);
not touched, left to the skills-in-repo TODO.

## Order and risk

X0 → X1 → X2 → X3 → X4 → X5 → X6. X0 and X1 change no observable exit code: they
add the machinery, so the suite stays green and can be committed alone. The risk
is X2's breadth: 171 sites, and a missed one falls to 70 rather than 1, which is
visible in the X4 tests but only for paths a test provokes. Mitigate by working
verb by verb and running X5 after each verb group, not only at the end.

Out of scope: harvey (uses the library and reads errors, not codes), the Deno
tools (they adopt DR-0003 as they are next changed), a `78` path for `kb`
(no configuration file).
