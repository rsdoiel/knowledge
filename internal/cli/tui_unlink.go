package cli

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	knowledge "github.com/rsdoiel/knowledge"
)

// unlinkItem is one link of an observation in the picker: a concept or a source.
type unlinkItem struct {
	concept string // the concept's name, for a concept link
	source  int64  // the source's id, for a source link
	title   string // the source's title
}

func (i unlinkItem) Title() string {
	if i.source != 0 {
		return fmt.Sprintf("source %d", i.source)
	}
	return fmt.Sprintf("concept %q", i.concept)
}

func (i unlinkItem) Description() string {
	if i.source != 0 {
		return i.title
	}
	return "linked concept"
}

func (i unlinkItem) FilterValue() string { return i.Title() + " " + i.title }

// planProjectConceptUnlink plans `kb unlink project PROJECT CONCEPT`: the link
// goes and the concept stays, with whatever else it is linked to.
func planProjectConceptUnlink(kb *knowledge.KnowledgeBase, project, concept string) removalPlan {
	return removalPlan{
		Verb:       "Unlink",
		Title:      fmt.Sprintf("concept %q from project %s", concept, project),
		Confirm:    concept,
		Equivalent: fmt.Sprintf("kb unlink project %s %s", shellWord(project), shellWord(concept)),
		Lines: []string{
			"only the link between the project and the concept goes; the concept stays",
			"the project may still reach it through its observations and records",
		},
		Apply: func() error { return kb.UnlinkProjectConcept(project, concept) },
	}
}

// planObservationConceptUnlink plans `kb unlink observation ID CONCEPT`.
func planObservationConceptUnlink(kb *knowledge.KnowledgeBase, id int64, concept string) removalPlan {
	return removalPlan{
		Verb:       "Unlink",
		Title:      fmt.Sprintf("concept %q from observation %d", concept, id),
		Confirm:    concept,
		Equivalent: fmt.Sprintf("kb unlink observation %d %s", id, shellWord(concept)),
		Lines:      []string{"only the link goes; the observation and the concept stay"},
		Apply:      func() error { return kb.UnlinkObservationConcept(id, concept) },
	}
}

// planObservationSourceUnlink plans `kb unlink source OBS_ID SOURCE_ID`.
func planObservationSourceUnlink(kb *knowledge.KnowledgeBase, obs, src int64, title string) removalPlan {
	lines := []string{"only the link goes; the observation and the source stay"}
	if t := strings.TrimSpace(title); t != "" {
		lines = append([]string{"“" + t + "”"}, lines...)
	}
	return removalPlan{
		Verb:       "Unlink",
		Title:      fmt.Sprintf("source %d from observation %d", src, obs),
		Confirm:    strconv.FormatInt(src, 10),
		Equivalent: fmt.Sprintf("kb unlink source %d %d", obs, src),
		Lines:      lines,
		Apply:      func() error { return kb.UnlinkObservationSource(obs, src) },
	}
}

// unlinkConceptFromProject opens the gate for cutting the selected concept's link
// to the project whose Concepts tab this is.
func (m *tuiModel) unlinkConceptFromProject() (tea.Model, tea.Cmd) {
	it, ok := m.conceptList.SelectedItem().(conceptItem)
	if !ok || m.selectedProject == nil {
		return m, nil
	}
	return m.beginRemoval(planProjectConceptUnlink(m.kb, m.selectedProject.Name, it.c.Name), nil)
}

// openUnlink lists the links of the selected observation, to pick one to cut.
func (m *tuiModel) openUnlink() (tea.Model, tea.Cmd) {
	it, ok := m.observationList.SelectedItem().(observationItem)
	if !ok {
		return m, nil
	}
	concepts, err := m.kb.ObservationConcepts(it.o.ID)
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	sources, err := m.kb.ObservationSources(it.o.ID)
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	var items []list.Item
	for _, c := range concepts {
		items = append(items, unlinkItem{concept: c.Name})
	}
	for _, s := range sources {
		items = append(items, unlinkItem{source: s.ID, title: s.Title})
	}
	if len(items) == 0 {
		m.notice = fmt.Sprintf("observation %d has no concept or source links", it.o.ID)
		return m, nil
	}
	m.unlinkObs = it.o.ID
	m.unlinkList = newBrowserList(m.isDark, items, fmt.Sprintf("Unlink — observation %d", it.o.ID), m.cols()-4, m.listHeight())
	m.clearFilter(viewUnlink)
	m.setState(viewUnlink)
	return m, nil
}

// updateUnlink handles a key in the link picker: Enter opens the gate for the
// selected link, q goes back to the observations.
func (m *tuiModel) updateUnlink(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.setState(viewObservations)
		return m, nil
	case "esc":
		return m, nil
	case "enter":
		it, ok := m.unlinkList.SelectedItem().(unlinkItem)
		if !ok {
			return m, nil
		}
		var p removalPlan
		if it.source != 0 {
			p = planObservationSourceUnlink(m.kb, m.unlinkObs, it.source, it.title)
		} else {
			p = planObservationConceptUnlink(m.kb, m.unlinkObs, it.concept)
		}
		model, cmd := m.beginRemoval(p, nil)
		// Back from the gate, or once it is done, is the observations the picker
		// came from: its links have just changed, so the picker would be stale.
		m.removeBack = viewObservations
		return model, cmd
	}
	var cmd tea.Cmd
	m.unlinkList, cmd = m.unlinkList.Update(msg)
	return m, cmd
}
