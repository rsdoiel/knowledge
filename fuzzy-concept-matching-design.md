# `kb document fuzzy-tag` — Fuzzy Concept-Name Matching — Design

**Status (2026-09-21):** Decisions confirmed, revised same day after
further discussion of CLI semantics — `fuzzy-tag` now mirrors `kb document
tag`'s shape and writes (via footnotes), rather than being a report-only
command as first drafted. Not yet implemented — see
`fuzzy-concept-matching-plan.md` (not yet written) for the phased build.

**References:**
- `TODO.md`, "Planned for v0.0.11," item 1 — the item this design resolves.
- `DR-0027` (density-linking threshold, `>1` occurrence + code-span
  exclusion), `DR-0028` (`kb concept suggest`, report-only candidate
  discovery), `DR-0029` (`kb document tag`, explicit wikilink insertion) —
  the three mechanisms this design extends or reuses rather than
  reinventing.
- `DR-0029`'s Rejected alternatives floated a frontmatter-`keywords:`-only
  insertion mode. Reconsidered here (decision 7) and set aside again, for a
  different, more specific reason than DR-0029 gave.
- A companion idea, folding fuzziness into `kb concept suggest`'s *new*-
  concept discovery instead of into matching *known* concepts, is a
  distinct mechanism and stays a separate, unstarted TODO.md item — not
  part of this design.

## Motivation

`MatchConceptNames`/`MatchConceptNameCounts` (`retrieval.go:234,272`), and
`eligibleConcepts` (`cmd/kb/documenttag.go:198`) which `kb document tag`
builds on, are exact whole-word matches. A typo, a plural, or a tense
variant of a known concept's name currently links nothing — TODO.md's
original framing named this alongside "close paraphrase," but a genuine
paraphrase (different wording, same meaning) is a semantic-similarity
problem, not a spelling-distance one, and pulling in embeddings or an LLM
call to solve it repeats a dependency this module has deliberately avoided
elsewhere (DR-0028's Rejected alternatives makes the same call for concept
discovery). **This design scopes "fuzzy" to spelling-level variants only —
typos, plurals, simple tense/suffix variants — not paraphrase.** True
paraphrase stays an open problem, not solved here.

## Decisions

1. **`fuzzy-tag` mirrors `kb document tag`'s shape and writes — it is not
   a report-only command.** Revised from the first draft of this design,
   which made it report-only like `kb concept suggest`. Settled instead on
   consistency with `tag`: same verb pattern (find eligible concepts,
   insert a link), same flags, differing only in matching algorithm
   (fuzzy vs. exact) and insertion mechanism (footnote vs. direct bracket).
   One mental model, two algorithms, rather than a second, differently-
   shaped command to learn.

2. **`kb document fuzzy-tag --project NAME [--concept NAME,...]
   [--dry-run]`** — the exact synopsis `kb document tag` already has
   (`cmd/kb/helptext.go:176`), same meanings: `--concept` forces specific
   known concepts, bypassing the normal distance/threshold gate, exactly as
   it does for exact matching in `tag`; `--dry-run` previews without
   writing. `--json` remains available the same way it is for every verb.
   `fuzzy-tag` never touches what `tag` does or vice versa — a full review
   still runs both, since each only ever handles the matches the other
   can't.

