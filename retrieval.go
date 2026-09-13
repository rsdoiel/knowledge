package knowledge

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

/** ConceptMatch is one entity (an observation, a record, or a document
 * gist/section) linked to one or more concepts a RecallByConceptNames
 * caller asked about.
 *
 * Fields:
 *   SourceType    (string)    — "observation", "record", "document_gist", or
 *                               "document_section" -- the same vocabulary
 *                               kb_fts.source_type uses.
 *   ID            (int64)     — internal database id of the entity (for a
 *                               document match, the document_sections row).
 *   ProjectID     (int64)     — owning project id (0 for a workspace-tier record).
 *   Title         (string)    — "" for an observation; the record's or
 *                               document's title otherwise.
 *   Body          (string)    — the entity's text -- for a document match,
 *                               only when SummaryStatus == "reviewed" (see
 *                               narrative-documents-design.md decision 8):
 *                               an unreviewed summary is findable by tag,
 *                               but never surfaced as trustworthy content.
 *   SummaryStatus (string)    — "" for observation/record matches;
 *                               "unsummarized"/"drafted"/"reviewed" for a
 *                               document match, explaining why Body may be
 *                               empty rather than leaving the caller to guess.
 *   MatchCount    (int)       — how many of the queried concepts this entity is linked to.
 *   CreatedAt     (time.Time) — Observation.CreatedAt, Record.IngestedAt, or
 *                               the document section's CreatedAt.
 */
type ConceptMatch struct {
	SourceType    string
	ID            int64
	ProjectID     int64
	Title         string
	Body          string
	SummaryStatus string
	MatchCount    int
	CreatedAt     time.Time
}

