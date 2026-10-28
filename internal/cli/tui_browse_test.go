package cli

import (
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/rsdoiel/knowledge"
)

// The Browse rows of Concepts, Documents and Sources, and Check, run the command's
// own read and show its output in the viewer with the command that does the same
// (knowledge DR-0066, DR-0067: a read has no gate). They were dimmed in v0.0.20 and
// v0.0.21.

func browseResult(t *testing.T, m *tuiModel, wants ...string) {
	t.Helper()
	if m.state != viewText {
		t.Fatalf("state = %s, want the viewer with the result\n%s", viewStateNames[m.state], m.View().Content)
	}
	v := m.View().Content
	for _, want := range wants {
		if !strings.Contains(v, want) {
			t.Errorf("the result lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "not in the TUI yet") || strings.Contains(v, "has no screen yet") {
		t.Errorf("the row still says it is not built:\n%s", v)
	}
}

// browseList asserts the model is on a Browse list showing every want.
func browseList(t *testing.T, m *tuiModel, wants ...string) {
	t.Helper()
	if m.state != viewBrowse {
		t.Fatalf("state = %s, want a selectable list\n%s", viewStateNames[m.state], m.View().Content)
	}
	v := m.View().Content
	for _, want := range wants {
		if !strings.Contains(v, want) {
			t.Errorf("the list lacks %q:\n%s", want, v)
		}
	}
}

func TestBrowse_ConceptsIsAListAndEnterShowsTheConcept(t *testing.T) {
	m, _, kb := layoutModel(t)
	for _, n := range []string{"Zebra crossing", "Aardvark"} {
		if _, err := kb.AddConcept(n, "about "+n); err != nil {
			t.Fatal(err)
		}
	}
	m = openLeaf(t, m, "Concepts", "Browse")
	browseList(t, m, "Aardvark", "Zebra crossing", "Enter show")
	m, _ = press(t, m, keyEnter) // the first row
	browseResult(t, m, "Aardvark", "equivalent:", "kb concept show")
	m, _ = press(t, m, ch('q'))
	browseList(t, m, "Zebra crossing")
	m, _ = press(t, m, ch('q'))
	if m.state != viewGroup {
		t.Errorf("q from the list gave %s, want the Concepts menu", viewStateNames[m.state])
	}
}

func TestBrowse_SourcesIsAListAndEnterShowsTheSource(t *testing.T) {
	m, _, kb := layoutModel(t)
	if _, err := kb.AddSource(knowledge.Source{Title: "A Cited Work On Zebras"}); err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Sources", "Browse")
	browseList(t, m, "A Cited Work On Zebras")
	m, _ = press(t, m, keyEnter)
	browseResult(t, m, "A Cited Work On Zebras", "equivalent:", "kb source show")
}

func TestBrowse_DocumentsIsAListAndEnterShowsTheDocument(t *testing.T) {
	m, _, kb := layoutModel(t)
	p, err := kb.ProjectByName("clasm")
	if err != nil || p == nil {
		t.Fatalf("no clasm project: %v", err)
	}
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: p.ID, Title: "The Zebra Chronicle", Format: "markdown", Path: "z.md"}); err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Documents", "Browse")
	browseList(t, m, "The Zebra Chronicle")
	m, _ = press(t, m, keyEnter)
	browseResult(t, m, "The Zebra Chronicle", "equivalent:", "kb document show")
}

// An empty list says so on the menu it came from and does not open a blank screen.
func TestBrowse_AnEmptyListSaysSoAndStaysOnTheMenu(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openLeaf(t, m, "Sources", "Browse")
	if m.state != viewGroup {
		t.Fatalf("state = %s, want the Sources menu", viewStateNames[m.state])
	}
	if v := m.View().Content; !strings.Contains(v, "no sources") {
		t.Errorf("an empty list does not say so:\n%s", v)
	}
}

func TestBrowse_CheckRunsCheckDBAndShowsTheCommand(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openRow(t, m, "Check")
	browseResult(t, m, "equivalent:", "kb check-db")
}

