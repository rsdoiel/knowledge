%kb-import(1) user manual | version 0.0.4 5ee81bb
% R. S. Doiel
% 2026-08-26

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

Unresolvable references (a uuid the file never defines a parent for) and
unrecognized record types are skipped, not fatal — only malformed JSON
aborts the import. The returned summary reports, per record type, how many
lines were read, newly imported, or skipped.

# SEE ALSO

kb(1), kb-export(1), kb-merge(1)

