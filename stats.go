package knowledge

import "fmt"

/** StatCount is one named number in a Stats report.
 *
 * Fields:
 *   Name  (string) — the table, join table or check the number is about.
 *   Count (int)    — how many rows.
 *
 * Example:
 *   s, _ := kb.Stats()
 *   for _, c := range s.Rows { fmt.Println(c.Name, c.Count) }
 */
type StatCount struct {
	Name  string
	Count int
}

/** ProjectStats is one project's line in a Stats report.
 *
 * Fields:
 *   Name         (string) — the project's name.
 *   Observations (int)    — its observations.
 *   Records      (int)    — its decision records.
 *   Documents    (int)    — its documents.
 *   Concepts     (int)    — its concepts: the union of those tagged on the
 *                           project, on its observations and on its records,
 *                           each counted once (DR-0073).
 *
 * Example:
 *   s, _ := kb.Stats()
 *   fmt.Println(s.Projects[0].Name, s.Projects[0].Concepts)
 */
type ProjectStats struct {
	Name         string
	Observations int
	Records      int
	Documents    int
	Concepts     int
}

/** Stats is the shape of a database: how many rows each table has, how many
 * links each join table has, the facts a curator otherwise checks with SQL,
 * and one line per project. Every list is in a fixed order.
 *
 * Fields:
 *   Rows     ([]StatCount)    — rows per table.
 *   Links    ([]StatCount)    — rows per join table.
 *   Checks   ([]StatCount)    — things that are worth knowing and usually zero
 *                               (an observation with no project) or are not
 *                               an error (a concept nothing links to).
 *   Projects ([]ProjectStats) — per project, by name.
 *
 * Example:
 *   s, err := kb.Stats()
 */
type Stats struct {
	Rows     []StatCount
	Links    []StatCount
	Checks   []StatCount
	Projects []ProjectStats
}

// statRows, statLinks and statChecks are the counts of Stats, in the order they are reported.
var statRows = []struct{ name, query string }{
	{"projects", `SELECT COUNT(*) FROM projects`},
	{"observations", `SELECT COUNT(*) FROM observations`},
	{"concepts", `SELECT COUNT(*) FROM concepts`},
	{"sources", `SELECT COUNT(*) FROM sources`},
	{"records", `SELECT COUNT(*) FROM records`},
	{"documents", `SELECT COUNT(*) FROM documents`},
	{"document_sections", `SELECT COUNT(*) FROM document_sections`},
}

var statLinks = []struct{ name, query string }{
	{"project_concepts", `SELECT COUNT(*) FROM project_concepts`},
	{"observation_concepts", `SELECT COUNT(*) FROM observation_concepts`},
	{"record_concepts", `SELECT COUNT(*) FROM record_concepts`},
	{"document_section_concepts", `SELECT COUNT(*) FROM document_section_concepts`},
	{"observation_sources", `SELECT COUNT(*) FROM observation_sources`},
	{"observation_relations", `SELECT COUNT(*) FROM observation_relations`},
	{"record_relations", `SELECT COUNT(*) FROM record_relations`},
}

var statChecks = []struct{ name, query string }{
	{"observations_without_project", `SELECT COUNT(*) FROM observations WHERE project_id IS NULL`},
	{"concepts_without_links", `SELECT COUNT(*) FROM concepts c WHERE
		NOT EXISTS (SELECT 1 FROM project_concepts WHERE concept_id = c.id)
		AND NOT EXISTS (SELECT 1 FROM observation_concepts WHERE concept_id = c.id)
		AND NOT EXISTS (SELECT 1 FROM record_concepts WHERE concept_id = c.id)
		AND NOT EXISTS (SELECT 1 FROM document_section_concepts WHERE concept_id = c.id)`},
	{"sources_without_links", `SELECT COUNT(*) FROM sources s WHERE
		NOT EXISTS (SELECT 1 FROM observation_sources WHERE source_id = s.id)`},
	// Workspace-tier records have no project row by design; one whose scope says
	// project and has none is the unusual case.
	{"records_without_project", `SELECT COUNT(*) FROM records WHERE scope = 'project' AND project_id IS NULL`},
	{"documents_without_project", `SELECT COUNT(*) FROM documents WHERE project_id IS NULL`},
}

/** Stats reports the shape of the database without changing anything.
 *
 * Returns:
 *   Stats — the counts, checks and per-project lines.
 *   error — a database failure.
 *
 * Example:
 *   s, err := kb.Stats()
 *   if err == nil { fmt.Println(len(s.Projects), "projects") }
 */
func (kb *KnowledgeBase) Stats() (Stats, error) {
	var s Stats
	for _, g := range []struct {
		into    *[]StatCount
		queries []struct{ name, query string }
	}{{&s.Rows, statRows}, {&s.Links, statLinks}, {&s.Checks, statChecks}} {
		for _, q := range g.queries {
			var n int
			if err := kb.db.QueryRow(q.query).Scan(&n); err != nil {
				return s, fmt.Errorf("knowledge: stats %s: %w", q.name, err)
			}
			*g.into = append(*g.into, StatCount{q.name, n})
		}
	}
	rows, err := kb.db.Query(`SELECT p.name,
		(SELECT COUNT(*) FROM observations WHERE project_id = p.id),
		(SELECT COUNT(*) FROM records WHERE project_id = p.id),
		(SELECT COUNT(*) FROM documents WHERE project_id = p.id),
		(SELECT COUNT(*) FROM (
			SELECT concept_id FROM project_concepts WHERE project_id = p.id
			UNION SELECT x.concept_id FROM observation_concepts x
				JOIN observations o ON o.id = x.observation_id WHERE o.project_id = p.id
			UNION SELECT x.concept_id FROM record_concepts x
				JOIN records r ON r.id = x.record_id WHERE r.project_id = p.id))
		FROM projects p ORDER BY p.name`)
	if err != nil {
		return s, fmt.Errorf("knowledge: stats projects: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p ProjectStats
		if err := rows.Scan(&p.Name, &p.Observations, &p.Records, &p.Documents, &p.Concepts); err != nil {
			return s, err
		}
		s.Projects = append(s.Projects, p)
	}
	return s, rows.Err()
}
