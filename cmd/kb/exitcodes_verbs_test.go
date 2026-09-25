package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// X2 of exit-codes-plan.md: every place kb can fail is classified (workspace
// DR-0003, knowledge DR-0047). This table provokes one failure of each class
// per verb that can produce it, on a scratch database, and asserts the exit
// code and the class name in the JSON error. It also asserts that nothing falls
// through to 70: an error nobody classified is a gap, and here it is a failing
// test rather than a silent exit 1.

// verbCase is one provoked failure. setup runs first, each entry a kb command
// that must succeed; files creates fixture files in the scratch directory.
type verbCase struct {
	name  string
	setup [][]string
	files map[string]string
	args  []string
	want  exitClass
	// noDB runs the command without --db, from an empty directory.
	noDB bool
}

func runVerbCase(t *testing.T, tc verbCase) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	for name, body := range tc.files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := []string{}
	if !tc.noDB {
		base = []string{"--db", filepath.Join(dir, "kb.db")}
		for _, s := range tc.setup {
			var out, errOut bytes.Buffer
			full := append(append([]string{}, base...), s...)
			if len(s) > 0 && s[0] == "--db" { // a setup command naming its own database
				full = s
			}
			if code := mainRun(full, &out, &errOut); code != 0 {
				t.Fatalf("setup %v: exit %d: %s", s, code, errOut.String())
			}
		}
	}
	// merge, index and init never open the ambient database and refuse --db.
	if len(tc.args) > 0 && (tc.args[0] == "merge" || tc.args[0] == "index" || tc.args[0] == "init") {
		base = nil
	}
	var out, errOut bytes.Buffer
	args := append(append([]string{"--json"}, base...), tc.args...)
	code := mainRun(args, &out, &errOut)

	if code != tc.want.Code {
		t.Errorf("kb %s: exit %d, want %d (%s); stderr: %.200s", strings.Join(tc.args, " "), code, tc.want.Code, tc.want.Name, errOut.String())
	}
	if code == classInternal.Code {
		t.Errorf("kb %s: exit 70, an error nothing classified: %.200s", strings.Join(tc.args, " "), errOut.String())
	}
	if s := strings.TrimSpace(errOut.String()); strings.HasPrefix(s, "{") {
		var env struct {
			Class string `json:"class"`
			Code  int    `json:"code"`
		}
		if err := json.Unmarshal([]byte(s), &env); err != nil {
			t.Errorf("stderr is not one JSON object: %.200q (%v)", s, err)
		} else if env.Class != tc.want.Name || env.Code != tc.want.Code {
			t.Errorf("envelope class %q code %d, want %q %d", env.Class, env.Code, tc.want.Name, tc.want.Code)
		}
	}
	if out.Len() != 0 && tc.want != classOK {
		t.Errorf("stdout = %.100q, want it empty on a failure", out.String())
	}
}

func seedProject() [][]string {
	return [][]string{{"project", "add", "P", "d"}}
}

func seedLinked() [][]string {
	return [][]string{
		{"project", "add", "P", "d"}, {"concept", "add", "C"},
		{"observation", "add", "--project", "P", "note", "body"}, {"link", "observation", "1", "C"},
	}
}

const goodRecord = "---\nid: \"0001\"\ntitle: \"ok\"\ndate: \"2026-09-25\"\nstatus: proposed\nkind: decision\ntrigger: design\nproject: P\nuuid: \"11111111-1111-7111-8111-111111111111\"\norigin_host: \"h\"\n---\n\nbody\n"

