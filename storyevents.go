package knowledge

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// storyEventsSchema creates story_events (DR-0075, DR-0079). It is derived
// from manuscript files by ingest and does not travel: export, import, merge
// and check-db leave it out, and a second machine rebuilds it by ingesting the
// same paths. "start" and "end" are SQL keywords, so the interval columns are
// start_at and end_at.
const storyEventsSchema = `
CREATE TABLE IF NOT EXISTS story_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id      INTEGER NOT NULL REFERENCES projects(id)  ON DELETE CASCADE,
    document_id     INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    section_id      INTEGER REFERENCES document_sections(id)  ON DELETE SET NULL,
    scene_id        TEXT NOT NULL DEFAULT '',
    label           TEXT NOT NULL DEFAULT '',
    story_time      TEXT NOT NULL DEFAULT '',
    relation        TEXT NOT NULL DEFAULT '',
    anchor_label    TEXT NOT NULL DEFAULT '',
    anchor_event_id INTEGER REFERENCES story_events(id) ON DELETE SET NULL,
    anchor_offset   TEXT NOT NULL DEFAULT '',
    start_at        TEXT NOT NULL DEFAULT '',
    end_at          TEXT NOT NULL DEFAULT '',
    precision       TEXT NOT NULL DEFAULT '',
    state           TEXT NOT NULL DEFAULT 'none',
    note            TEXT NOT NULL DEFAULT '',
    place           TEXT NOT NULL DEFAULT '',
    pov             TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT '',
    characters      TEXT NOT NULL DEFAULT '[]',
    foreshadows     TEXT NOT NULL DEFAULT '[]',
    pays_off        TEXT NOT NULL DEFAULT '[]',
    precedes        TEXT NOT NULL DEFAULT '[]',
    seq             INTEGER NOT NULL DEFAULT 0,
    UNIQUE (document_id, scene_id)
);
CREATE INDEX IF NOT EXISTS idx_story_events_project ON story_events(project_id, label);
`

/** StoryEvent is one row of story_events: a scene, or a prose scene file, placed
 * in story time (DR-0075, DR-0079).
 *
 * State is one of "dated" (an absolute time), "resolved" (a relative time
 * given an interval from its anchor), "edge" (an ordering relation to a found
 * anchor, no interval), "unresolved" (a relation whose anchor is missing,
 * ambiguous, undated or in a cycle; Note says which), or "none" (no usable
 * story_time; Note says why when it was unparseable).
 *
 * Example:
 *   evs, _ := kb.StoryEvents(projectID)
 *   for _, e := range evs { fmt.Println(e.Label, e.State, e.Time.Start) }
 */
type StoryEvent struct {
	ID            int64
	ProjectID     int64
	DocumentID    int64
	SectionID     int64
	SceneID       string
	Label         string
	StoryTimeText string
	Time          StoryTime
	AnchorEventID int64
	State         string
	Note          string
	Place         string
	POV           string
	Status        string
	Characters    []string
	Foreshadows   []string
	PaysOff       []string
	Precedes      []string
	Seq           int
}

// storyEventDraft is one event a parsed file yields, before it is written.
type storyEventDraft struct {
	sceneID string
	label   string
	heading string // the section it came from, matched by heading
	seq     int
	meta    StoryMeta
}

// storyEventDrafts decides which parts of a parsed file are events
// (DR-0079 item 4): a numbered Fountain scene, or a non-Fountain file whose
// front matter carries a narrative story key (status and place are ordinary
// front matter and only ride along).
func storyEventDrafts(p *ParsedDocument) []storyEventDraft {
	var out []storyEventDraft
	if p.Format == "fountain" {
		for _, s := range p.Sections {
			if s.SceneID == "" {
				continue
			}
			label := s.Story.Event
			if label == "" {
				label = s.SceneID
			}
			out = append(out, storyEventDraft{sceneID: s.SceneID, label: label, heading: s.Heading, seq: s.Seq, meta: s.Story})
		}
		return out
	}
	if p.Story.isNarrative() {
		heading := ""
		if len(p.Sections) > 0 {
			heading = p.Sections[0].Heading
		}
		out = append(out, storyEventDraft{label: p.Story.Event, heading: heading, meta: p.Story})
	}
	return out
}

