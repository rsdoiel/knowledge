%kb-init(1) user manual | version 0.0.5 e775390
% R. S. Doiel
% 2026-08-28

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

None beyond the standard options.

# EXAMPLES

~~~shell
kb init
kb init ~/NewWorkspace
~~~

