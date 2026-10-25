%kb-stats(1) user manual | version 0.0.27 b0e505d
% R. S. Doiel
% 2026-10-25

# NAME

kb-stats — row, link and check counts for the database

# SYNOPSIS

kb stats

# DESCRIPTION

Reports the shape of the database without changing it, so a question such as
"how many concept links are there" or "does any observation have no project"
needs no SQL. It takes no arguments.

The report has four parts.

rows
: how many rows each table has: projects, observations, concepts, sources,
  records, documents and document_sections.

links
: how many rows each join table has: project_concepts, observation_concepts,
  record_concepts, document_section_concepts, observation_sources,
  document_sources, record_sources, observation_relations and
  record_relations.

checks
: facts worth knowing, with a count. observations_without_project is normally
  0. concepts_without_links counts concepts nothing links to, which is not an
  error (a concept can be added before it is used); kb-concept(1)
  delete removes one. sources_without_links counts sources no observation,
  document or record cites. records_without_project counts project-scope records with no project
  row, and documents_without_project counts documents with none; workspace-tier
  records have no project by design and are not counted.

projects
: one line per project, by name: observations, records, documents and concepts.
  A project's concepts are those tagged on the project, on its observations and
  on its records, each counted once, as kb-search(1) scopes them.

The counts are the answer, so the exit status is 0 whatever they say. Use
kb-check-db(1) to ask whether the database has drifted from its dump.
kb-concept(1) show says which projects one concept reaches.

With --json the report is one object: rows, links and checks (each a map from
name to count) and projects (a list of name, observations, records, documents
and concepts).

# EXIT STATUS

The workspace convention, as described in kb(1).

0
: the report was written

2
: a flag or a surplus argument

66
: no workspace or database found

# SEE ALSO

kb(1), kb-check-db(1), kb-concept(1), kb-search(1)

