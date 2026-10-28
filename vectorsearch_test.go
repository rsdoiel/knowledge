package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rsdoiel/knowledge/embedtest"
)

// SemanticSearch ranks the indexed rows by cosine similarity to the query
// (semantic-search-plan.md T3.7): best chunk per row, a minimum score, a scope.

// scaled multiplies every vector by a constant: the same direction, a different length.
type scaled struct{ *embedtest.Fake }

func (s scaled) Embed(texts []string) ([][]float32, error) {
	out, err := s.Fake.Embed(texts)
	for _, v := range out {
		for i := range v {
			v[i] *= 5
		}
	}
	return out, err
}

// searchFixture indexes rows with distinct, known texts.
func searchFixture(t *testing.T) (*scopedFixture, *Vectors, *embedtest.Fake) {
	t.Helper()
	f, v, e := indexFixture(t)
	editBody(t, f.kb, "observation", f.obsAlpha, "slow inference on the raspberry pi")
	editBody(t, f.kb, "observation", f.obsBeta, "which licence was chosen for the compiler")
	editBody(t, f.kb, "record", f.recAlpha, "exit codes for command line tools")
	editBody(t, f.kb, "record", f.recBeta, "slow inference with the hailo accelerator")
	editBody(t, f.kb, "record", f.recWork, "database merge across machines")
	for _, id := range []int64{f.conceptAlpha, f.conceptBoth, f.conceptWork, f.conceptOrphan} {
		editBody(t, f.kb, "concept", id, fmt.Sprintf("unrelated concept number %d", id))
	}
	if _, err := f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{}); err != nil {
		t.Fatal(err)
	}
	return f, v, e
}

func semanticKeys(hits []SemanticHit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, fmt.Sprintf("%s:%d", h.SourceType, h.SourceID))
	}
	return out
}

func TestSemanticSearch_TheObviousMatchRanksFirst(t *testing.T) {
	f, v, e := searchFixture(t)
	res, err := f.kb.SemanticSearch(v, e, "slow inference on a raspberry pi", SemanticOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) < 2 {
		t.Fatalf("hits = %v, want the two inference rows", semanticKeys(res.Hits))
	}
	first, second := res.Hits[0], res.Hits[1]
	if first.SourceType != "observation" || first.SourceID != f.obsAlpha {
		t.Errorf("first hit = %s:%d, want the raspberry pi observation", first.SourceType, first.SourceID)
	}
	if second.SourceType != "record" || second.SourceID != f.recBeta {
		t.Errorf("second hit = %s:%d, want the hailo record", second.SourceType, second.SourceID)
	}
	if first.Score < second.Score || first.Score < 0.7 || first.Chunk != "slow inference on the raspberry pi" {
		t.Errorf("first = %+v, second score %.2f", first, second.Score)
	}
}

func TestSemanticSearch_ARowAppearsOnceWithItsBestChunk(t *testing.T) {
	f, v, e := indexFixture(t)
	para := func(w string) string { return strings.Repeat(w+" ", 12) }
	editBody(t, f.kb, "observation", f.obsAlpha, para("alpha")+"\n\n"+para("licence")+"\n\n"+para("gamma"))
	if _, err := f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{ChunkRunes: 100}); err != nil {
		t.Fatal(err)
	}
	res, err := f.kb.SemanticSearch(v, e, "licence licence", SemanticOptions{MinScore: NoCutoff})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, h := range res.Hits {
		if h.SourceType == "observation" && h.SourceID == f.obsAlpha {
			n++
			if h.ChunkIndex != 1 || !strings.HasPrefix(h.Chunk, "licence") {
				t.Errorf("hit = %+v, want chunk 1 (the licence paragraph)", h)
			}
		}
	}
	if n != 1 {
		t.Errorf("the row appears %d times, want once", n)
	}
}

func TestSemanticSearch_NothingCloseIsNoHitsNotAnError(t *testing.T) {
	f, v, e := searchFixture(t)
	res, err := f.kb.SemanticSearch(v, e, "zebra xylophone quartz", SemanticOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) != 0 || res.Total != 0 {
		t.Errorf("hits = %v, want none under the default cutoff", semanticKeys(res.Hits))
	}
	all, _ := f.kb.SemanticSearch(v, e, "zebra xylophone quartz", SemanticOptions{MinScore: NoCutoff, Limit: 1000})
	if len(all.Hits) != corpusSize(t, f.kb) {
		t.Errorf("with no cutoff got %d hits, want every indexed row (%d)", len(all.Hits), corpusSize(t, f.kb))
	}
}

func TestSemanticSearch_ACutoffAboveTheBestScoreDropsEverything(t *testing.T) {
	f, v, e := searchFixture(t)
	res, _ := f.kb.SemanticSearch(v, e, "slow inference on the raspberry pi", SemanticOptions{MinScore: 0.999})
	// An exact match scores 1 within float rounding; anything else is below.
	for _, h := range res.Hits {
		if h.Score < 0.999 {
			t.Errorf("hit %+v is below the cutoff", h)
		}
	}
	res, _ = f.kb.SemanticSearch(v, e, "slow inference", SemanticOptions{MinScore: 0.99})
	if len(res.Hits) != 0 {
		t.Errorf("hits = %v, want none at 0.99", semanticKeys(res.Hits))
	}
}

