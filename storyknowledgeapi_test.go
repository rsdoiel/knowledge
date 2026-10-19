package knowledge_test

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// WhoKnows and WhoLearns (DR-0083 item 5), written from outside the package as
// a consumer such as Foul Papers would call them. The manuscript has a flashback
// (told second, happening first), two scenes on the same day, a scene with no
// time at all, a retelling, and a fact used as backstory.

func knowledgeManuscript(t *testing.T) (*knowledge.KnowledgeBase, int64) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"a-ch1.md": "---\ntitle: One\nevent: ch1\nstory_time: 2024-06-01\n" +
			"learns: [\"Anna: the safe is empty\"]\n" +
			"knows: [\"Anna: the letter is forged\", \"Cass: the debt\", \"Eve: the key\"]\n---\n\nAnna opened the safe.\n",
		"b-flashback.md": "---\ntitle: Flashback\nevent: flashback\nstory_time: 1998-05-02\n" +
			"learns: [\"Anna: the letter is forged\", \"Cass: the letter is forged\"]\n---\n\nCass wrote it.\n",
		"c-later.md": "---\ntitle: Later\nevent: later\nstory_time: 2024-07-01\n" +
			"learns: [\"Bram: the debt, which is large\", \"Anna: The  Letter is forged\"]\n---\n\nBram told her again.\n",
		"d-rumour.md": "---\ntitle: Rumour\nevent: rumour\nstory_time: 2024-06-01\n" +
			"learns: [\"Anna: the dog is dead\"]\n---\n\nThe same day.\n",
		"e-floating.md": "---\ntitle: Floating\nevent: floating\nlearns: [\"Dot: a secret\"]\n---\n\nNo time at all.\n",
		"film.fountain": "INT. HALL - NIGHT #7#\n\n/* story: story_time=2024-06-02; learns=Bram: the lamp is lit */\n\nBram.\n",
		"g-twin1.md":    "---\ntitle: T1\nevent: twin\nstory_time: 2024-08-01\nlearns: [\"Fay: x marks it\"]\n---\n\none\n",
		"h-twin2.md":    "---\ntitle: T2\nevent: twin\nstory_time: 2024-08-02\n---\n\ntwo\n",
	}
	names := make([]string, 0, len(files))
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	kb, err := knowledge.Open(knowledge.DefaultPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kb.Close() })
	pid, err := kb.AddProject("work", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if _, err := kb.IngestDocument(pid, filepath.Join(dir, name), knowledge.DocumentIngestOptions{}); err != nil {
			t.Fatalf("IngestDocument(%s): %v", name, err)
		}
	}
	return kb, pid
}

func factTexts(fs []knowledge.KnownFact) string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Fact+"@"+f.LearnedIn)
	}
	return strings.Join(out, " | ")
}

func TestWhoKnows_AllAFactsAreInStoryOrderAndARetellingIsNotRepeated(t *testing.T) {
	kb, pid := knowledgeManuscript(t)
	got, err := kb.WhoKnows(pid, "Anna", "")
	if err != nil {
		t.Fatal(err)
	}
	want := "the letter is forged@flashback | the safe is empty@ch1 | the dog is dead@rumour"
	if s := factTexts(got.Facts); s != want {
		t.Errorf("facts = %q\nwant    %q", s, want)
	}
	if len(got.Unordered) != 0 || got.Character != "Anna" || got.Scene != "" {
		t.Errorf("result = %+v", got)
	}
}

func TestWhoKnows_AtASceneItIncludesThatSceneAndLeavesOutTheLaterOnes(t *testing.T) {
	kb, pid := knowledgeManuscript(t)
	got, err := kb.WhoKnows(pid, "anna", "ch1") // the character's case does not matter
	if err != nil {
		t.Fatal(err)
	}
	if s, want := factTexts(got.Facts), "the letter is forged@flashback | the safe is empty@ch1"; s != want {
		t.Errorf("facts = %q, want %q", s, want)
	}
	if s, want := factTexts(got.Unordered), "the dog is dead@rumour"; s != want {
		t.Errorf("unordered = %q, want %q: same day as ch1, so not ordered against it", s, want)
	}
	if got.Scene != "ch1" {
		t.Errorf("Scene = %q", got.Scene)
	}

	// The flashback is told second but happens first: at it, Anna knows only the forgery.
	got, err = kb.WhoKnows(pid, "Anna", "flashback")
	if err != nil {
		t.Fatal(err)
	}
	if s, want := factTexts(got.Facts), "the letter is forged@flashback"; s != want || len(got.Unordered) != 0 {
		t.Errorf("at flashback: facts = %q unordered = %q, want %q and none", s, factTexts(got.Unordered), want)
	}
}

