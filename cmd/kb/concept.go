package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["concept"] = cmdConcept
}

func cmdConcept(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return usageErrorf("usage: concept <add|list|rename|delete|suggest> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return cmdConceptAdd(kb, dl, jsonOut, rest, out)
	case "list":
		return cmdConceptList(kb, dl, jsonOut, rest, out)
	case "rename":
		return cmdConceptRename(kb, dl, jsonOut, rest, out)
	case "delete":
		return cmdConceptDelete(kb, dl, jsonOut, rest, out)
	case "suggest":
		return cmdConceptSuggest(kb, dl, jsonOut, rest, out)
	default:
		return usageErrorf("unknown concept subcommand %q", sub)
	}
}

func cmdConceptAdd(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("concept add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	identifierType := fs.String("identifier-type", "", "e.g. doi, orcid, ror")
	identifierValue := fs.String("identifier-value", "", "normalized identifier value")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return usageErrorf("usage: concept add NAME [DESCRIPTION] [--identifier-type T --identifier-value V]")
	}
	name, err := knowledge.CleanName("concept", rest[0])
	if err != nil {
		return err
	}
	desc := strings.Join(rest[1:], " ")
	var id int64
	if *identifierType != "" || *identifierValue != "" {
		id, err = logKBCall(dl, "AddConceptWithIdentifier", map[string]any{"name": name, "identifier_type": *identifierType, "identifier_value": *identifierValue}, func() (int64, error) {
			return kb.AddConceptWithIdentifier(name, desc, *identifierType, *identifierValue)
		})
	} else {
		id, err = logKBCall(dl, "AddConcept", map[string]any{"name": name}, func() (int64, error) {
			return kb.AddConcept(name, desc)
		})
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

// cmdConceptRename implements `concept rename OLD NEW` (DR-0024). Unlike
// project rename, nothing refuses this beyond a name collision: a concept
// has no corpus of external files to desync.
func cmdConceptRename(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	args, argErr := plainArgs(args, 2, 2, 2, "usage: concept rename OLD NEW")
	if argErr != nil {
		return argErr
	}
	old := args[0]
	new, err := knowledge.CleanName("concept", args[1])
	if err != nil {
		return err
	}
	err = logKBCallErr(dl, "RenameConcept", map[string]any{"old": old, "new": new}, func() error {
		return kb.RenameConcept(old, new)
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			Old string `json:"old"`
			New string `json:"new"`
		}{Old: old, New: new})
	}
	fmt.Fprintf(out, "concept %q renamed to %q\n", old, new)
	return nil
}

func cmdConceptList(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	args, argErr := plainArgs(args, 0, 0, 0, "usage: concept list")
	if argErr != nil {
		return argErr
	}
	concepts, err := logKBCall(dl, "Concepts", nil, kb.Concepts)
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

// cmdConceptSuggest implements `kb concept suggest [--project NAME]
// [--limit N]`: a read-only scan of every record body and document
// section body -- scoped to one project when --project is given -- that
// prints candidate new concepts ranked by corpus-wide distinctiveness. It
// never writes anything; a suggestion becomes a real concept only when a
// human runs `kb concept add`, the same mechanical-signal-then-human-
// curates pattern DR-0027's density-linking and document summary review
// both already use.
func cmdConceptSuggest(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("concept suggest", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	projectName := fs.String("project", "", "scope to one project")
	limit := fs.Int("limit", 20, "maximum number of suggestions")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return usageErrorf("usage: concept suggest [--project NAME] [--limit N]")
	}

	if *limit < 0 {
		return usageErrorf("invalid --limit %d; want zero or more", *limit)
	}
	suggestions, err := kb.SuggestConcepts(*projectName, *limit)
	if err != nil {
		return err
	}
	dl.Log("concept suggest", map[string]any{"project": *projectName, "candidates": len(suggestions.Candidates)})
	candidates, nearExisting := suggestions.Candidates, suggestions.NearExisting

	if jsonOut {
		return printJSON(out, map[string]any{"candidates": candidates, "near_existing": nearExisting})
	}
	if len(candidates) == 0 {
		fmt.Fprintln(out, "no candidate concepts found")
	}
	for i, c := range candidates {
		name := c.Term
		if len(c.Variants) > 0 {
			name = fmt.Sprintf("%s (+%s)", c.Term, strings.Join(c.Variants, ", "))
		}
		fmt.Fprintf(out, "%2d. %-24s  score=%.2f  occurrences=%d  items=%d\n", i+1, name, c.Score, c.Occurrences, c.Items)
	}
	if len(nearExisting) > 0 {
		fmt.Fprintln(out, "\nnear-existing (excluded from candidates):")
		for _, m := range nearExisting {
			fmt.Fprintf(out, "  %s  ~  %s  (distance %d)\n", m.Token, m.Concept, m.Distance)
		}
	}
	return nil
}

// conceptLinkSummary renders a ConceptUsage as "1 project(s), 2 record(s)",
// naming only the kinds that have links.
func conceptLinkSummary(u knowledge.ConceptUsage) string {
	var parts []string
	add := func(n int, what string) {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, what))
		}
	}
	add(u.Projects, "project(s)")
	add(u.Observations, "observation(s)")
	add(u.Records, "record(s)")
	add(u.DocumentSections, "document section(s)")
	return strings.Join(parts, ", ")
}

