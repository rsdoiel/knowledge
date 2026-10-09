package knowledge

import (
	"slices"
	"testing"
)

// transitionWant is the confirmed D0 table (DR-0060), written out by hand so
// the test does not read the implementation to learn the answer. Each cell is
// the set of targets from a source; "s" marks superseded, which is allowed only
// when superseded_by is already set.
//
//	from \ to   proposed accepted rejected cancelled superseded
//	proposed       -        Y        Y         Y        Y*
//	accepted       N        -        N         Y        Y*
//	rejected       Y        N        -         N        N
//	cancelled      Y        N        N         -        N
//	superseded     N        N        N         N        -
//
// * only with superseded_by set.
var transitionWant = map[string]map[string]bool{
	"proposed":   {"accepted": true, "rejected": true, "cancelled": true, "superseded": true},
	"accepted":   {"cancelled": true, "superseded": true},
	"rejected":   {"proposed": true},
	"cancelled":  {"proposed": true},
	"superseded": {},
}

// TestCanTransitionMatrix walks every ordered pair of statuses, with and
// without superseded_by, and checks CanTransition against the confirmed table.
func TestCanTransitionMatrix(t *testing.T) {
	for _, from := range RecordStatuses {
		for _, to := range RecordStatuses {
			for _, linked := range []bool{false, true} {
				want := transitionWant[from][to]
				if to == "superseded" && !linked {
					want = false
				}
				if got := CanTransition(from, to, linked); got != want {
					t.Errorf("CanTransition(%q, %q, superseded_by=%v) = %v, want %v",
						from, to, linked, got, want)
				}
			}
		}
	}
}

// TestCanTransitionRefusesSameStatus pins the rule that a same-status request
// is refused rather than treated as a no-op success.
func TestCanTransitionRefusesSameStatus(t *testing.T) {
	for _, s := range RecordStatuses {
		for _, linked := range []bool{false, true} {
			if CanTransition(s, s, linked) {
				t.Errorf("CanTransition(%q, %q, %v) = true, want false", s, s, linked)
			}
		}
	}
}

// TestCanTransitionUnknownStatus checks that a status outside the vocabulary
// is never a source or a target.
func TestCanTransitionUnknownStatus(t *testing.T) {
	for _, s := range RecordStatuses {
		if CanTransition("bogus", s, true) || CanTransition(s, "bogus", true) {
			t.Errorf("an unknown status was accepted against %q", s)
		}
	}
	if got := AllowedTransitions("bogus", true); len(got) != 0 {
		t.Errorf("AllowedTransitions(bogus) = %v, want none", got)
	}
}

// TestAllowedTransitionsAgreesWithCanTransition checks the list form against
// the predicate for every source, and that the list is in RecordStatuses order
// so the review prompt is stable.
func TestAllowedTransitionsAgreesWithCanTransition(t *testing.T) {
	for _, from := range RecordStatuses {
		for _, linked := range []bool{false, true} {
			var want []string
			for _, to := range RecordStatuses {
				if CanTransition(from, to, linked) {
					want = append(want, to)
				}
			}
			got := AllowedTransitions(from, linked)
			if !slices.Equal(got, want) {
				t.Errorf("AllowedTransitions(%q, %v) = %v, want %v", from, linked, got, want)
			}
		}
	}
}

// TestAllowedTransitionsExamples pins the cases the review prompt and the
// release notes quote.
func TestAllowedTransitionsExamples(t *testing.T) {
	cases := []struct {
		from   string
		linked bool
		want   []string
	}{
		{"proposed", false, []string{"accepted", "rejected", "cancelled"}},
		{"proposed", true, []string{"accepted", "superseded", "rejected", "cancelled"}},
		{"accepted", false, []string{"cancelled"}},
		{"accepted", true, []string{"superseded", "cancelled"}},
		{"rejected", false, []string{"proposed"}},
		{"cancelled", true, []string{"proposed"}},
		{"superseded", true, nil},
	}
	for _, c := range cases {
		got := AllowedTransitions(c.from, c.linked)
		if !slices.Equal(got, c.want) {
			t.Errorf("AllowedTransitions(%q, %v) = %v, want %v", c.from, c.linked, got, c.want)
		}
	}
}

// TestTransitionTableCoversVocabulary reads the hand-written table and
// RecordStatuses together, so a status added to the vocabulary without a row
// in the table, or a row for a status that is gone, fails here.
func TestTransitionTableCoversVocabulary(t *testing.T) {
	for _, s := range RecordStatuses {
		if _, ok := transitionWant[s]; !ok {
			t.Errorf("status %q is in RecordStatuses but has no row in the transition table", s)
		}
	}
	for from, row := range transitionWant {
		if !slices.Contains(RecordStatuses, from) {
			t.Errorf("transition table has a row for %q, which is not in RecordStatuses", from)
		}
		for to := range row {
			if !slices.Contains(RecordStatuses, to) {
				t.Errorf("transition table has a target %q, which is not in RecordStatuses", to)
			}
		}
	}
}