func TestWhoKnows_ASceneWithNoTimeIsNotOrderedAgainstAnything(t *testing.T) {
	kb, pid := knowledgeManuscript(t)
	got, err := kb.WhoKnows(pid, "Dot", "ch1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Facts) != 0 || factTexts(got.Unordered) != "a secret@floating" {
		t.Errorf("facts = %q unordered = %q", factTexts(got.Facts), factTexts(got.Unordered))
	}
}

func TestWhoKnows_ASceneIsNamedByLabelOrByAFountainSceneNumber(t *testing.T) {
	kb, pid := knowledgeManuscript(t)
	got, err := kb.WhoKnows(pid, "Bram", "7")
	if err != nil {
		t.Fatal(err)
	}
	if s, want := factTexts(got.Facts), "the lamp is lit@7"; s != want {
		t.Errorf("at scene 7: facts = %q, want %q (the debt is learned later)", s, want)
	}
}

func TestWhoKnows_BackstoryIsUseNotLearning(t *testing.T) {
	kb, pid := knowledgeManuscript(t)
	got, err := kb.WhoKnows(pid, "Cass", "")
	if err != nil {
		t.Fatal(err)
	}
	if s, want := factTexts(got.Facts), "the letter is forged@flashback"; s != want {
		t.Errorf("facts = %q, want %q: a knows-only fact is not listed", s, want)
	}
	// A character named only in knows is known, with nothing learned.
	eve, err := kb.WhoKnows(pid, "Eve", "")
	if err != nil || len(eve.Facts) != 0 {
		t.Errorf("Eve = %+v, %v, want an empty result and no error", eve, err)
	}
}

func TestWhoKnows_NotFoundAndAmbiguousAreTheirOwnErrors(t *testing.T) {
	kb, pid := knowledgeManuscript(t)
	if _, err := kb.WhoKnows(pid, "Nobody", ""); !errors.Is(err, knowledge.ErrNotFound) {
		t.Errorf("unknown character: err = %v, want ErrNotFound", err)
	}
	if _, err := kb.WhoKnows(pid, "Anna", "no-such-scene"); !errors.Is(err, knowledge.ErrNotFound) {
		t.Errorf("unknown scene: err = %v, want ErrNotFound", err)
	}
	_, err := kb.WhoKnows(pid, "Fay", "twin")
	if !errors.Is(err, knowledge.ErrAmbiguousScene) || !errors.Is(err, knowledge.ErrInvalid) {
		t.Errorf("ambiguous scene: err = %v, want ErrAmbiguousScene, which is also ErrInvalid (a bad command-line value)", err)
	}
	if err != nil && !strings.Contains(err.Error(), "twin") {
		t.Errorf("the message should name the scene: %v", err)
	}
}

func TestWhoLearns_ListsLearnersInStoryOrderMatchingByFactKey(t *testing.T) {
	kb, pid := knowledgeManuscript(t)
	got, err := kb.WhoLearns(pid, "The  LETTER is forged")
	if err != nil {
		t.Fatal(err)
	}
	var s []string
	for _, l := range got {
		s = append(s, l.Character+"@"+l.LearnedIn)
	}
	if want := "Anna@flashback Cass@flashback"; strings.Join(s, " ") != want {
		t.Errorf("learners = %q, want %q (Anna's retelling in a later scene is not a second learning)", strings.Join(s, " "), want)
	}
	if _, err := kb.WhoLearns(pid, "no such fact"); !errors.Is(err, knowledge.ErrNotFound) {
		t.Errorf("unknown fact: err = %v, want ErrNotFound", err)
	}
	if _, err := kb.WhoLearns(pid, "the debt"); !errors.Is(err, knowledge.ErrNotFound) {
		t.Errorf("a fact only used as backstory: err = %v, want ErrNotFound (no one learns it)", err)
	}
}
