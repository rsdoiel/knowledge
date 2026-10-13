package main

import (
	"regexp"
	"strings"
	"testing"
)

var ansiSeq = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestHighlightMarkdown(t *testing.T) {
	in := "---\nid: DR-0001\nstatus: proposed\n---\n\n# Title\n\n- item with `code` and **bold**\n> quoted\n\n```go\nx := 1\n```\n\nSee [kb](https://example.org).\n"
	got := highlightMarkdown(in)
	if plain := ansiSeq.ReplaceAllString(got, ""); plain != in {
		t.Fatalf("stripping the escapes does not give the input back:\n%q", plain)
	}
	for _, want := range []string{
		sgrKey + "status" + sgrReset,
		sgrHeading + "# Title" + sgrReset,
		sgrMarker + "-" + sgrReset,
		sgrCode + "`code`" + sgrReset,
		sgrBold + "**bold**" + sgrReset,
		sgrCode + "x := 1" + sgrReset,
		sgrLink + "kb" + sgrReset,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("highlight lacks %q in:\n%q", want, got)
		}
	}
	// A "key: value" line outside the frontmatter is prose, not a key.
	if body := highlightMarkdown("---\na: b\n---\nnote: plain\n"); !strings.Contains(body, "\nnote: plain") {
		t.Errorf("body line was treated as frontmatter: %q", body)
	}
}
