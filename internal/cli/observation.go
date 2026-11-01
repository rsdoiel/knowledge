package cli

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
		return usageErrorf("usage: observation <add|list|show|update|review|sources|delete> ...")
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
	case "review":
		return cmdObservationReview(kb, dl, jsonOut, rest, out)
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
	draft := fs.Bool("draft", false, "record a draft a person must promote (needs --by)")
	by := fs.String("by", "", "who drafted it, e.g. \"harvey qwen2.5:3b\" (with --draft)")
	confidence := fs.String("confidence", "", "the drafter's estimate, 0 to 1 (with --draft)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	rest := fs.Args()
	const addUsage = "usage: observation add --project NAME [--draft --by WHO [--confidence N]] KIND BODY..."
	if *project == "" || len(rest) < 2 {
		return usageErrorf(addUsage)
	}
	var conf *float64
	if *draft {
		if strings.TrimSpace(*by) == "" || strings.ContainsAny(*by, "\r\n") {
			return usageErrorf("--draft needs --by WHO, one non-blank line (%s)", addUsage)
		}
		if *sourceDOI != "" {
			return usageErrorf("--source-doi cannot be used with --draft; link the source after the draft is promoted")
		}
		if *confidence != "" {
			c, perr := strconv.ParseFloat(*confidence, 64)
			if perr != nil || c != c {
				return usageErrorf("invalid --confidence %q", *confidence)
			}
			if c < 0 || c > 1 {
				return usageErrorf("--confidence must be between 0 and 1, got %v", c)
			}
			conf = &c
		}
	} else if *by != "" || *confidence != "" {
		return usageErrorf("--by and --confidence are for a draft; add --draft (%s)", addUsage)
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
	if *draft {
		id, err = logKBCall(dl, "AddObservationDraft", map[string]any{"project_id": p.ID, "kind": kind, "by": *by}, func() (int64, error) {
			return kb.AddObservationDraft(p.ID, kind, body, *by, conf)
		})
		if err != nil {
			return err
		}
		if jsonOut {
			return printJSON(out, struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			}{ID: id, Status: "drafted"})
		}
		fmt.Fprintf(out, "observation drafted (id=%d, kind=%s); a person promotes it with: kb observation review promote %d\n", id, kind, id)
		return nil
	}
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
	status := fs.String("status", "reviewed", "reviewed, drafted or all")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := noExtraArgs(fs, "usage: observation list --project NAME [--status reviewed|drafted|all]"); err != nil {
		return err
	}
	if *project == "" {
		return usageErrorf("usage: observation list --project NAME [--status reviewed|drafted|all]")
	}
	switch *status {
	case "reviewed", "drafted", "all":
	default:
		return usageErrorf("invalid --status %q; want reviewed, drafted or all", *status)
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
	obs, err := logKBCall(dl, "ObservationsByStatus", map[string]any{"project_id": p.ID, "status": *status}, func() ([]knowledge.Observation, error) {
		return kb.ObservationsByStatus(p.ID, *status)
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
		mark := ""
		if o.Status == "drafted" {
			mark = " (draft)"
		}
		fmt.Fprintf(out, "%-4d  [%-10s] %s%s\n", o.ID, o.Kind, o.Body, mark)
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
	if o.Status == "drafted" {
		fmt.Fprintf(out, "  status: drafted by %s", o.GeneratedBy)
		if o.Confidence != nil {
			fmt.Fprintf(out, " (confidence %g)", *o.Confidence)
		}
		fmt.Fprintln(out)
	}
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
		if old.Status == "drafted" {
			// Updating a draft gives a draft by the same drafter; only
			// promote makes a row trusted (DR-0090 item 6).
			return kb.AddObservationDraft(old.ProjectID, old.Kind, body, old.GeneratedBy, nil)
		}
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

// cmdObservationReview implements `kb observation review list|promote`.
func cmdObservationReview(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return usageErrorf("usage: observation review <list|promote> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return cmdObservationReviewList(kb, dl, jsonOut, rest, out)
	case "promote":
		return cmdObservationReviewPromote(kb, dl, jsonOut, rest, out)
	default:
		return usageErrorf("unknown observation review subverb %q; want list or promote", sub)
	}
}

// cmdObservationReviewList implements `kb observation review list
// [--project P]`: the drafts awaiting a person, oldest first.
func cmdObservationReviewList(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	var projectName string
	positional, err := splitFlags(args, map[string]*string{"--project": &projectName}, nil)
	if err != nil {
		return err
	}
	if len(positional) != 0 {
		return usageErrorf("usage: observation review list [--project P]")
	}
	var projectID int64
	names := map[int64]string{}
	if projectName != "" {
		p, err := kb.ProjectByName(projectName)
		if err != nil || p == nil {
			return notFoundf("unknown project %q", projectName)
		}
		projectID = p.ID
	}
	if ps, err := kb.Projects(); err == nil {
		for _, p := range ps {
			names[p.ID] = p.Name
		}
	}
	items, err := logKBCall(dl, "ObservationReviewQueue", map[string]any{"project_id": projectID}, func() ([]knowledge.Observation, error) {
		return kb.ObservationReviewQueue(projectID)
	})
	if err != nil {
		return err
	}
	if jsonOut {
		if items == nil {
			items = []knowledge.Observation{}
		}
		return printJSON(out, items)
	}
	if len(items) == 0 {
		fmt.Fprintln(out, "(no drafts awaiting review)")
		return nil
	}
	for _, o := range items {
		conf := ""
		if o.Confidence != nil {
			conf = fmt.Sprintf(" %g", *o.Confidence)
		}
		fmt.Fprintf(out, "%-4d  %s  [%s] by %s%s: %s\n", o.ID, names[o.ProjectID], o.Kind, o.GeneratedBy, conf, o.Body)
	}
	return nil
}

// cmdObservationReviewPromote implements `kb observation review promote ID`.
func cmdObservationReviewPromote(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	positional, err := splitFlags(args, nil, nil)
	if err != nil {
		return err
	}
	if len(positional) != 1 {
		return usageErrorf("usage: observation review promote ID")
	}
	id, err := strconv.ParseInt(positional[0], 10, 64)
	if err != nil {
		return usageErrorf("invalid observation id %q", positional[0])
	}
	// Promoting is what makes a draft trusted, searchable and recalled into a
	// model's prompt, so it is a person's act (DR-0070, DR-0090): it needs a
	// terminal and nothing turns that off. The check comes before the lookup.
	if !atTerminal(out) {
		return usageErrorf("a draft can only be promoted by a person at a terminal: run kb observation review promote %d from an interactive shell (standard input and output must be a terminal)", id)
	}
	if err := logKBCallErr(dl, "PromoteObservation", map[string]any{"id": id}, func() error {
		return kb.PromoteObservation(id)
	}); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		}{ID: id, Status: "reviewed"})
	}
	fmt.Fprintf(out, "observation %d reviewed\n", id)
	return nil
}
