package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
	"github.com/rsdoiel/knowledge/embedtest"
)

// indexed is an embed fixture with its semantic index built.
func indexed(t *testing.T, n int) *embedFixture {
	t.Helper()
	f := newEmbedFixture(t, n)
	if code, _, e := f.run("embed"); code != 0 {
		t.Fatalf("embed: %s", e)
	}
	return f
}

func TestSearchSemantic_FindsByMeaningWithScoreAndChunk(t *testing.T) {
	f := newEmbedFixture(t, 0)
	f.obs("the compiler licence was chosen after reading the grant terms")
	f.obs("watering the tomato plants on tuesday")
	if code, _, e := f.run("embed"); code != 0 {
		t.Fatalf("embed: %s", e)
	}
	code, out, errOut := f.run("search", "-semantic", "compiler licence grant")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{"(semantic)", "compiler licence was chosen", "score 0."} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "tomato") || strings.ContainsAny(out, "«»") {
		t.Errorf("unrelated row or search marks in the output:\n%s", out)
	}
}

func TestSearchSemantic_JSONHasScoreAndChunk(t *testing.T) {
	f := indexed(t, 2)
	code, out, e := f.run("-json", "search", "p/", "-semantic", "slow inference")
	if code == 0 {
		t.Fatalf("flags after the scope are the term, so this is a search for words: %s", out)
	}
	code, out, e = f.run("-json", "search", "-semantic", "p/", "slow inference")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, e)
	}
	var doc struct {
		Mode     string  `json:"mode"`
		MinScore float64 `json:"min_score"`
		Total    int     `json:"total"`
		Results  []struct {
			Score *float64 `json:"score"`
			Chunk string   `json:"chunk"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err, out)
	}
	if doc.Mode != "semantic" || doc.MinScore != float64(knowledge.DefaultMinScore) || doc.Total != 2 || len(doc.Results) != 2 {
		t.Fatalf("%+v", doc)
	}
	for _, r := range doc.Results {
		if r.Score == nil || *r.Score <= 0 || r.Chunk == "" {
			t.Errorf("result lacks score or chunk: %+v", r)
		}
	}
}

func TestSearchSemantic_ExitCodes(t *testing.T) {
	f := indexed(t, 2)
	for _, tc := range []struct {
		name string
		args []string
		code int
	}{
		{"nothing above the cutoff", []string{"search", "-semantic", "xylophone"}, 1},
		{"a typed cutoff of -1 keeps rows", []string{"search", "-semantic", "-min-score", "-1", "xylophone"}, 0},
		{"kind not embedded", []string{"search", "-semantic", "/sources", "slow"}, 2},
		{"kind not embedded, hybrid", []string{"search", "-hybrid", "/projects", "slow"}, 2},
		{"both modes", []string{"search", "-semantic", "-hybrid", "slow"}, 2},
		{"min-score alone", []string{"search", "-min-score", "0.5", "slow"}, 2},
		{"min-score out of range", []string{"search", "-semantic", "-min-score", "2", "slow"}, 2},
		{"min-score not a number", []string{"search", "-semantic", "-min-score", "high", "slow"}, 2},
		{"min-score missing value", []string{"search", "-semantic", "-min-score"}, 2},
		{"blank term", []string{"search", "-semantic", " "}, 2},
		{"unknown scope", []string{"search", "-semantic", "nosuch/", "slow"}, 2},
	} {
		if code, out, e := f.run(tc.args...); code != tc.code {
			t.Errorf("%s: exit %d, want %d\n%s%s", tc.name, code, tc.code, out, e)
		}
	}
}

func TestSearchSemantic_NoIndexIs66(t *testing.T) {
	f := newEmbedFixture(t, 2)
	code, _, e := f.run("search", "-semantic", "slow")
	if code != 66 || !strings.Contains(e, "kb embed") {
		t.Errorf("exit %d: %s", code, e)
	}
}

func TestSearchSemantic_PlainBuildIs2AndNamesTheBuild(t *testing.T) {
	f := newEmbedFixture(t, 2)
	knowledge.RegisterEmbedder(nil)
	for _, m := range []string{"-semantic", "-hybrid"} {
		code, _, e := f.run("search", m, "slow")
		if code != 2 || !strings.Contains(e, "no semantic search") {
			t.Errorf("%s: exit %d: %s", m, code, e)
		}
	}
}

func TestSearchSemantic_ModelMissingIs66(t *testing.T) {
	f := indexed(t, 2)
	knowledge.RegisterEmbedder(missingModel{embedtest.New()})
	if code, _, e := f.run("search", "-semantic", "slow"); code != 66 {
		t.Errorf("exit %d: %s", code, e)
	}
}

func TestSearchSemantic_OtherModelIs65(t *testing.T) {
	f := indexed(t, 2)
	knowledge.RegisterEmbedder(embedtest.NewNamed("other-model"))
	if code, _, e := f.run("search", "-semantic", "slow"); code != 65 || !strings.Contains(e, "-rebuild") {
		t.Errorf("exit %d: %s", code, e)
	}
}

func TestSearchSemantic_IndexBeingBuiltIs75(t *testing.T) {
	f := indexed(t, 2)
	v, err := knowledge.OpenVectors(f.db)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	l, err := v.Lock()
	if err != nil {
		t.Fatal(err)
	}
	code, _, e := f.run("search", "-semantic", "slow")
	l.Release()
	if code != 75 || !strings.Contains(e, "being built") {
		t.Errorf("exit %d: %s", code, e)
	}
	if code, _, e := f.run("search", "-semantic", "slow inference on the Raspberry Pi"); code != 0 {
		t.Errorf("after release: exit %d: %s", code, e)
	}
}

func TestSearchSemantic_WarnsAboutRowsNotYetIndexed(t *testing.T) {
	f := indexed(t, 2)
	// Written by a verb the fixture's write-time indexing skips, then left behind.
	knowledge.RegisterEmbedder(nil)
	f.obs("a late note about slow inference")
	knowledge.RegisterEmbedder(f.fake)
	code, _, e := f.run("search", "-semantic", "slow inference")
	if code != 0 || !strings.Contains(e, "1 rows are not yet indexed") {
		t.Errorf("exit %d: %s", code, e)
	}
}

func TestSearchHybrid_RowsBothRankingsFoundComeFirst(t *testing.T) {
	f := newEmbedFixture(t, 0)
	f.obs("fuzzy inference notes")                            // words only: has "inference" but is far in meaning
	f.obs("slow inference on the Raspberry Pi board")         // both
	f.obs("sluggish model execution speed on small hardware") // meaning only if it scores
	if code, _, e := f.run("embed"); code != 0 {
		t.Fatal(e)
	}
	code, out, e := f.run("-json", "search", "-hybrid", "-min-score", "-1", "slow inference on the Raspberry Pi")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, e)
	}
	var doc struct {
		Results []struct {
			Snippet string `json:"snippet"`
			Via     string `json:"via"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Results) != 3 || doc.Results[0].Via != "both" || !strings.Contains(doc.Results[0].Snippet, "Raspberry Pi board") {
		t.Fatalf("%+v", doc.Results)
	}
	// -hybrid with a term the words half cannot parse still searches (phrase retry).
	if code, _, e := f.run("search", "-hybrid", "-min-score", "-1", "map-reduce: slow"); code != 0 {
		t.Errorf("hybrid with punctuation: exit %d: %s", code, e)
	}
}

