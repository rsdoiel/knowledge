package knowledge

import "testing"

// DR-0096: a design is DS-NNNN and a plan PL-NNNN, beside DR-NNNN. A bare
// number and DR- stay decisions, so every existing reference reads as before.
func TestParseRef_DesignAndPlanPrefixes(t *testing.T) {
	for _, tc := range []struct {
		in        string
		want      Ref
		wantShown string
	}{
		{"harvey/DR-0004", Ref{"harvey", "0004"}, "harvey/DR-0004"},
		{"harvey/4", Ref{"harvey", "0004"}, "harvey/DR-0004"},
		{"harvey/DS-0001", Ref{"harvey", "DS-0001"}, "harvey/DS-0001"},
		{"harvey/ds-1", Ref{"harvey", "DS-0001"}, "harvey/DS-0001"},
		{"harvey/PL-0012", Ref{"harvey", "PL-0012"}, "harvey/PL-0012"},
		{"PL-3", Ref{"", "PL-0003"}, "PL-0003"},
	} {
		got, err := ParseRef(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseRef(%q) = %+v, %v; want %+v", tc.in, got, err, tc.want)
			continue
		}
		if s := got.String(); s != tc.wantShown {
			t.Errorf("ParseRef(%q).String() = %q, want %q", tc.in, s, tc.wantShown)
		}
	}
	for _, bad := range []string{"harvey/XX-0001", "harvey/DS-", "harvey/DS-12x", "harvey/DSX-0001"} {
		if _, err := ParseRef(bad); err == nil {
			t.Errorf("ParseRef(%q) succeeded, want an error", bad)
		}
	}
}

func TestRecordLabel(t *testing.T) {
	for in, want := range map[string]string{"0096": "DR-0096", "DS-0001": "DS-0001", "PL-0002": "PL-0002"} {
		if got := RecordLabel(in); got != want {
			t.Errorf("RecordLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
