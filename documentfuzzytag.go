package knowledge

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Moved from cmd/kb/documentfuzzytag.go (library-lift-plan.md L2, DR-0035).
// The helpers are a verbatim move. FuzzyTagDocumentText is the body of the
// per-file loop that used to sit inline in cmdDocumentFuzzyTag, moved
// unchanged, including its two-breakpoint offset tracking (see the comment
// there for the corruption bug a single running delta caused).

// sectionHeadingPattern is fuzzy-tag's own copy of markdownHeadingPattern
// (documents.go:341) -- the same "own copy, not a shared internal"
// precedent firstH1HeadingPattern (documenttag.go) already sets for reusing
// a documents.go pattern from cmd/kb.
var sectionHeadingPattern = regexp.MustCompile(`(?m)^#{1,6}[ \t]+(.*)$`)

// footnoteLabelPattern matches both a marker ([^3]) and the start of a
// definition ([^3]:).
var footnoteLabelPattern = regexp.MustCompile(`\[\^(\d+)\]`)

// footnoteDefinitionLinePattern matches a whole footnote-definition line, so
// fuzzyExcludedSpans can keep a marker from ever landing inside one.
var footnoteDefinitionLinePattern = regexp.MustCompile(`(?m)^\[\^\d+\]:.*$`)

// nearestPrecedingHeading returns the text of the last sectionHeadingPattern
// match starting before pos, or "" if pos falls before any heading (gist/
// lead text).
func nearestPrecedingHeading(text string, pos int) string {
	var heading string
	for _, m := range sectionHeadingPattern.FindAllStringSubmatchIndex(text, -1) {
		if m[0] >= pos {
			break
		}
		heading = text[m[2]:m[3]]
	}
	return heading
}

// nextSectionBoundary returns the start offset of the next
// sectionHeadingPattern match after pos, or len(text) if none -- where a
// footnote definition for a marker at pos gets inserted.
func nextSectionBoundary(text string, pos int) int {
	for _, loc := range sectionHeadingPattern.FindAllStringIndex(text, -1) {
		if loc[0] > pos {
			return loc[0]
		}
	}
	return len(text)
}

