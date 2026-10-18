package knowledge

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"
)

// ─── story_events (DR-0075, DR-0079) ─────────────────────────────────────────

func ingestStory(t *testing.T, kb *KnowledgeBase, pid int64, path string) DocumentIngestResult {
	t.Helper()
	res, err := kb.IngestDocument(pid, path, DocumentIngestOptions{})
	if err != nil {
		t.Fatalf("IngestDocument(%s): %v", path, err)
	}
	return res
}

func storyEvents(t *testing.T, kb *KnowledgeBase, pid int64) []StoryEvent {
	t.Helper()
	evs, err := kb.StoryEvents(pid)
	if err != nil {
		t.Fatalf("StoryEvents: %v", err)
	}
	return evs
}

func eventByLabel(t *testing.T, evs []StoryEvent, label string) StoryEvent {
	t.Helper()
	for _, e := range evs {
		if e.Label == label {
			return e
		}
	}
	t.Fatalf("no event labelled %q in %+v", label, evs)
	return StoryEvent{}
}

const sceneFile = "---\ntitle: \"One\"\nevent: the-war\nstory_time: 1969-10-29\nplace: Kitchen\npov: Anna\nstatus: draft\n" +
	"characters: [the-long-fire/Anna]\nforeshadows: [e3]\npays_off: [e1]\nprecedes: [chapter-two]\n---\n\nAnna woke.\n"

func TestStoryEvents_AProseSceneFileIsOneEvent(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	res := ingestStory(t, kb, pid, writeTempFile(t, "one.md", sceneFile))
	if res.EventsWritten != 1 {
		t.Errorf("EventsWritten = %d, want 1", res.EventsWritten)
	}
	evs := storyEvents(t, kb, pid)
	if len(evs) != 1 {
		t.Fatalf("events = %+v, want one", evs)
	}
	e := evs[0]
	if e.SceneID != "" || e.Label != "the-war" || e.State != "dated" || e.Time.Precision != "day" ||
		e.Time.Start != "1969-10-29" || e.Time.End != "1969-10-29" || e.StoryTimeText != "1969-10-29" {
		t.Errorf("event = %+v", e)
	}
	if e.Place != "Kitchen" || e.POV != "Anna" || e.Status != "draft" ||
		!reflect.DeepEqual(e.Characters, []string{"the-long-fire/Anna"}) ||
		!reflect.DeepEqual(e.Foreshadows, []string{"e3"}) || !reflect.DeepEqual(e.PaysOff, []string{"e1"}) ||
		!reflect.DeepEqual(e.Precedes, []string{"chapter-two"}) {
		t.Errorf("story keys = %+v", e)
	}
	if e.SectionID == 0 || e.ProjectID != pid || e.DocumentID == 0 {
		t.Errorf("ids = %+v, want the section, project and document set", e)
	}
}

func TestStoryEvents_NoStoryKeysNoEvent(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	res := ingestStory(t, kb, pid, writeTempFile(t, "p.md", "---\ntitle: P\n---\n\ntext\n"))
	if res.EventsWritten != 0 || len(storyEvents(t, kb, pid)) != 0 {
		t.Errorf("a file with no story keys made events: %+v", res)
	}
}

const numberedFountain = "Title: X\n\nINT. KITCHEN - DAY #12A#\n\n/* story: story_time=1969-10-29; pov=Anna */\n\nAnna.\n\n" +
	"INT. HALL - NIGHT #12B#\n\n/* story: event=the-fall; story_time=after: 12A +1d */\n\nBram.\n\n" +
	"INT. KITCHEN - DAY\n\nNo number here.\n"

func TestStoryEvents_FountainNumberedScenesAreEventsLabelledByNumber(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "s.fountain", numberedFountain))
	evs := storyEvents(t, kb, pid)
	if len(evs) != 2 {
		t.Fatalf("events = %+v, want 2: the unnumbered scene is not an event", evs)
	}
	a := eventByLabel(t, evs, "12A")
	if a.SceneID != "12A" || a.POV != "Anna" || a.Seq != 0 || a.SectionID == 0 {
		t.Errorf("12A = %+v", a)
	}
	b := eventByLabel(t, evs, "the-fall") // `event=` beats the scene number as the label
	if b.SceneID != "12B" || b.Seq != 1 {
		t.Errorf("the-fall = %+v", b)
	}
	if b.State != "resolved" || b.Time.Start != "1969-10-30" || b.AnchorEventID != a.ID {
		t.Errorf("the-fall should resolve to 1969-10-30 from 12A: %+v", b)
	}
}

