package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCmdConcept_AddThenList(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"add", "WAL", "write-ahead", "logging"}, &out); err != nil {
		t.Fatalf("concept add: %v", err)
	}
	if !strings.Contains(out.String(), "WAL") {
		t.Errorf("add output = %q, want it to mention WAL", out.String())
	}

	out.Reset()
	if err := cmdConcept(kb, nil, false, []string{"list"}, &out); err != nil {
		t.Fatalf("concept list: %v", err)
	}
	if !strings.Contains(out.String(), "WAL") {
		t.Errorf("list output = %q, want it to mention WAL", out.String())
	}
}

func TestCmdConcept_AddWithIdentifier(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	err := cmdConcept(kb, nil, false, []string{"add", "--identifier-type", "orcid", "--identifier-value", "0000-0003-0900-6903", "Jane Doe"}, &out)
	if err != nil {
		t.Fatalf("concept add: %v", err)
	}
	concepts, err := kb.Concepts()
	if err != nil {
		t.Fatalf("Concepts: %v", err)
	}
	if len(concepts) != 1 || concepts[0].IdentifierType != "orcid" {
		t.Errorf("concepts = %+v, want one concept with IdentifierType=orcid", concepts)
	}
}

func TestCmdConcept_AddRequiresName(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"add"}, &out); err == nil {
		t.Error("expected an error when NAME is missing")
	}
}

// ─── rename (DR-0024) ────────────────────────────────────────────────────────

func TestCmdConcept_Rename(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("oldname", "a concept"); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"rename", "oldname", "newname"}, &out); err != nil {
		t.Fatalf("concept rename: %v", err)
	}
	concepts, err := kb.Concepts()
	if err != nil {
		t.Fatalf("Concepts: %v", err)
	}
	if len(concepts) != 1 || concepts[0].Name != "newname" {
		t.Errorf("concepts = %+v, want one concept named newname", concepts)
	}
}

func TestCmdConcept_RenameJSON(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("oldname", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, true, []string{"rename", "oldname", "newname"}, &out); err != nil {
		t.Fatalf("concept rename: %v", err)
	}
	var got struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v (%q)", err, out.String())
	}
	if got.Old != "oldname" || got.New != "newname" {
		t.Errorf("got = %+v, want Old=oldname New=newname", got)
	}
}

func TestCmdConcept_RenameRequiresTwoArguments(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("oldname", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"rename", "oldname"}, &out); err == nil {
		t.Error("expected an error when NEW is missing")
	}
}

func TestCmdConcept_RenameNotFound(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"rename", "nonexistent", "newname"}, &out); err == nil {
		t.Error("expected an error for a nonexistent concept")
	}
}

// kb-concept(1) is generated from ConceptHelpText, so a subcommand missing
// from it never reaches the man page.
func TestConceptHelpText_DocumentsRename(t *testing.T) {
	if !strings.Contains(ConceptHelpText, "rename") {
		t.Error("ConceptHelpText does not document rename")
	}
}

func TestCmdConcept_UnknownSubcommand(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"bogus"}, &out); err == nil {
		t.Error("expected an error for an unknown concept subcommand")
	}
}
