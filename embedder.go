package knowledge

import "sync"

// Semantic search (workspace DR-0088, DR-0089). The library knows only the
// shape of an embedder and a place to register one; the implementation that
// links Hugot lives in the nested module github.com/rsdoiel/knowledge/semantic,
// so a program that imports this package does not inherit that dependency tree.

/** Embedder turns text into vectors. An implementation is registered with
 * RegisterEmbedder by the program that links it; the plain kb registers none.
 *
 * Methods:
 *   Model() string — the model's name, recorded beside the vectors it made
 *   Dimensions() int — the length of each vector Embed returns
 *   Fingerprint() string — identifies the exact model files, so vectors made
 *     by a different build of the same name are not mixed
 *   Embed(texts []string) ([][]float32, error) — one vector per text, in
 *     order; an error means no vector in the batch can be trusted
 *
 * Example:
 *   type fake struct{}
 *   func (fake) Model() string       { return "fake" }
 *   func (fake) Dimensions() int     { return 2 }
 *   func (fake) Fingerprint() string { return "v1" }
 *   func (fake) Embed(t []string) ([][]float32, error) {
 *       out := make([][]float32, len(t))
 *       for i := range t { out[i] = []float32{1, 0} }
 *       return out, nil
 *   }
 *   knowledge.RegisterEmbedder(fake{})
 */
type Embedder interface {
	Model() string
	Dimensions() int
	Fingerprint() string
	Embed(texts []string) ([][]float32, error)
}

var (
	embedderMu sync.RWMutex
	embedder   Embedder
)

/** RegisterEmbedder makes e the embedder semantic search uses, replacing any
 * earlier one. Pass nil to remove it. It is meant to be called once from the
 * main of a program that links an implementation.
 *
 * Parameters:
 *   e (Embedder) — the embedder to use, or nil for none
 *
 * Example:
 *   knowledge.RegisterEmbedder(myEmbedder)
 */
func RegisterEmbedder(e Embedder) {
	embedderMu.Lock()
	defer embedderMu.Unlock()
	embedder = e
}

/** RegisteredEmbedder returns the embedder registered with RegisterEmbedder.
 *
 * Returns:
 *   Embedder — the registered embedder, or nil
 *   bool — true when one is registered
 *
 * Example:
 *   if e, ok := knowledge.RegisteredEmbedder(); ok {
 *       fmt.Println(e.Model())
 *   }
 */
func RegisteredEmbedder() (Embedder, bool) {
	embedderMu.RLock()
	defer embedderMu.RUnlock()
	return embedder, embedder != nil
}
