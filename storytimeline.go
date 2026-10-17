package knowledge

import (
	"sort"
	"strconv"
)

/** TimelineEntry is one row of a story timeline (DR-0075 item 9): an event in
 * the order it was asked for, with what is and is not known about that order.
 *
 * Unordered is true when this entry's place after the one before it is not
 * established: their intervals overlap, or they are different kinds of time, or
 * nothing relates them. It is reported, never guessed (DR-0075 item 8). Undated
 * is true, in story order, for an event with no interval and no relation to any
 * other event, and, in narrative order, for an event with no interval of its own.
 * InCycle is true for an event in, or ordered behind, a cycle of ordering
 * relations; a cycle is reported by StoryCheck.
 *
 * Example:
 *   es, _ := kb.StoryTimeline(pid, false)
 *   for _, e := range es { fmt.Println(e.Rank, e.Event.Label, e.Unordered) }
 */
type TimelineEntry struct {
	Event        StoryEvent
	DocumentPath string
	Rank         int
	Unordered    bool
	Undated      bool
	InCycle      bool
}

// storyOrdinalMode reports whether an event's time is an ordinal. Ordinals and
// calendar dates are never compared with each other.
func storyOrdinalMode(e StoryEvent) bool { return e.Time.Precision == "ordinal" }

// storyKeyBefore reports whether interval a lies wholly before interval b.
func storyKeyBefore(a, b StoryEvent) bool {
	if !a.Time.HasInterval() || !b.Time.HasInterval() || storyOrdinalMode(a) != storyOrdinalMode(b) {
		return false
	}
	if storyOrdinalMode(a) {
		ae, errA := strconv.Atoi(a.Time.End)
		bs, errB := strconv.Atoi(b.Time.Start)
		return errA == nil && errB == nil && ae < bs
	}
	return a.Time.End < b.Time.Start
}

// storyGraph holds what is provably before what among a project's events:
// intervals that do not overlap, "before:"/"after:" relations to a found anchor,
// and "precedes" lists. before[i] are the events i comes before.
type storyGraph struct {
	evs    []StoryEvent
	before [][]int
	// declared[i] are the events i is declared before (relations and lists only,
	// not intervals), for contradiction checks.
	declared [][]int
}

// uniqueLabels maps each label held by exactly one event to its index.
func uniqueLabels(evs []StoryEvent) map[string]int {
	count := map[string]int{}
	at := map[string]int{}
	for i, e := range evs {
		if e.Label != "" {
			count[e.Label]++
			at[e.Label] = i
		}
	}
	for l, n := range count {
		if n != 1 {
			delete(at, l)
		}
	}
	return at
}

func newStoryGraph(evs []StoryEvent) *storyGraph {
	g := &storyGraph{evs: evs, before: make([][]int, len(evs)), declared: make([][]int, len(evs))}
	byID := map[int64]int{}
	for i, e := range evs {
		byID[e.ID] = i
	}
	seen := map[[2]int]bool{}
	add := func(from, to int, declared bool) {
		if from == to {
			return
		}
		if declared {
			g.declared[from] = append(g.declared[from], to)
		}
		if seen[[2]int{from, to}] {
			return
		}
		seen[[2]int{from, to}] = true
		g.before[from] = append(g.before[from], to)
	}
	labels := uniqueLabels(evs)
	for i, e := range evs {
		if j, ok := byID[e.AnchorEventID]; ok && e.AnchorEventID != 0 {
			switch e.Time.Relation {
			case "before":
				add(i, j, true)
			case "after":
				add(j, i, true)
			}
		}
		for _, l := range e.Precedes {
			if j, ok := labels[l]; ok {
				add(i, j, true)
			}
		}
	}
	for i := range evs {
		for j := range evs {
			if i != j && storyKeyBefore(evs[i], evs[j]) {
				add(i, j, false)
			}
		}
	}
	return g
}

// reaches reports whether to can be reached from from along before edges.
func (g *storyGraph) reaches(from, to int) bool {
	seen := make([]bool, len(g.evs))
	stack := []int{from}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == to {
			return true
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		stack = append(stack, g.before[n]...)
	}
	return false
}

