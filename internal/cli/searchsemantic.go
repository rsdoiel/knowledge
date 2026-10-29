package cli

import (
	"io"
	"strconv"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

// searchMode is how kb search ranks: by words (the default), by meaning, or both.
type searchMode string

const (
	searchLexical  searchMode = ""
	searchSemantic searchMode = "semantic"
	searchHybrid   searchMode = "hybrid"
)

// searchShown is the most rows a search prints.
const searchShown = 50

// semanticFetch is how many rows the search takes from the semantic ranking
// before showing the best searchShown of them: more than any knowledge base
// has, so the total and the counts per kind are of everything above the cutoff,
// and a hybrid search does not lose a row ranked low by meaning but high by words.
const semanticFetch = 1 << 20

/** parseSearchMode takes the search-mode flags off the front of kb search's
 * arguments: -semantic, -hybrid and -min-score VALUE, with one or two dashes.
 * Only leading flags are read; the first word that is not one of them, and
 * everything after it, is the scope and the term (DR-0041), so a term that
 * contains "-semantic" is searched for as written.
 *
 * Parameters:
 *   args ([]string) — the arguments after the verb
 *
 * Returns:
 *   searchMode — lexical unless -semantic or -hybrid was given
 *   *float32 — the -min-score value, nil when it was not given
 *   []string — the arguments left, the scope and the term
 *   error — a usage error for both modes together, a missing or unusable
 *     -min-score value, or -min-score without a mode; flag.ErrHelp for -h
 *
 * Example:
 *   mode, min, rest, err := parseSearchMode([]string{"-hybrid", "-min-score", "0.4", "p/", "slow"})
 */
func parseSearchMode(args []string) (searchMode, *float32, []string, error) {
	mode := searchLexical
	var min *float32
	set := func(m searchMode) error {
		if mode != searchLexical && mode != m {
			return usageErrorf("-semantic and -hybrid cannot be combined; use one")
		}
		mode = m
		return nil
	}
	i := 0
loop:
	for ; i < len(args); i++ {
		switch strings.TrimPrefix(strings.TrimPrefix(args[i], "-"), "-") {
		case "semantic":
			if !strings.HasPrefix(args[i], "-") {
				break loop
			}
			if err := set(searchSemantic); err != nil {
				return mode, nil, nil, err
			}
		case "hybrid":
			if !strings.HasPrefix(args[i], "-") {
				break loop
			}
			if err := set(searchHybrid); err != nil {
				return mode, nil, nil, err
			}
		case "min-score":
			if !strings.HasPrefix(args[i], "-") {
				break loop
			}
			if i+1 >= len(args) {
				return mode, nil, nil, usageErrorf("%s requires a value, from -1 to 1", args[i])
			}
			f, err := strconv.ParseFloat(args[i+1], 32)
			if err != nil || f < knowledge.NoCutoff || f > 1 {
				return mode, nil, nil, usageErrorf("%s %q is not a number from -1 to 1 (-1 keeps every row)", args[i], args[i+1])
			}
			v := float32(f)
			min = &v
			i++
		default:
			break loop
		}
	}
	rest := args[i:]
	if min != nil && mode == searchLexical {
		return mode, nil, nil, usageErrorf("-min-score applies to -semantic and -hybrid searches")
	}
	return mode, min, rest, nil
}

// meaningHit is one row of a semantic or hybrid search: the row as a search
// result shows it, and what the ranking says about it.
type meaningHit struct {
	R     knowledge.KBSearchResult
	Score *float64 // the best chunk's cosine similarity; nil for a row only the words found
	Chunk string   // that chunk's text
	Via   string   // for -hybrid, which ranking found the row: both, words or meaning
}

// meaningResult is what meaningSearch found: the best searchShown rows, and the
// totals of everything above the cutoff.
type meaningResult struct {
	Hits      []meaningHit
	Total     int
	Counts    map[string]int // per search kind (records, observations, ...)
	Cutoff    float64
	Unindexed int // rows in scope with no chunks yet, which the search could not see
}

/** meaningSearch ranks the knowledge base by meaning (searchSemantic) or by
 * meaning and words together (searchHybrid), for kb search and for the TUI, so
 * the two cannot disagree. -semantic ranks the rows by how close their best
 * chunk is to the query; -hybrid fuses that with the lexical ranking (a row both
 * found comes first). Only the embedded kinds are searched. It prints nothing.
 *
 * Parameters:
 *   kb (*knowledge.KnowledgeBase) — the knowledge base
 *   dl (*DebugLog) — the debug log, may be nil
 *   mode (searchMode) — searchSemantic or searchHybrid
 *   minScore (*float32) — the cutoff, nil for the default
 *   scope (knowledge.SearchScope) — where to look
 *   scopeName (string) — what to call the scope in the debug log
 *   term (string) — plain text for -semantic; FTS5 in the lexical half of -hybrid
 *
 * Returns:
 *   *meaningResult — the hits, empty (Total 0) when nothing reaches the cutoff
 *   error — a build without the feature or a kind that is not embedded (usage,
 *     exit 2), a missing model or index (66), an index built by another model
 *     (65), an index being built (75), or a search failure
 *
 * Example:
 *   res, err := meaningSearch(kb, nil, searchHybrid, nil, knowledge.SearchScope{}, "everywhere", "slow inference")
 */
func meaningSearch(kb *knowledge.KnowledgeBase, dl *DebugLog, mode searchMode, minScore *float32,
	scope knowledge.SearchScope, scopeName, term string) (*meaningResult, error) {
	emb, ok := knowledge.RegisteredEmbedder()
	if !ok {
		return nil, usageErrorf("this build of kb has no semantic search, so -%s is not available; use the semantic build of kb", mode)
	}
	for _, k := range scope.Kinds {
		if !containsKind(knowledge.EmbeddedKinds, k) {
			return nil, usageErrorf("%s is not embedded, so it cannot be searched by meaning; the kinds that are: %s",
				k, strings.Join(knowledge.EmbeddedKinds, ", "))
		}
	}
	if c, ok := emb.(knowledge.Checker); ok {
		if err := c.Available(); err != nil {
			return nil, err
		}
	}
	v, err := knowledge.OpenVectors(kb.Path())
	if err != nil {
		return nil, err
	}
	defer v.Close()
	if busy, err := v.Building(); err != nil {
		return nil, err
	} else if busy {
		return nil, classErrorf(classTempFail, "the semantic index is being built by kb embed; search again when it has finished")
	}

	opts := knowledge.SemanticOptions{Scope: scope, Limit: semanticFetch}
	cutoff := float64(knowledge.DefaultMinScore)
	if minScore != nil {
		opts.MinScore, cutoff = *minScore, float64(*minScore)
		if *minScore == 0 {
			opts.MinScore = 1e-9 // 0 means "the default" to the library; a typed 0 is a cutoff of zero
		}
	}
	sr, err := logKBCall(dl, "SemanticSearch", map[string]any{"term": term, "scope": scopeName, "mode": string(mode)},
		func() (*knowledge.SemanticResult, error) { return kb.SemanticSearch(v, emb, term, opts) })
	if err != nil {
		return nil, err
	}
	chunkOf := map[knowledge.RowKey]knowledge.SemanticHit{}
	var semKeys []knowledge.RowKey
	for _, h := range sr.Hits {
		k := knowledge.RowKey{SourceType: h.SourceType, SourceID: h.SourceID}
		chunkOf[k] = h
		semKeys = append(semKeys, k)
	}

	// order is the rows to show, best first, and via says which ranking found each.
	order := semKeys
	via := map[knowledge.RowKey]string{}
	if mode == searchHybrid {
		lexKeys, err := logKBCall(dl, "LexicalRanking", map[string]any{"term": term}, func() ([]knowledge.RowKey, error) {
			return kb.LexicalRanking(term, scope)
		})
		if err != nil {
			return nil, err
		}
		order = nil
		for _, f := range knowledge.FuseRankings(lexKeys, semKeys, 0) {
			k := knowledge.RowKey{SourceType: f.SourceType, SourceID: f.SourceID}
			order = append(order, k)
			switch {
			case f.InBoth():
				via[k] = "both"
			case f.LexicalRank > 0:
				via[k] = "words"
			default:
				via[k] = "meaning"
			}
		}
	}
	res := &meaningResult{Total: len(order), Counts: map[string]int{}, Cutoff: cutoff, Unindexed: sr.Unindexed}
	kindOf := map[string]string{}
	for k, st := range searchTypeOf {
		kindOf[st] = k
	}
	for _, k := range order {
		res.Counts[kindOf[k.SourceType]]++
	}
	if len(order) > searchShown {
		order = order[:searchShown]
	}
	rows, err := kb.HitsFor(order)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		k := knowledge.RowKey{SourceType: r.SourceType, SourceID: r.SourceID}
		h := meaningHit{R: r, Via: via[k]}
		if sh, ok := chunkOf[k]; ok {
			score := float64(sh.Score)
			h.Score, h.Chunk = &score, sh.Chunk
		}
		res.Hits = append(res.Hits, h)
	}
	return res, nil
}