// Observations are a project's: Browse goes to the project list, where Enter opens
// a project and o its observations, and says so.
func TestBrowse_ObservationsGoesToTheProjectList(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openLeaf(t, m, "Observations", "Browse")
	if m.state != viewProjects {
		t.Fatalf("state = %s, want the project list", viewStateNames[m.state])
	}
	if v := m.View().Content; !strings.Contains(v, "observations") {
		t.Errorf("the project list does not say how to reach observations:\n%s", v)
	}
}

// Recall and Suggest take arguments, so they ask for them in a form and then show
// the command's own output; a read has no confirmation (DR-0067).

func TestBrowse_RecallAsksForTextAndShowsWhatIsLinked(t *testing.T) {
	m, _, kb := layoutModel(t)
	if _, err := kb.AddConcept("Zebra", "a striped animal"); err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Concepts", "Recall…")
	if m.state != viewForm {
		t.Fatalf("state = %s, want a form asking for the text", viewStateNames[m.state])
	}
	// text, then concept names and project left empty
	m, _ = press(t, m, append(typed("I saw a zebra"), keyEnter, keyEnter, keyEnter)...)
	browseResult(t, m, "matched concepts", "Zebra", "equivalent:", "kb concept recall", "zebra")
	m, _ = press(t, m, ch('q'))
	if m.state != viewGroup {
		t.Errorf("q from the result gave %s, want the Concepts menu", viewStateNames[m.state])
	}
}

func TestBrowse_RecallWithNothingToLookForSaysSo(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openLeaf(t, m, "Concepts", "Recall…")
	m, _ = press(t, m, keyEnter, keyEnter, keyEnter)
	if m.state != viewGroup {
		t.Fatalf("state = %s, want the Concepts menu", viewStateNames[m.state])
	}
	if v := m.View().Content; !strings.Contains(v, "text or concept names") {
		t.Errorf("no explanation of what to give:\n%s", v)
	}
}

func TestBrowse_RecallGivesADashLeadingTextToTheCommandWhole(t *testing.T) {
	m, _, kb := layoutModel(t)
	if _, err := kb.AddConcept("Zebra", "a striped animal"); err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Concepts", "Recall…")
	m, _ = press(t, m, append(typed("--force zebra"), keyEnter, keyEnter, keyEnter)...)
	browseResult(t, m, "Zebra", "equivalent:")
}

func TestBrowse_SuggestAsksForAProjectAndALimit(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openLeaf(t, m, "Concepts", "Suggest…")
	if m.state != viewForm {
		t.Fatalf("state = %s, want a form", viewStateNames[m.state])
	}
	// project left empty; the limit starts at 20, so clear it and give 5
	m, _ = press(t, m, keyEnter)
	back := tea.KeyPressMsg{Code: tea.KeyBackspace}
	m, _ = press(t, m, append([]tea.Msg{back, back}, append(typed("5"), keyEnter)...)...)
	if m.state != viewText {
		t.Fatalf("state = %s, want the viewer\n%s", viewStateNames[m.state], m.View().Content)
	}
	v := m.View().Content
	for _, want := range []string{"kb concept suggest", "--limit 5", "equivalent:"} {
		if !strings.Contains(v, want) {
			t.Errorf("the result lacks %q:\n%s", want, v)
		}
	}
}

func TestBrowse_SuggestRefusesALimitThatIsNotANumber(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openLeaf(t, m, "Concepts", "Suggest…")
	m, _ = press(t, m, keyEnter)
	back := tea.KeyPressMsg{Code: tea.KeyBackspace}
	m, _ = press(t, m, append([]tea.Msg{back, back}, append(typed("many"), keyEnter)...)...)
	if m.state != viewForm {
		t.Fatalf("state = %s; a bad limit should keep the form open", viewStateNames[m.state])
	}
	if v := m.View().Content; !strings.Contains(v, "number") {
		t.Errorf("the field does not say why it was refused:\n%s", v)
	}
}

// Check retractions sends each registered DOI to a service and marks hits as
// retracted: a write that reaches the network. It asks before it runs, and the
// class is changing, so `:` asks too (DR-0067).

