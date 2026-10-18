package knowledge

import (
	"fmt"
	"regexp"
	"strings"
)

// documentSourcesSchema and recordSourcesSchema are the citation join tables
// of DR-0081: shaped like observation_sources, with the version and hash of the
// cited work at the time it was read. A document or record deletes its links
// with it; a source cited by any of them cannot be deleted.
const documentSourcesSchema = `
CREATE TABLE IF NOT EXISTS document_sources (
    document_id    INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    source_id      INTEGER NOT NULL REFERENCES sources(id)   ON DELETE RESTRICT,
    relationship   TEXT    NOT NULL DEFAULT 'cited',
    source_version TEXT    NOT NULL DEFAULT '',
    source_hash    TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (document_id, source_id)
);
`

const recordSourcesSchema = `
CREATE TABLE IF NOT EXISTS record_sources (
    record_id      INTEGER NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    source_id      INTEGER NOT NULL REFERENCES sources(id) ON DELETE RESTRICT,
    relationship   TEXT    NOT NULL DEFAULT 'cited',
    source_version TEXT    NOT NULL DEFAULT '',
    source_hash    TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (record_id, source_id)
);
`

/** SourceLink is one citation: a source, and how and at which version a
 * observation, document or record cites it (DR-0081).
 *
 * Fields:
 *   Source        (Source) — the cited work, with its retraction state.
 *   Relationship  (string) — "cited" unless the link says otherwise.
 *   SourceVersion (string) — the version of the work that was read; "" when not recorded.
 *   SourceHash    (string) — its content hash at that time; "" when not recorded.
 *
 * Example:
 *   links, _ := kb.DocumentSources(docID)
 *   for _, l := range links { fmt.Println(l.Source.Title, l.Relationship, l.SourceVersion) }
 */
type SourceLink struct {
	Source        Source
	Relationship  string
	SourceVersion string
	SourceHash    string
}

/** SourceCitations counts what cites one source (DR-0081): it is what stops
 * RemoveSource, so a caller can say so before asking for a delete.
 *
 * Example:
 *   c, _ := kb.SourceCitations(1)
 *   fmt.Println(c.Total()) // 3
 */
type SourceCitations struct {
	Observations int
	Documents    int
	Records      int
}

/** Total returns how many rows cite the source, of any kind.
 *
 * Returns:
 *   int — Observations + Documents + Records.
 *
 * Example:
 *   if c.Total() > 0 { fmt.Println("cited") }
 */
func (c SourceCitations) Total() int { return c.Observations + c.Documents + c.Records }

/** String describes the citations for a message: "2 observation(s), 1 document(s)",
 * leaving out the kinds with none.
 *
 * Returns:
 *   string — the description; "" when nothing cites the source.
 *
 * Example:
 *   fmt.Println(SourceCitations{Documents: 1}) // "1 document(s)"
 */
func (c SourceCitations) String() string {
	var parts []string
	if c.Observations > 0 {
		parts = append(parts, fmt.Sprintf("%d observation(s)", c.Observations))
	}
	if c.Documents > 0 {
		parts = append(parts, fmt.Sprintf("%d document(s)", c.Documents))
	}
	if c.Records > 0 {
		parts = append(parts, fmt.Sprintf("%d record(s)", c.Records))
	}
	return strings.Join(parts, ", ")
}

// linkSource is the shared body of the three Link*Source calls.
func (kb *KnowledgeBase) linkSource(table, ownerCol, ownerTable, ownerWhat string, ownerID, sourceID int64, relationship, version, hash string) error {
	if err := kb.requireRow(ownerTable, ownerID, ownerWhat); err != nil {
		return err
	}
	if err := kb.requireRow("sources", sourceID, "source"); err != nil {
		return err
	}
	if strings.TrimSpace(relationship) == "" {
		relationship = "cited"
	}
	_, err := kb.db.Exec(
		`INSERT OR IGNORE INTO `+table+` (`+ownerCol+`, source_id, relationship, source_version, source_hash)
		 VALUES (?, ?, ?, ?, ?)`,
		ownerID, sourceID, relationship, version, hash,
	)
	return err
}

/** LinkObservationSourceAt is LinkObservationSource with the version and content
 * hash of the cited work at the time it was read (DR-0081 item 2). A second link
 * of the same pair is ignored, the first write staying.
 *
 * Parameters:
 *   observationID (int64)  — observation primary key.
 *   sourceID      (int64)  — source primary key.
 *   relationship  (string) — label; "" means "cited".
 *   version       (string) — version of the work read; "" when not recorded.
 *   hash          (string) — its content hash; "" when not recorded.
 *
 * Returns:
 *   error — ErrNotFound for an observation or source that does not exist, or on database failure.
 *
 * Example:
 *   err := kb.LinkObservationSourceAt(42, 1, "cited", "2nd ed", "sha256:ab12")
 */
func (kb *KnowledgeBase) LinkObservationSourceAt(observationID, sourceID int64, relationship, version, hash string) error {
	return kb.linkSource("observation_sources", "observation_id", "observations", "observation", observationID, sourceID, relationship, version, hash)
}

/** LinkDocumentSource records that a document cites a source (DR-0081 item 3).
 * A second link of the same pair is ignored, the first write staying.
 *
 * Parameters:
 *   documentID   (int64)  — document primary key.
 *   sourceID     (int64)  — source primary key.
 *   relationship (string) — label; "" means "cited".
 *   version      (string) — version of the work read; "" when not recorded.
 *   hash         (string) — its content hash; "" when not recorded.
 *
 * Returns:
 *   error — ErrNotFound for a document or source that does not exist, or on database failure.
 *
 * Example:
 *   err := kb.LinkDocumentSource(7, 1, "cited", "", "")
 */
