package knowledge

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

/** SearchKinds is the closed vocabulary of kinds a scoped search accepts, in
 * the order a menu and completion list them (knowledge DR-0073). One list serves
 * the parser, completion and the help text, so they cannot disagree. "sources"
 * joined it with DR-0080, which indexed sources and amended DR-0013.
 *
 * Example:
 *   for _, k := range knowledge.SearchKinds { fmt.Println(k) }
 */
var SearchKinds = []string{"records", "observations", "concepts", "documents", "sources", "projects"}

// searchKindSource maps a search kind to the source_type the index stores.
var searchKindSource = map[string]string{
	"records":      "record",
	"observations": "observation",
	"concepts":     "concept",
	"documents":    "document_summary",
	"sources":      "source",
	"projects":     "project",
}

/** SearchScope says where a scoped search looks (knowledge DR-0073, DR-0074).
 *
 * Fields:
 *   Project   (string)   — a project name; "" for no project restriction.
 *   Workspace (bool)     — the workspace tier only; cannot be combined with Project.
 *   Kinds     ([]string) — members of SearchKinds; empty for every kind.
 *   Limit     (int)      — most results to return; 0 for the default, 50.
 *
 * Example:
 *   sc := knowledge.SearchScope{Project: "harvey", Kinds: []string{"records"}}
 */
type SearchScope struct {
	Project   string
	Workspace bool
	Kinds     []string
	Limit     int
}

/** ScopedSearchResult is what SearchScoped returns.
 *
 * Fields:
 *   Results ([]KBSearchResult) — the best matches, up to the limit, ranked.
 *   Total   (int)              — how many rows matched in scope, before the limit.
 *   Counts  (map[string]int)   — matches per kind, keyed by the SearchKinds name.
 *   Spans   (map[int64][]string) — for each concept in Results, keyed by its id, the
 *                                 names of the projects whose context holds it, sorted.
 *   SourceSpans (map[int64][]string) — the same for each source in Results, keyed by its
 *                                 id: the projects whose observations, documents or records
 *                                 cite it. A source nothing cites has none.
 *
 * Example:
 *   r, _ := kb.SearchScoped("cache", knowledge.SearchScope{})
 *   fmt.Printf("showing %d of %d\n", len(r.Results), r.Total)
 */
type ScopedSearchResult struct {
	Results []KBSearchResult
	Total   int
	Counts  map[string]int
	Spans   map[int64][]string

	SourceSpans map[int64][]string
}

// pidExpr is a kb_fts row's project id as an integer; a missing one is 0.
const pidExpr = `CAST(COALESCE(NULLIF(kb_fts.project_id, ''), 0) AS INTEGER)`

// projectSourceIDs is the sources cited by a project's observations, documents
// or records (DR-0080 item 3); it takes the project id three times.
// workspaceSourceIDs is the sources cited by workspace-tier records and by
// documents with no project.
const projectSourceIDs = `
	SELECT os.source_id FROM observation_sources os
	  JOIN observations o ON o.id = os.observation_id WHERE o.project_id = ?
	UNION
	SELECT ds.source_id FROM document_sources ds
	  JOIN documents d ON d.id = ds.document_id WHERE d.project_id = ?
	UNION
	SELECT rs.source_id FROM record_sources rs
	  JOIN records r ON r.id = rs.record_id WHERE r.project_id = ?`

const workspaceSourceIDs = `
	SELECT rs.source_id FROM record_sources rs
	  JOIN records r ON r.id = rs.record_id WHERE r.project_id IS NULL
	UNION
	SELECT ds.source_id FROM document_sources ds
	  JOIN documents d ON d.id = ds.document_id WHERE d.project_id IS NULL`

