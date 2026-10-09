package main

import (
	"strings"
)

/** deepLink decides whether a command line opens the interface, and where
 * (knowledge DR-0066 amendment). A complete command always runs as a command and
 * prints, on a terminal or not, so scripts can rely on it. Only an incomplete one
 * opens the interface, and only on a terminal: bare kb opens the top menu, a bare
 * group its menu, and a leaf that needs an argument it was not given the screen
 * where one is picked. The global -i opens the interface at a command and refuses
 * a command that has no screen yet, rather than printing as if it had not been
 * asked.
 *
 * Parameters:
 *   rest ([]string) — the verb and its arguments, after the global options
 *   forced (bool) — -i was given
 *   terminal (bool) — standard input and output are a terminal
 *
 * Returns:
 *   start (tuiStart) — where the interface opens, when open is true
 *   open (bool) — the command line opens the interface
 *   err (error) — a usage error: -i without a terminal or for a command with no screen, or bare kb without a terminal
 *
 * Example:
 *   start, open, err := deepLink([]string{"record"}, false, true) // the Records menu
 */
func deepLink(rest []string, forced, terminal bool) (start tuiStart, open bool, err error) {
	groups := map[string]bool{}
	for _, it := range topMenu() {
		if it.Group {
			groups[it.Verb] = true
		}
	}
	if forced {
		if !terminal {
			return start, false, usageErrorf("-i opens the interface and needs a terminal on standard input and output")
		}
		switch {
		case len(rest) == 0:
			return tuiStart{state: viewMenu}, true, nil
		case len(rest) == 1 && groups[rest[0]]:
			return tuiStart{state: viewGroup, group: rest[0]}, true, nil
		case len(rest) == 2 && rest[0] == "record" && rest[1] == "list":
			return tuiStart{state: viewRecordScope}, true, nil
		case len(rest) == 2 && rest[0] == "record" && rest[1] == "pending":
			return tuiStart{state: viewRecordScope, status: "proposed"}, true, nil
		case len(rest) == 2 && rest[0] == "project" && rest[1] == "list":
			return tuiStart{state: viewProjects}, true, nil
		case len(rest) >= 2 && rest[0] == "search":
			return tuiStart{state: viewSearch, term: strings.Join(rest[1:], " ")}, true, nil
		case len(rest) == 3 && rest[0] == "record" && rest[1] == "show":
			return tuiStart{state: viewRecordScope, all: true, ref: rest[2]}, true, nil
		}
		return start, false, usageErrorf("-i has no screen for %q yet; it opens the menus, record list, record pending, record show REF, project list and search TERM (kb verbs lists the commands)", strings.Join(rest, " "))
	}
	if len(rest) == 0 {
		if !terminal {
			return start, false, usageErrorf("kb opens its interface only on a terminal (standard input and output); give a verb, or see kb help")
		}
		return tuiStart{state: viewMenu}, true, nil
	}
	if !terminal {
		return start, false, nil
	}
	switch {
	case len(rest) == 1 && groups[rest[0]]:
		return tuiStart{state: viewGroup, group: rest[0]}, true, nil
	case len(rest) == 2 && rest[0] == "record" && rest[1] == "show":
		return tuiStart{state: viewRecordScope, all: true}, true, nil
	case len(rest) == 2 && rest[0] == "project" && rest[1] == "show":
		return tuiStart{state: viewProjects}, true, nil
	}
	return start, false, nil
}
