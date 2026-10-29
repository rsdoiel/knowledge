// Package evalset measures how well the lexical, semantic and hybrid searches
// find the rows a person would want, against a set of labelled queries (semantic
// plan T9). It is used by tests: the tunables of semantic search (the cutoff, the
// chunk size, the fusion constant) are set from its numbers, and the next model
// comparison reuses it.
package evalset

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

/** Matcher names a relevant row in a way that survives id renumbering: a record
 * by its qualified reference, an observation by a substring of its text, a concept
 * by its name.
 *
 * Fields:
 *   Type     (string) — "record", "observation" or "concept" (a kb_fts source_type)
 *   Ref      (string) — for a record, "project/DR-0001" or "workspace/DR-0001"
 *   Contains (string) — for an observation, text its body contains
 *   Name     (string) — for a concept, its name
 */
type Matcher struct {
	Type     string `json:"type"`
	Ref      string `json:"ref,omitempty"`
	Contains string `json:"contains,omitempty"`
	Name     string `json:"name,omitempty"`
}

/** Query is one labelled query.
 *
 * Fields:
 *   ID       (string)    — a short stable name, q01
 *   Query    (string)    — the text typed to kb search, as is, for all three modes
 *   Note     (string)    — why the query is in the set
 *   NoAnswer (bool)      — nothing in the corpus answers it; a good search returns nothing
 *   Relevant ([]Matcher) — the rows that answer it; any one found is a success
 */
type Query struct {
	ID       string    `json:"id"`
	Query    string    `json:"query"`
	Note     string    `json:"note,omitempty"`
	NoAnswer bool      `json:"no_answer,omitempty"`
	Relevant []Matcher `json:"relevant,omitempty"`
}

/** Set is a labelled query set as kept in testdata.
 *
 * Example:
 *   set, err := evalset.Load("testdata/labelled_queries.json")
 */
type Set struct {
	About   string  `json:"about"`
	Queries []Query `json:"queries"`
}

/** Load reads a labelled query set and checks it: ids are unique, a query has
 * text, and it is either no-answer or has at least one relevant row.
 *
 * Parameters:
 *   path (string) — the JSON file
 *
 * Returns:
 *   *Set — the set
 *   error — the file is unreadable, not JSON, or a query is malformed
 *
 * Example:
 *   set, err := evalset.Load("internal/evalset/testdata/labelled_queries.json")
 */
func Load(path string) (*Set, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Set
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("evalset: %s: %w", path, err)
	}
	seen := map[string]bool{}
	for _, q := range s.Queries {
		switch {
		case q.ID == "" || seen[q.ID]:
			return nil, fmt.Errorf("evalset: query id %q is blank or repeated", q.ID)
		case strings.TrimSpace(q.Query) == "":
			return nil, fmt.Errorf("evalset: %s has no query text", q.ID)
		case q.NoAnswer == (len(q.Relevant) > 0):
			return nil, fmt.Errorf("evalset: %s must be either no_answer or have relevant rows", q.ID)
		}
		for _, m := range q.Relevant {
			if m.Type == "record" && m.Ref == "" || m.Type == "observation" && m.Contains == "" || m.Type == "concept" && m.Name == "" ||
				m.Type != "record" && m.Type != "observation" && m.Type != "concept" {
				return nil, fmt.Errorf("evalset: %s has a malformed matcher %+v", q.ID, m)
			}
		}
		seen[q.ID] = true
	}
	return &s, nil
}

/** Resolve finds the rows each query's matchers name in a knowledge base. A
 * matcher that names no row is a stale label and is reported, not skipped.
 *
 * Parameters:
 *   kb (*knowledge.KnowledgeBase) — the corpus the set is run against
 *
 * Returns:
 *   map[string]map[knowledge.RowKey]bool — for each query id, its relevant rows
 *   []string — one line per matcher that names no row
 *   error — a database error
 *
 * Example:
 *   rel, stale, err := set.Resolve(kb)
 */
