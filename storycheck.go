package knowledge

import (
	"fmt"
	"sort"
	"strings"
)

/*
* StoryFinding is one thing StoryCheck noticed about a work (DR-0075 item 10).
  - It is a report, never a repair: nothing in the knowledge base or the
  - manuscript is changed because of it.
    *
  - Kind is one of "missing-file", "unnumbered-scene", "scene-comment-no-id", "scene-id-disagreement", "scene-id-repeated",
    "scene-comment-syntax", "fountain-style-heading", "front-matter-ignored", "duplicate-label",
  - "unresolved-time", "unparseable-time", "unknown-reference", "contradiction",
  - "cycle", "two-places", "foreshadow-order", "knowledge-before-learned", "unknown-fact", "fact-number-differs" or "late-source". Document is the file path, and
  - Message says what and where.
    *
  - Example:
  - fs, _ := kb.StoryCheck(pid)
  - for _, f := range fs { fmt.Printf("%s: %s: %s\n", f.Kind, f.Document, f.Message) }
*/
type StoryFinding struct {
	Kind     string
	Document string
	Message  string
}

// storyFindingOrder is the order kinds are listed in: file problems first,
// then identity, then time, then continuity.
var storyFindingOrder = []string{
	"missing-file", "unnumbered-scene", "duplicate-label", "unparseable-time", "unresolved-time",
	"unknown-reference", "contradiction", "cycle", "two-places", "foreshadow-order",
	"knowledge-before-learned", "unknown-fact", "fact-number-differs", "late-source",
}

func storyEventName(e StoryEvent) string {
	switch {
	case e.Label != "":
		return e.Label
	case e.SceneID != "":
		return "scene " + e.SceneID
	}
	return "(unlabelled event)"
}

/** StoryCheck reports continuity problems in a project's story (DR-0075
 * item 10, DR-0079), changing nothing:
 *
 *   - missing-file: a Fountain document whose file cannot be read, so its
 *     scene numbers could not be checked.
 *   - unnumbered-scene: a Fountain scene with no scene number, which therefore
 *     has no event.
 *   - scene-comment-no-id, scene-id-disagreement, scene-id-repeated,
 *     scene-comment-syntax, fountain-style-heading, front-matter-ignored: a
 *     Markdown file's scenes are not what they seem to say (DR-0082): a story
 *     comment on a heading with no id, a heading {#id} and a comment id that
 *     differ, a scene id used twice, an unknown key or a word with no "=" in a
 *     comment, a heading ending like a Fountain "#12A#" (which is text, not an
 *     id), and a front matter event or story_time the file's scene ids make
 *     moot.
 *   - duplicate-label: one label on several events.
 *   - unparseable-time, unresolved-time: a story_time that is not understood, or
 *     a relative one whose anchor is missing, ambiguous, undated or cyclic.
 *   - unknown-reference: a foreshadows, pays_off or precedes name that names no
 *     single event.
 *   - contradiction: a declared order (before:, after:, precedes) that the
 *     events' own dates contradict.
 *   - cycle: events whose declared order cannot be satisfied.
 *   - two-places: a character in two different places at the same day or
 *     ordinal; wider intervals overlap innocently and are not flagged.
 *   - foreshadow-order: within one document, a scene that foreshadows an
 *     earlier scene, or pays off a later one. Across documents the reading
 *     order is not known and nothing is said.
 *   - knowledge-before-learned: a scene says a character knows a fact, and every
 *     scene where that character learns it is provably later in story order. A
 *     knows with no learns anywhere is backstory and is not reported, nor is one
 *     whose learning is unordered against it (DR-0083).
 *   - unknown-fact: two different facts, each of at least 12 characters, that are
 *     one edit apart, which is probably one fact typed twice. Reported, never
 *     merged.
 *   - fact-number-differs: the same, where the one edit is a digit ("41 coins",
 *     "42 coins"): two figures that may be two facts or a continuity slip, not a
 *     typo. Reported, never merged.
 *   - late-source: a document cites a source with the relationship "evidence"
 *     whose published_date is after the end of one of the document's events. A
 *     source with no date, or an event with no calendar interval, gives nothing.
 *
 * Parameters:
 *   projectID (int64) — the project.
 *
 * Returns:
 *   []StoryFinding — ordered by kind, document and message; empty (not nil)
 *     when nothing is wrong.
 *   error          — on database failure.
 *
 * Example:
 *   fs, err := kb.StoryCheck(pid)
 */
