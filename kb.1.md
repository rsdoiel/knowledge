%kb(1) user manual | version 0.0.14 e470e93
% R. S. Doiel
% 2026-09-25

# NAME

kb — command-line and interactive interface for a knowledge base

# SYNOPSIS

kb [-help|-license|-version]

kb [-db PATH] [-json] [-debug] VERB [PARAMETERS...]

kb help [TOPIC]

kb

# DESCRIPTION

kb reads and writes a github.com/rsdoiel/knowledge knowledge base:
projects, observations, concepts, sources, decision records and narrative
documents, with full-text search and a cross-machine merge tool. Every verb
follows the "TOOL VERB PARAMETERS"
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

link, unlink
: link projects/observations to concepts, and remove those links — see
  kb-link(1) and kb-unlink(1)

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

document
: ingest, draft, review and tag narrative documents (Markdown, Fountain,
  text) at graduated abstraction levels — see kb-document(1)

index
: generate a decisions/index.md from a directory of records — see
  kb-index(1)

init
: create a new, empty workspace — see kb-init(1)

# ARGUMENTS

The global options -json, -db and -debug go before the verb: kb
-json project list. After the verb they are refused, not ignored. So is any
other flag a verb does not have, and any surplus argument.
init, index and merge never open the ambient database, so -db is refused for
them as well; init takes its target as kb init PATH, merge as -a, -b and -out.

A name that begins with a dash is given after --, as in
kb project show -- -name, so that it is not read as a flag. Every verb
that takes a name, title or path accepts --, record, document, ingest and
source add included. Words that follow a verb's fixed arguments and are free
text (an observation body, a project description, a retraction note) are taken
as they are, dashes included.

A project or concept name is one line. Surrounding whitespace is trimmed and
any interior run of whitespace, a newline or a tab included, becomes a single
space, so a [[wikilink]] wrapped across two lines names the same concept as
the one-line spelling. A name with any other control character is refused.

# EXIT STATUS

kb follows the workspace exit-code convention (workspace DR-0003, applied
here by DR-0047): 0 and 1 answer the question that was asked, 2 says the command
was wrong, and the sysexits(3) numbers above that say what went wrong, so a
script or a calling tool can tell them apart from the number alone. A command that
works through many items (ingest, index --all, source check-retractions) does all it
can, prints the counts, and then exits with the class of the first failure: it does
not exit 0 with failures, and one bad item does not stop the good ones. With -json
the error on stderr carries the class name beside the number, for example
{"error": "...", "class": "no_input", "code": 66}.

0
: success. A listing that matches nothing is still success

1
: the command ran correctly and the answer is no: search found nothing; there
  is no such project, concept, observation, source, record or document; a
  record list filter value that nothing carries; an index that is stale; or the
  current state forbids the operation (a concept or source still linked, a
  rename onto a name that already exists, a project that still owns records, a
  document section not yet drafted)

2
: usage error: the command line itself is wrong and nothing was attempted. An
  unknown verb, subverb or flag; a missing or surplus argument; a missing
  required flag; any bad value passed on the command line, whether it does not
  parse (a malformed id or date), is outside a closed vocabulary (an observation
  kind, a project status, a record status, kind or trigger given to record new
  or set-status), or is blank. A script can tell "fix the command" (2) from
  "handle the result" (1)

65
: content the command read is wrong: a malformed record file, JSONL or
  document; a file that is not a knowledge base, including a zero-byte one; a
  merge identity collision without -force; a record whose identity or uuid
  conflicts with what is already stored (ingest under the wrong workspace root
  shows as constraint failures). Distinct from 2: the command was right and the
  data was not

66
: a named input or the workspace is missing: no such file or directory, a
  directory where a file is needed, or no agents/knowledge.db here

69
: a network service could not be reached: source check-retractions, after it has
  tried every source. The sources it could not look up are not known to be clear

70
: an internal error, or an error nothing classified. It is never the answer for
  a known condition: report it

73
: an output could not be created: the file exists, or its directory cannot be
  made (merge -out, export -out, init, index, record new)

74
: a read or write failed part way: a disk or database I/O error

75
: the database is locked. Retrying later may succeed

77
: the operating system refused access

# SEE ALSO

kb-project(1), kb-observation(1), kb-concept(1),
kb-link(1), kb-source(1), kb-search(1),
kb-merge(1), kb-export(1), kb-import(1),
kb-ingest(1), kb-record(1), kb-document(1),
kb-index(1), kb-init(1), kb-topics(1)

