package main

// HelpText is the primary kb(1) man page. Shown by bare `kb -h`/`kb help`.
// Generates kb.1.md.
const HelpText = `%{app_name}(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name} — command-line and interactive interface for a knowledge base

# SYNOPSIS

{app_name} [-help|-license|-version]

{app_name} [-db PATH] [-json] [-debug] VERB [PARAMETERS...]

{app_name} help [TOPIC]

{app_name}

# DESCRIPTION

{app_name} reads and writes a github.com/rsdoiel/knowledge knowledge base:
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
: display this help page. "{app_name} help TOPIC" reaches the same text by
  verb, and "{app_name} VERB -help" prints that verb's page

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
: open the interface at this command instead of running it. {app_name} -i
  record pending opens the Pending screen, {app_name} -i record show
  harvey/DR-0004 opens the Records screen with the cursor on that record, and
  {app_name} -i alone opens the top menu. It needs a terminal on standard
  input and output and cannot be combined with -json. A command with no
  screen yet is a usage error that says so; it is not run instead. See
  INTERACTIVE INTERFACE

A global option must precede the verb. Parsing stops at the first
non-option argument, which is what lets a verb's own flags through
untouched: in "{app_name} -json ingest DIR --dry-run", -json is global and
--dry-run belongs to ingest.

-debug
: write a JSONL trace of every knowledge-base call (and, in the TUI,
  every input event and view change) to ./kb-debug-<timestamp>.jsonl in
  the current directory. The path is printed to stderr once at startup.
  Applies to every verb and the TUI. Omitting --debug costs nothing —
  no file is written and behavior is unchanged.

# INTERACTIVE INTERFACE

Bare {app_name}, on a terminal, opens a menu with one entry per verb group
(Projects, Records, Observations, Concepts, Sources, Documents, Search, Ingest,
Index, Check), so choosing from it teaches the command line. A header names the
workspace directory, its database and what it holds, because a machine can have
more than one workspace. A group opens its own menu of the things that need no
selection; rows that are not built yet are dimmed with the release that brings
them, and choosing one prints the command that does the same. Projects: Browse
shows the projects, and opening a project shows its observations, concepts and
records as tabs with their counts. Records: Browse (newest first) and Pending
(oldest first) show one line a record, in the scope {app_name} record list would
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
as {app_name} record set-status REF does, and by the same function. Esc, q and Ctrl-C
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
file first, or retire the record with {app_name} record set-status REF cancelled).
When it deletes, the screen shows the command that does the same. Deleting is local:
{app_name} merge or {app_name} import from a database that still has the thing brings it back.

Changes are made the same way: the item is shown with its current value, the new
one is picked or typed, and old to new is confirmed with y before anything is
written. On the project list s sets a project's status (each status is one key),
e edits its description and r renames it; on an observation e writes a correction
(as {app_name} observation update does, the new text is a new observation that
supersedes the old and the original wording is kept); on a concept e renames it. On a record u makes the selected record
supersede another: you type which one (its scope is filled in, so a number is
enough), the field refuses what {app_name} record supersede refuses (a record that
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
at the next {app_name} ingest.

Index (on the top menu) and Records → Format files… run {app_name} index and {app_name}
record fmt straight away, with no confirmation, since both only write generated
files or canonical form and are safe to repeat. They run on the Records scope's
decisions directory (agents/projects/NAME/decisions for a project, the whole agents
tree with every scope, which a widens), the command's own output is shown on a
screen you leave with q, and so is the command that does the same. A scope with no
decisions directory says so and nothing is created.

Ingest (top menu), Records → Fuzzy-tag… and Documents → Ingest a document…, Tag… and
Fuzzy-tag… show a plan first: the command's own --dry-run output, scrollable with
j and k, with nothing written. y applies it by running the same command without
--dry-run, and the result is shown with the command that does the same; n, q and Esc
leave with nothing done, and Enter does nothing, so a habitual key cannot write. With
every scope the project is asked first; Ingest a document asks for the file, the
project and an optional title. Documents → Frontmatter… is still the command line.

Documents → Review queue lists the sections waiting for a person, gist first, with
each one's size, tag density and confidence. Enter on one is a unit of two steps. The
summary is written: in $VISUAL or $EDITOR with the interface suspended (the file starts
with a few # lines, which are removed), or, with neither set, in a text area (Enter
accepts, Ctrl-J starts a new line, Esc cancels). An empty or unchanged text is
refused. Then it is shown above its source and y saves it as written by a person
and promotes it; e writes it again; n, q and Esc leave with nothing done, and what
was approved earlier stays. A section that already has a draft starts at the second
step, so a draft is read beside its source before it is promoted, and promoted as it
is keeps its author. A gist is written from the document's sections in order. The
promote is a person's act at a terminal (DR-0070), which being here meets.

: opens a command line on the menus and the browsing screens: type any command as
you would at the shell (the leading {app_name} is optional) and press Enter. The line is
run by the command line's own code, and what happens next follows the verb's class
(see {app_name} verbs): a read or index runs and its output is shown on a screen you
leave with q; adding and changing writes show the command and ask y, with
record set-status showing old to new; delete asks for the name to be typed, as d
does, for a project, observation, concept or record; ingest and the document and
record fuzzy-tag verbs show their --dry-run first; document review promote ID
opens the review form for a drafted section and document draft asks first; merge,
import, init, export and completion are command line only. While you type, every key is text; Esc cancels.

A complete command always runs as a command and prints, on a terminal or not, so
scripts can rely on it. Only an incomplete one opens the interface, and only on
a terminal: bare {app_name}, a bare group ({app_name} record opens the Records menu), and
{app_name} record show or {app_name} project show with no argument. Without a terminal
those are usage errors, exit 2. -i opens the interface at a command that has a
screen; see GLOBAL OPTIONS.

# WORKSPACE AND ENVIRONMENT

A workspace is a directory with an agents/ directory holding knowledge.db, the
working copy, or knowledge.jsonl, the export that is tracked in git. Without
-db, {app_name} walks up from the current directory to the nearest ancestor that
has either file, as git finds .git, so every verb works from any subdirectory
of a workspace. Where one directory has both, the database is used. A directory
elsewhere on the machine is a different workspace and is never reached: each
knowledge base is independent of the others.

A fresh clone has knowledge.jsonl but no knowledge.db, because databases are not
tracked. {app_name} then says so and names the remedy,
"{app_name} import -in agents/knowledge.jsonl", which builds the database in the
workspace rather than in the current directory. It does not suggest init, which
would start an empty history beside the real one.

When the workspace found is not the current directory, {app_name} says which on
standard error, once per command: "kb: using the workspace at DIR (found above
the current directory)". Standard output is untouched, so -json stays parseable.

These environment variables are read, and an option on the command line wins
over each:

KB_DB
: the path to the database, as -db. Not read by init, index, merge or
  completion, which refuse -db

KB_PROJECT
: the project to act on when none is given and the current directory belongs to
  no project (record verbs; see {app_name}-record(1)). The directory is closer
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
: manage projects — see {app_name}-project(1)

observation
: manage observations — see {app_name}-observation(1)

concept
: manage concepts, show one with what links to it, recall by text — see {app_name}-concept(1)

link, unlink
: link projects/observations to concepts, and remove those links — see
  {app_name}-link(1) and {app_name}-unlink(1)

source
: manage cited sources and retraction checking — see {app_name}-source(1)

search, summary, format
: full-text search and formatted views — see {app_name}-search(1)

merge
: reconcile two knowledge.db files that drifted independently (e.g. across
  machines) into a fresh, deduped output — see {app_name}-merge(1)

check-db
: compare the database with the JSONL dump beside it by full content, and say
  whether to import, export or both — see {app_name}-check-db(1)

export, import
: write/read a portable JSON-L snapshot of the database — the no-file-access
  alternative to merge, for syncing over a channel that can only move plain
  text (paste, email, git) — see {app_name}-export(1) and {app_name}-import(1)

ingest
: index a tree of decision records into the knowledge base — see
  {app_name}-ingest(1)

record
: read and maintain decision records — list, pending, show, new, set-status,
  supersede, fmt, fuzzy-tag (near-miss concept mentions) — see
  {app_name}-record(1)

document
: ingest, draft, review and tag narrative documents (Markdown, Fountain,
  text) at graduated abstraction levels — see {app_name}-document(1)

index
: generate a decisions/index.md from a directory of records — see
  {app_name}-index(1)

init
: create a new, empty workspace — see {app_name}-init(1)

completion
: write a shell completion script for bash or PowerShell, or install it — see
  {app_name}-completion(1)

verbs
: print the verb table: every verb, subverb, flag and write class, as text or
  with -json — see {app_name}-verbs(1)

# ARGUMENTS

The global options -json, -db and -debug go before the verb: {app_name}
-json project list. After the verb they are refused, not ignored. So is any
other flag a verb does not have, and any surplus argument.
init, index, merge and completion never open the ambient database, so -db is
refused for them as well; init takes its target as kb init PATH, merge as -a, -b
and -out.

A name that begins with a dash is given after --, as in
{app_name} project show -- -name, so that it is not read as a flag. Every verb
that takes a name, title or path accepts --, record, document, ingest and
source add included. Words that follow a verb's fixed arguments and are free
text (an observation body, a project description, a retraction note) are taken
as they are, dashes included.

A project or concept name is one line. Surrounding whitespace is trimmed and
any interior run of whitespace, a newline or a tab included, becomes a single
space, so a [[wikilink]] wrapped across two lines names the same concept as
the one-line spelling. A name with any other control character is refused.

# EXIT STATUS

{app_name} follows the workspace exit-code convention (workspace DR-0003, applied
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

{app_name}-project(1), {app_name}-observation(1), {app_name}-concept(1),
{app_name}-link(1), {app_name}-source(1), {app_name}-search(1),
{app_name}-merge(1), {app_name}-export(1), {app_name}-import(1),
{app_name}-ingest(1), {app_name}-record(1), {app_name}-document(1),
{app_name}-index(1), {app_name}-init(1), {app_name}-completion(1),
{app_name}-topics(1)

`

