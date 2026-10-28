%kb-merge(1) user manual | version 0.0.28 c100b07
% R. S. Doiel
% 2026-10-28

# NAME

kb-merge — reconcile two knowledge.db files that drifted independently

# SYNOPSIS

kb merge -a PATH -b PATH -out PATH [-force]

# DESCRIPTION

merge reads two knowledge.db files (e.g. from two machines that have
drifted independently) read-only and writes their deduped union to a
fresh -out file, which must not already exist. It never modifies -a or
-b; placing the merged file into position is left to you. Every table
travels — projects, concepts, sources, observations, decision records and
record relations, plus the join tables, among them the citation tables
document_sources and record_sources (DR-0081) — so a table that would lose
rows says so in the per-table summary rather than merging quietly short.
A concept's kind (DR-0078) travels with it and follows the same
last-writer-wins rule as its description. Story events and the story knowledge
(learns and knows) kept with them do not travel: they are
derived from manuscript files, so the merged database has none until
kb document ingest is run on the same paths.
Each side is copied to a scratch file and brought up to the current schema
before ATTACHing, so a database predating decision records (or any other
table) still merges instead of failing outright.

Unlike every other verb, merge operates entirely on the explicit -a/-b/-out
paths — a --db is refused, and it never opens (or creates) the ambient
./agents/knowledge.db.

Both inputs must exist, be non-empty database files, and be two different
files; anything else is refused before any file is touched. A mistyped -a
therefore fails, instead of merging an empty database in its place and
leaving a zero-byte file at the typo. A zero-byte file is refused even when a
-wal file sits beside it: SQLite discards that -wal on opening an empty main
file, so merge refuses first and leaves the -wal untouched. Each refusal has
its own exit status, listed under EXIT STATUS.

If a project or concept with the same name exists in both files under
different internal identities (a collision — typically from before a
database's identifiers were established), merge aborts and lists them
unless -force is given, in which case b's identity is reconciled to a's so
both sides' observations and links survive under one merged entity. A
decision record collides the same way, keyed by its identity — workspace,
project, scope and record id — rather than by project_id or the project's
own uuid.

A record held by both files under the same identity but different text is
reported as a content divergence (same record, different prose) even when
it is not also a collision: the merge keeps a's copy and never blocks on
this, but prints the diverging record ids and checksums so the operator
knows which Markdown files to reconcile by hand. With --json, divergences
appear as content_divergences alongside collisions_reconciled and the
per-table tables summary, instead of the plain-text report.

To find out whether a database and the JSONL dump beside it have drifted, before
deciding whether to merge, import or export, use kb-check-db(1). It is
read-only and shares merge's rules for what counts as the same row.

# EXIT STATUS

The workspace convention, as described in kb(1).

0
: the merged file was written. Content divergences are reported and are still 0;
  collisions reconciled by -force are too

2
: -a, -b or -out missing, a surplus argument, a bad flag, a --db (merge never
  opens the ambient database), or -a and -b naming the same file

65
: an input that is zero bytes or is not a knowledge base; or an identity
  collision, which is reported and aborts the merge unless -force is given

66
: an input that does not exist, is a directory, or is not a regular file

73
: -out already exists, or its directory cannot be made

# SEE ALSO

kb(1), kb-check-db(1), kb-export(1), kb-import(1)

