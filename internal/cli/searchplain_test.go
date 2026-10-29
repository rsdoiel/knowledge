package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// plainSearchCases are plain (lexical) searches whose output must not change
// when the semantic flags are added to kb search (semantic plan T8). The golden
// file holds the output recorded before the flags existed; set
// KB_UPDATE_GOLDEN=1 to rewrite it on purpose.
var plainSearchCases = [][]string{
	{"search", "slow"},
	{"search", "inference"},
	{"search", "nonexistentzzqq"},
	{"search", "p/", "slow"},
	{"search", "/observations", "slow"},
	{"-json", "search", "slow"},
	{"-json", "search", "p/", "slow"},
	{"search", "--", "-dash"},
	{"search", "--json", "slow"},
}

func TestSearchPlain_OutputIsByteIdentical(t *testing.T) {
	pkg, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	f := newEmbedFixture(t, 3)
	f.obs("a different note about caching")
	var got strings.Builder
	for _, args := range plainSearchCases {
		var o, e bytes.Buffer
		code := mainRun(append([]string{"--db", f.db}, args...), &o, &e)
		got.WriteString("$ kb " + strings.Join(args, " ") + "\n")
		got.WriteString(o.String())
		got.WriteString("-- stderr --\n" + e.String())
		got.WriteString("-- exit " + strconv.Itoa(code) + " --\n\n")
	}
	golden := filepath.Join(pkg, "testdata", "search_plain.golden")
	if os.Getenv("KB_UPDATE_GOLDEN") != "" {
		os.MkdirAll(filepath.Dir(golden), 0o755)
		if err := os.WriteFile(golden, []byte(got.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Errorf("plain search output changed:\n--- got ---\n%s\n--- want ---\n%s", got.String(), want)
	}
}
