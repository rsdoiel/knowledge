# Lifting `cmd/kb` Logic into the Library — Implementation Plan

See [library-lift-design.md](library-lift-design.md) for rationale and the
decisions this implements (DR-0035, `proposed`). Work items L1 → L5.
TDD-first: the library-level test for each item is written and confirmed red
before the code moves. Pure move: no behavior changes, and the existing
`cmd/kb` tests pass **unchanged** after every item. One commit per item,
made only when RSDOIEL asks.

Every new exported symbol gets the `/** … */` block (description,
parameters, returns, example).

---

## L1 — Document ingest

**Move:** `ingestNewDocument`, `reingestChangedDocument`, `tagSection`,
`tagDensity`, `densityLinkCandidates`, `wholeDocumentText`,
`stripCodeSpans` from `cmd/kb/document.go` into a new `documentingest.go`
(root package).

**New API (sketch):** `(*KnowledgeBase).IngestDocument(projectID, path,
title, opts) (DocumentIngestResult, error)`. The result carries what
`documentIngestSummary` carries today (created/updated/unchanged, sections
added/updated/stale, removed headings, parser warning).

**Red tests first:** ingest a fixture → new document, gist + sections
present; re-ingest unchanged → no-op; re-ingest with one changed body →
`summary_stale` set, summary kept; removed heading reported, not deleted.
Port the assertions from `cmd/kb/document_test.go`.

**Also settle:** which path form `IngestDocument` stores (relative to what).
Document it in the function's block; it becomes a cross-consumer contract.

**DONE 2026-09-23** (uncommitted). `documentingest.go` holds `IngestDocument`,
`DocumentIngestOptions` (Title, Format, DryRun) and `DocumentIngestResult`
(same JSON tags as the old summary; `cmd/kb` aliases it), plus the verbatim-moved
bodies. `StripCodeSpans` is exported because `concept.go`, `documentfrontmatter.go`
and `documenttag.go` in `cmd/kb` also call it (the plan listed only
`documenttag.go`; the compiler found the rest). **Stored-path form:** the path is
stored exactly as passed, and `DocumentByPath` is an exact string match; this is
documented on `IngestDocument`. **Temporary duplication:** `documenttag.go` keeps
local copies of the two code-span regexes, removed when L2 moves
`excludedSpans`. 16 new library tests written red first; all pre-existing
`cmd/kb` tests pass unmodified. Live check: old (HEAD) and new binaries run on
copies of the real `knowledge.db` over 9 real files (fresh, repeat, changed
file with a renamed heading, dry run, missing file): stdout and the resulting
documents/sections/concept-link tables were byte-identical.

**Done when:** `cmdDocumentIngest` is parse → `IngestDocument` → render;
`go test ./...` green with `cmd/kb` tests untouched.

## L2 — Tag and fuzzy-tag text transforms

**Move:** `tagDocumentText`, `insertWikilink`, `alreadyWikilinked`,
`excludedSpans`, `eligibleConcepts` from `documenttag.go`; `insertFootnote`,
`nearestPrecedingHeading`, `nextSectionBoundary`, `nextFootnoteLabel`,
`fuzzyExcludedSpans`, `fuzzyThreshold`, `fuzzyEligible` from
`documentfuzzytag.go`.

**New API (sketch):** `TagDocumentText`, `FuzzyTagDocumentText`,
`(*KnowledgeBase).EligibleTagConcepts`. Text in, text out; **no file I/O in
the library.** `cmdDocumentTag`/`cmdDocumentFuzzyTag` keep reading and
writing files.