func TestStoryEvents_NumberedSceneWithoutMetadataIsStillAnEvent(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "s.fountain", "INT. HALL #5#\n\nText.\n"))
	evs := storyEvents(t, kb, pid)
	if len(evs) != 1 || evs[0].Label != "5" || evs[0].State != "none" {
		t.Errorf("events = %+v, want one undated event labelled 5", evs)
	}
}

func TestStoryEvents_ReingestReplacesAndASkippedIngestRebuilds(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	path := writeTempFile(t, "one.md", sceneFile)
	ingestStory(t, kb, pid, path)

	// An unchanged file is "skipped" -- and still rebuilds its events, which is
	// what makes a second machine work after import or merge (DR-0079 item 5).
	if _, err := kb.db.Exec(`DELETE FROM story_events`); err != nil {
		t.Fatal(err)
	}
	res := ingestStory(t, kb, pid, path)
	if res.Action != "skipped" || res.EventsWritten != 1 || len(storyEvents(t, kb, pid)) != 1 {
		t.Errorf("skipped ingest: %+v, want skipped with the event rebuilt", res)
	}

	// A changed file replaces its event, not adds a second.
	os.WriteFile(path, []byte(strings.Replace(sceneFile, "pov: Anna", "pov: Bram", 1)), 0o644)
	res = ingestStory(t, kb, pid, path)
	evs := storyEvents(t, kb, pid)
	if res.Action != "updated" || len(evs) != 1 || evs[0].POV != "Bram" {
		t.Errorf("changed ingest: %+v, events %+v", res, evs)
	}

	// Removing the story keys removes the event.
	os.WriteFile(path, []byte("---\ntitle: One\n---\n\nAnna woke.\n"), 0o644)
	ingestStory(t, kb, pid, path)
	if evs := storyEvents(t, kb, pid); len(evs) != 0 {
		t.Errorf("events = %+v, want none once the story keys are gone", evs)
	}
}

func TestStoryEvents_DryRunWritesNothing(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	if _, err := kb.IngestDocument(pid, writeTempFile(t, "one.md", sceneFile), DocumentIngestOptions{DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if evs := storyEvents(t, kb, pid); len(evs) != 0 {
		t.Errorf("a dry run wrote events: %+v", evs)
	}
}

func relFile(label, time string) string {
	return "---\ntitle: " + label + "\nevent: " + label + "\nstory_time: \"" + time + "\"\n---\n\ntext " + label + "\n"
}

func TestStoryEvents_RelativeEventsResolveWhateverTheIngestOrder(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	later := writeTempFile(t, "later.md", relFile("later", "after: the-war +3d"))
	edge := writeTempFile(t, "edge.md", relFile("edge", "before: the-war"))
	anchor := writeTempFile(t, "war.md", relFile("the-war", "1969-10-29"))

	ingestStory(t, kb, pid, later)
	ingestStory(t, kb, pid, edge)
	evs := storyEvents(t, kb, pid)
	if l := eventByLabel(t, evs, "later"); l.State != "unresolved" || l.Note == "" || l.AnchorEventID != 0 {
		t.Errorf("before its anchor exists: %+v", l)
	}
	ingestStory(t, kb, pid, anchor)
	evs = storyEvents(t, kb, pid)
	war := eventByLabel(t, evs, "the-war")
	l := eventByLabel(t, evs, "later")
	if l.State != "resolved" || l.Time.Start != "1969-11-01" || l.Time.Precision != "day" || l.AnchorEventID != war.ID {
		t.Errorf("later after the anchor arrives: %+v", l)
	}
	e := eventByLabel(t, evs, "edge")
	if e.State != "edge" || e.Time.HasInterval() || e.AnchorEventID != war.ID {
		t.Errorf("edge: %+v, want an ordering edge to the-war with no interval", e)
	}
}

func TestStoryEvents_ChainsResolve(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "c.md", relFile("c", "after: b +1d")))
	ingestStory(t, kb, pid, writeTempFile(t, "b.md", relFile("b", "after: a +1d")))
	ingestStory(t, kb, pid, writeTempFile(t, "a.md", relFile("a", "1969-10-29")))
	if c := eventByLabel(t, storyEvents(t, kb, pid), "c"); c.State != "resolved" || c.Time.Start != "1969-10-31" {
		t.Errorf("c = %+v, want 1969-10-31 through b", c)
	}
}

