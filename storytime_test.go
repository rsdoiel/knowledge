package knowledge

import "testing"

// ─── the story_time grammar (DR-0079 item 2) ─────────────────────────────────

func TestParseStoryTime(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want StoryTime
	}{
		{"1969-10-29", StoryTime{Precision: "day", Start: "1969-10-29", End: "1969-10-29"}},
		{"1969-10", StoryTime{Precision: "month", Start: "1969-10-01", End: "1969-10-31"}},
		{"1969-02", StoryTime{Precision: "month", Start: "1969-02-01", End: "1969-02-28"}},
		{"1969", StoryTime{Precision: "year", Start: "1969-01-01", End: "1969-12-31"}},
		{" circa 1890 ", StoryTime{Precision: "circa", Start: "1890-01-01", End: "1890-12-31"}},
		{"ordinal: 12", StoryTime{Precision: "ordinal", Start: "12", End: "12"}},
		{"ordinal:-3", StoryTime{Precision: "ordinal", Start: "-3", End: "-3"}},
		{"after: e12", StoryTime{Relation: "after", Anchor: "e12"}},
		{"after: e12 +3d", StoryTime{Relation: "after", Anchor: "e12", Offset: "+3d"}},
		{"after: e12 3d", StoryTime{Relation: "after", Anchor: "e12", Offset: "+3d"}},
		{"before: the-war", StoryTime{Relation: "before", Anchor: "the-war"}},
		{"before: the-war 2y", StoryTime{Relation: "before", Anchor: "the-war", Offset: "+2y"}},
		{"after: 12A +4", StoryTime{Relation: "after", Anchor: "12A", Offset: "+4"}},
		{"", StoryTime{}},
	} {
		got, err := ParseStoryTime(tc.in)
		if err != nil {
			t.Errorf("ParseStoryTime(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseStoryTime(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestParseStoryTime_Refuses(t *testing.T) {
	for _, in := range []string{
		"next tuesday", "1969-13-01", "1969-02-30", "circa", "circa soon", "ordinal: x", "ordinal:",
		"after:", "after: ", "after: e12 -3d", "after: e12 3 days", "sometime: e12", "69",
	} {
		if _, err := ParseStoryTime(in); err == nil {
			t.Errorf("ParseStoryTime(%q) succeeded, want an error", in)
		}
	}
}

func TestStoryTime_HasInterval(t *testing.T) {
	if (StoryTime{}).HasInterval() || (StoryTime{Relation: "after", Anchor: "x"}).HasInterval() {
		t.Error("an empty time and an edge have no interval")
	}
	if !(StoryTime{Precision: "day", Start: "1969-10-29", End: "1969-10-29"}).HasInterval() {
		t.Error("a day has an interval")
	}
}

func TestResolveStoryTime(t *testing.T) {
	day := StoryTime{Precision: "day", Start: "1969-10-29", End: "1969-10-29"}
	month := StoryTime{Precision: "month", Start: "1969-10-01", End: "1969-10-31"}
	ord := StoryTime{Precision: "ordinal", Start: "12", End: "12"}
	for _, tc := range []struct {
		name   string
		rel    StoryTime
		anchor StoryTime
		want   StoryTime
	}{
		{"after a day", StoryTime{Relation: "after", Offset: "+3d"}, day, StoryTime{Precision: "day", Start: "1969-11-01", End: "1969-11-01"}},
		{"before a day", StoryTime{Relation: "before", Offset: "+3d"}, day, StoryTime{Precision: "day", Start: "1969-10-26", End: "1969-10-26"}},
		{"after a month's end", StoryTime{Relation: "after", Offset: "+1w"}, month, StoryTime{Precision: "day", Start: "1969-11-07", End: "1969-11-07"}},
		{"before a month's start", StoryTime{Relation: "before", Offset: "+1d"}, month, StoryTime{Precision: "day", Start: "1969-09-30", End: "1969-09-30"}},
		{"months", StoryTime{Relation: "after", Offset: "+2m"}, day, StoryTime{Precision: "day", Start: "1969-12-29", End: "1969-12-29"}},
		{"years", StoryTime{Relation: "before", Offset: "+2y"}, day, StoryTime{Precision: "day", Start: "1967-10-29", End: "1967-10-29"}},
		{"ordinal after", StoryTime{Relation: "after", Offset: "+3"}, ord, StoryTime{Precision: "ordinal", Start: "15", End: "15"}},
		{"ordinal before", StoryTime{Relation: "before", Offset: "+20"}, ord, StoryTime{Precision: "ordinal", Start: "-8", End: "-8"}},
	} {
		got, err := ResolveStoryTime(tc.rel, tc.anchor)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if got.Precision != tc.want.Precision || got.Start != tc.want.Start || got.End != tc.want.End {
			t.Errorf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestResolveStoryTime_Refuses(t *testing.T) {
	day := StoryTime{Precision: "day", Start: "1969-10-29", End: "1969-10-29"}
	ord := StoryTime{Precision: "ordinal", Start: "12", End: "12"}
	edge := StoryTime{Relation: "after", Anchor: "x"}
	for name, c := range map[string]struct{ rel, anchor StoryTime }{
		"no offset":               {StoryTime{Relation: "after"}, day},
		"anchor without interval": {StoryTime{Relation: "after", Offset: "+1d"}, edge},
		"day unit on ordinal":     {StoryTime{Relation: "after", Offset: "+3d"}, ord},
		"bare number on dates":    {StoryTime{Relation: "after", Offset: "+3"}, day},
	} {
		if _, err := ResolveStoryTime(c.rel, c.anchor); err == nil {
			t.Errorf("%s: succeeded, want an error", name)
		}
	}
}
