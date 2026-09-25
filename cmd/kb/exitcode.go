package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"

	sqlite "github.com/glebarez/go-sqlite"
	knowledge "github.com/rsdoiel/knowledge"
)

/** exitClass is one row of the workspace exit-code table (workspace DR-0003,
 * applied to kb by DR-0047): a class name a machine-readable caller can match
 * on, and the number the process exits with.
 *
 * Fields:
 *   Name (string) — the class name in `--json` error output, for example
 *                   "no_input".
 *   Code (int)    — the process exit status.
 *
 * Example:
 *   os.Exit(classNoInput.Code) // 66
 */
type exitClass struct {
	Name string
	Code int
}

// The classes of the workspace table. 64 is deliberately absent: usage is 2.
var (
	classOK           = exitClass{"ok", 0}
	classNegative     = exitClass{"negative", 1}
	classUsage        = exitClass{"usage", 2}
	classData         = exitClass{"data", 65}
	classNoInput      = exitClass{"no_input", 66}
	classUnavailable  = exitClass{"unavailable", 69}
	classInternal     = exitClass{"internal", 70}
	classCantCreate   = exitClass{"cant_create", 73}
	classIO           = exitClass{"io", 74}
	classTempFail     = exitClass{"temp_fail", 75}
	classNoPermission = exitClass{"no_permission", 77}
	classConfig       = exitClass{"config", 78}
)

// allExitClasses lists every class, so a test can check the table as a whole.
var allExitClasses = []exitClass{
	classOK, classNegative, classUsage, classData, classNoInput, classUnavailable,
	classInternal, classCantCreate, classIO, classTempFail, classNoPermission, classConfig,
}

// unclassifiedFallback is the class an error that nothing classified exits
// with: internal (70), so a site nobody classified shows up as an "internal
// error" instead of hiding as a negative answer (DR-0047 item 4). Until X2 of
// exit-codes-plan.md it was negative (1), the code v0.0.13 gave every such error.
var unclassifiedFallback = classInternal

/** classedError carries an exit class with an error. Error and Unwrap forward
 * to the wrapped error, so a message reads exactly as it did before it was
 * classified, and errors.Is and errors.As reach the cause.
 *
 * Fields:
 *   class (exitClass) — the class the error exits with.
 *   err   (error)     — the underlying error.
 *
 * Example:
 *   return noInputf("%s does not exist", path)
 */
type classedError struct {
	class exitClass
	err   error
}

func (c *classedError) Error() string { return c.err.Error() }
func (c *classedError) Unwrap() error { return c.err }

/** classedAs marks an existing error with a class and leaves nil alone. It is
 * for reclassifying what a helper returned: a verb that reads files wraps the
 * library's invalid-value error as classData, where the same error from an
 * argument is a usage error. The outermost class wins.
 *
 * Parameters:
 *   class (exitClass) — the class to give the error.
 *   err   (error)     — the error to mark, or nil.
 *
 * Returns:
 *   error — nil for a nil err, otherwise a *classedError wrapping err.
 *
 * Example:
 *   return classedAs(classData, fmt.Errorf("%s: %w", path, err))
 */
func classedAs(class exitClass, err error) error {
	if err == nil {
		return nil
	}
	return &classedError{class: class, err: err}
}

// classErrorf is fmt.Errorf that marks its result with a class. %w works as it
// does in fmt.Errorf.
func classErrorf(class exitClass, format string, a ...any) error {
	return &classedError{class: class, err: fmt.Errorf(format, a...)}
}

/** negativef is fmt.Errorf that marks its result as a normal negative answer
 * (exit 1): the command ran correctly and the answer is no. Use it for a
 * refusal the current state forces, such as a concept that is still linked.
 *
 * Parameters:
 *   format (string) — a fmt.Errorf format.
 *   a      (...any) — the format's arguments.
 *
 * Returns:
 *   error — a *classedError of classNegative.
 *
 * Example:
 *   return negativef("concept %q is still linked; use --force", name)
 */
func negativef(format string, a ...any) error { return classErrorf(classNegative, format, a...) }

