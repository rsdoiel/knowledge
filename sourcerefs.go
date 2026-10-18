package knowledge

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

/** CitedRef is one entry of a `sources:` front matter list, understood
 * (DR-0081 item 4): a bare DOI, an absolute URL, or the id of a source already
 * in the knowledge base.
 *
 * Fields:
 *   Kind  (string) — "doi", "url" or "id".
 *   Value (string) — the DOI or URL as written, or the source id in decimal.
 *
 * Example:
 *   refs, warnings := knowledge.ParseSourceRefs([]string{"10.1234/sparql", "source:3"})
 *   // refs[0].Kind == "doi", refs[1] == CitedRef{"id", "3"}
 */
type CitedRef struct {
	Kind  string
	Value string
}

// pastedDOIPrefix matches the forms of a DOI that are a prefix on the bare
// 10.NNNN/suffix, which kb source add also refuses.
var pastedDOIPrefix = regexp.MustCompile(`(?i)^(doi:|https?://(dx\.)?doi\.org/)`)

/** ParseSourceRefs reads the entries of a `sources:` list. Blank entries are
 * skipped silently. An entry that is not a bare DOI, an absolute URL or
 * `source:ID` (a positive integer) is reported and skipped, the rest are kept:
 * it is data read from a file, never a reason to fail an ingest.
 *
 * A pasted `doi:` or doi.org prefix is refused, not rewritten, for the reason
 * `kb source add` gives: a mistyped DOI would otherwise be checked for
 * retraction, found nowhere, and read as "not retracted".
 *
 * Parameters:
 *   raw ([]string) — the entries as written.
 *
 * Returns:
 *   []CitedRef — the entries understood, in order, duplicates removed.
 *   []string   — a message for each entry that was not.
 *
 * Example:
 *   refs, warnings := knowledge.ParseSourceRefs([]string{"10.1234/x", "doi:10.1/y"})
 *   // len(refs) == 1; warnings names "doi:10.1/y" and says to write the bare DOI
 */
func ParseSourceRefs(raw []string) ([]CitedRef, []string) {
	var refs []CitedRef
	var warnings []string
	seen := map[CitedRef]bool{}
	for _, entry := range raw {
		e := strings.TrimSpace(entry)
		if e == "" {
			continue
		}
		var ref CitedRef
		switch {
		case strings.HasPrefix(strings.ToLower(e), "source:"):
			n, err := strconv.ParseInt(strings.TrimSpace(e[len("source:"):]), 10, 64)
			if err != nil || n <= 0 {
				warnings = append(warnings, fmt.Sprintf("sources: %q is not source:ID with a positive source id", e))
				continue
			}
			ref = CitedRef{"id", strconv.FormatInt(n, 10)}
		case doiShape.MatchString(e):
			ref = CitedRef{"doi", e}
		case pastedDOIPrefix.MatchString(e):
			warnings = append(warnings, fmt.Sprintf("sources: %q has a doi: or doi.org prefix; write the bare DOI, 10.NNNN/suffix", e))
			continue
		case validAbsoluteURL(e):
			ref = CitedRef{"url", e}
		default:
			warnings = append(warnings, fmt.Sprintf("sources: %q is not a bare DOI, an absolute URL or source:ID", e))
			continue
		}
		if !seen[ref] {
			seen[ref] = true
			refs = append(refs, ref)
		}
	}
	return refs, warnings
}

// sourcesFrontmatter is the one key of a document's front matter this file
// reads. A pointer, so `sources: []` (present, empty: the file manages its
// citations and cites nothing) differs from no key at all.
type sourcesFrontmatter struct {
	Sources *yamlList `yaml:"sources"`
}

