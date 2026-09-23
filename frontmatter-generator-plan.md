# `kb document frontmatter` — Implementation Plan

See [frontmatter-generator-design.md](frontmatter-generator-design.md) for
the full rationale and confirmed decisions. Work items ordered FM1 → FM6.
TDD-first: each phase's tests are written and confirmed red before its
implementation, per this workspace's standing convention. New file:
`cmd/kb/documentfrontmatter.go`, unless noted otherwise.

---

## FM1 — Git/filesystem provenance primitives (pure, no DB)

- `gitFirstCommit(path string) (author, date string, ok bool)` —
  `git log --follow --format=%an%x1f%aI --diff-filter=A -- PATH`, oldest
  entry (`--diff-filter=A` isolates the add; if that yields nothing, e.g. a
  moved/renamed file, fall back to the oldest `--follow` line with no
  filter). `ok=false` on any `exec.Command` failure (non-repo, untracked
  file) — detected from the command's own exit error, not a `.git`
  pre-check (design decision 3).
- `gitLastCommit(path string) (date string, ok bool)` —
  `git log -1 --format=%aI -- PATH`.
- `gitConfigUserName() (string, ok bool)` — `git config user.name`, run
  with cwd at the target file's directory.
- `fsBirthOrModTime(path string) (time.Time, error)` — birth time where the
  OS exposes it, `mtime` otherwise (build-tag or runtime fallback, confirm
  platform support during implementation).
- `detectByline(body string) (author string, ok bool)` — after
  `firstH1Heading`'s match line, scan forward to the next blank line or
  heading; match a leading `By `, `Author: `, or `Written by ` (confirm
  during implementation whether this should be case-insensitive).

### Tests (`cmd/kb/documentfrontmatter_test.go`)

- `TestGitFirstCommit_ReturnsOldestAuthorAndDate` — real `git init`/
  `git commit` in `t.TempDir()` (module's first `exec.Command` dependency,
  so this is necessarily a real-git integration test, not a mock).
- `TestGitFirstCommit_FalseWhenNotARepo`
- `TestGitLastCommit_ReturnsMostRecentCommitDate`
- `TestGitConfigUserName_ReturnsConfiguredName`
- `TestFsBirthOrModTime_ReturnsATimeForAnyFile`
- `TestDetectByline_FindsEachLeadInForm` — table test over `By `,
  `Author: `, `Written by `.
- `TestDetectByline_FalseWhenNoLeadInPresent`

### Acceptance criteria

- `go test ./cmd/kb/...` green. Confirm CI/dev machines have `git` on
  `PATH` (the Toolchain dependencies table doesn't list it today — flag as
  a new build/test dependency in FM6).

---

## FM2 — Field proposals: title / author / dateCreated / dateModified

- `proposeTitle(body string) string` — thin wrapper on `firstH1Heading`
  (`documents.go:353`).
- `signal struct { Source, Value string }` — one candidate value with its
  provenance label, for author's multi-signal report (decision 4's "show
  all signals when they disagree").
- `proposeAuthor(path, body string) (winner string, signals []signal)` —
  byline (FM1) → `gitFirstCommit` author → `gitConfigUserName`, first
  present wins as `winner`; `signals` always carries every signal that
  fired, in that priority order.
- `proposeDateCreated(path string) (value, source string)` —
  `gitFirstCommit` date → `fsBirthOrModTime` fallback.
- `proposeDateModified(path string) (value, source string)` —
  `gitLastCommit` date → `fsBirthOrModTime`. Always computed (decision 4's
  "not absent-only" exception); the caller (FM5) decides whether to write
  it.

### Tests

- `TestProposeAuthor_PrefersBylineOverGit`
- `TestProposeAuthor_FallsBackToGitFirstCommitAuthor`
- `TestProposeAuthor_FallsBackToGitConfigWhenUntracked`
- `TestProposeAuthor_SignalsListsEveryFiredSignal` — byline + git disagree,
  assert both appear.
- `TestProposeDateCreated_PrefersGitOverFilesystem`
- `TestProposeDateModified_AlwaysComputesRegardlessOfExistingValue`

### Acceptance criteria

