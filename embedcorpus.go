package knowledge

import "fmt"

/** EmbeddedKinds are the search kinds semantic search embeds, in the order
 * SearchKinds lists them: observations, decision records, reviewed document
 * summaries and concepts. Projects and sources are short and lexical search
 * serves them (workspace DR-0088, semantic-search-design.md section 6).
 *
 * Example:
 *   for _, k := range knowledge.EmbeddedKinds { fmt.Println(k) }
 */
var EmbeddedKinds = []string{"records", "observations", "concepts", "documents"}

/** CorpusRow is one row of the knowledge base as semantic search sees it.
 *
 * Fields:
 *   SourceType (string) — kb_fts source_type: observation, record, concept or document_summary
 *   SourceID   (int64)  — the row's id in its own table
 *   Text       (string) — the text lexical search indexes for the row, which is the text embedded
 *
 * Example:
 *   rows, _ := kb.EmbedCorpus(knowledge.SearchScope{})
 *   fmt.Println(rows[0].SourceType, rows[0].SourceID)
 */
type CorpusRow struct {
	SourceType string
	SourceID   int64
	Text       string
}

/** EmbedCorpus returns the rows semantic search embeds or ranks, narrowed by
 * the same scope lexical search uses (SearchScoped), so the two cannot disagree
 * about what a scope contains. The text is read from the full-text index, so
 * there is one definition of a row's text: an unreviewed document summary is
 * absent because lexical search does not trust it either. Limit is ignored.
 *
 * Parameters:
 *   scope (SearchScope) — where to look; the zero value is every embedded row
 *
 * Returns:
 *   []CorpusRow — the rows, in no particular order
 *   error — ErrInvalid for a kind that is not embedded (the message names the
 *     choices), an unknown kind, or Project with Workspace; ErrNotFound for an
 *     unknown project; ErrNoMembers when no requested kind can exist in the scope
 *
 * Example:
 *   rows, err := kb.EmbedCorpus(knowledge.SearchScope{Project: "harvey", Kinds: []string{"records"}})
 */
func (kb *KnowledgeBase) EmbedCorpus(scope SearchScope) ([]CorpusRow, error) {
	if !kb.ftsAvailable {
		return nil, fmt.Errorf("full-text search is not available (FTS5 not compiled in)")
	}
	f, err := kb.scopeFilter(scope, EmbeddedKinds, true)
	if err != nil {
		return nil, err
	}
	rs, err := kb.db.Query(`
		SELECT kb_fts.source_type, CAST(kb_fts.source_id AS INTEGER), kb_fts.body
		FROM kb_fts WHERE 1 = 1`+f.where, f.args...)
	if err != nil {
		return nil, fmt.Errorf("knowledge: embed corpus: %w", err)
	}
	defer rs.Close()
	var rows []CorpusRow
	for rs.Next() {
		var r CorpusRow
		if err := rs.Scan(&r.SourceType, &r.SourceID, &r.Text); err != nil {
			return nil, fmt.Errorf("knowledge: embed corpus: %w", err)
		}
		rows = append(rows, r)
	}
	return rows, rs.Err()
}