// DocumentHelpText is shown by `kb document -h` and `kb help document`.
// Generates kb-document.1.md. Stub: only `ingest` exists so far
// (narrative-documents-plan.md W3); review/draft/promote/list/show land in
// W5/W8, and this page grows a full DESCRIPTION then.
const DocumentHelpText = `%{app_name}-document(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-document — ingest and review narrative documents at graduated abstraction levels

# SYNOPSIS

{app_name} document ingest PATH --project P [--title T] [--format F] [--dry-run]

{app_name} document list [--project P]

{app_name} document show ID

{app_name} document review list [--project P] [--status S]

{app_name} document draft SECTION_ID BODY --by WHO [--confidence N]

{app_name} document review promote SECTION_ID

{app_name} document tag --project P [--concept NAME,...] [--dry-run]

{app_name} document fuzzy-tag --project P [--concept NAME,...] [--dry-run]

{app_name} document frontmatter PATH [--accept FIELD,...] [--accept-keywords NAME,...] [--set FIELD=VALUE] [--dry-run]

{app_name} document delete ID [--force] [--dry-run]

# DESCRIPTION

A document is a narrative or article (Markdown, Fountain, or plain text)
ingested into two tables: one documents row plus one document_sections row
per structural unit, always including one ` + "`level = 'gist'`" + ` row for the
whole document. Unlike a decision record, a document takes no required
frontmatter -- title/author/publication-date/keywords are recognized when
present (antennaApp's own frontmatter vocabulary: title, description,
pubDate, author, keywords) but never required, since a story or screenplay
usually has none.

Every ` + "`[[Name]]`" + ` wikilink in a section's body, and every frontmatter
keyword, resolves to a concept and links to the record the same way
{app_name}-ingest(1) tags decision records -- case-insensitive, and
matching case becomes canonical for later mentions. A section's
tag_density (how many known concepts its text mentions, tagged or not) is
a mechanical triage signal, not a link -- see review, below.

Re-ingesting a changed file matches new sections to old ones by heading
text: an unchanged section is left alone; a changed one updates its body
and is flagged stale rather than losing its summary; a heading with no
match is reported, never deleted.

ingest
: parse and segment PATH, creating the document and its sections on first
  ingest, or reconciling a changed file against its existing rows

list
: print matching documents, one per line

show
: print one document's metadata plus every section (heading, summary
  status, stale flag, linked concepts) in one view

review list
: the triage queue -- sections needing attention, with their size,
  tag_density and confidence signals. Defaults to unsummarized and
  drafted sections; --status narrows to one exact status, including
  "reviewed" for auditing

draft
: write a gist or section summary. WHO is free text ("human" or a model
  identifier) -- {app_name} never calls a model itself, drafting is always
  supplied by the caller. Clears a section's stale flag, since a fresh
  draft is not stale against itself

review promote
: promote a drafted summary to reviewed -- the only human action that
  makes a summary trusted. Only a reviewed summary is indexed for search
  or returned as content by a concept-tag query; a section's raw body is
  never indexed at all, only its reviewed summary. It requires an
  interactive terminal on standard input and output (exit 2 without one,
  nothing written, no way to turn that off; see EXIT STATUS), so a model
  that drafted a summary cannot also make it trusted. The library call
  PromoteDocumentSummary has no such rule: a program that links the
  package, harvey for one, enforces its own

tag
: a pure file operation over every already-ingested document in --project:
  inserts an explicit ` + "`[[Name]]`" + ` wikilink, at its first safe occurrence,
  for each eligible concept. Never writes to the database -- the next
  ingest of a changed file links it through the wikilink path above,
  unchanged. With no --concept, eligible means mentioned more than once in
  the file outside code spans (the same threshold density-linking already
  applies); --concept NAME,... forces exactly those names regardless of
  occurrence count, but each must already be a known concept, checked
  before any file is touched. A name already wikilinked anywhere in a file
  is left alone, so a second run is a no-op; YAML frontmatter and the
  document's own first H1 heading (its title) are never written into
  either, even if a concept name genuinely occurs there. --dry-run reports
  without writing. See DR-0029 (knowledge/decisions/).

fuzzy-tag
: mirrors tag's shape, catching what tag's exact matching can't: a typo,
  plural, or simple tense variant of a known concept's name (Levenshtein
  distance, not paraphrase). Never bracket-wraps the near-miss text itself
  -- doing so would mint a duplicate concept from the misspelled or
  inflected spelling. Instead inserts a ` + "`[^n]`" + ` footnote marker at the
  match and a footnote definition carrying the canonical
  ` + "`[[Concept]]`" + ` elsewhere, so prose stays unedited and linking still
  resolves to the real concept. A concept already an exact match anywhere
  in the file is skipped entirely -- fuzzy matching is additive, never a
  duplicate of what tag already covers. With no --concept, eligible means
  within a length-based distance threshold; --concept NAME,... bypasses
  that threshold for exactly the names given, each already a known
  concept, checked before any file is touched. A concept already
  footnoted (or wikilinked) anywhere in a file is left alone, so a second
  run is a no-op. --dry-run reports without writing. Decision records have
  their own verb, {app_name}-record(1)'s fuzzy-tag: it adds the concept to
  the record's tags: instead of a footnote, never edits an accepted record,
  and skips a concept only when the record already links it, not when its
  plain name appears in the prose.

frontmatter
: propose-then-accept ` + "`title`" + `/` + "`author`" + `/` + "`dateCreated`" + `/
  ` + "`dateModified`" + `/` + "`keywords`" + ` for one document, PATH -- unlike
  tag/fuzzy-tag, not scoped by --project, since a human reviews one file at
  a time. A bare invocation (no --accept/--accept-keywords/--set) only
  reports; nothing is ever written without an explicit selection.
  ` + "`title`" + `/` + "`author`" + `/` + "`dateCreated`" + ` are absent-only -- never
  overwritten once present. ` + "`author`" + ` is a layered signal: a byline in
  the document's own prose wins, then git's earliest-commit author, then
  ` + "`git config user.name`" + ` -- when signals disagree, the report shows all
  of them, not just the winner. ` + "`dateModified`" + ` is the one exception to
  absent-only: always recomputed from git's most recent commit (filesystem
  mtime otherwise) and rewritten on every accepted run, since a stale value
  is worse than a missing one. ` + "`keywords`" + ` diffs the current list against
  two candidate sets: known concepts already mentioned in the text (a plain
  write on ` + "`--accept-keywords`" + `), and new candidate terms distinctive to
  this document specifically, scored against the rest of its project (or
  the whole corpus) with the document itself excluded from that comparison
  -- accepting one creates the concept first, the same checked-before-any-
  write discipline ` + "`tag`" + `'s ` + "`--concept`" + ` already holds to.
  ` + "`--set FIELD=VALUE`" + ` (repeatable) bypasses signal detection and the
  absent-only rule entirely -- a human's explicit assertion, not a
  proposal. Editing is surgical: only the frontmatter block changes, via a
  YAML node edit that leaves every other key, value, and comment in the
  block untouched; the document body is never modified. This module's
  first process dependency -- shells out to ` + "`git`" + ` for provenance,
  falling back to filesystem timestamps when ` + "`git`" + ` itself fails (not a
  repository, or a genuinely untracked file).

delete
: delete a document, its sections, their concept links and the search entries of
  any promoted summaries. A document with a section whose summary is reviewed is
  refused (exit 1): a reviewed summary is human-gated data and deleting it loses
  it. --force deletes anyway. Ingesting the file again recreates the document,
  without the reviewed summaries. --dry-run reports and changes nothing.

Deletion is local to one database, with no tombstone: {app_name}-merge(1) and
{app_name}-import(1) from a database that still has the row bring it back, so
delete it there as well. ` + "`agents/knowledge.jsonl`" + ` is re-exported from the database,
so a delete reaches a database rebuilt from it. See DR-0050 (knowledge/decisions/).

# VOCABULARIES

summary_status
: unsummarized, drafted, reviewed -- promotion is a one-way, human-only
  action for a first pass; no path un-reviews a promoted summary yet

# EXIT STATUS

The workspace convention, as described in {app_name}(1): 0 success; 1 the
command ran and the answer is no (no such document or section, a section that is
not drafted so there is nothing to promote); 2 the command line is wrong. Setting
a summary to reviewed is a person's act: review promote exits 2, and writes
nothing, unless standard input and standard output are both a terminal, with a
message that a person must promote it. No flag or environment variable turns that
off. draft, review list and every other verb here need no terminal, so a model or
a script can draft but not make a summary trusted.

# SEE ALSO

{app_name}-record(1), {app_name}-ingest(1), {app_name}-search(1)

`

