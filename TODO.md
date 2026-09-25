
# Action items

## Requested features

- [x] `[[double-bracket]]` inline concept tagging when ingesting Markdown
  record bodies, beyond formal frontmatter. Filed as
  `wikilink-tagging-feature-request.md` (2026-09-08) — inspired by
  [Build a digitally sovereign second brain](https://www.raspberrypi.com/news/build-a-digitally-sovereign-second-brain/)
  (Raspberry Pi magazine). Implemented 2026-09-13: see
  `wikilink-tagging-design.md` and `wikilink-tagging-plan.md` (W1-W5). Both
  `[[Name]]` body scanning and the previously-unused frontmatter `tags` list
  resolve into `record_concepts`, case-insensitively via a new
  `ResolveConceptName` (not `AddConcept`/`kb concept add`, which stay
  exact-match); visible via `kb record concepts RECORD_ID`; carried through
  `knowledge_merge.go` and `jsonl.go` export/import for portability.
  Observation bodies (`kb observation add`) remain out of scope, as decided
  at filing.

- [x] Concept-tag-based retrieval query (concept names → linked
  observations/records). Filed as `concept-tag-retrieval-feature-request.md`
  (2026-09-08). Implemented 2026-09-13: see `concept-tag-retrieval-design.md`
  and `concept-tag-retrieval-plan.md` (W1-W2). New `retrieval.go`:
  `MatchConceptNames(text)` (whole-word, case-insensitive match against
  known concepts) and `RecallByConceptNames(names, limit)` (merged,
  match-count-then-recency-ranked results across both `observation_concepts`
  and `record_concepts`, read-only — never creates a concept, unlike
  `ResolveConceptName`). Not project-scoped, per the original motivation.
  Consuming this from harvey's `UnifiedMemory.Recall` is separate,
  harvey-side work, not done here.

- [x] Ingest narratives/stories (Markdown, Fountain) and articles/posts
  (Markdown, text) as a new `documents` entity, stored/indexed at graduated
  abstraction levels (gist, section/scene, one row each) rather than as one
  flat body. Filed as `narrative-documents-feature-request.md`
  (2026-09-13); implemented 2026-09-13, see `narrative-documents-design.md`
  and `narrative-documents-plan.md` (W1-W8). PDF is a documented, reserved
  format value but explicitly not ingestible yet — no extraction tooling
  exists anywhere in the workspace. Fountain segmentation uses
  `github.com/rsdoiel/fountain` directly (the same module harvey already
  depends on), not a second parser. Tagging/tag_density reuse
  wikilink-tagging and concept-tag-retrieval verbatim — the first real
  validation that those two features' shapes generalize to a second
  consumer. Review workflow (`unsummarized` → `drafted` → `reviewed`)
  mirrors decision records' propose/accept split; only a reviewed summary
  is ever FTS-indexed or returned as trustworthy content from
  `RecallByConceptNames`, which widened to a third merged source. Re-ingest
  matches sections by heading and flags a changed one `summary_stale`
  rather than discarding its summary. Portability (merge + JSON-L) covers
  all three new tables. Consuming any of this from harvey's
  `UnifiedMemory.Recall`, and building an eventual interactive/dialogic
  re-ingest mode in harvey, remain explicitly out of scope here.

- [x] **DONE 2026-09-23 (DR-0038): `cancelled` is in the status vocabulary, documented in `kb help record`, with the reason-in-the-body convention.** *Original request:* **Add `cancelled` to the `status` vocabulary.** Raised 2026-09-21 from a
  real case in `~/WorkLab`: `cold` DR-0023 recorded eleven decisions for a
  parameterized report (cold#110), was reviewed and accepted, and the issue was
  cancelled the same day — the report already shipped in v0.0.53 turned out to
  answer the requester's need once he read its CSV into a spreadsheet and
  pivoted it. No second report was needed.

  None of the four current statuses tells that story:

  - `rejected` misstates it. The decisions were not rejected; they were
    accepted on their merits, and the reasoning is still sound. `rejected`
    belongs to a record whose decisions were *never adopted*.
  - `superseded` requires a replacement, and writing one to get the status is
    the tail wagging the dog: it inflates the corpus with a record whose only
    content is "this did not happen", and it labels the original as superseded
    by a decision that replaced nothing.
  - `accepted` leaves a reader believing the work is live. This is the one that
    actually costs something — a reader six months out finds an accepted record
    describing a report that does not exist and cannot tell whether it was
    never built or was built and removed.
  - `proposed` is simply false.

  **The distinction `cancelled` carries is temporal.** `rejected` is *never
  adopted*; `cancelled` is *adopted, then abandoned*. That difference is
  exactly what a reader needs, because a cancelled record's content stays
  valuable in a way a rejected one's usually does not — the design reasoning is
  what someone revisiting the question should start from, and cold DR-0023 says
  so explicitly.

  Worth deciding alongside it:

  - Should a cancelled record carry a *reason*, structurally or by convention?
    "Cancelled because the need was met elsewhere" and "cancelled because it
    was deprioritised" are different signals to whoever revisits it. A
    frontmatter field is probably overkill; a body convention may be enough.
  - Does `index.md` need to distinguish it, or is the status column sufficient?
    The column is already rendered, so likely nothing to do.
  - Vocabularies are documented rather than enforced — an unknown value parses
    and carries a warning — so `kb record set-status 0023 cancelled` already
    *works* today. That makes this mostly a documentation change
    (`DECISION_RECORD_FORMAT.md`, `kb-record(1)`'s VOCABULARIES section) plus
    whatever `kb record list --status` and the index renderer assume. Which is
    an argument for doing it properly rather than relying on the warning path:
    the value would otherwise spread through corpora as an undocumented
    convention.

  **Interim workaround used in the meantime:** a short cancellation record that
  supersedes the original, in `~/WorkLab/agents/projects/cold/decisions/`. It
  works and both sides are written, but it is the second bullet above and
  should be revisited once `cancelled` exists.

- [ ] **Ship the knowledge skills and an agent document in this repository**, so a
  language-model harness or a system like Claude Code can use `kb` after installing it,
  without hand-copying files out of the Laboratory workspace. Filed 2026-09-25 at
  RSDOIEL's request. State today, so the item starts from facts:
  - Three skills exist, `setup-knowledge-base`, `update-knowledge-base` and
    `review-knowledge-base`, each a `SKILL.md` plus `scripts/compiled.bash` and
    `compiled.ps1` (`compatibility: claude-code, harvey`). They live in the
    Laboratory root's `agents/skills/` and again in `harvey/agents/skills/`; **every file
    differs between the two copies**, and their `kb_version` pins disagree
    (`setup` says 0.0.10; `update` and `review` say 0.0.12). Nothing states which copy is
    canonical. This repository has neither skills nor an agent document.
  - Wanted: (1) the three skills in this repository, made canonical, with the Laboratory
    and `harvey` copies becoming consumers; (2) an agent document that tells a harness what
    `kb` is and how to use it safely: the verbs, `--json`, the exit-code table (workspace
    DR-0003), that writes go through `kb` and never raw SQL (it keeps the search index in
    step), the decision-record workflow, and the standing rule that a model may write a
    record but never accept one.
  - To decide: the layout (`skills/` or `agents/skills/`); one agent document or an
    `AGENTS.md` plus a `CLAUDE.md` that points at it, since harnesses look for different
    names; how they are distributed (in the release zips, and whether `make install`
    places them for Claude Code); how they are kept from drifting (a test that each
    skill's `kb_version` matches `codemeta.json`, and the four-field release-prep check
    extended to cover it); whether the Windows `.ps1` scripts stay.
  - **Sequence:** after the exit-code work (X6), because the skill scripts currently read
    `kb`'s exit status and the agent document should describe the new codes. `harvey`'s
    own skills sync (H2 of its learning-mode plan, skills 0.6.1) should be checked so this
    does not undo it. Wants a design note and a DR, since it fixes a canonical location.

## To explore

### Bugs collected for v0.0.13

Found 2026-09-24 by probing every verb with empty, malformed and unknown
input against a scratch copy of the real database (v0.0.12 plus `fd588ef`).
None fixed yet; each wants a test written red first. The export/import round
trip was also checked and is clean: 14 tables' row counts identical.

- [x] **FIXED 2026-09-24 (unreleased, for v0.0.13).** *Fix as landed:* new
  exported `knowledge.CleanName(kind, name)` (trim, refuse empty), applied in
  `AddProject`, `AddProjectWithStatus`, `AddConceptWithIdentifier` (so
  `AddConcept` too), `ResolveConceptName`, `RenameProject`,
  `RenameProjectRow` and `RenameConcept`; `AddObservationWithSource` refuses a
  whitespace-only body but stores a real body exactly as given. The legacy
  experiments-to-projects migration calls the unexported `addProject` so a
  strange old name cannot fail `Open`. `import` and `merge` use raw SQL and are
  deliberately untouched: a historical row must still load. The CLI cleans
  the name before echoing it, and `project rename` cleans NEW *before staging
  any file*. **Found while verifying, worse than the report:** `kb project
  rename demo ''` on a project that owns records went through. It rewrote the
  record to `project: ""`, renamed the row to `""`, and the next ingest hit
  the `records.uuid` UNIQUE deadlock DR-0026 exists to prevent. (A padded NEW
  was consistent in v0.0.12, padded in both the files and the row; trimming in
  the library alone would have put it in the files but not the row, so NEW is
  cleaned before any file is staged. An earlier line here called that a
  v0.0.12 bug; it was not.) The blank name is refused and the padded one
  trimmed now (checked old vs new binary on a real scratch corpus). 23 tests, 18 red
  first (`blankname_test.go` at the root and in `cmd/kb`); the other 5 are
  regression guards (untrimmed body, refusal already provided by the
  library). Real-corpus ingest compared old vs new binary: 302 dump rows,
  identical. Man pages not yet updated to mention the trimming. *Original
  report:* **`project add`, `concept add` and `observation add` accept an empty or
  whitespace-only name/body, and store names untrimmed.** `kb project add ''`
  and `kb project add '  '` both exit 0 and create two distinct projects
  (ids 15 and 16, names `""` and `"  "`); `kb concept add ''` creates an empty
  concept; `kb observation add --project P note ''` stores an empty body. All
  three then show up in every list. `' harvey '` is stored as a separate
  project from `harvey`, and a multi-line name is accepted. `record new`
  already refuses an empty `--title`, so the guard exists in one verb only.
  Likely fix: trim and reject empty in the library's `AddProject`,
  `AddConcept` and `AddObservation`, so `import`, `merge` and ingest are
  covered too, not only the CLI.

- [x] **FIXED 2026-09-24 (unreleased, for v0.0.13).** *Fix as landed:*
  `wantsVerbHelp` (`cmd/kb/main.go`) answers a help flag directly after the
  subverb (and after `document review`'s own subverb) with the verb's page,
  before any database is opened, so it works outside a workspace and creates
  nothing. A help flag after other flags is caught two ways: a FlagSet subverb
  returns `flag.ErrHelp`, which `dispatch` now turns into the page (exit 0);
  the hand-rolled parsers share `splitFlags`, which now reports an undeclared
  `-h`/`-help`/`--help` as `flag.ErrHelp` too (`concept delete` wrapped its
  error with `%v`, dropping the chain; now passes it through). Deliberately
  *not* a scan of every argument: `kb observation add --project p note pass -h
  to it` must record its body, and a test pins that. A genuinely unknown flag
  still errors. Checked live: all 33 subverbs x 3 spellings (99 runs) print the
  right page, exit 0, empty stderr, no database created. `subverbhelp_test.go`
  (every help case red first: 119 subtests for the first change, 7 more plus a
  `splitFlags` unit test for the second) and 2 guard tests that already passed. *Not
  changed:* `kb bogus add -help` still fails as an unknown verb (exit 1 outside a
  workspace, 2 inside). *Original report:* **A subverb's `-help` is not handled, so `kb VERB SUBVERB -help` does
  something different for each verb.** Top-level `kb VERB -help` prints the
  manual, as `kb -help` promises, but one level down: `project add`,
  `observation add`, `document tag` and `concept suggest` print the Go flag
  package's `kb: flag: help requested` and exit 1; `project show -help` looks
  up a project named `-help`; `project list -help` ignores it and lists;
  `document frontmatter -help` tries to open a file called `-help`;
  `record show|list|new`, `document list|ingest` and `concept delete` say
  `unknown flag "-help"`. Likely fix: treat `flag.ErrHelp` (and a bare `-help`
  in the hand-rolled parsers) as "print the verb's page, exit 0".

- [x] **FIXED 2026-09-24 (unreleased, for v0.0.13).** *Fix as landed:* a
  `usageError` type (`cmd/kb/usage.go`: `usageErrorf`, `wrapUsage`,
  `isUsageError`, `parseFlags`); `dispatch` exits 2 for one and 1 for anything
  else. About 75 sites converted: every `usage:` message, unknown
  subverb/flag, `X requires ...`, malformed ids, every FlagSet parse failure
  (via `parseFlags`), `splitFlags`' unknown flag and missing value, and the
  commands' own flag-value checks (`--confidence`, `--set`, `--accept`,
  surplus path arguments to `index`, `init`, `ingest`). **Decision taken, flag
  if you disagree:** a value the library rejects after the command line
  parsed (`observation add` with an unknown kind, `project set-status` with an
  unknown status) stays exit 1; and `unknown project`/`unknown concept` from
  `--project`/`--concept` are lookups, so exit 1 like `project show nosuch`.
  `kb -help`'s EXIT STATUS now says exactly this (`helptext.go` and `kb.1.md`;
  the HTML pages are not regenerated). Checked live, old vs new binary over 67
  commands: 35 changed, all 1 to 2 and all real command-line mistakes; the 32
  others unchanged (every success still 0, every not-found/no-results still 1).
  `usageexit_test.go`: 51 cases red first, 8 more mutation-verified, plus
  guards for runtime failures, `--json` and the exits that were already 2.
  My own mechanical edit dropped a word from one message ("document review
  subverb"); caught by auditing the diff and fixed. *Original report:* **Usage
  errors exit 1, but `kb -help` documents 2 for "usage error (bad flags,
  unknown verb)".** Only an unknown top-level verb exits 2. A missing
  argument (`search`, `project show`, `observation add`), an unknown flag
  (`observation add --bogus`) and a bad flag value (`concept suggest --limit
  abc`) all exit 1, indistinguishable from "the verb ran and found nothing".
  Either the code or the EXIT STATUS section is wrong; the workspace
  convention (root `CLAUDE.md`) is 2 for usage errors.

- [x] **FIXED 2026-09-24 (unreleased, for v0.0.13).** *Fix as landed:* the
  survey was wider than the three cases reported. Of 25 verbs probed, 15
  accepted a bogus flag and 19 dropped a surplus argument (recounted from the
  old binary; an earlier figure of 13 here was wrong); now all 25 refuse both
  with exit 2. Three families. Verbs with plain positionals checked only
  `len(args) < N`: new `plainArgs(args, min, max, fixed, usage)` (`usage.go`)
  rejects flag-shaped arguments among the fixed ones and enforces the count,
  with `--` as the terminator so a dash-leading name still works. FlagSet
  verbs never looked at `fs.Args()`: new `noExtraArgs`. The `record` verbs
  checked `<` where they needed `!=`. Also folded in: `source add T more
  words` silently stored the title "T" and lost the rest, `record new ...
  --title A decision` kept only "A", `source link ... --relationship` with no
  value or with a stray argument linked anyway (now `splitFlags`), `search ''`
  and `search ' '` are usage errors instead of "no results". **Behaviour
  change to know about:** a name that begins with a dash now needs `--` on the
  verbs that take a name (`kb project show -- -name`); concept delete already
  worked that way (DR-0039). Free-text tails (`project set-description P a -x
  b`, `source retract 1 note -x`, observation bodies) are unchanged. `project
  list --json` used to print plain text and exit 0, ignoring `--json`; it is
  now refused with a hint that global options go before the verb. Documented
  in a new ARGUMENTS section of `kb(1)` (`helptext.go`, `kb.1.md`; HTML not
  regenerated). Checked old vs new binary over 65 valid commands: identical
  database and identical exit codes except the two intended (`--json` after
  the verb, and a word-split title). `ignoredinput_test.go`: every behaviour
  case red first against a non-validating stub. *Original report:* **Some
  verbs silently ignore what they do not understand.** `kb project
  list -bogus` and `kb project list extra args` exit 0 and list; `kb project
  show x y` looks up `x` and drops `y`. `kb search ''` exits 1 with `no results
  for ""` where a usage error is the honest answer.

- [x] **FIXED 2026-09-24 (unreleased, for v0.0.13).** *Fix as landed:* the
  same silent drop hit `index` and `merge`, which share init's branch in
  `mainRun` (none of the three opens the ambient database), so an explicit
  `--db`/`-db` is now refused for all three with exit 2 and a message naming
  where the target really goes (`kb init PATH`; index reads record files;
  merge takes `-a`, `-b`, `-out`); `dbOptionRefusal` in `cmd/kb/main.go`.
  Refused rather than honoured because `--db` names a database *file* while
  init creates a *workspace* (`PATH/agents/knowledge.db`), and any verb that
  does use the database already open-or-creates an explicit path (DR-0022), so
  nothing is lost. A help request still wins (`kb --db x init -help` prints the
  page). Documented in `kb-init(1)` and the ARGUMENTS section of `kb(1)`
  (HTML not regenerated). Checked live, old vs new binary: old exited 0 for all
  three and created the wrong files, new exits 2 and creates nothing; plain
  `init`, `index` and `--db X project list` unchanged. `dbflag_test.go`, red
  first. *Original report:* **`kb --db PATH init` ignores `--db` and creates
  `./agents/knowledge.db` instead.** `init` takes its own `PATH` argument and never reads the global
  `--db`, which `kb-init(1)` implies but nowhere says, so `kb --db rt.db init`
  reports success while creating a database somewhere the caller did not
  name. Likely fix: refuse `--db` with `init` and name `kb init PATH`.

- [x] **FIXED 2026-09-24 (unreleased, for v0.0.13).** *Fix as landed:* wider
  than reported: every `record list` filter answered a mistake with "no
  matching records". `validateRecordFilters` (`cmd/kb/record.go`) now runs
  before the query. **The rule that mattered:** the status/kind/trigger
  vocabularies are documented, not enforced, so a record can legitimately carry
  a status outside them; rejecting every out-of-vocabulary value would have made
  such records unlistable. So a value is an error only if it is neither in the
  vocabulary nor carried by any record, and the message lists what is known.
  A real value that matches nothing (`--status rejected` on a database with no
  rejected record) stays a valid, empty answer. New library query
  `DistinctRecordValues(field)` (whitelisted to status, kind, trigger,
  initiative, since the name is spliced into SQL). `--project` naming no project
  is an error listing the known ones (exit 1, a lookup); an existing project
  with no records is still empty and exit 0. `--since` must be a real YYYY,
  YYYY-MM or YYYY-MM-DD (exit 2; `--since yesterday` used to compare as text
  and match nothing); `--workspace` with `--project` is exit 2, since
  workspace-tier records have no project. Validation lives in the CLI, not in
  `ListRecords`, which the library and harvey also call. Checked live on a copy
  of the real 44-record database, old vs new, over 25 filter combinations: the
  16 valid ones byte-identical, the 9 mistakes now errors. Documented under
  `list` in `kb-record(1)` (HTML not regenerated). One wording bug of mine
  ("known statuss") caught in that run and fixed test-first. Real data note:
  no record in this database carries an initiative, so `--initiative X` is
  always an error here, which is accurate. `recordfilters_test.go`,
  `recordvalues_test.go`, red first. *Original report:* **`kb record list
  --status bogus` and `--project nosuch` exit 0 with `no matching records`,** where `observation list --project nosuch` and `document
  list --project nosuch` both fail with a not-found error. An unknown status
  or project is a typo the caller wants told about, not an empty result.

- [x] **FIXED 2026-09-24 (test only; no product change).** *Fix as landed:* a
  helper `importSameNameProject(t, stampLocal, stampIncoming)` in
  `jsonl_test.go` pins both projects' `updated_at` explicitly (the idiom the
  DR-0025 tests already use), so the outcome no longer depends on which second
  each `AddProject` landed in. The original test now uses a *newer local* row,
  and the two neighbouring cases that used to happen only by accident are
  tests of their own: newer incoming wins, and an exact tie keeps the local
  row (which pins the strict `>` in `jsonl.go`). Verified: the old test failed
  3 of 3 with a forced 1.1 s tick between the creations; all three new tests
  pass with the same tick; three mutations of the comparison (`>=`, never, always)
  are each caught by exactly the test that names them; 600 focused runs and 200
  runs of the whole import/merge set are clean (the old test failed 1 in 40 and
  1 in 150). Swept for the same assumption elsewhere: a scan for tests that
  build two databases, cross-import or merge them, assert a value and never
  back-date found only this one (the other match, `Records_UnionAcceptance`,
  has empty descriptions on both sides and asserts on records), and 150 runs of
  every import/merge/export test found no other failure. 25 full runs of the
  root package and 8 of `cmd/kb` also passed, which on their own would have
  proved little for a 1-in-40 flake. *Original report:* **Flaky test:
  `TestImportJSONL_SameNameDifferentUUIDMergesUnderLocalProject`
  (`jsonl_test.go`) fails about 1 run in 40.** Found 2026-09-24 in a full
  `go test ./...` that had passed minutes earlier with no change to the root
  package (`-count=40` reproduced one failure: `kbA local project description =
  "kbB's version", want "kbA's version"`). Cause, confirmed: the test creates
  kbA's `shared` project and then kbB's, and `updated_at` is
  `CURRENT_TIMESTAMP` at one-second resolution. Usually the two tie and the
  local row wins, which is what the test asserts. When the clock ticks between
  the two `AddProject` calls kbB is genuinely newer and DR-0025's
  last-writer-wins correctly lets it win. Forcing a 1.1 s gap made the flip
  happen every time. So the product is right and the test's premise is wrong
  one run in forty. Likely fix: set both rows' `updated_at` explicitly (kbA
  newer or equal) instead of relying on wall-clock ties, and add a sibling
  test for the newer-incoming-wins case that is currently only accidental. Check the
  other `jsonl_test.go` and `knowledge_merge_test.go` cases that build two
  databases back to back for the same assumption.

