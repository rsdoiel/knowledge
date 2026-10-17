package knowledge

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

/** StoryTime is a parsed story_time (DR-0079 item 2): when an event happens
 * in the story, kept as an interval with a precision, or as an ordering
 * relation to another event, or both once the relation has been resolved.
 *
 * Start and End are ISO dates (2006-01-02) for the calendar precisions and
 * decimal integers for "ordinal". Relation, Anchor and Offset describe a
 * relative time ("after: e12 +3d"); Offset is "" for an ordering edge with no
 * interval, otherwise a sign, a whole number and a unit, always written with
 * its "+".
 *
 * Example:
 *   t, _ := knowledge.ParseStoryTime("circa 1890")
 *   fmt.Println(t.Precision, t.Start, t.End) // circa 1890-01-01 1890-12-31
 */
type StoryTime struct {
	Precision string
	Start     string
	End       string
	Relation  string
	Anchor    string
	Offset    string
}

/** HasInterval reports whether the time places the event on the timeline by
 * itself: a date, a year, a circa year, an ordinal or a resolved relation.
 *
 * Returns:
 *   bool — true when Start and End are set.
 *
 * Example:
 *   knowledge.StoryTime{Relation: "after", Anchor: "x"}.HasInterval() // false
 */
func (t StoryTime) HasInterval() bool { return t.Start != "" && t.End != "" }

const storyDateLayout = "2006-01-02"

var (
	storyYearPattern    = regexp.MustCompile(`^\d{4}$`)
	storyMonthPattern   = regexp.MustCompile(`^\d{4}-\d{2}$`)
	storyOffsetPattern  = regexp.MustCompile(`^\+?(\d+)([dwmy]?)$`)
	storyRelationPrefix = regexp.MustCompile(`^(after|before):\s*(.*)$`)
)

/** ParseStoryTime reads a story_time value.
 *
 * Parameters:
 *   s (string) — one of: 1969-10-29, 1969-10, 1969, circa 1890,
 *     ordinal: 12, after: LABEL [OFFSET], before: LABEL [OFFSET]. An
 *     offset is a whole number with a unit d, w, m or y, or a bare number
 *     against an ordinal anchor.
 *
 * Returns:
 *   StoryTime — the parsed time; the zero value for an empty string.
 *   error     — ErrInvalid for anything else, including an impossible
 *     date and a negative offset.
 *
 * Example:
 *   t, err := knowledge.ParseStoryTime("after: e12 +3d")
 *   // t.Relation == "after", t.Anchor == "e12", t.Offset == "+3d"
 */
func ParseStoryTime(s string) (StoryTime, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return StoryTime{}, nil
	}
	if m := storyRelationPrefix.FindStringSubmatch(s); m != nil {
		fields := strings.Fields(m[2])
		if len(fields) == 0 || len(fields) > 2 {
			return StoryTime{}, invalidf("knowledge: story_time %q: want %s: LABEL [OFFSET]", s, m[1])
		}
		t := StoryTime{Relation: m[1], Anchor: fields[0]}
		if len(fields) == 2 {
			om := storyOffsetPattern.FindStringSubmatch(fields[1])
			if om == nil {
				return StoryTime{}, invalidf("knowledge: story_time %q: offset %q is a whole number with a unit d, w, m or y", s, fields[1])
			}
			t.Offset = "+" + om[1] + om[2]
		}
		return t, nil
	}
	if rest, ok := strings.CutPrefix(s, "ordinal:"); ok {
		n, err := strconv.Atoi(strings.TrimSpace(rest))
		if err != nil {
			return StoryTime{}, invalidf("knowledge: story_time %q: ordinal wants a whole number", s)
		}
		return StoryTime{Precision: "ordinal", Start: strconv.Itoa(n), End: strconv.Itoa(n)}, nil
	}
	precision, text := "", s
	if rest, ok := strings.CutPrefix(s, "circa "); ok {
		precision, text = "circa", strings.TrimSpace(rest)
		if !storyYearPattern.MatchString(text) {
			return StoryTime{}, invalidf("knowledge: story_time %q: circa wants a four-digit year", s)
		}
	}
	switch {
	case storyYearPattern.MatchString(text):
		y, _ := strconv.Atoi(text)
		if precision == "" {
			precision = "year"
		}
		return StoryTime{Precision: precision, Start: fmt.Sprintf("%04d-01-01", y), End: fmt.Sprintf("%04d-12-31", y)}, nil
	case storyMonthPattern.MatchString(text):
		first, err := time.Parse("2006-01", text)
		if err != nil {
			return StoryTime{}, invalidf("knowledge: story_time %q: not a month", s)
		}
		last := first.AddDate(0, 1, -1)
		return StoryTime{Precision: "month", Start: first.Format(storyDateLayout), End: last.Format(storyDateLayout)}, nil
	}
	d, err := time.Parse(storyDateLayout, text)
	if err != nil {
		return StoryTime{}, invalidf("knowledge: story_time %q is not a date, a month, a year, circa YEAR, ordinal: N or after:/before: LABEL", s)
	}
	return StoryTime{Precision: "day", Start: d.Format(storyDateLayout), End: d.Format(storyDateLayout)}, nil
}

