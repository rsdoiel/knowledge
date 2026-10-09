%kb(1) user manual | version 0.0.19 5483417
% R. S. Doiel
% 2026-10-09

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

Run with no verb at all, on a terminal, to open the interactive interface
instead; see INTERACTIVE INTERFACE below.

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
: path to knowledge.db. Without it the workspace is found by walking up from
  the current directory — see WORKSPACE AND ENVIRONMENT

-json
: machine-readable JSON output instead of human-readable text. Applies to
  every verb. Errors always go to stderr, never stdout, in both modes —
  scripts consuming JSON output can rely on stdout staying valid JSON
  even when a call fails.

-i
: open the interface at this command instead of running it. kb -i
  record pending opens the Pending screen, kb -i record show
  harvey/DR-0004 opens the Records screen with the cursor on that record, and
  kb -i alone opens the top menu. It needs a terminal on standard
  input and output and cannot be combined with -json. A command with no
  screen yet is a usage error that says so; it is not run instead. See
  INTERACTIVE INTERFACE

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

# INTERACTIVE INTERFACE

Bare kb, on a terminal, opens a menu with one entry per verb group
(Projects, Records, Observations, Concepts, Sources, Documents, Search, Ingest,
Index, Check), so choosing from it teaches the command line. A header names the
workspace directory, its database and what it holds, because a machine can have
more than one workspace. A group opens its own menu of the things that need no
selection; rows that are not built yet are dimmed with the release that brings
them, and choosing one prints the command that does the same. Projects: Browse
shows the projects, and opening a project shows its observations, concepts and
records as tabs with their counts. Records: Browse (newest first) and Pending
(oldest first) show one line a record, in the scope kb record list would
use, and a widens it to every scope. Search asks for a term.

Keys: the arrow keys or j and k move; Enter opens; / searches; o, c and r switch
a project's tabs. q goes back to the screen above (the project list to the
Projects menu, a group menu to the top menu) and quits at the top menu. Esc
cancels something in progress, such as a search being typed, and does nothing
otherwise, so it never closes a screen. Ctrl-C quits from anywhere. A line at
the bottom of every screen lists the keys that apply. While you type in the
search prompt every key is text, q included, so a term that starts with q works;
Esc leaves the prompt, and q is a command again afterwards.

On a record, on the Records screens and on a project's Records tab, Enter reads
it and s sets its status. Both show the record file in the pager ($KB_PAGER, else
bat, else less -R), with the interface suspended while it runs and restored when
it ends; with no pager the record is shown in a built-in viewer instead (the
arrows or j and k scroll, space pages, q finishes). s then offers the moves the
record's status allows, each on one key, and asks for y before it writes, exactly
as kb record set-status REF does, and by the same function. Esc, q and Ctrl-C
back out at either step with nothing written. When it writes, the screen says what
was done and shows the command that does the same, so the interface teaches the
command line; a move the record's file now refuses is reported and writes nothing.
A record that is superseded has no move to make and says so before anything is
shown. Accepting is allowed here, since the interface is a terminal.

d deletes the selected thing, from the project list (a project), a project's
observations and concepts tabs, and the Records screens and tab (a record). A
delete is not confirmed with y: a screen shows what will be removed and what points
at it (for a record, the records that supersede or relate to it), and asks you to
type the thing's name, or a record's reference, or an observation's number, exactly.
Only an exact match and Enter delete; Esc cancels. While you type, every key is text,
q included, so a project called quokka can be confirmed. A delete that the command
line would refuse is refused here without a gate and says why: a project that owns
observations, records or documents, and a record whose file still exists (delete the
file first, or retire the record with kb record set-status REF cancelled).
When it deletes, the screen shows the command that does the same. Deleting is local:
kb merge or kb import from a database that still has the thing brings it back.

Changes are made the same way: the item is shown with its current value, the new
one is picked or typed, and old to new is confirmed with y before anything is
written. On the project list s sets a project's status (each status is one key),
e edits its description and r renames it; on an observation e writes a correction
(as kb observation update does, the new text is a new observation that
supersedes the old and the original wording is kept); on a concept e renames it. On a record u makes the selected record
supersede another: you type which one (its scope is filled in, so a number is
enough), the field refuses what kb record supersede refuses (a record that
is not there, itself, one in another tier), and the confirmation shows both records
and that the replaced one becomes superseded; both files and the database are
written. A text field starts from the current text; Enter accepts it, Ctrl-J starts a new
line, Ctrl-U clears the line, and Esc cancels. While you type every key is text,
q included. Each change is made by the command's own function, so a refusal (a
rename onto a name that is taken) is the command's refusal, shown on the screen
with nothing changed, and a success shows the command that does the same.

