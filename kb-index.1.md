%kb-index(1) user manual | version 0.0.11 dcee1cc
% R. S. Doiel
% 2026-09-19

# NAME

kb-index — generate a decisions/index.md from a directory of records

# SYNOPSIS

kb index PATH [--stdout|--check]

kb index ROOT --all [--check]

# DESCRIPTION

Regenerates PATH/index.md: one greppable line per record, newest first. The
file is generated and never hand-edited.

Newest-first and one-line-per-record preserve the affordance a single
top-inserted DECISIONS.md had — head, grep and awk reach the recent and the
relevant without reading the whole corpus. The index is what stays loadable as
the corpus grows; records are then read selectively.

Columns, in order: DR-<id>, date, status, kind, trigger, the supersession
flag, and the title. The title comes last because it is the only field that
may contain arbitrary text, including runs of spaces.

Every column always holds a value. An empty one renders as -, never as spaces:
awk's default separator is a run of whitespace, so a space-padded column is not
a field at all and the next column silently takes its position. With the
placeholder the title always starts at $7.

The supersession flag reads sup when superseded_by is non-empty, and - when it
is not. It fires regardless of status, so it is redundant for a wholly
superseded record whose status column already says so. Its real work is the
partial case, where a record stays accepted because most of its episode still
stands — without the flag such a record looks, in the index, exactly like one
nothing has touched.

The index is built from the record files, so it works before any ingest. The
attribution line names no tool: more than one generator has existed for this
format, and a file naming one of them cannot be reproduced byte-for-byte by
another without asserting something false about itself.

A record that cannot be parsed is an error, unlike in ingest. Dropping one
silently would make the index lie about what the corpus contains.

This command writes index.md and nothing else. The format has no
decisions/README.md, so one is never created.

--check compares PATH/index.md against a fresh render and reports drift as an
error instead of writing: missing, or different from what the current records
would produce. It exits non-zero either way, so it fits a pre-commit hook or
CI step; the remedy either way is running index without --check. --check and
--stdout cannot be combined.

--all walks ROOT and processes every directory that already has an index.md
-- the corpora that have opted into this convention -- refreshing or, with
--check, checking each one. A directory with record files but no index.md
yet is silently left alone: --all never creates one, the same rule
kb record set-status/supersede's own automatic refresh follows. Nested
corpora (each with their own index.md) are handled independently, never
folded together. One bad corpus does not stop the rest: --all keeps going
and reports every corpus, then exits non-zero if any needed attention, so a
pre-commit hook can gate on the whole workspace in one call rather than
naming each corpus by hand. --all and --stdout cannot be combined. See
TODO.md's index-regeneration item (index --check and the auto-refresh on
set-status/supersede) for the single-corpus half this completes.

Hidden directories are not descended into. A git worktree keeps a whole
second copy of the tree under .claude/worktrees/<name>/, corpus and
generated index.md included, so without this it is reported as a corpus in
its own right -- a duplicate under --check, and in write mode an edit to a
throwaway worktree rather than the real tree. The prune applies only to
directories descended into, never to ROOT itself, so naming a hidden
directory as ROOT still finds the corpora inside it.

# OPTIONS

--stdout
: write the index to standard output instead of index.md (PATH form only)

--check
: verify index.md is current without writing it; non-zero exit on drift

--all
: process every already-indexed corpus under ROOT instead of one PATH

# EXAMPLES

~~~shell
kb index clasm/decisions
kb index agents/decisions --stdout | head
kb index agents/decisions --check
kb index . --all
kb index . --all --check
~~~

