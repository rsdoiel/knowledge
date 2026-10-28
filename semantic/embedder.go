// Package semantic is the Hugot embedder for kb's semantic search
// (DR-0088, DR-0089). It lives in its own module so that the Hugot and GoMLX
// dependency tree reaches only the program built from here, never a program
// that imports github.com/rsdoiel/knowledge.
package semantic

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

// miniLMDimensions is the vector length of all-MiniLM-L6-v2, the first and
// provisional model (DR-0088).
const miniLMDimensions = 384

/** Hugot is a knowledge.Embedder backed by Hugot's pure-Go session, so it
 * needs no cgo and no daemon. The model is loaded on the first call to Embed,
 * not when the value is made, so registering it costs nothing for a command
 * that never embeds.
 *
 * Example:
 *   knowledge.RegisterEmbedder(semantic.NewHugot("/path/to/all-MiniLM-L6-v2"))
 */
type Hugot struct {
	dir string

	once    sync.Once
	session *hugot.Session
	pipe    *pipelines.FeatureExtractionPipeline
	err     error
}

/** NewHugot returns an embedder for the model in dir (a directory holding the
 * model's ONNX file and tokenizer). Nothing is read until Embed is called.
 *
 * Parameters:
 *   dir (string) — the model directory
 *
 * Returns:
 *   *Hugot — the embedder
 *
 * Example:
 *   e := semantic.NewHugot("models/all-MiniLM-L6-v2")
 */
func NewHugot(dir string) *Hugot { return &Hugot{dir: dir} }

/** Model returns the model's name, taken from its directory name. */
func (h *Hugot) Model() string { return filepath.Base(h.dir) }

/** Dimensions returns the length of the vectors Embed returns. */
func (h *Hugot) Dimensions() int { return miniLMDimensions }

/** Fingerprint identifies the model files. Until the model manager exists
 * (semantic-search-plan.md T5) it is the model directory's name.
 */
func (h *Hugot) Fingerprint() string { return h.Model() }

/** Embed returns one vector per text, in order, loading the model on first use.
 *
 * Parameters:
 *   texts ([]string) — the texts to embed
 *
 * Returns:
 *   [][]float32 — one vector per text
 *   error — the model could not be loaded, or inference failed
 *
 * Example:
 *   vecs, err := e.Embed([]string{"a sentence"})
 */
func (h *Hugot) Embed(texts []string) ([][]float32, error) {
	h.once.Do(func() {
		ctx := context.Background()
		h.session, h.err = hugot.NewGoSession(ctx)
		if h.err != nil {
			return
		}
		h.pipe, h.err = h.session.NewPipeline(hugot.FeatureExtractionConfig{ModelPath: h.dir, Name: "embed"})
	})
	if h.err != nil {
		return nil, fmt.Errorf("loading model %s: %w", h.dir, h.err)
	}
	out, err := h.pipe.RunPipeline(context.Background(), texts)
	if err != nil {
		return nil, fmt.Errorf("embedding with %s: %w", h.Model(), err)
	}
	return out.Embeddings, nil
}
