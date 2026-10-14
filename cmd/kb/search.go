package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["search"] = cmdSearch
	verbs["summary"] = cmdSummary
	verbs["format"] = cmdFormat
}

/** looksLikeSearchScope reports whether a search's first argument is written as
 * a scope (knowledge DR-0073): no spaces, and a slash in it. A term with a slash
 * in it is passed after `--`, so a first argument that looks like a scope and
 * does not resolve is a usage error and not a search that silently finds
 * nothing.
 *
 * Parameters:
 *   arg (string) — the first argument of kb search.
 *
 * Returns:
 *   bool — true for "harvey/", "harvey/records", "/records", "workspace/".
 *
 * Example:
 *   looksLikeSearchScope("harvey/records") // true
 *   looksLikeSearchScope("harvey")         // false
 */
func looksLikeSearchScope(arg string) bool {
	return strings.Contains(arg, "/") && !strings.ContainsAny(arg, " \t\n") && !strings.HasPrefix(arg, "-")
}

/** parseSearchScope turns a first argument of the form SCOPE/ or SCOPE/KIND into
 * a knowledge.SearchScope. SCOPE is "workspace" (any case), a project name, the
 * workspace directory's own name as an alias for the workspace tier (a project
 * wins over it, as for record references, DR-0057), or empty for everywhere.
 * KIND is empty or a member of knowledge.SearchKinds.
 *
 * Parameters:
 *   kb (*knowledge.KnowledgeBase) — for project names and the workspace name.
 *   arg (string) — the first argument; looksLikeSearchScope(arg) is true.
 *
 * Returns:
 *   knowledge.SearchScope — the scope, with Kinds set from KIND.
 *   string — what to call the scope in output: "workspace", the project, or "everywhere".
 *   error — a usage error naming the choices for an unknown scope or kind.
 *
 * Example:
 *   sc, name, err := parseSearchScope(kb, "harvey/records")
 */
func parseSearchScope(kb *knowledge.KnowledgeBase, arg string) (knowledge.SearchScope, string, error) {
	var sc knowledge.SearchScope
	i := strings.Index(arg, "/")
	word, kind := arg[:i], arg[i+1:]
	if kind != "" {
		ok := false
		for _, k := range knowledge.SearchKinds {
			if k == kind {
				ok = true
				sc.Kinds = []string{kind}
			}
		}
		if !ok {
			return sc, "", usageErrorf("unknown search kind %q in %q; the kinds are %s. To search for text that contains a slash, put it after --",
				kind, arg, strings.Join(knowledge.SearchKinds, ", "))
		}
	}
	name := word
	switch {
	case word == "":
		name = "everywhere"
	case strings.EqualFold(word, "workspace"):
		sc.Workspace, name = true, "workspace"
	default:
		p, err := kb.ProjectByName(word)
		if err != nil {
			return sc, "", err
		}
		switch {
		case p != nil:
			sc.Project = p.Name
		case strings.EqualFold(word, filepath.Base(recordRoot(kb, recordFlags{}))):
			sc.Workspace, name = true, "workspace"
		default:
			return sc, "", usageErrorf("unknown scope %q in %q: it is neither a project nor workspace. To search for text that contains a slash, put it after --",
				word, arg)
		}
	}
	return sc, name, nil
}

// searchKindOrder is the order of the result groups: the menu order of the kinds.
var searchKindOrder = knowledge.SearchKinds

// snippetOf collapses whitespace and cuts a snippet to a line's worth.
func snippetOf(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}

// scopedHit is one result of a scoped search as the command prints it.
type scopedHit struct {
	Kind      string   `json:"kind"`
	Ref       string   `json:"ref,omitempty"`
	Label     string   `json:"label"`
	Snippet   string   `json:"snippet"`
	Type      string   `json:"source_type"`
	ID        int64    `json:"source_id"`
	ProjectID int64    `json:"project_id,omitempty"`
	Projects  []string `json:"projects,omitempty"`
}

