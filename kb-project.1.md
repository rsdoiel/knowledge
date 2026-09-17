%kb-project(1) user manual | version 0.0.9 b78cc1b
% R. S. Doiel
% 2026-09-17

# NAME

kb-project — manage projects

# SYNOPSIS

kb project add [--status concept|active|paused|concluded] NAME [DESCRIPTION]

kb project list

kb project show NAME

kb project concepts NAME

kb project set-status NAME STATUS

kb project set-description NAME DESCRIPTION

kb project rename OLD NEW

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

rename
: rename a project and reindex it for search. Refuses if NEW already names
  another project, or if the project owns any records -- a decision
  record's project: frontmatter has to match the project's name, and
  renaming without rewriting the corpus's files would make the next
  kb-ingest(1) mint a phantom project under the old name and
  duplicate every record under it. See DR-0024 (knowledge/decisions/).
  Rewrite the corpus's project: frontmatter and re-ingest before renaming
  a project that owns records.

# CAVEATS

A description or status edited on two machines now reconciles: both
kb-merge(1) and kb-import(1) adopt whichever side's
updated_at is later (DR-0025), so a merge keeps the newer edit rather than
always keeping the first one applied. A *rename* is not covered by this --
merge/import still dedupe projects by name, so a project renamed on one
machine and left untouched on another arrives as two separate projects,
not one renamed one. See DR-0024/DR-0025 (knowledge/decisions/).

# SEE ALSO

kb-observation(1), kb-link(1), kb-search(1),
kb-merge(1)

