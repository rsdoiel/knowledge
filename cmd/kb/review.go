package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	"github.com/rsdoiel/knowledge"
)

// reviewStdin is where the review reads its keys. It is a variable so tests can
// supply them; production reads the process's own standard input, the same
// stream the terminal check looked at. A terminal is put in raw mode so each key
// acts at once; anything else is read as a stream of keys and its end cancels.
var reviewStdin io.Reader = os.Stdin

/** pagerCommand chooses the program that shows a record for review: $KB_PAGER
 * when set (split on spaces), else bat with the Markdown language when bat is
 * installed, else less with raw control characters passed through.
 *
 * Parameters:
 *   getenv (func(string) string) — environment lookup, os.Getenv in production
 *   lookPath (func(string) (string, error)) — executable lookup, exec.LookPath in production
 *
 * Returns:
 *   []string — the command and its arguments, or nil when there is no pager
 *
 * Example:
 *   cmd := pagerCommand(os.Getenv, exec.LookPath) // ["bat" "-l" "markdown"]
 */
func pagerCommand(getenv func(string) string, lookPath func(string) (string, error)) []string {
	if fields := strings.Fields(getenv("KB_PAGER")); len(fields) > 0 {
		return fields
	}
	if _, err := lookPath("bat"); err == nil {
		return []string{"bat", "-l", "markdown"}
	}
	if _, err := lookPath("less"); err == nil {
		return []string{"less", "-R"}
	}
	return nil
}

/** statusKey returns the letter that chooses a status in the review prompt: the
 * status's first letter. The letters are distinct across the vocabulary and none
 * is q, which backs out of the review.
 *
 * Parameters:
 *   status (string) — a record status
 *
 * Returns:
 *   rune — the key, lower case, or 0 for an empty status
 *
 * Example:
 *   statusKey("cancelled") // 'c'
 */
func statusKey(status string) rune {
	r, _ := utf8.DecodeRuneInString(strings.ToLower(status))
	if r == utf8.RuneError {
		return 0
	}
	return r
}

// reviewOptions are the statuses the review offers: what the transition table
// allows from the record's current status. A status outside the vocabulary can
// only be set to proposed.
func reviewOptions(from string, hasSupersededBy bool) []string {
	if !containsString(knowledge.RecordStatuses, from) {
		return []string{"proposed"}
	}
	return knowledge.AllowedTransitions(from, hasSupersededBy)
}

// showRecord puts the record in front of the person: through the pager when
// there is one and it starts, else straight to out. A pager that exits non-zero
// (a person quitting it, a closed pipe) is not an error; the review goes on.
func showRecord(out io.Writer, raw []byte) {
	if argv := pagerCommand(os.Getenv, exec.LookPath); argv != nil {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdin = bytes.NewReader(raw)
		cmd.Stdout = out
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err == nil {
			_ = cmd.Wait()
			return
		}
	}
	out.Write(raw)
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		fmt.Fprintln(out)
	}
}

/** recordReviewStatus is record set-status with no status: it shows the record,
 * offers the moves the transition table allows, asks for confirmation, and
 * writes only then (DR-0060). Backing out at either step writes nothing and is
 * exit 1; the review needs a terminal and takes no --json, exit 2.
 *
 * Parameters:
 *   kb (*knowledge.KnowledgeBase) — the open knowledge base
 *   jsonOut (bool) — true is refused, the review is interactive
 *   f (recordFlags) — the parsed flags; f.args[0] is the record reference
 *   out (io.Writer) — where the record, the prompts and the result go
 *
 * Returns:
 *   error — nil when a status was applied; a usage error without a terminal or
 *           with --json; a negative-class error when the record has no moves or
 *           the person backed out
 *
 * Example:
 *   err := recordReviewStatus(kb, false, f, os.Stdout)
 */
func recordReviewStatus(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer) error {
	if jsonOut {
		return usageErrorf("record set-status without a status is an interactive review and takes no --json; give the status to run it from a script: kb record set-status REF STATUS")
	}
	if !atTerminal(out) {
		return usageErrorf("reviewing a record needs a terminal on standard input and output; give the status to run it from a script: kb record set-status %s STATUS", f.args[0])
	}
	rec, err := resolveRecordForWrite(kb, f.args[0], f)
	if err != nil {
		return err
	}
	rf, raw, err := loadRecordFile(recordRoot(kb, f), rec)
	if err != nil {
		return err
	}
	ref := refOf(*rec, projectNames(kb)).String()
	from := rf.Record.Status
	options := reviewOptions(from, len(rf.SupersededBy) > 0)
	if len(options) == 0 {
		return negativef("%s is %s, which is final; there is no move to make", ref, from)
	}

	showRecord(out, raw)

	// One key chooses, one confirms, and Esc or q backs out at either, with no
	// Enter (DR-0065). The program is inline, so the record above stays on screen.
	chosen, confirmed, err := runReview(reviewStdin, out, ref, from, options)
	if err != nil {
		return err
	}
	if !confirmed {
		return negativef("cancelled; %s is unchanged", ref)
	}
	return applyRecordStatus(kb, false, f, out, rec, chosen)
}
