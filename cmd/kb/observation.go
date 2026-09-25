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

func cmdObservation(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return usageErrorf("usage: observation <add|list|show|update|sources|delete> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return cmdObservationAdd(kb, dl, jsonOut, rest, out)
	case "list":
		return cmdObservationList(kb, dl, jsonOut, rest, out)
	case "show":
		return cmdObservationShow(kb, dl, jsonOut, rest, out)
	case "update":
		return cmdObservationUpdate(kb, dl, jsonOut, rest, out)
	case "sources":
		return cmdObservationSources(kb, dl, jsonOut, rest, out)
	case "delete":
		return cmdObservationDelete(kb, dl, jsonOut, rest, out)
	default:
		return usageErrorf("unknown observation subcommand %q", sub)
	}
}

func cmdObservationAdd(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("observation add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project name (required)")
	sourceDOI := fs.String("source-doi", "", "normalized DOI of the source paper")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	rest := fs.Args()
	if *project == "" || len(rest) < 2 {
		return usageErrorf("usage: observation add --project NAME KIND BODY...")
	}
	p, err := logKBCall(dl, "ProjectByName", map[string]any{"name": *project}, func() (*knowledge.Project, error) {
		return kb.ProjectByName(*project)
	})
	if err != nil {
		return err
	}
	if p == nil {
		return notFoundf("project %q not found", *project)
	}
	kind := rest[0]
	body := strings.Join(rest[1:], " ")
	var id int64
	if *sourceDOI != "" {
		id, err = logKBCall(dl, "AddObservationWithSource", map[string]any{"project_id": p.ID, "kind": kind, "source_doi": *sourceDOI}, func() (int64, error) {
			return kb.AddObservationWithSource(p.ID, kind, body, *sourceDOI)
		})
	} else {
		id, err = logKBCall(dl, "AddObservation", map[string]any{"project_id": p.ID, "kind": kind}, func() (int64, error) {
			return kb.AddObservation(p.ID, kind, body)
		})
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

func cmdObservationList(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("observation list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project name (required)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := noExtraArgs(fs, "usage: observation list --project NAME"); err != nil {
		return err
	}
	if *project == "" {
		return usageErrorf("usage: observation list --project NAME")
	}
	p, err := logKBCall(dl, "ProjectByName", map[string]any{"name": *project}, func() (*knowledge.Project, error) {
		return kb.ProjectByName(*project)
	})
	if err != nil {
		return err
	}
	if p == nil {
		return notFoundf("project %q not found", *project)
	}
	obs, err := logKBCall(dl, "Observations", map[string]any{"project_id": p.ID}, func() ([]knowledge.Observation, error) {
		return kb.Observations(p.ID)
	})
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

// observationDetail is what `observation show` prints, in both JSON and text
// form — the observation itself plus its resolved supersession relations
// (DR-0023), mirroring recordDetail (record.go) one level down.
// *knowledge.Observation is embedded so its fields marshal at the top level,
// same as `observation show` returned before relations existed.
type observationDetail struct {
	*knowledge.Observation
	Supersedes   []int64 `json:"supersedes,omitempty"`
	SupersededBy []int64 `json:"superseded_by,omitempty"`
}

func cmdObservationShow(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	args, argErr := plainArgs(args, 1, 1, 1, "usage: observation show ID")
	if argErr != nil {
		return argErr
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return usageErrorf("invalid observation id %q", args[0])
	}
	o, err := logKBCall(dl, "ObservationByID", map[string]any{"id": id}, func() (*knowledge.Observation, error) {
		return kb.ObservationByID(id)
	})
	if err != nil {
		return notFoundf("observation %d not found", id)
	}
	rels, err := logKBCall(dl, "ObservationRelationsFor", map[string]any{"id": id}, func() ([]knowledge.RelatedObservation, error) {
		return kb.ObservationRelationsFor(id)
	})
	if err != nil {
		return err
	}
	detail := observationDetail{Observation: o}
	for _, rel := range rels {
		switch rel.Relationship {
		case "supersedes":
			detail.Supersedes = append(detail.Supersedes, rel.ObservationID)
		case "superseded_by":
			detail.SupersededBy = append(detail.SupersededBy, rel.ObservationID)
		}
	}

	if jsonOut {
		return printJSON(out, detail)
	}
	fmt.Fprintf(out, "[%s] %s\n", o.Kind, o.Body)
	if o.SourceDOI != "" {
		fmt.Fprintf(out, "  DOI (legacy): %s\n", o.SourceDOI)
	}
	for _, id := range detail.Supersedes {
		fmt.Fprintf(out, "  supersedes: %d\n", id)
	}
	for _, id := range detail.SupersededBy {
		fmt.Fprintf(out, "  superseded_by: %d\n", id)
	}
	return nil
}

// cmdObservationUpdate implements `observation update ID BODY...` (DR-0023):
// it never mutates row ID. It inserts a new observation with BODY, inheriting
// ID's project and kind, and links the two via a supersedes edge, so the
// original wording survives unchanged and the correction is just the new
// row plus this edge.
func cmdObservationUpdate(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) < 2 {
		return usageErrorf("usage: observation update ID BODY...")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return usageErrorf("invalid observation id %q", args[0])
	}
	body := strings.Join(args[1:], " ")

	old, err := logKBCall(dl, "ObservationByID", map[string]any{"id": id}, func() (*knowledge.Observation, error) {
		return kb.ObservationByID(id)
	})
	if err != nil {
		return notFoundf("observation %d not found", id)
	}
	newID, err := logKBCall(dl, "AddObservation", map[string]any{"project_id": old.ProjectID, "kind": old.Kind}, func() (int64, error) {
		return kb.AddObservation(old.ProjectID, old.Kind, body)
	})
	if err != nil {
		return err
	}
	if err := logKBCallErr(dl, "AddObservationRelation", map[string]any{"from_id": newID, "to_id": id, "relationship": "supersedes"}, func() error {
		return kb.AddObservationRelation(newID, id, "supersedes")
	}); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			ID         int64 `json:"id"`
			Supersedes int64 `json:"supersedes"`
		}{ID: newID, Supersedes: id})
	}
	fmt.Fprintf(out, "observation recorded (id=%d), supersedes %d\n", newID, id)
	return nil
}

func cmdObservationSources(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	args, argErr := plainArgs(args, 1, 1, 1, "usage: observation sources ID")
	if argErr != nil {
		return argErr
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return usageErrorf("invalid observation id %q", args[0])
	}
	sources, err := logKBCall(dl, "ObservationSources", map[string]any{"observation_id": id}, func() ([]knowledge.Source, error) {
		return kb.ObservationSources(id)
	})
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
