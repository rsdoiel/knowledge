%kb-record(1) user manual | version 0.0.17 2547ff0
% R. S. Doiel
% 2026-10-08

# NAME

kb-record — read and maintain decision records

# SYNOPSIS

kb record pending [SCOPE...] [--all] [--kind K] [--trigger T] [--initiative I] [--since DATE]

kb record list [SCOPE...] [--all] [--status S] [--kind K] [--trigger T] [--initiative I] [--since DATE]

kb record show RECORD_REF [--project P] [--workspace]

kb record set-status RECORD_REF STATUS [--project P] [--workspace] [--root DIR]

kb record supersede NEW_REF OLD_REF [--partial] [--project P] [--workspace] [--root DIR]

kb record new --title T --trigger G [--project P | --workspace] [--kind K] [--dir DIR] [--root DIR]

kb record fmt PATH [--dry-run]

kb record concepts RECORD_REF [--project P] [--workspace]

kb record fuzzy-tag --project P [--concept NAME,...] [--write] [--dry-run] [--root DIR]

kb record delete RECORD_REF [--project P] [--workspace] [--root DIR] [--dry-run]

# DESCRIPTION

A decision record is one file, indexed by ingest. new writes a project-scoped
record to agents/projects/PROJECT/decisions/ by default (--dir overrides
this) and a workspace-scoped one to agents/decisions/. Records are listed
oldest first, sorted by date and then by id — never by id alone, because ids
are identity, not chronology: a correction can carry a lower id than the
record it supersedes.

A record id is not by itself an identity, since two projects may each have a
DR-0001, and listings print the qualified form for that reason. A record is named by SCOPE/DR-NNNN, where SCOPE is a project name or
workspace (the workspace directory's own name is accepted as an alias for it,
and a project of that name wins): harvey/DR-0004, workspace/DR-0003. A bare id
resolves only where one record has it; otherwise the command lists the
qualified candidates rather than choosing one. A command that changes a record
(set-status, supersede, delete) never acts on a bare id with no scope: say
harvey/DR-0004, run it from inside the project, or set KB_PROJECT.

Scope. list takes scopes as arguments: kb record list harvey clasm workspace.
With none it uses the project the working directory belongs to (under
agents/projects/NAME/, or in a repository directory NAME/ beside it), else
the project KB_PROJECT names, else the whole workspace; --all widens it to the
whole workspace. An unknown scope is exit 1. --project and --workspace remain
as aliases for one scope and cannot be mixed with scope arguments or --all.

list
: print matching records, one per line, each starting with its SCOPE/DR-NNNN
  reference. --status, --kind, --trigger and
  --initiative filter on those fields and --since DATE (YYYY, YYYY-MM or
  YYYY-MM-DD) keeps records dated on or after it; filters combine.
  "no matching records" means a real filter matched nothing. A value that
  no record carries and the vocabularies below do not list is a typo, and
  is an error that names what is known (exit 1, a lookup that found nothing;
  record new and set-status, which write a value, exit 2 for the same thing).
  A bare record id that exists in more than one tier is ambiguous and exits 2,
  listing the qualified forms. A value outside the vocabularies
  that some record does carry still filters. --workspace and --project
  cannot be combined, since workspace-tier records have no project

pending
: list the records waiting for a decision: every record whose status is
  proposed, oldest first, in the scope list would use. The other list filters
  apply; --status does not (exit 2), since the status is what pending means.
  Nothing pending is success

show
: print one record with its body and its relations resolved in both
  directions, every one named SCOPE/DR-NNNN. Only supersedes is stored;
  superseded_by is its inverse

set-status
: set a record's status in both its file and the database. The promotion path
  from proposed to accepted. Also refreshes the corpus's index.md if one is
  already present, since status is one of the fields it renders — never
  creates one where the corpus has not already opted in. A status outside the
  vocabularies below is refused (exit 2) unless a record already carries it, the
  rule record new applies to --trigger and --kind and record list applies to its
  filters, so a typo such as "acepted" cannot leave a record in limbo; the file and
  database are untouched. Ingest of a hand-edited file still only warns.
  Only the moves in the transition table are made: proposed to accepted,
  rejected or superseded; accepted to cancelled or superseded; rejected or
  cancelled back to proposed; superseded is final. superseded also needs
  superseded_by already set, so use record supersede, which writes both sides.
  A move outside the table, including setting the status a record already has,
  is refused (exit 1) with the allowed moves named, and nothing is written. The
  table applies only when the current and the requested status are both in the
  vocabulary, so a record carrying some other value can still be repaired

supersede
: write both sides of a supersession — supersedes on NEW, superseded_by on
  OLD, the relation, and unless --partial, OLD's superseded status. Both
  files and the database are written together or not at all. Also refreshes
  index.md, same as set-status, for both NEW's and OLD's corpora if either
  already has one

new
: scaffold a record: allocate the next id for the tier, fill the fields a
  tool owns, set status to proposed, and print all five body headings whether
  or not they get filled. Writes the file; does not ingest it. --trigger is
  required here even though a converted record may carry an empty one,
  because on a newly authored record it is cheap and accurate to say where
  the need was discovered. --trigger and --kind follow the rule record list
  uses for its filters: a value outside the vocabularies below is refused
  unless a record already in the database carries it, and the error names
  what is known. That catches a typo before it is written into a file, where
  it would otherwise become a value the filter rule accepts

fmt
: rewrite every record under PATH into canonical form. This is the
  normalisation path ingest deliberately lacks, since ingest never writes to
  a record file

concepts
: list the concepts ingest linked to a record, from [[Name]] wikilinks in its
  body and its frontmatter tags list