// nextFootnoteLabel scans every footnoteLabelPattern match's captured number
// and returns one past the highest found, or 1 if none exist. Label
// numbering is whole-file, not per-section: CommonMark footnote labels are
// scoped to the whole rendered document regardless of source structure.
func nextFootnoteLabel(text string) int {
	max := 0
	for _, m := range footnoteLabelPattern.FindAllStringSubmatch(text, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

// fuzzyExcludedSpans is excludedSpans (documenttag.go), untouched, plus
// every footnoteDefinitionLinePattern match, so a marker can never land
// inside an existing footnote definition. A new function, not an edit to
// excludedSpans itself -- kb document tag must stay exactly as it is.
func fuzzyExcludedSpans(text string) []textSpan {
	spans := excludedSpans(text)
	for _, loc := range footnoteDefinitionLinePattern.FindAllStringIndex(text, -1) {
		spans = append(spans, textSpan{loc[0], loc[1]})
	}
	return spans
}

// insertFootnote splices a footnote marker and definition into text for a
// near-miss match at [matchStart, matchEnd). The definition is inserted
// first, immediately before nextSectionBoundary(text, matchEnd) (or at end
// of file) -- its insertion point is always >= matchEnd, so doing this
// first keeps matchEnd valid for the marker insertion that follows. The
// marker ([^label]) is then inserted immediately after matchEnd. matchStart
// is unused by the splice itself; it's part of the signature for callers
// that already have both offsets from a FuzzyConceptMatch.
func insertFootnote(text string, matchStart, matchEnd int, concept string, label int) string {
	boundary := nextSectionBoundary(text, matchEnd)
	def := fmt.Sprintf("\n[^%d]: see [[%s]]\n", label, concept)
	text = text[:boundary] + def + text[boundary:]
	marker := fmt.Sprintf("[^%d]", label)
	text = text[:matchEnd] + marker + text[matchEnd:]
	return text
}

// fuzzyThreshold is design decision 5's real, length-based eligibility
// threshold -- distinct from FuzzyMatchConceptNames's own generous
// maxFuzzyDistance candidate-generation ceiling.
func fuzzyThreshold(concept string) int {
	if len(concept) <= 8 {
		return 1
	}
	return 2
}

// The two false-positive gates below (DR-0036) were tuned on a live run over
// 159 real documents against the real 121-concept vocabulary, where 614
// proposals came back and most were noise. Both are provisional, like
// minFuzzyTermLength in cmd/kb/conceptcluster.go (DR-0034).
const (
	// minFuzzyConceptLength is the shortest concept name (in letters) a
	// fuzzy match is proposed for when no --concept is given. Below it a
	// single edit reaches unrelated common words: fts<-its, madr<-made,
	// kb<-db, cmt<-cmd, Go<-to, awk<-ask, tui<-ui, and at five letters
	// drift<-draft. The cost is real and accepted: a short concept's plural
	// (merge<-merges) is no longer found without an explicit --concept.
	minFuzzyConceptLength = 6

	// minFuzzyDistanceTwoPrefix is the shared leading letters a distance-2
	// match must have. The true distance-2 hits are inflections of a long
	// stem (idempotency<-idempotent, documents<-documented,
	// discovery<-discovered); the false ones only share a couple of opening
	// letters (retirement<-requirement, correction<-connection,
	// supersession<-suppression). Distance 1 is not gated by this: a
	// single-character typo can fall anywhere in the word.
	minFuzzyDistanceTwoPrefix = 6
)

// sharedPrefixLen returns how many leading letters a and b have in common,
// compared case-insensitively over runes.
func sharedPrefixLen(a, b string) int {
	ra, rb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	n := 0
	for n < len(ra) && n < len(rb) && ra[n] == rb[n] {
		n++
	}
	return n
}

/** FuzzyEligible applies design decision 5's real threshold to matches
 * already produced by FuzzyMatchConceptNames: with no explicit concepts
 * given, keep a match only if its concept is at least
 * minFuzzyConceptLength letters long, its Distance is within fuzzyThreshold
 * for the concept's length, and a distance-2 match shares at least
 * minFuzzyDistanceTwoPrefix leading letters with its concept (DR-0036); with
 * explicit concepts given, keep only matches whose
 * Concept is named there (any distance up to FuzzyMatchConceptNames's own
 * ceiling qualifies -- the --concept bypass, decision 2), dropping every
 * other concept's matches entirely. Either way, drop any match whose
 * [Start, End) overlaps a frontmatter block, first H1, code span, existing
 * wikilink, or footnote-definition line in text.
 *
 * Takes text (not just matches) because the exclusion spans are computed
 * from the raw text.
 *
 * Parameters:
 *   matches  ([]FuzzyConceptMatch) — from FuzzyMatchConceptNames(text).
 *   explicit ([]string)            — concept names to force, or nil for the
 *                                    length-based threshold.
 *   text     (string)              — the same text the matches came from.
 *
 * Returns:
 *   []FuzzyConceptMatch — the matches that survive, in their original order.
 *
 * Example:
 *   ms, _ := kb.FuzzyMatchConceptNames(text)
 *   eligible := knowledge.FuzzyEligible(ms, nil, text)
 */
func FuzzyEligible(matches []FuzzyConceptMatch, explicit []string, text string) []FuzzyConceptMatch {
	explicitSet := map[string]bool{}
	for _, name := range explicit {
		explicitSet[strings.ToLower(name)] = true
	}
	excluded := fuzzyExcludedSpans(text)

	var out []FuzzyConceptMatch
	for _, m := range matches {
		if len(explicit) > 0 {
			if !explicitSet[strings.ToLower(m.Concept)] {
				continue
			}
		} else if utf8.RuneCountInString(m.Concept) < minFuzzyConceptLength ||
			m.Distance > fuzzyThreshold(m.Concept) ||
			(m.Distance > 1 && sharedPrefixLen(m.Concept, m.Text) < minFuzzyDistanceTwoPrefix) {
			continue
		}
		safe := true
		for _, span := range excluded {
			if span.overlaps([]int{m.Start, m.End}) {
				safe = false
				break
			}
		}
		if safe {
			out = append(out, m)
		}
	}
	return out
}

/** FuzzyTagInsertion is one footnote inserted by FuzzyTagDocumentText, and is
 * also the per-entry JSON shape `kb document fuzzy-tag --json` prints, so its
 * tags are a compatibility contract.
 *
 * Fields:
 *   Section  (string) — heading text of the section the near-miss sits in,
 *                       or "" for lead text before any heading.
 *   Text     (string) — the near-miss as written in the document.
 *   Concept  (string) — the canonical concept the footnote links to.
 *   Distance (int)    — the Levenshtein distance that qualified the match.
 *   Footnote (int)    — the footnote label number assigned.
 *
 * Example:
 *   // {Section: "Background", Text: "chunkings", Concept: "chunking", Distance: 1, Footnote: 1}
 */
type FuzzyTagInsertion struct {
	Section  string `json:"section"`
	Text     string `json:"text"`
	Concept  string `json:"concept"`
	Distance int    `json:"distance"`
	Footnote int    `json:"footnote"`
}

/** FuzzyTagDocumentText inserts a footnote marker after each near-miss in
 * matches, and a footnote definition carrying the canonical [[Concept]]
 * before the next section heading (or at end of file). It never bracket-wraps
 * the near-miss itself: doing so would let ResolveConceptName mint a
 * duplicate concept from the misspelled or inflected spelling. It is text in,
 * text out: no file or database access, and the caller owns the write.
 *
 * matches must be the eligible matches (see FuzzyEligible) computed against
 * this same text: their Start/End offsets index into the original text. A
 * match whose concept is already wikilinked anywhere in the text is skipped,
 * so the function is idempotent on its own output.
 *
 * Parameters:
 *   text    (string)              — the document's raw content.
 *   matches ([]FuzzyConceptMatch) — eligible matches for that text.
 *
 * Returns:
 *   string              — the text with footnotes inserted.
 *   []FuzzyTagInsertion — one entry per footnote inserted, in match order;
 *                         nil when nothing was inserted.
 *
 * Example:
 *   ms, _ := kb.FuzzyMatchConceptNames(text)
 *   out, ins := knowledge.FuzzyTagDocumentText(text, knowledge.FuzzyEligible(ms, nil, text))
 */
func FuzzyTagDocumentText(text string, matches []FuzzyConceptMatch) (string, []FuzzyTagInsertion) {
	var entries []FuzzyTagInsertion
	// insertFootnote makes two splices per call: a marker right after
	// matchEnd, and a definition at the section boundary (which can be
	// well past matchEnd, e.g. end of file). A position between the two
	// -- exactly where a second match in the same section originally
	// sits -- only shifts by the marker's length until the boundary is
	// crossed, not by both combined. A single flat running delta
	// applied to every later match overcorrects that case, splicing
	// its marker at the wrong byte offset (confirmed live: it landed
	// inside the first footnote's own definition text). Tracking each
	// insertion's two shifts as separate breakpoints, applied in the
	// order recorded, gets every later match's offset right regardless
	// of how many matches share a section.
	type offsetShift struct{ at, amount int }
	var shifts []offsetShift
	for _, m := range matches {
		start, end := m.Start, m.End
		for _, s := range shifts {
			if start >= s.at {
				start += s.amount
			}
			if end >= s.at {
				end += s.amount
			}
		}
		if alreadyWikilinked(text, m.Concept) {
			continue
		}
		label := nextFootnoteLabel(text)
		section := nearestPrecedingHeading(text, start)
		boundary := nextSectionBoundary(text, end)
		markerLen := len(fmt.Sprintf("[^%d]", label))
		defLen := len(fmt.Sprintf("\n[^%d]: see [[%s]]\n", label, m.Concept))
		text = insertFootnote(text, start, end, m.Concept, label)
		shifts = append(shifts, offsetShift{at: end, amount: markerLen})
		shifts = append(shifts, offsetShift{at: boundary, amount: defLen})
		entries = append(entries, FuzzyTagInsertion{
			Section: section, Text: m.Text, Concept: m.Concept, Distance: m.Distance, Footnote: label,
		})
	}
	return text, entries
}
