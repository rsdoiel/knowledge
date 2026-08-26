%kb-record(1) user manual | version 0.0.4 14a471f
% R. S. Doiel
% 2026-08-26

# NAME

kb-record — read and maintain decision records

# SYNOPSIS

kb record list [--project P] [--workspace] [--status S] [--kind K] [--trigger T] [--initiative I] [--since DATE]

kb record show RECORD_ID [--project P] [--workspace]

kb record set-status RECORD_ID STATUS [--project P] [--workspace] [--root DIR]

kb record supersede NEW OLD [--partial] [--project P] [--workspace] [--root DIR]

kb record new --title T --trigger G (--project P | --workspace) [--kind K] [--root DIR]

kb record fmt PATH [--dry-run]

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

new
: scaffold a record: allocate the next id for the tier, fill the fields a
  tool owns, set status to proposed, and print all five body headings whether
  or not they get filled. Writes the file; does not ingest it. --trigger is
  required here even though a converted record may carry an empty one,
  because on a newly authored record it is cheap and accurate to say where
  the need was discovered

fmt
: rewrite every record under PATH into canonical form. This is the
  normalisation path ingest deliberately lacks, since ingest never writes to
  a record file

new, set-status, supersede and fmt are the only commands that write a record
file; ingest never does. A record is written proposed and stays proposed: a
model may write a record, but only the author accepts one.

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

--title T
: the new record's title, on new. The filename slug is derived from it,
  lowercased with punctuation stripped; the slug is cosmetic and the id is
  the identity

--kind K
: the new record's kind, on new. Defaults to decision

--dry-run
: on fmt, report what would change and write nothing

--root DIR
: the workspace root that stored record paths are relative to. Defaults to
  the parent of the directory holding the database

# EXAMPLES

Every correction in one project, and everything since a date:

~~~shell
kb record list --project clasm --kind correction
kb record list --project clasm --since 2026-08-01
~~~

Promote a proposed record, then wholly and partially supersede:

~~~shell
kb record set-status 0004 accepted --project knowledge
kb record supersede 0149 0148 --project clasm
kb record supersede 0159 0160 --project clasm --partial
~~~

Start a new record, and bring a corpus into canonical form:

~~~shell
kb record new --project clasm --title "Retry the profile attach" --trigger live-test
kb record fmt clasm/decisions --dry-run
~~~

