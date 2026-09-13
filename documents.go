package knowledge

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rsdoiel/fountain"
	"gopkg.in/yaml.v3"
)

// documentsSchema creates the narrative/article document tables. Applied
// after recordsSchema; CREATE TABLE IF NOT EXISTS is idempotent, following
// the same lazy-migration pattern as the rest of the schema. Unlike
// projects/observations/concepts/sources, there is no pre-existing data to
// backfill a uuid onto -- both tables are new, so AddDocument/
// AddDocumentSection stamp a uuid at insert time the same way AddRecord
// does, and the unique index is created directly here rather than through
// the generic backfillUUIDs path.
//
// See narrative-documents-design.md for the full rationale. summary_stale
// (design decision 10) is set when a section's body changes under an
// existing drafted/reviewed summary on re-ingest, so review work is never
// silently discarded.
const documentsSchema = `
CREATE TABLE IF NOT EXISTS documents (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id     INTEGER REFERENCES projects(id) ON DELETE SET NULL,
    title          TEXT NOT NULL,
    format         TEXT NOT NULL DEFAULT '',
    path           TEXT NOT NULL,
    author         TEXT NOT NULL DEFAULT '',
    published_date TEXT NOT NULL DEFAULT '',
    checksum       TEXT NOT NULL DEFAULT '',
    ingested_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    uuid           TEXT NOT NULL DEFAULT '',
    origin_host    TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_uuid ON documents(uuid);

CREATE TABLE IF NOT EXISTS document_sections (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id    INTEGER REFERENCES documents(id) ON DELETE CASCADE,
    level          TEXT NOT NULL,
    seq            INTEGER NOT NULL DEFAULT 0,
    heading        TEXT NOT NULL DEFAULT '',
    body           TEXT NOT NULL DEFAULT '',
    summary_body   TEXT NOT NULL DEFAULT '',
    summary_status TEXT NOT NULL DEFAULT 'unsummarized',
    summary_stale  INTEGER NOT NULL DEFAULT 0,
    source_size    INTEGER NOT NULL DEFAULT 0,
    tag_density    INTEGER NOT NULL DEFAULT 0,
    confidence     REAL,
    generated_by   TEXT NOT NULL DEFAULT '',
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    uuid           TEXT NOT NULL DEFAULT '',
    origin_host    TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_document_sections_uuid ON document_sections(uuid);
`

/** Document is a narrative or article ingested at the graduated-abstraction
 * levels described in narrative-documents-design.md: a documents row plus a
 * DocumentSection per gist/section.
 */
type Document struct {
	ID            int64
	ProjectID     int64
	Title         string
	Format        string
	Path          string
	Author        string
	PublishedDate string
	Checksum      string
	IngestedAt    time.Time
	UUID          string
	OriginHost    string
}

/** DocumentSection is one row of a document's decomposition: either the
 * document-level gist (Level == "gist", Body always empty) or one
 * structural unit (Level == "section", Body populated at ingest). Summary*
 * and Confidence describe the deferred, reviewable layer described in
 * narrative-documents-design.md decision 7; SummaryStale is set when Body
 * changes under an existing summary on re-ingest (decision 10).
 */
type DocumentSection struct {
	ID            int64
	DocumentID    int64
	Level         string
	Seq           int
	Heading       string
	Body          string
	SummaryBody   string
	SummaryStatus string
	SummaryStale  bool
	SourceSize    int
	TagDensity    int
	Confidence    *float64
	GeneratedBy   string
	CreatedAt     time.Time
	UUID          string
	OriginHost    string
}

/** AddDocument inserts a new document row.
 *
 * Parameters:
 *   d (Document) — fields to store; ID/UUID/OriginHost/IngestedAt are set
 *                  by this call and any given value is ignored.
 *
 * Returns:
 *   int64 — ID of the new document.
 *   error — on database failure.
 *
 * Example:
 *   id, err := kb.AddDocument(Document{ProjectID: pid, Title: "A Story",
 *       Format: "markdown", Path: "stories/a-story.md"})
 */
