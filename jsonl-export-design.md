# JSON-L export/import — Design

Step 4 of the original cross-machine-sync sequencing (`knowledge_db_merge_design.md`
in the Laboratory root), deferred when the merge tool (`MergeKnowledgeBases`,
`cmd/kb merge`) shipped 2026-07-27. `merge` requires direct file access to
both `knowledge.db` files (it `ATTACH`es them). JSON-L export/import is the
no-file-access alternative: a plain-text, line-oriented snapshot that can be
pasted, emailed, or committed to git and applied to a live, already-open
database with no second `.db` file involved.

## Confirmed decisions

**One self-describing JSON stream, not one file per table.** Each line is a
JSON object with a `"type"` discriminator (`project`, `concept`, `source`,
`observation`, `observation_concept`, `project_concept`,
`observation_source`). A single file is easier to move around (one `scp`,
one paste, one git-tracked path) than seven, and the `type` field is enough
for a reader — human or importer — to make sense of a line in isolation.

**Identity travels by `uuid`, not by autoincrement `id`.** Every table
already carries a `uuid` (backfilled by `Open`, `UNIQUE` indexed — see
`knowledge.go`'s `backfillUUIDs`/`idx_<table>_uuid`). Export writes each
row's `uuid`; parent references on child records (`observation`'s
`project_uuid`, the three join records' `*_uuid` pairs) use the parent's
`uuid`, never its local integer `id` — ids are meaningless across machines,
exactly the problem `uuid` was added to solve for `merge`.

**Export ordering is dependency order, but import doesn't rely on it.**
Export writes projects → concepts → sources → observations →
observation_concepts → project_concepts → observation_sources, so a human
skimming the file sees parents before children. Import buffers every line
into typed slices first (grouped by `type`), then applies them in that same
phase order regardless of the order lines appeared in the file — a
hand-edited or concatenated (`cat a.jsonl b.jsonl`) file shouldn't have to
get the interleaving right. Cost: the whole file is held in memory, but
`knowledge.db` here tops out at a few hundred observations, not a scale
where that matters.

**Import identity key differs by table, matching how each table already
resolves identity elsewhere in this package:**

- `project` / `concept`: keyed by **name** (the existing `UNIQUE(name)`
  constraint), via the existing `AddProject`/`AddConceptWithIdentifier`
  upsert helpers. This matches `merge.go`'s own model — `MergeKnowledgeBases`
  dedupes projects/concepts by name too (`INSERT OR IGNORE ... SELECT ...`,
  relying on the `UNIQUE(name)` constraint), correlating child rows by `uuid`
  only *after* that. Importing into a live db is the same shape: if
  "harvey" already exists locally, the local row (and its local `uuid`) is
  what every observation must attach to, not whatever `uuid` "harvey" had on
  the machine that exported it. A local `project_uuid`/`concept_uuid` map is
  built during import (`incoming uuid -> local id`) precisely so observation
  and join records can resolve against the *local* identity even when the
  local and remote `uuid` for the same name disagree.
- `source`: keyed the same way `AddSource` already dedupes — by
  `(identifier_type, identifier_value)` when both are set, otherwise always
  inserted as new (pre-existing behavior, not something this feature needs
  to change).
- `observation`: keyed by **`uuid`** via `INSERT ... ON CONFLICT(uuid) DO
  NOTHING`, using the existing unique index. Observations have no natural
  content key; `uuid` is authoritative, and re-importing the same export is
  then a no-op for observations already present.
- Join records (`observation_concept`, `project_concept`,
  `observation_source`): resolved by looking up local ids for both sides via
  the in-memory `uuid -> local id` maps built while importing their parent
  tables, then `INSERT OR IGNORE` on the existing composite primary key.
  Unresolvable references (a `uuid` the file never defined a parent for —
  e.g. a hand-trimmed file) are skipped, not fatal.

**No name-collision abort/`-force` flow, unlike `merge`.** `merge`'s
`-force` reconciles two independent files where either side could plausibly
be "right." Import only ever touches the live, already-open target
database — there's exactly one local row for a given name, and it always
wins by construction (the upsert helpers already used here update
description/status in place, never fork identity). This is simpler than
`merge` on purpose: JSON-L import is a one-way "bring in what I don't have
yet" operation, not a two-way reconciliation.

**Export scope: whole database by default, `-project NAME` to narrow.**
`ExportJSONL(kb, w, projectName string) error` — `projectName == ""` exports
everything. A named project exports: that project row; every concept linked
to it directly (`project_concepts`) *or* to one of its observations
(`observation_concepts`) — a concept only reachable through an observation
still needs to travel with it, or the receiving side would have a dangling
`concept_uuid` reference; every source cited by one of its observations;
all of the project's observations; and the three join tables filtered to
rows whose endpoints are all included above.

**`ImportSummary` return shape mirrors `MergeTableSummary`** (`knowledge_merge.go`)
for consistency: `[]ImportTableSummary{Table, Read, Imported, Skipped}` per
type, `Skipped` meaning "already present locally" (detected via 0
`RowsAffected` on the `ON CONFLICT ... DO NOTHING`/`INSERT OR IGNORE`, not a
separate existence query).

## Rejected alternatives

**Update-on-conflict for observations** (re-importing an edited export
overwrites the local row) — rejected in favor of insert-or-skip. Matches
`MergeKnowledgeBases`' existing "first write wins" semantics, and avoids a
foot-gun where importing an old backup file silently reverts a local edit.
If a real need for "push my edits" surfaces later, it's a different,
explicitly-named operation (`update_memory`-style, not a bulk import).

**Per-table files (`projects.jsonl`, `observations.jsonl`, ...)** — rejected;
one file is simpler to hand around, and the `type` field costs one field per
line.

**Streaming import (process each line as read, no buffering)** — rejected;
would require the file to already be in dependency order, which a
hand-edited or `cat`-concatenated file may not be. Buffer-then-phase is a few
hundred rows at most, not a real cost.

**CLI verb named `export`/`import` under a `-format jsonl` flag (for
future non-JSONL formats)** — rejected for now: no second format exists,
and `search`/`summary`/`format` already establishes the precedent of a
narrow, format-specific verb name here rather than a generic verb with a
format flag. `export`/`import` verbs hardcode JSON-L; a future format would
get its own verb (or a flag added then, when there's a real second case to
design against).

## CLI shape

New top-level verbs, following the existing `merge` precedent of a flat
`flag.FlagSet` (not a subcommand tree like `source`/`observation`):

```
kb export [-project NAME] [-out PATH]   # default: stdout
kb import [-in PATH]                    # default: stdin
```

Unlike `merge`, both operate on the ambient `--db` database (opened
normally by `mainRun`, no special-casing needed) — `export` reads it,
`import` writes into it.
