%kb-observation(1) user manual | version 0.0.29 7d292d7
% R. S. Doiel
% 2026-10-29

# NAME

kb-observation — manage observations

# SYNOPSIS

kb observation add --project NAME KIND BODY [--source-doi DOI]

kb observation add --project NAME --draft --by WHO [--confidence N] KIND BODY

kb observation list --project NAME [--status reviewed|drafted|all]

kb observation show ID

kb observation update ID BODY...

kb observation review list [--project NAME]

kb observation review promote ID

kb observation sources ID

kb observation delete ID [--force] [--dry-run]

# DESCRIPTION

An observation is a timestamped note attached to a project. KIND is one of
note, finding, decision, question, or hypothesis.

add
: record a new observation under --project; --source-doi records the
  normalized DOI of the paper it was extracted from, if any. With --draft the
  observation is a draft, for a model to record what it thinks is worth
  keeping: --by names the drafter (one line, e.g. "harvey qwen2.5:3b"),
  --confidence is the drafter's estimate from 0 to 1, and --source-doi is not
  allowed. A draft is not searchable and is not recalled, listed by default,
  summarised or counted with the observations until a person promotes it

list
: list a project's observations, most recent first. Only reviewed observations
  are listed unless --status says otherwise: drafted lists the drafts still
  awaiting review (a draft that a newer one supersedes is not among them), all
  lists both

show
: show a single observation by id, including its resolved supersedes/
  superseded_by relations if it has any

update
: correct an observation by superseding it, not by mutating it. ID's body
  is never touched; a new observation is inserted with BODY, inheriting ID's
  project and kind, and linked to ID by a supersedes edge. The original
  wording survives unchanged, so nothing needs a separate revision history --
  see DR-0023 (knowledge/decisions/). Cross-machine reconciliation carries
  the new supersedes edge the same as every other relation in this schema;
  there is no per-field last-writer-wins here, unlike
  kb-project(1)'s set-description. Updating a draft gives a draft by the same
  drafter, with no confidence; updating a reviewed observation gives a
  reviewed one. Only promote makes a row trusted

review list
: list the drafts awaiting a person, oldest first, with who drafted each and
  the confidence it gave; --project limits it to one project

review promote
: make a draft reviewed, so it is searchable and recalled. This is a person's
  act: it exits 2 unless standard input and output are a terminal, with no
  flag or environment variable to turn that off, and it checks before it looks
  the id up. A model may draft; it cannot promote. Run it from an interactive
  shell (in Claude Code the ! prefix has no terminal). A draft is rejected with
  delete, which needs no terminal

sources
: list the sources cited by an observation (see kb-source(1))

delete
: delete an observation and its search entry. One with concept links, source
  links, or supersession relations (in either direction) is refused with the
  counts (exit 1); --force removes those links and relations and the
  observation, and the concepts, sources and other observations stay. To remove
  a single link instead, see kb-unlink(1). --dry-run reports and changes
  nothing.

A draft travels between databases as its own record type (observation_draft)
in kb-export(1), kb-import(1) and kb-merge(1), so an older kb skips it
rather than reading it as a trusted observation. Importing an observation that
this database holds as a draft promotes it here too; a draft never downgrades
a reviewed row, and when a merge meets both, reviewed wins. A concept or
source that only a draft links still travels as an ordinary row.

Deletion is local to one database, with no tombstone: kb-merge(1) and
kb-import(1) from a database that still has the row bring it back, so
delete it there as well. `agents/knowledge.jsonl` is re-exported from the database,
so a delete reaches a database rebuilt from it. See DR-0050 (knowledge/decisions/).

# EXIT STATUS

The workspace convention, as described in kb(1).

0
: done. Listing the sources of an observation that has none, or of an id that
  matches nothing, is still 0

1
: no observation has that id (show, update, delete, review promote); the
  project named to add, list or review list does not exist; review promote of
  an observation that is not a draft; or delete of an observation that still
  has concept links, source links or supersession relations, which --force
  removes

2
: a missing or surplus argument, a malformed id, a kind outside the vocabulary
  (note, finding, decision, question, hypothesis), a --status other than
  reviewed, drafted or all, --draft without --by, --by or --confidence without
  --draft, a --confidence outside 0 to 1, review promote without a terminal, or
  an unknown subcommand

# SEE ALSO

kb-project(1), kb-link(1), kb-source(1)

