%kb-init(1) user manual | version 0.0.28 c100b07
% R. S. Doiel
% 2026-10-28

# NAME

kb-init — create a new, empty workspace

# SYNOPSIS

kb init [PATH]

# DESCRIPTION

Creates a schema-only PATH/agents/knowledge.db — no data, the same shape as
git init. PATH defaults to the current directory.

Every other kb verb that opens the ambient database (no --db given)
requires one to already exist, and fails toward "kb init" or
"kb import -in FILE" rather than silently creating one in whatever
directory it happens to be run from. init is how a genuinely new workspace,
with no prior agents/knowledge.jsonl to seed from, gets that first database.

A workspace being rebuilt or bootstrapped from an existing export is a
different case: "kb import -in agents/knowledge.jsonl" already
creates a missing target database on its own, so init has nothing to add
there — see kb-import(1).

init is idempotent. Run again against an already-initialized workspace, it
reports that and leaves the existing database untouched; it never truncates
or overwrites data.

# OPTIONS

None beyond the standard options. The global -db option does not apply: init
creates PATH/agents/knowledge.db, so the target is named as the PATH argument,
and an explicit -db is refused (exit 2) rather than ignored.

# EXAMPLES

~~~shell
kb init
kb init ~/NewWorkspace
~~~


# EXIT STATUS

The workspace convention, as described in kb(1).

0
: PATH/agents/knowledge.db was created, or it already existed and was left
  untouched (init is idempotent)

2
: more than one PATH, a bad flag, or a -db

73
: the workspace cannot be created: a path component is a file, or the directory
  cannot be made

77
: the operating system refused to let init create the directory or the database

# SEE ALSO

kb(1), kb-import(1), kb-ingest(1)
