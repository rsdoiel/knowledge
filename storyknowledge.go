package knowledge

import (
	"fmt"
	"sort"
	"strings"
)

/** ErrAmbiguousScene marks a scene name that matches more than one event: a label
 * shared by two events, or a scene number used in two files (DR-0083 item 5). It
 * is also an ErrInvalid, so errors.Is matches either: a command line that names
 * such a scene is a usage error (exit 2).
 *
 * Example:
 *   _, err := kb.WhoKnows(pid, "Anna", "twin")
 *   if errors.Is(err, knowledge.ErrAmbiguousScene) { // name the scene by its number instead }
 */
var ErrAmbiguousScene error = ambiguousScene{}

type ambiguousScene struct{}

func (ambiguousScene) Error() string        { return "ambiguous scene" }
func (ambiguousScene) Is(target error) bool { return target == ErrInvalid }

/** KnownFact is one fact a character learns, and where (DR-0083).
 *
 * Fact is the text as written in the scene that teaches it, FactKey its folded
 * identity (see FactKey), LearnedIn the name of that scene (its label, or its
 * scene number), SceneID the scene number (empty for a prose scene file), and
 * Document the file's path. EventID is the story event.
 *
 * Example:
 *   r, _ := kb.WhoKnows(pid, "Anna", "")
 *   fmt.Println(r.Facts[0].Fact, "in", r.Facts[0].LearnedIn)
 */
type KnownFact struct {
	Fact      string
	FactKey   string
	LearnedIn string
	SceneID   string
	Document  string
	EventID   int64
}

/** WhoKnowsResult is what WhoKnows returns (DR-0083 item 5).
 *
 * Character is the character as the story spells them, Scene the name of the
 * scene asked about ("" when none was). Facts are what the character has learned
 * at or before that scene, in story order. Unordered are facts learned only in
 * scenes whose order against the scene is not established; they are neither
 * counted as known nor left out (DR-0075 item 8). Both are empty, not nil.
 *
 * Example:
 *   r, _ := kb.WhoKnows(pid, "Anna", "12A")
 *   for _, f := range r.Facts { fmt.Println(f.Fact) }
 */
type WhoKnowsResult struct {
	Character string
	Scene     string
	Facts     []KnownFact
	Unordered []KnownFact
}

/** Learner is one character who learns a fact, from WhoLearns.
 *
 * Fact is the text as that character's scene writes it; LearnedIn, SceneID and
 * Document are as in KnownFact.
 *
 * Example:
 *   ls, _ := kb.WhoLearns(pid, "the letter is forged")
 *   fmt.Println(ls[0].Character, ls[0].LearnedIn)
 */
type Learner struct {
	Character string
	Fact      string
	LearnedIn string
	SceneID   string
	Document  string
}

// learnRow is one row of story_knowledge with the event it belongs to.
type learnRow struct {
	eventID   int64
	character string
	fact      string
	factKey   string
	how       string
}

