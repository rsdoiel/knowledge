package main

import (
	"io"

	knowledge "github.com/rsdoiel/knowledge"
)

// printHelp writes the help page for topic to out, substituting version
// metadata into the man-page-style header. topic == "" prints the main
// kb(1) page.
//
// Parameters:
//
//	out   (io.Writer) — destination writer.
//	topic (string)    — verb name or alias; "" for the main page.
//
// Returns:
//
//	bool — true if topic was recognized (or empty), false if unknown.
func printHelp(out io.Writer, topic string) bool {
	f := func(text string) {
		io.WriteString(out, knowledge.FmtHelp(text, "kb", knowledge.Version, knowledge.ReleaseDate, knowledge.ReleaseHash))
	}
	switch topic {
	case "":
		f(HelpText)
	case "project":
		f(ProjectHelpText)
	case "observation":
		f(ObservationHelpText)
	case "concept":
		f(ConceptHelpText)
	case "link":
		f(LinkHelpText)
	case "source":
		f(SourceHelpText)
	case "search", "summary", "format":
		f(SearchHelpText)
	case "merge":
		f(MergeHelpText)
	default:
		return false
	}
	return true
}