/** notFoundf is fmt.Errorf that marks its result as "no such item" (exit 1): a
 * name or id that is not in the knowledge base.
 *
 * Parameters:
 *   format (string) — a fmt.Errorf format.
 *   a      (...any) — the format's arguments.
 *
 * Returns:
 *   error — a *classedError of classNegative.
 *
 * Example:
 *   return notFoundf("project %q not found", name)
 */
func notFoundf(format string, a ...any) error { return classErrorf(classNegative, format, a...) }

/** dataErrorf is fmt.Errorf that marks its result as wrong content (exit 65):
 * a malformed record, JSONL, frontmatter or document, or a file that is not a
 * database. A bad value on the command line is a usage error instead.
 *
 * Parameters:
 *   format (string) — a fmt.Errorf format.
 *   a      (...any) — the format's arguments.
 *
 * Returns:
 *   error — a *classedError of classData.
 *
 * Example:
 *   return dataErrorf("%s: no frontmatter", path)
 */
func dataErrorf(format string, a ...any) error { return classErrorf(classData, format, a...) }

/** noInputf is fmt.Errorf that marks its result as a missing input (exit 66): a
 * named file or directory, or the workspace, that is not there or is the wrong
 * kind of thing.
 *
 * Parameters:
 *   format (string) — a fmt.Errorf format.
 *   a      (...any) — the format's arguments.
 *
 * Returns:
 *   error — a *classedError of classNoInput.
 *
 * Example:
 *   return noInputf("merge input -a: %s does not exist", path)
 */
func noInputf(format string, a ...any) error { return classErrorf(classNoInput, format, a...) }

/** cantCreatef is fmt.Errorf that marks its result as an output that cannot be
 * created (exit 73): the target exists, or its directory cannot be made.
 *
 * Parameters:
 *   format (string) — a fmt.Errorf format.
 *   a      (...any) — the format's arguments.
 *
 * Returns:
 *   error — a *classedError of classCantCreate.
 *
 * Example:
 *   return cantCreatef("%s already exists", path)
 */
func cantCreatef(format string, a ...any) error { return classErrorf(classCantCreate, format, a...) }

/** unavailablef is fmt.Errorf that marks its result as a service that cannot
 * be reached (exit 69), such as the retraction-check API.
 *
 * Parameters:
 *   format (string) — a fmt.Errorf format.
 *   a      (...any) — the format's arguments.
 *
 * Returns:
 *   error — a *classedError of classUnavailable.
 *
 * Example:
 *   return unavailablef("retraction service: %w", err)
 */
func unavailablef(format string, a ...any) error { return classErrorf(classUnavailable, format, a...) }

/** ioErrorf is fmt.Errorf that marks its result as a read or write that failed
 * part way (exit 74).
 *
 * Parameters:
 *   format (string) — a fmt.Errorf format.
 *   a      (...any) — the format's arguments.
 *
 * Returns:
 *   error — a *classedError of classIO.
 *
 * Example:
 *   return ioErrorf("writing %s: %w", path, err)
 */
func ioErrorf(format string, a ...any) error { return classErrorf(classIO, format, a...) }

/** classify returns the exit class of err, and whether anything classified it.
 * An explicit class (the outermost *classedError in the chain) wins. Otherwise
 * standard-library errors classify themselves: fs.ErrNotExist is no_input,
 * fs.ErrPermission no_permission, fs.ErrExist cant_create, any other file
 * error io, a network error unavailable, and a SQLite error by its result code
 * (see sqliteClass). The library's ErrNotFound and a *ConceptInUseError are
 * negative, and its ErrInvalid is usage. An error nothing classified is
 * reported as classInternal with classified false, so a caller can tell.
 * nil is classOK.
 *
 * Parameters:
 *   err (error) — the error to classify; may be nil.
 *
 * Returns:
 *   exitClass — the class.
 *   bool      — false only when nothing classified err.
 *
 * Example:
 *   class, ok := classify(fmt.Errorf("read: %w", fs.ErrNotExist)) // classNoInput, true
 */
