package main

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

// printError writes err to errOut, never to stdout. In JSON mode it's an
// {"error": "...", "class": "...", "code": N} envelope, where class and code
// are the exit class and status the process exits with (workspace DR-0003);
// otherwise a plain "kb: <message>" line.
func printError(errOut io.Writer, jsonOut bool, err error) {
	if jsonOut {
		class := exitCodeFor(err)
		envelope := struct {
			Error string `json:"error"`
			Class string `json:"class"`
			Code  int    `json:"code"`
		}{Error: err.Error(), Class: class.Name, Code: class.Code}
		_ = printJSON(errOut, envelope)
		return
	}
	fmt.Fprintf(errOut, "kb: %v\n", err)
}
