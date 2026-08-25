---
id: "0003"
title: "JSON-L export/import: the no-file-access alternative to `merge`"
date: "2026-08-08"
status: accepted
kind: decision
trigger: ""
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: []
tags: []
uuid: "af0a7f4a-cf54-44ce-962a-8fd9198f5d8f"
origin_host: "MACMINI-RD.local"
---

**Context.** Step 4 of the original cross-machine-sync sequencing
(`knowledge_db_merge_design.md` in the Laboratory root), deferred when the
merge tool shipped 2026-07-27 (see that date's entries below). `merge`
requires `ATTACH`-ing two live `.db` files; JSON-L export/import instead
produces a plain-text snapshot that can move over any channel that only
carries text — paste, email, a git commit. Full audit and confirmed
decisions in `jsonl-export-design.md`; phased TDD plan in
`jsonl-export-plan.md`.

**Decision.** `ExportJSONL(kb, w, projectName string) error` /
`ImportJSONL(kb, r io.Reader) ([]ImportTableSummary, error)` in the
`knowledge` package (`jsonl.go`), plus `kb export [-project NAME] [-out
PATH]` / `kb import [-in PATH]` verbs (`cmd/kb/jsonl.go`). One
self-describing JSON stream — each line a `"type"`-tagged record
(`project`, `concept`, `source`, `observation`, and the three join types),
identity carried by `uuid` (already `UNIQUE`-indexed on every table since
the 2026-07-26 UUID migration), never by the local autoincrement `id`.
Export writes parents before children; import buffers the whole stream by
type first and applies it in that same phase order regardless of the
file's actual line order, so a hand-edited or `cat`-concatenated file
doesn't need to get the interleaving right.

Import's identity rule differs by table, matching how each already
resolves identity elsewhere in this package: projects/concepts are
name-keyed (an existing local row wins as-is, matching `merge`'s own
"first write wins" model — a genuinely new row keeps its origin `uuid`, so
a later real `merge` between two machines that both imported the same
export can still recognize it as the same entity); sources are
identifier-keyed, matching `AddSource`; observations and joins are
uuid-keyed, since observations have no natural content key. Unresolvable
join references and unrecognized `"type"` values are skipped and counted,
not fatal — only malformed JSON aborts the import (with the offending
line number in the error).

**Two real bugs found by the TDD round-trip/re-import tests, neither
anticipated by the design doc:**
- **Sources with no identifier broke idempotent re-import**:
  `importSource` only deduped by `(identifier_type, identifier_value)`
  when both were set (matching `AddSource`'s existing "always insert new"
  behavior for identifier-less sources) — fine for a single call, but
  since `importSource` always carries the source's own `uuid` through,
  re-importing the same file a second time hit the `sources.uuid` UNIQUE
  index. Fixed by also checking `uuid` as a fallback before inserting.
- **Partial/incremental imports silently dropped observations**: a
  JSON-L file containing only `observation`/join records (project already
  synced by an earlier import, so this file doesn't re-include it) left
  `projectLocalID` empty for that project, since the map was only
  populated from `project` records actually present in *this* file — every
  observation referencing it was silently counted as `Skipped`
  (unresolvable), losing real data with no error surfaced. Fixed with
  `resolveLocalID`: check the in-memory map first, fall through to a
  direct `SELECT id FROM <table> WHERE uuid = ?` on a miss (correct because
  a row found this way already exists locally under that exact uuid — no
  identity ambiguity, unlike the name-collision case the map exists to
  handle), caching the result for reuse by later records in the same
  import.

Also manually verified end-to-end with the real `bin/kb` binary against
real data (not just the unit tests): whole-db and `-project`-scoped
export; import into a fresh db, including that FTS5 search actually finds
the imported rows (confirms the `kb_fts` maintenance added to the import
path, mirrored from `AddProject`/`AddConceptWithIdentifier`/
`AddObservationWithSource`, works — an easy thing to get right in unit
tests and still silently miss in the real index); a full re-import
reporting 100% `Skipped`; the intentionally-adversarial case — two
independently-created databases both with a project named `"harvey"`
under different `uuid`s — importing one's export into the other correctly
attached the observation to the *local* `"harvey"` row without touching
its description or creating a duplicate project; stdin as the default
`import` source; and both error paths (unknown `-project`, malformed
JSON).

**Rejected.** Update-on-conflict for observations (re-importing an edited
export overwrites the local row) — would let importing an old backup
silently revert a local edit; matches `MergeKnowledgeBases`'s existing
first-write-wins semantics instead. Per-table files
(`projects.jsonl`, `observations.jsonl`, ...) — one file is simpler to
hand around than several, and the `"type"` field costs one field per line.
Streaming import (apply each line as read, no buffering) — would require
the file to already be in dependency order. A `-format` flag for future
non-JSONL formats — no second format exists yet; `search`/`summary`/
`format` already establishes the precedent of a narrow, format-specific
verb name here.

**Consequences.** `go build`/`go vet`/`gofmt`/`go test` clean throughout
(TDD-first at every phase, red confirmed before each fix — both bugs above
were caught by tests written *before* the fix, not discovered by manual
poking afterward). `codemeta.json` bumped 0.0.2 → 0.0.3;
`version.go`/`CITATION.cff`/`about.md`/`README.md` regenerated via `cmt`.
`Makefile`'s `KB_TOPICS` hand-edited to add `export import` (not via `cmt
codemeta.json Makefile` — that target is hand-customized, see the
2026-07-27 Makefile-regen gotcha below). `kb-export.1.md`/`kb-import.1.md`
generated via `kb-topics-help` and `make man` (pandoc available on this
machine). `user_manual.md` gained a table row and a design-doc link. This
closes out step 4 of the original cross-machine-sync sequencing —
`knowledge_db_merge_design.md`'s deferred item — leaving no open items
from that plan.

---