func TestStoryEvents_DuplicateLabelAndCycleStayUnresolved(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "d1.md", relFile("dup", "1969-10-29")))
	ingestStory(t, kb, pid, writeTempFile(t, "d2.md", "---\ntitle: d2\nevent: dup\nstory_time: 1970\n---\n\nx\n"))
	ingestStory(t, kb, pid, writeTempFile(t, "u.md", relFile("user", "after: dup +1d")))
	if u := eventByLabel(t, storyEvents(t, kb, pid), "user"); u.State != "unresolved" || !strings.Contains(u.Note, "dup") {
		t.Errorf("user = %+v, want unresolved: the label names two events", u)
	}

	ingestStory(t, kb, pid, writeTempFile(t, "x.md", relFile("x", "after: y +1d")))
	ingestStory(t, kb, pid, writeTempFile(t, "y.md", relFile("y", "after: x +1d")))
	evs := storyEvents(t, kb, pid)
	for _, l := range []string{"x", "y"} {
		if e := eventByLabel(t, evs, l); e.State != "unresolved" {
			t.Errorf("%s = %+v, want unresolved in a cycle", l, e)
		}
	}
}

func TestStoryEvents_UnparseableStoryTimeIsKeptAndWarned(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	res := ingestStory(t, kb, pid, writeTempFile(t, "b.md", relFile("odd", "next tuesday")))
	e := eventByLabel(t, storyEvents(t, kb, pid), "odd")
	if e.State != "none" || e.StoryTimeText != "next tuesday" || e.Time.HasInterval() || e.Note == "" {
		t.Errorf("event = %+v, want it stored as written with no interval and a note", e)
	}
	if len(res.StoryWarnings) == 0 || !strings.Contains(strings.Join(res.StoryWarnings, " "), "next tuesday") {
		t.Errorf("StoryWarnings = %v, want the bad value named", res.StoryWarnings)
	}
}

func TestStoryEvents_RemovingADocumentRemovesItsEventsAndUnresolvesDependants(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "war.md", relFile("the-war", "1969-10-29")))
	ingestStory(t, kb, pid, writeTempFile(t, "l.md", relFile("later", "after: the-war +3d")))
	doc := kb.mustDocumentID(t, "war.md")
	if _, err := kb.DeleteDocument(doc, true); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	evs := storyEvents(t, kb, pid)
	if len(evs) != 1 || evs[0].Label != "later" || evs[0].State != "unresolved" {
		t.Errorf("events = %+v, want only later, now unresolved", evs)
	}
}

func (kb *KnowledgeBase) mustDocumentID(t *testing.T, suffix string) int64 {
	t.Helper()
	docs, err := kb.Documents(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if strings.HasSuffix(d.Path, suffix) {
			return d.ID
		}
	}
	t.Fatalf("no document %q", suffix)
	return 0
}

// ─── story_events is derived and does not travel (DR-0075 item 7, DR-0079 item 6) ───

func TestStoryEvents_DoNotTravelAndAnIngestRebuildsThemOnTheOtherMachine(t *testing.T) {
	src := openTestKB(t)
	pid, _ := src.AddProject("work", "")
	path := writeTempFile(t, "one.md", sceneFile)
	ingestStory(t, src, pid, path)
	if len(storyEvents(t, src, pid)) != 1 {
		t.Fatal("setup: the source has no event")
	}

	var buf bytes.Buffer
	if err := ExportJSONL(src, &buf, ""); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "story_event") || strings.Contains(buf.String(), "anchor_offset") {
		t.Errorf("export carries story events: %s", buf.String())
	}

	dst := openTestKB(t)
	if _, err := ImportJSONL(dst, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}
	p, err := dst.ProjectByName("work")
	if err != nil || p == nil {
		t.Fatalf("ProjectByName: %v %v", p, err)
	}
	if got := storyEvents(t, dst, p.ID); len(got) != 0 {
		t.Fatalf("import brought events: %+v", got)
	}
	// The document arrived with its checksum, so this ingest is a skip -- and
	// still rebuilds the events from the file.
	res := ingestStory(t, dst, p.ID, path)
	if res.Action != "skipped" {
		t.Fatalf("Action = %q, want skipped: the imported document carries its checksum", res.Action)
	}
	if got := storyEvents(t, dst, p.ID); len(got) != 1 || got[0].Label != "the-war" {
		t.Errorf("events after re-ingest = %+v, want the-war rebuilt", got)
	}
}