/** ResolveStoryTime gives a relative event its interval from its anchor's:
 * "after" adds the offset to the anchor's end, "before" subtracts it from the
 * anchor's start (DR-0079). The result has day precision for dates and
 * ordinal precision for ordinals. Month and year offsets use time.AddDate, so
 * 31 January plus one month is 3 March.
 *
 * Parameters:
 *   rel    (StoryTime) — a relative time with a Relation and an Offset.
 *   anchor (StoryTime) — the anchor event's time, which must have an interval.
 *
 * Returns:
 *   StoryTime — rel with Precision, Start and End filled in.
 *   error     — ErrInvalid when there is no offset, the anchor has no interval,
 *     or the offset's unit does not fit the anchor (a day offset on an
 *     ordinal, or a bare number on dates).
 *
 * Example:
 *   anchor, _ := knowledge.ParseStoryTime("1969-10-29")
 *   rel, _ := knowledge.ParseStoryTime("after: x +3d")
 *   got, _ := knowledge.ResolveStoryTime(rel, anchor) // 1969-11-01
 */
func ResolveStoryTime(rel, anchor StoryTime) (StoryTime, error) {
	if rel.Offset == "" {
		return StoryTime{}, invalidf("knowledge: story_time: %s %s has no offset, so it is an ordering edge only", rel.Relation, rel.Anchor)
	}
	if !anchor.HasInterval() {
		return StoryTime{}, invalidf("knowledge: story_time: anchor %q has no interval", rel.Anchor)
	}
	om := storyOffsetPattern.FindStringSubmatch(rel.Offset)
	if om == nil {
		return StoryTime{}, invalidf("knowledge: story_time: offset %q", rel.Offset)
	}
	n, _ := strconv.Atoi(om[1])
	unit := om[2]
	out := rel
	if anchor.Precision == "ordinal" {
		if unit != "" {
			return StoryTime{}, invalidf("knowledge: story_time: ordinal anchor %q takes a bare number offset, not %q", rel.Anchor, rel.Offset)
		}
		start, _ := strconv.Atoi(anchor.Start)
		end, _ := strconv.Atoi(anchor.End)
		v := end + n
		if rel.Relation == "before" {
			v = start - n
		}
		out.Precision, out.Start, out.End = "ordinal", strconv.Itoa(v), strconv.Itoa(v)
		return out, nil
	}
	if unit == "" {
		return StoryTime{}, invalidf("knowledge: story_time: offset %q on a dated anchor needs a unit d, w, m or y", rel.Offset)
	}
	base, err := time.Parse(storyDateLayout, anchor.End)
	if rel.Relation == "before" {
		base, err = time.Parse(storyDateLayout, anchor.Start)
		n = -n
	}
	if err != nil {
		return StoryTime{}, invalidf("knowledge: story_time: anchor %q date: %v", rel.Anchor, err)
	}
	switch unit {
	case "d":
		base = base.AddDate(0, 0, n)
	case "w":
		base = base.AddDate(0, 0, 7*n)
	case "m":
		base = base.AddDate(0, n, 0)
	case "y":
		base = base.AddDate(n, 0, 0)
	}
	out.Precision, out.Start, out.End = "day", base.Format(storyDateLayout), base.Format(storyDateLayout)
	return out, nil
}