func TestVerbs_NegativeIsExit1(t *testing.T) {
	for _, tc := range []verbCase{
		{name: "project show", args: []string{"project", "show", "nosuch"}},
		{name: "project rename missing", setup: seedProject(), args: []string{"project", "rename", "nosuch", "x"}},
		{name: "project rename onto an existing name", setup: [][]string{{"project", "add", "P", "d"}, {"project", "add", "Q", "d"}}, args: []string{"project", "rename", "P", "Q"}},
		{name: "project set-status missing", args: []string{"project", "set-status", "nosuch", "active"}},
		{name: "project concepts", args: []string{"project", "concepts", "nosuch"}},
		{name: "concept delete missing", args: []string{"concept", "delete", "nosuch"}},
		{name: "concept delete while linked", setup: seedLinked(), args: []string{"concept", "delete", "C"}},
		{name: "concept rename missing", args: []string{"concept", "rename", "nosuch", "x"}},
		{name: "concept rename onto an existing name", setup: [][]string{{"concept", "add", "A"}, {"concept", "add", "B"}}, args: []string{"concept", "rename", "A", "B"}},
		{name: "document review promote of a section not drafted", setup: [][]string{{"project", "add", "P", "d"}, {"document", "ingest", "doc.md", "--project", "P"}}, files: map[string]string{"doc.md": "## S\n\nbody\n"}, args: []string{"document", "review", "promote", "1"}},
		{name: "observation show", args: []string{"observation", "show", "99"}},
		{name: "observation update", args: []string{"observation", "update", "99", "text"}},
		{name: "observation list unknown project", args: []string{"observation", "list", "--project", "nosuch"}},
		{name: "observation add unknown project", args: []string{"observation", "add", "--project", "nosuch", "note", "b"}},
		{name: "source show", args: []string{"source", "show", "99"}},
		{name: "source retract", args: []string{"source", "retract", "99", "note"}},
		{name: "source remove", args: []string{"source", "remove", "99"}},
		{name: "source remove while linked", setup: [][]string{{"project", "add", "P", "d"}, {"observation", "add", "--project", "P", "note", "b"}, {"source", "add", "S"}, {"source", "link", "1", "1"}}, args: []string{"source", "remove", "1"}},
		{name: "source link missing ids", args: []string{"source", "link", "99", "99"}},
		{name: "link observation missing observation", setup: [][]string{{"concept", "add", "C"}}, args: []string{"link", "observation", "99", "C"}},
		{name: "document review promote missing", args: []string{"document", "review", "promote", "99"}},
		{name: "link project", setup: seedProject(), args: []string{"link", "project", "nosuch", "C"}},
		{name: "link observation unknown concept", setup: seedLinked(), args: []string{"link", "observation", "1", "nosuch"}},
		{name: "search finds nothing", args: []string{"search", "zzznotfound"}},
		{name: "record show", args: []string{"record", "show", "9999"}},
		{name: "record list unknown project", args: []string{"record", "list", "--project", "nosuch"}},
		{name: "record list unknown status", args: []string{"record", "list", "--status", "acepted"}},
		{name: "document show", args: []string{"document", "show", "99"}},
		{name: "document list unknown project", args: []string{"document", "list", "--project", "nosuch"}},
		{name: "document tag unknown project", args: []string{"document", "tag", "--project", "nosuch"}},
		{name: "concept suggest unknown project", args: []string{"concept", "suggest", "--project", "nosuch"}},
		{name: "export unknown project", args: []string{"export", "-project", "nosuch"}},
		{name: "index --check with no index", files: map[string]string{"recs/0001-ok.md": goodRecord}, args: []string{"index", "--check", "recs"}},
	} {
		tc.want = classNegative
		t.Run(tc.name, func(t *testing.T) { runVerbCase(t, tc) })
	}
}

func TestVerbs_UsageIsExit2(t *testing.T) {
	for _, tc := range []verbCase{
		{name: "unknown observation kind", setup: seedProject(), args: []string{"observation", "add", "--project", "P", "bogus", "b"}},
		{name: "unknown project status", setup: seedProject(), args: []string{"project", "set-status", "P", "bogus"}},
		{name: "blank project name", args: []string{"project", "add", ""}},
		{name: "blank source title", args: []string{"source", "add", ""}},
		{name: "bad source date", args: []string{"source", "add", "T", "--published", "notadate"}},
		{name: "bad source url", args: []string{"source", "add", "T", "--url", "x y"}},
		{name: "bad source doi", args: []string{"source", "add", "T", "--doi", "zzz"}},
		{name: "record new unknown trigger", args: []string{"record", "new", "--workspace", "--title", "T", "--trigger", "bogus", "--dir", "d"}},
		{name: "record new unknown kind", args: []string{"record", "new", "--workspace", "--title", "T", "--trigger", "design", "--kind", "bogus", "--dir", "d"}},
		{name: "index --all with --stdout", args: []string{"index", "--all", "--stdout", "."}},
		{name: "index --check with --stdout", args: []string{"index", "--check", "--stdout", "."}},
		{name: "unknown subverb", args: []string{"project", "bogus"}},
		{name: "malformed id", args: []string{"observation", "show", "abc"}},
		{name: "unknown --accept-keywords value", files: map[string]string{"doc.md": "# T\n\nbody\n"}, args: []string{"document", "frontmatter", "doc.md", "--accept-keywords", "zzznotaconcept"}},
		{name: "negative concept suggest limit", args: []string{"concept", "suggest", "--limit", "-1"}},
		{name: "unparseable date", args: []string{"record", "list", "--since", "yesterday"}},
	} {
		tc.want = classUsage
		t.Run(tc.name, func(t *testing.T) { runVerbCase(t, tc) })
	}
}

