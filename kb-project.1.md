%kb-project(1) user manual | version 0.0.10 f3d323d
% R. S. Doiel
% 2026-09-18

# NAME

kb-project — manage projects

# SYNOPSIS

kb project add [--status concept|active|paused|concluded] NAME [DESCRIPTION]

kb project list

kb project show NAME

kb project concepts NAME

kb project set-status NAME STATUS

kb project set-description NAME DESCRIPTION

kb project rename [--root PATH] [--dry-run] OLD NEW

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
  another project. If the project owns records, first rewrites every owned
  record's project: frontmatter to NEW -- both-or-neither, rolling back
  every file already written if any write fails -- before renaming the
  project row; no record's database row is touched, so the next
  kb-ingest(1) of that corpus sees a changed checksum against an
  unchanged identity and updates in place rather than minting a phantom
  project. --dry-run reports which files would be rewritten without
  writing anything. --root sets the workspace root record paths are
  relative to (default: inferred from the database path). See DR-0026
  (knowledge/decisions/), which supersedes DR-0024's outright refusal.

# CAVEATS

A description, status, or name edited on two machines now reconciles: both
kb-merge(1) and kb-import(1) adopt whichever side's
updated_at is later (DR-0025, generalized to name by DR-0026), so a project
renamed on one machine and left untouched on another arrives as one renamed
project, not two, regardless of merge/import order. See
DR-0024/DR-0025/DR-0026 (knowledge/decisions/).

# SEE ALSO

kb-observation(1), kb-link(1), kb-search(1),
kb-merge(1)

