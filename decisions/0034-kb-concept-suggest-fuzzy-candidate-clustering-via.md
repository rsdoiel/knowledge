---
id: "0034"
title: "kb concept suggest: fuzzy candidate clustering via shared Levenshtein primitives"
date: "2026-09-23"
status: proposed
kind: decision
trigger: implementation
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a0cf19-d4db-791b-8bd5-b1a8e9af78b4"
origin_host: "wren"
---

**Context.**

`scoreCandidateTerms` (`concept.go`) scores each single-word candidate term
independently. A concept whose mentions are split across spelling variants
— `chunking`/`chunkings`/`chunked` — is scored as several separate
low-count terms, each of which can individually fall below the `occ < 2`
floor or the `idf <= 0` distinctiveness filter even though the combined
mentions clearly signal one real, frequently-discussed concept. v0.0.11
item 5, designed in `fuzzy-concept-clustering-design.md` (2026-09-22,
full conversation) and planned in `fuzzy-concept-clustering-plan.md`
(phases FC1–FC5). Implemented and smoke-tested 2026-09-23, immediately
after DR-0032 (`kb document fuzzy-tag`), whose shared primitives this
feature reuses.

**Decision.**

1. `LevenshteinDistance`/`StripCommonSuffix` (`retrieval.go`), built for
   DR-0032, are **exported** — a correction the plan didn't anticipate:
   both were unexported, so `cmd/kb`, where this feature's code lives,
   could not call them across the package boundary. Exporting was the
   only way to satisfy the design's own explicit "shared, not duplicated"
   requirement (decision 1) rather than a second copy of the same
   algorithm.

2. **The same self-contradiction found in DR-0032 recurred here, in two
   places.** Decisions 3 (near-existing exclusion) and 4 (star
   clustering) both originally said to compare two terms by stemming
   *both* sides via `StripCommonSuffix` before measuring distance.
   Computed directly: `chunking`/`chunkings` (raw distance 1) becomes
   distance 3 once both sides are stemmed — this design's own worked
   example would never fire. Unlike DR-0032, a *pure* raw-distance
   check doesn't fully resolve it either: `chunking`/`chunked` (raw
   distance 3) only comes within threshold when *both* sides are stemmed
   (both reduce to `chunk`). **Fix**: a shared `fuzzyTermsClose(a, b,
   maxDistance)` (`cmd/kb/conceptcluster.go`) tries raw distance first,
   falling back to comparing both sides' stemmed forms — a *symmetric*
   fallback, unlike fuzzy-tag's asymmetric one (which never stems the
   canonical concept name), because here both terms are equally
   unverified candidates with no canonical side to protect. Star
   clustering's anti-drift property still holds under this corrected
   check: verified against the classic cat/cot/cog/dog chain (each
   adjacent pair distance 1, endpoints not) — the corrected function
   still refuses to let a chain-only relationship join a cluster.

3. **A second, distinct problem found live-smoke-testing against the real
   `agents/knowledge.db`, after both of the above were already fixed**: a
   flat `distance <= 1` threshold with no length floor produces heavy
   false-positive clustering specific to short, common candidate terms —
   `table (+tables, stable, able)`, `old (+holds, holding, hold, folding,
   fold, folded, cold, folds, told)`, `makes (+make, making, man, takes,
   map, marked, bakes, marks)`. This is a different failure mode than the
   chain-drift risk decision 4 already guards against, and one the
   design's own "flat threshold, no length-based split" reasoning (decision
   5) didn't anticipate — that reasoning is about not *loosening* the
   threshold for long words, not about *tightening* it for short ones.
   **Fixed with `minFuzzyTermLength` (6 characters, provisional)**: below
   this length, `fuzzyTermsClose` never compares two terms at all, in
   either tier. Re-running the same corpus afterward: every false positive
   above is gone; genuine merges (`rename`/`renamed`/`renaming`/`renames`,
   `import`/`imported`/`importing`/`imports`) are unaffected. One small
   residual case remains, accepted rather than chased further:
   `reference`/`preference` (distance 1, both well above the length
   floor) — two unrelated real words that happen to be a coincidental
   one-character edit apart, inherent to pure edit distance with no
   semantic/dictionary check.

4. Otherwise implemented per plan: `scoreCandidateTerms` restructured so
   `itemCounts` is a set of item indices per term, not a bare count (FC2,
   needed so a cluster's `df` is a *union* across members, not a sum);
   `excludeNearExisting` (FC3) drops a candidate fuzzy-close to an
   already-known concept before it ever reaches clustering — that's
   `fuzzy-tag`'s job, not this command's; `clusterCandidateTerms` (FC4)
   is star clustering (sorted by occurrence descending, each member
   checked only against its cluster's own seed, never another member);
   `candidateTerm` gained an optional `variants` field and
   `scoreCandidateTerms` now returns `(candidates, nearExisting)` (FC5).

5. **`--json`'s shape changes** from a bare array to
   `{"candidates": [...], "near_existing": [...]}` — an intentional,
   plan-specified breaking change for any existing consumer of `kb
   concept suggest --json`.

**Rationale.**

Computing the actual worked-example distances by hand before writing any
code (same discipline as DR-0032) caught both the algorithmic
contradiction and confirmed the length-gate fix actually resolves the
live false positives, rather than trusting the design document's
narrative description of its own correctness.

**Rejected alternatives.**

Stemming both sides unconditionally, as originally written in decisions 3
and 4 — rejected once hand-computation showed it breaks the design's own
motivating example. A pure raw-distance-only check — also rejected, since
it fails the design's *other* motivating pair (`chunking`/`chunked`),
which only clusters correctly when both sides are stemmed.

**Consequences.**

`fuzzy-concept-clustering-plan.md`'s FC1–FC5 test lists are all
implemented and green (`go test ./...`, `go vet ./...` clean), including
a length-gate-safe rebuild of the chain-drift test (the design's own
`cat`/`cot`/`cog`/`dog` example is now too short to exercise post-fix, so
the test uses synthetic 6-letter words with the same distance shape
instead). `fuzzy-concept-clustering-design.md` amended in place with both
corrections, not silently rewritten. `minFuzzyTermLength`'s value (6) and
the underlying distance-1 threshold both remain open for a future record
if further real-corpus use shows either needs adjusting.
