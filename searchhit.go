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

// recordSection names the part of a record that a passage's first marked word is
// in: "title" when it is in the title, otherwise the last **Label.** before it,
// or "" when it sits before any label. The index holds the title, a newline and
// the body as one text, and snippet() cuts its passage from that text, so the
// passage is located there.
func recordSection(title, body, passage string) string {
	norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	p := norm(passage)
	mark := strings.Index(p, "«")
	if mark < 0 {
		return ""
	}
	// Offset of the first marked word within the passage with markers removed.
	lead := strings.NewReplacer("«", "", "»", "").Replace(p[:mark])
	core := strings.NewReplacer("«", "", "»", "").Replace(p)
	trimmed := strings.TrimLeft(core, "… ")
	lead = strings.TrimLeft(lead, "… ")
	core = strings.TrimRight(trimmed, "… ")
	text := norm(title + "\n" + body)
	i := strings.Index(text, core)
	if i < 0 {
		return ""
	}
	pos := i + len(lead)
	if pos < len(norm(title)) {
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
