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

func TestCmdStory_CheckReportsAMarkdownSceneProblemAndStrictGatesIt(t *testing.T) {
	kb, _ := storyKB(t, map[string]string{"a.md": "## The fire\n<!-- story: pov=Anna -->\ntext\n"})
	var out bytes.Buffer
	if err := cmdStory(kb, nil, false, []string{"check", "--project", "work"}, &out); err != nil {
		t.Fatalf("story check (advisory) = %v, want exit 0", err)
	}
	if !strings.Contains(out.String(), "scene-comment-no-id") {
		t.Errorf("output = %q, want the scene-comment-no-id report", out.String())
	}
	out.Reset()
	err := cmdStory(kb, nil, false, []string{"check", "--project", "work", "--strict"}, &out)
	if err == nil || exitCodeFor(err) != classNegative {
		t.Errorf("story check --strict = %v, want the negative class (exit 1)", err)
	}
}

// The manuals say what the Markdown scene syntax is and name every check kind
// (DR-0082): a writer finds {#id}, the story comment and each report there.
func TestHelp_DescribesTheMarkdownSceneSyntaxAndEveryReport(t *testing.T) {
	for _, want := range []string{"{#12A}", "<!-- story:", "id=12A"} {
		if !strings.Contains(DocumentHelpText, want) {
			t.Errorf("kb document help does not mention %q", want)
		}
	}
	for _, kind := range []string{"scene-comment-no-id", "scene-id-disagreement", "scene-id-repeated",
		"scene-comment-syntax", "fountain-style-heading", "front-matter-ignored",
		"knowledge-before-learned", "unknown-fact", "fact-number-differs", "late-source"} {
		if !strings.Contains(StoryHelpText, kind) {
			t.Errorf("kb story help does not name the %s report", kind)
		}
	}
}

// The citation tables of DR-0081 are named where a person looks for what
// travels and what is counted.
func TestHelp_NamesTheCitationTables(t *testing.T) {
	for name, text := range map[string]string{
		"merge": MergeHelpText, "check-db": CheckDBHelpText, "stats": StatsHelpText,
	} {
		for _, want := range []string{"document_sources", "record_sources"} {
			if !strings.Contains(text, want) {
				t.Errorf("kb %s help does not mention %s", name, want)
			}
		}
	}
	// export and import speak of line types, which are singular.
	for name, text := range map[string]string{"export": ExportHelpText, "import": ImportHelpText} {
		for _, want := range []string{"document_source", "record_source"} {
			if !strings.Contains(text, want) {
				t.Errorf("kb %s help does not mention the %s line type", name, want)
			}
		}
	}
}

func TestHelp_DescribesLearnsAndKnowsAndTheirPortability(t *testing.T) {
	for name, c := range map[string]struct {
		text string
		want []string
	}{
		"document": {DocumentHelpText, []string{"learns", "knows", "the key repeats", "Character: fact"}},
		"story":    {StoryHelpText, []string{"learns", "knows"}},
		"export":   {ExportHelpText, []string{"story_knowledge"}},
		"import":   {ImportHelpText, []string{"story knowledge are not imported"}},
		"merge":    {MergeHelpText, []string{"story knowledge"}},
		"check-db": {CheckDBHelpText, []string{"story knowledge (DR-0083) are not compared"}},
	} {
		text := strings.Join(strings.Fields(c.text), " ")
		for _, w := range c.want {
			if !strings.Contains(text, w) {
				t.Errorf("kb %s help does not mention %q", name, w)
			}
		}
	}
}

// ─── kb story who-knows and who-learns (DR-0083 item 5) ──────────────────────

func knowsKB(t *testing.T) *knowledge.KnowledgeBase {
	t.Helper()
	kb, _ := storyKB(t, map[string]string{
		"a.md": sceneDoc("later", "2024-07-01", "learns: [\"Anna: the debt\"]\n"),
		"b.md": sceneDoc("earlier", "1998-05-02", "learns: [\"Anna: the letter is forged\", \"Cass: the letter is forged\"]\nknows: [\"Cass: the key\"]\n"),
		"c.md": "---\ntitle: c\nevent: floating\nlearns: [\"Anna: a secret\"]\n---\n\ntext\n",
	})
	return kb
}

