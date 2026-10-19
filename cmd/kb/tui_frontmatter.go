package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	knowledge "github.com/rsdoiel/knowledge"
)

/** pickItem is one proposal in the frontmatter checklist: a scalar field with its
 * proposed value, or a keyword.
 *
 * Fields:
 *   label   (string) — the row as shown.
 *   field   (string) — the scalar field's name, "" for a keyword.
 *   keyword (string) — the keyword's name, "" for a field.
 *   checked (bool)   — selected; every row starts unselected.
 */
type pickItem struct {
	label   string
	field   string
	keyword string
	checked bool
}

/** pickFlow is the frontmatter checklist: what `kb document frontmatter PATH`
 * proposes for one file, to select from. Nothing is selected to begin with, and
 * nothing is written from here: Enter plans the selection as a dry run.
 *
 * Fields:
 *   path   (string)     — the document.
 *   info   ([]string)   — fields already set or with no signal, shown and not selectable.
 *   items  ([]pickItem) — the proposals.
 *   cursor (int)        — the row the cursor is on.
 *   back   (viewState)  — the screen to return to.
 */
type pickFlow struct {
	path   string
	info   []string
	items  []pickItem
	cursor int
	back   viewState
}

// beginFrontmatter asks for the file, then shows what could be written to its
// frontmatter as a checklist (knowledge DR-0033: propose then accept).
func (m *tuiModel) beginFrontmatter() (tea.Model, tea.Cmd) {
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
	f := &formFlow{
		title:   "Document frontmatter",
		heading: "Frontmatter for a document",
		fields:  []formField{pathField},
		then:    func(v []string) error { return m.openPick(v[0]) },
	}
	return m.beginForm(f)
}

// openPick reads the proposals for path, with nothing written, and opens the
// checklist; a file with nothing to propose says so on the screen it came from.
func (m *tuiModel) openPick(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, result, err := m.kb.ApplyFrontmatter(path, raw, nil, knowledge.FrontmatterAccept{}, true)
	if err != nil {
		return err
	}
	p := &pickFlow{path: path, back: m.form.back}
	for _, f := range result.Fields {
		switch {
		case f.Proposed != "" && (f.Current == "" || (f.Field == "dateModified" && f.Current != f.Proposed)):
			label := fmt.Sprintf("%-13s %q", f.Field, f.Proposed)
			if f.Current != "" {
				label += fmt.Sprintf("  (now %s)", f.Current)
			}
			if len(f.Signals) > 1 {
				var parts []string
				for _, s := range f.Signals {
					parts = append(parts, s.Source)
				}
				label += "  (signals disagree: " + strings.Join(parts, ", ") + ")"
			}
			p.items = append(p.items, pickItem{label: label, field: f.Field})
		case f.Current != "":
			p.info = append(p.info, fmt.Sprintf("%s: already set: %s", f.Field, f.Current))
		default:
			p.info = append(p.info, fmt.Sprintf("%s: no signal available", f.Field))
		}
	}
	for _, k := range result.Keywords.Known {
		p.items = append(p.items, pickItem{label: fmt.Sprintf("%-13s %s  (a known concept the text mentions)", "keyword", k), keyword: k})
	}
	for _, c := range result.Keywords.New {
		p.items = append(p.items, pickItem{
			label:   fmt.Sprintf("%-13s %s  (new concept, score %.2f: accepting creates it)", "keyword", c.Term, c.Score),
			keyword: c.Term,
		})
	}
	if len(p.items) == 0 {
		m.setState(m.form.back)
		m.notice = fmt.Sprintf("nothing to propose for %s:\n%s", path, strings.Join(p.info, "\n"))
		return nil
	}
	m.pick = p
	m.setState(viewPick)
	return nil
}

// updatePick handles a key in the checklist: the arrows and j, k move, space
// toggles a row, a toggles every row, Enter plans the selection and q leaves.
func (m *tuiModel) updatePick(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	p := m.pick
	switch msg.String() {
	case "q":
		m.setState(p.back)
		m.notice = "nothing was written"
		return m, nil
	case "esc":
		return m, nil
	case "j", "down":
		if p.cursor < len(p.items)-1 {
			p.cursor++
		}
	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
	case "space":
		p.items[p.cursor].checked = !p.items[p.cursor].checked
	case "a":
		all := true
		for _, it := range p.items {
			all = all && it.checked
		}
		for i := range p.items {
			p.items[i].checked = !all
		}
	case "enter":
		return m.planPick()
	}
	return m, nil
}

// planPick turns the selection into the command's own arguments, shows the dry
// run, and applies it only on y.
func (m *tuiModel) planPick() (tea.Model, tea.Cmd) {
	p := m.pick
	var fields, keywords []string
	for _, it := range p.items {
		switch {
		case !it.checked:
		case it.field != "":
			fields = append(fields, it.field)
		default:
			keywords = append(keywords, it.keyword)
		}
	}
	if len(fields) == 0 && len(keywords) == 0 {
		m.notice = "nothing is selected: space selects a row, and nothing is written until you choose"
		return m, nil
	}
	args := []string{p.path}
	eq := "kb document frontmatter " + shellWord(p.path)
	if len(fields) > 0 {
		args = append(args, "--accept", strings.Join(fields, ","))
		eq += " --accept " + strings.Join(fields, ",")
	}
	if len(keywords) > 0 {
		args = append(args, "--accept-keywords", strings.Join(keywords, ","))
		eq += " --accept-keywords " + shellWord(strings.Join(keywords, ","))
	}
	text, err := runCapture(func(out *bytes.Buffer) error {
		return cmdDocumentFrontmatter(m.kb, false, append(append([]string{}, args...), "--dry-run"), out)
	})
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	m.beginPlan(&planFlow{
		title:      "Frontmatter — " + p.path,
		text:       text,
		equivalent: eq,
		apply: func() (string, error) {
			return runCapture(func(out *bytes.Buffer) error {
				return cmdDocumentFrontmatter(m.kb, false, args, out)
			})
		},
	}, p.back)
	return m, nil
}

// pickLegend names the checklist's keys.
func (m *tuiModel) pickLegend() string {
	return "↑/↓ j/k move   space toggle   a all   Enter review   q cancel"
}

// pickScreen is the checklist: what is already set, then one row per proposal.
func (m *tuiModel) pickScreen() (string, []string) {
	p := m.pick
	var body []string
	for _, l := range p.info {
		body = append(body, "  "+l)
	}
	if len(p.info) > 0 {
		body = append(body, "")
	}
	body = append(body, "Select what to write; nothing is selected to begin with.", "")
	for i, it := range p.items {
		pointer, box := "  ", "[ ]"
		if i == p.cursor {
			pointer = "> "
		}
		if it.checked {
			box = "[x]"
		}
		body = append(body, pointer+box+" "+it.label)
	}
	return "Frontmatter — " + p.path, body
}