func classify(err error) (exitClass, bool) {
	if err == nil {
		return classOK, true
	}
	var ce *classedError
	if errors.As(err, &ce) {
		return ce.class, true
	}
	// The library's own markers. An invalid value is a usage error when it came
	// from an argument, which is what the library cannot tell; a verb that reads
	// files wraps it with classedAs(classData, ...) so the outermost class wins.
	var inUse *knowledge.ConceptInUseError
	switch {
	case errors.Is(err, knowledge.ErrNotFound), errors.Is(err, knowledge.ErrInUse), errors.As(err, &inUse):
		return classNegative, true
	case errors.Is(err, knowledge.ErrInvalid):
		return classUsage, true
	}
	var se3 *sqlite.Error
	if errors.As(err, &se3) {
		return sqliteClass(se3.Code()), true
	}
	// Content that would not decode is wrong content, whichever verb read it.
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	if errors.As(err, &syn) || errors.As(err, &typ) || errors.Is(err, io.ErrUnexpectedEOF) {
		return classData, true
	}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return classNoInput, true
	case errors.Is(err, fs.ErrPermission):
		return classNoPermission, true
	case errors.Is(err, fs.ErrExist):
		return classCantCreate, true
	}
	var pe *fs.PathError
	var le *os.LinkError
	var se *os.SyscallError
	if errors.As(err, &pe) || errors.As(err, &le) || errors.As(err, &se) {
		return classIO, true
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return classUnavailable, true
	}
	return classInternal, false
}

/** exitCodeFor is the class dispatch exits with for err: what classify says,
 * except that an error nothing classified gets unclassifiedFallback.
 *
 * Parameters:
 *   err (error) — the error a verb returned; may be nil.
 *
 * Returns:
 *   exitClass — the class to exit with and to report in the JSON envelope.
 *
 * Example:
 *   os.Exit(exitCodeFor(err).Code)
 */
func exitCodeFor(err error) exitClass {
	class, classified := classify(err)
	if !classified {
		return unclassifiedFallback
	}
	return class
}

/** sqliteClass maps a SQLite result code to an exit class: busy or locked
 * (SQLITE_BUSY 5, SQLITE_LOCKED 6) is temp_fail, since a retry may succeed; a
 * corrupt file or one that is not a database (SQLITE_CORRUPT 11, SQLITE_NOTADB
 * 26) is data; any other code is io. Extended result codes carry the primary
 * code in their low byte.
 *
 * Parameters:
 *   code (int) — the driver's result code, possibly an extended one.
 *
 * Returns:
 *   exitClass — temp_fail, data or io.
 *
 * Example:
 *   sqliteClass(5) // classTempFail
 */
func sqliteClass(code int) exitClass {
	switch code & 0xff {
	case 5, 6:
		return classTempFail
	case 11, 26:
		return classData
	}
	return classIO
}

/** asContent reclassifies the library's invalid-value error as wrong content
 * (exit 65). A library ErrInvalid means usage (2) when it came from an argument,
 * which the library cannot tell; a verb that reads a file wraps the error it got
 * back with this, so a malformed record or document is 65. An error that already
 * carries an explicit class, or is not an ErrInvalid, is returned unchanged.
 *
 * Parameters:
 *   err (error) — an error from reading or parsing a file; may be nil.
 *
 * Returns:
 *   error — err, or err classed as data.
 *
 * Example:
 *   return asContent(fmt.Errorf("reading DR-%s: %w", id, err))
 */
func asContent(err error) error {
	var ce *classedError
	if err == nil || errors.As(err, &ce) || !errors.Is(err, knowledge.ErrInvalid) {
		return err
	}
	return classedAs(classData, err)
}

/** asCreate reclassifies a failure to create an output as cant_create (exit
 * 73): the file or directory could not be made. A missing parent or an existing
 * target is not "no input" here, it is an output that cannot be created. A
 * permission failure keeps its own class (77), a failure part way through
 * writing stays io (74), and an error that already carries an explicit class is
 * returned unchanged.
 *
 * Parameters:
 *   err (error) — an error from creating a file or directory; may be nil.
 *
 * Returns:
 *   error — err, or err classed as cant_create.
 *
 * Example:
 *   return asCreate(fmt.Errorf("creating %s: %w", dir, err))
 */
func asCreate(err error) error {
	var ce *classedError
	if err == nil || errors.As(err, &ce) || errors.Is(err, fs.ErrPermission) {
		return err
	}
	var pe *fs.PathError
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrExist) ||
		(errors.As(err, &pe) && (pe.Op == "open" || pe.Op == "mkdir" || pe.Op == "create")) {
		return classedAs(classCantCreate, err)
	}
	return err
}
