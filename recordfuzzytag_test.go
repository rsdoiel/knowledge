package knowledge

import (
	"reflect"
	"strings"
	"testing"
)

// `kb record fuzzy-tag` (DR-0052, GitHub issue #2). The worked example is the
// C0 table of record-fuzzy-tag's plan, computed against `toast` and the real
// records of the cold project.

const fuzzyRecordHead = "---\nid: \"0013\"\ntitle: \"t\"\ndate: \"2026-09-01\"\nstatus: proposed\nkind: decision\ntrigger: design\nproject: cold\nsupersedes: []\nsuperseded_by: []\nrelates_to: []\ndecisions: []\ntags: %s\nuuid: \"11111111-1111-7111-8111-111111111111\"\norigin_host: \"h\"\n---\n"

func fuzzyRecord(t *testing.T, tags, body string) *RecordFile {
	t.Helper()
	rf, err := ParseRecord([]byte(strings.Replace(fuzzyRecordHead, "%s", tags, 1)+body), "decisions/0013-x.md")
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	return rf
}

func kbWithConcepts(t *testing.T, names ...string) *KnowledgeBase {
	t.Helper()
	kb := openTestKB(t)
	for _, n := range names {
		if _, err := kb.AddConcept(n, ""); err != nil {
			t.Fatal(err)
		}
	}
	return kb
}

