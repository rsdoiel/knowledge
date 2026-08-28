%kb-topics(1) user manual | version 0.0.5 98e74cd
% R. S. Doiel
% 2026-08-28

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

source
: manage cited sources and retraction checking

search
: full-text search across projects, observations, concepts and records.
  summary and format share this page

merge
: reconcile two knowledge.db files that drifted independently

export
: write a portable JSON-L snapshot. import shares this page

import
: read a JSON-L snapshot back

ingest
: index a tree of decision records into the knowledge base

record
: read and maintain decision records

index
: generate a decisions/index.md from a directory of records

init
: create a new, empty workspace

# NOTES

The conventional spelling for this page is "help index". It is "help topics"
here because index is itself a verb, so "kb help index" is that verb's
manual page and cannot also be the topic list.