// ProjectHelpText is shown by `kb project -h` and `kb help project`.
// Generates kb-project.1.md.
const ProjectHelpText = `%{app_name}-project(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-project — manage projects

# SYNOPSIS

{app_name} project add [--status concept|active|paused|concluded] NAME [DESCRIPTION]

{app_name} project list

{app_name} project show NAME

{app_name} project concepts NAME

{app_name} project set-status NAME STATUS

{app_name} project set-description NAME DESCRIPTION

{app_name} project rename [--root PATH] [--dry-run] OLD NEW

{app_name} project delete NAME [--force] [--dry-run]

# DESCRIPTION

A project is the top-level container observations and concepts attach to.
Names are unique; adding a project whose name exists is refused (exit 1,
"already exists", naming its id) and changes nothing -- use set-status and
set-description to change either. The name workspace, in any case, is reserved
(exit 2) for add and rename: it names the workspace tier in a record reference
such as workspace/DR-0003, so a project of that name could never be reached.

add
: create a project. --status sets the initial status (default: active) and
  goes before NAME: a --status typed after NAME is a usage error (exit 2)
  rather than description text. Put -- before NAME to keep a flag-looking
  word in the description.

list
: list every project (bare rows — see {app_name}-format(1) for an
  assembled Markdown view with linked concepts/observations included)

show
: show a single project by name

concepts
: list the concepts linked to a project

set-status
: change an existing project's status. STATUS must be one of concept,
  active, paused, concluded.

set-description
: replace an existing project's description, reindexing it for
  {app_name}-search(1). A description is grounding context -- it is what
  show prints and what search returns -- so this is how one that has gone
  stale gets corrected. Trailing words are joined with a space, as in add;
  pass an explicit empty string to clear the description entirely.

rename
: rename a project and reindex it for search. Refuses if NEW already names
  another project. If the project owns records, first rewrites every owned
  record's project: frontmatter to NEW -- both-or-neither, rolling back
  every file already written if any write fails -- before renaming the
  project row; no record's database row is touched, so the next
  {app_name}-ingest(1) of that corpus sees a changed checksum against an
  unchanged identity and updates in place rather than minting a phantom
  project. --dry-run reports which files would be rewritten without
  writing anything. --root sets the workspace root record paths are
  relative to (default: inferred from the database path). See DR-0026
  (knowledge/decisions/), which supersedes DR-0024's outright refusal.

delete
: delete a project. Meant for a stray empty one (a failed ingest can leave one).
  A project that owns any observation, record or document is refused, with
  --force or without: one command must never destroy work, so delete or move
  the content first (exit 1). A project attached only to concepts is refused
  unless --force, which removes those links and the project; the concepts stay.
  --dry-run reports what would happen and changes nothing. A NAME that looks
  like a flag is given after --.

Deletion is local to one database, with no tombstone: {app_name}-merge(1) and
{app_name}-import(1) from a database that still has the row bring it back, so
delete it there as well. ` + "`agents/knowledge.jsonl`" + ` is re-exported from the database,
so a delete reaches a database rebuilt from it. See DR-0050 (knowledge/decisions/).

# CAVEATS

A description, status, or name edited on two machines now reconciles: both
{app_name}-merge(1) and {app_name}-import(1) adopt whichever side's
updated_at is later (DR-0025, generalized to name by DR-0026), so a project
renamed on one machine and left untouched on another arrives as one renamed
project, not two, regardless of merge/import order. See
DR-0024/DR-0025/DR-0026 (knowledge/decisions/).

# SEE ALSO

{app_name}-observation(1), {app_name}-link(1), {app_name}-search(1),
{app_name}-merge(1)

`

// ObservationHelpText is shown by `kb observation -h` and `kb help observation`.
// Generates kb-observation.1.md.
const ObservationHelpText = `%{app_name}-observation(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-observation — manage observations

# SYNOPSIS

{app_name} observation add --project NAME KIND BODY [--source-doi DOI]

{app_name} observation list --project NAME

{app_name} observation show ID

{app_name} observation update ID BODY...

{app_name} observation sources ID

{app_name} observation delete ID [--force] [--dry-run]

# DESCRIPTION

An observation is a timestamped note attached to a project. KIND is one of
note, finding, decision, question, or hypothesis.

add
: record a new observation under --project; --source-doi records the
  normalized DOI of the paper it was extracted from, if any

list
: list a project's observations, most recent first

show
: show a single observation by id, including its resolved supersedes/
  superseded_by relations if it has any

update
: correct an observation by superseding it, not by mutating it. ID's body
  is never touched; a new observation is inserted with BODY, inheriting ID's
  project and kind, and linked to ID by a supersedes edge. The original
  wording survives unchanged, so nothing needs a separate revision history --
  see DR-0023 (knowledge/decisions/). Cross-machine reconciliation carries
  the new supersedes edge the same as every other relation in this schema;
  there is no per-field last-writer-wins here, unlike
  {app_name}-project(1)'s set-description

sources
: list the sources cited by an observation (see {app_name}-source(1))

delete
: delete an observation and its search entry. One with concept links, source
  links, or supersession relations (in either direction) is refused with the
  counts (exit 1); --force removes those links and relations and the
  observation, and the concepts, sources and other observations stay. To remove
  a single link instead, see {app_name}-unlink(1). --dry-run reports and changes
  nothing.

Deletion is local to one database, with no tombstone: {app_name}-merge(1) and
{app_name}-import(1) from a database that still has the row bring it back, so
delete it there as well. ` + "`agents/knowledge.jsonl`" + ` is re-exported from the database,
so a delete reaches a database rebuilt from it. See DR-0050 (knowledge/decisions/).

# SEE ALSO

{app_name}-project(1), {app_name}-link(1), {app_name}-source(1)

`

