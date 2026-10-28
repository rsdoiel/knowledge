%kb-topics(1) user manual | version 0.0.28 c100b07
% R. S. Doiel
% 2026-10-28

# NAME

kb-topics — the index of help topics

# SYNOPSIS

kb help topics

# DESCRIPTION

Every topic below has its own manual page, reachable as "kb help TOPIC"
or "kb TOPIC -help", and installed as a man page by make install.

project
: manage projects

observation
: manage observations

concept
: manage concepts

link
: link projects and observations to concepts

unlink
: remove a link between a project or observation and a concept, or an observation and a source

source
: manage cited sources and retraction checking

search
: full-text search across projects, observations, concepts and records.
  summary and format share this page

merge
: reconcile two knowledge.db files that drifted independently

check-db
: compare the database with its JSONL dump by content and recommend import,
  export or both

stats
: row, link and check counts, and a line per project

story
: a writing project's story timeline, continuity report and who knows what

export
: write a portable JSON-L snapshot. import shares this page

import
: read a JSON-L snapshot back

ingest
: index a tree of decision records into the knowledge base

record
: read and maintain decision records

document
: ingest, draft, review and tag narrative documents at graduated
  abstraction levels

index
: generate a decisions/index.md from a directory of records

init
: create a new, empty workspace

model
: install, fetch and list the embedding models semantic search uses

embed
: build and update the semantic-search index

completion
: write or install a shell completion script for bash or PowerShell

verbs
: print the verb table, as text or JSON

# NOTES

The conventional spelling for this page is "help index". It is "help topics"
here because index is itself a verb, so "kb help index" is that verb's
manual page and cannot also be the topic list.
# EXIT STATUS

The workspace convention, as described in kb(1).

0
: the topic list was printed

2
: kb help was given a name that is not a topic

# SEE ALSO

kb(1), kb-verbs(1)