func narrativeLess(a, b StoryEvent) bool {
	if a.DocumentID != b.DocumentID {
		return a.DocumentID < b.DocumentID
	}
	if a.Seq != b.Seq {
		return a.Seq < b.Seq
	}
	return a.ID < b.ID
}

/** StoryTimeline lists a project's story events (DR-0075 item 9).
 *
 * In story order (narrative false) an event comes after every event provably
 * before it, by non-overlapping intervals or by a before:/after: relation or a
 * precedes list; ties go to narrative order. Where two neighbours' order is not
 * established the later is marked Unordered. Events with no interval and no
 * relation come last, marked Undated. In narrative order (narrative true) events
 * are listed by document and position in it, which is the order a reader meets
 * them, with the story time alongside.
 *
 * Parameters:
 *   projectID (int64) — the project.
 *   narrative (bool)  — list in reading order instead of story order.
 *
 * Returns:
 *   []TimelineEntry — one per event; empty (not nil) if none.
 *   error           — on database failure.
 *
 * Example:
 *   es, err := kb.StoryTimeline(pid, false)
 */
func (kb *KnowledgeBase) StoryTimeline(projectID int64, narrative bool) ([]TimelineEntry, error) {
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
	entry := func(e StoryEvent) TimelineEntry {
		return TimelineEntry{Event: e, DocumentPath: path[e.DocumentID], Undated: !e.Time.HasInterval()}
	}
	out := []TimelineEntry{}
	if narrative {
		sorted := append([]StoryEvent(nil), evs...)
		sort.SliceStable(sorted, func(i, j int) bool { return narrativeLess(sorted[i], sorted[j]) })
		for i, e := range sorted {
			en := entry(e)
			en.Rank = i + 1
			out = append(out, en)
		}
		return out, nil
	}

	g := newStoryGraph(evs)
	indeg := make([]int, len(evs))
	hasEdge := make([]bool, len(evs))
	for i, tos := range g.before {
		for _, j := range tos {
			indeg[j]++
			hasEdge[i], hasEdge[j] = true, true
		}
	}
	placed := func(i int) bool { return hasEdge[i] || evs[i].Time.HasInterval() }
	var ready []int
	var rest []int // no evidence at all: listed last
	for i := range evs {
		if !placed(i) {
			rest = append(rest, i)
			continue
		}
		if indeg[i] == 0 {
			ready = append(ready, i)
		}
	}
	var order []int
	done := make([]bool, len(evs))
	for len(ready) > 0 {
		best := 0
		for k := range ready {
			if narrativeLess(evs[ready[k]], evs[ready[best]]) {
				best = k
			}
		}
		n := ready[best]
		ready = append(ready[:best], ready[best+1:]...)
		order, done[n] = append(order, n), true
		for _, j := range g.before[n] {
			if indeg[j]--; indeg[j] == 0 {
				ready = append(ready, j)
			}
		}
	}
	var stuck []int
	for i := range evs {
		if placed(i) && !done[i] {
			stuck = append(stuck, i)
		}
	}
	sort.SliceStable(stuck, func(a, b int) bool { return narrativeLess(evs[stuck[a]], evs[stuck[b]]) })
	sort.SliceStable(rest, func(a, b int) bool { return narrativeLess(evs[rest[a]], evs[rest[b]]) })

	prev := -1
	for _, i := range order {
		en := entry(evs[i])
		en.Rank = len(out) + 1
		en.Undated = false // a date or a relation placed it
		en.Unordered = prev >= 0 && !g.reaches(prev, i)
		out = append(out, en)
		prev = i
	}
	for _, i := range stuck {
		en := entry(evs[i])
		en.Rank, en.InCycle, en.Unordered = len(out)+1, true, true
		out = append(out, en)
	}
	for _, i := range rest {
		en := entry(evs[i])
		en.Rank, en.Undated = len(out)+1, true
		out = append(out, en)
	}
	return out, nil
}
