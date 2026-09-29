package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// `kb concept show` and `kb concept recall` (DR-0051).

func runConceptCmd(t *testing.T, kb *knowledge.KnowledgeBase, jsonOut bool, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := cmdConcept(kb, nil, jsonOut, args, &out)
	return out.String(), err
}

// showFixture links "toast" to 12 observations in project alpha, and a record.
func showFixture(t *testing.T) *knowledge.KnowledgeBase {
	t.Helper()
	kb := openTestKB(t)
	cid, _ := kb.AddConcept("toast", "browned bread")
	pid, _ := kb.AddProject("alpha", "")
	for i := 1; i <= 12; i++ {
		oid, err := kb.AddObservation(pid, "note", fmt.Sprintf("obs %02d", i))
		if err != nil {
			t.Fatal(err)
		}
		if err := kb.LinkObservationConcept(oid, cid); err != nil {
			t.Fatal(err)
		}
	}
	rid, err := kb.AddRecord(knowledge.Record{RecordID: "0001", ProjectID: pid, Scope: "project",
		Path: "decisions/0001-x.md", Title: "use toast", Date: "2026-09-01", Status: "accepted",
		Kind: "decision", Body: "b", Checksum: "c"})
	if err != nil {
		t.Fatal(err)
	}
	if err := kb.LinkRecordConcept(rid, cid); err != nil {
		t.Fatal(err)
	}
	return kb
}

func TestConceptShow_HumanOutput(t *testing.T) {
	kb := showFixture(t)
	out, err := runConceptCmd(t, kb, false, "show", "toast")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	for _, want := range []string{"toast", "browned bread", "observations (12", "records (1", "use toast", "alpha"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "obs ") != 10 {
		t.Errorf("default limit should list 10 observations:\n%s", out)
	}
}

