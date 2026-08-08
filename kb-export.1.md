%kb-export(1) user manual | version 0.0.3 f716050
% R. S. Doiel
% 2026-08-08

# NAME

kb-export — write a portable JSON-L snapshot of the database

# SYNOPSIS

kb export [-project NAME] [-out PATH]

# DESCRIPTION

export writes the knowledge base (or, with -project, just one project and
everything reachable from it — its concepts, sources, and observations) as
newline-delimited JSON to -out, or to stdout when -out is omitted. Every
line is self-describing via a "type" field (project, concept, source,
observation, observation_concept, project_concept, observation_source),
in dependency order.

Unlike merge, export never touches a second database file — the resulting
file can be pasted, emailed, or committed to git, then applied elsewhere
with import. This is the no-file-access alternative to merge; when both
databases are reachable as files, merge is the more thorough tool (it
also detects and can reconcile name collisions).

With --json, a text confirmation is only meaningful once -out is given
(the JSON-L stream itself has already gone to stdout otherwise): it
becomes a {"lines_written": N, "path": "..."} object instead of the plain
text line.

# SEE ALSO

kb(1), kb-import(1), kb-merge(1)

