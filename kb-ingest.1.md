%kb-ingest(1) user manual | version 0.0.25 d5d8964
% R. S. Doiel
% 2026-10-19

# NAME

kb-ingest — index a tree of decision records into the knowledge base

# SYNOPSIS

kb ingest PATH [--dry-run] [--root DIR]

# DESCRIPTION

Walks PATH for decision record files named NNNN-slug.md, parses each one's
YAML frontmatter, and upserts it into the records table and the full-text
index. The generated index.md is not a record and is skipped. So is a file whose name begins with a
date (2026-09-29-worklog.md), which is a note: notes need no frontmatter and are
not records.

PATH is a directory, and is taken as written relative to the current
directory, not to the workspace root. A project's records live under
agents/projects/NAME/decisions and the workspace tier's under agents/decisions,
so from the workspace root those are the paths to give; from inside a project's
directory, decisions is enough. A file is refused ("is not a directory", exit
66), as is a directory that does not exist. Exactly one PATH is taken (a missing
or surplus one is a usage error, exit 2).

The walk descends into every subdirectory, so agents/projects/NAME ingests the
project's decisions directory and anything else under it that holds record
files, and agents/projects ingests every project at once. Below PATH the walk
skips the project layout's directories that never hold records: design, plans,
feature_requests, notes and hand-off. A directory given as PATH is read whatever
it is called. (kb index, by
contrast, takes one directory and does not descend.)

A record may list the works it cites in an optional sources frontmatter key
(DR-0081), in the same three forms as a document's: a bare DOI, an absolute
URL, or source:ID. Ingest links them through record_sources and rebuilds the
links on every run, an unchanged record included; a record with the key manages
the links it made (relationship cited), and one without it manages none. A
malformed entry is a warning, not a failure. A dry run reports the warnings and
links nothing.

Which project a record belongs to, and whether it is at the workspace tier, is
read from the record's own "project:" frontmatter, never from where the file
sits. A record with an empty project is workspace tier; a project name the
database does not know yet is created. Ingesting the wrong directory therefore
files records under whatever their frontmatter says, which is why --dry-run is
worth running first.

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

A file that cannot be ingested (a record that does not parse, two files claiming
one identity, a file that cannot be read) is reported in the summary and the rest
are still ingested, but the command then exits with the class of the first such
failure, in path order (65 for wrong content, 77 for a file it may not read, and
so on), instead of 0. A warning or an unresolved reference is not a failure.

Nothing about a reference is fatal. A target that is not in the database yet
leaves the relation unwritten and adds a line to the summary; re-run once it
has been ingested. Failing instead would make ingest order significant, which
is what the two passes exist to avoid.

Ingest is additive. A record whose file has vanished stays in the database and
is reported, never deleted — pruning would destroy data on a partial or
wrong-directory run. Ingest never writes to a record file; only record does.

Ingest is how a status written in a record file reaches the database, without the
transition table or the terminal rule that kb record set-status applies: the
file is the source of truth, so a status edited by hand is stored as it stands. The
summary reports it, so the bypass is visible and never silent. A line
"status: clasm/DR-0012 proposed -> accepted (edited in file)" means the stored status
differed from the file's; "status: clasm/DR-0013 arrived accepted" means a record was
first ingested already accepted. It is a report and not a failure, the status is
applied, and a move that set-status would refuse is reported like any other. The text
lists the first 20 (a whole tree ingested into an empty database arrives every
accepted record at once) and says how many it left out; -json kb ingest
lists them all in "status_changes", each with "ref", "from", "to" and "kind"
("edited" or "arrived"). --dry-run reports the same lines and writes nothing. A status
written by set-status is in the file and the database together, so the next ingest
does not report it.

A [[wikilink]] or a tag that matches no concept creates one, silently, so a typo
makes a concept. The summary says which: "concept: created Alpha, Beta" for a run
that wrote, "concept: would create Alpha, Beta" for --dry-run, which creates none;
-json kb ingest lists them all in "concepts_created". A name that differs
only in case from an existing concept is that concept, not a new one, and a
[[DR-0012]] is a record reference and is warned about, not made a concept.

Every [[Name]] found in a record's body, and every entry in its frontmatter
tags list, is resolved to a concept and linked to the record (kb record
concepts shows the result). A name that does not match an existing concept
creates one; matching is case-insensitive, so [[Computer]] and [[computer]]
resolve to the same concept regardless of where each mention falls in a
sentence, and the casing of whichever mention is resolved first becomes
canonical. This does not change kb concept add, which stays exact-match.

A wikilink may carry a display alias after a bar: [[the-long-fire/Anna|Anna]]
links the concept the-long-fire/Anna. Only the part before the bar is the name.

# OPTIONS

--dry-run
: report the same counts and write nothing

--root DIR
: treat DIR as the workspace root that stored paths are relative to, and
  whose base name is the workspace name recorded with each record.
  Defaults to the parent of the directory holding the database, so
  --db agents/knowledge.db gives a root of the workspace itself. Paths are
  stored relative because absolute ones do not survive merge between machines.
  A file outside DIR is stored under its base name only.

# EXAMPLES

From the workspace root, index one project's records, then the workspace tier:

~~~shell
kb ingest agents/projects/clasm/decisions
kb ingest agents/decisions
~~~

Preview without writing:

~~~shell
kb ingest agents/projects/clasm/decisions --dry-run
~~~

Every project's records at once, or from inside a project's directory:

~~~shell
kb ingest agents/projects
cd agents/projects/clasm && kb ingest decisions
~~~

