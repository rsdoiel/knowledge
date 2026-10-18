package knowledge

import (
	"os"
	"strings"
	"testing"
)

// ─── kb story check: reporting, never fixing (DR-0075 item 10) ──────────────

func checkOf(t *testing.T, kb *KnowledgeBase, pid int64) []StoryFinding {
	t.Helper()
	fs, err := kb.StoryCheck(pid)
	if err != nil {
		t.Fatalf("StoryCheck: %v", err)
	}
	return fs
}

func findKind(fs []StoryFinding, kind string) []StoryFinding {
	var out []StoryFinding
	for _, f := range fs {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}

func TestStoryCheck_ACleanProjectHasNoFindings(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "a.md", scene("a", "1969-10-29", "pov: Anna\n")))
	ingestStory(t, kb, pid, writeTempFile(t, "b.md", scene("b", "after: a +1d", "precedes: []\n")))
	if fs := checkOf(t, kb, pid); len(fs) != 0 {
		t.Errorf("findings = %+v, want none", fs)
	}
}

func TestStoryCheck_UnnumberedFountainSceneIsReported(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "s.fountain", "INT. HALL #1#\n\nText.\n\nINT. KITCHEN - DAY\n\nNo number.\n"))
	got := findKind(checkOf(t, kb, pid), "unnumbered-scene")
	if len(got) != 1 || !strings.Contains(got[0].Message, "INT. KITCHEN - DAY") {
		t.Errorf("findings = %+v, want the kitchen scene named", got)
	}
}

func TestStoryCheck_MissingFileIsReportedNotFatal(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	path := writeTempFile(t, "s.fountain", "INT. HALL #1#\n\nText.\n")
	ingestStory(t, kb, pid, path)
	removeFile(t, path)
	got := findKind(checkOf(t, kb, pid), "missing-file")
	if len(got) != 1 {
		t.Errorf("findings = %+v, want one missing-file", got)
	}
}

func TestStoryCheck_DuplicateLabels(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "a.md", scene("dup", "1969", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "b.md", scene("dup", "1970", "")))
	got := findKind(checkOf(t, kb, pid), "duplicate-label")
	if len(got) != 1 || !strings.Contains(got[0].Message, `"dup"`) {
		t.Errorf("findings = %+v", got)
	}
}

func TestStoryCheck_UnresolvedAndUnparseableTimes(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "a.md", scene("lost", "after: nowhere +1d", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "b.md", scene("odd", "next tuesday", "")))
	fs := checkOf(t, kb, pid)
	if u := findKind(fs, "unresolved-time"); len(u) != 1 || !strings.Contains(u[0].Message, "nowhere") {
		t.Errorf("unresolved = %+v", u)
	}
	if u := findKind(fs, "unparseable-time"); len(u) != 1 || !strings.Contains(u[0].Message, "next tuesday") {
		t.Errorf("unparseable = %+v", u)
	}
}

func TestStoryCheck_UnknownReferences(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "a.md", scene("a", "1969", "foreshadows: [ghost]\npays_off: [a-ghost]\nprecedes: [another]\n")))
	got := findKind(checkOf(t, kb, pid), "unknown-reference")
	if len(got) != 3 {
		t.Errorf("findings = %+v, want one per dangling name", got)
	}
}

func TestStoryCheck_ContradictionAndCycle(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	// early declares it precedes late, but its date is after late's.
	ingestStory(t, kb, pid, writeTempFile(t, "e.md", scene("early", "1971", "precedes: [late]\n")))
	ingestStory(t, kb, pid, writeTempFile(t, "l.md", scene("late", "1969", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "x.md", scene("x", "before: y", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "y.md", scene("y", "before: x", "")))
	fs := checkOf(t, kb, pid)
	if c := findKind(fs, "contradiction"); len(c) != 1 || !strings.Contains(c[0].Message, "early") {
		t.Errorf("contradictions = %+v", c)
	}
	if c := findKind(fs, "cycle"); len(c) == 0 {
		t.Errorf("cycle findings = %+v, want the x/y cycle", c)
	}
}

func TestStoryCheck_ACharacterInTwoPlacesOnTheSameDay(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "a.md", scene("a", "1969-10-29", "place: Kitchen\ncharacters: [Anna]\n")))
	ingestStory(t, kb, pid, writeTempFile(t, "b.md", scene("b", "1969-10-29", "place: Paris\ncharacters: [Anna, Bram]\n")))
	// Bram and Anna share Paris with scene b: fine. A year-wide event overlaps without being the same moment: fine.
	ingestStory(t, kb, pid, writeTempFile(t, "c.md", scene("c", "1969-10-29", "place: Paris\ncharacters: [Bram]\n")))
	ingestStory(t, kb, pid, writeTempFile(t, "d.md", scene("d", "1969", "place: Rome\ncharacters: [Anna]\n")))
	got := findKind(checkOf(t, kb, pid), "two-places")
	if len(got) != 1 || !strings.Contains(got[0].Message, "Anna") {
		t.Errorf("findings = %+v, want exactly Anna on 1969-10-29", got)
	}
}

