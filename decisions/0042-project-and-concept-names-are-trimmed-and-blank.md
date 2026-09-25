---
id: "0042"
title: "Project and concept names are trimmed and blank ones refused in the library; import and merge stay raw"
date: "2026-09-24"
status: accepted
kind: correction
trigger: live-test
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0024", "0026", "0040"]
initiative: ""
session: ""
decisions: []
tags: [library, names, validation, rename, v0.0.13]
uuid: "01a0d5ac-46e2-7d3b-8928-19b0e1635221"
origin_host: "wren"
---

**Context.**

Nothing stopped an empty name. On a scratch copy of the real database, `kb project
add ''` and `kb project add '  '` each created a project (ids 15 and 16, named
`""` and two spaces), `kb concept add ''` created an empty concept, and `kb
observation add --project p note ''` stored an empty body. All of them then
appeared in every list, and none could be selected by typing its name. A padded
name was a different project from the trimmed one: `' harvey '` sat beside
`harvey`. Only `record new` refused an empty `--title`, so the guard existed in
one verb.

Ingest already trims wikilink and tag text before it resolves a concept, so the
library was inconsistent with its own most-used caller.

Checking the change on a real scratch corpus found a second, worse problem in the
CLI. `kb project rename` on a project that owns records rewrites every record's
`project:` frontmatter *before* it calls the library (DR-0026). On the v0.0.12
binary, `kb project rename demo ''` wrote `project: ""` into the record, renamed
the row to an empty string, and left the next ingest failing on `UNIQUE constraint
failed: records.uuid`, the deadlock DR-0026 exists to prevent.

**Decision.**

1. `CleanName(kind, name)` is exported. It trims surrounding whitespace and refuses
   a name that is empty afterwards. It is applied in `AddProject`,
   `AddProjectWithStatus`, `AddConceptWithIdentifier` (so `AddConcept`),
   `ResolveConceptName`, `RenameProject`, `RenameProjectRow` and `RenameConcept`.
   Interior whitespace, including a newline, is left alone.
2. `AddObservation` and `AddObservationWithSource` refuse a blank body but store a
   real one exactly as given, surrounding whitespace included.
3. The guard is in the library, not the CLI, so ingest, Harvey and a script get it
   as well.
4. `import` and `merge` use raw SQL and are unchanged, so a database that already
   holds such a row still loads. The legacy experiments-to-projects migration
   calls the unexported `addProject`, so an odd old name cannot make `Open` fail.
5. The CLI cleans a name before echoing it, and `project rename` cleans NEW
   **before it stages any file**, so the files and the row agree on the name.
   `--dry-run` refuses a blank name too.
6. `concept add` stays exact-match (`Chunking` and `CHUNKING` are two concepts);
   `ResolveConceptName` stays case-insensitive. Only whitespace changed.

**Rationale.**

A blank name is never meaningful, and trimming matches what ingest already does,
so a padded name reaches the row it plainly means instead of forking a duplicate.
Putting the guard under every caller is what makes it hold; a CLI-only check would
have left Harvey and ingest able to mint the same rows.

Leaving `import` and `merge` raw is deliberate: refusing there would make an
existing database with one bad row unloadable, turning a cosmetic defect into an
outage. The right place to clean such a row is a `kb` verb run on purpose.

Step 5 is the part a unit test on the library would not have found. Trimming in
the library alone would have put a padded NEW into the rewritten files while the
row held the trimmed name: the same desync DR-0026 guards against, created by this
fix. (In v0.0.12 a padded rename was at least consistent, both padded.)

**Rejected alternatives.**

- Refuse a padded name instead of trimming it. Harsher, and it would disagree with
  ingest, which trims.
- Validate in the CLI only. Leaves the library API, ingest and Harvey unguarded.
- Validate in `import` and `merge` as well. Makes existing databases unloadable.
- Trim observation bodies. A body's leading whitespace and trailing newline are its
  own formatting.
- Refuse control characters and interior newlines. Not decided here; see below.

**Consequences.**

Library callers now get an error for a blank name or body, and a padded name
resolves to the existing row. Harvey is pinned to a v0.0.13 pseudo-version until
the tag exists, so it meets this only when it bumps. No existing data is affected:
the real database has no blank or padded name in `projects`, `concepts` or
`observations`, and no blank source title (checked 2026-09-24).

Two gaps, deliberately left, for review. A multi-line name is still accepted.
`kb source add ''` (and a malformed `--published` or `--url`) still succeeds; it is
filed as a low-priority item in `TODO.md`, not fixed, so the rule "a blank name is
refused" is currently true of projects, concepts and observations only.

Status `proposed`: promotion is the author's call.