/** searchMeaning runs kb search -semantic or -hybrid and prints the result
 * grouped by kind with counts, or as JSON. Finding nothing is exit 1; the other
 * exit classes are meaningSearch's.
 *
 * Parameters:
 *   kb (*knowledge.KnowledgeBase) — the knowledge base
 *   dl (*DebugLog) — the debug log, may be nil
 *   jsonOut (bool) — print JSON
 *   mode (searchMode) — searchSemantic or searchHybrid
 *   minScore (*float32) — the cutoff, nil for the default
 *   scope (knowledge.SearchScope) — where to look
 *   scopeName (string) — what to call the scope in output
 *   term (string) — plain text for -semantic; FTS5 in the lexical half of -hybrid
 *   out (io.Writer) — where results go
 *
 * Returns:
 *   error — as above; nil when there are results
 *
 * Example:
 *   err := searchMeaning(kb, nil, false, searchHybrid, nil, knowledge.SearchScope{}, "everywhere", "slow inference", os.Stdout)
 */
func searchMeaning(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, mode searchMode, minScore *float32,
	scope knowledge.SearchScope, scopeName, term string, out io.Writer) error {
	res, err := meaningSearch(kb, dl, mode, minScore, scope, scopeName, term)
	if err != nil {
		return err
	}
	if res.Unindexed > 0 {
		advise("%d rows are not yet indexed for semantic search and cannot be found by it; run kb embed", res.Unindexed)
	}
	if res.Total == 0 {
		return negativef("no results for %q in %s by %s (cutoff %.2f)", term, scopeName, mode, res.Cutoff)
	}
	var hits []scopedHit
	for _, mh := range res.Hits {
		r := mh.R
		h := scopedHit{Kind: r.Kind, Ref: r.Ref, Label: r.Label, Snippet: r.Snippet, Status: r.Status,
			Type: r.SourceType, ID: r.SourceID, ProjectID: r.ProjectID, Via: mh.Via, Score: mh.Score, Chunk: mh.Chunk,
			Sources: jsonSources(r.HitProvenance), Material: jsonMaterialOf(r.HitProvenance), Retracted: r.Retracted}
		h.marker = retractionMarker(r.HitProvenance)
		hits = append(hits, h)
	}
	return printScoped(out, jsonOut, searchPrint{scope: scope, scopeName: scopeName, term: term, mode: string(mode),
		minScore: &res.Cutoff, total: res.Total, counts: res.Counts}, hits)
}

func containsKind(kinds []string, k string) bool {
	for _, x := range kinds {
		if x == k {
			return true
		}
	}
	return false
}