func TestCmdStory_WhoKnowsAtASceneListsKnownAndUnorderedFacts(t *testing.T) {
	kb := knowsKB(t)
	var out bytes.Buffer
	if err := cmdStory(kb, nil, false, []string{"who-knows", "Anna", "later", "--project", "work"}, &out); err != nil {
		t.Fatalf("story who-knows: %v", err)
	}
	s := out.String()
	known, unordered, _ := strings.Cut(s, "Not ordered against")
	if !strings.Contains(known, "the letter is forged") || !strings.Contains(known, "the debt") || strings.Contains(known, "a secret") {
		t.Errorf("known part = %q", known)
	}
	if !strings.Contains(s, "Not ordered against later") || !strings.Contains(unordered, "a secret") {
		t.Errorf("output = %q, want the undated secret listed as not ordered against later", s)
	}
	// The scene may come after the flag.
	out.Reset()
	if err := cmdStory(kb, nil, false, []string{"who-knows", "--project", "work", "Anna", "earlier"}, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "the debt") || !strings.Contains(out.String(), "the letter is forged") {
		t.Errorf("at earlier = %q", out.String())
	}
}

func TestCmdStory_WhoKnowsWithNoSceneListsEverythingInStoryOrder(t *testing.T) {
	kb := knowsKB(t)
	var out bytes.Buffer
	if err := cmdStory(kb, nil, false, []string{"who-knows", "Anna", "--project", "work"}, &out); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	i, j := strings.Index(s, "the letter is forged"), strings.Index(s, "the debt")
	if i < 0 || j < 0 || i > j || strings.Contains(s, "Not ordered") {
		t.Errorf("output = %q, want the forgery (1998) before the debt (2024) and no unordered section", s)
	}
}

func TestCmdStory_WhoKnowsJSON(t *testing.T) {
	kb := knowsKB(t)
	var out bytes.Buffer
	if err := cmdStory(kb, nil, true, []string{"who-knows", "Anna", "earlier", "--project", "work"}, &out); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Character string `json:"character"`
		Scene     string `json:"scene"`
		Facts     []struct {
			Fact      string `json:"fact"`
			LearnedIn string `json:"learned_in"`
			Document  string `json:"document"`
		} `json:"facts"`
		Unordered []struct {
			Fact string `json:"fact"`
		} `json:"unordered"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json = %q: %v", out.String(), err)
	}
	if got.Character != "Anna" || got.Scene != "earlier" || len(got.Facts) != 1 || got.Facts[0].Fact != "the letter is forged" ||
		got.Facts[0].LearnedIn != "earlier" || got.Facts[0].Document == "" || len(got.Unordered) != 1 || got.Unordered[0].Fact != "a secret" {
		t.Errorf("json = %+v", got)
	}
	out.Reset()
	if err := cmdStory(kb, nil, true, []string{"who-knows", "Cass", "--project", "work"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"unordered": []`) {
		t.Errorf("empty lists should be [] and not null: %q", out.String())
	}
}

