# Decision Records — index

Generated file. Do not hand-edit.

```
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
