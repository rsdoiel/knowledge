package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The vocabulary of the command line is checked before the workspace. An
// unknown verb is a usage error (exit 2) whether or not a database is there;
// before this, `kb version` in a directory with no workspace said "no
// agents/knowledge.db here" and exited 66, which sent the reader looking for a
// missing database instead of a mistyped verb.

func TestMainRun_UnknownVerbIsUsageBeforeTheWorkspaceIsLooked_For(t *testing.T) {
	for _, args := range [][]string{
		{"frobnicate"},
		{"frobnicate", "extra"},
		{"--json", "frobnicate"},
		{"--db", "rt.db", "frobnicate"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			t.Setenv("KB_DB", "")
			var out, errOut bytes.Buffer
			code := mainRun(args, &out, &errOut)
			if code != 2 {
				t.Errorf("exit %d, want 2; stderr=%q", code, errOut.String())
			}
			first, _, _ := strings.Cut(errOut.String(), "\n")
			if !strings.Contains(first, "frobnicate") || strings.Contains(first, "knowledge.db") {
				t.Errorf("first line of stderr = %q, want the unknown verb named and no word of the database", first)
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Errorf("an unknown verb left files behind: %v", entries)
			}
		})
	}
}

// With --json the unknown verb is also reported as the error envelope on
// stdout, like every other error (workspace DR-0010): one JSON object, class
// "usage", code 2. The plain line stays on stderr, without the usage page.
func TestMainRun_UnknownVerbUnderJSONIsTheEnvelope(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("KB_DB", "")
	var out, errOut bytes.Buffer
	code := mainRun([]string{"--json", "frobnicate"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	var env struct {
		Error string `json:"error"`
		Class string `json:"class"`
		Code  int    `json:"code"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%q", err, out.String())
	}
	if env.Class != "usage" || env.Code != 2 || !strings.Contains(env.Error, "frobnicate") {
		t.Errorf("envelope = %+v, want class usage, code 2, the verb named", env)
	}
	if !strings.HasPrefix(errOut.String(), "kb: ") || strings.Contains(errOut.String(), "# NAME") {
		t.Errorf("stderr = %q, want the plain line and no usage page under --json", errOut.String())
	}
}
