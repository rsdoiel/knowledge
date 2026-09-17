---
id: "0024"
title: "kb project rename and kb concept rename, refusing when a corpus exists"
date: "2026-09-17"
status: accepted
kind: decision
trigger: request
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: ["kb project rename OLD NEW renames the row and refreshes its kb_fts entry via the existing refreshProjectFTS, but refuses outright when the project owns any records, since a corpus's frontmatter project: field would desync from the database", "kb concept rename OLD NEW ships in the same pass: concepts carry no on-disk file whose content must track the name, so it has no corpus risk and no refuse condition beyond a name collision", "Corrects TODO.md's diagnosis: set-description's reindex does not cause the kb_fts duplicate it was blamed for -- refreshProjectFTS deletes by source_id, not by name, and a live repro confirms it repairs a stale label correctly. The real duplicate comes from re-ingesting a renamed project's corpus, which silently mints a phantom project row and a duplicate record row -- verified live, not merely asserted", "Rewriting a corpus's frontmatter automatically is out of scope for this pass; refusing is the safe default until that lands"]
tags: [request, projects, concepts, fts, data-integrity]
uuid: "01a0b118-3358-79d0-878b-4ca737de062f"
origin_host: "wren"
---

**Context.** `TODO.md` filed this 2026-09-15: there is no `kb project rename`,
so the only route is SQL against `projects.name`, and the note claimed
`set-description`'s reindex then "deletes the `kb_fts` row by the *new*
name, so the row carrying the old name survives" — a project indexed twice
under both names. That specific claim does not survive a repro against the
current code. `refreshProjectFTS` deletes by `source_id = ?`, the stable
row id, not by name:

```go
_, _ = kb.db.Exec(
    `DELETE FROM kb_fts WHERE source_type = 'project' AND source_id = ?`, id)
```

Reproduced live: `project add caltechcampuspubs_static`, raw
`UPDATE projects SET name = 'ep3StaticSite' ...`, then
`project set-description ep3StaticSite "..."` — `kb_fts` ends with exactly
one row, `source_id=1`, `label=ep3StaticSite`. `set-description` repairs the
stale label rather than orphaning it.

**The real duplicate comes from a different, equally natural mistake:
using `project add NEWNAME` to "rename."** `ON CONFLICT(name)` only fires
when the name already exists, so `add`ing a name that doesn't match anything
inserts a genuinely new row with its own id and its own `kb_fts` entry.
Reproduced live: `project add caltechcampuspubs_static` then
`project add ep3StaticSite` leaves two real rows in both `projects` and
`kb_fts` (ids 1 and 2), each independently valid and independently
searchable — not a stale label, two live projects for one real thing, and
whichever one nobody remembers to use again quietly stops accumulating
history.

**The harder failure is real and worse than the note described: it corrupts
data, not just search.** A decision record's `project:` frontmatter has to
match `projects.name`, because `kb ingest` resolves a record's project by
reading that field and calling `ProjectByName` on every run — not once, at
authoring time. Reproduced live: a project with one ingested record, renamed
via raw SQL, then the *same corpus* re-ingested without touching the files.
`ing.projectID("oldname")` finds no match, so it *silently creates a new
project row* named `oldname`; `RecordByIdentity` then finds no existing row
under that new id, so the record is inserted *again* — a second `records`
row, same `record_id` and `path`, attached to the phantom project. The
original row stays behind, orphaned under the renamed project, permanently
invisible to future ingests of that corpus:

```
projects: 1|newname   2|oldname
records:  1|1|0001|agents/projects/oldname/decisions/0001-fixture.md
          2|2|0001|agents/projects/oldname/decisions/0001-fixture.md
```

This is reachable today by the *correct-looking* raw-SQL rename, the moment
anyone re-ingests. There is no safe way to rename a project with a corpus
under the current tool.

**Documents share a narrower version of the risk, not blocking.**
`kb document ingest PATH --project NAME` also resolves by name each run, but
the name comes from the command line the caller types, not from a stored
file attribute re-read automatically the way a record's `project:`
frontmatter is. Renaming doesn't desync anything already on disk; it only
bites if a script or habit keeps passing the old `--project` value.

