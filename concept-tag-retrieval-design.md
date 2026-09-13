# Concept-tag-based retrieval — Design

**Status (2026-09-13):** Decisions confirmed. See
[concept-tag-retrieval-plan.md](concept-tag-retrieval-plan.md) for the phased
plan.

**References:**
- `concept-tag-retrieval-feature-request.md` — the filed idea this design
  resolves.
- `wikilink-tagging-design.md`/`wikilink-tagging-plan.md` — shipped
  2026-09-13; this design consumes what it built (`record_concepts`,
  case-insensitive concept resolution) rather than re-deriving it.

## Motivation, and what's changed since filing

Filed 2026-09-08, motivated by harvey's `UnifiedMemory.recallKB`
(`memory_unified.go:276-314` as of filing) being a naive substring match
scoped to the current project only, with a flat 0.5 score and no concept
awareness. The fix proposed there: a concept-name → linked-entities query,
consumed as a cheap, embedder-free first pass ahead of harvey's existing
vector RAG fallback.

One of the feature request's open questions has a different answer today
than it did on 2026-09-08: **"Records, not just observations?"** was left as
"depends on where wikilink-tagging lands its concept↔record links —
possibly a new `record_concepts` table." That table now exists — shipped in
`wikilink-tagging-plan.md` W1. So this design is in scope for both
observations and records from the start, not observations-only.

## What `knowledge` has today, checked directly

- `Concepts()` lists every concept by name — the candidate set to match
  query text against.
- `observation_concepts` and `record_concepts` are the two link tables this
  query reads from. Both are sparse today (`kb link` is the only
  pre-wikilink-tagging way observation↔concept links were made manually),
  but that is a data-density problem, not a design blocker — the query
  works the same whether it finds one link or a hundred.
- No query anywhere goes concept name → linked observations or records;
  only the reverse exists (`ProjectConcepts`, `RecordConcepts`).
- `ResolveConceptName` (from wikilink-tagging) is **not** reusable here —
  it creates a concept on a miss, which is correct for ingest but wrong for
  a read-only retrieval path: a prompt mentioning a word that happens to
  match no concept must resolve to "no match," never to a newly created
  empty concept.

## Decisions

1. **Two separate functions, not one**, matching the feature request's own
   split of "which concepts does this prompt mention" from "what do those
   concepts link to":

   ```go
   func (kb *KnowledgeBase) MatchConceptNames(text string) ([]string, error)
   func (kb *KnowledgeBase) RecallByConceptNames(names []string, limit int) ([]ConceptMatch, error)
   ```

   Keeping them separate means a caller that already knows which concepts
   it cares about (not extracting them from free text) can call
   `RecallByConceptNames` directly — harvey's chunk-analysis paths, not just
   `UnifiedMemory.Recall`, are plausible future callers of just the second
   half.

2. **`MatchConceptNames` does whole-word, case-insensitive matching, not
   substring.** Loads `Concepts()` once, and for each concept name checks
   for a word-boundary match in `text` (case-folded). Substring matching
   alone would let a short concept name like `"RAG"` false-positive inside
   an unrelated word like `"storage"` — worth guarding against explicitly
   since concept names are free-form and can be short. Returns matched
   concept **names** (not ids) — the caller-facing contract stays in the
   same vocabulary as `[[wikilink]]` tags and `kb concept add`.

3. **`RecallByConceptNames` resolves names to concept ids read-only** — a
   `SELECT id FROM concepts WHERE name = ? COLLATE NOCASE` per distinct
   name (deduped case-insensitively so `"Foo"` and `"foo"` in the same call
   don't double-count), skipping any name with no match. This deliberately
   does **not** call `ResolveConceptName` (decision above) — nothing in a
   read path may create a concept as a side effect.

4. **One merged, ranked result across both entity types.** New struct:

   ```go
   type ConceptMatch struct {
       SourceType string    // "observation" or "record" — same vocabulary as kb_fts.source_type
       ID         int64     // internal db id of the observation or record
       ProjectID  int64
       Title      string    // "" for observations; record title for records
       Body       string
       MatchCount int       // distinct queried concepts this entity is linked to
       CreatedAt  time.Time // Observation.CreatedAt, or Record.IngestedAt
   }
   ```

   Two SQL queries (one joining `observation_concepts`, one joining
   `record_concepts`, each grouped by entity id with `COUNT(DISTINCT
   concept_id)`), merged in Go and sorted by `MatchCount` descending, then
   `CreatedAt` descending, then truncated to `limit`. A single Go-side merge
   is simpler than a `UNION` across two differently-shaped SELECTs, and
   keeps each query's SQL readable on its own.

5. **Not project-scoped**, matching the feature request's explicit
   motivation: today's `recallKB` missing cross-project concept matches was
   one of the three named limitations. `RecallByConceptNames` returns
   `ProjectID` on every result so a caller can filter or weight by
   workspace-current-project afterward, but the query itself doesn't
   narrow by project.

6. **Ranking is match-count-then-recency, nothing fancier, for a first
   pass.** The feature request left "ranking beyond match count" open
   (recency, source-project affinity, link count). Recency as the tiebreak
   is enough to make ranking deterministic and useful without inventing a
   scoring formula nothing has validated yet; project-affinity weighting is
   explicitly deferred (decision 8).

7. **`limit` is a caller-supplied parameter, not a hardcoded constant.**
   Answers the feature request's "token budget interaction" question
   directly: harvey's `UnifiedMemory.Recall` already has a budget-aware
   `add()` closure: it decides how many results are worth asking for, this
   function doesn't guess on its behalf. (Today's `recallKB` hardcodes 5 —
   that hardcoding was never a `knowledge`-side decision to begin with.)

8. **No embedder, no new dependency** — pure SQL and string matching,
   consistent with the feature request's stated motivation (Pi-class,
   CPU-only hardware). Vector RAG stays a separate, later fallback in
   harvey; this design does not touch it.

## Deferred, explicitly

- Consuming this from harvey's `UnifiedMemory.Recall` — that's a harvey-side
  change, in a different repository, consumed via the `go.mod replace`
  pending `knowledge`'s own publish (per `CLAUDE.md`). This design and its
  plan cover the `knowledge`-side query API only.
- Project-affinity ranking (workspace-current-project weighted above
  others) — noted in decision 6, real signal but not validated by any real
  usage yet.
- Widening past observations/records to the `documents` entity from
  `narrative-documents-feature-request.md` — that entity doesn't exist yet;
  this query's shape (merge-and-rank across `ConceptMatch` rows) should
  generalize to it later without a redesign, but that's a claim to verify
  when that feature actually gets built, not now.