func (s *Set) Resolve(kb *knowledge.KnowledgeBase) (map[string]map[knowledge.RowKey]bool, []string, error) {
	rows, err := kb.EmbedCorpus(knowledge.SearchScope{})
	if err != nil {
		return nil, nil, err
	}
	keys := make([]knowledge.RowKey, len(rows))
	for i, r := range rows {
		keys[i] = knowledge.RowKey{SourceType: r.SourceType, SourceID: r.SourceID}
	}
	hits, err := kb.HitsFor(keys)
	if err != nil {
		return nil, nil, err
	}
	out := map[string]map[knowledge.RowKey]bool{}
	var stale []string
	for _, q := range s.Queries {
		rel := map[knowledge.RowKey]bool{}
		for _, m := range q.Relevant {
			found := false
			for _, h := range hits {
				if h.SourceType != m.Type {
					continue
				}
				if m.Type == "record" && h.Ref == m.Ref || m.Type == "observation" && strings.Contains(h.Snippet, m.Contains) ||
					m.Type == "concept" && h.Label == m.Name {
					rel[knowledge.RowKey{SourceType: h.SourceType, SourceID: h.SourceID}] = true
					found = true
				}
			}
			if !found {
				stale = append(stale, fmt.Sprintf("%s: nothing matches %+v", q.ID, m))
			}
		}
		out[q.ID] = rel
	}
	return out, stale, nil
}

// Ranking is one query's three rankings, computed once so the cutoff and the
// fusion constant can be swept without embedding again.
type ranking struct {
	query    Query
	relevant map[knowledge.RowKey]bool
	lexical  []knowledge.RowKey
	semantic []knowledge.SemanticHit // every indexed row, best first, no cutoff
}

/** Prepared holds the lexical and semantic rankings of every query in a set, so
 * Score can try different settings cheaply.
 */
type Prepared struct {
	rankings []ranking
}

/** Prepare ranks every query by words and by meaning against an index. The
 * semantic ranking is taken with no cutoff and kept whole.
 *
 * Parameters:
 *   kb (*knowledge.KnowledgeBase) — the corpus
 *   v (*knowledge.Vectors) — its index, built by e
 *   e (knowledge.Embedder) — the model under test
 *   set (*Set) — the labelled queries
 *   relevant (map[string]map[knowledge.RowKey]bool) — from Resolve
 *
 * Returns:
 *   *Prepared — the rankings
 *   error — a search error
 *
 * Example:
 *   p, err := evalset.Prepare(kb, v, emb, set, rel)
 */
func Prepare(kb *knowledge.KnowledgeBase, v *knowledge.Vectors, e knowledge.Embedder, set *Set, relevant map[string]map[knowledge.RowKey]bool) (*Prepared, error) {
	return PrepareWith(kb, v, e, set, relevant, nil)
}

/** PrepareWith is Prepare with a rewrite of the query for the full-text half, to
 * measure a different words search than the one typed (AnyWord, for one).
 *
 * Parameters:
 *   lexicalTerm (func(string) string) — maps the query to the FTS5 term; nil means the query as typed
 *
 * Example:
 *   p, err := evalset.PrepareWith(kb, v, emb, set, rel, evalset.AnyWord)
 */
func PrepareWith(kb *knowledge.KnowledgeBase, v *knowledge.Vectors, e knowledge.Embedder, set *Set, relevant map[string]map[knowledge.RowKey]bool,
	lexicalTerm func(string) string) (*Prepared, error) {
	p := &Prepared{}
	for _, q := range set.Queries {
		term := q.Query
		if lexicalTerm != nil {
			term = lexicalTerm(term)
		}
		lex, err := kb.LexicalRanking(term, knowledge.SearchScope{})
		if err != nil {
			return nil, fmt.Errorf("evalset: %s lexical: %w", q.ID, err)
		}
		sem, err := kb.SemanticSearch(v, e, q.Query, knowledge.SemanticOptions{MinScore: knowledge.NoCutoff, Limit: 1 << 20})
		if err != nil {
			return nil, fmt.Errorf("evalset: %s semantic: %w", q.ID, err)
		}
		p.rankings = append(p.rankings, ranking{query: q, relevant: relevant[q.ID], lexical: lex, semantic: sem.Hits})
	}
	return p, nil
}

