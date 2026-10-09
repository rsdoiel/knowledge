package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

/** planFlow is a plan-then-apply write in progress (knowledge DR-0068, DR-0067
 * class plan_apply): the command's own dry run, shown and scrollable, and an
 * apply that is the same command without --dry-run.
 *
 * Fields:
 *   title (string)             — the screen title and what a notice calls it
 *   text (string)              — the dry run's output
 *   equivalent (string)        — the command that applies it, shown with the result
 *   apply (func() (string, error)) — runs the write; returns the command's output
 *   back (viewState)           — the screen to return to
 *
 * Example:
 *   m.beginPlan(&planFlow{title: "Ingest", text: out, equivalent: "kb ingest DIR",
 *       apply: func() (string, error) { return run(false) }}, m.state)
 */
type planFlow struct {
	title      string
	text       string
	equivalent string
	apply      func() (string, error)
	back       viewState
}

// beginPlan shows a plan. Nothing has been written yet.
func (m *tuiModel) beginPlan(p *planFlow, back viewState) {
	p.back = back
	m.plan = p
	h := m.bodyHeight() - 3
	if m.bodyHeight() <= 0 {
		h = 17
	}
	if h < 3 {
		h = 3
	}
	vp := viewport.New(m.cols()-4, h)
	vp.SetContent(lipgloss.NewStyle().Width(m.cols() - 4).Render(strings.TrimRight(p.text, "\n")))
	m.planView = vp
	m.setState(viewPlan)
}

// updatePlan handles a key on a plan. y applies; n, q and Esc leave with nothing
// done; the scroll keys move the text; everything else, Enter included, is
// ignored so that a habitual key cannot write.
func (m *tuiModel) updatePlan(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	for _, k := range keysOf(msg) {
		switch k.String() {
		case "y":
			return m.applyPlan()
		case "n", "q", "esc":
			m.setState(m.plan.back)
			m.notice = fmt.Sprintf("cancelled; nothing was applied (%s)", m.plan.title)
			return m, nil
		case "j", "k", "up", "down", " ", "pgup", "pgdown":
			m.planView, _ = m.planView.Update(k)
		}
	}
	return m, nil
}

// applyPlan runs the write and shows what the command said and the command that
// does the same. A failure that produced output is shown with the output.
func (m *tuiModel) applyPlan() (tea.Model, tea.Cmd) {
	p := m.plan
	m.setState(p.back)
	out, err := p.apply()
	if err != nil && strings.TrimSpace(out) == "" {
		m.notice = err.Error()
		return m, nil
	}
	m.reloadAfterRemoval()
	text := strings.TrimRight(out, "\n")
	if err != nil {
		text += "\n\nerror: " + err.Error()
	}
	m.reviewRef, m.reviewBack = p.title+" — done", p.back
	m.reviewRaw = []byte(text + "\n\nequivalent:  " + p.equivalent)
	m.openViewer("result")
	return m, nil
}

// planScreen is the plan's screen: what it is, the command that applies it, then
// the dry run.
func (m *tuiModel) planScreen() (string, []string) {
	lines := []string{
		"dry run — nothing has been written",
		"y runs:  " + m.plan.equivalent,
		"",
	}
	return m.plan.title, append(lines, strings.Split(strings.TrimRight(m.planView.View(), "\n"), "\n")...)
}

// runCapture runs a command function with a buffer for its output.
func runCapture(run func(out *bytes.Buffer) error) (string, error) {
	var out bytes.Buffer
	err := run(&out)
	return out.String(), err
}

// planIngest plans `kb ingest DIR` for the current Records scope.
func (m *tuiModel) planIngest() (tea.Model, tea.Cmd) {
	dir, rel, _, err := m.decisionsTarget()
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	text, err := runCapture(func(out *bytes.Buffer) error {
		return cmdIngest(m.kb, nil, false, []string{dir, "--dry-run"}, out)
	})
	if err != nil && strings.TrimSpace(text) == "" {
		m.notice = err.Error()
		return m, nil
	}
	if err != nil {
		text += "\nerror: " + err.Error()
	}
	m.beginPlan(&planFlow{
		title:      "Ingest — " + rel,
		text:       text,
		equivalent: "kb ingest " + rel,
		apply: func() (string, error) {
			return runCapture(func(out *bytes.Buffer) error {
				return cmdIngest(m.kb, nil, false, []string{dir}, out)
			})
		},
	}, m.state)
	return m, nil
}

