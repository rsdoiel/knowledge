---
id: "0035"
title: "Move cmd/kb workflow logic into the knowledge library as exported functions"
date: "2026-09-23"
status: accepted
kind: decision
trigger: design
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: []
tags: []
uuid: "01a0d021-190e-7ea8-b7e3-4641a777ea5b"
origin_host: "wren"
---

**Context.**

`cmd/kb` is `package main`, so the workflow logic implemented there —
document ingest, `document tag`/`fuzzy-tag`/`frontmatter`, `concept suggest`
with clustering, record ingest — is callable only by the `kb` binary. Harvey
needs it in-process for a learning mode (see
`harvey/knowledge-learning-mode-design.md`). RSDOIEL chose, 2026-09-23, to
move the logic into the library rather than shell out to `kb` or copy it.
Designed in `library-lift-design.md`.

**Decision.**

1. Move the workflow logic into the root `knowledge` package as exported
   functions returning result structs. `cmd/kb` keeps flag parsing and
   rendering only, and calls the library.
2. Text-transforming verbs are lifted as pure text-to-text functions; the
   caller owns the file write.
3. Pure move, behavior-preserving, one commit per lift item; existing
   `cmd/kb` tests are the characterization net.
4. Order L1 document ingest, L2 tag/fuzzy-tag, L3 concept suggest, L4
   frontmatter, L5 record ingest (last, optional). L1–L4 ship as v0.0.12.
5. Frontmatter git provenance sits behind an injected `Provenance`
   interface with a git-backed default.

**Rationale.**

One copy of each algorithm. v0.0.11's fuzzy features each needed a
hand-computed correction to their own design; a second copy would drift from
those fixes. Returning text instead of writing files keeps a consumer's own
permission model in charge of its writes.

**Rejected alternatives.**

- Copy the code into harvey: two diverging implementations of algorithms
  that already needed correcting.
- Harvey shells out to `kb --json`: cheapest, but requires the binary on
  every install, and gives an interactive loop no typed results.

**Consequences.**

New exported surface to document and maintain; `--debug` traces become
coarser for lifted verbs; harvey's items 3 and 4 are gated on v0.0.12.
Status `proposed`: promotion is the author's call.