/** Params are the settings Score tries.
 *
 * Fields:
 *   MinScore (float32) — the semantic cutoff
 *   FusionK  (int)     — the fusion constant; zero means the library's default
 */
type Params struct {
	MinScore float32
	FusionK  int
}

/** Result is the score of one search mode over a set.
 *
 * Fields:
 *   Recall5, Recall10 (float64) — mean over the answerable queries of the relevant rows found in the top k,
 *                                 divided by the most that could be (the smaller of k and the number relevant)
 *   MRR       (float64) — mean over the answerable queries of 1 / the rank of the first relevant row, 0 if none
 *   Empty     (int)     — answerable queries for which the mode returned nothing
 *   Quiet     (int)     — no-answer queries for which the mode returned nothing (the good outcome)
 *   Noisy     (int)     — no-answer queries for which it returned something
 *   MeanRows  (float64) — mean rows returned over all queries
 */
type Result struct {
	Recall5, Recall10, MRR float64
	Empty, Quiet, Noisy    int
	MeanRows               float64
}

/** Report is the three modes' results at one setting.
 *
 * Fields:
 *   Params   (Params)  — the setting
 *   Lexical, Semantic, Hybrid (Result) — the modes
 *   PerQuery ([]QueryRank) — each query's first relevant rank per mode, for finding the misses
 */
type Report struct {
	Params                    Params
	Lexical, Semantic, Hybrid Result
	PerQuery                  []QueryRank
}

/** QueryRank is where a query's first relevant row came in each mode (0: not returned), and how many rows each returned. */
type QueryRank struct {
	ID                        string
	Answerable                bool
	LexRank, SemRank, HybRank int
	LexRows, SemRows, HybRows int
}

// firstAndRecall scores one ordered list against a query's relevant rows.
func firstAndRecall(list []knowledge.RowKey, rel map[knowledge.RowKey]bool) (first int, r5, r10 float64) {
	found5, found10 := 0, 0
	for i, k := range list {
		if !rel[k] {
			continue
		}
		if first == 0 {
			first = i + 1
		}
		if i < 5 {
			found5++
		}
		if i < 10 {
			found10++
		}
	}
	den := func(k int) float64 {
		if len(rel) < k {
			return float64(len(rel))
		}
		return float64(k)
	}
	return first, float64(found5) / den(5), float64(found10) / den(10)
}

/** Score evaluates the three modes at one setting. Lexical is the query as typed
 * to full-text search; semantic keeps the rows at or above the cutoff; hybrid
 * fuses the two as kb search -hybrid does.
 *
 * Parameters:
 *   par (Params) — the cutoff and fusion constant
 *
 * Returns:
 *   Report — the results
 *
 * Example:
 *   rep := p.Score(evalset.Params{MinScore: 0.3})
 *   fmt.Print(rep)
 */
func (p *Prepared) Score(par Params) Report {
	rep := Report{Params: par}
	type acc struct {
		r5, r10, rr, rows   float64
		empty, quiet, noisy int
	}
	var a [3]acc
	answerable := 0
	for _, r := range p.rankings {
		var sem []knowledge.RowKey
		for _, h := range r.semantic {
			if h.Score >= par.MinScore {
				sem = append(sem, knowledge.RowKey{SourceType: h.SourceType, SourceID: h.SourceID})
			}
		}
		var hyb []knowledge.RowKey
		for _, f := range knowledge.FuseRankings(r.lexical, sem, par.FusionK) {
			hyb = append(hyb, knowledge.RowKey{SourceType: f.SourceType, SourceID: f.SourceID})
		}
		lists := [3][]knowledge.RowKey{r.lexical, sem, hyb}
		qr := QueryRank{ID: r.query.ID, Answerable: !r.query.NoAnswer}
		ranks := [3]*int{&qr.LexRank, &qr.SemRank, &qr.HybRank}
		counts := [3]*int{&qr.LexRows, &qr.SemRows, &qr.HybRows}
		for m, list := range lists {
			a[m].rows += float64(len(list))
			*counts[m] = len(list)
			if r.query.NoAnswer {
				if len(list) == 0 {
					a[m].quiet++
				} else {
					a[m].noisy++
				}
				continue
			}
			first, r5, r10 := firstAndRecall(list, r.relevant)
			*ranks[m] = first
			a[m].r5 += r5
			a[m].r10 += r10
			if first > 0 {
				a[m].rr += 1 / float64(first)
			}
			if len(list) == 0 {
				a[m].empty++
			}
		}
		if !r.query.NoAnswer {
			answerable++
		}
		rep.PerQuery = append(rep.PerQuery, qr)
	}
	mk := func(x acc) Result {
		n := float64(answerable)
		if n == 0 {
			n = 1
		}
		return Result{Recall5: x.r5 / n, Recall10: x.r10 / n, MRR: x.rr / n, Empty: x.empty, Quiet: x.quiet, Noisy: x.noisy,
			MeanRows: x.rows / float64(len(p.rankings))}
	}
	rep.Lexical, rep.Semantic, rep.Hybrid = mk(a[0]), mk(a[1]), mk(a[2])
	return rep
}

