package knowledge

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// The removal verbs' library half (DR-0050). Each delete refuses while something
// depends on the target, says what and how many, and changes nothing; force removes
// the links and the row where that is allowed; the rows go in one transaction and
// the search entry after it, as DeleteConcept does. A target that is not there is
// ErrNotFound, never a silent success.

/** UsageCount is one line of an InUseError: what depends on the target, and how many.
 *
 * Fields:
 *   What (string) — a short noun phrase, for example "concept link(s)".
 *   N    (int)    — how many.
 *
 * Example:
 *   c := knowledge.UsageCount{What: "observation(s)", N: 3}
 */
type UsageCount struct {
	What string
	N    int
}

/** InUseError is what a delete returns, without changing anything, when the target
 * still has something depending on it and the delete was not forced (or, for a
 * project that owns content, whatever force says). It matches ErrInUse under
 * errors.Is, so a caller that only needs "refused" tests that, and errors.As reaches
 * the counts. ConceptInUseError is the older, concept-only form.
 *
 * Fields:
 *   Entity (string)      — "project", "observation", "document" or "record".
 *   Name   (string)      — the target's name or id, as a caller gave it.
 *   Counts ([]UsageCount) — what depends on it; only entries with N > 0 are shown.
 *
 * Example:
 *   var inUse *knowledge.InUseError
 *   if errors.As(err, &inUse) { fmt.Println(inUse.Counts) }
 */
type InUseError struct {
	Entity string
	Name   string
	Counts []UsageCount
}

func (e *InUseError) Error() string {
	var parts []string
	for _, c := range e.Counts {
		if c.N > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.N, c.What))
		}
	}
	return fmt.Sprintf("knowledge: %s %s is in use: %s", e.Entity, e.Name, strings.Join(parts, ", "))
}

func (e *InUseError) Is(target error) bool { return target == ErrInUse }

// isNoRows reports whether err is the driver's "no such row".
func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

// ─── projects ────────────────────────────────────────────────────────────────

/** ProjectUsage counts what depends on one project.
 *
 * Fields:
 *   Name         (string) — the project's name.
 *   Observations (int)    — observations it owns.
 *   Records      (int)    — decision records it owns.
 *   Documents    (int)    — documents it owns.
 *   Concepts     (int)    — concept links it has.
 *
 * Example:
 *   u, _ := kb.ProjectUsage("harvey")
 *   fmt.Println(u.Owned())
 */
type ProjectUsage struct {
	Name         string
	Observations int
	Records      int
	Documents    int
	Concepts     int
}

/** Owned returns how many observations, records and documents the project owns:
 * the content that DeleteProject never removes.
 *
 * Returns:
 *   int — Observations + Records + Documents.
 *
 * Example:
 *   if u.Owned() > 0 { fmt.Println("not empty") }
 */
func (u ProjectUsage) Owned() int { return u.Observations + u.Records + u.Documents }

/** Total returns Owned plus the concept links.
 *
 * Returns:
 *   int — everything that depends on the project.
 *
 * Example:
 *   fmt.Println(u.Total())
 */
func (u ProjectUsage) Total() int { return u.Owned() + u.Concepts }

func (kb *KnowledgeBase) projectIDByName(name string) (int64, error) {
	p, err := kb.ProjectByName(name)
	if err != nil {
		return 0, err
	}
	if p == nil {
		return 0, notFoundf("knowledge: project %q not found", name)
	}
	return p.ID, nil
}

