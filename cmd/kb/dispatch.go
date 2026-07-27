package main

import (
	"fmt"
	"io"

	knowledge "github.com/rsdoiel/knowledge"
)

// verbFunc is the signature every verb handler implements. kb is already
// open; args holds only the arguments after the verb name itself.
type verbFunc func(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error

// verbs maps verb name to handler. Populated incrementally: empty for now,
// entries added via init() in each verb group's own file as it's built.
var verbs = map[string]verbFunc{}

// dispatch looks up args[0] in verbs and calls it. Callers must ensure
// args is non-empty and not a help request before calling dispatch --
// those cases are handled earlier, in mainRun, since they don't require
// an open KnowledgeBase.
func dispatch(verbs map[string]verbFunc, kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out, errOut io.Writer) int {
	verb := args[0]
	fn, ok := verbs[verb]
	if !ok {
		fmt.Fprintf(errOut, "kb: unknown verb %q\n", verb)
		printUsage(errOut)
		return 2
	}
	if err := fn(kb, jsonOut, args[1:], out); err != nil {
		printError(errOut, jsonOut, err)
		return 1
	}
	return 0
}
