# Narrative/document ingestion, graduated abstraction levels — Design

**Status (2026-09-13):** Decisions confirmed. See
[narrative-documents-plan.md](narrative-documents-plan.md) for the phased
plan.

**References:**
- `narrative-documents-feature-request.md` — the filed idea this design
  resolves; every open question it left is answered below.
- `wikilink-tagging-design.md`, `concept-tag-retrieval-design.md` — both
  shipped 2026-09-13; this design is the first real test of whether their
  shape actually generalizes, which the concept-tag-retrieval design
  predicted but didn't verify.

## What's different about this one

The previous two designs each extended one existing thing (a join table, a
query). This one adds a new entity with real internal structure — a
document isn't a single row the way an observation or a record is, it's a
document plus a sequence of sections plus, for each of those, a summary that
may not exist yet. Getting the shape of that right matters more than usual,
because every other decision here (tagging, retrieval, review) sits on top
of it.

## Decisions

### 1. Schema: two tables, not one — `documents` and `document_sections`

```sql
CREATE TABLE documents (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id     INTEGER REFERENCES projects(id) ON DELETE SET NULL,
    title          TEXT NOT NULL,
    format         TEXT NOT NULL DEFAULT '',   -- markdown | fountain | text | pdf
    path           TEXT NOT NULL,
    author         TEXT NOT NULL DEFAULT '',   -- from frontmatter `author`, if present (decision 3)
    published_date TEXT NOT NULL DEFAULT '',   -- from frontmatter `pubDate`/`datePublished`/`dateCreated` (decision 3)
    checksum       TEXT NOT NULL DEFAULT '',
    ingested_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    uuid           TEXT NOT NULL DEFAULT '',
    origin_host    TEXT NOT NULL DEFAULT ''
);

CREATE TABLE document_sections (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id    INTEGER REFERENCES documents(id) ON DELETE CASCADE,
    level          TEXT NOT NULL,             -- 'gist' | 'section'
    seq            INTEGER NOT NULL DEFAULT 0,
    heading        TEXT NOT NULL DEFAULT '',  -- scene/markdown heading; '' for gist
    body           TEXT NOT NULL DEFAULT '',  -- raw source text; always '' for 'gist'
    summary_body   TEXT NOT NULL DEFAULT '',
    summary_status TEXT NOT NULL DEFAULT 'unsummarized', -- unsummarized|drafted|reviewed
    summary_stale  INTEGER NOT NULL DEFAULT 0,  -- set true when body changes under an existing summary (decision 10)
    source_size    INTEGER NOT NULL DEFAULT 0,  -- word count
    tag_density    INTEGER NOT NULL DEFAULT 0,  -- MatchConceptNames() count
    confidence     REAL,                        -- NULL until drafted
    generated_by   TEXT NOT NULL DEFAULT '',    -- 'human' or a model identifier
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    uuid           TEXT NOT NULL DEFAULT '',
    origin_host    TEXT NOT NULL DEFAULT ''
);
```

