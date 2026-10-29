package knowledge

import (
	"fmt"
	"strings"
)

// hybridLexicalDepth is how many lexical matches a hybrid search
// takes into the fusion: deep enough that a row the semantic ranking puts first
// is not lost for want of a lexical place.
const hybridLexicalDepth = 200

// keyedColumns is searchColumns for a lookup by key: there is no MATCH, so there
// is no snippet() to call, and the passage is left for the caller to fill.
var keyedColumns = strings.Replace(searchColumns, "snippet(kb_fts, 0, '«', '»', '…', 24)", "''", 1)

/** LexicalRanking returns the rows a full-text search finds in an embedded kind,
 * best first, as the lexical half of a hybrid search. It looks only in
 * EmbeddedKinds (a row that cannot be searched by meaning would only be in one
 * of the two rankings) and goes deeper than SearchScoped's page.
 *
 * Parameters:
 *   term (string) — FTS5 query; one that is not valid syntax is searched as a phrase, as Search does
 *   scope (SearchScope) — where to look; its Kinds must be in EmbeddedKinds
 *
 * Returns:
 *   []RowKey — up to 200 rows, best first; empty when nothing matches
 *   error — ErrInvalid for a kind that is not embedded, as SearchScoped otherwise
 *
 * Example:
 *   keys, err := kb.LexicalRanking("exit codes", knowledge.SearchScope{})
 */
func (kb *KnowledgeBase) LexicalRanking(term string, scope SearchScope) ([]RowKey, error) {
	if !kb.ftsAvailable {
		return nil, fmt.Errorf("full-text search is not available (FTS5 not compiled in)")
	}
	f, err := kb.scopeFilter(scope, EmbeddedKinds, true)
	if err != nil {
		return nil, err
	}
	run := func(t string) ([]RowKey, error) {
		rs, err := kb.db.Query(`
			SELECT kb_fts.source_type, CAST(COALESCE(NULLIF(kb_fts.source_id, ''), 0) AS INTEGER)
			FROM kb_fts WHERE kb_fts MATCH ?`+f.where+`
			ORDER BY bm25(kb_fts) LIMIT ?`, append(append([]any{t}, f.args...), hybridLexicalDepth)...)
		if err != nil {
			return nil, err
		}
		defer rs.Close()
		var keys []RowKey
		for rs.Next() {
			var k RowKey
			if err := rs.Scan(&k.SourceType, &k.SourceID); err != nil {
				return nil, err
			}
			keys = append(keys, k)
		}
		return keys, rs.Err()
	}
	keys, err := run(term)
	if err != nil {
		keys, err = run(quotedPhrase(term))
	}
	if err != nil {
		return nil, fmt.Errorf("knowledge: search %q: %w", term, err)
	}
	return keys, nil
}

/** HitsFor looks up the rows named by keys and returns them in the same order,
 * with the fields a search result shows and their provenance (DR-0080), but no
 * passage: there is no query to mark one, so the caller sets it, for a semantic
 * hit from the matching chunk. A key with no row (deleted since) is left out.
 *
 * Parameters:
 *   keys ([]RowKey) — the rows to look up
 *
 * Returns:
 *   []KBSearchResult — one per key that still exists, in the order of keys
 *   error — a database error
 *
 * Example:
 *   hits, err := kb.HitsFor([]knowledge.RowKey{{SourceType: "record", SourceID: 12}})
 */
func (kb *KnowledgeBase) HitsFor(keys []RowKey) ([]KBSearchResult, error) {
	out := make([]KBSearchResult, 0, len(keys))
	for _, k := range keys {
		var r KBSearchResult
		err := kb.db.QueryRow(`
			SELECT `+keyedColumns+pidExpr+`
			FROM   kb_fts`+searchJoins+`
			WHERE  kb_fts.source_type = ? AND CAST(COALESCE(NULLIF(kb_fts.source_id, ''), 0) AS INTEGER) = ?
			LIMIT  1`, k.SourceType, k.SourceID).Scan(&r.Kind, &r.Label, &r.Snippet, &r.SourceType, &r.SourceID,
			&r.Passage, &r.Status, &r.Ref, &r.ProjectID)
		if err != nil {
			if isNoRows(err) {
				continue
			}
			return nil, fmt.Errorf("knowledge: hit %s:%d: %w", k.SourceType, k.SourceID, err)
		}
		out = append(out, r)
	}
	if err := kb.fillHitProvenance(out); err != nil {
		return nil, err
	}
	return out, nil
}
