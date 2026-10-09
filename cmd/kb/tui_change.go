package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	knowledge "github.com/rsdoiel/knowledge"
)

// Phases of a change.
const (
	changePick    = iota // a key picks the new value, or text is typed
	changeConfirm        // y applies old to new
)

/** changeFlow is one changing write in progress (knowledge DR-0067): the item is
 * shown with its current value, the new value is picked with a key or typed, and
 * old to new is confirmed. The write is the command's own function, so its rules
 * and its errors are the command's. Nothing is written until y.
 *
 * Fields:
 *   subject    (string)           — what is being changed, for the screen.
 *   old        (string)           — its current value.
 *   chooser    (*chooserModel)    — set when the new value is picked from options.
 *   edit       (*editModel)       — set when the new value is typed.
 *   confirm    (*confirmModel)    — the confirmation, built once there is a value.
 *   value      (string)           — the new value.
 *   phase      (int)              — changePick or changeConfirm.
 *   back       (viewState)        — the screen to return to.
 *   describe   (func(string) string) — what the success line says.
 *   equivalent (func(string) string) — the command that does the same.
 *   apply      (func(string) error)  — the write.
 *   note       (string)           — what the write really does, when that is not obvious.
 *   validate   (func)             — checks a typed value before the confirmation and describes the change.
 *   summary    ([]string)         — validate's description, shown on the confirmation.
 */
type changeFlow struct {
	subject    string
	old        string
	chooser    *chooserModel
	edit       *editModel
	confirm    *confirmModel
	value      string
	phase      int
	back       viewState
	describe   func(string) string
	equivalent func(string) string
	apply      func(string) error
	note       string // explains what the write really does, shown on the confirmation
	// validate is asked when a typed value is accepted, before the confirmation. It
	// returns the lines that describe the change, or the reason it cannot be made,
	// which is shown in the field so the person can correct it.
	validate func(string) ([]string, error)
	summary  []string // the lines validate returned, shown instead of was/now
}

