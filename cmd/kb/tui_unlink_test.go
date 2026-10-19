package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	knowledge "github.com/rsdoiel/knowledge"
)

// `u` cuts a link from the terminal interface (knowledge DR-0067: unlink is a
// removing verb, so it has a gate): on a project's Concepts tab it unlinks the
// selected concept from the project; on an observation it lists the
// observation's concept and source links to pick one. Both end in the same typed
// confirmation as a delete, and show the command that does the same.

// unlinkModel is the project alpha with one observation that has a concept and a
// source linked, and the concept streaming linked to the project.
func unlinkModel(t *testing.T) (*tuiModel, *knowledge.KnowledgeBase) {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("KB_PROJECT", "")
	m := newTestTUIModel(t)
	kb := m.kb
	p, _ := kb.ProjectByName("alpha")
	obs, err := kb.Observations(p.ID)
	if err != nil || len(obs) == 0 {
		t.Fatalf("no observation: %v", err)
	}
	cid, _ := kb.AddConcept("caching", "keep it close")
	if err := kb.LinkObservationConcept(obs[0].ID, cid); err != nil {
		t.Fatal(err)
	}
	sid, err := kb.AddSource(knowledge.Source{Title: "A cited paper"})
	if err != nil {
		t.Fatal(err)
	}
	if err := kb.LinkObservationSource(obs[0].ID, sid, "cited"); err != nil {
		t.Fatal(err)
	}
	fresh, err := newTUIModelAt(kb, nil, tuiStart{state: viewProjects})
	if err != nil {
		t.Fatal(err)
	}
	next, _ := fresh.Update(windowSize80())
	return next.(*tuiModel), kb
}

// windowSize80 is the size a terminal reports before anything is drawn.
func windowSize80() tea.Msg { return tea.WindowSizeMsg{Width: 80, Height: 24} }

