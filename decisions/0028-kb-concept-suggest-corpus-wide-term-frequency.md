---
id: "0028"
title: "kb concept suggest: corpus-wide term-frequency candidate discovery"
date: "2026-09-18"
status: accepted
kind: decision
trigger: request
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0027"]
initiative: ""
session: ""
decisions: ["kb concept suggest [--project NAME] [--limit N] is a new, read-only verb that scans every record body and document section body and prints candidate new concepts, ranked by corpus-wide distinctiveness (a TF-IDF shape: occurs several times, confined to relatively few items) rather than raw frequency", "A candidate must occur more than once anywhere in the scanned corpus and must not be ubiquitous (idf collapses to zero when a term appears in every item) -- the same >1 floor DR-0027's density-linking uses, for the same reason", "Code spans/fenced code blocks are stripped before matching (reusing stripCodeSpans from document ingest), a small built-in English stopword list is excluded outright, an already-existing concept name is never suggested again, and a bare record reference (dr-0013, adr-0004) is filtered as a distinct, non-concept token shape", "It never writes to the database -- a suggestion becomes a real concept only when a human runs kb concept add, the same mechanical-signal-then-human-curates pattern DR-0027's density-linking and document summary review already use"]
tags: [request, concepts, records, documents, retrieval, term-frequency]
uuid: "01a0b6a6-2832-747f-bba9-a6f2bb57e6c2"
origin_host: "wren"
---

**Context.** Filed in `TODO.md` 2026-09-18, prompted directly by a question
about DR-0027's density-linking: `MatchConceptNames`/`MatchConceptNameCounts`
can only ever match text against concepts that already exist — they have no
way to propose a *new* one. As the corpus grows past what a human curating
by hand keeps up with, that is a real gap: two candidate techniques were
raised, neither costed — corpus-wide term-frequency/distinctiveness scoring
to surface candidate concepts, and Levenshtein/fuzzy matching of existing
concept names against prose to catch a near-miss an exact whole-word match
cannot. This record scopes and ships the first; the second stays filed,
uncosted, in `TODO.md`.

**Decision.** A new read-only verb, `kb concept suggest [--project NAME]
[--limit N]`, scans every record body and every document section body
(`Level == "section"`, not the gist's curated summary), optionally scoped to
one project, and prints candidate new concepts ranked by corpus-wide
distinctiveness. A candidate term is single-word, lowercase, at least three
characters, and:

- occurs **more than once** anywhere in the scanned corpus — the same `>1`
  floor DR-0027's density-linking uses, for the same reason: one incidental
  mention is too weak a signal alone;
- is **not ubiquitous** — its idf (`log(N / df)`, `N` total items scanned,
  `df` items containing it at least once) must be positive, so a term
  present in nearly every item (not distinctive, just common) scores at or
  below zero and is dropped, regardless of how many times it is repeated;
- is not inside a code span or fenced code block (`stripCodeSpans`, reused
  as-is from document ingest);
- is not already the name of an existing concept (case-insensitive);
- is not a small built-in English stopword;
- is not shaped like a bare record reference (`^[a-z]+-[0-9]+$`, e.g.
  `dr-0013`, `adr-0004`) — a real statistical signal, but never a usable
  concept name.

Score is `occurrences × idf`. Output is a plain ranked list (or `--json`);
`--limit` caps how many print (default 20). The command never writes to the
database — a suggestion becomes a real concept only when a human runs
`kb concept add`.

**Rationale.** TF-IDF-shaped scoring, rather than raw frequency, is what
makes "distinctive" mean something: a term repeated in almost every
record/document (`name`, `row`, `test` — all found live, see Consequences)
is exactly as frequent as a genuinely topical one, and only the *rarity
across the corpus* half of the formula tells them apart. The `>1` floor and
code-span exclusion are lifted directly from DR-0027's density-linking,
which costed the identical trade-off for a related problem (linking to
known concepts) and found the combination caught the false positives a
hand-run prototype actually hit; reusing the same filters here rather than
re-deriving new ones from scratch keeps the two passes' behavior legible
together. Never writing to the database is the same conservative default
DR-0027 chose for density-linking and document review already established:
a mechanical pass proposes, a human commits — especially apt here, since
this pass, unlike density-linking, has no existing-concept anchor to check
itself against at all; every suggestion is a guess about what should exist.

**Rejected alternatives.** Embeddings or an LLM call for keyword extraction
— raised as the alternative when this was first discussed; rejected for now
because it pulls in exactly the kind of dependency (a vector store, network
calls, nondeterminism) this module has deliberately avoided everywhere else,
not because it would necessarily do worse. Worth reconsidering explicitly if
the statistical pass's signal-to-noise ratio proves too weak in practice —
not assumed as the starting point. Multi-word phrase extraction (n-grams) —
scoped out of v1 as combinatorially bigger for uncertain benefit; many good
concept names in this corpus already are single words (`chunking`,
`workspace`, `collision`), and the single-word pass is what TODO.md's own
wording ("term frequency analysis") named directly. Auto-creating concepts
above some score threshold instead of only suggesting — rejected outright,
the same reasoning as DR-0027: a mechanical pass proposing a *brand-new*
entity with no human confirmation at all is a materially bigger claim than
auto-linking to something that already exists.

**Consequences.** Implementation: `documents.go` already exposed
`Documents(projectID)`, reused as-is rather than duplicated. `cmd/kb/concept.go`
gains `candidateTerm`, `candidateTermPattern`, `recordIDShapedPattern`,
`suggestStopwords`, `scoreCandidateTerms` (the pure, DB-free scoring core),
and `cmdConceptSuggest`; dispatched from `cmdConcept` alongside
add/list/rename. New tests in `cmd/kb/concept_test.go`, per this project's
TDD discipline, including a corpus-scale regression: the pure scoring
function is tested directly, without a database, so its filtering logic is
cheap to exercise exhaustively. Live-smoke-tested against the real
`agents/knowledge.db`: the unfiltered pass surfaced genuinely useful
candidates (`rename`, `harvey`, `divergence`, `collision`, `uuid`) alongside
generic corpus-wide nouns (`row`, `name`, `table`, `test`) and one
record-reference token (`dr-0013`, since fixed by the shape filter above) —
confirming this is a real, `~50%`-signal mechanical pass in the same
character as DR-0027's own live-tested prototype, not a finished curation
tool. `kb-concept(1)` regenerated. `TODO.md`'s programmatic-corpus-
improvement item is corrected to note this half shipped; the Levenshtein/
fuzzy-matching half stays open and uncosted.
