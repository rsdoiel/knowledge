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

CREATE TABLE IF NOT EXISTS document_section_concepts (
    section_id INTEGER REFERENCES document_sections(id) ON DELETE CASCADE,
    concept_id INTEGER REFERENCES concepts(id)          ON DELETE CASCADE,
    PRIMARY KEY (section_id, concept_id)
);
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
	if d.UUID == "" {
		u, err := uuid.NewV7()
		if err != nil {
			return 0, fmt.Errorf("knowledge: generate uuid: %w", err)
		}
		d.UUID = u.String()
	}
	if d.OriginHost == "" {
		d.OriginHost, _ = os.Hostname()
	}
	res, err := kb.db.Exec(
		`INSERT INTO documents (project_id, title, format, path, author, published_date, checksum, uuid, origin_host)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		projectValue(d.ProjectID), d.Title, d.Format, d.Path, d.Author, d.PublishedDate, d.Checksum, d.UUID, d.OriginHost,
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
	if s.UUID == "" {
		u, err := uuid.NewV7()
		if err != nil {
			return 0, fmt.Errorf("knowledge: generate uuid: %w", err)
		}
		s.UUID = u.String()
	}
	if s.OriginHost == "" {
		s.OriginHost, _ = os.Hostname()
	}
	if s.SummaryStatus == "" {
		s.SummaryStatus = "unsummarized"
	}
	res, err := kb.db.Exec(
		`INSERT INTO document_sections
		    (document_id, level, seq, heading, body, summary_body, summary_status,
		     summary_stale, source_size, tag_density, confidence, generated_by, uuid, origin_host)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.DocumentID, s.Level, s.Seq, s.Heading, s.Body, s.SummaryBody, s.SummaryStatus,
		boolToInt(s.SummaryStale), s.SourceSize, s.TagDensity, s.Confidence, s.GeneratedBy, s.UUID, s.OriginHost,
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

/** DocumentByID returns the document with the given internal id, or nil if
 * none exists.
 *
 * Parameters:
 *   id (int64) — the document's internal id.
 *
 * Returns:
 *   *Document — the matching document, or nil when not found.
 *   error     — on database failure.
 *
 * Example:
 *   d, err := kb.DocumentByID(42)
 */
func (kb *KnowledgeBase) DocumentByID(id int64) (*Document, error) {
	var d Document
	var ts string
	err := kb.db.QueryRow(
		`SELECT id, IFNULL(project_id, 0), title, format, path, author, published_date, checksum, ingested_at, uuid, origin_host
		 FROM documents WHERE id = ?`, id,
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

// markdownH1Pattern matches only a true H1 (a single '#'), the level a
// fallback title has to come from -- a deeper heading names a section, not
// the document.
var markdownH1Pattern = regexp.MustCompile(`^#\s+(.*)$`)

// firstH1Heading returns the first true H1 heading in body, or "" if none.
// Used as a Markdown document's title when frontmatter supplies none (a
// colleague's MADR-style ADR being the motivating case, but this is a
// general fallback): a document's own text almost always names it better
// than the caller's filepath.Base would.
func firstH1Heading(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if m := markdownH1Pattern.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

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

// fountainAnnotationTypes are element types the fountain library itself
// treats as out-of-band annotations rather than narrative prose -- its own
// String()/ToHTML() only include them when a ShowNotes/ShowSection/
// ShowSynopsis flag is explicitly set (fountain.go). segmentFountain
// follows the same default: excluded from a section's body entirely, not
// just hidden from rendering. This matters beyond tidiness -- standard
// Fountain reserves [[double-brackets]] for Notes, which collides directly
// with wikilink tagging's own [[Name]] syntax. A Note's raw "[[...]]" text
// left in the body would otherwise be re-matched as a bogus concept name at
// tag time (found via a real Harvey session file, whose own [[write: path
// -- ok]]/[[read: path -- ok]] file-event convention uses exactly this
// syntax).
var fountainAnnotationTypes = map[int]bool{
	fountain.NoteType:     true,
	fountain.BoneyardType: true,
	fountain.SectionType:  true,
	fountain.SynopsisType: true,
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
		return nil, nil, invalidf("knowledge: parse fountain: %w", err)
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
		if fountainAnnotationTypes[e.Type] {
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
		return nil, invalidf("knowledge: %s: pdf documents are not yet supported (no text-extraction path exists)", path)
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
		if pd.Title == "" {
			pd.Title = firstH1Heading(body)
		}
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
func (kb *KnowledgeBase) UpdateDocumentSectionBody(id int64, body string, sourceSize, tagDensity int, stale bool) error {
	_, err := kb.db.Exec(
		`UPDATE document_sections SET body = ?, source_size = ?, tag_density = ?, summary_stale = ? WHERE id = ?`,
		body, sourceSize, tagDensity, boolToInt(stale), id,
	)
	return err
}

/** UpdateDocumentSectionTagDensity updates only a section's tag_density,
 * without touching body, summary, or stale state. Used to refresh the gist
 * row's density on re-ingest (design decision 6) -- the gist has no body of
 * its own to update, only a density computed from the whole document's
 * current text.
 *
 * Parameters:
 *   id         (int64) — the section's internal id.
 *   tagDensity (int)   — the new density value.
 *
 * Returns:
 *   error — on database failure.
 *
 * Example:
 *   err := kb.UpdateDocumentSectionTagDensity(gistSectionID, 4)
 */
func (kb *KnowledgeBase) UpdateDocumentSectionTagDensity(id int64, tagDensity int) error {
	_, err := kb.db.Exec(`UPDATE document_sections SET tag_density = ? WHERE id = ?`, tagDensity, id)
	return err
}

/** DraftDocumentSummary writes a gist or section summary (design decision
 * 7): sets summary_body, summary_status to "drafted", generatedBy (a caller
 * identity -- "human" or a model name; knowledge never calls a model
 * itself), and confidence. A fresh draft clears summary_stale, since it is
 * by definition not stale against itself. Range-validating confidence is
 * the caller's job (the CLI does it); this is a trusted data-layer write.
 *
 * Parameters:
 *   sectionID   (int64)    — the document_sections row to draft.
 *   body        (string)   — the summary text.
 *   generatedBy (string)   — who/what produced it.
 *   confidence  (*float64) — self-reported confidence, or nil.
 *
 * Returns:
 *   error — on database failure.
 *
 * Example:
 *   c := 0.8
 *   err := kb.DraftDocumentSummary(sectionID, "...", "human", &c)
 */
func (kb *KnowledgeBase) DraftDocumentSummary(sectionID int64, body, generatedBy string, confidence *float64) error {
	_, err := kb.db.Exec(
		`UPDATE document_sections
		 SET summary_body = ?, summary_status = 'drafted', generated_by = ?, confidence = ?, summary_stale = 0
		 WHERE id = ?`,
		body, generatedBy, confidence, sectionID,
	)
	return err
}

/** PromoteDocumentSummary promotes a drafted summary to reviewed (design
 * decision 7) -- the only human action that makes a summary trusted. This
 * is also the only point a summary is indexed into kb_fts (decision 8): a
 * section's raw body is never indexed, only its reviewed summary, and only
 * once it is reviewed.
 *
 * Parameters:
 *   sectionID (int64) — the document_sections row to promote.
 *
 * Returns:
 *   error — if the section does not exist, is not currently "drafted", or
 *           on database failure.
 *
 * Example:
 *   err := kb.PromoteDocumentSummary(sectionID)
 */
func (kb *KnowledgeBase) PromoteDocumentSummary(sectionID int64) error {
	var status, summaryBody, heading, level string
	var documentID int64
	err := kb.db.QueryRow(
		`SELECT summary_status, summary_body, heading, level, document_id FROM document_sections WHERE id = ?`,
		sectionID,
	).Scan(&status, &summaryBody, &heading, &level, &documentID)
	if err == sql.ErrNoRows {
		return notFoundf("knowledge: no document section with id %d", sectionID)
	}
	if err != nil {
		return err
	}
	if status != "drafted" {
		return conflictf("knowledge: document section %d is %q, not drafted; draft it first", sectionID, status)
	}
	if _, err := kb.db.Exec(`UPDATE document_sections SET summary_status = 'reviewed' WHERE id = ?`, sectionID); err != nil {
		return err
	}

	var title string
	var projectID int64
	if err := kb.db.QueryRow(
		`SELECT title, IFNULL(project_id, 0) FROM documents WHERE id = ?`, documentID,
	).Scan(&title, &projectID); err != nil {
		return err
	}
	kb.indexDocumentSummaryFTS(sectionID, level, title, heading, summaryBody, projectID)
	return nil
}

// indexDocumentSummaryFTS writes the kb_fts entry for a promoted summary
// (design decision 8), the same delete-then-reinsert shape every other
// writer uses (see indexRecordFTS). Never called for anything but a
// reviewed summary's body -- raw section text has no writer into kb_fts at
// all.
func (kb *KnowledgeBase) indexDocumentSummaryFTS(sectionID int64, level, title, heading, summaryBody string, projectID int64) {
	if !kb.ftsAvailable {
		return
	}
	label := title
	if heading != "" {
		label = title + " — " + heading
	}
	_, _ = kb.db.Exec(
		`DELETE FROM kb_fts WHERE source_type = 'document_summary' AND source_id = ?`, sectionID)
	_, _ = kb.db.Exec(
		`INSERT INTO kb_fts(body, kind, label, descr, source_type, source_id, project_id)
		 VALUES (?, ?, ?, ?, 'document_summary', ?, ?)`,
		summaryBody, level, label, heading, sectionID, projectID)
}

/** DocumentReviewItem is one row of the review queue (kb document review
 * list): a document_sections row plus the document context needed to
 * display it.
 */
type DocumentReviewItem struct {
	DocumentSection
	DocumentTitle string
}

/** DocumentReviewQueue returns document_sections rows needing attention
 * (design decision 7), optionally scoped to one project and one status.
 * With status == "", the default is the actual triage queue --
 * "unsummarized" and "drafted" rows, not "reviewed" ones, which need no
 * further attention. Passing an explicit status (including "reviewed")
 * overrides that default, for auditing.
 *
 * Parameters:
 *   projectID (int64)  — scope to one project, or 0 for every project.
 *   status    (string) — an exact status to filter to, or "" for the
 *                        unsummarized+drafted default.
 *
 * Returns:
 *   []DocumentReviewItem — matching sections, gist first then by seq within
 *                          each document; nil when none.
 *   error                — on database failure.
 *
 * Example:
 *   items, err := kb.DocumentReviewQueue(0, "")
 */
func (kb *KnowledgeBase) DocumentReviewQueue(projectID int64, status string) ([]DocumentReviewItem, error) {
	query := `SELECT s.id, s.document_id, s.level, s.seq, s.heading, s.body, s.summary_body, s.summary_status,
	                 s.summary_stale, s.source_size, s.tag_density, s.confidence, s.generated_by, s.created_at,
	                 s.uuid, s.origin_host, d.title
	          FROM document_sections s
	          JOIN documents d ON d.id = s.document_id`
	var where []string
	var args []any
	if projectID != 0 {
		where = append(where, "d.project_id = ?")
		args = append(args, projectID)
	}
	if status != "" {
		where = append(where, "s.summary_status = ?")
		args = append(args, status)
	} else {
		where = append(where, "s.summary_status IN ('unsummarized', 'drafted')")
	}
	query += " WHERE " + strings.Join(where, " AND ")
	query += " ORDER BY d.id, CASE s.level WHEN 'gist' THEN 0 ELSE 1 END, s.seq"

	rows, err := kb.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DocumentReviewItem
	for rows.Next() {
		var it DocumentReviewItem
		var stale int
		var confidence sql.NullFloat64
		var ts string
		if err := rows.Scan(&it.ID, &it.DocumentID, &it.Level, &it.Seq, &it.Heading, &it.Body,
			&it.SummaryBody, &it.SummaryStatus, &stale, &it.SourceSize, &it.TagDensity,
			&confidence, &it.GeneratedBy, &ts, &it.UUID, &it.OriginHost, &it.DocumentTitle); err != nil {
			return nil, err
		}
		it.SummaryStale = stale != 0
		if confidence.Valid {
			v := confidence.Float64
			it.Confidence = &v
		}
		it.CreatedAt = parseTimestamp(ts)
		out = append(out, it)
	}
	return out, rows.Err()
}

/** LinkDocumentSectionConcept associates a document section with a concept.
 * Duplicate links are silently ignored. Verbatim shape of
 * LinkRecordConcept (knowledge.go), for the same relationship one level
 * down (design decision 5).
 *
 * Parameters:
 *   sectionID (int64) — ID of the document_sections row.
 *   conceptID (int64) — ID of the concept.
 *
 * Returns:
 *   error — on database failure.
 *
 * Example:
 *   err := kb.LinkDocumentSectionConcept(sectionID, conceptID)
 */
func (kb *KnowledgeBase) LinkDocumentSectionConcept(sectionID, conceptID int64) error {
	_, err := kb.db.Exec(
		`INSERT OR IGNORE INTO document_section_concepts (section_id, concept_id) VALUES (?, ?)`,
		sectionID, conceptID,
	)
	return err
}

/** ClearDocumentSectionConcepts deletes every concept link a section carries,
 * without deleting the concepts themselves. Verbatim shape of
 * ClearRecordConcepts (knowledge.go): re-ingest calls this before re-tagging
 * a section's current wikilinks and keywords, so a tag dropped from the
 * source text is actually dropped from the database rather than only ever
 * accumulating.
 *
 * Parameters:
 *   sectionID (int64) — ID of the document_sections row.
 *
 * Returns:
 *   error — on database failure.
 *
 * Example:
 *   err := kb.ClearDocumentSectionConcepts(sectionID)
 */
func (kb *KnowledgeBase) ClearDocumentSectionConcepts(sectionID int64) error {
	_, err := kb.db.Exec(`DELETE FROM document_section_concepts WHERE section_id = ?`, sectionID)
	return err
}

/** DocumentSectionConcepts returns all concepts linked to the given document
 * section id, ordered by concept id. Mirrors RecordConcepts (knowledge.go).
 *
 * Parameters:
 *   sectionID (int64) — the document_sections row's internal id.
 *
 * Returns:
 *   []Concept — linked concepts; nil when none.
 *   error     — on database failure.
 *
 * Example:
 *   concepts, err := kb.DocumentSectionConcepts(sectionID)
 */
func (kb *KnowledgeBase) DocumentSectionConcepts(sectionID int64) ([]Concept, error) {
	rows, err := kb.db.Query(
		`SELECT c.id, c.name, c.description, c.identifier_type, c.identifier_value
		 FROM concepts c
		 JOIN document_section_concepts dsc ON dsc.concept_id = c.id
		 WHERE dsc.section_id = ?
		 ORDER BY c.id`,
		sectionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Concept
	for rows.Next() {
		var c Concept
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.IdentifierType, &c.IdentifierValue); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
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

/** Documents lists documents, optionally scoped to one project, ordered by
 * id (i.e. ingestion order). Mirrors DocumentReviewQueue's projectID
 * convention: 0 means every project.
 *
 * Parameters:
 *   projectID (int64) — scope to one project, or 0 for every project.
 *
 * Returns:
 *   []Document — matching documents; nil when none.
 *   error      — on database failure.
 *
 * Example:
 *   docs, err := kb.Documents(0)
 */
func (kb *KnowledgeBase) Documents(projectID int64) ([]Document, error) {
	query := `SELECT id, IFNULL(project_id, 0), title, format, path, author, published_date, checksum, ingested_at, uuid, origin_host
	          FROM documents`
	var args []any
	if projectID != 0 {
		query += ` WHERE project_id = ?`
		args = append(args, projectID)
	}
	query += ` ORDER BY id`
	rows, err := kb.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Document
	for rows.Next() {
		var d Document
		var ts string
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Title, &d.Format, &d.Path, &d.Author,
			&d.PublishedDate, &d.Checksum, &ts, &d.UUID, &d.OriginHost); err != nil {
			return nil, err
		}
		d.IngestedAt = parseTimestamp(ts)
		out = append(out, d)
	}
	return out, rows.Err()
}
