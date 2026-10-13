package main

import (
	"strings"
	"testing"

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

func TestBrowse_ConceptsListsConceptsAndShowsTheCommand(t *testing.T) {
	m, _, kb := layoutModel(t)
	if _, err := kb.AddConcept("Zebra crossing", "a place to cross"); err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Concepts", "Browse")
	browseResult(t, m, "Zebra crossing", "equivalent:", "kb concept list")
	m, _ = press(t, m, ch('q'))
	if m.state != viewGroup {
		t.Errorf("q from the result gave %s, want the Concepts menu", viewStateNames[m.state])
	}
}

func TestBrowse_SourcesListsSourcesAndShowsTheCommand(t *testing.T) {
	m, _, kb := layoutModel(t)
	if _, err := kb.AddSource(knowledge.Source{Title: "A Cited Work On Zebras"}); err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Sources", "Browse")
	browseResult(t, m, "A Cited Work On Zebras", "equivalent:", "kb source list")
}

func TestBrowse_DocumentsListsDocumentsAndShowsTheCommand(t *testing.T) {
	m, _, kb := layoutModel(t)
	p, err := kb.ProjectByName("clasm")
	if err != nil || p == nil {
		t.Fatalf("no clasm project: %v", err)
	}
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: p.ID, Title: "The Zebra Chronicle", Format: "markdown", Path: "z.md"}); err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Documents", "Browse")
	browseResult(t, m, "The Zebra Chronicle", "equivalent:", "kb document list")
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