// searchScoped runs a scoped search and prints it grouped by kind with counts.
func searchScoped(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, scope knowledge.SearchScope, scopeName string, term string, out io.Writer) error {
	res, err := logKBCall(dl, "SearchScoped", map[string]any{"term": term, "scope": scopeName, "kinds": scope.Kinds}, func() (*knowledge.ScopedSearchResult, error) {
		return kb.SearchScoped(term, scope)
	})
	if err != nil {
		return err
	}
	if res.Total == 0 {
		return negativef("no results for %q in %s", term, scopeName)
	}
	names := projectNames(kb)
	var hits []scopedHit
	for _, r := range res.Results {
		h := scopedHit{Kind: r.Kind, Label: r.Label, Snippet: r.Snippet, Type: r.SourceType, ID: r.SourceID, ProjectID: r.ProjectID,
			Projects: res.Spans[r.SourceID]}
		if r.SourceType == "record" {
			if rec, rerr := kb.RecordByID(r.SourceID); rerr == nil && rec != nil {
				h.Ref = refOf(*rec, names).String()
			}
		}
		if r.SourceType != "concept" {
			h.Projects = nil
		}
		hits = append(hits, h)
	}
	if jsonOut {
		return printJSON(out, struct {
			Scope   string         `json:"scope"`
			Kinds   []string       `json:"kinds,omitempty"`
			Term    string         `json:"term"`
			Total   int            `json:"total"`
			Shown   int            `json:"shown"`
			Counts  map[string]int `json:"counts"`
			Results []scopedHit    `json:"results"`
		}{scopeName, scope.Kinds, term, res.Total, len(hits), res.Counts, hits})
	}
	var parts []string
	for _, k := range searchKindOrder {
		if n := res.Counts[k]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, k))
		}
	}
	noun := "matches"
	if res.Total == 1 {
		noun = "match"
	}
	fmt.Fprintf(out, "%s: %d %s — %s\n", scopeName, res.Total, noun, strings.Join(parts, " · "))
	typeOf := map[string]string{"records": "record", "observations": "observation", "concepts": "concept", "documents": "document_summary", "projects": "project"}
	for _, k := range searchKindOrder {
		if res.Counts[k] == 0 {
			continue
		}
		fmt.Fprintf(out, "\n%s (%d)\n", k, res.Counts[k])
		for _, h := range hits {
			if h.Type != typeOf[k] {
				continue
			}
			who := h.Label
			switch {
			case h.Ref != "":
				who = h.Ref
			case h.Type == "observation":
				who = fmt.Sprintf("%s #%d", h.Label, h.ID)
			}
			line := "  " + who
			if s := snippetOf(h.Snippet, 100); s != "" {
				line += " — " + s
			}
			if len(h.Projects) > 0 {
				line += "   [" + strings.Join(h.Projects, ", ") + "]"
			}
			fmt.Fprintln(out, line)
		}
	}
	if res.Total > len(hits) {
		fmt.Fprintf(out, "\nshowing %d of %d\n", len(hits), res.Total)
	}
	return nil
}

func cmdSearch(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) > 0 && args[0] != "--" && looksLikeSearchScope(args[0]) {
		scope, name, err := parseSearchScope(kb, args[0])
		if err != nil {
			return err
		}
		rest, argErr := plainArgs(args[1:], 1, -1, 1, "usage: search SCOPE/[KIND] TERM")
		if argErr != nil {
			return argErr
		}
		term := strings.Join(rest, " ")
		if strings.TrimSpace(term) == "" {
			return usageErrorf("usage: search SCOPE/[KIND] TERM")
		}
		return searchScoped(kb, dl, jsonOut, scope, name, term, out)
	}
	// The first word is checked for flag shape, so a mistyped option (`kb search
	// --json foo`) is a usage error, not a search for the text "--json foo" that
	// finds nothing. `--` gives a dash-leading term on purpose; later words are
	// free text, dashes included (DR-0041).
	args, argErr := plainArgs(args, 1, -1, 1, "usage: search TERM")
	if argErr != nil {
		return argErr
	}
	term := strings.Join(args, " ")
	// A blank term matches nothing by construction: the command line is wrong,
	// not the result empty.
	if strings.TrimSpace(term) == "" {
		return usageErrorf("usage: search TERM")
	}
	results, err := logKBCall(dl, "Search", map[string]any{"term": term}, func() ([]knowledge.KBSearchResult, error) {
		return kb.Search(term)
	})
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return negativef("no results for %q", term)
	}
	if jsonOut {
		return printJSON(out, results)
	}
	for _, r := range results {
		// The bracket shows which table the hit came from, not the hit's own
		// kind. A record's kind is "decision" or "correction", which would be
		// indistinguishable in this column from a decision-kind observation.
		label := r.SourceType
		if label == "" {
			label = r.Kind
		}
		switch {
		case r.Label != "" && r.Snippet != "":
			fmt.Fprintf(out, "[%-10s] %s — %s\n", label, r.Label, r.Snippet)
		case r.Label != "":
			fmt.Fprintf(out, "[%-10s] %s\n", label, r.Label)
		default:
			fmt.Fprintf(out, "[%-10s] %s\n", label, r.Snippet)
		}
	}
	return nil
}

func cmdSummary(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	args, argErr := plainArgs(args, 0, 0, 0, "usage: summary")
	if argErr != nil {
		return argErr
	}
	s, err := logKBCall(dl, "Summary", nil, kb.Summary)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			Summary string `json:"summary"`
		}{Summary: s})
	}
	fmt.Fprint(out, s)
	return nil
}

func cmdFormat(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("format", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project name; omit for all projects")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := noExtraArgs(fs, "usage: format [--project NAME]"); err != nil {
		return err
	}
	var projectID int64
	if *project != "" {
		p, err := logKBCall(dl, "ProjectByName", map[string]any{"name": *project}, func() (*knowledge.Project, error) {
			return kb.ProjectByName(*project)
		})
		if err != nil {
			return err
		}
		if p == nil {
			return notFoundf("project %q not found", *project)
		}
		projectID = p.ID
	}
	md, err := logKBCall(dl, "FormatMarkdown", map[string]any{"project_id": projectID}, func() (string, error) {
		return kb.FormatMarkdown(projectID)
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			Markdown string `json:"markdown"`
		}{Markdown: md})
	}
	fmt.Fprint(out, md)
	return nil
}