func TestBrowse_CheckRetractionsIsAChangingWrite(t *testing.T) {
	class, ok := commandClass([]string{"source", "check-retractions"})
	if !ok || class != classChanging {
		t.Errorf("class = %v (found %v), want classChanging: it marks sources retracted and uses the network", class, ok)
	}
}

func TestBrowse_CheckRetractionsAsksBeforeAnyLookup(t *testing.T) {
	m, _, kb := layoutModel(t)
	id, err := kb.AddSource(knowledge.Source{Title: "A Source With A DOI", IdentifierType: "doi", IdentifierValue: "10.1234/zebra"})
	if err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Sources", "Check retractions…")
	if m.state != viewPlan {
		t.Fatalf("state = %s, want the confirmation before anything is looked up\n%s", viewStateNames[m.state], m.View().Content)
	}
	v := m.View().Content
	for _, want := range []string{"Watch service", "network access", "kb source check-retractions", "y runs"} {
		if !strings.Contains(v, want) {
			t.Errorf("the confirmation lacks %q:\n%s", want, v)
		}
	}
	m, _ = press(t, m, ch('n'))
	if m.state != viewGroup {
		t.Errorf("n gave %s, want the Sources menu", viewStateNames[m.state])
	}
	src, err := kb.ShowSource(id)
	if err != nil || src == nil {
		t.Fatalf("source %d: %v", id, err)
	}
	if src.Retracted {
		t.Error("a cancelled check marked the source retracted")
	}
}

// With no DOI source there is nothing to look up and no network use, so y can run
// in a test and shows the command's own counts.
func TestBrowse_CheckRetractionsRunsOnYAndShowsTheResult(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = openLeaf(t, m, "Sources", "Check retractions…")
	m, _ = press(t, m, ch('y'))
	if strings.Contains(m.View().Content, "has not been run yet") {
		t.Errorf("y did not run the command:\n%s", m.View().Content)
	}
}

// Enter on a search result and on an observation shows the item through the
// command's own show, with the command that does the same; q comes back to the
// list it was opened from.

func searchFor(t *testing.T, m *tuiModel, term string) *tuiModel {
	t.Helper()
	m, _ = press(t, m, append(append([]tea.Msg{ch('S')}, typed(term)...), keyEnter)...)
	if m.state != viewSearch {
		t.Fatalf("state = %s, want the search results\n%s", viewStateNames[m.state], m.View().Content)
	}
	return m
}

func TestBrowse_EnterOnAnObservationSearchResultShowsTheObservation(t *testing.T) {
	m, _, kb := layoutModel(t)
	p, _ := kb.ProjectByName("clasm")
	if _, err := kb.AddObservation(p.ID, "finding", "a zzzebra was seen near the gate"); err != nil {
		t.Fatal(err)
	}
	m = searchFor(t, m, "zzzebra")
	m, _ = press(t, m, keyEnter)
	browseResult(t, m, "zzzebra was seen", "equivalent:", "kb observation show")
	m, _ = press(t, m, ch('q'))
	if m.state != viewSearch {
		t.Errorf("q gave %s, want the search results", viewStateNames[m.state])
	}
}

func TestBrowse_EnterOnAConceptSearchResultShowsTheConcept(t *testing.T) {
	m, _, kb := layoutModel(t)
	if _, err := kb.AddConcept("Zzzebra", "a striped animal"); err != nil {
		t.Fatal(err)
	}
	m = searchFor(t, m, "zzzebra")
	m, _ = press(t, m, keyEnter)
	browseResult(t, m, "Zzzebra", "equivalent:", "kb concept show")
}

func TestBrowse_EnterOnARecordSearchResultShowsTheRecord(t *testing.T) {
	m, _, _ := layoutModel(t)
	m = searchFor(t, m, "rough")
	m, _ = press(t, m, keyEnter)
	browseResult(t, m, "equivalent:", "kb record show")
}

