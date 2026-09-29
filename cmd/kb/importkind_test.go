package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `kb import` keeps an observation kind outside the vocabulary and says so; the
// run still exits 0, because a legacy value is a fixable row, not a failure.

const legacyKindDump = `{"type":"project","uuid":"p-1","name":"legacy","description":"","status":"active","created_at":"2026-06-18 00:00:00"}
{"type":"observation","uuid":"o-1","origin_host":"unknown","project_uuid":"p-1","kind":"release","body":"cut release 1.0","source_doi":"","created_at":"2026-06-18 00:00:00"}
`

func TestImport_UnknownObservationKindWarnsAndExitsZero(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.jsonl")
	if err := os.WriteFile(in, []byte(legacyKindDump), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := mainRun([]string{"--db", filepath.Join(dir, "kb.db"), "import", "-in", in}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "warning:") || !strings.Contains(out.String(), `"release"`) {
		t.Errorf("output lacks a warning naming the kind:\n%s", out.String())
	}

	var showOut bytes.Buffer
	mainRun([]string{"--db", filepath.Join(dir, "kb.db"), "observation", "list", "--project", "legacy"}, &showOut, &errOut)
	if !strings.Contains(showOut.String(), "release") {
		t.Errorf("kind was not preserved:\n%s", showOut.String())
	}
}

func TestImport_UnknownObservationKindWarningIsInJSON(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.jsonl")
	if err := os.WriteFile(in, []byte(legacyKindDump), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := mainRun([]string{"--json", "--db", filepath.Join(dir, "kb.db"), "import", "-in", in}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"Warnings"`) {
		t.Errorf("--json output lacks Warnings:\n%s", out.String())
	}
}
