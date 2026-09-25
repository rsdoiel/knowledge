# Removal verbs: design note

Decisions: DR-0050 (`proposed`). Plan: `removal-verbs-plan.md`. Filed as a TODO item on
2026-09-25 after RSDOIEL asked whether a delete, remove or cancel verb was unimplemented.

## Where things stand

Running each plausible verb on a scratch database showed three commands that remove or retire
something today: `concept delete` (DR-0039), `source remove` and `source retract`. `project
delete`, `observation delete`, `document delete`, `record delete`, `unlink` and `document review
reject` all exit 2, unknown. The TODO had noted only one of them, in passing: a failed ingest
left a stray empty project "with no verb to remove the stray row".

## What depends on what

From the schema's foreign keys:

| Entity | Depends on it (rows to remove or refuse for) | Search entry (`kb_fts` source_type, source_id) |
|--------|----------------------------------------------|------------------------------------------------|
| project | `observations`, `records`, `documents` (owned); `project_concepts` (links) | `project`, project id |
| observation | `observation_concepts`, `observation_sources`, `observation_relations` (from and to) | `observation`, observation id |
| document | `document_sections`, which own `document_section_concepts` | `document_summary`, each section id (only promoted summaries are indexed) |
| record | `record_concepts`, `record_relations` (from and to) | `record`, record id |

## Constraints

1. **Local deletion.** `kb merge` and `kb import` into a non-empty database only add rows, so a
   database that still has the row brings it back (DR-0039). No tombstones. Every verb's page says
   so. `agents/knowledge.jsonl` is authoritative (workspace DR-0002); the hook re-exports it, so a
   delete reaches a database rebuilt from it.
2. **A record's file is the source of truth.** `ingest` is additive: it never deletes a row whose
   file vanished, only reports it. A row deleted while its file exists comes back the next time the
   file changes and is ingested. Accepted records are history (DR-0038); `superseded`, `rejected`
   and `cancelled` are how they are retired.
3. **A reviewed summary is human-gated data.** Deleting a document loses it.
4. **The library does no file I/O** (the rule from the library lift, DR-0035). A library function
   cannot check that a record's file is gone; the CLI does, and only then calls the library.

## Shapes

`project delete NAME [--force] [--dry-run]`: refused if the project owns any observation, record or
document, with or without `--force`. Attached only to concepts: refused unless `--force`, which
removes the concept links and the project. Never cascades.

`observation delete ID [--force] [--dry-run]`: refused while it has concept links, source links or
relations in either direction; `--force` removes them and the observation.

`document delete ID [--force] [--dry-run]`: refused while any section's summary is `reviewed`;
`--force` deletes anyway. Otherwise removes the document, its sections and their concept links, and
any search entries.

`record delete RECORD_ID [--project P | --workspace] [--root DIR] [--dry-run]`: only when the file is
gone. The CLI stats `ROOT/rec.Path`: present is a refusal (exit 1) pointing at deleting the file or
`set-status cancelled`; absent calls `DeleteRecord`, which removes the row, relations, concept links
and search entry in one transaction.

`unlink project PROJECT_NAME CONCEPT_NAME`, `unlink observation OBS_ID CONCEPT_NAME`, `unlink source
OBS_ID SOURCE_ID`: names exact; a missing entity or a missing link is exit 1 "not found", never a
silent success (the class of bug found in `source retract`).

## Library

A typed refusal `InUseError{Entity, Name, Counts}` that matches `ErrInUse` under `errors.Is`, so the
CLI's classifier already exits 1 for it. `ConceptInUseError` stays as it is. New functions, each
documented with a `/** */` block: `ProjectUsage` and `DeleteProject`; `ObservationUsage` and
`DeleteObservation`; `DocumentUsage` and `DeleteDocument`; `RecordUsage` and `DeleteRecord`;
`UnlinkProjectConcept`, `UnlinkObservationConcept`, `UnlinkObservationSource`. A missing target is
`ErrNotFound`. Rows go in one transaction, then the search entry, as `DeleteConcept` does.

## Exit codes (workspace DR-0003)

Refusal 1; target or link not found 1; usage (a bad flag, a malformed id) 2; a record file that cannot
be examined for a reason other than absence is classified from the error (77, 74).

## Not doing

`document review reject` (`knowledge` cannot un-draft a summary), `cancel` verbs (`record set-status
cancelled`, project `paused` and `concluded` already exist), tombstones, cascading project delete.
