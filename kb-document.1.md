%kb-document(1) user manual | version 0.0.10 f3d323d
% R. S. Doiel
% 2026-09-18

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

# VOCABULARIES

summary_status
: unsummarized, drafted, reviewed -- promotion is a one-way, human-only
  action for a first pass; no path un-reviews a promoted summary yet

# SEE ALSO

kb-record(1), kb-ingest(1), kb-search(1)