- [x] **FIXED 2026-09-24 (unreleased, for v0.0.13).** *Fix as landed:* cause
  confirmed: `checkpointAndCopy` opens each input with `sql.Open` and runs a
  pragma, and SQLite creates a file that is not there. `checkMergeInputs`
  (`cmd/kb/merge.go`) now runs before anything opens them and refuses: a
  missing input (exit 1, naming the flag and path), a directory or other
  non-file (it used to fail with SQLite's misleading `unable to open database
  file: out of memory (14)`), a zero-byte file (exactly what the old behaviour
  left behind), and `-a` and `-b` naming the same file by any spelling or
  symlink (exit 2; the "merge" was a copy, and a repeated path is almost
  certainly a typo, compared with `os.SameFile`). A refused merge changes
  nothing on disk, and a test asserts the directory listing is identical
  before and after. **One allowance I could not verify:** a zero-byte main
  file with a non-empty `-wal` is not refused, on the theory that a WAL
  database may hold data only in its sidecar. Probing showed a WAL database's
  main file is at least one page from the moment WAL mode is set, so I could
  not produce that state; it is kept as a defensive allowance (refusing a file
  that might hold real data is the worse mistake) and the comments say so.
  Checked live, old vs new binary: the six bad-input cases (five exited 0, one
  gave the misleading message) now fail cleanly and leave no files; a real merge
  of the actual database with an older backup gives an identical 14-table
  summary and identical merged content (518 rows compared). (My first content
  comparison hashed the empty string for every table because of a bad query;
  caught by the constant hash and redone.) Also corrected `kb-merge(1)`, which
  still said merge "ignores --db" (it is refused since the `--db` fix).
  `mergeinputs_test.go`, red first. *Original report:* **`kb merge` succeeds on
  input paths that do not exist, and leaves empty files behind.** Found 2026-09-24 while checking surplus arguments:
  `kb merge -a x -b y -out z` with neither `x` nor `y` present exits 0, prints
  the per-table summary, creates zero-byte `x` and `y` (SQLite opens a missing
  path and creates it) and writes a schema-only `z`. A mistyped `-a` therefore
  merges an empty database into the real one and reports success. Likely fix:
  stat both inputs first and refuse a missing one (exit 1, naming the path),
  with a test that no file is created. Not fixed with the ignored-input work;
  it is a missing-file check, not an argument-shape one.

- [ ] **Exit-codes X5 and X6 remain** (see `exit-codes-plan.md`). X0 to X4 are done. Known
  gaps, so nobody assumes otherwise: no old-versus-new comparison on the real database yet (X5, the
  DR-0040 method); the per-verb man pages, the upgrade note and the three skill scripts
  that read `kb`'s exit status still describe the old codes (X6). The main EXIT STATUS
  section, `ingest` and `source check-retractions` pages are current. **No release before
  X6.**

- [ ] **Two traps that are not exit-code questions** (noted in DR-0048). `kb project add
  NAME description --status paused` stores "description --status paused" as the
  description, because DR-0041 treats words after the fixed arguments as free text;
  the flag has to come before the name. And `kb project add` on an existing name says
  "added" though nothing was added. Both want a decision on whether a flag-looking
  trailing word should be refused, and on the wording.

- [ ] **DECIDED and ACCEPTED 2026-09-25, in progress: split the exit codes** (workspace
  DR-0003 for the convention, knowledge DR-0047 for `kb`, both `accepted`, DR-0040
  superseded, root `CLAUDE.md` updated; plan in
  `exit-codes-plan.md`, X0-X6). RSDOIEL chose to split now and follow POSIX where
  sensible: 0, 1 (a normal negative answer: nothing found, stale, state forbids), 2
  (usage, unchanged), then sysexits numbers (65 data, 66 no_input, 69 unavailable,
  70 internal, 73 cant_create, 74 io, 75 temp_fail, 77 no_permission). Bulk commands
  exit with the failure's class after processing everything (`kb ingest` used to exit
  0 with failures). The root `CLAUDE.md` must be updated once DR-0003 is accepted,
  since RSDOIEL wants this convention across projects. The original item follows,
  kept as the record of the questions it raised.

- [x] **(superseded by the item above) Revisit exit codes greater than 1.** Filed 2026-09-24 at RSDOIEL's
  request, as something useful to explore later, not a bug. `kb` exits 0, 1 or
  2 and nothing else. DR-0040 (accepted) settled the split, 2 for a mistake in
  the command line and 1 for everything else, and left these open, so this
  starts from them rather than reopening it:

  - **Should 1 be split further?** Today "not found", "no results", a value the
    library rejects and a database or file error all exit 1. A script that needs
    to tell "the record is not there" from "the database is unreadable" has to
    parse stderr. Candidates are `sysexits.h` (64 usage, 65 data, 66 no input,
    74 I/O), which DR-0040 set aside because the published contract is 0, 1, 2;
    or a smaller step, such as one code for "found nothing" and another for "could
    not run". The question to answer first is which distinctions a real script
    branches on, not which are available.
  - **The workspace convention disagrees.** The root `CLAUDE.md` (written for the
    Deno tools) says 1 when a search-style tool finds nothing and 2 on a usage or
    I/O error. `kb` keeps file and database errors at 1, as `kb -help` has said
    since v0.0.1. Decide whether the two should agree, and which moves. Moving
    `kb`'s I/O errors to 2 would collide with its own usage code.
  - **A value the library rejects** (an unknown observation kind or project
    status) exits 1 because the check lives in the library. Making it 2 wants a
    typed invalid-value error from the library, and a decision on which library
    errors count. `merge -a x -b x` is 2 and a missing `-a` is 1; worth checking
    that the whole set still reads as consistent once this is on the table.
  - **Nothing enforces the classification.** A new verb that returns a plain
    `fmt.Errorf` for a bad argument exits 1 again, silently. A test that runs
    every registered verb with a bogus flag and a surplus argument and asserts 2
    would make DR-0040 and DR-0041 permanent, and is worth having whatever is
    decided about the codes themselves.

  Any change here is a breaking change for scripts: it needs an upgrade note, and
  because DR-0040 is accepted it wants a new record that supersedes or amends it,
  not an edit.

- [x] **FIXED 2026-09-25 (DR-0045, `proposed`; unreleased).** **No validation on
  free-text-looking fields.** `source add T --published notadate` and `--url
  'not a url'` and an empty `source add ''` title all succeeded; `record new
  --trigger bogus` succeeded silently. Worked through with RSDOIEL, every
  recommendation taken. `AddSource` (library) now trims, refuses a blank title,
  requires `--published` to be YYYY, YYYY-MM or YYYY-MM-DD and a real date,
  requires a `url` identifier to be absolute with a scheme and host, and a `doi`
  to be the bare `10.NNNN/suffix` form (a pasted `https://doi.org/...` is refused,
  not rewritten). The DOI check was added beyond the filed item: a mistyped DOI
  goes to Retraction Watch and a miss reads as "not retracted". `record new`
  refuses a `--trigger` or `--kind` outside the vocabulary unless a record
  already carries it, the rule `record list` uses (one shared function). Import
  is not held to the checks. Verified by running old and new binaries on a copy
  of the real database: only the refusals differ, the seven real sources are
  untouched. Left open on purpose, listed in DR-0045: an ignored `--url` beside
  `--doi` is unchecked, a document's frontmatter `published_date` and
  `observation add --source-doi` are still free text, `record new --project P`
  does not require P to exist. Breaking for scripts: needs an upgrade note.

