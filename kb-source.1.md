%kb-source(1) user manual | version 0.0.13 1aa3e79
% R. S. Doiel
% 2026-09-24

# NAME

kb-source — manage cited sources and retraction checking

# SYNOPSIS

kb source add TITLE [--doi D] [--url U] [--authors A] [--published DATE] [--publisher P] [--rights R] [--version V]

kb source list

kb source show ID

kb source remove ID

kb source retract ID NOTE

kb source link OBS_ID SOURCE_ID [--relationship R]

kb source check-retractions

# DESCRIPTION

A source is a cited work (paper, page, dataset) an observation can link to
via source link, recording the relationship (default: cited).

add
: register a source; --doi/--url set its identifier (doi takes priority
  if both are given); adding one whose identifier already exists returns
  the existing source's id instead of duplicating it. The title is trimmed
  and must not be blank. --published must be YYYY, YYYY-MM or YYYY-MM-DD and
  a real date. --url must be an absolute URL with a scheme and a host. --doi
  must be the bare form 10.NNNN/suffix; a pasted doi: or https://doi.org/
  prefix is refused rather than rewritten, because a mistyped DOI would
  otherwise be checked against Retraction Watch, find nothing, and read as
  "not retracted". Other identifier types are not checked. Whitespace around
  a value is trimmed

remove
: delete a source — fails (exit 1) if it's still linked to any observation, and
  also for an ID that does not exist

retract
: mark a source retracted with a note (does not delete it)

check-retractions
: query the Retraction Watch API for every registered, non-retracted DOI
  source and mark hits as retracted; requires network access. It tries every
  source even if one lookup fails. A source it could not look up is not "not
  retracted": it is counted as not checked, its last-checked date is left alone,
  and the command exits 69 after printing the counts. Run it again when the
  service is reachable

# SEE ALSO

kb-observation(1)

