%kb-import(1) user manual | version 0.0.29 1ae6b7d
% R. S. Doiel
% 2026-10-29

# NAME

kb-import — apply a JSON-L snapshot (from export) to the database

# SYNOPSIS

kb import [-in PATH]

# DESCRIPTION

import reads a JSON-L stream produced by export — from -in, or stdin when
-in is omitted — and applies it to the already-open --db database, or, without
-db, to the database of the workspace found by walking up from the current
directory. In a fresh clone that has only agents/knowledge.jsonl, run
"kb import -in agents/knowledge.jsonl" from anywhere inside the
workspace to build agents/knowledge.db there.
Projects and concepts are matched by uuid first (DR-0026): a match
reconciles name/description/status (and a concept's kind) by whichever
side's updated_at is later, the same last-writer-wins rule merge uses. A
concept line with no "kind", as an older export wrote, leaves a local kind
alone and is "other" when new; an unknown kind refuses the whole file
before anything is written (exit 65).

A document_source or record_source line (DR-0081) whose document, record or
source is not in this database is skipped and counted, not an error; an older
file with none of them imports as before.

Story events and story knowledge are not imported, because they are not exported: after an import,
run kb document ingest on the same paths (an unchanged file is skipped
but still rebuilds its events). A uuid miss falls back
to matching by name (DR-0003) -- an existing local row under that name
wins as-is; a genuinely new one keeps its original uuid, for future
cross-machine merge compatibility. Sources are matched by identifier when
one is present. Observations and links are matched by uuid, so re-running
import against the same file is a no-op the second time.

Decision records are matched by identity — workspace, project, scope and
record id — the same tuple AddRecord and merge use, not by uuid: two
machines' ingest of the same file mint different uuids for it, so a
uuid-keyed match would treat that as new every time and duplicate the
record. An existing local record wins as-is. A record's project is
resolved by name for the same reason. Record relations are matched by
their endpoints' uuids, resolved against the records just imported in this
same run — that stays safe even though records themselves aren't uuid-keyed,
because the cache is built fresh from this file's own uuids on the way in.

Unresolvable references (a uuid the file never defines a parent for) and
unrecognized record types are skipped, not fatal — only malformed JSON
aborts the import. The returned summary reports, per record type, how many
lines were read, newly imported, or skipped.

An observation whose kind is outside the current vocabulary (note, finding,
decision, question, hypothesis) is imported with its kind unchanged and
reported as a "warning:" line (a "Warnings" list under --json). It is never
rewritten to note, and the run still exits 0: a value written before the
vocabulary was enforced should stay a fixable row, not fail the import or
change silently. check-db therefore agrees with the dump for such a row.

# EXIT STATUS

The workspace convention, as described in kb(1).

0
: the stream was applied. Lines skipped, record types not recognised, and
  observations whose kind is outside the vocabulary (a warning) are still 0;
  the summary says what was read, imported and skipped

2
: a bad flag, or a surplus argument

65
: a line of malformed JSON, a row the schema rejects, or a concept kind outside
  the vocabulary (the whole file is refused before anything is written)

66
: -in does not exist

74
: -in could not be read to the end, for example it is a directory, or the read
  failed part way

77
: the operating system refused to let -in be read

# SEE ALSO

kb(1), kb-export(1), kb-merge(1)

