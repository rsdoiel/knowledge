package knowledge

import (
	"strings"
	"testing"
)

// ─── the story timeline (DR-0075 item 9, DR-0079) ────────────────────────────

func labels(es []TimelineEntry) string {
	var out []string
	for _, e := range es {
		out = append(out, e.Event.Label)
	}
	return strings.Join(out, " ")
}

func scene(label, time string, extra string) string {
	return "---\ntitle: " + label + "\nevent: " + label + "\nstory_time: \"" + time + "\"\n" + extra + "---\n\ntext\n"
}

func timelineOf(t *testing.T, kb *KnowledgeBase, pid int64, narrative bool) []TimelineEntry {
	t.Helper()
	es, err := kb.StoryTimeline(pid, narrative)
	if err != nil {
		t.Fatalf("StoryTimeline: %v", err)
	}
	return es
}

func TestStoryTimeline_StoryOrderDiffersFromNarrativeOrder(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	// Written (and ingested) in the order a, b, c; the story runs b, a, c.
	ingestStory(t, kb, pid, writeTempFile(t, "a.md", scene("a", "1970-05-01", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "b.md", scene("b", "1969-10-29", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "c.md", scene("c", "1971-01-01", "")))

	if got := labels(timelineOf(t, kb, pid, false)); got != "b a c" {
		t.Errorf("story order = %q, want b a c", got)
	}
	if got := labels(timelineOf(t, kb, pid, true)); got != "a b c" {
		t.Errorf("narrative order = %q, want a b c", got)
	}
	es := timelineOf(t, kb, pid, false)
	for i, e := range es {
		if e.Unordered || e.Undated || e.Rank != i+1 || e.DocumentPath == "" {
			t.Errorf("entry %d = %+v, want ranked, ordered, dated, with a path", i, e)
		}
	}
}

func TestStoryTimeline_OverlapIsUnorderedNeverGuessed(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "y.md", scene("year", "1969", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "d.md", scene("day", "1969-10-29", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "z.md", scene("later", "1970-01-01", "")))
	es := timelineOf(t, kb, pid, false)
	if len(es) != 3 {
		t.Fatalf("entries = %s", labels(es))
	}
	if es[0].Unordered {
		t.Error("the first entry has nothing to be unordered against")
	}
	if !es[1].Unordered {
		t.Errorf("%s after %s: a day inside a year is unordered, got %+v", es[1].Event.Label, es[0].Event.Label, es[1])
	}
	if es[2].Unordered {
		t.Errorf("1970 follows 1969 provably, got %+v", es[2])
	}
}

func TestStoryTimeline_EdgesPlaceAnEventWithoutADate(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "w.md", scene("war", "1969-10-29", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "p.md", scene("prelude", "before: war", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "e.md", scene("epilogue", "after: war", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "x.md", scene("mid", "1970", "")))
	if got := labels(timelineOf(t, kb, pid, false)); got != "prelude war epilogue mid" && got != "prelude war mid epilogue" {
		t.Errorf("order = %q, want prelude first and war before epilogue", got)
	}
	es := timelineOf(t, kb, pid, false)
	if es[0].Event.Label != "prelude" || es[1].Event.Label != "war" {
		t.Errorf("order = %s, want prelude before war", labels(es))
	}
}

func TestStoryTimeline_PrecedesIsAnEdge(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "1.md", "---\nevent: second\npov: A\n---\n\nx\n"))
	ingestStory(t, kb, pid, writeTempFile(t, "2.md", "---\nevent: first\npov: A\nprecedes: [second]\n---\n\nx\n"))
	if got := labels(timelineOf(t, kb, pid, false)); got != "first second" {
		t.Errorf("order = %q, want first second though second was ingested first", got)
	}
}

func TestStoryTimeline_NoEvidenceMeansUndatedAndLast(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "u.md", "---\nevent: lost\npov: A\n---\n\nx\n"))
	ingestStory(t, kb, pid, writeTempFile(t, "d.md", scene("dated", "1969", "")))
	es := timelineOf(t, kb, pid, false)
	if labels(es) != "dated lost" || !es[1].Undated || es[0].Undated {
		t.Errorf("entries = %+v, want dated then the undated one, flagged", es)
	}
}

func TestStoryTimeline_OrdinalAndCalendarAreUnordered(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "o.md", scene("ord", "ordinal: 5", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "c.md", scene("cal", "1969-10-29", "")))
	es := timelineOf(t, kb, pid, false)
	if len(es) != 2 || !es[1].Unordered {
		t.Errorf("entries = %+v, want the pair unordered: the two kinds of time do not compare", es)
	}
}

func TestStoryTimeline_OrdinalsOrderNumerically(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	for i, n := range []string{"10", "9", "-3"} {
		ingestStory(t, kb, pid, writeTempFile(t, "o"+string(rune('a'+i))+".md", scene("n"+n, "ordinal: "+n, "")))
	}
	if got := labels(timelineOf(t, kb, pid, false)); got != "n-3 n9 n10" {
		t.Errorf("order = %q, want n-3 n9 n10 (numeric, not text)", got)
	}
}

func TestStoryTimeline_ACycleIsFlaggedAndStillListed(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "a.md", scene("a", "before: b", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "b.md", scene("b", "before: a", "")))
	es := timelineOf(t, kb, pid, false)
	if len(es) != 2 || !es[0].InCycle || !es[1].InCycle {
		t.Errorf("entries = %+v, want both listed and flagged as in a cycle", es)
	}
}

// Found by running kb story timeline on a small manuscript: an event placed
// only by a before:/after: relation was marked as having no relation at all.
func TestStoryTimeline_AnEventPlacedByARelationIsNotUndated(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "w.md", scene("war", "1969-10-29", "")))
	ingestStory(t, kb, pid, writeTempFile(t, "p.md", scene("prelude", "before: war", "")))
	for _, e := range timelineOf(t, kb, pid, false) {
		if e.Undated {
			t.Errorf("%s is Undated, but %s", e.Event.Label, "a relation or a date places it")
		}
	}
	// In reading order, Undated still says the event has no interval of its own.
	var prelude TimelineEntry
	for _, e := range timelineOf(t, kb, pid, true) {
		if e.Event.Label == "prelude" {
			prelude = e
		}
	}
	if !prelude.Undated {
		t.Errorf("narrative order: prelude = %+v, want Undated (no interval of its own)", prelude)
	}
}