func TestCmdStory_WhoLearnsListsLearnersInStoryOrder(t *testing.T) {
	kb := knowsKB(t)
	var out bytes.Buffer
	if err := cmdStory(kb, nil, false, []string{"who-learns", "The letter is  FORGED", "--project", "work"}, &out); err != nil {
		t.Fatalf("story who-learns: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "Anna") || !strings.Contains(s, "Cass") || strings.Index(s, "Anna") > strings.Index(s, "Cass") || !strings.Contains(s, "earlier") {
		t.Errorf("output = %q", s)
	}
	out.Reset()
	if err := cmdStory(kb, nil, true, []string{"who-learns", "the debt", "--project", "work"}, &out); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Fact     string `json:"fact"`
		Learners []struct {
			Character string `json:"character"`
			LearnedIn string `json:"learned_in"`
			Document  string `json:"document"`
		} `json:"learners"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json = %q: %v", out.String(), err)
	}
	if got.Fact != "the debt" || len(got.Learners) != 1 || got.Learners[0].Character != "Anna" || got.Learners[0].LearnedIn != "later" || got.Learners[0].Document == "" {
		t.Errorf("json = %+v", got)
	}
}

func TestStory_KnowledgeExitCodes(t *testing.T) {
	noTerminal(t)
	files := map[string]string{
		"a.md":  sceneDoc("later", "2024-07-01", "learns: [\"Anna: the debt\"]\n"),
		"t1.md": sceneDoc("twin", "2024-08-01", ""),
		"t2.md": sceneDoc("twin", "2024-08-02", ""),
	}
	setup := [][]string{{"project", "add", "P", "d"}}
	for _, f := range []string{"a.md", "t1.md", "t2.md"} {
		setup = append(setup, []string{"document", "ingest", "--no-prompt", f, "--project", "P"})
	}
	for _, tc := range []verbCase{
		{name: "who-knows ok", setup: setup, files: files, args: []string{"story", "who-knows", "Anna", "--project", "P"}, want: classOK},
		{name: "who-knows at a scene ok", setup: setup, files: files, args: []string{"story", "who-knows", "Anna", "later", "--project", "P"}, want: classOK},
		{name: "who-knows no character", setup: setup, files: files, args: []string{"story", "who-knows", "--project", "P"}, want: classUsage},
		{name: "who-knows no project", setup: setup, files: files, args: []string{"story", "who-knows", "Anna"}, want: classUsage},
		{name: "who-knows surplus argument", setup: setup, files: files, args: []string{"story", "who-knows", "Anna", "later", "extra", "--project", "P"}, want: classUsage},
		{name: "who-knows bad flag", setup: setup, files: files, args: []string{"story", "who-knows", "Anna", "--project", "P", "--bogus"}, want: classUsage},
		{name: "who-knows --narrative is not taken", setup: setup, files: files, args: []string{"story", "who-knows", "Anna", "--project", "P", "--narrative"}, want: classUsage},
		{name: "who-knows ambiguous scene", setup: setup, files: files, args: []string{"story", "who-knows", "Anna", "twin", "--project", "P"}, want: classUsage},
		{name: "who-knows unknown character", setup: setup, files: files, args: []string{"story", "who-knows", "Nobody", "--project", "P"}, want: classNegative},
		{name: "who-knows unknown scene", setup: setup, files: files, args: []string{"story", "who-knows", "Anna", "nowhere", "--project", "P"}, want: classNegative},
		{name: "who-knows unknown project", args: []string{"story", "who-knows", "Anna", "--project", "nosuch"}, want: classNegative},
		{name: "who-learns ok", setup: setup, files: files, args: []string{"story", "who-learns", "the debt", "--project", "P"}, want: classOK},
		{name: "who-learns no fact", setup: setup, files: files, args: []string{"story", "who-learns", "--project", "P"}, want: classUsage},
		{name: "who-learns no project", setup: setup, files: files, args: []string{"story", "who-learns", "the debt"}, want: classUsage},
		{name: "who-learns surplus argument", setup: setup, files: files, args: []string{"story", "who-learns", "the debt", "extra", "--project", "P"}, want: classUsage},
		{name: "who-learns blank fact", setup: setup, files: files, args: []string{"story", "who-learns", "  ", "--project", "P"}, want: classUsage},
		{name: "who-learns unknown fact", setup: setup, files: files, args: []string{"story", "who-learns", "no such fact", "--project", "P"}, want: classNegative},
		{name: "who-learns unknown project", args: []string{"story", "who-learns", "the debt", "--project", "nosuch"}, want: classNegative},
	} {
		t.Run(tc.name, func(t *testing.T) { runVerbCase(t, tc) })
	}
}

func TestHelp_DescribesWhoKnowsAndWhoLearns(t *testing.T) {
	text := strings.Join(strings.Fields(StoryHelpText), " ")
	for _, want := range []string{"who-knows CHARACTER [SCENE] --project P", "who-learns FACT --project P", "story order",
		"knows rows are use", "Not ordered against SCENE", "source link --document", "--relationship evidence"} {
		if !strings.Contains(text, want) {
			t.Errorf("kb story help does not mention %q", want)
		}
	}
}

func TestCmdStory_CheckReportsAKnowledgeProblemAndStrictGatesIt(t *testing.T) {
	kb, _ := storyKB(t, map[string]string{
		"a.md": sceneDoc("early", "2024-01-01", "knows: [\"Anna: the debt\"]\n"),
		"b.md": sceneDoc("late", "2024-03-01", "learns: [\"Anna: the debt\"]\n"),
	})
	var out bytes.Buffer
	if err := cmdStory(kb, nil, false, []string{"check", "--project", "work"}, &out); err != nil {
		t.Fatalf("advisory check failed: %v", err)
	}
	if !strings.Contains(out.String(), "knowledge-before-learned") {
		t.Errorf("report = %q", out.String())
	}
	out.Reset()
	if err := cmdStory(kb, nil, false, []string{"check", "--project", "work", "--strict"}, &out); err == nil {
		t.Error("--strict with a knowledge finding must fail")
	}
}

// There is no directory form of `document ingest`; the manual shows the loop that
// brings a directory of notes in, with the editor offered file by file (T6).
func TestHelp_DocumentIngestShowsTheLoopForADirectoryOfNotes(t *testing.T) {
	text := strings.Join(strings.Fields(DocumentHelpText), " ")
	for _, want := range []string{"There is no directory form", `for f in notes/*.md; do {app_name} document ingest "$f" --project P; done`,
		"--no-prompt", "decision records", "S searches, with the scope of the screen you are on"} {
		if !strings.Contains(text, want) && !strings.Contains(strings.Join(strings.Fields(HelpText), " "), want) {
			t.Errorf("the help does not say %q", want)
		}
	}
}
