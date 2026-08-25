package main

// HelpText is the primary kb(1) man page. Shown by bare `kb -h`/`kb help`.
// Generates kb.1.md.
const HelpText = `%{app_name}(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name} — command-line and interactive interface for a knowledge base

# SYNOPSIS

{app_name} [--db PATH] [--json] VERB [args...]

{app_name}

{app_name} help | -h | --help [VERB]

# DESCRIPTION

{app_name} reads and writes a github.com/rsdoiel/knowledge knowledge base:
projects, observations, concepts, and sources, with full-text search and a
cross-machine merge tool. Every verb follows the "TOOL VERB PARAMETERS"
model (the same shape as git and go), so scripts and other language-model
harnesses can drive it directly, not just people at a terminal.

Run with no verb at all to launch the interactive browser (TUI) instead —
a read-only view over the same data, for exploring projects, observations,
and concepts, and running searches, without leaving the terminal.

# GLOBAL FLAGS

--db PATH
: path to knowledge.db (default: ./agents/knowledge.db, relative to the
  current directory)

--json
: machine-readable JSON output instead of human-readable text. Applies to
  every verb. Errors always go to stderr, never stdout, in both modes —
  scripts consuming --json output can rely on stdout staying valid JSON
  even when a call fails.

--debug
: write a JSONL trace of every knowledge-base call (and, in the TUI,
  every input event and view change) to ./kb-debug-<timestamp>.jsonl in
  the current directory. The path is printed to stderr once at startup.
  Applies to every verb and the TUI. Omitting --debug costs nothing —
  no file is written and behavior is unchanged.

# VERBS

project
: manage projects — see {app_name}-project(1)

observation
: manage observations — see {app_name}-observation(1)

concept
: manage concepts — see {app_name}-concept(1)

link
: link projects/observations to concepts — see {app_name}-link(1)

source
: manage cited sources and retraction checking — see {app_name}-source(1)

search, summary, format
: full-text search and formatted views — see {app_name}-search(1)

merge
: reconcile two knowledge.db files that drifted independently (e.g. across
  machines) into a fresh, deduped output — see {app_name}-merge(1)

export, import
: write/read a portable JSON-L snapshot of the database — the no-file-access
  alternative to merge, for syncing over a channel that can only move plain
  text (paste, email, git) — see {app_name}-export(1) and {app_name}-import(1)

# EXIT STATUS

0
: success

1
: the verb ran but failed (database error, not found, etc.)

2
: usage error (bad flags, unknown verb)

# SEE ALSO

{app_name}-project(1), {app_name}-observation(1), {app_name}-concept(1),
{app_name}-link(1), {app_name}-source(1), {app_name}-search(1),
{app_name}-merge(1), {app_name}-export(1), {app_name}-import(1)

`

// ProjectHelpText is shown by `kb project -h` and `kb help project`.
// Generates kb-project.1.md.
const ProjectHelpText = `%{app_name}-project(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-project — manage projects

# SYNOPSIS

{app_name} project add [--status concept|active|paused|concluded] NAME [DESCRIPTION]

{app_name} project list

{app_name} project show NAME

{app_name} project concepts NAME

{app_name} project set-status NAME STATUS

# DESCRIPTION

A project is the top-level container observations and concepts attach to.
Names are unique; adding a project with an existing name is a no-op that
returns the existing project's id (its status, if any, is left unchanged).

add
: create a project, or return the id of the existing one with that name.
  --status sets the initial status (default: active).

list
: list every project (bare rows — see {app_name}-format(1) for an
  assembled Markdown view with linked concepts/observations included)

show
: show a single project by name

concepts
: list the concepts linked to a project

set-status
: change an existing project's status. STATUS must be one of concept,
  active, paused, concluded.

# SEE ALSO

{app_name}-observation(1), {app_name}-link(1), {app_name}-search(1)

`

// ObservationHelpText is shown by `kb observation -h` and `kb help observation`.
// Generates kb-observation.1.md.
const ObservationHelpText = `%{app_name}-observation(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-observation — manage observations

# SYNOPSIS

{app_name} observation add --project NAME KIND BODY [--source-doi DOI]

{app_name} observation list --project NAME

{app_name} observation show ID

{app_name} observation sources ID

# DESCRIPTION

An observation is a timestamped note attached to a project. KIND is one of
note, finding, decision, question, or hypothesis.

add
: record a new observation under --project; --source-doi records the
  normalized DOI of the paper it was extracted from, if any

list
: list a project's observations, most recent first

show
: show a single observation by id

sources
: list the sources cited by an observation (see {app_name}-source(1))

# SEE ALSO

{app_name}-project(1), {app_name}-link(1), {app_name}-source(1)

`

