# /// script
# requires-python = ">=3.10"
# ///
"""Compare two kb binaries on a copy of a real workspace (the DR-0040 method).

    uv run scripts/compare-exit-codes.py --base kb-v0.0.13 --new bin/kb \\
        --workspace ~/Laboratory

Every command runs with each binary, in plain and --json mode, on a fresh copy
of a template workspace, so no run sees another's changes and the real
workspace is never touched. The template holds the workspace's knowledge.db,
knowledge.jsonl, every decisions directory, the first ingested document, and a
few bad inputs. Names (a project, a concept with links, a record, a source, an
observation) are discovered from the database, so it works on any workspace.

The commands come from four groups: every command path in the new binary's
SYNOPSIS (path only, a bogus flag first and last, minimal, surplus arguments);
read-only commands on the real data; writes on the real data; and provoked
failures, one or more per exit class.

The report lists every command whose exit code changed, every difference in
stdout or stderr that a changed code does not explain, and any exit 70 (an
error nothing classified) or hang in the new binary. The script exits 1 when
the new binary produced a 70 or a timeout, so it can gate a release. Reading the
changed codes against the decision records is the reviewer's job: nothing here
decides that a change is right.

One pitfall: the workspace name is the base name of the root directory, and the
records were ingested under it. The run directory is therefore named after the
workspace, or `kb ingest` reports a spurious UNIQUE failure for every record.
"""

import argparse
import collections
import json
import os
import re
import shlex
import shutil
import sqlite3
import subprocess
import sys
import tempfile

PLACEHOLDER = re.compile(r"\{(\w+)\}")
UUID = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")
TIMESTAMP = re.compile(r"\d{4}-\d\d-\d\d[T ]\d\d:\d\d:\d\d(Z|\.\d+Z?)?")
VERBS = "project observation concept link source search merge export import ingest record document index init".split()
FREE_TEXT = ("BODY", "DESCRIPTION", "NOTE", "TERM")


# ─── commands ────────────────────────────────────────────────────────────────


def synopsis_lines(kb):
    lines = []
    for verb in VERBS:
        out = subprocess.run([kb, verb, "-help"], capture_output=True, text=True).stdout
        inside = False
        for line in out.splitlines():
            if line.startswith("# SYNOPSIS"):
                inside = True
            elif line.startswith("# ") and inside:
                inside = False
            elif inside and line.startswith("kb "):
                lines.append(line)
    return list(dict.fromkeys(lines))


def parse_synopsis(line):
    """Return (words, minimal args, ends in free text) for one SYNOPSIS line."""
    fields = line[3:].split()
    words = []
    while fields and re.fullmatch(r"[a-z][a-z-]*", fields[0]):
        words.append(fields.pop(0))
    depth, free, tokens = 0, False, []
    in_group, group_done = False, False
    for tok in fields:
        if tok.startswith("(") and depth == 0:
            in_group, group_done = True, False
            tok = tok[1:]
        clean = tok.strip("[]()")
        if clean == "|":
            group_done = True
        if re.fullmatch(r"[A-Z][A-Z_]*(\.\.\.)?", clean or "x") and (
            clean.rstrip(".") in FREE_TEXT or clean.endswith("...")
        ):
            free = True
        opens, closes = tok.count("["), tok.count("]")
        if depth == 0 and opens == 0 and (not in_group or not group_done) and clean and clean != "|":
            tokens.append(clean)
        depth += opens - closes
        if in_group and tok.endswith(")"):
            in_group = False
    args, i = [], 0
    while i < len(tokens):
        tok = tokens[i]
        if tok.startswith("-"):
            args.append(tok)
            if i + 1 < len(tokens) and re.fullmatch(r"[A-Z][A-Z_]*", tokens[i + 1]):
                args.append("x" + tok.lstrip("-"))
                i += 1
        elif re.fullmatch(r"[A-Z][A-Z_]*(\.\.\.)?", tok):
            name = tok.rstrip(".")
            args.append("1" if name in ("ID", "OBS_ID", "SOURCE_ID", "SECTION_ID", "RECORD_ID") else "x" + name.lower())
        i += 1
    return words, args, free


