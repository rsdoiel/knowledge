# `kb concept suggest` — Fuzzy Candidate Clustering — Design

**Status (2026-09-22):** Decisions confirmed in a full design conversation.
Not yet implemented — plan (`fuzzy-concept-clustering-plan.md`) not yet
written.

**References:**
- `TODO.md`, "Planned for v0.0.11," item 5 — the item this design resolves.
  Raised 2026-09-21 alongside `fuzzy-concept-matching-design.md` (item 1)
  but explicitly kept out of it as a distinct mechanism; folded into
  v0.0.11 as item 5 on 2026-09-22.
- `fuzzy-concept-matching-design.md` (`kb document fuzzy-tag`) — the
  sibling fuzzy feature. That one matches document *prose* against
  *already-known* concepts. This one clusters `concept suggest`'s own
  *candidate* terms *against each other*, before any of them are concepts.
  Decision 1 below shares normalization/distance code with it; nothing
  else is shared, since the two operate on different data shapes (raw file
  text with sections/footnotes vs. an in-memory term-frequency map).
- `DR-0028` (`kb concept suggest` itself, `cmd/kb/concept.go`) — the
  command this design extends in place, not a new verb.

## Motivation

`scoreCandidateTerms` (`cmd/kb/concept.go:184`) scores each single-word
candidate term independently. A concept whose mentions are split across
spelling variants — `chunking` / `chunkings` / `chunked` — is scored as
three separate low-count terms, each of which can individually fall below
the `occ < 2` floor or the `idf <= 0` distinctiveness filter even though
the *combined* mentions clearly signal one real, frequently-discussed
concept. The fix is to merge variant terms into one candidate before
scoring, not after.

## Decisions

1. **Shared primitives with `fuzzy-tag`, not a duplicate implementation.**
   Both features need a Levenshtein distance function and the same
   suffix-strip normalization (`fuzzy-concept-matching-design.md` decision
   5: lowercase, then strip one trailing suffix from `ing`/`es`/`ed`/`s`,
   longest-first). These land in `retrieval.go` as
   `levenshteinDistance(a, b string) int` and
   `stripCommonSuffix(s string) string`, called by both `FuzzyMatchConceptNames`
   (fuzzy-tag) and `clusterCandidateTerms` (this feature, decision 4). One
   normalization rule everywhere; a fix or tuning change applies to both
   features at once.

2. **Clustering runs on the raw occurrence map, before the `occ < 2` /
   `idf <= 0` filter — not on the already-filtered, already-limited
   output.** This is what actually fixes the chunking/chunkings/chunked
   case: a variant that's individually below threshold only survives if
   its count is merged into a cluster *before* the filter decides. Scope
   is the full per-corpus token vocabulary gathered in `scoreCandidateTerms`'s
   existing `occurrences`/`itemCounts` maps (`cmd/kb/concept.go:188-189`),
   not the top `--limit` results.

3. **A token that's fuzzy-close to an already-real concept is excluded
   from candidacy entirely, not clustered.** `concept suggest`'s job is
   discovering *new* concepts; a near-miss spelling of one that already
   exists is `fuzzy-tag`'s job, not this command's. Alongside the existing
   exact `known[tok]` check (`cmd/kb/concept.go:194`), add a fuzzy check:
   normalize the token, compare via `levenshteinDistance` against every
   normalized known concept name, and if `0 < distance <= 1`, drop the
   token from the candidate pool before it ever reaches clustering.

   This exclusion is not silent. Excluded tokens are collected into a
   separate `nearExisting` list (token, matched concept name, distance)
   and reported as a short trailing section — printed only when
   non-empty — so a human reviewing the output can confirm the tool
   suppressed it correctly rather than wondering why an expected candidate
   didn't appear:
   ```
   near-existing (excluded from candidates):
     chunkings  ~  chunking  (distance 1)
   ```
   `--json` gets a parallel `"near_existing"` array alongside
   `"candidates"`.