func TestVerbs_DataIsExit65(t *testing.T) {
	for _, tc := range []verbCase{
		{name: "database file is not a database", files: map[string]string{"kb.db": "this is text, not a database, but long enough to be read as a header..\n"}, args: []string{"project", "list"}},
		{name: "import of malformed JSONL", files: map[string]string{"in.jsonl": "{\"type\": \n"}, args: []string{"import", "-in", "in.jsonl"}},
		{name: "merge input is zero bytes", setup: seedProject(), files: map[string]string{"z.db": ""}, args: []string{"merge", "-a", "z.db", "-b", "kb.db", "-out", "o.db"}},
		{name: "index over a malformed record", files: map[string]string{"recs/0001-bad.md": "---\nid: \"0001\"\ntitle: \"\"\n---\nx\n"}, args: []string{"index", "recs"}},
		{name: "document ingest of an unsupported pdf", setup: seedProject(), files: map[string]string{"a.pdf": "%PDF-1.4\n"}, args: []string{"document", "ingest", "a.pdf", "--project", "P"}},
	} {
		tc.want = classData
		t.Run(tc.name, func(t *testing.T) { runVerbCase(t, tc) })
	}
}

func TestVerbs_NoInputIsExit66(t *testing.T) {
	for _, tc := range []verbCase{
		{name: "ingest of a missing directory", args: []string{"ingest", "/nonexistent"}},
		{name: "ingest of a file", files: map[string]string{"f.txt": "x"}, args: []string{"ingest", "f.txt"}},
		{name: "document ingest of a missing file", setup: seedProject(), args: []string{"document", "ingest", "/nonexistent.md", "--project", "P"}},
		{name: "import of a missing file", args: []string{"import", "-in", "/nonexistent.jsonl"}},
		{name: "merge with a missing input", setup: seedProject(), args: []string{"merge", "-a", "/nonexistent.db", "-b", "kb.db", "-out", "o.db"}},
		{name: "merge input is a directory", setup: seedProject(), files: map[string]string{"d/x": "x"}, args: []string{"merge", "-a", "d", "-b", "kb.db", "-out", "o.db"}},
		{name: "index of a missing directory", args: []string{"index", "/nonexistent"}},
		{name: "index of a file", files: map[string]string{"f.txt": "x"}, args: []string{"index", "f.txt"}},
		{name: "no workspace here", noDB: true, args: []string{"project", "list"}},
	} {
		tc.want = classNoInput
		t.Run(tc.name, func(t *testing.T) { runVerbCase(t, tc) })
	}
}

func TestVerbs_CantCreateIsExit73(t *testing.T) {
	for _, tc := range []verbCase{
		{name: "export into a missing directory", args: []string{"export", "-out", "/nonexistent/dir/o.jsonl"}},
		{name: "merge -out already exists", setup: [][]string{{"project", "add", "P", "d"}, {"--db", "b.db", "project", "add", "Q", "d"}}, files: map[string]string{"o.db": "x"}, args: []string{"merge", "-a", "kb.db", "-b", "b.db", "-out", "o.db"}},
		{name: "record new under a file", files: map[string]string{"blocker": "x"}, args: []string{"record", "new", "--workspace", "--title", "T", "--trigger", "design", "--dir", "blocker/sub", "--root", "."}},
		{name: "init under a file", noDB: true, files: map[string]string{"blocker": "x"}, args: []string{"init", "blocker/sub"}},
	} {
		tc.want = classCantCreate
		t.Run(tc.name, func(t *testing.T) { runVerbCase(t, tc) })
	}
}

func TestVerbs_IOIsExit74(t *testing.T) {
	tc := verbCase{name: "import of a directory", files: map[string]string{"d/x": "x"}, args: []string{"import", "-in", "d"}, want: classIO}
	runVerbCase(t, tc)
}

func TestVerbs_PermissionIsExit77(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: permissions are not enforced")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked.jsonl")
	if err := os.WriteFile(locked, []byte("{}\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o644) })
	runVerbCase(t, verbCase{name: "import of an unreadable file", args: []string{"import", "-in", locked}, want: classNoPermission})

	ro := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(ro, 0o755) })
	runVerbCase(t, verbCase{name: "init in a read-only directory", noDB: true, args: []string{"init", filepath.Join(ro, "sub")}, want: classNoPermission})
}

// A direct, unprovoked pass over every class this table covers is not enough to
// prove the fallback is gone: this pins that an error nobody classified is now
// 70 (DR-0047 item 4), the change X2 makes.
func TestExitCodeFor_UnclassifiedIsInternalFromX2(t *testing.T) {
	if unclassifiedFallback != classInternal {
		t.Errorf("unclassifiedFallback = %v, want internal (70) once X2 is done", unclassifiedFallback)
	}
}
