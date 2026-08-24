package knowledge

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
)

// Record type discriminators used by the "type" field of every JSON-L line.
// See jsonl-export-design.md for the full format rationale.
const (
	recProject            = "project"
	recConcept            = "concept"
	recSource             = "source"
	recObservation        = "observation"
	recObservationConcept = "observation_concept"
	recProjectConcept     = "project_concept"
	recObservationSource  = "observation_source"
)

// projectRecord is the JSON-L shape of one projects row. Identity travels
// by UUID, not the local autoincrement id -- see jsonl-export-design.md.
type projectRecord struct {
	Type        string `json:"type"`
	UUID        string `json:"uuid"`
	OriginHost  string `json:"origin_host"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

// conceptRecord is the JSON-L shape of one concepts row.
type conceptRecord struct {
	Type            string `json:"type"`
	UUID            string `json:"uuid"`
	OriginHost      string `json:"origin_host"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	IdentifierType  string `json:"identifier_type"`
	IdentifierValue string `json:"identifier_value"`
	CreatedAt       string `json:"created_at"`
}

// sourceRecord is the JSON-L shape of one sources row.
type sourceRecord struct {
	Type            string `json:"type"`
	UUID            string `json:"uuid"`
	OriginHost      string `json:"origin_host"`
	Title           string `json:"title"`
	IdentifierType  string `json:"identifier_type"`
	IdentifierValue string `json:"identifier_value"`
	Authors         string `json:"authors"`
	PublishedDate   string `json:"published_date"`
	Publisher       string `json:"publisher"`
	Rights          string `json:"rights"`
	Version         string `json:"version"`
	Retracted       bool   `json:"retracted"`
	RetractionNote  string `json:"retraction_note"`
	FirstSeenAt     string `json:"first_seen_at"`
}

// observationRecord is the JSON-L shape of one observations row.
// ProjectUUID is the owning project's uuid, not its local id.
type observationRecord struct {
	Type        string `json:"type"`
	UUID        string `json:"uuid"`
	OriginHost  string `json:"origin_host"`
	ProjectUUID string `json:"project_uuid"`
	Kind        string `json:"kind"`
	Body        string `json:"body"`
	SourceDOI   string `json:"source_doi"`
	CreatedAt   string `json:"created_at"`
}

// observationConceptRecord is the JSON-L shape of one observation_concepts
// join row, referencing both endpoints by uuid.
type observationConceptRecord struct {
	Type            string `json:"type"`
	ObservationUUID string `json:"observation_uuid"`
	ConceptUUID     string `json:"concept_uuid"`
}

// projectConceptRecord is the JSON-L shape of one project_concepts join row.
type projectConceptRecord struct {
	Type        string `json:"type"`
	ProjectUUID string `json:"project_uuid"`
	ConceptUUID string `json:"concept_uuid"`
}

// observationSourceRecord is the JSON-L shape of one observation_sources
// join row.
type observationSourceRecord struct {
	Type            string `json:"type"`
	ObservationUUID string `json:"observation_uuid"`
	SourceUUID      string `json:"source_uuid"`
	Relationship    string `json:"relationship"`
}

/** ExportJSONL writes the knowledge base (or, when projectName is
 * non-empty, just the reachable subset of one project) as a stream of
 * newline-delimited JSON records to w -- one project/concept/source/
 * observation/link per line, in dependency order (parents before
 * children), each self-describing via a "type" field. See
 * jsonl-export-design.md for the full format and the reachability rule
 * used when projectName is set. Import with ImportJSONL.
 *
 * Parameters:
 *   kb          (*KnowledgeBase) — the knowledge base to export.
 *   w           (io.Writer)      — destination for the JSON-L stream.
 *   projectName (string)         — "" exports every project; a specific
 *                                  name exports only that project and
 *                                  everything reachable from it.
 *
 * Returns:
 *   error — if projectName is set but no such project exists, or on
 *           database/encoding failure.
 *
 * Example:
 *   f, _ := os.Create("export.jsonl")
 *   defer f.Close()
 *   err := ExportJSONL(kb, f, "") // whole database
 */