def discover(db_path, workspace):
    """Real names from the database, for the commands that need one."""
    d = {}
    con = sqlite3.connect(f"file:{db_path}?mode=ro", uri=True)

    def one(sql, *a):
        try:
            row = con.execute(sql, a).fetchone()
        except sqlite3.Error:
            return None
        return row[0] if row else None

    projects = [r[0] for r in con.execute(
        "SELECT p.name FROM projects p LEFT JOIN observations o ON o.project_id = p.id "
        "GROUP BY p.id ORDER BY COUNT(o.id) DESC, p.id")]
    d["proj"] = projects[0] if projects else None
    d["proj2"] = projects[1] if len(projects) > 1 else None
    concepts = [r[0] for r in con.execute(
        "SELECT c.name FROM concepts c JOIN observation_concepts oc ON oc.concept_id = c.id "
        "GROUP BY c.id ORDER BY COUNT(*) DESC, c.id LIMIT 2")]
    d["concept"] = concepts[0] if concepts else one("SELECT name FROM concepts ORDER BY id")
    d["concept2"] = concepts[1] if len(concepts) > 1 else one("SELECT name FROM concepts WHERE name != ? ORDER BY id", d["concept"])
    d["obs"] = one("SELECT MIN(id) FROM observations")
    d["obs_max"] = one("SELECT MAX(id) FROM observations")
    d["src_linked"] = one("SELECT source_id FROM observation_sources LIMIT 1")
    d["src"] = one("SELECT MIN(id) FROM sources")
    row = con.execute(
        "SELECT r.record_id, p.name FROM records r JOIN projects p ON p.id = r.project_id "
        "ORDER BY r.id LIMIT 1").fetchone()
    d["rec"], d["rec_proj"] = row if row else (None, None)
    d["doc_path"] = one("SELECT path FROM documents ORDER BY id")
    d["section"] = one("SELECT MIN(id) FROM document_sections")
    con.close()
    d["decisions"] = sorted(
        os.path.relpath(os.path.join(root, "decisions"), workspace)
        for root, dirs, _ in os.walk(workspace)
        if "decisions" in dirs and root.count(os.sep) - workspace.count(os.sep) <= 2
        and "/.git" not in root and "/dist" not in root)
    d["dec"] = d["decisions"][0] if d["decisions"] else None
    return d


def commands(kb_new, names):
    out = []

    def add(name, args, cwd=".", db=None):
        out.append({"name": name, "args": args, "cwd": cwd, "db": db})

    for line in synopsis_lines(kb_new):
        words, args, free = parse_synopsis(line)
        key = " ".join(words)
        add(f"[A] {key} (path only)", words)
        add(f"[A] {key} (bogus flag first)", words + ["--bogus-flag-zzz"] + args)
        add(f"[A] {key} (minimal)", words + args)
        if not free:
            add(f"[A] {key} (bogus flag last)", words + args + ["--bogus-flag-zzz"])
            add(f"[A] {key} (surplus x2)", words + args + ["surplus-zzz", "surplus2-zzz"])

    # A command is generated only when every name it needs was found.
    def template(group, spec):
        for text in spec:
            needed = PLACEHOLDER.findall(text)
            if any(names.get(n) in (None, "") for n in needed):
                continue
            filled = PLACEHOLDER.sub(lambda m: shlex.quote(str(names[m.group(1)])), text)
            add(f"[{group}] {text}", shlex.split(filled))

    template("B", """project list|project show {proj}|project concepts {proj}|observation list --project {proj}|observation show {obs}|observation sources {obs}|concept list|source list|source show {src}|record list|record list --project {rec_proj}|record list --status accepted|record list --since 2026-01|record show {rec} --project {rec_proj}|record concepts {rec} --project {rec_proj}|document list|document show 1|document review list|concept suggest --project {proj}|concept suggest --limit 5|search {concept}|search zzznotfoundzzz|summary|format|format --project {proj}|export|export -project {proj}|index {dec} --check|index {dec} --stdout|index . --all --check|ingest {dec} --dry-run|record fmt {dec} --dry-run|document tag --project {proj} --dry-run|document fuzzy-tag --project {proj} --dry-run|source check-retractions|-version|-license|help|help project|project -help""".split("|"))
    template("B", ["document frontmatter {doc_path}", "document ingest {doc_path} --project {proj} --dry-run"])
    template("C", """project add T1 desc|observation add --project {proj} note body|concept add TestC|source add T|link project {proj} {concept}|record new --project {rec_proj} --title T --trigger design --dir newdir|record set-status {rec} accepted --project {rec_proj}|import -in agents/knowledge.jsonl|export -out out.jsonl|merge -a agents/knowledge.db -b b.db -out m.db|ingest {dec}|index {dec}|project rename {proj} renamed-zzz --dry-run|concept delete NoSuchConceptZ --dry-run""".split("|"))
    template("C", ["document ingest doc.md --project {proj}"])
    template("D", """project show nosuch|project rename nosuch x|project rename {proj} {proj2}|project set-status nosuch active|project set-status {proj} bogus|observation add --project {proj} bogus b|observation add --project nosuch note b|concept delete nosuch|concept delete {concept}|concept rename nosuch x|concept rename {concept} {concept2}|concept rename {concept} {concept}|source remove 99999|source retract 99999 n|source remove {src_linked}|source link 99999 99999|link project {proj} nosuch|link observation 99999 {concept}|record show 9999 --project {rec_proj}|record list --status acepted|record list --project nosuch|record list --kind bogus|record list --since 2026-13|record set-status {rec} bogus --project {rec_proj}|record set-status {rec} '' --project {rec_proj}|record supersede {rec} {rec} --project {rec_proj}|record new --workspace --title T --trigger bogus --dir newdir|record fmt nosuchdir|document show 99999|document review promote 99999|document review promote {section}|document review list --status bogus|export -project nosuch|ingest /nonexistent|ingest badrecs|ingest {dec} --root nosuchroot|ingest a b|index badrecs|index /nonexistent|index {dec} --stdout --check|import -in bad.jsonl|import -in nonexist.jsonl|import -in agents|merge -a nope.db -b nope2.db -out o.db|merge -a zero.db -b b.db -out o.db|merge -a agents/knowledge.db -b b.db -out b.db|merge -a b.db -b b.db -out o.db|document ingest a.pdf --project {proj}|concept suggest --limit -1|concept suggest --limit abc|concept suggest --project nosuch|search --json foo|init --bogus|init a.pdf/sub|export -out /nonexistent/dir/o.jsonl|export -out agents|project add ''|concept add ''|source add ''|observation add --project {proj} note ''|source add T --published notadate|source add T --doi zzz|document tag --project {proj} --concept zzznope|document frontmatter doc.md --accept-keywords zzznotaconcept""".split("|"))
    add("[D] --db is a text file: project list", ["project", "list"], db="bad.db")
    add("[D] no workspace here: project list", ["project", "list"], cwd="empty")
    return out


