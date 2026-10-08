package knowledge

import (
	"fmt"
	"strings"
)

/** Ref names a decision record: a scope and a record id. The scope is a project
 * name or "workspace" (knowledge DR-0057); it is empty for a bare id, which
 * resolves only where the id is unambiguous.
 *
 * Fields:
 *   Scope (string) — a project name, "workspace", or "" for a bare id.
 *   ID    (string) — the record id as stored, digits only and at least four
 *                    wide ("0004").
 *
 * Example:
 *   ref, _ := knowledge.ParseRef("harvey/DR-0004")
 *   fmt.Println(ref.Scope, ref.ID) // harvey 0004
 */
type Ref struct {
	Scope string
	ID    string
}

/** String renders the reference in the form kb prints: "harvey/DR-0004", or
 * "DR-0004" for a bare id.
 *
 * Returns:
 *   string — the reference.
 *
 * Example:
 *   knowledge.Ref{Scope: "harvey", ID: "0004"}.String() // "harvey/DR-0004"
 */
func (r Ref) String() string {
	if r.Scope == "" {
		return "DR-" + r.ID
	}
	return r.Scope + "/DR-" + r.ID
}

/** ParseRef reads a record reference: "SCOPE/DR-NNNN", "SCOPE/NNNN", "DR-NNNN"
 * or "NNNN". The "DR-" prefix is case-insensitive, surrounding space is
 * ignored, and an id shorter than four digits is padded with zeros, so
 * "harvey/4" is harvey/DR-0004. It does not check that the scope exists; that
 * is ResolveRef's job, since it needs the database.
 *
 * Parameters:
 *   s (string) — the reference as typed.
 *
 * Returns:
 *   Ref   — the parsed reference.
 *   error — matching ErrInvalid when s is empty, has an empty scope or id, has
 *           more than one "/", or has an id that is not all digits.
 *
 * Example:
 *   ref, err := knowledge.ParseRef("workspace/DR-0003")
 *   // ref == Ref{"workspace", "0003"}, err == nil
 */
func ParseRef(s string) (Ref, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Ref{}, invalidf("an empty record reference; want SCOPE/DR-NNNN, e.g. harvey/DR-0004")
	}
	scope, id := "", s
	if i := strings.Index(s, "/"); i >= 0 {
		scope, id = s[:i], s[i+1:]
		if scope == "" {
			return Ref{}, invalidf("record reference %q has no scope before the \"/\"; want SCOPE/DR-NNNN", s)
		}
		if strings.Contains(id, "/") {
			return Ref{}, invalidf("record reference %q has more than one \"/\"; want SCOPE/DR-NNNN", s)
		}
	}
	if len(id) >= 3 && strings.EqualFold(id[:3], "DR-") {
		id = id[3:]
	}
	if id == "" {
		return Ref{}, invalidf("record reference %q has no id; want SCOPE/DR-NNNN", s)
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return Ref{}, invalidf("record reference %q: the id %q is not a number; want SCOPE/DR-NNNN", s, id)
		}
	}
	if len(id) < 4 {
		id = fmt.Sprintf("%04s", id)
	}
	return Ref{Scope: scope, ID: id}, nil
}

/** AmbiguousRefError reports a bare record id that more than one tier holds. It
 * carries the qualified candidates so a caller can offer them. It matches
 * ErrInvalid under errors.Is, since an ambiguous reference is a value that
 * cannot be acted on as given.
 *
 * Fields:
 *   Ref        (Ref)   — the bare reference that was asked for.
 *   Candidates ([]Ref) — each tier's qualified reference, project tiers by
 *                        name and the workspace tier as "workspace".
 *
 * Example:
 *   var amb *knowledge.AmbiguousRefError
 *   if errors.As(err, &amb) {
 *       fmt.Println(amb.Candidates) // [clasm/DR-0001 cold/DR-0001 workspace/DR-0001]
 *   }
 */
