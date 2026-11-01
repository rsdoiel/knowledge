package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// K4 of the draft-then-promote plan (DR-0090): the commands.

func runObservation(t *testing.T, kb *knowledge.KnowledgeBase, jsonOut bool, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := cmdObservation(kb, nil, jsonOut, args, &out)
	return out.String(), err
}

func obsFixture(t *testing.T) *knowledge.KnowledgeBase {
	t.Helper()
	kb, _ := openWorkspaceKB(t)
	if _, err := kb.AddProject("P", "d"); err != nil {
		t.Fatal(err)
	}
	return kb
}

func obsCount(t *testing.T, kb *knowledge.KnowledgeBase) int {
	t.Helper()
	all := 0
	p, _ := kb.ProjectByName("P")
	for _, s := range []string{"reviewed", "drafted"} {
		o, err := kb.ObservationsByStatus(p.ID, s)
		if err != nil {
			t.Fatal(err)
		}
		all += len(o)
	}
	return all
}

func TestObservationAddDraft_WritesADraft(t *testing.T) {
	kb := obsFixture(t)
	out, err := runObservation(t, kb, true, "add", "--project", "P", "--draft", "--by", "harvey qwen2.5:3b", "--confidence", "0.6", "finding", "a", "model", "thought")
	if err != nil {
		t.Fatalf("add --draft: %v", err)
	}
	var got struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.Status != "drafted" || got.ID == 0 {
		t.Fatalf("output %q: %+v, %v", out, got, err)
	}
	o, _ := kb.ObservationByID(got.ID)
	if o.Status != "drafted" || o.GeneratedBy != "harvey qwen2.5:3b" || o.Confidence == nil || *o.Confidence != 0.6 || o.Body != "a model thought" {
		t.Errorf("stored %+v", o)
	}
}

func TestObservationAddDraft_UsageErrorsWriteNothing(t *testing.T) {
	cases := map[string][]string{
		"draft without by":         {"add", "--project", "P", "--draft", "note", "b"},
		"by without draft":         {"add", "--project", "P", "--by", "h", "note", "b"},
		"confidence without draft": {"add", "--project", "P", "--confidence", "0.5", "note", "b"},
		"blank by":                 {"add", "--project", "P", "--draft", "--by", " ", "note", "b"},
		"multi-line by":            {"add", "--project", "P", "--draft", "--by", "a\nb", "note", "b"},
		"confidence above 1":       {"add", "--project", "P", "--draft", "--by", "h", "--confidence", "1.5", "note", "b"},
		"confidence not a number":  {"add", "--project", "P", "--draft", "--by", "h", "--confidence", "abc", "note", "b"},
		"draft with source-doi":    {"add", "--project", "P", "--draft", "--by", "h", "--source-doi", "10.1/x", "note", "b"},
	}
	for name, args := range cases {
		kb := obsFixture(t)
		if _, err := runObservation(t, kb, false, args...); err == nil || !isUsageError(err) {
			t.Errorf("%s: err = %v, want a usage error (exit 2)", name, err)
		}
		if n := obsCount(t, kb); n != 0 {
			t.Errorf("%s: %d rows written", name, n)
		}
	}
}

// harvey's substring fallback reads `observation list`; a draft in a prompt is
// the failure DR-0090 exists to prevent.
func TestObservationList_DefaultOmitsDraftsWhichIsWhatHarveyDependsOn(t *testing.T) {
	kb := obsFixture(t)
	runObservation(t, kb, false, "add", "--project", "P", "note", "trusted words")
	runObservation(t, kb, false, "add", "--project", "P", "--draft", "--by", "h", "note", "drafted words")
	out, err := runObservation(t, kb, false, "list", "--project", "P")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "drafted words") || !strings.Contains(out, "trusted words") {
		t.Errorf("default list = %q", out)
	}
	out, _ = runObservation(t, kb, true, "list", "--project", "P")
	if strings.Contains(out, "drafted words") {
		t.Errorf("default --json list = %q", out)
	}
}

