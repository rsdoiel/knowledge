%kb-embed(1) user manual | version 0.0.29 7d292d7
% R. S. Doiel
% 2026-10-29

# NAME

kb-embed — bring the semantic-search index in step with the knowledge base

# SYNOPSIS

kb embed

kb embed -status

kb embed -rebuild

# DESCRIPTION

Semantic search finds rows by meaning, not by their words. It needs an index: a
vector for each passage of each observation, decision record, concept and
reviewed document summary, kept in agents/knowledge.vectors.db beside the
database. embed builds that index and keeps it current. It needs the semantic
build of kb and an installed model (kb-model(1)); the plain build
has no embedder and exits 2.

Semantic search is a preview in this release. Measured on a Raspberry Pi 5 (4
cores, 16 GB) over a copy of this workspace's knowledge base, 893 rows in about
4,100 passages: the first index took 14 to 17 minutes (about 5 passages a
second) and used up to 875 MB of memory; running embed again with nothing
changed took a fraction of a second. A search takes about half a second,
mostly loading the model. The semantic kb is about 33 MB against 21 MB for
the plain one, and the model is about 87 MB. Another machine will differ.
Run the first embed in a terminal you can leave, or detach it; it can be
interrupted and resumed.

Run with no flag, embed works out which rows are new or changed since they were
embedded, embeds only those, and drops the entries of rows that are gone. A
passage whose text did not change keeps its vector, so editing one paragraph of a
long record embeds one passage. The first run over a large knowledge base takes
minutes on a small machine; later runs take seconds.

The work is committed a batch at a time. Interrupt it (Ctrl-C) and at most one
batch is lost; run embed again to carry on where it stopped. Progress goes to
standard error.

-status
: report, and change nothing: the rows by kind, the model the index was built
  with, how many rows and passages are not indexed yet, and how many index
  entries belong to rows that are gone. It loads no model and works in the plain
  build too.

-rebuild
: delete the index and embed everything again. This is the remedy for an index
  built by a different model (exit 65), and the way to start over.

A command that writes rows (kb observation add, ingest, record and
concept changes, document review promote) also updates the index for the rows it
changed, when an index exists and the model is installed; this never changes the
command's result. When it cannot (no model, another embed running, a different
model, or more than a few dozen rows after a merge or an import) the command
says in one line that rows are not yet indexed. kb merge, import and
check-db leave the index alone, so rows they bring in are simply not indexed
until the next embed. The interactive interface's own edits do not update the
index at all; run embed to bring it up to date.

Only one embed writes the index at a time. A second gets exit 75 and may be run
again later; a run that was killed does not block the index for more than five
minutes.

# OPTIONS

The global -json option applies. The global -db option selects the database
whose index is built.

# FILES

agents/knowledge.vectors.db
: the index, beside the database; never part of the database or its JSON-L
  export. kb init adds it to .gitignore in a git workspace.

# EXAMPLES

~~~shell
kb model fetch
kb embed
kb embed -status
kb embed -rebuild
~~~

# EXIT STATUS

The workspace convention, as described in kb(1).

0
: the index is current (nothing may have needed doing), or -status reported

2
: a bad flag or argument, -status with -rebuild, or a build without semantic search

65
: the index was built by a different model than the one installed (run
  kb embed -rebuild), or the index file is not a vector index

66
: the model is not installed (run kb model fetch)

75
: another embed holds the index, or this one was interrupted; run it again

# SEE ALSO

kb(1), kb-model(1), kb-search(1)