// projectConceptIDs is the concepts that are in a project's context: tagged on
// the project, on any of its observations, or on any of its records (DR-0074).
// It takes the project id three times.
const projectConceptIDs = `
	SELECT concept_id FROM project_concepts WHERE project_id = ?
	UNION
	SELECT oc.concept_id FROM observation_concepts oc
	  JOIN observations o ON o.id = oc.observation_id WHERE o.project_id = ?
	UNION
	SELECT rc.concept_id FROM record_concepts rc
	  JOIN records r ON r.id = rc.record_id WHERE r.project_id = ?`

// scopeFilter is a SearchScope turned into a restriction on kb_fts rows.
type scopeFilter struct {
	where string // an " AND (...)" fragment over kb_fts, or "" for no restriction
	args  []any
	limit int
}

/** scopeFilter validates a scope and builds the restriction it puts on kb_fts
 * rows, shared by SearchScoped and the semantic-search corpus so there is one
 * definition of what a scope contains. allowed is the kinds the caller can
 * serve (SearchKinds for lexical search); a kind outside it is ErrInvalid. When
 * always is true the restriction is built even for an unscoped search with no
 * kinds, so a caller that serves fewer kinds than the index holds is limited to them.
 */
func (kb *KnowledgeBase) scopeFilter(scope SearchScope, allowed []string, always bool) (scopeFilter, error) {
	if scope.Project != "" && scope.Workspace {
		return scopeFilter{}, invalidf("knowledge: a search scope is a project or the workspace tier, not both")
	}
	if scope.Limit < 0 {
		return scopeFilter{}, invalidf("knowledge: search limit %d is negative", scope.Limit)
	}
	limit := scope.Limit
	if limit == 0 {
		limit = 50
	}
	for _, k := range scope.Kinds {
		if _, ok := searchKindSource[k]; !ok {
			return scopeFilter{}, invalidf("knowledge: unknown search kind %q (one of %s)", k, strings.Join(SearchKinds, ", "))
		}
		if !containsString(allowed, k) {
			return scopeFilter{}, invalidf("knowledge: kind %q cannot be searched this way (one of %s)", k, strings.Join(allowed, ", "))
		}
	}
	var pid int64
	if scope.Project != "" {
		err := kb.db.QueryRow(`SELECT id FROM projects WHERE name = ?`, scope.Project).Scan(&pid)
		if err == sql.ErrNoRows {
			return scopeFilter{}, notFoundf("knowledge: project %q not found", scope.Project)
		}
		if err != nil {
			return scopeFilter{}, fmt.Errorf("knowledge: search scope: %w", err)
		}
	}
	scoped := scope.Project != "" || scope.Workspace

	// The kinds that can have members in this scope.
	requested := scope.Kinds
	if len(requested) == 0 {
		requested = allowed
	}
	var kinds []string
	for _, k := range requested {
		switch {
		case k == "observations" && scope.Workspace:
		case k == "projects" && scoped:
		default:
			kinds = append(kinds, k)
		}
	}
	if len(kinds) == 0 {
		return scopeFilter{}, noMembersf("%s", noMembersReason(scope))
	}

	// where is "" for the global, all-kinds search, which is Search's own query.
	var where string
	var args []any
	if scoped || len(scope.Kinds) > 0 || always {
		var ors []string
		for _, k := range kinds {
			st := searchKindSource[k]
			switch {
			case !scoped:
				ors = append(ors, "kb_fts.source_type = '"+st+"'")
			case k == "concepts" && scope.Workspace:
				ors = append(ors, `(kb_fts.source_type = 'concept' AND CAST(kb_fts.source_id AS INTEGER) IN
					(SELECT rc.concept_id FROM record_concepts rc JOIN records r ON r.id = rc.record_id WHERE r.project_id IS NULL))`)
			case k == "concepts":
				ors = append(ors, `(kb_fts.source_type = 'concept' AND CAST(kb_fts.source_id AS INTEGER) IN (`+projectConceptIDs+`))`)
				args = append(args, pid, pid, pid)
			case k == "sources" && scope.Workspace:
				ors = append(ors, `(kb_fts.source_type = 'source' AND CAST(kb_fts.source_id AS INTEGER) IN (`+workspaceSourceIDs+`))`)
			case k == "sources":
				ors = append(ors, `(kb_fts.source_type = 'source' AND CAST(kb_fts.source_id AS INTEGER) IN (`+projectSourceIDs+`))`)
				args = append(args, pid, pid, pid)
			case scope.Workspace:
				ors = append(ors, "(kb_fts.source_type = '"+st+"' AND "+pidExpr+" = 0)")
			default:
				ors = append(ors, "(kb_fts.source_type = '"+st+"' AND "+pidExpr+" = ?)")
				args = append(args, pid)
			}
		}
		where = " AND (" + strings.Join(ors, " OR ") + ")"
	}
	return scopeFilter{where: where, args: args, limit: limit}, nil
}

