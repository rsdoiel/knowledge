# Narrative/document ingestion, graduated abstraction levels — Implementation Plan

See [narrative-documents-design.md](narrative-documents-design.md) for the
full rationale and confirmed decisions. Work items ordered W1 → W8.
TDD-first: each phase's tests are written and confirmed red before its
implementation. This is a bigger feature than the last two — eight phases,
not two or five — because it's a new entity with real internal structure,
not an extension of an existing one.

New code lives in `documents.go` (core schema + queries, mirroring
`records.go`'s role) and `cmd/kb/document.go` (the CLI verb, mirroring
`cmd/kb/record.go`), plus targeted extensions to `retrieval.go`,
`knowledge_merge.go`, `jsonl.go`, and `cmd/kb/helptext.go`.

---

## W1 — Schema + core types: `documents`, `document_sections`

### Files to create

`documents.go`:
- `documentsSchema` const (design decision 1, with `summary_stale` from
  decision 10), applied after `recordsSchema` in `OpenKnowledgeBase` —
  `documents` first, `document_sections` second (FK order).
- `Document` and `DocumentSection` structs mirroring the schema columns.
- `AddDocument(d Document) (int64, error)` — insert-only for now (no
  update-in-place path yet; that's W3's re-ingest logic).
- `AddDocumentSection(s DocumentSection) (int64, error)`.
- `DocumentSections(documentID int64) ([]DocumentSection, error)` — ordered
  by `level` (gist first) then `seq`.
- `DocumentByPath(path string) (*Document, error)` — needed by W3 to find
  an existing document on re-ingest, mirrors `RecordByIdentity`'s role.

### Tests to add (`documents_test.go`)

- `TestAddDocument_CreatesRow`
- `TestAddDocumentSection_CreatesRow`
- `TestDocumentSections_OrdersGistFirstThenBySeq`
- `TestDocumentByPath_FindsExisting`
- `TestDocumentByPath_UnknownPathReturnsNotFound`
- `TestDocumentSections_CascadesOnDocumentDelete`

### Acceptance criteria

- `go test ./...` and `go vet ./...` clean.

---

## W2 — Segmentation and metadata extraction, per format

### Files to modify

`go.mod`: `go get github.com/rsdoiel/fountain@v1.0.2` (same version harvey
already pins — design decision 4).

`documents.go`:
- `detectFormat(path string) string` — extension-based
  (`.md`/`.markdown`→markdown, `.fountain`/`.spmd`→fountain, else text).
- `segmentMarkdown(body string) []DocumentSection` — split on `^#{1,6}\s`
  lines (regex, no library).
- `segmentFountain(src []byte) ([]DocumentSection, map[string]string, error)`
  — `fountain.Parse(src)`; a new section per `fountain.SceneHeadingType`
  element; also returns the `TitlePage` key/value map (decision 4) for the
  caller to fold into metadata the same way frontmatter is.
- `segmentText(body string) []DocumentSection` — the whole file, one
  section, `seq=0`, `heading=""`.
- `extractFrontmatter(data []byte) (fields map[string]string, keywords []string, body string, warning string)`
  — tries `splitFrontmatter` (reused from `recordfile.go`); "no frontmatter"
  is not a warning, "unterminated frontmatter" is (decision 3). Loosely
  decodes into `title`/`description`/`pubDate`/`datePublished`/
  `dateCreated`/`author`/`keywords`, ignoring unrecognized keys.

### Tests to add (`documents_test.go`)

- `TestSegmentMarkdown_SplitsOnHeadings`
- `TestSegmentMarkdown_NoHeadingsIsOneSection`
- `TestSegmentFountain_SplitsOnSceneHeadings` — a small real Fountain
  fixture with two `INT.`/`EXT.` headings.
- `TestSegmentFountain_ExtractsTitlePage` — `Title:`/`Author:` lines.
- `TestSegmentText_IsAlwaysOneSection`
- `TestExtractFrontmatter_ParsesAntennaAppFields` — `title`, `description`,
  `pubDate`, `author`, `keywords` fixture matching antennaApp's real
  vocabulary (design decision 3).
- `TestExtractFrontmatter_MissingFrontmatterIsNotAWarning`
- `TestExtractFrontmatter_UnterminatedFrontmatterWarnsNotFails`
- `TestExtractFrontmatter_IgnoresUnrecognizedKeys`

### Acceptance criteria

- `go test ./...` green.

---

## W3 — `kb document ingest`, including re-ingest matching

### Files to create

`cmd/kb/document.go`:
- `verbs["document"] = cmdDocument`, dispatch to `document ingest|list|show`
  (review/draft/promote land in W5).
