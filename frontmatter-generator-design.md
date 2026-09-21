# `kb document frontmatter` — Frontmatter Generator — Design

**Status (2026-09-21):** Decisions confirmed, from a design conversation.
Not yet implemented — see `frontmatter-generator-plan.md` (not yet written)
for the phased build.

**References:**
- `frontmatter-generator-feature-request.md` — the filed idea this design
  resolves; its five open questions are answered by decisions 2–4 and 7
  below.
- `TODO.md`, "Planned for v0.0.11," item 2.
- DR-0027 — the existing first-H1 title fallback (`firstH1Heading`,
  `documents.go:353`) this design reuses rather than reimplementing, and
  the frontmatter-`keywords:` mechanism it originally considered and set
  aside for `kb document tag` specifically (not for a standalone tool,
  which is what this is).
- DR-0028 (`kb concept suggest`) — the `occurrences × idf` candidate-concept
  scoring this design adapts for a single-document scope (decision 4).
- DR-0029 (`kb document tag`) — the `--concept`-must-already-exist,
  checked-before-any-file-opens precedent decision 4's keyword-signal-(b)
  handling follows.

## Motivation

Document frontmatter is optional by this module's own design
(`extractFrontmatter`'s "no frontmatter is normal, not an error,"
`documents.go:481-489`), so a real document can be missing a title, an
author, a creation date, or any keywords at all, with nothing today
surfacing that gap or filling it. Several of those fields are mechanically
derivable — from the document's own content (`title`, and, per this
design's refinement, `author` via a byline) or from git/filesystem
provenance (`author`, `dateCreated`, `dateModified`) — without a model call
or a new dependency beyond (deliberately, see decision 3) `git` itself.

## Decisions

1. **Scope: documents only, not decision records.** Records already have
   most metadata tool-filled by `kb record new`/ingest and have no `author`
   field at all — adding one would be its own schema decision, out of scope
   here. Unchanged from the feature request.

2. **A new, single-file verb: `kb document frontmatter PATH [--accept
   FIELD,...] [--accept-keywords NAME,...] [--set FIELD=VALUE] [--dry-run]`.**
   `--set` is repeatable (one `FIELD=VALUE` pair per occurrence), a manual
   override that writes exactly the given value for that field, bypassing
   signal detection entirely — still through the same write path as an
   accepted proposal (decision 5). No `--project`: unlike `tag`/`fuzzy-tag`,
   this isn't a batch operation over a project's documents — a human reviews
   title/author/keyword proposals one file at a time, so a single `PATH`
   argument matches how it's actually used. `--dry-run` still earns its
   place here, unlike `fuzzy-tag` (which dropped it): bare invocation with
   no `--accept`/`--accept-keywords`/`--set` is already read-only by
   construction (nothing accepted, nothing written), but `--dry-run`
   combined *with* an accept/set selection previews the exact value that
   selection would write — useful in particular before accepting a
   signal-(b) keyword, which has the side effect of creating a new concept.

3. **Provenance source: shell out to `git`.** `exec.Command("git", "log",
   ...)` for real per-file commit history — `knowledge`'s first
   external-process dependency (checked: no file anywhere in the module
   calls `exec.Command` today). Falls back to filesystem `mtime`/birth time
   when `git` itself fails (an untracked file, or the binary running
   outside any git checkout) — detected by the command's own failure, not
   by checking for a `.git` directory first, since the file's repository
   root may not be the process's working directory.