func TestBrowse_EnterOnAnObservationInAProjectShowsIt(t *testing.T) {
	m, _, kb := layoutModel(t)
	p, _ := kb.ProjectByName("clasm")
	if _, err := kb.AddObservation(p.ID, "note", "the quick zzzebra jumps"); err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Projects", "Browse")
	m, _ = press(t, m, keyEnter) // the project: its observations tab
	if m.state != viewObservations {
		t.Fatalf("state = %s, want the observations tab", viewStateNames[m.state])
	}
	m, _ = press(t, m, keyEnter)
	browseResult(t, m, "quick zzzebra", "equivalent:", "kb observation show")
	m, _ = press(t, m, ch('q'))
	if m.state != viewObservations {
		t.Errorf("q gave %s, want the observations tab", viewStateNames[m.state])
	}
}

// d on a Browse list opens the removing gate for the item (DR-0067): the id typed
// exactly, what goes with it named, and a refusal where the command would refuse.

func TestBrowse_DeleteASourceNeedsItsIDTyped(t *testing.T) {
	m, _, kb := layoutModel(t)
	id, err := kb.AddSource(knowledge.Source{Title: "A Source To Delete"})
	if err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Sources", "Browse")
	m, _ = press(t, m, ch('d'))
	if m.state != viewRemove {
		t.Fatalf("state = %s, want the removing gate\n%s", viewStateNames[m.state], m.View().Content)
	}
	if v := m.View().Content; !strings.Contains(v, "A Source To Delete") || !strings.Contains(v, "nothing cites it") {
		t.Errorf("the gate does not say what is being deleted:\n%s", v)
	}
	m, _ = press(t, m, keyEsc)
	if s, err := kb.ShowSource(id); err != nil || s == nil {
		t.Fatalf("a cancelled delete removed the source: %v", err)
	}
	m, _ = press(t, m, ch('d'))
	m, _ = press(t, m, append(typed(strconv.FormatInt(id, 10)), keyEnter)...)
	if _, err := kb.ShowSource(id); err == nil {
		t.Error("the source is still there after the id was typed")
	}
	if m.state != viewGroup {
		t.Errorf("state = %s, want the Sources menu once the list is empty", viewStateNames[m.state])
	}
	if v := m.View().Content; !strings.Contains(v, "kb source remove "+strconv.FormatInt(id, 10)) {
		t.Errorf("the result does not show the equivalent command:\n%s", v)
	}
}

func TestBrowse_DeleteALinkedSourceIsRefused(t *testing.T) {
	m, _, kb := layoutModel(t)
	p, _ := kb.ProjectByName("clasm")
	oid, err := kb.AddObservation(p.ID, "note", "cites a source")
	if err != nil {
		t.Fatal(err)
	}
	sid, err := kb.AddSource(knowledge.Source{Title: "A Linked Source"})
	if err != nil {
		t.Fatal(err)
	}
	if err := kb.LinkObservationSource(oid, sid, "cites"); err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Sources", "Browse")
	m, _ = press(t, m, ch('d'))
	if m.state != viewBrowse {
		t.Fatalf("state = %s, want to stay on the list with a refusal", viewStateNames[m.state])
	}
	if v := m.View().Content; !strings.Contains(v, "cited by 1 observation") || !strings.Contains(v, "unlink") {
		t.Errorf("the refusal does not say what points at the source:\n%s", v)
	}
}

func TestBrowse_DeleteADocumentNeedsItsIDTyped(t *testing.T) {
	m, _, kb := layoutModel(t)
	p, _ := kb.ProjectByName("clasm")
	id, err := kb.AddDocument(knowledge.Document{ProjectID: p.ID, Title: "A Document To Delete", Format: "markdown", Path: "d.md"})
	if err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Documents", "Browse")
	m, _ = press(t, m, ch('d'))
	if m.state != viewRemove {
		t.Fatalf("state = %s, want the removing gate\n%s", viewStateNames[m.state], m.View().Content)
	}
	if v := m.View().Content; !strings.Contains(v, "re-ingest") || !strings.Contains(v, "A Document To Delete") {
		t.Errorf("the gate lacks what goes and what recreates it:\n%s", v)
	}
	m, _ = press(t, m, append(typed(strconv.FormatInt(id, 10)), keyEnter)...)
	if v := m.View().Content; !strings.Contains(v, "kb document delete "+strconv.FormatInt(id, 10)) {
		t.Errorf("the result does not show the equivalent command:\n%s", v)
	}
	if docs, _ := kb.Documents(0); len(docs) != 0 {
		t.Errorf("the document is still there: %+v", docs)
	}
}

