%kb-concept(1) user manual | version 0.0.0 f50285c
% R. S. Doiel
% 2026-07-27

# NAME

kb-concept — manage concepts

# SYNOPSIS

kb concept add NAME [DESCRIPTION] [--identifier-type T --identifier-value V]

kb concept list

# DESCRIPTION

A concept is a named idea or term that can be linked to projects and
observations (see kb-link(1)). Names are unique.

A concept may also represent a scholarly entity — a paper, person,
institution, or funder — by setting --identifier-type (e.g. doi, orcid,
ror, fundref) and --identifier-value (the normalized identifier).

# SEE ALSO

kb-link(1), kb-project(1)

