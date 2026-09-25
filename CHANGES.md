# Changes

Reconstructed for v0.0.1 through v0.0.3 from each tag's `codemeta.json`
release notes; maintained going forward.

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
