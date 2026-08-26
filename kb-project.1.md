%kb-project(1) user manual | version 0.0.4 4a1de01
% R. S. Doiel
% 2026-08-26

# NAME

kb-project — manage projects

# SYNOPSIS

kb project add [--status concept|active|paused|concluded] NAME [DESCRIPTION]

kb project list

kb project show NAME

kb project concepts NAME

kb project set-status NAME STATUS

# DESCRIPTION

A project is the top-level container observations and concepts attach to.
Names are unique; adding a project with an existing name is a no-op that
returns the existing project's id (its status, if any, is left unchanged).

add
: create a project, or return the id of the existing one with that name.
  --status sets the initial status (default: active).

list
: list every project (bare rows — see kb-format(1) for an
  assembled Markdown view with linked concepts/observations included)

show
: show a single project by name

concepts
: list the concepts linked to a project

set-status
: change an existing project's status. STATUS must be one of concept,
  active, paused, concluded.

# SEE ALSO

kb-observation(1), kb-link(1), kb-search(1)

