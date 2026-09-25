%kb-unlink(1) user manual | version 0.0.14 e470e93
% R. S. Doiel
% 2026-09-25

# NAME

kb-unlink — remove a link between a project or observation and a concept, or an observation and a source

# SYNOPSIS

kb unlink project PROJECT_NAME CONCEPT_NAME

kb unlink observation OBS_ID CONCEPT_NAME

kb unlink source OBS_ID SOURCE_ID

# DESCRIPTION

The inverse of kb-link(1) and of kb source link: it removes one link and
nothing else, so the project, observation, concept or source itself is untouched.
Concept and project names match exactly, including case, as for kb concept
delete: a destructive command must not guess. Removing a link that does not exist is
an error (exit 1), not a silent success, and so is naming a project, concept,
observation or source that does not exist.

To remove a concept from everything at once, see kb-concept(1)'s delete.
Like every delete, an unlink is local to one database: kb-merge(1) and
kb-import(1) from a database that still has the link bring it back.

# SEE ALSO

kb-link(1), kb-source(1), kb-concept(1), kb-observation(1)

