package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["stats"] = cmdStats
}

// cmdStats implements `kb stats`: how many rows each table has, how many links
// each join table has, the checks a curator otherwise writes SQL for, and a
// line per project. It only reads. The counts are the report, so it exits 0
// whatever they say; `kb check-db` is the verb that reports drift.
func cmdStats(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	const usage = "usage: stats"
	positional, err := splitFlags(args, nil, nil)
	if errors.Is(err, flag.ErrHelp) {
		return err
	}
	if err != nil {
		return usageErrorf("%v; %s", err, usage)
	}
	if len(positional) != 0 {
		return usageErrorf("%s", usage)
	}
	s, err := logKBCall(dl, "Stats", map[string]any{}, kb.Stats)
	if err != nil {
		return err
	}
	if jsonOut {
		asMap := func(list []knowledge.StatCount) map[string]int {
			m := make(map[string]int, len(list))
			for _, c := range list {
				m[c.Name] = c.Count
			}
			return m
		}
		type project struct {
			Name         string `json:"name"`
			Observations int    `json:"observations"`
			Records      int    `json:"records"`
			Documents    int    `json:"documents"`
			Concepts     int    `json:"concepts"`
		}
		projects := make([]project, 0, len(s.Projects))
		for _, p := range s.Projects {
			projects = append(projects, project{p.Name, p.Observations, p.Records, p.Documents, p.Concepts})
		}
		return printJSON(out, map[string]any{
			"rows": asMap(s.Rows), "links": asMap(s.Links), "checks": asMap(s.Checks), "projects": projects,
		})
	}
	section := func(title string, list []knowledge.StatCount) {
		fmt.Fprintf(out, "%s:\n", title)
		for _, c := range list {
			fmt.Fprintf(out, "  %-30s %d\n", c.Name, c.Count)
		}
		fmt.Fprintln(out)
	}
	section("rows", s.Rows)
	section("links", s.Links)
	section("checks", s.Checks)
	fmt.Fprintln(out, "projects:")
	if len(s.Projects) == 0 {
		fmt.Fprintln(out, "  (none)")
	} else {
		fmt.Fprintf(out, "  %-24s %12s %8s %10s %9s\n", "name", "observations", "records", "documents", "concepts")
		for _, p := range s.Projects {
			fmt.Fprintf(out, "  %-24s %12d %8d %10d %9d\n", p.Name, p.Observations, p.Records, p.Documents, p.Concepts)
		}
	}
	return nil
}
