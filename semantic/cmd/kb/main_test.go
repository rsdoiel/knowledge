package main

import (
	"os"
	"path/filepath"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// The semantic build registers an embedder and the plain kb does not
// (TestPlainBuild_RegistersNoEmbedder in the parent module's cmd/kb).
func TestSemanticBuild_RegistersTheHugotEmbedder(t *testing.T) {
	t.Cleanup(func() { knowledge.RegisterEmbedder(nil) })
	t.Setenv("KB_MODEL_DIR", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	register("")
	e, ok := knowledge.RegisteredEmbedder()
	if !ok {
		t.Fatal("no embedder registered")
	}
	if e.Model() != "all-MiniLM-L6-v2" || e.Dimensions() != 384 {
		t.Errorf("embedder = %q, %d dims; want all-MiniLM-L6-v2, 384", e.Model(), e.Dimensions())
	}
}

// With the model installed in the workspace, the embedder reads that copy, ahead
// of the user's.
func TestSemanticBuild_FindsTheInstalledModel(t *testing.T) {
	t.Cleanup(func() { knowledge.RegisterEmbedder(nil) })
	t.Setenv("KB_MODEL_DIR", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if testing.Short() {
		t.Skip("copies the model")
	}
	src := os.Getenv("KB_TEST_MODEL_DIR")
	if src == "" {
		src = "../../../../tmp/hugot-spike/models/KnightsAnalytics_all-MiniLM-L6-v2"
	}
	if _, err := os.Stat(filepath.Join(src, "model.onnx")); err != nil {
		t.Skip("no model directory: set KB_TEST_MODEL_DIR")
	}
	ws := t.TempDir()
	dst := filepath.Join(ws, "agents", "models", "all-MiniLM-L6-v2")
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	register(ws)
	e, _ := knowledge.RegisteredEmbedder()
	if fp := e.Fingerprint(); len(fp) < 20 || fp[len(fp)-1] == '?' {
		t.Errorf("fingerprint %q: the model was not found at %s", fp, dst)
	}
}
