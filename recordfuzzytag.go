package knowledge

import (
	"fmt"
	"regexp"
	"strings"
)

/** RecordFuzzyFinding is one concept a decision record mentions only by
 * near-miss spellings, and so does not link to.
 *
 * The JSON tags are the per-finding shape `kb record fuzzy-tag --json` prints.
 *
 * Fields:
 *   Concept       (string)   — the canonical concept name.
 *   Variants      ([]string) — the distinct near-miss spellings found, as written, in order
 *                              of first appearance.
 *   Count         (int)      — how many near-miss occurrences there are in all.
 *   ExactMentions (int)      — how many times the plain concept name appears in the body:
 *                              prose that links nothing, reported so a reader can see the
 *                              record already talks about the concept.
 *
 * Example:
 *   rep, _ := kb.RecordFuzzyReport(rf, []string{"toast"})
 *   fmt.Println(rep[0].Concept, rep[0].Variants) // toast [detoast]
 */
type RecordFuzzyFinding struct {
	Concept       string   `json:"concept"`
	Variants      []string `json:"variants"`
	Count         int      `json:"count"`
	ExactMentions int      `json:"exact_mentions"`
}

/** RecordFuzzyReport finds, in a decision record's body, near-miss spellings
 * of known concepts that the record does not already link. It writes nothing.
 *
 * It reuses the document matcher and its eligibility rules (FuzzyEligible).
 * The explicit-concept bypass is narrowed for records: FuzzyEligible would
 * keep every token within FuzzyMatchConceptNames's ceiling of a named
 * concept, which for a short concept is most short common words, so a named
 * concept is widened only to tokens that contain its name (detoast,
 * toasting) plus what the ordinary rules already admit. Two more things differ from documents
 * (DR-0052 item 7). Only the body is searched, so frontmatter text such as
 * `tags: [toast]` never matches. And a concept is skipped only when the
 * record already links it, in `tags:` or as a [[wikilink]], not when its
 * plain name appears in prose, since prose links nothing.
 *
 * Parameters:
 *   rf       (*RecordFile) — the parsed record.
 *   explicit ([]string)    — concept names to force, bypassing the length and distance
 *                            rules for exactly these; nil for the ordinary rules.
 *
 * Returns:
 *   []RecordFuzzyFinding — one per unlinked concept with at least one eligible near-miss,
 *                           ordered by first appearance; nil when none.
 *   error                — on database failure.
 *
 * Example:
 *   rf, _ := knowledge.ParseRecordFile("decisions/0013-x.md")
 *   findings, err := kb.RecordFuzzyReport(rf, nil)
 */
func (kb *KnowledgeBase) RecordFuzzyReport(rf *RecordFile, explicit []string) ([]RecordFuzzyFinding, error) {
	body := rf.Record.Body
	matches, err := kb.fuzzyMatchConceptNames(body, false)
	if err != nil {
		return nil, err
	}
	exact, err := kb.MatchConceptNameCounts(body)
	if err != nil {
		return nil, err
	}
	linked := recordLinkedConcepts(rf)

	// FuzzyEligible's explicit bypass keeps every token within
	// maxFuzzyDistance of a named concept, which for a short concept is most
	// short common words (to, that, has are all within 3 of "toast"). The
	// record verb narrows it: a named concept is widened only to tokens that
	// contain its name (detoast, toasting), on top of what the ordinary length
	// and distance rules already admit. document fuzzy-tag is untouched.
	ordinary := map[[2]any]bool{}
	if len(explicit) > 0 {
		for _, m := range FuzzyEligible(matches, nil, body) {
			ordinary[[2]any{m.Start, m.Concept}] = true
		}
	}

	var out []RecordFuzzyFinding
	byConcept := map[string]int{}
	for _, m := range FuzzyEligible(matches, explicit, body) {
		if m.Distance == 0 || linked[strings.ToLower(m.Concept)] {
			continue // the concept's own name, or already linked
		}
		if len(explicit) > 0 && !ordinary[[2]any{m.Start, m.Concept}] &&
			!strings.Contains(strings.ToLower(m.Text), strings.ToLower(m.Concept)) {
			continue
		}
		i, seen := byConcept[m.Concept]
		if !seen {
			i = len(out)
			byConcept[m.Concept] = i
			out = append(out, RecordFuzzyFinding{Concept: m.Concept, ExactMentions: exact[m.Concept]})
		}
		out[i].Count++
		known := false
		for _, v := range out[i].Variants {
			known = known || v == m.Text
		}
		if !known {
			out[i].Variants = append(out[i].Variants, m.Text)
		}
	}
	return out, nil
}

// recordLinkedConcepts is the lower-cased names a record already links: its
// frontmatter tags and any [[wikilink]] in the body outside code spans.
func recordLinkedConcepts(rf *RecordFile) map[string]bool {
	linked := map[string]bool{}
	for _, t := range rf.Tags {
		linked[strings.ToLower(strings.TrimSpace(t))] = true
	}
	for _, m := range wikilinkPattern.FindAllStringSubmatch(StripCodeSpans(rf.Record.Body), -1) {
		linked[strings.ToLower(strings.TrimSpace(m[1]))] = true
	}
	return linked
}

var plainTagPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _.-]*$`)

// tagScalar renders a tag for a YAML flow or block list: plain when that is
// unambiguous, double-quoted otherwise.
func tagScalar(s string) string {
	if plainTagPattern.MatchString(s) && strings.TrimSpace(s) == s {
		return s
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

/** AddRecordTags adds concepts to the frontmatter `tags:` of a record file's
 * raw bytes, changing nothing else: not the other frontmatter lines, not the
 * body, not the line endings. It is not RenderRecordFile, which normalises the
 * whole file.
 *
 * It edits three shapes of tags and refuses the rest rather than guessing:
 * a one-line flow list (`tags: []`, `tags: [a, b]`), a block list
 * (`tags:` then `- item` lines), and an absent field (a `tags:` line is
 * added before the closing fence). A name already present, compared without
 * regard to case, is not added again, so the call is idempotent.
 *
 * Parameters:
 *   raw      ([]byte)   — the whole record file.
 *   concepts ([]string) — names to add.
 *
 * Returns:
 *   []byte — the edited file; the input itself, unchanged, when there was nothing to add.
 *   error  — ErrInvalid for a file without frontmatter or a tags form it will not edit.
 *
 * Example:
 *   next, err := knowledge.AddRecordTags(raw, []string{"toast"})
 */
func AddRecordTags(raw []byte, concepts []string) ([]byte, error) {
	text := string(raw)
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	lines := strings.SplitAfter(text, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r\n") != "---" {
		return nil, invalidf("knowledge: record has no frontmatter to edit")
	}
	closing := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r\n") == "---" {
			closing = i
			break
		}
	}
	if closing < 0 {
		return nil, invalidf("knowledge: record frontmatter is not closed")
	}
	tagsAt := -1
	for i := 1; i < closing; i++ {
		if strings.HasPrefix(lines[i], "tags:") {
			tagsAt = i
			break
		}
	}

	// Which names are present already (block or flow), to skip them.
	have := map[string]bool{}
	if rf, err := ParseRecord([]byte(strings.ReplaceAll(text, "\r\n", "\n")), ""); err == nil {
		for _, t := range rf.Tags {
			have[strings.ToLower(strings.TrimSpace(t))] = true
		}
	}
	var add []string
	for _, c := range concepts {
		if k := strings.ToLower(strings.TrimSpace(c)); k != "" && !have[k] {
			have[k] = true
			add = append(add, c)
		}
	}
	if len(add) == 0 {
		return raw, nil
	}
	quoted := make([]string, len(add))
	for i, c := range add {
		quoted[i] = tagScalar(c)
	}

	var out []string
	switch {
	case tagsAt < 0:
		out = append(out, lines[:closing]...)
		out = append(out, "tags: ["+strings.Join(quoted, ", ")+"]"+eol)
		out = append(out, lines[closing:]...)
	default:
		rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimRight(lines[tagsAt], "\r\n"), "tags:"))
		switch {
		case strings.HasPrefix(rest, "[") && strings.HasSuffix(rest, "]"):
			inner := strings.TrimSpace(rest[1 : len(rest)-1])
			joined := strings.Join(quoted, ", ")
			if inner != "" {
				joined = inner + ", " + joined
			}
			out = append(out, lines[:tagsAt]...)
			out = append(out, "tags: ["+joined+"]"+eol)
			out = append(out, lines[tagsAt+1:]...)
		case rest == "":
			// Block list: continue after the last "- item" line, in the first item's indent.
			last, indent := tagsAt, "  "
			for i := tagsAt + 1; i < closing; i++ {
				t := strings.TrimLeft(lines[i], " \t")
				if !strings.HasPrefix(t, "- ") && strings.TrimRight(t, "\r\n") != "-" {
					break
				}
				if last == tagsAt {
					indent = lines[i][:len(lines[i])-len(t)]
				}
				last = i
			}
			out = append(out, lines[:last+1]...)
			for _, q := range quoted {
				out = append(out, indent+"- "+q+eol)
			}
			out = append(out, lines[last+1:]...)
		default:
			return nil, invalidf("knowledge: tags: is written in a form this edit will not change (%q); edit the file by hand", rest)
		}
	}
	next := []byte(strings.Join(out, ""))
	rf, err := ParseRecord([]byte(strings.ReplaceAll(string(next), "\r\n", "\n")), "")
	if err != nil {
		return nil, fmt.Errorf("knowledge: edited record no longer parses: %w", err)
	}
	for _, c := range add {
		found := false
		for _, t := range rf.Tags {
			found = found || strings.EqualFold(strings.TrimSpace(t), c)
		}
		if !found {
			return nil, invalidf("knowledge: tags: was not in a form this edit understands; edit the file by hand")
		}
	}
	return next, nil
}
