package knowledge

import (
	"errors"
	"fmt"
)

/** ErrInvalid marks an error the library raises because a value it was given is
 * not acceptable: a name that is blank, an observation kind or project status
 * outside its vocabulary, a malformed source date, url or DOI, a record or
 * frontmatter block that does not parse. Match it with errors.Is; the message is
 * unchanged. Whether it is a mistake in a command line or in a file's content is
 * the caller's to say: kb exits 2 for the first and 65 for the second (workspace
 * DR-0003, knowledge DR-0047).
 *
 * Example:
 *   if _, err := kb.AddObservation(id, "bogus", "text"); errors.Is(err, knowledge.ErrInvalid) {
 *       // the kind is not one of ValidObservationKinds
 *   }
 */
var ErrInvalid = errors.New("invalid")

/** ErrNotFound marks an error the library raises because a name it was asked
 * about is not in the knowledge base: a project or concept to rename, delete or
 * export. Match it with errors.Is; the message is unchanged. It is not used for
 * a lookup that returns (nil, nil), which is not an error.
 *
 * Example:
 *   if err := kb.RenameConcept("old", "new"); errors.Is(err, knowledge.ErrNotFound) {
 *       // there is no concept "old"
 *   }
 */
var ErrNotFound = errors.New("not found")

/** ErrConflict marks an error the library raises because the current state of
 * the knowledge base forbids an operation that was otherwise well formed: a
 * rename onto a name that is already taken, a project that still owns records, a
 * document section that is not in the state a promotion needs. Match it with
 * errors.Is; the message is unchanged. kb exits 1 for it (workspace DR-0003 class
 * "negative": the command ran correctly and the answer is no). An item that is
 * still referenced has its own marker, ErrInUse.
 *
 * Example:
 *   if err := kb.RenameConcept("old", "new"); errors.Is(err, knowledge.ErrConflict) {
 *       // "new" already exists
 *   }
 */
var ErrConflict = errors.New("conflict")

/** ErrInUse marks an error the library raises because an operation is refused
 * while the item is still referenced: a source still linked to an observation.
 * Match it with errors.Is; the message is unchanged. It is a refusal the current
 * state forces, so kb exits 1 for it (workspace DR-0003 class "negative"). A
 * concept still linked has its own typed error, *ConceptInUseError.
 *
 * Example:
 *   if err := kb.RemoveSource(1); errors.Is(err, knowledge.ErrInUse) {
 *       // unlink it first
 *   }
 */
var ErrInUse = errors.New("in use")

// sentinelError is an error with its own message that also matches a sentinel
// under errors.Is, so marking an error adds a class without adding a word to
// what a user reads. Unwrap reaches a cause given with %w, as fmt.Errorf's does.
type sentinelError struct {
	kind  error
	msg   string
	cause error
}

func (e *sentinelError) Error() string        { return e.msg }
func (e *sentinelError) Unwrap() error        { return e.cause }
func (e *sentinelError) Is(target error) bool { return target == e.kind }

// markedf is fmt.Errorf whose result also matches kind under errors.Is.
func markedf(kind error, format string, a ...any) error {
	err := fmt.Errorf(format, a...)
	return &sentinelError{kind: kind, msg: err.Error(), cause: errors.Unwrap(err)}
}

// invalidf is fmt.Errorf whose result matches ErrInvalid.
func invalidf(format string, a ...any) error { return markedf(ErrInvalid, format, a...) }

// conflictf is fmt.Errorf whose result matches ErrConflict.
func conflictf(format string, a ...any) error { return markedf(ErrConflict, format, a...) }

// inUsef is fmt.Errorf whose result matches ErrInUse.
func inUsef(format string, a ...any) error { return markedf(ErrInUse, format, a...) }

// notFoundf is fmt.Errorf whose result matches ErrNotFound.
func notFoundf(format string, a ...any) error { return markedf(ErrNotFound, format, a...) }
