%kb-search(1) user manual | version 0.0.27 b0e505d
% R. S. Doiel
% 2026-10-25

# NAME

kb-search — full-text search and formatted views

# SYNOPSIS

kb search TERM

kb search SCOPE/ TERM

kb search SCOPE/KIND TERM

kb summary

kb format [--project NAME]

# DESCRIPTION

search
: full-text search across observations, records, projects, concepts, sources
  (title, authors, publisher and identifier) and documents' reviewed summaries
  using the FTS5 index. TERM uses standard FTS5
  query syntax: multiple words are ANDed, "quoted phrases" match exactly,
  prefix* matches by prefix. A first word that starts with a dash is read as
  a mistyped option and refused (exit 2); give a dash-leading term after --,
  as in kb search -- -x. Later words are text as typed, dashes
  included. Finding nothing is exit 1.

  A first argument with a slash in it says where to look. SCOPE is a project
  name, workspace (the workspace tier only, as for records), the workspace
  directory's own name as an alias for workspace (a project of that name
  wins), or nothing, which means everywhere. KIND is one of records,
  observations, concepts, documents, sources and projects, or nothing for every
  kind. A project's sources are those its observations, documents and records
  cite; the workspace tier's are those cited by workspace-tier records and by
  documents with no project.

      kb search "exit codes"              every kind, everywhere
      kb search harvey/ "exit codes"      one project, every kind
      kb search harvey/records "exit codes"
      kb search workspace/ "exit codes"   the workspace tier only
      kb search /records "exit codes"     records in every scope

  A first word with no slash is part of the term, so kb search harvey
  memory is still harvey AND memory. A first argument that looks like a scope
  and is not one is a usage error (exit 2) that names the choices; text that
  contains a slash is given after --.

  A scoped search prints its matches grouped by kind with counts. A record
  is named by its qualified reference (harvey/DR-0004), and a concept shows
  the projects it is in, and a source the projects that cite it. At most 50 rows are printed; the line "showing 50 of
  N" says when more matched. With the global -json option it prints one
  object: scope, term, total, counts and results.

  Every search, scoped or not, shows where a hit is. A record hit carries its
  qualified reference and status ("harvey/DR-0004 (accepted)"), so the next
  step is kb record show REF. Under a hit, when part of the text
  matched, a line shows that passage with the matched words between « and »;
  for a record it starts with the section the match is in, the bold label of
  the paragraph ("Decision:"), or "title:" when it is in the title. In -json
  the fields are ref, status, passage and section (the unscoped list uses the
  library's names: Ref, Status, Passage, Section). Observations, concepts and
  projects have no reference or status, and a concept or project has no passage.

  A hit also says what supports it (DR-0080). In -json, sources lists the works
  its row cites (id, title, identifier, version, rights, retracted,
  retraction_note, relationship, and the version and hash that were read),
  material gives the file a document or record came from (path, checksum,
  format, ingested_at), and retracted is true when any cited work is retracted;
  an unscoped list carries the same as Sources, Material and Retracted. A hit
  that cites a retracted work ends with [retracted source: TITLE], one for each
  such work. This is a report: a retracted citation does not rank a hit lower,
  higher or out of the results.

  A project's concepts are those tagged on the project, on any of its
  observations and on any of its records. Projects are searched at the global
  scope only: workspace/observations and harvey/projects are exit 1 with the
  reason, since observations belong to projects and a project has no projects
  inside it. A document is found by its reviewed summaries.

summary
: a formatted overview of every project and its most recent observations

format
: a fully-assembled Markdown view of one project (--project NAME) or every
  project (no --project) — concepts and observations included inline,
  unlike the bare rows kb-project(1)'s show/list return

# EXIT STATUS

The workspace convention, as described in kb(1). This page also covers
summary and format.

0
: search found something, or summary or format printed its view

1
: search found nothing; a scope with no members (workspace/observations,
  harvey/projects); or format --project names a project that does not exist

2
: no TERM, a first word that starts with a dash and is not after --, an unknown
  flag, a scope or kind that does not exist, or a surplus argument to summary

# SEE ALSO

kb-project(1), kb-observation(1)

