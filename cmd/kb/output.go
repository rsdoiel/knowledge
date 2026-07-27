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

// printError writes err to errOut, never to stdout. In JSON mode it's a
// {"error": "..."} envelope; otherwise a plain "kb: <message>" line.
func printError(errOut io.Writer, jsonOut bool, err error) {
	if jsonOut {
		envelope := struct {
			Error string `json:"error"`
		}{Error: err.Error()}
		_ = printJSON(errOut, envelope)
		return
	}
	fmt.Fprintf(errOut, "kb: %v\n", err)
}
