package main

import (
	"fmt"
	"io"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["unlink"] = cmdUnlink
}

/** cmdUnlink implements `kb unlink project|observation|source ...`, the inverse of
 * `link` and `source link` (DR-0050). Names match exactly, as for `concept delete`: a
 * destructive command must not guess. Removing a link that does not exist is exit 1,
 * not a silent success.
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — the open knowledge base.
 *   dl      (*DebugLog)                — debug log, may be nil.
 *   jsonOut (bool)                     — emit the result as JSON.
 *   args    ([]string)                 — the subverb and its arguments.
 *   out     (io.Writer)                — where the result is written.
 *
 * Returns:
 *   error — a usage error for a bad command line, or a not-found for a missing
 *           entity or link.
 *
 * Example:
 *   err := cmdUnlink(kb, nil, false, []string{"project", "harvey", "RAG"}, os.Stdout)
 */
func cmdUnlink(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return usageErrorf("usage: unlink <project PROJECT_NAME CONCEPT_NAME | observation OBS_ID CONCEPT_NAME | source OBS_ID SOURCE_ID>")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "project":
		rest, err := plainArgs(rest, 2, 2, 2, "usage: unlink project PROJECT_NAME CONCEPT_NAME")
		if err != nil {
			return err
		}
		if err := logKBCallErr(dl, "UnlinkProjectConcept", map[string]any{"project": rest[0], "concept": rest[1]}, func() error {
			return kb.UnlinkProjectConcept(rest[0], rest[1])
		}); err != nil {
			return err
		}
		return unlinked(out, jsonOut, "project", rest[0], "concept", rest[1])
	case "observation":
		rest, err := plainArgs(rest, 2, 2, 2, "usage: unlink observation OBS_ID CONCEPT_NAME")
		if err != nil {
			return err
		}
		id, err := parseID("observation", rest[0])
		if err != nil {
			return err
		}
		if err := logKBCallErr(dl, "UnlinkObservationConcept", map[string]any{"observation": id, "concept": rest[1]}, func() error {
			return kb.UnlinkObservationConcept(id, rest[1])
		}); err != nil {
			return err
		}
		return unlinked(out, jsonOut, "observation", rest[0], "concept", rest[1])
	case "source":
		rest, err := plainArgs(rest, 2, 2, 2, "usage: unlink source OBS_ID SOURCE_ID")
		if err != nil {
			return err
		}
		obs, err := parseID("observation", rest[0])
		if err != nil {
			return err
		}
		src, err := parseID("source", rest[1])
		if err != nil {
			return err
		}
		if err := logKBCallErr(dl, "UnlinkObservationSource", map[string]any{"observation": obs, "source": src}, func() error {
			return kb.UnlinkObservationSource(obs, src)
		}); err != nil {
			return err
		}
		return unlinked(out, jsonOut, "observation", rest[0], "source", rest[1])
	default:
		return usageErrorf("unknown unlink subcommand %q; want project, observation or source", sub)
	}
}

func unlinked(out io.Writer, jsonOut bool, fromKind, from, toKind, to string) error {
	if jsonOut {
		return printJSON(out, map[string]any{"unlinked": true, fromKind: from, toKind: to})
	}
	_, err := fmt.Fprintf(out, "unlinked %s %s from %s %s\n", toKind, quoteIfName(to), fromKind, quoteIfName(from))
	return err
}

// quoteIfName quotes a name but not a bare id.
func quoteIfName(s string) string {
	for _, r := range s {
		if r < '0' || r > '9' {
			return fmt.Sprintf("%q", s)
		}
	}
	return s
}