- [x] **FIXED 2026-09-24 (unreleased, for v0.0.13).** **`kb search` fails on any term containing a hyphen** (`map-reduce`, `records-portability`, `cross-machine`, `JSON-L`, `spot-check`), with `SQL logic error: no such column: reduce (1)`. Found 2026-09-24 in v0.0.12 (and v0.0.11) while live-testing harvey's learning mode; confirmed against the real database with the installed `kb`. Cause (unverified in code, consistent with every failure): FTS5 reads `a-b` as `a` minus column `b`. v0.0.11's punctuation fix (`c702c9d`) only retries with the term quoted when the first attempt fails with an *FTS5 syntax error*; a hyphen raises a different error, `no such column`, so it is never retried. Dots (`v0.0.11`) work and hyphens do not, which is why the v0.0.11 fix looked complete. Hyphenated names are common here (concepts such as `map-reduce`, `harness-engineering`, `scholarly-provenance`), and harvey's `/kb search` goes through the same `Search`. Likely fix: also retry on `no such column` (or quote the term up front whenever it contains anything but letters, digits and the documented operators), with a test written red first for each hyphenated case above. **Fix as landed:** `Search` now retries once as a quoted phrase when the raw term fails for *any* reason (hyphen and colon give `no such column`, an unbalanced quote gives `unterminated string`, the rest `fts5: syntax error`), not only on a syntax error; a trailing `*` stays outside the quotes so prefix search still works. Six tests in `knowledge_test.go` (`TestSearch_*`), five red first; the last pins the documented syntax (AND, phrase, order, prefix, NOT, OR) unchanged. Checked with a built `kb` against a copy of the real database: 23 terms, 11 identical to v0.0.12 (all documented syntax, dots, `C++`, apostrophes), 12 that errored now return results (`map-reduce` 4, `records-portability` 9, `JSON-L` 17, `cross-machine` 30, ...) or a clean "no results", none that worked changed. Not yet in a release; the v0.0.13 CHANGES entry is RSDOIEL's step.

