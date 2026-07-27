package main

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["observation"] = cmdObservation
}

func cmdObservation(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: observation <add|list|show|sources> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return cmdObservationAdd(kb, jsonOut, rest, out)
	case "list":
		return cmdObservationList(kb, jsonOut, rest, out)
	case "show":
		return cmdObservationShow(kb, jsonOut, rest, out)
	case "sources":
		return cmdObservationSources(kb, jsonOut, rest, out)
	default:
		return fmt.Errorf("unknown observation subcommand %q", sub)
	}
}

func cmdObservationAdd(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("observation add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project name (required)")
	sourceDOI := fs.String("source-doi", "", "normalized DOI of the source paper")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if *project == "" || len(rest) < 2 {
		return fmt.Errorf("usage: observation add --project NAME KIND BODY...")
	}
	p, err := kb.ProjectByName(*project)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("project %q not found", *project)
	}
	kind := rest[0]
	body := strings.Join(rest[1:], " ")
	var id int64
	if *sourceDOI != "" {
		id, err = kb.AddObservationWithSource(p.ID, kind, body, *sourceDOI)
	} else {
		id, err = kb.AddObservation(p.ID, kind, body)
	}
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			ID int64 `json:"id"`
		}{ID: id})
	}
	fmt.Fprintf(out, "observation recorded (id=%d, kind=%s)\n", id, kind)
	return nil
}

func cmdObservationList(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("observation list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project name (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *project == "" {
		return fmt.Errorf("usage: observation list --project NAME")
	}
	p, err := kb.ProjectByName(*project)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("project %q not found", *project)
	}
	obs, err := kb.Observations(p.ID)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, obs)
	}
	if len(obs) == 0 {
		fmt.Fprintln(out, "(no observations)")
		return nil
	}
	for _, o := range obs {
		fmt.Fprintf(out, "%-4d  [%-10s] %s\n", o.ID, o.Kind, o.Body)
	}
	return nil
}

func cmdObservationShow(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: observation show ID")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid observation id %q", args[0])
	}
	o, err := kb.ObservationByID(id)
	if err != nil {
		return fmt.Errorf("observation %d not found", id)
	}
	if jsonOut {
		return printJSON(out, o)
	}
	fmt.Fprintf(out, "[%s] %s\n", o.Kind, o.Body)
	if o.SourceDOI != "" {
		fmt.Fprintf(out, "  DOI (legacy): %s\n", o.SourceDOI)
	}
	return nil
}

func cmdObservationSources(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: observation sources ID")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid observation id %q", args[0])
	}
	sources, err := kb.ObservationSources(id)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, sources)
	}
	if len(sources) == 0 {
		fmt.Fprintln(out, "(no linked sources)")
		return nil
	}
	for _, s := range sources {
		fmt.Fprintf(out, "%-4d  %s\n", s.ID, s.Title)
	}
	return nil
}
