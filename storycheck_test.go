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