/** SearchScoped performs the full-text search of Search, restricted to a scope
 * and to kinds, with the restriction applied in the query so a limit never
 * hides a match in scope behind a better-ranked one outside it. It returns the
 * total and a count per kind before the limit.
 *
 * With an empty scope it is the global search. A project scope covers that
 * project's observations, records, documents' reviewed summaries and concepts
 * (the union of those tagged on the project, on its observations and on its
 * records); the workspace tier covers workspace-tier records, the summaries of
 * documents with no project, and concepts linked to workspace-tier records.
 * "projects" is valid at the global scope only, and observations have no
 * workspace tier.
 *
 * Parameters:
 *   term (string) — FTS5 query; a term that is not valid syntax is searched as a phrase, as Search does.
 *   scope (SearchScope) — where to look.
 *
 * Returns:
 *   *ScopedSearchResult — the results, total and counts; never nil without an error.
 *   error — ErrInvalid for an unknown kind, a negative limit, or Project with Workspace;
 *     ErrNotFound for an unknown project; ErrNoMembers when every requested kind
 *     is impossible in the scope; a database failure otherwise.
 *
 * Example:
 *   r, err := kb.SearchScoped("exit codes", knowledge.SearchScope{Project: "harvey", Kinds: []string{"records"}})
 */
func (kb *KnowledgeBase) SearchScoped(term string, scope SearchScope) (*ScopedSearchResult, error) {
	if !kb.ftsAvailable {
		return nil, fmt.Errorf("full-text search is not available (FTS5 not compiled in)")
	}
	f, err := kb.scopeFilter(scope, SearchKinds, false)
	if err != nil {
		return nil, err
	}
	where, args, limit := f.where, f.args, f.limit
	out, err := kb.runScoped(term, where, args, limit)
	if err != nil {
		out, err = kb.runScoped(quotedPhrase(term), where, args, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("knowledge: search %q: %w", term, err)
	}
	if err := kb.fillConceptSpans(out); err != nil {
		return nil, err
	}
	if err := kb.fillSourceSpans(out); err != nil {
		return nil, err
	}
	return out, nil
}

// fillSourceSpans sets SourceSpans: for each source result, the names of the
// projects whose observations, documents or records cite it, sorted.
func (kb *KnowledgeBase) fillSourceSpans(out *ScopedSearchResult) error {
	for _, r := range out.Results {
		if r.SourceType != "source" {
			continue
		}
		rows, err := kb.db.Query(`
			SELECT DISTINCT p.name FROM projects p WHERE p.id IN (
				SELECT o.project_id FROM observation_sources os JOIN observations o ON o.id = os.observation_id
				 WHERE os.source_id = ?1
				UNION
				SELECT d.project_id FROM document_sources ds JOIN documents d ON d.id = ds.document_id
				 WHERE ds.source_id = ?1
				UNION
				SELECT r.project_id FROM record_sources rs JOIN records r ON r.id = rs.record_id
				 WHERE rs.source_id = ?1 AND r.project_id IS NOT NULL)
			ORDER BY p.name`, r.SourceID)
		if err != nil {
			return fmt.Errorf("knowledge: projects citing source %d: %w", r.SourceID, err)
		}
		var names []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				rows.Close()
				return err
			}
			names = append(names, n)
		}
		rows.Close()
		if out.SourceSpans == nil {
			out.SourceSpans = map[int64][]string{}
		}
		out.SourceSpans[r.SourceID] = names
	}
	return nil
}

