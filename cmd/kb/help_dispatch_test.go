package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
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

// Driven off the verbs map rather than a hardcoded list: a hardcoded one
// silently went stale when ingest, record and index were added, which is the
// drift this now catches.
func TestPrintHelp_EveryRegisteredVerbHasATopic(t *testing.T) {
	for verb := range verbs {
		var out bytes.Buffer
		if !printHelp(&out, verb) {
			t.Errorf("verb %q is registered but has no help topic", verb)
			continue
		}
		if out.Len() == 0 {
			t.Errorf("verb %q: help topic is empty", verb)
		}
	}
	// The aliases share a page with search and are not verbs in their own right.
	for _, alias := range []string{"summary", "format"} {
		var out bytes.Buffer
		if !printHelp(&out, alias) {
			t.Errorf("alias %q: expected printHelp to recognize it", alias)
		}
	}
}

// kb(1) is where someone discovers a verb exists at all, so a verb missing
// from it is invisible however good its own page is.
func TestHelpText_ListsEveryRegisteredVerb(t *testing.T) {
	var out bytes.Buffer
	printHelp(&out, "")
	page := out.String()
	for verb := range verbs {
		if !strings.Contains(page, verb) {
			t.Errorf("verb %q is registered but not mentioned in kb(1)", verb)
		}
	}
}

// The Makefile's KB_TOPICS drives kb-topics-help, which generates each verb's
// kb-TOPIC.1.md. MAN_PAGES_1 is then a shell glob over the .1.md files that
// already exist, so a verb missing from KB_TOPICS never gets a man page and
// make man reports success anyway -- silent, which is why this is a test.
func TestMakefile_KBTopicsCoversEveryVerb(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Skipf("no Makefile to check: %v", err)
	}
	var topics string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "KB_TOPICS") {
			_, topics, _ = strings.Cut(line, "=")
			break
		}
	}
	if topics == "" {
		t.Fatal("no KB_TOPICS assignment found in the Makefile")
	}
	listed := map[string]bool{}
	for _, f := range strings.Fields(topics) {
		listed[f] = true
	}
	// summary and format are aliases sharing search's page, so they have no
	// page of their own to generate.
	for _, alias := range []string{"summary", "format"} {
		listed[alias] = true
	}
	// Every topic needs a page, not only every verb: RELEASE_REVIEW.md asks
	// that each topic be renderable as a man page, and "topics" -- the topic
	// index itself -- is not a verb.
	for _, topic := range helpTopicNames() {
		if !listed[topic] {
			t.Errorf("topic %q has a help page but is missing from the Makefile's KB_TOPICS, so make man will not generate it", topic)
		}
	}
}

// helpTopicNames returns every topic printHelp recognises: the registered
// verbs plus the non-verb topics.
func helpTopicNames() []string {
	names := []string{"topics"}
	for verb := range verbs {
		names = append(names, verb)
	}
	return names
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

// The documented vocabularies must match what the code accepts. The plan's own
// W7 text listed a "release" observation kind that IsValidKind rejects, which
// is how this drift looks in practice.
func TestHelpText_VocabulariesMatchTheCode(t *testing.T) {
	for _, tc := range []struct {
		topic  string
		values []string
	}{
		{"observation", knowledge.ValidObservationKinds},
		{"record", knowledge.RecordStatuses},
		{"record", knowledge.RecordKinds},
		{"record", knowledge.RecordTriggers},
	} {
		var out bytes.Buffer
		printHelp(&out, tc.topic)
		page := out.String()
		for _, v := range tc.values {
			if !strings.Contains(page, v) {
				t.Errorf("kb help %s does not document %q, which the code accepts", tc.topic, v)
			}
		}
	}
}

// Observation kinds are enforced; record vocabularies are reported, not
// enforced. The asymmetry is deliberate -- observations predate the
// documented-not-enforced rule and harvey depends on the rejection -- so it is
// pinned here rather than left to be discovered.
func TestVocabularyEnforcementAsymmetry(t *testing.T) {
	if knowledge.IsValidKind("release") {
		t.Error("IsValidKind accepts \"release\"; the observation vocabulary is meant to be closed")
	}
	kb := openTestKB(t)
	pid, err := kb.AddProject("p", "")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if _, err := kb.AddObservation(pid, "release", "body"); err == nil {
		t.Error("an out-of-vocabulary observation kind was accepted; observations enforce")
	}
	// A record with an out-of-vocabulary kind parses, and warns.
	rf, err := knowledge.ParseRecord([]byte(recordWithKind("architecture")), "decisions/0001-x.md")
	if err != nil {
		t.Fatalf("an out-of-vocabulary record kind must parse: %v", err)
	}
	if len(rf.Warnings) == 0 {
		t.Error("an out-of-vocabulary record kind produced no warning; records report")
	}
}

func recordWithKind(kind string) string {
	return `---
id: "0001"
title: "Vocabulary probe"
date: "2026-08-25"
status: accepted
kind: ` + kind + `
trigger: design
project: p
phase: ""
supersedes: []
superseded_by: []
relates_to: []
initiative: ""
session: ""
decisions: []
tags: []
uuid: "55555555-5555-7555-8555-555555555555"
origin_host: "test"
---

**Context.** probe
`
}

// RELEASE_REVIEW.md: "The help guide system should support 'help index' which
// lists all the available topics."
//
// kb cannot use that spelling: index is a verb, so `kb help index` is the
// kb-index(1) page and must stay that way. `kb help topics` is the topic
// index instead — harvey accepts both spellings, and only this one is free
// here.
func TestPrintHelp_TopicsListsEveryTopic(t *testing.T) {
	var out bytes.Buffer
	if !printHelp(&out, "topics") {
		t.Fatal("kb help topics is not recognized")
	}
	page := out.String()
	for verb := range verbs {
		if !strings.Contains(page, verb) {
			t.Errorf("verb %q is missing from the topic index", verb)
		}
	}
	// Every name it advertises must actually resolve.
	for _, line := range strings.Split(page, "\n") {
		name, _, found := strings.Cut(strings.TrimSpace(line), " ")
		if !found || name == "" || strings.HasPrefix(name, "#") || strings.HasPrefix(name, "%") {
			continue
		}
		if _, known := verbs[name]; !known {
			continue
		}
		var probe bytes.Buffer
		if !printHelp(&probe, name) {
			t.Errorf("topic index advertises %q but printHelp does not recognize it", name)
		}
	}
}

// index is a verb, so this spelling must keep resolving to its manual.
func TestPrintHelp_IndexIsTheVerbPageNotTheTopicIndex(t *testing.T) {
	var out bytes.Buffer
	if !printHelp(&out, "index") {
		t.Fatal("kb help index is not recognized")
	}
	if !strings.Contains(out.String(), "kb-index(1)") {
		t.Errorf("kb help index should be the verb's manual page:\n%s", out.String()[:200])
	}
}
