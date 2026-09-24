package knowledge

import (
	"regexp"
	"sort"
	"strings"
)

// Moved from cmd/kb/documenttag.go (library-lift-plan.md L2, DR-0035). Every
// function here is a verbatim move: text in, text out, no file I/O. `kb
// document tag` keeps the project walk, the file reads and writes, and the
// both-or-neither rollback; a second consumer supplies its own write path.

// existingWikilinkPattern finds [[...]] spans already present in text, to
// exclude them from both re-matching (no nested brackets) and from
// alreadyWikilinked's idempotency check. Deliberately not wikilinkPattern
// (documentingest.go): that one captures the inner name for resolution; this
// one only needs the byte span of the whole bracketed form.
var existingWikilinkPattern = regexp.MustCompile(`\[\[[^\[\]]+\]\]`)

// textSpan is a half-open byte range [Start, End) within a string.
type textSpan struct{ Start, End int }

// overlaps reports whether s and loc (a regexp FindIndex-style [start, end)
// pair) share any byte.
func (s textSpan) overlaps(loc []int) bool {
	return loc[0] < s.End && s.Start < loc[1]
}

// excludedSpans returns the byte ranges of text that insertWikilink must
// never write into: a leading YAML frontmatter block, fenced code blocks,
// inline code spans, and any wikilink already present. Reuses the same
// fenced/inline patterns document ingest's density-linking already applies
// (DR-0027's stripCodeSpans) rather than a second definition of "code," and
// the same frontmatter delimiter recordfile.go's splitFrontmatter uses,
// applied positionally instead of by string-splitting since a caller here
// needs byte offsets into the original text, not a parsed-and-rejoined copy.
func excludedSpans(text string) []textSpan {
	var spans []textSpan
	if loc := frontmatterBlockPattern.FindStringIndex(text); loc != nil {
		spans = append(spans, textSpan{loc[0], loc[1]})
	}
	if loc := firstH1HeadingPattern.FindStringIndex(text); loc != nil {
		spans = append(spans, textSpan{loc[0], loc[1]})
	}
	for _, loc := range fencedCodeBlockPattern.FindAllStringIndex(text, -1) {
		spans = append(spans, textSpan{loc[0], loc[1]})
	}
	for _, loc := range inlineCodeSpanPattern.FindAllStringIndex(text, -1) {
		spans = append(spans, textSpan{loc[0], loc[1]})
	}
	for _, loc := range existingWikilinkPattern.FindAllStringIndex(text, -1) {
		spans = append(spans, textSpan{loc[0], loc[1]})
	}
	return spans
}

// frontmatterBlockPattern matches a leading "---\n...\n---" block, the same
// shape splitFrontmatter (recordfile.go) requires to treat a file as having
// frontmatter at all. Anchored to the very start of text (Go's regexp `^`
// without (?m) matches only the string start, not every line), since
// frontmatter is only ever a document's opening block, never a mid-body
// occurrence of three dashes.
var frontmatterBlockPattern = regexp.MustCompile(`(?s)^---\n.*?\n---`)

// firstH1HeadingPattern matches only the document's very first true H1
// line (a single '#'), wherever it falls in text -- found live,
// smoke-testing against a real document: a concept name in the title got
// wikilinked, mechanically correct (a real, distinctive match) but
// stylistically wrong, since a title should read as plain prose. `(?m)`
// so `^`/`$` match at any line start/end, not just the string's; only the
// first match is ever used (FindStringIndex, not FindAll), matching
// firstH1Heading's (documents.go) own "first H1 only" semantics.
var firstH1HeadingPattern = regexp.MustCompile(`(?m)^#[ \t]+.*$`)

// alreadyWikilinked reports whether name (case-insensitively) already
// appears inside some [[...]] span in text -- the idempotency check that
// makes a second `kb document tag` run on an unchanged file a no-op for
// this concept, the same "second run skips unchanged" discipline `kb
// ingest` and `kb document ingest` already apply, just checked structurally
// here rather than by comparing a checksum.
func alreadyWikilinked(text, name string) bool {
	pattern := regexp.MustCompile(`(?i)\[\[\s*` + regexp.QuoteMeta(name) + `\s*\]\]`)
	return pattern.MatchString(text)
}

