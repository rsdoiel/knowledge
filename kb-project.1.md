%kb-project(1) user manual | version 0.0.4 2b27b6d
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

kb project set-description NAME DESCRIPTION

# DESCRIPTION

A project is the top-level container observations and concepts attach to.
Names are unique; adding a project with an existing name is a no-op that
returns the existing project's id (its status and description are left
unchanged) -- use set-status and set-description to change either.

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

set-description
: replace an existing project's description, reindexing it for
  kb-search(1). A description is grounding context -- it is what
  show prints and what search returns -- so this is how one that has gone
  stale gets corrected. Trailing words are joined with a space, as in add;
  pass an explicit empty string to clear the description entirely.

# CAVEATS

A description edited on two machines does not yet reconcile. Both
kb-merge(1) and kb-import(1) resolve a conflict in favour of
the row already present, without consulting timestamps, so a merge keeps one
edit and drops the other. set-description records updated_at against a later
last-writer-wins pass, but nothing reads it across machines today.

# SEE ALSO

kb-observation(1), kb-link(1), kb-search(1),
kb-merge(1)

