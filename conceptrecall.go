package knowledge

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

/** TextRecallHit is one item RecallByText or RecallByNames found: a
 * ConceptMatch plus the two things a curator needs to judge it.
 *
 * Fields:
 *   ConceptMatch          — the item itself; SourceType is "observation", "record",
 *                           "document_gist" or "document_section", and MatchCount is
 *                           how many of the matched concepts it links to.
 *   Project  (string)     — the owning project's name; "" for a workspace-tier record.
 *   Concepts ([]string)   — exactly the matched concepts this item links to, in the
 *                           order of TextRecall.Matched.
 *
 * Example:
 *   for _, h := range r.Hits { fmt.Println(h.SourceType, h.ID, h.Project, h.Concepts) }
 */
type TextRecallHit struct {
	ConceptMatch
	Project  string
	Concepts []string
}

/** TextRecall is the result of RecallByText or RecallByNames.
 *
 * Fields:
 *   Matched  ([]string)      — the concept names the recall used: those found in the text,
 *                              or the canonical form of the names supplied. Empty when none.
 *   Hits     ([]TextRecallHit) — items linked to them, ranked by matched concepts
 *                              descending then recency, truncated to the limit.
 *   Projects ([]string)      — names of projects linked to a matched concept, listed apart
 *                              from Hits and not truncated by the limit.
 *
 * Example:
 *   r, _ := kb.RecallByText("how does toast work", 10, "")
 *   fmt.Println(r.Matched, len(r.Hits))
 */
type TextRecall struct {
	Matched  []string
	Hits     []TextRecallHit
	Projects []string
}

/** RecallByText finds the concepts named in text (MatchConceptNames), then
 * returns the observations, records and document sections linked to them,
 * with the concepts each one matched, so a caller can see why it came back.
 *
 * This is the wider sibling of RecallByConceptNames: it also covers document
 * sections, can be scoped to one project, and reports per-hit concepts.
 * RecallByConceptNames covers observations and records only, is never scoped,
 * and is unchanged, so a caller that depends on its exact results keeps them.
 * The ranking rule is the same: matched concepts descending, then recency.
 *
 * Text that matches no concept is a normal outcome, not an error: the result
 * is empty. Nothing is ever created.
 *
 * Parameters:
 *   text    (string) — free text, such as a prompt.
 *   limit   (int)    — maximum hits; non-positive means no cap.
 *   project (string) — restrict hits to this project by name; "" for all. An unknown
 *                      name is ErrNotFound. Workspace-tier records are excluded when set.
 *
 * Returns:
 *   TextRecall — matched concept names and ranked hits.
 *   error      — ErrNotFound for an unknown project, or a database failure.
 *
 * Example:
 *   r, err := kb.RecallByText("chunking and RAG", 5, "harvey")
 */
func (kb *KnowledgeBase) RecallByText(text string, limit int, project string) (TextRecall, error) {
	if err := kb.requireProject(project); err != nil {
		return TextRecall{}, err
	}
	names, err := kb.MatchConceptNames(text)
	if err != nil {
		return TextRecall{}, err
	}
	return kb.RecallByNames(names, limit, project)
}

/** RecallByNames is RecallByText without the matching step: the names are used
 * directly, case-insensitively, and one that matches no concept is skipped.
 * TextRecall.Matched holds the canonical spelling of those that exist.
 *
 * Parameters:
 *   names   ([]string) — concept names to recall by.
 *   limit   (int)      — maximum hits; non-positive means no cap.
 *   project (string)   — restrict hits to this project by name; "" for all.
 *
 * Returns:
 *   TextRecall — the concepts found and ranked hits.
 *   error      — ErrNotFound for an unknown project, or a database failure.
 *
 * Example:
 *   r, err := kb.RecallByNames([]string{"toast"}, 10, "")
 */
