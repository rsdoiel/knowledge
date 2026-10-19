%kb-document(1) user manual | version 0.0.25 d5d8964
% R. S. Doiel
% 2026-10-19

# NAME

kb-document — ingest and review narrative documents at graduated abstraction levels

# SYNOPSIS

kb document ingest PATH --project P [--title T] [--format F] [--dry-run] [--no-prompt]

kb document list [--project P]

kb document show ID

kb document review list [--project P] [--status S]

kb document draft SECTION_ID BODY --by WHO [--confidence N]

kb document review promote SECTION_ID

kb document tag --project P [--concept NAME,...] [--dry-run]

kb document fuzzy-tag --project P [--concept NAME,...] [--dry-run]

kb document frontmatter PATH [--accept FIELD,...] [--accept-keywords NAME,...] [--set FIELD=VALUE] [--dry-run]

kb document delete ID [--force] [--dry-run]

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

A wikilink may carry a display alias after a bar: `[[the-long-fire/Anna|Anna]]`
links the concept before the bar and shows the text after it. Only the part
before the bar is the concept's name.

Story events (DR-0075, DR-0079). A file whose front matter has a narrative key
(event, story_time, pov, characters, foreshadows, pays_off, precedes, learns or
knows; place and status alone are ordinary front matter and do not count, but travel with
an event) is one story event; a Fountain scene with a scene number (INT. KITCHEN - DAY
#12A#) is an event too, with its own metadata in a one-line boneyard after the
heading: /* story: story_time=1969-10-29; pov=Anna */. A scene with no number
is not an event. story_time is a date (1969-10-29), a month, a year,
circa YEAR, ordinal: N, or after:/before: LABEL with an optional offset such
as +3d (units d, w, m, y). The summary says how many events the file yields
and names every key or value it did not understand.

What a character learns or already knows (DR-0083). learns and knows each take
items of the form "Character: fact": learns says the character finds the fact
out in this scene, knows says the character already knew it and the scene
relies on it. In front matter they are YAML lists; in a boneyard or a story
comment the key repeats, once per fact, because a fact may hold a comma:
/* story: learns=Anna: the letter, which Cass wrote, is forged; learns=Bram: the debt */.
A fact is the same fact whatever its case or spacing. A repeated learns of one
fact by one character keeps the first. In a Markdown file with scene ids the
front matter learns and knows are ignored, as event and story_time are.

Several scenes in one Markdown file (DR-0082). A heading with an id is a scene
and an event; a heading without one is an ordinary section. Give the id on the
heading, `## The fire {#12A}`, or as id=12A in a story comment on the line after
the heading, `<!-- story: id=12A; story_time=1969-10-29; pov=Anna -->`, or both
(if they differ the comment wins and ingest says so). The comment takes the
same keys as a Fountain boneyard, is left out of the section's text, and is
read only directly after a heading. The file's front matter pov, place, status
and characters are the defaults for every scene; a scene's comment overrides
them field by field, and a list in the comment replaces the front matter's
list. With scene ids, front matter event and story_time are ignored (two scenes
cannot share them) and ingest says so. A heading ending in a Fountain-style
number, #12A#, is plain text, not an id. A scene id repeated in one file keeps
the first scene as the event and reports the rest. Headings inside fenced code
blocks are not headings, and a heading's closing # marks and {#id} are not part
of its text. Events are derived from
the file on every ingest, a skipped one included, so running this again on the
same paths rebuilds them after an import or merge; they are not exported,
imported, merged or compared by check-db.

Sources a document cites (DR-0081). A front matter key sources lists the works
the document rests on: a bare DOI (10.1234/sparql), an absolute URL, or
source:ID for a source already in the knowledge base. Ingest links each to the
document through document_sources: an existing identifier or id is linked, and a
DOI or URL not yet known becomes a new source titled by its identifier. The links
are rebuilt on every ingest of the file, a skipped one included. A file with the
key manages the links it made (relationship cited): drop an entry and its link
goes, and sources: [] clears them; links with another relationship, made by hand,
are kept. A file with no sources key manages none. An entry that is not one of the
three forms (a pasted doi: or doi.org prefix is refused, as kb source add refuses
it), or a source:ID that names no source, is reported as a source warning and
skipped, and never fails the ingest. The summary says how many sources were
linked. See kb-source(1) for the registry and retraction checking.

Re-ingesting a changed file matches new sections to old ones by heading
text: an unchanged section is left alone; a changed one updates its body
and is flagged stale rather than losing its summary; a heading with no
match is reported, never deleted.

ingest
: parse and segment PATH, creating the document and its sections on first
  ingest, or reconciling a changed file against its existing rows. On a
  terminal, a file with no frontmatter block (a note, say), or with a block that
  has no title, is first offered one (a block's other keys are kept, and only
  the fields it lacks are asked about): `title`, `author` and `dateCreated` are proposed one at a time as for
  the frontmatter subverb below (Enter accepts, text replaces, - skips, q
  stops), then the known concepts the text mentions as `keywords`, and the
  file is written only after a final confirmation. A new concept is never
  created here. Stopping, declining or input that ends leaves the file as it
  was, and the note is ingested as before. `--no-prompt` turns the offer off;
  it is also off with `--dry-run`, with the global `--json`, and whenever
  standard input or output is not a terminal, and nothing makes it appear in
  those cases. A file whose frontmatter block has a title is never offered one.
  There is no directory form: ingest takes one file, and kb ingest is for
  decision records and skips dated notes. To bring a directory of notes in, loop
  over its files, and each one that lacks a title is offered the editor in turn
  (q skips the editor for that file, which is still ingested):

      for f in notes/*.md; do kb document ingest "$f" --project P; done

  Add --no-prompt to ingest them all without being asked.

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
  never indexed at all, only its reviewed summary. It requires an
  interactive terminal on standard input and output (exit 2 without one,
  nothing written, no way to turn that off; see EXIT STATUS), so a model
  that drafted a summary cannot also make it trusted. The library call
  PromoteDocumentSummary has no such rule: a program that links the
  package, harvey for one, enforces its own

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
  run is a no-op. --dry-run reports without writing. Decision records have
  their own verb, kb-record(1)'s fuzzy-tag: it adds the concept to
  the record's tags: instead of a footnote, never edits an accepted record,
  and skips a concept only when the record already links it, not when its
  plain name appears in the prose.

frontmatter
: propose-then-accept `title`/`author`/`dateCreated`/
  `dateModified`/`keywords` for one document, PATH -- unlike
  tag/fuzzy-tag, not scoped by --project, since a human reviews one file at
  a time. A bare invocation (no --accept/--accept-keywords/--set) only
  reports; nothing is ever written without an explicit selection.
  `title`/`author`/`dateCreated` are absent-only -- never
  overwritten once present. `dateCreated` comes from a YYYY-MM-DD at the start
  of the file's name (notes/2026-09-29-worklog.md) when there is one, then
  git's first commit, then the file's time. `author` is a layered signal: a byline in
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

delete
: delete a document, its sections, their concept links and the search entries of
  any promoted summaries. A document with a section whose summary is reviewed is
  refused (exit 1): a reviewed summary is human-gated data and deleting it loses
  it. --force deletes anyway. Ingesting the file again recreates the document,
  without the reviewed summaries. --dry-run reports and changes nothing.

Deletion is local to one database, with no tombstone: kb-merge(1) and
kb-import(1) from a database that still has the row bring it back, so
delete it there as well. `agents/knowledge.jsonl` is re-exported from the database,
so a delete reaches a database rebuilt from it. See DR-0050 (knowledge/decisions/).

# VOCABULARIES

summary_status
: unsummarized, drafted, reviewed -- promotion is a one-way, human-only
  action for a first pass; no path un-reviews a promoted summary yet

# EXIT STATUS

The workspace convention, as described in kb(1): 0 success; 1 the
command ran and the answer is no (no such document or section, a section that is
not drafted so there is nothing to promote); 2 the command line is wrong. Setting
a summary to reviewed is a person's act: review promote exits 2, and writes
nothing, unless standard input and standard output are both a terminal, with a
message that a person must promote it. No flag or environment variable turns that
off. draft, review list and every other verb here need no terminal, so a model or
a script can draft but not make a summary trusted.

# SEE ALSO

kb-record(1), kb-ingest(1), kb-search(1)

