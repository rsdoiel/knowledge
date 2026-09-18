---
id: "0027"
title: "Foreign ADRs stay documents; MADR title and density-linking gaps closed"
date: "2026-09-18"
status: accepted
kind: decision
trigger: request
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: ["A colleague's MADR-format ADR, pulled from another repository, is modeled as a document, not a record: we do not own its status and can never promote or supersede it, and a document is exactly the shape for source material we read and summarise rather than cite as a peer", "ParseDocumentFile's markdown branch falls back to a document's first true H1 heading (a single '#') when frontmatter supplies no title:, instead of the caller's filepath.Base -- general, not MADR-specific, but MADR is what surfaced it since it has no frontmatter at all", "Document ingest gains density-based concept linking as a new step in tagSection, alongside the existing [[wikilink]]/keyword linking: a known concept mentioned more than once in a section, outside of inline code spans and fenced code blocks, is auto-linked. TagDensity itself is left counting the unfiltered text -- it is the raw signal the threshold is applied to, not the threshold's own output", "The MADR two-dialects identity/parsing question (TODO.md, filed 2026-09-15) remains open and is not resolved by this record: this closes only the two gaps found in the 'what works today' stopgap, not the format-adapter or scope-collision design question"]
tags: [request, documents, concepts, markdown, madr]
uuid: "01a0b69b-16d7-78db-b148-b64d192e49c1"
origin_host: "wren"
---

**Context.** `TODO.md` filed 2026-09-15, after pulling
`caltechlibrary/CL-Web-Components`, where a colleague records architecture
decisions independently in MADR format (`docs/decisions/NNNN-slug.md`, an
`# N. Title` H1, `- Status:`/`- Date:` bullets, `## Context and Problem
Statement`/`## Decision`/etc., cross-references as relative Markdown links).
`kb ingest` correctly refuses it outright (`no frontmatter`), and
`kb document ingest` already reads it as a stopgap without complaint — no
frontmatter required, it segments on the MADR headings. Two gaps were found
testing that stopgap: with no frontmatter `title:`, the document is titled
with its filename rather than its own `# N. Title` heading; and no concepts
are linked at all, since linking comes only from `[[wikilinks]]` and
frontmatter `keywords`, neither of which MADR has, even though `tag_density`
shows the mentions are there and simply have nowhere to go.

A hand-run prototype (2026-09-15) tested density-based linking directly
against those four ADRs: `MatchConceptNames`'s own whole-word regex, over
`wholeDocumentText`, the same text `tag_density` already scores. It produced
13 links from 95 known concepts and measurably improved retrieval — a query
naming `cmtools`, `documentation`, `tooling` returned ADR-0004 first, where
before it returned nothing from either retrieval pass. But 5 of the 13 were
coincidental, and all five were the same failure shape: a short, common
concept name colliding with a word used in a different sense inside quoted
code or an error string (`format` matching `const format = outputName`;
`git` matching an incidental `git push`), or a one-off mention with no real
topical weight.

**Decision.** A foreign ADR is modeled as a `document`, not a `record`: it
is source material we read and summarise, not a peer we can cite via
`relates_to` and never a status we own or may promote/supersede. This
settles the record-vs-document question the original `TODO.md` item left
open, for this case; it does not resolve the identity-collision or
format-adapter questions the item also raises for a possible future
`kb ingest`-level MADR reader, which stay filed and uncosted.

Two of the stopgap's gaps close: `ParseDocumentFile`'s markdown branch now
falls back to a document's first true H1 heading (`^#\s`, not `^#{1,6}\s`,
so a `##` section heading never qualifies) when frontmatter supplies no
`title:`. This is a general fallback, not MADR-specific — any titleless
Markdown document benefits — but MADR is what surfaced it, since it is the
first ingested format with no frontmatter at all.

Document ingest's `tagSection` gains a second, mechanical linking pass
alongside its existing `[[wikilink]]`/keyword resolution: a new
`densityLinkCandidates`, built on a new library method
`MatchConceptNameCounts` (an occurrence-counting sibling to
`MatchConceptNames`), auto-links a known concept mentioned **more than
once** in a section's text, after stripping fenced code blocks and inline
code spans. Both filters came directly from the prototype's own false
positives: the >1 threshold catches the one-off incidental mention, the
code-span exclusion catches `const format = outputName` and `git push`.
`TagDensity` itself is untouched — it keeps counting the raw, unfiltered
text, since it is the signal the threshold is applied to, not the
threshold's own output; the gap between the two numbers is deliberate, the
same way the original prototype's hand-pruned links versus density counts
already showed it to be. A name already resolved via `[[wikilink]]` or a
frontmatter keyword bypasses the threshold entirely, since that is a
deliberate signal, not an inferred one, and density-linking can only ever
attach to a concept that already exists — it never mints one, unlike a
wikilink.

**Rationale.** Document over record is the safer default precisely because
it is the more conservative claim: nothing about tooling, cross-references,
or status ownership is asserted over a repository we do not control, and
`kb document ingest` already exists to do exactly this job. Choosing record
instead would immediately raise the harder, still-unanswered question the
original item flagged — how `relates_to` behaves when the far side carries
a status we do not own — for no benefit this pass actually needs.

Threshold-plus-code-span-exclusion, rather than the suggestion-review
workflow the original item also costed (reusing document review's
`unsummarized → drafted → reviewed` gate one level down for concept links),
was chosen because it is mechanical and immediate, matching how
`[[wikilink]]`/keyword linking already works, and because the prototype's
own evidence shows the two filters would have caught 4 of its 5 false
positives without any human step at all. The fifth (`validation` matching a
different project's domain entirely) is a cross-topic miss neither filter
catches — accepted as the residual error rate a mechanical pass leaves
behind, not solved here.

**Rejected alternatives.** Unfiltered density linking, as the prototype
first tried it — rejected on its own evidence: 5 of 13 links were noise,
and shipping that noise into every future document ingest, not just a
hand-curated one-time pass, would compound rather than one-time cost.
Threshold only, or code-span exclusion only — each catches a real but
different failure mode found in the prototype; combining them was not
meaningfully more expensive than either alone, so there was no reason to
ship only half. The suggestion-review workflow — the more thorough answer,
explicitly not ruled out for later, but a new review surface (parallel to
document summary review) is a materially bigger change than this pass, and
nothing about the current false-positive rate demands it yet. Extending
`kb ingest` itself to parse MADR as records (a `--format madr` flag, a
`dialect` column, a new `scope` value) — the bigger question the original
item raised and this record does not answer; foreign ADRs stay reachable
today via `kb document ingest`, so there is no forcing function to solve
the harder identity-collision problem in this pass.

**Consequences.** Implementation: `documents.go` gains `firstH1Heading` and
a call to it from `ParseDocumentFile`'s markdown branch;
`retrieval.go` gains `MatchConceptNameCounts`; `cmd/kb/document.go` gains
`stripCodeSpans`, `densityLinkCandidates`, and a call to the latter from
`tagSection`. New tests across `documents_test.go`, `retrieval_test.go`, and
`cmd/kb/document_test.go`, per this project's TDD discipline. `TODO.md`'s
MADR item is corrected to note these two gaps closed, not removed — the
identity/format-adapter question it raises is still open. A further,
broader question raised alongside this one — other programmatic corpus-
improvement techniques as it scales, e.g. term-frequency-based concept
*discovery* (this record's linking only ever attaches to concepts that
already exist) or fuzzy/Levenshtein matching to catch a near-miss a
whole-word match cannot — is filed as its own, separate, uncosted
`TODO.md` item, not folded into this one.