// shellWord quotes a value for the equivalent command line: bare when it is a plain
// word, in double quotes when it has spaces, and $'...' when it has newlines.
func shellWord(s string) string {
	if regexp.MustCompile(`^[A-Za-z0-9_./:@%+=,-]+$`).MatchString(s) {
		return s
	}
	if strings.ContainsAny(s, "\n\r") {
		r := strings.NewReplacer(`\`, `\\`, "'", `\'`, "\n", `\n`, "\r", `\r`)
		return "$'" + r.Replace(s) + "'"
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`")
	return `"` + r.Replace(s) + `"`
}

// projectStatuses are a project's statuses and the key that picks each (concept
// and concluded both begin with c, so the second takes the l).
var projectStatuses = []string{"concept", "active", "paused", "concluded"}
var projectStatusKeys = map[string]rune{"concept": 'c', "active": 'a', "paused": 'p', "concluded": 'l'}

// beginChange opens a change flow on the current screen.
func (m *tuiModel) beginChange(f *changeFlow) (tea.Model, tea.Cmd) {
	f.back = m.state
	if f.chooser == nil && f.edit == nil {
		return m, nil
	}
	m.change = f
	m.setState(viewChange)
	return m, nil
}

// changeSelected starts the change for the selected item that the key names:
// "status" and "describe" and "rename" on the project list, "edit" on an
// observation (its text) or a concept (its name).
func (m *tuiModel) changeSelected(what string) (tea.Model, tea.Cmd) {
	switch m.state {
	case viewProjects:
		it, ok := m.projectList.SelectedItem().(projectItem)
		if !ok {
			return m, nil
		}
		return m.beginChange(m.projectChange(it.p, what))
	case viewObservations:
		it, ok := m.observationList.SelectedItem().(observationItem)
		if !ok {
			return m, nil
		}
		id := strconv.FormatInt(it.o.ID, 10)
		// observation update corrects by superseding (DR-0023): the original is kept
		// and a new observation is added. The screen says so, and reports its id.
		newID := "?"
		return m.beginChange(&changeFlow{
			subject: "observation " + id,
			old:     it.o.Body,
			edit:    newEdit(it.o.Body, false, m.cols()-8),
			note:    "This corrects it by superseding, as kb observation update does: a new observation is added and the original wording is kept.",
			describe: func(string) string {
				return "observation " + id + " corrected by observation " + newID + ", which supersedes it"
			},
			equivalent: func(v string) string { return "kb observation update " + id + " " + shellWord(v) },
			apply: func(v string) error {
				var out bytes.Buffer
				if err := cmdObservation(m.kb, nil, false, []string{"update", id, v}, &out); err != nil {
					return err
				}
				if sub := regexp.MustCompile(`id=(\d+)`).FindStringSubmatch(out.String()); sub != nil {
					newID = sub[1]
				}
				return nil
			},
		})
	case viewConcepts:
		it, ok := m.conceptList.SelectedItem().(conceptItem)
		if !ok {
			return m, nil
		}
		name := it.c.Name
		return m.beginChange(&changeFlow{
			subject:    fmt.Sprintf("concept %q", name),
			old:        name,
			edit:       newEdit(name, false, m.cols()-8),
			describe:   func(v string) string { return fmt.Sprintf("concept %q renamed to %q", name, v) },
			equivalent: func(v string) string { return "kb concept rename " + shellWord(name) + " " + shellWord(v) },
			apply: func(v string) error {
				return cmdConcept(m.kb, nil, false, []string{"rename", name, v}, &bytes.Buffer{})
			},
		})
	}
	return m, nil
}

// projectChange builds the flow for one of the project list's keys.
func (m *tuiModel) projectChange(p knowledge.Project, what string) *changeFlow {
	name := p.Name
	subject := fmt.Sprintf("project %q", name)
	run := func(args ...string) error {
		return cmdProject(m.kb, nil, false, args, &bytes.Buffer{})
	}
	switch what {
	case "status":
		var options []string
		for _, s := range projectStatuses {
			if s != p.Status {
				options = append(options, s)
			}
		}
		c := newChooser(subject, p.Status, options)
		c.keys = projectStatusKeys
		return &changeFlow{
			subject: subject, old: p.Status, chooser: c,
			describe:   func(v string) string { return fmt.Sprintf("%s status set to %s", subject, v) },
			equivalent: func(v string) string { return "kb project set-status " + shellWord(name) + " " + v },
			apply:      func(v string) error { return run("set-status", name, v) },
		}
	case "describe":
		return &changeFlow{
			subject: subject, old: p.Description, edit: newEdit(p.Description, true, m.cols()-8),
			describe:   func(string) string { return subject + " description updated" },
			equivalent: func(v string) string { return "kb project set-description " + shellWord(name) + " " + shellWord(v) },
			apply:      func(v string) error { return run("set-description", name, v) },
		}
	default: // rename
		return &changeFlow{
			subject: subject, old: name, edit: newEdit(name, false, m.cols()-8),
			describe:   func(v string) string { return fmt.Sprintf("%s renamed to %q", subject, v) },
			equivalent: func(v string) string { return "kb project rename " + shellWord(name) + " " + shellWord(v) },
			apply:      func(v string) error { return run("rename", name, v) },
		}
	}
}

// toConfirm moves a change to its confirmation once it has a new value.
func (f *changeFlow) toConfirm(value string) {
	f.value = value
	f.confirm = newConfirm(fmt.Sprintf("Change %s?", f.subject))
	f.phase = changeConfirm
}

// updateChange handles a key in a change: picking or typing, then y.
func (m *tuiModel) updateChange(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.change
	cancel := func() (tea.Model, tea.Cmd) {
		m.setState(f.back)
		m.notice = fmt.Sprintf("cancelled; %s is unchanged", f.subject)
		return m, nil
	}
	if f.phase == changePick {
		if f.edit != nil {
			f.edit.Update(msg)
			switch {
			case f.edit.cancelled:
				return cancel()
			case f.edit.submitted:
				if f.validate != nil {
					lines, err := f.validate(f.edit.value())
					if err != nil {
						f.edit.submitted = false
						f.edit.message = err.Error()
						return m, nil
					}
					f.summary = lines
				}
				f.toConfirm(f.edit.value())
			}
			return m, nil
		}
		for _, k := range keysOf(msg) {
			if f.chooser.step(k) {
				if f.chooser.cancelled {
					return cancel()
				}
				f.toConfirm(f.chooser.chosen)
				return m, nil
			}
		}
		return m, nil
	}
	for _, k := range keysOf(msg) {
		if f.confirm.step(k) {
			if f.confirm.cancelled {
				return cancel()
			}
			return m.finishChange()
		}
	}
	return m, nil
}

// finishChange makes the write through the command's function and says what was
// done and the command that does the same.
func (m *tuiModel) finishChange() (tea.Model, tea.Cmd) {
	f := m.change
	m.setState(f.back)
	if err := f.apply(f.value); err != nil {
		m.notice = err.Error()
		return m, nil
	}
	m.reloadAfterRemoval() // rebuilds the list the thing is shown in
	m.notice = fmt.Sprintf("✓ %s\nequivalent:  %s", f.describe(f.value), f.equivalent(f.value))
	return m, nil
}

// changeScreen is the change's screen: the item and its current value, then the
// chooser, the editor or the confirmation.
func (m *tuiModel) changeScreen() (string, []string) {
	f := m.change
	body := []string{f.subject, ""}
	// Text is wrapped to the window so a long description or note is not cut off.
	wrap := func(s string, indentBy int) []string {
		w := m.cols() - 4 - indentBy
		if w < 10 {
			w = 10
		}
		var out []string
		for _, l := range strings.Split(s, "\n") {
			for _, piece := range strings.Split(lipgloss.NewStyle().Width(w).Render(l), "\n") {
				out = append(out, strings.Repeat(" ", indentBy)+strings.TrimRight(piece, " "))
			}
		}
		return out
	}
	indent := func(s string) []string { return wrap(s, 4) }
	switch {
	case f.phase == changeConfirm && len(f.summary) > 0:
		for _, l := range f.summary {
			body = append(body, wrap(l, 0)...)
		}
		if f.note != "" {
			body = append(body, "")
			body = append(body, wrap(f.note, 0)...)
		}
		body = append(body, "", f.confirm.View())
	case f.phase == changeConfirm:
		body = append(body, "was:")
		body = append(body, indent(f.old)...)
		body = append(body, "now:")
		body = append(body, indent(f.value)...)
		if f.note != "" {
			body = append(body, "")
			body = append(body, wrap(f.note, 0)...)
		}
		body = append(body, "", f.confirm.View())
	case f.chooser != nil:
		body = append(body, f.chooser.View())
	default:
		body = append(body, "was:")
		body = append(body, indent(f.old)...)
		body = append(body, "", "new:")
		body = append(body, strings.Split(f.edit.View(), "\n")...)
	}
	return "Change — " + f.subject, body
}

// changeLegend names the keys of the current phase.
func (m *tuiModel) changeLegend() string {
	f := m.change
	switch {
	case f.phase == changeConfirm:
		return "y apply   n q Esc cancel   Ctrl-C quit"
	case f.edit != nil:
		return "Enter accept   Ctrl-J newline   Esc cancel   Ctrl-C quit"
	}
	return "press a key to pick   q Esc cancel   Ctrl-C quit"
}

// supersedeSelected starts a supersession with the selected record as the new one:
// the person types which record it replaces, and the field refuses what the command
// refuses before anything is confirmed.
func (m *tuiModel) supersedeSelected() (tea.Model, tea.Cmd) {
	ref, ok := m.selectedRecordRef()
	if !ok {
		return m, nil
	}
	prefix := ref[:strings.Index(ref, "/")+1] // the same scope, so a bare number completes it
	names := projectNames(m.kb)
	oldRef := "" // the replaced record's qualified reference, set when the field is accepted
	f := &changeFlow{
		subject: "record " + ref,
		old:     prefix,
		edit:    newEdit(prefix, false, m.cols()-8),
		note:    "Both records' files and the database are written. The replaced record's status becomes superseded (the command's --partial option, which leaves it, is not offered here).",
		validate: func(v string) ([]string, error) {
			newer, older, err := resolveSupersession(m.kb, ref, strings.TrimSpace(v), recordFlags{})
			if err != nil {
				return nil, err
			}
			root := recordRoot(m.kb, recordFlags{})
			for _, r := range []*knowledge.Record{newer, older} {
				if _, _, err := loadRecordFile(root, r); err != nil {
					return nil, err
				}
			}
			n, o := refOf(*newer, names).String(), refOf(*older, names).String()
			oldRef = o
			return []string{
				fmt.Sprintf("%s  %s", n, newer.Title),
				"supersedes",
				fmt.Sprintf("%s  %s  (%s)", o, older.Title, older.Status),
				"",
				fmt.Sprintf("%s becomes superseded, by %s.", o, n),
			}, nil
		},
		equivalent: func(string) string { return "kb record supersede " + ref + " " + oldRef },
		describe:   func(string) string { return ref + " supersedes " + oldRef + "; " + oldRef + " is now superseded" },
		apply: func(string) error {
			return recordSupersede(m.kb, false, recordFlags{args: []string{ref, oldRef}}, &bytes.Buffer{})
		},
	}
	return m.beginChange(f)
}