func TestRecordFuzzyReport_C0Table(t *testing.T) {
	// toast is 5 letters: under minFuzzyConceptLength, so without --concept
	// nothing is reported; with --concept toast every form is.
	for _, word := range []string{"detoast", "detoasts", "detoasting", "detoasted", "toasting"} {
		kb := kbWithConcepts(t, "toast")
		rf := fuzzyRecord(t, "[]", "\n**Context.**\n\nwe "+word+" the values.\n")
		none, err := kb.RecordFuzzyReport(rf, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(none) != 0 {
			t.Errorf("%s: reported without --concept: %+v", word, none)
		}
		got, err := kb.RecordFuzzyReport(rf, []string{"toast"})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Concept != "toast" || !reflect.DeepEqual(got[0].Variants, []string{word}) {
			t.Errorf("%s: report = %+v", word, got)
		}
	}
}

func TestRecordFuzzyReport_FrontmatterNeverMatches(t *testing.T) {
	kb := kbWithConcepts(t, "toast")
	rf := fuzzyRecord(t, "[detoast]", "\nplain body.\n")
	got, err := kb.RecordFuzzyReport(rf, []string{"toast"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("frontmatter text produced a match: %+v", got)
	}
}

func TestRecordFuzzyReport_LinkedConceptIsSkipped(t *testing.T) {
	kb := kbWithConcepts(t, "toast")
	body := "\nwe detoast the values.\n"
	if got, _ := kb.RecordFuzzyReport(fuzzyRecord(t, "[toast]", body), []string{"toast"}); len(got) != 0 {
		t.Errorf("tags: [toast] should skip toast, got %+v", got)
	}
	if got, _ := kb.RecordFuzzyReport(fuzzyRecord(t, "[Toast]", body), []string{"toast"}); len(got) != 0 {
		t.Errorf("tag match is case-insensitive, got %+v", got)
	}
	if got, _ := kb.RecordFuzzyReport(fuzzyRecord(t, "[]", body+"see [[toast]].\n"), []string{"toast"}); len(got) != 0 {
		t.Errorf("a [[toast]] wikilink counts as linked, got %+v", got)
	}
	if got, _ := kb.RecordFuzzyReport(fuzzyRecord(t, "[]", body), []string{"toast"}); len(got) != 1 {
		t.Errorf("same record without the tag must be reported, got %+v", got)
	}
}

func TestRecordFuzzyReport_PlainExactMentionIsCountedNotSkipped(t *testing.T) {
	kb := kbWithConcepts(t, "toast")
	rf := fuzzyRecord(t, "[]", "\nToast is used. We detoast it. toast again.\n")
	got, err := kb.RecordFuzzyReport(rf, []string{"toast"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("report = %+v", got)
	}
	if got[0].ExactMentions != 2 {
		t.Errorf("ExactMentions = %d, want 2", got[0].ExactMentions)
	}
	if !reflect.DeepEqual(got[0].Variants, []string{"detoast"}) || got[0].Count != 1 {
		t.Errorf("variants = %v count %d; the exact words are not near-misses", got[0].Variants, got[0].Count)
	}
}

func TestRecordFuzzyReport_DocumentRulesUnchangedWithoutConcept(t *testing.T) {
	// A long concept: the ordinary length/distance rules apply with no --concept.
	kb := kbWithConcepts(t, "chunking")
	rf := fuzzyRecord(t, "[]", "\nthe chunkings happen here.\n")
	got, err := kb.RecordFuzzyReport(rf, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Concept != "chunking" {
		t.Errorf("report = %+v", got)
	}
}

func TestFuzzyMatchConceptNames_StillSkipsExactMentions(t *testing.T) {
	// Guard: the record adapter needs the unskipped variant, but the public
	// function documents and keeps its exact-mention skip.
	kb := kbWithConcepts(t, "chunking")
	got, err := kb.FuzzyMatchConceptNames("chunking and chunkings")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("matches = %+v, want none: the exact name appears", got)
	}
}

// ─── AddRecordTags: the byte-exact tags: line editor ─────────────────────────

func TestAddRecordTags_EmptyFlowList(t *testing.T) {
	raw := strings.Replace(fuzzyRecordHead, "%s", "[]", 1) + "\nbody\n"
	got, err := AddRecordTags([]byte(raw), []string{"toast"})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(raw, "tags: []", "tags: [toast]", 1)
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddRecordTags_AppendsToFlowListAndQuotesWhenNeeded(t *testing.T) {
	raw := strings.Replace(fuzzyRecordHead, "%s", "[a, b]", 1) + "\nbody\n"
	got, err := AddRecordTags([]byte(raw), []string{"toast", "needs: quote"})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(raw, "tags: [a, b]", `tags: [a, b, toast, "needs: quote"]`, 1)
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	rf, err := ParseRecord(got, "x.md")
	if err != nil || !reflect.DeepEqual(rf.Tags, []string{"a", "b", "toast", "needs: quote"}) {
		t.Errorf("reparsed tags = %v, err %v", rf.Tags, err)
	}
}

func TestAddRecordTags_BlockList(t *testing.T) {
	raw := strings.Replace(fuzzyRecordHead, "tags: %s\n", "tags:\n  - a\n  - b\n", 1) + "\nbody\n"
	got, err := AddRecordTags([]byte(raw), []string{"toast"})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(raw, "  - b\n", "  - b\n  - toast\n", 1)
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddRecordTags_AbsentTagsLineIsInsertedBeforeClosingFence(t *testing.T) {
	raw := strings.Replace(fuzzyRecordHead, "tags: %s\n", "", 1) + "\nbody\n"
	got, err := AddRecordTags([]byte(raw), []string{"toast"})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(raw, "origin_host: \"h\"\n---", "origin_host: \"h\"\ntags: [toast]\n---", 1)
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddRecordTags_IdempotentAndCaseInsensitive(t *testing.T) {
	raw := strings.Replace(fuzzyRecordHead, "%s", "[Toast]", 1) + "\nbody\n"
	got, err := AddRecordTags([]byte(raw), []string{"toast"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != raw {
		t.Errorf("an already-tagged record changed:\n%s", got)
	}
}

func TestAddRecordTags_PreservesCRLFAndOnlyTouchesTheTagsLine(t *testing.T) {
	raw := strings.ReplaceAll(strings.Replace(fuzzyRecordHead, "%s", "[]", 1)+"\nbody  \nwith trailing spaces\n", "\n", "\r\n")
	got, err := AddRecordTags([]byte(raw), []string{"toast"})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(raw, "tags: []", "tags: [toast]", 1)
	if string(got) != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestAddRecordTags_UnsupportedFormsAreRefusedNotGuessed(t *testing.T) {
	for _, tags := range []string{"plain-scalar", "[a,\n  b]"} {
		raw := strings.Replace(fuzzyRecordHead, "%s", tags, 1) + "\nbody\n"
		if _, err := AddRecordTags([]byte(raw), []string{"toast"}); err == nil {
			t.Errorf("tags: %q was edited; want a refusal", tags)
		}
	}
	if _, err := AddRecordTags([]byte("no frontmatter"), []string{"toast"}); err == nil {
		t.Error("a file with no frontmatter was accepted")
	}
}

// Found by the smoke test on the real cold records: with --concept toast the
// bypass alone keeps every token within distance 3 of "toast" -- to, that,
// has, existing -- and flagged all 25 records. The explicit list widens a
// concept only to forms that contain its name (detoast, toasting) plus what
// the ordinary length and distance rules already allow.
func TestRecordFuzzyReport_ExplicitConceptDoesNotAdmitUnrelatedShortWords(t *testing.T) {
	kb := kbWithConcepts(t, "toast")
	rf := fuzzyRecord(t, "[]", "\nThat has to exist, as the coast says: it takes existing rows to detoast them, at most.\n")
	got, err := kb.RecordFuzzyReport(rf, []string{"toast"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !reflect.DeepEqual(got[0].Variants, []string{"detoast"}) {
		t.Errorf("report = %+v, want only detoast", got)
	}
}

func TestRecordFuzzyReport_ExplicitConceptStillGetsOrdinaryRules(t *testing.T) {
	// "chunkings" is distance 1 from a long concept: allowed by the ordinary
	// rules, and naming the concept must not lose it.
	kb := kbWithConcepts(t, "chunking")
	rf := fuzzyRecord(t, "[]", "\nthe chunkings happen here.\n")
	got, err := kb.RecordFuzzyReport(rf, []string{"chunking"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !reflect.DeepEqual(got[0].Variants, []string{"chunkings"}) {
		t.Errorf("report = %+v", got)
	}
}
