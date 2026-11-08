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

// widenBelow is the number of rows the typed query must find to be left as it
// is: fewer than this, and a term of plain words is also searched for any of
// its words (DR-0094).
const widenBelow = 3

// widenMinLetters is how long a word must be to count in the any-word search.
const widenMinLetters = 4

/** LexicalRanking returns the rows a full-text search finds in an embedded kind,
 * best first, as the lexical half of a hybrid search. It looks only in
 * EmbeddedKinds, takes the term as typed (every word must appear in a row), and
 * goes deeper than a results page so fusion has rows to work with. A term FTS5
 * cannot parse is retried as one quoted phrase.
 *
 * Parameters:
 *   term (string) — the search term, in FTS5 syntax
 *   scope (SearchScope) — where to look; Kinds must be among EmbeddedKinds
 *
 * Returns:
 *   []RowKey — the matching rows, best first (bm25), at most hybridLexicalDepth
 *   error — ErrInvalid for a scope with a kind that is not embedded, or a database error
 *
 * Example:
 *   keys, err := kb.LexicalRanking("exit codes", knowledge.SearchScope{})
 */
func (kb *KnowledgeBase) LexicalRanking(term string, scope SearchScope) ([]RowKey, error) {
	return kb.lexicalRun(term, scope)
}

/** LexicalRankingWide is LexicalRanking for a question typed as a sentence
 * (knowledge DR-0094). It runs the typed term first. When that finds fewer than
 * three rows and the term is plain words (no quote, no *, no parentheses, no
 * column filter, no capitalised AND, OR, NOT or NEAR), it also runs an OR of the
 * term's words of four letters or more, ranked by bm25. A term in FTS5 syntax is
 * never rewritten.
 *
 * Parameters:
 *   term (string) — the search term
 *   scope (SearchScope) — where to look; Kinds must be among EmbeddedKinds
 *
 * Returns:
 *   typed ([]RowKey) — the rows the typed term found, best first
 *   widened ([]RowKey) — the rows of the any-word search, best first, which include the typed rows;
 *     nil when the typed term was left alone
 *   error — as LexicalRanking
 *
 * Example:
 *   typed, widened, err := kb.LexicalRankingWide("when did the ferry schedule change", knowledge.SearchScope{})
 */
func (kb *KnowledgeBase) LexicalRankingWide(term string, scope SearchScope) (typed, widened []RowKey, err error) {
	typed, err = kb.lexicalRun(term, scope)
	if err != nil || len(typed) >= widenBelow || hasFTS5Syntax(term) {
		return typed, nil, err
	}
	any := anyWordTerm(term)
	if any == "" {
		return typed, nil, nil
	}
	widened, err = kb.lexicalRun(any, scope)
	if err != nil {
		return typed, nil, err
	}
	return typed, widened, nil
}

// lexicalRun is the search both rankings share: term as given, retried as a
// quoted phrase when FTS5 cannot parse it.
func (kb *KnowledgeBase) lexicalRun(term string, scope SearchScope) ([]RowKey, error) {
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

// hasFTS5Syntax reports whether term was written in FTS5 query syntax on purpose,
// as opposed to plain words with the punctuation of ordinary prose (a colon, a
// bracket, a quoted word). It is true for one of the operators AND, OR, NOT and
// NEAR in capitals, a word that ends in *, a word that starts with + or ^, a
// column filter on one of kb_fts's two searchable columns (body: and kind:), or
// a double-quoted phrase of two or more words.
func hasFTS5Syntax(term string) bool {
	for _, w := range strings.Fields(term) {
		switch w {
		case "AND", "OR", "NOT", "NEAR":
			return true
		}
		if strings.HasSuffix(w, "*") || strings.HasPrefix(w, "+") || strings.HasPrefix(w, "^") ||
			strings.HasPrefix(w, "body:") || strings.HasPrefix(w, "kind:") {
			return true
		}
	}
	for rest := term; ; {
		a := strings.Index(rest, `"`)
		if a < 0 {
			return false
		}
		b := strings.Index(rest[a+1:], `"`)
		if b < 0 {
			return false
		}
		if strings.ContainsAny(rest[a+1:a+1+b], " \t\n") {
			return true
		}
		rest = rest[a+1+b+1:]
	}
}

// anyWordTerm turns plain words into an FTS5 OR of the quoted words of at least
// widenMinLetters letters; "" when there are none.
func anyWordTerm(term string) string {
	var words []string
	for _, w := range strings.FieldsFunc(term, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_')
	}) {
		if len(w) >= widenMinLetters {
			words = append(words, `"`+w+`"`)
		}
	}
	return strings.Join(words, " OR ")
}
