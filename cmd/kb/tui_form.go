package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	knowledge "github.com/rsdoiel/knowledge"
)

/** formField is one question of an additive form (knowledge DR-0067): a line of
 * text or a choice made with one key.
 *
 * Fields:
 *   label    (string)            — what is asked, shown above the field.
 *   help     (string)            — a line saying what a good answer is.
 *   choices  ([]string)          — set for a choice field.
 *   keys     (map[string]rune)   — the key for a choice that is not its first letter.
 *   required (bool)              — whether an empty answer is refused.
 *   prefill  (string)            — the field's starting text.
 *   check    (func(string) error) — a check on a typed answer, shown in the field.
 */
type formField struct {
	label    string
	help     string
	choices  []string
	keys     map[string]rune
	required bool
	prefill  string
	check    func(string) error
}

/** formFlow is an additive write in progress: the fields are asked one at a time,
 * what has been entered stays on the screen, and the command that does the same
 * is shown with the confirmation. The write is the command's own function, so its
 * rules and errors are the command's. Esc cancels the whole form; nothing is
 * written until y.
 *
 * Fields:
 *   title   (string)        — what is being created, for the screen.
 *   fields  ([]formField)   — the questions, in order.
 *   values  ([]string)      — the answers so far.
 *   step    (int)           — the field being asked; len(fields) is the confirmation.
 *   chooser (*chooserModel) — set while a choice field is asked.
 *   edit    (*editModel)    — set while a text field is asked.
 *   confirm (*confirmModel) — the confirmation.
 *   back    (viewState)     — the screen to return to.
 *   command (func([]string) string)        — the equivalent command line.
 *   apply   (func([]string) (string, error)) — the write; returns what to say it did.
 *   after   (string)        — a line added to the success notice.
 *   context ([]string)      — facts already known, shown above the questions.
 */
type formFlow struct {
	title   string
	fields  []formField
	values  []string
	step    int
	chooser *chooserModel
	edit    *editModel
	confirm *confirmModel
	back    viewState
	command func([]string) string
	apply   func([]string) (string, error)
	after   string
	context []string // facts already known, shown above the questions ("project: alpha")
}

// beginForm opens a form on the current screen at its first field.
func (m *tuiModel) beginForm(f *formFlow) (tea.Model, tea.Cmd) {
	f.back = m.state
	f.values = make([]string, len(f.fields))
	m.form = f
	m.enterField(0)
	m.setState(viewForm)
	return m, nil
}

// enterField sets up the editor or the chooser for field i, or the confirmation.
func (m *tuiModel) enterField(i int) {
	f := m.form
	f.step, f.chooser, f.edit = i, nil, nil
	if i >= len(f.fields) {
		f.confirm = newConfirm("Create it?")
		return
	}
	fl := f.fields[i]
	if len(fl.choices) > 0 {
		c := newChooser(fl.label, "", fl.choices)
		c.keys, c.prompt = fl.keys, fl.label+" ->"
		f.chooser = c
		return
	}
	e := newEdit(fl.prefill, !fl.required, m.cols()-8)
	e.allowSame = true
	f.edit = e
}

// updateForm handles a key in a form.
func (m *tuiModel) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.form
	cancel := func() (tea.Model, tea.Cmd) {
		m.setState(f.back)
		m.notice = fmt.Sprintf("cancelled; nothing was created (%s)", f.title)
		return m, nil
	}
	switch {
	case f.step >= len(f.fields): // the confirmation
		for _, k := range keysOf(msg) {
			if f.confirm.step(k) {
				if f.confirm.cancelled {
					return cancel()
				}
				return m.finishForm()
			}
		}
	case f.edit != nil:
		f.edit.Update(msg)
		switch {
		case f.edit.cancelled:
			return cancel()
		case f.edit.submitted:
			v := f.edit.value()
			if check := f.fields[f.step].check; check != nil && v != "" {
				if err := check(v); err != nil {
					f.edit.submitted = false
					f.edit.message = err.Error()
					return m, nil
				}
			}
			f.values[f.step] = strings.TrimRight(v, "\n")
			m.enterField(f.step + 1)
		}
	case f.chooser != nil:
		for _, k := range keysOf(msg) {
			if f.chooser.step(k) {
				if f.chooser.cancelled {
					return cancel()
				}
				f.values[f.step] = f.chooser.chosen
				m.enterField(f.step + 1)
				return m, nil
			}
		}
	}
	return m, nil
}

// finishForm makes the write through the command's function and says what was done
// and the command that does the same. A refusal is the command's, as a notice.
func (m *tuiModel) finishForm() (tea.Model, tea.Cmd) {
	f := m.form
	m.setState(f.back)
	done, err := f.apply(f.values)
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	m.reloadAfterRemoval()
	if projects, err := m.kb.Projects(); err == nil {
		items := make([]list.Item, len(projects))
		for i, p := range projects {
			items[i] = projectItem{p}
		}
		at := m.projectList.Index()
		m.projectList.SetItems(items)
		if at < len(items) {
			m.projectList.Select(at)
		}
	}
	m.notice = "✓ " + done + "\nequivalent:  " + f.command(f.values)
	if f.after != "" {
		m.notice += "\n" + f.after
	}
	return m, nil
}