**Red tests first:** move the existing text-level tests; keep the two v0.0.11
regression cases (two near-misses in one section must not splice a marker
inside an earlier footnote's definition; second run is a no-op).

**Caution:** move verbatim. The fuzzy-tag byte-offset logic already had one
corruption bug; do not "tidy" it.

**DONE 2026-09-23** (uncommitted). `documenttag.go`/`documentfuzzytag.go` in the root
package: `TagDocumentText`, `(*KnowledgeBase).EligibleTagConcepts`, `FuzzyEligible`,
`FuzzyTagDocumentText`, `FuzzyTagInsertion` (same JSON tags; `cmd/kb` aliases it).
`FuzzyTagDocumentText` is the per-file loop that lived inline in `cmdDocumentFuzzyTag`,
moved verbatim with its two-breakpoint offset tracking. The two cmd unit-test files
(`documenttag_test.go`, `documentfuzzytag_test.go`) moved to the root with the logic they
test; 8 new tests cover the exported surface, including a text-level port of the
two-near-misses corruption regression. The `cmd` tests for the verbs are unmodified.
L1's temporary regex copy in `cmd/kb/documenttag.go` is gone; three regexes now have a
marked temporary copy in `documentfrontmatter.go` until L4. Live check: old (pre-L1
HEAD) vs new binary, both on scratch copies of 11 files, every mode incl. dry-run,
write, second run, `--concept`, and error cases: output (738 lines) and rewritten files
byte-identical; two near-misses in one section verified in isolation.
**Process slip, fixed:** my first comparison run used a database copy still holding real
document rows, so the old binary edited the real
`harvey/knowledge-learning-mode-feature-request.md`. Restored from git (the diff was only
tag/footnote insertions); the rerun deleted the copy's document rows and ran from a
scratch cwd.

## L3 — Concept suggest with clustering

**Move:** `scoreCandidateTerms`, `tokenizeCandidateItems` from `concept.go`;
`fuzzyTermsClose`, `excludeNearExisting`, `clusterCandidateTerms`,
`minFuzzyTermLength` from `conceptcluster.go`.

**New API (sketch):** `(*KnowledgeBase).SuggestConcepts(projectID, limit)
(ConceptSuggestions, error)` with `Candidates` and `NearExisting`, matching
the `--json` shape `{"candidates": …, "near_existing": …}`.

**Red tests first:** the chunking/chunkings/chunked cluster, the
cat/cot/cog/dog anti-drift chain, and the short-word false-positive gate
(`table`/`stable`/`able` must not cluster).

**DONE 2026-09-23** (uncommitted). `conceptsuggest.go` and `conceptcluster.go` in the
root package. New exported API: `(*KnowledgeBase).SuggestConcepts(project string,
limit int)` (takes a project *name*, not an id, since that is what `ListRecords`
filters on, and what a caller has), `ConceptSuggestions`, `ConceptCandidate`,
`NearExistingMatch` (same JSON tags; `cmd/kb` aliases `candidateTerm` to
`ConceptCandidate` for `documentfrontmatter.go` until L4), and `CandidateTerms`. That
last one was not in the plan: `documentfrontmatter.go` shared the tokenizer's
pattern, stopword list and record-id filter, and one exported function was cheaper
than a second copy of a 40-word list. `tokenizeCandidateItems` now takes its
tokens from it; that is the only non-verbatim change, and the moved tests pass
unchanged. `cmd/kb/conceptcluster.go` is deleted. The 29 unit tests for the moved
logic moved with it; the 21 CLI-level tests stayed. 9 new library tests cover
`SuggestConcepts` and `CandidateTerms`. The `--debug` trace for `concept suggest`
now logs the candidate count, no longer the item count (design decision 6).
Live check against the real `knowledge.db` (read-only): pre-L3 vs new binary,
24 invocations (text and `--json`, uncapped 1,583 lines, all 9 project scopes,
`--limit` values, error cases) byte-identical; real db unchanged. The frontmatter
scorer, also changed, matched on 7 real files apart from a pre-existing
ordering nondeterminism (filed in `TODO.md`).

## L4 — Frontmatter propose / apply

**Move:** `proposeTitle`/`Author`/`DateCreated`/`DateModified`,
`detectByline`, `knownKeywordProposals`, `scoreDocumentCandidateTerms`,
`comparisonScope`, and the `yaml.Node` writer helpers from
`documentfrontmatter.go`. The `git*` helpers become the default
implementation of a new `Provenance` interface (design decision 5).

**New API (sketch):** `ProposeFrontmatter`, `ApplyFrontmatter(raw, accepted)
([]byte, error)` returning bytes, not writing.

**Red tests first:** tests use a fake `Provenance`, so no real `git init` is
needed at the library level; keep one `GitProvenance` test using a real
`t.TempDir()` repo. Keep the v0.0.11 regressions: `title: ""` must not lock a
field, and `--set FIELD=` must write.

**DONE 2026-09-23** (uncommitted). `provenance.go`: the `Provenance` interface
(`FirstCommit`, `LastCommit`, `ConfigUserName(dir)`, `FileTime`) and `GitProvenance{}`, the
verbatim git and filesystem primitives and this module's only `os/exec`.
`documentfrontmatter.go`: `ProposeFrontmatter`, `ApplyFrontmatter`, `FrontmatterAccept`,
`FrontmatterResult`, `FrontmatterFieldReport`, `FrontmatterKeywordReport`,
`FrontmatterSignal`, `IsFrontmatterScalarField`, plus the verbatim proposers, scoring and
YAML helpers. Both functions take the file's bytes and return bytes: the library never
reads or writes the file, so a caller's own permission layer stays in charge
(`spliceFrontmatter` is the pure half of the old `writeDocumentFile`; the atomic write
stays in `cmd/kb` as `writeFileAtomic`). A nil `Provenance` means `GitProvenance`. One API
surprise, kept from the CLI: `ApplyFrontmatter` creates a new-candidate keyword's concept
in the database before returning (never on a dry run), so a caller that then fails to
write leaves the concept behind. 25 unit tests moved with the logic, and the git-backed
ones still run against a real repository; 17 new tests use a fake `Provenance`; 17
CLI-level tests stayed. `cmd/kb/documentfrontmatter.go` went from 936 lines to about 230.
The two temporary regex copies from L2 and the `candidateTerm` alias from L3 are gone.
Live check: pre-L3 binary vs new, 6,781 lines of output over every mode (reports and
`--json` for 7 files, single-field accepts, multi-field accept with key order normalized,
`--set`, empty `--set`, dry runs, keywords known/new/rejected in real and dry-run modes
with concept-table checks, error cases), in a scratch git repo with fixed commit dates:
identical, and the real repo and database untouched. **Two pre-existing nondeterminisms
were found and filed in `TODO.md` rather than fixed inside a pure move.**

## L5 — Record ingest (deferred, optional)

`ingester` in `ingest.go` (629 lines, the most stateful piece). **Do not
start** until L1–L4 have shipped and harvey's need is concrete (see design
decision 4). If started, gets its own short plan.

