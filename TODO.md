
# Action items

## Requested features

- [x] `[[double-bracket]]` inline concept tagging when ingesting Markdown
  record bodies, beyond formal frontmatter. Filed as
  `wikilink-tagging-feature-request.md` (2026-09-08) — inspired by
  [Build a digitally sovereign second brain](https://www.raspberrypi.com/news/build-a-digitally-sovereign-second-brain/)
  (Raspberry Pi magazine). Implemented 2026-09-13: see
  `wikilink-tagging-design.md` and `wikilink-tagging-plan.md` (W1-W5). Both
  `[[Name]]` body scanning and the previously-unused frontmatter `tags` list
  resolve into `record_concepts`, case-insensitively via a new
  `ResolveConceptName` (not `AddConcept`/`kb concept add`, which stay
  exact-match); visible via `kb record concepts RECORD_ID`; carried through
  `knowledge_merge.go` and `jsonl.go` export/import for portability.
  Observation bodies (`kb observation add`) remain out of scope, as decided
  at filing.

- [x] Concept-tag-based retrieval query (concept names → linked
  observations/records). Filed as `concept-tag-retrieval-feature-request.md`
  (2026-09-08). Implemented 2026-09-13: see `concept-tag-retrieval-design.md`
  and `concept-tag-retrieval-plan.md` (W1-W2). New `retrieval.go`:
  `MatchConceptNames(text)` (whole-word, case-insensitive match against
  known concepts) and `RecallByConceptNames(names, limit)` (merged,
  match-count-then-recency-ranked results across both `observation_concepts`
  and `record_concepts`, read-only — never creates a concept, unlike
  `ResolveConceptName`). Not project-scoped, per the original motivation.
  Consuming this from harvey's `UnifiedMemory.Recall` is separate,
  harvey-side work, not done here.

- [x] Ingest narratives/stories (Markdown, Fountain) and articles/posts
  (Markdown, text) as a new `documents` entity, stored/indexed at graduated
  abstraction levels (gist, section/scene, one row each) rather than as one
  flat body. Filed as `narrative-documents-feature-request.md`
  (2026-09-13); implemented 2026-09-13, see `narrative-documents-design.md`
  and `narrative-documents-plan.md` (W1-W8). PDF is a documented, reserved
  format value but explicitly not ingestible yet — no extraction tooling
  exists anywhere in the workspace. Fountain segmentation uses
  `github.com/rsdoiel/fountain` directly (the same module harvey already
  depends on), not a second parser. Tagging/tag_density reuse
  wikilink-tagging and concept-tag-retrieval verbatim — the first real
  validation that those two features' shapes generalize to a second
  consumer. Review workflow (`unsummarized` → `drafted` → `reviewed`)
  mirrors decision records' propose/accept split; only a reviewed summary
  is ever FTS-indexed or returned as trustworthy content from
  `RecallByConceptNames`, which widened to a third merged source. Re-ingest
  matches sections by heading and flags a changed one `summary_stale`
  rather than discarding its summary. Portability (merge + JSON-L) covers
  all three new tables. Consuming any of this from harvey's
  `UnifiedMemory.Recall`, and building an eventual interactive/dialogic
  re-ingest mode in harvey, remain explicitly out of scope here.

- [ ] **Add `cancelled` to the `status` vocabulary.** Raised 2026-09-21 from a
  real case in `~/WorkLab`: `cold` DR-0023 recorded eleven decisions for a
  parameterized report (cold#110), was reviewed and accepted, and the issue was
  cancelled the same day — the report already shipped in v0.0.53 turned out to
  answer the requester's need once he read its CSV into a spreadsheet and
  pivoted it. No second report was needed.

  None of the four current statuses tells that story:

  - `rejected` misstates it. The decisions were not rejected; they were
    accepted on their merits, and the reasoning is still sound. `rejected`
    belongs to a record whose decisions were *never adopted*.
  - `superseded` requires a replacement, and writing one to get the status is
    the tail wagging the dog: it inflates the corpus with a record whose only
    content is "this did not happen", and it labels the original as superseded
    by a decision that replaced nothing.
  - `accepted` leaves a reader believing the work is live. This is the one that
    actually costs something — a reader six months out finds an accepted record
    describing a report that does not exist and cannot tell whether it was
    never built or was built and removed.
  - `proposed` is simply false.

  **The distinction `cancelled` carries is temporal.** `rejected` is *never
  adopted*; `cancelled` is *adopted, then abandoned*. That difference is
  exactly what a reader needs, because a cancelled record's content stays
  valuable in a way a rejected one's usually does not — the design reasoning is
  what someone revisiting the question should start from, and cold DR-0023 says
  so explicitly.

  Worth deciding alongside it:

  - Should a cancelled record carry a *reason*, structurally or by convention?
    "Cancelled because the need was met elsewhere" and "cancelled because it
    was deprioritised" are different signals to whoever revisits it. A
    frontmatter field is probably overkill; a body convention may be enough.
  - Does `index.md` need to distinguish it, or is the status column sufficient?
    The column is already rendered, so likely nothing to do.
  - Vocabularies are documented rather than enforced — an unknown value parses
    and carries a warning — so `kb record set-status 0023 cancelled` already
    *works* today. That makes this mostly a documentation change
    (`DECISION_RECORD_FORMAT.md`, `kb-record(1)`'s VOCABULARIES section) plus
    whatever `kb record list --status` and the index renderer assume. Which is
    an argument for doing it properly rather than relying on the warning path:
    the value would otherwise spread through corpora as an undocumented
    convention.

  **Interim workaround used in the meantime:** a short cancellation record that
  supersedes the original, in `~/WorkLab/agents/projects/cold/decisions/`. It
  works and both sides are written, but it is the second bullet above and
  should be revisited once `cancelled` exists.

## To explore

- [ ] **Programmatic corpus-improvement techniques, as the corpus grows.**
  Raised 2026-09-18 while discussing the MADR item below: decision records
  already benefit from being surfaced through multiple routes (frontmatter,
  `[[wikilinks]]`, `relates_to`), and documents are new and still being
  tuned (see DR-0026/DR-0027's density-linking work). Worth a standing
  question, not a single ticket: what other mechanical, no-dependency
  techniques could improve corpus linkage as it scales past what a human
  curating by hand keeps up with?

  Two concrete candidates raised so far:

  - **Corpus-wide term-frequency/distinctiveness scoring** (TF-IDF or
    similar) to surface *candidate* concepts — terms frequent in one
    document but rare elsewhere — for a human to review via
    `kb concept add`, rather than auto-creating them. This is the
    keyword/topic-discovery half of what density-linking (DR-0026/DR-0027)
    cannot do: `MatchConceptNameCounts` only matches text against concepts
    that already exist, it can't propose new ones. **Shipped 2026-09-18,
    see DR-0028**: `kb concept suggest [--project NAME] [--limit N]`, a
    new read-only verb, scores every record body and document section body
    this way and prints a ranked candidate list; never writes to the
    database. Live-tested against the real `agents/knowledge.db`: a real,
    roughly-50%-signal mechanical pass (`rename`/`harvey`/`divergence`/
    `collision`/`uuid` genuinely useful, alongside generic nouns like
    `row`/`name`/`table`/`test`), the same character as DR-0027's own
    density-linking prototype — a human still curates the output, this
    does not close the loop on its own.
  - **Levenshtein (or similar fuzzy) matching of concept names against
    record/document prose**, to surface records or documents that are
    topically relevant but unlinked because the wording differs from the
    concept's exact name (`MatchConceptNames`/`MatchConceptNameCounts` are
    both exact whole-word matches, so a near-miss — a typo, a plural, a
    close paraphrase — currently links nothing).

  A follow-on question surfaced once `concept suggest` had a real output:
  once a candidate list is confirmed real (`kb concept add`), there was
  nowhere for that judgment to land in the corpus itself — a concept gets
  linked implicitly on the next ingest (density-linking), but invisibly,
  with no durable record in the source file of *why* it applies. **Shipped
  2026-09-18, see DR-0029**: `kb document tag --project NAME [--concept
  NAME,...] [--dry-run]`, a new pure-file-operation verb, inserts an
  explicit `[[Name]]` wikilink at the first safe occurrence of each
  eligible concept per document — eligibility reuses DR-0027's
  density-linking threshold verbatim with no `--concept` given, or bypasses
  it for explicitly-named concepts. Never writes to the database; both-or-
  neither across a project's whole document set; idempotent on re-run.
  Live-smoke-tested against a scratch copy of a real document, which
  surfaced and got a real fix: a concept mention inside the document's own
  H1 title was getting wikilinked, mechanically correct but stylistically
  wrong, now excluded alongside frontmatter/code spans/existing wikilinks.
  A frontmatter-`keywords:`-only alternative was considered and set aside
  (DR-0029's Rejected alternatives) — worth revisiting only if a corpus
  specifically wants a no-prose-edit option.

  Both fit the pattern already established for document summaries and
  decision records: a mechanical pass surfaces candidates, a human curates
  rather than the pass auto-committing. The alternative — embeddings or an
  LLM call for either keyword extraction or fuzzy relevance — would likely
  do better, but pulls in exactly the kind of dependency (a vector store,
  network calls, nondeterminism) this module has deliberately avoided so
  far; worth weighing explicitly if either of the above turns out too weak
  in practice, not assumed as the starting point.

## Planned for v0.0.11

Scoped 2026-09-19, item 5 added 2026-09-22, item 6 (bug) added and fixed
2026-09-23, item 1 implemented 2026-09-23. Four items; item 1 done, items
2 and 5 not started, item 6 fixed. The 2026-09-19 scoping also carried
forward two items — `kb project rename`'s corpus-rewrite fix and
cross-machine rename reconciliation in `merge`/`import` — that DR-0026
(2026-09-18, see Done below) had already shipped the day before; removed
here 2026-09-22 as stale duplicates once that was noticed.

- [x] **New verb `kb document fuzzy-tag`**, promoted from the
  corpus-improvement-techniques item above and refined 2026-09-21: see
  `fuzzy-concept-matching-design.md`. `MatchConceptNames`/
  `MatchConceptNameCounts` are exact whole-word matches today, so a typo,
  plural, or tense variant of a known concept's name currently links
  nothing (paraphrase is explicitly out of scope — a semantic problem, not
  a spelling one). Mirrors `kb document tag`'s own shape (`--project`,
  `--concept`, `--dry-run`) with fuzzy matching substituted for exact, but
  never bracket-wraps the near-miss text itself — doing so would let
  `ResolveConceptName` mint a duplicate concept from the misspelled/
  inflected form. Instead inserts a footnote marker at the near-miss and a
  footnote definition carrying the canonical `[[Concept]]`, so prose stays
  unedited and linking still resolves to the real, existing concept.

  **Implemented 2026-09-23** per `fuzzy-concept-matching-plan.md`'s F1–F4.
  Found and corrected a real self-contradiction in the design's decision 5
  while implementing F1: stemming *both* the concept name and the
  candidate token (as originally written) breaks the design's own worked
  examples — verified by hand, then fixed to raw-distance-first with a
  stemmed-token-only fallback (see `fuzzy-concept-matching-design.md`'s
  amended decision 5, and DR-0032 for the full writeup). `retrieval.go`
  gained `FuzzyMatchConceptNames`/`levenshteinDistance`/
  `stripCommonSuffix`; new `cmd/kb/documentfuzzytag.go` holds the footnote
  mechanics and `cmdDocumentFuzzyTag`, wired into `document`'s subverb
  switch and documented in `DocumentHelpText`/`kb-document.1.md`. All of
  F1–F4's listed tests pass, `go vet`/`go build` clean, and live-smoke-
  tested against a real scratch document (footnote placed correctly,
  idempotent on rerun, concept actually linked after re-ingest). Decision
  record DR-0032 authored `proposed`, not yet promoted.

- [ ] **A standalone frontmatter-generator command, for documents.**
  Explicitly *not* an alternative to `kb document tag` and not folded into
  it — tagging links known concepts into existing prose; this is a
  separate, more general documentation-maintenance tool for
  producing/maintaining a document's frontmatter block on its own terms.
  Filed as `frontmatter-generator-feature-request.md` (2026-09-19):
  propose-then-accept for `title`/`author`/`dateCreated`/`keywords`, plus a
  **new `dateModified` field** added to the document frontmatter schema as
  part of this work. Keyword proposals split into known-concept matches
  (plain write) and new candidate concepts (must be explicitly created on
  accept, never silently auto-minted at next ingest). **Design and plan
  finished** — see `frontmatter-generator-design.md` and
  `frontmatter-generator-plan.md`: verb is `kb document frontmatter PATH
  [--accept FIELD,...] [--accept-keywords NAME,...] [--set FIELD=VALUE]
  [--dry-run]`; provenance shells out to `git` (this module's first
  `exec.Command` dependency), falling back to filesystem mtime/birth time.
  Not yet implemented.

- [ ] **Fuzzy-aware `kb concept suggest`**, folded in 2026-09-22 from the
  "To explore" list: see `fuzzy-concept-clustering-design.md`. A distinct
  mechanism from item 1's `fuzzy-tag` — that one matches document text
  against *already-known* concepts; this one clusters `concept suggest`'s
  own *candidate* terms *against each other*, before any of them are
  concepts, so a term split across spelling variants (`chunking`/
  `chunkings`/`chunked`) scores as one merged candidate instead of several
  individually-sub-threshold ones. Ten decisions settled: shares
  Levenshtein/suffix-strip normalization code with `fuzzy-tag` (decision
  1); clusters the raw pre-filter occurrence map, not the already-limited
  output (decision 2); a token fuzzy-close to an existing concept is
  excluded from candidacy and reported separately rather than clustered
  (decision 3); "star" clustering bounded by distance-to-seed rather than
  chain/transitive clustering, to avoid unrelated terms drifting together
  (decision 4); flat distance-1 threshold, tighter than `fuzzy-tag`'s
  (decision 5); canonical spelling = the highest-occurrence seed (decision
  6); cluster item-count(df) must union item sets across members, not sum
  them, or idf comes out wrong (decision 7); output extends `candidateTerm`
  with an optional `variants` field (decision 8); always on, no flag
  (decision 9); length-difference pruning keeps the pairwise comparison
  cheap (decision 10). **Plan finished** — see
  `fuzzy-concept-clustering-plan.md`. No code written.

- [x] **Bug: `kb search TERM` threw a raw SQLite error instead of a normal
  "no results" for any term containing bare punctuation FTS5's query
  grammar treats as significant** (a `.` was the case found; likely also
  `-`, `:`, `*`, parens). Root cause confirmed live 2026-09-23:
  `Search` (`knowledge.go`, `WHERE kb_fts MATCH ?`) bound the user's raw
  term directly as the MATCH string. FTS5 parses that string with its own
  query grammar (`AND`/`OR`/`NOT`/`NEAR`/column-filters/phrases) *before*
  tokenizing, so an unquoted `.` — not a token character for `unicode61`
  and not a grammar operator either — had nowhere to go:
  `sqlite3 ... MATCH 'v0.0.11'` → `Error: fts5: syntax error near "."`,
  while `MATCH '"v0.0.11"'` (quoted phrase, grammar doesn't look inside
  quotes) returns 9 rows.

  **Fixed 2026-09-23.** `kb-search(1)`/the CLI helptext document raw FTS5
  query syntax as a supported feature (multi-word AND, quoted phrases,
  `prefix*`), so the fix could not blanket-quote every term — that would
  have silently turned `kb search foo bar` from an AND of two terms into
  an adjacency phrase. Instead `Search` now retries once, only when the
  first attempt's error is specifically an FTS5 syntax error
  (`isFTS5SyntaxError`, matched on `"fts5: syntax error"` in the driver's
  error text), rewriting the term as a quoted, `"`-escaped phrase before
  the retry. The `glebarez/go-sqlite` driver surfaces a MATCH parse error
  while stepping rows, not from `Query` itself, so the query logic moved
  into a `runSearch` helper whose error is observed via `rows.Err()`
  before `Search` decides whether to retry. TDD: `knowledge_test.go`
  gained `TestSearch_TermWithBarePunctuationDoesNotError` (confirmed red
  against the unfixed code) and `TestSearch_MultiWordTermStillANDs` as a
  regression guard for the documented AND syntax. `go build`/`go vet`/
  `go test ./...` clean; live-verified with `kb search "v0.0.11"` against
  the real `agents/knowledge.db` (exit 0, one result, no driver error).
  Not yet committed or given a decision record.

## Done

- [x] **Two decision-record dialects now exist in one organisation, and `kb`
  can only read one of them.** Raised 2026-09-15 after pulling
  `caltechlibrary/CL-Web-Components`, where a colleague had been recording
  architecture decisions independently; by 2026-09-18 it was two repositories
  and twelve records, `workflows/docs/decisions` being the second. Filed as a
  design question rather than a bug: the refusal `kb ingest` gave
  (`no frontmatter: file does not start with ---`, one per file) was always
  the right refusal, but it left no partial path.

  **Closed 2026-09-18 by DR-0027**, which answered what this item itself
  called "the real question to answer first": a colleague's ADR is a
  **document**, not a record. We do not own its status and can never promote
  or supersede it, and a document is exactly the shape for source material we
  read and summarise. `kb document ingest` reads MADR without complaint — no
  frontmatter required, segmenting on its `## Context and Problem Statement` /
  `## Decision` / `## Consequences` headings, 5-14 sections each.

  **Answering it that way dissolves most of what this item was worried
  about.** The identity collision — both dialects numbering from 0001, both
  claiming `(WorkLab, CL-Web-Components, project, 0001)` — cannot arise,
  because a document never enters the records identity tuple at all. So the
  `dialect` column, the third `external` scope value, and the `--format madr`
  adapter are all moot rather than deferred: each existed only to make a
  foreign ADR safe *as a record*.

  **Two gaps in the stopgap were real and were fixed in the same pass** (also
  DR-0027): with no frontmatter `title:`, a document used to be titled with
  its *filename* — `ParseDocumentFile` now falls back to the first true H1,
  general rather than MADR-specific; and no concepts used to be linked at all,
  since linking came only from `[[wikilinks]]` and frontmatter `keywords`,
  neither of which MADR has. Document ingest now also auto-links a known
  concept mentioned more than once outside code spans
  (`MatchConceptNameCounts`).

  **The cost accepted, stated plainly: a document cannot be cited from
  `relates_to`.** That is the one thing the record reading would have bought.
  There is no `document_relations` table and no record-to-document edge, so a
  decision record of ours cannot formally point at the colleague's ADR it is
  responding to — the link has to live in prose. This is a known, accepted
  limitation of DR-0027, not an oversight; it is worth reopening only if
  citing a foreign ADR by id turns out to be something we actually reach for
  repeatedly, and reaching for it twice is a better trigger than predicting it
  now.

  **Evidence, 2026-09-15: density-based linking was run by hand against those
  four ADRs before any of it shipped, and it was useful but lossy — roughly
  60% signal.** This is what set DR-0027's threshold, so it is kept here
  rather than summarised away. The script mirrored kb's own semantics rather
  than inventing new ones: the `(?i)\b<name>\b` whole-word match from
  `MatchConceptNames`, computed over `wholeDocumentText`. Confirmation the
  mirror was faithful: the match counts came out 3, 2, 4, 4 — identical to the
  `tag_density` already stored on each gist.

  It produced 13 links from 95 known concepts, and the effect was real: a
  query naming `cmtools`, `documentation` and `tooling` returned ADR-0004
  first, above a workspace decision record, where before it returned nothing
  from either retrieval pass — FTS5 ANDs its tokens, so a conversational
  question misses a document with no concept links at all.

  Eight of the thirteen were topical. **Five were coincidental, and the way
  they failed is the useful part**, because all five are the same failure — a
  concept name colliding with a word used in a different sense:

  - `format` matched `unsupported format` in a quoted *error message*, and
    separately matched the variable in `const format = outputName`.
  - `index` matched "a committed search index", a different sense from the
    `index` concept (a generated `decisions/index.md`).
  - `git` matched an incidental `git push` in a sentence about publishing.
  - `validation` matched a phrase describing a *third* project's domain, not
    the document's own subject.

  So the lesson was narrower than "density is too noisy". Whole-word matching
  is not the problem; **short, common, single-word concept names are**, and
  prose quoting code and error strings makes them worse. Of the three
  refinements costed at the time, DR-0027 shipped the first two — require more
  than one occurrence, and exclude matches inside code spans and fenced blocks
  — which between them kill `const format = outputName` and probably
  `git push`. The third, writing density matches as *suggestions* a human
  confirms, was not built; `kb document tag` (DR-0029) covers the same ground
  from the other end, by making a confirmed link explicit in the file.

  **The five coincidental links were pruned by hand the same day.** Two things
  the prune showed. It cost no coverage: all four ADRs stayed reachable from
  concept-tag recall, because each retained at least one topical link, so the
  noisy matches were redundant rather than load-bearing. And `tag_density` is
  deliberately *not* updated to match — it counts mentions, not links — so
  gists 19 and 30 read density 4 against 2 links each. DR-0027 preserved that
  divergence on purpose: density is the raw signal, links are what survived
  review, and the gap between them is exactly the quantity a suggestion
  workflow would surface.

- [x] **`kb project rename`'s documented escape hatch didn't work, so the
  refusal was absolute for any project that owns records.** Found 2026-09-17
  from WorkLab, trying to carry out that workspace's `codemeta` →
  `codemetatools` rename (WorkLab `codemeta` DR-0006, accepted). The manual
  path the refusal recommended — rewrite the corpus's `project:` frontmatter,
  re-ingest — was itself blocked: a record whose `project:` no longer
  matched its database row made `upsertAll` treat the file as *new* under
  `(workspace, project_id, scope, record_id)` identity, but the file still
  carried its old, stable `uuid`, and `records.uuid` is `UNIQUE` — the
  `INSERT` collided. A first attempt was worse than a no-op: the failed
  ingest still minted an empty project under the new name, which then
  tripped rename's *other* guard (`NEW` already exists), leaving the
  database stuck with no verb to remove the stray row.

  **The fix turned out simpler than either shape this item originally
  sketched** (routing through `ingest` with `uuid`-aware reparenting, or a
  general rename-event log): `records.project_id` never needs to change on
  a rename, since it is a stable foreign key and only `projects.name`
  changes. So the corpus rewrite never has to go through `ingest`'s
  identity resolution at all — it edits every owned record's file in place,
  touches no database row, and leaves the very next ordinary `kb ingest` to
  see a changed checksum against an unchanged identity, the ordinary UPDATE
  path, not the INSERT path that collided.

  Tracing the merge/import code while designing this turned up two further,
  live-verified bugs this item hadn't anticipated: `kb merge` could
  silently keep a stale name or drop a side outright depending on merge
  order (`INSERT OR IGNORE` couldn't distinguish a `uuid` collision from a
  name collision), and `kb import` hard-failed the *entire* import, not one
  row, on the same collision. Both are fixed in the same pass — shipping
  the rename completion without it would have made the corruption risk
  worse, not smaller, by making the trigger routine.

  See DR-0026 (`knowledge/decisions/`), which supersedes DR-0024's outright
  refusal (partial) and DR-0025's "a rename crossing machines does not
  reconcile" follow-on (partial). Shipped: `kb project rename OLD NEW`, when
  `OLD` owns records, rewrites every owned record's `project:` frontmatter
  both-or-neither before renaming the project row, via a new
  `RenameProjectRow` library method that skips `RenameProject`'s
  records-owning guard; `--dry-run` and `--root` are new flags. `kb merge`'s
  and `kb import`'s `projects`/`concepts` conflict resolution moved from
  name-keyed to `uuid`-primary, reconciling `name` by `updated_at` the same
  way `description`/`status` already did (DR-0025), with a name-keyed
  fallback retained only for the rare case of two independently created,
  never-synced entities that happen to share a name (DR-0003, unchanged).
  `reportMissing`'s advice no longer names the nonexistent `kb record
  remove` verb, found stale while reproducing this.

- [x] `kb record new`'s default write path moved to `agents/projects/<project>/decisions/`
  for project scope (`agents/decisions/` unchanged for `--workspace`), plus a
  `--dir` override. See `agents-projects-layout-feature-request.md` (filed
  2026-08-28), `DR-0021` and `DR-0022`, and
  `record-layout-and-workspace-init-plan.md`. Migrating existing corpora
  under the old shape (`clasm/decisions/`, `cold/decisions/`,
  `agents/decisions/caltechauthors/`) and whether `kb` should index `plans/`
  or `feature_requests/` at all are both explicitly out of scope, not
  overlooked — see `DR-0021`'s Consequences.

- [x] `kb record new` (and `kb observation add`) run from *inside* a project
  directory no longer silently creates a stray nested corpus or an empty
  ambient `agents/`. Fixed at the path-resolution layer, as this item itself
  suggested: `cmd/kb/main.go`'s ambient `--db` resolution now refuses to open
  (and so auto-create) a database that doesn't exist, naming `kb init` and
  `kb import -in FILE` instead — a wrong-cwd invocation fails loudly rather
  than fabricating a workspace where it happens to stand. See `DR-0021` item 4
  and `DR-0022` (the guard applies only to the true ambient default, not an
  explicit `--db PATH`).

- [x] `kb project set-description NAME DESCRIPTION` — projects had no way to
  correct a description after `add`, `set-status` being the only in-place
  mutation. Raised 2026-08-26 from the `dev-process` project, whose
  description named `DESIGN_DECIDE_PLAN.md` after that file was renamed to
  `DESIGN_REVIEW_PLAN_IMPLEMENT.md`. Refreshes the FTS row and touches
  `updated_at`. See DR-0012.

- [x] `kb concept add` no longer wipes a description. The request had listed
  `concept` among the add-only verbs; it was the opposite —
  `AddConceptWithIdentifier` carried `description = excluded.description`
  unconditionally while both identifier columns beside it were guarded, so
  `kb concept add NAME` with no description silently cleared the stored one
  in the row and in the FTS index. The guard now covers `description` too.
  See DR-0012.

- [x] `kb ingest` pruned neither a relation nor a concept link dropped from a
  record's frontmatter/body, so `record_relations` and `record_concepts` only
  ever grew (found 2026-08-27 and 2026-09-15 respectively, authoring `clasm`
  DR-0170/DR-0171 and WorkLab's DR-0010). Fixed 2026-09-15: `resolveAll`
  now calls the new `ClearRecordRelationsFrom(fromID)` before re-adding a
  record's own supersedes/relates_to edges each run, scoped to forward edges
  so the other side's own declarations survive; `linkWikilinkTags` calls the
  new `ClearRecordConcepts(recordID)` the same way before re-linking. The
  same gap existed in `document_section_concepts` via `tagSection`, fixed
  with the new `ClearDocumentSectionConcepts(sectionID)`.

- [x] `[[NNNN]]` (e.g. `[[0007]]`, `[[DR-0007]]`) in a record body minted a
  junk concept instead of citing a record — found 2026-09-15, same session as
  the concept-prune item above. `linkWikilinkTags` (`cmd/kb/ingest.go`) now
  matches such a name against `recordIDLikeWikilink`
  (`^(?i:dr-)?[0-9]{4}$`), skips linking it, and adds a warning pointing at
  `supersedes`/`relates_to` instead.

- [x] `kb ingest` never updated `records.path` on a pure file move (found
  2026-08-28 migrating `clasm`'s 173-record corpus under the
  `agents/projects/<project>/` layout; WorkLab's data separately repaired
  out-of-band on 2026-09-15, see the git history for that record). Fixed
  2026-09-15: `upsertAll`'s checksum-match (skip) branch now calls the new
  `UpdateRecordPath` when the incoming path differs. The
  "DR-%s was stored at %s" warning now fires only when the checksum *also*
  differs — a path change alone is an ordinary move, not a possible slug
  collision. `reportMissing`'s message no longer asserts deletion as the only
  explanation (it used to say "use kb record remove to drop it" for a record
  that had only moved out of the ingested prefix, which would have deleted
  five live WorkLab records); it now names a move as the other possibility
  and asks for a re-ingest of the new location first.

- [x] `kb search` exited 0 when it found nothing, against the workspace's
  search-tool convention (`~/Laboratory/CLAUDE.md`: exit 1 on no match).
  Fixed 2026-09-15: `cmdSearch` returns an error instead of printing "no
  results" and returning nil, so it now exits 1 in both text and `--json`
  mode.

- [x] `kb observation update` didn't exist — reached for in real usage
  2026-09-16, the same amend-vs-supersede question DR-0012 deliberately left
  open for observations. See DR-0023 (`knowledge/decisions/`): an observation
  is corrected by superseding it, never by mutating its body. Shipped
  2026-09-16: `kb observation update ID BODY...` inserts a new observation
  (inheriting `ID`'s project and kind) and links it to `ID` via a new
  `observation_relations` table — `record_relations`' own shape, reused —
  with `AddObservationRelation`/`ObservationRelationsFor` mirroring
  `AddRecordRelation`/`RelationsFor`. The old observation's body is never
  touched, so history retention is free; no `updated_at` column was added,
  since the new observation's own `created_at` is the correction timestamp.
  `kb observation show ID` now resolves and prints `supersedes`/
  `superseded_by`, mirroring `kb record show`. `kb merge` and JSON-L
  export/import carry `observation_relations` from this release, not as a
  follow-on, per the standing rule that a table missing from the merge
  summary is a table whose loss goes unreported.

- [x] `kb project rename` and `kb concept rename` didn't exist. **Correcting
  this item's own earlier diagnosis**: `set-description`'s reindex was
  blamed for the `kb_fts` duplicate, but `refreshProjectFTS` deletes by
  `source_id`, not by name — a live repro confirmed `set-description`
  correctly repairs a stale label after a raw rename, one row, not two. The
  real duplicate came from a different natural mistake, `project add
  NEWNAME` used to "rename" (`ON CONFLICT(name)` doesn't fire for a new
  name, so it inserts a second, fully independent project). The harder
  failure was worse than described, too: re-ingesting a renamed project's
  corpus without rewriting its files' `project:` frontmatter silently mints
  a *phantom* project under the old name and *duplicates* the record under
  it — reproduced live, not merely asserted. See DR-0024
  (`knowledge/decisions/`) for the full repros and the design. Shipped
  2026-09-17: `kb project rename OLD NEW` refuses if `NEW` exists or if the
  project owns any records, otherwise renames and reuses the existing
  `refreshProjectFTS`. `kb concept rename OLD NEW` ships in the same pass
  with no corpus-refuse condition, since every concept link is a foreign
  key to `concepts.id`, never a name matched from a file — via a new
  `refreshConceptFTS` factored out of `AddConceptWithIdentifier`'s own
  inline version. Rewriting a corpus's frontmatter automatically to lift the
  refusal remains open, filed as its own future record in DR-0024's
  Rejected alternatives.

- [x] Cross-machine reconciliation of an edited project or concept
  description. Deferred out of the `set-description` work (DR-0012) as a
  policy inversion. **Two of this item's own claims were stale by the time
  it was picked up**: `updated_at` already travelled through `merge` for
  `projects` — the bug was `INSERT OR IGNORE` never consulting it, insertion
  order alone deciding the winner, not a missing column; and observations
  need none of this at all, since DR-0023 already resolved observation
  correction via supersession, retiring the question this item was still
  asking. See DR-0025 (`knowledge/decisions/`) for both corrections in full,
  and its repros. Shipped 2026-09-17: `concepts` gained `updated_at`
  (lazily migrated, backfilled to `created_at`, mirroring how
  `concepts.created_at` itself was added). `kb merge`'s `projects`/`concepts`
  passes changed from `INSERT OR IGNORE` to `INSERT OR IGNORE` (new rows)
  plus a guarded `UPDATE ... FROM ... WHERE incoming.updated_at >
  existing.updated_at` (conflicts) — two statements rather than one
  `INSERT ... SELECT ... ON CONFLICT DO UPDATE`, because the pure-Go SQLite
  driver this project uses rejects an UPSERT clause after a SELECT-form
  INSERT ("near DO: syntax error"), confirmed live, even though the same
  UPSERT works after a VALUES-form INSERT elsewhere in this codebase.
  `importProject`/`importConcept` gained the matching comparison at the Go
  level, plus a new `UpdatedAt` field on `projectRecord`/`conceptRecord`.
  `RenameProject`, `RenameConcept`, and `AddConceptWithIdentifier`'s
  `ON CONFLICT` branch now touch `updated_at`, closing two gaps DR-0024 and
  DR-0012 left open now that the column is compared for real.
  `kb-project(1)`/`kb-concept(1)`'s CAVEATS sections now describe what
  actually happens: descriptions/status reconcile by timestamp; a *rename*
  crossing machines still does not, since `merge`/`import` dedupe by name —
  filed as its own follow-on in DR-0025, not solved here.

- [x] A mechanism for knowing when `index.md` needs regenerating — the
  single-corpus half (`--check`, and `set-status`/`supersede` auto-refresh)
  shipped 2026-09-15/16; the multi-corpus half shipped 2026-09-17.
  `kb index ROOT --all [--check]` discovers every corpus under `ROOT` and
  refreshes or checks each one, continuing past one corpus's failure so it
  does not hide the rest, then exits non-zero if any needed attention — one
  call for a whole workspace instead of naming each corpus by hand.

  **Found running it live against the real workspace, before it shipped:
  matching on the filename `index.md` alone is unsafe.** The first version
  walked for any file named `index.md`; against the real Laboratory tree it
  also matched several files with nothing to do with `kb` — a llamafile
  docs-site front page, a Jekyll blog index — and reported them as "stale"
  corpora under `--check`. In write mode it would have silently overwritten
  them with an empty decision-records template. Fixed before ever running in
  write mode against real data, by requiring both signals together: the
  file's own content has to open with this format's generated-file heading,
  *and* the directory has to hold at least one record file. Regression tests
  cover both the check and write paths directly, since this is exactly the
  kind of bug a test written after the fact would not have caught with
  confidence.

  Resolves the "does the index belong on disk at all" question left open
  alongside this item, by default rather than by new argument: `--check` and
  `--all` remove the staleness that was the only real cost of keeping it, so
  the existing case for it — `head`, `grep`, `awk` reach a corpus without
  `kb` installed — stands uncontested. Removing the file was never
  implemented or seriously pursued.
