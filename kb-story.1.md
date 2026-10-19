%kb-story(1) user manual | version 0.0.25 d5d8964
% R. S. Doiel
% 2026-10-19

# NAME

kb-story — the story timeline, continuity report and who knows what for a writing project

# SYNOPSIS

kb story timeline --project P [--narrative]

kb story check --project P [--strict]

kb story who-knows CHARACTER [SCENE] --project P

kb story who-learns FACT --project P

# DESCRIPTION

A writing project is a kb project whose manuscript files have been
ingested with kb-document(1) ingest. Story events are derived from
those files: a Fountain scene with a scene number (INT. KITCHEN - DAY #12A#),
a Markdown heading with a scene id (## The fire {#12A}, or id=12A in a story
comment under it; see kb-document(1)), or a file whose front matter has a
narrative key (event, story_time, pov, characters, foreshadows, pays_off,
precedes, learns, knows). Every verb only reads, and every one
needs --project.

timeline
: list the project's events. By default they are in story order: an event comes
  after every event provably before it, by intervals that do not overlap, by a
  before: or after: relation, or by a precedes list; ties go to reading order.
  Where the order of an event after the one above it is not established
  (overlapping intervals, an ordinal against a calendar date, or no relation) the
  row is marked ~ and is never guessed. A row marked ? has no date and no relation
  to any other event and is listed last (with --narrative, ? means no date of its
  own); a row marked ! is in, or ordered behind, a
  cycle of before/after relations. With --narrative the events are listed in
  reading order, by document and position in it, with the story time alongside.
  --json gives a list of rows: rank, label, scene_id, story_time, start, end,
  precision, state, place, pov, status, characters, document, unordered, undated,
  in_cycle.

check
: report continuity problems and change nothing. The kinds are missing-file (a
  Fountain file that cannot be read, so its scene numbers were not checked),
  unnumbered-scene (a Fountain scene with no scene number, which therefore has no
  event), duplicate-label, unparseable-time, unresolved-time (a relative time whose
  anchor is missing, ambiguous, undated or cyclic), unknown-reference (a
  foreshadows, pays_off or precedes name that names no single event),
  contradiction (a declared order the events' dates contradict), cycle,
  two-places (a character in two places on the same day or ordinal; wider
  intervals overlap innocently and are not flagged) and foreshadow-order (within one
  document, a scene foreshadowing an earlier scene or paying off a later one;
  across documents the reading order is not known). For Markdown scenes:
  scene-comment-no-id (a story comment on a heading with no id),
  scene-id-disagreement (the heading's {#id} and the comment's id= differ; the
  comment wins), scene-id-repeated (one id used twice in a file),
  scene-comment-syntax (an unknown key, or a word with no "=", in a story
  comment), fountain-style-heading (a heading ending #12A#, which is text and
  not an id; reported once per file) and front-matter-ignored (an event or
  story_time, learns or knows in front matter, ignored because the file has scene ids).
  Three kinds read what characters learn and know (DR-0083):
  knowledge-before-learned (a scene says a character knows a fact, and every
  scene where the character learns it is later in story order; a knows with no
  learns anywhere is backstory, and a learns that is not ordered against the
  knows is not reported), unknown-fact (two different facts of at least 12
  characters that are one edit apart, probably one fact typed twice; they are
  reported and never merged) and late-source (a document cites a source with the
  relationship "evidence" whose published date is after the end of one of the
  document's events; a source with no date, or an event with no calendar
  interval, is silent). late-source does nothing until a citation is labelled:
  kb source link --document ID SOURCE_ID --relationship evidence (a
  "cited" link, which is what the sources: front matter makes, is never
  reported). The report is advice: the
  exit status is 0 however many findings there are. With --strict it is 1 when
  there are any. --json gives a list of kind, document and message.

who-knows
: list what a character learns, in story order, with the scene each fact is
  learned in. With a SCENE (a scene number such as 12A, or an event label) only
  the facts learned at or before that scene are listed, the scene itself
  included. Facts learned only in scenes whose order against SCENE is not
  established (overlapping times, or no time at all) are listed apart, under
  "Not ordered against SCENE", and are never guessed either way. A fact comes
  from a learns item of the form "Character: fact" in a scene (see
  kb-document(1)); knows rows are use, not learning, so they are never
  listed, but a character who has only knows rows is found, with nothing learned.
  Story order is the only order: a flashback told late is placed where it
  happens. A fact learned in several scenes is listed once, at the earliest.
  The character is matched without regard to case. --json gives character,
  scene, facts and unordered, each fact with fact, learned_in, scene_id and
  document.

who-learns
: list who learns a fact, and in which scene, in story order. The fact is
  matched without regard to case or spacing. A character who learns it twice is
  listed once, at the earliest. A fact that is only used (a knows item) has no
  learner and is not found. --json gives fact and learners, each with character,
  learned_in, scene_id and document.

# EXIT STATUS

The workspace convention, as described in kb(1).

0
: the report was written (check, without --strict, whatever it found)

1
: an unknown project; check --strict found something; or, for who-knows and
  who-learns, a character, scene or fact that names nothing

2
: a missing --project, an unknown subverb or flag, a flag the subverb does not
  take (--narrative on check, --strict on timeline), a missing or surplus
  argument, a blank fact, or a scene name that matches several events

# SEE ALSO

kb(1), kb-document(1), kb-concept(1)

