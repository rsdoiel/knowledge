%kb-search(1) user manual | version 0.0.4 0fe99a8
% R. S. Doiel
% 2026-08-26

# NAME

kb-search — full-text search and formatted views

# SYNOPSIS

kb search TERM

kb summary

kb format [--project NAME]

# DESCRIPTION

search
: full-text search across observations, projects, and concepts using the
  FTS5 index. TERM uses standard FTS5 query syntax: multiple words are
  ANDed, "quoted phrases" match exactly, prefix* matches by prefix.

summary
: a formatted overview of every project and its most recent observations

format
: a fully-assembled Markdown view of one project (--project NAME) or every
  project (no --project) — concepts and observations included inline,
  unlike the bare rows kb-project(1)'s show/list return

# SEE ALSO

kb-project(1), kb-observation(1)