type AmbiguousRefError struct {
	Ref        Ref
	Candidates []Ref
}

// Error names the candidates in full, so the message is enough to retry with.
func (e *AmbiguousRefError) Error() string {
	var names []string
	for _, c := range e.Candidates {
		names = append(names, c.String())
	}
	return fmt.Sprintf("%s is ambiguous; qualify it as one of: %s", e.Ref, strings.Join(names, ", "))
}

// Is lets errors.Is(err, ErrInvalid) match.
func (e *AmbiguousRefError) Is(target error) bool { return target == ErrInvalid }

/** ResolveRef finds the record a reference names. A qualified reference's scope
 * is, in order: "workspace" (any case), the workspace tier; an exact project
 * name; the workspace's own directory name (any case), as an alias for the
 * workspace tier. A project therefore wins over the alias, and cannot be
 * shadowed by the directory it sits in. A bare id resolves only when exactly one
 * tier holds it; otherwise the result is an *AmbiguousRefError. Only records of
 * the named workspace are considered.
 *
 * Parameters:
 *   ref       (Ref)    — the reference, from ParseRef.
 *   workspace (string) — the workspace's name as records are stamped with it,
 *                        which is the base name of its root; "" means this
 *                        database's own.
 *
 * Returns:
 *   *Record — the record.
 *   error   — matching ErrNotFound when the scope is unknown or the tier holds
 *             no such id, or *AmbiguousRefError (matching ErrInvalid) for an
 *             ambiguous bare id.
 *
 * Example:
 *   ref, _ := knowledge.ParseRef("clasm/DR-0178")
 *   rec, err := kb.ResolveRef(ref, "")
 */
func (kb *KnowledgeBase) ResolveRef(ref Ref, workspace string) (*Record, error) {
	if workspace == "" {
		workspace = kb.workspace
	}
	if ref.Scope == "" {
		return kb.resolveBareRef(ref, workspace)
	}
	if strings.EqualFold(ref.Scope, "workspace") {
		return kb.refInTier(workspace, 0, "workspace", ref, "the workspace tier")
	}
	p, err := kb.ProjectByName(ref.Scope)
	if err != nil {
		return nil, err
	}
	if p != nil {
		return kb.refInTier(workspace, p.ID, "project", ref, "project "+p.Name)
	}
	if strings.EqualFold(ref.Scope, workspace) {
		return kb.refInTier(workspace, 0, "workspace", ref, "the workspace tier")
	}
	return nil, notFoundf("unknown scope %q in %s: it is neither a project nor the workspace", ref.Scope, ref)
}

func (kb *KnowledgeBase) refInTier(workspace string, projectID int64, scope string, ref Ref, where string) (*Record, error) {
	r, err := kb.RecordByIdentity(workspace, projectID, scope, ref.ID)
	if err != nil {
		return nil, notFoundf("no record DR-%s in %s", ref.ID, where)
	}
	return r, nil
}

func (kb *KnowledgeBase) resolveBareRef(ref Ref, workspace string) (*Record, error) {
	all, err := kb.RecordsByRecordID(ref.ID)
	if err != nil {
		return nil, err
	}
	var matches []Record
	for _, m := range all {
		if m.Workspace == workspace {
			matches = append(matches, m)
		}
	}
	switch len(matches) {
	case 0:
		return nil, notFoundf("no record %s", ref)
	case 1:
		return &matches[0], nil
	}
	projects, err := kb.Projects()
	if err != nil {
		return nil, err
	}
	names := map[int64]string{}
	for _, p := range projects {
		names[p.ID] = p.Name
	}
	amb := &AmbiguousRefError{Ref: ref}
	for _, m := range matches {
		scope := "workspace"
		if m.Scope != "workspace" {
			scope = names[m.ProjectID]
		}
		amb.Candidates = append(amb.Candidates, Ref{Scope: scope, ID: m.RecordID})
	}
	return nil, amb
}