func TestBrowse_DeleteAConceptNeedsItsNameTyped(t *testing.T) {
	m, _, kb := layoutModel(t)
	if _, err := kb.AddConcept("Zebra", "striped"); err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Concepts", "Browse")
	// Concept names in the fixture may include others; move to Zebra
	for i := 0; i < 20 && !strings.Contains(m.browseList.SelectedItem().(browseItem).title, "Zebra"); i++ {
		m, _ = press(t, m, ch('j'))
	}
	m, _ = press(t, m, ch('d'))
	if m.state != viewRemove {
		t.Fatalf("state = %s, want the removing gate", viewStateNames[m.state])
	}
	m, _ = press(t, m, append(typed("Zebra"), keyEnter)...)
	if has, _ := kb.HasConcept("Zebra"); has {
		t.Error("the concept is still there")
	}
}

// The `:` command line reaches the same gates for source remove and document
// delete, which had no screen to select from before the Browse lists.

func TestBrowse_ColonSourceRemoveOpensTheGate(t *testing.T) {
	m, _, kb := layoutModel(t)
	id, err := kb.AddSource(knowledge.Source{Title: "Typed At The Colon"})
	if err != nil {
		t.Fatal(err)
	}
	m = runCommand(t, m, "source remove "+strconv.FormatInt(id, 10))
	if m.state != viewRemove {
		t.Fatalf("state = %s, want the removing gate\n%s", viewStateNames[m.state], m.View().Content)
	}
	if v := m.View().Content; !strings.Contains(v, "Typed At The Colon") {
		t.Errorf("the gate does not name the source:\n%s", v)
	}
}

func TestBrowse_ColonDocumentDeleteOpensTheGate(t *testing.T) {
	m, _, kb := layoutModel(t)
	p, _ := kb.ProjectByName("clasm")
	id, err := kb.AddDocument(knowledge.Document{ProjectID: p.ID, Title: "Typed Document", Format: "markdown", Path: "t.md"})
	if err != nil {
		t.Fatal(err)
	}
	m = runCommand(t, m, "document delete "+strconv.FormatInt(id, 10))
	if m.state != viewRemove {
		t.Fatalf("state = %s, want the removing gate\n%s", viewStateNames[m.state], m.View().Content)
	}
}

// `/` in a Browse list filters the rows (design/scoped-search-design.md, step A):
// it finds in what is in front of the person, as it does in the viewer. The global
// search is `S` there. A filter that matches nothing says so and names the search.

func typeKeys(t *testing.T, m *tuiModel, s string) *tuiModel {
	t.Helper()
	for _, r := range s {
		m, _ = press(t, m, ch(r))
	}
	return m
}

func browseConcepts(t *testing.T) *tuiModel {
	t.Helper()
	m, _, kb := layoutModel(t)
	for _, n := range []string{"Zebra crossing", "Aardvark", "Cache invalidation"} {
		if _, err := kb.AddConcept(n, "about "+n); err != nil {
			t.Fatal(err)
		}
	}
	return openLeaf(t, m, "Concepts", "Browse")
}

func TestBrowse_SlashFiltersTheList(t *testing.T) {
	m := browseConcepts(t)
	m, _ = press(t, m, ch('/'))
	if m.searching {
		t.Fatalf("/ started the global search; in a list it filters")
	}
	m = typeKeys(t, m, "cache")
	m, _ = press(t, m, keyEnter)
	browseList(t, m, "Cache invalidation")
	v := m.View().Content
	for _, gone := range []string{"Aardvark", "Zebra crossing"} {
		if strings.Contains(v, gone) {
			t.Errorf("the filter left %q in the list:\n%s", gone, v)
		}
	}
	// Enter on the filtered list shows the item that is selected, not the first of all.
	m, _ = press(t, m, keyEnter)
	browseResult(t, m, "Cache invalidation")
	m, _ = press(t, m, ch('q'))
	browseList(t, m, "Cache invalidation") // back to the filtered list
}

