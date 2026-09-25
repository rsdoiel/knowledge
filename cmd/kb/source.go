package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["source"] = cmdSource
}

func cmdSource(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return usageErrorf("usage: source <add|list|show|remove|retract|link|check-retractions> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return cmdSourceAdd(kb, dl, jsonOut, rest, out)
	case "list":
		return cmdSourceList(kb, dl, jsonOut, rest, out)
	case "show":
		return cmdSourceShow(kb, dl, jsonOut, rest, out)
	case "remove":
		return cmdSourceRemove(kb, dl, jsonOut, rest, out)
	case "retract":
		return cmdSourceRetract(kb, dl, jsonOut, rest, out)
	case "link":
		return cmdSourceLink(kb, dl, jsonOut, rest, out)
	case "check-retractions":
		return cmdSourceCheckRetractions(kb, dl, jsonOut, rest, out)
	default:
		return usageErrorf("unknown source subcommand %q", sub)
	}
}

// parseSourceFlags consumes the --doi/--url/--authors/--published/
// --publisher/--rights/--version flags (in any order, interleaved with
// TITLE) and returns the assembled Source plus whether a title was found.
// parseSourceFlags separates flags from positional arguments via the shared
// splitFlags (cmd/kb/flagsplit.go). doi and url are captured separately so
// that doi's priority over url (kb-source(1): "doi takes priority if both
// are given") can be applied after parsing, rather than needing splitFlags
// itself to know about that precedence.
//
// Tightened from its previous hand-rolled form: an unrecognized flag, or a
// recognized one missing its value, now errors instead of being silently
// dropped -- matching every other verb's flag parsing, and closing a real
// gap where a mistyped flag used to vanish with no feedback.
func parseSourceFlags(args []string) (knowledge.Source, error) {
	var s knowledge.Source
	var doi, url string
	strFlags := map[string]*string{
		"--doi": &doi, "--url": &url,
		"--authors": &s.Authors, "--published": &s.PublishedDate,
		"--publisher": &s.Publisher, "--rights": &s.Rights, "--version": &s.Version,
	}
	positional, err := splitFlags(args, strFlags, nil)
	if err != nil {
		return s, err
	}
	switch {
	case doi != "":
		s.IdentifierType, s.IdentifierValue = "doi", doi
	case url != "":
		s.IdentifierType, s.IdentifierValue = "url", url
	}
	if len(positional) == 0 {
		return s, usageErrorf("source add requires a TITLE")
	}
	if len(positional) > 1 {
		return s, usageErrorf("source add takes one TITLE, got %d arguments (quote a multi-word title)", len(positional))
	}
	s.Title = positional[0]
	return s, nil
}

func cmdSourceAdd(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	s, err := parseSourceFlags(args)
	if err != nil {
		return usageErrorf("usage: source add TITLE [--doi D] [--url U] [--authors A] [--published DATE] [--publisher P] [--rights R] [--version V]: %w", err)
	}
	id, err := logKBCall(dl, "AddSource", map[string]any{"title": s.Title}, func() (int64, error) {
		return kb.AddSource(s)
	})
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

func cmdSourceList(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	args, argErr := plainArgs(args, 0, 0, 0, "usage: source list")
	if argErr != nil {
		return argErr
	}
	sources, err := logKBCall(dl, "ListSources", nil, kb.ListSources)
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

func cmdSourceShow(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	id, err := parseSourceID(args, "source show ID")
	if err != nil {
		return err
	}
	s, err := logKBCall(dl, "ShowSource", map[string]any{"id": id}, func() (*knowledge.Source, error) {
		return kb.ShowSource(id)
	})
	if err != nil {
		return notFoundf("source %d not found", id)
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

func cmdSourceRemove(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	id, err := parseSourceID(args, "source remove ID")
	if err != nil {
		return err
	}
	if err := logKBCallErr(dl, "RemoveSource", map[string]any{"id": id}, func() error {
		return kb.RemoveSource(id)
	}); err != nil {
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

func cmdSourceRetract(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	args, argErr := plainArgs(args, 2, -1, 1, "usage: source retract ID NOTE")
	if argErr != nil {
		return argErr
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return usageErrorf("invalid source id %q", args[0])
	}
	note := strings.Join(args[1:], " ")
	if err := logKBCallErr(dl, "RetractSource", map[string]any{"id": id, "note": note}, func() error {
		return kb.RetractSource(id, note)
	}); err != nil {
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

func cmdSourceLink(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	relationship := "cited"
	args, err := splitFlags(args, map[string]*string{"--relationship": &relationship}, nil)
	if err != nil {
		return err
	}
	if len(args) != 2 {
		return usageErrorf("usage: source link OBS_ID SOURCE_ID [--relationship R]")
	}
	obsID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return usageErrorf("invalid observation id %q", args[0])
	}
	sourceID, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return usageErrorf("invalid source id %q", args[1])
	}
	if err := logKBCallErr(dl, "LinkObservationSource", map[string]any{"observation_id": obsID, "source_id": sourceID, "relationship": relationship}, func() error {
		return kb.LinkObservationSource(obsID, sourceID, relationship)
	}); err != nil {
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

func cmdSourceCheckRetractions(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	args, argErr := plainArgs(args, 0, 0, 0, "usage: source check-retractions")
	if argErr != nil {
		return argErr
	}
	// CheckRetractions writes a human-readable line per DOI checked
	// directly to its out parameter; suppress that in JSON mode so stdout
	// stays cleanly parseable, same reasoning as cmdMerge's progressOut.
	progressOut := out
	if jsonOut {
		progressOut = io.Discard
	}
	start := time.Now()
	checked, updated, err := kb.CheckRetractions(
		func(doi string) (bool, string, error) {
			return knowledge.CheckDOIRetraction(doi, knowledge.DefaultRetractionWatchURL)
		},
		progressOut,
	)
	// CheckRetractions returns (int, int, error) -- doesn't fit
	// logKBCall's single-result shape, so this is logged directly rather
	// than forcing an awkward wrapper type.
	fields := map[string]any{
		"method":      "CheckRetractions",
		"duration_ms": time.Since(start).Milliseconds(),
		"checked":     checked,
		"updated":     updated,
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	dl.Log("kb_call", fields)
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
	args, err := plainArgs(args, 1, 1, 1, "usage: "+usage)
	if err != nil {
		return 0, err
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return 0, usageErrorf("invalid source id %q", args[0])
	}
	return id, nil
}
