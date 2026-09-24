%kb-observation(1) user manual | version 0.0.12 036bd79
% R. S. Doiel
% 2026-09-23

# NAME

kb-observation — manage observations

# SYNOPSIS

kb observation add --project NAME KIND BODY [--source-doi DOI]

kb observation list --project NAME

kb observation show ID

kb observation update ID BODY...

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
: show a single observation by id, including its resolved supersedes/
  superseded_by relations if it has any

update
: correct an observation by superseding it, not by mutating it. ID's body
  is never touched; a new observation is inserted with BODY, inheriting ID's
  project and kind, and linked to ID by a supersedes edge. The original
  wording survives unchanged, so nothing needs a separate revision history --
  see DR-0023 (knowledge/decisions/). Cross-machine reconciliation carries
  the new supersedes edge the same as every other relation in this schema;
  there is no per-field last-writer-wins here, unlike
  kb-project(1)'s set-description

sources
: list the sources cited by an observation (see kb-source(1))

# SEE ALSO

kb-project(1), kb-link(1), kb-source(1)