func ExportJSONL(kb *KnowledgeBase, w io.Writer, projectName string) error {
	var projectID int64
	scoped := projectName != ""
	if scoped {
		p, err := kb.ProjectByName(projectName)
		if err != nil {
			return fmt.Errorf("knowledge: export: %w", err)
		}
		if p == nil {
			return fmt.Errorf("knowledge: export: project %q not found", projectName)
		}
		projectID = p.ID
	}

	enc := json.NewEncoder(w)

	if err := exportProjects(kb, enc, scoped, projectID); err != nil {
		return err
	}
	if err := exportConcepts(kb, enc, scoped, projectID); err != nil {
		return err
	}
	if err := exportSources(kb, enc, scoped, projectID); err != nil {
		return err
	}
	if err := exportObservations(kb, enc, scoped, projectID); err != nil {
		return err
	}
	if err := exportObservationConcepts(kb, enc, scoped, projectID); err != nil {
		return err
	}
	if err := exportProjectConcepts(kb, enc, scoped, projectID); err != nil {
		return err
	}
	if err := exportObservationSources(kb, enc, scoped, projectID); err != nil {
		return err
	}
	return nil
}

func exportProjects(kb *KnowledgeBase, enc *json.Encoder, scoped bool, projectID int64) error {
	query := `SELECT id, uuid, origin_host, name, description, status, created_at FROM projects`
	args := []any{}
	if scoped {
		query += ` WHERE id = ?`
		args = append(args, projectID)
	}
	query += ` ORDER BY created_at`
	rows, err := kb.db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("knowledge: export projects: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		r := projectRecord{Type: recProject}
		if err := rows.Scan(&id, &r.UUID, &r.OriginHost, &r.Name, &r.Description, &r.Status, &r.CreatedAt); err != nil {
			return fmt.Errorf("knowledge: export projects: %w", err)
		}
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("knowledge: export projects: %w", err)
		}
	}
	return rows.Err()
}

func exportConcepts(kb *KnowledgeBase, enc *json.Encoder, scoped bool, projectID int64) error {
	query := `SELECT id, uuid, origin_host, name, description, identifier_type, identifier_value, created_at FROM concepts`
	args := []any{}
	if scoped {
		query += ` WHERE id IN (
			SELECT concept_id FROM project_concepts WHERE project_id = ?
			UNION
			SELECT oc.concept_id FROM observation_concepts oc
			JOIN observations o ON o.id = oc.observation_id
			WHERE o.project_id = ?
		)`
		args = append(args, projectID, projectID)
	}
	query += ` ORDER BY name`
	rows, err := kb.db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("knowledge: export concepts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		r := conceptRecord{Type: recConcept}
		if err := rows.Scan(&id, &r.UUID, &r.OriginHost, &r.Name, &r.Description, &r.IdentifierType, &r.IdentifierValue, &r.CreatedAt); err != nil {
			return fmt.Errorf("knowledge: export concepts: %w", err)
		}
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("knowledge: export concepts: %w", err)
		}
	}
	return rows.Err()
}

func exportSources(kb *KnowledgeBase, enc *json.Encoder, scoped bool, projectID int64) error {
	query := `SELECT id, uuid, origin_host, title, identifier_type, identifier_value,
	                 authors, published_date, publisher, rights, version,
	                 retracted, retraction_note, first_seen_at
	          FROM sources`
	args := []any{}
	if scoped {
		query += ` WHERE id IN (
			SELECT os.source_id FROM observation_sources os
			JOIN observations o ON o.id = os.observation_id
			WHERE o.project_id = ?
		)`
		args = append(args, projectID)
	}
	query += ` ORDER BY id`
	rows, err := kb.db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("knowledge: export sources: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var retracted int
		r := sourceRecord{Type: recSource}
		if err := rows.Scan(&id, &r.UUID, &r.OriginHost, &r.Title, &r.IdentifierType, &r.IdentifierValue,
			&r.Authors, &r.PublishedDate, &r.Publisher, &r.Rights, &r.Version,
			&retracted, &r.RetractionNote, &r.FirstSeenAt); err != nil {
			return fmt.Errorf("knowledge: export sources: %w", err)
		}
		r.Retracted = retracted != 0
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("knowledge: export sources: %w", err)
		}
	}
	return rows.Err()
}