func TestStoryEvents_AreNotMerged(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	pa, _ := a.AddProject("work", "")
	ingestStory(t, a, pa, writeTempFile(t, "one.md", sceneFile))
	pb, _ := b.AddProject("other", "")
	ingestStory(t, b, pb, writeTempFile(t, "two.md", relFile("two", "1970")))

	merged := openMergedTestKB(t, a, b)
	if got := storyEvents(t, merged, 0); len(got) != 0 {
		t.Errorf("merge brought events: %+v", got)
	}
}

func TestDiffDatabases_StoryEventsAreNotCompared(t *testing.T) {
	a := diffFixture(t, "a")
	b := twin(t, a)
	mutate(t, a, func(kb *KnowledgeBase) {
		pid, _ := kb.AddProject("work", "")
		ingestStory(t, kb, pid, writeTempFile(t, "one.md", sceneFile))
	})
	diffs, err := DiffDatabases(a, b)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diffs {
		if d.Table == "story_events" {
			t.Errorf("check-db compares story_events: %+v", d)
		}
	}
}

// Found on real data: the foulpapers design brief has `status:` in its front
// matter and became a story event. status and place are ordinary front matter
// keys, so alone they do not make an event; they ride along on one.
func TestStoryEvents_StatusOrPlaceAloneIsNotAnEvent(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "brief.md", "---\ntitle: Brief\nstatus: draft\nplace: Pi\n---\n\ntext\n"))
	if evs := storyEvents(t, kb, pid); len(evs) != 0 {
		t.Errorf("events = %+v, want none", evs)
	}
	ingestStory(t, kb, pid, writeTempFile(t, "scene.md", "---\ntitle: S\nstatus: draft\npov: Anna\n---\n\ntext\n"))
	if evs := storyEvents(t, kb, pid); len(evs) != 1 || evs[0].Status != "draft" {
		t.Errorf("events = %+v, want one, carrying its status", evs)
	}
}

// ─── Markdown scenes (DR-0082) ───────────────────────────────────────────────

const markdownChapter = "---\ntitle: \"Chapter One\"\npov: Anna\nplace: Kitchen\nstatus: draft\ncharacters: [Anna, Bram]\n---\n\n" +
	"# Chapter One\n\nAn opening paragraph that is not a scene.\n\n" +
	"## The fire {#12A}\n<!-- story: story_time=1969-10-29 -->\nAnna woke.\n\n" +
	"## The fall\n<!-- story: id=12B; event=the-fall; story_time=after: 12A +1d; pov=Bram; characters=Bram -->\nBram fell.\n\n" +
	"## An interlude\n\nNo id here, so not a scene.\n"

func TestStoryEvents_MarkdownScenesAreEventsKeyedOnTheirID(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	res := ingestStory(t, kb, pid, writeTempFile(t, "ch1.md", markdownChapter))
	if res.EventsWritten != 2 {
		t.Errorf("EventsWritten = %d, want 2 (the chapter and the interlude have no id)", res.EventsWritten)
	}
	evs := storyEvents(t, kb, pid)
	if len(evs) != 2 {
		t.Fatalf("events = %+v, want 2", evs)
	}
	a := eventByLabel(t, evs, "12A") // no event=, so the id labels it
	if a.SceneID != "12A" || a.SectionID == 0 || a.State != "dated" || a.Time.Start != "1969-10-29" {
		t.Errorf("12A = %+v", a)
	}
	b := eventByLabel(t, evs, "the-fall")
	if b.SceneID != "12B" || b.Seq <= a.Seq || b.State != "resolved" || b.Time.Start != "1969-10-30" || b.AnchorEventID != a.ID {
		t.Errorf("the-fall = %+v", b)
	}
}

