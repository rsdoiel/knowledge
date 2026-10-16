package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// A hit says where it is and carries enough to act on (TODO, raised 2026-10-14).

func searchHitKB(t *testing.T) *knowledge.KnowledgeBase {
	t.Helper()
	kb := openTestKB(t)
	p, _ := kb.AddProject("alpha", "")
	_, err := kb.AddRecord(knowledge.Record{RecordID: "0001", ProjectID: p, Scope: "project", Path: "decisions/0001-x.md",
		Title: "Breakfast policy", Date: "2026-10-01", Status: "accepted", Kind: "decision", Checksum: "c1",
		Body: "\n**Context.** We needed a breakfast.\n\n**Decision.** Use toast for breakfast every day.\n"})
	if err != nil {
		t.Fatal(err)
	}
	return kb
}

func runSearchCmd(t *testing.T, kb *knowledge.KnowledgeBase, jsonOut bool, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	if err := cmdSearch(kb, nil, jsonOut, args, &out); err != nil {
		t.Fatalf("search %v: %v", args, err)
	}
	return out.String()
}

func TestSearch_UnscopedHitNamesTheRecordItsStatusAndThePassage(t *testing.T) {
	out := runSearchCmd(t, searchHitKB(t), false, "toast")
	for _, want := range []string{"alpha/DR-0001", "(accepted)", "Decision:", "«toast»"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestSearch_ScopedHitCarriesTheSameThings(t *testing.T) {
	out := runSearchCmd(t, searchHitKB(t), false, "alpha/", "toast")
	for _, want := range []string{"alpha/DR-0001", "(accepted)", "Decision:", "«toast»"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestSearch_JSONCarriesRefStatusPassageAndSection(t *testing.T) {
	kb := searchHitKB(t)
	var scoped struct {
		Results []struct {
			Ref     string `json:"ref"`
			Status  string `json:"status"`
			Passage string `json:"passage"`
			Section string `json:"section"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(runSearchCmd(t, kb, true, "alpha/", "toast")), &scoped); err != nil || len(scoped.Results) != 1 {
		t.Fatalf("scoped JSON: %v", err)
	}
	if r := scoped.Results[0]; r.Ref != "alpha/DR-0001" || r.Status != "accepted" || r.Section != "Decision" || !strings.Contains(r.Passage, "«toast»") {
		t.Errorf("scoped = %+v", r)
	}
	var unscoped []struct{ Ref, Status, Passage, Section string }
	if err := json.Unmarshal([]byte(runSearchCmd(t, kb, true, "toast")), &unscoped); err != nil || len(unscoped) != 1 {
		t.Fatalf("unscoped JSON: %v", err)
	}
	if r := unscoped[0]; r.Ref != "alpha/DR-0001" || r.Status != "accepted" || r.Section != "Decision" {
		t.Errorf("unscoped = %+v", r)
	}
}
