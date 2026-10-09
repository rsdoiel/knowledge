package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// kb-TOPIC.1.md is generated from the help text in helptext.go (make
// kb-topics-help runs `kb TOPIC -help` into it), so the Go source is the one to
// edit. An edit made only to the .1.md is erased by the next regeneration and
// never reaches `kb help TOPIC`; this test finds that. It compares everything
// below the three header lines, which carry the version, hash and date.

func bodyBelowHeader(s string) string {
	lines := strings.SplitN(s, "\n", 4)
	if len(lines) < 4 {
		return s
	}
	return lines[3]
}

func TestManPages_MatchTheirHelpText(t *testing.T) {
	for _, topic := range makefileTopics(t) {
		t.Run(topic, func(t *testing.T) {
			var out bytes.Buffer
			if !printHelp(&out, topic) {
				t.Fatalf("no help topic %q", topic)
			}
			onDisk, err := os.ReadFile("../../kb-" + topic + ".1.md")
			if err != nil {
				t.Fatalf("read the man page source: %v", err)
			}
			if bodyBelowHeader(out.String()) != bodyBelowHeader(string(onDisk)) {
				t.Errorf("kb-%s.1.md differs from `kb help %s`; edit helptext.go and regenerate the page with `kb %s -help > kb-%s.1.md`", topic, topic, topic, topic)
			}
		})
	}
}

// makefileTopics reads the verb list that drives kb-topics-help.
func makefileTopics(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatalf("read the Makefile: %v", err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "KB_TOPICS") {
			_, list, _ := strings.Cut(line, "=")
			return strings.Fields(list)
		}
	}
	t.Fatal("no KB_TOPICS assignment found in the Makefile")
	return nil
}
