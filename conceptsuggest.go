package knowledge

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// Moved from cmd/kb/concept.go (library-lift-plan.md L3, DR-0035). The
// scoring and tokenization bodies are a verbatim move, except that
// tokenizeCandidateItems now takes its tokens from CandidateTerms so there is
// one definition of what counts as a candidate word. SuggestConcepts is the
// corpus-gathering half that used to sit inline in cmdConceptSuggest.

/** ConceptCandidate is one suggested new concept: a term ranked by corpus-wide
 * distinctiveness (TF-IDF shaped), not raw frequency. It is also the entry
 * shape of `kb concept suggest --json`, so its tags are a compatibility
 * contract.
 *
 * Fields:
 *   Term        (string)   — the candidate's canonical spelling: the
 *                            highest-occurrence member of its spelling cluster.
 *   Occurrences (int)      — mentions across the corpus, summed over variants.
 *   Items       (int)      — how many distinct items (record bodies, document
 *                            sections) mention it, unioned over variants.
 *   Score       (float64)  — Occurrences times the inverse item frequency.
 *   Variants    ([]string) — spelling variants merged into this candidate;
 *                            empty (and omitted from JSON) for a singleton.
 *
 * Example:
 *   // {Term: "rename", Occurrences: 41, Items: 8, Score: 70.8, Variants: ["renamed", "renaming"]}
 */
type ConceptCandidate struct {
	Term        string   `json:"term"`
	Occurrences int      `json:"occurrences"`
	Items       int      `json:"items"`
	Score       float64  `json:"score"`
	Variants    []string `json:"variants,omitempty"`
}

/** ConceptSuggestions is what SuggestConcepts returns, and the JSON shape of
 * `kb concept suggest --json` (a bare array before v0.0.11).
 *
 * Fields:
 *   Candidates   ([]ConceptCandidate) — suggested new concepts, best first;
 *                                       nil when none.
 *   NearExisting ([]NearExistingMatch) — terms excluded from candidacy because
 *                                       they are a close spelling of an
 *                                       existing concept (fuzzy-tag's job).
 *
 * Example:
 *   s, _ := kb.SuggestConcepts("harvey", 20)
 *   for _, c := range s.Candidates { fmt.Println(c.Term, c.Score) }
 */
type ConceptSuggestions struct {
	Candidates   []ConceptCandidate  `json:"candidates"`
	NearExisting []NearExistingMatch `json:"near_existing"`
}

// candidateTermPattern extracts single-word candidate terms: a letter
// followed by two or more letters/digits/hyphens/underscores, i.e. a
// minimum length of 3 -- short tokens are almost never a usable concept
// name and are pure noise at corpus scale.
var candidateTermPattern = regexp.MustCompile(`[a-z][a-z0-9_-]{2,}`)

// recordIDShapedPattern matches a bare record reference like dr-0013 or
// adr-0004: a letters-only prefix, a hyphen, then digits only. It is
// filtered outright rather than scored -- a record reference is a
// legitimate statistical signal (distinctive, often repeated) but never a
// usable concept name, so leaving it in would put the same kind of noise
// in every single run's output, not just an occasional false positive.
var recordIDShapedPattern = regexp.MustCompile(`^[a-z]+-[0-9]+$`)

// suggestStopwords is a small built-in list of common English function
// words, excluded from candidacy outright rather than left to the
// distinctiveness score alone -- at corpus scale they are frequent enough
// in nearly every item that idf would usually zero them anyway, but a
// smaller or lopsided corpus could let one slip through, and there is no
// reason to spend a candidate slot on "with" or "about".
var suggestStopwords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`the and for that this with from into
		through during before after above below under again further then
		once here there when where why how all any both each few more most
		other some such nor not only own same than too very just but not
		are was were been being have has had does did doing will would
		shall should may might must can could you your yours they them
		their theirs she her hers him his its our ours who whom which what
		about over out off down up`) {
		suggestStopwords[w] = true
	}
}

/** CandidateTerms returns the lowercased single-word candidate terms in text,
 * in order of appearance with repeats kept. Fenced code blocks and inline code
 * spans are stripped first (StripCodeSpans), then English stopwords and bare
 * record references (dr-0013, adr-0004) are dropped. Filtering out terms that
 * are already known concepts is left to the caller. This is the one
 * definition of "a candidate word" that concept suggestion and the
 * frontmatter keyword scorer share.
 *
 * Parameters:
 *   text (string) — free text, e.g. a document section body.
 *
 * Returns:
 *   []string — the surviving terms, each at least three characters long.
 *
 * Example:
 *   knowledge.CandidateTerms("The chunking and `code` again") // ["chunking", "again"]
 */
func CandidateTerms(text string) []string {
	var out []string
	for _, tok := range candidateTermPattern.FindAllString(strings.ToLower(StripCodeSpans(text)), -1) {
		if suggestStopwords[tok] || recordIDShapedPattern.MatchString(tok) {
			continue
		}
		out = append(out, tok)
	}
	return out
}

