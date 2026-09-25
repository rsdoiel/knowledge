---
id: "0045"
title: "Refuse a malformed source and an unknown record trigger or kind at authoring time"
date: "2026-09-25"
status: proposed
kind: decision
trigger: request
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0040", "0042", "0043"]
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a0d951-df0e-7edb-b3f9-d5bcfbc10863"
origin_host: "wren"
---

**Context.**

DR-0042 refused blank names for projects, concepts and observations and left
two gaps on purpose: `kb source add ''` and a malformed `--published` or `--url`
still succeeded. RSDOIEL filed them as a low-priority item in `TODO.md`, and
on 2026-09-25 asked to work through it. Probing a scratch database confirmed
the gaps and found two more. `source add` accepted a blank or whitespace-only
title, `--published notadate`, `--published 2026-13-45`, `--url 'not a url'`
and `--doi zzz`, each with exit 0. `record new` accepted `--trigger bogus` and
`--kind bogus` and wrote the file; ingest only warns about the vocabulary later.

Two of these do real damage rather than storing a wart. A mistyped DOI is sent
by `kb source check-retractions` to Retraction Watch; a miss reads as "not
retracted", so a typo produces false reassurance. And a mistyped trigger
legitimises itself: DR-0043's filter rule accepts any value some record
carries, so once a record has been written with `live-tets`, `record list
--trigger live-tets` stops erroring and the typo is an established value.

The real database holds seven sources. All pass the checks below. They also
carry identifier types outside the documented list (`huggingface`, `file`).

**Decision.**

1. `AddSource` trims every field and refuses a blank title, the rule DR-0042
   applies to names. The check is in the library, not the CLI, so harvey and
   any script get it. An identifier or date padded with whitespace is trimmed
   before it is stored or used as the dedupe key.
2. `published_date`, when set, must be `YYYY`, `YYYY-MM` or `YYYY-MM-DD` and a
   real calendar date. Partial dates stay legal because a citation often has
   only a year. This is the form `record list --since` already accepts.
3. An identifier of type `url` must parse with a scheme and a host and contain
   no whitespace. `mailto:` and `file:///path` are refused. An identifier of
   type `doi` must be the bare form `10.<digits>[.<digits>]*/<suffix>`. The
   check is on shape only, because the module's own fixtures use `10.1/a`.
   Nothing is normalised: a pasted `doi:` or `https://doi.org/` form is refused
   with a message that says what to supply, so the dedupe key stays what the
   caller wrote.
4. Identifier types other than `doi` and `url` are not checked.
5. `record new` applies the rule `record list` uses for its filters to
   `--trigger` and `--kind`: a value outside the documented vocabulary is an
   error unless a record already in the database carries it. The rule is one
   function, `checkRecordVocabulary`, shared by both verbs. The error names
   what is known. Nothing is written when it fires.
6. The JSONL importer keeps its own `importSource` and is not held to items
   1-4, so an export from a database that already holds an odd value restores.

**Rationale.**

A check that lives where the data enters is the only one that holds for every
caller, and `AddSource` is that place. For the DOI, the failure is not stored
junk but a wrong answer later, from a tool that reports on the strength of the
stored value. Reusing DR-0043's rule for `record new` keeps one definition of
"a value we recognise", and it closes the self-legitimising case at the moment
the value would be written into a file. Checking `doi` and `url` only follows
the real data: the database already holds identifier types no list documents.

**Rejected alternatives.**

- *CLI-only checks.* Leaves harvey and every library caller unprotected;
  RSDOIEL chose the library for the blank title and the same reasoning covers
  the rest.
- *Strict `YYYY-MM-DD`.* Matches the documented format literally but makes a
  source with only a year impossible to add. RSDOIEL chose the three forms.
- *Rewriting a pasted `https://doi.org/...` to the bare DOI.* Friendlier, but
  it changes the dedupe key silently and hides that the caller supplied the
  wrong form. Refusing with a hint costs one retry.
- *A four-digit minimum on the registrant code.* True of real DOIs, but it
  would refuse the module's own fixtures for no protection the shape check
  lacks.
- *Warn on stderr and still write, for `record new`.* Matches ingest's
  warn-only behaviour, but a warned typo still lands in the file and then
  passes DR-0043's rule. RSDOIEL chose refusal.
- *Validating every identifier type.* The real database carries types no
  vocabulary lists, so the check would refuse existing practice.

**Consequences.**

A script that pipes bad values into `source add` or `record new` now fails
where it used to succeed. The errors exit 1, because they come from a library
check or from the record-list rule, which DR-0040 leaves at 1; whether a
rejected value should be 2 belongs to the exit-code item RSDOIEL skipped for
this session. It wants an upgrade note in the next release.

No existing data is affected: the seven real sources pass, and a scratch copy
run with the old and new binaries differed only in the refusals. Import is
unaffected by design.

Left as they are, and worth knowing. `--doi D --url bad` uses the DOI and
ignores the URL, as the manual says, so the ignored URL is not checked. A
document's `published_date`, taken from its frontmatter, is still free text.
`observation add --source-doi` writes a plain column, not a source row, and is
unchecked. `record new --project P` does not require P to exist in the database.
Ingest of a hand-written record with an unknown trigger still only warns. The
date check duplicates a few lines of `checkSinceDate` in `cmd/kb`; sharing them
would mean exporting a helper for two callers.

Status `proposed`: promotion is the author's call.