func (kb *KnowledgeBase) LinkDocumentSource(documentID, sourceID int64, relationship, version, hash string) error {
	return kb.linkSource("document_sources", "document_id", "documents", "document", documentID, sourceID, relationship, version, hash)
}

/** LinkRecordSource records that a decision record cites a source (DR-0081
 * item 3), as LinkDocumentSource does for a document.
 *
 * Parameters:
 *   recordID     (int64)  — record primary key.
 *   sourceID     (int64)  — source primary key.
 *   relationship (string) — label; "" means "cited".
 *   version      (string) — version of the work read; "" when not recorded.
 *   hash         (string) — its content hash; "" when not recorded.
 *
 * Returns:
 *   error — ErrNotFound for a record or source that does not exist, or on database failure.
 *
 * Example:
 *   err := kb.LinkRecordSource(3, 1, "cited", "", "")
 */
func (kb *KnowledgeBase) LinkRecordSource(recordID, sourceID int64, relationship, version, hash string) error {
	return kb.linkSource("record_sources", "record_id", "records", "record", recordID, sourceID, relationship, version, hash)
}

// sourceLinks lists the links of one owner row, ordered by source id.
func (kb *KnowledgeBase) sourceLinks(table, ownerCol string, ownerID int64) ([]SourceLink, error) {
	rows, err := kb.db.Query(
		`SELECT s.id, s.title, s.identifier_type, s.identifier_value, s.authors, s.published_date, s.publisher,
		        s.rights, s.version, s.retracted, s.retraction_note, l.relationship, l.source_version, l.source_hash
		 FROM sources s JOIN `+table+` l ON l.source_id = s.id
		 WHERE l.`+ownerCol+` = ? ORDER BY s.id`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SourceLink{}
	for rows.Next() {
		var l SourceLink
		var retracted int
		if err := rows.Scan(&l.Source.ID, &l.Source.Title, &l.Source.IdentifierType, &l.Source.IdentifierValue,
			&l.Source.Authors, &l.Source.PublishedDate, &l.Source.Publisher, &l.Source.Rights, &l.Source.Version,
			&retracted, &l.Source.RetractionNote, &l.Relationship, &l.SourceVersion, &l.SourceHash); err != nil {
			return nil, err
		}
		l.Source.Retracted = retracted != 0
		out = append(out, l)
	}
	return out, rows.Err()
}

/** ObservationSourceLinks lists the sources an observation cites with the link's
 * relationship, version and hash, ordered by source id. ObservationSources is
 * the older call without the link columns.
 *
 * Parameters:
 *   observationID (int64) — observation primary key.
 *
 * Returns:
 *   []SourceLink — empty (not nil) when none.
 *   error        — on database failure.
 *
 * Example:
 *   links, err := kb.ObservationSourceLinks(42)
 */
func (kb *KnowledgeBase) ObservationSourceLinks(observationID int64) ([]SourceLink, error) {
	return kb.sourceLinks("observation_sources", "observation_id", observationID)
}

/** DocumentSources lists the sources a document cites, ordered by source id.
 *
 * Parameters:
 *   documentID (int64) — document primary key.
 *
 * Returns:
 *   []SourceLink — empty (not nil) when none.
 *   error        — on database failure.
 *
 * Example:
 *   links, err := kb.DocumentSources(7)
 */
func (kb *KnowledgeBase) DocumentSources(documentID int64) ([]SourceLink, error) {
	return kb.sourceLinks("document_sources", "document_id", documentID)
}

/** RecordSources lists the sources a decision record cites, ordered by source id.
 *
 * Parameters:
 *   recordID (int64) — record primary key.
 *
 * Returns:
 *   []SourceLink — empty (not nil) when none.
 *   error        — on database failure.
 *
 * Example:
 *   links, err := kb.RecordSources(3)
 */
func (kb *KnowledgeBase) RecordSources(recordID int64) ([]SourceLink, error) {
	return kb.sourceLinks("record_sources", "record_id", recordID)
}

/** SourceCitations reports what cites a source, without changing anything.
 *
 * Parameters:
 *   id (int64) — source primary key.
 *
 * Returns:
 *   SourceCitations — the counts per kind.
 *   error           — on database failure.
 *
 * Example:
 *   c, err := kb.SourceCitations(1)
 */
func (kb *KnowledgeBase) SourceCitations(id int64) (SourceCitations, error) {
	var c SourceCitations
	for _, q := range []struct {
		dst *int
		sql string
	}{
		{&c.Observations, `SELECT COUNT(*) FROM observation_sources WHERE source_id = ?`},
		{&c.Documents, `SELECT COUNT(*) FROM document_sources WHERE source_id = ?`},
		{&c.Records, `SELECT COUNT(*) FROM record_sources WHERE source_id = ?`},
	} {
		if err := kb.db.QueryRow(q.sql, id).Scan(q.dst); err != nil {
			return SourceCitations{}, err
		}
	}
	return c, nil
}

// doiPrefixPattern matches the pasted forms of a DOI that name the same thing
// as the bare 10.NNNN/suffix: "doi:" and the doi.org resolver URLs.
var doiPrefixPattern = regexp.MustCompile(`(?i)^(?:doi:\s*|https?://(?:dx\.)?doi\.org/)`)

// normalizeDOI strips a pasted doi: or doi.org prefix. AddObservationWithSource
// accepts a DOI as pasted, as its documentation always showed; AddSource then
// holds the bare form to the shape kb source add holds it to.
func normalizeDOI(s string) string {
	s = strings.TrimSpace(s)
	return doiPrefixPattern.ReplaceAllString(s, "")
}