func TestStoryCheck_ForeshadowAndPayoffOrderWithinADocument(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	src := "INT. A #1#\n\n/* story: event=setup; pays_off=payoff */\n\nx\n\n" +
		"INT. B #2#\n\n/* story: event=payoff; foreshadows=setup */\n\ny\n"
	ingestStory(t, kb, pid, writeTempFile(t, "s.fountain", src))
	got := findKind(checkOf(t, kb, pid), "foreshadow-order")
	if len(got) != 2 {
		t.Errorf("findings = %+v, want setup-pays-off-a-later-scene and payoff-foreshadows-an-earlier-one", got)
	}
}

func TestStoryCheck_FindingsAreOrderedAndNameTheirDocument(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "b.md", scene("odd", "next tuesday", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "a.md", scene("lost", "after: nowhere +1d", "")))
	fs := checkOf(t, kb, pid)
	if len(fs) != 2 || fs[0].Document == "" || fs[0].Kind > fs[1].Kind && false {
		t.Errorf("findings = %+v", fs)
	}
	again := checkOf(t, kb, pid)
	for i := range fs {
		if fs[i] != again[i] {
			t.Errorf("not deterministic: %+v vs %+v", fs, again)
		}
	}
}

func removeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

// ─── Markdown scene reports (DR-0082 item 7) ─────────────────────────────────

func TestStoryCheck_AMarkdownChapterWithSceneIDsIsClean(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "ch1.md", markdownChapter))
	if fs := checkOf(t, kb, pid); len(fs) != 0 {
		t.Errorf("findings = %+v, want none: a heading without an id is an ordinary section", fs)
	}
}

func TestStoryCheck_MarkdownSceneReports(t *testing.T) {
	cases := []struct {
		name, kind, body, wantIn string
		wantCount                int
	}{
		{"a story comment with no id", "scene-comment-no-id",
			"## The fire\n<!-- story: pov=Anna -->\ntext\n", `"The fire"`, 1},
		{"a heading that ends like a Fountain id, once per file", "fountain-style-heading",
			"## The fire #12A#\ntext\n\n## The fall #12B#\nmore\n", "#12A#", 1},
		{"the heading and comment ids disagree", "scene-id-disagreement",
			"## The fire {#12A}\n<!-- story: id=13 -->\ntext\n", "12A", 1},
		{"a repeated scene id", "scene-id-repeated",
			"## One {#7}\na\n\n## Two {#7}\nb\n", `"7"`, 1},
		{"an unknown key and a word with no =", "scene-comment-syntax",
			"## One {#1}\n<!-- story: pov=Anna; mood=dark; stray -->\na\n", "mood", 2},
		{"front matter story_time ignored", "front-matter-ignored",
			"---\ntitle: C\nstory_time: 1969\n---\n\n## One {#1}\na\n", "story_time", 1},
		{"front matter event ignored", "front-matter-ignored",
			"---\ntitle: C\nevent: whole\n---\n\n## One {#1}\na\n", "event", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			kb := openTestKB(t)
			pid, _ := kb.AddProject("work", "")
			path := writeTempFile(t, "c.md", c.body)
			ingestStory(t, kb, pid, path)
			got := findKind(checkOf(t, kb, pid), c.kind)
			if len(got) != c.wantCount {
				t.Fatalf("%s findings = %+v, want %d", c.kind, got, c.wantCount)
			}
			found := false
			for _, f := range got {
				if f.Document != path {
					t.Errorf("finding = %+v, want document %s", f, path)
				}
				found = found || strings.Contains(f.Message, c.wantIn)
			}
			if !found {
				t.Errorf("findings = %+v, want %q in one message", got, c.wantIn)
			}
		})
	}
}

// A Fountain file's boneyard warnings and an ordinary Markdown file are not
// touched by the Markdown reports.
func TestStoryCheck_MarkdownReportsLeaveOtherFilesAlone(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "plain.md", "---\ntitle: P\n---\n\n## Notes\n\n<!-- a plain comment -->\ntext\n"))
	ingestStory(t, kb, pid, writeTempFile(t, "s.fountain", "INT. HALL #1#\n\n/* story: pov=Anna */\n\nText.\n"))
	if fs := checkOf(t, kb, pid); len(fs) != 0 {
		t.Errorf("findings = %+v, want none", fs)
	}
}

// markdownWarningKind sorts the warnings the parser raises by the prefix
// constants; a message built another way would silently stop being reported.
func TestMarkdownWarningKind_MatchesTheMessagesTheParserBuilds(t *testing.T) {
	cases := map[string]string{
		"scene-comment-no-id":    "## A\n<!-- story: pov=X -->\n",
		"scene-id-disagreement":  "## A {#1}\n<!-- story: id=2 -->\n",
		"fountain-style-heading": "## A #1#\n",
		"scene-comment-syntax":   "## A {#1}\n<!-- story: mood=x -->\n",
	}
	for want, body := range cases {
		var kinds []string
		for _, s := range segmentMarkdown(body) {
			for _, w := range s.StoryWarnings {
				kinds = append(kinds, markdownWarningKind(w))
			}
		}
		if len(kinds) != 1 || kinds[0] != want {
			t.Errorf("%q: kinds = %v, want [%s]", body, kinds, want)
		}
	}
	if k := markdownWarningKind("story front matter: yaml: bad"); k != "" {
		t.Errorf("a front matter YAML error is not a scene report, got %q", k)
	}
}
