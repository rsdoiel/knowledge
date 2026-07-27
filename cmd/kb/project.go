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

func cmdProject(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: project <add|list|show|concepts> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return cmdProjectAdd(kb, dl, jsonOut, rest, out)
	case "list":
		return cmdProjectList(kb, dl, jsonOut, rest, out)
	case "show":
		return cmdProjectShow(kb, dl, jsonOut, rest, out)
	case "concepts":
		return cmdProjectConcepts(kb, dl, jsonOut, rest, out)
	default:
		return fmt.Errorf("unknown project subcommand %q", sub)
	}
}

func cmdProjectAdd(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: project add NAME [DESCRIPTION]")
	}
	name := args[0]
	desc := strings.Join(args[1:], " ")
	id, err := logKBCall(dl, "AddProject", map[string]any{"name": name, "description": desc}, func() (int64, error) {
		return kb.AddProject(name, desc)
	})
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

func cmdProjectList(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	projects, err := logKBCall(dl, "Projects", nil, kb.Projects)
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

func cmdProjectShow(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: project show NAME")
	}
	name := args[0]
	p, err := logKBCall(dl, "ProjectByName", map[string]any{"name": name}, func() (*knowledge.Project, error) {
		return kb.ProjectByName(name)
	})
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("project %q not found", name)
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

func cmdProjectConcepts(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: project concepts NAME")
	}
	name := args[0]
	p, err := logKBCall(dl, "ProjectByName", map[string]any{"name": name}, func() (*knowledge.Project, error) {
		return kb.ProjectByName(name)
	})
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("project %q not found", name)
	}
	concepts, err := logKBCall(dl, "ProjectConcepts", map[string]any{"project_id": p.ID}, func() ([]knowledge.Concept, error) {
		return kb.ProjectConcepts(p.ID)
	})
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
