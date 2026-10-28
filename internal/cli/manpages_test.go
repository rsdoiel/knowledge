package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// kb-TOPIC.1.md is generated from the help text in helptext.go (make
// kb-topics-help runs `kb TOPIC -help` into it), so the Go source is the one to
// edit. An edit made only to the .1.md is erased by the next regeneration and
// never reaches `kb help TOPIC`; this test finds that. It compares everything
// below the three header lines, which carry the version, hash and date.

func bodyBelowHeader(s string) string {
	lines := strings.SplitN(s, "\n", 4)
	if len(lines) < 4 {
		return s
	}
	return lines[3]
}

func TestManPages_MatchTheirHelpText(t *testing.T) {
	for _, topic := range makefileTopics(t) {
		t.Run(topic, func(t *testing.T) {
			var out bytes.Buffer
			if !printHelp(&out, topic) {
				t.Fatalf("no help topic %q", topic)
			}
			onDisk, err := os.ReadFile("../../kb-" + topic + ".1.md")
			if err != nil {
				t.Fatalf("read the man page source: %v", err)
			}
			if bodyBelowHeader(out.String()) != bodyBelowHeader(string(onDisk)) {
				t.Errorf("kb-%s.1.md differs from `kb help %s`; edit helptext.go and regenerate the page with `kb %s -help > kb-%s.1.md`", topic, topic, topic, topic)
			}
		})
	}
}

// makefileTopics reads the verb list that drives kb-topics-help.
func makefileTopics(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatalf("read the Makefile: %v", err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "KB_TOPICS") {
			_, list, _ := strings.Cut(line, "=")
			return strings.Fields(list)
		}
	}
	t.Fatal("no KB_TOPICS assignment found in the Makefile")
	return nil
}

// A man page is for a person who installed kb; TODO.md is the developers' file
// and is not shipped with it.
func TestManPages_DoNotPointAtTODO(t *testing.T) {
	for _, topic := range append(makefileTopics(t), "") { // "" is kb(1) itself
		var out bytes.Buffer
		if !printHelp(&out, topic) {
			continue
		}
		if strings.Contains(out.String(), "TODO.md") {
			t.Errorf("kb help %s refers to TODO.md, which a user of kb does not have", topic)
		}
	}
}

// makeVariable asks make for the value of a variable in a Makefile copied to an
// empty directory, so no kb-*.1.md file is there to be globbed, and returns it with
// make's stderr.
func makeVariable(t *testing.T, name string) (value, stderr string) {
	t.Helper()
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("no make")
	}
	b, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatalf("read the Makefile: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("make", "-s", "-f", "Makefile", "--no-builtin-rules", "--no-builtin-variables",
		"--eval", "zz-print-%: ; @echo '$($*)'", "zz-print-"+name)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("make: %v\n%s", err, errOut.String())
	}
	return strings.TrimSpace(out.String()), errOut.String()
}

// The list of pages `make man` builds comes from KB_TOPICS, not from a glob over
// the kb-*.1.md files that happen to exist: a missing source file would otherwise
// drop its page from `make man` and `make install` and still report success.
func TestMakefile_ManPagesComeFromTheTopicList(t *testing.T) {
	got, _ := makeVariable(t, "MAN_PAGES_1")
	have := map[string]bool{}
	for _, f := range strings.Fields(got) {
		have[f] = true
	}
	want := []string{"kb.1"}
	for _, topic := range makefileTopics(t) {
		want = append(want, "kb-"+topic+".1")
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("MAN_PAGES_1 = %q lacks %s, though no .1.md file needs to exist for it to be listed", got, w)
		}
	}
	if len(have) != len(want) {
		t.Errorf("MAN_PAGES_1 has %d pages, want %d: kb and one for each topic", len(have), len(want))
	}
}

// Every man page source in the directory is built: a page added by hand without a
// topic is a topic missing from KB_TOPICS, and this says so.
func TestMakefile_EveryManPageSourceIsListed(t *testing.T) {
	got, _ := makeVariable(t, "MAN_PAGES_1")
	have := map[string]bool{}
	for _, f := range strings.Fields(got) {
		have[f] = true
	}
	files, _ := filepath.Glob("../../*.1.md")
	if len(files) == 0 {
		t.Fatal("no .1.md files found")
	}
	for _, f := range files {
		page := strings.TrimSuffix(filepath.Base(f), ".md")
		if !have[page] {
			t.Errorf("%s is a man page source, but %s is not in MAN_PAGES_1: add its topic to KB_TOPICS", filepath.Base(f), page)
		}
	}
}

// With no section 3 or 7 pages the build says nothing about them; the globs used
// to print "ls: cannot access '*.3.md'" on every build.
func TestMakefile_NoManSectionListIsNoisy(t *testing.T) {
	for _, v := range []string{"MAN_PAGES_3", "MAN_PAGES_7", "DOCS"} {
		_, stderr := makeVariable(t, v)
		for _, line := range strings.Split(stderr, "\n") {
			// The empty directory has no .go files either; that is not the lists.
			if strings.Contains(line, ".md") {
				t.Errorf("reading %s printed %q", v, line)
			}
		}
	}
}

// kb(1) names every manual page in SEE ALSO. The verb pages were added one at a
// time and the list was not: unlink, check-db, stats, story and verbs were missing
// though the VERBS section mentions them.
func TestManPages_KbSeeAlsoListsEveryPage(t *testing.T) {
	var out bytes.Buffer
	if !printHelp(&out, "") {
		t.Fatal("no kb help")
	}
	_, seeAlso, ok := strings.Cut(out.String(), "# SEE ALSO")
	if !ok {
		t.Fatal("kb(1) has no SEE ALSO")
	}
	for _, topic := range makefileTopics(t) {
		if page := "kb-" + topic + "(1)"; !strings.Contains(seeAlso, page) {
			t.Errorf("kb(1) SEE ALSO lacks %s", page)
		}
	}
}
