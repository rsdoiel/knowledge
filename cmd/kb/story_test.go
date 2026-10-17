package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// ─── kb story timeline and kb story check (DR-0075 item 9, DR-0078 item 4) ──

func sceneDoc(label, time, extra string) string {
	return "---\ntitle: " + label + "\nevent: " + label + "\nstory_time: \"" + time + "\"\n" + extra + "---\n\ntext\n"
}

func storyKB(t *testing.T, docs map[string]string) (*knowledge.KnowledgeBase, string) {
	t.Helper()
	kb, root := openWorkspaceKB(t)
	p, err := kb.AddProject("work", "")
	if err != nil {
		t.Fatal(err)
	}
	// ingest in name order so the narrative order is predictable
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		body, ok := docs[name]
		if !ok {
			continue
		}
		path := writeDocFixture(t, root+"/docs", name, body)
		if _, err := kb.IngestDocument(p, path, knowledge.DocumentIngestOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	return kb, root
}

func TestCmdStory_TimelineStoryOrderAndNarrativeOrder(t *testing.T) {
	kb, _ := storyKB(t, map[string]string{
		"a.md": sceneDoc("later", "1970-05-01", ""), "b.md": sceneDoc("earlier", "1969-10-29", ""),
	})
	var out bytes.Buffer
	if err := cmdStory(kb, nil, false, []string{"timeline", "--project", "work"}, &out); err != nil {
		t.Fatalf("story timeline: %v", err)
	}
	if i, j := strings.Index(out.String(), "earlier"), strings.Index(out.String(), "later"); i < 0 || j < 0 || i > j {
		t.Errorf("timeline = %q, want earlier before later", out.String())
	}
	out.Reset()
	if err := cmdStory(kb, nil, false, []string{"timeline", "--narrative", "--project", "work"}, &out); err != nil {
		t.Fatal(err)
	}
	if i, j := strings.Index(out.String(), "earlier"), strings.Index(out.String(), "later"); i < 0 || j < 0 || j > i {
		t.Errorf("narrative = %q, want later (ingested first) before earlier", out.String())
	}
}

func TestCmdStory_TimelineMarksUnorderedAndExplainsTheMarks(t *testing.T) {
	kb, _ := storyKB(t, map[string]string{"a.md": sceneDoc("year", "1969", ""), "b.md": sceneDoc("day", "1969-10-29", "")})
	var out bytes.Buffer
	if err := cmdStory(kb, nil, false, []string{"timeline", "--project", "work"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "~") || !strings.Contains(out.String(), "not established") {
		t.Errorf("timeline = %q, want a ~ mark and its legend", out.String())
	}
}

func TestCmdStory_TimelineJSON(t *testing.T) {
	kb, _ := storyKB(t, map[string]string{"a.md": sceneDoc("one", "1969-10-29", "pov: Anna\n")})
	var out bytes.Buffer
	if err := cmdStory(kb, nil, true, []string{"timeline", "--project", "work"}, &out); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || len(got) != 1 {
		t.Fatalf("json = %q: %v", out.String(), err)
	}
	for _, k := range []string{"rank", "label", "story_time", "start", "end", "precision", "state", "pov", "document", "unordered", "undated", "in_cycle"} {
		if _, ok := got[0][k]; !ok {
			t.Errorf("timeline json lacks %q: %v", k, got[0])
		}
	}
}

func TestCmdStory_CheckIsAdvisoryUnlessStrict(t *testing.T) {
	kb, _ := storyKB(t, map[string]string{"a.md": sceneDoc("odd", "next tuesday", "")})
	var out bytes.Buffer
	if err := cmdStory(kb, nil, false, []string{"check", "--project", "work"}, &out); err != nil {
		t.Fatalf("story check reported findings and must still succeed: %v", err)
	}
	if !strings.Contains(out.String(), "unparseable-time") || !strings.Contains(out.String(), "next tuesday") {
		t.Errorf("check = %q", out.String())
	}
	out.Reset()
	err := cmdStory(kb, nil, false, []string{"check", "--strict", "--project", "work"}, &out)
	if err == nil {
		t.Fatal("--strict with findings must fail")
	}
	if got := exitCodeFor(err); got != classNegative {
		t.Errorf("--strict class = %v, want negative (exit 1)", got)
	}
}

func TestCmdStory_CheckCleanAndJSON(t *testing.T) {
	kb, _ := storyKB(t, map[string]string{"a.md": sceneDoc("fine", "1969-10-29", "")})
	var out bytes.Buffer
	if err := cmdStory(kb, nil, false, []string{"check", "--strict", "--project", "work"}, &out); err != nil {
		t.Fatalf("clean project with --strict: %v", err)
	}
	if !strings.Contains(out.String(), "no findings") {
		t.Errorf("check = %q", out.String())
	}
	kb2, _ := storyKB(t, map[string]string{"a.md": sceneDoc("odd", "next tuesday", "")})
	out.Reset()
	if err := cmdStory(kb2, nil, true, []string{"check", "--project", "work"}, &out); err != nil {
		t.Fatal(err)
	}
	var got []map[string]string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || len(got) != 1 || got[0]["kind"] != "unparseable-time" || got[0]["message"] == "" {
		t.Errorf("json = %q: %v", out.String(), err)
	}
}

func TestStory_ExitCodes(t *testing.T) {
	noTerminal(t) // a bare `kb story` would open the interface on a terminal
	scene := sceneDoc("odd", "next tuesday", "")
	setup := [][]string{{"project", "add", "P", "d"}, {"document", "ingest", "--no-prompt", "s.md", "--project", "P"}}
	files := map[string]string{"s.md": scene}
	for _, tc := range []verbCase{
		{name: "no subverb", args: []string{"story"}, want: classUsage},
		{name: "unknown subverb", args: []string{"story", "bogus", "--project", "P"}, want: classUsage},
		{name: "timeline without a project", args: []string{"story", "timeline"}, want: classUsage},
		{name: "check without a project", args: []string{"story", "check"}, want: classUsage},
		{name: "timeline bad flag", setup: setup, files: files, args: []string{"story", "timeline", "--project", "P", "--bogus"}, want: classUsage},
		{name: "check surplus argument", setup: setup, files: files, args: []string{"story", "check", "--project", "P", "extra"}, want: classUsage},
		{name: "--narrative on check", setup: setup, files: files, args: []string{"story", "check", "--project", "P", "--narrative"}, want: classUsage},
		{name: "--strict on timeline", setup: setup, files: files, args: []string{"story", "timeline", "--project", "P", "--strict"}, want: classUsage},
		{name: "timeline unknown project", args: []string{"story", "timeline", "--project", "nosuch"}, want: classNegative},
		{name: "check unknown project", args: []string{"story", "check", "--project", "nosuch"}, want: classNegative},
		{name: "check with findings is exit 0", setup: setup, files: files, args: []string{"story", "check", "--project", "P"}, want: classOK},
		{name: "timeline is exit 0", setup: setup, files: files, args: []string{"story", "timeline", "--project", "P"}, want: classOK},
	} {
		t.Run(tc.name, func(t *testing.T) { runVerbCase(t, tc) })
	}
}

// --strict prints the report and then exits 1, so it is checked on its own: the
// shared table asserts an empty stdout on a failure.
func TestStory_StrictCheckPrintsTheReportThenExitsOne(t *testing.T) {
	noTerminal(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeDocFixture(t, dir, "s.md", sceneDoc("odd", "next tuesday", ""))
	db := dir + "/kb.db"
	for _, a := range [][]string{{"project", "add", "P", "d"}, {"document", "ingest", "--no-prompt", "s.md", "--project", "P"}} {
		var o, e bytes.Buffer
		if code := mainRun(append([]string{"--db", db}, a...), &o, &e); code != 0 {
			t.Fatalf("setup %v: exit %d: %s", a, code, e.String())
		}
	}
	var out, errOut bytes.Buffer
	code := mainRun([]string{"--db", db, "story", "check", "--project", "P", "--strict"}, &out, &errOut)
	if code != 1 || !strings.Contains(out.String(), "unparseable-time") || errOut.Len() == 0 {
		t.Errorf("exit %d, stdout %q, stderr %q; want 1, the report, and a message", code, out.String(), errOut.String())
	}
}