func (kb *KnowledgeBase) StoryCheck(projectID int64) ([]StoryFinding, error) {
	evs, err := kb.StoryEvents(projectID)
	if err != nil {
		return nil, err
	}
	docs, err := kb.Documents(projectID)
	if err != nil {
		return nil, err
	}
	path := map[int64]string{}
	for _, d := range docs {
		path[d.ID] = d.Path
	}
	out := []StoryFinding{}
	add := func(kind, doc, format string, a ...any) {
		out = append(out, StoryFinding{Kind: kind, Document: doc, Message: fmt.Sprintf(format, a...)})
	}

	// Scenes with no number: the one check that reads the files, because a
	// scene with no number has no event to look at.
	for _, d := range docs {
		if d.Format != "fountain" {
			continue
		}
		parsed, err := ParseDocumentFile(d.Path, d.Format)
		if err != nil {
			add("missing-file", d.Path, "cannot read the file, so its scene numbers were not checked: %v", err)
			continue
		}
		for _, s := range parsed.Sections {
			if s.Heading != "" && s.SceneID == "" {
				add("unnumbered-scene", d.Path, "scene %q has no scene number, so it is not an event", s.Heading)
			}
		}
	}

	// Markdown scenes (DR-0082 item 7): the parse warnings that mean a scene
	// is not what the writer meant, and a scene id repeated in one file.
	for _, d := range docs {
		if d.Format != "markdown" {
			continue
		}
		parsed, err := ParseDocumentFile(d.Path, d.Format)
		if err != nil {
			add("missing-file", d.Path, "cannot read the file, so its scenes were not checked: %v", err)
			continue
		}
		for _, w := range parsed.StoryWarnings {
			if kind := markdownWarningKind(w); kind != "" {
				add(kind, d.Path, "%s", w)
			}
		}
		seen := map[string]bool{}
		for _, s := range parsed.Sections {
			if s.SceneID == "" {
				continue
			}
			if seen[s.SceneID] {
				add("scene-id-repeated", d.Path, "scene id %q is repeated; the first scene is the event, %q is not", s.SceneID, s.Heading)
			}
			seen[s.SceneID] = true
		}
	}

	byLabel := map[string][]int{}
	for i, e := range evs {
		if e.Label != "" {
			byLabel[e.Label] = append(byLabel[e.Label], i)
		}
	}
	for l, idx := range byLabel {
		if len(idx) > 1 {
			var where []string
			for _, i := range idx {
				where = append(where, path[evs[i].DocumentID])
			}
			sort.Strings(where)
			add("duplicate-label", where[0], "label %q is on %d events (%s)", l, len(idx), strings.Join(where, ", "))
		}
	}

	for _, e := range evs {
		doc := path[e.DocumentID]
		switch e.State {
		case "none":
			if e.Note != "" {
				add("unparseable-time", doc, "%s: story_time %q: %s", storyEventName(e), e.StoryTimeText, e.Note)
			}
		case "unresolved":
			add("unresolved-time", doc, "%s: story_time %q is not resolved: %s", storyEventName(e), e.StoryTimeText, e.Note)
		}
		for _, ref := range []struct {
			key   string
			names []string
		}{{"foreshadows", e.Foreshadows}, {"pays_off", e.PaysOff}, {"precedes", e.Precedes}} {
			for _, n := range ref.names {
				switch c := len(byLabel[n]); {
				case c == 0:
					add("unknown-reference", doc, "%s: %s %q names no event", storyEventName(e), ref.key, n)
				case c > 1:
					add("unknown-reference", doc, "%s: %s %q names %d events", storyEventName(e), ref.key, n, c)
				}
			}
		}
	}

	g := newStoryGraph(evs)
	for i, tos := range g.declared {
		for _, j := range tos {
			if storyKeyBefore(evs[j], evs[i]) {
				add("contradiction", path[evs[i].DocumentID], "%s is declared before %s, but its date is after it", storyEventName(evs[i]), storyEventName(evs[j]))
			}
		}
	}
	// A cycle among the declared orders only: intervals can never form one.
	indeg := make([]int, len(evs))
	for _, tos := range g.declared {
		for _, j := range tos {
			indeg[j]++
		}
	}
	var queue []int
	for i := range evs {
		if indeg[i] == 0 {
			queue = append(queue, i)
		}
	}
	removed := make([]bool, len(evs))
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		removed[n] = true
		for _, j := range g.declared[n] {
			if indeg[j]--; indeg[j] == 0 {
				queue = append(queue, j)
			}
		}
	}
	var stuck []string
	stuckDoc := ""
	for i, e := range evs {
		if !removed[i] && len(g.declared[i]) > 0 {
			stuck = append(stuck, storyEventName(e))
			if stuckDoc == "" {
				stuckDoc = path[e.DocumentID]
			}
		}
	}
	if len(stuck) > 0 {
		add("cycle", stuckDoc, "the declared order cannot be satisfied: %s", strings.Join(stuck, ", "))
	}

	type moment struct{ who, mode, at string }
	places := map[moment]map[string][]string{}
	for _, e := range evs {
		if e.Place == "" || !e.Time.HasInterval() || e.Time.Start != e.Time.End {
			continue
		}
		mode := "date"
		if storyOrdinalMode(e) {
			mode = "ordinal"
		}
		for _, who := range e.Characters {
			k := moment{who, mode, e.Time.Start}
			if places[k] == nil {
				places[k] = map[string][]string{}
			}
			p := strings.ToLower(strings.TrimSpace(e.Place))
			places[k][p] = append(places[k][p], storyEventName(e))
		}
	}
	for k, byPlace := range places {
		if len(byPlace) < 2 {
			continue
		}
		var parts []string
		for p, names := range byPlace {
			sort.Strings(names)
			parts = append(parts, fmt.Sprintf("%s (%s)", p, strings.Join(names, ", ")))
		}
		sort.Strings(parts)
		add("two-places", "", "%s is in %d places at %s: %s", k.who, len(byPlace), k.at, strings.Join(parts, "; "))
	}

	unique := uniqueLabels(evs)
	for _, e := range evs {
		for _, n := range e.Foreshadows {
			if j, ok := unique[n]; ok && evs[j].DocumentID == e.DocumentID && evs[j].Seq < e.Seq {
				add("foreshadow-order", path[e.DocumentID], "%s foreshadows %s, which comes earlier in the document", storyEventName(e), storyEventName(evs[j]))
			}
		}
		for _, n := range e.PaysOff {
			if j, ok := unique[n]; ok && evs[j].DocumentID == e.DocumentID && evs[j].Seq > e.Seq {
				add("foreshadow-order", path[e.DocumentID], "%s pays off %s, which comes later in the document", storyEventName(e), storyEventName(evs[j]))
			}
		}
	}

	more, err := kb.knowledgeFindings(projectID, evs, g, path)
	if err != nil {
		return nil, err
	}
	out = append(out, more...)

	rank := map[string]int{}
	for i, k := range storyFindingOrder {
		rank[k] = i
	}
	sort.SliceStable(out, func(a, b int) bool {
		if rank[out[a].Kind] != rank[out[b].Kind] {
			return rank[out[a].Kind] < rank[out[b].Kind]
		}
		if out[a].Document != out[b].Document {
			return out[a].Document < out[b].Document
		}
		return out[a].Message < out[b].Message
	})
	return out, nil
}