**Concepts carry none of this risk.** Every reference to a concept —
`record_concepts`, `observation_concepts`, `project_concepts`,
`document_section_concepts` — is a foreign key to `concepts.id`, never a
name matched from an external file. `kb concept add` already upserts by
name (DR-0012), so a concept rename has no corpus to desync and no ingest
path to re-derive an identity from.

**Decision.** `kb project rename OLD NEW`: resolves `OLD`, refuses if `NEW`
already exists (`UNIQUE` would refuse anyway; this gives a clear error
instead of a raw constraint failure), refuses outright if the project owns
any records (`SELECT COUNT(*) FROM records WHERE project_id = ?`, naming the
count and that its corpus's `project:` frontmatter would desync), otherwise
updates `projects.name` and calls the existing `refreshProjectFTS` — the
same repair path `set-description` already uses, now reached by the verb
that should have existed instead of a raw `UPDATE`. `kb concept rename
OLD NEW` ships in the same pass: resolves `OLD`, refuses if `NEW` already
exists, otherwise updates `concepts.name` and refreshes its `kb_fts` row via
a new `refreshConceptFTS`, factored out of `AddConceptWithIdentifier`'s
inline delete-then-insert the way `refreshProjectFTS` was factored out for
projects. Neither verb touches `updated_at` reconciliation across machines —
same scope boundary DR-0012 drew for `set-description`, unrelated to this
change.

**Rationale.** Refusing when a project owns records is the only option that
doesn't risk the corruption just demonstrated; rewriting every record file's
frontmatter automatically is the real fix but is materially more work — the
same both-or-neither write `record supersede` already does across files and
database, generalized to an arbitrary-sized corpus — and shipping a refusal
today is strictly better than today's actual state, which is a raw SQL path
with no refusal and no warning at all. Concept rename costs little enough to
include now rather than file separately: no corpus, no refuse condition
beyond the name collision every rename needs anyway, and leaving it out
would mean a typo'd concept name stays fixable only by hand SQL while a
typo'd project name has a real verb, an asymmetry with no justification once
the harder case is solved. Reusing `refreshProjectFTS` rather than writing
new FTS logic means rename inherits a repair path already covered by tests
and already proven correct in this record's own repro.

**Rejected alternatives.** Rewriting a corpus's `project:` frontmatter
automatically as part of this pass — the complete fix, deferred because it's
a distinctly bigger change (walking a corpus, writing N files, and deciding
what happens to a file `kb rename` can't write, e.g. permissions or a
dirty working tree) that deserves its own record rather than riding along.
Allowing rename on a project with records, with a warning instead of a
refusal — rejected because the corruption is not hypothetical; the repro
above shows it happening on the very next `ingest`, and a warning a script
doesn't read is not a safeguard. Skipping `kb concept rename` to keep this
record narrowly about the corpus-desync problem — rejected because concept
rename has no corpus problem to be narrow about; deferring it buys no
safety, only asymmetry. Blocking rename when a project has documents, the
same way records block it — rejected: documents have no stored attribute
that desyncs on rename, only a command-line flag a caller might mistype,
which is a usage error each time it happens rather than a standing landmine.

**Consequences.** Implementation: `RenameProject(old, new)` and
`RenameConcept(old, new)` on `*KnowledgeBase`, the former checking
`records` ownership before writing; `refreshConceptFTS` factored out of
`AddConceptWithIdentifier`; `cmdProjectRename`/`cmdConceptRename` dispatched
alongside the existing subcommands in `cmd/kb/project.go`/`cmd/kb/concept.go`;
help text and `kb-project(1)`/`kb-concept(1)` regenerated; `TODO.md`'s
project-rename item corrected (the `set-description` diagnosis was wrong)
and closed, noting rewriting a corpus's frontmatter automatically as the
follow-on this record explicitly deferred.
