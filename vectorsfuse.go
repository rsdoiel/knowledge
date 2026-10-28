package knowledge

import "sort"

/** DefaultFusionK is the constant in reciprocal-rank fusion, 1/(k + rank). The
 * usual value is 60. It is a starting point, to be checked against the labelled
 * query set (semantic-search-plan.md T9).
 */
const DefaultFusionK = 60

/** FusedRow is one row of a hybrid search with where each ranking put it.
 *
 * Fields:
 *   SourceType   (string)  — kb_fts source_type of the row
 *   SourceID     (int64)   — the row's id
 *   Score        (float64) — the sum of 1/(k + rank) over the rankings that found the row
 *   LexicalRank  (int)     — its place in the lexical ranking from 1, or 0 if that ranking did not find it
 *   SemanticRank (int)     — its place in the semantic ranking from 1, or 0 if that ranking did not find it
 *
 * Example:
 *   for _, r := range knowledge.FuseRankings(lexical, semantic, 0) {
 *       fmt.Println(r.SourceType, r.SourceID, r.InBoth())
 *   }
 */
type FusedRow struct {
	SourceType   string
	SourceID     int64
	Score        float64
	LexicalRank  int
	SemanticRank int
}

/** InBoth reports whether both rankings found the row. */
func (r FusedRow) InBoth() bool { return r.LexicalRank > 0 && r.SemanticRank > 0 }

/** FuseRankings merges the lexical and the semantic ranking of one query into
 * one, by reciprocal-rank fusion: a row's score is the sum of 1/(k + rank) over
 * the rankings that found it. A row found by both rankings comes before any row
 * found by one (the design, semantic-search-design.md section 4: plain fusion
 * would let a first place in one ranking outscore a fiftieth place in both);
 * within each group the higher score comes first, and a tie goes to the better
 * lexical rank, then the better semantic rank, then type and id, so the order is
 * the same every time. A row listed twice counts once, at its best place.
 *
 * Parameters:
 *   lexical ([]RowKey) — the rows lexical search found, best first
 *   semantic ([]RowKey) — the rows semantic search found, best first
 *   k (int) — the fusion constant; zero or less means DefaultFusionK
 *
 * Returns:
 *   []FusedRow — every row either ranking found, best first
 *
 * Example:
 *   fused := knowledge.FuseRankings(lexicalKeys, semanticKeys, knowledge.DefaultFusionK)
 */
func FuseRankings(lexical, semantic []RowKey, k int) []FusedRow {
	if k <= 0 {
		k = DefaultFusionK
	}
	rows := map[RowKey]*FusedRow{}
	add := func(list []RowKey, lexicalList bool) {
		rank := 0
		for _, key := range list {
			r := rows[key]
			if r == nil {
				r = &FusedRow{SourceType: key.SourceType, SourceID: key.SourceID}
				rows[key] = r
			}
			if (lexicalList && r.LexicalRank > 0) || (!lexicalList && r.SemanticRank > 0) {
				continue // a repeat: the first, best place stands
			}
			rank++
			if lexicalList {
				r.LexicalRank = rank
			} else {
				r.SemanticRank = rank
			}
			r.Score += 1 / float64(k+rank)
		}
	}
	add(lexical, true)
	add(semantic, false)

	out := make([]FusedRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, *r)
	}
	// 0 means "not found", which sorts after every real rank.
	rankKey := func(r int) int {
		if r == 0 {
			return 1 << 30
		}
		return r
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch {
		case a.InBoth() != b.InBoth():
			return a.InBoth()
		case a.Score != b.Score:
			return a.Score > b.Score
		case rankKey(a.LexicalRank) != rankKey(b.LexicalRank):
			return rankKey(a.LexicalRank) < rankKey(b.LexicalRank)
		case rankKey(a.SemanticRank) != rankKey(b.SemanticRank):
			return rankKey(a.SemanticRank) < rankKey(b.SemanticRank)
		case a.SourceType != b.SourceType:
			return a.SourceType < b.SourceType
		}
		return a.SourceID < b.SourceID
	})
	return out
}
