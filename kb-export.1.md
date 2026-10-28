%kb-export(1) user manual | version 0.0.28 c100b07
% R. S. Doiel
% 2026-10-28

# NAME

kb-export — write a portable JSON-L snapshot of the database

# SYNOPSIS

kb export [-project NAME] [-out PATH]

# DESCRIPTION

export writes the knowledge base (or, with -project, just one project and
everything reachable from it — its concepts, sources, observations and
decision records) as newline-delimited JSON to -out, or to stdout when
-out is omitted. Every line is self-describing via a "type" field
(project, concept, source, observation, observation_concept,
observation_relation, project_concept, observation_source, record,
record_relation, record_concept, document, document_section,
document_section_concept, document_source, record_source), in dependency
order. A document_source or record_source line joins a document or a decision
record to a source it cites (DR-0081), and carries the cited version and hash;
a -project export carries the sources the project's observations, documents and
records cite.

A concept line carries its "kind" (character, place, theme or other).

Story events (story_events, DR-0075) and the story knowledge kept with them
(story_knowledge, DR-0083) are not exported: they are derived from
the manuscript files by kb-document(1) ingest and are rebuilt there.

A -project export carries only that project's decision records — a
workspace-tier record belongs to no project, so it has no principled claim
on a scoped export, and appears only in an unscoped one. A record relation
crossing out of the scoped project (to another project, or to the
workspace tier) is likewise excluded along with the record on the far side
of it.

Unlike merge, export never touches a second database file — the resulting
file can be pasted, emailed, or committed to git, then applied elsewhere
with import. This is the no-file-access alternative to merge; when both
databases are reachable as files, merge is the more thorough tool (it
also detects and can reconcile name collisions).

With --json, a text confirmation is only meaningful once -out is given
(the JSON-L stream itself has already gone to stdout otherwise): it
becomes a {"lines_written": N, "path": "..."} object instead of the plain
text line.

# EXIT STATUS

The workspace convention, as described in kb(1).

0
: the dump was written. An -out that names a file that already exists replaces
  it

1
: -project names a project that does not exist

2
: a bad flag, or a surplus argument

73
: -out cannot be created: it names a directory, or its directory is missing

74
: a write failed part way, for example a full disk

77
: the operating system refused the write to -out

# SEE ALSO

kb(1), kb-import(1), kb-merge(1)