# ─── running ─────────────────────────────────────────────────────────────────


def build_template(workspace, template):
    for sub in ("agents", "empty", "badrecs", "newdir"):
        os.makedirs(os.path.join(template, sub), exist_ok=True)
    shutil.copy(os.path.join(workspace, "agents", "knowledge.db"), os.path.join(template, "agents"))
    jsonl = os.path.join(workspace, "agents", "knowledge.jsonl")
    if os.path.exists(jsonl):
        shutil.copy(jsonl, os.path.join(template, "agents"))
    else:
        open(os.path.join(template, "agents", "knowledge.jsonl"), "w").close()
    for sidecar in ("knowledge.db-wal", "knowledge.db-shm"):
        path = os.path.join(template, "agents", sidecar)
        if os.path.exists(path):
            os.remove(path)
    shutil.copy(os.path.join(template, "agents", "knowledge.db"), os.path.join(template, "b.db"))
    names = discover(os.path.join(template, "agents", "knowledge.db"), workspace)
    for rel in names["decisions"]:
        shutil.copytree(os.path.join(workspace, rel), os.path.join(template, rel), dirs_exist_ok=True)
    if names["doc_path"]:
        src = os.path.join(workspace, names["doc_path"])
        if os.path.exists(src):
            os.makedirs(os.path.dirname(os.path.join(template, names["doc_path"])) or template, exist_ok=True)
            shutil.copy(src, os.path.join(template, names["doc_path"]))
    fixtures = {
        "bad.db": "not a database, just text long enough to be read as a sqlite header....\n",
        "zero.db": "", "bad.jsonl": '{"type": \n', "a.pdf": "%PDF-1.4\n", "doc.md": "# T\n\nbody\n",
        "badrecs/0002-bad.md": '---\nid: "0002"\ntitle: ""\n---\nbroken\n',
        "badrecs/0003-none.md": "no frontmatter\n",
    }
    for rel, text in fixtures.items():
        with open(os.path.join(template, rel), "w") as f:
            f.write(text)
    return names


def normalise(text, root):
    text = text.replace(root, "W")
    text = UUID.sub("UUID", text)
    text = TIMESTAMP.sub("TS", text)
    text = re.sub(r'\n\s*"class": "[a-z_]+",', "", text)
    text = re.sub(r'\n\s*"code": \d+\n', "\n", text)
    return re.sub(r",\n(\s*)\}", r"\n\1}", text)


def run_one(kb, cmd, json_mode, template, work, workspace_name, timeout):
    base = os.path.join(work, "run")
    shutil.rmtree(base, ignore_errors=True)
    root = os.path.join(base, workspace_name)
    shutil.copytree(template, root)
    args = [kb] + (["--json"] if json_mode else []) + (["--db", cmd["db"]] if cmd["db"] else []) + cmd["args"]
    try:
        p = subprocess.run(args, cwd=os.path.join(root, cmd["cwd"]), capture_output=True, text=True,
                           stdin=subprocess.DEVNULL, timeout=timeout)
        return p.returncode, normalise(p.stdout, root), normalise(p.stderr, root)
    except subprocess.TimeoutExpired:
        return -1, "", "TIMEOUT"


# ─── report ──────────────────────────────────────────────────────────────────


