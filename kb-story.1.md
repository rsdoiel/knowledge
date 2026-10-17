%kb-story(1) user manual | version 0.0.24 0010b45
% R. S. Doiel
% 2026-10-17

# NAME

kb-story — the story timeline and continuity report for a writing project

# SYNOPSIS

kb story timeline --project P [--narrative]

kb story check --project P [--strict]

# DESCRIPTION

A writing project is a kb project whose manuscript files have been
ingested with kb-document(1) ingest. Story events are derived from
those files: a Fountain scene with a scene number (INT. KITCHEN - DAY #12A#),
or a file whose front matter has a narrative key (event, story_time, pov,
characters, foreshadows, pays_off, precedes). Both verbs only read, and both
need --project.

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
  across documents the reading order is not known). The report is advice: the
  exit status is 0 however many findings there are. With --strict it is 1 when
  there are any. --json gives a list of kind, document and message.

# EXIT STATUS

The workspace convention, as described in kb(1).

0
: the report was written (check, without --strict, whatever it found)

1
: an unknown project; or check --strict found something

2
: a missing --project, an unknown subverb or flag, a flag the subverb does not
  take (--narrative on check, --strict on timeline), or a surplus argument

# SEE ALSO

kb(1), kb-document(1), kb-concept(1)

