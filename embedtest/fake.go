// Package embedtest is a deterministic fake embedder for tests of semantic
// search, in this module's tests, the CLI's and the semantic module's. It
// needs no model: texts that share words get similar vectors.
package embedtest

import (
	"errors"
	"hash/fnv"
	"math"
	"strings"
	"sync"
	"unicode"
)

// dimensions is the length of the fake's vectors.
const dimensions = 32

/** Fake is a deterministic embedder. A text becomes a bag of its lower-cased
 * words hashed into 32 dimensions and scaled to unit length, so two texts that
 * share words score a higher cosine than two that do not. It counts the batches
 * and texts it is given, and can be told to fail, so tests can assert what was
 * and was not embedded. It implements knowledge.Embedder.
 *
 * Example:
 *   f := embedtest.New()
 *   knowledge.RegisterEmbedder(f)
 *   vecs, _ := f.Embed([]string{"slow inference on the Pi"})
 *   fmt.Println(len(vecs[0]), f.Texts()) // 32 1
 */
type Fake struct {
	model string

	mu        sync.Mutex
	calls     int
	texts     int
	failAfter int // fail every batch once this many have succeeded; -1 for never
}

/** New returns a fake embedder named "fake".
 *
 * Returns:
 *   *Fake — the embedder
 *
 * Example:
 *   f := embedtest.New()
 */
func New() *Fake { return NewNamed("fake") }

/** NewNamed returns a fake embedder with the given model name. Different names
 * give different fingerprints, to test an index built by another model.
 *
 * Parameters:
 *   model (string) — the model name
 *
 * Returns:
 *   *Fake — the embedder
 *
 * Example:
 *   other := embedtest.NewNamed("other-model")
 */
func NewNamed(model string) *Fake { return &Fake{model: model, failAfter: -1} }

/** Model returns the model name. */
func (f *Fake) Model() string { return f.model }

/** Dimensions returns the vector length, 32. */
func (f *Fake) Dimensions() int { return dimensions }

/** Fingerprint identifies the fake model: it differs between names. */
func (f *Fake) Fingerprint() string { return "fake:" + f.model }

/** Calls returns how many batches Embed has been given. */
func (f *Fake) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

/** Texts returns how many texts Embed has embedded in all. */
func (f *Fake) Texts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.texts
}

/** FailAfter makes every batch after the first n fail, to test an interrupted
 * or failing run.
 *
 * Parameters:
 *   n (int) — how many batches succeed before the failures start
 *
 * Example:
 *   f.FailAfter(2) // the third batch and later return an error
 */
func (f *Fake) FailAfter(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failAfter = n
}

/** Embed returns one unit-length vector per text. An empty batch is an error,
 * as a real model's would be.
 *
 * Parameters:
 *   texts ([]string) — the texts to embed
 *
 * Returns:
 *   [][]float32 — one vector per text
 *   error — the batch was empty or FailAfter has tripped
 */
func (f *Fake) Embed(texts []string) ([][]float32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(texts) == 0 {
		return nil, errors.New("embedtest: empty batch")
	}
	if f.failAfter >= 0 && f.calls >= f.failAfter {
		return nil, errors.New("embedtest: injected failure")
	}
	f.calls++
	f.texts += len(texts)
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = vector(t)
	}
	return out, nil
}

func vector(text string) []float32 {
	v := make([]float32, dimensions)
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, w := range words {
		h := fnv.New32a()
		h.Write([]byte(w))
		v[h.Sum32()%dimensions]++
	}
	var n float64
	for _, x := range v {
		n += float64(x) * float64(x)
	}
	if n == 0 {
		v[0] = 1 // no words: a fixed unit vector, so cosine is defined
		return v
	}
	n = math.Sqrt(n)
	for i := range v {
		v[i] = float32(float64(v[i]) / n)
	}
	return v
}
