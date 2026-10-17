package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["story"] = cmdStory
}

// cmdStory implements `kb story timeline` and `kb story check` (DR-0075). Both
// only read; check reports and does not repair, and is advisory unless asked
// with --strict (DR-0078 item 4).
func cmdStory(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return usageErrorf("usage: story <timeline|check> --project P")
	}
	switch args[0] {
	case "timeline":
		return cmdStoryTimeline(kb, dl, jsonOut, args[1:], out)
	case "check":
		return cmdStoryCheck(kb, dl, jsonOut, args[1:], out)
	}
	return usageErrorf("unknown story subcommand %q", args[0])
}

// storyProject parses --project and the verb's own bool flags, and finds the
// project. It owns the usage errors the two subverbs share.
func storyProject(kb *knowledge.KnowledgeBase, args []string, usage string, bools map[string]*bool) (*knowledge.Project, error) {
	var name string
	positional, err := splitFlags(args, map[string]*string{"--project": &name}, bools)
	if errors.Is(err, flag.ErrHelp) {
		return nil, err
	}
	if err != nil {
		return nil, usageErrorf("%v; %s", err, usage)
	}
	if len(positional) != 0 || name == "" {
		return nil, usageErrorf("%s", usage)
	}
	p, err := kb.ProjectByName(name)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, notFoundf("unknown project %q", name)
	}
	return p, nil
}

// storyTimeText is the time column of a timeline row: the interval, the time as
// written when it has none, or a dash.
func storyTimeText(e knowledge.StoryEvent) string {
	switch {
	case e.Time.HasInterval() && e.Time.Start == e.Time.End:
		return e.Time.Start
	case e.Time.HasInterval():
		return e.Time.Start + ".." + e.Time.End
	case e.StoryTimeText != "":
		return e.StoryTimeText
	}
	return "-"
}

func storyLabel(e knowledge.StoryEvent) string {
	switch {
	case e.Label != "":
		return e.Label
	case e.SceneID != "":
		return e.SceneID
	}
	return "(event)"
}

func cmdStoryTimeline(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	narrative := false
	p, err := storyProject(kb, args, "usage: story timeline --project P [--narrative]", map[string]*bool{"--narrative": &narrative})
	if err != nil {
		return err
	}
	entries, err := logKBCall(dl, "StoryTimeline", map[string]any{"project": p.Name, "narrative": narrative}, func() ([]knowledge.TimelineEntry, error) {
		return kb.StoryTimeline(p.ID, narrative)
	})
	if err != nil {
		return err
	}
	if jsonOut {
		type row struct {
			Rank      int      `json:"rank"`
			Label     string   `json:"label"`
			SceneID   string   `json:"scene_id"`
			StoryTime string   `json:"story_time"`
			Start     string   `json:"start"`
			End       string   `json:"end"`
			Precision string   `json:"precision"`
			State     string   `json:"state"`
			Place     string   `json:"place"`
			POV       string   `json:"pov"`
			Status    string   `json:"status"`
			Chars     []string `json:"characters"`
			Document  string   `json:"document"`
			Unordered bool     `json:"unordered"`
			Undated   bool     `json:"undated"`
			InCycle   bool     `json:"in_cycle"`
		}
		rows := make([]row, 0, len(entries))
		for _, en := range entries {
			e := en.Event
			chars := e.Characters
			if chars == nil {
				chars = []string{}
			}
			rows = append(rows, row{en.Rank, e.Label, e.SceneID, e.StoryTimeText, e.Time.Start, e.Time.End, e.Time.Precision,
				e.State, e.Place, e.POV, e.Status, chars, en.DocumentPath, en.Unordered, en.Undated, en.InCycle})
		}
		return printJSON(out, rows)
	}
	if len(entries) == 0 {
		fmt.Fprintln(out, "no story events")
		return nil
	}
	var unordered, undated, cycle bool
	for _, en := range entries {
		mark := " "
		switch {
		case en.InCycle:
			mark, cycle = "!", true
		case en.Unordered:
			mark, unordered = "~", true
		case en.Undated:
			mark, undated = "?", true
		}
		fmt.Fprintf(out, "%3d %s %-24s %-22s %s\n", en.Rank, mark, storyTimeText(en.Event), storyLabel(en.Event), en.DocumentPath)
	}
	if unordered {
		fmt.Fprintln(out, "\n~ order after the row above is not established (overlapping or unrelated times)")
	}
	if undated && narrative {
		fmt.Fprintln(out, "? no date of its own (it may still be placed by a before:/after: relation)")
	} else if undated {
		fmt.Fprintln(out, "? no date and no relation to another event")
	}
	if cycle {
		fmt.Fprintln(out, "! in, or ordered behind, a cycle of before/after relations; see kb story check")
	}
	return nil
}

func cmdStoryCheck(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	strict := false
	p, err := storyProject(kb, args, "usage: story check --project P [--strict]", map[string]*bool{"--strict": &strict})
	if err != nil {
		return err
	}
	findings, err := logKBCall(dl, "StoryCheck", map[string]any{"project": p.Name}, func() ([]knowledge.StoryFinding, error) {
		return kb.StoryCheck(p.ID)
	})
	if err != nil {
		return err
	}
	if jsonOut {
		type row struct {
			Kind     string `json:"kind"`
			Document string `json:"document"`
			Message  string `json:"message"`
		}
		rows := make([]row, 0, len(findings))
		for _, f := range findings {
			rows = append(rows, row{f.Kind, f.Document, f.Message})
		}
		if err := printJSON(out, rows); err != nil {
			return err
		}
	} else if len(findings) == 0 {
		fmt.Fprintln(out, "no findings")
	} else {
		for _, f := range findings {
			if f.Document != "" {
				fmt.Fprintf(out, "%-18s %s: %s\n", f.Kind, f.Document, f.Message)
			} else {
				fmt.Fprintf(out, "%-18s %s\n", f.Kind, f.Message)
			}
		}
		fmt.Fprintf(out, "\n%d findings\n", len(findings))
	}
	if strict && len(findings) > 0 {
		return negativef("%d story findings", len(findings))
	}
	return nil
}
