package knowledge

import (
	"slices"
	"strings"
)

/** ConceptKinds is the closed vocabulary of concept kinds (DR-0078): what a
 * concept names. "other" is the default, and the value of every concept made
 * before kinds existed.
 *
 * Example:
 *   fmt.Println(strings.Join(knowledge.ConceptKinds, ", ")) // character, place, theme, other
 */
var ConceptKinds = []string{"character", "place", "theme", "other"}

/** ValidConceptKind reports whether kind is one of ConceptKinds.
 *
 * Parameters:
 *   kind (string) — a candidate concept kind.
 *
 * Returns:
 *   bool — true when kind is in the vocabulary; the empty string is not.
 *
 * Example:
 *   knowledge.ValidConceptKind("place") // true
 */
func ValidConceptKind(kind string) bool {
	return slices.Contains(ConceptKinds, kind)
}

/** SetConceptKind sets what an existing concept names, and advances its
 * updated_at so merge and import carry the change (DR-0025). Adding the
 * concept again, from prose or `kb concept add`, never resets its kind.
 *
 * Parameters:
 *   name (string) — the concept's exact name.
 *   kind (string) — one of ConceptKinds.
 *
 * Returns:
 *   error — ErrInvalid for a kind outside the vocabulary, ErrNotFound when
 *     no concept has that name, or a database failure.
 *
 * Example:
 *   err := kb.SetConceptKind("the-long-fire/Anna", "character")
 */
func (kb *KnowledgeBase) SetConceptKind(name, kind string) error {
	if !ValidConceptKind(kind) {
		return invalidf("knowledge: concept kind %q is not one of %s", kind, strings.Join(ConceptKinds, ", "))
	}
	res, err := kb.db.Exec(`UPDATE concepts SET kind = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?`, kind, name)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return notFoundf("knowledge: concept %q not found", name)
	}
	return nil
}

/** ConceptsOfKind returns the concepts of one kind, ordered by name.
 *
 * Parameters:
 *   kind (string) — one of ConceptKinds.
 *
 * Returns:
 *   []Concept — the matching concepts; empty (not nil) if none.
 *   error     — ErrInvalid for a kind outside the vocabulary, or a database failure.
 *
 * Example:
 *   characters, err := kb.ConceptsOfKind("character")
 */
func (kb *KnowledgeBase) ConceptsOfKind(kind string) ([]Concept, error) {
	if !ValidConceptKind(kind) {
		return nil, invalidf("knowledge: concept kind %q is not one of %s", kind, strings.Join(ConceptKinds, ", "))
	}
	all, err := kb.Concepts()
	if err != nil {
		return nil, err
	}
	out := []Concept{}
	for _, c := range all {
		if c.Kind == kind {
			out = append(out, c)
		}
	}
	return out, nil
}
