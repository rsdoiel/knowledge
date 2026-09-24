package knowledge

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

/** ConceptMatch is one entity (an observation, a record, or a document
 * gist/section) linked to one or more concepts a RecallByConceptNames
 * caller asked about.
 *
 * Fields:
 *   SourceType    (string)    — "observation", "record", "document_gist", or
 *                               "document_section" -- the same vocabulary
 *                               kb_fts.source_type uses.
 *   ID            (int64)     — internal database id of the entity (for a
 *                               document match, the document_sections row).
 *   ProjectID     (int64)     — owning project id (0 for a workspace-tier record).
 *   Title         (string)    — "" for an observation; the record's or
 *                               document's title otherwise.
 *   Body          (string)    — the entity's text -- for a document match,
 *                               only when SummaryStatus == "reviewed" (see
 *                               narrative-documents-design.md decision 8):
 *                               an unreviewed summary is findable by tag,
 *                               but never surfaced as trustworthy content.
 *   SummaryStatus (string)    — "" for observation/record matches;
 *                               "unsummarized"/"drafted"/"reviewed" for a
 *                               document match, explaining why Body may be
 *                               empty rather than leaving the caller to guess.
 *   MatchCount    (int)       — how many of the queried concepts this entity is linked to.
 *   CreatedAt     (time.Time) — Observation.CreatedAt, Record.IngestedAt, or
 *                               the document section's CreatedAt.
 */
type ConceptMatch struct {
	SourceType    string
	ID            int64
	ProjectID     int64
	Title         string
	Body          string
	SummaryStatus string
	MatchCount    int
	CreatedAt     time.Time
}