func (kb *KnowledgeBase) AddDocument(d Document) (int64, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return 0, fmt.Errorf("knowledge: generate uuid: %w", err)
	}
	host, _ := os.Hostname()
	res, err := kb.db.Exec(
		`INSERT INTO documents (project_id, title, format, path, author, published_date, checksum, uuid, origin_host)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		projectValue(d.ProjectID), d.Title, d.Format, d.Path, d.Author, d.PublishedDate, d.Checksum, u.String(), host,
	)
	if err != nil {
		return 0, fmt.Errorf("knowledge: add document: %w", err)
	}
	return res.LastInsertId()
}

/** AddDocumentSection inserts a new document_sections row.
 *
 * Parameters:
 *   s (DocumentSection) — fields to store; ID/UUID/OriginHost/CreatedAt are
 *                         set by this call and any given value is ignored.
 *
 * Returns:
 *   int64 — ID of the new section.
 *   error — on database failure.
 *
 * Example:
 *   id, err := kb.AddDocumentSection(DocumentSection{DocumentID: docID,
 *       Level: "section", Seq: 0, Heading: "Chapter One", Body: "..."})
 */
func (kb *KnowledgeBase) AddDocumentSection(s DocumentSection) (int64, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return 0, fmt.Errorf("knowledge: generate uuid: %w", err)
	}
	host, _ := os.Hostname()
	if s.SummaryStatus == "" {
		s.SummaryStatus = "unsummarized"
	}
	res, err := kb.db.Exec(
		`INSERT INTO document_sections
		    (document_id, level, seq, heading, body, summary_body, summary_status,
		     summary_stale, source_size, tag_density, confidence, generated_by, uuid, origin_host)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.DocumentID, s.Level, s.Seq, s.Heading, s.Body, s.SummaryBody, s.SummaryStatus,
		boolToInt(s.SummaryStale), s.SourceSize, s.TagDensity, s.Confidence, s.GeneratedBy, u.String(), host,
	)
	if err != nil {
		return 0, fmt.Errorf("knowledge: add document section: %w", err)
	}
	return res.LastInsertId()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

/** DocumentSections returns every section of a document, gist first, then
 * by Seq.
 *
 * Parameters:
 *   documentID (int64) — the document's internal id.
 *
 * Returns:
 *   []DocumentSection — the document's sections; nil when none.
 *   error             — on database failure.
 *
 * Example:
 *   sections, err := kb.DocumentSections(docID)
 */