// ConceptHelpText is shown by `kb concept -h` and `kb help concept`.
// Generates kb-concept.1.md.
const ConceptHelpText = `%{app_name}-concept(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-concept — manage concepts

# SYNOPSIS

{app_name} concept add NAME [DESCRIPTION] [--identifier-type T --identifier-value V]

{app_name} concept list

{app_name} concept show NAME [--limit N]

{app_name} concept recall [TEXT... | -] [--concept NAME,...] [--project NAME] [--limit N]

{app_name} concept rename OLD NEW

{app_name} concept delete NAME [--force] [--dry-run]

{app_name} concept suggest [--project NAME] [--limit N]

# DESCRIPTION

A concept is a named idea or term that can be linked to projects and
observations (see {app_name}-link(1)). Names are unique.

add on an existing name updates that concept rather than creating a second
one, which is also how a concept's description is corrected -- there is no
separate set-description here, unlike {app_name}-project(1). An omitted or
empty DESCRIPTION preserves the stored one rather than clearing it, so
running add just to assert a concept exists cannot lose text. The same holds
for --identifier-type and --identifier-value.

A concept may also represent a scholarly entity — a paper, person,
institution, or funder — by setting --identifier-type (e.g. doi, orcid,
ror, fundref) and --identifier-value (the normalized identifier).

show
: read-only: one concept's description, identifier if set, and what links to
  it. Per kind (projects, observations, records, document sections) it prints
  the full count and up to --limit items, newest first (default 10; --limit 0
  prints counts only). NAME must match exactly, including case, as delete
  does; a miss is exit 1, and when a name differing only in case exists it is
  offered ("did you mean"). A NAME that looks like a flag is passed after --.
  ` + "`--json`" + ` gives one object: name, description, identifier_type,
  identifier_value, counts, and a list per kind. This is how to judge whether
  a {app_name} concept suggest candidate is already covered, without raw SQL.
  See DR-0051 (knowledge/decisions/).

recall
: read-only: find the concepts named in some text, then list the
  observations, records and document sections linked to them. Text comes from
  the arguments, or from stdin when the argument is -. --concept NAME,... names
  concepts directly (case-insensitive; an unknown name is skipped); given with
  text, the two are unioned. --project NAME restricts hits to one project
  (workspace-tier records are then excluded; an unknown project is exit 1).
  --limit N caps the hits (default 10). Output starts with a
  ` + "`matched concepts:`" + ` line, then one line per hit: kind, id, project, the
  matched concepts it links to, and an excerpt. Ranking is the number of matched
  concepts descending, then recency. Projects linked to a matched concept are
  listed on a ` + "`projects:`" + ` line, apart from the hits. Text that matches no
  concept says so and exits 1; no text and no --concept is exit 2. A document
  section's excerpt is its summary only once reviewed. ` + "`--json`" + ` gives
  matched, hits (kind, id, project, concepts, excerpt) and projects. Nothing is
  written and no concept is created. It shares its ranking with the library's
  RecallByConceptNames, which is unchanged and adds no project filter or
  per-hit concepts.
  See DR-0051 (knowledge/decisions/).

rename
: rename a concept and reindex it for search. Refuses only if NEW already
  names another concept -- unlike {app_name}-project(1)'s rename, a concept
  has no corpus of external files to desync, since every link to it
  (record_concepts, observation_concepts, project_concepts,
  document_section_concepts) is a foreign key, never a name matched from a
  file. See DR-0024 (knowledge/decisions/).

delete
: remove a concept, its links, and its search entry. NAME must match exactly,
  including case. A concept still linked to a project, observation, record or
  document section is refused (exit 1, the current state forbids it), with the
  counts, and nothing changes; --force
  unlinks it from all of them and deletes it (the projects, observations,
  records and documents themselves are untouched, only the links go).
  --dry-run reports what would happen and changes nothing. A NAME that looks
  like a flag (---, -x) is passed after --, as everywhere else. The concepts
  most worth deleting are junk ones minted from a documentation example, so
  this exists to remove them. See DR-0038 (knowledge/decisions/).

  Two things delete does not do, and prints a note about each. It does not
  touch files: a record or document that still contains [[Name]], or lists the
  name in tags or keywords, recreates the concept when that file is next ingested
  after it changes (an unchanged file is skipped) or when the database is rebuilt
  from the files,
  so remove the mention too. And it does not propagate: a database that still
  holds the concept brings it back on the next {app_name}-merge(1) or
  {app_name}-import(1) into this one, so delete it there as well.

suggest
: read-only: scans every record body and document section body (scoped to
  one project with --project) and prints candidate new concepts, ranked by
  corpus-wide distinctiveness -- a term mentioned several times but
  confined to relatively few items, rather than spread evenly across
  nearly all of them (not distinctive) or mentioned only once anywhere
  (too weak a signal alone). Code spans and fenced code blocks are
  excluded, a name already naming an existing concept is never suggested
  again, and a bare record reference (dr-0013, adr-0004) is filtered
  outright rather than scored. --limit caps the number printed (default
  20). Never writes anything -- a suggestion becomes a real concept only
  when a human runs concept add. See DR-0028 (knowledge/decisions/).

  Spelling variants of the same underlying term (` + "`chunking`" + `/
  ` + "`chunkings`" + `/` + "`chunked`" + `) are merged into one candidate before
  scoring, not after -- individually each might fall below the occurrence
  or distinctiveness floor even though the combined mentions clearly
  signal one real concept. The merged candidate's name is its highest-
  occurrence spelling; other spellings are printed inline,
  ` + "`chunking (+chunkings, chunked)`" + `. Always on, no flag. A candidate
  term that's instead a fuzzy near-miss of an *already-known* concept is
  excluded from candidacy outright -- that's ` + "`fuzzy-tag`" + `'s job, not
  this command's -- and reported separately in a trailing
  ` + "`near-existing (excluded from candidates):`" + ` section, printed only
  when non-empty. ` + "`--json`" + ` carries both:
  ` + "`{\"candidates\": [...], \"near_existing\": [...]}`" + `. See DR-0034
  (knowledge/decisions/).

# CAVEATS

A description or name edited on two machines now reconciles: both
{app_name}-merge(1) and {app_name}-import(1) adopt whichever side's
updated_at is later (DR-0025, generalized to name by DR-0026), so a concept
renamed on one machine and left untouched on another arrives as one renamed
concept, not two, regardless of merge/import order.

A deleted concept is not remembered. Deletion is local to one database: there is
no tombstone, so {app_name}-merge(1) and {app_name}-import(1) treat a concept
the other side still has as new and add it back. Under the authoritative
agents/knowledge.jsonl flow (each machine rebuilds its database from the
committed export) a deletion travels with the next export, provided every
machine rebuilds before it exports.

# EXIT STATUS

The workspace convention, as described in {app_name}(1). For concept show and
concept recall: 0 success; 1 no such concept (show), or no concept matched or no
such project (recall); 2 a missing, surplus or malformed argument, an unknown
flag, a negative or non-numeric --limit, or no text and no --concept; 66 no
workspace here; 74 stdin could not be read.

# SEE ALSO

{app_name}-link(1), {app_name}-project(1)

`

// VerbsHelpText is shown by `kb verbs -h` and `kb help verbs`.
// Generates kb-verbs.1.md.
const VerbsHelpText = `%{app_name}-verbs(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-verbs — print the verb table: every verb, subverb, flag and what it does to the database

# SYNOPSIS

{app_name} verbs

{app_name} -json verbs

# DESCRIPTION

Prints the one table that describes {app_name}'s command language, so a person,
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
{app_name} -json verbs, not {app_name} verbs -json: the global options go before the
verb.

# EXIT STATUS

The workspace convention, as described in {app_name}(1): 0 success; 2 an unknown
flag or a surplus argument, or -db, which does not apply to this verb.

# SEE ALSO

{app_name}(1), {app_name}-completion(1)

`

// UnlinkHelpText is shown by `kb unlink -h` and `kb help unlink`.
const UnlinkHelpText = `%{app_name}-unlink(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-unlink — remove a link between a project or observation and a concept, or an observation and a source

# SYNOPSIS

{app_name} unlink project PROJECT_NAME CONCEPT_NAME

{app_name} unlink observation OBS_ID CONCEPT_NAME

{app_name} unlink source OBS_ID SOURCE_ID

# DESCRIPTION

The inverse of {app_name}-link(1) and of {app_name} source link: it removes one link and
nothing else, so the project, observation, concept or source itself is untouched.
Concept and project names match exactly, including case, as for {app_name} concept
delete: a destructive command must not guess. Removing a link that does not exist is
an error (exit 1), not a silent success, and so is naming a project, concept,
observation or source that does not exist.

To remove a concept from everything at once, see {app_name}-concept(1)'s delete.
Like every delete, an unlink is local to one database: {app_name}-merge(1) and
{app_name}-import(1) from a database that still has the link bring it back.

# SEE ALSO

{app_name}-link(1), {app_name}-source(1), {app_name}-concept(1), {app_name}-observation(1)

`

// LinkHelpText is shown by `kb link -h` and `kb help link`.
// Generates kb-link.1.md.
const LinkHelpText = `%{app_name}-link(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-link — link projects/observations to concepts

# SYNOPSIS

{app_name} link project PROJECT_NAME CONCEPT_NAME

{app_name} link observation OBS_ID CONCEPT_NAME

# DESCRIPTION

Links are many-to-many and idempotent — linking the same pair twice is a
silent no-op.

# SEE ALSO

{app_name}-project(1), {app_name}-observation(1), {app_name}-concept(1), {app_name}-unlink(1)

`