func jsonList(items []string) string {
	if items == nil {
		items = []string{}
	}
	b, _ := json.Marshal(items)
	return string(b)
}

func parseJSONList(s string) []string {
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil || len(out) == 0 {
		return nil
	}
	return out
}

// replaceStoryEvents rewrites a document's events from drafts, then resolves
// the project's relative events. It returns a warning for each story_time it
// could not parse; those events are kept, as written, with no interval.
func (kb *KnowledgeBase) replaceStoryEvents(projectID, documentID int64, drafts []storyEventDraft) ([]string, error) {
	sections, err := kb.DocumentSections(documentID)
	if err != nil {
		return nil, err
	}
	sectionByHeading := map[string]int64{}
	for _, s := range sections {
		if s.Level != "section" {
			continue
		}
		if _, seen := sectionByHeading[s.Heading]; !seen {
			sectionByHeading[s.Heading] = s.ID
		}
	}
	tx, err := kb.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("knowledge: write story events: %w", err)
	}
	defer tx.Rollback()
	// Dependants of the rows about to be deleted fall back to NULL by the
	// anchor_event_id foreign key; the resolve pass below sets them again.
	if _, err := tx.Exec(`DELETE FROM story_events WHERE document_id = ?`, documentID); err != nil {
		return nil, fmt.Errorf("knowledge: write story events: %w", err)
	}
	var warnings []string
	for _, d := range drafts {
		t, perr := ParseStoryTime(d.meta.StoryTime)
		state, note := "none", ""
		switch {
		case perr != nil:
			note = perr.Error()
			warnings = append(warnings, fmt.Sprintf("event %s: %v", eventName(d), perr))
			t = StoryTime{}
		case t.Relation != "":
			state = "unresolved" // settled by the resolve pass
		case t.HasInterval():
			state = "dated"
		}
		var sectionID any
		if id, ok := sectionByHeading[d.heading]; ok {
			sectionID = id
		}
		if _, err := tx.Exec(
			`INSERT INTO story_events (project_id, document_id, section_id, scene_id, label, story_time, relation,
			     anchor_label, anchor_offset, start_at, end_at, precision, state, note, place, pov, status,
			     characters, foreshadows, pays_off, precedes, seq)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			projectID, documentID, sectionID, d.sceneID, d.label, d.meta.StoryTime, t.Relation,
			t.Anchor, t.Offset, t.Start, t.End, t.Precision, state, note, d.meta.Place, d.meta.POV, d.meta.Status,
			jsonList(d.meta.Characters), jsonList(d.meta.Foreshadows), jsonList(d.meta.PaysOff), jsonList(d.meta.Precedes), d.seq,
		); err != nil {
			return nil, fmt.Errorf("knowledge: write story events: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("knowledge: write story events: %w", err)
	}
	return warnings, kb.resolveStoryEvents(projectID)
}

func eventName(d storyEventDraft) string {
	switch {
	case d.label != "":
		return d.label
	case d.sceneID != "":
		return d.sceneID
	}
	return "(unlabelled)"
}

const storyEventColumns = `id, project_id, document_id, IFNULL(section_id, 0), scene_id, label, story_time, relation,
	anchor_label, IFNULL(anchor_event_id, 0), anchor_offset, start_at, end_at, precision, state, note, place, pov, status,
	characters, foreshadows, pays_off, precedes, seq`

func scanStoryEvent(rows *sql.Rows) (StoryEvent, error) {
	var e StoryEvent
	var chars, fore, pays, prec string
	err := rows.Scan(&e.ID, &e.ProjectID, &e.DocumentID, &e.SectionID, &e.SceneID, &e.Label, &e.StoryTimeText, &e.Time.Relation,
		&e.Time.Anchor, &e.AnchorEventID, &e.Time.Offset, &e.Time.Start, &e.Time.End, &e.Time.Precision, &e.State, &e.Note,
		&e.Place, &e.POV, &e.Status, &chars, &fore, &pays, &prec, &e.Seq)
	e.Characters, e.Foreshadows, e.PaysOff, e.Precedes = parseJSONList(chars), parseJSONList(fore), parseJSONList(pays), parseJSONList(prec)
	return e, err
}

/** StoryEvents returns a project's story events in document and narrative
 * order (DR-0079).
 *
 * Parameters:
 *   projectID (int64) — the project, or 0 for every project.
 *
 * Returns:
 *   []StoryEvent — the events; empty (not nil) if none.
 *   error        — on database failure.
 *
 * Example:
 *   evs, err := kb.StoryEvents(pid)
 */
func (kb *KnowledgeBase) StoryEvents(projectID int64) ([]StoryEvent, error) {
	query := `SELECT ` + storyEventColumns + ` FROM story_events`
	var args []any
	if projectID != 0 {
		query += ` WHERE project_id = ?`
		args = append(args, projectID)
	}
	query += ` ORDER BY document_id, seq, id`
	rows, err := kb.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("knowledge: story events: %w", err)
	}
	defer rows.Close()
	out := []StoryEvent{}
	for rows.Next() {
		e, err := scanStoryEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("knowledge: story events: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// resolveStoryEvents settles every relative event in a project from scratch:
// it finds each one's anchor by label (exactly one other event, or the event is
// left unresolved with a note), and, for a relation with an offset, gives it an
// interval once its anchor has one, repeating until nothing changes so chains
// resolve in any ingest order. A cycle never gains an interval. Idempotent.
func (kb *KnowledgeBase) resolveStoryEvents(projectID int64) error {
	if projectID == 0 {
		return nil
	}
	evs, err := kb.StoryEvents(projectID)
	if err != nil {
		return err
	}
	byLabel := map[string][]int{}
	for i, e := range evs {
		if e.Label != "" {
			byLabel[e.Label] = append(byLabel[e.Label], i)
		}
	}
	var relative []int
	for i := range evs {
		if evs[i].Time.Relation == "" {
			continue
		}
		relative = append(relative, i)
		e := &evs[i]
		e.AnchorEventID, e.State, e.Note = 0, "unresolved", ""
		e.Time.Precision, e.Time.Start, e.Time.End = "", "", ""
	}
	done := map[int]bool{}
	for pass := 0; pass <= len(relative); pass++ {
		changed := false
		for _, i := range relative {
			if done[i] {
				continue
			}
			e := &evs[i]
			var cands []int
			for _, j := range byLabel[e.Time.Anchor] {
				if j != i {
					cands = append(cands, j)
				}
			}
			switch {
			case len(cands) == 0:
				e.Note = fmt.Sprintf("anchor %q names no event", e.Time.Anchor)
				continue
			case len(cands) > 1:
				e.Note = fmt.Sprintf("anchor %q names %d events", e.Time.Anchor, len(cands))
				continue
			}
			anchor := &evs[cands[0]]
			e.AnchorEventID = anchor.ID
			if e.Time.Offset == "" {
				e.State, e.Note = "edge", ""
				done[i], changed = true, true
				continue
			}
			if !anchor.Time.HasInterval() {
				e.Note = fmt.Sprintf("anchor %q has no date yet, or is part of a cycle", e.Time.Anchor)
				continue
			}
			got, rerr := ResolveStoryTime(e.Time, anchor.Time)
			done[i], changed = true, true
			if rerr != nil {
				e.Note = rerr.Error()
				continue
			}
			e.Time, e.State, e.Note = got, "resolved", ""
		}
		if !changed {
			break
		}
	}
	for _, i := range relative {
		e := evs[i]
		var anchorID any
		if e.AnchorEventID != 0 {
			anchorID = e.AnchorEventID
		}
		if _, err := kb.db.Exec(
			`UPDATE story_events SET anchor_event_id = ?, start_at = ?, end_at = ?, precision = ?, state = ?, note = ? WHERE id = ?`,
			anchorID, e.Time.Start, e.Time.End, e.Time.Precision, e.State, e.Note, e.ID,
		); err != nil {
			return fmt.Errorf("knowledge: resolve story events: %w", err)
		}
	}
	return nil
}
