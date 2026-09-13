%kb-document(1) user manual | version 0.0.5 59c5f8f
% R. S. Doiel
% 2026-08-28

# NAME

kb-document — ingest narrative documents at graduated abstraction levels

# SYNOPSIS

kb document ingest PATH --project P [--title T] [--format F] [--dry-run]

# DESCRIPTION

Ingests a narrative or article (Markdown, Fountain, or plain text) into
documents/document_sections, segmented per format and tagged the same way
records are. See narrative-documents-design.md for the full model; this page
covers ingest only -- review/draft/promote and list/show are not built yet.

# SEE ALSO

kb-record(1), kb-ingest(1)