// SourceHelpText is shown by `kb source -h` and `kb help source`.
// Generates kb-source.1.md.
const SourceHelpText = `%{app_name}-source(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-source — manage cited sources and retraction checking

# SYNOPSIS

{app_name} source add TITLE [--doi D] [--url U] [--authors A] [--published DATE] [--publisher P] [--rights R] [--version V]

{app_name} source list

{app_name} source show ID

{app_name} source remove ID

{app_name} source retract ID NOTE

{app_name} source link OBS_ID SOURCE_ID [--relationship R]

{app_name} source check-retractions

# DESCRIPTION

A source is a cited work (paper, page, dataset) an observation can link to
via source link, recording the relationship (default: cited).

add
: register a source; --doi/--url set its identifier (doi takes priority
  if both are given); adding one whose identifier already exists returns
  the existing source's id instead of duplicating it. The title is trimmed
  and must not be blank. --published must be YYYY, YYYY-MM or YYYY-MM-DD and
  a real date. --url must be an absolute URL with a scheme and a host. --doi
  must be the bare form 10.NNNN/suffix; a pasted doi: or https://doi.org/
  prefix is refused rather than rewritten, because a mistyped DOI would
  otherwise be checked against Retraction Watch, find nothing, and read as
  "not retracted". Other identifier types are not checked. Whitespace around
  a value is trimmed

remove
: delete a source — fails (exit 1) if it's still linked to any observation, and
  also for an ID that does not exist

retract
: mark a source retracted with a note (does not delete it)

check-retractions
: query the Retraction Watch API for every registered, non-retracted DOI
  source and mark hits as retracted; requires network access. It tries every
  source even if one lookup fails. A source it could not look up is not "not
  retracted": it is counted as not checked, its last-checked date is left alone,
  and the command exits 69 after printing the counts. Run it again when the
  service is reachable

# SEE ALSO

{app_name}-observation(1)

`

// SearchHelpText is shown by `kb search -h`, `kb summary -h`, `kb format
// -h`, and `kb help search`. Generates kb-search.1.md.
const SearchHelpText = `%{app_name}-search(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-search — full-text search and formatted views

# SYNOPSIS

{app_name} search TERM

{app_name} summary

{app_name} format [--project NAME]

# DESCRIPTION

search
: full-text search across observations, projects, and concepts using the
  FTS5 index. TERM uses standard FTS5 query syntax: multiple words are
  ANDed, "quoted phrases" match exactly, prefix* matches by prefix. A first
  word that starts with a dash is read as a mistyped option and refused
  (exit 2); give a dash-leading term after --, as in {app_name} search -- -x.
  Later words are text as typed, dashes included. Finding nothing is exit 1

summary
: a formatted overview of every project and its most recent observations

format
: a fully-assembled Markdown view of one project (--project NAME) or every
  project (no --project) — concepts and observations included inline,
  unlike the bare rows {app_name}-project(1)'s show/list return

# SEE ALSO

{app_name}-project(1), {app_name}-observation(1)

`

// MergeHelpText is shown by `kb merge -h` and `kb help merge`.
// Generates kb-merge.1.md.
const MergeHelpText = `%{app_name}-merge(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-merge — reconcile two knowledge.db files that drifted independently

# SYNOPSIS

{app_name} merge -a PATH -b PATH -out PATH [-force]

# DESCRIPTION

merge reads two knowledge.db files (e.g. from two machines that have
drifted independently) read-only and writes their deduped union to a
fresh -out file, which must not already exist. It never modifies -a or
-b; placing the merged file into position is left to you. Every table
travels — projects, concepts, sources, observations, decision records and
record relations, plus the four join tables — so a table that would lose
rows says so in the per-table summary rather than merging quietly short.
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
file, so merge refuses first and leaves the -wal untouched. The exit status
says which: 66 for an input that does not exist or is not a file, 65 for a
zero-byte file or one that is not a knowledge base, 2 for -a and -b naming the
same file, 73 when -out already exists, and 65 for an identity collision
reported without -force.

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
deciding whether to merge, import or export, use {app_name}-check-db(1). It is
read-only and shares merge's rules for what counts as the same row.

# SEE ALSO

{app_name}(1), {app_name}-check-db(1), {app_name}-export(1), {app_name}-import(1)

`

// CheckDBHelpText is shown by `kb check-db -h` and `kb help check-db`.
// Generates kb-check-db.1.md.
const CheckDBHelpText = `%{app_name}-check-db(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-check-db — compare the database with its JSONL dump

# SYNOPSIS

{app_name} check-db [--jsonl FILE]

# DESCRIPTION

agents/knowledge.jsonl is the authoritative copy and the agents tree is kept in
git across machines, so the database and the dump drift. check-db says which
way, without changing either. It is read-only.

The dump defaults to the database's path with .jsonl in place of .db;
--jsonl FILE names another. The resolved paths are printed at the top of the
report. Nothing else is searched for, because agents/ can hold several dumps
and a wrong guess gives a confident wrong verdict.

It decides by a full content diff, never by file time. The dump is read into a
scratch database and compared with a copy of this one, table by table, using
the same idea of "the same row" as {app_name}-merge(1): name for projects and
concepts, uuid for observations, sources, documents and sections, workspace,
project, scope and record id for decision records, and the pair of parents for
links. The report lists rows only in the JSONL, rows only in this database, and
rows on both sides that differ (a project's description or status, a concept's
description or identifier, an observation's kind or body, a record's checksum,
and so on), with per-table counts.

The recommendation is one of: in sync; {app_name} import (the JSONL is ahead);
{app_name} export (the database is ahead); or diverged (import, then export).
File modification times are printed as a hint and never change the verdict: a
git pull refreshes a file's time without changing its content.

Rows only in the JSONL are labelled "never received, or deleted here". There
are no tombstones, so a deletion made here looks exactly like a row never
received, and the import recommendation warns that it brings back anything
deleted on purpose. See DR-0053 (knowledge/decisions/).

With --json the report is one object: in_sync, recommendation, database, jsonl,
tables (each with database and jsonl counts, only_in_database, only_in_jsonl and
different), collisions and file_times.

# EXIT STATUS

0
: in sync

1
: not in sync (a check that reports drift); the report is still printed on
  stdout

2
: a bad flag, a missing value for --jsonl, or a surplus argument

65
: the dump is malformed

66
: there is no dump at the path

# SEE ALSO

{app_name}(1), {app_name}-merge(1), {app_name}-export(1), {app_name}-import(1)

`

// ExportHelpText is shown by `kb export -h` and `kb help export`.
// Generates kb-export.1.md.
const ExportHelpText = `%{app_name}-export(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-export — write a portable JSON-L snapshot of the database

# SYNOPSIS

{app_name} export [-project NAME] [-out PATH]

# DESCRIPTION

export writes the knowledge base (or, with -project, just one project and
everything reachable from it — its concepts, sources, observations and
decision records) as newline-delimited JSON to -out, or to stdout when
-out is omitted. Every line is self-describing via a "type" field
(project, concept, source, observation, observation_concept,
observation_relation, project_concept, observation_source, record,
record_relation), in dependency order.

A -project export carries only that project's decision records — a
workspace-tier record belongs to no project, so it has no principled claim
on a scoped export, and appears only in an unscoped one. A record relation
crossing out of the scoped project (to another project, or to the
workspace tier) is likewise excluded along with the record on the far side
of it.

Unlike merge, export never touches a second database file — the resulting
file can be pasted, emailed, or committed to git, then applied elsewhere
with import. This is the no-file-access alternative to merge; when both
databases are reachable as files, merge is the more thorough tool (it
also detects and can reconcile name collisions).

Exit status: 73 when -out cannot be created (it names a directory, or its
directory is missing), 77 when the OS refuses, 74 if a write fails part way, 1
for a -project that does not exist.

With --json, a text confirmation is only meaningful once -out is given
(the JSON-L stream itself has already gone to stdout otherwise): it
becomes a {"lines_written": N, "path": "..."} object instead of the plain
text line.

# SEE ALSO

{app_name}(1), {app_name}-import(1), {app_name}-merge(1)

`