func exportObservations(kb *KnowledgeBase, enc *json.Encoder, scoped bool, projectID int64) error {
	query := `SELECT o.id, o.uuid, o.origin_host, p.uuid, o.kind, o.body, o.source_doi, o.created_at
	          FROM observations o
	          JOIN projects p ON p.id = o.project_id`
	args := []any{}
	if scoped {
		query += ` WHERE o.project_id = ?`
		args = append(args, projectID)
	}
	query += ` ORDER BY o.created_at`
	rows, err := kb.db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("knowledge: export observations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		r := observationRecord{Type: recObservation}
		if err := rows.Scan(&id, &r.UUID, &r.OriginHost, &r.ProjectUUID, &r.Kind, &r.Body, &r.SourceDOI, &r.CreatedAt); err != nil {
			return fmt.Errorf("knowledge: export observations: %w", err)
		}
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("knowledge: export observations: %w", err)
		}
	}
	return rows.Err()
}

func exportObservationConcepts(kb *KnowledgeBase, enc *json.Encoder, scoped bool, projectID int64) error {
	query := `SELECT o.uuid, c.uuid
	          FROM observation_concepts oc
	          JOIN observations o ON o.id = oc.observation_id
	          JOIN concepts c ON c.id = oc.concept_id`
	args := []any{}
	if scoped {
		query += ` WHERE o.project_id = ?`
		args = append(args, projectID)
	}
	rows, err := kb.db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("knowledge: export observation_concepts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		r := observationConceptRecord{Type: recObservationConcept}
		if err := rows.Scan(&r.ObservationUUID, &r.ConceptUUID); err != nil {
			return fmt.Errorf("knowledge: export observation_concepts: %w", err)
		}
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("knowledge: export observation_concepts: %w", err)
		}
	}
	return rows.Err()
}

func exportProjectConcepts(kb *KnowledgeBase, enc *json.Encoder, scoped bool, projectID int64) error {
	query := `SELECT p.uuid, c.uuid
	          FROM project_concepts pc
	          JOIN projects p ON p.id = pc.project_id
	          JOIN concepts c ON c.id = pc.concept_id`
	args := []any{}
	if scoped {
		query += ` WHERE pc.project_id = ?`
		args = append(args, projectID)
	}
	rows, err := kb.db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("knowledge: export project_concepts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		r := projectConceptRecord{Type: recProjectConcept}
		if err := rows.Scan(&r.ProjectUUID, &r.ConceptUUID); err != nil {
			return fmt.Errorf("knowledge: export project_concepts: %w", err)
		}
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("knowledge: export project_concepts: %w", err)
		}
	}
	return rows.Err()
}

func exportObservationSources(kb *KnowledgeBase, enc *json.Encoder, scoped bool, projectID int64) error {
	query := `SELECT o.uuid, s.uuid, os.relationship
	          FROM observation_sources os
	          JOIN observations o ON o.id = os.observation_id
	          JOIN sources s ON s.id = os.source_id`
	args := []any{}
	if scoped {
		query += ` WHERE o.project_id = ?`
		args = append(args, projectID)
	}
	rows, err := kb.db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("knowledge: export observation_sources: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		r := observationSourceRecord{Type: recObservationSource}
		if err := rows.Scan(&r.ObservationUUID, &r.SourceUUID, &r.Relationship); err != nil {
			return fmt.Errorf("knowledge: export observation_sources: %w", err)
		}
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("knowledge: export observation_sources: %w", err)
		}
	}
	return rows.Err()
}

// ─── Import ───────────────────────────────────────────────────────────────

/** ImportTableSummary reports, per JSON-L record type, how many lines were
 * read, how many resulted in a new row, and how many were skipped —
 * because a matching row already existed locally, or (for join records)
 * because one of the referenced uuids couldn't be resolved.
 */
