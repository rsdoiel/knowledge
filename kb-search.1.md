%kb-search(1) user manual | version 0.0.13 1aa3e79
% R. S. Doiel
% 2026-09-24

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
  ANDed, "quoted phrases" match exactly, prefix* matches by prefix. A first
  word that starts with a dash is read as a mistyped option and refused
  (exit 2); give a dash-leading term after --, as in kb search -- -x.
  Later words are text as typed, dashes included. Finding nothing is exit 1

summary
: a formatted overview of every project and its most recent observations

format
: a fully-assembled Markdown view of one project (--project NAME) or every
  project (no --project) — concepts and observations included inline,
  unlike the bare rows kb-project(1)'s show/list return

# SEE ALSO

kb-project(1), kb-observation(1)

