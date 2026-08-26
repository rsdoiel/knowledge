%kb-merge(1) user manual | version 0.0.4 b727417
% R. S. Doiel
% 2026-08-26

# NAME

kb-merge — reconcile two knowledge.db files that drifted independently

# SYNOPSIS

kb merge -a PATH -b PATH -out PATH [-force]

# DESCRIPTION

merge reads two knowledge.db files (e.g. from two machines that have
drifted independently) read-only and writes their deduped union to a
fresh -out file, which must not already exist. It never modifies -a or
-b; placing the merged file into position is left to you.

Unlike every other verb, merge operates entirely on the explicit -a/-b/-out
paths — it ignores --db and never opens (or creates) the ambient
./agents/knowledge.db.

If a project or concept with the same name exists in both files under
different internal identities (a collision — typically from before a
database's identifiers were established), merge aborts and lists them
unless -force is given, in which case b's identity is reconciled to a's so
both sides' observations and links survive under one merged entity.

# SEE ALSO

kb(1)