type ImportTableSummary struct {
	Table    string
	Read     int
	Imported int
	Skipped  int
}

/** ImportJSONL reads a stream written by ExportJSONL (or hand-built in the
 * same format) and applies it to kb, an already-open, live knowledge base.
 * The whole stream is buffered and grouped by record type before anything
 * is written, then applied in dependency order (projects/concepts/sources,
 * then observations, then the three join types) regardless of the order
 * lines appeared in r — see jsonl-export-design.md for the full identity
 * and conflict-resolution rules (name-keyed for projects/concepts,
 * identifier-keyed for sources, uuid-keyed for observations and joins).
 * Unresolvable references and unrecognized "type" values are skipped, not
 * fatal; only malformed JSON aborts the import.
 *
 * Parameters:
 *   kb (*KnowledgeBase) — the live knowledge base to import into.
 *   r  (io.Reader)      — a JSON-L stream, as produced by ExportJSONL.
 *
 * Returns:
 *   []ImportTableSummary — one entry per record type seen (including any
 *                          unrecognized type, reported with Imported=0).
 *   error                 — on malformed JSON (with the offending line
 *                          number) or database failure.
 *
 * Example:
 *   f, _ := os.Open("export.jsonl")
 *   defer f.Close()
 *   summary, err := ImportJSONL(kb, f)
 */
