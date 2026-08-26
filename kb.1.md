%kb(1) user manual | version 0.0.3 4aff73c
% R. S. Doiel
% 2026-08-08

# NAME

kb — command-line and interactive interface for a knowledge base

# SYNOPSIS

kb [--db PATH] [--json] VERB [args...]

kb

kb help | -h | --help [VERB]

# DESCRIPTION

kb reads and writes a github.com/rsdoiel/knowledge knowledge base:
projects, observations, concepts, and sources, with full-text search and a
cross-machine merge tool. Every verb follows the "TOOL VERB PARAMETERS"
model (the same shape as git and go), so scripts and other language-model
harnesses can drive it directly, not just people at a terminal.

Run with no verb at all to launch the interactive browser (TUI) instead —
a read-only view over the same data, for exploring projects, observations,
and concepts, and running searches, without leaving the terminal.

# GLOBAL FLAGS

--db PATH
: path to knowledge.db (default: ./agents/knowledge.db, relative to the
  current directory)

--json
: machine-readable JSON output instead of human-readable text. Applies to
  every verb. Errors always go to stderr, never stdout, in both modes —
  scripts consuming --json output can rely on stdout staying valid JSON
  even when a call fails.

--debug
: write a JSONL trace of every knowledge-base call (and, in the TUI,
  every input event and view change) to ./kb-debug-<timestamp>.jsonl in
  the current directory. The path is printed to stderr once at startup.
  Applies to every verb and the TUI. Omitting --debug costs nothing —
  no file is written and behavior is unchanged.

# VERBS

project
: manage projects — see kb-project(1)

observation
: manage observations — see kb-observation(1)

concept
: manage concepts — see kb-concept(1)

link
: link projects/observations to concepts — see kb-link(1)

source
: manage cited sources and retraction checking — see kb-source(1)

search, summary, format
: full-text search and formatted views — see kb-search(1)

merge
: reconcile two knowledge.db files that drifted independently (e.g. across
  machines) into a fresh, deduped output — see kb-merge(1)

export, import
: write/read a portable JSON-L snapshot of the database — the no-file-access
  alternative to merge, for syncing over a channel that can only move plain
  text (paste, email, git) — see kb-export(1) and kb-import(1)

# EXIT STATUS

0
: success

1
: the verb ran but failed (database error, not found, etc.)

2
: usage error (bad flags, unknown verb)

# SEE ALSO

kb-project(1), kb-observation(1), kb-concept(1),
kb-link(1), kb-source(1), kb-search(1),
kb-merge(1), kb-export(1), kb-import(1)