New things are made with a form: the New project…, New observation…, New concept…,
New source… and New record… rows of the menus, and n on the project list (a project),
a project's observations tab (an observation in that project), its concepts tab (a
concept) and the Records screens (a decision record). The fields are asked one at a
time: a line of text (Enter accepts, Ctrl-J starts a new line, an optional field may
be left empty) or a choice made with one key (an observation's kind: note, finding,
decision, q[u]estion, hypothesis). What has been entered stays on the screen, and the
last screen shows the command that does the same and asks for y. Esc cancels the whole
form at any step and nothing is written; while you type every key is text, q
included. The write is the command's own function, so its refusal (a project that
exists, a source's bad date) is shown as it would be on the command line. A new
decision record is written proposed, as the command does, and reaches the database
at the next kb ingest.

Index (on the top menu) and Records → Format files… run kb index and kb
record fmt straight away, with no confirmation, since both only write generated
files or canonical form and are safe to repeat. They run on the Records scope's
decisions directory (agents/projects/NAME/decisions for a project, the whole agents
tree with every scope, which a widens), the command's own output is shown on a
screen you leave with q, and so is the command that does the same. A scope with no
decisions directory says so and nothing is created.

A complete command always runs as a command and prints, on a terminal or not, so
scripts can rely on it. Only an incomplete one opens the interface, and only on
a terminal: bare kb, a bare group (kb record opens the Records menu), and
kb record show or kb project show with no argument. Without a terminal
those are usage errors, exit 2. -i opens the interface at a command that has a
screen; see GLOBAL OPTIONS.

# WORKSPACE AND ENVIRONMENT

A workspace is a directory with an agents/ directory holding knowledge.db, the
working copy, or knowledge.jsonl, the export that is tracked in git. Without
-db, kb walks up from the current directory to the nearest ancestor that
has either file, as git finds .git, so every verb works from any subdirectory
of a workspace. Where one directory has both, the database is used. A directory
elsewhere on the machine is a different workspace and is never reached: each
knowledge base is independent of the others.

A fresh clone has knowledge.jsonl but no knowledge.db, because databases are not
tracked. kb then says so and names the remedy,
"kb import -in agents/knowledge.jsonl", which builds the database in the
workspace rather than in the current directory. It does not suggest init, which
would start an empty history beside the real one.

When the workspace found is not the current directory, kb says which on
standard error, once per command: "kb: using the workspace at DIR (found above
the current directory)". Standard output is untouched, so -json stays parseable.

These environment variables are read, and an option on the command line wins
over each:

KB_DB
: the path to the database, as -db. Not read by init, index, merge or
  completion, which refuse -db

KB_PROJECT
: the project to act on when none is given and the current directory belongs to
  no project (record verbs; see kb-record(1)). The directory is closer
  evidence, so it wins over this

KB_CEILING_DIRECTORIES
: directories, separated as PATH is, that the walk up never enters or passes, as
  git's GIT_CEILING_DIRECTORIES. It keeps a scratch directory inside a real
  workspace from reaching that workspace, and is set to the temporary directory
  by the tests

KB_QUIET
: any value but empty, 0 or false silences advisory notes on standard error (the
  workspace note, and a deprecated flag's). It never silences an error

KB_PAGER
: the program that shows a record for review in record set-status with no
  status, with its arguments (KB_PAGER="bat -l markdown"). Unset, kb uses bat
  with the Markdown language if bat is installed, then less -R, and with
  neither prints the record straight through

# VERBS

project
: manage projects — see kb-project(1)

observation
: manage observations — see kb-observation(1)

concept
: manage concepts, show one with what links to it, recall by text — see kb-concept(1)

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

check-db
: compare the database with the JSONL dump beside it by full content, and say
  whether to import, export or both — see kb-check-db(1)

export, import
: write/read a portable JSON-L snapshot of the database — the no-file-access
  alternative to merge, for syncing over a channel that can only move plain
  text (paste, email, git) — see kb-export(1) and kb-import(1)

ingest
: index a tree of decision records into the knowledge base — see
  kb-ingest(1)

record
: read and maintain decision records — list, pending, show, new, set-status,
  supersede, fmt, fuzzy-tag (near-miss concept mentions) — see
  kb-record(1)

document
: ingest, draft, review and tag narrative documents (Markdown, Fountain,
  text) at graduated abstraction levels — see kb-document(1)

index
: generate a decisions/index.md from a directory of records — see
  kb-index(1)

init
: create a new, empty workspace — see kb-init(1)

completion
: write a shell completion script for bash or PowerShell, or install it — see
  kb-completion(1)

verbs
: print the verb table: every verb, subverb, flag and write class, as text or
  with -json — see kb-verbs(1)

# ARGUMENTS

The global options -json, -db and -debug go before the verb: kb
-json project list. After the verb they are refused, not ignored. So is any
other flag a verb does not have, and any surplus argument.
init, index, merge and completion never open the ambient database, so -db is
refused for them as well; init takes its target as kb init PATH, merge as -a, -b
and -out.

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
  rename or add onto a name that already exists, a project that still owns
  records, a document section not yet drafted); a record scope that is neither
  a project nor the workspace

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
  directory where a file is needed, or no workspace above the current directory
  (or one with only knowledge.jsonl, which is to be imported)

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
kb-index(1), kb-init(1), kb-completion(1),
kb-topics(1)

