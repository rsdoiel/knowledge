package main

import (
	"strings"
	"testing"
)

// A refused wikilink is reported by `kb document ingest` in both forms, and is not
// a failure: the exit stays 0 (DR-0003).
func TestCmdDocumentIngest_ReportsARefusedWikilink(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	if _, err := kb.AddProject("alpha", ""); err != nil {
		t.Fatal(err)
	}
	path := writeDocFixture(t, root+"/docs", "a.md", "## First\n\nSee [[bad\x01name]] here.\n")

	out, err := runDocument(t, kb, false, "ingest", path, "--project", "alpha", "--no-prompt")
	if err != nil {
		t.Fatalf("document ingest: %v", err)
	}
	if !strings.Contains(out, "tag warning:") || !strings.Contains(out, `bad\x01name`) {
		t.Errorf("output = %q, want a tag warning naming the link", out)
	}

	path = writeDocFixture(t, root+"/docs", "b.md", "## First\n\nSee [[bad\x01name]] here.\n")
	out, err = runDocument(t, kb, true, "ingest", path, "--project", "alpha", "--no-prompt")
	if err != nil {
		t.Fatalf("document ingest --json: %v", err)
	}
	if !strings.Contains(out, `"tag_warnings"`) {
		t.Errorf("json = %s, want tag_warnings", out)
	}
}
