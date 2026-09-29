%kb-check-db(1) user manual | version 0.0.14 229bafd
% R. S. Doiel
% 2026-09-25

# NAME

kb-check-db — compare the database with its JSONL dump

# SYNOPSIS

kb check-db [--jsonl FILE]

# DESCRIPTION

agents/knowledge.jsonl is the authoritative copy and the agents tree is kept in
git across machines, so the database and the dump drift. check-db says which
way, without changing either. It is read-only.

The dump defaults to the database's path with .jsonl in place of .db;
--jsonl FILE names another. The resolved paths are printed at the top of the
report. Nothing else is searched for, because agents/ can hold several dumps
and a wrong guess gives a confident wrong verdict.

It decides by a full content diff, never by file time. The dump is read into a
scratch database and compared with a copy of this one, table by table, using
the same idea of "the same row" as kb-merge(1): name for projects and
concepts, uuid for observations, sources, documents and sections, workspace,
project, scope and record id for decision records, and the pair of parents for
links. The report lists rows only in the JSONL, rows only in this database, and
rows on both sides that differ (a project's description or status, a concept's
description or identifier, an observation's kind or body, a record's checksum,
and so on), with per-table counts.

The recommendation is one of: in sync; kb import (the JSONL is ahead);
kb export (the database is ahead); or diverged (import, then export).
File modification times are printed as a hint and never change the verdict: a
git pull refreshes a file's time without changing its content.

Rows only in the JSONL are labelled "never received, or deleted here". There
are no tombstones, so a deletion made here looks exactly like a row never
received, and the import recommendation warns that it brings back anything
deleted on purpose. See DR-0053 (knowledge/decisions/).

With --json the report is one object: in_sync, recommendation, database, jsonl,
tables (each with database and jsonl counts, only_in_database, only_in_jsonl and
different), collisions and file_times.

# EXIT STATUS

0
: in sync

1
: not in sync (a check that reports drift); the report is still printed on
  stdout

2
: a bad flag, a missing value for --jsonl, or a surplus argument

65
: the dump is malformed

66
: there is no dump at the path

# SEE ALSO

kb(1), kb-merge(1), kb-export(1), kb-import(1)

