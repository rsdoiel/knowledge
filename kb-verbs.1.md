%kb-verbs(1) user manual | version 0.0.20 548bdc2
% R. S. Doiel
% 2026-10-09

# NAME

kb-verbs — print the verb table: every verb, subverb, flag and what it does to the database

# SYNOPSIS

kb verbs

kb -json verbs

# DESCRIPTION

Prints the one table that describes kb's command language, so a person,
a script or a model has a single place to ask what exists. Completion, the
subverb help flags and the interactive interface read the same table, so they
cannot disagree with it.

Plain text lists a row per verb and per subverb with its write class. With the
global -json option it prints a document with a "verbs" array. Each verb has
"name", "summary" and "tui" (whether the interactive interface offers it); a verb
with no subverbs has a "class", and one with subverbs lists them, each with its
own "class" and "tui", and a subverb's own subverbs (document review has list
and promote). "flags" lists the flags the verb accepts, subverbs included.

The classes say what a verb does to the knowledge base:

read
: reads only

additive
: adds knowledge

changing
: changes something that exists

removing
: removes knowledge, or links other records rely on

direct
: writes files or an index and is run without asking, such as index and
  record fmt

plan_apply
: has a --dry-run plan, then applies it

guided
: a multi-step workflow with a person in it, such as document ingest and
  review promote

cli_only
: not offered in the interactive interface (merge, import, init, export,
  completion, verbs)

The verb opens no database, so it works anywhere and --db does not apply to it.
kb -json verbs, not kb verbs -json: the global options go before the
verb.

# EXIT STATUS

The workspace convention, as described in kb(1): 0 success; 2 an unknown
flag or a surplus argument, or -db, which does not apply to this verb.

# SEE ALSO

kb(1), kb-completion(1)