// ImportHelpText is shown by `kb import -h` and `kb help import`.
// Generates kb-import.1.md.
const ImportHelpText = `%{app_name}-import(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-import — apply a JSON-L snapshot (from export) to the database

# SYNOPSIS

{app_name} import [-in PATH]

# DESCRIPTION

import reads a JSON-L stream produced by export — from -in, or stdin when
-in is omitted — and applies it to the already-open --db database, or, without
-db, to the database of the workspace found by walking up from the current
directory. In a fresh clone that has only agents/knowledge.jsonl, run
"{app_name} import -in agents/knowledge.jsonl" from anywhere inside the
workspace to build agents/knowledge.db there.
Projects and concepts are matched by uuid first (DR-0026): a match
reconciles name/description/status by whichever side's updated_at is
later, the same last-writer-wins rule merge uses. A uuid miss falls back
to matching by name (DR-0003) -- an existing local row under that name
wins as-is; a genuinely new one keeps its original uuid, for future
cross-machine merge compatibility. Sources are matched by identifier when
one is present. Observations and links are matched by uuid, so re-running
import against the same file is a no-op the second time.

Decision records are matched by identity — workspace, project, scope and
record id — the same tuple AddRecord and merge use, not by uuid: two
machines' ingest of the same file mint different uuids for it, so a
uuid-keyed match would treat that as new every time and duplicate the
record. An existing local record wins as-is. A record's project is
resolved by name for the same reason. Record relations are matched by
their endpoints' uuids, resolved against the records just imported in this
same run — that stays safe even though records themselves aren't uuid-keyed,
because the cache is built fresh from this file's own uuids on the way in.

Unresolvable references (a uuid the file never defines a parent for) and
unrecognized record types are skipped, not fatal — only malformed JSON
aborts the import. The returned summary reports, per record type, how many
lines were read, newly imported, or skipped. Exit status: 65 for malformed JSON
or a row the schema rejects, 66 when -in does not exist, 77 when it cannot be
read, 74 for a read that fails part way.

An observation whose kind is outside the current vocabulary (note, finding,
decision, question, hypothesis) is imported with its kind unchanged and
reported as a "warning:" line (a "Warnings" list under --json). It is never
rewritten to note, and the run still exits 0: a value written before the
vocabulary was enforced should stay a fixable row, not fail the import or
change silently. check-db therefore agrees with the dump for such a row.

# SEE ALSO

{app_name}(1), {app_name}-export(1), {app_name}-merge(1)

`

// IngestHelpText is the kb-ingest(1) man page.
const IngestHelpText = `%{app_name}-ingest(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-ingest — index a tree of decision records into the knowledge base

# SYNOPSIS

{app_name} ingest PATH [--dry-run] [--root DIR]

# DESCRIPTION

Walks PATH for decision record files named NNNN-slug.md, parses each one's
YAML frontmatter, and upserts it into the records table and the full-text
index. The generated index.md is not a record and is skipped.

A record's identity is its (project, scope, id) triple. Re-ingesting a tree
is cheap and safe to repeat: a file whose checksum is unchanged is skipped.

Ingest runs in two passes. The first stores every record; the second resolves
supersedes and relates_to into relations. A record may cite one the walk has
not reached yet, so a single pass would fail on a forward reference.

A relates_to entry is [SCOPE:]ID — a bare id inherits the citing record's own
project and scope, clasm:0160 names another project, and workspace:0001 names
the workspace tier. An optional DR- prefix is stripped. supersedes and
superseded_by are same-tier only, so a qualified entry in either is reported
as malformed rather than resolved. superseded_by is never stored directly: it
is the inverse of the supersedes on the other record.

A file that cannot be ingested (a record that does not parse, two files claiming
one identity, a file that cannot be read) is reported in the summary and the rest
are still ingested, but the command then exits with the class of the first such
failure, in path order (65 for wrong content, 77 for a file it may not read, and
so on), instead of 0. A warning or an unresolved reference is not a failure.

Nothing about a reference is fatal. A target that is not in the database yet
leaves the relation unwritten and adds a line to the summary; re-run once it
has been ingested. Failing instead would make ingest order significant, which
is what the two passes exist to avoid.

Ingest is additive. A record whose file has vanished stays in the database and
is reported, never deleted — pruning would destroy data on a partial or
wrong-directory run. Ingest never writes to a record file; only record does.

Ingest is how a status written in a record file reaches the database, without the
transition table or the terminal rule that {app_name} record set-status applies: the
file is the source of truth, so a status edited by hand is stored as it stands. The
summary reports it, so the bypass is visible and never silent. A line
"status: clasm/DR-0012 proposed -> accepted (edited in file)" means the stored status
differed from the file's; "status: clasm/DR-0013 arrived accepted" means a record was
first ingested already accepted. It is a report and not a failure, the status is
applied, and a move that set-status would refuse is reported like any other. The text
lists the first 20 (a whole tree ingested into an empty database arrives every
accepted record at once) and says how many it left out; -json {app_name} ingest
lists them all in "status_changes", each with "ref", "from", "to" and "kind"
("edited" or "arrived"). --dry-run reports the same lines and writes nothing. A status
written by set-status is in the file and the database together, so the next ingest
does not report it.

A [[wikilink]] or a tag that matches no concept creates one, silently, so a typo
makes a concept. The summary says which: "concept: created Alpha, Beta" for a run
that wrote, "concept: would create Alpha, Beta" for --dry-run, which creates none;
-json {app_name} ingest lists them all in "concepts_created". A name that differs
only in case from an existing concept is that concept, not a new one, and a
[[DR-0012]] is a record reference and is warned about, not made a concept.

Every [[Name]] found in a record's body, and every entry in its frontmatter
tags list, is resolved to a concept and linked to the record (kb record
concepts shows the result). A name that does not match an existing concept
creates one; matching is case-insensitive, so [[Computer]] and [[computer]]
resolve to the same concept regardless of where each mention falls in a
sentence, and the casing of whichever mention is resolved first becomes
canonical. This does not change kb concept add, which stays exact-match.

# OPTIONS

--dry-run
: report the same counts and write nothing

--root DIR
: treat DIR as the workspace root that stored paths are relative to.
  Defaults to the parent of the directory holding the database, so
  --db agents/knowledge.db gives a root of the workspace itself. Paths are
  stored relative because absolute ones do not survive merge between machines.

# EXAMPLES

Index one project's records, then the workspace tier:

~~~shell
{app_name} ingest clasm/decisions
{app_name} ingest agents/decisions
~~~

Preview without writing:

~~~shell
{app_name} ingest clasm/decisions --dry-run
~~~

`

