package main

import (
	"io"
	"os"

	"github.com/charmbracelet/x/term"
)

/** isInteractive reports whether a person is at the keyboard: both the input and
 * the output are terminals. A character device is not enough, because /dev/null
 * is one, so the test asks the terminal driver and not the file mode.
 *
 * Parameters:
 *   in (io.Reader) — the input stream, normally os.Stdin
 *   out (io.Writer) — the output stream, normally os.Stdout
 *
 * Returns:
 *   bool — true only when both are *os.File values attached to a terminal
 *
 * Example:
 *   if !isInteractive(os.Stdin, os.Stdout) {
 *       return usageErrorf("a person must accept a record")
 *   }
 */
func isInteractive(in io.Reader, out io.Writer) bool {
	return isTerminalStream(in) && isTerminalStream(out)
}

// isTerminalStream reports whether a stream is an open file attached to a
// terminal. Anything that is not an *os.File (a buffer, a pipe wrapper, nil) is
// not one.
func isTerminalStream(v any) bool {
	f, ok := v.(*os.File)
	return ok && f != nil && term.IsTerminal(f.Fd())
}

// atTerminal answers isInteractive for the process's own standard input and the
// given output. It is a variable only so tests can stand in for a terminal; it
// is not reachable from the command line or the environment, which is the point
// of DR-0061: nothing a caller can pass turns the check off.
var atTerminal = func(out io.Writer) bool { return isInteractive(os.Stdin, out) }
