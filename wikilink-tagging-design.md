# `kb ingest` — inline `[[wikilink]]` concept tagging — Design

**Status (2026-09-13):** Decisions confirmed. See
[wikilink-tagging-plan.md](wikilink-tagging-plan.md) for the phased plan.

**References:**
- `wikilink-tagging-feature-request.md` — the filed idea this design resolves.
- `narrative-documents-feature-request.md` — a second, independent consumer
  of the same `[[Name]] → concept` mechanism, not built by this design, but
  the reason the mechanism should live in `knowledge` rather than being
  ingest-only from the start (see decision 3).

## Motivation, and two corrections

Filed as `wikilink-tagging-feature-request.md`, inspired by
[Build a digitally sovereign second brain](https://www.raspberrypi.com/news/build-a-digitally-sovereign-second-brain/)
(Hinchliffe, *Raspberry Pi* magazine — the article actually demonstrates
Logseq + Syncthing, not Obsidian as the feature request's motivation section
says; same `[[double-bracket]]` convention either way, so the design intent
is unaffected). Two of the feature request's open questions turn out to have
concrete answers once checked against the current code, not just design
choices:

**"Does this ride on the observation that ingest already creates per
record?"** No — checked `cmd/kb/upsertAll` (`cmd/kb/ingest.go:232-291`)
directly: `kb ingest` never calls `AddObservation`. A record's only existing
path to a concept today is `linkInitiative` (`cmd/kb/ingest.go:343-352`),
which links the record's *project* to a concept named after the
`initiative` field — one concept per record, scoped to the project, not to
the record. There is no existing record-to-concept relationship to reuse.
This settles the feature request's schema question: a new join table is
required, not optional.

**"Does `[[Name]]` conflict with the unused `Tags` frontmatter field?"**
Checked `recordfile.go:76` and every reference to it: `Tags []string` is
parsed by `ParseRecordFile` and carried through `RecordFile`/`Record`
(`recordfile.go:219,368`), but nothing in `cmd/kb/ingest.go` ever reads it —
it has been dead weight since it was added. Rather than leave that question
open, this design resolves it: `Tags` and `[[wikilink]]` feed the same
mechanism (decision 3).

## Decisions

1. **New join table `record_concepts`**, added to `recordsSchema`
   (`records.go`), same shape as `observation_concepts`/`project_concepts`:

   ```sql
   CREATE TABLE IF NOT EXISTS record_concepts (
       record_id  INTEGER REFERENCES records(id)  ON DELETE CASCADE,
       concept_id INTEGER REFERENCES concepts(id) ON DELETE CASCADE,
       PRIMARY KEY (record_id, concept_id)
   );
   ```

   `CREATE TABLE IF NOT EXISTS` is idempotent on an existing database, same
   lazy-migration pattern the rest of the schema uses — no `ALTER`, no
   backfill needed since this is a brand-new relationship with nothing to
   migrate.

2. **New method `LinkRecordConcept(recordID, conceptID int64) error`** in
   `knowledge.go`, verbatim shape of `LinkObservationConcept`/
   `LinkProjectConcept` — `INSERT OR IGNORE`, duplicate links a silent no-op.

3. **Two sources feed the same concept-linking call, not one**:
   - `[[Name]]` tokens scanned from the record **body only** (not title, not
     other frontmatter), via a simple non-nested regex: `\[\[([^\[\]]+)\]\]`.
   - Each entry in the frontmatter **`Tags`** list, resolved exactly the same
     way. This gives the dead `Tags` field a purpose without new schema,
     rather than leaving "does this conflict with Tags" as an open question —
     they don't conflict, they're the same mechanism from two entry points
     (inline-while-writing vs. declared-up-front).

   For each name found (from either source), trimmed of surrounding
   whitespace: resolve via the new `ResolveConceptName` (decision 4), then
   `ing.kb.LinkRecordConcept(recordDBID, conceptID)`.

4. **Case-insensitive resolution, scoped to this path only — not a change
   to `AddConcept`/`kb concept add`.** Revised after review: a wikilink is
   embedded in ordinary prose, and English sentence position dictates
   capitalization independent of what the concept actually is —
   `[[Computers]] are ... I use a [[computer]] regularly` names one concept
   twice, not two concepts. Folding this is correct for text scanned out of
   prose. It is *not* correct to widen `kb concept add`'s own semantics the
   same way: a human typing `kb concept add Bug` at the CLI is a single
   deliberate act, not an artifact of sentence grammar, and forcing
   case-insensitive uniqueness onto `concepts.name` globally means an actual
   schema migration (SQLite can't widen a `UNIQUE` column's collation via
   `ALTER TABLE`; it requires a full table rebuild — new table, copy, drop,
   rename) for a problem this feature doesn't have.

   So: a new method,
   `(kb *KnowledgeBase) ResolveConceptName(name string) (int64, error)` —
   `SELECT id FROM concepts WHERE name = ? COLLATE NOCASE LIMIT 1`; on a hit,
   return the existing id (whatever casing it was originally stored with,
   however that concept was created — manually or via an earlier wikilink);
   on a miss, `AddConcept(name, "")` creates it verbatim, and that casing
   becomes canonical for every later mention regardless of case. Checked the
   live `agents/knowledge.db`: `SELECT LOWER(name)... HAVING COUNT(*) > 1`
   returns nothing today, so there's no pre-existing case-duplicate to worry
   about at the code level either — this is a clean, additive method with no
   migration.

   `AddConcept`/`AddConceptWithIdentifier` are untouched — `kb concept add`
   keeps its exact-match behavior, no risk to existing callers or tests.

5. **Escaping is out of scope**, per the feature request's original decision
   — Markdown that legitimately needs literal `[[`/`]]` is a known,
   unaddressed gap.

6. **New unexported `(ing *ingester) linkWikilinkTags(rf *knowledge.RecordFile,
   recordDBID int64)`**, called unconditionally immediately after
   `ing.linkInitiative(rf, projectID)` in `upsertAll` — same call site, same
   unconditional-per-run shape (runs whether the record was added, updated,
   or skipped as unchanged), guarded only by `ing.dryRun` and
   `recordDBID == 0` (mirrors why `linkInitiative` guards on `projectID == 0`).
   Because both `ResolveConceptName` and `LinkRecordConcept` are idempotent,
   re-ingesting an unchanged file is a safe no-op rather than a duplicate-link
   error — no new idempotency mechanism needed.

7. **Portability parity is required, not optional.** `record_concepts` must
   be added everywhere `observation_concepts`/`project_concepts` already
   are, or it becomes the one relationship in the schema that silently
   doesn't survive a merge or export/import round-trip:
   - `knowledge_merge.go`: a uuid-joined `INSERT OR IGNORE` block (mirroring
     the existing `observation_concepts`/`project_concepts` blocks at
     `knowledge_merge.go:416-443`), **and** an entry in the `allTables`
     summary list (`knowledge_merge.go:461-465`) — the comment there is
     explicit that a table missing from that list is a table whose loss is
     never reported (DR-0013).
   - `jsonl.go`: a `recordConceptRecord` JSON-L type, an
     `exportRecordConcepts` function, and an import branch — mirroring
     `observationConceptRecord`/`exportObservationConcepts` exactly.
8. **No `kb_fts` changes.** `record_concepts` is a pure join table; concepts
   are already indexed by `AddConceptWithIdentifier` itself. This feature
   touches none of the four-writer FTS mechanics.
9. **New read path: `kb record concepts ID`**, mirroring
   `kb observation sources ID` — lists the concepts linked to a record. A
   write-only feature with no way to see what it wrote is incomplete; this
   is the minimum visibility needed to verify the feature works and to debug
   a record's tags later.
10. **Documentation.** `kb-ingest.1.md`/`kb-record.1.md` are generated, not
    hand-maintained — checked the Makefile: each is produced by
    `./bin/kb TOPIC -help >kb-TOPIC.1.md` (the `kb-topics-help` target), and
    the actual source text is `IngestHelpText`/`RecordHelpText`, Go string
    constants in `cmd/kb/helptext.go` (lines 563 and 633). So this decision
    is: edit those two constants (a paragraph on `[[wikilink]]`/`Tags`
    resolution and the case-insensitivity behavior for `IngestHelpText`; the
    new `concepts` subverb's synopsis/description for `RecordHelpText`),
    then regenerate the `.1.md` files by running `kb-topics-help` — never
    hand-edit the `.1.md` files themselves, they'd be overwritten.

## Deferred, explicitly

- Scanning `kb observation add` bodies for `[[Name]]` — the feature request
  scoped this to `kb ingest` (records) only for a first pass; nothing here
  changes that.
- Widening `kb concept add`/`AddConcept` to case-insensitive uniqueness
  globally (decision 4 scopes normalization to wikilink/Tags resolution
  only) — a real schema migration, not needed to solve the problem this
  design was asked to solve.
- Escaping (decision 5) — known gap, not blocking.
- Everything in `narrative-documents-feature-request.md` — that consumes
  `ResolveConceptName` the same way but is a separate entity (`documents`,
  not `records`) and a separate design cycle.
