package knowledge

import (
	"context"
	"fmt"
	"os"
)

/** EmbedOptions tunes EmbedIndex.
 *
 * Fields:
 *   BatchSize  (int)                 — texts per Embed call; zero means 16
 *   ChunkRunes (int)                 — the longest chunk, in characters; zero means DefaultChunkRunes
 *   Progress   (func(EmbedProgress)) — called after each committed batch; may be nil
 *
 * Example:
 *   opts := knowledge.EmbedOptions{BatchSize: 8, Progress: func(p knowledge.EmbedProgress) {
 *       fmt.Printf("%d of %d rows\n", p.Done, p.Total)
 *   }}
 */
type EmbedOptions struct {
	BatchSize  int
	ChunkRunes int
	Progress   func(EmbedProgress)
}

/** EmbedProgress is reported after each batch EmbedIndex commits.
 *
 * Fields:
 *   Done           (int) — rows embedded so far in this run
 *   Total          (int) — rows this run has to embed
 *   ChunksEmbedded (int) — chunks sent to the embedder so far in this run
 */
type EmbedProgress struct {
	Done, Total, ChunksEmbedded int
}

/** EmbedResult says what EmbedIndex did, also when it stopped early.
 *
 * Fields:
 *   RowsEmbedded   (int) — rows whose chunks were written
 *   RowsSkipped    (int) — rows already up to date
 *   RowsRemoved    (int) — rows whose chunks were dropped because the row is gone or has no text
 *   ChunksEmbedded (int) — chunks sent to the embedder; a chunk whose text did not change is not
 */
type EmbedResult struct {
	RowsEmbedded, RowsSkipped, RowsRemoved, ChunksEmbedded int
}

// embedWork is one row that needs chunks written.
type embedWork struct {
	row    CorpusRow
	chunks []string
	reuse  map[string][]float32 // vectors already stored for chunks whose text is unchanged, by hash
}

/** EmbedIndex brings the vector sidecar in step with the corpus. A row is split
 * into chunks and compared with what is stored: a chunk whose text is unchanged
 * keeps its vector, a new or changed one is sent to the embedder, and a row that
 * is gone, or has no text, loses its chunks. Embedding happens in batches and
 * each finished row is committed, so an interrupted run, by error or by
 * cancelling ctx, has lost at most the batch in flight and a rerun carries on.
 * It holds the sidecar's lock for the whole run (ErrLocked when another run has
 * it) and refreshes it after each batch, aborting if the lock was taken over.
 * It never builds an index for a different model: that is ErrModelMismatch, and
 * the remedy is ResetVectors.
 *
 * Parameters:
 *   ctx (context.Context) — cancelled to stop between batches
 *   v (*Vectors) — the sidecar to update
 *   e (Embedder) — the model; it must be the one v was built with
 *   opts (EmbedOptions) — batch size, chunk size and progress reporting
 *
 * Returns:
 *   EmbedResult — what was done, valid also when an error is returned
 *   error — ctx's error when cancelled; ErrModelMismatch; ErrLocked; the embedder's error;
 *     a database error
 *
 * Example:
 *   res, err := kb.EmbedIndex(ctx, v, embedder, knowledge.EmbedOptions{})
 *   fmt.Printf("%d rows embedded, %d up to date\n", res.RowsEmbedded, res.RowsSkipped)
 */
