package knowledge

import (
	"database/sql"
	"fmt"
	"strings"
)

/** ConceptUsage counts what links to one concept, so a caller can decide
 * whether deleting it would strip real data. A concept is linked from
 * projects and observations (which have no ingest step, so nothing recreates
 * those links), and from records and document sections (which re-link on their
 * next ingest if their files still name the concept).
 *
 * Fields:
 *   Name             (string) — the concept's name.
 *   Projects         (int)    — projects it is attached to.
 *   Observations     (int)    — observations it is attached to.
 *   Records          (int)    — decision records that link to it.
 *   DocumentSections (int)    — document sections that link to it.
 *
 * Example:
 *   u, _ := kb.ConceptUsage("C++")
 *   fmt.Println(u.Total())
 */
type ConceptUsage struct {
	Name             string
	Projects         int
	Observations     int
	Records          int
	DocumentSections int
}

/** Total returns how many links the concept has, across every kind.
 *
 * Returns:
 *   int — the sum of Projects, Observations, Records and DocumentSections.
 *
 * Example:
 *   if u.Total() == 0 { fmt.Println("unused") }
 */
func (u ConceptUsage) Total() int {
	return u.Projects + u.Observations + u.Records + u.DocumentSections
}

/** ConceptInUseError is what DeleteConcept returns, without changing anything,
 * when the concept still has links and force was not given. Test for it with
 * errors.As to report the counts.
 *
 * Fields:
 *   Usage (ConceptUsage) — the links that would have been removed.
 *
 * Example:
 *   var inUse *knowledge.ConceptInUseError
 *   if errors.As(err, &inUse) { fmt.Println(inUse.Usage.Records) }
 */
type ConceptInUseError struct {
	Usage ConceptUsage
}

// Error describes the links that block the delete. It says "force" rather than
// a command-line flag, since this is library code; cmd/kb rewrites it.
func (e *ConceptInUseError) Error() string {
	var parts []string
	add := func(n int, what string) {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, what))
		}
	}
	add(e.Usage.Projects, "project(s)")
	add(e.Usage.Observations, "observation(s)")
	add(e.Usage.Records, "record(s)")
	add(e.Usage.DocumentSections, "document section(s)")
	return fmt.Sprintf("knowledge: concept %q is still linked to %s; nothing was deleted (force is needed to unlink and delete)",
		e.Usage.Name, strings.Join(parts, ", "))
}

// conceptIDByExactName returns the id of the concept with exactly this name,
// or an error if there is none. The match is exact, including case: unlike
// ResolveConceptName, which deliberately merges case variants when minting, a
// destructive verb must not guess which concept was meant.
func (kb *KnowledgeBase) conceptIDByExactName(name string) (int64, error) {
	var id int64
	err := kb.db.QueryRow(`SELECT id FROM concepts WHERE name = ?`, name).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, notFoundf("knowledge: concept %q not found", name)
	}
	if err != nil {
		return 0, fmt.Errorf("knowledge: look up concept: %w", err)
	}
	return id, nil
}

/** ConceptUsage reports what links to the named concept, without changing
 * anything. The name must match exactly, including case.
 *
 * Parameters:
 *   name (string) — the concept's name.
 *
 * Returns:
 *   ConceptUsage — the link counts.
 *   error        — if no concept has that name, or on database failure.
 *
 * Example:
 *   u, err := kb.ConceptUsage("...")
 */
func (kb *KnowledgeBase) ConceptUsage(name string) (ConceptUsage, error) {
	id, err := kb.conceptIDByExactName(name)
	if err != nil {
		return ConceptUsage{}, err
	}
	return kb.conceptUsageByID(name, id)
}

func (kb *KnowledgeBase) conceptUsageByID(name string, id int64) (ConceptUsage, error) {
	u := ConceptUsage{Name: name}
	for _, c := range []struct {
		query string
		into  *int
	}{
		{`SELECT COUNT(*) FROM project_concepts WHERE concept_id = ?`, &u.Projects},
		{`SELECT COUNT(*) FROM observation_concepts WHERE concept_id = ?`, &u.Observations},
		{`SELECT COUNT(*) FROM record_concepts WHERE concept_id = ?`, &u.Records},
		{`SELECT COUNT(*) FROM document_section_concepts WHERE concept_id = ?`, &u.DocumentSections},
	} {
		if err := kb.db.QueryRow(c.query, id).Scan(c.into); err != nil {
			return ConceptUsage{}, fmt.Errorf("knowledge: count concept links: %w", err)
		}
	}
	return u, nil
}

/** DeleteConcept removes the named concept: its row, every link to it, and its
 * full-text entry, in one transaction for the rows. The projects, observations,
 * records and document sections it was linked to are left alone; only the
 * links go. Without force, a concept that still has links is refused with a
 * *ConceptInUseError and nothing changes, so a mistyped name cannot silently
 * strip tags from real data.
 *
 * Two things this does not do, and a caller should tell the user. It does not
 * touch files: a record or document that still contains [[Name]], or lists the
 * name in tags or keywords, recreates the concept when that file is next ingested
 * after it changes (an unchanged file is skipped) or when the database is rebuilt
 * from the files.
 * And it does not propagate: a database that still holds the concept brings it
 * back on the next merge or import into this one.
 *
 * Parameters:
 *   name  (string) — the concept's name, matched exactly including case.
 *   force (bool)   — unlink the concept from everything and delete it anyway.
 *
 * Returns:
 *   ConceptUsage — the links that existed, which were removed when force was
 *                  given or there were none.
 *   error        — *ConceptInUseError when linked and !force, an error if no
 *                  concept has that name, or on database failure.
 *
 * Example:
 *   u, err := kb.DeleteConcept("...", true)
 */
func (kb *KnowledgeBase) DeleteConcept(name string, force bool) (ConceptUsage, error) {
	id, err := kb.conceptIDByExactName(name)
	if err != nil {
		return ConceptUsage{}, err
	}
	u, err := kb.conceptUsageByID(name, id)
	if err != nil {
		return ConceptUsage{}, err
	}
	if u.Total() > 0 && !force {
		return u, &ConceptInUseError{Usage: u}
	}

	tx, err := kb.db.Begin()
	if err != nil {
		return ConceptUsage{}, fmt.Errorf("knowledge: delete concept: %w", err)
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`DELETE FROM project_concepts WHERE concept_id = ?`,
		`DELETE FROM observation_concepts WHERE concept_id = ?`,
		`DELETE FROM record_concepts WHERE concept_id = ?`,
		`DELETE FROM document_section_concepts WHERE concept_id = ?`,
		`DELETE FROM concepts WHERE id = ?`,
	} {
		if _, err := tx.Exec(stmt, id); err != nil {
			return ConceptUsage{}, fmt.Errorf("knowledge: delete concept: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return ConceptUsage{}, fmt.Errorf("knowledge: delete concept: %w", err)
	}
	if kb.ftsAvailable {
		_, _ = kb.db.Exec(`DELETE FROM kb_fts WHERE source_type = 'concept' AND source_id = ?`, id)
	}
	return u, nil
}
