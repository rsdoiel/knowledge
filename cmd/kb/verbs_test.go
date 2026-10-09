package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// `kb verbs` prints the verb table (knowledge DR-0059, v0.0.19 T0b) for models
// and scripts: every verb, its summary, its subverbs and their write classes,
// and its flags. --json is the global option and goes before the verb, as for
// every other kb verb. The verb opens no database.

type verbsDoc struct {
	Verbs []struct {
		Name     string `json:"name"`
		Summary  string `json:"summary"`
		Class    string `json:"class"`
		TUI      bool   `json:"tui"`
		Flags    []string
		Subverbs []struct {
			Name     string `json:"name"`
			Class    string `json:"class"`
			TUI      bool   `json:"tui"`
			Subverbs []struct {
				Name  string `json:"name"`
				Class string `json:"class"`
			} `json:"subverbs"`
		} `json:"subverbs"`
	} `json:"verbs"`
}

func TestVerbs_JSONIsTheTable(t *testing.T) {
	t.Chdir(t.TempDir())
	code, out, errOut := runMain(t, "--json", "verbs")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	assertValidJSON(t, []byte(out))
	var doc verbsDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(doc.Verbs) != len(verbTable) {
		t.Fatalf("%d verbs printed, the table has %d", len(doc.Verbs), len(verbTable))
	}
	for i, v := range verbTable {
		if doc.Verbs[i].Name != v.Name || doc.Verbs[i].Summary != v.Summary {
			t.Errorf("verb %d = %q / %q, want %q / %q", i, doc.Verbs[i].Name, doc.Verbs[i].Summary, v.Name, v.Summary)
		}
	}
	find := func(name string) (idx int) {
		for i, v := range doc.Verbs {
			if v.Name == name {
				return i
			}
		}
		t.Fatalf("no verb %q in the output", name)
		return -1
	}
	rec := doc.Verbs[find("record")]
	var sawSetStatus bool
	for _, s := range rec.Subverbs {
		if s.Name == "set-status" {
			sawSetStatus = true
			if s.Class != "changing" || !s.TUI {
				t.Errorf("record set-status = class %q tui %v, want changing / true", s.Class, s.TUI)
			}
		}
		if s.Name == "delete" && s.Class != "removing" {
			t.Errorf("record delete class = %q, want removing", s.Class)
		}
	}
	if !sawSetStatus {
		t.Error("record has no set-status subverb in the output")
	}
	doc2 := doc.Verbs[find("document")]
	var promote string
	for _, s := range doc2.Subverbs {
		if s.Name == "review" {
			for _, n := range s.Subverbs {
				if n.Name == "promote" {
					promote = n.Class
				}
			}
		}
	}
	if promote != "guided" {
		t.Errorf("document review promote class = %q, want guided", promote)
	}
	if v := doc.Verbs[find("merge")]; v.Class != "cli_only" || v.TUI {
		t.Errorf("merge = class %q tui %v, want cli_only / false", v.Class, v.TUI)
	}
	if v := doc.Verbs[find("ingest")]; v.Class != "plan_apply" || !v.TUI {
		t.Errorf("ingest = class %q tui %v, want plan_apply / true", v.Class, v.TUI)
	}
}

func TestVerbs_TextListsEveryVerbAndSubverbWithItsClass(t *testing.T) {
	t.Chdir(t.TempDir())
	code, out, errOut := runMain(t, "verbs")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	for _, want := range []string{"record set-status", "changing", "record delete", "removing", "document review promote",
		"guided", "merge", "cli_only", "ingest", "plan_apply", "index", "direct"} {
		if !strings.Contains(out, want) {
			t.Errorf("the text output lacks %q:\n%s", want, out)
		}
	}
}

func TestVerbs_OpensNoDatabase(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if code, _, errOut := runMain(t, "verbs"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("kb verbs created %v in a directory with no workspace", entries)
	}
}

func TestVerbs_ExitCodes(t *testing.T) {
	t.Chdir(t.TempDir())
	for name, args := range map[string][]string{
		"unknown flag":        {"verbs", "--bogus"},
		"surplus argument":    {"verbs", "extra"},
		"trailing --json":     {"verbs", "--json"},
		"--db does not apply": {"--db", "x.db", "verbs"},
	} {
		if code, _, _ := runMain(t, args...); code != 2 {
			t.Errorf("%s: exit %d, want 2", name, code)
		}
	}
}

// Every class has the name the JSON and the text output use.
func TestWriteClass_Names(t *testing.T) {
	for class, want := range map[writeClass]string{
		classRead: "read", classAdditive: "additive", classChanging: "changing", classRemoving: "removing",
		classDirect: "direct", classPlanApply: "plan_apply", classGuided: "guided", classCLIOnly: "cli_only",
	} {
		if got := class.String(); got != want {
			t.Errorf("class %d = %q, want %q", class, got, want)
		}
	}
}