## Release

After L4: bump to **v0.0.12** via `codemeta.json` + `cmt` regeneration
(never hand-edit generated files), `CHANGES.md` section, a smoke test of
every affected `kb` verb against a copy of the real `agents/knowledge.db`
**before** tagging, and `--prerelease` on the GitHub release. Tag/publish is
RSDOIEL's step.

---

## Addendum — false-positive fixes (2026-09-23, DR-0036, uncommitted)

Done between L2 and L3, as a separate change from the pure moves above, after
the L2 tree was green. Found while updating harvey's skills; measured over 159
real documents before any code changed. Three fixes, each with tests written
red first: (1) `FuzzyEligible` gets a concept-length floor (6) and a 6-letter
shared-prefix rule for distance-2 matches (614 → 224 proposals, all 14 measured
false positives gone, all 10 true positives kept); (2) `fuzzyTermsClose` requires
a shared first letter in both tiers (first tried as a stem-prefix rule; the
red test showed `nesting/testing` is a *raw* distance-1 match, so that could
not work); (3) `--accept-keywords` validates before any write, and `--dry-run`
no longer creates concepts (a fourth bug, found while writing the tests). One
existing test was corrected because it had passed an unproposed term. Verified
old-vs-new on real data; the constants are provisional. The fuzzy-tag gate lives
in the library (`documentfuzzytag.go`), so harvey inherits it. The clustering
and keyword fixes are in `cmd/kb` and move with L3 and L4.

