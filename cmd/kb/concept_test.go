package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCmdConcept_AddThenList(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdConcept(kb, false, []string{"add", "WAL", "write-ahead", "logging"}, &out); err != nil {
		t.Fatalf("concept add: %v", err)
	}
	if !strings.Contains(out.String(), "WAL") {
		t.Errorf("add output = %q, want it to mention WAL", out.String())
	}

	out.Reset()
	if err := cmdConcept(kb, false, []string{"list"}, &out); err != nil {
		t.Fatalf("concept list: %v", err)
	}
	if !strings.Contains(out.String(), "WAL") {
		t.Errorf("list output = %q, want it to mention WAL", out.String())
	}
}

func TestCmdConcept_AddWithIdentifier(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	err := cmdConcept(kb, false, []string{"add", "--identifier-type", "orcid", "--identifier-value", "0000-0003-0900-6903", "Jane Doe"}, &out)
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
	if err := cmdConcept(kb, false, []string{"add"}, &out); err == nil {
		t.Error("expected an error when NAME is missing")
	}
}

func TestCmdConcept_UnknownSubcommand(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdConcept(kb, false, []string{"bogus"}, &out); err == nil {
		t.Error("expected an error for an unknown concept subcommand")
	}
}
