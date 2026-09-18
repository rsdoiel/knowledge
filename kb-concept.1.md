%kb-concept(1) user manual | version 0.0.9 3e593d0
% R. S. Doiel
% 2026-09-17

# NAME

kb-concept — manage concepts

# SYNOPSIS

kb concept add NAME [DESCRIPTION] [--identifier-type T --identifier-value V]

kb concept list

kb concept rename OLD NEW

# DESCRIPTION

A concept is a named idea or term that can be linked to projects and
observations (see kb-link(1)). Names are unique.

add on an existing name updates that concept rather than creating a second
one, which is also how a concept's description is corrected -- there is no
separate set-description here, unlike kb-project(1). An omitted or
empty DESCRIPTION preserves the stored one rather than clearing it, so
running add just to assert a concept exists cannot lose text. The same holds
for --identifier-type and --identifier-value.

A concept may also represent a scholarly entity — a paper, person,
institution, or funder — by setting --identifier-type (e.g. doi, orcid,
ror, fundref) and --identifier-value (the normalized identifier).

rename
: rename a concept and reindex it for search. Refuses only if NEW already
  names another concept -- unlike kb-project(1)'s rename, a concept
  has no corpus of external files to desync, since every link to it
  (record_concepts, observation_concepts, project_concepts,
  document_section_concepts) is a foreign key, never a name matched from a
  file. See DR-0024 (knowledge/decisions/).

# CAVEATS

A description or name edited on two machines now reconciles: both
kb-merge(1) and kb-import(1) adopt whichever side's
updated_at is later (DR-0025, generalized to name by DR-0026), so a concept
renamed on one machine and left untouched on another arrives as one renamed
concept, not two, regardless of merge/import order.

# SEE ALSO

kb-link(1), kb-project(1)

