# Frontmatter enhancement for documents — Feature Request

> **2026-09-19: Filed.** Captured from a conversation with RSDOIEL about
> scoping v0.0.11. No design/decide/plan cycle has been run yet. This
> document preserves the idea and the decisions already made about its
> shape — a starting point for that cycle, not a committed design.
>
> Decided in the filing conversation:
> - Targets **documents only**, not decision records. Records already have
>   most of their metadata tool-filled by `kb record new`/ingest, and have
>   no `author` field at all — adding one would be its own schema decision,
>   out of scope here.
> - A **new, standalone subcommand**, explicitly not a mode of
>   `kb document tag` and not merged into it: tagging links known concepts
>   into a document's prose; this instead maintains the frontmatter block
>   itself, for general documentation writing/maintenance, not only for
>   documents ingested as narrative content.
> - **Propose, then accept** — mirroring `kb concept suggest`'s read-only
>   report and `kb document tag`'s `--dry-run`/`--concept` selective-apply
>   shape. Default invocation is read-only. A field already set is never
>   overwritten — this command only fills gaps, consistent with document
>   frontmatter being optional by design.
> - Fields in scope: `title`, `author`, `dateCreated`, `keywords`, and a
>   **new `dateModified` field**, added to the schema as part of this work
>   — it does not exist in `documentFrontmatter` today.
> - `keywords` proposals split into two kinds with different accept
>   behavior: known concepts found in the document's own text but not yet
>   listed (a plain write, the concept already exists), and new candidate
>   concepts distinctive to the document but not yet known to the corpus at
>   all (accepting one **explicitly creates the concept**, the same effect
>   as `kb concept add`, never a bare string left for `ResolveConceptName`
>   to silently resolve-or-create at the next `kb document ingest`).

## Motivation

`TODO.md`'s "To explore" section has carried a standing question since
2026-09-18 — what other mechanical, no-dependency techniques could improve
corpus linkage as it scales past what a human curating by hand keeps up
with — and DR-0029 explicitly considered and set aside a
frontmatter-`keywords:`-only mechanism as *tagging's* primary approach,
noting it was "[n]ot ruled out as a *future*, smaller, no-prose-edit option
for a corpus that specifically wants to avoid inline edits — just not built
here." This feature request is that option, built — but scoped wider than
just keywords, and explicitly as its own tool rather than an alternative
inside `kb document tag`.

Document frontmatter is optional by this module's own design
(`extractFrontmatter`'s "no frontmatter is normal, not an error" decision;
see below) — which means a real document in this corpus can be missing a
title, an author, a creation date, or any keywords at all, with nothing
today surfacing that gap or filling it. Several of those fields are
mechanically derivable — from the document's own content (`title`) or from
git/filesystem provenance (`author`, `dateCreated`, `dateModified`) —
without needing a model call or new external dependency beyond (possibly)
`git` itself.

## What `knowledge` assumes today

- `documentFrontmatter` (`documents.go:471-479`) carries exactly `title`,
  `description`, `pubDate`, `datePublished`, `dateCreated`, `author`,
  `keywords` — **no `dateModified` field at all**, even though the
  vocabulary this mirrors (antennaApp's own post frontmatter — see
  antennaApp's `helptext.go` documented fields and `schema.go`'s handling
  of `doc.FrontMatter["dateModified"]`) treats `dateModified` as distinct
  from `datePublished`/`pubDate`. This is a real gap in the schema, not
  just a missing feature, and closing it is explicitly in scope here.
