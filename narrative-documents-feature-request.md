# Narrative/document ingestion, at graduated abstraction levels — Feature Request

> **2026-09-13: Filed.** Captured from a conversation with RSDOIEL about
> enhancing the knowledge base to hold stories, narratives and articles, and
> about using the base to help small models. No design/decide/plan cycle has
> been run yet. This document preserves the idea and the decisions already
> made about its shape — a starting point for that cycle, not a committed
> design.
>
> Decided in the filing conversation:
> - In scope: narratives/stories (Markdown, Fountain, PDF) and articles/posts
>   (Markdown, text, PDF).
> - Embedder-based retrieval is explicitly deferred — this feature is scoped
>   to work without one, on top of the concept-tag graph
>   (`wikilink-tagging-feature-request.md`,
>   `concept-tag-retrieval-feature-request.md`), not as a replacement for
>   either.
> - The central structural idea is **graduated abstraction levels**, not flat
>   chunking: a document is stored/indexed as a gist, one or more
>   section/scene summaries, and full text — not as one blob or as uniform
>   fixed-size chunks.
> - Generating those summaries is **deferred work, not an ingest-time
>   requirement**, and it is tracked, not fire-and-forget: ingest stores full
>   text and segmentation immediately; gist/section summaries are produced
>   later, by a model or a human, and go through a review step before being
>   trusted the way decision records already require authorship and
>   promotion to be separate acts (`proposed` → `accepted`; "a model may
>   write a record but may not accept one" — the same rule applies here to
>   summaries).

## Motivation

The prompting for this came from thinking about how humans use stories.
Immediate recall is limited — comparable to a model's context window — but
humans also have deeper recall, and stories are how a lot of information
survives the trip between the two: a story compresses causal and relational
structure (who did what, why, with what consequence) into a form that is
cheap to recall as a whole and only expanded into detail when the detail is
actually needed. That is a different shape than a flat list of facts, and it
suggests the knowledge base should not treat a story the same way it treats
an observation.

Deferring summary generation raises a second question this document also
answers: how does anyone — human or model — know what still needs doing, and
whether a given piece of deferred work is safe to hand to a model at all? The
model of this is ordinary classroom instruction: a student is given an
assignment, does the work, and then the student and teacher review what was
actually absorbed before it counts as learned. Applied here, "the work" is a
gist or section summary, "the student" can be a model or a human, and the
review step is what decides whether a draft is trustworthy — the same
separation of authorship from acceptance that decision records already
enforce. What classroom instruction adds on top is *triage before the work is
assigned*: a teacher doesn't hand every assignment to the same student
regardless of difficulty. Proposal item 5 below makes that triage concrete —
quantifiable signals (source size, tag density) decide whether a piece of
deferred work is small/simple enough to route to a model, or ought to go to
a human directly.

