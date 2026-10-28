package cli

import (
	"errors"
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
		// The title as stored: AddSource tidies it, so the flag's text may differ.
		title := strings.Join(strings.Fields(s.Title), " ")
		return printJSON(out, struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		}{ID: id, Title: title})
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
	cites := make([]knowledge.SourceCitations, len(sources))
	for i, s := range sources {
		if cites[i], err = kb.SourceCitations(s.ID); err != nil {
			return err
		}
	}
	if jsonOut {
		rows := make([]sourceRow, len(sources))
		for i, s := range sources {
			rows[i] = sourceRow{Source: s, Citations: citationsJSON(cites[i])}
		}
		return printJSON(out, rows)
	}
	if len(sources) == 0 {
		fmt.Fprintln(out, "(no sources)")
		return nil
	}
	for i, s := range sources {
		retracted := ""
		if s.Retracted {
			retracted = " [RETRACTED]"
		}
		cited := ""
		if cites[i].Total() > 0 {
			cited = "  — cited by " + cites[i].String()
		}
		fmt.Fprintf(out, "%-4d  %s%s%s\n", s.ID, s.Title, retracted, cited)
	}
	return nil
}

// citationsJSON is what cites a source, as --json prints it.
type citationsJSON struct {
	Observations int `json:"observations"`
	Documents    int `json:"documents"`
	Records      int `json:"records"`
}

// sourceRow is a source with its citations, for --json. Source is embedded so the
// source's own fields keep the names they always had.
type sourceRow struct {
	knowledge.Source
	Citations citationsJSON `json:"citations"`
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
	cites, err := kb.SourceCitations(id)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, sourceRow{Source: *s, Citations: citationsJSON(cites)})
	}
	fmt.Fprintf(out, "%d  %s\n", s.ID, s.Title)
	if s.IdentifierType != "" {
		fmt.Fprintf(out, "  %s: %s\n", s.IdentifierType, s.IdentifierValue)
	}
	if s.Retracted {
		fmt.Fprintf(out, "  [RETRACTED] %s\n", s.RetractionNote)
	}
	if cites.Total() > 0 {
		fmt.Fprintf(out, "  cited by %s\n", cites)
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

// unsetFlag marks a string flag the person did not give, so a flag given with a
// blank value can be told from one left out.
const unsetFlag = "\x00"

// cmdSourceLink implements `kb source link`: an observation, a document or a
// decision record cites a source. The version and hash say which version of the
// work was read, and are optional (DR-0081).
func cmdSourceLink(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	const usage = "usage: source link OBS_ID SOURCE_ID | --document DOC_ID SOURCE_ID | --record REF SOURCE_ID [--relationship R] [--version V] [--hash H]"
	relationship, doc, rec, version, hash := "cited", unsetFlag, unsetFlag, unsetFlag, unsetFlag
	args, err := splitFlags(args, map[string]*string{"--relationship": &relationship, "--document": &doc, "--record": &rec,
		"--version": &version, "--hash": &hash}, nil)
	if err != nil {
		return err
	}
	for name, v := range map[string]string{"--version": version, "--hash": hash, "--relationship": relationship} {
		if v != unsetFlag && strings.TrimSpace(v) == "" {
			return usageErrorf("%s must not be blank; %s", name, usage)
		}
	}
	if version == unsetFlag {
		version = ""
	}
	if hash == unsetFlag {
		hash = ""
	}
	if doc != unsetFlag && rec != unsetFlag {
		return usageErrorf("--document and --record name different kinds of row; give one; %s", usage)
	}
	switch {
	case doc != unsetFlag:
		if len(args) != 1 {
			return usageErrorf("%s", usage)
		}
		docID, err := strconv.ParseInt(doc, 10, 64)
		if err != nil {
			return usageErrorf("invalid document id %q", doc)
		}
		sourceID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return usageErrorf("invalid source id %q", args[0])
		}
		if err := logKBCallErr(dl, "LinkDocumentSource", map[string]any{"document_id": docID, "source_id": sourceID, "relationship": relationship}, func() error {
			return kb.LinkDocumentSource(docID, sourceID, relationship, version, hash)
		}); err != nil {
			return err
		}
		return linkedSource(out, jsonOut, "document", fmt.Sprint(docID), docID, sourceID, relationship, version, hash)
	case rec != unsetFlag:
		if len(args) != 1 {
			return usageErrorf("%s", usage)
		}
		sourceID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return usageErrorf("invalid source id %q", args[0])
		}
		r, err := resolveRecord(kb, rec, recordFlags{})
		if err != nil {
			return err
		}
		if err := logKBCallErr(dl, "LinkRecordSource", map[string]any{"record_id": r.ID, "source_id": sourceID, "relationship": relationship}, func() error {
			return kb.LinkRecordSource(r.ID, sourceID, relationship, version, hash)
		}); err != nil {
			return err
		}
		return linkedSource(out, jsonOut, "record", rec, r.ID, sourceID, relationship, version, hash)
	}
	if len(args) != 2 {
		return usageErrorf("%s", usage)
	}
	obsID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return usageErrorf("invalid observation id %q", args[0])
	}
	sourceID, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return usageErrorf("invalid source id %q", args[1])
	}
	if err := logKBCallErr(dl, "LinkObservationSourceAt", map[string]any{"observation_id": obsID, "source_id": sourceID, "relationship": relationship}, func() error {
		return kb.LinkObservationSourceAt(obsID, sourceID, relationship, version, hash)
	}); err != nil {
		return err
	}
	return linkedSource(out, jsonOut, "observation", fmt.Sprint(obsID), obsID, sourceID, relationship, version, hash)
}