/** String formats a report as a table: one line per mode.
 *
 * Example:
 *   fmt.Print(rep)
 */
func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "cutoff %.2f, fusion k %d\n", r.Params.MinScore, effectiveK(r.Params.FusionK))
	fmt.Fprintf(&b, "%-9s %8s %9s %6s %6s %6s %6s %9s\n", "mode", "recall@5", "recall@10", "MRR", "empty", "quiet", "noisy", "mean rows")
	for _, row := range []struct {
		name string
		r    Result
	}{{"lexical", r.Lexical}, {"semantic", r.Semantic}, {"hybrid", r.Hybrid}} {
		fmt.Fprintf(&b, "%-9s %8.3f %9.3f %6.3f %6d %6d %6d %9.1f\n", row.name, row.r.Recall5, row.r.Recall10, row.r.MRR,
			row.r.Empty, row.r.Quiet, row.r.Noisy, row.r.MeanRows)
	}
	return b.String()
}

func effectiveK(k int) int {
	if k <= 0 {
		return knowledge.DefaultFusionK
	}
	return k
}

/** Misses lists the answerable queries a mode did not find in its top ten, for
 * reading the failures.
 *
 * Parameters:
 *   mode (string) — "lexical", "semantic" or "hybrid"
 *
 * Returns:
 *   []string — "q06 rank 14 (31 rows)" per query, in id order
 *
 * Example:
 *   for _, m := range rep.Misses("semantic") { fmt.Println(m) }
 */
func (r Report) Misses(mode string) []string {
	var out []string
	for _, q := range r.PerQuery {
		if !q.Answerable {
			continue
		}
		rank, rows := q.LexRank, q.LexRows
		switch mode {
		case "semantic":
			rank, rows = q.SemRank, q.SemRows
		case "hybrid":
			rank, rows = q.HybRank, q.HybRows
		}
		if rank == 0 || rank > 10 {
			where := "not returned"
			if rank > 0 {
				where = fmt.Sprintf("first at rank %d", rank)
			}
			out = append(out, fmt.Sprintf("%s %s (%d rows)", q.ID, where, rows))
		}
	}
	sort.Strings(out)
	return out
}

/** AnyWord turns a query into an FTS5 term that matches a row containing any of
 * its words of four letters or more, each quoted, so a question typed as a
 * sentence finds rows by bm25 instead of requiring every word. It is a candidate
 * for the words half of a hybrid search, measured here before it is adopted.
 *
 * Parameters:
 *   q (string) — the query as typed
 *
 * Returns:
 *   string — the OR of the quoted words; the query itself when it has none that long
 *
 * Example:
 *   evalset.AnyWord("why did the model give empty replies") // "model" OR "give" OR "empty" OR "replies"
 */
func AnyWord(q string) string {
	var words []string
	for _, w := range strings.FieldsFunc(q, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_')
	}) {
		if len(w) >= 4 {
			words = append(words, `"`+w+`"`)
		}
	}
	if len(words) == 0 {
		return q
	}
	return strings.Join(words, " OR ")
}
