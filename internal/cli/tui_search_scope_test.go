package cli

import (
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// The Search group, the scoped prompt and the S key (knowledge DR-0073, DR-0074,
// design/scoped-search-design.md section 7, step E). The prompt takes the same
// grammar as the command line, so the menu rows only prefill what the command
// would be told, and what runs is shown with its equivalent.

func TestSearchGroup_HasAnEverywhereRowAndOneRowPerKind(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openRow(t, m, "Search")
	if m.state != viewGroup || m.group != "search" {
		t.Fatalf("state = %s group %q, want the Search menu\n%s", viewStateNames[m.state], m.group, m.View().Content)
	}
	v := m.View().Content
	for _, want := range []string{"Everywhere…", "Workspace tier…", "Records…", "Observations…", "Concepts…", "Documents…", "Projects…"} {
		if !strings.Contains(v, want) {
			t.Errorf("the Search menu lacks %q:\n%s", want, v)
		}
	}
}

func TestSearchGroup_ARowOpensThePromptWithTheScopePrefilled(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openLeaf(t, m, "Search", "Records…")
	if !m.searching {
		t.Fatalf("Records… should open the search prompt\n%s", m.View().Content)
	}
	if v := m.View().Content; !strings.Contains(v, "/records") {
		t.Errorf("the prompt should show the scope it will run, /records:\n%s", v)
	}
	m, _ = press(t, m, typed("first")...)
	m, _ = press(t, m, keyEnter)
	if m.state != viewSearch {
		t.Fatalf("state = %s, want the results\n%s", viewStateNames[m.state], m.View().Content)
	}
	v := m.View().Content
	for _, want := range []string{"clasm/DR-0001", "kb search /records first"} {
		if !strings.Contains(v, want) {
			t.Errorf("the results lack %q:\n%s", want, v)
		}
	}
}

func TestSearchPrompt_TakesTheCommandLineGrammar(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openLeaf(t, m, "Search", "Everywhere…")
	m, _ = press(t, m, typed("clasm/records rough")...)
	m, _ = press(t, m, keyEnter)
	if m.state != viewSearch {
		t.Fatalf("state = %s\n%s", viewStateNames[m.state], m.View().Content)
	}
	v := m.View().Content
	if !strings.Contains(v, "A rough one") || strings.Contains(v, "The first") {
		t.Errorf("clasm/records rough should find the rough record only:\n%s", v)
	}
}

func TestSearchPrompt_AUsageErrorStaysInThePrompt(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openLeaf(t, m, "Search", "Everywhere…")
	m, _ = press(t, m, typed("nosuch/ x")...)
	m, _ = press(t, m, keyEnter)
	if !m.searching {
		t.Fatalf("a bad scope should keep the prompt open so it can be corrected\n%s", m.View().Content)
	}
	if v := m.View().Content; !strings.Contains(v, "unknown scope") {
		t.Errorf("the error should be shown:\n%s", v)
	}
}

func TestSearchPrompt_NoMembersIsSaidNotShownAsAnEmptyList(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openLeaf(t, m, "Search", "Everywhere…")
	m, _ = press(t, m, typed("workspace/observations x")...)
	m, _ = press(t, m, keyEnter)
	if !m.searching {
		t.Fatalf("the prompt should stay open\n%s", m.View().Content)
	}
	if v := m.View().Content; !strings.Contains(v, "workspace tier") {
		t.Errorf("the reason should be shown:\n%s", v)
	}
}

func TestSearchPrompt_NothingFoundSaysSoInTheResults(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openLeaf(t, m, "Search", "Everywhere…")
	m, _ = press(t, m, typed("zzzznothing")...)
	m, _ = press(t, m, keyEnter)
	if m.state != viewSearch {
		t.Fatalf("state = %s", viewStateNames[m.state])
	}
	if v := m.View().Content; !strings.Contains(v, "no results") {
		t.Errorf("an empty result should say so:\n%s", v)
	}
}

func TestSearchKey_CapitalSOpensThePromptInEveryContext(t *testing.T) {
	m, _, _ := layoutModel(t)
	// The top menu.
	m, _ = press(t, m, ch('S'))
	if !m.searching {
		t.Fatalf("S at the top menu should open the prompt\n%s", m.View().Content)
	}
	m, _ = press(t, m, keyEsc)
	// A group menu.
	m = openRow(t, m, "Records")
	m, _ = press(t, m, ch('S'))
	if !m.searching {
		t.Errorf("S in a group menu should open the prompt")
	}
}

func TestSearchKey_InAProjectTheScopeIsThatProject(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openLeaf(t, m, "Projects", "Browse")
	m, _ = press(t, m, keyEnter) // the project's observations
	if m.state != viewObservations {
		t.Fatalf("state = %s, want the project's observations", viewStateNames[m.state])
	}
	m, _ = press(t, m, ch('S'))
	if !m.searching {
		t.Fatalf("S should open the prompt\n%s", m.View().Content)
	}
	if v := m.View().Content; !strings.Contains(v, "clasm/") {
		t.Errorf("the prompt should be scoped to the project it was opened in, clasm/:\n%s", v)
	}
}

func TestSearchKey_InABrowseListTheScopeIsTheKind(t *testing.T) {
	m, _, kb := layoutModel(t)
	if _, err := kb.AddConcept("Aardvark", "about the aardvark"); err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Concepts", "Browse")
	m, _ = press(t, m, ch('S'))
	if !m.searching {
		t.Fatalf("S should open the prompt\n%s", m.View().Content)
	}
	if v := m.View().Content; !strings.Contains(v, "/concepts") {
		t.Errorf("the prompt should be scoped to the kind, /concepts:\n%s", v)
	}
}

func TestSearchKey_LegendNamesIt(t *testing.T) {
	m, _, _ := layoutModel(t)
	if v := m.View().Content; !strings.Contains(v, "S search") {
		t.Errorf("the top menu legend should name S:\n%s", v)
	}
}

func TestSearchGroup_ThisProjectRowAppearsWhenAProjectIsKnown(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openRow(t, m, "Search")
	if strings.Contains(m.View().Content, "This project") {
		t.Errorf("no project is selected yet:\n%s", m.View().Content)
	}
	m, _ = press(t, m, ch('q'))
	m = openLeaf(t, m, "Projects", "Browse")
	m, _ = press(t, m, keyEnter)
	m, _ = press(t, m, ch('q'), ch('q'), ch('q')) // tabs, project list, Projects menu: the top menu
	m = openRow(t, m, "Search")
	if !strings.Contains(m.View().Content, "This project (clasm)…") {
		t.Errorf("a selected project should be offered:\n%s", m.View().Content)
	}
}

func TestDeepLink_BareSearchOpensTheSearchMenuOnATerminal(t *testing.T) {
	start, open, err := deepLink([]string{"search"}, false, true)
	if err != nil || !open || start.state != viewGroup || start.group != "search" {
		t.Errorf("kb search on a terminal = %+v open %v err %v, want the Search menu", start, open, err)
	}
	_, open, err = deepLink([]string{"search"}, false, false)
	if open || err != nil {
		t.Errorf("without a terminal it is left to the command (a usage error): open %v err %v", open, err)
	}
	start, open, err = deepLink([]string{"search", "harvey/records"}, false, true)
	if err != nil || !open || start.prompt != "harvey/records" {
		t.Errorf("a scope with no term should open the prompt on it: %+v open %v err %v", start, open, err)
	}
	_, open, _ = deepLink([]string{"search", "harvey/records", "exit"}, false, true)
	if open {
		t.Errorf("a complete command runs as a command")
	}
}

// sources became a search kind in v0.0.25 (DR-0080), so S on the sources list
// is scoped to them like the concepts and documents lists.
func TestSearchKey_InTheSourcesBrowseListTheScopeIsSources(t *testing.T) {
	m, _, kb := layoutModel(t)
	if _, err := kb.AddSource(knowledge.Source{Title: "A Later History", PublishedDate: "1975"}); err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Sources", "Browse")
	m, _ = press(t, m, ch('S'))
	if !m.searching {
		t.Fatalf("S should open the prompt\n%s", m.View().Content)
	}
	if v := m.View().Content; !strings.Contains(v, "/sources") {
		t.Errorf("the prompt should be scoped to the kind, /sources:\n%s", v)
	}
}