// storyKnowledgeRows returns every story_knowledge row of a project, in the
// order they were written.
func (kb *KnowledgeBase) storyKnowledgeRows(projectID int64) ([]learnRow, error) {
	rows, err := kb.db.Query(`SELECT k.event_id, k.character, k.fact, k.fact_key, k.how
		FROM story_knowledge k JOIN story_events e ON e.id = k.event_id
		WHERE e.project_id = ? ORDER BY k.id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("knowledge: read story knowledge: %w", err)
	}
	defer rows.Close()
	var out []learnRow
	for rows.Next() {
		var r learnRow
		if err := rows.Scan(&r.eventID, &r.character, &r.fact, &r.factKey, &r.how); err != nil {
			return nil, fmt.Errorf("knowledge: read story knowledge: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// storyOrder is a project's events with their story order and the provable
// before-relation, for the knowledge queries.
type storyOrder struct {
	events []StoryEvent
	index  map[int64]int // event id to position in events
	rank   map[int64]int // event id to its place in story order
	graph  *storyGraph
	path   map[int64]string // document id to path
}

func (kb *KnowledgeBase) loadStoryOrder(projectID int64) (*storyOrder, error) {
	evs, err := kb.StoryEvents(projectID)
	if err != nil {
		return nil, err
	}
	tl, err := kb.StoryTimeline(projectID, false)
	if err != nil {
		return nil, err
	}
	docs, err := kb.Documents(projectID)
	if err != nil {
		return nil, err
	}
	o := &storyOrder{events: evs, index: map[int64]int{}, rank: map[int64]int{}, graph: newStoryGraph(evs), path: map[int64]string{}}
	for i, e := range evs {
		o.index[e.ID] = i
	}
	for _, t := range tl {
		o.rank[t.Event.ID] = t.Rank
	}
	for _, d := range docs {
		o.path[d.ID] = d.Path
	}
	return o, nil
}

// atOrBefore reports whether event a is provably at or before event b.
func (o *storyOrder) atOrBefore(a, b int64) bool {
	return a == b || o.graph.reaches(o.index[a], o.index[b])
}

// resolveScene finds the one event a scene name means: an event's label, or a
// scene number. Nothing matching is ErrNotFound; several is ErrAmbiguousScene.
func (o *storyOrder) resolveScene(name string) (StoryEvent, error) {
	var hits []StoryEvent
	for _, e := range o.events {
		if e.Label == name || (e.SceneID != "" && e.SceneID == name) {
			hits = append(hits, e)
		}
	}
	switch len(hits) {
	case 0:
		return StoryEvent{}, fmt.Errorf("knowledge: no scene %q: %w", name, ErrNotFound)
	case 1:
		return hits[0], nil
	}
	var where []string
	for _, e := range hits {
		where = append(where, o.path[e.DocumentID])
	}
	return StoryEvent{}, fmt.Errorf("knowledge: scene %q names %d events (%s): %w", name, len(hits), strings.Join(where, ", "), ErrAmbiguousScene)
}

func (o *storyOrder) known(r learnRow) KnownFact {
	e := o.events[o.index[r.eventID]]
	return KnownFact{Fact: r.fact, FactKey: r.factKey, LearnedIn: storyEventName(e), SceneID: e.SceneID, Document: o.path[e.DocumentID], EventID: e.ID}
}

/** WhoKnows lists what a character learns, in story order, up to a scene
 * (DR-0083 item 5).
 *
 * A fact counts when the character has a learns row for it in a scene that is
 * provably at or before the scene asked about (the scene itself counts). A fact
 * learned in several scenes is listed once, at the earliest. A knows row is use,
 * not learning, and is never listed. Facts learned only in scenes whose order
 * against the scene is not established are returned in Unordered, not guessed.
 * With scene "" every fact the character learns is returned.
 *
 * Parameters:
 *   projectID (int64)  — the project.
 *   character (string) — the character, matched without regard to case.
 *   scene     (string) — a scene number or an event label; "" for no limit.
 *
 * Returns:
 *   WhoKnowsResult — Facts and Unordered, both empty (not nil) when there are none.
 *   error          — ErrNotFound when no scene row names the character, or the
 *     scene names no event; ErrAmbiguousScene (also ErrInvalid) when it names
 *     several; or a database failure. A character named only in knows rows is
 *     found, with nothing learned.
 *
 * Example:
 *   r, err := kb.WhoKnows(pid, "Anna", "12A")
 *   for _, f := range r.Facts { fmt.Printf("%s (%s)\n", f.Fact, f.LearnedIn) }
 */
func (kb *KnowledgeBase) WhoKnows(projectID int64, character, scene string) (WhoKnowsResult, error) {
	res := WhoKnowsResult{Facts: []KnownFact{}, Unordered: []KnownFact{}}
	character = strings.TrimSpace(character)
	rows, err := kb.storyKnowledgeRows(projectID)
	if err != nil {
		return res, err
	}
	var mine []learnRow
	for _, r := range rows {
		if strings.EqualFold(r.character, character) {
			if res.Character == "" {
				res.Character = r.character
			}
			mine = append(mine, r)
		}
	}
	if res.Character == "" {
		return res, fmt.Errorf("knowledge: no scene names the character %q: %w", character, ErrNotFound)
	}
	o, err := kb.loadStoryOrder(projectID)
	if err != nil {
		return res, err
	}
	var at StoryEvent
	if scene = strings.TrimSpace(scene); scene != "" {
		if at, err = o.resolveScene(scene); err != nil {
			return res, err
		}
		res.Scene = storyEventName(at)
	}

	// Each fact the character learns, with the scenes that teach it, earliest first.
	var keys []string
	teaches := map[string][]learnRow{}
	for _, r := range mine {
		if r.how != "learns" {
			continue
		}
		if _, seen := teaches[r.factKey]; !seen {
			keys = append(keys, r.factKey)
		}
		teaches[r.factKey] = append(teaches[r.factKey], r)
	}
	type placed struct {
		fact KnownFact
		rank int
	}
	var facts, unordered []placed
	for _, key := range keys {
		rs := teaches[key]
		sort.SliceStable(rs, func(i, j int) bool { return o.rank[rs[i].eventID] < o.rank[rs[j].eventID] })
		if scene == "" {
			facts = append(facts, placed{o.known(rs[0]), o.rank[rs[0].eventID]})
			continue
		}
		var firstUnordered *learnRow
		known := false
		for i := range rs {
			switch {
			case o.atOrBefore(rs[i].eventID, at.ID):
				facts = append(facts, placed{o.known(rs[i]), o.rank[rs[i].eventID]})
				known = true
			case !o.atOrBefore(at.ID, rs[i].eventID) && firstUnordered == nil:
				firstUnordered = &rs[i]
			}
			if known {
				break
			}
		}
		if !known && firstUnordered != nil {
			unordered = append(unordered, placed{o.known(*firstUnordered), o.rank[firstUnordered.eventID]})
		}
	}
	for _, list := range []*[]placed{&facts, &unordered} {
		sort.SliceStable(*list, func(i, j int) bool { return (*list)[i].rank < (*list)[j].rank })
	}
	for _, p := range facts {
		res.Facts = append(res.Facts, p.fact)
	}
	for _, p := range unordered {
		res.Unordered = append(res.Unordered, p.fact)
	}
	return res, nil
}

/** WhoLearns lists who learns a fact, and where, in story order (DR-0083 item 5).
 *
 * The fact is matched by its folded key (see FactKey), so case and spacing do
 * not matter. A character who learns it in several scenes is listed once, at
 * the earliest. A fact that is only used (a knows row, backstory) has no
 * learner.
 *
 * Parameters:
 *   projectID (int64)  — the project.
 *   fact      (string) — the fact's text.
 *
 * Returns:
 *   []Learner — one per character, in story order.
 *   error     — ErrInvalid for a blank fact; ErrNotFound when no scene teaches
 *     it; or a database failure.
 *
 * Example:
 *   ls, err := kb.WhoLearns(pid, "the letter is forged")
 *   for _, l := range ls { fmt.Println(l.Character, l.LearnedIn) }
 */
func (kb *KnowledgeBase) WhoLearns(projectID int64, fact string) ([]Learner, error) {
	key := FactKey(fact)
	if key == "" {
		return nil, fmt.Errorf("knowledge: a fact needs some text: %w", ErrInvalid)
	}
	rows, err := kb.storyKnowledgeRows(projectID)
	if err != nil {
		return nil, err
	}
	var hits []learnRow
	for _, r := range rows {
		if r.how == "learns" && r.factKey == key {
			hits = append(hits, r)
		}
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("knowledge: no scene teaches %q: %w", strings.TrimSpace(fact), ErrNotFound)
	}
	o, err := kb.loadStoryOrder(projectID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(hits, func(i, j int) bool { return o.rank[hits[i].eventID] < o.rank[hits[j].eventID] })
	out := []Learner{}
	seen := map[string]bool{}
	for _, r := range hits {
		who := strings.ToLower(r.character)
		if seen[who] {
			continue
		}
		seen[who] = true
		k := o.known(r)
		out = append(out, Learner{Character: r.character, Fact: r.fact, LearnedIn: k.LearnedIn, SceneID: k.SceneID, Document: k.Document})
	}
	return out, nil
}
