package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `index --all` reports every corpus, then exits. A stale index is a normal
// negative answer (1), like `index --check`; a corpus that could not be indexed
// at all (a malformed record, an unwritable index) is a failure and exits with its
// class. When both occur the failure wins: it is the more serious thing to tell a
// caller, and the stale ones are still listed (X3, DR-0047 item 6).

func indexAllRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)
	for _, c := range []string{"a", "b"} {
		dir := filepath.Join(root, c, "decisions")
		writeRaw(t, dir, "0001-ok.md", goodRecord)
		var out, errOut bytes.Buffer
		if code := mainRun([]string{"index", dir}, &out, &errOut); code != 0 {
			t.Fatalf("index %s: exit %d: %s", dir, code, errOut.String())
		}
	}
	return root
}

func runIndexAll(t *testing.T, root string, extra ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := mainRun(append([]string{"index", "--all", root}, extra...), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestIndexAll_StaleOnlyIsNegative(t *testing.T) {
	root := indexAllRoot(t)
	// Change a record's title after its index was written: the index is stale.
	path := filepath.Join(root, "a", "decisions", "0001-ok.md")
	b, _ := os.ReadFile(path)
	if err := os.WriteFile(path, bytes.Replace(b, []byte(`title: "ok"`), []byte(`title: "changed"`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runIndexAll(t, root, "--check")
	if code != 1 {
		t.Errorf("exit %d, want 1 (a stale index is a negative answer); stdout %q", code, out)
	}
	if !strings.Contains(out, "stale") {
		t.Errorf("stdout %q should list the stale corpus", out)
	}
}

func TestIndexAll_AFailedCorpusExitsWithItsClassAndTheOthersAreStillReported(t *testing.T) {
	root := indexAllRoot(t)
	// One corpus is stale, another has a record that cannot be parsed.
	a := filepath.Join(root, "a", "decisions", "0001-ok.md")
	b, _ := os.ReadFile(a)
	os.WriteFile(a, bytes.Replace(b, []byte(`title: "ok"`), []byte(`title: "changed"`), 1), 0o644)
	writeRaw(t, filepath.Join(root, "b", "decisions"), "0002-bad.md", "---\nid: \"0002\"\ntitle: \"\"\n---\nx\n")

	code, out, _ := runIndexAll(t, root, "--check")
	if code != 65 {
		t.Errorf("exit %d, want 65: the malformed record is a failure and outranks the stale index; stdout %q", code, out)
	}
	if !strings.Contains(out, "stale") || !strings.Contains(out, "error") {
		t.Errorf("stdout %q should still report both the stale corpus and the failed one", out)
	}
}

func TestIndexAll_AllCurrentExitsZero(t *testing.T) {
	root := indexAllRoot(t)
	if code, out, errOut := runIndexAll(t, root, "--check"); code != 0 {
		t.Errorf("exit %d, want 0; stdout %q stderr %q", code, out, errOut)
	}
}
