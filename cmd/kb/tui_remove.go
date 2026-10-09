package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	knowledge "github.com/rsdoiel/knowledge"
)

/** removalPlan is what the removing gate shows and does for one selected thing
 * (knowledge DR-0067): what will go and what points at it, what must be typed to
 * confirm, the command-line equivalent, and the delete itself. A plan with a
 * Refusal is not a gate: the thing cannot be deleted from here, and the reason is
 * shown instead.
 *
 * Fields:
 *   Title      (string)   — what is being deleted, for the screen ("project \"empty\"").
 *   Confirm    (string)   — the exact text the person must type.
 *   Lines      ([]string) — what will be removed and what points at it.
 *   Refusal    (string)   — why it cannot be deleted; empty when it can.
 *   Equivalent (string)   — the command that does the same.
 *   Apply      (func() error) — the delete, through the same library call the command uses.
 */
type removalPlan struct {
	Title      string
	Confirm    string
	Lines      []string
	Refusal    string
	Equivalent string
	Apply      func() error
}

// maxNamedPointers is how many records that point at a record the gate names.
const maxNamedPointers = 8

// planProjectDelete plans `kb project delete NAME`. A project that owns any
// observation, record or document is refused, as the command refuses it; one with
// only concept links is deleted with those links, which the plan lists.
func planProjectDelete(kb *knowledge.KnowledgeBase, name string) (removalPlan, error) {
	u, err := kb.ProjectUsage(name)
	if err != nil {
		return removalPlan{}, err
	}
	p := removalPlan{Title: fmt.Sprintf("project %q", name), Confirm: name, Equivalent: "kb project delete " + name}
	if u.Owned() > 0 {
		p.Refusal = fmt.Sprintf("project %q owns %s; nothing was deleted (delete or move its observations, records and documents first; a project delete never removes content)", name, ownedSummary(u))
		return p, nil
	}
	if u.Concepts > 0 {
		p.Lines = []string{fmt.Sprintf("its %s will be removed (the concepts stay)", plural(u.Concepts, "concept link(s)"))}
		p.Equivalent += " --force"
	} else {
		p.Lines = []string{"it owns nothing and is linked to nothing"}
	}
	force := u.Concepts > 0
	p.Apply = func() error { _, err := kb.DeleteProject(name, force); return err }
	return p, nil
}

// planObservationDelete plans `kb observation delete ID`: its concept and source
// links and its supersessions go with it.
func planObservationDelete(kb *knowledge.KnowledgeBase, id int64, body string) (removalPlan, error) {
	u, err := kb.ObservationUsage(id)
	if err != nil {
		return removalPlan{}, err
	}
	p := removalPlan{Title: fmt.Sprintf("observation %d", id), Confirm: strconv.FormatInt(id, 10), Equivalent: fmt.Sprintf("kb observation delete %d", id)}
	if body = strings.TrimSpace(body); body != "" {
		if r := []rune(body); len(r) > 60 {
			body = string(r[:60]) + "…"
		}
		p.Lines = append(p.Lines, "“"+body+"”")
	}
	if u.Total() > 0 {
		p.Lines = append(p.Lines, fmt.Sprintf("its %s will be removed", observationLinks(u)))
		p.Equivalent += " --force"
	}
	force := u.Total() > 0
	p.Apply = func() error { _, err := kb.DeleteObservation(id, force); return err }
	return p, nil
}

// planConceptDelete plans `kb concept delete NAME`: the concept is unlinked from
// everything and deleted.
func planConceptDelete(kb *knowledge.KnowledgeBase, name string) (removalPlan, error) {
	u, err := kb.ConceptUsage(name)
	if err != nil {
		return removalPlan{}, err
	}
	p := removalPlan{Title: fmt.Sprintf("concept %q", name), Confirm: name, Equivalent: "kb concept delete " + name}
	linked := conceptLinkSummary(u)
	if linked != "" {
		p.Lines = append(p.Lines, fmt.Sprintf("it is linked to %s; the links will be removed", linked))
		p.Equivalent += " --force"
	} else {
		p.Lines = append(p.Lines, "it is linked to nothing")
	}
	if u.Records > 0 || u.DocumentSections > 0 {
		p.Lines = append(p.Lines, "a record or document that still contains [[wikilink]] or lists it in tags or keywords recreates it when that file is next ingested after it changes")
	}
	force := linked != ""
	p.Apply = func() error { _, err := kb.DeleteConcept(name, force); return err }
	return p, nil
}

// planRecordDelete plans `kb record delete REF`. A record row can be dropped only
// when its file is gone, and the gate says which records point at it.
func planRecordDelete(kb *knowledge.KnowledgeBase, rec *knowledge.Record, ref string) (removalPlan, error) {
	p := removalPlan{Title: "record " + ref, Confirm: ref, Equivalent: "kb record delete " + ref}
	path, err := resolveWithinRoot(recordRoot(kb, recordFlags{}), rec.Path)
	if err != nil {
		return removalPlan{}, err
	}
	if _, statErr := os.Stat(path); statErr == nil {
		p.Refusal = fmt.Sprintf("%s's file still exists (%s); nothing was deleted: delete the file first, or retire the record with kb record set-status %s cancelled", ref, rec.Path, ref)
		return p, nil
	}
	u, err := kb.RecordUsage(rec.ID)
	if err != nil {
		return removalPlan{}, err
	}
	names := projectNames(kb)
	rels, err := kb.RelationsFor(rec.ID)
	if err != nil {
		return removalPlan{}, err
	}
	for i, rel := range rels {
		if i == maxNamedPointers {
			p.Lines = append(p.Lines, fmt.Sprintf("and %d more", len(rels)-maxNamedPointers))
			break
		}
		other, err := kb.RecordByID(rel.RecordID)
		if err != nil || other == nil {
			continue
		}
		o := refOf(*other, names).String()
		switch rel.Relationship {
		case "superseded_by":
			p.Lines = append(p.Lines, o+" supersedes it")
		case "supersedes":
			p.Lines = append(p.Lines, "it supersedes "+o)
		default:
			p.Lines = append(p.Lines, o+" relates to it")
		}
	}
	p.Lines = append(p.Lines, fmt.Sprintf("its database row will be dropped, with %s and %s (its file is already gone)",
		plural(u.Concepts, "concept link(s)"), plural(u.RelationsFrom+u.RelationsTo, "relation(s)")))
	p.Apply = func() error { _, err := kb.DeleteRecord(rec.ID); return err }
	return p, nil
}