- `go test ./cmd/kb/...` green.

---

## FM3 — Keyword proposals (DB read-only)

- **(a) Known-concept matches:** `knownKeywordProposals(kb, text string,
  currentKeywords []string) ([]string, error)` — calls `eligibleConcepts`
  (`cmd/kb/documenttag.go:198`) against every known concept name, then
  subtracts `currentKeywords` (case-insensitive) so only genuinely new
  matches are proposed.
- **(b) New candidate concepts:** requires splitting `scoreCandidateTerms`'s
  single-corpus assumption into a target-vs-comparison-scope shape.
  - New `scoreDocumentCandidateTerms(targetText string, comparisonItems
    []string, known map[string]bool) []candidateTerm` in
    `cmd/kb/concept.go` (sibling to `scoreCandidateTerms`, not a
    modification of it — that function's existing single-corpus contract
    and tests stay untouched): tokenize `targetText` alone for
    `occurrences`/per-term presence, then compute `idf` from
    `comparisonItems` (`df` = how many comparison-scope items also contain
    the term; smoothing form, e.g. `log((len(comparisonItems)+1) /
    (df+1))`, to confirm during implementation so a term absent from every
    comparison item still scores rather than dividing by zero).
    `comparisonItems` explicitly excludes the target document — that's the
    fix for decision 4b's idf-degeneracy bug.
  - `comparisonScope(kb, doc *knowledge.Document) ([]string, error)` — if
    `doc.ProjectID != 0`, the project's other records + document sections
    (mirroring `cmdConceptSuggest`'s existing gather loop,
    `cmd/kb/concept.go:267-290`, minus the target); otherwise whole corpus
    minus target.

### Tests

- `TestKnownKeywordProposals_ExcludesAlreadyListedKeywords`
- `TestScoreDocumentCandidateTerms_ExcludingTargetFixesIdfDegeneracy` — the
  exact regression case the design calls out: naive same-scope idf
  collapses to 0, this function must not.
- `TestScoreDocumentCandidateTerms_TermAbsentFromComparisonScopeStillScores`
- `TestComparisonScope_ScopesToProjectWhenDocumentBelongsToOne`
- `TestComparisonScope_FallsBackToWholeCorpusWhenNoProject`

### Acceptance criteria

- `go test ./cmd/kb/...` green.

---

## FM4 — `yaml.Node` surgical frontmatter writer

