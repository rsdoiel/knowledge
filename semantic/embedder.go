// Package semantic is the Hugot embedder for kb's semantic search
// (DR-0088, DR-0089). It lives in its own module so that the Hugot and GoMLX
// dependency tree reaches only the program built from here, never a program
// that imports github.com/rsdoiel/knowledge.
package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

// miniLMDimensions is the vector length of all-MiniLM-L6-v2, the first and
// provisional model (DR-0088).
const miniLMDimensions = 384

// maxTokens is the longest input the model accepts. The spike showed that one
// longer input crashes the whole batch, so no text goes past it.
const maxTokens = 512

// batchSize is how many texts go through the pipeline together. Texts are
// sorted by length first, so a batch pads to little more than its longest.
const batchSize = 8

// maxShapes is how many distinct (batch size, sequence length) shapes one
// session is given before it is replaced. Hugot's pure-Go backend compiles an
// executable for every new shape and keeps it for the life of the session, about
// 200 MB each, and its bucketing options apply only to the XLA backend. With
// length-sorted batches nearly every batch is a new shape, so an unbounded
// session took a first index of a thousand rows past 14 GB and the kernel's
// out-of-memory killer ended it. Replacing the session every few shapes bounds
// the memory; each new shape costs one compile either way.
const maxShapes = 4

// fingerprintFiles are the files that decide what a vector means.
var fingerprintFiles = []string{"model.onnx", "tokenizer.json", "config.json"}

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

	mu      sync.Mutex // serialises Embed: a replaced session must not be in use
	session *hugot.Session
	pipe    *pipelines.FeatureExtractionPipeline
	shapes  map[[2]int]bool // shapes the current session has compiled

	fpOnce sync.Once
	fp     string
}

/** NewHugot returns an embedder for the model in dir (a directory holding the
 * model's ONNX file and tokenizer). Nothing is read until Embed or Fingerprint
 * is called.
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

/** Available reports whether the model's files are installed in the model
 * directory, without loading the model.
 *
 * Returns:
 *   error — nil when every file is present; otherwise an error wrapping
 *     fs.ErrNotExist that names the directory and says how to install the model
 *
 * Example:
 *   if err := e.Available(); err != nil { fmt.Println(err) }
 */
func (h *Hugot) Available() error {
	for _, name := range fingerprintFiles {
		if fi, err := os.Stat(filepath.Join(h.dir, name)); err != nil || !fi.Mode().IsRegular() {
			return fmt.Errorf("the embedding model %s is not installed at %s (run: kb model fetch): %w", h.Model(), h.dir, fs.ErrNotExist)
		}
	}
	return nil
}

/** Fingerprint identifies the model files: the model's name and a hash of its
 * ONNX file, tokenizer and config, so an index built by one set of files is
 * not searched with another. It is computed once. When the files cannot be
 * read the fingerprint is the name alone with a "?" suffix, which matches no
 * index built from real files.
 *
 * Returns:
 *   string — "name:hex" where hex is the first 16 digits of a SHA-256
 *
 * Example:
 *   fmt.Println(e.Fingerprint()) // all-MiniLM-L6-v2:3f0c9a41b27d85e6
 */
func (h *Hugot) Fingerprint() string {
	h.fpOnce.Do(func() {
		sum := sha256.New()
		for _, name := range fingerprintFiles {
			f, err := os.Open(filepath.Join(h.dir, name))
			if err != nil {
				h.fp = h.Model() + "?"
				return
			}
			fmt.Fprintf(sum, "%s\n", name)
			_, err = io.Copy(sum, f)
			f.Close()
			if err != nil {
				h.fp = h.Model() + "?"
				return
			}
		}
		h.fp = h.Model() + ":" + hex.EncodeToString(sum.Sum(nil))[:16]
	})
	return h.fp
}

