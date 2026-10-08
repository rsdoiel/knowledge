package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every environment variable kb reads is documented on the kb(1) page. The
// names are found by scanning the source, so a variable added later without a
// line in HelpText fails here, as a verb without a help topic does in
// help_dispatch_test.go.
func TestHelpText_DocumentsEveryEnvironmentVariableKBReads(t *testing.T) {
	name := regexp.MustCompile(`"(KB_[A-Z_]+)"`)
	seen := map[string]bool{}
	for _, pattern := range []string{"*.go", "../../*.go"} {
		files, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range name.FindAllSubmatch(src, -1) {
				seen[string(m[1])] = true
			}
		}
	}
	if len(seen) < 4 {
		t.Fatalf("found only %v; the scan is broken", seen)
	}
	for v := range seen {
		if !strings.Contains(HelpText, v) {
			t.Errorf("%s is read by kb but the kb(1) page never mentions it", v)
		}
	}
}

// The record page documents every subverb the verb accepts, in its SYNOPSIS.
func TestRecordHelpText_NamesEverySubverb(t *testing.T) {
	for _, sub := range []string{"list", "pending", "show", "new", "set-status", "supersede", "fmt", "concepts", "fuzzy-tag", "delete"} {
		if !strings.Contains(RecordHelpText, "record "+sub) {
			t.Errorf("RecordHelpText has no line for record %s", sub)
		}
	}
	for _, phrase := range []string{"SCOPE/DR-NNNN", "KB_PROJECT", "deprecated", "pending"} {
		if !strings.Contains(RecordHelpText, phrase) {
			t.Errorf("RecordHelpText does not mention %q", phrase)
		}
	}
}

// A project cannot be called workspace; the project page says why.
func TestProjectHelpText_ExplainsTheReservedName(t *testing.T) {
	if !strings.Contains(ProjectHelpText, "reserved") {
		t.Error("ProjectHelpText does not say that the name workspace is reserved")
	}
}

// How the workspace is found is part of the kb(1) page, with the way to name one.
func TestHelpText_ExplainsWorkspaceDiscovery(t *testing.T) {
	for _, phrase := range []string{"walks up", "knowledge.jsonl", "import -in", "KB_DB"} {
		if !strings.Contains(HelpText, phrase) {
			t.Errorf("the kb(1) page does not mention %q", phrase)
		}
	}
}
