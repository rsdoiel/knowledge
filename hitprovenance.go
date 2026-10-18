package knowledge

import (
	"database/sql"
	"strings"
	"sync/atomic"
	"time"
)

/** SourceRef is one cited work as a search or recall hit reports it (DR-0080):
 * enough to show what supports the hit and whether that support was retracted,
 * without a second query.
 *
 * Fields:
 *   ID, Title, IdentifierType, IdentifierValue, PublishedDate, Version, Rights,
 *   Retracted, RetractionNote — the source row.
 *   Relationship  (string) — how the hit's row cites it ("cited" by default).
 *   SourceVersion (string) — the version of the work that was read, "" when not recorded.
 *   SourceHash    (string) — its content hash at that time, "" when not recorded.
 *
 * Example:
 *   for _, s := range hit.Sources { fmt.Println(s.Title, s.IdentifierValue, s.Retracted) }
 */
type SourceRef struct {
	ID              int64
	Title           string
	IdentifierType  string
	IdentifierValue string
	PublishedDate   string
	Version         string
	Rights          string
	Retracted       bool
	RetractionNote  string
	Relationship    string
	SourceVersion   string
	SourceHash      string
}

/** MaterialRef is the file a document or record came from (DR-0080): where it
 * is, the checksum it had when it was read, and when. It is the other sense of
 * "source" (the material read into the knowledge base), kept apart from the
 * works a row cites.
 *
 * Fields:
 *   Path       (string)    — the file path as ingested.
 *   Checksum   (string)    — the content checksum recorded at ingest.
 *   Format     (string)    — "markdown", "fountain" or "text" for a document;
 *                            "markdown" for a record.
 *   IngestedAt (time.Time) — when it was last ingested.
 *
 * Example:
 *   if hit.Material != nil { fmt.Println(hit.Material.Path, hit.Material.Checksum) }
 */
type MaterialRef struct {
	Path       string
	Checksum   string
	Format     string
	IngestedAt time.Time
}

/** HitProvenance is what supports a hit (DR-0080): the works its row cites, the
 * file its row came from, and a flag that any cited work is retracted.
 * KBSearchResult and the recall hit embed it.
 *
 * Fields:
 *   Sources   ([]SourceRef) — the cited works; empty, never nil, when none.
 *   Material  (*MaterialRef) — the file, for a document or record; nil for
 *                              every other kind.
 *   Retracted (bool)         — true when any entry of Sources is retracted.
 *
 * Example:
 *   if hit.Retracted { fmt.Println("rests on a retracted source") }
 */
type HitProvenance struct {
	Sources   []SourceRef
	Material  *MaterialRef
	Retracted bool
}

// provTarget names one hit whose Provenance is to be filled: its kind (a
// kb_fts or ConceptMatch source type), the id of its row in that table, and
// where to put the answer.
type provTarget struct {
	sourceType string
	id         int64
	dst        *HitProvenance
}

// provenanceQueries counts the statements fillProvenance has run, so a test can
// hold a page of hits to a handful of queries rather than one per hit.
var provenanceQueryCount atomic.Int64

func (kb *KnowledgeBase) provQuery(q string, args ...any) (*sql.Rows, error) {
	provenanceQueryCount.Add(1)
	return kb.db.Query(q, args...)
}