// scoreCandidateTerms scores single-word candidate terms across a corpus
// of items (each item's raw text -- a record body or document section
// body), for `kb concept suggest`. A term is a candidate for a new
// concept when it is distinctive: mentioned several times overall but
// confined to relatively few items, the classic TF-IDF shape, rather than
// spread evenly across nearly every item (idf collapses to zero, filtered
// out as not distinctive) or mentioned only once anywhere in the whole
// corpus (below the >1 occurrence floor -- the same threshold DR-0027's
// density-linking uses, for the same reason: a single incidental mention
// is too weak a signal on its own).
//
// known is the set of already-existing concept names, lowercased --
// suggesting one again wastes a candidate slot. Code spans and fenced code
// blocks are stripped before matching (StripCodeSpans, shared with
// document ingest's density-linking, DR-0027), for the same reason: a
// short, common word colliding with a word used in a different sense
// inside quoted code is exactly the false-positive shape found there.
//
// Returned sorted by score descending, term ascending on a tie, for
// deterministic output; nil for an empty corpus or when nothing survives
// the filters.
func scoreCandidateTerms(items []string, known map[string]bool) (candidates []ConceptCandidate, nearExisting []NearExistingMatch) {
	if len(items) == 0 {
		return nil, nil
	}
	occurrences, itemCounts := tokenizeCandidateItems(items, known)

	// v0.0.11 item 5: a token fuzzy-close to an already-known concept is
	// excluded from candidacy entirely (fuzzy-tag's job, not this
	// command's), and clustering runs on the raw, pre-filter occurrence
	// map -- a variant individually below the occ<2/idf<=0 floor only
	// survives merged into a cluster, not scored on its own.
	survivors, nearExisting := excludeNearExisting(occurrences, known)
	clusters := clusterCandidateTerms(occurrences, itemCounts, survivors)

	n := float64(len(items))
	var out []ConceptCandidate
	for _, c := range clusters {
		if c.Occurrences < 2 {
			continue
		}
		df := len(c.Items)
		idf := math.Log(n / float64(df))
		if idf <= 0 {
			continue
		}
		out = append(out, ConceptCandidate{
			Term: c.Seed, Variants: c.Variants,
			Occurrences: c.Occurrences, Items: df, Score: float64(c.Occurrences) * idf,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Term < out[j].Term
	})
	return out, nearExisting
}

// tokenizeCandidateItems is scoreCandidateTerms's shared tokenization pass,
// factored out so fuzzy clustering (FC4) can build its own occurrence/
// item-index maps identically before scoreCandidateTerms's filter loop
// runs. itemCounts is a set of item indices per term (FC2, design decision
// 7), not a bare count -- needed so a cluster's df can be computed as the
// size of a *union* across members, not a sum; two mentions of the same
// term within one item still count once toward that term's df.
func tokenizeCandidateItems(items []string, known map[string]bool) (occurrences map[string]int, itemCounts map[string]map[int]bool) {
	occurrences = map[string]int{}
	itemCounts = map[string]map[int]bool{}
	for i, item := range items {
		seenInItem := map[string]bool{}
		for _, tok := range CandidateTerms(item) {
			if known[tok] {
				continue
			}
			occurrences[tok]++
			if !seenInItem[tok] {
				seenInItem[tok] = true
				if itemCounts[tok] == nil {
					itemCounts[tok] = map[int]bool{}
				}
				itemCounts[tok][i] = true
			}
		}
	}
	return occurrences, itemCounts
}

/** SuggestConcepts is a read-only scan of every record body and every
 * document section body (gist rows are ignored), optionally scoped to one
 * project, that returns candidate new concepts ranked by corpus-wide
 * distinctiveness. It never writes anything: a suggestion becomes a real
 * concept only when a human adds it (`kb concept add`), the same mechanical-
 * signal-then-human-curates pattern density-linking and document summary
 * review already use. Spelling variants of one term are merged before scoring,
 * and a term that is a near-miss of an already-known concept is excluded from
 * candidacy and reported in NearExisting instead.
 *
 * Parameters:
 *   project (string) — a project name to scope the scan to, or "" for the
 *                      whole knowledge base.
 *   limit   (int)    — cap on Candidates; zero or negative means no cap.
 *
 * Returns:
 *   ConceptSuggestions — the candidates (best first) and the near-existing
 *                        pairs. NearExisting is not capped by limit.
 *   error              — for an unknown project, or on database failure.
 *
 * Example:
 *   s, err := kb.SuggestConcepts("harvey", 20)
 */
func (kb *KnowledgeBase) SuggestConcepts(project string, limit int) (ConceptSuggestions, error) {
	var projectID int64
	if project != "" {
		p, err := kb.ProjectByName(project)
		if err != nil {
			return ConceptSuggestions{}, err
		}
		if p == nil {
			return ConceptSuggestions{}, notFoundf("unknown project %q", project)
		}
		projectID = p.ID
	}

	concepts, err := kb.Concepts()
	if err != nil {
		return ConceptSuggestions{}, err
	}
	known := map[string]bool{}
	for _, c := range concepts {
		known[strings.ToLower(c.Name)] = true
	}

	records, err := kb.ListRecords(RecordFilter{Project: project})
	if err != nil {
		return ConceptSuggestions{}, err
	}
	var items []string
	for _, r := range records {
		items = append(items, r.Body)
	}

	docs, err := kb.Documents(projectID)
	if err != nil {
		return ConceptSuggestions{}, err
	}
	for _, d := range docs {
		sections, err := kb.DocumentSections(d.ID)
		if err != nil {
			return ConceptSuggestions{}, err
		}
		for _, s := range sections {
			if s.Level == "section" {
				items = append(items, s.Body)
			}
		}
	}

	candidates, nearExisting := scoreCandidateTerms(items, known)
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return ConceptSuggestions{Candidates: candidates, NearExisting: nearExisting}, nil
}