// RecordHelpText is the kb-record(1) man page.
const RecordHelpText = `%{app_name}-record(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-record — read and maintain decision records

# SYNOPSIS

{app_name} record pending [SCOPE...] [--all] [--kind K] [--trigger T] [--initiative I] [--since DATE]

{app_name} record list [SCOPE...] [--all] [--status S] [--kind K] [--trigger T] [--initiative I] [--since DATE]

{app_name} record show RECORD_REF [--project P] [--workspace]

{app_name} record set-status RECORD_REF [STATUS] [--project P] [--workspace] [--root DIR]

{app_name} record supersede NEW_REF OLD_REF [--partial] [--project P] [--workspace] [--root DIR]

{app_name} record new --title T --trigger G [--project P | --workspace] [--kind K] [--dir DIR] [--root DIR]

{app_name} record fmt PATH [--dry-run]

{app_name} record concepts RECORD_REF [--project P] [--workspace]

{app_name} record fuzzy-tag --project P [--concept NAME,...] [--write] [--dry-run] [--root DIR]

{app_name} record delete RECORD_REF [--project P] [--workspace] [--root DIR] [--dry-run]

# DESCRIPTION

A decision record is one file, indexed by ingest. new writes a project-scoped
record to agents/projects/PROJECT/decisions/ by default (--dir overrides
this) and a workspace-scoped one to agents/decisions/. Records are listed
oldest first, sorted by date and then by id — never by id alone, because ids
are identity, not chronology: a correction can carry a lower id than the
record it supersedes.

A record id is not by itself an identity, since two projects may each have a
DR-0001, and listings print the qualified form for that reason. A record is named by SCOPE/DR-NNNN, where SCOPE is a project name or
workspace (the workspace directory's own name is accepted as an alias for it,
and a project of that name wins): harvey/DR-0004, workspace/DR-0003. A bare id
resolves only where one record has it; otherwise the command lists the
qualified candidates rather than choosing one. A command that changes a record
(set-status, supersede, delete) never acts on a bare id with no scope: say
harvey/DR-0004, run it from inside the project, or set KB_PROJECT.

Scope. list takes scopes as arguments: kb record list harvey clasm workspace.
With none it uses the project the working directory belongs to (under
agents/projects/NAME/, or in a repository directory NAME/ beside it), else
the project KB_PROJECT names, else the whole workspace; --all widens it to the
whole workspace. An unknown scope is exit 1. --project and --workspace remain
as aliases for one scope and cannot be mixed with scope arguments or --all.

list
: print matching records, one per line, each starting with its SCOPE/DR-NNNN
  reference. --status, --kind, --trigger and
  --initiative filter on those fields and --since DATE (YYYY, YYYY-MM or
  YYYY-MM-DD) keeps records dated on or after it; filters combine.
  "no matching records" means a real filter matched nothing. A value that
  no record carries and the vocabularies below do not list is a typo, and
  is an error that names what is known (exit 1, a lookup that found nothing;
  record new and set-status, which write a value, exit 2 for the same thing).
  A bare record id that exists in more than one tier is ambiguous and exits 2,
  listing the qualified forms. A value outside the vocabularies
  that some record does carry still filters. --workspace and --project
  cannot be combined, since workspace-tier records have no project

pending
: list the records waiting for a decision: every record whose status is
  proposed, oldest first, in the scope list would use. The other list filters
  apply; --status does not (exit 2), since the status is what pending means.
  Nothing pending is success

show
: print one record with its body and its relations resolved in both
  directions, every one named SCOPE/DR-NNNN. Only supersedes is stored;
  superseded_by is its inverse

set-status
: set a record's status in both its file and the database. The promotion path
  from proposed to accepted. Also refreshes the corpus's index.md if one is
  already present, since status is one of the fields it renders — never
  creates one where the corpus has not already opted in. A status outside the
  vocabularies below is refused (exit 2) unless a record already carries it, the
  rule record new applies to --trigger and --kind and record list applies to its
  filters, so a typo such as "acepted" cannot leave a record in limbo; the file and
  database are untouched. Ingest of a hand-edited file still only warns.
  Only the moves in the transition table are made: proposed to accepted,
  rejected, cancelled or superseded; accepted to cancelled or superseded; rejected or
  cancelled back to proposed; superseded is final. superseded also needs
  superseded_by already set, so use record supersede, which writes both sides.
  A move outside the table, including setting the status a record already has,
  is refused (exit 1) with the allowed moves named, and nothing is written. A
  record whose status is outside the vocabulary can be set to proposed and
  nothing else, so it reaches accepted in two moves.
  Setting a record to accepted is for a person: standard input and standard
  output must both be a terminal, or the command exits 2 with a message saying
  so and writes nothing. No flag or environment variable turns that off, and
  every other move works without a terminal, so a script can propose, reject,
  cancel and supersede but not accept.
  With no STATUS it is a review, for a person at a terminal: the record is
  shown through $KB_PAGER (else bat, else less -R, else printed), then the
  prompt names the current status and the moves the table allows, for example
  harvey/DR-0004 is proposed -> [a]ccepted [r]ejected [c]ancelled [q]uit. A
  status is chosen by pressing its first letter, at once, with no Enter. A
  confirmation follows (Set ... from proposed to accepted?) and nothing is
  written until you press y. Esc, q or Ctrl-C backs out at either step, as does
  n at the confirmation, again at once; Enter does nothing, so a habitual Enter is
  never consent to a write. Backing out, or end of input when the keys come from
  a pipe, leaves the record unchanged and is exit 1, since the command ran and
  the answer is no. The review without a terminal, or with --json, is exit 2;
  the form with a STATUS is what a script uses

supersede
: write both sides of a supersession — supersedes on NEW, superseded_by on
  OLD, the relation, and unless --partial, OLD's superseded status. Both
  files and the database are written together or not at all. Also refreshes
  index.md, same as set-status, for both NEW's and OLD's corpora if either
  already has one

new
: scaffold a record: allocate the next id for the tier, fill the fields a
  tool owns, set status to proposed, and print all five body headings whether
  or not they get filled. Writes the file; does not ingest it. --trigger is
  required here even though a converted record may carry an empty one,
  because on a newly authored record it is cheap and accurate to say where
  the need was discovered. --trigger and --kind follow the rule record list
  uses for its filters: a value outside the vocabularies below is refused
  unless a record already in the database carries it, and the error names
  what is known. That catches a typo before it is written into a file, where
  it would otherwise become a value the filter rule accepts

fmt
: rewrite every record under PATH into canonical form. This is the
  normalisation path ingest deliberately lacks, since ingest never writes to
  a record file

concepts
: list the concepts ingest linked to a record, from [[Name]] wikilinks in its
  body and its frontmatter tags list

fuzzy-tag
: report near-miss spellings of known concepts in a project's records (a
  detoast for the concept toast), which exact matching never links. Per
  record it lists the concept, the variants found, how many times, the plain
  exact mentions that link nothing (reported as "exact mention, not linked")
  and the tags: line that would link the concept. Only the body is searched,
  never the frontmatter. A concept is skipped for a record only when the record
  already links it, in tags: or as a [[wikilink]]. Without --concept the same
  conservative length and distance rules as {app_name}-document(1)'s fuzzy-tag
  apply; --concept NAME,... names concepts whose variants are reported up to the
  matcher's ceiling (a 5-letter concept such as toast is under the default length
  floor, so name it). By default nothing is written. --write adds the concept to
  the tags: of records whose status is proposed and reports how many accepted
  records it skipped; an accepted record is history and is never modified, nor is
  any other status. Only the tags: line changes (a one-line flow list, a block
  list, or a new line if absent); a form it will not edit is refused and nothing
  is written, since writes are both-or-neither. --dry-run with --write says what
  would happen and writes nothing. The database is not touched: run
  {app_name} ingest on the records directory to link the new tags. An unknown
  project or concept is exit 1. See DR-0052 (knowledge/decisions/).

delete
: drop the database row of a record whose file is already gone: its relations, its
  concept links and its search entry go with it. A record's file is the truth and
  ingest is additive, so a row whose file vanished otherwise stays (ingest only
  reports it). While the file exists this is refused (exit 1), because the next
  ingest of the changed file would only undo it, and {app_name} never deletes a
  decision record from disk: delete the file yourself first, or retire the record
  with set-status cancelled. --dry-run reports and changes nothing.

new, set-status, supersede, fmt and fuzzy-tag --write are the only commands that
write a record file; ingest never does. A record is written proposed and stays proposed: a
model may write a record, but only the author accepts one.

# EXIT STATUS

The workspace convention, as described in {app_name}(1). A scope that is neither a
project nor the workspace is 1, and so is a set-status move the transition
table does not allow; set-status accepted without a terminal on standard input
and output is 2; a malformed reference, a bare id that is
ambiguous, a change (set-status, supersede, delete) given a bare id with no
scope, and record new with no scope anywhere are 2. For record fuzzy-tag:
0 the report was produced, or the tags were written, including "nothing found";
1 no such project or concept; 2 a bad flag, a missing --project, or a surplus
argument, and on any record verb a --concept or --write it does not take; 65 a
record file that is malformed or whose tags: is in a form the edit will not
change (nothing is written); 66 a record's file is missing, or there is no
workspace here; 74 a write failed part way (writes already made are undone).

# VOCABULARIES

These are the documented values. They are reported against, not enforced: an
unknown value parses and carries a warning, because a typo in a file several
harnesses write should be a fixable row, not a failed run.

status
: proposed, accepted, superseded, rejected, cancelled

  rejected means the decisions were considered and not pursued. superseded
  means a later record replaced them. cancelled means the work was pursued or
  explored and then abandoned; it need not have been accepted first. Its
  reasoning still stands and is where anyone revisiting the question should
  start, but it is not live. Say why in the record's body, since "the need was
  met another way" and "deprioritised" tell a later reader
  different things; there is no field for it. Use kb record set-status ID
  cancelled, without writing a replacement record.

kind
: decision, correction, refinement

trigger
: design, plan-review, implementation, live-test, release-review, request,
  external. May be empty on a record converted from an existing log

# OPTIONS

--all
: on list and pending, every project and the workspace tier, whatever the current
  directory is

--project P
: restrict to, or resolve within, project P. On new and fuzzy-tag it is how the
  project is named. Elsewhere it is deprecated: give the scope as an argument to
  list and pending, or qualify the record as P/DR-NNNN. It still works and says
  so on standard error

--workspace
: restrict to, or resolve within, the workspace tier. On new it names the tier.
  Elsewhere it is deprecated in the same way: use workspace as the scope or
  workspace/DR-NNNN

--partial
: on supersede, leave OLD accepted instead of marking it superseded. Use when
  a later record invalidates one decision inside a multi-decision episode
  while the rest still stand

--title T
: the new record's title, on new. The filename slug is derived from it,
  lowercased with punctuation stripped; the slug is cosmetic and the id is
  the identity

--kind K
: the new record's kind, on new. Defaults to decision

--dir DIR
: on new, write to DIR instead of the default (agents/projects/PROJECT/decisions,
  or agents/decisions on --workspace). Relative to --root, like every other
  stored record path

--dry-run
: on fmt, report what would change and write nothing

--root DIR
: the workspace root that stored record paths are relative to. Defaults to
  the parent of the directory holding the database

# EXAMPLES

Every correction in one project, and everything since a date, then two scopes
at once and the workspace tier:

~~~shell
{app_name} record list clasm --kind correction
{app_name} record list clasm --since 2026-08-01
{app_name} record list clasm cold workspace
~~~

What is waiting for a decision, in every project or in one:

~~~shell
{app_name} record pending --all
{app_name} record pending knowledge
~~~

Inside a project directory the project is the scope, and --all widens it:

~~~shell
cd ~/Laboratory/harvey && {app_name} record list
cd ~/Laboratory/harvey && {app_name} record list --all
~~~

Promote a proposed record, then wholly and partially supersede. A change names
the record by its qualified reference, since a bare id is not enough to act on:

~~~shell
{app_name} record set-status knowledge/DR-0004 accepted
{app_name} record supersede clasm/DR-0149 clasm/DR-0148
{app_name} record supersede clasm/DR-0159 clasm/DR-0160 --partial
~~~

Start a new record, and bring a corpus into canonical form:

~~~shell
{app_name} record new --project clasm --title "Retry the profile attach" --trigger live-test
{app_name} record new --title "Run from inside the project" --trigger design
{app_name} record new --project clasm --title "Filed under the old layout" --trigger request --dir clasm/decisions
{app_name} record fmt clasm/decisions --dry-run
~~~

`

