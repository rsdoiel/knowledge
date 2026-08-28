# Decision Records — index

Generated file. Do not hand-edit.

```
DR-0020  2026-08-28  accepted     decision     implementation   -     W5/W6 records portability: decisionRecord/decisionRecordRelation naming, project resolved by name, relation endpoints by same-import uuid cache
DR-0019  2026-08-28  accepted     decision     plan-review      -     A project-scoped JSON-L export excludes workspace-tier records; only an unscoped export carries them
DR-0018  2026-08-27  accepted     decision     implementation   -     A record's project is matched across databases by name, and a divergence is reported on the path that aborts as well as the one that succeeds
DR-0017  2026-08-27  accepted     decision     implementation   -     The record FTS shape is written twice and held together by a test, rather than extracted into a shared writer
DR-0016  2026-08-27  accepted     decision     implementation   -     A record with a broken project reference is skipped rather than silently retiered, and a name collision still orphans the losing side's records
DR-0015  2026-08-27  accepted     correction   implementation   -     kb index indexes the directory it is given, and a live corpus is asserted by property rather than by remembered count
DR-0014  2026-08-27  accepted     decision     plan-review      -     merge normalises throwaway copies of its inputs, and the workspace name travels from the original path rather than the temp directory
DR-0013  2026-08-27  accepted     decision     design           -     Decision records travel on every portability path, workspace tier included, and a diverged body is reported not silently resolved
DR-0012  2026-08-26  accepted     decision     request          -     A project description is correctable in place; a concept description is not clobbered by an empty one
DR-0011  2026-08-26  accepted     decision     design           -     A record's identity gains the workspace name, derived from the root rather than stored in frontmatter
DR-0010  2026-08-26  accepted     correction   implementation   -     Standard options come from a FlagSet and are answered before any database exists
DR-0009  2026-08-26  accepted     correction   release-review   -     The Makefile is hand-maintained despite its generated header, and the grounding query was truncating records not filtering them
DR-0008  2026-08-25  accepted     decision     implementation   -     kb index builds from files not the database, treats a malformed record as fatal, and search gains source_type
DR-0007  2026-08-25  accepted     decision     implementation   -     Record ids are not identities, writers re-render canonically, and a uuid guard blocks cross-workspace collisions
DR-0006  2026-08-25  accepted     decision     implementation   -     Ingest creates unknown projects, links `initiative` as a concept, and re-resolves references for skipped records
DR-0005  2026-08-25  accepted     decision     plan-review      -     The frontmatter struct declaration is the format specification, and the corpus is normalised to it
DR-0004  2026-08-25  accepted     correction   implementation   -     Record identity is indexed over `IFNULL(project_id, -1)`, because a NULL defeats a unique index
DR-0003  2026-08-08  accepted     decision     -                -     JSON-L export/import: the no-file-access alternative to `merge`
DR-0002  2026-07-27  accepted     decision     -                -     `kb --debug`: nil-safe JSONL trace of every knowledge-base call and TUI event
DR-0001  2026-07-27  accepted     decision     request          -     `cmd/kb`: a `<TOOL> <VERB> <PARAMETERS>` CLI, and a read-mostly TUI
```