// fillProvenance sets HitProvenance on every target in a fixed number of queries,
// whatever the number of hits. A target of a kind with no provenance (a concept,
// a project, a source) gets an empty non-nil Sources.
func (kb *KnowledgeBase) fillProvenance(targets []provTarget) error {
	var obsIDs, recIDs, secIDs []int64
	for _, t := range targets {
		t.dst.Sources = []SourceRef{}
		switch t.sourceType {
		case "observation":
			obsIDs = append(obsIDs, t.id)
		case "record":
			recIDs = append(recIDs, t.id)
		case "document_summary", "document_gist", "document_section":
			secIDs = append(secIDs, t.id)
		}
	}
	if len(obsIDs)+len(recIDs)+len(secIDs) == 0 {
		return nil
	}

	// Sections -> their documents.
	docOfSection := map[int64]int64{}
	var docIDs []int64
	if len(secIDs) > 0 {
		rows, err := kb.provQuery(`SELECT id, document_id FROM document_sections WHERE id IN (`+placeholders(len(secIDs))+`)`, int64Args(secIDs)...)
		if err != nil {
			return err
		}
		seen := map[int64]bool{}
		for rows.Next() {
			var sid, did int64
			if err := rows.Scan(&sid, &did); err != nil {
				rows.Close()
				return err
			}
			docOfSection[sid] = did
			if !seen[did] {
				seen[did] = true
				docIDs = append(docIDs, did)
			}
		}
		rows.Close()
	}

	obsLinks, err := kb.linksFor("observation_sources", "observation_id", obsIDs)
	if err != nil {
		return err
	}
	recLinks, err := kb.linksFor("record_sources", "record_id", recIDs)
	if err != nil {
		return err
	}
	docLinks, err := kb.linksFor("document_sources", "document_id", docIDs)
	if err != nil {
		return err
	}

	recMat := map[int64]*MaterialRef{}
	if len(recIDs) > 0 {
		rows, err := kb.provQuery(`SELECT id, path, checksum, ingested_at FROM records WHERE id IN (`+placeholders(len(recIDs))+`)`, int64Args(recIDs)...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			var ts string
			m := &MaterialRef{Format: "markdown"}
			if err := rows.Scan(&id, &m.Path, &m.Checksum, &ts); err != nil {
				rows.Close()
				return err
			}
			m.IngestedAt = parseTimestamp(ts)
			recMat[id] = m
		}
		rows.Close()
	}
	docMat := map[int64]*MaterialRef{}
	if len(docIDs) > 0 {
		rows, err := kb.provQuery(`SELECT id, path, checksum, format, ingested_at FROM documents WHERE id IN (`+placeholders(len(docIDs))+`)`, int64Args(docIDs)...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			var ts string
			m := &MaterialRef{}
			if err := rows.Scan(&id, &m.Path, &m.Checksum, &m.Format, &ts); err != nil {
				rows.Close()
				return err
			}
			m.IngestedAt = parseTimestamp(ts)
			docMat[id] = m
		}
		rows.Close()
	}

	for _, t := range targets {
		var links []SourceRef
		var mat *MaterialRef
		switch t.sourceType {
		case "observation":
			links = obsLinks[t.id]
		case "record":
			links, mat = recLinks[t.id], recMat[t.id]
		case "document_summary", "document_gist", "document_section":
			did := docOfSection[t.id]
			links, mat = docLinks[did], docMat[did]
		default:
			continue
		}
		if links != nil {
			t.dst.Sources = links
		}
		if mat != nil {
			c := *mat
			t.dst.Material = &c
		}
		for _, s := range t.dst.Sources {
			if s.Retracted {
				t.dst.Retracted = true
			}
		}
	}
	return nil
}

// linksFor reads the citation rows of one join table for a set of owner ids in
// one statement, grouped by owner and ordered by source id.
func (kb *KnowledgeBase) linksFor(table, ownerCol string, ids []int64) (map[int64][]SourceRef, error) {
	out := map[int64][]SourceRef{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := kb.provQuery(
		`SELECT l.`+ownerCol+`, s.id, s.title, s.identifier_type, s.identifier_value, s.published_date, s.version, s.rights,
		        s.retracted, s.retraction_note, l.relationship, l.source_version, l.source_hash
		 FROM `+table+` l JOIN sources s ON s.id = l.source_id
		 WHERE l.`+ownerCol+` IN (`+placeholders(len(ids))+`) ORDER BY l.`+ownerCol+`, s.id`, int64Args(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var owner int64
		var retracted int
		var s SourceRef
		if err := rows.Scan(&owner, &s.ID, &s.Title, &s.IdentifierType, &s.IdentifierValue, &s.PublishedDate, &s.Version,
			&s.Rights, &retracted, &s.RetractionNote, &s.Relationship, &s.SourceVersion, &s.SourceHash); err != nil {
			return nil, err
		}
		s.Retracted = retracted != 0
		out[owner] = append(out[owner], s)
	}
	return out, rows.Err()
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func int64Args(ids []int64) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}
