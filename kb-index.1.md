%kb-index(1) user manual | version 0.0.4 922ae87
% R. S. Doiel
% 2026-08-26

# NAME

kb-index — generate a decisions/index.md from a directory of records

# SYNOPSIS

kb index PATH [--stdout]

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

# OPTIONS

--stdout
: write the index to standard output instead of index.md

# EXAMPLES

~~~shell
kb index clasm/decisions
kb index agents/decisions --stdout | head
~~~