3. **Insertion mechanism: a footnote, not a bracket-wrap.** `insertWikilink`
   (`cmd/kb/documenttag.go:113`) wraps the *literal matched span* — safe
   for `tag`, where the match is always the concept's exact name, but wrong
   here: bracketing a near-miss verbatim, e.g. `[[chunkings]]`, would make
   `ResolveConceptName` mint a new, duplicate concept rather than link the
   existing "chunking," since resolution is exact-match (case-insensitive)
   only. A footnote sidesteps this by construction — the prose word is
   never touched, and the footnote *definition* always carries the
   canonical spelling: `chunkings[^3]` at the match, `[^3]: see [[chunking]]`
   elsewhere. It resolves correctly every time because the bracketed text
   is always the real concept name, never the near-miss.

   **Operates on the raw file, exactly like `tag` — not the database.**
   `kb document tag` never reads `kb.DocumentSections`; it reads the file
   directly (`os.ReadFile`, `cmd/kb/documenttag.go:315`) and inserts into
   *that* text, because documents have no `RenderDocumentFile` — no
   lossless way to turn stored, already-segmented rows back into the
   original file. `fuzzy-tag` writes too now (decision 1), so it inherits
   the same constraint: matching and insertion both happen against the
   file's current raw bytes, never DB-stored section text. This also
   avoids a staleness risk a DB-based approach would have — the file may
   have been hand-edited since the last `kb document ingest`, and working
   from its current bytes directly means that can never matter.

   Concrete mechanics, all computed from the one raw-text string already in
   hand (no second file read, no DB call):
   - **Section boundaries, for reporting and placement, come from a fresh
     regex scan of the raw text**, not from `document_sections` rows: reuse
     `markdownHeadingPattern` (`documents.go:341`,
     `^#{1,6}\s+(.*)$`) — matching how `segmentMarkdown` itself finds
     section boundaries at ingest, just recomputed independently here,
     the same "own copy, not a shared internal" precedent
     `firstH1HeadingPattern` (`cmd/kb/documenttag.go:75`) already sets for
     reusing a `documents.go` pattern from `cmd/kb`.
   - **Marker placement**: `[^n]` immediately after the first safe
     occurrence of the near-miss text, using the same excluded-span rules
     `tag` already has (`excludedSpans`, `cmd/kb/documenttag.go:39` —
     frontmatter, the document's own H1, fenced/inline code, existing
     `[[wikilinks]]`), extended to also exclude any existing footnote
     definition line (`^\[\^\d+\]:.*$`) so a marker can't land inside one.
   - **Definition placement**: appended immediately before the next
     heading after the marker (or at end of file, if the marker falls in
     the last section) — the raw-text equivalent of "end of this section,"
     found the same way `segmentMarkdown` finds section ends, just without
     going through the database.
   - **Label numbering is whole-file, not per-section — trivially, since
     there's only one text to begin with.** Scan the same raw file text
     already in hand for existing `\[\^(\d+)\]` labels and number new
     footnotes starting one past the highest found. Markdown/CommonMark
     footnote labels are scoped to the whole rendered document regardless
     of source structure, so reusing one across two different parts of the
     file would collide in any renderer that collects them.
   - **Idempotency is free, no new mechanism needed.** A footnote
     definition is just raw text containing `[[Concept]]`, so the
     *existing* `alreadyWikilinked` check (`cmd/kb/documenttag.go:83`,
     "does `[[Name]]` appear anywhere in text") already recognizes a
     concept that's been footnoted on an earlier run — no separate
     "already footnoted" check to write. This also means, same as `tag`'s
     own `insertWikilink` (which stops at the first safe match): at most
     one footnote per concept per section, not one per near-miss
     occurrence — a second near-miss spelling of the same concept in the
     same section is already covered once the first is footnoted.

4. **Report content (`--dry-run` output) mirrors `tag`'s own "would
   tag"/"tagged" lines**, with "footnote" in place of "tag" as the verb and
   the near-miss/canonical/distance triple included:
   ```
   would footnote path/to/doc.md [Background]: "chunkings" ~ chunking (distance 1)
   ```
   `--json` extends the existing per-file result shape
   (`documentTagResult`, `cmd/kb/documenttag.go:217`) with a `fuzzy` array:
   `{"section": "Background", "text": "chunkings", "concept": "chunking",
   "distance": 1, "footnote": 3}`.

