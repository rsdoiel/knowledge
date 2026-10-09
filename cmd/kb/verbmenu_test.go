package main

import (
	"strings"
	"testing"
)

// The TUI's menus are derived from the verb table (knowledge DR-0066 and its
// 2026-10-09 amendment, v0.0.19 T5): one top-menu row per verb group in the
// table's terms, so using the menu teaches the command language; group menus of
// the leaves with no natural item; unbuilt entries dimmed with the release and
// the equivalent command.

func labels(items []menuItem) string {
	var ls []string
	for _, it := range items {
		ls = append(ls, it.Label)
	}
	return strings.Join(ls, ", ")
}

func TestMenu_TheTopMenuIsOneRowPerVerbGroupInOrder(t *testing.T) {
	want := "Projects, Records, Observations, Concepts, Sources, Documents, Search, Ingest, Index, Check"
	if got := labels(topMenu()); got != want {
		t.Errorf("top menu = %s\nwant       %s", got, want)
	}
}

func TestMenu_WhatIsBuilt(t *testing.T) {
	built := map[string]bool{}
	for _, it := range topMenu() {
		built[it.Label] = it.Built
	}
	for label, want := range map[string]bool{
		"Projects": true, "Records": true, "Search": true, "Index": true,
		"Observations": true, "Concepts": true, "Sources": true, "Documents": true,
		"Ingest": true, "Check": false,
	} {
		if built[label] != want {
			t.Errorf("%s: built = %v, want %v", label, built[label], want)
		}
	}
}

func TestMenu_TheRecordsGroup(t *testing.T) {
	items := groupMenu("record")
	if got, want := labels(items), "Browse, Pending, New record…, Format files…, Fuzzy-tag…"; got != want {
		t.Fatalf("records menu = %s, want %s", got, want)
	}
	for i, wantBuilt := range []bool{true, true, true, true, true} {
		if items[i].Built != wantBuilt {
			t.Errorf("%s: built = %v, want %v", items[i].Label, items[i].Built, wantBuilt)
		}
	}
	if items[0].Sub != "list" || items[1].Sub != "pending" {
		t.Errorf("Browse and Pending run %q and %q, want list and pending", items[0].Sub, items[1].Sub)
	}
}

// Groups with nothing built are still groups: they open to a menu of dimmed rows.
func TestMenu_EveryGroupHasLeaves(t *testing.T) {
	for _, it := range topMenu() {
		if !it.Group {
			continue
		}
		if len(groupMenu(it.Verb)) == 0 {
			t.Errorf("%s is a group with no leaves", it.Label)
		}
	}
}

// Every row says what the command-line equivalent is, and every unbuilt row says
// when it arrives.
func TestMenu_EveryRowCarriesItsEquivalentAndItsRelease(t *testing.T) {
	var all []menuItem
	all = append(all, topMenu()...)
	for _, it := range topMenu() {
		if it.Group {
			all = append(all, groupMenu(it.Verb)...)
		}
	}
	for _, it := range all {
		if it.Group {
			continue // a group's rows are checked through its leaves
		}
		if !strings.HasPrefix(it.Equivalent, "kb ") || !strings.Contains(it.Equivalent, it.Verb) {
			t.Errorf("%s: equivalent %q should be a kb command naming %s", it.Label, it.Equivalent, it.Verb)
		}
		if it.Desc == "" {
			t.Errorf("%s: no description", it.Label)
		}
		if !it.Built && it.Since == "" {
			t.Errorf("%s: not built and no release named", it.Label)
		}
		if it.Built && it.Since != "" {
			t.Errorf("%s: built but still names release %s", it.Label, it.Since)
		}
	}
}

// The menus only offer what the table says the TUI offers, and the table's rows
// that are not in the menus are the ones that act on an item or are CLI only.
func TestMenu_OnlyTUIVerbsAppear(t *testing.T) {
	inMenu := map[string]bool{}
	for _, it := range topMenu() {
		inMenu[it.Verb] = true
	}
	for _, v := range verbTable {
		cliOnly := len(v.Subverbs) == 0 && v.Class == classCLIOnly
		if cliOnly && inMenu[v.Name] {
			t.Errorf("%s is CLI only and has a menu row", v.Name)
		}
	}
	for _, name := range []string{"merge", "import", "init", "export", "completion", "verbs"} {
		if inMenu[name] {
			t.Errorf("%s appears in the top menu", name)
		}
	}
}

func TestMenu_RowsAreInOrderWithNoTies(t *testing.T) {
	seen := map[int]string{}
	for _, v := range verbTable {
		if v.Menu == nil {
			continue
		}
		if prev, dup := seen[v.Menu.Order]; dup {
			t.Errorf("%s and %s share top-menu position %d", prev, v.Name, v.Menu.Order)
		}
		seen[v.Menu.Order] = v.Name
	}
	for _, v := range verbTable {
		pos := map[int]string{}
		for _, s := range v.Subverbs {
			if s.Menu == nil {
				continue
			}
			if prev, dup := pos[s.Menu.Order]; dup {
				t.Errorf("%s: %s and %s share menu position %d", v.Name, prev, s.Name, s.Menu.Order)
			}
			pos[s.Menu.Order] = s.Name
		}
	}
}
