---
id: "0032"
title: "kb document fuzzy-tag: Levenshtein near-miss matching via footnote insertion"
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
uuid: "01a0ceca-5af9-7a3f-88d1-8520a6e668d4"
origin_host: "wren"
---

**Context.**

`MatchConceptNames`/`MatchConceptNameCounts` (`retrieval.go`) and `kb
document tag` (DR-0029) match a known concept's name exactly, whole-word.
A typo, plural, or simple tense variant of a concept's name links nothing.
v0.0.11 item 1, designed in `fuzzy-concept-matching-design.md` (2026-09-21,
revised 2026-09-22), scoped a new verb, `kb document fuzzy-tag`, to close
this gap for spelling-level variants only — not paraphrase, which is a
semantic-similarity problem this module has deliberately avoided elsewhere
(DR-0028). Implemented and smoke-tested 2026-09-23, following
`fuzzy-concept-matching-plan.md`'s phases F1–F4.

**Decision.**

1. `FuzzyMatchConceptNames(text) ([]FuzzyConceptMatch, error)` added to
   `retrieval.go`, mirroring `MatchConceptNames`'s shape and location. For
   each known concept not already an exact whole-word match anywhere in
   `text`, it tokenizes once and compares each token (or, for a multi-word
   concept name, each contiguous run of tokens) against the concept name.

2. **Distance is raw Levenshtein by default; `stripCommonSuffix` (lowercase,
   strip one trailing suffix from `ing`/`es`/`ed`/`s`, longest-first) is
   applied only to the candidate token, and only as a fallback when raw
   distance exceeds `maxFuzzyDistance` (3).** This corrects the design
   doc's original wording, which said to normalize *both* sides before
   measuring distance — verified by hand during implementation that doing
   so breaks the design's own three worked examples (stemming `chunking`
   itself via its trailing `ing` turns it into `chunk`, which no longer
   resembles either `chunkings` or `chunkibg`, driving their distance to 3
   instead of the claimed 1). Raw distance alone reproduces all three
   worked numbers exactly; the stemmed-token fallback exists for the case
   raw distance actually needs it: a short concept name against a longer
   suffixed token (e.g. `chunk` vs `chunkings`, raw distance 4, falls back
   to stemmed distance 3). `fuzzy-concept-matching-design.md` decision 5 is
   amended in place with a dated correction note, not silently rewritten.

3. **Insertion is a footnote, never a bracket-wrap**, exactly as designed:
   `[^n]` after the near-miss, `[^n]: see [[Concept]]` at the next section
   boundary (or end of file), carrying the canonical spelling so
   `ResolveConceptName` can never mint a duplicate concept from a
   misspelled or inflected form. `cmd/kb/documentfuzzytag.go` (new file)
   holds the raw-text mechanics (`nearestPrecedingHeading`,
   `nextSectionBoundary`, `nextFootnoteLabel`, `fuzzyExcludedSpans`,
   `insertFootnote`) and `cmdDocumentFuzzyTag`'s wiring, matching
   `cmd/kb document tag`'s shape: `--project`, `--concept` (bypasses the
   real, length-based eligibility threshold for exactly the names given),
   `--dry-run`, both-or-neither staged writes, `--json`.

4. **Eligibility (`fuzzyEligible`) takes `text` as a third parameter**,
   correcting the plan's own listed two-parameter signature, which omitted
   it — `fuzzyExcludedSpans(text)` needs the raw text to find frontmatter,
   code spans, existing wikilinks, and footnote-definition lines to
   exclude a marker from landing inside any of them.

5. Wired into `cmd document`'s subverb switch as `fuzzy-tag`, alongside
   `tag`. Documented in `DocumentHelpText` (`cmd/kb/helptext.go`);
   `kb-document.1.md` regenerated via `make kb-topics-help`, never
   hand-edited.

**Rationale.**

Raw-distance-first, stemmed-token-fallback is the only reading of decision
5 that satisfies all three of the design's own worked examples
simultaneously — computed directly (see Decision 2) rather than assumed.
The footnote mechanism reuses `alreadyWikilinked`'s existing idempotency
check for free: a footnote definition is just text containing
`[[Concept]]`, so a second `fuzzy-tag` run on an unchanged file is already
a no-op with no new mechanism. Ascending processing order with a running
byte-offset `delta` (rather than reversing match order) keeps
`cmdDocumentFuzzyTag`'s report in natural file order while still handling
the fact that each insertion shifts every later match's original offsets.

**Rejected alternatives.**

Stemming both the concept name and the candidate token before measuring
distance, per the design doc's original wording — rejected because it is
empirically wrong: it breaks the plural and typo cases the design itself
uses as worked examples. See fuzzy-concept-matching-design.md's amended
decision 5 for the full computation.

**Consequences.**

`fuzzy-concept-matching-plan.md`'s F1–F4 test lists are all implemented
and green (`go test ./...`, `go vet ./...` clean), plus a live smoke test:
a scratch document with a deliberate plural typo, real `fuzzy-tag` run,
footnote inserted correctly by eye, re-ingested, and `kb document show`
confirmed the concept actually linked on both the gist and section rows.
Distance-threshold tuning (design's explicitly-deferred item) remains open
for a future record if the current 1/2-by-length split proves too strict
or loose in practice.
