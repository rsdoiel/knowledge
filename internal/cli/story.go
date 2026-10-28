package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["story"] = cmdStory
}

// cmdStory implements `kb story timeline`, `check`, `who-knows` and `who-learns`
// (DR-0075, DR-0083). All only read; check reports and does not repair, and is
// advisory unless asked with --strict (DR-0078 item 4).
func cmdStory(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return usageErrorf("usage: story <timeline|check|who-knows|who-learns> --project P")
	}
	switch args[0] {
	case "timeline":
		return cmdStoryTimeline(kb, dl, jsonOut, args[1:], out)
	case "check":
		return cmdStoryCheck(kb, dl, jsonOut, args[1:], out)
	case "who-knows":
		return cmdStoryWhoKnows(kb, dl, jsonOut, args[1:], out)
	case "who-learns":
		return cmdStoryWhoLearns(kb, dl, jsonOut, args[1:], out)
	}
	return usageErrorf("unknown story subcommand %q", args[0])
}

// storyProject parses --project and the verb's own bool flags, and finds the
// project. It owns the usage errors the story subverbs share. It takes no
// positional argument; see storyArgs for the ones that do.
func storyProject(kb *knowledge.KnowledgeBase, args []string, usage string, bools map[string]*bool) (*knowledge.Project, error) {
	p, _, err := storyArgs(kb, args, usage, bools, 0, 0)
	return p, err
}

// storyArgs is storyProject for a subverb that takes between min and max
// positional arguments, which it returns. They may come before or after
// --project.
func storyArgs(kb *knowledge.KnowledgeBase, args []string, usage string, bools map[string]*bool, min, max int) (*knowledge.Project, []string, error) {
	var name string
	positional, err := splitFlags(args, map[string]*string{"--project": &name}, bools)
	if errors.Is(err, flag.ErrHelp) {
		return nil, nil, err
	}
	if err != nil {
		return nil, nil, usageErrorf("%v; %s", err, usage)
	}
	if len(positional) < min || len(positional) > max || name == "" {
		return nil, nil, usageErrorf("%s", usage)
	}
	p, err := kb.ProjectByName(name)
	if err != nil {
		return nil, nil, err
	}
	if p == nil {
		return nil, nil, notFoundf("unknown project %q", name)
	}
	return p, positional, nil
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

// knownFactRow is a fact in the JSON of who-knows.
type knownFactRow struct {
	Fact      string `json:"fact"`
	LearnedIn string `json:"learned_in"`
	SceneID   string `json:"scene_id"`
	Document  string `json:"document"`
}

func knownFactRows(facts []knowledge.KnownFact) []knownFactRow {
	rows := make([]knownFactRow, 0, len(facts))
	for _, f := range facts {
		rows = append(rows, knownFactRow{f.Fact, f.LearnedIn, f.SceneID, f.Document})
	}
	return rows
}

func printKnownFacts(out io.Writer, facts []knowledge.KnownFact) {
	for _, f := range facts {
		fmt.Fprintf(out, "  %-40s %-18s %s\n", f.Fact, f.LearnedIn, f.Document)
	}
}

// cmdStoryWhoKnows implements `kb story who-knows CHARACTER [SCENE]` (DR-0083
// item 5): what a character has learned, in story order, up to a scene.
func cmdStoryWhoKnows(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	p, pos, err := storyArgs(kb, args, "usage: story who-knows CHARACTER [SCENE] --project P", nil, 1, 2)
	if err != nil {
		return err
	}
	scene := ""
	if len(pos) == 2 {
		scene = pos[1]
	}
	res, err := logKBCall(dl, "WhoKnows", map[string]any{"project": p.Name, "character": pos[0], "scene": scene}, func() (knowledge.WhoKnowsResult, error) {
		return kb.WhoKnows(p.ID, pos[0], scene)
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			Character string         `json:"character"`
			Scene     string         `json:"scene"`
			Facts     []knownFactRow `json:"facts"`
			Unordered []knownFactRow `json:"unordered"`
		}{res.Character, res.Scene, knownFactRows(res.Facts), knownFactRows(res.Unordered)})
	}
	switch {
	case res.Scene != "":
		fmt.Fprintf(out, "%s knows at %s:\n", res.Character, res.Scene)
	default:
		fmt.Fprintf(out, "%s learns:\n", res.Character)
	}
	if len(res.Facts) == 0 {
		fmt.Fprintln(out, "  (nothing)")
	}
	printKnownFacts(out, res.Facts)
	if len(res.Unordered) > 0 {
		fmt.Fprintf(out, "\nNot ordered against %s (learned in a scene whose order is not established):\n", res.Scene)
		printKnownFacts(out, res.Unordered)
	}
	return nil
}

// cmdStoryWhoLearns implements `kb story who-learns FACT` (DR-0083 item 5): who
// learns a fact, and in which scene, in story order.
func cmdStoryWhoLearns(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	p, pos, err := storyArgs(kb, args, "usage: story who-learns FACT --project P", nil, 1, 1)
	if err != nil {
		return err
	}
	learners, err := logKBCall(dl, "WhoLearns", map[string]any{"project": p.Name, "fact": pos[0]}, func() ([]knowledge.Learner, error) {
		return kb.WhoLearns(p.ID, pos[0])
	})
	if err != nil {
		return err
	}
	fact := strings.TrimSpace(pos[0])
	if jsonOut {
		type row struct {
			Character string `json:"character"`
			LearnedIn string `json:"learned_in"`
			SceneID   string `json:"scene_id"`
			Document  string `json:"document"`
		}
		rows := make([]row, 0, len(learners))
		for _, l := range learners {
			rows = append(rows, row{l.Character, l.LearnedIn, l.SceneID, l.Document})
		}
		return printJSON(out, struct {
			Fact     string `json:"fact"`
			Learners []row  `json:"learners"`
		}{fact, rows})
	}
	fmt.Fprintf(out, "%s\n", fact)
	for _, l := range learners {
		fmt.Fprintf(out, "  %-20s %-18s %s\n", l.Character, l.LearnedIn, l.Document)
	}
	return nil
}
