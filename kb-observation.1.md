<<<<<<< HEAD
%kb-observation(1) user manual | version 0.0.2 a21f85b
=======
%kb-observation(1) user manual | version 0.0.3 193fa97
>>>>>>> 6f81c33478fb839db70903bb8800aa73c795ab0b
% R. S. Doiel
% 2026-08-08

# NAME

kb-observation — manage observations

# SYNOPSIS

kb observation add --project NAME KIND BODY [--source-doi DOI]

kb observation list --project NAME

kb observation show ID

kb observation sources ID

# DESCRIPTION

An observation is a timestamped note attached to a project. KIND is one of
note, finding, decision, question, or hypothesis.

add
: record a new observation under --project; --source-doi records the
  normalized DOI of the paper it was extracted from, if any

list
: list a project's observations, most recent first

show
: show a single observation by id

sources
: list the sources cited by an observation (see kb-source(1))

# SEE ALSO

kb-project(1), kb-link(1), kb-source(1)