func (kb *KnowledgeBase) RecallByNames(names []string, limit int, project string) (TextRecall, error) {
	var r TextRecall
	if err := kb.requireProject(project); err != nil {
		return r, err
	}
	// Resolve in input order, deduping case variants, keeping the canonical name.
	order := map[int64]int{}
	var ids []int64
	for _, n := range names {
		var id int64
		var canon string
		err := kb.db.QueryRow(`SELECT id, name FROM concepts WHERE name = ?1 COLLATE NOCASE LIMIT 1`, n).Scan(&id, &canon)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return r, fmt.Errorf("knowledge: resolve concept name %q: %w", n, err)
		}
		if _, dup := order[id]; dup {
			continue
		}
		order[id] = len(ids)
		ids = append(ids, id)
		r.Matched = append(r.Matched, canon)
	}
	if len(ids) == 0 {
		return r, nil
	}
	in := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	filter := func() (string, []any) {
		if project == "" {
			return "", args
		}
		return " AND p.name = ?", append(append([]any{}, args...), project)
	}

	type kind struct {
		query string
		scan  func(rows *sql.Rows) (id int64, concept int64, h TextRecallHit, err error)
	}
	kinds := []kind{
		{
			query: `SELECT o.id, IFNULL(o.project_id, 0), IFNULL(p.name, ''), o.body, o.created_at, x.concept_id
				FROM observations o JOIN observation_concepts x ON x.observation_id = o.id
				LEFT JOIN projects p ON p.id = o.project_id
				WHERE x.concept_id IN (` + in + `)%s ORDER BY o.id`,
			scan: func(rows *sql.Rows) (int64, int64, TextRecallHit, error) {
				var h TextRecallHit
				var ts string
				var cid int64
				err := rows.Scan(&h.ID, &h.ProjectID, &h.Project, &h.Body, &ts, &cid)
				h.SourceType, h.CreatedAt = "observation", parseTimestamp(ts)
				return h.ID, cid, h, err
			},
		},
		{
			query: `SELECT r.id, IFNULL(r.project_id, 0), IFNULL(p.name, ''), r.title, r.body, r.ingested_at, x.concept_id
				FROM records r JOIN record_concepts x ON x.record_id = r.id
				LEFT JOIN projects p ON p.id = r.project_id
				WHERE x.concept_id IN (` + in + `)%s ORDER BY r.id`,
			scan: func(rows *sql.Rows) (int64, int64, TextRecallHit, error) {
				var h TextRecallHit
				var ts string
				var cid int64
				err := rows.Scan(&h.ID, &h.ProjectID, &h.Project, &h.Title, &h.Body, &ts, &cid)
				h.SourceType, h.CreatedAt = "record", parseTimestamp(ts)
				return h.ID, cid, h, err
			},
		},
		{
			query: `SELECT s.id, IFNULL(d.project_id, 0), IFNULL(p.name, ''), d.title, s.level, s.heading,
					s.summary_body, s.summary_status, s.created_at, x.concept_id
				FROM document_sections s JOIN documents d ON d.id = s.document_id
				JOIN document_section_concepts x ON x.section_id = s.id
				LEFT JOIN projects p ON p.id = d.project_id
				WHERE x.concept_id IN (` + in + `)%s ORDER BY s.id`,
			scan: func(rows *sql.Rows) (int64, int64, TextRecallHit, error) {
				var h TextRecallHit
				var level, heading, summary, ts string
				var cid int64
				err := rows.Scan(&h.ID, &h.ProjectID, &h.Project, &h.Title, &level, &heading,
					&summary, &h.SummaryStatus, &ts, &cid)
				if level == "gist" {
					h.SourceType = "document_gist"
				} else {
					h.SourceType = "document_section"
					if heading != "" {
						h.Title += " / " + heading
					}
				}
				// Only a reviewed summary is trustworthy content (same rule as RecallByConceptNames).
				if h.SummaryStatus == "reviewed" {
					h.Body = summary
				}
				h.CreatedAt = parseTimestamp(ts)
				return h.ID, cid, h, err
			},
		},
	}

	var hits []TextRecallHit
	for _, k := range kinds {
		where, qargs := filter()
		rows, err := kb.db.Query(fmt.Sprintf(k.query, where), qargs...)
		if err != nil {
			return r, fmt.Errorf("knowledge: recall by concept: %w", err)
		}
		index := map[int64]int{}
		for rows.Next() {
			id, cid, h, err := k.scan(rows)
			if err != nil {
				rows.Close()
				return r, err
			}
			i, seen := index[id]
			if !seen {
				i = len(hits)
				index[id] = i
				hits = append(hits, h)
			}
			hits[i].MatchCount++
			hits[i].Concepts = append(hits[i].Concepts, r.Matched[order[cid]])
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return r, err
		}
	}
	for i := range hits {
		sort.SliceStable(hits[i].Concepts, func(a, b int) bool {
			return conceptPos(r.Matched, hits[i].Concepts[a]) < conceptPos(r.Matched, hits[i].Concepts[b])
		})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].MatchCount != hits[j].MatchCount {
			return hits[i].MatchCount > hits[j].MatchCount
		}
		return hits[i].CreatedAt.After(hits[j].CreatedAt)
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	r.Hits = hits

	prows, err := kb.db.Query(`SELECT DISTINCT p.name FROM projects p
		JOIN project_concepts x ON x.project_id = p.id
		WHERE x.concept_id IN (`+in+`) ORDER BY p.name`, args...)
	if err != nil {
		return r, fmt.Errorf("knowledge: recall projects by concept: %w", err)
	}
	defer prows.Close()
	for prows.Next() {
		var n string
		if err := prows.Scan(&n); err != nil {
			return r, err
		}
		if project == "" || n == project {
			r.Projects = append(r.Projects, n)
		}
	}
	return r, prows.Err()
}

func conceptPos(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return len(list)
}

// requireProject returns ErrNotFound when name is non-empty and no project has it.
func (kb *KnowledgeBase) requireProject(name string) error {
	if name == "" {
		return nil
	}
	var id int64
	err := kb.db.QueryRow(`SELECT id FROM projects WHERE name = ?`, name).Scan(&id)
	if err == sql.ErrNoRows {
		return notFoundf("knowledge: project %q not found", name)
	}
	if err != nil {
		return fmt.Errorf("knowledge: look up project: %w", err)
	}
	return nil
}
