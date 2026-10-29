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
