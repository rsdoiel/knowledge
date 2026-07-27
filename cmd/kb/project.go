package main

import (
	"fmt"
	"io"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["project"] = cmdProject
}

func cmdProject(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: project <add|list|show|concepts> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return cmdProjectAdd(kb, jsonOut, rest, out)
	case "list":
		return cmdProjectList(kb, jsonOut, rest, out)
	case "show":
		return cmdProjectShow(kb, jsonOut, rest, out)
	case "concepts":
		return cmdProjectConcepts(kb, jsonOut, rest, out)
	default:
		return fmt.Errorf("unknown project subcommand %q", sub)
	}
}

func cmdProjectAdd(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: project add NAME [DESCRIPTION]")
	}
	name := args[0]
	desc := strings.Join(args[1:], " ")
	id, err := kb.AddProject(name, desc)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}{ID: id, Name: name})
	}
	fmt.Fprintf(out, "project %q added (id=%d)\n", name, id)
	return nil
}

func cmdProjectList(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	projects, err := kb.Projects()
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, projects)
	}
	if len(projects) == 0 {
		fmt.Fprintln(out, "(no projects)")
		return nil
	}
	for _, p := range projects {
		fmt.Fprintf(out, "%-4d  %-24s  %-10s  %s\n", p.ID, p.Name, p.Status, p.Description)
	}
	return nil
}

func cmdProjectShow(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: project show NAME")
	}
	p, err := kb.ProjectByName(args[0])
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("project %q not found", args[0])
	}
	if jsonOut {
		return printJSON(out, p)
	}
	fmt.Fprintf(out, "%d  %s  [%s]\n", p.ID, p.Name, p.Status)
	if p.Description != "" {
		fmt.Fprintf(out, "  %s\n", p.Description)
	}
	return nil
}

func cmdProjectConcepts(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: project concepts NAME")
	}
	p, err := kb.ProjectByName(args[0])
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("project %q not found", args[0])
	}
	concepts, err := kb.ProjectConcepts(p.ID)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, concepts)
	}
	if len(concepts) == 0 {
		fmt.Fprintln(out, "(no concepts)")
		return nil
	}
	for _, c := range concepts {
		fmt.Fprintf(out, "%-4d  %s\n", c.ID, c.Name)
	}
	return nil
}