// cmdConceptDelete implements `concept delete NAME [--force] [--dry-run]`
// (DR-0038). It refuses a concept that still has links unless --force is
// given, previews with --dry-run, and always says the two things deleting
// cannot do: files that still name the concept recreate it on the next ingest,
// and a database that still has it brings it back on merge or import.
//
// `--` ends flag parsing, so a name that looks like a flag ("---", "-x") can
// be deleted: junk concepts are exactly the ones with such names.
func cmdConceptDelete(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	const usage = "usage: concept delete NAME [--force] [--dry-run]"
	flagArgs, afterDash := args, []string(nil)
	for i, a := range args {
		if a == "--" {
			flagArgs, afterDash = args[:i], args[i+1:]
			break
		}
	}
	var force, dryRun bool
	positional, err := splitFlags(flagArgs, nil, map[string]*bool{"--force": &force, "--dry-run": &dryRun})
	if errors.Is(err, flag.ErrHelp) {
		return err
	}
	if err != nil {
		return usageErrorf("%v; %s", err, usage)
	}
	positional = append(positional, afterDash...)
	if len(positional) != 1 {
		return usageErrorf("%s", usage)
	}
	name := positional[0]

	u, err := logKBCall(dl, "ConceptUsage", map[string]any{"name": name}, func() (knowledge.ConceptUsage, error) {
		return kb.ConceptUsage(name)
	})
	if err != nil {
		return err
	}

	var notes []string
	if u.Records > 0 || u.DocumentSections > 0 {
		notes = append(notes, "a record or document that still contains [[wikilink]] or lists it in tags or "+
			"keywords recreates the concept when that file is next ingested after it changes, or whenever "+
			"the database is rebuilt from the files; remove the mention from the file too")
	}
	if !dryRun {
		notes = append(notes, "kb merge or kb import from a database that still has this concept brings it "+
			"back; delete it there as well")
	}

	if !dryRun {
		u, err = logKBCall(dl, "DeleteConcept", map[string]any{"name": name, "force": force}, func() (knowledge.ConceptUsage, error) {
			return kb.DeleteConcept(name, force)
		})
		if err != nil {
			var inUse *knowledge.ConceptInUseError
			if errors.As(err, &inUse) {
				return negativef("concept %q is still linked to %s; nothing was deleted "+
					"(use --force to unlink and delete, or --dry-run to preview)", name, conceptLinkSummary(inUse.Usage))
			}
			return err
		}
	}

	if jsonOut {
		result := struct {
			Concept string `json:"concept"`
			Deleted bool   `json:"deleted"`
			DryRun  bool   `json:"dry_run"`
			Links   struct {
				Projects         int `json:"projects"`
				Observations     int `json:"observations"`
				Records          int `json:"records"`
				DocumentSections int `json:"document_sections"`
			} `json:"links"`
			Notes []string `json:"notes,omitempty"`
		}{Concept: name, Deleted: !dryRun, DryRun: dryRun, Notes: notes}
		result.Links.Projects, result.Links.Observations = u.Projects, u.Observations
		result.Links.Records, result.Links.DocumentSections = u.Records, u.DocumentSections
		return printJSON(out, result)
	}
	switch {
	case dryRun && u.Total() == 0:
		fmt.Fprintf(out, "would delete concept %q (no links)\n", name)
	case dryRun:
		fmt.Fprintf(out, "would delete concept %q (linked to %s; --force is needed to delete it)\n", name, conceptLinkSummary(u))
	case u.Total() == 0:
		fmt.Fprintf(out, "concept %q deleted\n", name)
	default:
		fmt.Fprintf(out, "concept %q deleted (unlinked from %s)\n", name, conceptLinkSummary(u))
	}
	for _, n := range notes {
		fmt.Fprintln(out, "note:", n)
	}
	return nil
}
