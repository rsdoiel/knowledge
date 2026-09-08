# Concept-tag-based retrieval — Feature Request

> **2026-09-08: Filed.** Captured from a conversation with RSDOIEL about
> small-model context budgets in harvey. No design/decide/plan cycle has been
> run yet. This document preserves the idea and the decisions already made
> about its shape — a starting point for that cycle, not a committed design.
>
> Decided in the filing conversation:
> - Lives in **`knowledge`, as a query API** — harvey's `UnifiedMemory` calls
>   it, rather than harvey growing its own concept-matching logic.
> - Targets **general per-prompt context injection**
>   (`UnifiedMemory.Recall`), not specifically the `/read-chunks` map-reduce
>   path — though nothing here rules that path out later.
> - **Supplements, not replaces,** the existing vector RAG store: cheap
>   tag-match runs first; RAG (when enabled and an embedder is configured)
>   stays available as the semantic-similarity fallback.

## Motivation

harvey's small-model work (see `TODO.md`'s chunk-timing benchmark entry) has
spent real effort making large-document analysis survive constrained,
CPU-only hardware (GPULayers default fix, per-model timing, chunking guard
fixes). Runtime KB retrieval hasn't had the same attention — see "what harvey
assumes today" below. A concept/tag-based lookup is attractive specifically
*because* the hardware is constrained: it needs no embedder, no vector index,
and no extra model call — just a name match against rows `knowledge` already
stores, which is cheap enough to run on a Pi on every prompt.

This also gives the `[[wikilink]]` tagging feature
(`wikilink-tagging-feature-request.md`) a consumer. That document proposes
*writing* denser concept links; this one proposes *reading* them back out
selectively. Related but independent — this is useful today against however
sparse the current `observation_concepts`/`project_concepts` links are, and
gets more useful as tagging density grows, but doesn't require wikilinks to
ship first.

## What harvey assumes today

`memory_unified.go:276-314` (`UnifiedMemory.recallKB`) is the entire current
KB retrieval path:

```go
obs, err := kb.Observations(u.cfg.CurrentProjectID)
...
for _, o := range obs {
    if qLower != "" && !strings.Contains(strings.ToLower(o.Body), qLower) {
        continue
    }
    out = append(out, UnifiedResult{Source: "kb", ..., Score: 0.5})
    if len(out) >= 5 {
        break
    }
}
```

Three limitations this feature request addresses:
- **Substring match on the query as typed**, not on concepts — a query about
  "chunking" won't match an observation that says "map-reduce" even if both
  are tagged with the same concept.
- **Scoped to `CurrentProjectID` only** — an observation from a different
  project tagged with a relevant concept is invisible, even though
  `observation_concepts` is not project-scoped.
- **Fixed score of 0.5, first-5-found** — no ranking signal at all; order is
  whatever `kb.Observations` returns.

`knowledge` itself has no query in its public API today that goes
concept → linked observations/records; only the reverse direction exists
(`ProjectConcepts(projectID)`).

## What is already fine

- **`Concepts()`** (`knowledge.go:960`) already lists every concept by name —
  the candidate set to match query text against.
- **`observation_concepts`/`project_concepts`** already express the
  many-to-many links this would read from; nothing new to store on the
  concept side.
- **`kb link`** (`cmd/kb/link.go`) is the existing way observation↔concept
  links get made today — manually. Sparse, but real data to retrieve
  against now, not a hypothetical.

## Proposal

1. `knowledge` grows a query, roughly:
   `ObservationsByConceptNames(names []string) ([]Observation, error)` —
   given concept names already known to match, return linked observations
   (and, once the schema question below is settled, records) ordered by
   concept-match count, not project-scoped.
2. Query-text-to-concept-name matching (the "which concepts does this prompt
   mention" step) is a separate, simpler piece: substring/case-insensitive
   match of the prompt against `Concepts()` names. Whether this lives in
   `knowledge` (as a helper) or in harvey (calling `Concepts()` itself and
   doing the match locally) is an open question below.
3. harvey's `recallKB` calls the new query as a first pass; the existing
   substring-over-project-observations behavior becomes the fallback when no
   concept name matches, preserving today's behavior rather than regressing
   it for KB content that's never been tagged.
4. No embedder, no new dependency — this stays a pure SQL/string-match path,
   consistent with why it's worth building for small/CPU-only models.

## Open questions for the design cycle

- **Concept-name matching belongs where?** In `knowledge` (so any consumer
  gets the same matching logic) or in harvey (keeps `knowledge`'s API surface
  to pure data queries)? Precedent from the wikilink feature request leans
  toward `knowledge` owning name resolution generally.
- **Ranking beyond match count.** Recency (`created_at`), source project
  matching harvey's current workspace, or concept-link count — not decided.
- **Records, not just observations.** This depends on where
  `wikilink-tagging-feature-request.md` lands its concept↔record links (open
  question there too — possibly a new `record_concepts` table). If that
  table doesn't exist yet, this feature request's first pass is
  observations-only.
- **Token budget interaction.** `UnifiedMemory.Recall`'s existing budget-aware
  `add()` closure should keep working unchanged — confirm the new source
  doesn't need special-casing there.

## Related

- Filing conversation, 2026-09-08 (Laboratory root).
- `wikilink-tagging-feature-request.md` — writes the concept links this reads.
- `agents-projects-layout-feature-request.md` — the template both of these
  documents follow.
- harvey's `TODO.md`, chunk-timing benchmark entry — the small-model
  CPU-only-hardware context this is motivated by.
