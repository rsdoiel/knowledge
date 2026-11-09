package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

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
		class := exitCodeFor(err)
		envelope := struct {
			Error string `json:"error"`
			Class string `json:"class"`
			Code  int    `json:"code"`
		}{Error: err.Error(), Class: class.Name, Code: class.Code}
		_ = printJSON(out, envelope)
	}
}
