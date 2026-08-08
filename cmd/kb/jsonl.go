package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["export"] = cmdExport
	verbs["import"] = cmdImport
}

// countingWriter wraps an io.Writer and counts the newlines written
// through it, so cmdExport can report a line count without a second pass
// over the data.
type countingWriter struct {
	w     io.Writer
	lines int
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.lines += bytes.Count(p, []byte("\n"))
	return c.w.Write(p)
}

// cmdExport implements the export verb. With no -out, the JSON-L stream
// itself is written straight to out (stdout in normal use) regardless of
// --json -- there is no other sensible stdout output for this verb when
// piping into `kb import` or a file redirect is the point. --json only
// changes behavior when -out is given: it replaces the text confirmation
// line with a small {"lines_written","path"} envelope, since a raw JSON-L
// stream and a single JSON summary object can't share one stdout stream.
func cmdExport(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "export only this project and everything reachable from it")
	outPath := fs.String("out", "", "write to this path instead of stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dest := out
	if *outPath != "" {
		f, err := os.Create(*outPath)
		if err != nil {
			return fmt.Errorf("export: create %s: %w", *outPath, err)
		}
		defer f.Close()
		dest = f
	}

	cw := &countingWriter{w: dest}
	if err := logKBCallErr(dl, "ExportJSONL", map[string]any{"project": *project, "out": *outPath}, func() error {
		return knowledge.ExportJSONL(kb, cw, *project)
	}); err != nil {
		return err
	}

	if *outPath == "" {
		return nil
	}
	if jsonOut {
		return printJSON(out, struct {
			LinesWritten int    `json:"lines_written"`
			Path         string `json:"path"`
		}{LinesWritten: cw.lines, Path: *outPath})
	}
	fmt.Fprintf(out, "wrote %d line(s) to %s\n", cw.lines, *outPath)
	return nil
}

// cmdImport implements the import verb, reading a JSON-L stream (as
// produced by export) from -in, or stdin when -in is omitted, and applying
// it to the already-open kb.
func cmdImport(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	inPath := fs.String("in", "", "read from this path instead of stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}

	src := io.Reader(os.Stdin)
	if *inPath != "" {
		f, err := os.Open(*inPath)
		if err != nil {
			return fmt.Errorf("import: open %s: %w", *inPath, err)
		}
		defer f.Close()
		src = f
	}

	summary, err := logKBCall(dl, "ImportJSONL", map[string]any{"in": *inPath}, func() ([]knowledge.ImportTableSummary, error) {
		return knowledge.ImportJSONL(kb, src)
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, summary)
	}
	fmt.Fprintf(out, "%-20s %6s %8s %7s\n", "type", "read", "imported", "skipped")
	for _, s := range summary {
		fmt.Fprintf(out, "%-20s %6d %8d %7d\n", s.Table, s.Read, s.Imported, s.Skipped)
	}
	return nil
}
