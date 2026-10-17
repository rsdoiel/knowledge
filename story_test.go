package knowledge

import (
	"reflect"
	"strings"
	"testing"
)

// ─── story front matter and per-scene metadata (DR-0075, DR-0078) ───────────

func TestParseDocumentFile_StoryFrontmatter(t *testing.T) {
	src := "---\ntitle: \"Chapter One\"\nstory_time: 1969-10-29\nplace: Anna's kitchen\npov: Anna\n" +
		"status: draft\ncharacters: [the-long-fire/Anna, the-long-fire/Bram]\nforeshadows: [e12]\n" +
		"pays_off: [e3]\nprecedes: [chapter-two]\nsomethingElse: ignored\n---\n\nAnna woke.\n"
	pd, err := ParseDocumentFile(writeTempFile(t, "c1.md", src), "")
	if err != nil {
		t.Fatalf("ParseDocumentFile: %v", err)
	}
	want := StoryMeta{
		StoryTime: "1969-10-29", Place: "Anna's kitchen", POV: "Anna", Status: "draft",
		Characters:  []string{"the-long-fire/Anna", "the-long-fire/Bram"},
		Foreshadows: []string{"e12"}, PaysOff: []string{"e3"}, Precedes: []string{"chapter-two"},
	}
	if !reflect.DeepEqual(pd.Story, want) {
		t.Errorf("Story = %+v, want %+v", pd.Story, want)
	}
	if len(pd.StoryWarnings) != 0 {
		t.Errorf("StoryWarnings = %v, want none", pd.StoryWarnings)
	}
}

func TestParseDocumentFile_NoStoryKeysIsZero(t *testing.T) {
	pd, err := ParseDocumentFile(writeTempFile(t, "n.md", "---\ntitle: \"Plain\"\n---\n\nbody\n"), "")
	if err != nil {
		t.Fatalf("ParseDocumentFile: %v", err)
	}
	if !pd.Story.IsZero() {
		t.Errorf("Story = %+v, want zero", pd.Story)
	}
}

const storyFountain = "Title: The Long Fire\n\nINT. KITCHEN - DAY #12A#\n\n" +
	"/* story: story_time=1969-10-29; pov=Anna; characters=the-long-fire/Anna, the-long-fire/Bram */\n\n" +
	"Anna enters.\n\nANNA\nHi.\n\nINT. KITCHEN - DAY\n\nNobody here.\n\n" +
	"/* just a boneyard */\n"

func TestParseDocumentFile_FountainSceneNumberAndStoryBoneyard(t *testing.T) {
	pd, err := ParseDocumentFile(writeTempFile(t, "s.fountain", storyFountain), "")
	if err != nil {
		t.Fatalf("ParseDocumentFile: %v", err)
	}
	if len(pd.Sections) != 2 {
		t.Fatalf("sections = %d, want 2: %+v", len(pd.Sections), pd.Sections)
	}
	a, b := pd.Sections[0], pd.Sections[1]
	if a.SceneID != "12A" {
		t.Errorf("scene 1 SceneID = %q, want 12A", a.SceneID)
	}
	if b.SceneID != "" {
		t.Errorf("scene 2 SceneID = %q, want empty (no number, no identity)", b.SceneID)
	}
	want := StoryMeta{
		StoryTime: "1969-10-29", POV: "Anna",
		Characters: []string{"the-long-fire/Anna", "the-long-fire/Bram"},
	}
	if !reflect.DeepEqual(a.Story, want) {
		t.Errorf("scene 1 Story = %+v, want %+v", a.Story, want)
	}
	if !b.Story.IsZero() {
		t.Errorf("scene 2 Story = %+v, want zero: the story boneyard belongs to scene 1 only", b.Story)
	}
	if strings.Contains(a.Body, "story:") || strings.Contains(a.Body, "story_time") {
		t.Errorf("scene 1 body still holds the story boneyard: %q", a.Body)
	}
	if !strings.Contains(a.Body, "Anna enters.") {
		t.Errorf("scene 1 body lost its action: %q", a.Body)
	}
	// An ordinary boneyard is cut material, kept out of the body like every
	// other Fountain annotation, and it is not story metadata.
	if strings.Contains(b.Body, "just a boneyard") || !b.Story.IsZero() {
		t.Errorf("scene 2 body = %q, Story = %+v, want the ordinary boneyard dropped and no story", b.Body, b.Story)
	}
}

func TestParseDocumentFile_FountainBoneyardUnknownKeyWarns(t *testing.T) {
	src := "INT. HALL #1#\n\n/* story: story_tme=1969; pov=Anna; bare */\n\nText.\n"
	pd, err := ParseDocumentFile(writeTempFile(t, "w.fountain", src), "")
	if err != nil {
		t.Fatalf("ParseDocumentFile: %v", err)
	}
	if pd.Sections[0].Story.POV != "Anna" {
		t.Errorf("POV = %q, want the valid pair kept", pd.Sections[0].Story.POV)
	}
	joined := strings.Join(pd.StoryWarnings, "\n")
	if !strings.Contains(joined, "story_tme") || !strings.Contains(joined, "bare") {
		t.Errorf("StoryWarnings = %q, want both the unknown key and the bare word named", joined)
	}
}

func TestParseDocumentFile_FrontmatterUnknownStoryTypeWarnsNotFails(t *testing.T) {
	src := "---\ntitle: T\ncharacters: not-a-list\n---\n\nbody\n"
	pd, err := ParseDocumentFile(writeTempFile(t, "x.md", src), "")
	if err != nil {
		t.Fatalf("ParseDocumentFile: %v", err)
	}
	if !reflect.DeepEqual(pd.Story.Characters, []string{"not-a-list"}) {
		t.Errorf("Characters = %v, want a single scalar accepted as a one-item list", pd.Story.Characters)
	}
}

func TestParseDocumentFile_FountainMultiLineStoryBoneyard(t *testing.T) {
	src := "INT. HALL #2#\n\n/* story:\n   story_time=circa 1890;\n   place=the hall\n*/\n\nText.\n"
	pd, err := ParseDocumentFile(writeTempFile(t, "m.fountain", src), "")
	if err != nil {
		t.Fatalf("ParseDocumentFile: %v", err)
	}
	got := pd.Sections[0]
	if got.Story.StoryTime != "circa 1890" || got.Story.Place != "the hall" {
		t.Errorf("Story = %+v, want the multi-line boneyard read", got.Story)
	}
	if strings.Contains(got.Body, "story") || got.Body != "Text." {
		t.Errorf("Body = %q, want %q", got.Body, "Text.")
	}
}
