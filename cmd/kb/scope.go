package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

/** inferredProject names the project the command should act on when none was
 * given (DR-0058): the project the working directory belongs to, else the one
 * KB_PROJECT names, else none. The directory is the closer evidence, so it wins
 * over the environment.
 *
 * Parameters:
 *   kb (*knowledge.KnowledgeBase) — the open knowledge base.
 *   f  (recordFlags)              — supplies --root, the workspace root.
 *
 * Returns:
 *   string — the project name, or "" for the whole workspace.
 *   error  — not found (exit 1) when KB_PROJECT names no project.
 *
 * Example:
 *   name, err := inferredProject(kb, f)
 */
func inferredProject(kb *knowledge.KnowledgeBase, f recordFlags) (string, error) {
	if cwd, err := os.Getwd(); err == nil {
		name, err := kb.ProjectForDir(recordRoot(kb, f), cwd)
		if err != nil {
			return "", err
		}
		if name != "" {
			return name, nil
		}
	}
	if env := strings.TrimSpace(os.Getenv("KB_PROJECT")); env != "" {
		p, err := kb.ProjectByName(env)
		if err != nil {
			return "", err
		}
		if p == nil {
			return "", notFoundf("KB_PROJECT names %q, which is not a project", env)
		}
		return p.Name, nil
	}
	return "", nil
}

/** recordScopes turns the scope arguments of a verb into the record filters it
 * runs, one per scope. Precedence (DR-0058): explicit scopes (positional
 * arguments, or the --project and --workspace aliases), then --all, then the
 * project inferred from the directory or KB_PROJECT, then the whole workspace.
 * Explicit scopes, the aliases and --all cannot be mixed.
 *
 * Parameters:
 *   kb (*knowledge.KnowledgeBase) — the open knowledge base.
 *   f  (recordFlags)              — the parsed flags; f.args are scope names.
 *
 * Returns:
 *   []knowledge.RecordFilter — Project and Scope set for each scope; a single
 *                              empty filter for the whole workspace.
 *   error                    — a usage error for mixed sources, not found for
 *                              an unknown scope.
 *
 * Example:
 *   filters, err := recordScopes(kb, recordFlags{args: []string{"harvey", "workspace"}})
 */
func recordScopes(kb *knowledge.KnowledgeBase, f recordFlags) ([]knowledge.RecordFilter, error) {
	aliases := f.project != "" || f.workspace
	switch {
	case len(f.args) > 0 && aliases:
		return nil, usageErrorf("give scopes as arguments or with --project/--workspace, not both")
	case f.all && (len(f.args) > 0 || aliases):
		return nil, usageErrorf("--all cannot be combined with a scope")
	}

	names := f.args
	if f.project != "" {
		names = []string{f.project}
	}
	if f.workspace {
		names = []string{"workspace"}
	}
	if len(names) == 0 && !f.all {
		inferred, err := inferredProject(kb, f)
		if err != nil {
			return nil, err
		}
		if inferred != "" {
			names = []string{inferred}
		}
	}
	if len(names) == 0 {
		return []knowledge.RecordFilter{{}}, nil
	}

	workspace := filepath.Base(recordRoot(kb, f))
	var filters []knowledge.RecordFilter
	seen := map[string]bool{}
	for _, name := range names {
		filter, key, err := scopeFilter(kb, name, workspace)
		if err != nil {
			return nil, err
		}
		if !seen[key] {
			seen[key] = true
			filters = append(filters, filter)
		}
	}
	return filters, nil
}

// scopeFilter classifies one scope name: "workspace" (any case) is the
// workspace tier; an exact project name is that project; the workspace
// directory's name (any case) is the workspace tier again. The same order as
// knowledge.ResolveRef, so a scope means the same in a reference and in a list.
func scopeFilter(kb *knowledge.KnowledgeBase, name, workspace string) (knowledge.RecordFilter, string, error) {
	if strings.EqualFold(name, "workspace") {
		return knowledge.RecordFilter{Scope: "workspace"}, "workspace", nil
	}
	p, err := kb.ProjectByName(name)
	if err != nil {
		return knowledge.RecordFilter{}, "", err
	}
	if p != nil {
		return knowledge.RecordFilter{Project: p.Name, Scope: "project"}, "project:" + p.Name, nil
	}
	if strings.EqualFold(name, workspace) {
		return knowledge.RecordFilter{Scope: "workspace"}, "workspace", nil
	}
	return knowledge.RecordFilter{}, "", notFoundf("unknown scope %q: it is neither a project nor the workspace", name)
}

// listRecordsIn runs each filter and merges the results oldest first, by date
// then record id then owner, dropping a record reached twice. A single filter
// is returned as the library ordered it.
func listRecordsIn(kb *knowledge.KnowledgeBase, base knowledge.RecordFilter, scopes []knowledge.RecordFilter) ([]knowledge.Record, error) {
	var all []knowledge.Record
	seen := map[int64]bool{}
	for _, s := range scopes {
		filter := base
		filter.Project, filter.Scope = s.Project, s.Scope
		records, err := kb.ListRecords(filter)
		if err != nil {
			return nil, err
		}
		for _, r := range records {
			if !seen[r.ID] {
				seen[r.ID] = true
				all = append(all, r)
			}
		}
	}
	if len(scopes) > 1 {
		sort.SliceStable(all, func(i, j int) bool {
			a, b := all[i], all[j]
			if a.Date != b.Date {
				return a.Date < b.Date
			}
			if a.RecordID != b.RecordID {
				return a.RecordID < b.RecordID
			}
			return fmt.Sprint(a.Scope, a.ProjectID) < fmt.Sprint(b.Scope, b.ProjectID)
		})
	}
	return all, nil
}
