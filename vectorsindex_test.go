package knowledge

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsdoiel/knowledge/embedtest"
)

// EmbedIndex brings the sidecar in step with the corpus (semantic-search-plan.md
// T3.5): it embeds only what is new or changed, drops what is gone, commits as it
// goes so an interrupted run resumes, and refuses a different model.

// batchRecorder wraps an embedder and records the size of each batch.
type batchRecorder struct {
	*embedtest.Fake
	sizes []int
}

func (b *batchRecorder) Embed(texts []string) ([][]float32, error) {
	b.sizes = append(b.sizes, len(texts))
	return b.Fake.Embed(texts)
}

type shortEmbedder struct{ *embedtest.Fake }

func (shortEmbedder) Embed(texts []string) ([][]float32, error) { return [][]float32{}, nil }

func indexFixture(t *testing.T) (*scopedFixture, *Vectors, *embedtest.Fake) {
	t.Helper()
	f := newScopedFixture(t)
	e := embedtest.New()
	v, err := CreateVectors(filepath.Join(t.TempDir(), "knowledge.db"), e)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { v.Close() })
	return f, v, e
}

func corpusSize(t *testing.T, kb *KnowledgeBase) int {
	t.Helper()
	rows, err := kb.EmbedCorpus(SearchScope{})
	if err != nil {
		t.Fatal(err)
	}
	return len(rows)
}

func TestEmbedIndex_FirstRunEmbedsEveryRow(t *testing.T) {
	f, v, e := indexFixture(t)
	res, err := f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	n := corpusSize(t, f.kb)
	rows, _ := v.IndexedRows()
	if len(rows) != n || res.RowsEmbedded != n || res.RowsSkipped != 0 || res.RowsRemoved != 0 {
		t.Errorf("indexed %d rows, result %+v; want %d embedded", len(rows), res, n)
	}
	if res.ChunksEmbedded != e.Texts() || e.Texts() < n {
		t.Errorf("ChunksEmbedded = %d, embedder saw %d texts, want at least %d", res.ChunksEmbedded, e.Texts(), n)
	}
}

func TestEmbedIndex_SecondRunEmbedsNothing(t *testing.T) {
	f, v, e := indexFixture(t)
	f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{})
	calls := e.Calls()
	res, err := f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if e.Calls() != calls {
		t.Errorf("the embedder was called %d more times on an unchanged corpus", e.Calls()-calls)
	}
	if res.RowsEmbedded != 0 || res.RowsSkipped != corpusSize(t, f.kb) {
		t.Errorf("result = %+v, want everything skipped", res)
	}
}

// editBody changes what lexical search indexes for a row, which is the corpus's input.
func editBody(t *testing.T, kb *KnowledgeBase, sourceType string, id int64, body string) {
	t.Helper()
	if _, err := kb.db.Exec(`UPDATE kb_fts SET body = ? WHERE source_type = ? AND CAST(source_id AS INTEGER) = ?`, body, sourceType, id); err != nil {
		t.Fatal(err)
	}
}

func TestEmbedIndex_AnEditedRowReembedsOnlyItsChangedChunks(t *testing.T) {
	f, v, e := indexFixture(t)
	para := func(w string) string { return strings.Repeat(w+" ", 12) } // 12 words, about 60 characters
	editBody(t, f.kb, "observation", f.obsAlpha, para("alpha")+"\n\n"+para("beta")+"\n\n"+para("gamma"))
	opts := EmbedOptions{ChunkRunes: 100}
	if _, err := f.kb.EmbedIndex(context.Background(), v, e, opts); err != nil {
		t.Fatal(err)
	}
	if got, _ := v.RowChunks("observation", f.obsAlpha); len(got) != 3 {
		t.Fatalf("the row has %d chunks, want 3", len(got))
	}
	before := e.Texts()
	editBody(t, f.kb, "observation", f.obsAlpha, para("alpha")+"\n\n"+para("CHANGED")+"\n\n"+para("gamma"))
	res, err := f.kb.EmbedIndex(context.Background(), v, e, opts)
	if err != nil {
		t.Fatal(err)
	}
	if d := e.Texts() - before; d != 1 || res.ChunksEmbedded != 1 || res.RowsEmbedded != 1 {
		t.Errorf("re-embedded %d texts, result %+v; want exactly the one changed chunk", d, res)
	}
	got, _ := v.RowChunks("observation", f.obsAlpha)
	if len(got) != 3 || !strings.HasPrefix(got[1].Text, "CHANGED") {
		t.Errorf("chunks after the edit: %+v", got)
	}
}