// conceptIDsByNames resolves names to existing concept ids, case-insensitively,
// read-only: a name matching no concept is silently skipped, and no concept is
// ever created as a side effect (unlike ResolveConceptName, which is for
// ingest, not retrieval). Resolved ids are deduped, since two case-variant
// input names can resolve to the same concept.
func conceptIDsByNames(kb *KnowledgeBase, names []string) ([]int64, error) {
	seen := map[int64]bool{}
	var ids []int64
	for _, name := range names {
		var id int64
		err := kb.db.QueryRow(
			`SELECT id FROM concepts WHERE name = ?1 COLLATE NOCASE LIMIT 1`, name,
		).Scan(&id)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("knowledge: resolve concept name %q: %w", name, err)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

/** RecallByConceptNames returns observations and records linked to any of the
 * given concept names, merged into one list ranked by how many of the
 * queried concepts each entity matches (descending), then by recency
 * (descending), truncated to limit.
 *
 * A name matching no concept is silently skipped rather than erroring — a
 * prompt mentioning nothing tagged is a normal outcome, not a failure — and
 * no concept is ever created as a side effect of calling this (unlike
 * ResolveConceptName, used at ingest time). The query is not scoped to any
 * one project: ProjectID is reported on every result so a caller can filter
 * or weight afterward.
 *
 * Parameters:
 *   names ([]string) — concept names to match against, as returned by
 *                       MatchConceptNames or supplied directly.
 *   limit (int)       — maximum results to return; non-positive means no cap.
 *
 * Returns:
 *   []ConceptMatch — ranked matches; nil when none.
 *   error          — on database failure.
 *
 * Example:
 *   matches, err := kb.RecallByConceptNames([]string{"chunking", "RAG"}, 5)
 */
func (kb *KnowledgeBase) RecallByConceptNames(names []string, limit int) ([]ConceptMatch, error) {
	ids, err := conceptIDsByNames(kb, names)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	var matches []ConceptMatch

	obsRows, err := kb.db.Query(fmt.Sprintf(`
		SELECT o.id, o.project_id, o.body, o.created_at, COUNT(DISTINCT oc.concept_id) AS match_count
		FROM observations o
		JOIN observation_concepts oc ON oc.observation_id = o.id
		WHERE oc.concept_id IN (%s)
		GROUP BY o.id`, placeholders), args...)
	if err != nil {
		return nil, fmt.Errorf("knowledge: recall observations by concept: %w", err)
	}
	defer obsRows.Close()
	for obsRows.Next() {
		var m ConceptMatch
		var ts string
		if err := obsRows.Scan(&m.ID, &m.ProjectID, &m.Body, &ts, &m.MatchCount); err != nil {
			return nil, err
		}
		m.SourceType = "observation"
		m.CreatedAt = parseTimestamp(ts)
		matches = append(matches, m)
	}
	if err := obsRows.Err(); err != nil {
		return nil, err
	}

	recRows, err := kb.db.Query(fmt.Sprintf(`
		SELECT r.id, r.project_id, r.title, r.body, r.ingested_at, COUNT(DISTINCT rc.concept_id) AS match_count
		FROM records r
		JOIN record_concepts rc ON rc.record_id = r.id
		WHERE rc.concept_id IN (%s)
		GROUP BY r.id`, placeholders), args...)
	if err != nil {
		return nil, fmt.Errorf("knowledge: recall records by concept: %w", err)
	}
	defer recRows.Close()
	for recRows.Next() {
		var m ConceptMatch
		var ts string
		if err := recRows.Scan(&m.ID, &m.ProjectID, &m.Title, &m.Body, &ts, &m.MatchCount); err != nil {
			return nil, err
		}
		m.SourceType = "record"
		m.CreatedAt = parseTimestamp(ts)
		matches = append(matches, m)
	}
	if err := recRows.Err(); err != nil {
		return nil, err
	}

	docRows, err := kb.db.Query(fmt.Sprintf(`
		SELECT s.id, IFNULL(d.project_id, 0), d.title, s.level, s.heading, s.summary_body, s.summary_status,
		       s.created_at, COUNT(DISTINCT dsc.concept_id) AS match_count
		FROM document_sections s
		JOIN documents d ON d.id = s.document_id
		JOIN document_section_concepts dsc ON dsc.section_id = s.id
		WHERE dsc.concept_id IN (%s)
		GROUP BY s.id`, placeholders), args...)
	if err != nil {
		return nil, fmt.Errorf("knowledge: recall document sections by concept: %w", err)
	}
	defer docRows.Close()
	for docRows.Next() {
		var m ConceptMatch
		var level, heading, summaryBody, ts string
		if err := docRows.Scan(&m.ID, &m.ProjectID, &m.Title, &level, &heading, &summaryBody,
			&m.SummaryStatus, &ts, &m.MatchCount); err != nil {
			return nil, err
		}
		if level == "gist" {
			m.SourceType = "document_gist"
		} else {
			m.SourceType = "document_section"
			if heading != "" {
				m.Title = m.Title + " / " + heading
			}
		}
		// Design decision 8: only a reviewed summary is trustworthy content.
		// An unsummarized/drafted match still surfaces (findable by tag),
		// with SummaryStatus explaining why Body is empty.
		if m.SummaryStatus == "reviewed" {
			m.Body = summaryBody
		}
		m.CreatedAt = parseTimestamp(ts)
		matches = append(matches, m)
	}
	if err := docRows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].MatchCount != matches[j].MatchCount {
			return matches[i].MatchCount > matches[j].MatchCount
		}
		return matches[i].CreatedAt.After(matches[j].CreatedAt)
	})
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

/** MatchConceptNames returns every known concept name that appears in text as
 * a whole word, case-insensitively, including a name that begins or ends in
 * punctuation such as C++ or .NET (see conceptNameMatches). Whole-word matching (not substring) is
 * deliberate: a short concept name like "RAG" must not match inside an
 * unrelated word like "storage". Concept names are otherwise free-form, so
 * this compiles one small regex per concept per call rather than assuming
 * any structure to exploit — call frequency here is "once per prompt," not a
 * hot loop.
 *
 * Parameters:
 *   text (string) — free text to search, e.g. a prompt.
 *
 * Returns:
 *   []string — matched concept names, in Concepts()'s own order; nil when none.
 *   error    — on database failure.
 *
 * Example:
 *   names, err := kb.MatchConceptNames("how does chunking interact with RAG?")
 */
func (kb *KnowledgeBase) MatchConceptNames(text string) ([]string, error) {
	concepts, err := kb.Concepts()
	if err != nil {
		return nil, err
	}
	var matched []string
	for _, c := range concepts {
		if len(conceptNameMatches(text, c.Name)) > 0 {
			matched = append(matched, c.Name)
		}
	}
	return matched, nil
}

