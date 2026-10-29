%kb-source(1) user manual | version 0.0.29 7d292d7
% R. S. Doiel
% 2026-10-29

# NAME

kb-source — manage cited sources and retraction checking

# SYNOPSIS

kb source add TITLE [--doi D] [--url U] [--authors A] [--published DATE] [--publisher P] [--rights R] [--version V]

kb source list

kb source show ID

kb source remove ID

kb source retract ID NOTE

kb source link OBS_ID SOURCE_ID [--relationship R] [--version V] [--hash H]

kb source link --document DOC_ID SOURCE_ID [--relationship R] [--version V] [--hash H]

kb source link --record REF SOURCE_ID [--relationship R] [--version V] [--hash H]

kb source check-retractions

# DESCRIPTION

A source is a cited work (paper, page, dataset) that an observation, a document
or a decision record can cite (DR-0081), recording the relationship (default:
cited). source link makes the citation by hand; a document or record can also
list its sources in a sources front matter key, which ingest links (see
kb-document(1) and kb-ingest(1)).

link
: cite a source: OBS_ID SOURCE_ID for an observation, --document DOC_ID for a
  document, --record REF for a decision record (a qualified reference such as
  harvey/DR-0004). --version and --hash record which version of the work was
  read and its content hash then; both are optional and neither may be blank.
  Citing the same pair again changes nothing. An unknown document, record,
  observation or source is exit 1; --document with --record, a bad id or a
  blank value is exit 2. An observation link needs no flag.

list, show
: list the sources, or show one; each says what cites it ("cited by 2
  observation(s), 1 document(s)"). With --json each source has a citations
  object: observations, documents and records.

add
: register a source; --doi/--url set its identifier (doi takes priority
  if both are given); adding one whose identifier already exists returns
  the existing source's id instead of duplicating it. The title is made one
  line, as a project or concept name is: it is trimmed and every run of
  whitespace in it (a newline, a tab) becomes one space, so a hard-wrapped paste
  is stored as one line. A blank title, or one holding a control character, is
  refused (exit 2); a title already stored, and one read by import, is not
  rewritten. --published must be YYYY, YYYY-MM or YYYY-MM-DD and
  a real date. --url must be an absolute URL with a scheme and a host. --doi
  must be the bare form 10.NNNN/suffix; a pasted doi: or https://doi.org/
  prefix is refused rather than rewritten, because a mistyped DOI would
  otherwise be checked against Retraction Watch, find nothing, and read as
  "not retracted". Other identifier types are not checked. Whitespace around
  a value is trimmed

remove
: delete a source — fails (exit 1) if an observation, a document or a record
  still cites it (the message says which), and also for an ID that does not
  exist

retract
: mark a source retracted with a note (does not delete it)

check-retractions
: query the Retraction Watch API for every registered, non-retracted DOI
  source and mark hits as retracted; requires network access. It tries every
  source even if one lookup fails. A source it could not look up is not "not
  retracted": it is counted as not checked, its last-checked date is left alone,
  and the command exits 69 after printing the counts. Run it again when the
  service is reachable

# EXIT STATUS

The workspace convention, as described in kb(1).

0
: done. check-retractions with nothing to check is 0

1
: no source, observation, document or record has the id or reference given
  (show, retract, remove, link); or remove of a source that an observation, a
  document or a record still cites

2
: a missing or surplus argument, a malformed id, a blank title or one holding a
  control character, a DOI, URL or date that does not parse, --document given
  with --record, or an unknown subcommand

69
: check-retractions could not reach the retraction service for at least one
  source. It checks every source first and prints the counts; a source it could
  not look up is counted as not checked and its last-checked date is left alone,
  so run it again when the service is reachable

# SEE ALSO

kb-observation(1)