func TestObservationList_StatusFlag(t *testing.T) {
	kb := obsFixture(t)
	runObservation(t, kb, false, "add", "--project", "P", "note", "trusted words")
	runObservation(t, kb, false, "add", "--project", "P", "--draft", "--by", "h", "note", "drafted words")
	for status, want := range map[string]struct{ has, hasNot string }{
		"drafted":  {"drafted words", "trusted words"},
		"reviewed": {"trusted words", "drafted words"},
	} {
		out, err := runObservation(t, kb, false, "list", "--project", "P", "--status", status)
		if err != nil || !strings.Contains(out, want.has) || strings.Contains(out, want.hasNot) {
			t.Errorf("--status %s = %q, %v", status, out, err)
		}
	}
	out, _ := runObservation(t, kb, false, "list", "--project", "P", "--status", "all")
	if !strings.Contains(out, "trusted words") || !strings.Contains(out, "drafted words") {
		t.Errorf("--status all = %q", out)
	}
	if _, err := runObservation(t, kb, false, "list", "--project", "P", "--status", "bogus"); err == nil || !isUsageError(err) {
		t.Errorf("bad status: %v, want usage error", err)
	}
}

func TestObservationList_JSONRowsCarryStatus(t *testing.T) {
	kb := obsFixture(t)
	runObservation(t, kb, false, "add", "--project", "P", "--draft", "--by", "h", "--confidence", "0.25", "note", "d")
	out, _ := runObservation(t, kb, true, "list", "--project", "P", "--status", "drafted")
	var rows []struct {
		Status      string
		GeneratedBy string
		Confidence  *float64
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 1 || rows[0].Status != "drafted" || rows[0].GeneratedBy != "h" || rows[0].Confidence == nil {
		t.Errorf("rows %q: %+v, %v", out, rows, err)
	}
}

func TestObservationShow_ReportsStatusDrafterAndConfidence(t *testing.T) {
	kb := obsFixture(t)
	runObservation(t, kb, false, "add", "--project", "P", "--draft", "--by", "harvey", "--confidence", "0.4", "note", "d")
	out, err := runObservation(t, kb, false, "show", "1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"drafted", "harvey", "0.4"} {
		if !strings.Contains(out, want) {
			t.Errorf("show lacks %q: %q", want, out)
		}
	}
	runObservation(t, kb, false, "add", "--project", "P", "note", "r")
	out, _ = runObservation(t, kb, false, "show", "2")
	if strings.Contains(out, "drafted") || strings.Contains(out, "by ") {
		t.Errorf("show of a reviewed row mentions drafting: %q", out)
	}
}

func TestObservationUpdate_DraftStaysADraftWithItsDrafter(t *testing.T) {
	kb := obsFixture(t)
	runObservation(t, kb, false, "add", "--project", "P", "--draft", "--by", "harvey", "--confidence", "0.9", "note", "first")
	if _, err := runObservation(t, kb, false, "update", "1", "second"); err != nil {
		t.Fatal(err)
	}
	o, _ := kb.ObservationByID(2)
	if o.Status != "drafted" || o.GeneratedBy != "harvey" || o.Body != "second" {
		t.Errorf("update of a draft stored %+v", o)
	}
	if n := ftsRows(t, kb); n != 0 {
		t.Errorf("update of a draft indexed %d rows", n)
	}
}

func TestObservationUpdate_ReviewedStaysReviewed(t *testing.T) {
	kb := obsFixture(t)
	runObservation(t, kb, false, "add", "--project", "P", "note", "first")
	runObservation(t, kb, false, "update", "1", "second")
	o, _ := kb.ObservationByID(2)
	if o.Status != "reviewed" || o.GeneratedBy != "" {
		t.Errorf("update of a reviewed row stored %+v", o)
	}
}

func ftsRows(t *testing.T, kb *knowledge.KnowledgeBase) int {
	t.Helper()
	hits, err := kb.Search("first")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, h := range hits {
		if h.SourceType == "observation" {
			n++
		}
	}
	return n
}

func TestObservationReviewList(t *testing.T) {
	kb := obsFixture(t)
	kb.AddProject("Q", "d")
	runObservation(t, kb, false, "add", "--project", "P", "note", "trusted")
	runObservation(t, kb, false, "add", "--project", "P", "--draft", "--by", "h", "note", "draft in P")
	runObservation(t, kb, false, "add", "--project", "Q", "--draft", "--by", "h", "note", "draft in Q")
	out, err := runObservation(t, kb, false, "review", "list")
	if err != nil || !strings.Contains(out, "draft in P") || !strings.Contains(out, "draft in Q") || strings.Contains(out, "trusted") {
		t.Errorf("review list = %q, %v", out, err)
	}
	out, _ = runObservation(t, kb, false, "review", "list", "--project", "Q")
	if strings.Contains(out, "draft in P") || !strings.Contains(out, "draft in Q") {
		t.Errorf("review list --project Q = %q", out)
	}
	if _, err := runObservation(t, kb, false, "review", "list", "--project", "nosuch"); err == nil || exitCodeFor(err).Code != 1 {
		t.Errorf("unknown project: %v, want exit 1", err)
	}
}

func TestObservationReviewList_ASupersededDraftIsNotQueued(t *testing.T) {
	kb := obsFixture(t)
	runObservation(t, kb, false, "add", "--project", "P", "--draft", "--by", "h", "note", "old wording")
	runObservation(t, kb, false, "update", "1", "new wording")
	out, _ := runObservation(t, kb, false, "review", "list")
	if strings.Contains(out, "old wording") || !strings.Contains(out, "new wording") {
		t.Errorf("review list = %q, want only the newer draft", out)
	}
}

func TestObservationReviewPromote_NeedsATerminal(t *testing.T) {
	kb := obsFixture(t)
	runObservation(t, kb, false, "add", "--project", "P", "--draft", "--by", "h", "note", "promote me")
	noTerminal(t)
	out, err := runObservation(t, kb, false, "review", "promote", "1")
	if err == nil || !isUsageError(err) || !strings.Contains(err.Error(), "person") ||
		!strings.Contains(err.Error(), "kb observation review promote 1") {
		t.Fatalf("promote without a terminal = %q, %v", out, err)
	}
	if o, _ := kb.ObservationByID(1); o.Status != "drafted" {
		t.Error("promoted despite the refusal")
	}
	// The check comes before the lookup: an unknown id is still 2.
	if _, err := runObservation(t, kb, false, "review", "promote", "999"); err == nil || !isUsageError(err) {
		t.Errorf("unknown id without a terminal = %v, want usage error", err)
	}
}

func TestObservationReviewPromote_AtATerminal(t *testing.T) {
	kb := obsFixture(t)
	runObservation(t, kb, false, "add", "--project", "P", "--draft", "--by", "h", "note", "promote me please")
	runObservation(t, kb, false, "add", "--project", "P", "note", "already trusted")
	out, err := runObservation(t, kb, true, "review", "promote", "1")
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	var got struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	if json.Unmarshal([]byte(out), &got) != nil || got.Status != "reviewed" || got.ID != 1 {
		t.Errorf("output %q", out)
	}
	if hits, _ := kb.Search("promote me please"); len(hits) == 0 {
		t.Error("the promoted observation is not searchable")
	}
	if _, err := runObservation(t, kb, false, "review", "promote", "2"); err == nil || exitCodeFor(err).Code != 1 {
		t.Errorf("promote of a reviewed row: %v, want exit 1", err)
	}
	if _, err := runObservation(t, kb, false, "review", "promote", "999"); err == nil || exitCodeFor(err).Code != 1 {
		t.Errorf("promote of an unknown id: %v, want exit 1", err)
	}
	for _, bad := range [][]string{{"review", "promote"}, {"review", "promote", "x"}, {"review", "promote", "1", "2"}, {"review", "bogus"}, {"review"}} {
		if _, err := runObservation(t, kb, false, bad...); err == nil || !isUsageError(err) {
			t.Errorf("%v: %v, want usage error", bad, err)
		}
	}
}

func TestObservationDelete_RemovesADraftWithoutATerminal(t *testing.T) {
	kb := obsFixture(t)
	runObservation(t, kb, false, "add", "--project", "P", "--draft", "--by", "h", "note", "reject me")
	noTerminal(t)
	if _, err := runObservation(t, kb, false, "delete", "1"); err != nil {
		t.Fatalf("delete of a draft: %v", err)
	}
	if _, err := kb.ObservationByID(1); err == nil {
		t.Error("the draft is still there")
	}
}

func TestObservationList_SupersededDraftIsNotListedAsDrafted(t *testing.T) {
	kb := obsFixture(t)
	runObservation(t, kb, false, "add", "--project", "P", "--draft", "--by", "h", "note", "old wording")
	runObservation(t, kb, false, "update", "1", "new wording")
	out, _ := runObservation(t, kb, false, "list", "--project", "P", "--status", "drafted")
	if strings.Contains(out, "old wording") || !strings.Contains(out, "new wording") {
		t.Errorf("list --status drafted = %q", out)
	}
}