---

## Addendum 2 — bugs fixed before v0.0.12 (2026-09-23, DR-0037, uncommitted)

RSDOIEL's bar: no known bugs before the release. Six items, each written red
first, then verified on real data with an old-vs-new binary comparison.

1. **Map-order output**, five sites: `EligibleTagConcepts` (sorted), `frontmatter`
   accepted/`--set` fields (one pass in canonical order; a test also guards that
   `frontmatterFieldOrder` covers exactly the scalar fields), the unknown `--set`
   field in an error (alphabetical), `RemovedHeadings` (the old document's order, each
   heading once), and `project rename`'s notes (sorted). The last two were found by a
   static audit of every `range` over a map, not from the TODO.
2. **Punctuation-edged concept names** (`C++`, `F#`, `.NET`): a shared
   `conceptNameMatches` replaces the `\b` regexes in `MatchConceptNames`,
   `MatchConceptNameCounts` and `insertWikilink`. A differential test against the old
   regex on 4,000 random strings agrees exactly for letter-edged names. The old rule
   was wrong both ways: it missed `C++` and matched `.NET` inside `ASP.NET`.
3. **A name with no letter or digit never matches.** Found by the live check: the real
   database holds a junk concept `...` that `\b` had always kept inert, and the new
   matcher would have made it match every ellipsis.
4. **Wikilinks inside code minted concepts** (documents and records). Over 159 real
   documents ingest goes from 213 concepts to 147; none is newly minted and all 66
   that stopped are examples inside code, not real tags.

Verification of the finished tree: 6,781 lines of `document frontmatter` output
identical to the pre-change binary; 60 read-only invocations against the real database
each stable across 5 runs and identical to the pre-change binary; the file-taking verbs
give one output across 12 full runs where the pre-fix binary gave 12. The real database
was never written. **Known and left for the user:** the real database's junk concept `...`
(id 162), which cannot match now (`kb concept delete` was added afterwards, see Addendum 3; it is linked to two records); the `cancelled`-status request
and the corpus-improvement exploration in `TODO.md` is an open question, not a bug (the `cancelled`-status request was done afterwards, Addendum 3).

---

## Addendum 3 — two more items for v0.0.12 (2026-09-23, DR-0038, DR-0039, uncommitted)

RSDOIEL approved DR-0037 (`accepted`, at their instruction) and asked for the concept delete
verb and the `cancelled` status to be included in the release.

1. **`cancelled` status (DR-0038).** Added to `RecordStatuses`; documented in `kb help
   record` with the adopted-then-abandoned meaning and the reason-in-the-body convention.
   Tests written red first; `list --status cancelled` and the index rendering already worked
   and are pinned by tests. `DECISION_RECORD_FORMAT.md` is not on this machine and still
   needs the line wherever it lives.
2. **`kb concept delete` (DR-0039).** Two design questions put to RSDOIEL and answered:
   local delete with a documented caveat (no tombstone), and refuse a linked concept unless
   `--force`. `ConceptUsage` and `DeleteConcept` do the rows in one transaction, then the
   search entry; the CLI adds `--dry-run` and the `--` terminator, since junk concept names
   look like flags. Live check on a scratch copy of the real database: `...` refused
   (2 records), previewed, then deleted with `--force`; record and record row untouched,
   search entry gone, `integrity_check` ok; the real database was not touched.
   **A claim of mine was wrong and a test caught it:** I had written that a file still
   naming the concept "recreates it the next time it is ingested". Ingest skips an unchanged
   file, so the concept stays deleted until that file changes (or the database is rebuilt
   from files); the note, help text and library doc now say so. Also corrected: `...` is
   linked to two records, not one.

The man pages (`kb-record.1.md`, `kb-concept.1.md`) are generated from `helptext.go` by
`make kb-topics-help` and are regenerated at release prep, when the version is bumped.
