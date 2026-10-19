package knowledge

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"
)

// ─── learns and knows (DR-0083 items 1 to 4 and 6) ───────────────────────────

const knowledgeScene = "---\ntitle: \"One\"\nevent: the-letter\nstory_time: 1969-10-29\n" +
	"learns:\n  - \"Anna: the letter is forged\"\n  - \"Bram: The  Letter is forged\"\n" +
	"knows: \"Cass: the debt\"\n---\n\nAnna read the letter.\n"

// knowledgeRow is one row of story_knowledge, read straight from the table.
type knowledgeRow struct {
	Label, Character, Fact, Key, How string
}

func knowledgeRows(t *testing.T, kb *KnowledgeBase) []knowledgeRow {
	t.Helper()
	rows, err := kb.db.Query(`SELECT e.label, k.character, k.fact, k.fact_key, k.how
		FROM story_knowledge k JOIN story_events e ON e.id = k.event_id ORDER BY k.id`)
	if err != nil {
		t.Fatalf("query story_knowledge: %v", err)
	}
	defer rows.Close()
	var out []knowledgeRow
	for rows.Next() {
		var r knowledgeRow
		if err := rows.Scan(&r.Label, &r.Character, &r.Fact, &r.Key, &r.How); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func TestParseDocumentFile_FrontmatterLearnsAndKnows(t *testing.T) {
	p, err := ParseDocumentFile(writeTempFile(t, "one.md", knowledgeScene), "markdown")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Anna: the letter is forged", "Bram: The  Letter is forged"}; !reflect.DeepEqual(p.Story.Learns, want) {
		t.Errorf("Learns = %q, want %q", p.Story.Learns, want)
	}
	if want := []string{"Cass: the debt"}; !reflect.DeepEqual(p.Story.Knows, want) {
		t.Errorf("Knows = %q, want %q (a single value reads as a one-item list)", p.Story.Knows, want)
	}
}

func TestStoryMeta_LearnsOrKnowsAloneIsNarrative(t *testing.T) {
	for _, m := range []StoryMeta{{Learns: []string{"A: x"}}, {Knows: []string{"A: x"}}} {
		if m.IsZero() {
			t.Errorf("%+v is zero", m)
		}
		if !m.isNarrative() {
			t.Errorf("%+v is not narrative, so a file with only it would not be an event", m)
		}
	}
}

func TestParseDocumentFile_AnItemWithNoCharacterOrFactWarnsAndIsDropped(t *testing.T) {
	src := "---\ntitle: T\nlearns: [\"no colon here\", \": no character\", \"Anna:\", \"Anna: the debt\"]\n---\n\ntext\n"
	p, err := ParseDocumentFile(writeTempFile(t, "bad.md", src), "markdown")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Anna: the debt"}; !reflect.DeepEqual(p.Story.Learns, want) {
		t.Errorf("Learns = %q, want %q", p.Story.Learns, want)
	}
	n := 0
	for _, w := range p.StoryWarnings {
		if strings.Contains(w, "learns") {
			n++
		}
	}
	if n != 3 {
		t.Errorf("want 3 warnings naming learns, got %d in %q", n, p.StoryWarnings)
	}
}

func TestParseStoryBoneyard_LearnsAndKnowsRepeatAndKeepTheirCommas(t *testing.T) {
	m, warns, ok := ParseStoryBoneyard("/* story: pov=Anna; learns=Anna: the letter, which Cass wrote, is forged; learns=Bram: the debt; knows=Cass: the debt */")
	if !ok || len(warns) != 0 {
		t.Fatalf("ok=%v warns=%q", ok, warns)
	}
	if want := []string{"Anna: the letter, which Cass wrote, is forged", "Bram: the debt"}; !reflect.DeepEqual(m.Learns, want) {
		t.Errorf("Learns = %q, want %q", m.Learns, want)
	}
	if want := []string{"Cass: the debt"}; !reflect.DeepEqual(m.Knows, want) {
		t.Errorf("Knows = %q, want %q", m.Knows, want)
	}
}

