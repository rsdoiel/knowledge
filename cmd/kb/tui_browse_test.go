package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rsdoiel/knowledge"
)

// The Browse rows of Concepts, Documents and Sources, and Check, run the command's
// own read and show its output in the viewer with the command that does the same
// (knowledge DR-0066, DR-0067: a read has no gate). They were dimmed in v0.0.20 and
// v0.0.21.

func browseResult(t *testing.T, m *tuiModel, wants ...string) {
	t.Helper()
	if m.state != viewText {
		t.Fatalf("state = %s, want the viewer with the result\n%s", viewStateNames[m.state], m.View())
	}
	v := m.View()
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
		t.Fatalf("state = %s, want a selectable list\n%s", viewStateNames[m.state], m.View())
	}
	v := m.View()
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
	if v := m.View(); !strings.Contains(v, "no sources") {
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
	if v := m.View(); !strings.Contains(v, "observations") {
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
	if v := m.View(); !strings.Contains(v, "text or concept names") {
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
	back := tea.KeyMsg{Type: tea.KeyBackspace}
	m, _ = press(t, m, append([]tea.Msg{back, back}, append(typed("5"), keyEnter)...)...)
	if m.state != viewText {
		t.Fatalf("state = %s, want the viewer\n%s", viewStateNames[m.state], m.View())
	}
	v := m.View()
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
	back := tea.KeyMsg{Type: tea.KeyBackspace}
	m, _ = press(t, m, append([]tea.Msg{back, back}, append(typed("many"), keyEnter)...)...)
	if m.state != viewForm {
		t.Fatalf("state = %s; a bad limit should keep the form open", viewStateNames[m.state])
	}
	if v := m.View(); !strings.Contains(v, "number") {
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
		t.Fatalf("state = %s, want the confirmation before anything is looked up\n%s", viewStateNames[m.state], m.View())
	}
	v := m.View()
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
	if strings.Contains(m.View(), "has not been run yet") {
		t.Errorf("y did not run the command:\n%s", m.View())
	}
}
