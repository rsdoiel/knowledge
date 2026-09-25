package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	knowledge "github.com/rsdoiel/knowledge"
)

// verbFunc is the signature every verb handler implements. kb is already
// open; dl is non-nil only when --debug was passed (nil-safe either way);
// args holds only the arguments after the verb name itself.
type verbFunc func(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error

// verbs maps verb name to handler. Populated incrementally: empty for now,
// entries added via init() in each verb group's own file as it's built.
var verbs = map[string]verbFunc{}

// dispatch looks up args[0] in verbs and calls it. Callers must ensure
// args is non-empty and not a help request before calling dispatch --
// those cases are handled earlier, in mainRun, since they don't require
// an open KnowledgeBase.
func dispatch(verbs map[string]verbFunc, kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out, errOut io.Writer) int {
	verb := args[0]
	fn, ok := verbs[verb]
	if !ok {
		fmt.Fprintf(errOut, "kb: unknown verb %q\n", verb)
		printHelp(errOut, "")
		return 2
	}
	if err := fn(kb, dl, jsonOut, args[1:], out); err != nil {
		// A help flag that follows other flags (`project add --status active
		// -help`) is not caught before the verb runs; the subverb's FlagSet
		// reports it as ErrHelp. Answer with the verb's page, as for any
		// other help request, not with the package's "flag: help requested".
		if errors.Is(err, flag.ErrHelp) && printHelp(out, verb) {
			return 0
		}
		printError(errOut, jsonOut, err)
		return exitCodeFor(err).Code
	}
	return 0
}
