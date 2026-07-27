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

func cmdSearch(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: search TERM")
	}
	term := strings.Join(args, " ")
	results, err := kb.Search(term)
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
		switch {
		case r.Label != "" && r.Snippet != "":
			fmt.Fprintf(out, "[%-10s] %s — %s\n", r.Kind, r.Label, r.Snippet)
		case r.Label != "":
			fmt.Fprintf(out, "[%-10s] %s\n", r.Kind, r.Label)
		default:
			fmt.Fprintf(out, "[%-10s] %s\n", r.Kind, r.Snippet)
		}
	}
	return nil
}

func cmdSummary(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	s, err := kb.Summary()
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

func cmdFormat(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("format", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project name; omit for all projects")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var projectID int64
	if *project != "" {
		p, err := kb.ProjectByName(*project)
		if err != nil {
			return err
		}
		if p == nil {
			return fmt.Errorf("project %q not found", *project)
		}
		projectID = p.ID
	}
	md, err := kb.FormatMarkdown(projectID)
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