// noMembersReason says why a scope and kind cannot have rows.
func noMembersReason(scope SearchScope) string {
	switch {
	case scope.Workspace && len(scope.Kinds) > 0 && !containsString(scope.Kinds, "records"):
		return "observations and projects have no workspace tier: observations belong to projects, and a project is not in a tier; the workspace tier has records"
	case scope.Project != "":
		return fmt.Sprintf("a project has no projects inside it: %q is a project; search the global scope (/projects) for projects", scope.Project)
	}
	return "this scope has no members of that kind"
}

func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// runScoped runs the counts and the ranked rows for one raw term.
func (kb *KnowledgeBase) runScoped(term, where string, args []any, limit int) (*ScopedSearchResult, error) {
	out := &ScopedSearchResult{Counts: map[string]int{}}
	kindOf := map[string]string{}
	for k, st := range searchKindSource {
		kindOf[st] = k
	}
	crows, err := kb.db.Query(
		`SELECT kb_fts.source_type, COUNT(*) FROM kb_fts WHERE kb_fts MATCH ?`+where+` GROUP BY kb_fts.source_type`,
		append([]any{term}, args...)...)
	if err != nil {
		return nil, err
	}
	for crows.Next() {
		var st string
		var n int
		if err := crows.Scan(&st, &n); err != nil {
			crows.Close()
			return nil, err
		}
		name := kindOf[st]
		if name == "" {
			name = st
		}
		out.Counts[name] += n
		out.Total += n
	}
	if err := crows.Err(); err != nil {
		crows.Close()
		return nil, err
	}
	crows.Close()

	rows, err := kb.db.Query(`
		SELECT `+searchColumns+pidExpr+`
		FROM   kb_fts`+searchJoins+`
		WHERE  kb_fts MATCH ?`+where+`
		ORDER  BY bm25(kb_fts)
		LIMIT  ?`, append(append([]any{term}, args...), limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r KBSearchResult
		if err := rows.Scan(&r.Kind, &r.Label, &r.Snippet, &r.SourceType, &r.SourceID,
			&r.Passage, &r.Status, &r.Ref, &r.ProjectID); err != nil {
			return nil, err
		}
		out.Results = append(out.Results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i := range out.Results {
		kb.finishHit(&out.Results[i])
	}
	if err := kb.fillHitProvenance(out.Results); err != nil {
		return nil, err
	}
	return out, nil
}

// fillConceptSpans sets Spans: for each concept result, the names of the projects
// whose context holds the concept, sorted. A result type without a Projects field
// keeps KBSearchResult comparable for callers that compare results.
func (kb *KnowledgeBase) fillConceptSpans(out *ScopedSearchResult) error {
	for _, r := range out.Results {
		if r.SourceType != "concept" {
			continue
		}
		rows, err := kb.db.Query(`
			SELECT DISTINCT p.name FROM projects p WHERE p.id IN (
				SELECT project_id FROM project_concepts WHERE concept_id = ?1
				UNION
				SELECT o.project_id FROM observation_concepts oc JOIN observations o ON o.id = oc.observation_id
				 WHERE oc.concept_id = ?1
				UNION
				SELECT r.project_id FROM record_concepts rc JOIN records r ON r.id = rc.record_id
				 WHERE rc.concept_id = ?1 AND r.project_id IS NOT NULL)
			ORDER BY p.name`, r.SourceID)
		if err != nil {
			return fmt.Errorf("knowledge: projects of concept %d: %w", r.SourceID, err)
		}
		var names []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				rows.Close()
				return err
			}
			names = append(names, n)
		}
		rows.Close()
		sort.Strings(names)
		if out.Spans == nil {
			out.Spans = map[int64][]string{}
		}
		out.Spans[r.SourceID] = names
	}
	return nil
}
