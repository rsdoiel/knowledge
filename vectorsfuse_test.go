package knowledge

import (
	"fmt"
	"math"
	"testing"
)

// FuseRankings merges the lexical and the semantic ranking of a hybrid search
// (semantic-search-plan.md T3.8): reciprocal-rank fusion, with a row found by
// both ranked above a row found by either alone.

func rk(ids ...int) []RowKey {
	var out []RowKey
	for _, id := range ids {
		out = append(out, RowKey{"observation", int64(id)})
	}
	return out
}

func fusedIDs(rows []FusedRow) string {
	s := ""
	for _, r := range rows {
		s += fmt.Sprintf("%d ", r.SourceID)
	}
	return s
}

func TestFuseRankings_ARowInBothListsRanksAboveRowsInOne(t *testing.T) {
	got := FuseRankings(rk(1, 2, 3), rk(9, 3, 8), 0)
	if got[0].SourceID != 3 || !got[0].InBoth() {
		t.Errorf("order = %s, want 3 first (it is in both)", fusedIDs(got))
	}
}

func TestFuseRankings_BothBeatsOneEvenWhenTheOneIsRankedFirst(t *testing.T) {
	// Row 1000 is 80th in both lists: 2/140 = 0.0143. Row 1 is first in one list:
	// 1/61 = 0.0164. Plain reciprocal-rank fusion puts row 1 first; the design says
	// a row found by both comes first.
	var lex, sem []int
	for i := 1; i < 80; i++ {
		lex = append(lex, i)
		sem = append(sem, 2000+i)
	}
	lex = append(lex, 1000)
	sem = append(sem, 1000)
	got := FuseRankings(rk(lex...), rk(sem...), 0)
	if got[0].SourceID != 1000 {
		t.Fatalf("first = %d, want 1000 (in both lists)", got[0].SourceID)
	}
	var one FusedRow
	for _, r := range got {
		if r.SourceID == 1 {
			one = r
		}
	}
	if !(one.Score > got[0].Score) {
		t.Errorf("the test no longer separates the tier from the score: row 1 scores %v, row 1000 %v", one.Score, got[0].Score)
	}
}

func TestFuseRankings_ScoreIsTheSumOfReciprocalRanks(t *testing.T) {
	got := FuseRankings(rk(1, 2), rk(2, 3), 60)
	byID := map[int64]FusedRow{}
	for _, r := range got {
		byID[r.SourceID] = r
	}
	want := map[int64]float64{1: 1.0 / 61, 2: 1.0/62 + 1.0/61, 3: 1.0 / 62}
	for id, w := range want {
		if math.Abs(byID[id].Score-w) > 1e-12 {
			t.Errorf("row %d score = %v, want %v", id, byID[id].Score, w)
		}
	}
	if r := byID[2]; r.LexicalRank != 2 || r.SemanticRank != 1 {
		t.Errorf("row 2 ranks = %d, %d; want lexical 2, semantic 1", r.LexicalRank, r.SemanticRank)
	}
	if r := byID[1]; r.SemanticRank != 0 || r.InBoth() {
		t.Errorf("row 1 = %+v: it is in the lexical list only", r)
	}
}

func TestFuseRankings_WithinATierHigherScoreWins(t *testing.T) {
	got := FuseRankings(rk(1, 2, 3, 4), rk(4, 3, 2, 1), 0)
	// 1 and 4 have ranks (1,4) and (4,1); 2 and 3 have (2,3) and (3,2): the sums of
	// reciprocals are larger for the ends, so 1 and 4 come before 2 and 3.
	if got[0].SourceID == 2 || got[0].SourceID == 3 {
		t.Errorf("order = %s, want an end row first", fusedIDs(got))
	}
}

func TestFuseRankings_OneListIsThatList(t *testing.T) {
	if got := FuseRankings(rk(5, 6, 7), nil, 0); fusedIDs(got) != "5 6 7 " {
		t.Errorf("lexical only = %s", fusedIDs(got))
	}
	if got := FuseRankings(nil, rk(7, 6, 5), 0); fusedIDs(got) != "7 6 5 " {
		t.Errorf("semantic only = %s", fusedIDs(got))
	}
	if got := FuseRankings(nil, nil, 0); len(got) != 0 {
		t.Errorf("both empty = %v", got)
	}
}

func TestFuseRankings_TiesBreakTheSameWayEveryTime(t *testing.T) {
	// Rows 1 and 2 swap places between the lists: equal scores, both in both.
	want := fusedIDs(FuseRankings(rk(1, 2), rk(2, 1), 0))
	for i := 0; i < 20; i++ {
		if got := fusedIDs(FuseRankings(rk(1, 2), rk(2, 1), 0)); got != want {
			t.Fatalf("run %d gave %q, first run %q", i, got, want)
		}
	}
	if want != "1 2 " {
		t.Errorf("tie order = %q, want the lexical rank to decide: 1 2", want)
	}
}

func TestFuseRankings_ARepeatedRowCountsOnceAtItsBestRank(t *testing.T) {
	got := FuseRankings(rk(1, 2, 1), nil, 0)
	if len(got) != 2 || got[0].SourceID != 1 || got[0].LexicalRank != 1 {
		t.Errorf("got %+v", got)
	}
}

func TestFuseRankings_RowsOfDifferentTypesWithTheSameIDAreDifferentRows(t *testing.T) {
	lex := []RowKey{{"observation", 1}, {"record", 1}}
	got := FuseRankings(lex, []RowKey{{"record", 1}}, 0)
	if len(got) != 2 || got[0].SourceType != "record" {
		t.Errorf("got %+v, want record:1 first (in both) and observation:1 second", got)
	}
}

func TestFuseRankings_KChangesHowSteeplyRankMatters(t *testing.T) {
	small := FuseRankings(rk(1), nil, 1)[0].Score
	large := FuseRankings(rk(1), nil, 100)[0].Score
	if !(small > large) {
		t.Errorf("score with k=1 (%v) should exceed k=100 (%v)", small, large)
	}
}
