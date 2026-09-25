# scripts

## compare-exit-codes.py

Compares two `kb` binaries, an older release and the build under test, on a copy of a
real workspace. It is the check DR-0040 used for the exit-code sweep and DR-0049 says is
worth keeping: a release that changes what `kb` says to a script should be run against
the last release, on real data, with a wide set of failures, before it is tagged.

```bash
go build -o bin/kb ./cmd/kb
# an older release, however you have it (a release zip, or built from a tag):
uv run scripts/compare-exit-codes.py --base ~/bin/kb-v0.0.13 --new bin/kb --workspace ~/Laboratory
```

What it does, and what to read in its report:

- Every command runs with each binary, in plain and `--json` mode, on a fresh copy of a
  template workspace, so no run sees another's changes and the real workspace is never
  touched (the script checks the real `knowledge.db` did not change and says so if it did).
- The template holds the workspace's `agents/knowledge.db`, `agents/knowledge.jsonl`,
  every `decisions/` directory, the first ingested document, and a few bad inputs. Names
  (a project, a concept with links, a record, a source) are discovered from the database;
  a command that needs a name the database does not have is skipped and listed.
- The commands are every path in the new binary's SYNOPSIS (path only, a bogus flag first
  and last, minimal, surplus arguments), read-only commands on the real data, writes on
  the real data, and provoked failures, one or more per exit class.
- The report lists each command whose exit code changed (old to new), then every difference
  in stdout or stderr that a changed code does not explain, then differences that appear
  only in `--json` mode, then any exit 70 (an error nothing classified) or hang in the new
  binary. The script exits 1 when there is a 70 or a hang, so it can gate a release.
- **Reading the changed codes against the decision records is your job.** Nothing here
  decides a change is right. Expect version and hash lines to differ, and check every other
  difference is one a decision record calls for.

Options: `--work DIR` keeps the scratch files (the Laboratory convention is `~/Laboratory/tmp`),
`--results FILE` writes the raw results as JSON, `--timeout N` changes the per-command limit.
Standard library only; run it with `uv run`.

**One pitfall.** The workspace name is the base name of the root directory, and the records
were ingested under it. The script names its run directory after the workspace for that
reason. Run against a directory with another name, `kb ingest` reports a `UNIQUE constraint
failed: records.uuid` for every record, and that failure is not a bug in the code under test.