func TestUnlink_ProjectConceptFromTheConceptsTab(t *testing.T) {
	m, kb := unlinkModel(t)
	m, _ = press(t, m, keyEnter, ch('c')) // alpha, its Concepts tab
	if m.state != viewConcepts {
		t.Fatalf("state = %s", viewStateNames[m.state])
	}
	if v := m.View().Content; !strings.Contains(v, "u unlink") {
		t.Errorf("the tab's legend should offer u unlink:\n%s", v)
	}
	m, _ = press(t, m, ch('u'))
	if m.state != viewRemove {
		t.Fatalf("state = %s, want the gate\n%s", viewStateNames[m.state], m.View().Content)
	}
	v := m.View().Content
	for _, want := range []string{"Unlink", `concept "streaming"`, "project alpha", "kb unlink project alpha streaming"} {
		if !strings.Contains(v, want) {
			t.Errorf("the gate lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "Delete") {
		t.Errorf("an unlink is not a delete:\n%s", v)
	}
	m, _ = press(t, m, typed("streaming")...)
	m, _ = press(t, m, keyEnter)
	if m.state != viewConcepts {
		t.Fatalf("state = %s, want back on the tab", viewStateNames[m.state])
	}
	for _, want := range []string{"unlinked", "kb unlink project alpha streaming"} {
		if !strings.Contains(m.View().Content, want) {
			t.Errorf("the result lacks %q:\n%s", want, m.View().Content)
		}
	}
	p, _ := kb.ProjectByName("alpha")
	if cs, _ := kb.ProjectConcepts(p.ID); len(cs) != 0 {
		t.Errorf("the link is still there: %v", cs)
	}
	if _, err := kb.ConceptUsage("streaming"); err != nil {
		t.Errorf("the concept itself was deleted; an unlink keeps it: %v", err)
	}
}

func TestUnlink_AWrongConfirmationAndEscChangeNothing(t *testing.T) {
	m, kb := unlinkModel(t)
	m, _ = press(t, m, keyEnter, ch('c'), ch('u'))
	m, _ = press(t, m, typed("stream")...)
	m, _ = press(t, m, keyEnter)
	if m.state != viewRemove {
		t.Fatalf("a wrong confirmation left the gate: %s", viewStateNames[m.state])
	}
	m, _ = press(t, m, keyEsc)
	if m.state != viewConcepts || !strings.Contains(m.View().Content, "cancelled") {
		t.Errorf("Esc: state %s\n%s", viewStateNames[m.state], m.View().Content)
	}
	p, _ := kb.ProjectByName("alpha")
	if cs, _ := kb.ProjectConcepts(p.ID); len(cs) != 1 {
		t.Errorf("the link changed: %v", cs)
	}
}

func TestUnlink_AnObservationListsItsLinksToPickFrom(t *testing.T) {
	m, kb := unlinkModel(t)
	m, _ = press(t, m, keyEnter) // alpha's observations
	if m.state != viewObservations {
		t.Fatalf("state = %s", viewStateNames[m.state])
	}
	if v := m.View().Content; !strings.Contains(v, "u unlink") {
		t.Errorf("the tab's legend should offer u unlink:\n%s", v)
	}
	m, _ = press(t, m, ch('u'))
	if m.state != viewUnlink {
		t.Fatalf("state = %s, want the link picker\n%s", viewStateNames[m.state], m.View().Content)
	}
	v := m.View().Content
	for _, want := range []string{`concept "caching"`, "source", "A cited paper"} {
		if !strings.Contains(v, want) {
			t.Errorf("the picker lacks %q:\n%s", want, v)
		}
	}
	// The concept is first; Enter opens its gate.
	m, _ = press(t, m, keyEnter)
	if m.state != viewRemove {
		t.Fatalf("state = %s, want the gate", viewStateNames[m.state])
	}
	if v := m.View().Content; !strings.Contains(v, "kb unlink observation") || !strings.Contains(v, "caching") {
		t.Errorf("the gate should name the command:\n%s", v)
	}
	m, _ = press(t, m, typed("caching")...)
	m, _ = press(t, m, keyEnter)
	if m.state != viewObservations {
		t.Fatalf("state = %s, want back on the observations", viewStateNames[m.state])
	}
	p, _ := kb.ProjectByName("alpha")
	obs, _ := kb.Observations(p.ID)
	if cs, _ := kb.ObservationConcepts(obs[0].ID); len(cs) != 0 {
		t.Errorf("the concept link is still there: %v", cs)
	}
	if ss, _ := kb.ObservationSources(obs[0].ID); len(ss) != 1 {
		t.Errorf("the source link should be untouched: %v", ss)
	}
}

func TestUnlink_ASourceLinkIsConfirmedByItsID(t *testing.T) {
	m, kb := unlinkModel(t)
	m, _ = press(t, m, keyEnter, ch('u'))
	m, _ = press(t, m, ch('j'), keyEnter) // the source row
	if m.state != viewRemove {
		t.Fatalf("state = %s", viewStateNames[m.state])
	}
	v := m.View().Content
	if !strings.Contains(v, "kb unlink source") || !strings.Contains(v, "A cited paper") {
		t.Errorf("the gate should name the source and the command:\n%s", v)
	}
	m, _ = press(t, m, typed("1")...)
	m, _ = press(t, m, keyEnter)
	p, _ := kb.ProjectByName("alpha")
	obs, _ := kb.Observations(p.ID)
	if ss, _ := kb.ObservationSources(obs[0].ID); len(ss) != 0 {
		t.Errorf("the source link is still there: %v", ss)
	}
}

func TestUnlink_AnObservationWithNoLinksSaysSo(t *testing.T) {
	m, kb := unlinkModel(t)
	p, _ := kb.ProjectByName("beta")
	if _, err := kb.AddObservation(p.ID, "note", "nothing is linked to this"); err != nil {
		t.Fatal(err)
	}
	fresh, err := newTUIModelAt(kb, nil, tuiStart{state: viewProjects})
	if err != nil {
		t.Fatal(err)
	}
	next, _ := fresh.Update(windowSize80())
	m = next.(*tuiModel)
	m, _ = press(t, m, ch('j'), keyEnter, ch('u')) // beta's observation
	if m.state != viewObservations || !strings.Contains(m.View().Content, "no concept or source links") {
		t.Errorf("state %s\n%s", viewStateNames[m.state], m.View().Content)
	}
}

func TestUnlink_QAndEscLeaveThePicker(t *testing.T) {
	m, _ := unlinkModel(t)
	m, _ = press(t, m, keyEnter, ch('u'))
	m, _ = press(t, m, ch('q'))
	if m.state != viewObservations {
		t.Errorf("q gave %s, want the observations", viewStateNames[m.state])
	}
}

func TestUnlink_SIsNotOfferedWhileTyping(t *testing.T) {
	m, _ := unlinkModel(t)
	m, _ = press(t, m, keyEnter, ch('/'))
	m, _ = press(t, m, ch('u'))
	if m.state != viewObservations || !m.filterTyping() {
		t.Errorf("a typed u acted as a command: %s", viewStateNames[m.state])
	}
}