// conceptIDsByNames resolves names to existing concept ids, case-insensitively,
// read-only: a name matching no concept is silently skipped, and no concept is
// ever created as a side effect (unlike ResolveConceptName, which is for
// ingest, not retrieval). Resolved ids are deduped, since two case-variant
// input names can resolve to the same concept.
func conceptIDsByNames(kb *KnowledgeBase, names []string) ([]int64, error) {
	seen := map[int64]bool{}
	var ids []int64
	for _, name := range names {
		var id int64
		err := kb.db.QueryRow(
			`SELECT id FROM concepts WHERE name = ?1 COLLATE NOCASE LIMIT 1`, name,
		).Scan(&id)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("knowledge: resolve concept name %q: %w", name, err)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

/** RecallByConceptNames returns observations and records linked to any of the
 * given concept names, merged into one list ranked by how many of the
 * queried concepts each entity matches (descending), then by recency
 * (descending), truncated to limit.
 *
 * A name matching no concept is silently skipped rather than erroring — a
 * prompt mentioning nothing tagged is a normal outcome, not a failure — and
 * no concept is ever created as a side effect of calling this (unlike
 * ResolveConceptName, used at ingest time). The query is not scoped to any
 * one project: ProjectID is reported on every result so a caller can filter
 * or weight afterward.
 *
 * Parameters:
 *   names ([]string) — concept names to match against, as returned by
 *                       MatchConceptNames or supplied directly.
 *   limit (int)       — maximum results to return; non-positive means no cap.
 *
 * Returns:
 *   []ConceptMatch — ranked matches; nil when none.
 *   error          — on database failure.
 *
 * Example:
 *   matches, err := kb.RecallByConceptNames([]string{"chunking", "RAG"}, 5)
 */
func (kb *KnowledgeBase) RecallByConceptNames(names []string, limit int) ([]ConceptMatch, error) {
	ids, err := conceptIDsByNames(kb, names)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	var matches []ConceptMatch

	obsRows, err := kb.db.Query(fmt.Sprintf(`
		SELECT o.id, o.project_id, o.body, o.created_at, COUNT(DISTINCT oc.concept_id) AS match_count
		FROM observations o
		JOIN observation_concepts oc ON oc.observation_id = o.id
		WHERE oc.concept_id IN (%s)
		GROUP BY o.id`, placeholders), args...)
	if err != nil {
		return nil, fmt.Errorf("knowledge: recall observations by concept: %w", err)
	}
	defer obsRows.Close()
	for obsRows.Next() {
		var m ConceptMatch
		var ts string
		if err := obsRows.Scan(&m.ID, &m.ProjectID, &m.Body, &ts, &m.MatchCount); err != nil {
			return nil, err
		}
		m.SourceType = "observation"
		m.CreatedAt = parseTimestamp(ts)
		matches = append(matches, m)
	}
	if err := obsRows.Err(); err != nil {
		return nil, err
	}

	recRows, err := kb.db.Query(fmt.Sprintf(`
		SELECT r.id, r.project_id, r.title, r.body, r.ingested_at, COUNT(DISTINCT rc.concept_id) AS match_count
		FROM records r
		JOIN record_concepts rc ON rc.record_id = r.id
		WHERE rc.concept_id IN (%s)
		GROUP BY r.id`, placeholders), args...)
	if err != nil {
		return nil, fmt.Errorf("knowledge: recall records by concept: %w", err)
	}
	defer recRows.Close()
	for recRows.Next() {
		var m ConceptMatch
		var ts string
		if err := recRows.Scan(&m.ID, &m.ProjectID, &m.Title, &m.Body, &ts, &m.MatchCount); err != nil {
			return nil, err
		}
		m.SourceType = "record"
		m.CreatedAt = parseTimestamp(ts)
		matches = append(matches, m)
	}
	if err := recRows.Err(); err != nil {
		return nil, err
	}

	docRows, err := kb.db.Query(fmt.Sprintf(`
		SELECT s.id, IFNULL(d.project_id, 0), d.title, s.level, s.heading, s.summary_body, s.summary_status,
		       s.created_at, COUNT(DISTINCT dsc.concept_id) AS match_count
		FROM document_sections s
		JOIN documents d ON d.id = s.document_id
		JOIN document_section_concepts dsc ON dsc.section_id = s.id
		WHERE dsc.concept_id IN (%s)
		GROUP BY s.id`, placeholders), args...)
	if err != nil {
		return nil, fmt.Errorf("knowledge: recall document sections by concept: %w", err)
	}
	defer docRows.Close()
	for docRows.Next() {
		var m ConceptMatch
		var level, heading, summaryBody, ts string
		if err := docRows.Scan(&m.ID, &m.ProjectID, &m.Title, &level, &heading, &summaryBody,
			&m.SummaryStatus, &ts, &m.MatchCount); err != nil {
			return nil, err
		}
		if level == "gist" {
			m.SourceType = "document_gist"
		} else {
			m.SourceType = "document_section"
			if heading != "" {
				m.Title = m.Title + " / " + heading
			}
		}
		// Design decision 8: only a reviewed summary is trustworthy content.
		// An unsummarized/drafted match still surfaces (findable by tag),
		// with SummaryStatus explaining why Body is empty.
		if m.SummaryStatus == "reviewed" {
			m.Body = summaryBody
		}
		m.CreatedAt = parseTimestamp(ts)
		matches = append(matches, m)
	}
	if err := docRows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].MatchCount != matches[j].MatchCount {
			return matches[i].MatchCount > matches[j].MatchCount
		}
		return matches[i].CreatedAt.After(matches[j].CreatedAt)
	})
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

/** MatchConceptNames returns every known concept name that appears in text as
 * a whole word, case-insensitively. Whole-word matching (not substring) is
 * deliberate: a short concept name like "RAG" must not match inside an
 * unrelated word like "storage". Concept names are otherwise free-form, so
 * this compiles one small regex per concept per call rather than assuming
 * any structure to exploit — call frequency here is "once per prompt," not a
 * hot loop.
 *
 * Parameters:
 *   text (string) — free text to search, e.g. a prompt.
 *
 * Returns:
 *   []string — matched concept names, in Concepts()'s own order; nil when none.
 *   error    — on database failure.
 *
 * Example:
 *   names, err := kb.MatchConceptNames("how does chunking interact with RAG?")
 */
func (kb *KnowledgeBase) MatchConceptNames(text string) ([]string, error) {
	concepts, err := kb.Concepts()
	if err != nil {
		return nil, err
	}
	var matched []string
	for _, c := range concepts {
		pattern := `(?i)\b` + regexp.QuoteMeta(c.Name) + `\b`
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		if re.MatchString(text) {
			matched = append(matched, c.Name)
		}
	}
	return matched, nil
}
