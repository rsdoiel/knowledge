%kb-link(1) user manual | version 0.0.29 1ae6b7d
% R. S. Doiel
% 2026-10-29

# NAME

kb-link — link projects/observations to concepts

# SYNOPSIS

kb link project PROJECT_NAME CONCEPT_NAME

kb link observation OBS_ID CONCEPT_NAME

# DESCRIPTION

Links are many-to-many and idempotent — linking the same pair twice is a
silent no-op.

# EXIT STATUS

The workspace convention, as described in kb(1).

0
: the link now exists. Linking a pair that is already linked is a no-op and
  also 0

1
: the project, observation or concept named does not exist

2
: a missing or surplus argument, a malformed observation id, or an unknown
  subcommand

# SEE ALSO

kb-project(1), kb-observation(1), kb-concept(1), kb-unlink(1)