// beginRemoval opens the gate for a plan, or shows why there is none.
func (m *tuiModel) beginRemoval(p removalPlan, err error) (tea.Model, tea.Cmd) {
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	if p.Refusal != "" {
		m.notice = p.Refusal
		return m, nil
	}
	m.removal = &p
	m.removeInput = newTypedConfirm(p.Confirm)
	m.removeBack = m.state
	m.setState(viewRemove)
	return m, nil
}

// removeSelected plans the delete of whatever is selected on the current screen.
func (m *tuiModel) removeSelected() (tea.Model, tea.Cmd) {
	switch m.state {
	case viewProjects:
		if it, ok := m.projectList.SelectedItem().(projectItem); ok {
			p, err := planProjectDelete(m.kb, it.p.Name)
			return m.beginRemoval(p, err)
		}
	case viewObservations:
		if it, ok := m.observationList.SelectedItem().(observationItem); ok {
			p, err := planObservationDelete(m.kb, it.o.ID, it.o.Body)
			return m.beginRemoval(p, err)
		}
	case viewConcepts:
		if it, ok := m.conceptList.SelectedItem().(conceptItem); ok {
			p, err := planConceptDelete(m.kb, it.c.Name)
			return m.beginRemoval(p, err)
		}
	case viewRecords, viewRecordScope:
		ref, ok := m.selectedRecordRef()
		if !ok {
			return m, nil
		}
		rec, err := resolveRecordForWrite(m.kb, ref, recordFlags{})
		if err != nil {
			return m.beginRemoval(removalPlan{}, err)
		}
		p, err := planRecordDelete(m.kb, rec, ref)
		return m.beginRemoval(p, err)
	}
	return m, nil
}

// updateRemove hands a key to the typed confirmation. Every printable key is text
// here, q included; Esc cancels and Enter deletes only on an exact match.
func (m *tuiModel) updateRemove(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.removeInput.Update(msg)
	switch {
	case m.removeInput.cancelled:
		m.setState(m.removeBack)
		m.notice = fmt.Sprintf("cancelled; %s is unchanged", m.removal.Title)
	case m.removeInput.confirmed:
		return m.finishRemoval()
	}
	return m, nil
}

// finishRemoval does the delete through the library call the command uses, then
// refreshes the screen it came from and says what was done and the command that
// does the same.
func (m *tuiModel) finishRemoval() (tea.Model, tea.Cmd) {
	p := m.removal
	m.setState(m.removeBack)
	if err := p.Apply(); err != nil {
		m.notice = err.Error()
		return m, nil
	}
	m.reloadAfterRemoval()
	m.notice = fmt.Sprintf("✓ deleted %s\nequivalent:  %s\n%s", p.Title, p.Equivalent, "(a delete is local: "+localDeleteNote+")")
	return m, nil
}

// reloadAfterRemoval rebuilds the list the thing was deleted from and the counts
// that mention it, keeping the cursor near where it was.
func (m *tuiModel) reloadAfterRemoval() {
	if recs, err := m.kb.ListRecords(knowledge.RecordFilter{}); err == nil {
		m.recordCount = len(recs)
	}
	keep := func(l *list.Model, at int) {
		if n := len(l.Items()); n > 0 {
			if at >= n {
				at = n - 1
			}
			l.Select(at)
		}
	}
	switch m.state {
	case viewProjects:
		at := m.projectList.Index()
		projects, err := m.kb.Projects()
		if err != nil {
			m.setErr(err)
			return
		}
		items := make([]list.Item, len(projects))
		for i, p := range projects {
			items[i] = projectItem{p}
		}
		m.projectList.SetItems(items)
		keep(&m.projectList, at)
	case viewObservations:
		at := m.observationList.Index()
		if err := m.loadObservations(); err != nil {
			m.setErr(err)
			return
		}
		keep(&m.observationList, at)
	case viewConcepts:
		at := m.conceptList.Index()
		if err := m.loadConcepts(); err != nil {
			m.setErr(err)
			return
		}
		keep(&m.conceptList, at)
	case viewRecords:
		at := m.recordList.Index()
		if err := m.loadRecords(); err != nil {
			m.setErr(err)
			return
		}
		keep(&m.recordList, at)
	case viewRecordScope:
		at := m.scopeList.Index()
		if err := m.loadRecordScope(); err != nil {
			m.setErr(err)
			return
		}
		keep(&m.scopeList, at)
	}
}

// removeScreen is the gate's screen: the plan, then the typed confirmation.
func (m *tuiModel) removeScreen() (string, []string) {
	body := []string{"Delete " + m.removal.Title, ""}
	if len(m.removal.Lines) > 0 {
		body = append(body, "This will remove, or is pointed at by:")
		for _, l := range m.removal.Lines {
			body = append(body, "  "+l)
		}
		body = append(body, "")
	}
	body = append(body, "A delete is local: "+localDeleteNote+".", "")
	body = append(body, strings.Split(m.removeInput.View(), "\n")...)
	return "Delete — " + m.removal.Title, body
}
