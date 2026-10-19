package knowledge

import (
	"regexp"
	"strings"
)

// searchColumns are the columns of a search hit, in the order the scans read
// them, ending where the caller appends the project id expression.
const searchColumns = `kb_fts.kind,
		       CASE WHEN kb_fts.source_type = 'observation'
		            THEN COALESCE(p.name, '') ELSE kb_fts.label END,
		       CASE WHEN kb_fts.source_type = 'observation'
		            THEN kb_fts.body ELSE kb_fts.descr END,
		       kb_fts.source_type,
		       CAST(COALESCE(NULLIF(kb_fts.source_id, ''), 0) AS INTEGER),
		       CASE WHEN kb_fts.source_type IN ('concept', 'project') THEN ''
		            ELSE snippet(kb_fts, 0, '«', '»', '…', 24) END,
		       CASE WHEN kb_fts.source_type = 'record' THEN COALESCE(rec.status, '') ELSE '' END,
		       CASE WHEN kb_fts.source_type = 'record' AND rec.id IS NOT NULL
		            THEN (CASE WHEN rec.scope = 'workspace' THEN 'workspace' ELSE COALESCE(rp.name, '') END)
		                 || '/DR-' || rec.record_id
		            ELSE '' END,
		       `

// searchJoins bring in the observation's project, and the record row and its
// project, that searchColumns read.
const searchJoins = `
		LEFT JOIN projects p ON kb_fts.source_type = 'observation'
		                     AND p.id = kb_fts.project_id
		LEFT JOIN records rec ON kb_fts.source_type = 'record'
		                     AND rec.id = CAST(COALESCE(NULLIF(kb_fts.source_id, ''), 0) AS INTEGER)
		LEFT JOIN projects rp ON rp.id = rec.project_id`

// boldLabel matches a record's paragraph label: **Decision.** or **Context**.
var boldLabel = regexp.MustCompile(`\*\*([^*\n]+?)\.?\*\*`)

// finishHit completes a hit the query could not: a passage with no marked word
// is dropped, and a record's Section is found by locating the passage in its
// body and taking the nearest label before it.
func (kb *KnowledgeBase) finishHit(r *KBSearchResult) {
	if !strings.Contains(r.Passage, "«") {
		r.Passage = ""
		return
	}
	if r.SourceType != "record" {
		return
	}
	var title, body string
	if kb.db.QueryRow(`SELECT title, body FROM records WHERE id = ?`, r.SourceID).Scan(&title, &body) != nil {
		return
	}
	r.Section = recordSection(title, body, r.Passage)
	if r.Section != "title" {
		if tight := recordPassage(title, body, r.Passage); tight != "" {
			r.Passage = tight
		}
	}
}

// markedTerm matches a word the snippet marked.
var markedTerm = regexp.MustCompile(`«([^»]+)»`)

// leadingLabel matches the bold label that opens a record paragraph.
var leadingLabel = regexp.MustCompile(`^\*\*[^*\n]+?\.?\*\*\s*`)

// recordPassageRunes is the longest passage a record hit carries.
const recordPassageRunes = 200

// recordPassage narrows a record's snippet to the paragraph the first marked
// word is in. A short record fits in one snippet, so the snippet was the title and
// the start of the body as well as the match. The paragraph loses its bold
// label (Section names it), has the snippet's marked words marked again, and is
// cut around the first mark when it is long. "" means the paragraph could not be
// found, and the caller keeps the snippet.
func recordPassage(title, body, passage string) string {
	norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	terms := markedTerm.FindAllStringSubmatch(passage, -1)
	if len(terms) == 0 {
		return ""
	}
	var quoted []string
	for _, t := range terms {
		quoted = append(quoted, regexp.QuoteMeta(t[1]))
	}
	re, err := regexp.Compile(`(?i)\b(` + strings.Join(quoted, "|") + `)\b`)
	if err != nil {
		return ""
	}
	_, pos, ok := locateMark(title, body, passage)
	if !ok {
		return ""
	}
	// The text is the title and the paragraphs, each normalised and joined by a
	// space, so a paragraph's place in it is a running offset.
	off := len(norm(title)) + 1
	for _, para := range strings.Split(body, "\n\n") {
		para = norm(para)
		if para == "" {
			continue
		}
		if pos >= off && pos < off+len(para) {
			para = norm(leadingLabel.ReplaceAllString(para, ""))
			return cutAround(re.ReplaceAllString(para, "«$1»"), recordPassageRunes)
		}
		off += len(para) + 1
	}
	return ""
}

// cutAround shortens s to about max runes around its first «, with … on a cut side.
func cutAround(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	at := strings.Index(s, "«")
	if at < 0 {
		return string(r[:max]) + "…"
	}
	mark := len([]rune(s[:at]))
	start := max2(0, mark-max/2)
	end := start + max
	if end > len(r) {
		end = len(r)
		start = max2(0, end-max)
	}
	out := string(r[start:end])
	if start > 0 {
		out = "…" + strings.TrimLeft(out, " ")
	}
	if end < len(r) {
		out = strings.TrimRight(out, " ") + "…"
	}
	return out
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// fillHitProvenance sets Provenance on a page of hits in a fixed number of
// queries (DR-0080).
func (kb *KnowledgeBase) fillHitProvenance(hits []KBSearchResult) error {
	targets := make([]provTarget, len(hits))
	for i := range hits {
		targets[i] = provTarget{sourceType: hits[i].SourceType, id: hits[i].SourceID, dst: &hits[i].HitProvenance}
	}
	return kb.fillProvenance(targets)
}

// locateMark finds the first marked word of a snippet in a record's text (the
// title, a newline and the body, white space collapsed): the text, and the offset.
func locateMark(title, body, passage string) (text string, pos int, ok bool) {
	norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	p := norm(passage)
	mark := strings.Index(p, "«")
	if mark < 0 {
		return "", 0, false
	}
	// Offset of the first marked word within the passage with markers removed.
	lead := strings.NewReplacer("«", "", "»", "").Replace(p[:mark])
	core := strings.NewReplacer("«", "", "»", "").Replace(p)
	trimmed := strings.TrimLeft(core, "… ")
	lead = strings.TrimLeft(lead, "… ")
	core = strings.TrimRight(trimmed, "… ")
	text = norm(title + "\n" + body)
	i := strings.Index(text, core)
	if i < 0 {
		return "", 0, false
	}
	return text, i + len(lead), true
}

// recordSection names the part of a record that a passage's first marked word is
// in: "title" when it is in the title, otherwise the last **Label.** before it,
// or "" when it sits before any label. The index holds the title, a newline and
// the body as one text, and snippet() cuts its passage from that text, so the
// passage is located there.
func recordSection(title, body, passage string) string {
	text, pos, ok := locateMark(title, body, passage)
	if !ok {
		return ""
	}
	if pos < len(strings.Join(strings.Fields(title), " ")) {
		return "title"
	}
	section := ""
	for _, m := range boldLabel.FindAllStringSubmatchIndex(text, -1) {
		if m[0] > pos {
			break
		}
		section = text[m[2]:m[3]]
	}
	return section
}
