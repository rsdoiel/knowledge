package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["source"] = cmdSource
}

func cmdSource(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: source <add|list|show|remove|retract|link|check-retractions> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return cmdSourceAdd(kb, jsonOut, rest, out)
	case "list":
		return cmdSourceList(kb, jsonOut, rest, out)
	case "show":
		return cmdSourceShow(kb, jsonOut, rest, out)
	case "remove":
		return cmdSourceRemove(kb, jsonOut, rest, out)
	case "retract":
		return cmdSourceRetract(kb, jsonOut, rest, out)
	case "link":
		return cmdSourceLink(kb, jsonOut, rest, out)
	case "check-retractions":
		return cmdSourceCheckRetractions(kb, jsonOut, rest, out)
	default:
		return fmt.Errorf("unknown source subcommand %q", sub)
	}
}

// parseSourceFlags consumes the --doi/--url/--authors/--published/
// --publisher/--rights/--version flags (in any order, interleaved with
// TITLE) and returns the assembled Source plus whether a title was found.
func parseSourceFlags(args []string) (knowledge.Source, bool) {
	var s knowledge.Source
	haveTitle := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--doi":
			if i+1 < len(args) {
				i++
				s.IdentifierType = "doi"
				s.IdentifierValue = args[i]
			}
		case "--url":
			if i+1 < len(args) {
				i++
				if s.IdentifierType == "" {
					s.IdentifierType = "url"
					s.IdentifierValue = args[i]
				}
			}
		case "--authors":
			if i+1 < len(args) {
				i++
				s.Authors = args[i]
			}
		case "--published":
			if i+1 < len(args) {
				i++
				s.PublishedDate = args[i]
			}
		case "--publisher":
			if i+1 < len(args) {
				i++
				s.Publisher = args[i]
			}
		case "--rights":
			if i+1 < len(args) {
				i++
				s.Rights = args[i]
			}
		case "--version":
			if i+1 < len(args) {
				i++
				s.Version = args[i]
			}
		default:
			if !haveTitle {
				s.Title = args[i]
				haveTitle = true
			}
		}
	}
	return s, haveTitle
}

func cmdSourceAdd(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	s, haveTitle := parseSourceFlags(args)
	if !haveTitle {
		return fmt.Errorf("usage: source add TITLE [--doi D] [--url U] [--authors A] [--published DATE] [--publisher P] [--rights R] [--version V]")
	}
	id, err := kb.AddSource(s)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		}{ID: id, Title: s.Title})
	}
	fmt.Fprintf(out, "source added (id=%d)\n", id)
	return nil
}

func cmdSourceList(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	sources, err := kb.ListSources()
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, sources)
	}
	if len(sources) == 0 {
		fmt.Fprintln(out, "(no sources)")
		return nil
	}
	for _, s := range sources {
		retracted := ""
		if s.Retracted {
			retracted = " [RETRACTED]"
		}
		fmt.Fprintf(out, "%-4d  %s%s\n", s.ID, s.Title, retracted)
	}
	return nil
}

func cmdSourceShow(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	id, err := parseSourceID(args, "source show ID")
	if err != nil {
		return err
	}
	s, err := kb.ShowSource(id)
	if err != nil {
		return fmt.Errorf("source %d not found", id)
	}
	if jsonOut {
		return printJSON(out, s)
	}
	fmt.Fprintf(out, "%d  %s\n", s.ID, s.Title)
	if s.IdentifierType != "" {
		fmt.Fprintf(out, "  %s: %s\n", s.IdentifierType, s.IdentifierValue)
	}
	if s.Retracted {
		fmt.Fprintf(out, "  [RETRACTED] %s\n", s.RetractionNote)
	}
	return nil
}

func cmdSourceRemove(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	id, err := parseSourceID(args, "source remove ID")
	if err != nil {
		return err
	}
	if err := kb.RemoveSource(id); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			ID      int64 `json:"id"`
			Removed bool  `json:"removed"`
		}{ID: id, Removed: true})
	}
	fmt.Fprintf(out, "source %d removed\n", id)
	return nil
}

func cmdSourceRetract(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: source retract ID NOTE")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid source id %q", args[0])
	}
	note := strings.Join(args[1:], " ")
	if err := kb.RetractSource(id, note); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			ID        int64  `json:"id"`
			Retracted bool   `json:"retracted"`
			Note      string `json:"note"`
		}{ID: id, Retracted: true, Note: note})
	}
	fmt.Fprintf(out, "source %d retracted\n", id)
	return nil
}

func cmdSourceLink(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: source link OBS_ID SOURCE_ID [--relationship R]")
	}
	obsID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid observation id %q", args[0])
	}
	sourceID, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid source id %q", args[1])
	}
	relationship := "cited"
	rest := args[2:]
	for i := 0; i < len(rest); i++ {
		if rest[i] == "--relationship" && i+1 < len(rest) {
			relationship = rest[i+1]
			i++
		}
	}
	if err := kb.LinkObservationSource(obsID, sourceID, relationship); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			ObservationID int64  `json:"observation_id"`
			SourceID      int64  `json:"source_id"`
			Relationship  string `json:"relationship"`
		}{ObservationID: obsID, SourceID: sourceID, Relationship: relationship})
	}
	fmt.Fprintf(out, "linked observation %d to source %d (%s)\n", obsID, sourceID, relationship)
	return nil
}

func cmdSourceCheckRetractions(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	// CheckRetractions writes a human-readable line per DOI checked
	// directly to its out parameter; suppress that in JSON mode so stdout
	// stays cleanly parseable, same reasoning as cmdMerge's progressOut.
	progressOut := out
	if jsonOut {
		progressOut = io.Discard
	}
	checked, updated, err := kb.CheckRetractions(
		func(doi string) (bool, string, error) {
			return knowledge.CheckDOIRetraction(doi, knowledge.DefaultRetractionWatchURL)
		},
		progressOut,
	)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			Checked int `json:"checked"`
			Updated int `json:"updated"`
		}{Checked: checked, Updated: updated})
	}
	fmt.Fprintf(out, "Checked %d DOI source(s); %d newly marked as retracted.\n", checked, updated)
	return nil
}

func parseSourceID(args []string, usage string) (int64, error) {
	if len(args) == 0 {
		return 0, fmt.Errorf("usage: %s", usage)
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid source id %q", args[0])
	}
	return id, nil
}