func TestEmbedIndex_ARowThatShrinksLosesTheChunksItNoLongerHas(t *testing.T) {
	f, v, e := indexFixture(t)
	long := strings.Repeat("word ", 60)
	editBody(t, f.kb, "observation", f.obsAlpha, long)
	opts := EmbedOptions{ChunkRunes: 80}
	f.kb.EmbedIndex(context.Background(), v, e, opts)
	if got, _ := v.RowChunks("observation", f.obsAlpha); len(got) < 3 {
		t.Fatalf("setup: %d chunks", len(got))
	}
	editBody(t, f.kb, "observation", f.obsAlpha, "short now")
	f.kb.EmbedIndex(context.Background(), v, e, opts)
	got, _ := v.RowChunks("observation", f.obsAlpha)
	if len(got) != 1 || got[0].Text != "short now" {
		t.Errorf("chunks = %+v, want just \"short now\"", got)
	}
}

func TestEmbedIndex_ADeletedRowLosesItsChunks(t *testing.T) {
	f, v, e := indexFixture(t)
	f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{})
	if _, err := f.kb.DeleteObservation(f.obsBeta, true); err != nil {
		t.Fatal(err)
	}
	res, err := f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.RowsRemoved != 1 {
		t.Errorf("RowsRemoved = %d, want 1", res.RowsRemoved)
	}
	if got, _ := v.RowChunks("observation", f.obsBeta); len(got) != 0 {
		t.Errorf("the deleted observation still has %d chunks", len(got))
	}
}

func TestEmbedIndex_ARowWithNoTextHasNoChunks(t *testing.T) {
	f, v, e := indexFixture(t)
	editBody(t, f.kb, "observation", f.obsAlpha, "   \n\n ")
	if _, err := f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{}); err != nil {
		t.Fatal(err)
	}
	if got, _ := v.RowChunks("observation", f.obsAlpha); len(got) != 0 {
		t.Errorf("a blank row has %d chunks", len(got))
	}
}

func TestEmbedIndex_BatchesAreNoLargerThanBatchSize(t *testing.T) {
	f, v, _ := indexFixture(t)
	rec := &batchRecorder{Fake: embedtest.New()}
	if _, err := f.kb.EmbedIndex(context.Background(), v, rec, EmbedOptions{BatchSize: 4}); err != nil {
		t.Fatal(err)
	}
	if len(rec.sizes) < 2 {
		t.Fatalf("only %d batch(es) for %d rows", len(rec.sizes), corpusSize(t, f.kb))
	}
	for _, n := range rec.sizes {
		if n > 4 {
			t.Errorf("a batch of %d texts, BatchSize is 4", n)
		}
	}
}

func TestEmbedIndex_AnInterruptedRunResumesWithoutRepeatingWork(t *testing.T) {
	f, v, e := indexFixture(t)
	e.FailAfter(2) // two batches succeed, then the embedder fails
	res, err := f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{BatchSize: 2})
	if err == nil {
		t.Fatal("the failing embedder reported no error")
	}
	done := res.RowsEmbedded
	rows, _ := v.IndexedRows()
	if done == 0 || len(rows) != done || done >= corpusSize(t, f.kb) {
		t.Fatalf("after the failure %d rows are reported and %d stored of %d", done, len(rows), corpusSize(t, f.kb))
	}
	e.FailAfter(-1)
	seen := e.Texts()
	res, err = f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{BatchSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.RowsSkipped != done || res.RowsEmbedded != corpusSize(t, f.kb)-done {
		t.Errorf("resume result %+v; want %d skipped", res, done)
	}
	if e.Texts()-seen != res.ChunksEmbedded {
		t.Errorf("resume embedded %d texts, result says %d", e.Texts()-seen, res.ChunksEmbedded)
	}
	if rows, _ := v.IndexedRows(); len(rows) != corpusSize(t, f.kb) {
		t.Errorf("%d rows stored after resuming, want %d", len(rows), corpusSize(t, f.kb))
	}
}

