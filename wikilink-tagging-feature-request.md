# `[[double-bracket]]` inline concept tagging — Feature Request

> **2026-09-08: Filed.** Captured from a conversation with RSDOIEL. No
> design/decide/plan cycle has been run on the `kb` side yet. This document
> preserves the idea and the decisions already made about its shape — it is a
> starting point for that cycle, not a committed design.
>
> Decided in the filing conversation:
> - `[[Name]]` resolves to a **concept** only (not records, projects, or
>   observations — no cross-type disambiguation needed).
> - A `[[Name]]` naming a concept that doesn't exist yet **auto-creates** it,
>   the same way `[[wikilinks]]` behave in Obsidian/second-brain tools.
> - Scope is **`kb ingest` (records) only** for a first pass — observation
>   bodies (`kb observation add`) are out of scope until this proves useful on
>   records.

## Motivation

Inspired by [Build a digitally sovereign second brain](https://www.raspberrypi.com/news/build-a-digitally-sovereign-second-brain/)
(Raspberry Pi magazine), which uses `[[double-bracket]]` wikilink notation in
plain Markdown as a lightweight way to build a link graph across notes,
without a database or a formal schema — the notation itself *is* the schema,
readable in any plain-text editor.

`kb`'s decision records already have a formal way to express relationships:
YAML frontmatter (`relates_to`, `supersedes`, `initiative`). What they don't
have is a low-friction way to tag a concept **from inside the prose**, at the
point where the author is actually thinking about it, without stopping to
edit frontmatter. `[[Name]]` inline is that: write the sentence, name the
concept where it's relevant, done.

## What `kb` assumes today

Record frontmatter already has a `Tags []string` field
(`recordfile.go:76`, `yaml:"tags,flow"`) — but nothing in `cmd/kb/ingest.go`
reads it. It's parsed and then dropped; no record has ever had its tags
materialized as concepts. The only existing frontmatter-to-concept path is
`initiative`, which `linkInitiative` (`cmd/kb/ingest.go:340`) turns into an
`AddConcept` call plus a `project_concepts` link — one concept per record, not
a list, and scoped to the project rather than the record.

So there is no precedent yet for "many concepts per record, materialized from
free text." `[[Name]]` in the body would be the first.

## What is already fine

- **`AddConceptWithIdentifier`/`AddConcept`** (`knowledge.go:881`) already do
  find-or-create by name — an ingest-time `[[Name]]` scan can call straight
  into the existing function, no new concept-creation path needed.
- **`observation_concepts`/`project_concepts`** join tables already express
  many-to-many concept links; if `[[Name]]` in a record body links to the
  record's own observation (records already get ingested alongside an
  observation row per `DR-0012`/`DR-0021` era work — verify current shape),
  the linking mechanism is a straight reuse.
- **FTS reindexing on concept insert** is already handled by `AddConcept`, so
  a newly-coined concept is searchable immediately, same as one added via
  `kb concept add`.

## Proposal

1. During `kb ingest`, after frontmatter parsing, scan the record body for
   `[[Name]]` tokens (simple regex, no nested-bracket handling needed for a
   first pass).
2. For each distinct `Name` found: `AddConcept(Name, "")` (find-or-create,
   matching decision 2 above), then link it to whatever the record already
   links concepts to today (see "already fine" above — needs the actual
   current linking target confirmed during design, not assumed here).
3. Re-running ingest on an unchanged file should not create duplicate links —
   same idempotency requirement as every other ingest-time relation.
4. Out of scope for this pass (explicitly, per the filing conversation):
   resolving `[[Name]]` to anything other than a concept; scanning
   `kb observation add` bodies; nested or aliased links (`[[Name|Alias]]`,
   which Obsidian supports but this proposal does not need yet).

## Open questions for the design cycle

- **Where do the links land?** Concept-to-record isn't an existing join table
  the way concept-to-observation and concept-to-project are (see `knowledge.db`
  schema table in root `CLAUDE.md`) — does this need a new `record_concepts`
  table, or does it ride on the observation that ingest already creates per
  record?
- **Case/whitespace normalization.** `[[bug]]` and `[[Bug]]` — same concept or
  two? `AddConceptWithIdentifier`'s existing find-or-create presumably does
  exact-name matching; confirm before this doubles up concepts that only
  differ by case.
- **Does this ever conflict with the existing `Tags` frontmatter field**
  sitting unused? Worth deciding whether `[[Name]]` supersedes `Tags`,
  activates it, or the two stay independent.
- **Escaping.** Markdown that legitimately needs literal `[[`/`]]` (unlikely
  in decision records, but not impossible) — probably not worth solving for
  a first pass, but worth naming as a known gap.

## Related

- Filing conversation, 2026-09-08 (Laboratory root).
- `agents-projects-layout-feature-request.md` — the template this document
  follows.
- Raspberry Pi magazine: [Build a digitally sovereign second brain](https://www.raspberrypi.com/news/build-a-digitally-sovereign-second-brain/).