This is also the direct next step after
[Build a digitally sovereign second brain](https://www.raspberrypi.com/news/build-a-digitally-sovereign-second-brain/)
(Hinchliffe, *Raspberry Pi* magazine, 2026-09-07 — now cited in the database
as source id 13, linked from observation 401 on the `knowledge` project). The
wikilink-tagging feature request that article motivated gives concepts a
cheap, low-friction way to be named from inside prose; this feature request
is about giving that prose somewhere richer to live than an observation
body — full documents, at more than one altitude.

## What `knowledge` assumes today

- **`sources`** is for citations (papers, pages, datasets), not bodies of
  text to search — DR-0013 deliberately left it out of `kb_fts` (see
  `knowledge.go:1214-1216`), reasoning that what a source's searchable text
  should be is its own decision. A narrative or article is exactly the case
  DR-0013 postponed: unlike a citation, its whole point *is* to be searched.
  That makes `sources` the wrong table to extend and argues for a new entity
  rather than loosening DR-0013's boundary.
- **`kb_fts`** has four writers today (`observation`, `project`, `concept`,
  `record`), each a flat `(body, kind, label, descr, source_type, source_id,
  project_id)` row, delete-then-reinsert on every write, with a matching
  block required in `rebuildFTSIfNeeded` and in `Search`'s display-
  reconstruction `CASE` (see `knowledge-structures-2026-09-13.md` for the
  full mechanics). Nothing today models one logical document as *multiple*
  indexed rows at different granularities — every existing `source_type` is
  one row per entity.
- **Fountain parsing already exists** in harvey (`harvey/FOUNTAIN_FORMAT.md`)
  — scene headings (`INT./EXT.`) give a narrative document free,
  author-supplied structural boundaries that an article or a PDF doesn't
  have.
- **No PDF text extraction exists anywhere in the workspace yet.** This is a
  real gap, not an oversight to design around: PDF-format documents can't be
  ingested until something (a Go library, or shelling out to
  `pdftotext`/poppler) does that extraction.
- **Concept tagging of free text** is exactly the mechanism
  `wikilink-tagging-feature-request.md` proposes, but scoped there to
  `kb ingest` record bodies only. Documents are a second, independent
  consumer of the same `[[Name]] → AddConcept` mechanism, not something that
  feature request already covers.

## What is already fine

- **`AddConcept`/`AddConceptWithIdentifier`** find-or-create by name — same
  reuse point the wikilink feature request already identified. A document's
  gist or section summaries can be scanned for `[[Name]]` the same way a
  record body would be.
- **The FTS four-writer pattern is a known checklist**, not something to
  invent: pick a `source_type` literal, add a live insert/delete pair, match
  it in `rebuildFTSIfNeeded`, extend `Search`'s reconstruction `CASE`. Adding
  multiple rows per document (one per abstraction level) is a variation on
  this pattern, not a departure from it — each level's row just carries the
  same `source_id` with a different `kind` or a level marker.
- **`concept-tag-retrieval-feature-request.md`'s proposed query**
  (`ObservationsByConceptNames`) generalizes to documents with the same
  shape: concept names in, linked entities out. A `DocumentsByConceptNames`
  (or a widened, type-spanning version of the same query) is additive, not a
  redesign.

## Proposal

1. A new entity — working name `documents` — carrying at minimum: `title`,
   `format` (`markdown` / `fountain` / `pdf` / `text`), a path or reference to
   the source file, and project scope (same shape as `records`' project
   linkage).
2. Each document is represented at **graduated abstraction levels**, not as
   one body:
   - **Gist** — one or two sentences, the level cheap enough to always be a
     candidate for concept-tag retrieval, analogous to immediate recall.
   - **Section/scene summaries** — one per structural unit. Fountain's scene
     headings give this boundary for free; Markdown/article headings
     (`#`/`##`) give an equivalent boundary; PDF has neither cleanly and
     needs its own segmentation strategy, an open question below.
   - **Full text** — the deep layer, retrieved only once a higher level has
     already matched.

   Each level is `[[wikilink]]`-taggable the same way a record body would be,
   so a search or a concept-tag-retrieval call can be told which altitude it
   is asking for.
3. Segmentation and summary generation are per-format concerns, not one
   generic chunker: Fountain uses scene headings, Markdown/articles use
   heading levels, PDF needs a segmentation strategy decided separately once
   extraction exists.
4. Retrieval-side: a small model's cheap first pass stays concept-tag
   matching against gists (near-zero token cost); descending to a section
   summary or full text is a deliberate second call once the gist-level match
   indicates it's worth the tokens. No embedder is required for this to work
   at all — semantic RAG over the full-text level remains available later as
   a further fallback, per the existing `concept-tag-retrieval-feature-request.md`
   sequencing (tag-match first, RAG-embedder as fallback), not as something
   this feature request needs to build.
5. **Deferral and review, not ingest-time generation.** Ingest stores a
   document's full text and segmentation and stops there — it does not block
   on writing a gist or section summaries. Each level that doesn't have one
   yet carries a status:
   - **`unsummarized`** — segmented, no summary attempted.
   - **`drafted`** — a model or a human has written one; not yet trusted for
     retrieval beyond low-stakes use.
   - **`reviewed`** — a human has confirmed it. Only a human promotes to this
     state, mirroring decision records' rule that authorship and acceptance
     are separate acts.

   Each `unsummarized` item carries **quantifiable signals alongside the
   content**, so triage doesn't require reading the content first: source
   size (word/token count of the segment), a concept-tag density (how many
   `[[wikilink]]`/gazetteer-matched concepts the segment already carries —
   mechanical, no model needed to compute), and, once drafted, a confidence
   value (0.0-1.0, same scale as harvey's memory confidence scores, for
   consistency across the workspace) recording how much the drafting model
   or human trusts its own output.

   A queue (`kb document review list`, or similar) surfaces `unsummarized`
   and `drafted` items ordered/filterable by those signals, so a human can
   decide two separate things at two separate points: **before** drafting,
   whether an item is small/simple enough to route to a model versus needing
   a human to write it directly (the size and tag-density signals inform
   this); and **after** a draft exists, whether to promote it to `reviewed`,
   send it back, or rewrite it by hand. Nothing here requires build-time
   automation of the triage decision itself for a first pass — the queue
   surfaces the signals, a human still decides.

## Open questions for the design cycle

- **Schema shape for the levels.** One `documents` row plus a child table
  (`document_sections`, one row per level per document) that can each carry
  independent concept tags and FTS rows? Or a single row per document with
  fixed gist/summary/body columns, trading tagging granularity for schema
  simplicity? The four-writer FTS pattern assumes one row per entity today —
  multiple levels per document is the first case that doesn't fit that
  assumption cleanly.
- **Who ends up drafting a given summary — model or human — is decided by
  the triage step above, not fixed in advance**; hand-authored text (a
  logline is normal authorial practice) always takes precedence over a
  generated one when both exist. What's still open: which model does the
  drafting when one is used (local/small at ingest time, or something bigger
  invoked out-of-band — see the read/write split discussed in the filing
  conversation: the small model that runs at *retrieval* time should never
  be the one generating summaries at ingest time), and where that model call
  lives (a `kb` hook that shells out, or a separate harvey-driven job that
  writes back through `kb`).
- **Exact shape of the confidence value.** Self-reported by the drafting
  model, or computed from a heuristic (e.g. how much of the segment's own
  concept tags actually appear reflected in the summary text)? Self-reported
  confidence from a small model may not be trustworthy on its own — worth
  deciding whether it's a signal alongside the mechanical ones (size, tag
  density) rather than the sole basis for triage.
