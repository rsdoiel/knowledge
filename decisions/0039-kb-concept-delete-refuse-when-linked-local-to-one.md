---
id: "0039"
title: "kb concept delete: refuse when linked, local to one database"
date: "2026-09-23"
status: proposed
kind: decision
trigger: request
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0037"]
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a0d0b6-b5dc-7648-a104-c473f9eac416"
origin_host: "wren"
---

**Context.**

`kb` had no way to remove a concept. The real database holds a junk concept
`...` (linked to two records), minted from a documentation example before
DR-0037 stopped wikilinks in code from minting concepts; other databases
ingested with v0.0.11 may hold more. Its removal was a manual `sqlite3` job, and
raw SQL is exactly what the workspace forbids for writes, since it skips the
search index. Deletion also touches the cross-machine sync: `agents/knowledge.jsonl`
is authoritative (workspace DR-0002) and each machine rebuilds its database from
it, but `kb merge` and `kb import` into a non-empty database only ever add rows.

**Decision.**

1. `kb concept delete NAME [--force] [--dry-run]`, backed by
   `ConceptUsage` and `DeleteConcept` in the library. NAME matches exactly,
   including case: unlike minting, which deliberately merges case variants, a
   destructive verb must not guess.
2. A concept still linked to a project, observation, record or document section
   is refused with the counts and nothing changes. `--force` unlinks it from all
   of them and deletes it; the linked things themselves are untouched. The rows
   go in one transaction, then the search entry.
3. `--dry-run` reports and changes nothing. `--` ends flag parsing, so a junk
   name that looks like a flag (`---`) can be deleted.
4. Deletion is local to one database, with no tombstone. The man page and the
   command's own output say that a database that still has the concept brings it
   back on merge or import, and that a file which still names it recreates it
   when that file is next ingested after it changes, or on a rebuild from files
   (checked: an unchanged file is skipped).

**Rationale.**

Refusing by default protects tagged data from a typo. A tombstone that
propagates through merge, export and import is correct in every path but adds a
table, a JSONL record type and merge rules, and would delay the release; the
authoritative-JSONL flow already carries a deletion in the next export as long as
every machine rebuilds before it exports. RSDOIEL chose local delete with a
documented caveat, and refuse-unless-`--force`, on 2026-09-23.

**Rejected alternatives.**

- Tombstones: correct everywhere, but its own design cycle. Revisit if merge
  resurrecting a deleted concept proves to bite in practice.
- Always unlink and delete: a mistyped name would silently strip tags.
- A case-insensitive lookup: a destructive verb must not guess.
- Deleting the real database's junk concept by hand: what this replaces.

**Consequences.**

A concept can be removed cleanly, search entry included. Deleting on one machine
does not delete on another, and that is stated where a user will read it. A
concept that files still name can come back; the note says which files to edit.
Status `proposed`: promotion is the author's call.
