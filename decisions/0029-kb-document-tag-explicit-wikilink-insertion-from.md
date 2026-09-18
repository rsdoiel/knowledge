---
id: "0029"
title: "kb document tag: explicit wikilink insertion from known concepts"
date: "2026-09-18"
status: accepted
kind: decision
trigger: request
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0027", "0028"]
initiative: ""
session: ""
decisions: ["kb document tag --project NAME [--concept NAME,...] [--dry-run] is a new, pure-file-operation verb: it inserts an explicit [[Name]] wikilink at the first safe occurrence of each eligible concept, in every already-ingested document for a project, and never writes to the database -- the next kb document ingest of a changed file links it through the wikilink path that already exists", "With no --concept, eligibility reuses DR-0027's density-linking threshold verbatim (>1 occurrence, outside code spans) via a new eligibleConcepts helper built on MatchConceptNameCounts; --concept NAME,... forces exactly those names regardless of occurrence count, but each must already be a known concept, checked before any file is touched", "Insertion is a minimal, position-based text edit on the raw file bytes -- no document parse-and-rerender cycle -- excluding YAML frontmatter, fenced code blocks, inline code spans, any span already inside an existing [[...]], and (found live, smoke-testing against a real document) the document's own first H1 heading, so a concept mention in the title is never wrapped", "A concept already wikilinked anywhere in a file is left alone, making a second run a no-op; writes across a project's whole document set are both-or-neither, restoring every file already written in the run if a later one fails"]
tags: [request, documents, concepts, wikilinks, markdown]
uuid: "01a0b6b7-b31b-712f-b9af-43f86da7dca9"
origin_host: "wren"
---