- [x] **FIXED 2026-09-23 (DR-0037).** **`document frontmatter` prints `known-concept proposals` in a different order every run.** Found 2026-09-23 by comparing two builds: the same binary, same file, six runs gave six different orderings (same items each time). Cause: `EligibleTagConcepts` (`documenttag.go`, moved from `cmd/kb` in L2) builds its result by ranging over a map, so the order is unspecified, and `knownKeywordProposals` passes it straight to the report. Same class as the `excludeNearExisting` tie-break bug fixed before v0.0.11. Likely fix: `sort.Strings` the result, with a test written red first. Not fixed inside L3 because L1-L3 are pure moves.

- [x] **FIXED 2026-09-23 (DR-0037).** **`document frontmatter --accept` writes new frontmatter keys in a different order every run.** Found 2026-09-23 during L4: the same command (`--accept title,author,dateCreated,dateModified` on a file with no frontmatter) run 8 times gave two different key orders, 4 runs each. Cause: `frontmatterRun` (moved from `cmd/kb` in L4) applies accepted fields by ranging over a map, and `setMappingField` appends each new key as it goes, so key order in the file follows map iteration order. Same for several `--set` flags. Likely fix: apply in the canonical order title, author, dateCreated, dateModified (and sort `--set` keys the same way), with a test written red first. Moved verbatim in L4, not fixed there, so the old-vs-new comparison stayed exact (it compared one field at a time and normalized key order for the multi-field case). Related to the entry above and to the `excludeNearExisting` tie-break bug fixed before v0.0.11.

- [x] **FIXED 2026-09-25 (DR-0046, `proposed`; unreleased). DR-0041, DR-0042 and
  DR-0044's open questions**, each probed before choosing, every recommendation
  taken. (1) `--` now ends flag recognition in `splitFlags`, so `record`,
  `document`, `ingest` and `source add` accept a dash-leading title or path.
  (2) `CleanName` makes a name one line: interior whitespace runs (newline, tab)
  collapse to a space and any other control character is refused; ingest cleans
  wikilinks and tags first, so a hard-wrapped `[[a<newline>b]]` is the ordinary
  concept `a b` (the previous binary minted two newline concepts from 212 real
  documents). (3) `merge` refuses every zero-byte input: the WAL allowance was
  tested and protects nothing (SQLite discards the WAL of a zero-byte main file),
  and it made merge exit 0 on an empty input and delete the input's `-wal`. Old
  and new binaries agree on real data: 45 records, 75 concepts, 157 links.
  Left open, listed in DR-0046: `kb search -- -x` does not consume `--`; a
  `source add` title still may hold a newline; `document tag` does not recognise a
  wrapped existing link. Breaking for scripts: needs an upgrade note.

