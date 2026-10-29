package evalset

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
	"github.com/rsdoiel/knowledge/embedtest"
)

func TestLoad_TheLabelledSetIsWellFormed(t *testing.T) {
	s, err := Load("testdata/labelled_queries.json")
	if err != nil {
		t.Fatal(err)
	}
	var answerable, none int
	for _, q := range s.Queries {
		if q.NoAnswer {
			none++
		} else {
			answerable++
		}
	}
	if answerable < 25 || none < 3 {
		t.Errorf("%d answerable and %d no-answer queries; the plan asks for about 30 with the weak cases labelled", answerable, none)
	}
}

func TestLoad_RefusesAMalformedSet(t *testing.T) {
	for name, body := range map[string]string{
		"repeated id":      `{"queries":[{"id":"a","query":"x","no_answer":true},{"id":"a","query":"y","no_answer":true}]}`,
		"no text":          `{"queries":[{"id":"a","query":" ","no_answer":true}]}`,
		"neither":          `{"queries":[{"id":"a","query":"x"}]}`,
		"both":             `{"queries":[{"id":"a","query":"x","no_answer":true,"relevant":[{"type":"concept","name":"c"}]}]}`,
		"matcher no field": `{"queries":[{"id":"a","query":"x","relevant":[{"type":"record"}]}]}`,
		"unknown type":     `{"queries":[{"id":"a","query":"x","relevant":[{"type":"source","name":"s"}]}]}`,
		"not json":         `nope`,
	} {
		path := filepath.Join(t.TempDir(), "q.json")
		os.WriteFile(path, []byte(body), 0o644)
		if _, err := Load(path); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}

func TestFirstAndRecall(t *testing.T) {
	k := func(id int64) knowledge.RowKey { return knowledge.RowKey{SourceType: "record", SourceID: id} }
	rel := map[knowledge.RowKey]bool{k(2): true, k(9): true, k(40): true}
	var list []knowledge.RowKey
	for i := int64(1); i <= 12; i++ {
		list = append(list, k(i))
	}
	first, r5, r10 := firstAndRecall(list, rel)
	// 2 is at rank 2, 9 at rank 9: one of the three in the top 5 (of at most 3), two in the top 10.
	if first != 2 || r5 != 1.0/3 || r10 != 2.0/3 {
		t.Errorf("first %d r5 %v r10 %v", first, r5, r10)
	}
	// One relevant row found at rank 1 is a full score at k=5: recall is capped by what could be found.
	if _, r5, _ := firstAndRecall(list, map[knowledge.RowKey]bool{k(1): true}); r5 != 1 {
		t.Errorf("single relevant row found first: r5 = %v, want 1", r5)
	}
	if first, r5, r10 := firstAndRecall(nil, rel); first != 0 || r5 != 0 || r10 != 0 {
		t.Errorf("empty list: %d %v %v", first, r5, r10)
	}
}

// With the fake embedder (texts sharing words are close) the harness resolves
// labels by ref, text and name, and scores the three modes.
func TestPrepareAndScore_WithTheFakeEmbedder(t *testing.T) {
	db := filepath.Join(t.TempDir(), "kb.db")
	kb, err := knowledge.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()
	p, _ := kb.AddProject("alpha", "")
	kb.AddObservation(p, "note", "the heron stands in the shallow river at dawn")
	kb.AddObservation(p, "note", "tax forms are due in the spring")
	kb.AddObservation(p, "note", "a river of paperwork floods the office")
	set := &Set{Queries: []Query{
		{ID: "a", Query: "heron river dawn", Relevant: []Matcher{{Type: "observation", Contains: "heron stands"}}},
		{ID: "b", Query: "xylophone", NoAnswer: true},
	}}
	rel, stale, err := set.Resolve(kb)
	if err != nil || len(stale) != 0 || len(rel["a"]) != 1 || len(rel["b"]) != 0 {
		t.Fatalf("resolve: %v %v %v", rel, stale, err)
	}
	set.Queries = append(set.Queries, Query{ID: "c", Query: "x", Relevant: []Matcher{{Type: "observation", Contains: "no such text"}}})
	if _, stale, _ := set.Resolve(kb); len(stale) != 1 || !strings.Contains(stale[0], "c:") {
		t.Errorf("a label that matches nothing must be reported: %v", stale)
	}
	set.Queries = set.Queries[:2]

	emb := embedtest.New()
	v, err := knowledge.CreateVectors(db, emb)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if _, err := kb.EmbedIndex(context.Background(), v, emb, knowledge.EmbedOptions{}); err != nil {
		t.Fatal(err)
	}
	prep, err := Prepare(kb, v, emb, set, rel)
	if err != nil {
		t.Fatal(err)
	}
	rep := prep.Score(Params{MinScore: 0.3})
	if rep.Semantic.MRR != 1 || rep.Semantic.Recall5 != 1 || rep.Hybrid.MRR != 1 {
		t.Errorf("the heron row should be first:\n%s", rep)
	}
	if rep.Lexical.MRR != 1 {
		t.Errorf("lexical AND of heron river dawn finds exactly that row:\n%s", rep)
	}
	if rep.Semantic.Quiet != 0 && rep.Semantic.Noisy != 0 && rep.Semantic.Quiet+rep.Semantic.Noisy != 1 {
		t.Errorf("one no-answer query: %+v", rep.Semantic)
	}
	// A cutoff of 1 keeps only an exact vector; the answerable query then comes back empty.
	strict := prep.Score(Params{MinScore: 1})
	if strict.Semantic.Empty != 1 || strict.Semantic.Quiet != 1 {
		t.Errorf("at cutoff 1:\n%s", strict)
	}
	if got := strict.Misses("semantic"); len(got) != 1 || !strings.HasPrefix(got[0], "a not returned") {
		t.Errorf("misses = %v", got)
	}
	if !strings.Contains(rep.String(), "recall@5") {
		t.Errorf("table:\n%s", rep)
	}
}
