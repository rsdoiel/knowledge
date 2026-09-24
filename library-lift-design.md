# Lifting `cmd/kb` Logic into the `knowledge` Library — Design

**Status (2026-09-23):** Decisions drafted with recommendations, awaiting
RSDOIEL's review. Nothing implemented. See `library-lift-plan.md` for the
phased build and `decisions/0035-*.md` (status `proposed`) for the record.

**References:**
- `../harvey/knowledge-learning-mode-design.md` — the consumer this lift
  exists for. That document's decision 1 (Harvey consumes the library, it
  does not shell out to `kb`) is what makes this lift a prerequisite.
- `module-extraction-design.md` / `-plan.md` — the reverse move (harvey →
  this module), and the "pure move, one commit per item" ground rule this
  design reuses.
- `DR-0032`, `DR-0033`, `DR-0034` — the three v0.0.11 features whose logic
  currently lives in `package main` and is unreachable from any importer.

## Motivation

`cmd/kb` is `package main`. Go cannot import it, so everything implemented
there is available to exactly one caller: the `kb` binary. Sizing the
current state (line counts from `wc -l`, 2026-09-23):

| Capability | Where its logic lives | Lines | Importable today? |
|---|---|---|---|
| Document ingest (new + re-ingest, tagging, density) | `cmd/kb/document.go` `ingestNewDocument`/`reingestChangedDocument` | 703 (file) | No |
| Record ingest | `cmd/kb/ingest.go` `ingester` | 629 | No |
| `document tag` | `cmd/kb/documenttag.go` `tagDocumentText`, `eligibleConcepts` | 377 | No |
| `document fuzzy-tag` | `cmd/kb/documentfuzzytag.go` | 362 | No |
| `document frontmatter` | `cmd/kb/documentfrontmatter.go` | 907 | No |
| `concept suggest` + fuzzy clustering | `cmd/kb/concept.go`, `conceptcluster.go` | 351 + 197 | No |
| Review queue, draft, promote, recall, fuzzy *matching* | root package (`documents.go`, `retrieval.go`) | — | **Yes** |

The library already owns the storage and review primitives; the *workflows*
that compose them are what's stranded. A second consumer (Harvey) would
otherwise either shell out to `kb` and parse `--json`, or copy the logic.
RSDOIEL chose the third path — lift it — on 2026-09-23.