func ImportJSONL(kb *KnowledgeBase, r io.Reader) ([]ImportTableSummary, error) {
	var (
		projects     []projectRecord
		concepts     []conceptRecord
		sources      []sourceRecord
		observations []observationRecord
		obsConcepts  []observationConceptRecord
		projConcepts []projectConceptRecord
		obsSources   []observationSourceRecord
	)
	readCount := map[string]int{}
	var typeOrder []string // first-seen order, so unknown types get a stable summary order too

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			return nil, fmt.Errorf("knowledge: import: line %d: %w", lineNo, err)
		}
		if _, seen := readCount[envelope.Type]; !seen {
			typeOrder = append(typeOrder, envelope.Type)
		}
		readCount[envelope.Type]++

		switch envelope.Type {
		case recProject:
			var rec projectRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				return nil, fmt.Errorf("knowledge: import: line %d: %w", lineNo, err)
			}
			projects = append(projects, rec)
		case recConcept:
			var rec conceptRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				return nil, fmt.Errorf("knowledge: import: line %d: %w", lineNo, err)
			}
			concepts = append(concepts, rec)
		case recSource:
			var rec sourceRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				return nil, fmt.Errorf("knowledge: import: line %d: %w", lineNo, err)
			}
			sources = append(sources, rec)
		case recObservation:
			var rec observationRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				return nil, fmt.Errorf("knowledge: import: line %d: %w", lineNo, err)
			}
			observations = append(observations, rec)
		case recObservationConcept:
			var rec observationConceptRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				return nil, fmt.Errorf("knowledge: import: line %d: %w", lineNo, err)
			}
			obsConcepts = append(obsConcepts, rec)
		case recProjectConcept:
			var rec projectConceptRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				return nil, fmt.Errorf("knowledge: import: line %d: %w", lineNo, err)
			}
			projConcepts = append(projConcepts, rec)
		case recObservationSource:
			var rec observationSourceRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				return nil, fmt.Errorf("knowledge: import: line %d: %w", lineNo, err)
			}
			obsSources = append(obsSources, rec)
		default:
			// Unknown type: counted (readCount above) but nothing to buffer.
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("knowledge: import: %w", err)
	}

	summaries := map[string]*ImportTableSummary{}
	summaryFor := func(table string) *ImportTableSummary {
		if s, ok := summaries[table]; ok {
			return s
		}
		s := &ImportTableSummary{Table: table, Read: readCount[table]}
		summaries[table] = s
		return s
	}
	for _, ty := range typeOrder {
		summaryFor(ty)
	}

	projectLocalID := map[string]int64{} // incoming project uuid -> local id
	conceptLocalID := map[string]int64{} // incoming concept uuid -> local id
	sourceLocalID := map[string]int64{}  // incoming source uuid -> local id

	s := summaryFor(recProject)
	for _, rec := range projects {
		localID, isNew, err := importProject(kb, rec)
		if err != nil {
			return nil, fmt.Errorf("knowledge: import project %q: %w", rec.Name, err)
		}
		projectLocalID[rec.UUID] = localID
		if isNew {
			s.Imported++
		} else {
			s.Skipped++
		}
	}

	s = summaryFor(recConcept)
	for _, rec := range concepts {
		localID, isNew, err := importConcept(kb, rec)
		if err != nil {
			return nil, fmt.Errorf("knowledge: import concept %q: %w", rec.Name, err)
		}
		conceptLocalID[rec.UUID] = localID
		if isNew {
			s.Imported++
		} else {
			s.Skipped++
		}
	}

	s = summaryFor(recSource)
	for _, rec := range sources {
		localID, isNew, err := importSource(kb, rec)
		if err != nil {
			return nil, fmt.Errorf("knowledge: import source %q: %w", rec.Title, err)
		}
		sourceLocalID[rec.UUID] = localID
		if isNew {
			s.Imported++
		} else {
			s.Skipped++
		}
	}

	s = summaryFor(recObservation)
	for _, rec := range observations {
		localProjectID, ok := resolveLocalID(kb, projectLocalID, "projects", rec.ProjectUUID)
		if !ok {
			s.Skipped++
			continue
		}
		isNew, err := importObservation(kb, rec, localProjectID)
		if err != nil {
			return nil, fmt.Errorf("knowledge: import observation %q: %w", rec.UUID, err)
		}
		if isNew {
			s.Imported++
		} else {
			s.Skipped++
		}
	}

	s = summaryFor(recObservationConcept)
	for _, rec := range obsConcepts {
		obsID, ok := resolveLocalID(kb, nil, "observations", rec.ObservationUUID)
		conceptID, cok := resolveLocalID(kb, conceptLocalID, "concepts", rec.ConceptUUID)
		if !ok || !cok {
			s.Skipped++
			continue
		}
		isNew, err := insertOrIgnore(kb, `INSERT OR IGNORE INTO observation_concepts (observation_id, concept_id) VALUES (?, ?)`, obsID, conceptID)
		if err != nil {
			return nil, fmt.Errorf("knowledge: import observation_concept: %w", err)
		}
		if isNew {
			s.Imported++
		} else {
			s.Skipped++
		}
	}

	s = summaryFor(recProjectConcept)
	for _, rec := range projConcepts {
		projectID, ok := resolveLocalID(kb, projectLocalID, "projects", rec.ProjectUUID)
		conceptID, cok := resolveLocalID(kb, conceptLocalID, "concepts", rec.ConceptUUID)
		if !ok || !cok {
			s.Skipped++
			continue
		}
		isNew, err := insertOrIgnore(kb, `INSERT OR IGNORE INTO project_concepts (project_id, concept_id) VALUES (?, ?)`, projectID, conceptID)
		if err != nil {
			return nil, fmt.Errorf("knowledge: import project_concept: %w", err)
		}
		if isNew {
			s.Imported++
		} else {
			s.Skipped++
		}
	}

	s = summaryFor(recObservationSource)
	for _, rec := range obsSources {
		obsID, ok := resolveLocalID(kb, nil, "observations", rec.ObservationUUID)
		sourceID, sok := resolveLocalID(kb, sourceLocalID, "sources", rec.SourceUUID)
		if !ok || !sok {
			s.Skipped++
			continue
		}
		isNew, err := insertOrIgnore(kb, `INSERT OR IGNORE INTO observation_sources (observation_id, source_id, relationship) VALUES (?, ?, ?)`, obsID, sourceID, rec.Relationship)
		if err != nil {
			return nil, fmt.Errorf("knowledge: import observation_source: %w", err)
		}
		if isNew {
			s.Imported++
		} else {
			s.Skipped++
		}
	}

	out := make([]ImportTableSummary, 0, len(typeOrder))
	for _, ty := range typeOrder {
		out = append(out, *summaries[ty])
	}
	return out, nil
}

