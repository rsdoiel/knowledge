package knowledge

import "slices"

// recordTransitions is the D0 table (DR-0060): for each status, the statuses a
// record may move to. superseded is listed where it is reachable, but see
// CanTransition: it also needs superseded_by. Whether a person must be at a
// terminal for accepted (DR-0061) is the command's rule, not the table's.
var recordTransitions = map[string][]string{
	"proposed":   {"accepted", "rejected", "cancelled", "superseded"},
	"accepted":   {"cancelled", "superseded"},
	"rejected":   {"proposed"},
	"cancelled":  {"proposed"},
	"superseded": {},
}

/** AllowedTransitions returns the statuses a record may move to from the given
 * status, in RecordStatuses order. The table is DR-0060 as confirmed for D0: the
 * single source the direct form, the review prompt and the TUI all read.
 *
 * Parameters:
 *   from (string) — the record's current status
 *   hasSupersededBy (bool) — whether the record already carries superseded_by
 *
 * Returns:
 *   []string — the permitted target statuses; empty for a terminal or unknown status
 *
 * Example:
 *   AllowedTransitions("proposed", false) // [accepted rejected]
 */
func AllowedTransitions(from string, hasSupersededBy bool) []string {
	var out []string
	for _, to := range RecordStatuses {
		if CanTransition(from, to, hasSupersededBy) {
			out = append(out, to)
		}
	}
	return out
}

/** CanTransition reports whether a record may move from one status to another.
 *
 * Parameters:
 *   from (string) — the current status
 *   to (string) — the requested status
 *   hasSupersededBy (bool) — whether the record already carries superseded_by
 *
 * Returns:
 *   bool — true when the move is in the table; a same-status request is false
 *
 * Example:
 *   CanTransition("rejected", "accepted", false) // false
 */
func CanTransition(from, to string, hasSupersededBy bool) bool {
	if from == to || !slices.Contains(RecordStatuses, to) {
		return false
	}
	if to == "superseded" && !hasSupersededBy {
		return false
	}
	return slices.Contains(recordTransitions[from], to)
}