- `documentIngest`: reads the file, extracts frontmatter/title-page
  metadata (W2), segments by format (W2), and either inserts a new
  `documents` row + its sections (first ingest) or reconciles against an
  existing one by `DocumentByPath` (re-ingest — design decision 10):
  - Unchanged checksum: skip entirely, matching `kb ingest`'s own
    skip-on-unchanged behavior.
  - Changed checksum: match new sections to old by `(level, heading)`.
    Matched + body unchanged: leave the row alone. Matched + body changed:
    update `body`, set `summary_stale = 1`, leave `summary_body`/
    `summary_status`/`confidence` untouched. Unmatched old: leave the row
    in place, report it (mirrors `kb ingest`'s vanished-record reporting).
    Unmatched new: insert fresh, `summary_status = 'unsummarized'`.

### Tests to add (`cmd/kb/document_test.go`)

- `TestDocumentIngest_CreatesDocumentAndSections`
- `TestDocumentIngest_TitleFromFrontmatterWhenFlagOmitted`
- `TestDocumentIngest_TitleFlagOverridesFrontmatter`
- `TestDocumentIngest_UnchangedFileIsSkipped`
- `TestDocumentIngest_MatchedHeadingUnchangedBodyPreservesSummary` — seed a
  section with a `reviewed` summary, re-ingest identical content, assert
  `summary_status`/`summary_body` unchanged.
- `TestDocumentIngest_MatchedHeadingChangedBodySetsStale` — same seed, edit
  the section's underlying text, re-ingest, assert `summary_stale = true`
  and the summary itself is still there, not deleted.
- `TestDocumentIngest_RemovedHeadingIsReportedNotDeleted`
- `TestDocumentIngest_NewHeadingInsertsUnsummarizedSection`
- `TestDocumentIngest_RejectsPDFExplicitly` — design decision 2's explicit
  "not yet supported" error, not silent mishandling.
- `TestDocumentIngest_DryRunWritesNothing`

### Acceptance criteria

- `go test ./...` green.
- Manual smoke test (per `feedback_smoke_test_data_transforms`): ingest a
  real small Fountain fixture and a real Markdown fixture with antennaApp-
  style frontmatter, inspect the resulting rows directly, then edit one
  section and re-ingest to see the stale flag and preserved summary with
  your own eyes, not just in a unit test.

---

## W4 — Concept tagging: wikilinks, frontmatter keywords, `tag_density`

### Files to modify

`documents.go`:
- `document_section_concepts` table (same shape as `record_concepts`),
  added to `documentsSchema`.
- `LinkDocumentSectionConcept(sectionID, conceptID int64) error` —
  verbatim shape of `LinkRecordConcept`.
- `DocumentSectionConcepts(sectionID int64) ([]Concept, error)` — mirrors
  `RecordConcepts`.

`cmd/kb/document.go`:
- Reuse `wikilinkPattern` from `cmd/kb/ingest.go` (design decision 5): scan
  each `'section'` row's `body` for `[[Name]]`, resolve via
  `ResolveConceptName`, link. Scan the **whole document's** concatenated
  raw text the same way for the `'gist'` row. Also resolve frontmatter
  `keywords` (decision 3) into the gist row's links, same call.
- `tag_density` (design decision 6): `MatchConceptNames(body)` count per
  section; for the gist row, the count against the whole document's text.

### Tests to add

- `TestDocumentIngest_WikilinkInSectionBodyBecomesConcept`
- `TestDocumentIngest_GistTaggedFromWholeDocumentText`
- `TestDocumentIngest_FrontmatterKeywordsBecomeGistConcepts`
- `TestDocumentIngest_TagDensityReflectsMatchConceptNamesCount`
- `TestDocumentIngest_ReingestUnchangedDoesNotDuplicateLinks`

### Acceptance criteria

- `go test ./...` green.

---

## W5 — Review workflow: `review list`, `draft`, `review promote`

### Files to modify

`cmd/kb/document.go`:
- `kb document review list [--project P] [--status S]` — prints
  `document_sections` rows (any status) with size/tag_density/confidence/
  stale columns, the actual triage queue from design decision 7.
- `kb document draft SECTION_ID BODY --by WHO [--confidence N]` — sets
  `summary_body`, `summary_status = 'drafted'`, `generated_by`,
  `confidence` (validated to `[0,1]` if given); clears `summary_stale`
  (a fresh draft is by definition not stale against itself).
- `kb document review promote SECTION_ID` — sets `summary_status =
  'reviewed'`; this is the only place that writes the `kb_fts`
  `'document_summary'` entry (design decision 8) — delete-then-reinsert,
  matching the existing four-writer pattern exactly.

### Tests to add

- `TestDocumentReviewList_ShowsUnsummarizedAndDrafted`
- `TestDocumentDraft_SetsStatusAndClearsStale`
- `TestDocumentDraft_RejectsConfidenceOutOfRange`
- `TestDocumentReviewPromote_IndexesInFTS`
- `TestDocumentReviewPromote_RawSectionBodyNeverIndexed` — regression guard
  for decision 8's other half: confirm a section's raw `body` never appears
  in `kb_fts` regardless of status.
- `TestDocumentReviewPromote_RequiresDraftedFirst` — promoting an
  `unsummarized` section is a usage error, not a silent no-op.

### Acceptance criteria

- `go test ./...` green.

---

## W6 — Widen `RecallByConceptNames` to documents

### Files to modify

`retrieval.go`:
- `ConceptMatch` gains `SummaryStatus string` (design decision 9).
- A third query in `RecallByConceptNames`, joining
  `document_section_concepts` → `document_sections` → `documents`;
  `Body` is `summary_body` when `summary_status == "reviewed"`, else `""`;
  `SourceType` is `"document_gist"` or `"document_section"` (from
  `level`); `SummaryStatus` always set for these rows.

### Tests to add (`retrieval_test.go`)

- `TestRecallByConceptNames_ReturnsLinkedDocumentSection`
- `TestRecallByConceptNames_UnreviewedDocumentMatchHasEmptyBodyButSetStatus`
  — this is the regression guard for decision 9's actual point: an
  `unsummarized`/`drafted` match must still surface (findable by tag) with
  `Body == ""` and `SummaryStatus` explaining why, never silently omitted.
- `TestRecallByConceptNames_MergesAllThreeSourceTypes` — one matching
  observation, one record, one document section in a single call; assert
  all three appear, ranked together.

### Acceptance criteria

- `go test ./...` green.
- Re-run the end-to-end smoke test from `concept-tag-retrieval-plan.md`
  (`MatchConceptNames` → `RecallByConceptNames`) with a real ingested
  document in the mix, confirming three-way composition for real, not just
  in a unit test.

---

## W7 — Portability: `knowledge_merge.go`, `jsonl.go`

### Files to modify

`knowledge_merge.go`: merge blocks for `documents`, `document_sections`,
`document_section_concepts` (uuid-joined, mirroring `records`/
`record_relations`/`record_concepts`); all three added to `allTables`
(now 13) — per DR-0013, a table missing from that list is a table whose
loss during merge is never reported.

`jsonl.go`: `documentRecord`, `documentSectionRecord`,
`documentSectionConceptRecord` types; `exportDocuments`/
`exportDocumentSections`/`exportDocumentSectionConcepts`; matching import
branches, resolved by uuid the same way records are.

### Tests to add

- `TestMergeKnowledgeBases_DocumentsSurvive`
- `TestMergeKnowledgeBases_DocumentSectionsSurvive`
- `TestMergeKnowledgeBases_DocumentSectionConceptsSurvive`
- Update `TestMergeKnowledgeBases_SummaryIncludesRecordTables` (or its
  successor) for the new table count — same pattern as
  `wikilink-tagging-plan.md` W4 hitting this exact hardcoded-count test.
- `TestImportJSONL_Documents_RoundTrip`
- `TestImportJSONL_Documents_ReimportIsNoOp`
- `TestImportJSONL_DocumentSectionConcepts_UnresolvableEndpointSkippedNotFatal`

### Acceptance criteria

- `go test ./...` green.
- Real export → fresh-database import round trip via the built binary, same
  as `wikilink-tagging-plan.md` W4's acceptance check, confirming a
  document with a reviewed summary and concept tags survives intact.

---

## W8 — `kb document list`/`show`, documentation

### Files to modify

`cmd/kb/document.go`:
- `kb document list [--project P]` — one line per document.
- `kb document show ID` — document metadata plus every section (heading,
  status, stale flag, linked concepts) in one view — no separate
  `concepts` subverb (design note: fold into `show` from day one, unlike
  records, which grew `concepts` as an add-on).

`cmd/kb/helptext.go`: new `DocumentHelpText` constant, following
`RecordHelpText`'s structure; add `document` to `KB_TOPICS` in the
Makefile so `make kb-topics-help` generates `kb-document.1.md`.

### Acceptance criteria

- `go test ./...` green.
- `make kb-topics-help` regenerates `kb-document.1.md` cleanly (new file,
  so no pre-existing-content diff to check, but confirm it lands where the
  other generated man pages do).
