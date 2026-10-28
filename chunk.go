package knowledge

import (
	"strings"
	"unicode/utf8"
)

/** DefaultChunkRunes is the longest chunk, in characters, that Chunk is asked
 * for when semantic search embeds a row: about 128 tokens. It is a starting
 * value, to be tuned against the labelled query set (semantic-search-plan.md T9).
 */
const DefaultChunkRunes = 400

/** Chunk splits text into pieces of at most max characters for embedding. It
 * breaks at paragraph boundaries where it can, then at word boundaries, and cuts
 * a single word only when that word is longer than max. Short paragraphs are
 * packed together. Text with no words gives no chunks. The result is the same
 * for the same input.
 *
 * Parameters:
 *   text (string) — the text to split
 *   max (int) — the most characters in a chunk; zero or less means DefaultChunkRunes
 *
 * Returns:
 *   []string — the chunks, in order, each non-empty and trimmed
 *
 * Example:
 *   for _, c := range knowledge.Chunk(body, knowledge.DefaultChunkRunes) {
 *       fmt.Println(len(c))
 *   }
 */
func Chunk(text string, max int) []string {
	if max <= 0 {
		max = DefaultChunkRunes
	}
	var (
		chunks []string
		cur    []string // paragraphs or word runs packed in the chunk being built
		curLen int
	)
	flush := func() {
		if len(cur) > 0 {
			chunks = append(chunks, strings.Join(cur, "\n\n"))
			cur, curLen = nil, 0
		}
	}
	for _, para := range splitParagraphs(text) {
		n := utf8.RuneCountInString(para)
		if n > max {
			flush()
			chunks = append(chunks, chunkWords(para, max)...)
			continue
		}
		if curLen > 0 && curLen+2+n > max {
			flush()
		}
		if curLen > 0 {
			curLen += 2
		}
		cur = append(cur, para)
		curLen += n
	}
	flush()
	return chunks
}

// splitParagraphs splits text at blank lines and returns each paragraph with its
// own whitespace collapsed to single spaces; empty paragraphs are dropped.
func splitParagraphs(text string) []string {
	var out []string
	var lines []string
	emit := func() {
		if p := strings.Join(strings.Fields(strings.Join(lines, " ")), " "); p != "" {
			out = append(out, p)
		}
		lines = lines[:0]
	}
	for _, ln := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(ln) == "" {
			emit()
			continue
		}
		lines = append(lines, ln)
	}
	emit()
	return out
}

// chunkWords packs the words of one over-long paragraph into chunks of at most
// max characters, cutting a word that is itself longer than max.
func chunkWords(para string, max int) []string {
	var chunks []string
	var b strings.Builder
	n := 0
	flush := func() {
		if n > 0 {
			chunks = append(chunks, b.String())
			b.Reset()
			n = 0
		}
	}
	for _, w := range strings.Fields(para) {
		for utf8.RuneCountInString(w) > max {
			flush()
			cut := 0
			for i := 0; i < max; i++ {
				_, size := utf8.DecodeRuneInString(w[cut:])
				cut += size
			}
			chunks = append(chunks, w[:cut])
			w = w[cut:]
		}
		wn := utf8.RuneCountInString(w)
		if n > 0 && n+1+wn > max {
			flush()
		}
		if n > 0 {
			b.WriteByte(' ')
			n++
		}
		b.WriteString(w)
		n += wn
	}
	flush()
	return chunks
}