- `extractFrontmatter` (`documents.go:486-521`) treats a document with no
  frontmatter block at all as the ordinary case, not an error ("design
  decision 3" in the code's own comment) — a Fountain screenplay or a plain
  story usually has none. This command has to be able to *add* a
  frontmatter block where none exists, not only edit one that's already
  there.
- `keywords:` resolves through `tagSection` (`cmd/kb/document.go:317-348`)
  → `kb.ResolveConceptName` per entry, at every `kb document ingest` —
  case-insensitive, **find-or-create**. Any string that lands in
  `keywords:` becomes a real concept on the next ingest, whether or not a
  human meant for it to be created.
- `kb document tag`'s `--concept NAME,...` (`cmd/kb/documenttag.go:256+`)
  resolves each named concept against `kb.Concepts()` and refuses the
  *whole call* on an unknown name, checked before any file is opened — it
  deliberately never relies on `ResolveConceptName`'s auto-create for a
  human-named candidate. This is the precedent this feature request's
  "new candidate keywords must be explicitly created on accept" decision
  follows.
- `eligibleConcepts`/`densityLinkCandidates`
  (`cmd/kb/documenttag.go:198-212`, `cmd/kb/document.go:393+`) already
  implement "a known concept mentioned more than once in this text,
  outside code spans" — directly reusable for the first keyword signal
  (known concepts, unlisted).
- `cmdConceptSuggest` (`cmd/kb/concept.go:225-259`) scores candidate terms
  as `occurrences × idf`, `idf = log(N/df)`, over a set of "items" (each
  record body or document section is one item), keeping only `idf > 0`.
  This is the shape for the second keyword signal (new candidate
  concepts) — but it is **not a straight reuse** for a single document:
  if the item set being scored shrinks to just that one document's own
  sections, every term trivially satisfies `df == N`, `idf` collapses to
  `0`, and nothing survives the filter. A per-document distinctiveness
  signal needs occurrences counted within the target document but `idf`
  computed against a wider comparison scope — see open questions.
- **No file in this module shells out to `git` or reads filesystem
  timestamps for provenance today** (checked: no `exec.Command` calls
  anywhere in the codebase). Deriving `author`/`dateCreated`/`dateModified`
  from git or the filesystem would be a new capability, not a reuse of an
  existing helper.

## What is already fine

- `frontmatterBlockPattern`/`splitFrontmatter` (`recordfile.go`, reused by
  `documenttag.go` and `documents.go`) already locate and parse a leading
  `---`-delimited block — the read side needs no new parser.
- The propose-then-write, both-or-neither-across-a-run pattern is already
  fully worked out by `kb document tag`
  (`cmdDocumentTag`, `cmd/kb/documenttag.go:256+`): stage every write in
  memory, touch disk only once every file in the run is ready, restore
  from original bytes on any failure. This command's accept step is the
  same shape applied to frontmatter fields instead of inline wikilinks.
- `kb.Concepts()` / `kb.ResolveConceptName()` / the explicit
  create-a-concept path `kb concept add` already uses — no new database
  schema needed on the concept side of keywords.
- The narrative-documents review workflow's `unsummarized` → `drafted` →
  `reviewed` states already establish, elsewhere in this corpus, that a
  mechanical draft and a human-trusted value are different things with
  different states. The propose/accept split here is the same idea applied
  to metadata fields instead of prose summaries.

## Proposal

1. A new subcommand, working name `kb document frontmatter PATH` (final
   verb name and CLI shape open — see below), dispatched from `cmdDocument`
   alongside `ingest`/`tag`/`draft`/`review`/`list`/`show`.
2. Default invocation is read-only: report, per field, what's missing and
   what would be proposed, with a source/reason for each proposal (e.g.
   "title: missing, proposed from first H1: '...'"). A field that already
   has a value is never overwritten.
3. Field-by-field checks:
   - **`title`** — absent only; propose from the document's first true H1,
     the same fallback `ParseDocumentFile` already applies at ingest time
     for a missing frontmatter `title:` (DR-0027), reused here for a
     command a human runs deliberately.
   - **`author`** — absent only; propose from the file's git history (e.g.
     earliest committing author) or `git config user.name`/`user.email` as
     a fallback for an untracked file.
   - **`dateCreated`** — absent only; propose from git (first commit
     touching the file) or filesystem birth/ctime as a fallback when git
     history is unavailable.
   - **`dateModified`** — new field, added to `documentFrontmatter` as part
     of this work. Propose from git (most recent commit touching the file)
     or filesystem mtime as a fallback. Whether "never overwrite" applies
     to this field the same way it does to the other three is an open
     question below — a modified-time field is expected to change on a
     re-run, unlike the others.
   - **`keywords`** — a diff, not a binary: current `keywords:` list versus
     two candidate sets — (a) known concepts matched in the document's own
     text but not yet listed (`eligibleConcepts`/`densityLinkCandidates`'s
     existing threshold), and (b) new candidate concepts distinctive to
     this document specifically but not yet known to the corpus at all (a
     document-scoped variant of `cmdConceptSuggest`'s scoring — see open
     question).