// ConceptHelpText is shown by `kb concept -h` and `kb help concept`.
// Generates kb-concept.1.md.
const ConceptHelpText = `%{app_name}-concept(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-concept — manage concepts

# SYNOPSIS

{app_name} concept add NAME [DESCRIPTION] [--identifier-type T --identifier-value V]

{app_name} concept list

# DESCRIPTION

A concept is a named idea or term that can be linked to projects and
observations (see {app_name}-link(1)). Names are unique.

A concept may also represent a scholarly entity — a paper, person,
institution, or funder — by setting --identifier-type (e.g. doi, orcid,
ror, fundref) and --identifier-value (the normalized identifier).

# SEE ALSO

{app_name}-link(1), {app_name}-project(1)

`

// LinkHelpText is shown by `kb link -h` and `kb help link`.
// Generates kb-link.1.md.
const LinkHelpText = `%{app_name}-link(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-link — link projects/observations to concepts

# SYNOPSIS

{app_name} link project PROJECT_NAME CONCEPT_NAME

{app_name} link observation OBS_ID CONCEPT_NAME

# DESCRIPTION

Links are many-to-many and idempotent — linking the same pair twice is a
silent no-op.

# SEE ALSO

{app_name}-project(1), {app_name}-observation(1), {app_name}-concept(1)

`

// SourceHelpText is shown by `kb source -h` and `kb help source`.
// Generates kb-source.1.md.
const SourceHelpText = `%{app_name}-source(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-source — manage cited sources and retraction checking

# SYNOPSIS

{app_name} source add TITLE [--doi D] [--url U] [--authors A] [--published DATE] [--publisher P] [--rights R] [--version V]

{app_name} source list

{app_name} source show ID

{app_name} source remove ID

{app_name} source retract ID NOTE

{app_name} source link OBS_ID SOURCE_ID [--relationship R]

{app_name} source check-retractions

# DESCRIPTION

A source is a cited work (paper, page, dataset) an observation can link to
via source link, recording the relationship (default: cited).

add
: register a source; --doi/--url set its identifier (doi takes priority
  if both are given); adding one whose identifier already exists returns
  the existing source's id instead of duplicating it

remove
: delete a source — fails if it's still linked to any observation

retract
: mark a source retracted with a note (does not delete it)

check-retractions
: query the Retraction Watch API for every registered, non-retracted DOI
  source and mark hits as retracted; requires network access

# SEE ALSO

{app_name}-observation(1)

`

// SearchHelpText is shown by `kb search -h`, `kb summary -h`, `kb format
// -h`, and `kb help search`. Generates kb-search.1.md.
const SearchHelpText = `%{app_name}-search(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-search — full-text search and formatted views

# SYNOPSIS

{app_name} search TERM

{app_name} summary

{app_name} format [--project NAME]

# DESCRIPTION

search
: full-text search across observations, projects, and concepts using the
  FTS5 index. TERM uses standard FTS5 query syntax: multiple words are
  ANDed, "quoted phrases" match exactly, prefix* matches by prefix.

summary
: a formatted overview of every project and its most recent observations

format
: a fully-assembled Markdown view of one project (--project NAME) or every
  project (no --project) — concepts and observations included inline,
  unlike the bare rows {app_name}-project(1)'s show/list return

# SEE ALSO

{app_name}-project(1), {app_name}-observation(1)

`

// MergeHelpText is shown by `kb merge -h` and `kb help merge`.
// Generates kb-merge.1.md.
const MergeHelpText = `%{app_name}-merge(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-merge — reconcile two knowledge.db files that drifted independently

# SYNOPSIS

{app_name} merge -a PATH -b PATH -out PATH [-force]

# DESCRIPTION

merge reads two knowledge.db files (e.g. from two machines that have
drifted independently) read-only and writes their deduped union to a
fresh -out file, which must not already exist. It never modifies -a or
-b; placing the merged file into position is left to you.

Unlike every other verb, merge operates entirely on the explicit -a/-b/-out
paths — it ignores --db and never opens (or creates) the ambient
./agents/knowledge.db.

If a project or concept with the same name exists in both files under
different internal identities (a collision — typically from before a
database's identifiers were established), merge aborts and lists them
unless -force is given, in which case b's identity is reconciled to a's so
both sides' observations and links survive under one merged entity.

# SEE ALSO

{app_name}(1)

`

