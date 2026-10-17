package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// ─── concept kind (DR-0078) ──────────────────────────────────────────────────

func TestCmdConcept_AddWithKind(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"add", "--kind", "character", "Anna", "the", "protagonist"}, &out); err != nil {
		t.Fatalf("concept add --kind: %v", err)
	}
	cs, _ := kb.Concepts()
	if len(cs) != 1 || cs[0].Kind != "character" || cs[0].Description != "the protagonist" {
		t.Errorf("concepts = %+v, want Anna as a character with its description", cs)
	}
}

func TestCmdConcept_AddWithUnknownKindWritesNothing(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	err := cmdConcept(kb, nil, false, []string{"add", "--kind", "villain", "Anna"}, &out)
	if err == nil {
		t.Fatal("expected an error for an unknown kind")
	}
	if cs, _ := kb.Concepts(); len(cs) != 0 {
		t.Errorf("concepts = %+v, want none: the bad kind must be refused before anything is written", cs)
	}
}

func TestCmdConcept_SetKind(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("Kitchen", "")
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"set-kind", "Kitchen", "place"}, &out); err != nil {
		t.Fatalf("concept set-kind: %v", err)
	}
	if cs, _ := kb.ConceptsOfKind("place"); len(cs) != 1 || cs[0].Name != "Kitchen" {
		t.Errorf("places = %+v", cs)
	}
}

func TestCmdConcept_ListByKindAndKindInOutput(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("Anna", "")
	kb.AddConcept("Grief", "")
	kb.SetConceptKind("Anna", "character")

	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"list", "--kind", "character"}, &out); err != nil {
		t.Fatalf("concept list --kind: %v", err)
	}
	if !strings.Contains(out.String(), "Anna") || strings.Contains(out.String(), "Grief") {
		t.Errorf("list --kind character = %q, want only Anna", out.String())
	}

	out.Reset()
	cmdConcept(kb, nil, false, []string{"list"}, &out)
	if !strings.Contains(out.String(), "character") {
		t.Errorf("plain list = %q, want a non-default kind shown", out.String())
	}

	out.Reset()
	if err := cmdConcept(kb, nil, true, []string{"list"}, &out); err != nil {
		t.Fatal(err)
	}
	var got []struct {
		Name string
		Kind string
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("list --json: %v: %s", err, out.String())
	}
	kinds := map[string]string{}
	for _, c := range got {
		kinds[c.Name] = c.Kind
	}
	if kinds["Anna"] != "character" || kinds["Grief"] != "other" {
		t.Errorf("json kinds = %v", kinds)
	}
}

func TestCmdConcept_ShowSaysTheKind(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("Anna", "")
	kb.SetConceptKind("Anna", "character")
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"show", "Anna"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "kind: character") {
		t.Errorf("show = %q, want a kind line", out.String())
	}
	out.Reset()
	cmdConcept(kb, nil, true, []string{"show", "Anna"}, &out)
	if !strings.Contains(out.String(), `"kind": "character"`) && !strings.Contains(out.String(), `"kind":"character"`) {
		t.Errorf("show --json = %q, want kind", out.String())
	}
}

func TestConceptKind_ExitCodes(t *testing.T) {
	for _, tc := range []verbCase{
		{name: "add unknown kind is usage", args: []string{"concept", "add", "--kind", "villain", "A"}, want: classUsage},
		{name: "set-kind unknown kind is usage", setup: [][]string{{"concept", "add", "A"}}, args: []string{"concept", "set-kind", "A", "villain"}, want: classUsage},
		{name: "set-kind missing argument is usage", args: []string{"concept", "set-kind", "A"}, want: classUsage},
		{name: "set-kind surplus argument is usage", args: []string{"concept", "set-kind", "A", "place", "x"}, want: classUsage},
		{name: "list unknown kind is usage", args: []string{"concept", "list", "--kind", "villain"}, want: classUsage},
		{name: "set-kind missing concept is negative", args: []string{"concept", "set-kind", "nosuch", "place"}, want: classNegative},
		{name: "import with an unknown kind is data", files: map[string]string{"bad.jsonl": `{"type":"concept","uuid":"u1","origin_host":"h","name":"A","description":"","identifier_type":"","identifier_value":"","kind":"villain","created_at":"2026-01-01 00:00:00","updated_at":"2026-01-01 00:00:00"}` + "\n"}, args: []string{"import", "-in", "bad.jsonl"}, want: classData},
	} {
		t.Run(tc.name, func(t *testing.T) { runVerbCase(t, tc) })
	}
}
