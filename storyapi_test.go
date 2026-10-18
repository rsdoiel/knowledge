package knowledge_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// A consumer such as Foul Papers imports package knowledge and nothing in
// cmd/kb (DR-0035, DR-0075 S5). This test is written from outside the package,
// so it can only use what is exported, and it drives the whole phase-1 flow:
// a manuscript is ingested, its characters are given a kind, and the story is
// listed in story order and reading order, checked, moved to a second machine
// and rebuilt there.

func writeManuscript(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{
		"01-fire.md": "---\ntitle: The Fire\nevent: the-fire\nstory_time: 1969-10-29\nplace: Kitchen\npov: Anna\n" +
			"characters: [the-long-fire/Anna, the-long-fire/Bram]\nprecedes: [aftermath]\n---\n\nAnna woke to the smell of smoke.\n",
		"02-aftermath.md": "---\ntitle: Aftermath\nevent: aftermath\nstory_time: \"after: the-fire +3d\"\npov: Bram\n---\n\nThree days on.\n",
		"00-prologue.md":  "---\ntitle: Prologue\nevent: prologue\nstory_time: \"before: the-fire\"\npov: Bram\n---\n\nBefore any of it.\n",
		"film.fountain":   "INT. KITCHEN - DAY #1#\n\n/* story: story_time=1969-10-29; place=Kitchen; characters=the-long-fire/Anna */\n\nAnna.\n\nINT. HALL - NIGHT\n\nNo number.\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

func TestStoryLibrary_TheWholePhaseOneFlowFromOutsideThePackage(t *testing.T) {
	dir := t.TempDir()
	writeManuscript(t, dir)
	kb, err := knowledge.Open(knowledge.DefaultPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()
	pid, err := kb.AddProject("the-long-fire", "a novel")
	if err != nil {
		t.Fatal(err)
	}

	// Look at a file's story metadata without writing anything.
	pd, err := knowledge.ParseDocumentFile(filepath.Join(dir, "01-fire.md"), "")
	if err != nil {
		t.Fatal(err)
	}
	if pd.Story.POV != "Anna" || pd.Story.Event != "the-fire" || pd.Story.IsZero() || len(pd.StoryWarnings) != 0 {
		t.Errorf("parsed story = %+v %v", pd.Story, pd.StoryWarnings)
	}

	for _, name := range []string{"00-prologue.md", "01-fire.md", "02-aftermath.md", "film.fountain"} {
		res, err := kb.IngestDocument(pid, filepath.Join(dir, name), knowledge.DocumentIngestOptions{})
		if err != nil {
			t.Fatalf("IngestDocument(%s): %v", name, err)
		}
		if res.EventsWritten != 1 {
			t.Errorf("%s: EventsWritten = %d, want 1", name, res.EventsWritten)
		}
	}

	// Characters are concepts with a kind.
	for _, c := range []string{"the-long-fire/Anna", "the-long-fire/Bram"} {
		if _, err := kb.AddConcept(c, ""); err != nil {
			t.Fatal(err)
		}
		if err := kb.SetConceptKind(c, "character"); err != nil {
			t.Fatal(err)
		}
	}
	chars, err := kb.ConceptsOfKind("character")
	if err != nil || len(chars) != 2 || !knowledge.ValidConceptKind(chars[0].Kind) {
		t.Errorf("characters = %+v, %v", chars, err)
	}

	// The events, in story order and in reading order.
	story, err := kb.StoryTimeline(pid, false)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, e := range story {
		order = append(order, e.Event.Label)
	}
	if got := strings.Join(order, " "); !strings.HasPrefix(got, "prologue the-fire") || !strings.HasSuffix(got, "aftermath") {
		t.Errorf("story order = %q, want prologue, the-fire ... aftermath", got)
	}
	reading, _ := kb.StoryTimeline(pid, true)
	if len(reading) != 4 || reading[0].Event.Label != "prologue" {
		t.Errorf("reading order = %+v", reading)
	}
	evs, err := kb.StoryEvents(pid)
	if err != nil || len(evs) != 4 {
		t.Fatalf("StoryEvents = %d, %v", len(evs), err)
	}
	for _, e := range evs {
		if e.Label == "aftermath" && (e.State != "resolved" || e.Time.Start != "1969-11-01") {
			t.Errorf("aftermath = %+v, want resolved to 1969-11-01", e)
		}
	}

	// The check reports the unnumbered screenplay scene and the dangling precedes.
	findings, err := kb.StoryCheck(pid)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, f := range findings {
		kinds[f.Kind] = true
	}
	if !kinds["unnumbered-scene"] || kinds["unresolved-time"] {
		t.Errorf("findings = %+v, want the unnumbered scene and nothing unresolved", findings)
	}

	// A second machine: events do not travel, a re-ingest of the same paths rebuilds them.
	var dump bytes.Buffer
	if err := knowledge.ExportJSONL(kb, &dump, ""); err != nil {
		t.Fatal(err)
	}
	other, err := knowledge.Open(knowledge.DefaultPath(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := knowledge.ImportJSONL(other, bytes.NewReader(dump.Bytes())); err != nil {
		t.Fatal(err)
	}
	p2, err := other.ProjectByName("the-long-fire")
	if err != nil || p2 == nil {
		t.Fatalf("project after import: %v %v", p2, err)
	}
	if got, _ := other.StoryEvents(p2.ID); len(got) != 0 {
		t.Fatalf("events travelled: %+v", got)
	}
	for _, name := range []string{"00-prologue.md", "01-fire.md", "02-aftermath.md", "film.fountain"} {
		if _, err := other.IngestDocument(p2.ID, filepath.Join(dir, name), knowledge.DocumentIngestOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := other.StoryEvents(p2.ID); len(got) != 4 {
		t.Errorf("after re-ingest: %d events, want 4", len(got))
	}

	// Removing a manuscript file's document removes its events.
	docs, _ := kb.Documents(pid)
	for _, d := range docs {
		if strings.HasSuffix(d.Path, "film.fountain") {
			if _, err := kb.DeleteDocument(d.ID, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got, _ := kb.StoryEvents(pid); len(got) != 3 {
		t.Errorf("after delete: %d events, want 3", len(got))
	}
}

// A Markdown chapter with several scenes (DR-0082) is driven through the same
// exported calls a consumer uses: parse it, ingest it, list and check its
// events. testdata/markdown-chapter.md is the worked example of the syntax.
func TestStoryLibrary_AMultiSceneMarkdownChapterFromOutsideThePackage(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "markdown-chapter.md"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "ch1.md")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	kb, err := knowledge.Open(knowledge.DefaultPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()
	pid, err := kb.AddProject("the-long-fire", "a novel")
	if err != nil {
		t.Fatal(err)
	}

	// Preview: the scene ids and per-scene metadata, writing nothing.
	pd, err := knowledge.ParseDocumentFile(path, "")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, s := range pd.Sections {
		if s.SceneID != "" {
			ids = append(ids, s.SceneID)
		}
	}
	if strings.Join(ids, ",") != "1A,1B,1C" || len(pd.StoryWarnings) != 0 {
		t.Fatalf("scene ids = %v, warnings = %v", ids, pd.StoryWarnings)
	}

	res, err := kb.IngestDocument(pid, path, knowledge.DocumentIngestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.EventsWritten != 3 || len(res.StoryWarnings) != 0 {
		t.Errorf("ingest = %+v, want 3 events and no warnings", res)
	}

	// The timeline puts them in story order, the fall resolved from the fire.
	entries, err := kb.StoryTimeline(pid, false)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, e := range entries {
		order = append(order, e.Event.Label)
	}
	if strings.Join(order, ",") != "1A,the-fall,1C" {
		t.Errorf("story order = %v, want 1A, the-fall, 1C", order)
	}
	evs, err := kb.StoryEvents(pid)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range evs {
		switch e.Label {
		case "1A":
			if e.POV != "Anna" || e.Place != "Kitchen" || e.Time.Start != "1969-10-29" {
				t.Errorf("1A = %+v: front matter defaults and its own date", e)
			}
		case "the-fall":
			if e.POV != "Bram" || e.Time.Start != "1969-10-30" {
				t.Errorf("the-fall = %+v", e)
			}
		case "1C":
			if e.Place != "Hospital" || e.POV != "Anna" || e.Time.Start != "1969-11-01" {
				t.Errorf("1C = %+v: its own place, the default pov", e)
			}
		}
	}

	findings, err := kb.StoryCheck(pid)
	if err != nil || len(findings) != 0 {
		t.Errorf("StoryCheck = %+v, %v, want a clean chapter", findings, err)
	}
}