func TestStoryEvents_MarkdownFrontMatterIsTheDefaultAndTheSceneOverrides(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "ch1.md", markdownChapter))
	evs := storyEvents(t, kb, pid)
	a := eventByLabel(t, evs, "12A")
	if a.POV != "Anna" || a.Place != "Kitchen" || a.Status != "draft" || !reflect.DeepEqual(a.Characters, []string{"Anna", "Bram"}) {
		t.Errorf("12A should take every front matter default: %+v", a)
	}
	b := eventByLabel(t, evs, "the-fall")
	if b.POV != "Bram" || b.Place != "Kitchen" || b.Status != "draft" {
		t.Errorf("the-fall overrides pov only: %+v", b)
	}
	if !reflect.DeepEqual(b.Characters, []string{"Bram"}) {
		t.Errorf("a list in the scene replaces the front matter list, got %v", b.Characters)
	}
}

func TestStoryEvents_MarkdownFrontMatterTimeAndEventAreIgnoredWhenThereAreSceneIDs(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	src := "---\ntitle: C\nevent: whole-chapter\nstory_time: 1969\n---\n\n## One {#1}\n<!-- story: story_time=1970 -->\ntext\n"
	res := ingestStory(t, kb, pid, writeTempFile(t, "c.md", src))
	evs := storyEvents(t, kb, pid)
	if len(evs) != 1 || evs[0].Label != "1" || evs[0].StoryTimeText != "1970" {
		t.Fatalf("events = %+v, want one event 1 dated by its own scene", evs)
	}
	var event, tm bool
	for _, w := range res.StoryWarnings {
		event = event || strings.Contains(w, "front matter event")
		tm = tm || strings.Contains(w, "front matter story_time")
	}
	if !event || !tm {
		t.Errorf("warnings = %v, want one for event and one for story_time", res.StoryWarnings)
	}
}

func TestStoryEvents_MarkdownFileWithNoSceneIDIsStillOneEventFromFrontMatter(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	src := "---\ntitle: C\nevent: the-war\nstory_time: 1969\n---\n\n## Part one\ntext\n\n## Part two\nmore\n"
	ingestStory(t, kb, pid, writeTempFile(t, "c.md", src))
	evs := storyEvents(t, kb, pid)
	if len(evs) != 1 || evs[0].Label != "the-war" || evs[0].SceneID != "" {
		t.Errorf("events = %+v, want today's single event", evs)
	}
}

func TestStoryEvents_MarkdownRenamedHeadingKeepsTheEvent(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	path := writeTempFile(t, "c.md", "## The fire {#1}\n<!-- story: story_time=1969 -->\ntext\n")
	ingestStory(t, kb, pid, path)
	if err := os.WriteFile(path, []byte("## The great fire {#1}\n<!-- story: story_time=1969 -->\ntext\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ingestStory(t, kb, pid, path)
	evs := storyEvents(t, kb, pid)
	if len(evs) != 1 || evs[0].SceneID != "1" || evs[0].Label != "1" || evs[0].SectionID == 0 {
		t.Errorf("events = %+v, want the same event on the renamed section", evs)
	}
}

func TestStoryEvents_ASceneIDRepeatedInOneFileDoesNotFailTheIngest(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	src := "## One {#7}\n<!-- story: story_time=1969 -->\na\n\n## Two {#7}\n<!-- story: story_time=1970 -->\nb\n"
	res := ingestStory(t, kb, pid, writeTempFile(t, "dup.md", src))
	if len(storyEvents(t, kb, pid)) != 1 || res.EventsWritten != 1 {
		t.Errorf("events = %+v, result %+v: the first scene with an id is kept", storyEvents(t, kb, pid), res)
	}
	found := false
	for _, w := range res.StoryWarnings {
		found = found || strings.Contains(w, "scene id \"7\" is repeated")
	}
	if !found {
		t.Errorf("warnings = %v, want a repeated-scene-id warning", res.StoryWarnings)
	}
}

func TestStoryEvents_ASceneIDRepeatedInAFountainFileDoesNotFailTheIngest(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	res := ingestStory(t, kb, pid, writeTempFile(t, "d.fountain", "INT. A #3#\n\nx\n\nINT. B #3#\n\ny\n"))
	if len(storyEvents(t, kb, pid)) != 1 || res.EventsWritten != 1 || len(res.StoryWarnings) == 0 {
		t.Errorf("events = %+v, result %+v", storyEvents(t, kb, pid), res)
	}
}
