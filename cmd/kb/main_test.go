package main

import (
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// The plain kb links no embedder (semantic-search-plan.md T2): the semantic
// build registers one from its own main, so nothing this program imports may.
func TestPlainBuild_RegistersNoEmbedder(t *testing.T) {
	if e, ok := knowledge.RegisteredEmbedder(); ok {
		t.Errorf("the plain kb has embedder %q registered, want none", e.Model())
	}
}
