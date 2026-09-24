---
id: "0036"
title: "Fuzzy-match false-positive gates and keyword acceptance validation"
date: "2026-09-23"
status: proposed
kind: correction
trigger: live-test
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0032", "0033", "0034"]
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a0d05e-7664-7ace-890b-2f2f99116047"
origin_host: "wren"
---

**Context.**

Updating the harvey skills for v0.0.11 (2026-09-23), live checks against real
data showed the three v0.0.11 fuzzy/keyword features misbehaving in ways the
synthetic fixtures never exercised. A dry run of `kb document fuzzy-tag` over
159 real documents against the real 121-concept vocabulary returned 614
proposals, most of them noise: `fts<-its` (96 times), `drift<-draft` (47),
`madr<-made` (30), `kb<-db`, `cmt<-cmd`, `Go<-to`, `awk<-ask`, `tui<-ui`, and
distance-2 substitutions on long words (`retirement<-requirement`,
`correction<-connection`, `supersession<-suppression`). `kb concept suggest`'s
near-existing block reported `nested~testing`, `nesting~testing` and
`around~grounding`. And `kb document frontmatter --accept-keywords` wrote an
arbitrary string into a file and minted a real concept for it, and did so
even under `--dry-run`. Corrects DR-0032, DR-0033 and DR-0034, which stay
`accepted`: their decisions stand, this changes thresholds and closes gaps.

**Decision.**

1. `FuzzyEligible` (no `--concept`) drops a match whose concept is shorter
   than `minFuzzyConceptLength` (6 letters), and drops a distance-2 match
   sharing fewer than `minFuzzyDistanceTwoPrefix` (6) leading letters with its
   concept. An explicit `--concept` still bypasses both, as it bypassed the
   distance threshold before.
2. `fuzzyTermsClose` (concept suggest's near-existing and clustering) requires
   the two terms to share a first letter, in both the raw and the stemmed tier.
3. `--accept-keywords` accepts a name only if it is a known concept or one of
   the report's proposed new candidates, checked before any concept is created
   or any byte written, and aborting the whole call. On `--dry-run` it never
   creates a concept.

**Rationale.**

Measured, not guessed: over the corpus above the two fuzzy-tag gates take 614
proposals to 224, all 14 measured false positives to zero, and keep all 10
measured true positives. The true distance-2 hits are inflections of a long
stem (`idempotency<-idempotent`); the false ones share only two or three
opening letters. Every genuine near-existing pair shares its first letter;
the three false ones did not. The same first-letter rule also stopped
`preference` merging into `reference` and `treats/treating/treated` merging
into `created`. Item 3 restores the check-before-write rule DR-0033 already
named, mirroring `kb document tag --concept`.

**Rejected alternatives.**

- A minimum concept length of 5: keeps `merge<-merged` (13 real hits) but
  also keeps `drift<-draft` (47 false ones).
- Allowing short concepts through when the difference is a plain inflection:
  more machinery for a case `--concept` already covers.
- Applying the first-letter rule to fuzzy-tag as well: not needed once the
  length gate is in, and a first-letter typo is possible there.
- Rejecting an unproposed keyword only with a warning: the probe showed the
  write and the minted concept are the harm, not the absence of a message.

**Consequences.**

Both gate constants are provisional, tuned on one corpus, like
`minFuzzyTermLength` in DR-0034. A short concept's plural (`merge<-merges`,
`help<-helps`) is no longer found without `--concept`; residual doubtful hits
remain (`schema<-scheme`, `format<-formal`, `ingest<-investing`, about 4% of
what survives). `concept suggest --json` output changes only in which pairs it
contains, not its shape. One existing test,
`AcceptKeywordsCreatesConceptForNewCandidateBeforeWriting`, had been passing an
unproposed term and is corrected to a genuinely proposed one; it exercised the
gap, not the behavior it was named for. Status `proposed`: promotion is the
author's call.
