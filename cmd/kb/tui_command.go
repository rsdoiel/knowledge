package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// The `:` command line (knowledge DR-0066 point 2, DR-0067, v0.0.20 U6). A line
// typed here is the command line's own: the same verbs, the same flags, the same
// functions, run in-process. What differs is the gate, which follows the verb's
// write class in the verb table, so a write typed here meets the screen it would
// meet from the menus.

/** commandFlow is the `:` prompt: one line of text and the screen it returns to.
 *
 * Fields:
 *   edit (*editModel) — the line being typed
 *   back (viewState)  — the screen `:` was typed on
 *
 * Example:
 *   m.openCommand() // opens the prompt over the current screen
 */
type commandFlow struct {
	edit *editModel
	back viewState
}

/** splitCommandLine splits a typed line into words as a shell would for the cases
 * a kb command needs: runs of blanks separate words; single quotes keep
 * everything; double quotes keep everything but let a backslash escape a quote, a
 * backslash or a dollar; a backslash outside quotes escapes the next character.
 * It is the inverse of shellWord.
 *
 * Parameters:
 *   line (string) — the typed line
 *
 * Returns:
 *   []string — the words
 *   error    — a quote that is never closed, or a trailing backslash
 *
 * Example:
 *   words, err := splitCommandLine(`concept add Fresh "a new idea"`)
 *   // ["concept" "add" "Fresh" "a new idea"], nil
 */
func splitCommandLine(line string) ([]string, error) {
	var words []string
	var cur strings.Builder
	inWord := false
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		case r == '\'':
			inWord = true
			j := i + 1
			for j < len(rs) && rs[j] != '\'' {
				cur.WriteRune(rs[j])
				j++
			}
			if j >= len(rs) {
				return nil, usageErrorf("an opening ' is never closed")
			}
			i = j
		case r == '"':
			inWord = true
			j := i + 1
			for j < len(rs) && rs[j] != '"' {
				if rs[j] == '\\' && j+1 < len(rs) && strings.ContainsRune(`"\$`, rs[j+1]) {
					j++
				}
				cur.WriteRune(rs[j])
				j++
			}
			if j >= len(rs) {
				return nil, usageErrorf(`an opening " is never closed`)
			}
			i = j
		case r == '\\':
			if i+1 >= len(rs) {
				return nil, usageErrorf("a backslash ends the line")
			}
			inWord = true
			i++
			cur.WriteRune(rs[i])
		default:
			inWord = true
			cur.WriteRune(r)
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, nil
}

/** commandClass finds the write class of a typed command in the verb table.
 *
 * Parameters:
 *   args ([]string) — the words after any leading "kb"
 *
 * Returns:
 *   writeClass — the class of the deepest verb or subverb the words name
 *   bool       — false when the first word is not a verb
 *
 * Example:
 *   class, ok := commandClass([]string{"record", "delete", "clasm/DR-0001"}) // classRemoving, true
 */
func commandClass(args []string) (writeClass, bool) {
	if len(args) == 0 {
		return classNone, false
	}
	if args[0] == "help" {
		return classRead, true
	}
	var spec *verbSpec
	for i := range verbTable {
		if verbTable[i].Name == args[0] {
			spec = &verbTable[i]
		}
	}
	if spec == nil {
		return classNone, false
	}
	class, subs := spec.Class, spec.Subverbs
	for _, a := range args[1:] {
		var next *subverbSpec
		for i := range subs {
			if subs[i].Name == a {
				next = &subs[i]
			}
		}
		if next == nil {
			break
		}
		class, subs = next.Class, next.Subverbs
	}
	if class == classNone {
		class = classRead // a bare group or an unknown subverb: the command's own usage error
	}
	return class, true
}

// canOpenCommand reports whether `:` opens the prompt on this screen: the
// browsing screens, and never while text is being typed.
func (m *tuiModel) canOpenCommand() bool {
	if m.capturingText() {
		return false
	}
	switch m.state {
	case viewMenu, viewGroup, viewProjects, viewObservations, viewConcepts,
		viewRecords, viewRecordScope, viewSearch, viewReviewQueue:
		return true
	}
	return false
}

// openCommand opens the prompt over the current screen.
func (m *tuiModel) openCommand() (tea.Model, tea.Cmd) {
	e := newEdit("", true, m.cols()-8)
	e.allowSame = true
	m.command = &commandFlow{edit: e, back: m.state}
	m.setState(viewCommand)
	return m, nil
}

// updateCommand takes a key at the prompt. Every printable key is text, q and `:`
// included; Enter runs the line, Esc cancels.
func (m *tuiModel) updateCommand(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.command
	c.edit.Update(msg)
	switch {
	case c.edit.cancelled:
		m.setState(c.back)
	case c.edit.submitted:
		m.setState(c.back)
		line := strings.Join(strings.Fields(strings.ReplaceAll(c.edit.value(), "\n", " ")), " ")
		return m.runCommandLine(c.edit.value(), line)
	}
	return m, nil
}