// ExportHelpText is shown by `kb export -h` and `kb help export`.
// Generates kb-export.1.md.
const ExportHelpText = `%{app_name}-export(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-export — write a portable JSON-L snapshot of the database

# SYNOPSIS

{app_name} export [-project NAME] [-out PATH]

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

{app_name}(1), {app_name}-import(1), {app_name}-merge(1)

`

// ImportHelpText is shown by `kb import -h` and `kb help import`.
// Generates kb-import.1.md.
const ImportHelpText = `%{app_name}-import(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-import — apply a JSON-L snapshot (from export) to the database

# SYNOPSIS

{app_name} import [-in PATH]

# DESCRIPTION

import reads a JSON-L stream produced by export — from -in, or stdin when
-in is omitted — and applies it to the already-open --db database.
Projects and concepts are matched by name (an existing local row always
wins as-is; a genuinely new one keeps its original uuid, for future
cross-machine merge compatibility). Sources are matched by identifier when
one is present. Observations and links are matched by uuid, so re-running
import against the same file is a no-op the second time.

Unresolvable references (a uuid the file never defines a parent for) and
unrecognized record types are skipped, not fatal — only malformed JSON
aborts the import. The returned summary reports, per record type, how many
lines were read, newly imported, or skipped.

# SEE ALSO

{app_name}(1), {app_name}-export(1), {app_name}-merge(1)

`

// IngestHelpText is the kb-ingest(1) man page.
const IngestHelpText = `%{app_name}-ingest(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-ingest — index a tree of decision records into the knowledge base

# SYNOPSIS

{app_name} ingest PATH [--dry-run] [--root DIR]

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
{app_name} ingest clasm/decisions
{app_name} ingest agents/decisions
~~~

Preview without writing:

~~~shell
{app_name} ingest clasm/decisions --dry-run
~~~

`

// RecordHelpText is the kb-record(1) man page.
const RecordHelpText = `%{app_name}-record(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-record — read and maintain decision records

# SYNOPSIS

{app_name} record list [--project P] [--workspace] [--status S] [--kind K] [--trigger T] [--initiative I] [--since DATE]

{app_name} record show RECORD_ID [--project P] [--workspace]

{app_name} record set-status RECORD_ID STATUS [--project P] [--workspace] [--root DIR]

{app_name} record supersede NEW OLD [--partial] [--project P] [--workspace] [--root DIR]

# DESCRIPTION

A decision record is one file under a project's decisions/ directory, indexed
by ingest. Records are listed oldest first, sorted by date and then by id —
never by id alone, because ids are identity, not chronology: a correction can
carry a lower id than the record it supersedes.

A record id is not by itself an identity, since two projects may each have a
DR-0001. Where a bare id is ambiguous, the command reports the candidates and
asks for --project or --workspace rather than choosing one.

list
: print matching records, one per line

show
: print one record with its body and its relations resolved in both
  directions. Only supersedes is stored; superseded_by is its inverse

set-status
: set a record's status in both its file and the database. The promotion path
  from proposed to accepted

supersede
: write both sides of a supersession — supersedes on NEW, superseded_by on
  OLD, the relation, and unless --partial, OLD's superseded status. Both
  files and the database are written together or not at all

set-status and supersede are the only commands that write a record file;
ingest never does.

# VOCABULARIES

These are the documented values. They are reported against, not enforced: an
unknown value parses and carries a warning, because a typo in a file several
harnesses write should be a fixable row, not a failed run.

status
: proposed, accepted, superseded, rejected

kind
: decision, correction, refinement

trigger
: design, plan-review, implementation, live-test, release-review, request,
  external. May be empty on a record converted from an existing log

# OPTIONS

--project P
: restrict to, or resolve within, project P

--workspace
: restrict to, or resolve within, the workspace tier

--partial
: on supersede, leave OLD accepted instead of marking it superseded. Use when
  a later record invalidates one decision inside a multi-decision episode
  while the rest still stand

--root DIR
: the workspace root that stored record paths are relative to. Defaults to
  the parent of the directory holding the database

# EXAMPLES

Every correction in one project, and everything since a date:

~~~shell
{app_name} record list --project clasm --kind correction
{app_name} record list --project clasm --since 2026-08-01
~~~

Promote a proposed record, then wholly and partially supersede:

~~~shell
{app_name} record set-status 0004 accepted --project knowledge
{app_name} record supersede 0149 0148 --project clasm
{app_name} record supersede 0159 0160 --project clasm --partial
~~~

`