func TestParseStoryComment_LearnsAndKnowsRepeat(t *testing.T) {
	m, id, warns, ok := ParseStoryComment("<!-- story: id=12A; learns=Anna: a, b; learns=Bram: c; knows=Cass: d -->")
	if !ok || id != "12A" || len(warns) != 0 {
		t.Fatalf("ok=%v id=%q warns=%q", ok, id, warns)
	}
	if want := []string{"Anna: a, b", "Bram: c"}; !reflect.DeepEqual(m.Learns, want) {
		t.Errorf("Learns = %q, want %q", m.Learns, want)
	}
	if want := []string{"Cass: d"}; !reflect.DeepEqual(m.Knows, want) {
		t.Errorf("Knows = %q, want %q", m.Knows, want)
	}
}

// DR-0083 item 3: only learns and knows may repeat. Any other key repeated is
// not reported today (the last value wins, silently); this pins that, so a change
// is a decision and not an accident.
func TestParseStoryComment_ARepeatOfAnyOtherKeyStillTakesTheLastValue(t *testing.T) {
	m, _, warns, ok := ParseStoryComment("<!-- story: pov=Anna; pov=Bram -->")
	if !ok || m.POV != "Bram" || len(warns) != 0 {
		t.Errorf("POV=%q warns=%q ok=%v", m.POV, warns, ok)
	}
}

func TestStoryKnowledge_AProseSceneFileWritesRowsWithAFoldedFactKey(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "one.md", knowledgeScene))
	want := []knowledgeRow{
		{"the-letter", "Anna", "the letter is forged", "the letter is forged", "learns"},
		{"the-letter", "Bram", "The  Letter is forged", "the letter is forged", "learns"},
		{"the-letter", "Cass", "the debt", "the debt", "knows"},
	}
	if got := knowledgeRows(t, kb); !reflect.DeepEqual(got, want) {
		t.Errorf("rows = %+v\nwant   %+v", got, want)
	}
}

func TestStoryKnowledge_ARepeatedLearnsKeepsTheFirstSilently(t *testing.T) {
	src := "---\ntitle: T\nevent: e\nlearns: [\"Anna: the debt\", \"anna:  The debt\", \"Bram: the debt\"]\nknows: [\"Anna: the debt\"]\n---\n\ntext\n"
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	res := ingestStory(t, kb, pid, writeTempFile(t, "t.md", src))
	got := knowledgeRows(t, kb)
	var learns, knows int
	for _, r := range got {
		switch r.How {
		case "learns":
			learns++
		case "knows":
			knows++
		}
	}
	if learns != 2 || knows != 1 {
		t.Errorf("learns=%d knows=%d, want 2 and 1 (a knows is not folded into a learns): %+v", learns, knows, got)
	}
	if got[0].Fact != "the debt" {
		t.Errorf("first learns = %+v, want the first spelling kept", got[0])
	}
	for _, w := range res.StoryWarnings {
		if strings.Contains(w, "debt") {
			t.Errorf("a repeated learns raised a warning: %q", w)
		}
	}
}

func TestStoryKnowledge_AReIngestReplacesRowsAndDeletingTheDocumentRemovesThem(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	path := writeTempFile(t, "one.md", knowledgeScene)
	ingestStory(t, kb, pid, path)
	if len(knowledgeRows(t, kb)) != 3 {
		t.Fatal("setup: want 3 rows")
	}
	changed := strings.Replace(knowledgeScene, "learns:\n  - \"Anna: the letter is forged\"\n  - \"Bram: The  Letter is forged\"\n", "learns: \"Dot: the key\"\n", 1)
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	ingestStory(t, kb, pid, path)
	got := knowledgeRows(t, kb)
	if len(got) != 2 || got[0].Character != "Dot" {
		t.Errorf("rows after re-ingest = %+v, want Dot's and Cass's only", got)
	}
	docs, err := kb.Documents(pid)
	if err != nil || len(docs) != 1 {
		t.Fatalf("Documents = %+v, %v", docs, err)
	}
	if _, err := kb.DeleteDocument(docs[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if got := knowledgeRows(t, kb); len(got) != 0 {
		t.Errorf("rows after DeleteDocument = %+v, want none", got)
	}
}

func TestStoryKnowledge_AMarkdownSceneOwnsItsRowsAndFrontMatterIsIgnored(t *testing.T) {
	src := "---\ntitle: C\nlearns: [\"Zed: ignored\"]\nknows: [\"Zed: also ignored\"]\n---\n\n" +
		"## The fire {#1A}\n<!-- story: story_time=1969-10-29; learns=Anna: the fire; knows=Bram: the lamp -->\nText.\n\n" +
		"## The fall {#1B}\n<!-- story: story_time=1969-10-30; learns=Bram: the fall -->\nText.\n"
	p, err := ParseDocumentFile(writeTempFile(t, "c.md", src), "markdown")
	if err != nil {
		t.Fatal(err)
	}
	var fm int
	for _, w := range p.StoryWarnings {
		if strings.HasPrefix(w, "front matter learns is ignored") || strings.HasPrefix(w, "front matter knows is ignored") {
			fm++
		}
	}
	if fm != 2 {
		t.Errorf("want a front-matter-ignored warning for learns and for knows, got %d in %q", fm, p.StoryWarnings)
	}
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "c.md", src))
	want := []knowledgeRow{
		{"1A", "Anna", "the fire", "the fire", "learns"},
		{"1A", "Bram", "the lamp", "the lamp", "knows"},
		{"1B", "Bram", "the fall", "the fall", "learns"},
	}
	if got := knowledgeRows(t, kb); !reflect.DeepEqual(got, want) {
		t.Errorf("rows = %+v\nwant   %+v", got, want)
	}
	for _, w := range kindsOf(t, kb, pid) {
		if w == "front-matter-ignored" {
			return
		}
	}
	t.Errorf("StoryCheck did not report front-matter-ignored")
}