func TestSemanticSearch_ScopeRestrictsTheCandidates(t *testing.T) {
	f, v, e := searchFixture(t)
	res, err := f.kb.SemanticSearch(v, e, "slow inference", SemanticOptions{Scope: SearchScope{Project: "beta"}, MinScore: NoCutoff, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range res.Hits {
		if h.SourceType == "observation" && h.SourceID == f.obsAlpha || h.SourceType == "record" && h.SourceID == f.recAlpha {
			t.Errorf("project beta search returned alpha's %s:%d", h.SourceType, h.SourceID)
		}
	}
	if len(res.Hits) == 0 || res.Hits[0].SourceID != f.recBeta {
		t.Errorf("hits = %v, want beta's hailo record first", semanticKeys(res.Hits))
	}
	kind, err := f.kb.SemanticSearch(v, e, "slow inference", SemanticOptions{Scope: SearchScope{Kinds: []string{"records"}}, MinScore: NoCutoff, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range kind.Hits {
		if h.SourceType != "record" {
			t.Errorf("kind records returned %s", h.SourceType)
		}
	}
}

func TestSemanticSearch_LimitAndTotal(t *testing.T) {
	f, v, e := searchFixture(t)
	res, err := f.kb.SemanticSearch(v, e, "anything", SemanticOptions{MinScore: NoCutoff, Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) != 3 || res.Total != corpusSize(t, f.kb) {
		t.Errorf("%d hits, Total %d; want 3 of %d", len(res.Hits), res.Total, corpusSize(t, f.kb))
	}
	for i := 1; i < len(res.Hits); i++ {
		if res.Hits[i].Score > res.Hits[i-1].Score {
			t.Errorf("hits are not in descending score order: %v", res.Hits)
		}
	}
}

func TestSemanticSearch_RowsNotYetIndexedAreCounted(t *testing.T) {
	f, v, e := searchFixture(t)
	if _, err := f.kb.AddObservation(f.alpha, "note", "added after the index was built"); err != nil {
		t.Fatal(err)
	}
	res, err := f.kb.SemanticSearch(v, e, "anything", SemanticOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Unindexed != 1 {
		t.Errorf("Unindexed = %d, want 1", res.Unindexed)
	}
}

func TestSemanticSearch_ADeletedRowIsNotReturnedBeforeTheNextIndexRun(t *testing.T) {
	f, v, e := searchFixture(t)
	if _, err := f.kb.DeleteObservation(f.obsAlpha, true); err != nil {
		t.Fatal(err)
	}
	res, _ := f.kb.SemanticSearch(v, e, "slow inference on a raspberry pi", SemanticOptions{MinScore: NoCutoff, Limit: 100})
	for _, h := range res.Hits {
		if h.SourceType == "observation" && h.SourceID == f.obsAlpha {
			t.Error("a deleted observation is still returned")
		}
	}
}

func TestSemanticSearch_VectorLengthDoesNotChangeTheScore(t *testing.T) {
	f, v, e := searchFixture(t)
	a, _ := f.kb.SemanticSearch(v, e, "slow inference on a raspberry pi", SemanticOptions{})
	b, err := f.kb.SemanticSearch(v, scaled{e}, "slow inference on a raspberry pi", SemanticOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Hits) == 0 || len(a.Hits) != len(b.Hits) || b.Hits[0].Score-a.Hits[0].Score > 1e-4 || a.Hits[0].Score-b.Hits[0].Score > 1e-4 {
		t.Errorf("scores differ with vector length: %+v vs %+v", a.Hits, b.Hits)
	}
}

func TestSemanticSearch_Errors(t *testing.T) {
	f, v, e := searchFixture(t)
	for name, tc := range map[string]struct {
		query string
		opts  SemanticOptions
		e     Embedder
		want  error
	}{
		"blank query":       {"  ", SemanticOptions{}, e, ErrInvalid},
		"cutoff too high":   {"x", SemanticOptions{MinScore: 1.5}, e, ErrInvalid},
		"cutoff too low":    {"x", SemanticOptions{MinScore: -2}, e, ErrInvalid},
		"kind not embedded": {"x", SemanticOptions{Scope: SearchScope{Kinds: []string{"sources"}}}, e, ErrInvalid},
		"unknown project":   {"x", SemanticOptions{Scope: SearchScope{Project: "nope"}}, e, ErrNotFound},
		"another model":     {"x", SemanticOptions{}, embedtest.NewNamed("other"), ErrModelMismatch},
	} {
		if _, err := f.kb.SemanticSearch(v, tc.e, tc.query, tc.opts); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
}