- [ ] **Programmatic corpus-improvement techniques, as the corpus grows.**
  Raised 2026-09-18 while discussing the MADR item below: decision records
  already benefit from being surfaced through multiple routes (frontmatter,
  `[[wikilinks]]`, `relates_to`), and documents are new and still being
  tuned (see DR-0026/DR-0027's density-linking work). Worth a standing
  question, not a single ticket: what other mechanical, no-dependency
  techniques could improve corpus linkage as it scales past what a human
  curating by hand keeps up with?

  Two concrete candidates raised so far:

  - **Corpus-wide term-frequency/distinctiveness scoring** (TF-IDF or
    similar) to surface *candidate* concepts — terms frequent in one
    document but rare elsewhere — for a human to review via
    `kb concept add`, rather than auto-creating them. This is the
    keyword/topic-discovery half of what density-linking (DR-0026/DR-0027)
    cannot do: `MatchConceptNameCounts` only matches text against concepts
    that already exist, it can't propose new ones. **Shipped 2026-09-18,
    see DR-0028**: `kb concept suggest [--project NAME] [--limit N]`, a
    new read-only verb, scores every record body and document section body
    this way and prints a ranked candidate list; never writes to the
    database. Live-tested against the real `agents/knowledge.db`: a real,
    roughly-50%-signal mechanical pass (`rename`/`harvey`/`divergence`/
    `collision`/`uuid` genuinely useful, alongside generic nouns like
    `row`/`name`/`table`/`test`), the same character as DR-0027's own
    density-linking prototype — a human still curates the output, this
    does not close the loop on its own.
  - **Levenshtein (or similar fuzzy) matching of concept names against
    record/document prose**, to surface records or documents that are
    topically relevant but unlinked because the wording differs from the
    concept's exact name (`MatchConceptNames`/`MatchConceptNameCounts` are
    both exact whole-word matches, so a near-miss — a typo, a plural, a
    close paraphrase — currently links nothing).

  A follow-on question surfaced once `concept suggest` had a real output:
  once a candidate list is confirmed real (`kb concept add`), there was
  nowhere for that judgment to land in the corpus itself — a concept gets
  linked implicitly on the next ingest (density-linking), but invisibly,
  with no durable record in the source file of *why* it applies. **Shipped
  2026-09-18, see DR-0029**: `kb document tag --project NAME [--concept
  NAME,...] [--dry-run]`, a new pure-file-operation verb, inserts an
  explicit `[[Name]]` wikilink at the first safe occurrence of each
  eligible concept per document — eligibility reuses DR-0027's
  density-linking threshold verbatim with no `--concept` given, or bypasses
  it for explicitly-named concepts. Never writes to the database; both-or-
  neither across a project's whole document set; idempotent on re-run.
  Live-smoke-tested against a scratch copy of a real document, which
  surfaced and got a real fix: a concept mention inside the document's own
  H1 title was getting wikilinked, mechanically correct but stylistically
  wrong, now excluded alongside frontmatter/code spans/existing wikilinks.
  A frontmatter-`keywords:`-only alternative was considered and set aside
  (DR-0029's Rejected alternatives) — worth revisiting only if a corpus
  specifically wants a no-prose-edit option.

  Both fit the pattern already established for document summaries and
  decision records: a mechanical pass surfaces candidates, a human curates
  rather than the pass auto-committing. The alternative — embeddings or an
  LLM call for either keyword extraction or fuzzy relevance — would likely
  do better, but pulls in exactly the kind of dependency (a vector store,
  network calls, nondeterminism) this module has deliberately avoided so
  far; worth weighing explicitly if either of the above turns out too weak
  in practice, not assumed as the starting point.

- [x] **FIXED 2026-09-23 (DR-0037).** **`MatchConceptNames`/`MatchConceptNameCounts` can never match a
  concept name whose first or last character is non-word punctuation**
  (`\b` fires only between a word character and a non-word character, so a
  concept like `C++`, `F#`, or `.NET` mentioned as `"...using C++ for
  this..."` never matches — the space right after the trailing `+` isn't a
  word/non-word transition at all). Found reviewing `FuzzyMatchConceptNames`
  (v0.0.11 item 1) before release, since it reuses `MatchConceptNames` for
  its exact-match skip check, but the limitation is in the original
  function (2026-09-13), not new. Affects `RecallByConceptNames`, `tag`,
  `fuzzy-tag`'s exact-match exclusion, and density-linking, all silently —
  no error, just a mention that never resolves. Not fixed here: it's a
  regex-anchoring change to a widely-used, foundational function, deserving
  its own dedicated design/test pass rather than a quick patch alongside
  unrelated work.

- [x] **FIXED 2026-09-23 (DR-0037): two more bugs found while fixing those.** (1) A `[[...]]` inside a code span or fenced block minted a concept in both document and record ingest, which is where junk concepts like `...`, `recall: ...` and `tool: ...` came from; ingest now scans the code-stripped text. (2) The determinism audit found two more map-order sites, `RemovedHeadings` after a re-ingest and the notes `project rename` prints; both are ordered now. A repeat-run check (60 read-only invocations x 5, and the file-taking verbs x 12 runs, against a pre-fix binary that gave 12 different outputs in 12 runs) now finds none. **Follow-up (DR-0039): `kb concept delete` now exists.** The real `agents/knowledge.db` still holds the junk concept `...` (id 162, linked to two records, not one as first written). It is inert; removing it is `kb concept delete ... --force`, awaiting a go-ahead because it is real data.

- [x] **DONE 2026-09-23 (DR-0039): `kb concept delete`.** `ConceptUsage`/`DeleteConcept`/`ConceptInUseError` in the library; the verb refuses a linked concept unless `--force`, previews with `--dry-run`, accepts a flag-shaped name after `--`, matches the name exactly, and says in its output and man page that a merge/import from a database that still has the concept brings it back (no tombstone, chosen 2026-09-23) and that a file still naming it recreates it when next ingested after a change.

## Planned for v0.0.11

Scoped 2026-09-19, item 5 added 2026-09-22, item 6 (bug) added and fixed
2026-09-23, items 1, 2, and 5 implemented 2026-09-23. **All four items
done.** The 2026-09-19 scoping also carried forward two items — `kb
project rename`'s corpus-rewrite fix and cross-machine rename
reconciliation in `merge`/`import` — that DR-0026 (2026-09-18, see Done
below) had already shipped the day before; removed here 2026-09-22 as
stale duplicates once that was noticed.

- [x] **New verb `kb document fuzzy-tag`**, promoted from the
  corpus-improvement-techniques item above and refined 2026-09-21: see
  `fuzzy-concept-matching-design.md`. `MatchConceptNames`/
  `MatchConceptNameCounts` are exact whole-word matches today, so a typo,
  plural, or tense variant of a known concept's name currently links
  nothing (paraphrase is explicitly out of scope — a semantic problem, not
  a spelling one). Mirrors `kb document tag`'s own shape (`--project`,
  `--concept`, `--dry-run`) with fuzzy matching substituted for exact, but
  never bracket-wraps the near-miss text itself — doing so would let
  `ResolveConceptName` mint a duplicate concept from the misspelled/
  inflected form. Instead inserts a footnote marker at the near-miss and a
  footnote definition carrying the canonical `[[Concept]]`, so prose stays
  unedited and linking still resolves to the real, existing concept.

  **Implemented 2026-09-23** per `fuzzy-concept-matching-plan.md`'s F1–F4.
  Found and corrected a real self-contradiction in the design's decision 5
  while implementing F1: stemming *both* the concept name and the
  candidate token (as originally written) breaks the design's own worked
  examples — verified by hand, then fixed to raw-distance-first with a
  stemmed-token-only fallback (see `fuzzy-concept-matching-design.md`'s
  amended decision 5, and DR-0032 for the full writeup). `retrieval.go`
  gained `FuzzyMatchConceptNames`/`levenshteinDistance`/
  `stripCommonSuffix`; new `cmd/kb/documentfuzzytag.go` holds the footnote
  mechanics and `cmdDocumentFuzzyTag`, wired into `document`'s subverb
  switch and documented in `DocumentHelpText`/`kb-document.1.md`. All of
  F1–F4's listed tests pass, `go vet`/`go build` clean, and live-smoke-
  tested against a real scratch document (footnote placed correctly,
  idempotent on rerun, concept actually linked after re-ingest). Decision
  record DR-0032 authored `proposed`, not yet promoted.

  **Pre-release bug fix (2026-09-23):** an automated pre-release review
  found that two fuzzy near-misses in the same section (no heading
  between them) corrupted the file — a flat, single running `delta`
  applied to every match after the first overcorrected any match still in
  the same section as an earlier one, splicing a later marker at the
  wrong byte offset (confirmed: it landed *inside* the first footnote's
  own definition text). Fixed by tracking each insertion's two splices
  (marker, definition) as separate positional breakpoints rather than one
  flat delta. Regression test added (confirmed red against the unfixed
  code first), live-verified again with a real two-near-miss file.

