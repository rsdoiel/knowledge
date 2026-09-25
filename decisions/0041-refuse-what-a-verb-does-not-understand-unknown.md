---
id: "0041"
title: "Refuse what a verb does not understand: unknown flags, surplus arguments, misplaced global options and --db"
date: "2026-09-24"
status: accepted
kind: correction
trigger: live-test
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0010", "0022", "0039", "0040"]
initiative: ""
session: ""
decisions: []
tags: [cli, arguments, flags, help, v0.0.13]
uuid: "01a0d5ac-46da-7daf-a5bc-2d3182438228"
origin_host: "wren"
---

**Context.**

Most verbs checked only that they had *enough* arguments. Anything else was
dropped, and the command exited 0. Of 25 verbs probed with the v0.0.12 binary, 15
accepted a bogus flag and 19 ignored a surplus argument. The damage was concrete:
`source add T more words` stored the title `T` and lost the rest; `record new
--title A decision` kept only `A`; `source link 1 1 --relationship` with no value
linked anyway; `project list --json` printed plain text and exited 0, ignoring
the option; `kb --db rt.db init` reported success while creating
`./agents/knowledge.db`, and `rt.db` never existed. `index` and `merge` dropped
`--db` the same way. A subverb's help flag was handled differently by every verb:
`project add -help` printed `kb: flag: help requested` and exited 1, `project
show -help` looked up a project named `-help`, and `document frontmatter -help`
tried to open a file called `-help`.

The cause is one habit in three shapes: verbs with plain positionals checked
`len(args) < N`; FlagSet verbs never looked at `fs.Args()`; the `record` verbs
checked `<` where they needed `!=`.

**Decision.**

1. A flag a verb does not have, and an argument beyond its arity, is a usage error
   (DR-0040), not dropped. `plainArgs(args, min, max, fixed, usage)` serves verbs
   with no flags of their own; `noExtraArgs(fs, usage)` serves FlagSet verbs;
   `splitFlags` reports an unknown flag and a flag missing its value (which
   `source link --relationship` now uses); the `record` verbs check for exactly
   the arguments they take.
2. Free text is exempt. A verb whose trailing words are prose (an observation
   body, a project description, a retraction note, a concept description) takes
   them as typed, dashes included. `plainArgs` checks only the first `fixed`
   arguments for flag shape. `source add` takes one TITLE, so a multi-word title
   must be quoted; `search` with a blank term is a usage error.
3. `--` ends flag recognition, as `concept delete` already did (DR-0039). **A name
   that begins with a dash must follow it:** `kb project show -- -name`.
4. `--json`, `--db` and `--debug` are global options and go before the verb. After
   it they are refused, with a message saying where they go.
5. `--db` given to `init`, `index` or `merge` is refused, not honoured. None of
   the three opens the ambient database, and the message names the right form
   (`kb init PATH`; `index` reads record files; `merge` takes `-a`, `-b`, `-out`).
6. A help flag is a request for help only where a verb's first argument would go:
   directly after the verb, or directly after its subverb (and after `document
   review`'s own subverb). That is answered before any database is opened
   (DR-0010), so it works outside a workspace and creates nothing. A help flag
   after other flags reaches the subverb's parser, which reports `flag.ErrHelp`,
   and `dispatch` turns that into the same page with exit 0. It does not scan
   every argument for `-h`.

**Rationale.**

Dropped input changes what a command means, and the exit code and stdout say
nothing happened. Refusing costs one clear message; not refusing cost data
(`record new` with a truncated title) and correctness (`--json` producing text).

Free text is the exemption because those verbs join their trailing arguments on
purpose, and `kb observation add --project p note pass -h to it` has to record
its body. That is also why help is position-limited rather than a scan: a scan
would swallow the body.

`--db` is refused rather than honoured because it names a database *file* while
`init` creates a *workspace* (`PATH/agents/knowledge.db`), and any verb that does
use the database already open-or-creates an explicit path (DR-0022), so nothing
is lost by refusing.

**Rejected alternatives.**

- Warn on stderr and carry on. Scripts ignore stderr, and the truncated title is
  still stored.
- Treat an unrecognised dash-leading token as a name when it is not a known flag.
  It would make a typo'd flag (`--josn`) into a project name, which is the silent
  failure again.
- Scan every argument for a help flag. Breaks free text (above).
- Honour `--db` for `init` by creating a bare database at that path. It gives
  `init` two meanings and skips the workspace layout; `kb --db X project list`
  already creates an empty database at X for anyone who wants one.
- Leave the drop and document it. The drop is the bug; documenting it would
  bless the truncated title.

**Consequences.**

A script that passed `--json` after the verb now fails where it used to print
plain text. A name that begins with a dash needs `--` on the verbs that take one;
junk concept names such as `---` are exactly that case.

Not universal, and worth reviewing: verbs parsed by `splitFlags` (`record`,
`document ingest`, `document list`, `source add`, `ingest`) do not accept `--`, so
a value starting with a dash cannot be given to them. Nothing in a record id,
path or title needs it today.

Nothing forces a new verb to call `plainArgs` or `noExtraArgs`: the guarantee is
the survey (25 verbs, now 0 accepting either), and a test that runs every verb
with a bogus flag would make it permanent. Checked against the v0.0.12 binary
over 65 commands: identical database and identical exit codes apart from the two
intended cases (`--json` after the verb, and a title split by the shell).

Status `proposed`: promotion is the author's call.