def report(results):
    plain = {r["name"]: r for r in results if not r["json"]}
    js = {r["name"]: r for r in results if r["json"]}
    identical = sum(1 for r in plain.values() if r["old"] == r["new"])
    print(f"commands: {len(plain)}   identical in plain mode: {identical}   "
          f"identical in --json mode: {sum(1 for r in js.values() if r['old'] == r['new'])}")
    changed = [(n, r) for n, r in plain.items() if r["old"][0] != r["new"][0]]
    print(f"\n== exit code changed (plain): {len(changed)}")
    for n, r in changed:
        note = "" if (js[n]["old"][0], js[n]["new"][0]) == (r["old"][0], r["new"][0]) else \
            f"   (--json: {js[n]['old'][0]} -> {js[n]['new'][0]})"
        print(f"  {r['old'][0]:>3} -> {r['new'][0]:>3}  {n}{note}")
    print("\n== transitions:")
    for (a, b), c in sorted(collections.Counter((r["old"][0], r["new"][0]) for _, r in changed).items()):
        print(f"  {a:>3} -> {b:>3}: {c}")
    same = [(n, r) for n, r in plain.items() if r["old"][0] == r["new"][0]]
    out_diff = [(n, r) for n, r in same if r["old"][1] != r["new"][1]]
    err_diff = [(n, r) for n, r in same if r["old"][1] == r["new"][1] and r["old"][2] != r["new"][2]]
    for title, rows, idx in (("stdout differs", out_diff, 1), ("only stderr differs", err_diff, 2)):
        print(f"\n== same exit code, {title}: {len(rows)}")
        for n, r in rows[:80]:
            print(f"  {n}\n     old: {r['old'][idx][:140]!r}\n     new: {r['new'][idx][:140]!r}")
    jonly = [n for n in plain if plain[n]["old"] == plain[n]["new"] and js[n]["old"] != js[n]["new"]]
    print(f"\n== differs only in --json mode: {len(jonly)}")
    for n in jonly[:40]:
        r = js[n]
        print(f"  {n}\n     old: {r['old'][0]} {r['old'][1][:80]!r} {r['old'][2][:80]!r}\n"
              f"     new: {r['new'][0]} {r['new'][1][:80]!r} {r['new'][2][:80]!r}")
    bad = [(n, r["new"][0]) for n, r in list(plain.items()) + list(js.items()) if r["new"][0] in (70, -1)]
    print("\n== 70 (unclassified error) or timeout in the new binary:")
    print("  none" if not bad else "\n".join(f"  {code}  {n}" for n, code in bad))
    return 1 if bad else 0


def main():
    ap = argparse.ArgumentParser(description="Compare two kb binaries on a copy of a real workspace.")
    ap.add_argument("--base", required=True, help="the older kb binary (for example a release build)")
    ap.add_argument("--new", required=True, help="the kb binary under test")
    ap.add_argument("--workspace", required=True, help="a workspace with agents/knowledge.db (read only)")
    ap.add_argument("--work", help="scratch directory (default: a temporary one, removed afterwards)")
    ap.add_argument("--timeout", type=int, default=30, help="seconds per command (default 30)")
    ap.add_argument("--results", help="write the raw results as JSON to this file")
    a = ap.parse_args()

    workspace = os.path.abspath(os.path.expanduser(a.workspace))
    db = os.path.join(workspace, "agents", "knowledge.db")
    if not os.path.exists(db):
        sys.exit(f"no {db}")
    before = os.stat(db).st_mtime_ns, os.path.getsize(db)
    base, new = os.path.abspath(a.base), os.path.abspath(a.new)
    work = os.path.abspath(a.work) if a.work else tempfile.mkdtemp(prefix="kb-compare-")
    os.makedirs(work, exist_ok=True)
    template = os.path.join(work, "template")
    shutil.rmtree(template, ignore_errors=True)
    names = build_template(workspace, template)
    cmds = commands(new, names)
    skipped = [k for k, v in names.items() if v in (None, "", [])]
    print(f"workspace {workspace}\ncommands: {len(cmds)}   names not found (their commands are skipped): {skipped or 'none'}\n")

    wname = os.path.basename(workspace.rstrip(os.sep))
    results = []
    for cmd in cmds:
        for jm in (False, True):
            results.append({"name": cmd["name"], "json": jm, "args": cmd["args"],
                            "old": run_one(base, cmd, jm, template, work, wname, a.timeout),
                            "new": run_one(new, cmd, jm, template, work, wname, a.timeout)})
    if a.results:
        with open(a.results, "w") as f:
            json.dump(results, f)
    code = report(results)
    if (os.stat(db).st_mtime_ns, os.path.getsize(db)) != before:
        print("\nWARNING: the real knowledge.db changed during the run; that should never happen.")
        code = 1
    if not a.work:
        shutil.rmtree(work, ignore_errors=True)
    return code


if __name__ == "__main__":
    sys.exit(main())