fuzzy-tag
: report near-miss spellings of known concepts in a project's records (a
  detoast for the concept toast), which exact matching never links. Per
  record it lists the concept, the variants found, how many times, the plain
  exact mentions that link nothing (reported as "exact mention, not linked")
  and the tags: line that would link the concept. Only the body is searched,
  never the frontmatter. A concept is skipped for a record only when the record
  already links it, in tags: or as a [[wikilink]]. Without --concept the same
  conservative length and distance rules as kb-document(1)'s fuzzy-tag
  apply; --concept NAME,... names concepts whose variants are reported up to the
  matcher's ceiling (a 5-letter concept such as toast is under the default length
  floor, so name it). By default nothing is written. --write adds the concept to
  the tags: of records whose status is proposed and reports how many accepted
  records it skipped; an accepted record is history and is never modified, nor is
  any other status. Only the tags: line changes (a one-line flow list, a block
  list, or a new line if absent); a form it will not edit is refused and nothing
  is written, since writes are both-or-neither. --dry-run with --write says what
  would happen and writes nothing. The database is not touched: run
  kb ingest on the records directory to link the new tags. An unknown
  project or concept is exit 1. See DR-0052 (knowledge/decisions/).

delete
: drop the database row of a record whose file is already gone: its relations, its
  concept links and its search entry go with it. A record's file is the truth and
  ingest is additive, so a row whose file vanished otherwise stays (ingest only
  reports it). While the file exists this is refused (exit 1), because the next
  ingest of the changed file would only undo it, and kb never deletes a
  decision record from disk: delete the file yourself first, or retire the record
  with set-status cancelled. --dry-run reports and changes nothing.

new, set-status, supersede, fmt and fuzzy-tag --write are the only commands that
write a record file; ingest never does. A record is written proposed and stays proposed: a
model may write a record, but only the author accepts one.

# EXIT STATUS

The workspace convention, as described in kb(1). A scope that is neither a
project nor the workspace is 1, and so is a set-status move the transition
table does not allow; a malformed reference, a bare id that is
ambiguous, a change (set-status, supersede, delete) given a bare id with no
scope, and record new with no scope anywhere are 2. For record fuzzy-tag:
0 the report was produced, or the tags were written, including "nothing found";
1 no such project or concept; 2 a bad flag, a missing --project, or a surplus
argument, and on any record verb a --concept or --write it does not take; 65 a
record file that is malformed or whose tags: is in a form the edit will not
change (nothing is written); 66 a record's file is missing, or there is no
workspace here; 74 a write failed part way (writes already made are undone).

# VOCABULARIES

These are the documented values. They are reported against, not enforced: an
unknown value parses and carries a warning, because a typo in a file several
harnesses write should be a fixable row, not a failed run.

status
: proposed, accepted, superseded, rejected, cancelled

  rejected means the decisions were never adopted. superseded means a later
  record replaced them. cancelled means they were adopted and then the work was
  abandoned: its reasoning still stands and is where anyone revisiting the
  question should start, but it is not live. Say why in the record's body, since
  "the need was met another way" and "deprioritised" tell a later reader
  different things; there is no field for it. Use kb record set-status ID
  cancelled, without writing a replacement record.

kind
: decision, correction, refinement

trigger
: design, plan-review, implementation, live-test, release-review, request,
  external. May be empty on a record converted from an existing log

# OPTIONS

--all
: on list and pending, every project and the workspace tier, whatever the current
  directory is

--project P
: restrict to, or resolve within, project P. On new and fuzzy-tag it is how the
  project is named. Elsewhere it is deprecated: give the scope as an argument to
  list and pending, or qualify the record as P/DR-NNNN. It still works and says
  so on standard error

--workspace
: restrict to, or resolve within, the workspace tier. On new it names the tier.
  Elsewhere it is deprecated in the same way: use workspace as the scope or
  workspace/DR-NNNN

--partial
: on supersede, leave OLD accepted instead of marking it superseded. Use when
  a later record invalidates one decision inside a multi-decision episode
  while the rest still stand

--title T
: the new record's title, on new. The filename slug is derived from it,
  lowercased with punctuation stripped; the slug is cosmetic and the id is
  the identity

--kind K
: the new record's kind, on new. Defaults to decision

--dir DIR
: on new, write to DIR instead of the default (agents/projects/PROJECT/decisions,
  or agents/decisions on --workspace). Relative to --root, like every other
  stored record path

--dry-run
: on fmt, report what would change and write nothing

--root DIR
: the workspace root that stored record paths are relative to. Defaults to
  the parent of the directory holding the database

# EXAMPLES

Every correction in one project, and everything since a date, then two scopes
at once and the workspace tier:

~~~shell
kb record list clasm --kind correction
kb record list clasm --since 2026-08-01
kb record list clasm cold workspace
~~~

What is waiting for a decision, in every project or in one:

~~~shell
kb record pending --all
kb record pending knowledge
~~~

Inside a project directory the project is the scope, and --all widens it:

~~~shell
cd ~/Laboratory/harvey && kb record list
cd ~/Laboratory/harvey && kb record list --all
~~~

Promote a proposed record, then wholly and partially supersede. A change names
the record by its qualified reference, since a bare id is not enough to act on:

~~~shell
kb record set-status knowledge/DR-0004 accepted
kb record supersede clasm/DR-0149 clasm/DR-0148
kb record supersede clasm/DR-0159 clasm/DR-0160 --partial
~~~

Start a new record, and bring a corpus into canonical form:

~~~shell
kb record new --project clasm --title "Retry the profile attach" --trigger live-test
kb record new --title "Run from inside the project" --trigger design
kb record new --project clasm --title "Filed under the old layout" --trigger request --dir clasm/decisions
kb record fmt clasm/decisions --dry-run
~~~