func TestBrowse_FilterIsCaseInsensitiveAndMatchesDescription(t *testing.T) {
	m := browseConcepts(t)
	m, _ = press(t, m, ch('/'))
	m = typeKeys(t, m, "ABOUT ZEB")
	m, _ = press(t, m, keyEnter)
	browseList(t, m, "Zebra crossing")
	if strings.Contains(m.View().Content, "Aardvark") {
		t.Errorf("Aardvark matched 'ABOUT ZEB':\n%s", m.View().Content)
	}
}

func TestBrowse_TypingTakesEveryKeyAsText(t *testing.T) {
	m := browseConcepts(t)
	m, _ = press(t, m, ch('/'))
	// q, d and S are commands in the list and text in the prompt.
	m = typeKeys(t, m, "qdS")
	if m.state != viewBrowse {
		t.Fatalf("a typed q left the list: state %s", viewStateNames[m.state])
	}
	if m.searching {
		t.Errorf("a typed S started the search")
	}
	if !strings.Contains(m.View().Content, "qdS") {
		t.Errorf("the prompt does not show what was typed:\n%s", m.View().Content)
	}
}

func TestBrowse_EscAbandonsTheFilter(t *testing.T) {
	m := browseConcepts(t)
	m, _ = press(t, m, ch('/'))
	m = typeKeys(t, m, "cache")
	m, _ = press(t, m, keyEsc) // abandons what is being typed
	browseList(t, m, "Aardvark", "Zebra crossing", "Cache invalidation")
	// An accepted filter is cleared by Esc as well, and q still goes back.
	m, _ = press(t, m, ch('/'))
	m = typeKeys(t, m, "cache")
	m, _ = press(t, m, keyEnter)
	m, _ = press(t, m, keyEsc)
	browseList(t, m, "Aardvark", "Zebra crossing", "Cache invalidation")
	m, _ = press(t, m, ch('q'))
	if m.state != viewGroup {
		t.Errorf("q gave %s, want the Concepts menu", viewStateNames[m.state])
	}
}

func TestBrowse_NoMatchSaysSoAndNamesTheSearch(t *testing.T) {
	m := browseConcepts(t)
	m, _ = press(t, m, ch('/'))
	m = typeKeys(t, m, "nonesuch")
	m, _ = press(t, m, keyEnter)
	v := m.View().Content
	if !strings.Contains(v, `no concept matches "nonesuch"`) || !strings.Contains(v, "S searches") {
		t.Errorf("an empty filter should say so and name S:\n%s", v)
	}
	// Enter on an empty list does nothing and does not panic.
	m, _ = press(t, m, keyEnter)
	if m.state != viewBrowse {
		t.Errorf("Enter on the empty list left it: %s", viewStateNames[m.state])
	}
}

func TestBrowse_CapitalSIsTheGlobalSearch(t *testing.T) {
	m := browseConcepts(t)
	m, _ = press(t, m, ch('S'))
	if !m.searching {
		t.Fatalf("S in a Browse list did not start the search\n%s", m.View().Content)
	}
}

func TestBrowse_LegendNamesTheFilterAndTheSearch(t *testing.T) {
	m := browseConcepts(t)
	v := m.View().Content
	if !strings.Contains(v, "/ filter") || !strings.Contains(v, "S search") || strings.Contains(v, "/ search") {
		t.Errorf("legend should say '/ filter' and 'S search':\n%s", v)
	}
}

func TestBrowse_DeleteActsOnTheFilteredSelection(t *testing.T) {
	m := browseConcepts(t)
	m, _ = press(t, m, ch('/'))
	m = typeKeys(t, m, "zebra")
	m, _ = press(t, m, keyEnter)
	m, _ = press(t, m, ch('d'))
	if v := m.View().Content; !strings.Contains(v, "Zebra crossing") || strings.Contains(v, "Aardvark") {
		t.Errorf("the gate should name the filtered row, Zebra crossing:\n%s", v)
	}
}