- **Whether any auto-promotion path should exist at all**, e.g. a
  high-confidence, low-complexity draft skipping straight to `reviewed`
  without a human touching it. Decided for a first pass: no — every
  promotion to `reviewed` is a human action, full stop. Worth revisiting
  only once there's enough real queue volume to know whether that's a
  bottleneck.
- **PDF extraction tooling.** Nothing in the workspace does this today;
  needs a decision (Go library vs. shelling out to `pdftotext`) before any
  PDF-format document can be ingested, independent of everything else in
  this proposal.
- **Does full text get indexed in `kb_fts` at all**, or does deep-level
  retrieval work by reference back to the source file (mirroring how
  `sources` deliberately stays unindexed per DR-0013), with only gist and
  section levels living in the searchable index? This is the same boundary
  DR-0013 drew for citations, being asked again for documents.
- **Relationship to `concept-tag-retrieval-feature-request.md`'s query.**
  Does `ObservationsByConceptNames` widen to cover documents (returning
  gist-level rows by default, with an explicit deeper call for more), or does
  this feature request need its own query? Depends on how the existing
  feature request's open "records too?" question gets resolved.
- **Depends on `wikilink-tagging-feature-request.md` landing first**, at
  least for the tagging mechanism, even though this proposal's consumer
  (documents, not records) is different from that request's first-pass scope
  (`kb ingest` record bodies only).

## Related

- Filing conversation, 2026-09-13 (Laboratory root).
- `wikilink-tagging-feature-request.md` — the tagging mechanism this reuses.
- `concept-tag-retrieval-feature-request.md` — the retrieval query this
  generalizes to documents; also the doc that first raised small-model
  context-budget motivation.
- `agents-projects-layout-feature-request.md` — the template this document
  follows.
- Raspberry Pi magazine: [Build a digitally sovereign second brain](https://www.raspberrypi.com/news/build-a-digitally-sovereign-second-brain/)
  (source id 13 in `agents/knowledge.db`, linked from observation 401,
  project `knowledge`).
- `knowledge-structures-2026-09-13.md` (Laboratory root, untracked) — the
  FTS four-writer mechanics this proposal's schema questions build on.
