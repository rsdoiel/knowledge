package knowledge

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

/** DefaultMinScore is the cosine similarity a chunk must reach to be returned
 * when SemanticOptions.MinScore is zero. Without a cutoff nearest-neighbour
 * search never finds nothing. It was set from the labelled query set
 * (semantic-search-plan.md T9) for all-MiniLM-L6-v2: recall at 10 is unchanged
 * from 0.20 to 0.35 and falls at 0.40, and 0.35 halves the rows returned
 * compared with 0.30. Another model needs its own measurement.
 */
const DefaultMinScore = 0.35

/** NoCutoff, as SemanticOptions.MinScore, returns every indexed row however far
 * from the query it is: cosine similarity is never below -1.
 */
const NoCutoff = -1

// defaultSemanticLimit is the most rows SemanticSearch returns when Limit is zero,
// as for lexical search.
const defaultSemanticLimit = 50

/** SemanticOptions tunes SemanticSearch.
 *
 * Fields:
 *   Scope    (SearchScope) — which rows to rank, as for SearchScoped; only EmbeddedKinds can be asked for
 *   MinScore (float32)     — the lowest cosine similarity returned; zero means DefaultMinScore, NoCutoff means all
 *   Limit    (int)         — the most rows returned; zero means 50
 *
 * Example:
 *   opts := knowledge.SemanticOptions{Scope: knowledge.SearchScope{Project: "harvey"}, MinScore: 0.4}
 */
type SemanticOptions struct {
	Scope    SearchScope
	MinScore float32
	Limit    int
}

/** SemanticHit is one row found by SemanticSearch, with the chunk that matched.
 *
 * Fields:
 *   SourceType (string)  — kb_fts source_type of the row
 *   SourceID   (int64)   — the row's id
 *   Score      (float32) — cosine similarity of the best chunk, at most 1
 *   Chunk      (string)  — the best chunk's text, for the passage line
 *   ChunkIndex (int)     — the best chunk's position in the row
 *
 * Example:
 *   fmt.Printf("%s:%d %.2f %s\n", h.SourceType, h.SourceID, h.Score, h.Chunk)
 */
type SemanticHit struct {
	SourceType string
	SourceID   int64
	Score      float32
	Chunk      string
	ChunkIndex int
}

/** SemanticResult is what SemanticSearch returns.
 *
 * Fields:
 *   Hits      ([]SemanticHit) — up to Limit rows, best first
 *   Total     (int)           — how many rows reached the cutoff, before the limit
 *   Unindexed (int)           — rows in scope with no chunks yet, which the search could not see
 *
 * Example:
 *   r, _ := kb.SemanticSearch(v, e, "slow inference", knowledge.SemanticOptions{})
 *   fmt.Printf("showing %d of %d\n", len(r.Hits), r.Total)
 */
type SemanticResult struct {
	Hits      []SemanticHit
	Total     int
	Unindexed int
}

/** SemanticSearch ranks the indexed rows by how close their best chunk is to
 * the query, by cosine similarity, and returns those at or above the cutoff.
 * Nothing close is an empty result, not an error. It only reads: rows with no
 * chunks yet are counted in Unindexed and never indexed here. The index must
 * have been built by the same model as e.
 *
 * Parameters:
 *   v (*Vectors) — the open sidecar
 *   e (Embedder) — the model that embeds the query; it must be the one v was built with
 *   query (string) — plain text; no search syntax applies
 *   opts (SemanticOptions) — scope, cutoff and limit
 *
 * Returns:
 *   *SemanticResult — the hits, the total before the limit, and the unindexed count
 *   error — ErrInvalid for a blank query, a cutoff outside -1 to 1 or a kind that
 *     is not embedded; ErrNotFound for an unknown project; ErrModelMismatch; the
 *     embedder's error; a database error
 *
 * Example:
 *   r, err := kb.SemanticSearch(v, embedder, "why was the compiler licence chosen", knowledge.SemanticOptions{})
 */
func (kb *KnowledgeBase) SemanticSearch(v *Vectors, e Embedder, query string, opts SemanticOptions) (*SemanticResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, invalidf("knowledge: semantic search needs a query")
	}
	min := opts.MinScore
	if min == 0 {
		min = DefaultMinScore
	}
	if min < NoCutoff || min > 1 {
		return nil, invalidf("knowledge: minimum score %v is outside -1 to 1", opts.MinScore)
	}
	limit := opts.Limit
	if limit < 0 {
		return nil, invalidf("knowledge: search limit %d is negative", limit)
	}
	if limit == 0 {
		limit = defaultSemanticLimit
	}
	rows, err := kb.EmbedCorpus(opts.Scope)
	if err != nil {
		return nil, err
	}
	if err := v.CheckModel(e); err != nil {
		return nil, err
	}
	candidate := make(map[RowKey]bool, len(rows))
	for _, r := range rows {
		candidate[RowKey{r.SourceType, r.SourceID}] = true
	}

	qv, err := e.Embed([]string{query})
	if err != nil {
		return nil, fmt.Errorf("knowledge: embedding the query with %s: %w", e.Model(), err)
	}
	if len(qv) != 1 || len(qv[0]) != v.meta.Dimensions {
		return nil, fmt.Errorf("knowledge: model %s returned a malformed query vector", e.Model())
	}
	q := qv[0]
	qn := norm(q)
	if qn == 0 {
		return nil, invalidf("knowledge: the query has a zero vector")
	}

	dbRows, err := v.db.Query(`SELECT source_type, source_id, chunk_index, text, vector FROM vector_chunks`)
	if err != nil {
		return nil, fmt.Errorf("knowledge: vector index: %w", err)
	}
	defer dbRows.Close()
	best := map[RowKey]SemanticHit{}
	for dbRows.Next() {
		var h SemanticHit
		var blob []byte
		if err := dbRows.Scan(&h.SourceType, &h.SourceID, &h.ChunkIndex, &h.Chunk, &blob); err != nil {
			return nil, fmt.Errorf("knowledge: vector index: %w", err)
		}
		key := RowKey{h.SourceType, h.SourceID}
		if !candidate[key] {
			continue
		}
		cv, err := decodeVector(blob, v.meta.Dimensions)
		if err != nil {
			return nil, fmt.Errorf("knowledge: vector index %s:%d chunk %d: %w", h.SourceType, h.SourceID, h.ChunkIndex, err)
		}
		cn := norm(cv)
		if cn == 0 {
			continue
		}
		h.Score = dot(q, cv) / (qn * cn)
		if cur, ok := best[key]; !ok || h.Score > cur.Score || h.Score == cur.Score && h.ChunkIndex < cur.ChunkIndex {
			best[key] = h
		}
	}
	if err := dbRows.Err(); err != nil {
		return nil, fmt.Errorf("knowledge: vector index: %w", err)
	}

	res := &SemanticResult{}
	for key := range candidate {
		if _, ok := best[key]; !ok {
			res.Unindexed++
		}
	}
	var hits []SemanticHit
	for _, h := range best {
		if h.Score >= min {
			hits = append(hits, h)
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		switch {
		case a.Score != b.Score:
			return a.Score > b.Score
		case a.SourceType != b.SourceType:
			return a.SourceType < b.SourceType
		}
		return a.SourceID < b.SourceID
	})
	res.Total = len(hits)
	if len(hits) > limit {
		hits = hits[:limit]
	}
	res.Hits = hits
	return res, nil
}

func dot(a, b []float32) float32 {
	var s float32
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

func norm(a []float32) float32 { return float32(math.Sqrt(float64(dot(a, a)))) }