4. **Field-by-field behavior:**
   - **`title`** — absent only; proposed from `firstH1Heading`
     (`documents.go:353`), the exact function DR-0027 already uses at
     ingest time, called here for a command a human runs deliberately
     instead.
   - **`author`** — absent only; a layered signal, not a single source,
     because git's own commit-author string is often *not* the form a human
     wants cited (a raw email/handle, or simply whatever `user.name` was
     configured at commit time, which may predate a preferred citation
     form):
     1. **A byline in the document's own prose** wins when present — scan
        the text immediately following the first H1 (before the next
        paragraph break or heading) for one of a small, deliberate set of
        lead-ins: `By `, `Author: `, `Written by `. Highest confidence,
        since it's the author's own chosen wording, and mirrors the same
        "trust the document's own structure" philosophy `firstH1Heading`
        already uses for `title`.
     2. **git's earliest-commit author** (`git log --follow --format=%an
        --diff-filter=A` or equivalent, exact form to confirm during
        implementation) as fallback — original authorship, not "whoever
        touched it last."
     3. **`git config user.name`** as a last resort, for an untracked file
        with no commit history at all.

     When more than one signal is available and they disagree, the report
     shows all of them, not just the winner — e.g. `author: missing.
     byline found: "R. S. Doiel". git author: "rsdoiel" (lower confidence).
     proposing byline value.` — so a human can judge the mismatch rather
     than have it hidden behind a single guess. `author` stays a bare
     string (`documentFrontmatter.Author`'s existing type, no schema
     change); a file with more than one git committer just takes the
     earliest, no multi-value list.
   - **`dateCreated`** — absent only; git's first commit touching the file,
     filesystem birth time (or `ctime` where birth time isn't available) as
     fallback.
   - **`dateModified`** — **new field**, added to `documentFrontmatter`
     (decision 5) — does not exist in the schema today even though the
     antennaApp vocabulary this struct mirrors treats it as distinct from
     `datePublished`/`pubDate`. **Does not follow the "absent only" rule the
     other three fields do** — refreshed to the proposed value on every
     accepted run regardless of whatever value (if any) was already there.
     A stale `dateModified` is worse than a missing one; the other three
     are provenance facts that shouldn't change once true, but this one is
     inherently a live field. Source: git's most recent commit touching the
     file, filesystem `mtime` as fallback.
   - **`keywords`** — a diff against the current `keywords:` list, not a
     binary, over two candidate sets:
     - **(a) Known concepts** mentioned in the document's own text but not
       yet listed — reuses `eligibleConcepts`/`densityLinkCandidates`
       (`cmd/kb/documenttag.go:198`, `cmd/kb/document.go:393`) as-is, the
       same `>1`-occurrence, code-span-excluded threshold `tag` and
       `fuzzy-tag` both already apply. Accepting one is a plain write.
     - **(b) New candidate concepts** distinctive to this document
       specifically, not yet known to the corpus at all — a
       single-document-scoped variant of `cmdConceptSuggest`'s
       (`cmd/kb/concept.go:225`) `occurrences × idf` scoring. The feature
       request flagged a real degeneracy here: naively scoping the whole
       calculation to just the target document makes `df == N` trivially,
       collapsing every term's `idf` to `0`. **Resolved**: count
       `occurrences` from the target document only, but compute `idf` from
       the comparison scope with the target document *excluded* — the
       project's other documents/records if the file belongs to one,
       otherwise the whole corpus minus the target. Excluding the target
       from its own comparison set is what restores genuine corpus-wide
       rarity as the signal. Accepting a signal-(b) keyword **explicitly
       creates the concept first** (the same effect as `kb concept add
       NAME`), mirroring `kb document tag --concept`'s "must already exist,
       checked before any file opens" rule (`cmd/kb/documenttag.go:256`) —
       never left as a bare string for `ResolveConceptName` to
       silently resolve-or-create at the next `kb document ingest`.

5. **Write mechanism: a surgical edit of just the frontmatter block, via
   `yaml.Node` — not `documentFrontmatter` (the struct) and not a full-file
   rewrite.** Two separate reasons rule out the obvious-looking
   unmarshal-into-`documentFrontmatter`-then-remarshal approach:
   - `documentFrontmatter`'s own doc comment states its decode is
     "tolerant... unrecognized keys ignored" (`documents.go:467-479`) —
     correct for read-only extraction at ingest, but round-tripping through
     it on a *write* would silently **drop any frontmatter key the struct
     doesn't know about** — a real data-loss risk for a document carrying
     a custom field this module never asked it to.
   - Documents have no `RenderDocumentFile` — no lossless way to turn
     parsed/segmented state back into a file (the same gap
     `fuzzy-concept-matching-design.md`'s decision 7 already found and
     avoided by operating on raw file text instead). A full-document
     parse-mutate-render cycle isn't available here either.

   `yaml.Node` (already available via `gopkg.in/yaml.v3`, already a
   dependency) reads and edits only the mapping between the `---`
   delimiters `splitFrontmatter` (`recordfile.go:401`) locates, in place:
   find-or-append the specific keys being written, leave every other key,
   value, and comment in the block untouched, then splice the re-encoded
   block back between the original delimiters. Everything outside the
   frontmatter block — the whole document body — is never read into memory
   as anything other than the raw bytes on either side of it. **For a
   document with no frontmatter block at all** (`splitFrontmatter` errors,
   the ordinary case per `extractFrontmatter`'s own design decision 3): a
   fresh block is constructed containing only the fields being written this
   run, and prepended to the file's existing raw content.

   A bounded cost accepted here, revisit only if it proves to matter: a
   `yaml.Node` edit still can't guarantee `keywords`'s existing `flow`-style
   formatting (`yaml:"keywords,flow"`) survives if that list is one of the
   fields being touched — a block-style re-render there is a cosmetic
   difference, not a correctness one.

6. **Never overwrites an existing value** for `title`/`author`/
   `dateCreated` — enhancement only, consistent with document frontmatter
   being optional by this module's design. `dateModified` is the one
   exception (decision 4).

7. **Body text is never modified**, matching the same rule `tag`/
   `fuzzy-tag` already hold to. A detected byline only feeds the `author`
   proposal — the byline sentence itself is left exactly as written, even
   after its content is copied into frontmatter. This does mean the same
   information can legitimately appear in both places at once; that's
   accepted, not a bug to fix.

8. **No statistical or model-based authorship detection.** Considered and
   declined in the design conversation: the layered byline → git ->
   `git config` signal (decision 4) is fully deterministic, needs no new
   dependency beyond `git` itself, and `--set` (decision 2) already covers
   the case where none of the three signals produces the citation form a
   human actually wants. Worth reopening only if the deterministic layers
   prove insufficient in practice, not assumed as a gap to fill now.

9. **Single-file scope makes "both-or-neither" trivial** — unlike `tag`/
   `fuzzy-tag`'s multi-file runs, there's exactly one file to write, so the
   existing staged-write-then-rollback-on-any-failure pattern collapses to
   an ordinary atomic single-file replace (write to a temp file in the same
   directory, then rename over the original) rather than needing the
   multi-file bookkeeping `cmdDocumentTag` has.

## Deferred, explicitly

- **The exact `git log` invocation and flags** for earliest/latest-commit
  author and date (decision 3/4) — a real command to pin down during
  implementation, not a design-level concern.
- **`yaml.Node` implementation detail** (decision 5) — the plan's job, not
  this document's.
- **Record frontmatter** (decision 1) — explicitly out of scope, unchanged
  from the feature request.
- **Statistical/ML authorship detection** (decision 8) — declined for now,
  not ruled out forever.