func (kb *KnowledgeBase) DocumentSections(documentID int64) ([]DocumentSection, error) {
	rows, err := kb.db.Query(
		`SELECT id, document_id, level, seq, heading, body, summary_body, summary_status,
		        summary_stale, source_size, tag_density, confidence, generated_by, created_at, uuid, origin_host
		 FROM document_sections
		 WHERE document_id = ?
		 ORDER BY CASE level WHEN 'gist' THEN 0 ELSE 1 END, seq`,
		documentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DocumentSection
	for rows.Next() {
		s, err := scanDocumentSection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// scanDocumentSection scans one document_sections row in the column order
// DocumentSections/documentSectionByHeading share.
func scanDocumentSection(rows *sql.Rows) (DocumentSection, error) {
	var s DocumentSection
	var stale int
	var confidence sql.NullFloat64
	var ts string
	if err := rows.Scan(&s.ID, &s.DocumentID, &s.Level, &s.Seq, &s.Heading, &s.Body,
		&s.SummaryBody, &s.SummaryStatus, &stale, &s.SourceSize, &s.TagDensity,
		&confidence, &s.GeneratedBy, &ts, &s.UUID, &s.OriginHost); err != nil {
		return s, err
	}
	s.SummaryStale = stale != 0
	if confidence.Valid {
		v := confidence.Float64
		s.Confidence = &v
	}
	s.CreatedAt = parseTimestamp(ts)
	return s, nil
}

/** DocumentByPath returns the document stored at path, or nil if none exists.
 *
 * Parameters:
 *   path (string) — the document's stored path.
 *
 * Returns:
 *   *Document — the matching document, or nil when not found.
 *   error     — on database failure.
 *
 * Example:
 *   d, err := kb.DocumentByPath("stories/a-story.md")
 */
func (kb *KnowledgeBase) DocumentByPath(path string) (*Document, error) {
	var d Document
	var ts string
	err := kb.db.QueryRow(
		`SELECT id, IFNULL(project_id, 0), title, format, path, author, published_date, checksum, ingested_at, uuid, origin_host
		 FROM documents WHERE path = ? LIMIT 1`, path,
	).Scan(&d.ID, &d.ProjectID, &d.Title, &d.Format, &d.Path, &d.Author, &d.PublishedDate, &d.Checksum, &ts, &d.UUID, &d.OriginHost)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.IngestedAt = parseTimestamp(ts)
	return &d, nil
}

// detectFormat infers a document's format from its file extension
// (narrative-documents-design.md decision 3): .md/.markdown -> markdown,
// .fountain/.spmd -> fountain (.spmd per harvey's own session-recording
// convention), anything else -> text. Never returns "pdf" -- PDF has no
// extraction path yet (decision 2) and must be requested (and rejected)
// explicitly, not guessed into existence.
func detectFormat(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown":
		return "markdown"
	case ".fountain", ".spmd":
		return "fountain"
	case ".pdf":
		return "pdf"
	default:
		return "text"
	}
}

// markdownHeadingPattern matches an ATX-style Markdown heading line.
var markdownHeadingPattern = regexp.MustCompile(`^#{1,6}\s+(.*)$`)

// segmentMarkdown splits body on ATX headings (design decision 4). A body
// with no headings at all becomes a single section with no heading, the
// same degenerate case segmentText always produces.
func segmentMarkdown(body string) []DocumentSection {
	var sections []DocumentSection
	heading := ""
	var buf strings.Builder
	haveContent := false
	flush := func() {
		if !haveContent {
			return
		}
		sections = append(sections, DocumentSection{
			Level: "section", Seq: len(sections), Heading: heading, Body: strings.TrimSpace(buf.String()),
		})
	}
	for _, line := range strings.Split(body, "\n") {
		if m := markdownHeadingPattern.FindStringSubmatch(line); m != nil {
			flush()
			heading = strings.TrimSpace(m[1])
			buf.Reset()
			haveContent = true
			continue
		}
		haveContent = true
		buf.WriteString(line)
		buf.WriteString("\n")
	}
	flush()
	return sections
}

// segmentFountain splits a Fountain source on scene headings (design
// decision 4), using github.com/rsdoiel/fountain -- the same module harvey
// already depends on, not a second implementation of the same parsing.
// Content preceding the first scene heading, if any, becomes a section with
// no heading. The title page is returned as a plain key/value map for the
// caller to fold in alongside frontmatter (decision 3).
func segmentFountain(src []byte) ([]DocumentSection, map[string]string, error) {
	doc, err := fountain.Parse(src)
	if err != nil {
		return nil, nil, fmt.Errorf("knowledge: parse fountain: %w", err)
	}
	titlePage := map[string]string{}
	for _, e := range doc.TitlePage {
		titlePage[e.Name] = strings.TrimSpace(e.Content)
	}

	var sections []DocumentSection
	heading := ""
	var body strings.Builder
	haveContent := false
	flush := func() {
		if !haveContent {
			return
		}
		sections = append(sections, DocumentSection{
			Level: "section", Seq: len(sections), Heading: heading, Body: strings.TrimSpace(body.String()),
		})
	}
	for _, e := range doc.Elements {
		if e.Type == fountain.SceneHeadingType {
			flush()
			heading = strings.TrimSpace(e.Content)
			body.Reset()
			haveContent = true
			continue
		}
		haveContent = true
		body.WriteString(e.Content)
		body.WriteString("\n")
	}
	flush()
	return sections, titlePage, nil
}

// segmentText treats the whole input as one section -- the degenerate case
// that proves the schema needs no special-casing for a document with no
// internal structure (design decision 4).
func segmentText(body string) []DocumentSection {
	return []DocumentSection{{Level: "section", Seq: 0, Body: strings.TrimSpace(body)}}
}

// documentFrontmatter is a loose, tolerant decode of whatever YAML
// frontmatter a document happens to carry -- every field optional,
// unrecognized keys ignored. Field names match antennaApp's own documented
// vocabulary (design decision 3), not a generic Jekyll/Hugo guess.
type documentFrontmatter struct {
	Title         string   `yaml:"title"`
	Description   string   `yaml:"description"`
	PubDate       string   `yaml:"pubDate"`
	DatePublished string   `yaml:"datePublished"`
	DateCreated   string   `yaml:"dateCreated"`
	Author        string   `yaml:"author"`
	Keywords      []string `yaml:"keywords,flow"`
}

// extractFrontmatter reuses splitFrontmatter (recordfile.go) but, unlike a
// record, treats "no frontmatter" as the normal case, not an error (design
// decision 3): a Fountain screenplay or a plain story usually has none. An
// unterminated frontmatter block returns a warning and falls back to
// treating the whole input as body, the same non-fatal-warning philosophy
// kb ingest already uses for records. fields carries whichever of
// title/description/published_date/author were present; keywords is the
// frontmatter's keyword list, resolved into concepts by the caller
// (decision 5), not here.
func extractFrontmatter(data []byte) (fields map[string]string, keywords []string, body string, warning string) {
	front, rest, err := splitFrontmatter(string(data))
	if err != nil {
		if strings.Contains(err.Error(), "closing --- not found") {
			return nil, nil, string(data), "unterminated frontmatter block; ingesting whole file as body"
		}
		return nil, nil, string(data), ""
	}
	var fm documentFrontmatter
	if err := yaml.Unmarshal([]byte(front), &fm); err != nil {
		return nil, nil, string(data), "malformed frontmatter; ingesting whole file as body"
	}
	fields = map[string]string{}
	if fm.Title != "" {
		fields["title"] = fm.Title
	}
	if fm.Description != "" {
		fields["description"] = fm.Description
	}
	published := fm.PubDate
	if published == "" {
		published = fm.DatePublished
	}
	if published == "" {
		published = fm.DateCreated
	}
	if published != "" {
		fields["published_date"] = published
	}
	if fm.Author != "" {
		fields["author"] = fm.Author
	}
	return fields, fm.Keywords, rest, ""
}

/** ParsedDocument is the result of reading and segmenting one document file,
 * before anything is written to the database. Sections holds only
 * `'section'`-level rows (design decision 1) -- the caller creates the
 * `'gist'` row itself, seeded from GistSeed when a frontmatter description
 * or Fountain title page provided one.
 */
type ParsedDocument struct {
	Format        string
	Title         string
	Author        string
	PublishedDate string
	Keywords      []string
	GistSeed      string
	Sections      []DocumentSection
	Checksum      string
	Warning       string
}

/** ParseDocumentFile reads path, determines its format, and segments it per
 * narrative-documents-design.md decisions 2-4: Markdown frontmatter and
 * Fountain title pages are both recognized metadata sources (decision 3),
 * PDF is rejected outright rather than silently mishandled (decision 2).
 *
 * Parameters:
 *   path           (string) — path to the document file.
 *   formatOverride (string) — "markdown"/"fountain"/"text"/"pdf", or "" to
 *                             infer from the file extension.
 *
 * Returns:
 *   *ParsedDocument — the parsed document, ready for the caller to reconcile
 *                     against any existing row (see cmd/kb's document
 *                     ingest, which owns that reconciliation the same way
 *                     cmd/kb/ingest.go owns it for records).
 *   error           — on a read failure, a malformed Fountain source, or a
 *                     PDF document (not yet supported, no extraction path
 *                     exists).
 *
 * Example:
 *   pd, err := knowledge.ParseDocumentFile("stories/a-story.md", "")
 */
func ParseDocumentFile(path, formatOverride string) (*ParsedDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("knowledge: read document %s: %w", path, err)
	}
	format := formatOverride
	if format == "" {
		format = detectFormat(path)
	}
	if format == "pdf" {
		return nil, fmt.Errorf("knowledge: %s: pdf documents are not yet supported (no text-extraction path exists)", path)
	}

	pd := &ParsedDocument{Format: format, Checksum: checksum(data)}
	switch format {
	case "markdown":
		fields, keywords, body, warning := extractFrontmatter(data)
		pd.Title = fields["title"]
		pd.Author = fields["author"]
		pd.PublishedDate = fields["published_date"]
		pd.GistSeed = fields["description"]
		pd.Keywords = keywords
		pd.Warning = warning
		pd.Sections = segmentMarkdown(body)
	case "fountain":
		sections, titlePage, err := segmentFountain(data)
		if err != nil {
			return nil, err
		}
		pd.Sections = sections
		pd.Title = titlePage["Title"]
		pd.Author = titlePage["Author"]
	default: // text
		pd.Sections = segmentText(string(data))
	}
	return pd, nil
}

/** UpdateDocumentMetadata updates a document's title/author/published_date/
 * checksum in place, and refreshes ingested_at. Used when re-ingesting a
 * changed file (design decision 10) -- the document row itself is always
 * updated in place; only its sections need the careful match-by-heading
 * treatment.
 *
 * Parameters:
 *   id            (int64)  — the document's internal id.
 *   title, author, publishedDate, checksum (string) — the new values.
 *
 * Returns:
 *   error — on database failure.
 *
 * Example:
 *   err := kb.UpdateDocumentMetadata(docID, "New Title", "", "", newChecksum)
 */
func (kb *KnowledgeBase) UpdateDocumentMetadata(id int64, title, author, publishedDate, checksum string) error {
	_, err := kb.db.Exec(
		`UPDATE documents SET title = ?, author = ?, published_date = ?, checksum = ?, ingested_at = CURRENT_TIMESTAMP WHERE id = ?`,
		title, author, publishedDate, checksum, id,
	)
	return err
}

/** UpdateDocumentSectionBody updates a section's raw body and word-count
 * signal on re-ingest, and sets its stale flag (design decision 10) --
 * summary_body/summary_status/confidence are deliberately left untouched,
 * since the whole point is that an existing summary is never silently
 * discarded when its underlying text changes.
 *
 * Parameters:
 *   id         (int64)  — the section's internal id.
 *   body       (string) — the new raw body.
 *   sourceSize (int)    — the new word count.
 *   stale      (bool)   — whether this change should mark the section stale
 *                         (true whenever an existing drafted/reviewed
 *                         summary is now describing different text).
 *
 * Returns:
 *   error — on database failure.
 *
 * Example:
 *   err := kb.UpdateDocumentSectionBody(secID, newBody, 42, true)
 */
func (kb *KnowledgeBase) UpdateDocumentSectionBody(id int64, body string, sourceSize int, stale bool) error {
	_, err := kb.db.Exec(
		`UPDATE document_sections SET body = ?, source_size = ?, summary_stale = ? WHERE id = ?`,
		body, sourceSize, boolToInt(stale), id,
	)
	return err
}

/** MarkDocumentSectionStale sets a section's summary_stale flag without
 * touching its body or summary -- used on the gist row when any section of
 * its document changes on re-ingest (design decision 10), since a
 * whole-document gist describes the document as a whole.
 *
 * Parameters:
 *   id (int64) — the section's internal id.
 *
 * Returns:
 *   error — on database failure.
 *
 * Example:
 *   err := kb.MarkDocumentSectionStale(gistSectionID)
 */
func (kb *KnowledgeBase) MarkDocumentSectionStale(id int64) error {
	_, err := kb.db.Exec(`UPDATE document_sections SET summary_stale = 1 WHERE id = ?`, id)
	return err
}