// withProject builds a plan for a project: the scope's, or, with every scope,
// the one the person names first.
func (m *tuiModel) withProject(heading string, build func(project string) (*planFlow, error)) (tea.Model, tea.Cmd) {
	if name := m.recordScopeName(); name != "all scopes" {
		p, err := build(name)
		if err != nil {
			m.notice = err.Error()
			return m, nil
		}
		m.beginPlan(p, m.state)
		return m, nil
	}
	f := &formFlow{
		title:   heading,
		heading: heading,
		fields:  []formField{m.projectField("the project to run it on")},
		plan: func(v []string) (*planFlow, error) {
			return build(strings.TrimSpace(v[0]))
		},
	}
	return m.beginForm(f)
}

// planRecordFuzzyTag plans `kb record fuzzy-tag --project P`; its apply is --write.
func (m *tuiModel) planRecordFuzzyTag() (tea.Model, tea.Cmd) {
	return m.withProject("Fuzzy-tag records", func(project string) (*planFlow, error) {
		text, err := runCapture(func(out *bytes.Buffer) error {
			return recordFuzzyTag(m.kb, false, recordFlags{project: project}, out)
		})
		if err != nil {
			return nil, err
		}
		return &planFlow{
			title:      "Fuzzy-tag records — " + project,
			text:       text,
			equivalent: "kb record fuzzy-tag --project " + shellWord(project) + " --write",
			apply: func() (string, error) {
				return runCapture(func(out *bytes.Buffer) error {
					return recordFuzzyTag(m.kb, false, recordFlags{project: project, write: true}, out)
				})
			},
		}, nil
	})
}

// planDocumentEdit plans `kb document tag` or `kb document fuzzy-tag` for a project.
func (m *tuiModel) planDocumentEdit(verb string) (tea.Model, tea.Cmd) {
	run := cmdDocumentTag
	if verb == "fuzzy-tag" {
		run = cmdDocumentFuzzyTag
	}
	return m.withProject("Document "+verb, func(project string) (*planFlow, error) {
		text, err := runCapture(func(out *bytes.Buffer) error {
			return run(m.kb, false, []string{"--project", project, "--dry-run"}, out)
		})
		if err != nil {
			return nil, err
		}
		return &planFlow{
			title:      "Document " + verb + " — " + project,
			text:       text,
			equivalent: "kb document " + verb + " --project " + shellWord(project),
			apply: func() (string, error) {
				return runCapture(func(out *bytes.Buffer) error {
					return run(m.kb, false, []string{"--project", project}, out)
				})
			},
		}, nil
	})
}

// planDocumentIngest asks for the path, the project and a title, then plans
// `kb document ingest PATH --project P`.
func (m *tuiModel) planDocumentIngest() (tea.Model, tea.Cmd) {
	pathField := formField{label: "path", help: "the document file, relative to where kb runs", required: true,
		check: func(v string) error {
			if strings.HasPrefix(v, "-") {
				return fmt.Errorf("a path cannot start with a dash")
			}
			if fi, err := os.Stat(v); err != nil || fi.IsDir() {
				return fmt.Errorf("%s is not a file", v)
			}
			return nil
		}}
	titleField := formField{label: "title", help: "its title; empty takes it from the document"}
	args := func(v []string, dry bool) []string {
		a := []string{v[0], "--project", strings.TrimSpace(v[1])}
		if v[2] != "" {
			a = append(a, "--title", v[2])
		}
		if dry {
			a = append(a, "--dry-run")
		}
		return a
	}
	f := &formFlow{
		title:   "Ingest a document",
		heading: "Ingest a document",
		fields:  []formField{pathField, m.projectField("the project the document belongs to"), titleField},
		plan: func(v []string) (*planFlow, error) {
			text, err := runCapture(func(out *bytes.Buffer) error {
				return cmdDocumentIngest(m.kb, false, args(v, true), out)
			})
			if err != nil {
				return nil, err
			}
			eq := "kb document ingest " + shellWord(v[0]) + " --project " + shellWord(strings.TrimSpace(v[1]))
			if v[2] != "" {
				eq += " --title " + shellWord(v[2])
			}
			return &planFlow{
				title:      "Ingest — " + v[0],
				text:       text,
				equivalent: eq,
				apply: func() (string, error) {
					return runCapture(func(out *bytes.Buffer) error {
						return cmdDocumentIngest(m.kb, false, args(v, false), out)
					})
				},
			}, nil
		},
	}
	return m.beginForm(f)
}

// projectField asks for an existing project, prefilled from the scope.
func (m *tuiModel) projectField(help string) formField {
	prefill := m.recordScopeName()
	if prefill == "all scopes" {
		prefill = ""
	}
	return formField{label: "project", help: help, required: true, prefill: prefill,
		check: func(v string) error {
			if p, err := m.kb.ProjectByName(strings.TrimSpace(v)); err != nil || p == nil {
				return fmt.Errorf("no project %q: see the project list", v)
			}
			return nil
		}}
}
