%kb(1) user manual | version 0.0.5 a947121
% R. S. Doiel
% 2026-08-28

# NAME

kb — command-line and interactive interface for a knowledge base

# SYNOPSIS

kb [-help|-license|-version]

kb [-db PATH] [-json] [-debug] VERB [PARAMETERS...]

kb help [TOPIC]

kb

# DESCRIPTION

kb reads and writes a github.com/rsdoiel/knowledge knowledge base:
projects, observations, concepts, and sources, with full-text search and a
cross-machine merge tool. Every verb follows the "TOOL VERB PARAMETERS"
model (the same shape as git and go), so scripts and other language-model
harnesses can drive it directly, not just people at a terminal.

Run with no verb at all to launch the interactive browser (TUI) instead —
a read-only view over the same data, for exploring projects, observations,
and concepts, and running searches, without leaving the terminal.

# STANDARD OPTIONS

Each is accepted in either dash form: -help and --help are the same option.
All three are answered and exited immediately, without opening or creating a
knowledge base.

-help
: display this help page. "kb help TOPIC" reaches the same text by
  verb, and "kb VERB -help" prints that verb's page

-license
: display the license

-version
: display the program name, version and release hash

The version and license come from version.go, which is regenerated from
codemeta.json, and every help page carries the version it was generated
from — so a page naming a different release is a stale artifact rather than
a difference of opinion.

# GLOBAL OPTIONS

-db PATH
: path to knowledge.db (default: ./agents/knowledge.db, relative to the
  current directory)

-json
: machine-readable JSON output instead of human-readable text. Applies to
  every verb. Errors always go to stderr, never stdout, in both modes —
  scripts consuming JSON output can rely on stdout staying valid JSON
  even when a call fails.

A global option must precede the verb. Parsing stops at the first
non-option argument, which is what lets a verb's own flags through
untouched: in "kb -json ingest DIR --dry-run", -json is global and
--dry-run belongs to ingest.

-debug
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

ingest
: index a tree of decision records into the knowledge base — see
  kb-ingest(1)

record
: read and maintain decision records — list, show, new, set-status,
  supersede, fmt — see kb-record(1)

index
: generate a decisions/index.md from a directory of records — see
  kb-index(1)

init
: create a new, empty workspace — see kb-init(1)

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
kb-merge(1), kb-export(1), kb-import(1),
kb-init(1)

