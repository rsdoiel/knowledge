package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

// recallStdin is where `concept recall -` reads its text; tests replace it.
var recallStdin io.Reader = os.Stdin

// excerpt collapses text to one line of at most n runes, for listings.
func excerpt(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// parseLimitFlag turns a --limit value into a non-negative count; a bad value
// on the command line is a usage error (exit 2).
func parseLimitFlag(v string) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, usageErrorf("invalid --limit %q; want zero or more", v)
	}
	return n, nil
}

// splitAtDoubleDash separates args before a bare "--" from those after it.
func splitAtDoubleDash(args []string) (before, after []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

// cmdConceptShow implements `concept show NAME [--limit N]` (DR-0051): one
// concept's description, link counts and up to N items of each kind, newest
// first. The name matches exactly, as `concept delete` does.
func cmdConceptShow(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	const usage = "usage: concept show NAME [--limit N]"
	flagArgs, afterDash := splitAtDoubleDash(args)
	limitArg := "10"
	positional, err := splitFlags(flagArgs, map[string]*string{"--limit": &limitArg}, nil)
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
	limit, err := parseLimitFlag(limitArg)
	if err != nil {
		return err
	}
	name := positional[0]
	d, err := logKBCall(dl, "ConceptDetail", map[string]any{"name": name, "limit": limit}, func() (knowledge.ConceptDetailResult, error) {
		return kb.ConceptDetail(name, limit)
	})
	if err != nil {
		return err
	}

	kinds := []struct {
		key, label string
		count      int
		items      []knowledge.ConceptDetailItem
	}{
		{"projects", "projects", d.Usage.Projects, d.Projects},
		{"observations", "observations", d.Usage.Observations, d.Observations},
		{"records", "records", d.Usage.Records, d.Records},
		{"document_sections", "document sections", d.Usage.DocumentSections, d.DocumentSections},
	}
	if jsonOut {
		type item struct {
			ID        int64  `json:"id"`
			Project   string `json:"project,omitempty"`
			Title     string `json:"title,omitempty"`
			Excerpt   string `json:"excerpt,omitempty"`
			CreatedAt string `json:"created_at,omitempty"`
		}
		result := map[string]any{
			"name":             d.Concept.Name,
			"description":      d.Concept.Description,
			"kind":             d.Concept.Kind,
			"identifier_type":  d.Concept.IdentifierType,
			"identifier_value": d.Concept.IdentifierValue,
		}
		counts := map[string]int{}
		for _, k := range kinds {
			counts[k.key] = k.count
			items := make([]item, 0, len(k.items))
			for _, it := range k.items {
				ts := ""
				if !it.CreatedAt.IsZero() {
					ts = it.CreatedAt.Format("2006-01-02")
				}
				items = append(items, item{it.ID, it.Project, it.Title, excerpt(it.Body, 120), ts})
			}
			result[k.key] = items
		}
		result["counts"] = counts
		type span struct {
			Project          string `json:"project"`
			Tagged           bool   `json:"tagged"`
			Observations     int    `json:"observations"`
			Records          int    `json:"records"`
			DocumentSections int    `json:"document_sections"`
		}
		spans := make([]span, 0, len(d.Spans))
		for _, sp := range d.Spans {
			spans = append(spans, span{sp.Project, sp.Tagged, sp.Observations, sp.Records, sp.DocumentSections})
		}
		result["spans"] = spans
		return printJSON(out, result)
	}

	fmt.Fprintf(out, "concept: %s\n", d.Concept.Name)
	if d.Concept.Description != "" {
		fmt.Fprintf(out, "description: %s\n", d.Concept.Description)
	}
	if d.Concept.Kind != "" && d.Concept.Kind != "other" {
		fmt.Fprintf(out, "kind: %s\n", d.Concept.Kind)
	}
	if d.Concept.IdentifierType != "" || d.Concept.IdentifierValue != "" {
		fmt.Fprintf(out, "identifier: %s %s\n", d.Concept.IdentifierType, d.Concept.IdentifierValue)
	}
	if len(d.Spans) > 0 {
		noun := "projects"
		if len(d.Spans) == 1 {
			noun = "project"
		}
		fmt.Fprintf(out, "\nspans %d %s:\n", len(d.Spans), noun)
		for _, sp := range d.Spans {
			var parts []string
			if sp.Tagged {
				parts = append(parts, "tagged")
			}
			for _, c := range []struct {
				n     int
				label string
			}{{sp.Observations, "observations"}, {sp.Records, "records"}, {sp.DocumentSections, "document sections"}} {
				if c.n > 0 {
					parts = append(parts, fmt.Sprintf("%d %s", c.n, c.label))
				}
			}
			fmt.Fprintf(out, "  %s  %s\n", sp.Project, strings.Join(parts, ", "))
		}
	}
	for _, k := range kinds {
		if len(k.items) < k.count && limit > 0 {
			fmt.Fprintf(out, "\n%s (%d, showing %d):\n", k.label, k.count, len(k.items))
		} else {
			fmt.Fprintf(out, "\n%s (%d):\n", k.label, k.count)
		}
		for _, it := range k.items {
			line := fmt.Sprintf("  #%d", it.ID)
			if it.Project != "" && k.key != "projects" {
				line += "  " + it.Project
			}
			if k.key == "projects" {
				line = "  " + it.Project
			}
			if !it.CreatedAt.IsZero() {
				line += "  " + it.CreatedAt.Format("2006-01-02")
			}
			text := it.Title
			if text != "" && it.Body != "" && k.key != "records" {
				text += ": " + it.Body
			} else if text == "" {
				text = it.Body
			}
			if text != "" {
				line += "  " + excerpt(text, 100)
			}
			fmt.Fprintln(out, line)
		}
	}
	return nil
}

// cmdConceptRecall implements `concept recall [TEXT... | -] [--concept
// NAME,...] [--project NAME] [--limit N]` (DR-0051): the items linked to the
// concepts found in the text, plus any named with --concept, ranked by how
// many of those concepts each matches. Text and --concept names are unioned.
// Nothing matching is exit 1; nothing supplied is exit 2.
func cmdConceptRecall(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	const usage = "usage: concept recall [TEXT... | -] [--concept NAME,...] [--project NAME] [--limit N]"
	var conceptArg, project string
	limitArg := "10"
	// A bare "-" means stdin; splitFlags would call it an unknown flag. A flag's
	// value is never "-" in practice, so removing it here is safe.
	fromStdin := false
	var rest []string
	for _, a := range args {
		if a == "-" {
			fromStdin = true
			continue
		}
		rest = append(rest, a)
	}
	positional, err := splitFlags(rest, map[string]*string{"--concept": &conceptArg, "--project": &project, "--limit": &limitArg}, nil)
	if errors.Is(err, flag.ErrHelp) {
		return err
	}
	if err != nil {
		return usageErrorf("%v; %s", err, usage)
	}
	limit, err := parseLimitFlag(limitArg)
	if err != nil {
		return err
	}
	var names []string
	for _, n := range strings.Split(conceptArg, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	text := strings.Join(positional, " ")
	if fromStdin {
		raw, rerr := io.ReadAll(recallStdin)
		if rerr != nil {
			return ioErrorf("read stdin: %v", rerr)
		}
		text = strings.TrimSpace(text + " " + string(raw))
	}
	if strings.TrimSpace(text) == "" && len(names) == 0 {
		return usageErrorf("%s (no text and no --concept given)", usage)
	}

	if strings.TrimSpace(text) != "" {
		matched, merr := kb.MatchConceptNames(text)
		if merr != nil {
			return merr
		}
		names = append(matched, names...)
	}
	r, err := logKBCall(dl, "RecallByNames", map[string]any{"names": names, "limit": limit, "project": project}, func() (knowledge.TextRecall, error) {
		return kb.RecallByNames(names, limit, project)
	})
	if err != nil {
		return err
	}
	if len(r.Matched) == 0 {
		return negativef("no concept matched; nothing to recall")
	}

	if jsonOut {
		type hit struct {
			Kind     string   `json:"kind"`
			ID       int64    `json:"id"`
			Project  string   `json:"project"`
			Concepts []string `json:"concepts"`
			Title    string   `json:"title,omitempty"`
			Excerpt  string   `json:"excerpt"`

			// Provenance (DR-0080).
			Sources   []jsonSource  `json:"sources"`
			Material  *jsonMaterial `json:"material,omitempty"`
			Retracted bool          `json:"retracted"`
		}
		hits := make([]hit, 0, len(r.Hits))
		for _, h := range r.Hits {
			hits = append(hits, hit{h.SourceType, h.ID, h.Project, h.Concepts, h.Title, excerpt(h.Body, 120),
				jsonSources(h.HitProvenance), jsonMaterialOf(h.HitProvenance), h.Retracted})
		}
		projects := r.Projects
		if projects == nil {
			projects = []string{}
		}
		return printJSON(out, map[string]any{"matched": r.Matched, "hits": hits, "projects": projects})
	}
	fmt.Fprintf(out, "matched concepts: %s\n", strings.Join(r.Matched, ", "))
	if len(r.Projects) > 0 {
		fmt.Fprintf(out, "projects: %s\n", strings.Join(r.Projects, ", "))
	}
	if len(r.Hits) == 0 {
		fmt.Fprintln(out, "(no linked observations, records or document sections)")
	}
	for _, h := range r.Hits {
		text := h.Title
		if text != "" && h.Body != "" {
			text += ": "
		}
		text += h.Body
		if h.SourceType == "document_section" || h.SourceType == "document_gist" {
			if h.SummaryStatus != "reviewed" && h.Body == "" {
				text += " (" + h.SummaryStatus + ")"
			}
		}
		proj := h.Project
		if proj == "" {
			proj = "(workspace)"
		}
		fmt.Fprintf(out, "%s #%d  %s  [%s]  %s%s\n", h.SourceType, h.ID, proj, strings.Join(h.Concepts, ", "), excerpt(text, 100),
			retractionMarker(h.HitProvenance))
	}
	return nil
}