// importProject resolves rec by name (the same identity key AddProject
// itself uses): an existing local row wins as-is (isNew=false, its own
// uuid/description untouched), otherwise a new row is inserted carrying
// rec's own uuid/origin_host/created_at so a later cross-machine merge can
// still recognize it. See jsonl-export-design.md's "Import identity key"
// section for why this differs from the uuid-keyed observations path.
func importProject(kb *KnowledgeBase, rec projectRecord) (localID int64, isNew bool, err error) {
	err = kb.db.QueryRow(`SELECT id FROM projects WHERE name = ?`, rec.Name).Scan(&localID)
	if err == nil {
		return localID, false, nil
	}
	if err != sql.ErrNoRows {
		return 0, false, err
	}
	status := rec.Status
	if !validProjectStatuses[status] {
		status = "active"
	}
	res, err := kb.db.Exec(
		`INSERT INTO projects (name, description, status, created_at, uuid, origin_host) VALUES (?, ?, ?, ?, ?, ?)`,
		rec.Name, rec.Description, status, rec.CreatedAt, rec.UUID, rec.OriginHost,
	)
	if err != nil {
		return 0, false, err
	}
	localID, err = res.LastInsertId()
	if err != nil {
		return 0, false, err
	}
	if kb.ftsAvailable {
		_, _ = kb.db.Exec(
			`INSERT INTO kb_fts(body, kind, label, descr, source_type, source_id, project_id)
			 VALUES (?, 'project', ?, ?, 'project', ?, ?)`,
			rec.Name+" "+rec.Description, rec.Name, rec.Description, localID, localID)
	}
	return localID, true, nil
}

// importConcept mirrors importProject's name-keyed identity rule.
func importConcept(kb *KnowledgeBase, rec conceptRecord) (localID int64, isNew bool, err error) {
	err = kb.db.QueryRow(`SELECT id FROM concepts WHERE name = ?`, rec.Name).Scan(&localID)
	if err == nil {
		return localID, false, nil
	}
	if err != sql.ErrNoRows {
		return 0, false, err
	}
	res, err := kb.db.Exec(
		`INSERT INTO concepts (name, description, identifier_type, identifier_value, created_at, uuid, origin_host)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.Name, rec.Description, rec.IdentifierType, rec.IdentifierValue, rec.CreatedAt, rec.UUID, rec.OriginHost,
	)
	if err != nil {
		return 0, false, err
	}
	localID, err = res.LastInsertId()
	if err != nil {
		return 0, false, err
	}
	if kb.ftsAvailable {
		_, _ = kb.db.Exec(
			`INSERT INTO kb_fts(body, kind, label, descr, source_type, source_id, project_id)
			 VALUES (?, 'concept', ?, ?, 'concept', ?, 0)`,
			rec.Name+" "+rec.Description, rec.Name, rec.Description, localID)
	}
	return localID, true, nil
}

// importSource mirrors AddSource's identity rule: dedupe by
// (identifier_type, identifier_value) when both are set. A source with
// neither has no natural content key -- AddSource's own answer to that
// case (always insert a fresh row) is fine for a single call, but here it
// would violate the sources.uuid UNIQUE index the moment the same export
// file is imported a second time, since importSource always carries the
// source's own uuid through. A uuid fallback check makes re-import
// idempotent for identifier-less sources without changing AddSource's
// behavior for anything else.
func importSource(kb *KnowledgeBase, rec sourceRecord) (localID int64, isNew bool, err error) {
	if rec.IdentifierType != "" && rec.IdentifierValue != "" {
		err = kb.db.QueryRow(
			`SELECT id FROM sources WHERE identifier_type = ? AND identifier_value = ?`,
			rec.IdentifierType, rec.IdentifierValue,
		).Scan(&localID)
		if err == nil {
			return localID, false, nil
		}
		if err != sql.ErrNoRows {
			return 0, false, err
		}
	}
	err = kb.db.QueryRow(`SELECT id FROM sources WHERE uuid = ?`, rec.UUID).Scan(&localID)
	if err == nil {
		return localID, false, nil
	}
	if err != sql.ErrNoRows {
		return 0, false, err
	}
	retracted := 0
	if rec.Retracted {
		retracted = 1
	}
	res, err := kb.db.Exec(
		`INSERT INTO sources (title, identifier_type, identifier_value, authors, published_date,
		                       publisher, rights, version, retracted, retraction_note, first_seen_at, uuid, origin_host)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.Title, rec.IdentifierType, rec.IdentifierValue, rec.Authors, rec.PublishedDate,
		rec.Publisher, rec.Rights, rec.Version, retracted, rec.RetractionNote, rec.FirstSeenAt, rec.UUID, rec.OriginHost,
	)
	if err != nil {
		return 0, false, err
	}
	localID, err = res.LastInsertId()
	return localID, true, err
}

