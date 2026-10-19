package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

/** decisionsTarget returns the directory `index` and `record fmt` run on for the
 * current Records scope, as the commands lay a workspace out: a project's records
 * are in agents/projects/NAME/decisions, and every scope is the agents tree.
 *
 * Returns:
 *   dir (string) — the absolute directory
 *   rel (string) — the same, relative to the workspace root, for the screen and the command
 *   all (bool)   — true when the scope is every scope
 *   err (error)  — the directory a project's scope names does not exist
 *
 * Example:
 *   dir, rel, all, err := m.decisionsTarget() // ".../agents/projects/clasm/decisions", "agents/projects/clasm/decisions"
 */
func (m *tuiModel) decisionsTarget() (dir, rel string, all bool, err error) {
	root := recordRoot(m.kb, recordFlags{})
	name := m.recordScopeName()
	if name == "all scopes" {
		return filepath.Join(root, "agents"), "agents", true, nil
	}
	rel = filepath.Join("agents", "projects", name, "decisions")
	dir = filepath.Join(root, rel)
	if fi, statErr := os.Stat(dir); statErr != nil || !fi.IsDir() {
		return "", rel, false, fmt.Errorf("project %s has no decisions directory (%s); nothing was run", name, rel)
	}
	return dir, rel, false, nil
}

/** runDirect runs `kb index` or `kb record fmt` for the current scope with no
 * confirmation (knowledge DR-0067: they write generated files or canonical form
 * and are safe to repeat) and shows the command's own output on a result screen
 * with the command that does the same. A failure is the command's error as a
 * notice.
 *
 * Parameters:
 *   what (string) — "index" or "fmt"
 *
 * Returns:
 *   (tea.Model, tea.Cmd) — the model, showing the result
 *
 * Example:
 *   return m.runDirect("fmt")
 */
func (m *tuiModel) runDirect(what string) (tea.Model, tea.Cmd) {
	dir, rel, all, err := m.decisionsTarget()
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	var out bytes.Buffer
	var title, equivalent string
	switch what {
	case "fmt":
		title, equivalent = "Format files — "+rel, "kb record fmt "+rel
		err = recordFmt(m.kb, false, recordFlags{args: []string{dir}}, &out)
	default:
		title = "Index — " + rel
		args := []string{dir}
		equivalent = "kb index " + rel
		if all {
			args = append(args, "--all")
			equivalent += " --all"
		}
		err = cmdIndex(nil, nil, false, args, &out)
	}
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	text := strings.TrimRight(out.String(), "\n")
	m.reviewRef, m.reviewBack = title, m.state
	m.reviewRaw = []byte(text + "\n\nequivalent:  " + equivalent)
	m.openViewer("result")
	return m, nil
}

/** showRead runs a read verb that needs no argument (check-db, concept list,
 * document list, source list) in-process, as the command line does, and shows its
 * output in the viewer with the command that does the same. A read has no gate
 * (knowledge DR-0067). A failure is shown as the command's own error, under what it
 * printed.
 *
 * Parameters:
 *   args ([]string) — the verb and subverb, such as {"concept", "list"}
 *
 * Returns:
 *   (tea.Model, tea.Cmd) — the model, showing the result
 *
 * Example:
 *   return m.showRead([]string{"source", "list"})
 */
func (m *tuiModel) showRead(args []string) (tea.Model, tea.Cmd) {
	return m.showReadFrom(args, m.state)
}

// showReadFrom is showRead with the screen q returns to named: a list shows an
// item and q comes back to the list.
func (m *tuiModel) showReadFrom(args []string, back viewState) (tea.Model, tea.Cmd) {
	text, err := m.runArgs(args)
	if text == "" && err == nil {
		text = "(nothing to list)"
	}
	words := make([]string, len(args))
	for i, a := range args {
		words[i] = shellWord(a)
	}
	text += "\n\nequivalent:  kb " + strings.Join(words, " ")
	return m.showCommandResult(args, text, err, back, false)
}
