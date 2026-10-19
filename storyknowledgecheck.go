package knowledge

import (
	"fmt"
	"sort"
	"strings"
)

// knowledgeFindings are the three checks of DR-0083 item 7: a character used a
// fact before learning it, two facts that are probably one, and evidence dated
// after the event it is cited for. They are advisory, like every other kind.
func (kb *KnowledgeBase) knowledgeFindings(projectID int64, evs []StoryEvent, g *storyGraph, path map[int64]string) ([]StoryFinding, error) {
	rows, err := kb.storyKnowledgeRows(projectID)
	if err != nil {
		return nil, err
	}
	index := map[int64]int{}
	for i, e := range evs {
		index[e.ID] = i
	}
	var out []StoryFinding
	add := func(kind, doc, format string, a ...any) {
		out = append(out, StoryFinding{Kind: kind, Document: doc, Message: fmt.Sprintf(format, a...)})
	}
	sceneOf := func(id int64) (string, string) {
		e := evs[index[id]]
		return storyEventName(e), path[e.DocumentID]
	}

	// knowledge-before-learned
	type who struct{ character, key string }
	learns := map[who][]int64{}
	for _, r := range rows {
		if r.how == "learns" {
			k := who{strings.ToLower(r.character), r.factKey}
			learns[k] = append(learns[k], r.eventID)
		}
	}
	reported := map[string]bool{}
	for _, r := range rows {
		ls := learns[who{strings.ToLower(r.character), r.factKey}]
		if r.how != "knows" || len(ls) == 0 {
			continue // backstory: nothing teaches it, so nothing contradicts it
		}
		allLater := true
		for _, l := range ls {
			if l == r.eventID || !g.reaches(index[r.eventID], index[l]) {
				allLater = false
				break
			}
		}
		dup := fmt.Sprintf("%d\x00%s\x00%s", r.eventID, strings.ToLower(r.character), r.factKey)
		if !allLater || reported[dup] {
			continue
		}
		reported[dup] = true
		sort.Slice(ls, func(i, j int) bool { return index[ls[i]] < index[ls[j]] })
		use, doc := sceneOf(r.eventID)
		learned, _ := sceneOf(ls[0])
		add("knowledge-before-learned", doc, "%s uses %q in %s, but does not learn it until %s", r.character, r.fact, use, learned)
	}

	// unknown-fact
	type factAt struct{ fact, scene, doc string }
	facts := map[string]factAt{}
	var keys []string
	for _, r := range rows {
		if _, seen := facts[r.factKey]; seen {
			continue
		}
		scene, doc := sceneOf(r.eventID)
		facts[r.factKey] = factAt{r.fact, scene, doc}
		keys = append(keys, r.factKey)
	}
	sort.Strings(keys)
	for i, a := range keys {
		if len([]rune(a)) < minTypoFactLength {
			continue
		}
		for _, b := range keys[i+1:] {
			if len([]rune(b)) >= minTypoFactLength && editDistanceIsOne(a, b) {
				fa, fb := facts[a], facts[b]
				add("unknown-fact", fa.doc, "%q (%s) is one edit from %q (%s); is it one fact typed twice?", fa.fact, fa.scene, fb.fact, fb.scene)
			}
		}
	}

	// late-source
	docs, err := kb.Documents(projectID)
	if err != nil {
		return nil, err
	}
	for _, d := range docs {
		links, err := kb.DocumentSources(d.ID)
		if err != nil {
			return nil, err
		}
		for _, l := range links {
			start, ok := sourceDateStart(l.Source.PublishedDate)
			if l.Relationship != "evidence" || !ok {
				continue
			}
			for _, e := range evs {
				if e.DocumentID == d.ID && e.Time.HasInterval() && !storyOrdinalMode(e) && len(e.Time.End) == len(storyDateLayout) && start > e.Time.End {
					add("late-source", d.Path, "%s ends %s, but its evidence %q was published %s", storyEventName(e), e.Time.End, l.Source.Title, l.Source.PublishedDate)
				}
			}
		}
	}
	return out, nil
}

// minTypoFactLength is the shortest fact key unknown-fact looks at: below it, one
// edit is a large share of the text and two different facts are common.
const minTypoFactLength = 12

// sourceDateStart pads a source's published_date (YYYY, YYYY-MM or YYYY-MM-DD)
// to the first day it could mean, so it can be compared with an event's end. A
// date of another shape gives false: no finding is made from it.
func sourceDateStart(s string) (string, bool) {
	switch {
	case storyYearPattern.MatchString(s):
		return s + "-01-01", true
	case storyMonthPattern.MatchString(s):
		return s + "-01", true
	case len(s) == len(storyDateLayout):
		if _, err := ParseStoryTime(s); err == nil {
			return s, true
		}
	}
	return "", false
}

// editDistanceIsOne reports whether a and b differ by exactly one inserted,
// deleted or replaced character. Equal strings are distance zero and give false.
func editDistanceIsOne(a, b string) bool {
	x, y := []rune(a), []rune(b)
	if len(x) < len(y) {
		x, y = y, x
	}
	if len(x)-len(y) > 1 {
		return false
	}
	i := 0
	for i < len(y) && x[i] == y[i] {
		i++
	}
	if i == len(y) {
		return len(x) != len(y) // y is a prefix of x: one trailing character more
	}
	if len(x) == len(y) {
		i++ // replaced: the rest must match
		return string(x[i:]) == string(y[i:])
	}
	return string(x[i+1:]) == string(y[i:]) // inserted: skip x[i]
}