// commandScreen is the prompt's screen.
func (m *tuiModel) commandScreen() (string, []string) {
	body := []string{"kb ...", ""}
	body = append(body, strings.Split(m.command.edit.View(), "\n")...)
	body = append(body, "",
		"Type a command as on the command line, without the leading kb.",
		"A write shows what it will do and asks first; kb verbs lists them.")
	return "Command", body
}

// cmdWords is a typed command rendered back as one shell line.
func cmdWords(args []string) string {
	words := make([]string, len(args))
	for i, a := range args {
		words[i] = shellWord(a)
	}
	return "kb " + strings.Join(words, " ")
}

// runArgs runs a command in-process through the command line's own dispatcher and
// returns what it printed. A failure is its message and exit status after the
// output, and err is set.
func (m *tuiModel) runArgs(args []string) (string, error) {
	var out, errOut bytes.Buffer
	code := dispatch(verbs, m.kb, nil, false, args, &out, &errOut)
	text := strings.TrimRight(out.String(), "\n")
	if code == 0 {
		return text, nil
	}
	if e := strings.TrimRight(errOut.String(), "\n"); e != "" {
		if text != "" {
			text += "\n"
		}
		text += e
	}
	return text, fmt.Errorf("exit status %d", code)
}

// showCommandResult shows a finished command in the built-in viewer. After a
// write the lists and counts are refreshed first.
func (m *tuiModel) showCommandResult(args []string, text string, err error, back viewState, wrote bool) (tea.Model, tea.Cmd) {
	if wrote {
		m.reloadAfterRemoval()
	}
	if err != nil {
		text += "\n\n" + err.Error()
	}
	m.reviewRef, m.reviewBack = cmdWords(args), back
	m.reviewRaw = []byte(strings.TrimRight(text, "\n"))
	m.openViewer("result")
	return m, nil
}

// refuse leaves the prompt's screen with a notice.
func (m *tuiModel) refuse(format string, a ...any) (tea.Model, tea.Cmd) {
	m.notice = fmt.Sprintf(format, a...)
	return m, nil
}

// runCommandLine acts on a typed line by the write class of its verb.
func (m *tuiModel) runCommandLine(raw, line string) (tea.Model, tea.Cmd) {
	args, err := splitCommandLine(line)
	if err != nil {
		return m.refuse("%v", err)
	}
	if len(args) > 0 && args[0] == "kb" {
		args = args[1:]
	}
	if len(args) == 0 {
		return m, nil
	}
	if strings.HasPrefix(args[0], "-") {
		return m.refuse("%s is a global option, which the interface does not take; give a verb (kb verbs lists them)", args[0])
	}
	class, ok := commandClass(args)
	if !ok {
		return m.refuse("unknown verb %q; kb verbs lists the commands", args[0])
	}
	back := m.state
	hasFlag := func(names ...string) bool {
		for _, a := range args {
			for _, n := range names {
				if a == n || a == "-"+n || strings.HasPrefix(a, n+"=") || strings.HasPrefix(a, "-"+n+"=") {
					return true
				}
			}
		}
		return false
	}
	switch class {
	case classCLIOnly:
		return m.refuse("%s is command line only: %s", args[0], cmdWords(args))
	case classRead:
		if len(args) >= 3 && args[0] == "record" && args[1] == "set-status" {
			break
		}
		text, err := m.runArgs(args)
		return m.showCommandResult(args, text, err, back, false)
	case classDirect:
		text, err := m.runArgs(args)
		return m.showCommandResult(args, text, err, back, true)
	case classRemoving:
		return m.removingFromCommand(args)
	case classPlanApply, classGuided:
		if class == classGuided && args[0] == "document" {
			switch {
			case len(args) == 4 && args[1] == "review" && args[2] == "promote":
				id, err := strconv.ParseInt(args[3], 10, 64)
				if err != nil {
					return m.refuse("invalid section id %q", args[3])
				}
				return m.openFromCommand(id)
			case len(args) > 1 && args[1] == "draft":
				return m.confirmTyped(args, classChanging)
			case len(args) > 1 && args[1] == "ingest":
			default:
				return m.refuse("%s has no screen here; from the command line: %s", strings.Join(args[:min(len(args), 3)], " "), cmdWords(args))
			}
		}
		reportOnly := hasFlag("--dry-run", "dry-run") ||
			(args[0] == "record" && len(args) > 1 && args[1] == "fuzzy-tag" && !hasFlag("--write", "write")) ||
			(args[0] == "document" && len(args) > 1 && args[1] == "frontmatter" &&
				!hasFlag("--accept", "accept", "--accept-keywords", "accept-keywords", "--set", "set"))
		if reportOnly {
			text, err := m.runArgs(args)
			return m.showCommandResult(args, text, err, back, false)
		}
		text, err := m.runArgs(append(append([]string(nil), args...), "--dry-run"))
		if err != nil {
			return m.showCommandResult(args, text, err, back, false)
		}
		m.beginPlan(&planFlow{
			title:      cmdWords(args),
			text:       text,
			equivalent: cmdWords(args),
			apply:      func() (string, error) { return m.runArgs(args) },
		}, back)
		return m, nil
	}
	// Additive and changing writes. A status change is the interface's own: the
	// terminal rule for `accepted` is met by being here, and the move is checked
	// against the table before anything is shown.
	if args[0] == "record" && len(args) >= 2 && args[1] == "set-status" && len(args) <= 4 {
		return m.statusFromCommand(args)
	}
	return m.confirmTyped(args, class)
}