// formScreen is the form's screen: the answers so far, then the question or the
// confirmation.
func (m *tuiModel) formScreen() (string, []string) {
	f := m.form
	body := append([]string(nil), f.context...)
	wrap := func(s string) []string {
		return strings.Split(lipgloss.NewStyle().Width(m.cols()-4).Render(s), "\n")
	}
	for i := 0; i < f.step && i < len(f.fields); i++ {
		v := f.values[i]
		if v == "" {
			v = "(none)"
		}
		body = append(body, f.fields[i].label+": "+strings.ReplaceAll(v, "\n", " ⏎ "))
	}
	if f.step >= len(f.fields) {
		body = append(body, "", "This is the same as:")
		body = append(body, wrap("  "+f.command(f.values))...)
		body = append(body, "", f.confirm.View())
		return "New — " + f.title, body
	}
	if len(body) > 0 {
		body = append(body, "")
	}
	fl := f.fields[f.step]
	body = append(body, fl.label+fieldMark(fl))
	if fl.help != "" {
		body = append(body, wrap(fl.help)...)
	}
	body = append(body, "")
	if f.chooser != nil {
		body = append(body, wrap(f.chooser.View())...)
	} else {
		body = append(body, strings.Split(f.edit.View(), "\n")...)
	}
	return "New — " + f.title, body
}

// fieldMark marks a field that may be left empty.
func fieldMark(fl formField) string {
	if len(fl.choices) == 0 && !fl.required {
		return " (optional)"
	}
	return ""
}

// formLegend names the keys of the current step.
func (m *tuiModel) formLegend() string {
	f := m.form
	switch {
	case f.step >= len(f.fields):
		return "y create   n q Esc cancel   Ctrl-C quit"
	case f.edit != nil:
		return "Enter accept   Ctrl-J newline   Esc cancel the form   Ctrl-C quit"
	}
	return "press a key to pick   q Esc cancel the form   Ctrl-C quit"
}

// ─── the five forms ──────────────────────────────────────────────────────────

func notDash(what string) func(string) error {
	return func(v string) error {
		if strings.HasPrefix(v, "-") {
			return fmt.Errorf("%s cannot start with a dash", what)
		}
		return nil
	}
}

// newProjectForm: kb project add NAME [DESCRIPTION].
func (m *tuiModel) newProjectForm() *formFlow {
	return &formFlow{
		title: "project",
		fields: []formField{
			{label: "name", help: "the project's name, as it will be typed on the command line", required: true, check: notDash("a project name")},
			{label: "description", help: "one line saying what it is"},
		},
		command: func(v []string) string {
			c := "kb project add " + shellWord(v[0])
			if v[1] != "" {
				c += " " + shellWord(v[1])
			}
			return c
		},
		apply: func(v []string) (string, error) {
			args := []string{"add", v[0]}
			if v[1] != "" {
				args = append(args, v[1])
			}
			if err := cmdProject(m.kb, nil, false, args, &bytes.Buffer{}); err != nil {
				return "", err
			}
			return fmt.Sprintf("project %q added", v[0]), nil
		},
	}
}

// observationKinds are the kinds and the key for each (question takes u, since q
// cancels).
var observationKeys = map[string]rune{"note": 'n', "finding": 'f', "decision": 'd', "question": 'u', "hypothesis": 'h'}

// newObservationForm: kb observation add --project P KIND BODY [--source-doi DOI].
// The project is asked only when it is not already known.
func (m *tuiModel) newObservationForm(project string) *formFlow {
	var fields []formField
	known := project != ""
	if !known {
		prefill := m.recordScopeName()
		if prefill == "all scopes" {
			prefill = ""
		}
		fields = append(fields, formField{label: "project", help: "the project the observation belongs to", required: true, prefill: prefill,
			check: func(v string) error {
				if p, err := m.kb.ProjectByName(strings.TrimSpace(v)); err != nil || p == nil {
					return fmt.Errorf("no project %q: see the project list", v)
				}
				return nil
			}})
	}
	fields = append(fields,
		formField{label: "kind", choices: knowledge.ValidObservationKinds, keys: observationKeys},
		formField{label: "text", help: "the observation itself; Ctrl-J starts a new line", required: true},
		formField{label: "source DOI", help: "the paper it was taken from, as 10.NNNN/suffix"},
	)
	offset := 0
	if !known {
		offset = 1
	}
	proj := func(v []string) string {
		if known {
			return project
		}
		return strings.TrimSpace(v[0])
	}
	var context []string
	if known {
		context = []string{"project: " + project}
	}
	return &formFlow{
		title:   "observation",
		context: context,
		fields:  fields,
		command: func(v []string) string {
			c := "kb observation add --project " + shellWord(proj(v))
			if v[offset+2] != "" {
				c += " --source-doi " + shellWord(v[offset+2])
			}
			return c + " " + v[offset] + " " + shellWord(v[offset+1])
		},
		apply: func(v []string) (string, error) {
			args := []string{"add", "--project", proj(v)}
			if v[offset+2] != "" {
				args = append(args, "--source-doi", v[offset+2])
			}
			args = append(args, v[offset], v[offset+1])
			var out bytes.Buffer
			if err := cmdObservation(m.kb, nil, false, args, &out); err != nil {
				return "", err
			}
			return fmt.Sprintf("%s added to project %q", v[offset], proj(v)), nil
		},
	}
}

