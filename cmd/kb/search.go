package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["search"] = cmdSearch
	verbs["summary"] = cmdSummary
	verbs["format"] = cmdFormat
}

func cmdSearch(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: search TERM")
	}
	term := strings.Join(args, " ")
	results, err := logKBCall(dl, "Search", map[string]any{"term": term}, func() ([]knowledge.KBSearchResult, error) {
		return kb.Search(term)
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, results)
	}
	if len(results) == 0 {
		fmt.Fprintf(out, "no results for %q\n", term)
		return nil
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
	if err := fs.Parse(args); err != nil {
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
			return fmt.Errorf("project %q not found", *project)
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