func TestEmbedIndex_CancellingStopsBetweenBatchesAndResumes(t *testing.T) {
	f, v, e := indexFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	var calls int
	res, err := f.kb.EmbedIndex(ctx, v, e, EmbedOptions{BatchSize: 2, Progress: func(EmbedProgress) {
		calls++
		cancel()
	}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if calls != 1 || res.RowsEmbedded == 0 || res.RowsEmbedded >= corpusSize(t, f.kb) {
		t.Errorf("progress calls = %d, result %+v; want a stop after the first batch", calls, res)
	}
	if _, err := f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{BatchSize: 2}); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if rows, _ := v.IndexedRows(); len(rows) != corpusSize(t, f.kb) {
		t.Errorf("%d rows after resuming, want %d", len(rows), corpusSize(t, f.kb))
	}
}

func TestEmbedIndex_AlreadyCancelledEmbedsNothing(t *testing.T) {
	f, v, e := indexFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.kb.EmbedIndex(ctx, v, e, EmbedOptions{}); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if e.Calls() != 0 {
		t.Error("the embedder was called after cancellation")
	}
}

func TestEmbedIndex_ProgressCountsUpToTheTotal(t *testing.T) {
	f, v, e := indexFixture(t)
	var last EmbedProgress
	var n int
	_, err := f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{BatchSize: 3, Progress: func(p EmbedProgress) {
		if p.Done <= last.Done {
			t.Errorf("progress went from %d to %d", last.Done, p.Done)
		}
		last = p
		n++
	}})
	if err != nil {
		t.Fatal(err)
	}
	if last.Total != corpusSize(t, f.kb) || last.Done != last.Total || n < 2 {
		t.Errorf("last progress %+v after %d calls; want Done == Total == %d", last, n, corpusSize(t, f.kb))
	}
}

func TestEmbedIndex_ADifferentModelIsErrModelMismatchUntilReset(t *testing.T) {
	f, v, e := indexFixture(t)
	f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{})
	other := embedtest.NewNamed("other")
	if _, err := f.kb.EmbedIndex(context.Background(), v, other, EmbedOptions{}); !errors.Is(err, ErrModelMismatch) {
		t.Fatalf("err = %v, want ErrModelMismatch", err)
	}
	if other.Calls() != 0 {
		t.Error("the other model embedded something before the check")
	}
	dbPath := strings.TrimSuffix(v.Path(), ".vectors.db") + ".db"
	v.Close()
	v2, err := ResetVectors(dbPath, other)
	if err != nil {
		t.Fatal(err)
	}
	defer v2.Close()
	if rows, _ := v2.IndexedRows(); len(rows) != 0 {
		t.Errorf("a reset index holds %d rows", len(rows))
	}
	if _, err := f.kb.EmbedIndex(context.Background(), v2, other, EmbedOptions{}); err != nil {
		t.Fatalf("after reset: %v", err)
	}
	if v2.Meta().Model != "other" {
		t.Errorf("model = %q after reset, want other", v2.Meta().Model)
	}
}

func TestEmbedIndex_AnEmbedderThatReturnsTheWrongNumberOfVectorsIsAnError(t *testing.T) {
	f, v, _ := indexFixture(t)
	_, err := f.kb.EmbedIndex(context.Background(), v, shortEmbedder{embedtest.New()}, EmbedOptions{})
	if err == nil || !strings.Contains(err.Error(), "vectors") {
		t.Errorf("err = %v, want a complaint about the number of vectors", err)
	}
	if rows, _ := v.IndexedRows(); len(rows) != 0 {
		t.Errorf("%d rows stored from a bad batch", len(rows))
	}
}
