package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrintHelp_MainPage(t *testing.T) {
	var out bytes.Buffer
	if !printHelp(&out, "") {
		t.Fatal("expected the main page (topic \"\") to be recognized")
	}
	if !strings.Contains(out.String(), "kb") {
		t.Errorf("main help = %q, want it to mention kb", out.String())
	}
}

func TestPrintHelp_AllVerbTopicsRecognized(t *testing.T) {
	for _, topic := range []string{"project", "observation", "concept", "link", "source", "search", "summary", "format", "merge"} {
		var out bytes.Buffer
		if !printHelp(&out, topic) {
			t.Errorf("topic %q: expected printHelp to recognize it", topic)
		}
		if out.Len() == 0 {
			t.Errorf("topic %q: expected non-empty help text", topic)
		}
	}
}

func TestPrintHelp_UnknownTopicReturnsFalse(t *testing.T) {
	var out bytes.Buffer
	if printHelp(&out, "bogus") {
		t.Error("expected an unknown topic to return false")
	}
	if out.Len() != 0 {
		t.Errorf("expected nothing written for an unknown topic, got %q", out.String())
	}
}

// TestPrintHelp_OutputIsValidPandocManPage confirms each help text is
// actually well-formed Pandoc-Markdown man-page source, not just
// text that looks plausible -- the real acceptance criterion from
// cli-tui-plan.md W8, checked by piping through the same pandoc
// invocation the (future) Makefile targets use.
func TestPrintHelp_OutputIsValidPandocManPage(t *testing.T) {
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc not installed; skipping man-page-validity check")
	}
	for _, topic := range []string{"", "project", "observation", "concept", "link", "source", "search", "merge"} {
		t.Run("topic="+topic, func(t *testing.T) {
			var out bytes.Buffer
			if !printHelp(&out, topic) {
				t.Fatalf("printHelp(%q) not recognized", topic)
			}
			cmd := exec.Command("pandoc", "--from", "markdown", "--to", "man", "-s")
			cmd.Stdin = &out
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Errorf("pandoc rejected topic %q's help text: %v\nstderr: %s", topic, err, stderr.String())
			}
		})
	}
}

func TestMainRun_VerbDashHPrintsHelpWithoutOpeningDB(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var out, errOut bytes.Buffer
	code := mainRun([]string{"project", "-h"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (errOut=%q)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "project") {
		t.Errorf("output = %q, want it to mention project", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "agents", "knowledge.db")); err == nil {
		t.Error("expected no ./agents/knowledge.db to be created by kb project -h")
	}
}

func TestMainRun_HelpVerbPrintsTopicHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	code := mainRun([]string{"help", "merge"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (errOut=%q)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "merge") {
		t.Errorf("output = %q, want it to mention merge", out.String())
	}
}

func TestMainRun_HelpUnknownTopicIsUsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	code := mainRun([]string{"help", "bogus"}, &out, &errOut)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}