- `frontmatterNode(raw []byte) (node *yaml.Node, bodyOffset int, hadBlock
  bool, err error)` — locates the `---`/`---` block via `splitFrontmatter`
  (`recordfile.go:401`) positionally (reuse, don't reimplement);
  `yaml.Unmarshal` the frontmatter text into a `yaml.Node` (not
  `documentFrontmatter`, decision 5). `hadBlock=false` when none exists —
  caller starts from an empty mapping node.
- `setMappingField(node *yaml.Node, key string, value any) error` —
  find-or-append a scalar/sequence key in the top-level mapping, in place;
  must not disturb sibling keys, comments, or ordering it doesn't touch.
- `renderFrontmatter(node *yaml.Node) (string, error)` — re-encode via
  `yaml.Node`'s own marshal back to a `---`-delimited block.
- `writeDocumentFile(path string, raw []byte, hadBlock bool, bodyOffset
  int, newBlock string) error` — splice `newBlock` in place of the
  original block (or prepend, when `!hadBlock`) against `raw`, write to a
  temp file in the same directory, `os.Rename` over the original (decision
  9's single-file atomic write).

### Tests

- `TestFrontmatterNode_ParsesExistingBlock`
- `TestFrontmatterNode_NoBlockReturnsEmptyMappingHadBlockFalse`
- `TestSetMappingField_AppendsNewKey`
- `TestSetMappingField_OverwritesExistingKeyInPlace`
- `TestSetMappingField_LeavesUnknownKeysAndCommentsUntouched` — the
  data-loss regression this whole decision exists to prevent.
- `TestWriteDocumentFile_PrependsBlockWhenNoneExisted`
- `TestWriteDocumentFile_BodyBytesUnchangedOutsideFrontmatterBlock`
- `TestWriteDocumentFile_AtomicReplaceViaTempFileAndRename`

### Acceptance criteria

- `go test ./cmd/kb/...` green.

---

## FM5 — `cmdDocumentFrontmatter` wiring

- Add a `DateModified` field to `documentFrontmatter` (`documents.go:471-
  479`) — the one schema change (decision 4). Check whether
  `extractFrontmatter` (`documents.go:490`) needs a matching field for
  read-side consistency; confirm during implementation whether anything
  downstream consumes it yet.
- `cmdDocumentFrontmatter(kb, jsonOut bool, args []string, out io.Writer)
  error` — parse `PATH`, `--accept FIELD,...`, `--accept-keywords
  NAME,...`, repeatable `--set FIELD=VALUE`, `--dry-run` (design decision
  2). Resolve `kb.DocumentByPath(path)` (`documents.go:274`) if already
  ingested (for FM3's project scoping); proceed even when not ingested
  (whole-corpus comparison scope).
- Build a per-field report: current value (if any), proposed value,
  source/signals (FM2's `signal` list for author). Bare invocation with no
  `--accept`/`--accept-keywords`/`--set` is read-only by construction —
  just prints the report.
- Apply: for each accepted field or `--set` override, call
  `setMappingField`; for accepted signal-(b) keywords, `kb.AddConcept(name,
  "")` (`knowledge.go:1087`) *before* writing the keyword (decision 4b,
  mirrors `documenttag.go`'s `--concept` must-exist-first rule) — fail the
  whole call if concept creation fails, nothing written.
- `--dry-run` combined with an accept/set selection: build the report and
  the would-be write, print it, skip the actual `writeDocumentFile` call.

### Tests (`cmd/kb/documentfrontmatter_cmd_test.go`)

- `TestCmdDocumentFrontmatter_BareInvocationIsReadOnly`
- `TestCmdDocumentFrontmatter_AcceptsTitleField`
- `TestCmdDocumentFrontmatter_NeverOverwritesExistingTitleAuthorOrDateCreated`
- `TestCmdDocumentFrontmatter_DateModifiedAlwaysRefreshedOnAcceptedRun`
- `TestCmdDocumentFrontmatter_SetOverridesBypassesSignalDetection`
- `TestCmdDocumentFrontmatter_AcceptKeywordsPlainWriteForKnownConceptMatch`
- `TestCmdDocumentFrontmatter_AcceptKeywordsCreatesConceptForNewCandidateBeforeWriting`
- `TestCmdDocumentFrontmatter_DryRunWithAcceptPreviewsWithoutWriting`
- `TestCmdDocumentFrontmatter_BodyNeverModified`
- `TestCmdDocumentFrontmatter_JSONOutputShape`

### Acceptance criteria

- `go test ./cmd/kb/...` green.
- Manual smoke test (per `feedback_smoke_test_data_transforms`): a scratch
  document missing frontmatter entirely, run without flags to see the
  report, then with `--accept title,author --accept-keywords X`, inspect
  the file by eye, then `kb document ingest` it and confirm via
  `kb document show`.

---

## FM6 — Documentation

- `cmd/kb/helptext.go`'s `DocumentHelpText` (`:154`): add the
  `frontmatter` synopsis line and a description paragraph.
- Note in `codemeta.json`/`README.md`'s toolchain section (or wherever
  `git` is first mentioned as a runtime dependency) that
  `kb document frontmatter` shells out to `git` — this module's first
  process dependency; regenerate via `cmt` per repo convention, don't
  hand-edit.
- `make kb-topics-help` to regenerate `kb-document.1.md`.

### Acceptance criteria

- `git diff kb-document.1.md` shows only the expected addition, regenerated
  from the edited `helptext.go` constant — no hand-edits to the `.1.md`
  file itself.

---

## After FM6: a decision record

Once implemented and smoke-tested, author a decision record via
`kb record new --project knowledge`, following DR-0029's own precedent
(`kb document tag`'s original implementation) as the most directly related
prior work. Authored `proposed`, promoted by the user — not self-promoted,
per this workspace's standing rule that a model may write a record but not
accept one.