func TestSearchSemantic_AtMostFiftyRowsWithTheTotal(t *testing.T) {
	f := newEmbedFixture(t, 0)
	for i := range 60 {
		f.obs(fmt.Sprintf("row %d about slow inference", i))
	}
	if code, _, e := f.run("embed"); code != 0 {
		t.Fatal(e)
	}
	code, out, e := f.run("search", "-semantic", "slow inference")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, e)
	}
	if !strings.Contains(out, "60 matches (semantic)") || !strings.Contains(out, "showing 50 of 60") {
		t.Errorf("total or cap missing:\n%s", out[:200])
	}
}

func TestSearchSemantic_ADashLeadingTermAfterTheFlagsIsText(t *testing.T) {
	f := indexed(t, 2)
	// After the flags, `--` ends them: the term is "-semantic" and finds nothing by meaning.
	code, out, e := f.run("-json", "search", "-semantic", "-min-score", "-1", "--", "-semantic")
	if code != 0 || !strings.Contains(out, `"term": "-semantic"`) {
		t.Errorf("exit %d: %s%s", code, out, e)
	}
	// Without a mode flag the word is still an unknown option, as before.
	if code, _, _ := f.run("search", "-semantic-ish"); code != 2 {
		t.Errorf("exit %d", code)
	}
}

// DR-0094: the words half of -hybrid runs the typed query first and, when that
// finds under three rows and the term is plain words, widens to any word.

// hybridJSON runs kb -json search -hybrid and decodes the rows.
func hybridJSON(t *testing.T, f *embedFixture, args ...string) (code int, rows []struct {
	Snippet string `json:"snippet"`
	Via     string `json:"via"`
}) {
	t.Helper()
	code, out, _ := f.run(append([]string{"-json", "search", "-hybrid"}, args...)...)
	var doc struct {
		Results []struct {
			Snippet string `json:"snippet"`
			Via     string `json:"via"`
		} `json:"results"`
	}
	if code == 0 {
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			t.Fatalf("not JSON: %v\n%s", err, out)
		}
	}
	return code, doc.Results
}