5. **Matching algorithm** (revised to operate on the raw file text per
   decision 3, otherwise unchanged from the first draft):
   - Scope: for each document's file, for each known concept **not
     already an exact match anywhere in the file's text**
     (`kb.MatchConceptNames(text)` on the same whole-file text `tag`'s own
     `eligibleConcepts` already scans — anything it finds is skipped;
     fuzzy matching is additive, never a duplicate of what exact matching
     already covers). Matches whole-file scope for consistency with `tag`,
     which never scoped its own threshold check tighter than that either.
   - Exclusion: reuse `excludedSpans` (`cmd/kb/documenttag.go:39`) as-is —
     frontmatter, the document's own H1, fenced/inline code, and existing
     `[[wikilink]]` spans are all off-limits to fuzzy matching the same way
     they already are to `tag`'s insertion.
   - Tokenize the remaining text on whitespace/punctuation; for a
     multi-word concept name, compare against the same-length run of
     consecutive tokens.
   - Normalize both the token (or token-run) and the concept name:
     lowercase, then strip **one** trailing suffix from a fixed list —
     `ing`, `es`, `ed`, `s`, tried longest-first — before measuring
     distance. Catches plain plural/tense variants cheaply (`chunking` vs
     `chunkings`, `chunk` vs `chunked`) without a real stemmer dependency;
     deliberately crude, not linguistically complete.
   - Compute Levenshtein distance on the normalized forms. A hit qualifies
     when `0 < distance <= threshold`, where `threshold = 1` for concept
     names of 8 characters or fewer and `2` otherwise — a provisional,
     easily-tuned starting point, not a validated constant. Distance `0`
     after normalization is excluded on purpose: that's a plural/tense
     variant of an *already-linked* word in a different surface form, not a
     new signal.
   - Performance: `O(concepts × tokens)` string-distance computations per
     document. Same acceptance as `MatchConceptNames`'s own doc comment —
     "call frequency here is once per CLI invocation," not a hot loop.

6. **Never rewrites the matched word.** A footnote marker is appended
   immediately after it, but the word's own spelling, casing, and grammar
   are untouched — the "document still needs to read well in English"
   constraint from the motivating conversation holds exactly the same way
   it does for `tag` (DR-0029): fuzzy-tag adds punctuation-like annotation,
   never corrects prose.

7. **Frontmatter `keywords:` insertion — considered again, rejected again,
   for a sharper reason than DR-0029 gave.** Both a footnote and a
   `keywords:` entry are equally safe against the duplicate-concept problem
   (decision 3) — either way the bracketed/listed text is the canonical
   spelling, never the near-miss. What decided it: `keywords:` only
   resolves at the **gist** (whole-document) level
   (`tagSection`'s frontmatter-keywords branch, `cmd/kb/document.go:307`,
   only runs for the gist row), which doesn't match decision 1's goal of
   mirroring `tag`'s per-passage granularity, and it requires `fuzzy-tag`
   to gain a frontmatter-editing capability that doesn't exist anywhere yet
   (no `RenderDocumentFile`, no prior surgical-frontmatter-edit code — see
   `knowledge_dr0026_to_dr0029` memory) for no matching benefit, since the
   footnote mechanism already reuses existing resolution code with zero new
   capability required.

## Deferred, explicitly

- **Paraphrase/semantic-similarity detection** — out of scope per the
  Motivation section; would need embeddings or an LLM call, a dependency
  this module has avoided elsewhere (DR-0028).
- **Fuzzy-aware `kb concept suggest`** — a distinct mechanism (clustering
  *candidate* terms against each other, not matching known concepts
  against document text); filed separately in `TODO.md`, not part of this
  design.
- **A real stemmer** — the suffix-strip list in decision 5 is deliberately
  minimal; not worth a dependency unless it proves too weak in practice.
- **Tuning the distance threshold** — decision 5's `1`/`2` split by name
  length is a starting point, expected to need adjustment once run against
  a real corpus (the same "live-smoke-test before trusting it" pattern
  DR-0027/DR-0029 both needed).
