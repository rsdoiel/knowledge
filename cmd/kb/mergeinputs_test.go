package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// merge opened each input with sql.Open and a pragma, and SQLite creates a
// file that is not there. So `kb merge -a x -b y -out z` with neither present
// exited 0, left zero-byte x and y behind, and wrote a schema-only z: a
// mistyped -a merged an empty database and reported success.

func mergeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	for name, project := range map[string]string{"a.db": "pa", "b.db": "pb"} {
		var out, errOut bytes.Buffer
		if code := mainRun([]string{"--db", name, "project", "add", project, "d"}, &out, &errOut); code != 0 {
			t.Fatalf("seed %s: exit %d: %s", name, code, errOut.String())
		}
	}
	return dir
}

func listing(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		info, _ := e.Info()
		names = append(names, fmt.Sprintf("%s:%d", e.Name(), info.Size()))
	}
	sort.Strings(names)
	return strings.Join(names, " ")
}

func runMergeArgs(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := mainRun(append([]string{"merge"}, args...), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestMerge_MissingInputIsRefusedAndCreatesNothing(t *testing.T) {
	dir := mergeDir(t)
	before := listing(t, dir)
	for _, tc := range []struct {
		name string
		args []string
		path string
	}{
		{"a missing", []string{"-a", "missing.db", "-b", "b.db", "-out", "o.db"}, "missing.db"},
		{"b missing", []string{"-a", "a.db", "-b", "missing.db", "-out", "o.db"}, "missing.db"},
		{"both missing", []string{"-a", "x", "-b", "y", "-out", "o.db"}, "x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errOut := runMergeArgs(tc.args...)
			if code != 1 {
				t.Errorf("exit %d, want 1; stdout=%.100q stderr=%q", code, out, errOut)
			}
			if !strings.Contains(errOut, tc.path) || !strings.Contains(errOut, "does not exist") {
				t.Errorf("stderr = %q, want it to name %q and say it does not exist", errOut, tc.path)
			}
			if after := listing(t, dir); after != before {
				t.Errorf("a refused merge changed the directory:\n before: %s\n after:  %s", before, after)
			}
		})
	}
}

func TestMerge_NotAKnowledgeBaseIsRefused(t *testing.T) {
	dir := mergeDir(t)
	if err := os.Mkdir("adir", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("empty.db", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	before := listing(t, dir)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"directory as a", []string{"-a", "adir", "-b", "b.db", "-out", "o.db"}, "is a directory"},
		{"directory as b", []string{"-a", "a.db", "-b", "adir", "-out", "o.db"}, "is a directory"},
		{"empty file as a", []string{"-a", "empty.db", "-b", "b.db", "-out", "o.db"}, "is empty"},
		{"empty file as b", []string{"-a", "a.db", "-b", "empty.db", "-out", "o.db"}, "is empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errOut := runMergeArgs(tc.args...)
			if code != 1 {
				t.Errorf("exit %d, want 1; stdout=%.100q stderr=%q", code, out, errOut)
			}
			if !strings.Contains(errOut, tc.want) {
				t.Errorf("stderr = %q, want it to contain %q", errOut, tc.want)
			}
			if strings.Contains(errOut, "out of memory") {
				t.Errorf("stderr = %q leaks SQLite's misleading message", errOut)
			}
			if after := listing(t, dir); after != before {
				t.Errorf("a refused merge changed the directory:\n before: %s\n after:  %s", before, after)
			}
		})
	}
}

// -a and -b naming one file is a slip of the fingers: the "merge" is a copy.
func TestMerge_SameFileTwiceIsAUsageError(t *testing.T) {
	dir := mergeDir(t)
	if err := os.Symlink("a.db", "link.db"); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	before := listing(t, dir)
	for _, args := range [][]string{
		{"-a", "a.db", "-b", "a.db", "-out", "o.db"},
		{"-a", "a.db", "-b", "./a.db", "-out", "o.db"},
		{"-a", "a.db", "-b", filepath.Join(dir, "a.db"), "-out", "o.db"},
		{"-a", "a.db", "-b", "link.db", "-out", "o.db"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			code, _, errOut := runMergeArgs(args...)
			if code != 2 {
				t.Errorf("exit %d, want 2; stderr=%q", code, errOut)
			}
			if !strings.Contains(errOut, "same file") {
				t.Errorf("stderr = %q, want it to say -a and -b are the same file", errOut)
			}
			if after := listing(t, dir); after != before {
				t.Errorf("directory changed: %s -> %s", before, after)
			}
		})
	}
}

// What must keep working.
func TestMerge_ValidMergeStillWorks(t *testing.T) {
	mergeDir(t)
	code, out, errOut := runMergeArgs("-a", "a.db", "-b", "b.db", "-out", "o.db")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "merged knowledge base written to o.db") {
		t.Errorf("stdout = %q", out)
	}
	var list, e bytes.Buffer
	if c := mainRun([]string{"--db", "o.db", "project", "list"}, &list, &e); c != 0 {
		t.Fatalf("listing the merged db: exit %d: %s", c, e.String())
	}
	if !strings.Contains(list.String(), "pa") || !strings.Contains(list.String(), "pb") {
		t.Errorf("merged projects = %q, want both pa and pb", list.String())
	}
	// The output still may not exist already (existing behaviour).
	if code, _, errOut := runMergeArgs("-a", "a.db", "-b", "b.db", "-out", "o.db"); code != 1 || !strings.Contains(errOut, "already exists") {
		t.Errorf("merging onto an existing -out: exit %d, stderr %q", code, errOut)
	}
}

func TestMerge_RefusalUnderJSONIsAnEnvelope(t *testing.T) {
	mergeDir(t)
	var out, errOut bytes.Buffer
	code := mainRun([]string{"--json", "merge", "-a", "missing.db", "-b", "b.db", "-out", "o.db"}, &out, &errOut)
	if code != 1 || out.Len() != 0 {
		t.Fatalf("exit %d, stdout %q", code, out.String())
	}
	var env struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(errOut.Bytes(), &env); err != nil || !strings.Contains(env.Error, "does not exist") {
		t.Errorf("stderr = %q (%v), want an envelope naming the missing file", errOut.String(), err)
	}
}

// A zero-byte main file counts as empty only when its -wal sidecar has nothing
// either. Not an observed state (a WAL database's main file is at least a page
// from the moment WAL mode is set), so this pins a defensive allowance: a file
// that might hold real data is not refused.
func TestCheckMergeInput(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, size int) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, bytes.Repeat([]byte{'x'}, size), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	full := write("full.db", 100)
	empty := write("empty.db", 0)
	walOnly := write("walonly.db", 0)
	write("walonly.db-wal", 4096)
	emptyWal := write("emptywal.db", 0)
	write("emptywal.db-wal", 0)
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, path, want string // want "" means accepted
	}{
		{"a real file", full, ""},
		{"zero bytes, nothing in a WAL", empty, "is empty"},
		{"zero bytes, empty WAL", emptyWal, "is empty"},
		{"zero bytes but data in the WAL", walOnly, ""},
		{"missing", filepath.Join(dir, "nope.db"), "does not exist"},
		{"directory", sub, "is a directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkMergeInput("-a", tc.path)
			switch {
			case tc.want == "" && err != nil:
				t.Errorf("checkMergeInput(%s) = %v, want it accepted", tc.name, err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Errorf("checkMergeInput(%s) = %v, want an error containing %q", tc.name, err, tc.want)
			case tc.want != "" && err != nil && !strings.Contains(err.Error(), "-a"):
				t.Errorf("error %q should name the flag -a", err)
			}
		})
	}
}
