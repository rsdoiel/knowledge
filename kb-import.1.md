%kb-import(1) user manual | version 0.0.5 e775390
% R. S. Doiel
% 2026-08-28

# NAME

kb-import — apply a JSON-L snapshot (from export) to the database

# SYNOPSIS

kb import [-in PATH]

# DESCRIPTION

import reads a JSON-L stream produced by export — from -in, or stdin when
-in is omitted — and applies it to the already-open --db database.
Projects and concepts are matched by name (an existing local row always
wins as-is; a genuinely new one keeps its original uuid, for future
cross-machine merge compatibility). Sources are matched by identifier when
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

# SEE ALSO

kb(1), kb-export(1), kb-merge(1)