// newConceptForm: kb concept add NAME [DESCRIPTION].
func (m *tuiModel) newConceptForm() *formFlow {
	return &formFlow{
		title: "concept",
		fields: []formField{
			{label: "name", help: "the concept's name; matching is not case-sensitive", required: true, check: notDash("a concept name")},
			{label: "description", help: "one line saying what it means"},
		},
		command: func(v []string) string {
			c := "kb concept add " + shellWord(v[0])
			if v[1] != "" {
				c += " " + shellWord(v[1])
			}
			return c
		},
		apply: func(v []string) (string, error) {
			args := []string{"add", v[0]}
			if v[1] != "" {
				args = append(args, v[1])
			}
			if err := cmdConcept(m.kb, nil, false, args, &bytes.Buffer{}); err != nil {
				return "", err
			}
			return fmt.Sprintf("concept %q added", v[0]), nil
		},
	}
}

// newSourceForm: kb source add TITLE [--authors A] [--published DATE] [--url U]
// [--doi D]. --publisher, --rights and --version are left to the command line.
func (m *tuiModel) newSourceForm() *formFlow {
	flags := []string{"", "--authors", "--published", "--url", "--doi"}
	return &formFlow{
		title: "source",
		fields: []formField{
			{label: "title", help: "the work's title", required: true},
			{label: "authors", help: "who wrote it"},
			{label: "published", help: "YYYY, YYYY-MM or YYYY-MM-DD"},
			{label: "url", help: "an absolute URL with a scheme"},
			{label: "DOI", help: "the bare form, 10.NNNN/suffix"},
		},
		command: func(v []string) string {
			c := "kb source add " + shellWord(v[0])
			for i := 1; i < len(v); i++ {
				if v[i] != "" {
					c += " " + flags[i] + " " + shellWord(v[i])
				}
			}
			return c
		},
		apply: func(v []string) (string, error) {
			args := []string{"add", v[0]}
			for i := 1; i < len(v); i++ {
				if v[i] != "" {
					args = append(args, flags[i], v[i])
				}
			}
			if err := cmdSource(m.kb, nil, false, args, &bytes.Buffer{}); err != nil {
				return "", err
			}
			return fmt.Sprintf("source %q added", v[0]), nil
		},
	}
}

// triggerKeys and recordKindKeys are the keys that pick a trigger and a kind.
var triggerKeys = map[string]rune{
	"design": 'd', "plan-review": 'p', "implementation": 'i', "live-test": 'l',
	"release-review": 'v', "request": 'r', "external": 'x',
}
var recordKindKeys = map[string]rune{"decision": 'd', "correction": 'c', "refinement": 'r'}

// newRecordForm: kb record new (--project P | --workspace) --title T --trigger G
// --kind K. The record is written proposed; it reaches the database at the next
// ingest, as with the command.
func (m *tuiModel) newRecordForm(scope string) *formFlow {
	return &formFlow{
		title: "decision record",
		fields: []formField{
			{label: "scope", help: "a project name, or workspace for the workspace tier", required: true, prefill: scope,
				check: func(v string) error {
					v = strings.TrimSpace(v)
					if strings.EqualFold(v, "workspace") {
						return nil
					}
					if p, err := m.kb.ProjectByName(v); err != nil || p == nil {
						return fmt.Errorf("no project %q: give a project name, or workspace", v)
					}
					return nil
				}},
			{label: "title", help: "what was decided, in a line", required: true},
			{label: "trigger", choices: knowledge.RecordTriggers, keys: triggerKeys},
			{label: "kind", choices: knowledge.RecordKinds, keys: recordKindKeys},
		},
		after: "The record is written proposed. Run kb ingest on its directory to bring it into the database.",
		command: func(v []string) string {
			scopeFlag := "--project " + shellWord(strings.TrimSpace(v[0]))
			if strings.EqualFold(strings.TrimSpace(v[0]), "workspace") {
				scopeFlag = "--workspace"
			}
			return "kb record new " + scopeFlag + " --title " + shellWord(v[1]) + " --trigger " + v[2] + " --kind " + v[3]
		},
		apply: func(v []string) (string, error) {
			f := recordFlags{title: v[1], trigger: v[2], kind: v[3]}
			if strings.EqualFold(strings.TrimSpace(v[0]), "workspace") {
				f.workspace = true
			} else {
				f.project = strings.TrimSpace(v[0])
			}
			var out bytes.Buffer
			if err := recordNew(m.kb, false, f, &out); err != nil {
				return "", err
			}
			return strings.TrimSpace(out.String()), nil
		},
	}
}
