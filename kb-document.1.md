%kb-document(1) user manual | version 0.0.11 bb57801
% R. S. Doiel
% 2026-09-23

# NAME

kb-document — ingest and review narrative documents at graduated abstraction levels

# SYNOPSIS

kb document ingest PATH --project P [--title T] [--format F] [--dry-run]

kb document list [--project P]

kb document show ID

kb document review list [--project P] [--status S]

kb document draft SECTION_ID BODY --by WHO [--confidence N]

kb document review promote SECTION_ID

kb document tag --project P [--concept NAME,...] [--dry-run]

kb document fuzzy-tag --project P [--concept NAME,...] [--dry-run]

kb document frontmatter PATH [--accept FIELD,...] [--accept-keywords NAME,...] [--set FIELD=VALUE] [--dry-run]

# DESCRIPTION

A document is a narrative or article (Markdown, Fountain, or plain text)
ingested into two tables: one documents row plus one document_sections row
per structural unit, always including one `level = 'gist'` row for the
whole document. Unlike a decision record, a document takes no required
frontmatter -- title/author/publication-date/keywords are recognized when
present (antennaApp's own frontmatter vocabulary: title, description,
pubDate, author, keywords) but never required, since a story or screenplay
usually has none.

Every `[[Name]]` wikilink in a section's body, and every frontmatter
keyword, resolves to a concept and links to the record the same way
kb-ingest(1) tags decision records -- case-insensitive, and
matching case becomes canonical for later mentions. A section's
tag_density (how many known concepts its text mentions, tagged or not) is
a mechanical triage signal, not a link -- see review, below.

Re-ingesting a changed file matches new sections to old ones by heading
text: an unchanged section is left alone; a changed one updates its body
and is flagged stale rather than losing its summary; a heading with no
match is reported, never deleted.

ingest
: parse and segment PATH, creating the document and its sections on first
  ingest, or reconciling a changed file against its existing rows

list
: print matching documents, one per line

show
: print one document's metadata plus every section (heading, summary
  status, stale flag, linked concepts) in one view

review list
: the triage queue -- sections needing attention, with their size,
  tag_density and confidence signals. Defaults to unsummarized and
  drafted sections; --status narrows to one exact status, including
  "reviewed" for auditing

draft
: write a gist or section summary. WHO is free text ("human" or a model
  identifier) -- kb never calls a model itself, drafting is always
  supplied by the caller. Clears a section's stale flag, since a fresh
  draft is not stale against itself

review promote
: promote a drafted summary to reviewed -- the only human action that
  makes a summary trusted. Only a reviewed summary is indexed for search
  or returned as content by a concept-tag query; a section's raw body is
  never indexed at all, only its reviewed summary

tag
: a pure file operation over every already-ingested document in --project:
  inserts an explicit `[[Name]]` wikilink, at its first safe occurrence,
  for each eligible concept. Never writes to the database -- the next
  ingest of a changed file links it through the wikilink path above,
  unchanged. With no --concept, eligible means mentioned more than once in
  the file outside code spans (the same threshold density-linking already
  applies); --concept NAME,... forces exactly those names regardless of
  occurrence count, but each must already be a known concept, checked
  before any file is touched. A name already wikilinked anywhere in a file
  is left alone, so a second run is a no-op; YAML frontmatter and the
  document's own first H1 heading (its title) are never written into
  either, even if a concept name genuinely occurs there. --dry-run reports
  without writing. See DR-0029 (knowledge/decisions/).

fuzzy-tag
: mirrors tag's shape, catching what tag's exact matching can't: a typo,
  plural, or simple tense variant of a known concept's name (Levenshtein
  distance, not paraphrase). Never bracket-wraps the near-miss text itself
  -- doing so would mint a duplicate concept from the misspelled or
  inflected spelling. Instead inserts a `[^n]` footnote marker at the
  match and a footnote definition carrying the canonical
  `[[Concept]]` elsewhere, so prose stays unedited and linking still
  resolves to the real concept. A concept already an exact match anywhere
  in the file is skipped entirely -- fuzzy matching is additive, never a
  duplicate of what tag already covers. With no --concept, eligible means
  within a length-based distance threshold; --concept NAME,... bypasses
  that threshold for exactly the names given, each already a known
  concept, checked before any file is touched. A concept already
  footnoted (or wikilinked) anywhere in a file is left alone, so a second
  run is a no-op. --dry-run reports without writing.

frontmatter
: propose-then-accept `title`/`author`/`dateCreated`/
  `dateModified`/`keywords` for one document, PATH -- unlike
  tag/fuzzy-tag, not scoped by --project, since a human reviews one file at
  a time. A bare invocation (no --accept/--accept-keywords/--set) only
  reports; nothing is ever written without an explicit selection.
  `title`/`author`/`dateCreated` are absent-only -- never
  overwritten once present. `author` is a layered signal: a byline in
  the document's own prose wins, then git's earliest-commit author, then
  `git config user.name` -- when signals disagree, the report shows all
  of them, not just the winner. `dateModified` is the one exception to
  absent-only: always recomputed from git's most recent commit (filesystem
  mtime otherwise) and rewritten on every accepted run, since a stale value
  is worse than a missing one. `keywords` diffs the current list against
  two candidate sets: known concepts already mentioned in the text (a plain
  write on `--accept-keywords`), and new candidate terms distinctive to
  this document specifically, scored against the rest of its project (or
  the whole corpus) with the document itself excluded from that comparison
  -- accepting one creates the concept first, the same checked-before-any-
  write discipline `tag`'s `--concept` already holds to.
  `--set FIELD=VALUE` (repeatable) bypasses signal detection and the
  absent-only rule entirely -- a human's explicit assertion, not a
  proposal. Editing is surgical: only the frontmatter block changes, via a
  YAML node edit that leaves every other key, value, and comment in the
  block untouched; the document body is never modified. This module's
  first process dependency -- shells out to `git` for provenance,
  falling back to filesystem timestamps when `git` itself fails (not a
  repository, or a genuinely untracked file).

# VOCABULARIES

summary_status
: unsummarized, drafted, reviewed -- promotion is a one-way, human-only
  action for a first pass; no path un-reviews a promoted summary yet

# SEE ALSO

kb-record(1), kb-ingest(1), kb-search(1)