func (kb *KnowledgeBase) EmbedIndex(ctx context.Context, v *Vectors, e Embedder, opts EmbedOptions) (EmbedResult, error) {
	var res EmbedResult
	if err := ctx.Err(); err != nil {
		return res, err
	}
	if err := v.CheckModel(e); err != nil {
		return res, err
	}
	size, chunkRunes := opts.BatchSize, opts.ChunkRunes
	if size <= 0 {
		size = 16
	}
	if chunkRunes <= 0 {
		chunkRunes = DefaultChunkRunes
	}
	lock, err := v.Lock()
	if err != nil {
		return res, err
	}
	defer lock.Release()
	plan, err := kb.planEmbed(v, chunkRunes)
	if err != nil {
		return res, err
	}
	// Drop what is gone before embedding anything, so an interrupted run never
	// leaves chunks for a deleted row.
	for _, k := range plan.remove {
		if err := v.DeleteRow(k.SourceType, k.SourceID); err != nil {
			return res, err
		}
		res.RowsRemoved++
	}
	res.RowsSkipped = plan.skipped
	todo := plan.work

	var pending []embedWork
	pendingTexts := 0
	need := func(w embedWork) int {
		n := 0
		for _, c := range w.chunks {
			if _, ok := w.reuse[ChunkHash(c)]; !ok {
				n++
			}
		}
		return n
	}
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		type ref struct{ work, chunk int }
		var texts []string
		var refs []ref
		for wi, w := range pending {
			for ci, c := range w.chunks {
				if _, ok := w.reuse[ChunkHash(c)]; !ok {
					texts = append(texts, c)
					refs = append(refs, ref{wi, ci})
				}
			}
		}
		vecs := make([][]float32, len(texts))
		for i := 0; i < len(texts); i += size {
			if err := ctx.Err(); err != nil {
				return err
			}
			j := min(i+size, len(texts))
			out, err := e.Embed(texts[i:j])
			if err != nil {
				return fmt.Errorf("knowledge: embedding with %s: %w", e.Model(), err)
			}
			if len(out) != j-i {
				return fmt.Errorf("knowledge: model %s returned %d vectors for %d texts", e.Model(), len(out), j-i)
			}
			copy(vecs[i:j], out)
		}
		// Still the owner of the lock? Checked after the slow part and before any
		// write, so a run that stalled and lost its claim writes nothing.
		if err := lock.Heartbeat(); err != nil {
			return err
		}
		fresh := make([]map[int][]float32, len(pending))
		for i, r := range refs {
			if fresh[r.work] == nil {
				fresh[r.work] = map[int][]float32{}
			}
			fresh[r.work][r.chunk] = vecs[i]
		}
		for wi, w := range pending {
			stored := make([]StoredChunk, len(w.chunks))
			for ci, c := range w.chunks {
				h := ChunkHash(c)
				vec, ok := w.reuse[h]
				if !ok {
					vec = fresh[wi][ci]
				}
				stored[ci] = StoredChunk{Index: ci, Hash: h, Text: c, Vector: vec}
			}
			if err := v.PutRow(w.row.SourceType, w.row.SourceID, stored); err != nil {
				return err
			}
			res.RowsEmbedded++
		}
		res.ChunksEmbedded += len(texts)
		pending, pendingTexts = nil, 0
		if opts.Progress != nil {
			opts.Progress(EmbedProgress{Done: res.RowsEmbedded, Total: len(todo), ChunksEmbedded: res.ChunksEmbedded})
		}
		return nil
	}
	for _, w := range todo {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		pending = append(pending, w)
		pendingTexts += need(w)
		if pendingTexts >= size {
			if err := flush(); err != nil {
				return res, err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return res, err
	}
	return res, flush()
}

/** ResetVectors deletes the sidecar of a database, if there is one, and creates
 * a fresh one for the embedder. It is the explicit rebuild: EmbedIndex refuses a
 * different model, and this is how the user chooses to start over.
 *
 * Parameters:
 *   dbPath (string) — the knowledge database's path (the sidecar's is derived)
 *   e (Embedder) — the embedder whose vectors the new sidecar will hold
 *
 * Returns:
 *   *Vectors — the new, empty sidecar
 *   error — an OS error removing the old file, or as CreateVectors
 *
 * Example:
 *   v, err := knowledge.ResetVectors("agents/knowledge.db", embedder)
 */
func ResetVectors(dbPath string, e Embedder) (*Vectors, error) {
	path := VectorsPath(dbPath)
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("knowledge: remove %s: %w", p, err)
		}
	}
	return CreateVectors(dbPath, e)
}

// embedPlan is what EmbedIndex would do, worked out without changing anything.
type embedPlan struct {
	work    []embedWork // rows to embed, new or changed
	remove  []RowKey    // rows whose chunks go: the row is gone, or has no text
	skipped int         // rows already up to date
	corpus  []CorpusRow // rows in the corpus
}

