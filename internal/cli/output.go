package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

/** bulkError is what a bulk command returns when some items failed. Err is the
 * one-line count that goes to standard error; Items are the failures, each with
 * its own class, and under --json each is written as its own error object
 * (workspace DR-0011) in place of one object for the whole run. Error, Unwrap
 * and the exit class come from Err.
 *
 * Example:
 *   return &bulkError{Err: classedAs(class, fmt.Errorf("%d failed", n)), Items: failures}
 */
type bulkError struct {
	Err   error
	Items []error
}

func (b *bulkError) Error() string { return b.Err.Error() }
func (b *bulkError) Unwrap() error { return b.Err }

// printJSON writes v to out as indented JSON.
func printJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printError reports err. The human line "kb: <message>" always goes to errOut,
// as it did before, so a log or a terminal shows it. In JSON mode the error is
// also written to out as an {"error": "...", "class": "...", "code": N}
// object, where class and code are the exit class and status the process exits
// with (workspace DR-0003), so that standard output is always JSON a program
// can read, an error included (workspace DR-0011).
func printError(out, errOut io.Writer, jsonOut bool, err error) {
	fmt.Fprintf(errOut, "kb: %v\n", err)
	if jsonOut {
		var bulk *bulkError
		if errors.As(err, &bulk) && len(bulk.Items) > 0 {
			for _, item := range bulk.Items {
				writeEnvelope(out, item)
			}
			return
		}
		writeEnvelope(out, err)
	}
}

// writeEnvelope writes err to out as the DR-0011 error object.
func writeEnvelope(out io.Writer, err error) {
	class := exitCodeFor(err)
	envelope := struct {
		Error string `json:"error"`
		Class string `json:"class"`
		Code  int    `json:"code"`
	}{Error: err.Error(), Class: class.Name, Code: class.Code}
	_ = printJSON(out, envelope)
}
