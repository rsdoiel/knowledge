package main

import (
	"fmt"
	"io"
	"strconv"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["link"] = cmdLink
}

func cmdLink(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: link <project|observation> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "project":
		return cmdLinkProject(kb, jsonOut, rest, out)
	case "observation":
		return cmdLinkObservation(kb, jsonOut, rest, out)
	default:
		return fmt.Errorf("unknown link subcommand %q", sub)
	}
}

// conceptByName finds a concept by its (unique) name. Returns (nil, nil)
// if no concept has that name.
func conceptByName(kb *knowledge.KnowledgeBase, name string) (*knowledge.Concept, error) {
	concepts, err := kb.Concepts()
	if err != nil {
		return nil, err
	}
	for _, c := range concepts {
		if c.Name == name {
			return &c, nil
		}
	}
	return nil, nil
}

func cmdLinkProject(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: link project PROJECT_NAME CONCEPT_NAME")
	}
	p, err := kb.ProjectByName(args[0])
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("project %q not found", args[0])
	}
	c, err := conceptByName(kb, args[1])
	if err != nil {
		return err
	}
	if c == nil {
		return fmt.Errorf("concept %q not found", args[1])
	}
	if err := kb.LinkProjectConcept(p.ID, c.ID); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			ProjectID int64 `json:"project_id"`
			ConceptID int64 `json:"concept_id"`
		}{ProjectID: p.ID, ConceptID: c.ID})
	}
	fmt.Fprintf(out, "linked project %q to concept %q\n", args[0], args[1])
	return nil
}

func cmdLinkObservation(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: link observation OBS_ID CONCEPT_NAME")
	}
	obsID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid observation id %q", args[0])
	}
	c, err := conceptByName(kb, args[1])
	if err != nil {
		return err
	}
	if c == nil {
		return fmt.Errorf("concept %q not found", args[1])
	}
	if err := kb.LinkObservationConcept(obsID, c.ID); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			ObservationID int64 `json:"observation_id"`
			ConceptID     int64 `json:"concept_id"`
		}{ObservationID: obsID, ConceptID: c.ID})
	}
	fmt.Fprintf(out, "linked observation %d to concept %q\n", obsID, args[1])
	return nil
}