// planEmbed compares the corpus with what the sidecar stores. It reads both and
// writes neither, so EmbedIndex and EmbedStatus cannot disagree about what is
// missing.
func (kb *KnowledgeBase) planEmbed(v *Vectors, chunkRunes int) (embedPlan, error) {
	var plan embedPlan
	corpus, err := kb.EmbedCorpus(SearchScope{})
	if err != nil {
		return plan, err
	}
	plan.corpus = corpus
	inCorpus := make(map[RowKey]bool, len(corpus))
	for _, r := range corpus {
		inCorpus[RowKey{r.SourceType, r.SourceID}] = true
	}
	indexed, err := v.IndexedRows()
	if err != nil {
		return plan, err
	}
	for _, k := range indexed {
		if !inCorpus[k] {
			plan.remove = append(plan.remove, k)
		}
	}
	for _, r := range corpus {
		chunks := Chunk(r.Text, chunkRunes)
		stored, err := v.RowChunks(r.SourceType, r.SourceID)
		if err != nil {
			return plan, err
		}
		if len(chunks) == 0 {
			if len(stored) > 0 {
				plan.remove = append(plan.remove, RowKey{r.SourceType, r.SourceID})
			} else {
				plan.skipped++
			}
			continue
		}
		reuse := make(map[string][]float32, len(stored))
		for _, c := range stored {
			reuse[c.Hash] = c.Vector
		}
		if len(stored) == len(chunks) {
			same := true
			for i, c := range chunks {
				if stored[i].Hash != ChunkHash(c) {
					same = false
					break
				}
			}
			if same {
				plan.skipped++
				continue
			}
		}
		plan.work = append(plan.work, embedWork{row: r, chunks: chunks, reuse: reuse})
	}
	return plan, nil
}

/** EmbedStatus says how far the vector sidecar is from the corpus.
 *
 * Fields:
 *   Rows          (int)            — rows in the corpus
 *   RowsByType    (map[string]int) — corpus rows by kb_fts source_type
 *   RowsUpToDate  (int)            — rows whose chunks are all stored
 *   RowsToEmbed   (int)            — rows new or changed since they were embedded
 *   RowsToRemove  (int)            — indexed rows that are gone or have no text
 *   ChunksStored  (int)            — chunks in the sidecar
 *   ChunksToEmbed (int)            — chunks a run would send to the embedder
 */
type EmbedStatus struct {
	Rows, RowsUpToDate, RowsToEmbed, RowsToRemove, ChunksStored, ChunksToEmbed int
	RowsByType                                                                 map[string]int
}

/** EmbedStatus reports what EmbedIndex would do, changing nothing and loading
 * no model.
 *
 * Parameters:
 *   v (*Vectors) — the sidecar to compare
 *   chunkRunes (int) — the longest chunk; zero means DefaultChunkRunes
 *
 * Returns:
 *   EmbedStatus — the counts
 *   error — a database error
 *
 * Example:
 *   st, err := kb.EmbedStatus(v, 0)
 *   fmt.Printf("%d of %d rows not indexed\n", st.RowsToEmbed, st.Rows)
 */
func (kb *KnowledgeBase) EmbedStatus(v *Vectors, chunkRunes int) (EmbedStatus, error) {
	if chunkRunes <= 0 {
		chunkRunes = DefaultChunkRunes
	}
	plan, err := kb.planEmbed(v, chunkRunes)
	if err != nil {
		return EmbedStatus{}, err
	}
	st := EmbedStatus{
		Rows: len(plan.corpus), RowsUpToDate: plan.skipped, RowsToEmbed: len(plan.work),
		RowsToRemove: len(plan.remove), RowsByType: map[string]int{},
	}
	for _, w := range plan.work {
		for _, c := range w.chunks {
			if _, ok := w.reuse[ChunkHash(c)]; !ok {
				st.ChunksToEmbed++
			}
		}
	}
	for _, r := range plan.corpus {
		st.RowsByType[r.SourceType]++
	}
	if st.ChunksStored, err = v.ChunkCount(); err != nil {
		return st, err
	}
	return st, nil
}
