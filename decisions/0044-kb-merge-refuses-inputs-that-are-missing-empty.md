---
id: "0044"
title: "kb merge refuses inputs that are missing, empty, not a file, or the same file twice"
date: "2026-09-24"
status: accepted
kind: correction
trigger: live-test
project: knowledge
phase: ""
supersedes: []
superseded_by: []
relates_to: ["0014", "0040", "0041"]
initiative: ""
session: ""
decisions: []
tags: [merge, inputs, validation, v0.0.13]
uuid: "01a0d5ac-46f3-74c4-beaf-2595bd305bfd"
origin_host: "wren"
---

**Context.**

`kb merge -a x -b y -out z` with neither `x` nor `y` present exited 0, printed the
per-table summary, created zero-byte `x` and `y`, and wrote a schema-only `z`. A
mistyped `-a` therefore merged an empty database into the real one and reported
success. The merge is the one verb whose output people copy over each machine's
`agents/knowledge.db` (its own message says so), so a wrong "success" is not
cosmetic.

Cause, confirmed: `checkpointAndCopy` opens each input with `sql.Open` and runs
`PRAGMA wal_checkpoint`, and SQLite creates a file that is not there. Other bad
inputs behaved badly too. A zero-byte file (which is exactly what the bug leaves
behind) merged silently. A directory failed with SQLite's misleading `unable to
open database file: out of memory (14)`. `-a a.db -b a.db`, or the same file under
another spelling, "merged" a database with itself.

**Decision.**

1. Before anything opens an input, `checkMergeInputs` stats both and refuses:
   a missing file (exit 1, naming the flag and the path); a directory or other
   non-regular file; and a zero-byte file. `os.Stat` follows symlinks.
2. `-a` and `-b` naming the same file, by any spelling or symlink (`os.SameFile`),
   is a usage error (exit 2, DR-0040): nothing to merge, and a repeated path is
   almost certainly a typo.
3. A refused merge changes nothing on disk. A test compares the directory listing
   before and after.
4. **A zero-byte main file with a non-empty `-wal` sidecar is not refused.** This
   is a defensive allowance and not an observed case. It assumes a WAL database
   might hold its data only in the sidecar, which merge would checkpoint before
   copying. Probing with the driver in use did not produce it: a WAL database's
   main file is at least one page from the moment WAL mode is set.
5. The checks are in the CLI, next to `checkpointAndCopy`. `MergeKnowledgeBases`
   is unchanged: it receives the already-checkpointed scratch copies (DR-0014) and
   never sees the user's paths.
6. An input that exists but is not a database still fails with SQLite's own `file
   is not a database`, as before.

**Rationale.**

Refusing before opening is the only order that works, because opening is what
creates the file. Refusing the zero-byte file matters because it is the leftover
of the old behaviour: without it, the second mistyped run would succeed against
the artefact of the first.

Item 4 exists because refusing a file that might hold real data is a worse
mistake than accepting an odd one, and the old behaviour accepted everything. It
is kept although unverified, and the comments and this record say so, so a future
reader does not take it for a documented WAL rule.

**Rejected alternatives.**

- Refuse only a missing input. Leaves the zero-byte artefact mergeable.
- Refuse any zero-byte main file, WAL or not. Simpler, and probably equivalent in
  practice, but it rests on the same unverified belief in the other direction.
- Open read-only so SQLite never creates the file. Stops the stray file but not
  the empty merge, and the error would still be SQLite's.
- Validate inside `MergeKnowledgeBases`. It never receives the original paths.
- Treat `-a`/`-b` naming one file as a harmless no-op. It reports a merge that did
  not happen.

**Consequences.**

A `.db` of zero bytes left by an earlier failed merge is now refused and can simply
be deleted. A legitimate merge is unchanged: a real merge of the actual database
with `knowledge.db.pre-merge-20260828` gives an identical 14-table summary and
identical merged content (518 rows compared) from the v0.0.12 binary and the new
one. Six bad-input cases from the survey (a missing `-a`, a missing `-b`, a
directory, an empty file, and the same file twice under two spellings) now fail
cleanly and create nothing; five of the six used to exit 0.

Status `proposed`: promotion is the author's call.