// IndexHelpText is the kb-index(1) man page.
const IndexHelpText = `%{app_name}-index(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-index — generate a decisions/index.md from a directory of records

# SYNOPSIS

{app_name} index PATH [--stdout|--check]

{app_name} index ROOT --all [--check]

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

--check compares PATH/index.md against a fresh render and reports drift as an
error instead of writing: missing, or different from what the current records
would produce. It exits 1 for either (a normal "no": the index is out of date),
so it fits a pre-commit hook or CI step; the remedy either way is running index
without --check. A record that cannot be read or parsed is a failure, not drift,
and exits with its own class instead: 65 for a malformed record, 66 for a PATH
that is missing or not a directory. --check and
--stdout cannot be combined.

--all walks ROOT and processes every directory that already has an index.md
-- the corpora that have opted into this convention -- refreshing or, with
--check, checking each one. A directory with record files but no index.md
yet is silently left alone: --all never creates one, the same rule
kb record set-status/supersede's own automatic refresh follows. Nested
corpora (each with their own index.md) are handled independently, never
folded together. One bad corpus does not stop the rest: --all keeps going
and reports every corpus, then exits non-zero if any needed attention, so a
pre-commit hook can gate on the whole workspace in one call rather than
naming each corpus by hand: 1 if the only trouble is stale indexes, and the
class of the first failure (65 for a malformed record, 77 for a file it may
not write) if any corpus could not be indexed at all, since a failure is more
serious than drift. --all and --stdout cannot be combined. See
TODO.md's index-regeneration item (index --check and the auto-refresh on
set-status/supersede) for the single-corpus half this completes.

Hidden directories are not descended into. A git worktree keeps a whole
second copy of the tree under .claude/worktrees/<name>/, corpus and
generated index.md included, so without this it is reported as a corpus in
its own right -- a duplicate under --check, and in write mode an edit to a
throwaway worktree rather than the real tree. The prune applies only to
directories descended into, never to ROOT itself, so naming a hidden
directory as ROOT still finds the corpora inside it.

# OPTIONS

--stdout
: write the index to standard output instead of index.md (PATH form only)

--check
: verify index.md is current without writing it; exit 1 on drift

--all
: process every already-indexed corpus under ROOT instead of one PATH

# EXAMPLES

~~~shell
{app_name} index clasm/decisions
{app_name} index agents/decisions --stdout | head
{app_name} index agents/decisions --check
{app_name} index . --all
{app_name} index . --all --check
~~~

`

// InitHelpText is the kb-init(1) man page.
const InitHelpText = `%{app_name}-init(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-init — create a new, empty workspace

# SYNOPSIS

{app_name} init [PATH]

# DESCRIPTION

Creates a schema-only PATH/agents/knowledge.db — no data, the same shape as
git init. PATH defaults to the current directory.

Every other {app_name} verb that opens the ambient database (no --db given)
requires one to already exist, and fails toward "{app_name} init" or
"{app_name} import -in FILE" rather than silently creating one in whatever
directory it happens to be run from. init is how a genuinely new workspace,
with no prior agents/knowledge.jsonl to seed from, gets that first database.

A workspace being rebuilt or bootstrapped from an existing export is a
different case: "{app_name} import -in agents/knowledge.jsonl" already
creates a missing target database on its own, so init has nothing to add
there — see {app_name}-import(1).

init is idempotent. Run again against an already-initialized workspace, it
reports that and leaves the existing database untouched; it never truncates
or overwrites data.

# OPTIONS

None beyond the standard options. The global -db option does not apply: init
creates PATH/agents/knowledge.db, so the target is named as the PATH argument,
and an explicit -db is refused (exit 2) rather than ignored.

# EXAMPLES

~~~shell
{app_name} init
{app_name} init ~/NewWorkspace
~~~

`

// CompletionHelpText is the kb-completion(1) man page.
const CompletionHelpText = `%{app_name}-completion(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-completion — write or install a shell completion script

# SYNOPSIS

{app_name} completion bash|powershell [-install]

# DESCRIPTION

Writes a completion script for the named shell to standard output. The shell
name is case-insensitive and pwsh is accepted for powershell. The script
completes:

- the verbs and, after help, the help topics;
- the subverbs of project, observation, concept, source, link, record and
  document (and document review);
- each verb's flags, once a word beginning with a dash is started;
- paths after -db, -in, -out, -a, -b, -jsonl, -root and -dir;
- project names after --project, and after project show, concepts,
  set-status, set-description, rename and delete. They are read by running
  "{app_name} project list" in the current directory, honouring a -db typed
  earlier on the line, so they work only where a workspace is found.

The verb list is read from the verbs {app_name} itself registers, so a new
verb is completed without anyone adding it. The subverbs and flags are kept
in a table, which the tests compare with the code.

Neither form opens a database, so neither needs a workspace.

# OPTIONS

-install
: instead of writing the script to standard output, install it so it loads in
  every new shell, and print where it went. With bash the script goes to
  $XDG_DATA_HOME/bash-completion/completions/{app_name}, or the same path under
  ~/.local/share, which bash-completion loads on demand. A file already there
  is replaced only if {app_name} wrote it; anything else is left alone and the
  command exits 73. With PowerShell the script is written as
  {app_name}-completion.ps1 beside the profile (~/.config/powershell, or
  Documents\PowerShell on Windows), and one line that dot-sources it is added
  to Microsoft.PowerShell_profile.ps1 once. Windows PowerShell 5.1, whose
  profile is under Documents\WindowsPowerShell, is not covered.

The global -db option does not apply and is refused (exit 2).

# EXAMPLES

Try it in the current bash session:

~~~shell
source <({app_name} completion bash)
~~~

Install it for every session:

~~~shell
{app_name} completion bash -install
{app_name} completion powershell -install
~~~

# EXIT STATUS

The workspace convention, as described in {app_name}(1). 0 success; 2 no shell
named, a shell that is not supported, a surplus argument or an unknown flag;
66 no home directory found for -install; 73 -install found a file that
{app_name} did not write, or could not create the directory; 74 a write failed
part way.

# SEE ALSO

{app_name}(1), {app_name}-topics(1)

`

// TopicsHelpText is the topic index: kb help topics.
//
// Named "topics" rather than the conventional "index" because index is a verb
// here, so kb help index is kb-index(1) and cannot also be this. harvey
// accepts both spellings; only this one is free in kb.
const TopicsHelpText = `%{app_name}-topics(1) user manual | version {version} {release_hash}
% R. S. Doiel
% {release_date}

# NAME

{app_name}-topics — the index of help topics

# SYNOPSIS

{app_name} help topics

# DESCRIPTION

Every topic below has its own manual page, reachable as "{app_name} help TOPIC"
or "{app_name} TOPIC -help", and installed as a man page by make install.

project
: manage projects

observation
: manage observations

concept
: manage concepts

link
: link projects and observations to concepts

unlink
: remove a link between a project or observation and a concept, or an observation and a source

source
: manage cited sources and retraction checking

search
: full-text search across projects, observations, concepts and records.
  summary and format share this page

merge
: reconcile two knowledge.db files that drifted independently

check-db
: compare the database with its JSONL dump by content and recommend import,
  export or both

export
: write a portable JSON-L snapshot. import shares this page

import
: read a JSON-L snapshot back

ingest
: index a tree of decision records into the knowledge base

record
: read and maintain decision records

document
: ingest, draft, review and tag narrative documents at graduated
  abstraction levels

index
: generate a decisions/index.md from a directory of records

init
: create a new, empty workspace

completion
: write or install a shell completion script for bash or PowerShell

verbs
: print the verb table, as text or JSON

# NOTES

The conventional spelling for this page is "help index". It is "help topics"
here because index is itself a verb, so "{app_name} help index" is that verb's
manual page and cannot also be the topic list.
`