4. **Clustering algorithm: "star" clustering, bounded by distance-to-seed
   — not chain (single-linkage) clustering.** Plain transitive union-find
   was considered and rejected: it lets A-B-C-D chain together when each
   adjacent pair is within threshold but the endpoints are unrelated (the
   classic cat/cot/cog/dog drift), which risks silently collapsing two
   genuinely distinct new concepts into one canonical label before a human
   ever sees them separately.

   Mechanics, operating on the surviving (post-decision-3) raw token list:
   - Sort tokens by occurrence count descending, term ascending on ties
     (deterministic, same tie-break style `scoreCandidateTerms` already
     uses at `cmd/kb/concept.go:217-221`).
   - Walk the sorted list. The first unassigned token becomes a new
     cluster's seed.
   - Scan all still-unassigned tokens; any token within the distance
     threshold **of the seed** (never of another member) joins this
     cluster. This bounds every member's distance to the seed directly —
     no drift through intermediate members.
   - Repeat from the next unassigned token until none remain.
   - A cluster of size 1 (no variants found) is the common case and
     renders exactly as today.

5. **Threshold: flat `distance <= 1`, normalized-length difference must
   also be `<= 1`** — no length-based 1-or-2 split like fuzzy-tag uses.
   Fuzzy-tag verifies a match against one specific, already-real concept
   name; this clusters unverified candidates against other unverified
   candidates, where a looser threshold risks exactly the unintentional
   collapsing decision 4 is designed to avoid. The length-difference check
   is also the performance pruning step (decision 7).

6. **Canonical spelling = the cluster's seed.** The seed is already chosen
   by descending occurrence count (decision 4), so "most-mentioned form
   wins" and "clustering seed" and "canonical label" are the same value —
   no second pass, no separate tie-break rule to maintain. Considered and
   set aside: preferring the shortest member (more likely the lemma, e.g.
   `chunk` over `chunkings`) or a hybrid of the two. Deferred as a possible
   refinement if occurrence-seeding produces poor canonical labels in
   practice against a real corpus — not blocking a first implementation.

7. **Cluster roll-up: occurrences sum, items(df) unions — not sum.**
   `itemCounts` (`cmd/kb/concept.go:189,198-201`) today tracks a scalar
   count per term. For a cluster, `df` must be the number of *distinct
   items* touched by *any* member, not the sum of each member's
   individual item count — an item containing both "chunking" and
   "chunkings" must count once toward the cluster's `df`, or `idf` comes
   out wrong. This requires `itemCounts` to become a per-term set of item
   indices (`map[string]map[int]bool` or equivalent) rather than a bare
   int, so a cluster's `df` can be computed as the size of the unioned
   set across its members. `occurrences` stays a simple per-member sum —
   double-counting total mentions across variants is correct, that's the
   whole point of merging them.

8. **Output shape extends `candidateTerm`, doesn't replace it.** Add an
   optional `Variants []string` field (`json:"variants,omitempty"`) —
   empty for singleton clusters, so existing single-term output and
   `--json` consumers see no change. Text rendering for a real cluster:
   ```
   12. chunking (+chunkings, chunked)  score=8.40  occurrences=11  items=4
   ```

9. **No CLI flag — always on.** `concept suggest` is already read-only and
   cheap; clustering doesn't change that, and decision 5's tight threshold
   plus decision 10's pruning keep the added cost small. `--project` and
   `--limit` behave exactly as today, applied after clustering.

10. **Performance: length-difference pruning before the Levenshtein DP.**
    `|len(a) - len(b)|` is a lower bound on edit distance, so a
    normalized-length gap greater than the threshold (1, per decision 5)
    can never pass — checked with a single integer comparison before the
    O(length) distance computation runs. No further indexing (bucketing,
    BK-tree) planned for a first pass; validate against the real
    `agents/knowledge.db` the same live-smoke-test way DR-0028/DR-0029
    were validated, and only add more structure if that shows it's
    actually slow.

## Deferred, explicitly

- **Paraphrase/semantic-similarity detection** — out of scope, same
  reasoning as `fuzzy-concept-matching-design.md`'s Motivation: a
  different problem needing embeddings or an LLM call, a dependency this
  module has avoided elsewhere (DR-0028).
- **A real stemmer** — inherits fuzzy-tag's own deferral; the suffix-strip
  list is deliberately minimal.
- **Shortest-variant or hybrid canonical selection** (decision 6) — a
  possible refinement over seed-as-canonical, not adopted now.
- **Tuning the distance threshold** — decision 5's flat `1` is a
  conservative starting point, expected to need validation against a real
  corpus, same as fuzzy-tag's own threshold.
