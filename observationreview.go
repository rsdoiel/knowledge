package knowledge

import (
	"database/sql"
	"strings"
)

/** AddObservationDraft records an observation written by a model. It is
 * stored with status "drafted", is not written to kb_fts, and stays out of
 * search and recall until a person promotes it with PromoteObservation
 * (DR-0090). The library has no terminal rule; the command line does.
 *
 * Parameters:
 *   projectID  (int64)    — ID of the owning project.
 *   kind       (string)   — one of: note, finding, decision, question, hypothesis.
 *   body       (string)   — the observation text.
 *   by         (string)   — who drafted it, e.g. "harvey qwen2.5:3b"; one non-blank line.
 *   confidence (*float64) — the drafter's estimate in 0 to 1, or nil.
 *
 * Returns:
 *   int64 — ID of the new drafted observation.
 *   error — ErrInvalid for a bad kind, blank body, bad by or confidence outside 0 to 1.
 *
 * Example:
 *   c := 0.7
 *   id, err := kb.AddObservationDraft(1, "finding", "WAL helps", "harvey qwen2.5:3b", &c)
 */
func (kb *KnowledgeBase) AddObservationDraft(projectID int64, kind, body, by string, confidence *float64) (int64, error) {
	if strings.TrimSpace(by) == "" || strings.ContainsAny(by, "\r\n") {
		return 0, invalidf("knowledge: drafter name must be one non-blank line")
	}
	if confidence != nil && (*confidence < 0 || *confidence > 1 || *confidence != *confidence) {
		return 0, invalidf("knowledge: confidence must be between 0 and 1")
	}
	return kb.insertObservation(projectID, kind, body, "", "drafted", by, confidence)
}

/** PromoteObservation makes a drafted observation reviewed and indexes it, so
 * search and recall find it. It has no terminal rule (DR-0070); the command
 * `kb observation review promote` adds that.
 *
 * Parameters:
 *   id (int64) — the observation to promote.
 *
 * Returns:
 *   error — ErrNotFound for an unknown id, ErrConflict when it is not drafted.
 *
 * Example:
 *   err := kb.PromoteObservation(12)
 */
func (kb *KnowledgeBase) PromoteObservation(id int64) error {
	var status, kind, body string
	var projectID sql.NullInt64
	err := kb.db.QueryRow(`SELECT status, kind, body, project_id FROM observations WHERE id = ?`, id).
		Scan(&status, &kind, &body, &projectID)
	if err == sql.ErrNoRows {
		return notFoundf("knowledge: no observation with id %d", id)
	}
	if err != nil {
		return err
	}
	if status != "drafted" {
		return conflictf("knowledge: observation %d is %q, not drafted", id, status)
	}
	res, err := kb.db.Exec(`UPDATE observations SET status = 'reviewed' WHERE id = ? AND status = 'drafted'`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return conflictf("knowledge: observation %d is not drafted", id)
	}
	if kb.ftsAvailable {
		_, _ = kb.db.Exec(`DELETE FROM kb_fts WHERE source_type = 'observation' AND source_id = ?`, id)
		_, _ = kb.db.Exec(
			`INSERT INTO kb_fts(body, kind, label, descr, source_type, source_id, project_id)
			 VALUES (?, ?, '', '', 'observation', ?, ?)`,
			body, kind, id, projectID.Int64)
	}
	return nil
}

/** ObservationReviewQueue returns the drafted observations awaiting review,
 * oldest first. A draft that a newer observation supersedes is not queued: the
 * newer one is what is left to review.
 *
 * Parameters:
 *   projectID (int64) — scope to one project, or 0 for every project.
 *
 * Returns:
 *   []Observation — drafted rows; nil when none.
 *   error         — on database failure.
 *
 * Example:
 *   queue, err := kb.ObservationReviewQueue(0)
 */
func (kb *KnowledgeBase) ObservationReviewQueue(projectID int64) ([]Observation, error) {
	q := `SELECT id, IFNULL(project_id, 0), kind, body, source_doi, created_at, status, generated_by, confidence
	      FROM observations WHERE status = 'drafted'
	        AND id NOT IN (SELECT to_id FROM observation_relations WHERE relationship = 'supersedes')`
	var args []any
	if projectID != 0 {
		q += " AND project_id = ?"
		args = append(args, projectID)
	}
	q += " ORDER BY id"
	rows, err := kb.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Observation
	for rows.Next() {
		var o Observation
		var ts string
		var conf sql.NullFloat64
		if err := rows.Scan(&o.ID, &o.ProjectID, &o.Kind, &o.Body, &o.SourceDOI, &ts, &o.Status, &o.GeneratedBy, &conf); err != nil {
			return nil, err
		}
		o.CreatedAt = parseTimestamp(ts)
		if conf.Valid {
			v := conf.Float64
			o.Confidence = &v
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

/** ObservationsByStatus returns a project's observations with the given
 * status, newest first. Observations is this with "reviewed". A drafted
 * observation that a newer one supersedes is not returned.
 *
 * Parameters:
 *   projectID (int64)  — ID of the project to query.
 *   status    (string) — "reviewed", "drafted" or "all".
 *
 * Returns:
 *   []Observation — matching rows; empty (not nil) if none.
 *   error         — ErrInvalid for an unknown status, or on database failure.
 *
 * Example:
 *   drafts, err := kb.ObservationsByStatus(1, "drafted")
 */
func (kb *KnowledgeBase) ObservationsByStatus(projectID int64, status string) ([]Observation, error) {
	switch status {
	case "reviewed", "drafted", "all":
	default:
		return nil, invalidf("knowledge: unknown observation status %q; must be reviewed, drafted or all", status)
	}
	q := `SELECT id, project_id, kind, body, source_doi, created_at, status, generated_by, confidence
	      FROM observations WHERE project_id = ?`
	args := []any{projectID}
	if status != "all" {
		q += " AND status = ?"
		args = append(args, status)
	}
	if status == "drafted" {
		// A draft a newer observation supersedes is no longer awaiting review.
		q += " AND id NOT IN (SELECT to_id FROM observation_relations WHERE relationship = 'supersedes')"
	}
	q += " ORDER BY created_at DESC"
	rows, err := kb.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Observation{}
	for rows.Next() {
		var o Observation
		var ts string
		var conf sql.NullFloat64
		if err := rows.Scan(&o.ID, &o.ProjectID, &o.Kind, &o.Body, &o.SourceDOI, &ts, &o.Status, &o.GeneratedBy, &conf); err != nil {
			return nil, err
		}
		o.CreatedAt = parseTimestamp(ts)
		if conf.Valid {
			v := conf.Float64
			o.Confidence = &v
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