/** insertWikilink wraps the first safe occurrence of name in text with
 * [[ ]], preserving the matched text's original casing, for `kb document
 * tag` (TODO.md's programmatic-corpus-improvement item). "Safe" excludes a
 * leading YAML frontmatter block, fenced code blocks, inline code spans,
 * and any span already inside an existing [[...]] wikilink -- inserting
 * into any of those would corrupt metadata, quoted code, or nest brackets.
 * A name already wikilinked anywhere in text is left alone rather than
 * wrapped a second time.
 *
 * Parameters:
 *   text (string) — the document's raw content (or the portion being
 *                    scanned).
 *   name (string) — the concept name to find, matched case-insensitively
 *                    as a whole word/phrase (word-boundary anchored, like
 *                    MatchConceptNames).
 *
 * Returns:
 *   string — text with the first safe occurrence of name wrapped in
 *            [[ ]], or text unchanged if inserted is false.
 *   bool   — whether an insertion was made.
 *
 * Example:
 *   out, inserted := insertWikilink("we discussed chunking today", "chunking")
 *   // out == "we discussed [[chunking]] today", inserted == true
 */
func insertWikilink(text, name string) (string, bool) {
	if alreadyWikilinked(text, name) {
		return text, false
	}

	excluded := excludedSpans(text)
	for _, loc := range conceptNameMatches(text, name) {
		safe := true
		for _, span := range excluded {
			if span.overlaps(loc) {
				safe = false
				break
			}
		}
		if !safe {
			continue
		}
		matched := text[loc[0]:loc[1]]
		return text[:loc[0]] + "[[" + matched + "]]" + text[loc[1]:], true
	}
	return text, false
}

/** TagDocumentText applies insertWikilink for every name in names against
 * text, returning the fully-updated text and the subset of names actually
 * inserted. Names are applied longest-first (ties broken alphabetically,
 * for determinism), so a longer phrase is wrapped as one unit before a
 * shorter name it contains gets a chance to fragment it -- e.g. "chunking
 * strategy" wraps whole rather than "chunking" wrapping first and leaving
 * "strategy" dangling outside the brackets.
 *
 * Parameters:
 *   text  (string)   — the document's raw content to tag.
 *   names ([]string) — candidate concept names to look for.
 *
 * Returns:
 *   string   — text with every safely-found candidate wrapped in [[ ]].
 *   []string — the names actually inserted, in application order; nil if
 *              none matched.
 *
 * Example:
 *   out, inserted := TagDocumentText(body, []string{"chunking", "workspace"})
 */
func TagDocumentText(text string, names []string) (string, []string) {
	sorted := append([]string(nil), names...)
	sort.Slice(sorted, func(i, j int) bool {
		if len(sorted[i]) != len(sorted[j]) {
			return len(sorted[i]) > len(sorted[j])
		}
		return sorted[i] < sorted[j]
	})
	var inserted []string
	for _, name := range sorted {
		next, ok := insertWikilink(text, name)
		if ok {
			text = next
			inserted = append(inserted, name)
		}
	}
	return text, inserted
}

/** EligibleTagConcepts returns the subset of candidates mentioned more than
 * once in text, outside code spans -- the same >1-occurrence,
 * code-span-excluded threshold DR-0027's density-linking already applies
 * to decide a concept is genuinely relevant to a piece of text, reused
 * here (via MatchConceptNameCounts) so a concept `kb document tag` would
 * insert explicitly is exactly one density-linking would already have
 * linked implicitly on the next ingest -- this tool just makes that link
 * visible in the prose instead of leaving it inferred.
 *
 * The result is sorted case-insensitively (then exactly), so it is stable from
 * one call to the next.
 *
 * Parameters:
 *   text       (string)   — the document's raw content.
 *   candidates ([]string) — concept names eligibility is restricted to.
 *
 * Returns:
 *   []string — candidates mentioned more than once outside code spans; nil
 *              when none qualify.
 *   error    — on database failure.
 *
 * Example:
 *   names, err := kb.EligibleTagConcepts(body, []string{"chunking", "workspace"})
 */
func (kb *KnowledgeBase) EligibleTagConcepts(text string, candidates []string) ([]string, error) {
	allowed := map[string]bool{}
	for _, c := range candidates {
		allowed[strings.ToLower(c)] = true
	}
	counts, err := kb.MatchConceptNameCounts(StripCodeSpans(text))
	if err != nil {
		return nil, err
	}
	var out []string
	for name, count := range counts {
		if count > 1 && allowed[strings.ToLower(name)] {
			out = append(out, name)
		}
	}
	// Sorted, case-insensitively then exactly, so the result does not depend on
	// the map's randomized iteration order (DR-0037).
	sort.Slice(out, func(i, j int) bool {
		li, lj := strings.ToLower(out[i]), strings.ToLower(out[j])
		if li != lj {
			return li < lj
		}
		return out[i] < out[j]
	})
	return out, nil
}
