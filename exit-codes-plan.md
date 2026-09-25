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

## X4 — Enforcement

**Add:** a test that runs every registered verb and subverb with a bogus flag and
with a surplus argument and asserts 2, and one that asserts no provoked failure in
X2 or X3 exits 70. Together they replace the guarantee DR-0040 admitted was only a
survey.

**Red-first check:** temporarily change one verb's argument check to a plain
`fmt.Errorf`, confirm the test names that verb, and revert.

---

## X5 — Old versus new on real data

**Run:** the DR-0040 method. Build the pre-change binary from `HEAD`, run the same
~70 commands (every verb with empty, malformed, unknown and surplus input, plus one
provoked failure per class) against a copy of the real database from a scratch
working directory, and diff exit codes and output. Every change of code must be one
the DR-0047 table calls for; output must differ only by the envelope fields.

**Done when:** each difference is explained by a row of DR-0047's table. Read the
real `agents/knowledge.db` only through the copy.

---

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

## Order and risk

X0 → X1 → X2 → X3 → X4 → X5 → X6. X0 and X1 change no observable exit code: they
add the machinery, so the suite stays green and can be committed alone. The risk
is X2's breadth: 171 sites, and a missed one falls to 70 rather than 1, which is
visible in the X4 tests but only for paths a test provokes. Mitigate by working
verb by verb and running X5 after each verb group, not only at the end.

Out of scope: harvey (uses the library and reads errors, not codes), the Deno
tools (they adopt DR-0003 as they are next changed), a `78` path for `kb`
(no configuration file).