func TestConceptShow_LimitZeroIsCountsOnly(t *testing.T) {
	kb := showFixture(t)
	out, err := runConceptCmd(t, kb, false, "show", "toast", "--limit", "0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "observations (12") || strings.Contains(out, "obs 12") {
		t.Errorf("--limit 0 output:\n%s", out)
	}
}

func TestConceptShow_JSONIsOneObject(t *testing.T) {
	kb := showFixture(t)
	out, err := runConceptCmd(t, kb, true, "show", "toast", "--limit", "2")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Counts      map[string]int `json:"counts"`
		Obs         []struct {
			ID      int64  `json:"id"`
			Excerpt string `json:"excerpt"`
		} `json:"observations"`
		Records []struct {
			Title string `json:"title"`
		} `json:"records"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("not one JSON object: %v\n%s", err, out)
	}
	if v.Name != "toast" || v.Counts["observations"] != 12 || v.Counts["records"] != 1 {
		t.Errorf("decoded = %+v", v)
	}
	if len(v.Obs) != 2 || v.Obs[0].Excerpt != "obs 12" || len(v.Records) != 1 {
		t.Errorf("items = %+v", v)
	}
}

func TestConceptShow_Errors(t *testing.T) {
	kb := showFixture(t)
	if _, err := kb.AddConcept("Jam", ""); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		want exitClass
		text string
	}{
		{"unknown name", []string{"show", "nosuch"}, classNegative, ""},
		{"case variant hint", []string{"show", "jam"}, classNegative, `"Jam"`},
		{"missing name", []string{"show"}, classUsage, ""},
		{"surplus argument", []string{"show", "toast", "extra"}, classUsage, ""},
		{"negative limit", []string{"show", "toast", "--limit", "-1"}, classUsage, ""},
		{"non-numeric limit", []string{"show", "toast", "--limit", "x"}, classUsage, ""},
		{"unknown flag", []string{"show", "toast", "--bogus"}, classUsage, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runConceptCmd(t, kb, false, tc.args...)
			if err == nil {
				t.Fatal("want an error")
			}
			if got := exitCodeFor(err); got != tc.want {
				t.Errorf("class = %v, want %v (%v)", got, tc.want, err)
			}
			if tc.text != "" && !strings.Contains(err.Error(), tc.text) {
				t.Errorf("error %q lacks %q", err, tc.text)
			}
		})
	}
}

func TestConceptShow_DashLeadingNameAfterDoubleDash(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("-x", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := runConceptCmd(t, kb, false, "show", "--", "-x"); err != nil {
		t.Errorf("show -- -x: %v", err)
	}
}

// ─── recall ──────────────────────────────────────────────────────────────────

func recallFixture(t *testing.T) *knowledge.KnowledgeBase {
	t.Helper()
	kb := openTestKB(t)
	toast, _ := kb.AddConcept("toast", "")
	jam, _ := kb.AddConcept("jam", "")
	a, _ := kb.AddProject("alpha", "")
	b, _ := kb.AddProject("beta", "")
	o1, _ := kb.AddObservation(a, "note", "toast and jam together")
	o2, _ := kb.AddObservation(b, "note", "just jam here")
	for _, c := range []int64{toast, jam} {
		kb.LinkObservationConcept(o1, c)
	}
	kb.LinkObservationConcept(o2, jam)
	return kb
}

func withStdin(t *testing.T, s string) {
	t.Helper()
	old := recallStdin
	recallStdin = strings.NewReader(s)
	t.Cleanup(func() { recallStdin = old })
}

func TestConceptRecall_TextArguments(t *testing.T) {
	kb := recallFixture(t)
	out, err := runConceptCmd(t, kb, false, "recall", "how", "do", "jam", "and", "toast", "go")
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !strings.Contains(out, "matched concepts:") || !strings.Contains(out, "toast") ||
		!strings.Contains(out, "observation") || !strings.Contains(out, "toast and jam together") {
		t.Errorf("output:\n%s", out)
	}
}

func TestConceptRecall_Stdin(t *testing.T) {
	kb := recallFixture(t)
	withStdin(t, "some text about toast\n")
	out, err := runConceptCmd(t, kb, false, "recall", "-")
	if err != nil {
		t.Fatalf("recall -: %v", err)
	}
	if !strings.Contains(out, "toast and jam together") {
		t.Errorf("output:\n%s", out)
	}
}

func TestConceptRecall_ConceptFlagSkipsMatching(t *testing.T) {
	kb := recallFixture(t)
	out, err := runConceptCmd(t, kb, false, "recall", "--concept", "jam,Toast")
	if err != nil {
		t.Fatalf("recall --concept: %v", err)
	}
	if !strings.Contains(out, "just jam here") || !strings.Contains(out, "toast and jam together") {
		t.Errorf("output:\n%s", out)
	}
}

func TestConceptRecall_TextAndConceptAreUnioned(t *testing.T) {
	kb := recallFixture(t)
	out, err := runConceptCmd(t, kb, false, "recall", "only", "toast", "--concept", "jam")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "matched concepts: toast, jam") && !strings.Contains(out, "matched concepts: jam, toast") {
		t.Errorf("want both concepts matched:\n%s", out)
	}
}

func TestConceptRecall_ProjectFilter(t *testing.T) {
	kb := recallFixture(t)
	out, err := runConceptCmd(t, kb, false, "recall", "--concept", "jam", "--project", "beta")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "toast and jam together") || !strings.Contains(out, "just jam here") {
		t.Errorf("output:\n%s", out)
	}
}

func TestConceptRecall_LimitDefaultsTo10AndIsApplied(t *testing.T) {
	kb := recallFixture(t)
	out, err := runConceptCmd(t, kb, false, "recall", "--concept", "jam", "--limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "observation #") != 1 {
		t.Errorf("--limit 1 output:\n%s", out)
	}
}

func TestConceptRecall_JSONShape(t *testing.T) {
	kb := recallFixture(t)
	out, err := runConceptCmd(t, kb, true, "recall", "toast")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Matched []string `json:"matched"`
		Hits    []struct {
			Kind     string   `json:"kind"`
			ID       int64    `json:"id"`
			Project  string   `json:"project"`
			Concepts []string `json:"concepts"`
			Excerpt  string   `json:"excerpt"`
		} `json:"hits"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("not one JSON object: %v\n%s", err, out)
	}
	if len(v.Matched) != 1 || v.Matched[0] != "toast" || len(v.Hits) != 1 ||
		v.Hits[0].Kind != "observation" || v.Hits[0].Project != "alpha" || len(v.Hits[0].Concepts) != 1 {
		t.Errorf("decoded = %+v", v)
	}
}

func TestConceptRecall_Errors(t *testing.T) {
	kb := recallFixture(t)
	withStdin(t, "   \n")
	for _, tc := range []struct {
		name string
		args []string
		want exitClass
	}{
		{"no concept matched", []string{"recall", "nothing", "relevant"}, classNegative},
		{"unknown --concept names only", []string{"recall", "--concept", "nosuch"}, classNegative},
		{"unknown project", []string{"recall", "toast", "--project", "nosuch"}, classNegative},
		{"no input at all", []string{"recall"}, classUsage},
		{"blank stdin", []string{"recall", "-"}, classUsage},
		{"negative limit", []string{"recall", "toast", "--limit", "-1"}, classUsage},
		{"non-numeric limit", []string{"recall", "toast", "--limit", "x"}, classUsage},
		{"unknown flag", []string{"recall", "toast", "--bogus"}, classUsage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runConceptCmd(t, kb, false, tc.args...)
			if err == nil {
				t.Fatal("want an error")
			}
			if got := exitCodeFor(err); got != tc.want {
				t.Errorf("class = %v, want %v (%v)", got, tc.want, err)
			}
		})
	}
}

var _ io.Reader = recallStdin