func (kb *KnowledgeBase) countWhere(query string, args ...any) (int, error) {
	var n int
	if err := kb.db.QueryRow(query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

/** ProjectUsage reports what depends on the named project, without changing anything.
 *
 * Parameters:
 *   name (string) — the project's name, matched exactly.
 *
 * Returns:
 *   ProjectUsage — the counts.
 *   error        — ErrNotFound if there is no such project, or on database failure.
 *
 * Example:
 *   u, err := kb.ProjectUsage("stray")
 */
func (kb *KnowledgeBase) ProjectUsage(name string) (ProjectUsage, error) {
	id, err := kb.projectIDByName(name)
	if err != nil {
		return ProjectUsage{}, err
	}
	u := ProjectUsage{Name: name}
	for _, q := range []struct {
		dst *int
		sql string
	}{
		{&u.Observations, `SELECT COUNT(*) FROM observations WHERE project_id = ?`},
		{&u.Records, `SELECT COUNT(*) FROM records WHERE project_id = ?`},
		{&u.Documents, `SELECT COUNT(*) FROM documents WHERE project_id = ?`},
		{&u.Concepts, `SELECT COUNT(*) FROM project_concepts WHERE project_id = ?`},
	} {
		if *q.dst, err = kb.countWhere(q.sql, id); err != nil {
			return ProjectUsage{}, fmt.Errorf("knowledge: project usage: %w", err)
		}
	}
	return u, nil
}

/** DeleteProject removes a project. A project that owns any observation, record or
 * document is always refused, with force or without: one call must never destroy
 * work, so the content has to be deleted or moved first. A project with only concept
 * links is refused unless force is true, which removes those links and the project;
 * the concepts themselves stay. It exists to remove a stray empty project.
 *
 * Parameters:
 *   name  (string) — the project's name, matched exactly.
 *   force (bool)   — remove concept links too.
 *
 * Returns:
 *   ProjectUsage — what depended on it (removed, or, on refusal, what blocked it).
 *   error        — ErrNotFound if there is no such project; an *InUseError (matching
 *                  ErrInUse), with nothing changed, if it is not deletable.
 *
 * Example:
 *   u, err := kb.DeleteProject("stray", false)
 */
func (kb *KnowledgeBase) DeleteProject(name string, force bool) (ProjectUsage, error) {
	id, err := kb.projectIDByName(name)
	if err != nil {
		return ProjectUsage{}, err
	}
	u, err := kb.ProjectUsage(name)
	if err != nil {
		return ProjectUsage{}, err
	}
	if u.Owned() > 0 {
		return u, &InUseError{Entity: "project", Name: fmt.Sprintf("%q", name), Counts: []UsageCount{
			{"observation(s)", u.Observations}, {"record(s)", u.Records}, {"document(s)", u.Documents}}}
	}
	if u.Concepts > 0 && !force {
		return u, &InUseError{Entity: "project", Name: fmt.Sprintf("%q", name), Counts: []UsageCount{{"concept link(s)", u.Concepts}}}
	}
	if err := kb.deleteRows("delete project", id, "project",
		`DELETE FROM project_concepts WHERE project_id = ?`,
		`DELETE FROM projects WHERE id = ?`); err != nil {
		return ProjectUsage{}, err
	}
	return u, nil
}

// deleteRows runs statements, each taking the one id, in one transaction, then removes
// the search entries of the given source type for that id.
func (kb *KnowledgeBase) deleteRows(what string, id int64, ftsType string, stmts ...string) error {
	tx, err := kb.db.Begin()
	if err != nil {
		return fmt.Errorf("knowledge: %s: %w", what, err)
	}
	defer tx.Rollback()
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt, id); err != nil {
			return fmt.Errorf("knowledge: %s: %w", what, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("knowledge: %s: %w", what, err)
	}
	if kb.ftsAvailable && ftsType != "" {
		_, _ = kb.db.Exec(`DELETE FROM kb_fts WHERE source_type = ? AND source_id = ?`, ftsType, id)
	}
	return nil
}

// ─── observations ────────────────────────────────────────────────────────────

/** ObservationUsage counts what depends on one observation.
 *
 * Fields:
 *   ID            (int64) — the observation's id.
 *   Concepts      (int)   — concept links.
 *   Sources       (int)   — source links.
 *   RelationsFrom (int)   — relations where it is the newer side (it supersedes another).
 *   RelationsTo   (int)   — relations where it is the older side (another supersedes it).
 *
 * Example:
 *   u, _ := kb.ObservationUsage(7)
 *   fmt.Println(u.Total())
 */
type ObservationUsage struct {
	ID            int64
	Concepts      int
	Sources       int
	RelationsFrom int
	RelationsTo   int
}

/** Total returns how many links and relations the observation has.
 *
 * Returns:
 *   int — the sum of all four counts.
 *
 * Example:
 *   if u.Total() == 0 { fmt.Println("unlinked") }
 */
func (u ObservationUsage) Total() int {
	return u.Concepts + u.Sources + u.RelationsFrom + u.RelationsTo
}

/** ObservationUsage reports what depends on the observation, without changing anything.
 *
 * Parameters:
 *   id (int64) — the observation's id.
 *
 * Returns:
 *   ObservationUsage — the counts.
 *   error            — ErrNotFound if there is no such observation, or on database failure.
 *
 * Example:
 *   u, err := kb.ObservationUsage(7)
 */
func (kb *KnowledgeBase) ObservationUsage(id int64) (ObservationUsage, error) {
	if err := kb.requireRow("observations", id, "observation"); err != nil {
		return ObservationUsage{}, err
	}
	u := ObservationUsage{ID: id}
	var err error
	for _, q := range []struct {
		dst *int
		sql string
	}{
		{&u.Concepts, `SELECT COUNT(*) FROM observation_concepts WHERE observation_id = ?`},
		{&u.Sources, `SELECT COUNT(*) FROM observation_sources WHERE observation_id = ?`},
		{&u.RelationsFrom, `SELECT COUNT(*) FROM observation_relations WHERE from_id = ?`},
		{&u.RelationsTo, `SELECT COUNT(*) FROM observation_relations WHERE to_id = ?`},
	} {
		if *q.dst, err = kb.countWhere(q.sql, id); err != nil {
			return ObservationUsage{}, fmt.Errorf("knowledge: observation usage: %w", err)
		}
	}
	return u, nil
}

/** DeleteObservation removes an observation. One with concept links, source links or
 * supersession relations (either direction) is refused unless force is true, which
 * removes those links and relations and the observation; the concepts, sources and the
 * other observations stay.
 *
 * Parameters:
 *   id    (int64) — the observation's id.
 *   force (bool)  — remove links and relations too.
 *
 * Returns:
 *   ObservationUsage — what depended on it.
 *   error            — ErrNotFound if there is no such observation; an *InUseError
 *                      (matching ErrInUse), with nothing changed, if it is linked and
 *                      force is false.
 *
 * Example:
 *   u, err := kb.DeleteObservation(7, true)
 */
func (kb *KnowledgeBase) DeleteObservation(id int64, force bool) (ObservationUsage, error) {
	u, err := kb.ObservationUsage(id)
	if err != nil {
		return ObservationUsage{}, err
	}
	if u.Total() > 0 && !force {
		return u, &InUseError{Entity: "observation", Name: fmt.Sprint(id), Counts: []UsageCount{
			{"concept link(s)", u.Concepts}, {"source link(s)", u.Sources},
			{"relation(s) it supersedes", u.RelationsFrom}, {"relation(s) that supersede it", u.RelationsTo}}}
	}
	tx, err := kb.db.Begin()
	if err != nil {
		return ObservationUsage{}, fmt.Errorf("knowledge: delete observation: %w", err)
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`DELETE FROM observation_concepts WHERE observation_id = ?`,
		`DELETE FROM observation_sources WHERE observation_id = ?`,
		`DELETE FROM observation_relations WHERE from_id = ? OR to_id = ?`,
		`DELETE FROM observations WHERE id = ?`,
	} {
		args := []any{id}
		if strings.Contains(stmt, "from_id = ? OR to_id = ?") {
			args = []any{id, id}
		}
		if _, err := tx.Exec(stmt, args...); err != nil {
			return ObservationUsage{}, fmt.Errorf("knowledge: delete observation: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return ObservationUsage{}, fmt.Errorf("knowledge: delete observation: %w", err)
	}
	if kb.ftsAvailable {
		_, _ = kb.db.Exec(`DELETE FROM kb_fts WHERE source_type = 'observation' AND source_id = ?`, id)
	}
	return u, nil
}

// ─── documents ───────────────────────────────────────────────────────────────

/** DocumentUsage counts what a document holds.
 *
 * Fields:
 *   ID               (int64)  — the document's id.
 *   Path             (string) — the path it was ingested from.
 *   Sections         (int)    — sections, the gist row included.
 *   ReviewedSections (int)    — sections whose summary a human has reviewed.
 *   ConceptLinks     (int)    — section-to-concept links.
 *
 * Example:
 *   u, _ := kb.DocumentUsage(1)
 *   fmt.Println(u.ReviewedSections)
 */
type DocumentUsage struct {
	ID               int64
	Path             string
	Sections         int
	ReviewedSections int
	ConceptLinks     int
}

/** DocumentUsage reports what the document holds, without changing anything.
 *
 * Parameters:
 *   id (int64) — the document's id.
 *
 * Returns:
 *   DocumentUsage — the counts.
 *   error         — ErrNotFound if there is no such document, or on database failure.
 *
 * Example:
 *   u, err := kb.DocumentUsage(1)
 */
func (kb *KnowledgeBase) DocumentUsage(id int64) (DocumentUsage, error) {
	u := DocumentUsage{ID: id}
	err := kb.db.QueryRow(`SELECT path FROM documents WHERE id = ?`, id).Scan(&u.Path)
	if err != nil {
		if isNoRows(err) {
			return DocumentUsage{}, notFoundf("knowledge: no document with id %d", id)
		}
		return DocumentUsage{}, err
	}
	for _, q := range []struct {
		dst *int
		sql string
	}{
		{&u.Sections, `SELECT COUNT(*) FROM document_sections WHERE document_id = ?`},
		{&u.ReviewedSections, `SELECT COUNT(*) FROM document_sections WHERE document_id = ? AND summary_status = 'reviewed'`},
		{&u.ConceptLinks, `SELECT COUNT(*) FROM document_section_concepts WHERE section_id IN (SELECT id FROM document_sections WHERE document_id = ?)`},
	} {
		if *q.dst, err = kb.countWhere(q.sql, id); err != nil {
			return DocumentUsage{}, fmt.Errorf("knowledge: document usage: %w", err)
		}
	}
	return u, nil
}

/** DeleteDocument removes a document, its sections and their concept links, and the
 * search entries of any promoted summaries. A document with a section whose summary is
 * reviewed is refused unless force is true: a reviewed summary is human-gated data and
 * deleting it loses it. Ingesting the file again recreates the document, without the
 * reviewed summaries.
 *
 * Parameters:
 *   id    (int64) — the document's id.
 *   force (bool)  — delete even though a reviewed summary is lost.
 *
 * Returns:
 *   DocumentUsage — what the document held.
 *   error         — ErrNotFound if there is no such document; an *InUseError (matching
 *                   ErrInUse), with nothing changed, if a reviewed summary blocks it.
 *
 * Example:
 *   u, err := kb.DeleteDocument(1, false)
 */
func (kb *KnowledgeBase) DeleteDocument(id int64, force bool) (DocumentUsage, error) {
	u, err := kb.DocumentUsage(id)
	if err != nil {
		return DocumentUsage{}, err
	}
	if u.ReviewedSections > 0 && !force {
		return u, &InUseError{Entity: "document", Name: fmt.Sprint(id), Counts: []UsageCount{{"reviewed summary(ies)", u.ReviewedSections}}}
	}
	var sectionIDs []int64
	rows, err := kb.db.Query(`SELECT id FROM document_sections WHERE document_id = ?`, id)
	if err != nil {
		return DocumentUsage{}, fmt.Errorf("knowledge: delete document: %w", err)
	}
	for rows.Next() {
		var sid int64
		if err := rows.Scan(&sid); err != nil {
			rows.Close()
			return DocumentUsage{}, fmt.Errorf("knowledge: delete document: %w", err)
		}
		sectionIDs = append(sectionIDs, sid)
	}
	rows.Close()

	tx, err := kb.db.Begin()
	if err != nil {
		return DocumentUsage{}, fmt.Errorf("knowledge: delete document: %w", err)
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`DELETE FROM document_section_concepts WHERE section_id IN (SELECT id FROM document_sections WHERE document_id = ?)`,
		`DELETE FROM document_sections WHERE document_id = ?`,
		`DELETE FROM documents WHERE id = ?`,
	} {
		if _, err := tx.Exec(stmt, id); err != nil {
			return DocumentUsage{}, fmt.Errorf("knowledge: delete document: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return DocumentUsage{}, fmt.Errorf("knowledge: delete document: %w", err)
	}
	if kb.ftsAvailable {
		for _, sid := range sectionIDs {
			_, _ = kb.db.Exec(`DELETE FROM kb_fts WHERE source_type = 'document_summary' AND source_id = ?`, sid)
		}
	}
	return u, nil
}

// ─── records ─────────────────────────────────────────────────────────────────

/** RecordUsage counts what a record row is linked to.
 *
 * Fields:
 *   ID            (int64)  — the row's id.
 *   RecordID      (string) — the record's id within its tier, for example "0040".
 *   Path          (string) — the path stored for its file, relative to the workspace root.
 *   Concepts      (int)    — concept links.
 *   RelationsFrom (int)    — relations where it is the source.
 *   RelationsTo   (int)    — relations where it is the target.
 *
 * Example:
 *   u, _ := kb.RecordUsage(12)
 *   fmt.Println(u.Path)
 */
type RecordUsage struct {
	ID            int64
	RecordID      string
	Path          string
	Concepts      int
	RelationsFrom int
	RelationsTo   int
}

/** RecordUsage reports what a record row is linked to, without changing anything.
 *
 * Parameters:
 *   id (int64) — the record row's id.
 *
 * Returns:
 *   RecordUsage — the counts and the stored path.
 *   error       — ErrNotFound if there is no such record, or on database failure.
 *
 * Example:
 *   u, err := kb.RecordUsage(12)
 */
func (kb *KnowledgeBase) RecordUsage(id int64) (RecordUsage, error) {
	u := RecordUsage{ID: id}
	err := kb.db.QueryRow(`SELECT record_id, path FROM records WHERE id = ?`, id).Scan(&u.RecordID, &u.Path)
	if err != nil {
		if isNoRows(err) {
			return RecordUsage{}, notFoundf("knowledge: no record with id %d", id)
		}
		return RecordUsage{}, err
	}
	for _, q := range []struct {
		dst *int
		sql string
	}{
		{&u.Concepts, `SELECT COUNT(*) FROM record_concepts WHERE record_id = ?`},
		{&u.RelationsFrom, `SELECT COUNT(*) FROM record_relations WHERE from_id = ?`},
		{&u.RelationsTo, `SELECT COUNT(*) FROM record_relations WHERE to_id = ?`},
	} {
		if *q.dst, err = kb.countWhere(q.sql, id); err != nil {
			return RecordUsage{}, fmt.Errorf("knowledge: record usage: %w", err)
		}
	}
	return u, nil
}

/** DeleteRecord removes a record's row, its relations in both directions, its concept
 * links and its search entry. It is for a row whose file has gone: the library does no
 * file I/O, so it cannot check that, and the caller must (kb's record delete does). A
 * row deleted while its file still exists comes back the next time the file changes and
 * is ingested. The relations of other records that pointed at this one go with it.
 *
 * Parameters:
 *   id (int64) — the record row's id.
 *
 * Returns:
 *   RecordUsage — what the row was linked to.
 *   error       — ErrNotFound if there is no such record, or on database failure.
 *
 * Example:
 *   u, err := kb.DeleteRecord(12)
 */
func (kb *KnowledgeBase) DeleteRecord(id int64) (RecordUsage, error) {
	u, err := kb.RecordUsage(id)
	if err != nil {
		return RecordUsage{}, err
	}
	tx, err := kb.db.Begin()
	if err != nil {
		return RecordUsage{}, fmt.Errorf("knowledge: delete record: %w", err)
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`DELETE FROM record_concepts WHERE record_id = ?`,
		`DELETE FROM record_relations WHERE from_id = ? OR to_id = ?`,
		`DELETE FROM records WHERE id = ?`,
	} {
		args := []any{id}
		if strings.Contains(stmt, "from_id = ? OR to_id = ?") {
			args = []any{id, id}
		}
		if _, err := tx.Exec(stmt, args...); err != nil {
			return RecordUsage{}, fmt.Errorf("knowledge: delete record: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return RecordUsage{}, fmt.Errorf("knowledge: delete record: %w", err)
	}
	if kb.ftsAvailable {
		_, _ = kb.db.Exec(`DELETE FROM kb_fts WHERE source_type = 'record' AND source_id = ?`, id)
	}
	return u, nil
}

// ─── unlinking ───────────────────────────────────────────────────────────────

func (kb *KnowledgeBase) unlink(query string, missing func() error, args ...any) error {
	res, err := kb.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("knowledge: unlink: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return missing()
	}
	return nil
}

/** UnlinkProjectConcept removes the link between a project and a concept. Both names
 * match exactly, as for DeleteConcept: a destructive call must not guess. Removing a
 * link that does not exist is ErrNotFound, not a success.
 *
 * Parameters:
 *   project (string) — the project's name.
 *   concept (string) — the concept's name.
 *
 * Returns:
 *   error — ErrNotFound for a missing project, concept or link; or on database failure.
 *
 * Example:
 *   err := kb.UnlinkProjectConcept("harvey", "RAG")
 */
func (kb *KnowledgeBase) UnlinkProjectConcept(project, concept string) error {
	pid, err := kb.projectIDByName(project)
	if err != nil {
		return err
	}
	cid, err := kb.conceptIDByExactName(concept)
	if err != nil {
		return err
	}
	return kb.unlink(`DELETE FROM project_concepts WHERE project_id = ? AND concept_id = ?`,
		func() error { return notFoundf("knowledge: project %q is not linked to concept %q", project, concept) }, pid, cid)
}

/** UnlinkObservationConcept removes the link between an observation and a concept. The
 * concept name matches exactly. Removing a link that does not exist is ErrNotFound.
 *
 * Parameters:
 *   observationID (int64)  — the observation's id.
 *   concept       (string) — the concept's name.
 *
 * Returns:
 *   error — ErrNotFound for a missing observation, concept or link; or on database failure.
 *
 * Example:
 *   err := kb.UnlinkObservationConcept(7, "RAG")
 */
func (kb *KnowledgeBase) UnlinkObservationConcept(observationID int64, concept string) error {
	if err := kb.requireRow("observations", observationID, "observation"); err != nil {
		return err
	}
	cid, err := kb.conceptIDByExactName(concept)
	if err != nil {
		return err
	}
	return kb.unlink(`DELETE FROM observation_concepts WHERE observation_id = ? AND concept_id = ?`,
		func() error {
			return notFoundf("knowledge: observation %d is not linked to concept %q", observationID, concept)
		}, observationID, cid)
}

/** UnlinkObservationSource removes the link between an observation and a source.
 * Removing a link that does not exist is ErrNotFound.
 *
 * Parameters:
 *   observationID (int64) — the observation's id.
 *   sourceID      (int64) — the source's id.
 *
 * Returns:
 *   error — ErrNotFound for a missing observation, source or link; or on database failure.
 *
 * Example:
 *   err := kb.UnlinkObservationSource(7, 2)
 */
func (kb *KnowledgeBase) UnlinkObservationSource(observationID, sourceID int64) error {
	if err := kb.requireRow("observations", observationID, "observation"); err != nil {
		return err
	}
	if err := kb.requireRow("sources", sourceID, "source"); err != nil {
		return err
	}
	return kb.unlink(`DELETE FROM observation_sources WHERE observation_id = ? AND source_id = ?`,
		func() error {
			return notFoundf("knowledge: observation %d is not linked to source %d", observationID, sourceID)
		}, observationID, sourceID)
}
