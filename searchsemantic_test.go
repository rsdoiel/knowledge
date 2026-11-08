package knowledge

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// The library pieces behind kb search -semantic and -hybrid (semantic plan T8).

func TestHitsFor_ReturnsTheDisplayFieldsInTheOrderAsked(t *testing.T) {
	kb := searchHitFixture(t)
	lex, err := kb.Search("toast")
	if err != nil {
		t.Fatal(err)
	}
	var keys []RowKey
	for i := len(lex) - 1; i >= 0; i-- { // the reverse of the search order
		keys = append(keys, RowKey{lex[i].SourceType, lex[i].SourceID})
	}
	keys = append(keys, RowKey{"observation", 9999}) // no such row
	got, err := kb.HitsFor(keys)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(lex) {
		t.Fatalf("got %d hits, want %d (a missing row is left out)", len(got), len(lex))
	}
	for i, h := range got {
		want := lex[len(lex)-1-i]
		if h.SourceType != want.SourceType || h.SourceID != want.SourceID || h.Ref != want.Ref ||
			h.Label != want.Label || h.Status != want.Status {
			t.Errorf("hit %d = %+v, want the fields of %+v", i, h, want)
		}
		if h.Passage != "" {
			t.Errorf("hit %d has a passage %q: with no query there is none", i, h.Passage)
		}
	}
}

func TestLexicalRanking_OnlyEmbeddedKindsAndDeeperThanAPage(t *testing.T) {
	kb := openTestKB(t)
	p, _ := kb.AddProject("alpha", "stoat")
	for i := 0; i < 80; i++ {
		kb.AddObservation(p, "note", "stoat number "+strings.Repeat("x", i%3))
	}
	keys, err := kb.LexicalRanking("stoat", SearchScope{})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 80 {
		t.Errorf("got %d keys, want 80 (the project row matches too and is not an embedded kind)", len(keys))
	}
	for _, k := range keys {
		if k.SourceType == "project" {
			t.Error("a project is not an embedded kind")
		}
	}
	if _, err := kb.LexicalRanking("stoat", SearchScope{Kinds: []string{"projects"}}); !errors.Is(err, ErrInvalid) {
		t.Errorf("projects: err = %v, want ErrInvalid", err)
	}
	if k, err := kb.LexicalRanking("map-reduce", SearchScope{}); err != nil || len(k) != 0 {
		t.Errorf("unparseable term: %v %v", k, err)
	}
}

func TestBuilding_IsTrueWhileAFreshClaimIsHeld(t *testing.T) {
	a, b, _, clockB := twoHandles(t)
	if busy, err := b.Building(); err != nil || busy {
		t.Fatalf("no claim: %v %v", busy, err)
	}
	l, _ := a.Lock()
	if busy, _ := b.Building(); !busy {
		t.Error("a held claim is a build in progress")
	}
	*clockB = t0.Add(LockStaleAfter + time.Second)
	if busy, _ := b.Building(); busy {
		t.Error("a stale claim is a dead run, not a build in progress")
	}
	*clockB = t0
	l.Release()
	if busy, _ := b.Building(); busy {
		t.Error("released")
	}
}

// DR-0094: LexicalRanking stays the query as typed; the widening lives in the
// hybrid search, so the evaluation set's own any-word measurement is unchanged.
func TestLexicalRanking_AQuestionTypedAsASentenceStillMatchesNothing(t *testing.T) {
	kb := openTestKB(t)
	p, _ := kb.AddProject("alpha", "ferries")
	kb.AddObservation(p, "note", "the ferry timetable changed in winter")
	keys, err := kb.LexicalRanking("when did the ferry schedule change for the winter timetable", SearchScope{})
	if err != nil || len(keys) != 0 {
		t.Errorf("keys = %v, err = %v: the typed query ANDs every word", keys, err)
	}
}

func TestHasFTS5Syntax(t *testing.T) {
	for term, want := range map[string]bool{
		// plain words, including the punctuation of ordinary prose
		"when did the ferry change": false, "map-reduce slow": false, "ferry and winter": false,
		"Note: the ferry timetable changed": false, "ferry (the winter one) timetable": false,
		`why did the "ferry" change, again?`: false, "title:ferry": false, "a^b ferry": false,
		// written on purpose in FTS5 syntax
		"ferry AND winter": true, "ferry NOT winter": true, "ferry NEAR winter": true, "ferry OR winter": true,
		`"ferry timetable"`: true, `winter "ferry timetable" change`: true, "timeta*": true, "+ferry winter": true,
		"^ferry": true, "kind:record ferry": true, "body:ferry": true,
	} {
		if got := hasFTS5Syntax(term); got != want {
			t.Errorf("hasFTS5Syntax(%q) = %v, want %v", term, got, want)
		}
	}
}

func TestAnyWordTerm(t *testing.T) {
	if got := anyWordTerm("why did the model give empty replies"); got != `"model" OR "give" OR "empty" OR "replies"` {
		t.Errorf("got %s", got)
	}
	if got := anyWordTerm("is it so"); got != "" {
		t.Errorf("words under four letters: got %q, want none", got)
	}
}

func TestLexicalRankingWide_WidensOnlyBelowThreeRowsAndOnlyPlainWords(t *testing.T) {
	kb := openTestKB(t)
	p, _ := kb.AddProject("alpha", "ferries")
	kb.AddObservation(p, "note", "the ferry timetable changed in winter")
	kb.AddObservation(p, "note", "ferry repair costs")
	typed, wide, err := kb.LexicalRankingWide("when did the ferry schedule change for the winter timetable", SearchScope{})
	if err != nil || len(typed) != 0 || len(wide) != 2 {
		t.Fatalf("typed %d, widened %d, err %v: want 0 and 2", len(typed), len(wide), err)
	}
	if typed, wide, _ := kb.LexicalRankingWide("ferry AND schedule", SearchScope{}); len(typed) != 0 || wide != nil {
		t.Errorf("FTS5 syntax was widened: %d, %v", len(typed), wide)
	}
	kb.AddObservation(p, "note", "ferry alpha")
	kb.AddObservation(p, "note", "ferry beta")
	if typed, wide, _ := kb.LexicalRankingWide("ferry", SearchScope{}); len(typed) != 4 || wide != nil {
		t.Errorf("a query that finds rows was widened: %d, %v", len(typed), wide)
	}
}
