package main

import (
	"sort"
	"strings"
	"testing"
)

// The verb table (knowledge DR-0059, v0.0.19 T0) is the one description of kb's
// command language. These tests pin its shape; completion_golden_test.go pins
// that the output derived from it did not change.

func tableVerbNames() []string {
	var names []string
	for _, v := range verbTable {
		names = append(names, v.Name)
	}
	sort.Strings(names)
	return names
}

func TestVerbTable_NamesAreTheRegisteredVerbs(t *testing.T) {
	var registered []string
	for name := range verbs {
		registered = append(registered, name)
	}
	sort.Strings(registered)
	got := tableVerbNames()
	if strings.Join(got, " ") != strings.Join(registered, " ") {
		t.Errorf("the table has verbs %v, the registered verbs are %v", got, registered)
	}
}

func TestVerbTable_EveryRowIsDescribed(t *testing.T) {
	for _, v := range verbTable {
		if strings.TrimSpace(v.Summary) == "" {
			t.Errorf("verb %q has no summary", v.Name)
		}
		if len(v.Subverbs) == 0 && v.Class == classNone {
			t.Errorf("verb %q has no subverbs and no class", v.Name)
		}
		var walk func(path string, subs []subverbSpec)
		walk = func(path string, subs []subverbSpec) {
			for _, s := range subs {
				if len(s.Subverbs) == 0 && s.Class == classNone {
					t.Errorf("%s %s has no class", path, s.Name)
				}
				walk(path+" "+s.Name, s.Subverbs)
			}
		}
		walk(v.Name, v.Subverbs)
	}
}

func TestVerbTable_NoDuplicateNames(t *testing.T) {
	seen := map[string]bool{}
	for _, v := range verbTable {
		if seen[v.Name] {
			t.Errorf("verb %q appears twice", v.Name)
		}
		seen[v.Name] = true
		sub := map[string]bool{}
		for _, s := range v.Subverbs {
			if sub[s.Name] {
				t.Errorf("%s %s appears twice", v.Name, s.Name)
			}
			sub[s.Name] = true
		}
	}
}

// The removing class is the one that gets the extra gate in the TUI (DR-0067),
// so the set is written out here and a change to it is a deliberate edit of
// this test, not a side effect of editing the table.
func TestVerbTable_TheRemovingVerbsAreExactlyThese(t *testing.T) {
	want := []string{
		"concept delete", "document delete", "observation delete", "project delete",
		"record delete", "source remove", "unlink",
	}
	var got []string
	for _, v := range verbTable {
		if len(v.Subverbs) == 0 && v.Class == classRemoving {
			got = append(got, v.Name)
		}
		for _, s := range v.Subverbs {
			if s.Class == classRemoving {
				got = append(got, v.Name+" "+s.Name)
			}
		}
	}
	sort.Strings(got)
	if strings.Join(got, ", ") != strings.Join(want, ", ") {
		t.Errorf("removing verbs = %v, want %v", got, want)
	}
}

// What the design brief says about the verbs that write nothing and the verbs
// kb never offers in the TUI.
func TestVerbTable_ClassesFollowTheDesign(t *testing.T) {
	class := func(path string) writeClass {
		parts := strings.Fields(path)
		for _, v := range verbTable {
			if v.Name != parts[0] {
				continue
			}
			if len(parts) == 1 {
				return v.Class
			}
			for _, s := range v.Subverbs {
				if s.Name != parts[1] {
					continue
				}
				if len(parts) == 2 {
					return s.Class
				}
				for _, n := range s.Subverbs {
					if n.Name == parts[2] {
						return n.Class
					}
				}
			}
		}
		t.Fatalf("no row for %q", path)
		return classNone
	}
	for path, want := range map[string]writeClass{
		"record list": classRead, "record show": classRead, "record pending": classRead, "search": classRead,
		"check-db": classRead, "document review list": classRead,
		"record new": classAdditive, "observation add": classAdditive, "link project": classAdditive, "link observation": classAdditive,
		"record set-status": classChanging, "record supersede": classChanging, "observation update": classChanging,
		"index": classDirect, "record fmt": classDirect,
		"ingest": classPlanApply, "document tag": classPlanApply, "document fuzzy-tag": classPlanApply,
		"document frontmatter": classPlanApply, "record fuzzy-tag": classPlanApply,
		"document ingest": classGuided, "document draft": classGuided, "document review promote": classGuided,
		"merge": classCLIOnly, "import": classCLIOnly, "init": classCLIOnly, "export": classCLIOnly,
		"completion": classCLIOnly,
	} {
		if got := class(path); got != want {
			t.Errorf("%s: class %v, want %v", path, got, want)
		}
	}
}
