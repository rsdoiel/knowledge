%kb-source(1) user manual | version 0.0.1 72e0ed6
% R. S. Doiel
% 2026-07-27

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
  the existing source's id instead of duplicating it

remove
: delete a source — fails if it's still linked to any observation

retract
: mark a source retracted with a note (does not delete it)

check-retractions
: query the Retraction Watch API for every registered, non-retracted DOI
  source and mark hits as retracted; requires network access

# SEE ALSO

kb-observation(1)

