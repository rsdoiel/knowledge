package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

func TestCmdObservation_AddRequiresProject(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	err := cmdObservation(kb, nil, false, []string{"add", "note", "some body"}, &out)
	if err == nil {
		t.Error("expected an error when --project is missing")
	}
}

func TestCmdObservation_AddThenList(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("proj", ""); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdObservation(kb, nil, false, []string{"add", "--project", "proj", "finding", "a", "real", "finding"}, &out); err != nil {
		t.Fatalf("observation add: %v", err)
	}
	if !strings.Contains(out.String(), "recorded") {
		t.Errorf("add output = %q, want confirmation text", out.String())
	}

	out.Reset()
	if err := cmdObservation(kb, nil, false, []string{"list", "--project", "proj"}, &out); err != nil {
		t.Fatalf("observation list: %v", err)
	}
	if !strings.Contains(out.String(), "a real finding") {
		t.Errorf("list output = %q, want it to contain the observation body", out.String())
	}
}

func TestCmdObservation_AddWithSourceDOI(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("proj", "")
	var out bytes.Buffer
	if err := cmdObservation(kb, nil, false, []string{"add", "--project", "proj", "--source-doi", "10.1/x", "note", "from a paper"}, &out); err != nil {
		t.Fatalf("observation add: %v", err)
	}
	obs, err := kb.Observations(pid)
	if err != nil {
		t.Fatalf("Observations: %v", err)
	}
	if len(obs) != 1 || obs[0].SourceDOI != "10.1/x" {
		t.Errorf("obs = %+v, want one observation with SourceDOI=10.1/x", obs)
	}
}

func TestCmdObservation_ShowByID(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("proj", "")
	id, _ := kb.AddObservation(pid, "note", "specific body")
	var out bytes.Buffer
	if err := cmdObservation(kb, nil, false, []string{"show", fmt.Sprint(id)}, &out); err != nil {
		t.Fatalf("observation show: %v", err)
	}
	if !strings.Contains(out.String(), "specific body") {
		t.Errorf("show output = %q, want it to contain the body", out.String())
	}
}

func TestCmdObservation_ShowJSON(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("proj", "")
	id, _ := kb.AddObservation(pid, "note", "specific body")
	var out bytes.Buffer
	if err := cmdObservation(kb, nil, true, []string{"show", fmt.Sprint(id)}, &out); err != nil {
		t.Fatalf("observation show: %v", err)
	}
	var got struct {
		Body string `json:"Body"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v (%q)", err, out.String())
	}
	if got.Body != "specific body" {
		t.Errorf("got.Body = %q, want %q", got.Body, "specific body")
	}
}

func TestCmdObservation_ShowNotFound(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdObservation(kb, nil, false, []string{"show", "999999"}, &out); err == nil {
		t.Error("expected an error for a nonexistent observation id")
	}
}

// ─── update (DR-0023) ────────────────────────────────────────────────────────

func TestCmdObservation_UpdateInsertsNewObservationAndSupersedes(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("proj", "")
	old, _ := kb.AddObservation(pid, "finding", "original wording")

	var out bytes.Buffer
	if err := cmdObservation(kb, nil, false, []string{"update", fmt.Sprint(old), "corrected", "wording"}, &out); err != nil {
		t.Fatalf("observation update: %v", err)
	}

	obs, err := kb.Observations(pid)
	if err != nil {
		t.Fatalf("Observations: %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("got %d observations, want 2 (old one untouched, new one added)", len(obs))
	}
	oldNow, err := kb.ObservationByID(old)
	if err != nil {
		t.Fatalf("ObservationByID(old): %v", err)
	}
	if oldNow.Body != "original wording" {
		t.Errorf("old observation body = %q, want it untouched", oldNow.Body)
	}

	var newer *knowledge.Observation
	for i := range obs {
		if obs[i].ID != old {
			newer = &obs[i]
		}
	}
	if newer == nil || newer.Body != "corrected wording" {
		t.Fatalf("new observation = %+v, want body %q", newer, "corrected wording")
	}
	if newer.Kind != "finding" {
		t.Errorf("new observation kind = %q, want it inherited from the old one (%q)", newer.Kind, "finding")
	}

	rels, err := kb.ObservationRelationsFor(newer.ID)
	if err != nil {
		t.Fatalf("ObservationRelationsFor: %v", err)
	}
	if len(rels) != 1 || rels[0].Relationship != "supersedes" || rels[0].ObservationID != old {
		t.Errorf("relations for new observation = %+v, want one supersedes edge to %d", rels, old)
	}
}

func TestCmdObservation_UpdateRequiresIDAndBody(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("proj", "")
	id, _ := kb.AddObservation(pid, "note", "a")

	var out bytes.Buffer
	if err := cmdObservation(kb, nil, false, []string{"update", fmt.Sprint(id)}, &out); err == nil {
		t.Error("expected an error when BODY is missing")
	}
	if err := cmdObservation(kb, nil, false, nil, &out); err == nil {
		t.Error("expected an error for observation update with no arguments")
	}
}

func TestCmdObservation_UpdateUnknownObservationFails(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdObservation(kb, nil, false, []string{"update", "999999", "a", "new", "body"}, &out); err == nil {
		t.Error("expected an error for a nonexistent observation id")
	}
}

func TestCmdObservation_ShowResolvesSupersessionRelations(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("proj", "")
	old, _ := kb.AddObservation(pid, "note", "original")

	var updateOut bytes.Buffer
	if err := cmdObservation(kb, nil, true, []string{"update", fmt.Sprint(old), "corrected"}, &updateOut); err != nil {
		t.Fatalf("observation update: %v", err)
	}
	var updated struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(updateOut.Bytes(), &updated); err != nil {
		t.Fatalf("decoding update output: %v (%q)", err, updateOut.String())
	}

	var oldOut bytes.Buffer
	if err := cmdObservation(kb, nil, false, []string{"show", fmt.Sprint(old)}, &oldOut); err != nil {
		t.Fatalf("observation show (old): %v", err)
	}
	if !strings.Contains(oldOut.String(), fmt.Sprint(updated.ID)) {
		t.Errorf("show(old) output = %q, want it to name the superseding observation %d", oldOut.String(), updated.ID)
	}

	var newOut bytes.Buffer
	if err := cmdObservation(kb, nil, false, []string{"show", fmt.Sprint(updated.ID)}, &newOut); err != nil {
		t.Fatalf("observation show (new): %v", err)
	}
	if !strings.Contains(newOut.String(), fmt.Sprint(old)) {
		t.Errorf("show(new) output = %q, want it to name the superseded observation %d", newOut.String(), old)
	}
}

func TestCmdObservation_Sources(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("proj", "")
	oid, _ := kb.AddObservation(pid, "note", "body")
	sid, err := kb.AddSource(knowledge.Source{Title: "A Paper"})
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if err := kb.LinkObservationSource(oid, sid, "cited"); err != nil {
		t.Fatalf("LinkObservationSource: %v", err)
	}
	var out bytes.Buffer
	if err := cmdObservation(kb, nil, false, []string{"sources", fmt.Sprint(oid)}, &out); err != nil {
		t.Fatalf("observation sources: %v", err)
	}
	if !strings.Contains(out.String(), "A Paper") {
		t.Errorf("sources output = %q, want it to mention the linked source", out.String())
	}
}
