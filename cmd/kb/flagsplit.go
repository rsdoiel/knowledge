package main

import (
	"flag"
	"fmt"
	"strings"
)

// splitFlags separates recognized flags from positional arguments, filling
// the given pointers, and returns everything else as positional in order.
// Unlike flag.FlagSet, it tolerates flags in any position relative to
// positionals -- required for commands like "document ingest PATH --project
// P" or "ingest PATH --dry-run", where a positional argument comes first.
//
// This is deliberately not a fit for parseGlobalFlags (main.go): that one
// relies on flag.FlagSet's real stop-at-first-non-flag behavior on purpose,
// to delimit global options from the verb and the verb's own arguments
// ("kb -json ingest DIR --dry-run" -- -json is global, --dry-run is
// ingest's). Folding it into this helper would erase that boundary.
//
// An unrecognized "-"-prefixed argument, or a recognized string flag with no
// following value, is an error. boolFlags may be nil when a caller has none.
// An undeclared -h, -help or --help is a request for help, not a typo, and is
// reported as flag.ErrHelp so dispatch can print the verb's page, exactly as
// it does for the FlagSet-based subverbs.
func splitFlags(args []string, strFlags map[string]*string, boolFlags map[string]*bool) ([]string, error) {
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if target, ok := strFlags[arg]; ok {
			if i+1 >= len(args) {
				return nil, usageErrorf("%s requires a value", arg)
			}
			*target = args[i+1]
			i++
			continue
		}
		if target, ok := boolFlags[arg]; ok {
			*target = true
			continue
		}
		if isHelpFlag(arg) {
			return nil, flag.ErrHelp
		}
		if strings.HasPrefix(arg, "-") {
			return nil, withGlobalHint(fmt.Sprintf("unknown flag %q", arg), []string{arg})
		}
		positional = append(positional, arg)
	}
	return positional, nil
}
