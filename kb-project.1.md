%kb-project(1) user manual | version 0.0.0 f50285c
% R. S. Doiel
% 2026-07-27

# NAME

kb-project — manage projects

# SYNOPSIS

kb project add NAME [DESCRIPTION]

kb project list

kb project show NAME

kb project concepts NAME

# DESCRIPTION

A project is the top-level container observations and concepts attach to.
Names are unique; adding a project with an existing name is a no-op that
returns the existing project's id.

add
: create a project, or return the id of the existing one with that name

list
: list every project (bare rows — see kb-format(1) for an
  assembled Markdown view with linked concepts/observations included)

show
: show a single project by name

concepts
: list the concepts linked to a project

# SEE ALSO

kb-observation(1), kb-link(1), kb-search(1)

