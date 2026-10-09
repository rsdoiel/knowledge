# Changes

Reconstructed for v0.0.1 through v0.0.3 from each tag's `codemeta.json`
release notes; maintained going forward.

## v0.0.20 — unreleased

The rest of the TUI writes behind the same gates (knowledge DR-0067 and the 2026-10-09 plan,
`agents/projects/knowledge/plans/tui-keys-and-write-flows-plan.md`, release v0.0.20). Written and tested red first.

### Added

- **The removing gate** (DR-0067, DR-0064 amendment; plan item U3). `d` deletes the selected project (the project
  list), observation or concept (a project's tabs), or record (the Records screens and tab). It is not a y/n: a
  screen shows what will be removed and what points at it, for a record the records that supersede or relate to
  it, and asks for the thing's name, a record's qualified reference, or an observation's number, typed exactly.
  Only an exact match and `Enter` delete; `Esc` cancels; there is no flag. While the name is typed every key is
  text, so a project called `quokka` can be confirmed (tested on a real terminal). The delete is the library call
  the command uses (`DeleteProject`, `DeleteObservation`, `DeleteConcept`, `DeleteRecord`), and the plan shows the
  command's own usage summary, so the two cannot disagree. A delete the command would refuse is refused with no
  gate and the reason: a project that owns observations, records or documents, and a record whose file still
  exists (delete the file, or retire it with `set-status REF cancelled`). After a delete the screen shows
  `equivalent:  kb ... delete ...` and the lists, counts and header are refreshed.
- Long notices wrap to the window instead of being cut at the edge, and a legend that is wider than the window
  drops its spacing, then its movement hint, instead of losing its last key.

## v0.0.19 — 2026-10-09

The terminal interface can now change something, and `kb` describes its own command language in one table.
The decisions are knowledge DR-0059 (one verb table, in part) and DR-0064 to DR-0070 (accepted 2026-10-09),
from the design brief `agents/projects/knowledge/design/tui-keys-and-write-flows-design.md` and its plan.
Everything was written and tested red first. The race detector passed on darwin/arm64 at the last code commit
(`727f963`); the Linux-only pseudo-terminal tests skip there. The interface was tried by hand in a real
terminal on Linux. The next release, v0.0.20, brings the rest of the writes behind the same gates.

### Added

- **`kb verbs` prints the verb table** (DR-0059). One table in `cmd/kb/verbtable.go` now describes every verb:
  its summary, subverbs (and `document review`'s own), flags, which subverbs take a project, which have
  deprecated `--project`/`--workspace` aliases, and its **write class** (DR-0067: read, additive, changing,
  removing, direct, plan_apply, guided, cli_only). `kb verbs` prints it as text, and `kb -json verbs` as JSON
  (the global option goes before the verb), so a model or script has one description of the command
  language. It opens no database. Completion, the subverb help flags and the deprecation notes now read the
  table instead of hand-kept lists; the generated completion scripts are byte for byte what they were,
  held by golden files in `cmd/kb/testdata/`. Parsing is still per-verb (flags are declared three ways);
  the TUI menu and `:` will read the table in later items.
- **`kb ingest` reports a status that arrives through a file** (DR-0068). Ingest is how a status edited by
  hand in a record file reaches the database, without the transition table or the terminal rule; that is by
  design (the file is the source of truth) but nothing said so. The summary now prints
  `status: clasm/DR-0012 proposed -> accepted (edited in file)` for a stored status that differs from the
  file's, and `status: clasm/DR-0013 arrived accepted` for a record first ingested already accepted. A report,
  not a failure: the status is applied and the exit code is unchanged, and a move `set-status` would refuse is
  reported like any other. The text lists the first 20 and says how many it left out; `kb -json ingest` has
  every one in a new `status_changes` array (`ref`, `from`, `to`, `kind`). `--dry-run` reports the same.
- Table finding: `observation delete` and `project delete` are removing verbs too, beside the five DR-0067
  names; the table classes them so and a test pins the full set.

### Added

- **The first write in the TUI: read a record and set its status** (DR-0066, DR-0065, DR-0067, DR-0061). On
  the Records screens and a project's Records tab, `Enter` reads the selected record and `s` sets its status.
  Both show the record file in the pager (`$KB_PAGER`, else `bat`, else `less -R`) with the TUI suspended while
  it runs (`tea.ExecProcess`); with no pager, or one that cannot start, the record is shown in a built-in
  scrolling viewer. `s` then runs the T3 chooser and confirmation inside the TUI and writes through
  `applyRecordStatus`, the function `kb record set-status` uses, so the two cannot disagree. Esc, `q` and
  `Ctrl-C` back out at either step and write nothing; a move the file now refuses is a notice, checked against
  the record as it is on disk; a superseded record says it is final before any pager starts. The screen then
  shows `✓ REF status set to X` and `equivalent:  kb record set-status REF X`. Accepting is allowed because the
  TUI is a terminal. The list keeps its cursor on the record. Tested at unit level with a pager seam, and on a
  pseudo-terminal with a pager that waits for a key, showing the terminal is handed over and taken back.

### Changed

- **The TUI opens at a menu, in a frame, with tabs** (DR-0066 and its 2026-10-09 amendment). Bare `kb` on a
  terminal opens a top menu with one row per verb group, derived from the verb table, under a header naming the
  workspace directory, its database and what it holds. Each group opens its own menu. Rows with no screen yet
  are dimmed with the release that brings them, and choosing one shows the equivalent command. Back is
  structural, so a deep link has a way back: `q` goes to the screen above and quits only at the top menu.
  Every screen is drawn in one box (title, body, divider, legend) the width of the window; the frame no longer
  ends in a newline, which made the view a line taller than the window. A project's observations, concepts and
  records are tabs with counts and a rule under the active one (`o`, `c`, `r`), all three loaded when the
  project is opened. **New screens:** Records → Browse (newest first) and Pending (oldest first), one line a
  record with its qualified reference, in the scope `kb record list` would use (the same code), `a` widens it.
  **Keys typed together** ("qq", a paste) are taken in turn instead of being dropped.
- **Deep links.** A complete command always runs and prints, on a terminal or not. Only an incomplete one opens
  the interface, and only on a terminal: bare `kb`, a bare group (`kb record`), and `kb record show` or
  `kb project show` with no argument. Without a terminal those are usage errors (exit 2), and bare `kb`
  without one no longer tries to start a terminal program. **`-i` is a new global option** that opens the
  interface at a command: `-i record list`, `-i record pending`, `-i record show REF` (the cursor on that
  record), `-i project list`, `-i search TERM`, `-i` alone, or a group. A command with no screen yet is a
  usage error that says so. Opening the interface no longer creates a database where there is no workspace
  (exit 66, as every other verb).
- **The TUI follows clasm's key conventions, and every screen shows its keys** (DR-0064). **This changes
  today's keys.** `q` now goes back to the screen you came from (a search's results return to where the search
  began) and quits only at the project list; `Esc` cancels something in progress and otherwise does nothing,
  so it never closes a screen; `Ctrl-C` quits from anywhere, including while typing and over an error.
  A legend line at the bottom of every screen lists the keys that apply, and changes with the mode. While you
  type in the search prompt every key is text, `q` included, so a term that starts with `q` works; the model
  asks one question, `capturingText()`, before it binds any letter. An error is dismissed with `q` or `Enter`
  (it used to stay on screen for ever). The lists no longer carry quit keys, a hidden filter or help of their
  own, and the records list is resized with the others. A test per screen types `q j k a y n :` into the
  prompt, and the built binary on a pseudo-terminal shows a lone `Esc` does not quit.
- **The `kb record set-status REF` review takes single keys** (DR-0065). A status letter chooses at once and
  `y` confirms; `Esc`, `q` or `Ctrl-C` backs out at either step, as does `n` at the confirmation, all without
  Enter. `Enter` on the confirmation does nothing, so a habitual Enter is never consent to a write. It
  replaces the v0.0.18 prompt, which read whole lines and could not see a bare `Esc`; the whole-name answers
  and `[y/N]` default are gone. It runs as a small inline bubbletea program, the same component the TUI will
  embed, and a review fed from a pipe treats the end of input as a cancel instead of waiting. A text field
  never treats `q` as a command (DR-0064 amendment); the typed confirmation the removing gate will use
  (`typedConfirmModel`) is built and tested for it already.
- **`kb document review promote` needs a person at a terminal** (DR-0070, extending DR-0061). It exits 2,
  writing nothing, unless standard input and standard output are both terminals, with a message that a person
  must promote it. No flag or environment variable turns that off, and `draft`, `review list` and every other
  document verb need no terminal, so a model or script can draft a summary but cannot make it trusted. The
  library is unchanged: `PromoteDocumentSummary` has no terminal rule, and harvey's `LearnSession.Accept` is
  unaffected. Scripts that promote without a person at the keyboard must hand that step over. `kb document`
  gains an EXIT STATUS section.

## v0.0.18 — 2026-10-09

Review then decide. The decisions are knowledge DR-0060 and DR-0061 (accepted) and DR-0063 (proposed, for the author to promote), from the design brief `agents/projects/knowledge/design/scoped-refs-and-tui-parity-design.md`. Everything was written and tested red first. The release shipped before the race detector was run; it was run afterwards, on darwin/arm64 (Go 1.26.5) at the release commit `258600d`, and both packages pass (the Linux-only pseudo-terminal tests skip there).

### Changed

- **`kb record set-status` follows a transition table** (DR-0060). It used to write any status on any record.
  Now `proposed` may become `accepted`, `rejected`, `cancelled` or `superseded`; `accepted` may become
  `cancelled` or `superseded`; `rejected` and `cancelled` may go back to `proposed`; `superseded` is final.
  `superseded` also needs `superseded_by` already set, so use `kb record supersede`. A move outside the table,
  including setting the status a record already has, exits 1 with the allowed moves named and writes nothing.
  A record whose status is outside the vocabulary can be set to `proposed` and nothing else. Scripts that
  relied on arbitrary moves (for example `accepted` to `rejected`, use `cancelled`) will see exit 1.
- **`cancelled` is wider than DR-0038 said** (DR-0063): pursued or explored, then abandoned, and need not
  have been accepted. `rejected` is considered and not pursued. `DECISION_RECORD_FORMAT.md` and `kb-record(1)`
  carry the wording.

- **`accepted` needs a person at a terminal** (DR-0061). `kb record set-status REF accepted` exits 2 with a
  message that a person must accept it unless standard input and standard output are both terminals, and writes
  nothing. There is no flag or environment variable to turn it off; every other move works without a terminal.
  A script that promoted records must hand that step to the author. This guards the accidental path (a model or
  script following instructions), not a determined one: a process can allocate a pseudo-terminal, and a file can
  be edited by hand. The terminal test asks the driver, not the file mode, so `< /dev/null` does not count.
  `github.com/charmbracelet/x/term` and `golang.org/x/sys` are now direct dependencies; both were already built
  into `kb` through bubbletea.

### Added

- **`kb record set-status REF` with no status is a review** (DR-0060): the record is shown through `$KB_PAGER`
  (else `bat -l markdown`, else `less -R`, else printed), the prompt names the current status and the moves the
  table allows (`harvey/DR-0004 is proposed -> [a]ccepted [r]ejected [c]ancelled [q]uit`), and a confirmation
  line (`[y/N]`) comes before anything is written. Statuses are chosen by first letter or whole name; `q` backs
  out, which is not the status `cancelled` (`c`). Backing out at either step, an empty confirmation or end of
  input leaves the record unchanged and exits 1. The review needs a terminal and takes no `--json`, exit 2; the
  form with a status is unchanged and is what scripts use. Both forms end in one function, so they cannot
  disagree about what a status change is. `KB_PAGER` is new.
- Library: `AllowedTransitions` and `CanTransition`, so the CLI, the review prompt and the TUI read one table.

## v0.0.17 — 2026-10-08

Everything below is written and tested red first. The decisions are knowledge DR-0057 (qualified record
references) and DR-0058 (scope), accepted 2026-10-08, from the design brief
`agents/projects/knowledge/design/scoped-refs-and-tui-parity-design.md`. DR-0059 to DR-0062 (a verb table,
review-then-decide, the terminal requirement for `accepted`, a read/write TUI) are accepted but are not in this
release. The race detector was run on darwin/arm64 at the last code commit and the suite passed; the new
commands were checked read-only, with a scratch build, against both the Laboratory and the WorkLab workspaces
(277 records, 23 projects in the second, with `DR-0001` repeated ten times).

### Added

- **A workspace is found by walking up** (DR-0058): from the current directory to the nearest ancestor with
  `agents/knowledge.db` or `agents/knowledge.jsonl`, as git finds `.git`, so every verb works from any
  subdirectory. `-db` still wins. A fresh clone that has only the JSONL file is told to run
  `kb import -in agents/knowledge.jsonl`, not `kb init`, and `kb import` builds the database in the workspace
  and not in the current directory. Library: `knowledge.FindWorkspace`, `MarkerDB`, `MarkerJSONL`.
- **Environment:** `KB_DB` (the database, as `-db`), `KB_PROJECT` (the project to act on when the directory
  gives none), `KB_CEILING_DIRECTORIES` (directories the walk never enters or passes, as git's
  `GIT_CEILING_DIRECTORIES`), `KB_QUIET` (silences advisory notes, never an error). When the workspace found is
  not the current directory, a note on standard error says so; standard output is untouched.
- **Qualified record references** `SCOPE/DR-NNNN` (DR-0057), where SCOPE is a project name or `workspace`. The
  workspace directory's own name, in any case, is accepted for `workspace`, and a project of that name wins. A
  short id is padded (`harvey/4` is `harvey/DR-0004`). Library: `Ref`, `ParseRef`, `(*KnowledgeBase).ResolveRef`
  and `*AmbiguousRefError`, so harvey shares the syntax and the lookup.
- **Scope on the record verbs** (DR-0058): `kb record list harvey clasm workspace`; with no scope, the project the
  working directory belongs to (under `agents/projects/NAME/`, or a repository directory `NAME/` beside it), then
  `KB_PROJECT`, then the whole workspace; `--all` widens it. `record new` takes its project the same way.
  Library: `(*KnowledgeBase).ProjectForDir`.
- **`kb record pending [SCOPE...] [--all]`**: every `proposed` record in scope, oldest first, with the other
  `list` filters. `--status` is a usage error.

### Changed

- **Listings name records by reference.** `record list` starts each line with `harvey/DR-0004` (columns
  aligned) in place of the bare id and the `-` project column; `--json` gains `ref` beside the unchanged
  `record_id`, `project` and `scope`. `record show` heads with the reference and qualifies every relation.
  `set-status`, `supersede`, `delete`, `new` and `fuzzy-tag` confirm with references, and their `--json` gains
  `ref` (`new_ref` and `old_ref` for `supersede`).
- **A change needs a scope.** `set-status`, `supersede` and `delete` refuse a bare id with no scope, even when
  it is unique (exit 2, offering the qualified form). Say `harvey/DR-0004`, run it inside the project, or set
  `KB_PROJECT`. Reading verbs resolve a bare id inside the inferred project.
- **A bare id is looked up in this workspace's records only**, where it used to count every workspace's.
- **`record list` takes scope arguments**, so a trailing word is no longer a surplus argument (exit 2); an
  unknown scope is exit 1, as an unknown `--project` always was.
- **`--project` and `--workspace` are deprecated** on `list`, `pending`, `show`, `concepts`, `set-status`,
  `supersede` and `delete`. They still work, must agree with a qualified reference, and print one line on
  standard error naming the better spelling. `record new`, `record fuzzy-tag` and other verbs' `--project`
  are unchanged.
- **A project cannot be named `workspace`** (any case): `project add` and `project rename` refuse it, exit 2.
  It names the workspace tier in a reference, so such a project could never be reached.
- The tests set `KB_CEILING_DIRECTORIES`, `KB_DB` and `KB_QUIET` themselves, so a developer's shell cannot
  change them, and the record round-trip test reads only the workspace it sits in.

### Upgrade notes

- A script that reads `record list` output by column breaks: the first column is now the reference and its
  width varies. Use `--json`; `record_id` is unchanged and `ref` is new.
- A script or skill that changes a record with a bare id (`record set-status 0004 accepted --project X` still
  works, with a note) should use `X/DR-0004`. Skills that quote the old forms need `kb >= 0.0.17`.
- `kb` run from a subdirectory of a workspace used to fail with exit 66; it now finds the workspace. A scratch
  directory inside a real workspace therefore reaches it: set `KB_CEILING_DIRECTORIES` to the scratch directory
  to prevent that.

## v0.0.16 — 2026-10-06

Written and tested red first. The decision is knowledge DR-0056. It was checked by sourcing the generated
script in real bash and PowerShell (`TabExpansion2`) against the real workspace; that run found a
placeholder left unsubstituted in the project-name path, which the unit tests had missed, and it is fixed
and now tested.

### Added

- **`kb completion bash|powershell [-install]`** (DR-0056): writes a shell completion script to standard
  output, or with `-install` installs it. It completes verbs, subverbs, each verb's flags, help topics,
  paths after `-db`, `-in`, `-out`, `-a`, `-b`, `-jsonl`, `-root` and `-dir`, and project names (from
  `kb project list`, honouring an earlier `-db`) after `--project` and after the `project` subverbs that
  take a name. Bash installs to `$XDG_DATA_HOME/bash-completion/completions/kb` or the same under
  `~/.local/share` and never overwrites a file `kb` did not write (exit 73). PowerShell writes
  `kb-completion.ps1` beside the profile and adds one dot-source line, once. Windows PowerShell 5.1 is not
  covered. The verb opens no database and refuses `-db`.
- The verbs are read from the registered `verbs`; subverbs and flags are tables that tests compare with the
  code (each verb's own usage text, and a scan of the sources for declared flags), so a new flag or subverb
  fails `go test` until the tables are updated. The flag check is union-wide, not per verb.
- `kb-completion(1)`, and `completion` in `kb(1)`, `kb help topics` and the Makefile's `KB_TOPICS`.

### Upgrade notes

- Nothing that worked changes. `kb completion` did not exist before, so a script that called it got exit 2
  (unknown verb) and now gets a script.

## v0.0.15 — 2026-09-29

Everything below is written and tested red first. The decisions are knowledge DR-0051 (concept
show and recall), DR-0052 (record fuzzy-tag), DR-0053 (check-db), DR-0054 (import keeps an unknown
observation kind) and DR-0055 (project add). DR-0052 was amended twice
and DR-0051 once after acceptance, each with a dated note. The release was checked by running
the v0.0.14 build beside the new one on a copy of the real workspace, 415 commands in plain and
`--json` mode: the only changed exit codes are the two new verbs' own (2 to 1, from "unknown
subverb" to "not found"), and nothing exits 70. That comparison was made before the `kb import` and
`kb project add` fixes below, and `project add` on an existing name changes from exit 0 to 1.

### Added

- **`kb concept show NAME [--limit N]`** (DR-0051): a concept's description and identifier, and per
  kind (projects, observations, records, document sections) the full count and up to N items, newest
  first. Default 10; `--limit 0` gives counts only. The name matches exactly, as `concept delete`
  does, and a miss (exit 1) offers a case variant. `--` passes a name that looks like a flag; `--json`
  gives one object.
- **`kb concept recall [TEXT... | -] [--concept NAME,...] [--project NAME] [--limit N]`** (DR-0051):
  finds the concepts named in the text (arguments, or stdin with `-`) and lists the observations,
  records and document sections linked to them, ranked by matched concepts then recency, each with
  the concepts it matched and an excerpt. `--concept` names concepts directly and is unioned with
  any found in the text. Text that matches no concept, or an unknown project, is exit 1; nothing to
  recall from is exit 2. Nothing is written and no concept is created.
- **`kb record fuzzy-tag --project NAME [--concept NAME,...] [--write] [--dry-run] [--root DIR]`**
  (DR-0052): reports near-miss spellings of known concepts in decision records, with the variants, the
  count, the plain exact mentions that link nothing, and the `tags:` line that would link the concept.
  Only the body is searched; a concept is skipped for a record only when the record already links it
  (in `tags:` or as a `[[wikilink]]`). By default nothing is written. `--write` adds the concept to the
  `tags:` of `proposed` records, changes nothing else in the file (a one-line flow list, a block list, or
  a new line), never touches an accepted record, and is both-or-neither across the run; `kb ingest`
  then links the new tags. Without `--concept` the length and distance rules of `document fuzzy-tag`
  apply. A named concept is widened to tokens that contain its name (`detoast`, `toasting`) plus what
  those rules already admit; DR-0052's first amendment reused the document bypass unchanged, and a
  smoke test on real records showed it flags every short word within distance 3 of a short concept
  (`to`, `that`, `has` for `toast`), so the second amendment narrows it for records only.
- **`kb check-db [--jsonl FILE]`** (DR-0053): compares the database with the JSONL dump beside it (the
  database path with `.jsonl` for `.db`; the resolved paths are printed) by full content, never by file
  time, and recommends `kb import`, `kb export`, or both. The report has per-table counts, rows only
  in the JSONL, rows only in the database, and rows in both that differ. File times appear as a hint
  only. Exit 0 in sync, 1 not in sync (the report is still printed), 66 no dump, 65 malformed dump.
  There are no tombstones: rows only in the JSONL are labelled "never received, or deleted here", and
  the import recommendation warns that it brings back anything deleted on purpose.
- Library: `ConceptDetail`, `RecallByText` and `RecallByNames` (with `TextRecall` and `TextRecallHit`),
  `RecordFuzzyReport` and `AddRecordTags`, `CompareToJSONL` (with `DBComparison`, `TableDiff` and
  `RowDifference`), `DiffDatabases`, `PrepareMergeScratch` (with `MergeScratch`),
  `DetectIdentityIssues` and `CheckpointAndCopy`. `RecallByConceptNames` is unchanged and ranks the
  same way; `RecallByText` adds a project filter, the project and matched concepts on every hit, and
  the list of linked projects.

### Changed

- **`kb project add` refuses a name that already exists** (DR-0055): exit 1, `project "NAME" already
  exists (id=N)`, and nothing is written. It used to print "added", exit 0, drop the requested status
  and description, and move `updated_at`. **A script that re-ran `kb project add` and relied on exit 0
  now gets 1.** The library's `AddProject` is unchanged.
- **`kb project add NAME DESC --status X` is a usage error** (exit 2, "must come before NAME") instead
  of storing `DESC --status X` as the description. `--` before NAME keeps a flag-looking word as text.
- The detection half of `kb merge` (copy, normalise, collision and divergence reports) is now a
  read-only library pipeline, `PrepareMergeScratch` and `DetectIdentityIssues`, shared with
  `check-db`. `merge`'s output is unchanged, guarded by golden tests captured before the move.
- Harvey's `recallKB` calls `RecallByText`, the same call `kb concept recall` makes.

### Fixed

- **`kb import` no longer rewrites an observation kind outside the vocabulary to `note`** (DR-0054).
  The kind is kept and reported as a `warning:` line (a `Warnings` list under `--json`); the run
  exits 0. This also removes a false `kb check-db` "diverged" whose recommended fix would have
  corrupted the dump.
- A record file that does not parse is now exit 65 (wrong content) from `kb record set-status`,
  `kb record supersede`, `kb project rename` and `kb record fuzzy-tag`, where the first three exited 2.
  The file's content is what is wrong, not the command line.

### Upgrade notes

- Nothing that worked changes, apart from the malformed record file exit above. A script that treated
  2 from `record set-status` or `supersede` as "bad command line" will now also see 65 for a record
  file it cannot parse.
- `kb concept show`, `kb concept recall`, `kb record fuzzy-tag` and `kb check-db` did not exist before,
  so a script that called them got exit 2 (unknown verb) and now gets a real answer.

## v0.0.14 — 2026-09-25

Everything below is written and tested red first. The decisions are workspace DR-0003 and
knowledge DR-0045 to DR-0050.

`kb` now tells a script *what kind* of failure it was from the exit number alone, and
a batch of inputs that were accepted silently are refused. The exit codes follow one
convention for every command-line tool in the workspace (workspace DR-0003): 0 and 1
answer the question that was asked, 2 says the command was wrong, and the `sysexits(3)`
numbers above that say what went wrong. Read **Upgrade notes** before updating a script.
The whole change was checked by running the v0.0.13 build alongside the new one, 387
commands in plain and `--json` mode, on a copy of the real database.

### Added

- **Removal verbs** (DR-0050), so a mistake is fixable without raw SQL, which skips the search
  index: `kb project delete NAME` (for a stray empty project; a project that owns any observation,
  record or document is always refused, `--force` or not, and `--force` removes only its concept
  links), `kb observation delete ID` (refused while it has concept links, source links or
  supersessions; `--force` removes them), `kb document delete ID` (refused while any section's
  summary is reviewed, since that is human-gated data; `--force` deletes anyway) and `kb record
  delete RECORD_ID` (only once the record's file is gone, because `ingest` is additive and the file is
  the truth; `kb` never deletes a decision record from disk). `kb unlink project|observation|source`
  is the inverse of `link` and `source link`. Each takes `--dry-run`, refuses with what and how many
  (exit 1), and treats a target or link that is not there as exit 1, not success. A delete is local to
  one database: `kb merge` and `kb import` from a database that still has the row bring it back. Library:
  `DeleteProject`, `DeleteObservation`, `DeleteDocument`, `DeleteRecord`, their `...Usage` functions, the
  three `Unlink...` functions and `InUseError`.
- **Exit codes** (DR-0003, DR-0047, DR-0048, DR-0049): 0 success; 1 a normal negative
  answer (not found, no results, a stale index, an operation the current state forbids);
  2 usage; 65 wrong content; 66 a missing input or workspace; 69 an unreachable service;
  70 an internal error; 73 an output that cannot be created; 74 an I/O error; 75 a locked
  database; 77 permission denied. An error nothing classified is 70, never 1, so a gap
  shows as an internal error. The main page's EXIT STATUS section lists them.
- The `--json` error object carries the class and the number beside the message:
  `{"error": "...", "class": "no_input", "code": 66}`.
- Library: `ErrInvalid`, `ErrNotFound`, `ErrInUse`, `ErrConflict` (markers matched with
  `errors.Is`, message text unchanged) and `RetractionCheckError`.
- `kb source add` checks what it is given (DR-0045): a blank title, a `--published`
  that is not `YYYY`, `YYYY-MM` or `YYYY-MM-DD`, a `--url` that is not absolute, and a
  `--doi` that is not the bare `10.NNNN/suffix` form are refused. The DOI check matters:
  `check-retractions` sends the DOI to Retraction Watch, and a mistyped one used to read
  as "not retracted".

### Changed

- **Bulk commands do everything they can, then exit with the class of the first failure**
  instead of 0: `kb ingest` (65 for a record that does not parse or two files claiming
  one identity, 77 for one it may not read), `index --all` (a corpus that could not be
  indexed outranks a stale one), and `source check-retractions` (69, after trying every
  source). The summary is still printed, in `--json` too. A record that failed to insert
  is counted as failed only; it was counted as added as well.
- `source check-retractions` stamps `last_checked_at` only on a source it got an answer
  for. It used to stamp the ones it could not reach and report them checked. The `--json`
  result gains `failed`.
- A project or concept name is one line (DR-0046): surrounding whitespace is trimmed,
  any interior run of whitespace (a newline, a tab) becomes a single space, and a control
  character is refused. A hard-wrapped `[[wikilink]]` in a document or record is now the
  ordinary one-line concept; it used to mint a second concept with a newline in its name.
- `--` ends flag recognition for `record`, `document`, `ingest` and `source add` too
  (DR-0046), so a dash-leading title or path can be given.
- `kb merge` refuses a zero-byte input whether or not a `-wal` sits beside it (DR-0046):
  SQLite discards that WAL on opening an empty file, so the allowance protected nothing,
  and merge used to exit 0 with no rows and delete the `-wal`.
- `record new` and `record set-status` refuse a trigger, kind or status outside the
  vocabulary unless a record already carries it (DR-0045, DR-0048); they wrote it
  silently before. `record list` filters keep their lookup behaviour (exit 1).
- Failures before a verb runs (no workspace, the database will not open, the debug log)
  are reported like any other error, so they honour `--json` and carry a class.
- `kb search` and `kb init` read a flag-shaped first argument as a mistyped option
  (exit 2), and take a dash-leading term or path after `--`.

### Fixed

- Silent successes: `source retract 99` and `source remove 99` on an id that does not
  exist exited 0; `source link` and `link observation` with a missing id failed with a
  raw foreign-key error (74); `concept suggest --limit -1` was accepted; `kb init --bogus`
  created a workspace in a directory named `--bogus`; `kb search --json foo` searched for
  the text and answered "no results"; `kb ingest` with failed records exited 0.
- Refusals the current state forces are exit 1, not an internal error: a rename onto a
  name that exists, a project that still owns records, a section not yet drafted.
- A SQLite constraint violation is wrong data (65), not an I/O failure (74): an ingest
  under the wrong workspace root now says so.
- `document frontmatter --accept-keywords` with an unknown keyword is a usage error.
- `agents/skills/review-knowledge-base` passed `--db` to `kb index`, which has refused it
  since v0.0.13 (exit 2), and its `|| true` hid the failure: the index check never ran.

### Upgrade notes

For scripts. **Exit 1 no longer means "anything went wrong".** It is the normal "no"
(not found, nothing matched, a stale index, refused by the current state), as it is for
`grep` and `diff`. A script that treated any non-zero as failure is unaffected. One that
treated 1 as "the command failed" now also has to handle 2 and the numbers above it.

| What happened | v0.0.13 | now |
|---------------|---------|-----|
| A bad value on the command line: unknown observation kind or project status, a blank name, a bad `--published`, `--url` or `--doi`, a negative `--limit`; an unknown record status, trigger or kind given to `record new` or `set-status` | 1, or 0 (`record new`, `source add`) | 2 |
| A named input or the workspace is missing (`ingest`, `import -in`, `merge`, `index`, no `agents/knowledge.db`) | 1 | 66 |
| Wrong content: a malformed record, JSONL or document; a file that is not a knowledge base; a zero-byte merge input; a merge collision; a constraint violation | 1 | 65 |
| `ingest` with records that could not be ingested | 0 | 65, 77 or the first failure's class |
| An output cannot be created: `merge -out` exists, `export -out`, `init` | 1 | 73 |
| An I/O error; a locked database; permission denied | 1 | 74; 75; 77 |
| `check-retractions` with lookups that failed | 0 | 69 |
| `source retract` or `remove` on an id that does not exist | 0 | 1 |
| Not found, `search` finds nothing, a stale index, a concept or source still linked | 1 | 1 (unchanged) |
| An internal error | 1 | 70 |

- `--json` failures before the verb runs (no workspace, an unreadable database) are now
  JSON on stderr; they were plain text.
- `kb search` and `kb init` refuse a flag-shaped first word; `kb search -- -x` searches for
  a dash-leading term.
- `record set-status ID STATUS` refuses a status outside `proposed`, `accepted`,
  `superseded`, `rejected` and `cancelled` that no record carries.
- The three knowledge skills in `agents/skills/` branch on the new codes; a skill or script
  that ran `kb --db PATH index ...` must drop `--db` (it has been refused since v0.0.13).

The new removal commands change no existing status. A script that used `sqlite3` to delete a row
still works and still skips the search index; `kb` now has a supported form.

For library callers:

- `AddSource` trims and validates (`ErrInvalid`); `RemoveSource`, `RetractSource`,
  `LinkObservationSource`, `LinkObservationConcept` and `LinkProjectConcept` return
  `ErrNotFound` for an id that is not there (`RetractSource` and `RemoveSource` returned
  nil); `RemoveSource` on a linked source is `ErrInUse`.
- `CleanName` collapses interior whitespace and refuses control characters.
- `CheckRetractions` returns a `*RetractionCheckError` after checking every source when
  some could not be checked, and its `checked` counts the answered ones only.

## v0.0.13 — 2026-09-24

A sweep for commands that answer a mistake with success. Every verb was run
with empty, malformed, unknown and surplus input against a copy of the real
database, and each one that exited 0 for input it had not understood, or lost
part of it, is fixed, along with the `kb search` hyphen bug found while testing
Harvey's `/kb search`. Several changes are visible to scripts: usage errors now
exit 2, unknown flags and surplus arguments are refused instead of dropped, and
three verbs that ignored `--db` now refuse it. Read **Upgrade notes** before
updating a script. The decisions behind the changes are DR-0040 to DR-0044.
Every fix below was written red first, and the command-line changes were
checked by running a build from before the change alongside the new one, on a
copy of the real database.

### Added

- `CleanName(kind, name)`, exported (DR-0042): trims surrounding whitespace from a
  project or concept name and refuses one that is empty afterwards. It is the
  one definition of a usable name, shared by the library and the CLI.
- `DistinctRecordValues(field)` (DR-0043): the values a record's `status`, `kind`,
  `trigger` or `initiative` actually takes in this database, sorted and
  without the empty string. The field name is spliced into SQL, so only those
  four are accepted. It exists so `record list` can tell a typo from a real
  filter that matches nothing.
- An ARGUMENTS section in `kb(1)` (DR-0041): global options go before the verb,
  a name that begins with a dash follows `--`, and free text after a verb's
  fixed arguments is taken as typed. EXIT STATUS now says exactly what 1 and 2
  mean.

### Changed

- **Usage errors exit 2** (they exited 1; DR-0040). `kb -help` had always
  documented 2 for "usage error", but only an unknown verb produced it, so a
  script could not tell "fix the command" from "handle the result". Exit 2 now
  means the command line itself is wrong and nothing was attempted: an unknown
  verb, subverb or flag, a missing or surplus argument, a flag without its value
  or with one that does not parse, a malformed id, a missing required flag. Exit
  1 stays for a command line that was fine and a result that was not: not found,
  no results, a value the knowledge base rejects (an unknown kind or status), a
  database error. Over 67 commands run through the old and new binaries, 35
  changed from 1 to 2, all of them real mistakes, and the other 32 kept their
  codes. About 75 error sites were converted; a mechanical edit of mine dropped
  a word from one message and was caught by auditing the diff.
- **Unknown flags and surplus arguments are refused** (they were dropped;
  DR-0041). Of 25 verbs probed, 15 accepted a bogus flag and 19 ignored an extra
  argument; all 25 now refuse both. The damage was real: `source add T more
  words` stored the title `T` and lost the rest, `record new --title A decision`
  kept only `A`, `source link 1 1 --relationship` with no value linked anyway,
  and `project list --json` printed plain text and exited 0, ignoring the
  option. A global option after the verb is now refused with a message saying it
  goes before it (`kb --json project list`). Verbs with plain positionals check
  their fixed arguments for flag shape and their count; FlagSet verbs check
  their leftovers; the `record` verbs check for exactly the arguments they take.
  **A name that begins with a dash now needs `--`** on the verbs that take one
  (`kb project show -- -name`), as `concept delete` already did. Free-text tails
  are unchanged: `project set-description P a -x b`, a retraction note and an
  observation body may contain dashes. Over 65 commands the old and new binaries
  left an identical database and identical exit codes, apart from the two
  intended cases.
- **`init`, `index` and `merge` refuse `--db`** (DR-0041). None of them opens
  the ambient database, and `--db` was dropped silently: `kb --db rt.db init`
  reported success and created `./agents/knowledge.db`, and `rt.db` never
  existed. Refused rather than honoured because `--db` names a database file
  while `init` creates a workspace, and any verb that does use the database
  already open-or-creates an explicit path. `kb --db x init -help` still prints
  the page.
- **`record list` validates its filters** (DR-0043). Every mistake used to read
  as `no matching records`. A `--status`, `--kind`, `--trigger` or
  `--initiative` that no record carries and the vocabulary does not list is now
  an error naming what is known; `--project` naming no project is an error
  listing the known ones; `--since` must be a real `YYYY`, `YYYY-MM` or
  `YYYY-MM-DD`; and `--workspace` with `--project` is refused, since
  workspace-tier records have no project. The vocabularies are documented rather
  than enforced, so a value outside them that some record does carry still
  filters, and a value inside them that matches nothing is still a valid empty
  answer. On a copy of the real 44-record database, 16 valid filters were
  byte-identical and 9 mistakes became errors.
- **Project and concept names are trimmed, and a blank name or observation body
  is refused** (DR-0042). `kb project add '  '` created a project named two
  spaces, `kb concept add ''` an empty concept, and `kb observation add ... ''`
  an empty body; all three then showed up in every list, and `' harvey '` was a
  second project beside `harvey`. This is in the library, so ingest, Harvey and
  a script get it too. An observation body is only refused when blank and is
  otherwise stored exactly as given. `import` and `merge` use raw SQL and are
  unchanged, so an existing database with such rows still loads.
- `kb search ''` and `kb search ' '` are usage errors (exit 2, DR-0040,
  DR-0041), not `no results`.

### Fixed

- **`kb project rename demo ''` corrupted a project that owns records**
  (DR-0042). The command rewrites every owned record's `project:` frontmatter
  before the library sees the new name, so a blank name wrote `project: ""` into
  each file, renamed the row to an empty string, and the next ingest hit `UNIQUE
  constraint failed: records.uuid`, the deadlock DR-0026 exists to prevent. The
  name is now cleaned before any file is staged, so the files and the row cannot
  disagree about it, and `--dry-run` refuses too. Found while verifying the name
  fix on a real scratch corpus.
- **`kb merge` succeeded on inputs that do not exist** (DR-0044). `kb merge -a x
  -b y -out z` with neither present exited 0, created zero-byte `x` and `y`, and
  wrote a schema-only `z`, so a mistyped `-a` merged an empty database and
  reported success. Cause: each input was opened with `sql.Open` and a pragma,
  and SQLite creates a missing file. Now a missing input (exit 1, naming the
  flag and path), a directory or other non-file, and a zero-byte file are
  refused before anything is opened, as is `-a` and `-b` naming the same file by
  any spelling or symlink (exit 2). A directory used to fail with SQLite's
  `unable to open database file: out of memory (14)`. A refused merge changes
  nothing on disk, and a test compares the directory before and after. A
  zero-byte main file with a non-empty `-wal` is not refused; that state could
  not be produced with this driver, so it is a defensive allowance, not an
  observed case. A real merge of the actual database with an older backup gives
  an identical 14-table summary and identical merged content (518 rows
  compared).
- **`kb VERB SUBVERB -help` did something different for every verb** (DR-0041).
  `project add -help` printed `kb: flag: help requested` and exited 1, `project
  show -help` looked up a project named `-help`, `document frontmatter -help`
  tried to open a file called `-help`, and `record list -help` said `unknown
  flag`. It now prints the verb's page and exits 0, before any database is
  opened, so it works outside a workspace and creates nothing. A help flag after
  other flags (`project add --status active -help`) is answered too. It is not a
  scan of every argument: `kb observation add --project p note pass -h to it`
  still records its body. Checked over 33 subverbs in three spellings.
- **`kb search` failed on any term containing a hyphen** (`map-reduce`,
  `JSON-L`, `cross-machine`) with `SQL logic error: no such column: reduce`,
  because FTS5 reads `a-b` as `a` minus column `b`. v0.0.11's punctuation fix
  retried only on a syntax error, so a hyphen slipped past it. `Search` now
  retries once as a quoted phrase on any failure of the raw term; a trailing `*`
  stays outside the quotes so prefix search still works, and the documented
  syntax (AND, phrases, NOT, OR, prefix) is unchanged. Of 23 terms run against a
  copy of the real database, 11 gave identical results and 12 that errored now
  return results (`map-reduce` 4, `JSON-L` 17, `cross-machine` 30). Harvey's
  `/kb search` calls the same function and is fixed with it.
- `TestImportJSONL_SameNameDifferentUUIDMergesUnderLocalProject` failed about
  one run in 40 (test only; no product change). It asserted that the local
  project's description wins a tie, but `updated_at` has one-second resolution
  and DR-0025's last-writer-wins correctly lets the incoming row win when the
  clock ticks between the two creations. Both timestamps are now pinned, and
  the two cases that used to happen only by accident, newer incoming wins and
  an exact tie keeps the local row, are tests of their own. The old test failed
  3 of 3 with a forced tick; the new ones pass with it, and 600 runs are
  clean.

### Upgrade notes

For scripts:

- Exit 2 now means a usage error. A script that treated any non-zero as failure
  is unaffected. One that tested for exactly 1 to mean "not found" still gets 1
  for a lookup that found nothing, and now gets 2 for a mistyped command line.
- `--json`, `--db` and `--debug` go before the verb. After it they are refused,
  where they used to be ignored (`--json` after the verb printed plain text).
- A name that begins with a dash needs `--`: `kb project show -- -name`.
- `kb --db PATH init`, `index` and `merge` are refused. Use `kb init PATH`, and
  `merge`'s `-a`, `-b` and `-out`.
- `record list` exits 1 for a typo'd `--status`, `--kind`, `--trigger`,
  `--initiative` or `--project`, and 2 for a malformed `--since`. A script that
  relied on those returning an empty list should check for the value first.
- `kb merge` fails on a missing or empty input instead of merging an empty
  database. A zero-byte `.db` left by an earlier failed merge can be deleted.

For library callers:

- `AddProject`, `AddProjectWithStatus`, `AddConcept`, `AddConceptWithIdentifier`,
  `ResolveConceptName`, `RenameProject`, `RenameProjectRow` and `RenameConcept`
  trim the name and return an error for a blank one. `AddObservation` and
  `AddObservationWithSource` return an error for a blank body. A caller that
  passed untrimmed names now gets the trimmed name's row, not a new one.
- New exports: `CleanName` and `DistinctRecordValues`.

## v0.0.12 — 2026-09-23

The corpus-improvement toolchain moves out of `cmd/kb` and into the
importable `knowledge` package, so a second program can drive it in-process
instead of shelling out to `kb`, plus a concept delete verb, a `cancelled`
record status, and a sweep of bugs that only showed up by running a build from
before each change against the new one on real data. Every fix below was
written red first, and each move was checked by diffing the old binary's output
against the new one, byte for byte.

### Added

- **The workflow logic is now library API** (DR-0035). `cmd/kb` is `package
  main`, so everything implemented there was reachable from exactly one caller,
  the `kb` binary. It moved into the root package as exported functions that
  take text or bytes and return results, never touching `flag`, `io.Writer`,
  `os.Exit` or a file, so a consumer's own permission layer stays in charge of
  its writes. Each of the verbs below is now flag parsing, one library call and
  rendering, and its output and `--json` shape are unchanged. Record ingest
  (`kb ingest`) was not part of this move and stays in `cmd/kb`.
  - `IngestDocument` with `DocumentIngestOptions`/`DocumentIngestResult`:
    create or reconcile a document, tagging and density-scoring each section.
    The path is stored exactly as given and `DocumentByPath` is an exact match,
    so a caller must pass one form consistently.
  - `TagDocumentText`, `EligibleTagConcepts`, `FuzzyEligible`,
    `FuzzyTagDocumentText` and `FuzzyTagInsertion`: text in, text out, for
    `document tag` and `document fuzzy-tag`. `StripCodeSpans` is exported too.
  - `SuggestConcepts` with `ConceptSuggestions`, `ConceptCandidate` and
    `NearExistingMatch`, and `CandidateTerms`, the one definition of a
    candidate word that concept suggestion and the frontmatter keyword scorer
    share.
  - `ProposeFrontmatter` and `ApplyFrontmatter`, which take the file's bytes
    and return the new bytes. Git access sits behind a `Provenance` interface,
    with `GitProvenance` as the default and this module's only `os/exec`, so a
    caller can supply its own authorship and dates, and tests need no real
    repository. `ApplyFrontmatter` creates a new-candidate keyword's concept
    before returning (never on a dry run).
- `kb concept delete NAME [--force] [--dry-run]` (DR-0039), backed by
  `ConceptUsage`, `DeleteConcept` and `ConceptInUseError`. There was no way to
  remove a concept, so a junk one had to be deleted with raw SQL, which skips
  the search index. NAME matches exactly, including case. A concept still
  linked to a project, observation, record or document section is refused with
  the counts unless `--force`, which removes only the links. `--` lets a name
  that looks like a flag (`---`) be deleted. Deletion is local to one database,
  with no tombstone: the command's output and `kb-concept(1)` say that a
  `merge` or `import` from a database that still has the concept brings it
  back, and that a file still naming it recreates it when that file is next
  ingested after it changes (an unchanged file is skipped) or on a rebuild.
- `cancelled` joins the record `status` vocabulary (DR-0038): adopted, then
  abandoned. It differs from `rejected` (never adopted) and `superseded`
  (replaced), and needs no replacement record. The reason goes in the record's
  body, by convention. `kb help record` documents it.

### Changed

- **Concept names that begin or end in punctuation now match** (DR-0037).
  `C++`, `F#` and `.NET` could never match a mention, because the matcher was
  `\b` + name + `\b` and `\b` only fires between a word character and a
  non-word one; and `.NET` matched inside `ASP.NET`. A mention now counts when
  it is not glued to an ASCII word character on either side, which is exactly
  what `\b` meant for a name that starts and ends with a letter: a differential
  test against the old regex agrees on 4,000 random strings. A name with no
  letter or digit never matches, or a junk concept such as `...` would match
  every ellipsis. This reaches `MatchConceptNames`, `MatchConceptNameCounts`,
  `RecallByConceptNames`, tag density, density-linking and `document tag`.
- **A wikilink inside a code span or fenced block no longer mints or links a
  concept**, in both document and record ingest (DR-0037). Documentation about
  wikilink syntax was minting junk concepts (`...`, `recall: ...`); over 159
  real documents ingest went from 213 concepts to 147 with none newly minted,
  and all 66 that stopped were examples inside code. A database ingested with
  v0.0.11 may still hold some; `kb concept delete` removes them.
- **`document fuzzy-tag` proposes far fewer false matches** (DR-0036). A dry
  run over 159 real documents against the real vocabulary returned 614
  proposals, most of them noise: `fts` for `its` (96 times), `drift` for
  `draft` (47), `madr` for `made`, and distance-2 substitutions like
  `retirement` for `requirement`. A concept shorter than 6 letters is no
  longer fuzzy-matched without `--concept`, and a distance-2 match needs a
  6-letter shared prefix. That took 614 to 224 proposals, all 14 measured false
  positives to zero and kept all 10 measured true positives. The constants
  are provisional. A short concept's plural (`merge` for `merges`) now needs
  `--concept`.
- **`concept suggest` requires terms to share a first letter** to be treated as
  spelling variants or near-existing (DR-0036), which removes
  `nested ~ testing`, `nesting ~ testing` and `around ~ grounding`, and stops
  `preference` clustering into `reference` and `treats`/`treating`/`treated`
  into `created`.
- `document frontmatter --accept-keywords` accepts a name only if it is a known
  concept or one of the report's proposed new candidates, checked before
  anything is created or written (DR-0036). It used to write any string into
  the file and mint a real concept for it, and it did so under `--dry-run`.
- The `--debug` trace for the lifted verbs is coarser: one event at the call
  boundary rather than one per internal KB call, and `concept suggest` logs the
  candidate count rather than the item count.

### Fixed

- **Output that changed between identical runs** (DR-0037), found by repeating
  every verb and by an audit of each `range` over a map. Five sites: known-
  concept proposals in `document frontmatter` (now sorted); `--accept` and
  `--set` writing new frontmatter keys in map order, which changed the key
  order written into users' files (two orders in eight runs; now the fixed
  order title, author, dateCreated, dateModified); the unknown `--set` field
  named in an error; `RemovedHeadings` after a re-ingest, in text and `--json`
  (now the old document's own order); and the notes `project rename` prints
  when index refreshes fail (now sorted by directory). A pre-fix build gave 12
  different outputs in 12 runs of the same script; this one gives one.
- `document frontmatter --dry-run` with a new-candidate keyword created the
  concept in the database while leaving the file alone (DR-0036).

## v0.0.11 — 2026-09-23

Three new corpus-improvement features, plus a search bug fix — fuzzy
near-miss tagging, a frontmatter provenance generator, and fuzzy
candidate clustering, each of which surfaced and fixed a real
self-contradiction in its own design doc before shipping, verified by
hand rather than trusting the design's own worked examples.

### Added

- `kb document fuzzy-tag --project NAME [--concept NAME,...] [--dry-run]`
  (DR-0032) catches what `tag`'s exact whole-word matching can't: a
  typo, plural, or simple tense variant of a known concept's name
  (Levenshtein distance, not paraphrase). Inserts a footnote marker at
  the near-miss and a footnote definition carrying the canonical
  `[[Concept]]`, rather than bracket-wrapping the near-miss text itself,
  so `ResolveConceptName` never mints a duplicate concept from a
  misspelled or inflected form. `retrieval.go` gains the exported
  `LevenshteinDistance`/`StripCommonSuffix` primitives, shared with
  fuzzy clustering below. Live-smoke-tested against a real scratch
  document; a follow-up review caught and fixed a real corruption bug
  before release — two near-misses in the same section spliced a later
  footnote marker *inside* an earlier one's own definition text, from a
  single running byte-offset delta that overcorrected.
- `kb document frontmatter PATH [--accept FIELD,...] [--accept-keywords
  NAME,...] [--set FIELD=VALUE] [--dry-run]` (DR-0033): propose-then-
  accept `title`/`author`/`dateCreated`/`dateModified`/`keywords` for
  one document, deriving signals from the document's own prose and
  git/filesystem provenance — this module's first `exec.Command`
  dependency. `title`/`author`/`dateCreated` are absent-only, never
  overwritten once present; `dateModified` is the one exception, always
  refreshed on an accepted run. `author` is a layered signal (a byline
  in the prose, then git's earliest-commit author, then `git config
  user.name`), shown together when they disagree rather than collapsed
  to one guess. `keywords` diffs the current list against known-concept
  mentions and new candidate terms distinctive to the document itself,
  fixing an idf-degeneracy bug the design doc flagged in its own draft.
  Live-smoke-tested against a real git-tracked project; a follow-up
  review caught and fixed two bugs — an explicitly-empty frontmatter
  value (`title: ""`) locked a field from ever being filled in, and
  `--set FIELD=` (an explicit empty value, documented as bypassing the
  absent-only rule) silently no-op'd instead of writing.
- Fuzzy candidate clustering for `kb concept suggest` (DR-0034):
  spelling variants of the same underlying term (`chunking`/
  `chunkings`/`chunked`) now merge into one candidate before scoring,
  not after, so a signal split across variants no longer falls
  individually below the occurrence/distinctiveness floor. A candidate
  fuzzy-close to an already-known concept is excluded from candidacy
  entirely and reported in a trailing near-existing section — that's
  `fuzzy-tag`'s job, not this command's. `--json`'s shape changes from a
  bare array to `{"candidates": [...], "near_existing": [...]}` — a
  breaking change for any existing consumer. Live-smoke-tested against
  the real `agents/knowledge.db`, which found a second problem beyond
  the shared design correction: a flat distance-1 threshold with no
  length floor produced heavy false-positive clustering on short common
  words (`table`+`stable`+`able`, `old`+`cold`+`told`+`hold`+`fold`),
  fixed with a length gate.

### Fixed

- `kb search TERM` threw a raw SQLite error (`fts5: syntax error`)
  instead of a normal no-results outcome for any term containing bare
  punctuation FTS5's own query grammar treats as significant (a `.` in
  a version string, the case found). `Search` now retries once, quoting
  the term as an FTS5 phrase, but only when the first attempt fails with
  an FTS5 syntax error specifically, so `kb-search(1)`'s documented raw
  query syntax (multi-word AND, quoted phrases, `prefix*`) keeps working
  unchanged.

## v0.0.10 — 2026-09-18

The project-rename completion this module had refused to do since v0.0.9,
an answer to the foreign-ADR question open since 2026-09-15, and two
corpus-improvement verbs that surface candidates for a human to confirm
rather than committing anything themselves.

### Added

- `kb concept suggest [--project NAME] [--limit N]` (DR-0028), a
  read-only verb that scans every record body and document section body
  and prints candidate *new* concepts, ranked by corpus-wide
  distinctiveness — a TF-IDF shape: occurring several times, confined to
  relatively few items. It closes the half of corpus improvement that
  density-linking cannot reach, since `MatchConceptNameCounts` only
  matches against concepts that already exist and can never propose one.
  Never writes to the database. Live-tested against the real workspace
  corpus it is a roughly-50%-signal mechanical pass — `rename`,
  `harvey`, `divergence`, `collision`, `uuid` genuinely useful alongside
  generic nouns like `row` and `table` — so a human still curates.
- `kb document tag --project NAME [--concept NAME,...] [--dry-run]`
  (DR-0029), a pure file operation that gives that judgment somewhere to
  land in the corpus itself: it inserts an explicit `[[Name]]` wikilink
  at the first safe occurrence of each eligible concept in every
  already-ingested document for a project, so the next
  `kb document ingest` links it through the wikilink path that already
  exists. Eligibility reuses DR-0027's density threshold verbatim with no
  `--concept` given. A minimal position-based edit on the raw bytes, no
  parse-and-rerender cycle, excluding frontmatter, fenced blocks, inline
  code spans, anything already inside `[[...]]`, and — found live,
  smoke-testing against a real document — the document's own first H1.
  Both-or-neither across a project's document set; a second run is a
  no-op.
- Density-based concept linking on document ingest (DR-0027): a known
  concept mentioned more than once in a section, outside inline code
  spans and fenced blocks, is auto-linked. `tag_density` is deliberately
  left counting the unfiltered text — it is the raw signal the threshold
  is applied to, not the threshold's output.

### Changed

- `kb project rename OLD NEW` now completes for a project that owns
  records (DR-0026), lifting v0.0.9's outright refusal. The fix is
  smaller than either shape originally sketched: `records.project_id`
  never needs to change, since it is a stable foreign key and only
  `projects.name` moves, so the corpus rewrite is a pure file operation —
  every owned record's `project:` frontmatter edited in place,
  both-or-neither, `--dry-run` and `--root` supported — and the next
  ordinary `kb ingest` takes the UPDATE path rather than the INSERT that
  collided. New `RenameProjectRow` library method does the DB-only rename
  without the records-owning guard, called only after every file is
  confirmed rewritten.
- A colleague's MADR-format ADR is modeled as a **document**, not a
  record (DR-0027). We do not own its status and can never promote or
  supersede it. Answering it that way dissolves the identity collision
  the question had been stuck on — both dialects number from 0001, but a
  document never enters the records identity tuple at all — so the
  `dialect` column, the `external` scope value and the `--format madr`
  adapter are moot rather than deferred. Accepted cost, stated rather
  than glossed: a document cannot be cited from `relates_to`, so a record
  responding to a foreign ADR links it in prose.
- `ParseDocumentFile` falls back to a document's first true H1 when
  frontmatter supplies no `title:`, instead of the caller's filename
  (DR-0027). General rather than MADR-specific, but MADR is what
  surfaced it, having no frontmatter at all.
- `user_manual.md`'s verb table is current again — it was six verbs out
  of date (`ingest`, `record`, `index`, `document`, `init`, `topics`) and
  linked a `DECISIONS.md` removed when this module became the decision-
  record format's first conversion pilot. `kb(1)`'s DESCRIPTION and SEE
  ALSO now name records and documents too.

### Fixed

- `kb merge` could silently keep a stale project or concept name, or drop
  a side outright, depending on merge order, because `INSERT OR IGNORE`
  cannot distinguish a uuid collision from a name collision; `kb import`
  hard-failed the *entire* import rather than one row on the same
  collision (DR-0026). Conflict resolution for both moves from name-keyed
  to uuid-primary, reconciling `name` by `updated_at` the same way
  `description` and `status` already did, with a name-keyed fallback
  retained for two genuinely independent, never-synced entities that
  happen to share a name. Both were traced live, not merely suspected,
  and shipping the rename completion without them would have made the
  corruption risk worse by making the trigger routine.
- `kb index ROOT --all` descended into hidden directories (DR-0031), so a
  git worktree — which keeps a whole second copy of the tree under
  `.claude/worktrees/<name>/`, corpus and generated `index.md` included —
  was reported as a corpus in its own right. Under `--check` that is a
  duplicate of a corpus already listed; in write mode `--all` would have
  *rewritten* the worktree's copy, editing a throwaway tree instead of the
  real one. The two-signal rule added last release could not catch it:
  a worktree copy satisfies both signals correctly, because it is a
  byte-identical copy of something that genuinely is a corpus. The walk now
  prunes any dot-prefixed directory, but never `ROOT` itself, so naming a
  hidden directory as `ROOT` still finds the corpora inside it. Found
  running `--all` live against `~/WorkLab`, where it reported 8 corpora
  where 6 was right.
- `TestParseRecordFile_RoundTripsEveryLiveRecord` (DR-0030) named five
  fixed corpus paths, three of which went stale when DR-0021's
  `agents/projects/<project>/decisions/` layout was rolled out. It had
  quietly stopped exercising four fifths of the live corpus — 54 records
  of 267 — and reported that as a count shortfall rather than a discovery
  failure. Corpora are now discovered rather than listed, using the same
  two-signal rule `kb index --all` settled on (a record-shaped filename
  *and* real frontmatter in the file), which also keeps a foreign MADR
  corpus out of a round-trip test it would fail by construction. The
  remembered count floor is gone, in favour of the per-file property the
  test was always really asserting — the same lesson DR-0015 drew.

## v0.0.9 — 2026-09-17

Rename verbs for projects and concepts, cross-machine reconciliation for
their mutable fields, and a multi-corpus mode for `index.md` — three items
closed out of `TODO.md`, two of them correcting their own original
diagnosis along the way.

### Added

- `kb project rename OLD NEW` and `kb concept rename OLD NEW` (DR-0024)
  replace the only prior route, raw SQL against `projects.name`/
  `concepts.name`. `project rename` refuses if `NEW` already exists or if
  the project owns any records — a live repro showed that renaming a
  project with a corpus and then re-ingesting it silently mints a phantom
  project under the old name and duplicates the record under it, data
  corruption rather than just a stale search label. `concept rename` has
  no such corpus to desync and ships unconditionally beyond the name
  collision every rename needs. Both reuse `refreshProjectFTS`/a new
  `refreshConceptFTS` so search never lags a rename.
- Cross-machine last-writer-wins for a project or concept's mutable
  fields (DR-0025). `concepts` gains `updated_at`; `kb merge`'s conflict
  path becomes `INSERT OR IGNORE` for new rows plus a guarded
  `UPDATE ... FROM ... WHERE incoming.updated_at > existing.updated_at`
  for existing ones, so whichever side is actually newer wins regardless
  of which side is applied first. `kb import` gained the matching
  comparison. Observations needed none of this — DR-0023 already resolved
  observation correction via supersession.
- `kb index ROOT --all [--check]` discovers every corpus under `ROOT`
  that already has an `index.md`, refreshes or checks each one, keeps
  going past an individual corpus's failure, and exits non-zero if any
  needed attention — closing the index-staleness item's multi-corpus
  half. Running it live against the real workspace caught a bug before it
  shipped: matching on the filename alone picked up unrelated `index.md`
  files (a docs site, a blog) and would have silently overwritten them in
  write mode. Fixed by requiring both a content-heading match and an
  actual record file in the directory.

## v0.0.8 — 2026-09-16

A bug-fix-and-small-features release: closing the `index.md` staleness gap
at its source, and giving observations a correction path.

### Added

- `kb index PATH --check` compares a fresh render against `PATH/index.md`
  byte-for-byte and fails with "does not exist" or "is stale" instead of
  writing, naming `kb index PATH` as the remedy — same exit-code convention
  as `kb search`'s fix last release.
- `kb observation update ID BODY...` gives observations a correction path
  they never had. It does not mutate the old observation — it inserts a
  new one (inheriting the original's project and kind) and links the two
  with a new `observation_relations` table, `record_relations`' own shape
  reused rather than reinvented (see DR-0023, `knowledge/decisions/`,
  which reverses DR-0012's original observations-stay-immutable stance
  after review found that an in-place edit destroys history and answers
  the amend-vs-supersede question by accident). The old observation's
  body is never rewritten, so history retention is free; no `updated_at`
  column was added, since the new observation's own `created_at` is the
  correction timestamp. `kb observation show ID` now resolves and prints
  `supersedes`/`superseded_by`, mirroring `kb record show`. `kb merge`
  and JSON-L export/import carry `observation_relations` from this
  release, not as a follow-on, per the standing rule that a table missing
  from the merge summary is a table whose loss goes unreported.

### Fixed

- `kb record set-status` and `kb record supersede` now refresh a corpus's
  `index.md` themselves after a successful write, via the new
  `regenerateIndexIfPresent`, closing the gap `--check` only detects. Only
  when one is already present — never creating one where a corpus hasn't
  opted in. Together with `--check`, this fixes `index.md` silently
  drifting from `status`/`kind`/`trigger`/`superseded_by`/title changes,
  which bit WorkLab twice.

## v0.0.7 — 2026-09-15

A bug-fix release: four defects found running v0.0.6 against real corpora
(clasm, WorkLab, caltechauthors), all in `kb ingest`'s re-run behavior plus
`kb search`'s exit code.

### Fixed

- `kb ingest` no longer leaves stale edges behind on re-ingest. A
  `relates_to`/`supersedes` entry removed from a record's frontmatter, or a
  `[[wikilink]]` concept tag removed from its body, is now actually removed
  from `record_relations`/`record_concepts` — previously both only ever
  grew, since re-ingest inserted what a file currently declared but never
  deleted what it no longer declared. The new `ClearRecordRelationsFrom`/
  `ClearRecordConcepts` (and `ClearDocumentSectionConcepts` for
  `kb document ingest`) run before every re-insert.
- `[[0007]]`/`[[DR-0007]]` in a record body — the natural way to write "see
  DR-0007" — no longer silently mints a junk concept named after the record
  id. It is now skipped with a warning pointing at `supersedes`/
  `relates_to`, the actual way to cite another record.
- `kb ingest` now updates a record's stored `path` when its file moves but
  its content is unchanged, rather than leaving `records.path` (and so
  `kb export`'s `agents/knowledge.jsonl`) silently stale. The "DR-%s was
  stored at %s" warning now fires only when content changes alongside the
  path, since a path change alone is an ordinary move, not a possible id
  collision. The message for a record with no file at its stored path no
  longer asserts deletion as the only explanation and recommends
  `kb record remove` outright — a moved file looks identical to a deleted
  one, and the old wording would have walked a user into deleting live
  records.
- `kb search` now exits 1, in both text and `--json` mode, when it finds
  nothing, matching the workspace's search-tool convention instead of
  exiting 0 with an empty result.

## v0.0.6 — 2026-09-13

Three related features, each building on the last, extend `knowledge`
toward a small-model-friendly knowledge base: inline concept tagging,
embedder-free concept-based retrieval, and narrative/article ingestion at
graduated abstraction levels.

### Added

- `kb ingest` resolves `[[Name]]` wikilinks in a record's body, and its
  previously-unused `tags` frontmatter field, into concepts linked via a
  new `record_concepts` table — case-insensitive via a new
  `ResolveConceptName` (kept separate from `AddConcept`/`kb concept add`,
  which stays exact-match, since a human typing a concept name at the CLI
  is a deliberate act, unlike capitalization in prose). `kb record concepts
  RECORD_ID` shows the result.
- New `retrieval.go`: `MatchConceptNames` finds known concepts mentioned
  (whole-word, case-insensitive) in arbitrary text, and
  `RecallByConceptNames` returns observations, records, and documents
  linked to a given set of concepts, merged and ranked by match count then
  recency — a cheap, embedder-free first pass for small/CPU-only models,
  not project-scoped.
- New `documents`/`document_sections` entity ingests narratives and
  articles (Markdown, Fountain, plain text — PDF is a reserved format
  value, not yet supported, since no text-extraction path exists anywhere
  in this workspace) at graduated abstraction levels: a document-level
  gist plus one row per structural section (Fountain scene headings via
  `github.com/rsdoiel/fountain`, the same module harvey already depends
  on; Markdown headings; a whole file for plain text), each with its own
  summary lifecycle (`unsummarized` → `drafted` → `reviewed`, mirroring
  decision records' own author/promote split) via new
  `kb document ingest/list/show/draft/review` verbs. Frontmatter
  (title/description/pubDate/author/keywords, matching antennaApp's own
  documented vocabulary) seeds metadata and an initial gist when present,
  without ever being required. Re-ingesting a changed file matches
  sections by heading text and flags a changed one `summary_stale` rather
  than discarding its summary; a heading with no match is reported, never
  deleted. Only a reviewed summary is ever indexed for search or returned
  as trustworthy content by `RecallByConceptNames`.
- All three features carry through `kb merge` and `kb export`/`kb import`
  — `record_concepts`, `document_section_concepts`, and
  `documents`/`document_sections` all appear in every portability path,
  per the project's standing rule that a table missing from the merge
  summary is a table whose loss goes unreported.

### Changed

- The independent hand-rolled CLI flag/positional parsers in `ingest`,
  `record`, `source`, and the new `document ingest` were consolidated into
  one shared `splitFlags` helper; `source add` now errors on an
  unrecognized flag instead of silently dropping it.

## v0.0.5 — 2026-08-28

Decision records now travel on every portability path — `merge`, `export`,
`import` — not just `ingest`. Before this, `records` and `record_relations`
were invisible to all three, so running `merge` (or an export/import round
trip) discarded every decision record while reporting success (DR-0013).

### Added

- `kb merge` carries `records`/`record_relations` in its union, reports a
  record collision keyed by identity (workspace, project, scope, record id)
  rather than by `project_id` or a project's own uuid, and reports a
  **content divergence** — same record, different text — without blocking
  the merge on it. `--json` gains `content_divergences` alongside
  `collisions_reconciled` and the per-table `tables` summary.
- `kb export`/`kb import` carry `record`/`record_relation` JSON-L lines. A
  `-project`-scoped export carries only that project's records; a
  workspace-tier record, having no project, appears only in an unscoped
  export (DR-0019). Import matches a record by identity, not uuid — two
  machines' ingest of the same file mint different uuids for it, so
  identity is the normal case for "already present," not the exception
  (DR-0018) — and resolves its project by name for the same reason.
- `CollisionReport`/`ReconcileCollisions`/`DivergenceReport`,
  `NormalizeForMerge` (migrates a merge scratch copy to the current schema
  before ATTACHing, so a database predating a table still merges).
- Every table `merge` carries now appears in its per-table summary, so a
  table that would lose rows says so.

`kb record new`'s default write location for project-scoped records moves to
`agents/projects/<project>/decisions/` (was `<project>/decisions/`), part of
a workspace-wide reorganisation moving process artifacts out of project
repositories (DR-0021). `kb` also stops silently creating an ambient
`agents/knowledge.db` in whatever directory it happens to be run from — a new
`kb init` verb is now the explicit way to start a workspace (DR-0021,
DR-0022).

### Added

- `kb init [PATH]`: creates a schema-only, idempotent `agents/knowledge.db`,
  the same shape as `git init`. Documented at `kb-init(1)`.
- `kb record new --dir DIR`: overrides the default write location for a new
  record, relative to `--root` like every other stored record path.

### Changed

- `kb record new --project P` (no `--dir`) now writes to
  `agents/projects/P/decisions/` instead of `P/decisions/`. `--workspace`
  scope is unchanged (`agents/decisions/`). Existing corpora under the old
  layout are not migrated by this change.
- Any verb resolved through the ambient default (no `--db` given) now fails
  with a message pointing at `kb init` or `kb import -in FILE` if
  `agents/knowledge.db` doesn't already exist, instead of silently creating
  one — fixes `kb record new`/`kb observation add`, run from inside a
  project directory, building a stray nested workspace. An explicit `--db
  PATH` and `kb import` keep today's open-or-create behavior unchanged.

### Notes

- DR-0013 through DR-0019 cover the records-portability effort's design and
  implementation decisions; DR-0021 and DR-0022 cover the record layout and
  workspace-init change. See `knowledge/decisions/index.md`.

## v0.0.4 — 2026-08-26

Adds Decision Record support: episode-scoped Markdown files with YAML
frontmatter, kept in a project's `decisions/` directory and indexed as
first-class rows, so the reasoning behind a decision is retrievable rather than
living only in files the knowledge base never reads. The format is specified in
`~/WorkLab/DECISION_RECORD_FORMAT.md`; this release is its reference
implementation.

### Added

- **Schema**: `records` and `record_relations`. A record's identity is
  `(workspace, project, scope, id)`. The workspace name is the directory name
  of the workspace root, derived from the path rather than written in a file,
  because every workspace has an `agents/decisions/` and two of them may each
  hold a DR-0001. Records are indexed into `kb_fts` with
  `source_type = 'record'`. Existing databases migrate lazily on open.
- **`kb ingest PATH`** — walks a tree of `NNNN-slug.md` records and resolves
  `supersedes`/`relates_to` in two passes, so forward references work. Additive:
  a record whose file has vanished is reported, never deleted. Never writes to
  a record file.
- **`kb record list|show|new|set-status|supersede|fmt`** — `new` scaffolds with
  `status: proposed`; `supersede` writes both sides and the relation together
  or not at all; `fmt` normalises a tree and never writes to the database.
- **`kb index PATH`** — generates `decisions/index.md`, one greppable line per
  record, newest first. Byte-identical to the Deno generator it replaces.
- **Standard options** `-help`, `-license` and `-version`, which `kb` had
  lacked entirely, declared through a `flag.FlagSet` so each is accepted in
  either dash form.
- **`kb help topics`** — the topic index. Named `topics` rather than the
  conventional `index` because `index` is a verb here.
- **Library**: `ParseRecordFile`/`RenderRecordFile`, `ListRecords`,
  `RecordByIdentity`, `RecordsByRecordID`, `RecordsUnderPath`, `NewUUID`,
  `Today`, and a `SourceType` field on `KBSearchResult`.
- **TUI**: browse a project's records read-only, with `r`.
- Man pages for `kb-ingest(1)`, `kb-record(1)`, `kb-index(1)`.

### Changed

- `kb search` labels a hit by its source table rather than its own kind, so a
  decision record reads `[record]` rather than `[decision]`.
- Informational commands no longer open or create a database. `kb index` joins
  `kb merge` in this, since it builds from the record files and never queries
  one — it had been leaving a database in whatever directory it ran in.
- The frontmatter struct declaration is the canonical format specification:
  field order, flow-styled sequences, and a double-quoted string type for the
  seven fields the format requires quoted, so every writer produces
  byte-identical output.

### Fixed

- A record path read from the database is confined to the workspace root before
  it is read or written. `filepath.Join` cleans a path without confining it, so
  a path reaching the database by some route other than ingest could otherwise
  have made `set-status` rewrite an arbitrary file.

### Notes

- Vocabularies for record `status`/`kind`/`trigger` are documented and reported
  against, **not** enforced: in a format several tools write to, a typo should
  be a fixable row and not a failed run. Observation kinds remain enforced,
  which is a deliberate asymmetry.
- Adds `gopkg.in/yaml.v3` as a direct dependency.
- Verified against 205 real records across five corpora, all round-tripping
  byte-for-byte.

## v0.0.3 — 2026-08-08

Adds JSON-L export/import: `ExportJSONL`/`ImportJSONL` in the knowledge
package, plus `kb export [-project NAME] [-out PATH]` and `kb import [-in
PATH]`. A portable, no-file-access alternative to `merge` — the resulting file
can be pasted, emailed or committed to git, then applied elsewhere with
`import`. Projects and concepts are matched by name (existing local rows win),
sources by identifier, observations and links by uuid, so re-importing the same
file is a no-op.

## v0.0.2 — 2026-07-28

Adds project status management: `AddProjectWithStatus` and `SetProjectStatus`,
plus a `--status` flag on `kb project add` and a `kb project set-status NAME
STATUS` verb, all validated against `concept`/`active`/`paused`/`concluded`.
`AddProject`'s default behaviour is unchanged.

## v0.0.1 — 2026-07-27

Proof-of-concept pre-release. Full CRUD API (projects, observations, concepts,
sources) with FTS5 search and cross-machine merge; `cmd/kb` ships a git/go-style
CLI with `--json` output and a read-mostly bubbletea TUI; `--debug` emits a
JSONL trace of every knowledge-base call and TUI event. Extracted from harvey's
`knowledge.go`/`knowledge_merge.go`.