func ferryFixture(t *testing.T) *embedFixture {
	t.Helper()
	f := newEmbedFixture(t, 0)
	f.obs("the ferry timetable changed in winter")
	f.obs("tomato plants need watering on tuesday")
	f.obs("sourdough starter feeding schedule")
	if code, _, e := f.run("embed"); code != 0 {
		t.Fatalf("embed: %s", e)
	}
	return f
}

// Reproduces the report: a question typed as a sentence has words in more than
// one row, so the AND finds nothing and -hybrid has no words half.
func TestSearchHybrid_SentenceQuestionIsFoundByItsWords(t *testing.T) {
	f := ferryFixture(t)
	code, rows := hybridJSON(t, f, "when did the ferry schedule change for the winter timetable")
	if code != 0 || len(rows) == 0 {
		t.Fatalf("exit %d, %d rows: the words half found nothing", code, len(rows))
	}
	if !strings.Contains(rows[0].Snippet, "ferry timetable") {
		t.Errorf("first row = %q, want the ferry timetable row", rows[0].Snippet)
	}
	if rows[0].Via != "words" && rows[0].Via != "both" {
		t.Errorf("first row via %q: the words half did not find it", rows[0].Via)
	}
}

// Guard: a typed query that already finds three rows is not widened.
func TestSearchHybrid_QueryThatFindsRowsIsNotWidened(t *testing.T) {
	f := newEmbedFixture(t, 0)
	f.obs("ferry timetable one winter")
	f.obs("ferry timetable two summer")
	f.obs("ferry timetable three autumn")
	f.obs("ferry repair costs")
	f.obs("timetable for the bus")
	if code, _, e := f.run("embed"); code != 0 {
		t.Fatal(e)
	}
	// -min-score 0.99 leaves the semantic half empty, so only the words half shows.
	code, rows := hybridJSON(t, f, "-min-score", "0.99", "ferry timetable")
	if code != 0 || len(rows) != 3 {
		t.Errorf("exit %d, %d rows, want the 3 rows that hold both words", code, len(rows))
	}
}

// Guard: a term in FTS5 syntax is never rewritten, even when it finds nothing.
func TestSearchHybrid_Fts5SyntaxIsNeverWidened(t *testing.T) {
	f := ferryFixture(t)
	for _, term := range []string{`ferry AND zzzunknown`, `"ferry zzzunknown"`, `ferry NOT winter`, `timeta*`} {
		code, rows := hybridJSON(t, f, "-min-score", "0.99", term)
		want := map[string]int{`ferry AND zzzunknown`: 0, `"ferry zzzunknown"`: 0, `ferry NOT winter`: 0, `timeta*`: 1}[term]
		if want == 0 && code == 0 || want != 0 && len(rows) != want {
			t.Errorf("%q: exit %d, %d rows, want %d", term, code, len(rows), want)
		}
	}
}

// Guard (DR-0094 item 2): a question with no answer that shares one common word
// with a row still gets no rows, because the semantic half returned nothing.
func TestSearchHybrid_NoAnswerQuestionStaysEmpty(t *testing.T) {
	f := ferryFixture(t)
	code, rows := hybridJSON(t, f, "what colour was the weather balloon last winter")
	if code != 1 || len(rows) != 0 {
		t.Errorf("exit %d, %d rows, want exit 1 and none: a shared common word is not an answer", code, len(rows))
	}
}

// DR-0094 item 2 as built: the any-word search re-ranks the rows semantic search
// found and adds none, so a row that only shares a common word is not listed.
func TestSearchHybrid_WidenedWordsAddNoRowsOfTheirOwn(t *testing.T) {
	f := newEmbedFixture(t, 0)
	f.obs("the ferry timetable changed in winter")
	f.obs("sourdough starter feeding needs flour water salt and patience every single morning before work and the whole weekend schedule")
	if code, _, e := f.run("embed"); code != 0 {
		t.Fatal(e)
	}
	// A floor of 0.4 keeps the ferry row and drops the long sourdough row from the semantic half.
	code, rows := hybridJSON(t, f, "-min-score", "0.4", "when did the ferry schedule change for the winter timetable")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, r := range rows {
		if strings.Contains(r.Snippet, "sourdough") {
			t.Errorf("a row that shares only the word \"schedule\" was listed: %q via %s", r.Snippet, r.Via)
		}
	}
}

// The guard is for queries written in FTS5 syntax. A question typed in prose has
// colons, brackets and quotes, and is still widened.
func TestSearchHybrid_ProsePunctuationDoesNotStopTheWidening(t *testing.T) {
	f := ferryFixture(t)
	code, rows := hybridJSON(t, f, `when did the ferry schedule (the winter one) change: what is the "timetable" now?`)
	if code != 0 || len(rows) == 0 {
		t.Fatalf("exit %d, %d rows: punctuation stopped the widening", code, len(rows))
	}
	if rows[0].Via != "words" && rows[0].Via != "both" {
		t.Errorf("first row via %q, want the words half to have found it", rows[0].Via)
	}
}