**The lift is smaller than the line counts suggest.** Every lift candidate
imports only `knowledge`, `yaml.v3`, and stdlib (checked by reading each
file's import block). What ties them to `package main` is a thin layer:
`flag` parsing, `io.Writer` rendering, `printJSON`, and the `report*`
functions. The pure logic underneath — `tagDocumentText`, `insertFootnote`,
`proposeTitle`/`proposeAuthor`/…, `scoreCandidateTerms`,
`clusterCandidateTerms`, the `ingester` — is already separable.

## Decisions

1. **Library functions take data and return result structs. They never
   touch `flag`, `io.Writer`, `os.Exit`, or JSON.** `cmd/kb` keeps flag
   parsing and all rendering; each `cmdX` shrinks to *parse flags → call
   library → render result*. The result structs are what `--json` already
   serializes, so the JSON contract is unchanged by construction.

2. **Text-transforming verbs are lifted as pure text → text functions;
   the caller owns the file write.** `TagDocumentText(text, names)` returns
   the new text and what it inserted; it does not open a file. This is the
   decision with the most downstream weight: Harvey has its own
   permission-checked write path (`safe_mode`, `permissions` in
   `agents/harvey.yaml`), and a library function that quietly rewrites a
   user's document would bypass it. `cmd/kb` keeps doing the read/write
   itself, exactly as today.

3. **Pure move, behavior-preserving, one commit per lift item.** No
   behavior changes ride along. The existing `cmd/kb` tests are the
   characterization net: they stay and must pass unchanged. Tests that
   exercise lifted *logic* move with it into the root package; the cmd
   tests that remain prove the CLI surface didn't move. TDD still applies
   to the new exported surface — write the library-level test first,
   confirm red, then move the code.

4. **Lift order follows Harvey's need, not file size:**
   - **L1** document ingest (`IngestDocument`) — Harvey's session-to-
     document pipeline cannot exist without it.
   - **L2** tag / fuzzy-tag text transforms — the curation pass.
   - **L3** concept suggest (with clustering).
   - **L4** frontmatter propose/apply.
   - **L5** record ingest (`IngestRecords`) — **last and optional.** Harvey's
     decisions realignment (item 6) needs `kb ingest harvey/decisions` run
     occasionally, not in an interactive loop, and the `ingester` is the
     largest and most stateful piece. Recommend deferring until L1–L4 have
     shipped and Harvey's actual need is visible.

5. **Frontmatter provenance goes behind an injected interface.** L4 is this
   module's first `exec.Command` (git). Lifted as-is, every importer
   inherits a hidden subprocess. Proposed: a small `Provenance` interface
   (first-commit author/date, last-commit date, `git config user.name`,
   filesystem times) with `GitProvenance{}` as the default the CLI uses.
   Harvey can supply its own, and tests need no real `git init`. **Not
   verified:** whether Harvey's `allowed_commands`/`safe_mode` would
   actually have blocked a library-internal `exec` — it may not intercept
   at that layer at all. The interface is justified by testability and
   caller control regardless; the safe_mode angle is a bonus to check, not
   the rationale.

6. **Debug tracing stays in `cmd/kb`.** `logKBCall` wraps individual KB
   method calls in the CLI. Lifted functions call `kb` methods directly, so
   the `--debug` JSONL trace becomes coarser for lifted verbs: one event at
   the lifted-call boundary (the wrapper logs it) instead of one per
   internal KB call. Accepted cost. Note that `cmdDocumentTag`,
   `cmdDocumentFuzzyTag` and `cmdDocumentFrontmatter` don't receive a
   `DebugLog` today anyway, so for those the trace gets no worse.

7. **Additive only; one release.** L1–L4 ship together as v0.0.12. No
   existing exported symbol changes; `cmd/kb` output is byte-identical.
   Every new exported symbol carries the workspace's `/** … */` block
   (description, parameters, returns, example).

8. **Proposed names (signature sketches — final shapes are settled by the
   plan's red tests, not fixed here):**

   ```go
   // L1
   func (kb *KnowledgeBase) IngestDocument(projectID int64, path string, opts DocumentIngestOptions) (DocumentIngestResult, error) // opts: Title, Format, DryRun
   // L2
   func TagDocumentText(text string, names []string) (string, []string)
   func (kb *KnowledgeBase) EligibleTagConcepts(text string, candidates []string) ([]string, error)
   func FuzzyTagDocumentText(text string, matches []FuzzyConceptMatch) (string, []FuzzyTagInsertion)
   // L3
   func (kb *KnowledgeBase) SuggestConcepts(project string, limit int) (ConceptSuggestions, error) // project name, "" = all
   // L4
   func (kb *KnowledgeBase) ProposeFrontmatter(path string, raw []byte, prov Provenance) (FrontmatterResult, error)
   func (kb *KnowledgeBase) ApplyFrontmatter(path string, raw []byte, prov Provenance, accept FrontmatterAccept, dryRun bool) ([]byte, FrontmatterResult, error)
   ```

## Risks, stated rather than glossed

- **Two v0.0.11 features each hit a design self-contradiction that only
  hand-computation caught** (see DR-0032/0034). A lift that "just moves
  code" must not re-derive the algorithms — move them verbatim and let the
  existing red-green history stand. Any temptation to "clean up while
  we're here" is the failure mode.
- **`DocumentByPath` is keyed on the stored path string.** Once Harvey is a
  second ingester, relative-vs-absolute path convention becomes a
  cross-consumer contract, not an internal detail. L1 must document which
  form `IngestDocument` stores. (Checked only that the lookup is an exact
  string match; not yet checked what `cmdDocumentIngest` normalizes to.)
- **`DECISION_RECORD_FORMAT.md` is cited by root `CLAUDE.md` at
  `~/WorkLab/`, but that path doesn't exist on this machine.** Not
  blocking — `kb record new` scaffolds the format — but the pointer is stale
  here.

## Deferred, explicitly

- **L5 record ingest** — see decision 4.
- Renaming `KnowledgeBase`/`Open` to avoid the package-name stutter —
  already deferred since the module split; still its own follow-up.
- Any API for the TUI. It is a `cmd/kb` concern and stays there.

## Open questions for RSDOIEL

1. Is deferring L5 acceptable, or should the lift be complete before
   Harvey starts? (Recommendation: defer.)
2. `Provenance` as an interface (decision 5) versus lifting `exec.Command`
   directly and accepting the hidden subprocess. (Recommendation:
   interface.)