func kindsOf(t *testing.T, kb *KnowledgeBase, pid int64) []string {
	t.Helper()
	fs, err := kb.StoryCheck(pid)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range fs {
		out = append(out, f.Kind)
	}
	return out
}

func TestStoryKnowledge_AFountainSceneBoneyardWritesRows(t *testing.T) {
	src := "INT. HALL #1#\n\n/* story: story_time=1969-10-29; learns=Anna: the letter, forged; learns=Bram: the debt */\n\nAnna reads.\n"
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	ingestStory(t, kb, pid, writeTempFile(t, "s.fountain", src))
	got := knowledgeRows(t, kb)
	if len(got) != 2 || got[0].Fact != "the letter, forged" || got[1].Character != "Bram" || got[0].Label != "1" {
		t.Errorf("rows = %+v", got)
	}
}

// ─── story_knowledge is derived and does not travel (DR-0083 item 4) ─────────

func TestStoryKnowledge_DoesNotTravelAndAnIngestRebuildsItOnTheOtherMachine(t *testing.T) {
	src := openTestKB(t)
	pid, _ := src.AddProject("work", "")
	path := writeTempFile(t, "one.md", knowledgeScene)
	ingestStory(t, src, pid, path)

	var buf bytes.Buffer
	if err := ExportJSONL(src, &buf, ""); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"story_knowledge", "fact_key", "the letter is forged"} {
		if strings.Contains(buf.String(), s) {
			t.Errorf("export carries story knowledge (%q): %s", s, buf.String())
		}
	}
	dst := openTestKB(t)
	if _, err := ImportJSONL(dst, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("ImportJSONL: %v", err)
	}
	if got := knowledgeRows(t, dst); len(got) != 0 {
		t.Fatalf("import brought rows: %+v", got)
	}
	p, _ := dst.ProjectByName("work")
	ingestStory(t, dst, p.ID, path)
	if got := knowledgeRows(t, dst); len(got) != 3 {
		t.Errorf("rows after re-ingest = %+v, want 3 rebuilt", got)
	}
}

func TestStoryKnowledge_IsNotMerged(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	pa, _ := a.AddProject("work", "")
	ingestStory(t, a, pa, writeTempFile(t, "one.md", knowledgeScene))
	pb, _ := b.AddProject("other", "")
	ingestStory(t, b, pb, writeTempFile(t, "two.md", knowledgeScene))
	merged := openMergedTestKB(t, a, b)
	if got := knowledgeRows(t, merged); len(got) != 0 {
		t.Errorf("merge brought rows: %+v", got)
	}
}

func TestDiffDatabases_StoryKnowledgeIsNotCompared(t *testing.T) {
	a := diffFixture(t, "a")
	b := twin(t, a)
	mutate(t, a, func(kb *KnowledgeBase) {
		pid, _ := kb.AddProject("work", "")
		ingestStory(t, kb, pid, writeTempFile(t, "one.md", knowledgeScene))
	})
	diffs, err := DiffDatabases(a, b)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diffs {
		if d.Table == "story_knowledge" {
			t.Errorf("check-db compares story_knowledge: %+v", d)
		}
	}
}
