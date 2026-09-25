package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// `kb -help` documents exit 2 as "usage error (bad flags, unknown verb)" and
// exit 1 as "the verb ran but failed". Only an unknown top-level verb used to
// exit 2: a missing argument, an unknown subverb, an unknown flag or a
// malformed id all exited 1, indistinguishable from "not found" for a script
// that has to decide between fixing its command line and handling a result.

func runKB(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	var out, errOut bytes.Buffer
	code := mainRun(append([]string{"--db", dbPath}, args...), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestMainRun_UsageErrorsExitTwo(t *testing.T) {
	cases := map[string][][]string{
		"missing argument": {
			{"search"}, {"project", "show"}, {"project", "add"}, {"project", "concepts"},
			{"project", "set-status", "p"}, {"project", "rename", "a"},
			{"observation", "add"}, {"observation", "list"}, {"observation", "show"},
			{"concept", "add"}, {"concept", "rename", "a"}, {"concept", "delete"},
			{"link"}, {"link", "project", "p"}, {"source", "add"},
			{"record"}, {"record", "show"}, {"document"}, {"document", "show"},
			{"document", "review"}, {"document", "tag"}, {"merge"}, {"index"}, {"ingest"},
		},
		"unknown subverb": {
			{"project", "bogus"}, {"observation", "bogus"}, {"concept", "bogus"},
			{"link", "bogus"}, {"source", "bogus"}, {"record", "bogus"},
			{"document", "bogus"}, {"document", "review", "bogus"},
		},
		"unknown flag": {
			{"observation", "add", "--bogus", "x"}, {"record", "list", "--bogus"},
			{"document", "tag", "--bogus"}, {"concept", "delete", "--bogus", "x"},
			{"project", "rename", "--bogus", "a", "b"}, {"ingest", "--bogus", "x"},
			{"document", "ingest", "--bogus", "f"}, {"concept", "suggest", "--bogus"},
		},
		"bad flag value": {
			{"concept", "suggest", "--limit", "abc"},
		},
		"flag missing its value": {
			{"record", "list", "--status"}, {"document", "list", "--project"},
		},
		"malformed id": {
			{"observation", "show", "abc"}, {"observation", "update", "abc", "x"},
			{"source", "retract", "abc", "note"}, {"link", "observation", "abc", "c"},
			{"document", "show", "abc"}, {"document", "review", "promote", "abc"},
		},
		"missing required flag": {
			{"record", "new"}, {"record", "new", "--title", "t"},
		},
		"bad flag value the command validates itself": {
			{"document", "draft", "1", "body", "--by", "w", "--confidence", "abc"},
			{"document", "draft", "1", "body", "--by", "w", "--confidence", "2"},
			{"document", "frontmatter", "f.md", "--set", "noequals"},
			{"document", "frontmatter", "f.md", "--set", "nosuchfield=x"},
			{"document", "frontmatter", "f.md", "--accept", "nosuchfield"},
		},
		"surplus argument": {
			{"index", "a", "b"}, {"init", "a", "b"}, {"ingest", "a", "b"},
			{"document", "frontmatter", "f.md", "extra"},
		},
	}
	for name, group := range cases {
		for _, args := range group {
			t.Run(name+"/"+strings.Join(args, "_"), func(t *testing.T) {
				code, out, errOut := runKB(t, args...)
				if code != 2 {
					t.Errorf("exit %d, want 2; stdout=%.100q stderr=%q", code, out, errOut)
				}
				if !strings.HasPrefix(errOut, "kb: ") {
					t.Errorf("stderr = %q, want it to start with \"kb: \"", errOut)
				}
			})
		}
	}
}

// A usage error under --json is still exit 2, with the same envelope on stderr
// and nothing on stdout.
func TestMainRun_UsageErrorUnderJSONExitsTwo(t *testing.T) {
	code, out, errOut := runKB(t, "--json", "project", "show")
	if code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want it empty", out)
	}
	var env struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(errOut), &env); err != nil || env.Error == "" {
		t.Errorf("stderr = %q, want a {\"error\": ...} envelope (%v)", errOut, err)
	}
}

// Exit 1 is for a command line that was fine and a result that was not. A
// script must be able to tell these from the cases above.
func TestMainRun_RuntimeFailuresStillExitOne(t *testing.T) {
	for _, args := range [][]string{
		{"project", "show", "nosuch"},
		{"observation", "show", "999999"},
		{"record", "show", "9999"},
		{"concept", "delete", "nosuch"},
		{"concept", "rename", "nosuch", "x"},
		{"source", "show", "9999"},
		{"document", "show", "9999"},
		{"search", "zzzznomatch"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			code, out, errOut := runKB(t, args...)
			if code != 1 {
				t.Errorf("exit %d, want 1; stdout=%.100q stderr=%q", code, out, errOut)
			}
		})
	}
}

// Already exit 2 before this change; pinned so it stays that way. The unknown
// verb gets an explicit --db: without one, whether it reaches the dispatcher
// (exit 2) or stops at the missing ambient database (exit 1) depends on
// whether ./agents/knowledge.db happens to exist in the package directory,
// which made this pass in a tree with a stray one and fail in a clean checkout.
func TestMainRun_ExistingUsageExitsUnchanged(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	for _, args := range [][]string{
		{"--db", dbPath, "bogusverb"}, {"help", "nosuchtopic"}, {"--bogus"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := mainRun(args, &out, &errOut); code != 2 {
				t.Errorf("exit %d, want 2; stderr=%q", code, errOut.String())
			}
		})
	}
}

func TestUsageError_WrapsAndKeepsTheChain(t *testing.T) {
	inner := usageErrorf("usage: %s", "x")
	if inner.Error() != "usage: x" {
		t.Errorf("Error() = %q", inner.Error())
	}
	if !isUsageError(inner) {
		t.Error("isUsageError(usageErrorf(...)) = false")
	}
	if isUsageError(nil) {
		t.Error("isUsageError(nil) = true")
	}
	if !isUsageError(wrapUsage(errBoom)) {
		t.Error("wrapUsage did not mark the error as a usage error")
	}
	if wrapUsage(nil) != nil {
		t.Error("wrapUsage(nil) should stay nil")
	}
}

var errBoom = &boomError{}

type boomError struct{}

func (*boomError) Error() string { return "boom" }
