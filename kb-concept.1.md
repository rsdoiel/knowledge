%kb-concept(1) user manual | version 0.0.10 1e753e5
% R. S. Doiel
% 2026-09-18

# NAME

kb-concept — manage concepts

# SYNOPSIS

kb concept add NAME [DESCRIPTION] [--identifier-type T --identifier-value V]

kb concept list

kb concept rename OLD NEW

kb concept suggest [--project NAME] [--limit N]

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

suggest
: read-only: scans every record body and document section body (scoped to
  one project with --project) and prints candidate new concepts, ranked by
  corpus-wide distinctiveness -- a term mentioned several times but
  confined to relatively few items, rather than spread evenly across
  nearly all of them (not distinctive) or mentioned only once anywhere
  (too weak a signal alone). Code spans and fenced code blocks are
  excluded, a name already naming an existing concept is never suggested
  again, and a bare record reference (dr-0013, adr-0004) is filtered
  outright rather than scored. --limit caps the number printed (default
  20). Never writes anything -- a suggestion becomes a real concept only
  when a human runs concept add. See DR-0028 (knowledge/decisions/).

# CAVEATS

A description or name edited on two machines now reconciles: both
kb-merge(1) and kb-import(1) adopt whichever side's
updated_at is later (DR-0025, generalized to name by DR-0026), so a concept
renamed on one machine and left untouched on another arrives as one renamed
concept, not two, regardless of merge/import order.

# SEE ALSO

kb-link(1), kb-project(1)