// linkedSource reports one new citation. ownerRef is what the person typed or
// the id (a record's qualified reference), ownerID the row's id.
func linkedSource(out io.Writer, jsonOut bool, kind, ownerRef string, ownerID, sourceID int64, relationship, version, hash string) error {
	if jsonOut {
		m := map[string]any{kind + "_id": ownerID, "source_id": sourceID, "relationship": relationship}
		if kind == "record" {
			m["record"] = ownerRef
		}
		if version != "" {
			m["source_version"] = version
		}
		if hash != "" {
			m["source_hash"] = hash
		}
		return printJSON(out, m)
	}
	_, err := fmt.Fprintf(out, "linked %s %s to source %d (%s)\n", kind, ownerRef, sourceID, relationship)
	return err
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
	checked, updated, err := kb.CheckRetractions(func(doi string) (bool, string, error) {
		retracted, note, err := retractionChecker(doi)
		if err != nil {
			// Any failure to get an answer (no route, a bad status, an
			// undecodable reply) means the service is unavailable, exit 69:
			// nothing can be said about the DOI.
			return false, "", unavailablef("%s: %w", doi, err)
		}
		return retracted, note, nil
	}, progressOut)
	var checkErr *knowledge.RetractionCheckError
	failed := 0
	if errors.As(err, &checkErr) {
		failed = checkErr.Failed
	}
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
	fields["failed"] = failed
	dl.Log("kb_call", fields)
	if err != nil && checkErr == nil {
		return err // the database, not a lookup: nothing was reported
	}
	// Some lookups may have failed; the rest were still checked, so the counts
	// are printed either way, and err (if any) is returned after them.
	if jsonOut {
		if perr := printJSON(out, struct {
			Checked int `json:"checked"`
			Updated int `json:"updated"`
			Failed  int `json:"failed"`
		}{Checked: checked, Updated: updated, Failed: failed}); perr != nil {
			return perr
		}
		return err
	}
	fmt.Fprintf(out, "Checked %d DOI source(s); %d newly marked as retracted.\n", checked, updated)
	if failed > 0 {
		fmt.Fprintf(out, "%d could not be checked and are NOT known to be clear: run it again when the service is reachable.\n", failed)
	}
	return err
}

// retractionChecker looks a DOI up in the retraction service. It is a variable
// so a test can stand in for the network. Its errors are classed as unavailable
// by cmdSourceCheckRetractions, whatever the checker returns.
var retractionChecker = func(doi string) (bool, string, error) {
	return knowledge.CheckDOIRetraction(doi, knowledge.DefaultRetractionWatchURL)
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
