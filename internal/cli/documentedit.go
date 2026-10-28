package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

// promptIn is where the frontmatter editor reads the person's answers; tests
// replace it. It is read only after atTerminal says there is a terminal.
var promptIn io.Reader = os.Stdin

// hasFrontmatterBlock reports whether raw starts with a YAML frontmatter block.
func hasFrontmatterBlock(raw []byte) bool {
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	return bytes.HasPrefix(raw, []byte("---\n")) || bytes.HasPrefix(raw, []byte("---\r\n"))
}

// promptLine prints a prompt and reads one answer. ok is false when the input
// has ended with nothing more to read. The answer is trimmed.
func promptLine(r *bufio.Reader, out io.Writer, prompt string) (answer string, ok bool) {
	fmt.Fprint(out, prompt)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(out)
		return "", false
	}
	return strings.TrimSpace(line), true
}

// wantsFrontmatter reports whether a file should be offered the editor: it has
// no frontmatter block, or it has one with no title (absent or blank). A block
// that cannot be read is left to ingest, which warns about it.
func wantsFrontmatter(kb *knowledge.KnowledgeBase, path string, raw []byte) bool {
	if !hasFrontmatterBlock(raw) {
		return true
	}
	res, err := kb.ProposeFrontmatter(path, raw, nil)
	if err != nil {
		return false
	}
	for _, f := range res.Fields {
		if f.Field == "title" {
			return f.Current == ""
		}
	}
	return false
}

/** editFrontmatter offers frontmatter for a file that has none, or has a block
 * with no title, one field at a time, asking only about the fields the file
 * does not already have, and writes it only after the person confirms. It proposes with
 * ProposeFrontmatter (title from the first heading, author from a byline or git,
 * dateCreated from a YYYY-MM-DD in the file name, git or the file's time) and
 * offers the known concepts the text mentions as keywords. It never creates a
 * concept. At each prompt Enter accepts the proposal, text replaces it, "-"
 * skips the field, and "q" stops with nothing written. Input that ends counts
 * as "q".
 *
 * Parameters:
 *   kb   (*knowledge.KnowledgeBase) — for the proposals and the known concepts.
 *   in   (io.Reader)                — the person's answers.
 *   out  (io.Writer)                — the prompts.
 *   path (string)                   — the file to edit.
 *
 * Returns:
 *   bool  — true when the file was written.
 *   error — a failed read, proposal or write.
 *
 * Example:
 *   wrote, err := editFrontmatter(kb, os.Stdin, os.Stdout, "notes/2026-09-29-worklog.md")
 */
func editFrontmatter(kb *knowledge.KnowledgeBase, in io.Reader, out io.Writer, path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	res, err := kb.ProposeFrontmatter(path, raw, nil)
	if err != nil {
		return false, err
	}
	r := bufio.NewReader(in)
	has := "no frontmatter"
	if hasFrontmatterBlock(raw) {
		has = "frontmatter but no title"
	}
	fmt.Fprintf(out, "%s has %s. Enter accepts, text replaces, - skips, q stops.\n", path, has)

	set := map[string]string{}
	var order []string
	for _, field := range []string{"title", "author", "dateCreated"} {
		var proposed, current string
		var signals []knowledge.FrontmatterSignal
		for _, f := range res.Fields {
			if f.Field == field {
				proposed, current, signals = f.Proposed, f.Current, f.Signals
			}
		}
		if current != "" {
			continue // the file already has it: not asked, not changed
		}
		if len(signals) > 1 {
			var parts []string
			for _, s := range signals {
				parts = append(parts, s.Source+": "+s.Value)
			}
			fmt.Fprintf(out, "  %s signals: %s\n", field, strings.Join(parts, "; "))
		}
		prompt := field + ": "
		if proposed != "" {
			prompt = fmt.Sprintf("%s [%s]: ", field, proposed)
		}
		answer, ok := promptLine(r, out, prompt)
		if !ok || answer == "q" {
			fmt.Fprintln(out, "stopped; nothing written")
			return false, nil
		}
		switch {
		case answer == "-":
		case answer != "":
			set[field] = answer
			order = append(order, field)
		case proposed != "":
			set[field] = proposed
			order = append(order, field)
		}
	}

	var keywords []string
	if known := append([]string(nil), res.Keywords.Known...); len(known) > 0 {
		sort.Strings(known)
		answer, ok := promptLine(r, out, fmt.Sprintf("keywords [%s] (Enter accepts all, names choose, - none): ", strings.Join(known, ", ")))
		if !ok || answer == "q" {
			fmt.Fprintln(out, "stopped; nothing written")
			return false, nil
		}
		switch answer {
		case "":
			keywords = known
		case "-":
		default:
			concepts, err := kb.Concepts()
			if err != nil {
				return false, err
			}
			exists := map[string]bool{}
			for _, c := range concepts {
				exists[c.Name] = true
			}
			for _, n := range strings.Split(answer, ",") {
				if n = strings.TrimSpace(n); n == "" {
					continue
				}
				if !exists[n] {
					fmt.Fprintf(out, "  %q is not a known concept; skipped\n", n)
					continue
				}
				keywords = append(keywords, n)
			}
		}
	}

	if len(set) == 0 && len(keywords) == 0 {
		fmt.Fprintln(out, "nothing to write")
		return false, nil
	}
	fmt.Fprintln(out, "will write:")
	for _, f := range order {
		fmt.Fprintf(out, "  %s: %s\n", f, set[f])
	}
	if len(keywords) > 0 {
		fmt.Fprintf(out, "  keywords: %s\n", strings.Join(keywords, ", "))
	}
	answer, ok := promptLine(r, out, "write it to the file? [Y/n] ")
	if !ok || !(answer == "" || strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")) {
		fmt.Fprintln(out, "nothing written")
		return false, nil
	}
	next, _, err := kb.ApplyFrontmatter(path, raw, nil, knowledge.FrontmatterAccept{Keywords: keywords, Set: set}, false)
	if err != nil {
		return false, err
	}
	if err := writeFileAtomic(path, next); err != nil {
		return false, err
	}
	fmt.Fprintf(out, "wrote frontmatter to %s\n", path)
	return true, nil
}