// extractSourcesFrontmatter reads a document's `sources:` list. present is
// false when there is no front matter or no such key, in which case the file
// manages no citations.
func extractSourcesFrontmatter(data []byte) (entries []string, present bool) {
	front, _, err := splitFrontmatter(string(data))
	if err != nil {
		return nil, false
	}
	var fm sourcesFrontmatter
	if yaml.Unmarshal([]byte(front), &fm) != nil || fm.Sources == nil {
		return nil, false
	}
	return []string(*fm.Sources), true
}

/** SyncDocumentSources makes a document's `cited` links match the sources its
 * file lists (DR-0081 item 4): each entry is resolved to a source (an existing
 * identifier or id is linked; an unknown DOI or URL is made a source titled by
 * its identifier), links the file no longer lists are removed, and links with
 * any other relationship are left alone. It is run on every ingest of a file
 * with a `sources:` key, a skipped one included, as story events are.
 *
 * Parameters:
 *   documentID (int64)      — the document.
 *   refs       ([]CitedRef) — the entries, from ParseSourceRefs.
 *
 * Returns:
 *   int      — how many sources the document now cites through its file.
 *   []string — a message for each entry that could not be resolved
 *              (a source:ID that names no source).
 *   error    — on database failure.
 *
 * Example:
 *   refs, _ := knowledge.ParseSourceRefs([]string{"10.1234/sparql"})
 *   n, warnings, err := kb.SyncDocumentSources(docID, refs)
 */
func (kb *KnowledgeBase) SyncDocumentSources(documentID int64, refs []CitedRef) (int, []string, error) {
	return kb.syncSources("document_sources", "document_id", documentID, refs)
}

/** SyncRecordSources is SyncDocumentSources for a decision record's `sources:`
 * front matter.
 *
 * Parameters:
 *   recordID (int64)      — the record.
 *   refs     ([]CitedRef) — the entries, from ParseSourceRefs.
 *
 * Returns:
 *   int      — how many sources the record now cites through its file.
 *   []string — a message for each entry that could not be resolved.
 *   error    — on database failure.
 *
 * Example:
 *   n, warnings, err := kb.SyncRecordSources(recID, refs)
 */
func (kb *KnowledgeBase) SyncRecordSources(recordID int64, refs []CitedRef) (int, []string, error) {
	return kb.syncSources("record_sources", "record_id", recordID, refs)
}

func (kb *KnowledgeBase) syncSources(table, ownerCol string, ownerID int64, refs []CitedRef) (int, []string, error) {
	var warnings []string
	var ids []int64
	have := map[int64]bool{}
	for _, ref := range refs {
		var id int64
		var err error
		switch ref.Kind {
		case "id":
			id, _ = strconv.ParseInt(ref.Value, 10, 64)
			if err = kb.requireRow("sources", id, "source"); err != nil {
				warnings = append(warnings, fmt.Sprintf("sources: source:%d names no source", id))
				continue
			}
		default:
			id, err = kb.AddSource(Source{Title: ref.Value, IdentifierType: ref.Kind, IdentifierValue: ref.Value})
			if err != nil {
				return 0, warnings, err
			}
		}
		if !have[id] {
			have[id] = true
			ids = append(ids, id)
		}
	}
	// Drop the cited links the file no longer lists.
	q := `DELETE FROM ` + table + ` WHERE ` + ownerCol + ` = ? AND relationship = 'cited'`
	args := []any{ownerID}
	if len(ids) > 0 {
		q += ` AND source_id NOT IN (` + placeholders(len(ids)) + `)`
		args = append(args, int64Args(ids)...)
	}
	if _, err := kb.db.Exec(q, args...); err != nil {
		return 0, warnings, err
	}
	for _, id := range ids {
		if err := kb.linkSource(table, ownerCol, tableOwner[table], ownerWhat[table], ownerID, id, "cited", "", ""); err != nil {
			return 0, warnings, err
		}
	}
	return len(ids), warnings, nil
}

var tableOwner = map[string]string{"document_sources": "documents", "record_sources": "records"}
var ownerWhat = map[string]string{"document_sources": "document", "record_sources": "record"}
