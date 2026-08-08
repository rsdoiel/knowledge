# JSON-L export/import — Implementation Plan

See [jsonl-export-design.md](jsonl-export-design.md) for the full design and
confirmed decisions. TDD throughout, per project convention: `_test.go`
first, confirm red, then implement. Commit after each work item.

## W1 — Record types + `ExportJSONL`

**File:** `jsonl.go`, `jsonl_test.go` (package `knowledge`)

- Seven record structs (`projectRecord`, `conceptRecord`, `sourceRecord`,
  `observationRecord`, `observationConceptRecord`, `projectConceptRecord`,
  `observationSourceRecord`), each with a `Type string \`json:"type"\`` field
  set to a constant on construction, plus the fields listed in the design
  doc (uuid-keyed references, no local ids).
- `ExportJSONL(kb *KnowledgeBase, w io.Writer, projectName string) error`:
  whole-db when `projectName == ""`; scoped per the design doc's reachability
  rule otherwise. One `json.Marshal` + newline per record, dependency order.
- Tests: whole-db export against a small fixture produces valid, ordered
  JSONL with expected line counts; project-scoped export includes only
  reachable rows (including a concept reachable only via an observation, and
  excluding an unrelated project's data); empty database exports zero
  lines without error; unknown project name returns an error.

## W2 — `ImportJSONL`

**File:** `jsonl.go` (continued), `jsonl_test.go` (continued)

- `type ImportTableSummary struct { Table string; Read, Imported, Skipped int }`
- `ImportJSONL(kb *KnowledgeBase, r io.Reader) ([]ImportTableSummary, error)`:
  buffer all lines by `type` (unknown `type` values are skipped and counted,
  not fatal — forward-compat with a future record kind), then apply in
  phase order (projects/concepts/sources → observations → the three join
  types), building `uuid -> local id` maps as each phase completes.
- Tests: round-trip (`Export` one kb, `Import` into a fresh empty kb,
  contents match — projects, observations, concept links, source links);
  re-import of the same file is a no-op (`Skipped` counts equal `Read`,
  `Imported` all zero, row counts unchanged); importing into a kb that
  already has a same-named project merges observations under the local
  project row (verifies the uuid-mismatch case the design doc calls out);
  a join record whose `uuid` reference is missing from the file is skipped,
  not fatal; malformed JSON on one line errors out with the line number.

## W3 — CLI verbs

**File:** `cmd/kb/jsonl.go`, `cmd/kb/jsonl_test.go`

- `verbs["export"]`, `verbs["import"]`, both operating on the ambient `kb`
  (no `merge`-style special case in `main.go`).
- `export`: `-project NAME` (optional), `-out PATH` (optional, default
  stdout). `--json` mode wraps the line count in a `{"lines_written": N}`
  envelope instead of interleaving JSONL with a JSON summary object.
- `import`: `-in PATH` (optional, default stdin). `--json` mode prints
  `[]ImportTableSummary` as JSON; text mode prints the same table-format
  summary style as `merge`'s output.
- `logKBCall`/`dl.Log` wiring matching every other verb.
- Add both to `jsonaudit_test.go`'s case table (text + `--json`, using the
  existing fixture; `import` case reads from a small embedded JSONL fixture
  via `-in`, not stdin, to keep the test hermetic).

## W4 — Docs, decisions, version

- `helptext.go`: `ExportHelpText`, `ImportHelpText` consts (man-page style,
  matching `MergeHelpText`); `HelpText`'s VERBS section gains an entry.
- `help_dispatch.go`: route `"export"`/`"import"` topics.
- `Makefile`: add `export import` to `KB_TOPICS` (hand-edited, **not** via
  `cmt codemeta.json Makefile` — that target is hand-customized and `cmt`
  would blow away `KB_TOPICS`/`kb-topics-help`, see the 2026-07-27
  Makefile-regen gotcha in DECISIONS.md).
- Run `make kb-topics-help` (or the equivalent manual `./bin/kb export
  -help > kb-export.1.md` / same for `import`) to generate the checked-in
  `.1.md` sources, then `make man` if `pandoc` is available locally.
- `user_manual.md`: a section describing `export`/`import`, mirroring the
  existing `merge` section.
- `codemeta.json`: bump `version` (0.0.2 → 0.0.3) and `releaseNotes`; run
  `cmt codemeta.json version.go CITATION.cff about.md README.md`.
- `DECISIONS.md`: entry after everything is verified, following the
  existing format (context/decision/rejected/consequences), including any
  real bugs found along the way.
- `agents/knowledge.db` (Laboratory root): one `decision` observation on
  the `knowledge` project summarizing the shipped feature, linking the
  `cross-machine-sync` concept — closes out the step-4 deferred item.

## Non-goals for this pass

- A `-format` flag or any second export format.
- Streaming import (buffer-then-phase only, per the design doc).
- Compression/encryption of the JSONL file — it's plain text, same trust
  model as the `.db` file itself.
- Wiring `export`/`import` into harvey's `/kb` commands — this plan covers
  the `knowledge` module and its own `kb` CLI only.
