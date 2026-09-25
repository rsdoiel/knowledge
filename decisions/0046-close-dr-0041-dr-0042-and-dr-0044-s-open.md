---
id: "0046"
title: "Close DR-0041, DR-0042 and DR-0044's open questions: -- everywhere, one-line names, no WAL allowance"
date: "2026-09-25"
status: accepted
kind: decision
trigger: live-test
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0039", "0041", "0042", "0044", "0045"]
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a0d974-48cc-7254-aa4b-958870be0b57"
origin_host: "wren"
---

**Context.**

DR-0041, DR-0042 and DR-0044 each closed by naming something left open on
purpose, and on 2026-09-25 RSDOIEL asked to work through them. Each was probed on
scratch data before choosing, and each turned out to be a little different from
how the record described it.

*`--` on `splitFlags` verbs (DR-0041).* `record`, `document`, `ingest` and
`source add` did not accept `--`, so a value that began with a dash could not be
given to them. Only a positional is affected: a flag value already works
(`record new --title "-x"` wrote a record). That leaves a `source add` title,
which has no workaround, and an `ingest` or `document ingest` path, which has
`./`.

*Multi-line names (DR-0042).* A newline inside a project or concept name was
kept. It split a `project list` row across lines, made `record new --project`
create a directory with a newline in its name, and sent control characters to the
terminal as typed. The route that matters was not typing but ingest: the
wikilink pattern `[^\[\]]+` matches a newline, so a hard-wrapped
`[[deterministic<newline>output]]` minted a concept containing a newline, beside
a normal `deterministic output`. Re-ingesting 212 real Markdown files with the
previous binary produced two such concepts (`model<newline>  switch: ...` and
`rag: 3 chunks from sparqlset.db, top score<newline>0.87`).

*The WAL allowance (DR-0044 item 4).* `kb merge` accepted a zero-byte main file
when a non-empty `-wal` sat beside it, on the belief that the data might live
only in the WAL. DR-0044 said so was unverified. It is false. SQLite discards a
WAL whose main file is zero bytes when it opens it, even for a young database
with every page still in the WAL, and only the recently changed pages are in the
WAL of an older one. On that input the shipped binary exited 0, reported zero
rows from `-a`, and the input's `-wal` was gone afterwards.

**Decision.**

1. `splitFlags` treats a bare `--` as the end of flag recognition: everything
   after it is positional, dashes included. This covers every verb it serves
   (`record`, `document`, `ingest`, `source add` and `source link`, among
   others). A flag's value is taken first, so `--by --` gives the flag the value
   `--`. Only the first `--` is consumed.
2. `CleanName` makes a name one line: it trims, collapses every interior run of
   whitespace (newline, tab, a non-breaking space, repeated spaces) to a single
   space, refuses a name that is empty afterwards, and refuses any remaining
   control character (Unicode Cc, for example ESC or NUL). This reverses DR-0042
   item 1's "interior whitespace, including a newline, is left alone".
3. Document ingest and record ingest clean each wikilink and tag through
   `CleanName` before comparing or resolving it, so a wrapped wikilink is the
   ordinary concept and a wrapped and an unwrapped spelling in one section are one
   link. A name `CleanName` refuses is not a tag: document ingest skips it, and
   record ingest skips it with a warning. Neither fails the document or record.
4. `kb merge` refuses every zero-byte input, whether or not a `-wal` sits beside
   it. When there is one, the message says it was left untouched. The refusal
   happens before anything opens the file, so the `-wal` is exactly as found.
   This reverses DR-0044 item 4 and takes the alternative DR-0044 rejected as
   "probably equivalent in practice".

**Rationale.**

For `--`, the inconsistency is the defect: DR-0041 promised "a name that begins
with a dash needs `--`", and it was untrue for five verbs. One central change is
five lines and makes it true, where a per-verb change would leave the rule
depending on which parser a verb happens to use.

For names, normalising whitespace is what makes the wrapped wikilink resolve to
the concept the author plainly meant. Refusing instead would have failed
document ingest on a hard-wrapped link, or required skipping it, both worse than
resolving it. Unlike the DOI in DR-0045, which is refused rather than rewritten
because rewriting hides that the wrong form was supplied, a run of whitespace
inside a name carries no meaning to hide. Control characters are the opposite
case: never part of a real name, and harmful when echoed, so they are refused.

For merge, the allowance protected nothing and caused the failure it was meant to
prevent: a zero-byte input answered with success, and a user's file changed. It
is removed on evidence, not on the same unverified belief in the other direction.

**Rejected alternatives.**

- *`--` for `source add` only.* Fixes the one case with no workaround but leaves
  two rules and the same inconsistency for paths.
- *Leave `--` as documented.* Nothing in real use needs it today, but DR-0041's
  text was untrue as written.
- *Refuse control characters in typed names and normalise only wikilink captures.*
  Typed input is refused rather than rewritten, but it is two rules in two
  places, and the wrapped-wikilink fix would have to be repeated at every
  extraction site.
- *Leave multi-line names as they are.* Hard-wrapped wikilinks keep minting
  duplicate concepts with a newline in the name.
- *Keep the WAL allowance.* Leaves merge exiting 0 on an empty input and removing
  its `-wal`.

**Consequences.**

A project or concept whose name contained a run of whitespace, a newline or a tab
now resolves to its collapsed form. The real database holds none (0 of 131
concepts and 8 projects have a control character or a run of spaces, checked
2026-09-25), so no stored row changes. `import` and `merge` use raw SQL and are
unchanged, so a database that already holds such a name still loads; looking one
up by its original spelling will not find it, since the argument is cleaned.

Verified against real data with the previous binary and this one: re-ingesting the
45 decision records of all three tiers into a fresh database gave identical
concept sets (75) and identical record-concept links (157 rows). Re-ingesting 212
real Markdown documents gave identical link counts (226) and differed in exactly
the two concepts above, now one line each. They are still junk names, examples of
wikilink syntax written in prose rather than in code, which DR-0037 does not
catch, and remain the corpus-improvement question's business.

A script that passed a name with an embedded control character now fails, with
exit 1 from the library check (the exit-code question in DR-0040 is not reopened
here). A zero-byte merge input with a `-wal` was accepted before and is refused
now.

Left as they are. `kb search -- -x` searches for the text `-- -x`, because search
takes free text and does not consume `--`. A `source add` title is not put
through `CleanName`, so a title may still contain a newline. A wikilink that
`CleanName` refuses is dropped silently by document ingest, where record ingest
warns. `kb document tag` finds an existing `[[name]]` with a pattern that
allows `\s*` only at the ends, so an already-wrapped link is not recognised as
present: on a file holding `[[deterministic<newline>output]]` it also linked the
next plain mention of the concept (tested 2026-09-25).

Status `proposed`: promotion is the author's call. Nothing was superseded: DR-0041
and DR-0042 are amended in effect and DR-0044 loses item 4, so the relations
(`kb record supersede --partial` for DR-0044) are for the author to set when this
is accepted.