**Context.** Raised directly after DR-0028 shipped `kb concept suggest`:
once a plausible list of new concepts exists (candidate names surfaced,
then confirmed via `kb concept add`), there was nowhere for that judgment
to land in the corpus itself. A concept mentioned more than once already
gets linked implicitly on the next ingest (DR-0027's density-linking), but
that link is invisible in the source file — a reader of the Markdown never
sees it, and there is no durable, human-reviewable record of *why* the
concept applies. Two shapes were discussed for closing that gap: writing
`[[Name]]` markers into the prose itself, or writing candidate names into a
document's YAML `keywords:` frontmatter. The frontmatter route was set
aside because `tagSection` (cmd/kb/document.go) only ever reads frontmatter
`keywords` for the document's gist row, never for individual sections — it
would lose exactly the section-level precision inline wikilinks already
have, and would do nothing for a document like an MADR ADR that has no
frontmatter at all (DR-0027's own motivating case).

**Decision.** `kb document tag --project NAME [--concept NAME,...]
[--dry-run]`: for every document already ingested under `NAME`
(`kb.Documents`, already existed, reused as-is), reads the file fresh from
disk and inserts an explicit `[[Name]]` wikilink at the first safe
occurrence of each eligible concept — one insertion per concept per
document, not every occurrence, so the prose stays readable and any
mention beyond the first stays exactly what density-linking is already for.

Eligibility with no `--concept` given is DR-0027's density-linking
threshold, reused verbatim: a concept must be mentioned more than once in
the document, outside code spans. This is a deliberate identity, not a
coincidence — a concept this tool would tag is *always* one density-linking
would already have linked on the next ingest; the tool's only job is to
make that inference visible and durable in the source text, never to
invent a broader relevance rule. `--concept NAME,NAME,...` bypasses the
threshold for exactly the names given (the DR-0028 workflow this record
exists for: a human has already reviewed a candidate and wants it marked
even on a single mention), but every name must already exist as a real
concept — checked against `kb.Concepts()` before any file is opened, so a
typo fails the whole call rather than silently tagging nothing.

Insertion is a minimal, position-based edit directly on the file's raw
bytes: find the first regex match (case-insensitive, word/phrase-boundary
anchored, identical to `MatchConceptNames`'s own construction) that falls
outside a set of excluded spans, and wrap it in `[[ ]]`, preserving the
matched text's original casing. Excluded spans are a leading YAML
frontmatter block, fenced code blocks and inline code spans (reusing
DR-0027's `stripCodeSpans` patterns), any text already inside an existing
`[[...]]` (so a later concept's search cannot nest inside a wikilink a
prior one just inserted), and — added after a live smoke test against a
real document surfaced `# ... Feature [[Request]]` in a document's own
title — the document's first true H1 heading. When multiple concepts apply
to one document, longer names are applied before shorter ones they
contain (`tagDocumentText`), so e.g. "chunking strategy" wraps as one
phrase rather than "chunking" fragmenting it first. A concept already
wikilinked anywhere in the file, found via the same `[[...]]` scan, is
skipped outright — a second run is a no-op, the same "second run skips
unchanged" discipline `kb ingest`/`kb document ingest` already apply.

The command never writes to the database. Writes to files are both-or-
neither across the whole project sweep: any write failure restores every
file already written in the run from its original bytes, the same rollback
shape `kb project rename` (DR-0026) and `kb record supersede` already use.
`--dry-run` reports every file and concept that would change without
writing anything.

**Rationale.** Reusing DR-0027's exact threshold, rather than a separate
relevance rule for explicit tagging, keeps the two mechanisms in lockstep
by construction: there is no way for "what gets an explicit wikilink" and
"what gets linked implicitly" to silently drift apart, because they are
the same test applied at two different times (this tool now, `tagSection`
on the next ingest). A pure, position-based text edit rather than a
parse-and-rerender cycle was chosen because documents have no
`RenderDocumentFile` counterpart to `RenderRecordFile` — `ParseDocumentFile`
discards structure into `ParsedDocument.Sections` with no lossless path
back to Markdown — and building one only to make a single-word insertion
would risk reformatting far more of the file than intended; editing the
raw bytes directly guarantees everything but the inserted brackets is
untouched. Excluding the first H1 heading was not part of the original
design and is included here on live evidence, not speculation: a real
document in this workspace's own corpus had its title rewritten to
`# ... Feature [[Request]]` before the fix, a mechanically correct match
that is nonetheless wrong to make — a title is a name, not prose to be
annotated.

**Rejected alternatives.** Frontmatter-derived `keywords:` instead of
inline wikilinks — the alternative raised alongside this one; rejected as
this record's primary mechanism because it only ever reaches a document's
gist, never a section, and does nothing for a document with no frontmatter
at all (see Context). Not ruled out as a *future*, smaller, no-prose-edit
option for a corpus that specifically wants to avoid inline edits — just
not built here. Wrapping every occurrence of a concept, not just the
first — rejected as visually intrusive with no linking benefit over
first-only, since `tagSection` already dedupes by name per section/gist.
A full document parse-render round trip, so insertion could reason about
document structure (sections, headings) rather than raw text — rejected as
solving a bigger problem (building `RenderDocumentFile`) than this
insertion needs, and riskier: a lossy round trip could reformat content
having nothing to do with the inserted wikilink.

**Consequences.** Implementation: new `cmd/kb/documenttag.go` —
`insertWikilink`, `excludedSpans`/`textSpan`, `frontmatterBlockPattern`,
`firstH1HeadingPattern`, `alreadyWikilinked`, `tagDocumentText`,
`eligibleConcepts`, `documentTagResult`, `cmdDocumentTag`,
`reportDocumentTag`; dispatched from `cmdDocument` alongside
ingest/draft/review/list/show. New tests in `cmd/kb/documenttag_test.go`
(pure-function coverage, no database) and `cmd/kb/documenttag_cmd_test.go`
(CLI-level, including idempotency and project-scoping), per this project's
TDD discipline. Live-smoke-tested against a scratch copy of the real
`agents/knowledge.db` and its actual corpus (`harvey`'s
`knowledge-learning-mode-feature-request.md`) before and after the
title-heading fix, confirming both the bug and the fix on real data, not
just fixtures. `kb-document(1)` regenerated. `TODO.md`'s
programmatic-corpus-improvement item is corrected to note the
wikilink-insertion half of the "what happens once you have a plausible
concept list" question shipped; frontmatter-keywords and the
Levenshtein/fuzzy-matching candidate both stay open, uncosted.
