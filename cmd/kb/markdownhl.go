package main

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SGR sequences used by highlightMarkdown. Kept to the 16 basic colours so they
// follow the terminal's own theme.
const (
	sgrReset   = "\x1b[0m"
	sgrDim     = "\x1b[2m"
	sgrBold    = "\x1b[1m"
	sgrHeading = "\x1b[1;35m"
	sgrKey     = "\x1b[36m"
	sgrCode    = "\x1b[33m"
	sgrFence   = "\x1b[32m"
	sgrMarker  = "\x1b[34m"
	sgrLink    = "\x1b[4;34m"
	sgrQuote   = "\x1b[3;2m"
)

var (
	mdHeading = regexp.MustCompile(`^#{1,6}\s`)
	mdList    = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])(\s)`)
	mdKey     = regexp.MustCompile(`^([A-Za-z_][\w.-]*)(:)(.*)$`)
	mdCode    = regexp.MustCompile("`[^`]+`")
	mdBold    = regexp.MustCompile(`\*\*[^*]+\*\*`)
	mdLink    = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

/** highlightMarkdown colours Markdown text with ANSI escape sequences: YAML
 * frontmatter keys, headings, fenced code, list markers, block quotes, and
 * inline code, bold and links. It is line oriented, not a Markdown parser, which
 * is enough for the records kb shows. Line structure is preserved: the result
 * has the same lines as the input, so it can be wrapped or scrolled afterwards.
 *
 * Parameters:
 *   text (string) — the Markdown, usually a record file
 *
 * Returns:
 *   string — text with escape sequences added; strip them and the input returns
 *
 * Example:
 *   fmt.Print(highlightMarkdown("# Title\n\nSome `code`.\n"))
 */
func highlightMarkdown(text string) string {
	lines := strings.Split(text, "\n")
	inFront, inFence := false, false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case i == 0 && trimmed == "---":
			inFront = true
			lines[i] = sgrDim + line + sgrReset
		case inFront && trimmed == "---":
			inFront = false
			lines[i] = sgrDim + line + sgrReset
		case inFront:
			if m := mdKey.FindStringSubmatch(line); m != nil {
				lines[i] = sgrKey + m[1] + sgrReset + sgrDim + m[2] + sgrReset + m[3]
			}
		case strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~"):
			inFence = !inFence
			lines[i] = sgrFence + line + sgrReset
		case inFence:
			lines[i] = sgrCode + line + sgrReset
		case mdHeading.MatchString(line):
			lines[i] = sgrHeading + line + sgrReset
		case strings.HasPrefix(trimmed, ">"):
			lines[i] = sgrQuote + line + sgrReset
		default:
			lines[i] = highlightInline(mdList.ReplaceAllString(line, "$1"+sgrMarker+"$2"+sgrReset+"$3"))
		}
	}
	return strings.Join(lines, "\n")
}

// highlightInline colours inline code, bold and links within one line.
func highlightInline(line string) string {
	line = mdCode.ReplaceAllStringFunc(line, func(s string) string { return sgrCode + s + sgrReset })
	line = mdBold.ReplaceAllStringFunc(line, func(s string) string { return sgrBold + s + sgrReset })
	return mdLink.ReplaceAllString(line, "["+sgrLink+"$1"+sgrReset+"]"+sgrDim+"($2)"+sgrReset)
}

/** wrapHighlighted highlights Markdown and wraps it to a width in one step. The
 * text is highlighted before it is wrapped, so a fence or a frontmatter block is
 * seen whole, and the wrapper carries a style across the break it makes.
 *
 * Parameters:
 *   text (string) — the Markdown
 *   width (int) — the column width to wrap to
 *
 * Returns:
 *   string — the highlighted text, wrapped, lines joined by newlines
 *
 * Example:
 *   vp.SetContent(wrapHighlighted(string(raw), vp.Width))
 */
func wrapHighlighted(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(highlightMarkdown(text))
}