// load opens the session and pipeline if there is none. Vectors come back at
// unit length, the same as the fake embedder's, so a dot product is a cosine.
func (h *Hugot) load() error {
	if h.pipe != nil {
		return nil
	}
	session, err := hugot.NewGoSession(context.Background())
	if err != nil {
		return fmt.Errorf("loading model %s: %w", h.dir, err)
	}
	pipe, err := session.NewPipeline(hugot.FeatureExtractionConfig{
		ModelPath: h.dir,
		Name:      "embed",
		Options:   []hugot.FeatureExtractionOption{pipelines.WithNormalization()},
	})
	if err != nil {
		session.Destroy()
		return fmt.Errorf("loading model %s: %w", h.dir, err)
	}
	h.session, h.pipe, h.shapes = session, pipe, map[[2]int]bool{}
	return nil
}

// release drops the session and the executables it compiled.
func (h *Hugot) release() {
	if h.session != nil {
		h.session.Destroy()
	}
	h.session, h.pipe, h.shapes = nil, nil, nil
	runtime.GC()
	debug.FreeOSMemory()
}

// admit notes the shape of a batch about to run. A new shape, when the session
// already holds maxShapes of them, first replaces the session.
func (h *Hugot) admit(batch, longest int) error {
	key := [2]int{batch, longest}
	if h.pipe != nil && !h.shapes[key] && len(h.shapes) >= maxShapes {
		h.release()
	}
	if err := h.load(); err != nil {
		return err
	}
	h.shapes[key] = true
	return nil
}

// tokenCount returns how many tokens the model sees for text, including its
// special tokens.
func (h *Hugot) tokenCount(text string) int {
	return len(h.pipe.Model.Tokenizer.GoTokenizer.Tokenizer.EncodeWithAnnotations(text).IDs)
}

// guard returns text unchanged when it fits in maxTokens, otherwise the longest
// prefix, cut on a space where it can, that does. Chunking keeps real input
// well under the limit; this is the last defence against a crashed batch.
func (h *Hugot) guard(text string) (string, int) {
	n := h.tokenCount(text)
	for n > maxTokens {
		cut := len(text) * (maxTokens - 8) / n
		if cut >= len(text) {
			cut = len(text) - 1
		}
		for cut > 0 && text[cut]&0xC0 == 0x80 { // do not split a UTF-8 sequence
			cut--
		}
		if sp := lastSpace(text[:cut]); sp > cut/2 {
			cut = sp
		}
		text = text[:cut]
		n = h.tokenCount(text)
	}
	return text, n
}

func lastSpace(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ' ' || s[i] == '\n' {
			return i
		}
	}
	return -1
}

/** Embed returns one unit-length vector per text, in order, loading the model
 * on first use. Texts are sorted by token length and run in batches of 8; a
 * text over 512 tokens is cut to fit rather than sent whole.
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
	if len(texts) == 0 {
		return nil, nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.load(); err != nil { // the tokenizer is needed to measure texts
		return nil, err
	}
	type item struct {
		at, tokens int
		text       string
	}
	items := make([]item, len(texts))
	for i, t := range texts {
		g, n := h.guard(t)
		items[i] = item{i, n, g}
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].tokens < items[b].tokens })

	out := make([][]float32, len(texts))
	for lo := 0; lo < len(items); lo += batchSize {
		hi := min(lo+batchSize, len(items))
		batch := make([]string, 0, hi-lo)
		for _, it := range items[lo:hi] {
			batch = append(batch, it.text)
		}
		// Sorted by length, the last text of a batch is its longest, and so decides
		// the padded length Hugot compiles for.
		if err := h.admit(len(batch), items[hi-1].tokens); err != nil {
			return nil, err
		}
		res, err := h.pipe.RunPipeline(context.Background(), batch)
		if err != nil {
			return nil, fmt.Errorf("embedding with %s: %w", h.Model(), err)
		}
		if len(res.Embeddings) != len(batch) {
			return nil, fmt.Errorf("embedding with %s: %d vectors for %d texts", h.Model(), len(res.Embeddings), len(batch))
		}
		for j, it := range items[lo:hi] {
			out[it.at] = res.Embeddings[j]
		}
	}
	return out, nil
}
