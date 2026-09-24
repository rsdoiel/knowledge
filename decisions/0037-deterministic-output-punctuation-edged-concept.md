---
id: "0037"
title: "Deterministic output, punctuation-edged concept names, and wikilinks in code"
date: "2026-09-23"
status: accepted
kind: correction
trigger: live-test
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0027", "0033", "0036"]
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a0d08b-4cc0-7a01-81fe-eeeb9d3eb137"
origin_host: "wren"
---

**Context.**

The library lift (DR-0035) was verified by running a binary built from before
each step against the new one on real data. That, and a repeat-run check over
every verb, found bugs that had been in the code all along. Five sites returned
output in map-iteration order, so it changed from one run to the next:
`EligibleTagConcepts` (so `document frontmatter` listed its known-concept
proposals in a different order each run), `document frontmatter --accept`/`--set`
(new keys were appended to the file in map order: two orders in 8 runs, real
diff churn in users' files), the unknown `--set` field named in its error,
`RemovedHeadings` after a re-ingest (text and `--json`), and the notes
`project rename` prints when index refreshes fail. Separately, a concept whose
name begins or ends in punctuation (`C++`, `F#`, `.NET`) could never match a
mention, because `\b` only fires between a word character and a non-word one;
and `.NET` wrongly matched inside `ASP.NET`. Last, a `[[...]]` written inside a
code span or a fenced block, as an example of the syntax, minted a real concept
(that is where junk like `...`, `recall: ...` and `tool: ...` came from), though
`kb document tag`, density-linking and fuzzy-tag already exclude code. This
corrects DR-0027 (density and wikilink tagging), DR-0033 (frontmatter) and
DR-0036.

**Decision.**

1. Every output that was ordered by map iteration is now ordered on purpose:
   `EligibleTagConcepts` sorts case-insensitively then exactly; accepted and
   `--set` fields apply in one pass in the canonical order title, author,
   dateCreated, dateModified (an explicit `--set` still wins over an accepted
   proposal for the same field); an unknown `--set` field error names the
   alphabetically first; `RemovedHeadings` is the old document's own order,
   each heading once; `project rename` notes are in sorted directory order.
2. A concept name matches when it is not glued to an ASCII word character on
   either side (`conceptNameMatches`), which is what `\b` meant for names that
   start and end with a letter, so those behave exactly as before. It now also
   works for names that begin or end in punctuation. A name with no letter or
   digit never matches.
3. A wikilink inside a code span or fenced block is an example, not a tag, in
   both document ingest and record ingest.

**Rationale.**

Determinism: a tool whose output changes between identical runs cannot be
diffed, tested or trusted, and the key order was written into users' files.
Matching: the old rule was wrong in both directions, missing `C++` and matching
`.NET` inside `ASP.NET`. Neighbour checks are the only way to express it in RE2,
which has no lookbehind. The letter-or-digit rule is needed because with
punctuation names supported a junk concept such as `...` would otherwise match
every ellipsis; the live check found exactly that. Code exclusion makes minting
agree with every other place the workspace already excludes code. Evidence:
a differential test compares `conceptNameMatches` with the old regex on 4,000
random strings for letter-edged names and finds no difference; over 159 real
documents `links`, `tag` and `concept suggest` output was unchanged; ingest went
from 213 concepts to 147 with none newly minted, and all 66 that stopped being
minted are examples inside code (none is a real tag).

**Rejected alternatives.**

- Alphabetical order for `RemovedHeadings`: document order tells a reader where
  the missing sections were.
- A Unicode-aware word test in the matcher: it would change how letter-edged
  names match near accented text, which this correction does not set out to do.
- Deleting existing junk concepts automatically: `kb` has no concept-delete verb
  and the vocabulary is the user's data; the real database still holds one, the
  concept `...`, now inert.

**Consequences.**

Tag density and density-linking can now count a punctuation-edged concept, so
junk concepts of that shape already in a database (there is one, `...`, and it
cannot match) or minted before this change (`recall:` and the like) become live
matchers until removed. A record or document that relied on a wikilink inside
code to create a concept no longer gets one. Existing junk concepts are not
cleaned up. Status `proposed`: promotion is the author's call.