4. Accept is selective at two granularities: which *fields* to take, and
   — for `keywords` specifically — which individual *entries*. Accepting a
   signal-(a) keyword is a plain frontmatter write. Accepting a signal-(b)
   keyword **explicitly creates the concept first** (the same effect as
   `kb concept add NAME`), then writes it to `keywords:` — mirroring
   `kb document tag --concept`'s "must already exist, checked up front"
   rule rather than opening a second, looser path to the same database
   mutation.
5. Writing to a document with no frontmatter block at all inserts one,
   rather than refusing, consistent with `extractFrontmatter` already
   treating "no frontmatter" as the ordinary case for this module's
   documents.

## Open questions for the design cycle

- **Command name and CLI shape.** A single verb, propose-by-default, with
  flags for selective accept (e.g. `--accept title,author`,
  `--accept-keywords NAME,NAME2`, `--all`) — matching `kb index --check`'s
  single-verb-plus-flag convention — or a `suggest`/`apply` pair, matching
  `kb concept suggest` + `kb concept add`'s two-verb convention? Both
  conventions already exist in this codebase; not yet chosen.
- **Per-document distinctiveness scoring for new-candidate keywords.**
  `cmdConceptSuggest`'s `occurrences × idf` shape degenerates to zero if
  naively scoped to a single document's own sections (see above). Needs
  occurrence count from the target document and `idf` from a wider
  comparison scope (project? whole corpus?) — a real design question, not
  a straight reuse of the existing function signature.
- **Does this module take a new dependency on shelling out to `git`?** No
  file today calls `exec.Command` for anything; `git log`/`git blame` for
  `author`/`dateCreated`/`dateModified` would be the first external-process
  dependency in `knowledge`. Needs a decision on whether that's acceptable,
  and what the fallback is when `git` can't answer — an untracked file, or
  `knowledge` running (as an installed binary) against a document outside
  any git checkout at all.
- **Does `dateModified` get the same "never overwrite" treatment as the
  other three fields, or is it the one field this command is allowed to
  keep current on every run?** A stale `dateModified` is arguably worse
  than a missing one, which cuts against the "enhancement only, never
  overwrite" rule stated for `title`/`author`/`dateCreated`. Not decided.
- **Author format.** A bare name string (matching the existing `author`
  field's `string` type), or `Name <email>`? And if a file's git history
  shows more than one author, which wins — first committer, most recent,
  or is a single-string field simply the wrong shape for that case?
- **Where the `title` H1-fallback logic should live.** Does this command
  call `ParseDocumentFile` directly, or does a propose-without-writing read
  path need its own, lighter entry point that doesn't require running the
  full ingest pipeline?

## Related

- `wikilink-tagging-feature-request.md` / `concept-tag-retrieval-feature-request.md`
  / `narrative-documents-feature-request.md` — the concept-linking and
  document-ingestion mechanisms this reuses.
- DR-0027 (`knowledge/decisions/`) — the existing first-H1 title fallback
  and density-linking threshold this proposal generalizes into a
  standalone command.
- DR-0028 (`kb concept suggest`) — the TF-IDF-shaped candidate scoring the
  new-candidate-keyword signal reuses, with the per-document scoping
  question above still open.
- DR-0029 (`kb document tag`) — the propose/accept-shaped, both-or-neither
  write pattern this proposal follows, and the rejected
  frontmatter-`keywords:`-only alternative this proposal is, in effect,
  now building as its own tool rather than as a second tagging mechanism.
- `TODO.md`, "Planned for v0.0.11" — where this item is tracked at the
  release-scope level; this document is the feature-request write-up that
  entry pointed at.
- Filing conversation, 2026-09-19 (Laboratory root).
