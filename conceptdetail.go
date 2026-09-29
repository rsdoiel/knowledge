package knowledge

import (
	"fmt"
	"strings"
	"time"
)

/** ConceptDetailItem is one thing linked to a concept, as ConceptDetail lists
 * it: enough to recognise the item and judge the link, not the whole row.
 *
 * Fields:
 *   ID        (int64)     — internal database id (a document section's id for a section).
 *   Project   (string)    — owning project's name; for a project item its own name;
 *                           "" for a workspace-tier record.
 *   Title     (string)    — a record's or document's title; "" for an observation or project.
 *   Body      (string)    — an observation's text, a project's description, a record's or
 *                           section's text (a section's only when its summary is reviewed).
 *   CreatedAt (time.Time) — the time the list is ordered by: created, or ingested for a record.
 *
 * Example:
 *   d, _ := kb.ConceptDetail("toast", 10)
 *   for _, o := range d.Observations { fmt.Println(o.ID, o.Body) }
 */
type ConceptDetailItem struct {
	ID        int64
	Project   string
	Title     string
	Body      string
	CreatedAt time.Time
}

/** ConceptDetailResult is everything ConceptDetail knows about one concept.
 *
 * Fields:
 *   Concept          (Concept)             — the concept row: description and identifier.
 *   Usage            (ConceptUsage)        — the full link counts, whatever the limit.
 *   Projects         ([]ConceptDetailItem) — up to limit projects, newest first.
 *   Observations     ([]ConceptDetailItem) — up to limit observations, newest first.
 *   Records          ([]ConceptDetailItem) — up to limit records, newest first.
 *   DocumentSections ([]ConceptDetailItem) — up to limit document sections, newest first.
 *
 * Example:
 *   d, _ := kb.ConceptDetail("toast", 10)
 *   fmt.Println(d.Usage.Total(), len(d.Records))
 */
type ConceptDetailResult struct {
	Concept          Concept
	Usage            ConceptUsage
	Projects         []ConceptDetailItem
	Observations     []ConceptDetailItem
	Records          []ConceptDetailItem
	DocumentSections []ConceptDetailItem
}

/** ConceptDetail reports one concept's description, link counts and up to
 * limit linked items of each kind, newest first, without changing anything.
 * The name must match exactly, including case, as DeleteConcept requires, so
 * "show" and "delete" agree on what a name means. A miss is ErrNotFound; when
 * a concept differing only in case exists, the error text offers it.
 *
 * Parameters:
 *   name  (string) — the concept's name.
 *   limit (int)    — most items per kind; zero returns counts only, negative is treated as zero.
 *
 * Returns:
 *   ConceptDetailResult — the concept, its counts and its items.
 *   error               — ErrNotFound for an unknown name, or a database failure.
 *
 * Example:
 *   d, err := kb.ConceptDetail("toast", 10)
 */
func (kb *KnowledgeBase) ConceptDetail(name string, limit int) (ConceptDetailResult, error) {
	var d ConceptDetailResult
	id, err := kb.conceptIDByExactName(name)
	if err != nil {
		return d, kb.conceptNotFoundHint(name, err)
	}
	if limit < 0 {
		limit = 0
	}
	err = kb.db.QueryRow(`SELECT id, name, description, IFNULL(identifier_type, ''), IFNULL(identifier_value, '')
		FROM concepts WHERE id = ?`, id).
		Scan(&d.Concept.ID, &d.Concept.Name, &d.Concept.Description, &d.Concept.IdentifierType, &d.Concept.IdentifierValue)
	if err != nil {
		return d, fmt.Errorf("knowledge: read concept: %w", err)
	}
	if d.Usage, err = kb.conceptUsageByID(name, id); err != nil {
		return d, err
	}
	if limit == 0 {
		return d, nil
	}
	// Ties on time (CURRENT_TIMESTAMP is one-second) break on id, newest first.
	for _, q := range []struct {
		into  *[]ConceptDetailItem
		query string
	}{
		{&d.Projects, `SELECT p.id, p.name, '', p.description, p.created_at
			FROM projects p JOIN project_concepts x ON x.project_id = p.id
			WHERE x.concept_id = ? ORDER BY p.created_at DESC, p.id DESC LIMIT ?`},
		{&d.Observations, `SELECT o.id, IFNULL(p.name, ''), '', o.body, o.created_at
			FROM observations o JOIN observation_concepts x ON x.observation_id = o.id
			LEFT JOIN projects p ON p.id = o.project_id
			WHERE x.concept_id = ? ORDER BY o.created_at DESC, o.id DESC LIMIT ?`},
		{&d.Records, `SELECT r.id, IFNULL(p.name, ''), r.title, r.body, r.ingested_at
			FROM records r JOIN record_concepts x ON x.record_id = r.id
			LEFT JOIN projects p ON p.id = r.project_id
			WHERE x.concept_id = ? ORDER BY r.ingested_at DESC, r.id DESC LIMIT ?`},
		{&d.DocumentSections, `SELECT s.id, IFNULL(p.name, ''),
				CASE WHEN s.heading = '' THEN doc.title ELSE doc.title || ' / ' || s.heading END,
				CASE WHEN s.summary_status = 'reviewed' THEN s.summary_body ELSE '' END, s.created_at
			FROM document_sections s
			JOIN documents doc ON doc.id = s.document_id
			JOIN document_section_concepts x ON x.section_id = s.id
			LEFT JOIN projects p ON p.id = doc.project_id
			WHERE x.concept_id = ? ORDER BY s.created_at DESC, s.id DESC LIMIT ?`},
	} {
		rows, err := kb.db.Query(q.query, id, limit)
		if err != nil {
			return d, fmt.Errorf("knowledge: list concept links: %w", err)
		}
		for rows.Next() {
			var it ConceptDetailItem
			var ts string
			if err := rows.Scan(&it.ID, &it.Project, &it.Title, &it.Body, &ts); err != nil {
				rows.Close()
				return d, err
			}
			it.CreatedAt = parseTimestamp(ts)
			*q.into = append(*q.into, it)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return d, err
		}
	}
	return d, nil
}

// conceptNotFoundHint wraps a not-found error for name with the concept names
// that differ from it only in case, so a mistyped case is a one-step fix.
func (kb *KnowledgeBase) conceptNotFoundHint(name string, err error) error {
	rows, qerr := kb.db.Query(`SELECT name FROM concepts WHERE name = ? COLLATE NOCASE ORDER BY name`, name)
	if qerr != nil {
		return err
	}
	defer rows.Close()
	var variants []string
	for rows.Next() {
		var v string
		if rows.Scan(&v) == nil {
			variants = append(variants, fmt.Sprintf("%q", v))
		}
	}
	if len(variants) == 0 {
		return err
	}
	return notFoundf("knowledge: concept %q not found (did you mean %s? names match exactly, including case)",
		name, strings.Join(variants, ", "))
}