- [x] **A standalone frontmatter-generator command, for documents.**
  Explicitly *not* an alternative to `kb document tag` and not folded into
  it — tagging links known concepts into existing prose; this is a
  separate, more general documentation-maintenance tool for
  producing/maintaining a document's frontmatter block on its own terms.
  Filed as `frontmatter-generator-feature-request.md` (2026-09-19):
  propose-then-accept for `title`/`author`/`dateCreated`/`keywords`, plus a
  new `dateModified` field, and provenance shelling out to `git` (this
  module's first `exec.Command` dependency), falling back to filesystem
  mtime.

  **Implemented 2026-09-23** per `frontmatter-generator-plan.md`'s FM1–FM6.
  Verb: `kb document frontmatter PATH [--accept FIELD,...] [--accept-
  keywords NAME,...] [--set FIELD=VALUE] [--dry-run]`. New
  `cmd/kb/documentfrontmatter.go`: git/filesystem provenance primitives,
  a layered author signal (byline → git first-commit author → `git config
  user.name`, all shown when they disagree), `scoreDocumentCandidateTerms`
  (a sibling of `scoreCandidateTerms`, `concept.go`, fixing the design's
  own flagged idf-degeneracy bug by excluding the target document from its
  comparison scope), and a `yaml.Node` surgical frontmatter writer that
  never round-trips through a struct (would silently drop an unrecognized
  frontmatter key). `documentFrontmatter` (documents.go) was **not**
  modified as the plan specified — it's unexported, so `cmd/kb` cannot
  reference it across the package boundary either way; current field
  values are read directly from the parsed `yaml.Node` instead, and
  nothing downstream needs the new field surfaced yet. `codemeta.json`
  gained `git` as a runtime software requirement; `README.md` regenerated.
  All of FM1–FM6's listed tests pass, plus one regression test added after
  a bug found live-smoke-testing: the plain-text report silently hid
  `dateModified`'s always-fresh proposal once a stale value already
  existed (the "already set" branch matched first) — fixed to show both.
  `go vet`/`go build` clean; live-verified end-to-end against a real
  two-document project (git-tracked, real commits): title/author/
  dateCreated/dateModified proposed and accepted correctly, a known-
  concept keyword and a brand-new candidate concept both accepted and
  written, re-ingested, and both concepts confirmed actually linked via
  `kb document show`. Decision record DR-0033 authored `proposed`, not
  yet promoted.

  **Pre-release bug fix (2026-09-23):** an automated pre-release review
  found that a frontmatter key present with an explicit empty value (e.g.
  a template's `title: ""`) made the absent-only check treat it as
  already set forever, with no way to fill it in — compounded by
  `--set FIELD=` (an explicit empty value, documented as bypassing the
  absent-only rule) silently no-oping instead of writing, since the
  write-time guard couldn't distinguish "nothing proposed" from "a
  deliberate empty assertion." Fixed: an explicit empty value is now
  treated the same as absent for proposal purposes, and `--set` writes
  unconditionally regardless of value. Regression tests added (confirmed
  red first), live-verified.

- [x] **Fuzzy-aware `kb concept suggest`**, folded in 2026-09-22 from the
  "To explore" list: see `fuzzy-concept-clustering-design.md`. A distinct
  mechanism from item 1's `fuzzy-tag` — that one matches document text
  against *already-known* concepts; this one clusters `concept suggest`'s
  own *candidate* terms *against each other*, before any of them are
  concepts, so a term split across spelling variants (`chunking`/
  `chunkings`/`chunked`) scores as one merged candidate instead of several
  individually-sub-threshold ones.

  **Implemented 2026-09-23** per `fuzzy-concept-clustering-plan.md`'s
  FC1–FC5 (FC1 was already satisfied by item 1's
  `LevenshteinDistance`/`StripCommonSuffix` in `retrieval.go` — exported
  from their original unexported form so `cmd/kb` could call them at all,
  a cross-package gap the plan hadn't accounted for). Found and corrected
  the same class of self-contradiction item 1 hit: decisions 3 and 4's
  "stem both sides via `stripCommonSuffix`" breaks the design's own
  chunking/chunkings example (distance 3, not 1); fixed with a shared
  two-tier `fuzzyTermsClose` (raw distance first, both-sides-stemmed
  fallback second — symmetric here, unlike fuzzy-tag's asymmetric
  fallback, since both sides are equally unverified candidates). New
  `cmd/kb/conceptcluster.go`: `excludeNearExisting` (FC3),
  `clusterCandidateTerms` (FC4, star clustering — verified against the
  classic cat/cot/cog/dog chain-drift case). `scoreCandidateTerms`
  (`concept.go`) now returns `(candidates, nearExisting)`; `candidateTerm`
  gained an optional `variants` field; `cmdConceptSuggest` renders
  `chunking (+chunkings, chunked)` inline and a trailing near-existing
  section, `--json` gains a parallel `near_existing` array (an
  intentional, plan-specified breaking change to the JSON shape — was a
  bare array, now `{"candidates": [...], "near_existing": [...]}`).

  Live-smoke-tested against the real `agents/knowledge.db` and found a
  real, second problem beyond the design contradiction: a flat
  distance-≤1 threshold with no length floor produced heavy false-positive
  clustering on short, common terms (`table (+tables, stable, able)`,
  `old (+holds, holding, hold, ..., told)`, `makes (+make, ..., man, map,
  ...)`) — a different failure mode than the chain-drift risk decision 4
  already guards against. Fixed with a `minFuzzyTermLength` gate (6
  characters, provisional): re-running the same corpus afterward, all of
  the above false positives are gone and genuine merges (`rename`,
  `import`, `reference`) are unaffected, aside from one small residual
  case (`reference`/`preference`, a coincidental real-word collision)
  accepted as within this feature's already-declared "deliberately crude"
  bar. `fuzzy-concept-clustering-design.md` amended in place with both
  corrections. All FC1–FC5 tests pass, `go vet`/`go build` clean. Decision
  record DR-0034 authored `proposed`, not yet promoted.

  **Pre-release bug fix (2026-09-23):** an automated pre-release review
  found `excludeNearExisting` picked the nearest known-concept match by
  iterating an unordered Go map, keeping only a *strictly* smaller
  distance — an exact-distance tie between two known concepts was
  resolved by map iteration order, nondeterministically across runs,
  despite the surrounding code sorting its own output "for deterministic
  output." Fixed with an explicit alphabetical tie-break. Regression test
  added (confirmed red against the unfixed code, which failed on the
  first run), asserting a stable result across repeated calls.

- [x] **Bug: `kb search TERM` threw a raw SQLite error instead of a normal
  "no results" for any term containing bare punctuation FTS5's query
  grammar treats as significant** (a `.` was the case found; likely also
  `-`, `:`, `*`, parens). Root cause confirmed live 2026-09-23:
  `Search` (`knowledge.go`, `WHERE kb_fts MATCH ?`) bound the user's raw
  term directly as the MATCH string. FTS5 parses that string with its own
  query grammar (`AND`/`OR`/`NOT`/`NEAR`/column-filters/phrases) *before*
  tokenizing, so an unquoted `.` — not a token character for `unicode61`
  and not a grammar operator either — had nowhere to go:
  `sqlite3 ... MATCH 'v0.0.11'` → `Error: fts5: syntax error near "."`,
  while `MATCH '"v0.0.11"'` (quoted phrase, grammar doesn't look inside
  quotes) returns 9 rows.

  **Fixed 2026-09-23.** `kb-search(1)`/the CLI helptext document raw FTS5
  query syntax as a supported feature (multi-word AND, quoted phrases,
  `prefix*`), so the fix could not blanket-quote every term — that would
  have silently turned `kb search foo bar` from an AND of two terms into
  an adjacency phrase. Instead `Search` now retries once, only when the
  first attempt's error is specifically an FTS5 syntax error
  (`isFTS5SyntaxError`, matched on `"fts5: syntax error"` in the driver's
  error text), rewriting the term as a quoted, `"`-escaped phrase before
  the retry. The `glebarez/go-sqlite` driver surfaces a MATCH parse error
  while stepping rows, not from `Query` itself, so the query logic moved
  into a `runSearch` helper whose error is observed via `rows.Err()`
  before `Search` decides whether to retry. TDD: `knowledge_test.go`
  gained `TestSearch_TermWithBarePunctuationDoesNotError` (confirmed red
  against the unfixed code) and `TestSearch_MultiWordTermStillANDs` as a
  regression guard for the documented AND syntax. `go build`/`go vet`/
  `go test ./...` clean; live-verified with `kb search "v0.0.11"` against
  the real `agents/knowledge.db` (exit 0, one result, no driver error).
  Not yet committed or given a decision record.

## Done

- [x] **Two decision-record dialects now exist in one organisation, and `kb`
  can only read one of them.** Raised 2026-09-15 after pulling
  `caltechlibrary/CL-Web-Components`, where a colleague had been recording
  architecture decisions independently; by 2026-09-18 it was two repositories
  and twelve records, `workflows/docs/decisions` being the second. Filed as a
  design question rather than a bug: the refusal `kb ingest` gave
  (`no frontmatter: file does not start with ---`, one per file) was always
  the right refusal, but it left no partial path.

  **Closed 2026-09-18 by DR-0027**, which answered what this item itself
  called "the real question to answer first": a colleague's ADR is a
  **document**, not a record. We do not own its status and can never promote
  or supersede it, and a document is exactly the shape for source material we
  read and summarise. `kb document ingest` reads MADR without complaint — no
  frontmatter required, segmenting on its `## Context and Problem Statement` /
  `## Decision` / `## Consequences` headings, 5-14 sections each.

  **Answering it that way dissolves most of what this item was worried
  about.** The identity collision — both dialects numbering from 0001, both
  claiming `(WorkLab, CL-Web-Components, project, 0001)` — cannot arise,
  because a document never enters the records identity tuple at all. So the
  `dialect` column, the third `external` scope value, and the `--format madr`
  adapter are all moot rather than deferred: each existed only to make a
  foreign ADR safe *as a record*.

  **Two gaps in the stopgap were real and were fixed in the same pass** (also
  DR-0027): with no frontmatter `title:`, a document used to be titled with
  its *filename* — `ParseDocumentFile` now falls back to the first true H1,
  general rather than MADR-specific; and no concepts used to be linked at all,
  since linking came only from `[[wikilinks]]` and frontmatter `keywords`,
  neither of which MADR has. Document ingest now also auto-links a known
  concept mentioned more than once outside code spans
  (`MatchConceptNameCounts`).

  **The cost accepted, stated plainly: a document cannot be cited from
  `relates_to`.** That is the one thing the record reading would have bought.
  There is no `document_relations` table and no record-to-document edge, so a
  decision record of ours cannot formally point at the colleague's ADR it is
  responding to — the link has to live in prose. This is a known, accepted
  limitation of DR-0027, not an oversight; it is worth reopening only if
  citing a foreign ADR by id turns out to be something we actually reach for
  repeatedly, and reaching for it twice is a better trigger than predicting it
  now.

  **Evidence, 2026-09-15: density-based linking was run by hand against those
  four ADRs before any of it shipped, and it was useful but lossy — roughly
  60% signal.** This is what set DR-0027's threshold, so it is kept here
  rather than summarised away. The script mirrored kb's own semantics rather
  than inventing new ones: the `(?i)\b<name>\b` whole-word match from
  `MatchConceptNames`, computed over `wholeDocumentText`. Confirmation the
  mirror was faithful: the match counts came out 3, 2, 4, 4 — identical to the
  `tag_density` already stored on each gist.

  It produced 13 links from 95 known concepts, and the effect was real: a
  query naming `cmtools`, `documentation` and `tooling` returned ADR-0004
  first, above a workspace decision record, where before it returned nothing
  from either retrieval pass — FTS5 ANDs its tokens, so a conversational
  question misses a document with no concept links at all.

  Eight of the thirteen were topical. **Five were coincidental, and the way
  they failed is the useful part**, because all five are the same failure — a
  concept name colliding with a word used in a different sense:

  - `format` matched `unsupported format` in a quoted *error message*, and
    separately matched the variable in `const format = outputName`.
  - `index` matched "a committed search index", a different sense from the
    `index` concept (a generated `decisions/index.md`).
  - `git` matched an incidental `git push` in a sentence about publishing.
  - `validation` matched a phrase describing a *third* project's domain, not
    the document's own subject.

  So the lesson was narrower than "density is too noisy". Whole-word matching
  is not the problem; **short, common, single-word concept names are**, and
  prose quoting code and error strings makes them worse. Of the three
  refinements costed at the time, DR-0027 shipped the first two — require more
  than one occurrence, and exclude matches inside code spans and fenced blocks
  — which between them kill `const format = outputName` and probably
  `git push`. The third, writing density matches as *suggestions* a human
  confirms, was not built; `kb document tag` (DR-0029) covers the same ground
  from the other end, by making a confirmed link explicit in the file.

  **The five coincidental links were pruned by hand the same day.** Two things
  the prune showed. It cost no coverage: all four ADRs stayed reachable from
  concept-tag recall, because each retained at least one topical link, so the
  noisy matches were redundant rather than load-bearing. And `tag_density` is
  deliberately *not* updated to match — it counts mentions, not links — so
  gists 19 and 30 read density 4 against 2 links each. DR-0027 preserved that
  divergence on purpose: density is the raw signal, links are what survived
  review, and the gap between them is exactly the quantity a suggestion
  workflow would surface.

- [x] **`kb project rename`'s documented escape hatch didn't work, so the
  refusal was absolute for any project that owns records.** Found 2026-09-17
  from WorkLab, trying to carry out that workspace's `codemeta` →
  `codemetatools` rename (WorkLab `codemeta` DR-0006, accepted). The manual
  path the refusal recommended — rewrite the corpus's `project:` frontmatter,
  re-ingest — was itself blocked: a record whose `project:` no longer
  matched its database row made `upsertAll` treat the file as *new* under
  `(workspace, project_id, scope, record_id)` identity, but the file still
  carried its old, stable `uuid`, and `records.uuid` is `UNIQUE` — the
  `INSERT` collided. A first attempt was worse than a no-op: the failed
  ingest still minted an empty project under the new name, which then
  tripped rename's *other* guard (`NEW` already exists), leaving the
  database stuck with no verb to remove the stray row.

  **The fix turned out simpler than either shape this item originally
  sketched** (routing through `ingest` with `uuid`-aware reparenting, or a
  general rename-event log): `records.project_id` never needs to change on
  a rename, since it is a stable foreign key and only `projects.name`
  changes. So the corpus rewrite never has to go through `ingest`'s
  identity resolution at all — it edits every owned record's file in place,
  touches no database row, and leaves the very next ordinary `kb ingest` to
  see a changed checksum against an unchanged identity, the ordinary UPDATE
  path, not the INSERT path that collided.

  Tracing the merge/import code while designing this turned up two further,
  live-verified bugs this item hadn't anticipated: `kb merge` could
  silently keep a stale name or drop a side outright depending on merge
  order (`INSERT OR IGNORE` couldn't distinguish a `uuid` collision from a
  name collision), and `kb import` hard-failed the *entire* import, not one
  row, on the same collision. Both are fixed in the same pass — shipping
  the rename completion without it would have made the corruption risk
  worse, not smaller, by making the trigger routine.

  See DR-0026 (`knowledge/decisions/`), which supersedes DR-0024's outright
  refusal (partial) and DR-0025's "a rename crossing machines does not
  reconcile" follow-on (partial). Shipped: `kb project rename OLD NEW`, when
  `OLD` owns records, rewrites every owned record's `project:` frontmatter
  both-or-neither before renaming the project row, via a new
  `RenameProjectRow` library method that skips `RenameProject`'s
  records-owning guard; `--dry-run` and `--root` are new flags. `kb merge`'s
  and `kb import`'s `projects`/`concepts` conflict resolution moved from
  name-keyed to `uuid`-primary, reconciling `name` by `updated_at` the same
  way `description`/`status` already did (DR-0025), with a name-keyed
  fallback retained only for the rare case of two independently created,
  never-synced entities that happen to share a name (DR-0003, unchanged).
  `reportMissing`'s advice no longer names the nonexistent `kb record
  remove` verb, found stale while reproducing this.

- [x] `kb record new`'s default write path moved to `agents/projects/<project>/decisions/`
  for project scope (`agents/decisions/` unchanged for `--workspace`), plus a
  `--dir` override. See `agents-projects-layout-feature-request.md` (filed
  2026-08-28), `DR-0021` and `DR-0022`, and
  `record-layout-and-workspace-init-plan.md`. Migrating existing corpora
  under the old shape (`clasm/decisions/`, `cold/decisions/`,
  `agents/decisions/caltechauthors/`) and whether `kb` should index `plans/`
  or `feature_requests/` at all are both explicitly out of scope, not
  overlooked — see `DR-0021`'s Consequences.

- [x] `kb record new` (and `kb observation add`) run from *inside* a project
  directory no longer silently creates a stray nested corpus or an empty
  ambient `agents/`. Fixed at the path-resolution layer, as this item itself
  suggested: `cmd/kb/main.go`'s ambient `--db` resolution now refuses to open
  (and so auto-create) a database that doesn't exist, naming `kb init` and
  `kb import -in FILE` instead — a wrong-cwd invocation fails loudly rather
  than fabricating a workspace where it happens to stand. See `DR-0021` item 4
  and `DR-0022` (the guard applies only to the true ambient default, not an
  explicit `--db PATH`).

- [x] `kb project set-description NAME DESCRIPTION` — projects had no way to
  correct a description after `add`, `set-status` being the only in-place
  mutation. Raised 2026-08-26 from the `dev-process` project, whose
  description named `DESIGN_DECIDE_PLAN.md` after that file was renamed to
  `DESIGN_REVIEW_PLAN_IMPLEMENT.md`. Refreshes the FTS row and touches
  `updated_at`. See DR-0012.

- [x] `kb concept add` no longer wipes a description. The request had listed
  `concept` among the add-only verbs; it was the opposite —
  `AddConceptWithIdentifier` carried `description = excluded.description`
  unconditionally while both identifier columns beside it were guarded, so
  `kb concept add NAME` with no description silently cleared the stored one
  in the row and in the FTS index. The guard now covers `description` too.
  See DR-0012.

- [x] `kb ingest` pruned neither a relation nor a concept link dropped from a
  record's frontmatter/body, so `record_relations` and `record_concepts` only
  ever grew (found 2026-08-27 and 2026-09-15 respectively, authoring `clasm`
  DR-0170/DR-0171 and WorkLab's DR-0010). Fixed 2026-09-15: `resolveAll`
  now calls the new `ClearRecordRelationsFrom(fromID)` before re-adding a
  record's own supersedes/relates_to edges each run, scoped to forward edges
  so the other side's own declarations survive; `linkWikilinkTags` calls the
  new `ClearRecordConcepts(recordID)` the same way before re-linking. The
  same gap existed in `document_section_concepts` via `tagSection`, fixed
  with the new `ClearDocumentSectionConcepts(sectionID)`.

- [x] `[[NNNN]]` (e.g. `[[0007]]`, `[[DR-0007]]`) in a record body minted a
  junk concept instead of citing a record — found 2026-09-15, same session as
  the concept-prune item above. `linkWikilinkTags` (`cmd/kb/ingest.go`) now
  matches such a name against `recordIDLikeWikilink`
  (`^(?i:dr-)?[0-9]{4}$`), skips linking it, and adds a warning pointing at
  `supersedes`/`relates_to` instead.

- [x] `kb ingest` never updated `records.path` on a pure file move (found
  2026-08-28 migrating `clasm`'s 173-record corpus under the
  `agents/projects/<project>/` layout; WorkLab's data separately repaired
  out-of-band on 2026-09-15, see the git history for that record). Fixed
  2026-09-15: `upsertAll`'s checksum-match (skip) branch now calls the new
  `UpdateRecordPath` when the incoming path differs. The
  "DR-%s was stored at %s" warning now fires only when the checksum *also*
  differs — a path change alone is an ordinary move, not a possible slug
  collision. `reportMissing`'s message no longer asserts deletion as the only
  explanation (it used to say "use kb record remove to drop it" for a record
  that had only moved out of the ingested prefix, which would have deleted
  five live WorkLab records); it now names a move as the other possibility
  and asks for a re-ingest of the new location first.

- [x] `kb search` exited 0 when it found nothing, against the workspace's
  search-tool convention (`~/Laboratory/CLAUDE.md`: exit 1 on no match).
  Fixed 2026-09-15: `cmdSearch` returns an error instead of printing "no
  results" and returning nil, so it now exits 1 in both text and `--json`
  mode.

- [x] `kb observation update` didn't exist — reached for in real usage
  2026-09-16, the same amend-vs-supersede question DR-0012 deliberately left
  open for observations. See DR-0023 (`knowledge/decisions/`): an observation
  is corrected by superseding it, never by mutating its body. Shipped
  2026-09-16: `kb observation update ID BODY...` inserts a new observation
  (inheriting `ID`'s project and kind) and links it to `ID` via a new
  `observation_relations` table — `record_relations`' own shape, reused —
  with `AddObservationRelation`/`ObservationRelationsFor` mirroring
  `AddRecordRelation`/`RelationsFor`. The old observation's body is never
  touched, so history retention is free; no `updated_at` column was added,
  since the new observation's own `created_at` is the correction timestamp.
  `kb observation show ID` now resolves and prints `supersedes`/
  `superseded_by`, mirroring `kb record show`. `kb merge` and JSON-L
  export/import carry `observation_relations` from this release, not as a
  follow-on, per the standing rule that a table missing from the merge
  summary is a table whose loss goes unreported.

- [x] `kb project rename` and `kb concept rename` didn't exist. **Correcting
  this item's own earlier diagnosis**: `set-description`'s reindex was
  blamed for the `kb_fts` duplicate, but `refreshProjectFTS` deletes by
  `source_id`, not by name — a live repro confirmed `set-description`
  correctly repairs a stale label after a raw rename, one row, not two. The
  real duplicate came from a different natural mistake, `project add
  NEWNAME` used to "rename" (`ON CONFLICT(name)` doesn't fire for a new
  name, so it inserts a second, fully independent project). The harder
  failure was worse than described, too: re-ingesting a renamed project's
  corpus without rewriting its files' `project:` frontmatter silently mints
  a *phantom* project under the old name and *duplicates* the record under
  it — reproduced live, not merely asserted. See DR-0024
  (`knowledge/decisions/`) for the full repros and the design. Shipped
  2026-09-17: `kb project rename OLD NEW` refuses if `NEW` exists or if the
  project owns any records, otherwise renames and reuses the existing
  `refreshProjectFTS`. `kb concept rename OLD NEW` ships in the same pass
  with no corpus-refuse condition, since every concept link is a foreign
  key to `concepts.id`, never a name matched from a file — via a new
  `refreshConceptFTS` factored out of `AddConceptWithIdentifier`'s own
  inline version. Rewriting a corpus's frontmatter automatically to lift the
  refusal remains open, filed as its own future record in DR-0024's
  Rejected alternatives.

- [x] Cross-machine reconciliation of an edited project or concept
  description. Deferred out of the `set-description` work (DR-0012) as a
  policy inversion. **Two of this item's own claims were stale by the time
  it was picked up**: `updated_at` already travelled through `merge` for
  `projects` — the bug was `INSERT OR IGNORE` never consulting it, insertion
  order alone deciding the winner, not a missing column; and observations
  need none of this at all, since DR-0023 already resolved observation
  correction via supersession, retiring the question this item was still
  asking. See DR-0025 (`knowledge/decisions/`) for both corrections in full,
  and its repros. Shipped 2026-09-17: `concepts` gained `updated_at`
  (lazily migrated, backfilled to `created_at`, mirroring how
  `concepts.created_at` itself was added). `kb merge`'s `projects`/`concepts`
  passes changed from `INSERT OR IGNORE` to `INSERT OR IGNORE` (new rows)
  plus a guarded `UPDATE ... FROM ... WHERE incoming.updated_at >
  existing.updated_at` (conflicts) — two statements rather than one
  `INSERT ... SELECT ... ON CONFLICT DO UPDATE`, because the pure-Go SQLite
  driver this project uses rejects an UPSERT clause after a SELECT-form
  INSERT ("near DO: syntax error"), confirmed live, even though the same
  UPSERT works after a VALUES-form INSERT elsewhere in this codebase.
  `importProject`/`importConcept` gained the matching comparison at the Go
  level, plus a new `UpdatedAt` field on `projectRecord`/`conceptRecord`.
  `RenameProject`, `RenameConcept`, and `AddConceptWithIdentifier`'s
  `ON CONFLICT` branch now touch `updated_at`, closing two gaps DR-0024 and
  DR-0012 left open now that the column is compared for real.
  `kb-project(1)`/`kb-concept(1)`'s CAVEATS sections now describe what
  actually happens: descriptions/status reconcile by timestamp; a *rename*
  crossing machines still does not, since `merge`/`import` dedupe by name —
  filed as its own follow-on in DR-0025, not solved here.

- [x] A mechanism for knowing when `index.md` needs regenerating — the
  single-corpus half (`--check`, and `set-status`/`supersede` auto-refresh)
  shipped 2026-09-15/16; the multi-corpus half shipped 2026-09-17.
  `kb index ROOT --all [--check]` discovers every corpus under `ROOT` and
  refreshes or checks each one, continuing past one corpus's failure so it
  does not hide the rest, then exits non-zero if any needed attention — one
  call for a whole workspace instead of naming each corpus by hand.

  **Found running it live against the real workspace, before it shipped:
  matching on the filename `index.md` alone is unsafe.** The first version
  walked for any file named `index.md`; against the real Laboratory tree it
  also matched several files with nothing to do with `kb` — a llamafile
  docs-site front page, a Jekyll blog index — and reported them as "stale"
  corpora under `--check`. In write mode it would have silently overwritten
  them with an empty decision-records template. Fixed before ever running in
  write mode against real data, by requiring both signals together: the
  file's own content has to open with this format's generated-file heading,
  *and* the directory has to hold at least one record file. Regression tests
  cover both the check and write paths directly, since this is exactly the
  kind of bug a test written after the fact would not have caught with
  confidence.

  Resolves the "does the index belong on disk at all" question left open
  alongside this item, by default rather than by new argument: `--check` and
  `--all` remove the staleness that was the only real cost of keeping it, so
  the existing case for it — `head`, `grep`, `awk` reach a corpus without
  `kb` installed — stands uncontested. Removing the file was never
  implemented or seriously pursued.