// importObservation resolves rec by its own uuid -- unlike
// projects/concepts/sources, observations have no natural content key, so
// uuid (already UNIQUE-indexed by Open's backfillUUIDs) is authoritative.
func importObservation(kb *KnowledgeBase, rec observationRecord, localProjectID int64) (isNew bool, err error) {
	var existingID int64
	err = kb.db.QueryRow(`SELECT id FROM observations WHERE uuid = ?`, rec.UUID).Scan(&existingID)
	if err == nil {
		return false, nil
	}
	if err != sql.ErrNoRows {
		return false, err
	}
	kind := rec.Kind
	if !IsValidKind(kind) {
		kind = "note"
	}
	res, err := kb.db.Exec(
		`INSERT INTO observations (project_id, kind, body, source_doi, created_at, uuid, origin_host)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		localProjectID, kind, rec.Body, rec.SourceDOI, rec.CreatedAt, rec.UUID, rec.OriginHost,
	)
	if err != nil {
		return false, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return false, err
	}
	if kb.ftsAvailable {
		_, _ = kb.db.Exec(
			`INSERT INTO kb_fts(body, kind, label, descr, source_type, source_id, project_id)
			 VALUES (?, ?, '', '', 'observation', ?, ?)`,
			rec.Body, kind, id, localProjectID)
	}
	return true, nil
}

// resolveLocalID looks up a row's local id by its uuid in table, checking
// cache first when non-nil. A cache miss falls through to a direct
// database lookup rather than failing outright: the JSON-L file being
// imported may be a partial/incremental export (e.g. new observations for
// a project that was already synced in an earlier import, so this file
// never re-includes the project record itself) whose parent nonetheless
// already exists locally under that exact uuid. Table must be one of
// "projects", "concepts", "sources", or "observations" -- every one of
// them carries a UNIQUE-indexed uuid column (see Open's backfillUUIDs).
// A successful database-fallback lookup is written back into cache so
// later records in the same import reuse it instead of re-querying.
func resolveLocalID(kb *KnowledgeBase, cache map[string]int64, table, uuid string) (int64, bool) {
	if cache != nil {
		if id, ok := cache[uuid]; ok {
			return id, true
		}
	}
	var id int64
	if err := kb.db.QueryRow(`SELECT id FROM `+table+` WHERE uuid = ?`, uuid).Scan(&id); err != nil {
		return 0, false
	}
	if cache != nil {
		cache[uuid] = id
	}
	return id, true
}

// insertOrIgnore runs an INSERT OR IGNORE statement and reports whether it
// actually inserted a row (true) or hit an existing one and was ignored
// (false) — unlike the ON CONFLICT ... DO UPDATE statements used for
// projects/concepts, plain INSERT OR IGNORE makes RowsAffected an
// unambiguous new-vs-duplicate signal.
func insertOrIgnore(kb *KnowledgeBase, query string, args ...any) (isNew bool, err error) {
	res, err := kb.db.Exec(query, args...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
