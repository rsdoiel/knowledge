package semantic

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// modelDir returns a real all-MiniLM-L6-v2 directory, or skips the test. Set
// KB_TEST_MODEL_DIR to use one; the spike's copy is the fallback.
func modelDir(t *testing.T) string {
	t.Helper()
	dirs := []string{os.Getenv("KB_TEST_MODEL_DIR"), "../../tmp/hugot-spike/models/KnightsAnalytics_all-MiniLM-L6-v2"}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(d, "model.onnx")); err == nil {
			return d
		}
	}
	t.Skip("no model directory: set KB_TEST_MODEL_DIR")
	return ""
}

func dot(a, b []float32) float64 {
	var s float64
	for i := range a {
		s += float64(a[i]) * float64(b[i])
	}
	return s
}

func TestFingerprintFollowsFiles(t *testing.T) {
	dir := t.TempDir()
	h := NewHugot(dir)
	if got := h.Fingerprint(); !strings.HasSuffix(got, "?") {
		t.Errorf("missing files: fingerprint %q, want a ? suffix", got)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range fingerprintFiles {
		write(n, "one")
	}
	a := NewHugot(dir).Fingerprint()
	if a != NewHugot(dir).Fingerprint() {
		t.Error("fingerprint is not stable")
	}
	write("model.onnx", "two")
	if b := NewHugot(dir).Fingerprint(); a == b {
		t.Errorf("fingerprint %q did not change with the model file", a)
	}
}

func TestEmbedEmptyNeedsNoModel(t *testing.T) {
	vecs, err := NewHugot(t.TempDir()).Embed(nil)
	if vecs != nil || err != nil {
		t.Errorf("Embed(nil) = %v, %v", vecs, err)
	}
}

func TestEmbedMissingModel(t *testing.T) {
	if _, err := NewHugot(t.TempDir()).Embed([]string{"x"}); err == nil {
		t.Error("want an error for a directory with no model")
	}
}

func TestEmbedRealModel(t *testing.T) {
	h := NewHugot(modelDir(t))
	// 19 texts of mixed length: more than two batches, sorted order differs from input order.
	var texts []string
	for i := range 19 {
		texts = append(texts, strings.Repeat("the cat sat on the mat ", 1+(i*7)%13))
	}
	texts[3] = "exit codes for command-line tools"
	texts[11] = "which status values should a CLI return"
	vecs, err := h.Embed(texts)
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != len(texts) {
		t.Fatalf("got %d vectors for %d texts", len(vecs), len(texts))
	}
	for i, v := range vecs {
		if len(v) != h.Dimensions() {
			t.Fatalf("vector %d has %d dimensions, want %d", i, len(v), h.Dimensions())
		}
		if n := dot(v, v); math.Abs(n-1) > 1e-3 {
			t.Errorf("vector %d has squared length %f, want 1", i, n)
		}
	}
	// Order is preserved through the length sort: each text matches itself run alone.
	for _, i := range []int{0, 3, 11, 18} {
		alone, err := h.Embed([]string{texts[i]})
		if err != nil {
			t.Fatal(err)
		}
		if d := dot(alone[0], vecs[i]); d < 0.999 {
			t.Errorf("text %d: batched and single vectors differ (cosine %f)", i, d)
		}
	}
	if related, unrelated := dot(vecs[3], vecs[11]), dot(vecs[3], vecs[0]); related <= unrelated {
		t.Errorf("related texts score %f, unrelated %f", related, unrelated)
	}
}

func TestEmbedOverlongInputDoesNotCrash(t *testing.T) {
	h := NewHugot(modelDir(t))
	long := strings.Repeat("an overlong passage of text with many distinct words ", 400)
	vecs, err := h.Embed([]string{"short", long, "another short one", strings.Repeat("é", 5000)})
	if err != nil {
		t.Fatalf("an over-512-token input failed the batch: %v", err)
	}
	if len(vecs) != 4 {
		t.Fatalf("got %d vectors, want 4", len(vecs))
	}
	g, n := h.guard(long)
	if n > maxTokens || len(g) >= len(long) {
		t.Errorf("guard left %d tokens, %d of %d bytes", n, len(g), len(long))
	}
}

func TestFingerprintRealModel(t *testing.T) {
	dir := modelDir(t)
	fp := NewHugot(dir).Fingerprint()
	if strings.HasSuffix(fp, "?") || !strings.Contains(fp, ":") {
		t.Errorf("fingerprint of a real model = %q", fp)
	}
}

// Before Hugot v0.8.5 the pure-Go backend compiled and kept one executable
// (about 130 MB) for every distinct (batch, sequence length) it was given, and a
// first index of a thousand rows reached 14 GB and was killed by the kernel
// (hugot#154). From v0.8.5 a new shape costs a few MB, so one session serves
// them all.
func TestEmbedHoldsMemoryAcrossNewShapes(t *testing.T) {
	h := NewHugot(modelDir(t))
	words := strings.Fields("the quick brown fox jumps over the lazy dog while a slow tortoise considers every option carefully")
	var ms runtime.MemStats
	for n := 3; n < 43; n++ { // each call is a new sequence length
		if _, err := h.Embed([]string{strings.Join(words[:n%len(words)+1], " ") + strings.Repeat(" more", n)}); err != nil {
			t.Fatal(err)
		}
		runtime.ReadMemStats(&ms)
		if ms.HeapAlloc > 2<<30 {
			t.Fatalf("heap is %d MB after %d distinct shapes: compiled shapes are being kept per length", ms.HeapAlloc>>20, n-2)
		}
	}
}

func TestEmbedIsSafeForConcurrentCalls(t *testing.T) {
	h := NewHugot(modelDir(t))
	errs := make(chan error, 4)
	for i := range 4 {
		go func() {
			_, err := h.Embed([]string{strings.Repeat("word ", 5+i*7)})
			errs <- err
		}()
	}
	for range 4 {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}
