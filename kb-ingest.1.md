%kb-ingest(1) user manual | version 0.0.4 0fe99a8
% R. S. Doiel
% 2026-08-26

# NAME

kb-ingest — index a tree of decision records into the knowledge base

# SYNOPSIS

kb ingest PATH [--dry-run] [--root DIR]

# DESCRIPTION

Walks PATH for decision record files named NNNN-slug.md, parses each one's
YAML frontmatter, and upserts it into the records table and the full-text
index. The generated index.md is not a record and is skipped.

A record's identity is its (project, scope, id) triple. Re-ingesting a tree
is cheap and safe to repeat: a file whose checksum is unchanged is skipped.

Ingest runs in two passes. The first stores every record; the second resolves
supersedes and relates_to into relations. A record may cite one the walk has
not reached yet, so a single pass would fail on a forward reference.

A relates_to entry is [SCOPE:]ID — a bare id inherits the citing record's own
project and scope, clasm:0160 names another project, and workspace:0001 names
the workspace tier. An optional DR- prefix is stripped. supersedes and
superseded_by are same-tier only, so a qualified entry in either is reported
as malformed rather than resolved. superseded_by is never stored directly: it
is the inverse of the supersedes on the other record.

Nothing about a reference is fatal. A target that is not in the database yet
leaves the relation unwritten and adds a line to the summary; re-run once it
has been ingested. Failing instead would make ingest order significant, which
is what the two passes exist to avoid.

Ingest is additive. A record whose file has vanished stays in the database and
is reported, never deleted — pruning would destroy data on a partial or
wrong-directory run. Ingest never writes to a record file; only record does.

# OPTIONS

--dry-run
: report the same counts and write nothing

--root DIR
: treat DIR as the workspace root that stored paths are relative to.
  Defaults to the parent of the directory holding the database, so
  --db agents/knowledge.db gives a root of the workspace itself. Paths are
  stored relative because absolute ones do not survive merge between machines.

# EXAMPLES

Index one project's records, then the workspace tier:

~~~shell
kb ingest clasm/decisions
kb ingest agents/decisions
~~~

Preview without writing:

~~~shell
kb ingest clasm/decisions --dry-run
~~~

