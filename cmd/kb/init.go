package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["init"] = cmdInit
}

/** cmdInit implements the init verb: it creates a schema-only
 * agents/knowledge.db at PATH (default: cwd), the same shape as `git init`.
 * It never touches main.go's ambient --db resolution -- like merge and
 * index, it is exempted at the dispatch layer (main.go) and resolves its own
 * target path here, so a wrong-cwd invocation can't leave a stray ambient
 * database behind the way DR-0021 describes for other verbs.
 *
 * Idempotent: running it again against an already-initialized workspace
 * reports that fact and leaves the existing database untouched -- knowledge.Open
 * only ever adds missing schema, never drops data, but init still checks
 * first so it can say "already initialized" rather than implying it just
 * created something.
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — unused; init is dispatched with a
 *                                        nil kb, the same as merge and index.
 *   dl      (*DebugLog)                — debug log, may be nil.
 *   jsonOut (bool)                     — emit the result as JSON.
 *   args    ([]string)                 — an optional single PATH.
 *   out     (io.Writer)                — where the result is written.
 *
 * Returns:
 *   error — on more than one positional argument, or if the database cannot
 *           be created.
 *
 * Example:
 *   err := cmdInit(nil, nil, false, []string{"."}, os.Stdout)
 */
func cmdInit(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	root := "."
	switch len(args) {
	case 0:
	case 1:
		root = args[0]
	default:
		return fmt.Errorf("init takes a single optional PATH, got %d arguments", len(args))
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	dbPath := knowledge.DefaultPath(absRoot)

	alreadyInitialized := false
	if _, err := os.Stat(dbPath); err == nil {
		alreadyInitialized = true
	}

	dl.Log("init", map[string]any{"path": dbPath, "already_initialized": alreadyInitialized})

	newKB, err := knowledge.Open(dbPath)
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	newKB.Close()

	if jsonOut {
		return printJSON(out, map[string]any{
			"path":                dbPath,
			"already_initialized": alreadyInitialized,
		})
	}
	if alreadyInitialized {
		fmt.Fprintf(out, "%s already initialized\n", dbPath)
	} else {
		fmt.Fprintf(out, "initialized empty knowledge base at %s\n", dbPath)
	}
	return nil
}
