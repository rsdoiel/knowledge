package knowledge

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestChunk_NothingToChunk(t *testing.T) {
	for _, in := range []string{"", " ", "\n\n\t "} {
		if got := Chunk(in, 100); len(got) != 0 {
			t.Errorf("Chunk(%q) = %q, want none", in, got)
		}
	}
}

func TestChunk_ShortTextIsOneChunk(t *testing.T) {
	got := Chunk("  a short note \n", 100)
	if len(got) != 1 || got[0] != "a short note" {
		t.Errorf("Chunk = %q, want [\"a short note\"]", got)
	}
}

func TestChunk_NeverLongerThanMaxAndBreaksOnWords(t *testing.T) {
	text := strings.Repeat("alpha beta gamma delta epsilon ", 40)
	const max = 60
	got := Chunk(text, max)
	if len(got) < 2 {
		t.Fatalf("got %d chunks, want several", len(got))
	}
	for _, c := range got {
		if n := utf8.RuneCountInString(c); n > max || n == 0 {
			t.Errorf("chunk of %d runes (max %d): %q", n, max, c)
		}
		for _, w := range strings.Fields(c) {
			switch w {
			case "alpha", "beta", "gamma", "delta", "epsilon":
			default:
				t.Errorf("a word was cut: %q in %q", w, c)
			}
		}
	}
	if joined := strings.Join(strings.Fields(strings.Join(got, " ")), " "); joined != strings.Join(strings.Fields(text), " ") {
		t.Error("the chunks do not add up to the text: words were lost or reordered")
	}
}

func TestChunk_PrefersParagraphBreaks(t *testing.T) {
	text := "first paragraph here.\n\nsecond paragraph here.\n\nthird one."
	got := Chunk(text, 25)
	want := []string{"first paragraph here.", "second paragraph here.", "third one."}
	if len(got) != len(want) {
		t.Fatalf("Chunk = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("chunk %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestChunk_PacksShortParagraphsTogether(t *testing.T) {
	got := Chunk("one.\n\ntwo.\n\nthree.", 100)
	if len(got) != 1 {
		t.Errorf("Chunk = %q, want one chunk", got)
	}
}

func TestChunk_AWordLongerThanMaxIsCutAtMax(t *testing.T) {
	got := Chunk("x "+strings.Repeat("y", 25)+" z", 10)
	for _, c := range got {
		if utf8.RuneCountInString(c) > 10 {
			t.Errorf("chunk %q is longer than 10 runes", c)
		}
	}
	if strings.ReplaceAll(strings.Join(got, ""), " ", "") != "x"+strings.Repeat("y", 25)+"z" {
		t.Errorf("Chunk = %q, lost or reordered letters", got)
	}
}

func TestChunk_CountsRunesNotBytes(t *testing.T) {
	text := strings.Repeat("日本語のテキスト ", 20)
	for _, c := range Chunk(text, 12) {
		if n := utf8.RuneCountInString(c); n > 12 {
			t.Errorf("chunk of %d runes (max 12): %q", n, c)
		}
		if !utf8.ValidString(c) {
			t.Errorf("chunk is not valid UTF-8: %q", c)
		}
	}
}

func TestChunk_Deterministic(t *testing.T) {
	text := strings.Repeat("some words to split up. ", 50)
	a, b := Chunk(text, 70), Chunk(text, 70)
	if strings.Join(a, "|") != strings.Join(b, "|") {
		t.Error("two runs gave different chunks")
	}
}

func TestChunk_NonPositiveMaxUsesTheDefault(t *testing.T) {
	text := strings.Repeat("word ", 500)
	for _, c := range Chunk(text, 0) {
		if utf8.RuneCountInString(c) > DefaultChunkRunes {
			t.Errorf("chunk longer than DefaultChunkRunes (%d)", DefaultChunkRunes)
		}
	}
}
