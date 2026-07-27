package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestCmdSource_AddThenList(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdSource(kb, nil, false, []string{"add", "A Paper", "--doi", "10.1/x"}, &out); err != nil {
		t.Fatalf("source add: %v", err)
	}
	if !strings.Contains(out.String(), "added") {
		t.Errorf("add output = %q, want confirmation text", out.String())
	}

	out.Reset()
	if err := cmdSource(kb, nil, false, []string{"list"}, &out); err != nil {
		t.Fatalf("source list: %v", err)
	}
	if !strings.Contains(out.String(), "A Paper") {
		t.Errorf("list output = %q, want it to mention the source", out.String())
	}
}

func TestCmdSource_AddRequiresTitle(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdSource(kb, nil, false, []string{"add"}, &out); err == nil {
		t.Error("expected an error when TITLE is missing")
	}
}

func TestCmdSource_ShowByID(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	cmdSource(kb, nil, false, []string{"add", "A Paper", "--url", "https://example.org"}, &out)
	out.Reset()
	if err := cmdSource(kb, nil, false, []string{"show", "1"}, &out); err != nil {
		t.Fatalf("source show: %v", err)
	}
	if !strings.Contains(out.String(), "A Paper") {
		t.Errorf("show output = %q, want it to mention the title", out.String())
	}
}

func TestCmdSource_ShowNotFound(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdSource(kb, nil, false, []string{"show", "999"}, &out); err == nil {
		t.Error("expected an error for a nonexistent source")
	}
}

func TestCmdSource_RemoveUnlinkedSucceeds(t *testing.T) {
	kb := openTestKB(t)
	id, err := kb.AddSource(sourceStub("A Paper"))
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	var out bytes.Buffer
	if err := cmdSource(kb, nil, false, []string{"remove", fmt.Sprint(id)}, &out); err != nil {
		t.Fatalf("source remove: %v", err)
	}
	if _, err := kb.ShowSource(id); err == nil {
		t.Error("expected the source to be gone after remove")
	}
}

func TestCmdSource_RemoveLinkedFails(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("proj", "")
	oid, _ := kb.AddObservation(pid, "note", "body")
	sid, err := kb.AddSource(sourceStub("A Paper"))
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if err := kb.LinkObservationSource(oid, sid, "cited"); err != nil {
		t.Fatalf("LinkObservationSource: %v", err)
	}
	var out bytes.Buffer
	if err := cmdSource(kb, nil, false, []string{"remove", fmt.Sprint(sid)}, &out); err == nil {
		t.Error("expected an error removing a linked source")
	}
}

func TestCmdSource_Retract(t *testing.T) {
	kb := openTestKB(t)
	id, _ := kb.AddSource(sourceStub("A Paper"))
	var out bytes.Buffer
	if err := cmdSource(kb, nil, false, []string{"retract", fmt.Sprint(id), "found", "to", "be", "fraudulent"}, &out); err != nil {
		t.Fatalf("source retract: %v", err)
	}
	s, err := kb.ShowSource(id)
	if err != nil {
		t.Fatalf("ShowSource: %v", err)
	}
	if !s.Retracted || s.RetractionNote != "found to be fraudulent" {
		t.Errorf("source = %+v, want Retracted=true and the note", s)
	}
}

func TestCmdSource_Link(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("proj", "")
	oid, _ := kb.AddObservation(pid, "note", "body")
	sid, _ := kb.AddSource(sourceStub("A Paper"))
	var out bytes.Buffer
	if err := cmdSource(kb, nil, false, []string{"link", fmt.Sprint(oid), fmt.Sprint(sid)}, &out); err != nil {
		t.Fatalf("source link: %v", err)
	}
	sources, err := kb.ObservationSources(oid)
	if err != nil {
		t.Fatalf("ObservationSources: %v", err)
	}
	if len(sources) != 1 {
		t.Errorf("sources = %+v, want exactly one linked source", sources)
	}
}

func TestCmdSource_CheckRetractionsNoSources(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdSource(kb, nil, false, []string{"check-retractions"}, &out); err != nil {
		t.Fatalf("source check-retractions: %v", err)
	}
	if !strings.Contains(out.String(), "Checked 0") {
		t.Errorf("output = %q, want it to report checking 0 sources", out.String())
	}
}

func TestCmdSource_RemoveJSON(t *testing.T) {
	kb := openTestKB(t)
	id, _ := kb.AddSource(sourceStub("A Paper"))
	var out bytes.Buffer
	if err := cmdSource(kb, nil, true, []string{"remove", fmt.Sprint(id)}, &out); err != nil {
		t.Fatalf("source remove: %v", err)
	}
	assertValidJSON(t, out.Bytes())
}

func TestCmdSource_RetractJSON(t *testing.T) {
	kb := openTestKB(t)
	id, _ := kb.AddSource(sourceStub("A Paper"))
	var out bytes.Buffer
	if err := cmdSource(kb, nil, true, []string{"retract", fmt.Sprint(id), "reason"}, &out); err != nil {
		t.Fatalf("source retract: %v", err)
	}
	assertValidJSON(t, out.Bytes())
}

func TestCmdSource_LinkJSON(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("proj", "")
	oid, _ := kb.AddObservation(pid, "note", "body")
	sid, _ := kb.AddSource(sourceStub("A Paper"))
	var out bytes.Buffer
	if err := cmdSource(kb, nil, true, []string{"link", fmt.Sprint(oid), fmt.Sprint(sid)}, &out); err != nil {
		t.Fatalf("source link: %v", err)
	}
	assertValidJSON(t, out.Bytes())
}

func TestCmdSource_CheckRetractionsJSON(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdSource(kb, nil, true, []string{"check-retractions"}, &out); err != nil {
		t.Fatalf("source check-retractions: %v", err)
	}
	assertValidJSON(t, out.Bytes())
}

func TestCmdSource_UnknownSubcommand(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdSource(kb, nil, false, []string{"bogus"}, &out); err == nil {
		t.Error("expected an error for an unknown source subcommand")
	}
}
