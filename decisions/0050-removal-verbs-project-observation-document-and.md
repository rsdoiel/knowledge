---
id: "0050"
title: "Removal verbs: project, observation, document and record delete, and unlink"
date: "2026-09-25"
status: accepted
kind: decision
trigger: request
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0038", "0039", "0047"]
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a0da68-f4bd-7acf-9b55-1d6957ef5418"
origin_host: "wren"
---

**Context.**

On 2026-09-25 RSDOIEL asked whether any delete, remove or cancel verb was unimplemented.
Running each plausible verb on a scratch database showed that only `concept delete`
(DR-0039), `source remove` and `source retract` remove or retire anything; `project
delete`, `observation delete`, `document delete`, `record delete`, and any inverse of
`link` and `source link` all exit 2 as unknown. The only such gap the TODO had recorded
was a stray empty project left by a failed ingest, "with no verb to remove the stray
row". Every route to fixing a mistake was therefore a raw `sqlite3` write, which the
workspace forbids because it skips the search index. RSDOIEL wants these in v0.0.14 and,
asked, chose the shapes below; the design note is `removal-verbs-design.md`.

**Decision.**

1. Every removal verb follows `concept delete` (DR-0039): it refuses while something
   depends on the target and says what and how many, `--force` where a force makes
   sense, `--dry-run` to preview, one transaction for the rows and then the search
   entry, `--` for a dash-leading name, a library function with a typed refusal, and exit
   1 for a refusal (workspace DR-0003 class "negative"). A target that does not exist is
   exit 1, "not found", never a silent success.
2. `kb project delete NAME [--force] [--dry-run]`. A project that owns any observation,
   record or document is always refused, with `--force` or not: a single command must
   never destroy work. A project attached only to concepts is refused unless `--force`,
   which removes those concept links and the project. It exists to remove a stray empty
   project; it never cascades.
3. `kb observation delete ID [--force] [--dry-run]`. An observation with concept links,
   source links, or supersession relations (either direction) is refused; `--force`
   removes those links and the observation.
4. `kb document delete ID [--force] [--dry-run]`. A document with any section whose
   summary is `reviewed` is refused, since a reviewed summary is human-gated data;
   `--force` deletes anyway. A document with no reviewed section is deleted with its
   sections and their concept links. The output says a re-ingest of the file recreates it.
5. `kb record delete RECORD_ID [--project P | --workspace] [--root DIR] [--dry-run]`,
   only when the record's file is already gone. It removes the stale row, its relations,
   its concept links and its search entry. While the file exists it is refused, and the
   message points at deleting the file first, or `record set-status ID cancelled`. `ingest`
   stays additive; this is the one deliberate way to drop a row whose file vanished, so
   a decision record is never removed from disk by `kb`, and accepted records remain
   history (DR-0038).
6. A new top-level verb `kb unlink` mirrors `link`: `unlink project PROJECT_NAME
   CONCEPT_NAME`, `unlink observation OBS_ID CONCEPT_NAME`, `unlink source OBS_ID
   SOURCE_ID`. Names match exactly, as for `concept delete`. Removing a link that does
   not exist is exit 1, not success.
7. Deletion is local to one database, with no tombstone, exactly as DR-0039 decided: `kb
   merge` and `kb import` into a non-empty database only add rows, so a database that still
   has the row brings it back. Each verb's page and output say so. `agents/knowledge.jsonl`
   is authoritative (workspace DR-0002) and the pre-commit hook re-exports it, so a delete
   reaches a database rebuilt from it.
8. Not added: `document review reject` (`knowledge` cannot un-draft a summary), and a
   `cancel` on a project or record (records have `set-status cancelled`, projects have
   `paused` and `concluded`).

**Rationale.**

A fix that needs raw SQL is a fix the workspace rules do not allow, so a missing verb is
not a convenience gap. The refusal shapes come from what each thing is. A project is a
container, so it is deleted only when empty; the stray-row case that motivated the item is
exactly an empty project. An observation is a leaf with links, so `--force` unlinking is
enough. A document's value is its reviewed summaries, so that is what gates it. A record is
a file, and the file is the truth; refusing while it exists is what stops the next ingest
from silently undoing the delete, and it keeps deletion of decision history out of `kb`.
`unlink` is a separate verb so creating and removing a link read symmetrically.

**Rejected alternatives.**

- *`project delete --force` cascades into content.* One command that can delete hundreds
  of observations and every decision record row; rejected for the reason in item 2.
- *`record delete` removes the file and the row.* Convenient, but it deletes a decision
  record from disk, accepted ones included.
- *No `record delete`.* Leaves a row whose file vanished stuck, which `ingest` only reports.
- *`remove` subverbs (`link remove project P C`, `source unlink`).* No new top-level verb,
  but creating (`link project`) and removing (`link remove project`) stop being symmetrical.
- *Tombstones so a delete propagates through merge.* Decided against in DR-0039 and not
  reopened here.
- *Delete a document unconditionally.* Loses a human-reviewed summary with no warning.

**Consequences.**

Six new commands (`project delete`, `observation delete`, `document delete`, `record
delete`, and `unlink` with three subverbs, counted as one new verb). None changes an existing
exit code. The registry-driven tests (every command with a bogus flag and a surplus
argument) reach them once their SYNOPSIS lines exist. `kb-unlink(1)` is a new page and
`unlink` joins the topics. The library gains typed refusals matched with `errors.Is(err,
ErrInUse)` and the usage types that carry the counts.

A delete a script relied on doing with `sqlite3` now has a supported form; `sqlite3` still
works and still skips the search index. A record row deleted here and a file restored later
comes back on the next ingest, as the file is the truth.

Left open. Whether `observation delete` should refuse an observation that is the target of
a supersession by another (it does, as a relation) or only warn. Whether `project delete
--force` should offer a cascading form later, with its own record. Whether `unlink`'s
`--dry-run` is worth having; not included, since a link removal is small and reversible by
`link`.

Status `proposed`: promotion is the author's call.
