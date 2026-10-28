package main

import (
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// The semantic build registers an embedder and the plain kb does not
// (TestPlainBuild_RegistersNoEmbedder in the parent module's cmd/kb).
func TestSemanticBuild_RegistersTheHugotEmbedder(t *testing.T) {
	t.Cleanup(func() { knowledge.RegisterEmbedder(nil) })
	register("models/all-MiniLM-L6-v2")
	e, ok := knowledge.RegisteredEmbedder()
	if !ok {
		t.Fatal("no embedder registered")
	}
	if e.Model() != "all-MiniLM-L6-v2" || e.Dimensions() != 384 {
		t.Errorf("embedder = %q, %d dims; want all-MiniLM-L6-v2, 384", e.Model(), e.Dimensions())
	}
}