Every document gets exactly one `level='gist'` row (the document-level
summary target) plus one `level='section'` row per structural unit,
ordered by `seq`. This makes the gist and every section summary **the same
kind of row** — one status/size/density/confidence machinery, one review
queue query, no special-casing gist vs. section anywhere in code. The
asymmetry is real but contained to one column: a `'section'` row's `body`
is populated at ingest (it's the actual source text); a `'gist'` row's
`body` stays empty forever, because a whole-document gist has no "raw"
form of its own — it is always generated, never mechanically extracted.

This resolves the feature request's schema open question in favor of the
child-table option: a single row per document with fixed columns couldn't
hold a variable number of sections, and would need the gist/section
distinction baked into column names instead of data.

### 2. `documents` scope: Markdown, Fountain, and plain text. Not PDF, yet.

PDF extraction has no home anywhere in this workspace — building it out is
an unrelated yak to shave before a single PDF document could ever be
ingested. `format` reserves `'pdf'` as a documented, valid value so the
column doesn't need a migration later, but `kb document ingest` rejects a
`.pdf` file outright with an explicit "not yet supported" error rather than
pretending to handle it. Markdown, Fountain, and plain text need no new
dependency and cover the stated scope (stories, articles, posts) well
enough to ship.

### 3. Frontmatter is optional, but recognized and used when present

Revised after review: a short story or Fountain screenplay usually has no
frontmatter at all, and requiring an author to add YAML fences just to make
prose ingestible would be the wrong ergonomics — but a blog post is a
different case. It very often already **has** frontmatter, written for an
entirely different purpose (feeding a static site generator), and throwing
that away at ingest and asking for `--title` by hand would be discarding
real, already-authored metadata for no reason.

So: `kb document ingest` tries `splitFrontmatter` (the same helper
`recordfile.go` uses) first. Unlike a record, a document's frontmatter is
never required — `splitFrontmatter`'s "does not start with `---`" error is
the normal, expected case for Fountain and plain text, not a problem;
"starts with `---` but the closing fence is missing" is a warning (matching
records' non-fatal-warning philosophy), not a failure, and ingestion falls
back to treating the whole file as body either way.

Rather than inventing a generic frontmatter schema (most static-site
generators — Jekyll, Hugo — use their own field names, none of them
authoritative here), the recognized fields are **antennaApp's own
documented vocabulary** (`helptext.go`'s `post` command: "Required front
matter fields... Recommended additional fields"), since it's the one
already producing real posts in this workspace, not a guess at a convention
nothing here actually follows:

| Frontmatter field | Used for |
|---|---|
| `title` | `documents.title` (a `--title` flag still overrides) |
| `description` | seeds the **gist**: `summary_body`, `summary_status = 'drafted'`, `generated_by = 'human'` — antennaApp's own doc calls this "summary for RSS and search engines," which is exactly what a gist is, already written by a human. It still needs an explicit `kb document review promote` before it's trusted for search (decision 7) — the fact that a human wrote it into frontmatter doesn't get it a special exemption from the same review step everything else goes through. |
| `pubDate` (or `datePublished`/`dateCreated`, antennaApp's own aliases) | new `documents.published_date TEXT NOT NULL DEFAULT ''` |
| `author` | new `documents.author TEXT NOT NULL DEFAULT ''` |
| `keywords` (antennaApp's tag field — not `tags`) | resolved via `ResolveConceptName`, linked to the gist row exactly like a record's `Tags` frontmatter (decision 5) |

Every field is optional and unrecognized keys are ignored — this is a loose,
tolerant parse of someone else's schema, not the strict required-field
validation `recordfile.go` enforces on its own format. Format itself
(decision 2) is still inferred from extension or `--format`, never from
frontmatter — a blog post's frontmatter doesn't get to claim it's Fountain.

### 4. Segmentation is per-format; Fountain uses `github.com/rsdoiel/fountain` directly

Revised after review — this was wrong in the first draft. I'd assumed
harvey's Fountain handling was harvey's own code, making a shared
dependency backwards (harvey depends on `knowledge`, not the reverse), and
proposed `knowledge` hand-roll a minimal scene-heading regex to avoid that.
Checked instead: harvey's Fountain parsing is **not** harvey's own code —
`replay.go` and `recorder.go` both import `github.com/rsdoiel/fountain`
(currently pinned at v1.0.2) and call `fountain.ParseFile`/
`fountain.SceneHeadingType` directly. `fountain` is already a third,
independent module both `harvey` and `knowledge` can depend on without
either depending on the other — exactly the shape needed, already sitting
in `~/Laboratory/fountain`, no invention required.

So: `knowledge` adds `github.com/rsdoiel/fountain` as a real dependency.
`fountain.Parse`/`fountain.ParseFile` returns `*Fountain{TitlePage
[]*Element, Elements []*Element}`; a new section starts at each
`Element.Type == fountain.SceneHeadingType`, with the section body being
that element's `Content` plus everything up to the next scene heading.
This is the actual parser every other Fountain consumer in this workspace
already trusts, not a second, weaker implementation of the same idea — no
enhancement to the `fountain` module itself is needed for this; its
existing API already covers what segmentation requires.

One more thing this gets for free: `Fountain.TitlePage` is a parsed
key/value list (`Title:`, `Author:`, `Credit:`, etc. — Fountain's own
title-page convention, distinct from YAML frontmatter but the same idea).
Decision 3's frontmatter-mapping table applies here too: a Fountain
document's `Title`/`Author` title-page fields map to `documents.title`/
`documents.author` the same way a Markdown post's frontmatter does.

Markdown uses `^#{1,6}\s` heading lines for section boundaries (no library
needed — that's a one-line regex, not a parser to depend on). Plain text
has no internal structure to find: the whole file is one `'section'` row,
seq 0 — this is the case that proves the schema degrades correctly to "one
section" rather than needing special handling.

### 5. Concept tagging reuses wikilink-tagging's mechanism exactly, on section bodies

At ingest, each `'section'` row's raw `body` is scanned for `[[Name]]`
wikilinks, resolved via `ResolveConceptName` (case-insensitive, same as
records), and linked into a new `document_section_concepts` table (same
shape as `record_concepts`). The `'gist'` row is tagged too, but against
the **whole document's concatenated raw text** at ingest time — this makes
a document findable by tag immediately, without waiting for anyone to draft
a gist summary, the same way a record is taggable the moment it's ingested.
Frontmatter `keywords` (decision 3), when present, resolve into the same
gist-row links, exactly the same unification records already do for their
`Tags` field.

### 6. `tag_density` is computed with `MatchConceptNames`, not a new gazetteer

`tag_density` (the triage signal from the feature request) is exactly what
`MatchConceptNames` (shipped in `concept-tag-retrieval-plan.md`) already
computes: a count of known concept names appearing in text, whole-word,
case-insensitive. Running it against a section's raw body at ingest time is
the entire implementation — no new matching logic. This is the strongest
sign the previous two designs' shapes actually hold up under a real second
consumer, not just the concept-tag-retrieval design's own hopeful claim
that they would.

Note the distinction from decision 5: `MatchConceptNames` only **counts**
matches against existing concepts, it never creates one — a document
mentioning a concept nobody has named yet contributes to `tag_density` only
via an explicit `[[Name]]` wikilink (which does create), never via
incidental prose that happens to repeat an existing concept's name without
being marked up.

### 7. Review queue and workflow: `document_sections` is the whole review queue, no separate table

Because gist and section rows share one shape (decision 1), the review
queue is one query: `document_sections` filtered by `summary_status`,
ordered/filterable by `source_size`, `tag_density`, `confidence`. Three new
commands, deliberately minimal:

- `kb document review list [--project P] [--status S]` — the triage queue,
  printing size/density/confidence alongside each unsummarized or drafted
  row so a human can decide, per the feature request's classroom framing,
  whether a given row is small/simple enough to hand to a model or ought to
  be written by hand.
- `kb document draft SECTION_ID BODY --by WHO [--confidence N]` — writes
  `summary_body`, sets `summary_status = 'drafted'`, records `generated_by`
  (`WHO` is free text: `"human"` or a model identifier) and `confidence` if
  given. `kb` does not call any model itself here — `WHO` and `BODY` are
  supplied by whatever already decided to draft this (a human typing, or a
  harvey job piping in a model's output). This is the same boundary
  `concept-tag-retrieval-design.md` drew around harvey consumption: a pure
  data layer, not an LLM caller.
- `kb document review promote SECTION_ID` — human-only, sets
  `summary_status = 'reviewed'`. This is also the point where the summary
  becomes visible to search and retrieval (decisions 8 and 9) — nothing
  unreviewed is ever surfaced as trustworthy content, only findable by tag
  (decision 5's links are independent of review status, since a wikilink
  tag is author-asserted, not generated, and needs no review).

No auto-promotion path exists, confirming the feature request's own
first-pass decision unchanged.

### 8. `kb_fts` indexing: reviewed summaries only, raw section bodies never

New `source_type` literal `'document_summary'`, written/deleted the same
delete-then-reinsert way every other writer works, but **only when
`summary_status = 'reviewed'`** — indexed on promotion, removed from the
index if a summary is ever un-reviewed (the mechanism supports this even
though nothing in this design adds a command to un-review one). Raw
`document_sections.body` is never indexed, full stop — mirrors DR-0013's
`sources` boundary exactly: a section's full text is reference-only,
fetched by reading `documents.path` if truly needed, never searched
directly. This is the actual point of graduated abstraction: what's cheaply
searchable is the dense, reviewed summary, not the underlying prose.

`documents` itself gets a thin `'document'` FTS entry (title only, indexed
at ingest) — mirrors how `projects` are indexed by name, so a document is
findable by title even before anything about it is summarized.

### 9. `RecallByConceptNames` widens to a third source, not a parallel query

`retrieval.go`'s merge-in-Go pattern (one query per entity type, sorted
together in Go) extends cleanly: a third query joins
`document_section_concepts`, and its rows merge into the same ranked
`[]ConceptMatch` slice concept-tag-retrieval already returns. This is the
generalization `concept-tag-retrieval-design.md` predicted but explicitly
didn't build ("a claim to verify when that feature actually gets built,
not now") — now verified, by building it.

`ConceptMatch` gains one field: `SummaryStatus string` (empty for
observation/record matches; `"unsummarized"`/`"drafted"`/`"reviewed"` for a
document match). `Body` carries the summary text only when
`SummaryStatus == "reviewed"`; otherwise it's empty and `SummaryStatus`
tells the caller why, rather than leaving it to guess whether empty means
"no content" or "content exists but isn't trustworthy yet." `SourceType` is
`"document_gist"` or `"document_section"`.

### 10. Re-ingesting a changed document: match by heading, never silently delete review work

Resolved after discussion, in favor of the careful option flagged above,
and for a reason bigger than "it's more careful": a wipe-and-recreate
default would foreclose a real future direction rather than just being
locally worse. The stated goal is eventually interactive re-ingest in
harvey — a human, a model, or both together revisiting a document as it
changes, promotion and re-ingestion handled collaboratively, closer to a
Socratic dialogue building up what's known than a batch job. That mode
needs exactly the information a silent wipe would throw away: which
sections changed, which summaries might now be stale, which sections
vanished. Building the destructive default now would mean rebuilding this
decision later anyway, once that mode is wanted — better to get the
non-destructive shape right the first time, even though nothing in this
plan builds the interactive session itself.

So: on a checksum change, **match new sections to old ones by `(level,
heading)`**, not by position, and never delete silently:

- **Heading matches, body unchanged** — untouched, summary and status
  carry forward exactly as they were.
- **Heading matches, body changed** — the summary is kept, not deleted
  (deleting it would be exactly the data loss this decision exists to
  avoid), but it's now describing text that has moved out from under it.
  Flag it: a new `summary_stale` boolean column on `document_sections`,
  set true whenever the underlying `body` changes under an existing
  `drafted` or `reviewed` summary. Nothing currently reads this column to
  block anything automatically — it's a signal for the review queue
  (decision 7) to surface, and exactly the kind of thing a future
  interactive re-ingest session would ask about first.
- **Old heading has no match in the new content** — the section is
  reported, not deleted, mirroring `kb ingest`'s own existing rule for
  records almost verbatim ("a record whose file has vanished stays in the
  database and is reported, never deleted — pruning would destroy data").
  A renamed scene heading is indistinguishable from a removed one at this
  mechanical level, which is a known, named limitation, not a silent gap.
- **New heading has no match in the old content** — a genuinely new
  section, inserted fresh with `summary_status = 'unsummarized'`.

This is still entirely mechanical and non-interactive — `kb document
ingest` does the matching and reporting, nothing more. The interactive,
dialogic re-ingest harvey might build later is explicitly not this design's
job (see Deferred, below); this decision's job is only to make sure that
future mode has something real to work with instead of a clean slate every
time a file changes.

## Deferred, explicitly

- PDF ingestion (decision 2) — blocked on an extraction-tooling decision
  this design doesn't make.
- Any model call, anywhere in `knowledge` — drafting is always supplied by
  the caller (decision 7); `knowledge` never invokes a model.
- Un-reviewing a promoted summary — the FTS mechanism in decision 8 would
  support it, but no command exposes it in this pass.
- Auto-promotion — the feature request decided against it already; unchanged
  here.
- Consuming any of this from harvey's `UnifiedMemory.Recall` — separate,
  harvey-repo work, same boundary drawn in `concept-tag-retrieval-design.md`.
- **Interactive/dialogic re-ingest itself** (decision 10's stated
  motivation) — a harvey-driven session where a human, a model, or both
  review `summary_stale`/orphaned sections turn by turn and decide what to
  re-draft or re-promote. `knowledge`'s job here stops at exposing the
  signal (`summary_stale`, reported vanished sections); building the
  session that consumes it is future harvey work, not part of this plan.
- Portability (`knowledge_merge.go`, `jsonl.go`) for the three new tables —
  required before this ships, per DR-0013's rule that every table must
  travel, but it's mechanical repetition of the pattern
  `wikilink-tagging-plan.md` already established, worth its own plan phase
  rather than a design decision.
