package knowledge

import (
	"errors"
	"testing"
)

type fakeEmbedder struct{ model string }

func (f fakeEmbedder) Model() string     { return f.model }
func (fakeEmbedder) Dimensions() int     { return 2 }
func (fakeEmbedder) Fingerprint() string { return "v1" }
func (fakeEmbedder) Embed(t []string) ([][]float32, error) {
	if len(t) == 0 {
		return nil, errors.New("no texts")
	}
	out := make([][]float32, len(t))
	for i := range t {
		out[i] = []float32{1, 0}
	}
	return out, nil
}

func TestRegisterEmbedder_NoneByDefault(t *testing.T) {
	RegisterEmbedder(nil)
	if e, ok := RegisteredEmbedder(); ok || e != nil {
		t.Errorf("RegisteredEmbedder() = %v, %v; want none", e, ok)
	}
}

func TestRegisterEmbedder_RegistersAndReplaces(t *testing.T) {
	t.Cleanup(func() { RegisterEmbedder(nil) })
	RegisterEmbedder(fakeEmbedder{"a"})
	e, ok := RegisteredEmbedder()
	if !ok || e.Model() != "a" {
		t.Fatalf("RegisteredEmbedder() = %v, %v; want model a", e, ok)
	}
	RegisterEmbedder(fakeEmbedder{"b"})
	if e, _ := RegisteredEmbedder(); e.Model() != "b" {
		t.Errorf("model = %q after replacing, want b", e.Model())
	}
	RegisterEmbedder(nil)
	if _, ok := RegisteredEmbedder(); ok {
		t.Error("nil did not remove the embedder")
	}
}
