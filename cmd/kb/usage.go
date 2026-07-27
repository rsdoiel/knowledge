package main

import "io"

const usageText = `kb -- a command-line and interactive interface for a knowledge base
(projects, observations, concepts, sources).

Usage:
  kb [--db PATH] [--json] VERB [args...]
  kb                          launch the interactive browser (TUI)
  kb help | -h | --help       show this help

Global flags:
  --db PATH   path to knowledge.db (default: ./agents/knowledge.db)
  --json      machine-readable JSON output instead of text

Verbs:
  project add NAME [DESCRIPTION]
  project list
  project show NAME
  project concepts NAME

  observation add --project NAME KIND BODY [--source-doi DOI]
  observation list --project NAME
  observation show ID
  observation sources ID

  concept add NAME [DESCRIPTION] [--identifier-type T --identifier-value V]
  concept list

  link project PROJECT_NAME CONCEPT_NAME
  link observation OBS_ID CONCEPT_NAME

  source add TITLE [--doi D] [--url U] [--authors A] [--published DATE]
             [--publisher P] [--rights R] [--version V]
  source list
  source show ID
  source remove ID
  source retract ID NOTE
  source link OBS_ID SOURCE_ID [--relationship R]
  source check-retractions

  search TERM
  summary
  format --project NAME

  merge -a PATH -b PATH -out PATH [-force]
`

func printUsage(out io.Writer) {
	io.WriteString(out, usageText)
}
