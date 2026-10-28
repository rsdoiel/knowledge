package embedtest_test

import (
	"math"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
	"github.com/rsdoiel/knowledge/embedtest"
)

var _ knowledge.Embedder = (*embedtest.Fake)(nil)

func cosine(a, b []float32) float64 {
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	return dot
}

func TestFake_VectorsAreDeterministicAndUnitLength(t *testing.T) {
	f := embedtest.New()
	a, err := f.Embed([]string{"exit codes for tools"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := embedtest.New().Embed([]string{"exit codes for tools"})
	if len(a) != 1 || len(a[0]) != f.Dimensions() {
		t.Fatalf("got %d vectors of %d, want 1 of %d", len(a), len(a[0]), f.Dimensions())
	}
	for i := range a[0] {
		if a[0][i] != b[0][i] {
			t.Fatalf("dimension %d differs between two embedders", i)
		}
	}
	if n := cosine(a[0], a[0]); math.Abs(n-1) > 1e-5 {
		t.Errorf("self cosine = %v, want 1", n)
	}
}

func TestFake_SharedWordsScoreHigher(t *testing.T) {
	v, _ := embedtest.New().Embed([]string{
		"slow inference on the raspberry pi",
		"inference is slow on a raspberry pi",
		"which licence was chosen for the compiler",
	})
	near, far := cosine(v[0], v[1]), cosine(v[0], v[2])
	if near <= far {
		t.Errorf("similar texts score %.3f, unrelated %.3f; want similar higher", near, far)
	}
}

func TestFake_CountsBatchesAndTexts(t *testing.T) {
	f := embedtest.New()
	f.Embed([]string{"a", "b", "c"})
	f.Embed([]string{"d"})
	if f.Calls() != 2 || f.Texts() != 4 {
		t.Errorf("Calls, Texts = %d, %d; want 2, 4", f.Calls(), f.Texts())
	}
}

func TestFake_EmptyBatchIsAnError(t *testing.T) {
	if _, err := embedtest.New().Embed(nil); err == nil {
		t.Error("an empty batch returned no error")
	}
}

func TestFake_FailAfterInjectsAnError(t *testing.T) {
	f := embedtest.New()
	f.FailAfter(1)
	if _, err := f.Embed([]string{"a"}); err != nil {
		t.Fatalf("first batch: %v", err)
	}
	if _, err := f.Embed([]string{"b"}); err == nil {
		t.Error("second batch succeeded, want the injected failure")
	}
}

func TestFake_NamedModelsHaveDistinctFingerprints(t *testing.T) {
	a, b := embedtest.NewNamed("a"), embedtest.NewNamed("b")
	if a.Model() == b.Model() || a.Fingerprint() == b.Fingerprint() {
		t.Error("two models share a name or fingerprint")
	}
}