// hasLetterOrDigit reports whether s contains at least one letter or digit. A
// concept name without one ("...", "+") is not a word or a phrase and can never
// be a meaningful mention; conceptNameMatches refuses it, since with
// punctuation-edged names supported it would otherwise match every ellipsis.
func hasLetterOrDigit(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// isASCIIWordByte reports whether b is an ASCII word character, [0-9A-Za-z_]:
// the class Go's \b and \w use, kept so that a name made of letters behaves
// exactly as it did under `\b` + name + `\b`.
func isASCIIWordByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

/** conceptNameMatches returns the [start, end) byte offsets of every
 * case-insensitive, whole-word mention of name in text, left to right and
 * non-overlapping. A mention counts when it is not glued to a word character
 * on either side. For a name that starts and ends with a letter or digit that
 * is exactly what `\b` + name + `\b` meant, so those names behave as before.
 * It also holds for a name that starts or ends with punctuation (C++, F#,
 * .NET), which `\b` could not handle: \b only fires between a word character
 * and a non-word one, so the space after C++ was never a boundary, and .NET
 * matched inside ASP.NET. Go's regexp has no lookbehind, so the neighbours
 * are checked by hand. A candidate rejected for its neighbour does not hide
 * an overlapping one that starts a character later.
 *
 * Parameters:
 *   text (string) — the text to search.
 *   name (string) — a concept name, taken literally.
 *
 * Returns:
 *   [][]int — {start, end} pairs; nil for a name with no letter or digit, or no mention.
 *
 * Example:
 *   conceptNameMatches("using C++ today", "C++") // [[6 9]]
 */
func conceptNameMatches(text, name string) [][]int {
	if !hasLetterOrDigit(name) {
		return nil
	}
	re, err := regexp.Compile(`(?i)` + regexp.QuoteMeta(name))
	if err != nil {
		return nil
	}
	var out [][]int
	pos := 0
	for pos <= len(text) {
		loc := re.FindStringIndex(text[pos:])
		if loc == nil {
			break
		}
		start, end := pos+loc[0], pos+loc[1]
		glued := (start > 0 && isASCIIWordByte(text[start-1])) || (end < len(text) && isASCIIWordByte(text[end]))
		if glued {
			_, size := utf8.DecodeRuneInString(text[start:])
			pos = start + size
			continue
		}
		out = append(out, []int{start, end})
		pos = end
		if end == start {
			pos++
		}
	}
	return out
}

/** MatchConceptNameCounts is MatchConceptNames with an occurrence count
 * instead of a presence flag, for a caller that needs to threshold on
 * frequency rather than just detect a mention — document ingest's
 * density-based concept linking (TODO.md's MADR item) is the motivating
 * caller: a single incidental mention is too weak a signal to auto-link,
 * but several are not.
 *
 * Parameters:
 *   text (string) — free text to search, e.g. a document section's body.
 *
 * Returns:
 *   map[string]int — concept name to occurrence count; a concept with zero
 *                     occurrences is absent from the map, not present at 0.
 *   error          — on database failure.
 *
 * Example:
 *   counts, err := kb.MatchConceptNameCounts(sectionBody)
 *   if counts["RAG"] > 1 { fmt.Println("link it") }
 */
func (kb *KnowledgeBase) MatchConceptNameCounts(text string) (map[string]int, error) {
	concepts, err := kb.Concepts()
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, c := range concepts {
		if n := len(conceptNameMatches(text, c.Name)); n > 0 {
			counts[c.Name] = n
		}
	}
	return counts, nil
}

// maxFuzzyDistance is FuzzyMatchConceptNames's generous candidate-generation
// ceiling (fuzzy-concept-matching-design.md decision 5) -- not the real
// eligibility threshold, which is cmd/kb's tighter, concept-name-length-based
// job to apply (mirroring MatchConceptNameCounts -> densityLinkCandidates's
// own split between raw signal and applied policy).
const maxFuzzyDistance = 3

// fuzzyTokenPattern tokenizes on word-boundary runs of letters, digits, and
// apostrophes (e.g. "don't" stays one token), used once per
// FuzzyMatchConceptNames call rather than per concept.
var fuzzyTokenPattern = regexp.MustCompile(`\b[\p{L}\p{N}']+\b`)

// wholeGapPattern matches a run of nothing but whitespace, used to decide
// whether two adjacent tokens are "contiguous" for multi-word concept
// matching -- a gap containing anything else (punctuation, a sentence
// boundary) means the tokens are not really adjacent in the sense a
// multi-word concept name requires.
var wholeGapPattern = regexp.MustCompile(`^\s*$`)

/** LevenshteinDistance is the classic edit distance between a and b,
 * computed over runes (not bytes) since concept names and document prose
 * aren't guaranteed ASCII. Two-row iterative DP. Exported so cmd/kb's
 * fuzzy clustering (`kb concept suggest`, v0.0.11 item 5) can share this
 * exact implementation with FuzzyMatchConceptNames (`kb document
 * fuzzy-tag`), rather than a second copy of the same algorithm — design
 * decision 1 of fuzzy-concept-clustering-design.md.
 *
 * Parameters:
 *   a (string) — first string.
 *   b (string) — second string.
 *
 * Returns:
 *   int — the edit distance between a and b.
 *
 * Example:
 *   d := knowledge.LevenshteinDistance("chunking", "chunkings")
 *   fmt.Println(d) // 1
 */
func LevenshteinDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(rb)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// fuzzySuffixes is tried longest-first (fuzzy-concept-matching-design.md
// decision 5): a plain plural/tense-suffixed word strips its most specific
// matching ending, not just any one that happens to fit ("boxes" strips
// "es", not "s", leaving "box" rather than "boxe").
var fuzzySuffixes = []string{"ing", "es", "ed", "s"}

/** StripCommonSuffix lowercases s and strips exactly one trailing suffix
 * from a fixed list (`ing`/`es`/`ed`/`s`, longest match first) --
 * deliberately crude, not a real stemmer: a single strip, never iterative,
 * and returns s unchanged if none of the fixed suffixes match. Exported
 * for the same reason as LevenshteinDistance: shared verbatim between
 * `kb document fuzzy-tag` and `kb concept suggest`'s fuzzy clustering.
 *
 * Parameters:
 *   s (string) — the word to normalize.
 *
 * Returns:
 *   string — s with its longest matching suffix stripped, or unchanged.
 *
 * Example:
 *   knowledge.StripCommonSuffix("chunkings") // "chunking"
 */
func StripCommonSuffix(s string) string {
	s = strings.ToLower(s)
	for _, suf := range fuzzySuffixes {
		if strings.HasSuffix(s, suf) {
			return s[:len(s)-len(suf)]
		}
	}
	return s
}

// fuzzyToken is one tokenized word from FuzzyMatchConceptNames's input text.
type fuzzyToken struct {
	Text       string
	Start, End int
}

// fuzzyDistance compares concept against candidate (both lowercased by the
// caller) and reports the distance to record plus whether it qualifies as a
// candidate at all under maxFuzzyDistance.
//
// Raw Levenshtein distance is tried first -- alone, it already reproduces
// fuzzy-concept-matching-design.md's own worked examples exactly (a plural
// or single-character typo lands on distance 1 with no normalization at
// all). Stemming candidate is used only as a fallback, when raw distance
// overshoots the ceiling -- e.g. a short concept name against a longer
// suffixed token ("chunk" vs "chunkings"). Stemming *both* sides, as an
// earlier draft of the design described, was tried and rejected: it breaks
// the plural/typo cases outright (verified: it turns "chunking" itself into
// "chunk" via its own trailing "ing", which no longer resembles the token
// being compared against).
func fuzzyDistance(concept, candidate string) (distance int, ok bool) {
	raw := LevenshteinDistance(concept, candidate)
	if raw > 0 && raw <= maxFuzzyDistance {
		return raw, true
	}
	stemmed := LevenshteinDistance(concept, StripCommonSuffix(candidate))
	if stemmed <= maxFuzzyDistance {
		return stemmed, true
	}
	return 0, false
}

/** FuzzyConceptMatch is one spelling-level near-miss FuzzyMatchConceptNames
 * found in a piece of text: a typo, plural, or simple tense variant of a
 * known concept's name, not an exact match (MatchConceptNames already finds
 * those).
 *
 * Fields:
 *   Concept  (string) — canonical concept name, Concepts()'s own casing.
 *   Text     (string) — the actual text found, original casing preserved.
 *   Start    (int)    — byte offset into the input text where Text begins.
 *   End      (int)    — byte offset where Text ends (half-open range).
 *   Distance (int)    — Levenshtein distance recorded for this match (raw,
 *                        or the stemmed fallback distance when raw
 *                        overshot the candidate-generation ceiling).
 *
 * Example:
 *   matches, _ := kb.FuzzyMatchConceptNames("the chunkings here")
 *   fmt.Println(matches[0].Concept, matches[0].Distance) // "chunking" 1
 */
type FuzzyConceptMatch struct {
	Concept  string
	Text     string
	Start    int
	End      int
	Distance int
}

/** FuzzyMatchConceptNames returns spelling-level near-misses of known
 * concept names in text -- typos, plurals, and simple tense variants that
 * MatchConceptNames's exact whole-word matching would find nothing for.
 * Scoped deliberately to spelling distance, not paraphrase: a concept
 * already found as an exact match anywhere in text is skipped entirely
 * (fuzzy matching is additive, never a duplicate of what exact matching
 * already covers).
 *
 * Parameters:
 *   text (string) — free text to search, e.g. a document's raw content.
 *
 * Returns:
 *   []FuzzyConceptMatch — near-misses sorted by Start, then Concept; nil
 *                          when none.
 *   error               — on database failure.
 *
 * Example:
 *   matches, err := kb.FuzzyMatchConceptNames("the chunkings here")
 *   // matches[0] == {Concept: "chunking", Text: "chunkings", Distance: 1, ...}
 */
func (kb *KnowledgeBase) FuzzyMatchConceptNames(text string) ([]FuzzyConceptMatch, error) {
	concepts, err := kb.Concepts()
	if err != nil {
		return nil, err
	}
	exact, err := kb.MatchConceptNames(text)
	if err != nil {
		return nil, err
	}
	exactSet := map[string]bool{}
	for _, name := range exact {
		exactSet[strings.ToLower(name)] = true
	}

	var tokens []fuzzyToken
	for _, loc := range fuzzyTokenPattern.FindAllStringIndex(text, -1) {
		tokens = append(tokens, fuzzyToken{Text: text[loc[0]:loc[1]], Start: loc[0], End: loc[1]})
	}

	var out []FuzzyConceptMatch
	for _, c := range concepts {
		if exactSet[strings.ToLower(c.Name)] {
			continue
		}
		lowerConcept := strings.ToLower(c.Name)
		words := strings.Fields(c.Name)
		if len(words) <= 1 {
			for _, tok := range tokens {
				d, ok := fuzzyDistance(lowerConcept, strings.ToLower(tok.Text))
				if !ok {
					continue
				}
				out = append(out, FuzzyConceptMatch{
					Concept: c.Name, Text: tok.Text, Start: tok.Start, End: tok.End, Distance: d,
				})
			}
			continue
		}
		w := len(words)
		for i := 0; i+w <= len(tokens); i++ {
			contiguous := true
			for j := i; j < i+w-1; j++ {
				if !wholeGapPattern.MatchString(text[tokens[j].End:tokens[j+1].Start]) {
					contiguous = false
					break
				}
			}
			if !contiguous {
				continue
			}
			window := tokens[i : i+w]
			var parts []string
			for _, tok := range window {
				parts = append(parts, tok.Text)
			}
			candidate := strings.Join(parts, " ")
			d, ok := fuzzyDistance(lowerConcept, strings.ToLower(candidate))
			if !ok {
				continue
			}
			out = append(out, FuzzyConceptMatch{
				Concept: c.Name, Text: candidate, Start: window[0].Start, End: window[w-1].End, Distance: d,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		return out[i].Concept < out[j].Concept
	})
	return out, nil
}
