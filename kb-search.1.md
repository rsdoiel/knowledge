<<<<<<< HEAD
%kb-search(1) user manual | version 0.0.2 a21f85b
=======
%kb-search(1) user manual | version 0.0.3 193fa97
>>>>>>> 6f81c33478fb839db70903bb8800aa73c795ab0b
% R. S. Doiel
% 2026-08-08

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

