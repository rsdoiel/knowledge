package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["concept"] = cmdConcept
}

func cmdConcept(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: concept <add|list> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return cmdConceptAdd(kb, jsonOut, rest, out)
	case "list":
		return cmdConceptList(kb, jsonOut, rest, out)
	default:
		return fmt.Errorf("unknown concept subcommand %q", sub)
	}
}

func cmdConceptAdd(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("concept add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	identifierType := fs.String("identifier-type", "", "e.g. doi, orcid, ror")
	identifierValue := fs.String("identifier-value", "", "normalized identifier value")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return fmt.Errorf("usage: concept add NAME [DESCRIPTION] [--identifier-type T --identifier-value V]")
	}
	name := rest[0]
	desc := strings.Join(rest[1:], " ")
	var id int64
	var err error
	if *identifierType != "" || *identifierValue != "" {
		id, err = kb.AddConceptWithIdentifier(name, desc, *identifierType, *identifierValue)
	} else {
		id, err = kb.AddConcept(name, desc)
	}
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}{ID: id, Name: name})
	}
	fmt.Fprintf(out, "concept %q added (id=%d)\n", name, id)
	return nil
}

func cmdConceptList(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	concepts, err := kb.Concepts()
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