// confirmTyped shows a typed write and asks y before running it exactly as typed.
func (m *tuiModel) confirmTyped(args []string, class writeClass) (tea.Model, tea.Cmd) {
	m.beginPlan(&planFlow{
		title:      cmdWords(args),
		note:       "this command has not been run yet",
		text:       classNote(class),
		equivalent: cmdWords(args),
		apply:      func() (string, error) { return m.runArgs(args) },
	}, m.state)
	return m, nil
}

// classNote says in a sentence what kind of write the typed command is.
func classNote(c writeClass) string {
	if c == classChanging {
		return "It changes something that exists. y runs it exactly as typed."
	}
	return "It adds knowledge. y runs it exactly as typed."
}

// statusFromCommand handles `record set-status REF [STATUS]`: with a status, a
// confirmation showing old to new; without, the same status review `s` opens.
func (m *tuiModel) statusFromCommand(args []string) (tea.Model, tea.Cmd) {
	if len(args) < 3 {
		return m.refuse("usage: record set-status REF [STATUS]")
	}
	rec, err := resolveRecordForWrite(m.kb, args[2], recordFlags{})
	if err != nil {
		return m.refuse("%v", err)
	}
	ref := refOf(*rec, projectNames(m.kb)).String()
	if len(args) == 3 {
		return m.showRecord(ref, "status")
	}
	status := args[3]
	rf, _, err := loadRecordFile(recordRoot(m.kb, recordFlags{}), rec)
	if err != nil {
		return m.refuse("%v", err)
	}
	if err := checkTransition(rf.Record.Status, status, len(rf.SupersededBy) > 0); err != nil {
		return m.refuse("%v", err)
	}
	back := m.state
	m.beginPlan(&planFlow{
		title:      "Set status — " + ref,
		note:       "this command has not been run yet",
		text:       fmt.Sprintf("%s  %s\n\n  %s → %s", ref, rec.Title, rf.Record.Status, status),
		equivalent: "kb record set-status " + ref + " " + status,
		apply: func() (string, error) {
			var sink bytes.Buffer
			if err := applyRecordStatus(m.kb, false, recordFlags{}, &sink, rec, status); err != nil {
				return "", err
			}
			return fmt.Sprintf("%s status set to %s", ref, status), nil
		},
	}, back)
	return m, nil
}

// removingFromCommand sends a typed delete through the same typed-name gate `d`
// opens. Only the four things the interface can select have one; the others are
// refused with the command to run.
func (m *tuiModel) removingFromCommand(args []string) (tea.Model, tea.Cmd) {
	later := func() (tea.Model, tea.Cmd) {
		return m.refuse("%s has no confirmation screen here; run it from the command line: %s", strings.Join(args[:min(len(args), 2)], " "), cmdWords(args))
	}
	if len(args) != 3 || args[1] != "delete" {
		return later()
	}
	switch args[0] {
	case "project":
		p, err := planProjectDelete(m.kb, args[2])
		return m.beginRemoval(p, err)
	case "concept":
		p, err := planConceptDelete(m.kb, args[2])
		return m.beginRemoval(p, err)
	case "observation":
		id, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return m.refuse("observation delete takes the observation's number, not %q", args[2])
		}
		o, err := m.kb.ObservationByID(id)
		if err != nil || o == nil {
			return m.refuse("no observation %d", id)
		}
		p, err := planObservationDelete(m.kb, id, o.Body)
		return m.beginRemoval(p, err)
	case "record":
		rec, err := resolveRecordForWrite(m.kb, args[2], recordFlags{})
		if err != nil {
			return m.refuse("%v", err)
		}
		ref := refOf(*rec, projectNames(m.kb)).String()
		p, err := planRecordDelete(m.kb, rec, ref)
		return m.beginRemoval(p, err)
	}
	return later()
}
