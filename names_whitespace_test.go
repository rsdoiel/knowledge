package knowledge

import (
	"strings"
	"testing"
)

// A name is one line. CleanName used to trim the ends only, so a newline inside
// a name was kept: `project list` split a row, `record new --project` made a
// directory with a newline in its name, and, the route that matters, a
// hard-wrapped [[deterministic<newline>output]] in a document minted a concept
// with a newline in it, next to a normal "deterministic output". CleanName now
// collapses interior whitespace runs to one space and refuses any other control
// character (DR-0046, reversing DR-0042 item 1's "interior whitespace is left
// alone").

func TestCleanName_CollapsesInteriorWhitespace(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"a b", "a b"},
		{"a  b", "a b"},
		{"a\nb", "a b"},
		{"a\r\nb", "a b"},
		{"a\tb", "a b"},
		{"  first line\n  second line \n", "first line second line"},
		{"a \t\n b", "a b"},
		{"a b", "a b"},
		{"one two three", "one two three"},
	} {
		got, err := CleanName("concept", tc.in)
		if err != nil || got != tc.want {
			t.Errorf("CleanName(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}

func TestCleanName_RefusesControlCharacters(t *testing.T) {
	for _, in := range []string{"a\x00b", "a\x1b[31mred", "\x07", "a\x7fb", "a\u0090b", "tab\x0bvt\x00"} {
		if got, err := CleanName("project", in); err == nil {
			t.Errorf("CleanName(%q) = %q, want an error for the control character", in, got)
		}
	}
}

func TestCleanName_ControlCharacterErrorNamesTheKind(t *testing.T) {
	_, err := CleanName("concept", "a\x1bb")
	if err == nil {
		t.Fatal("want an error")
	}
	if got := err.Error(); !strings.Contains(got, "concept") || !strings.Contains(got, "control") {
		t.Errorf("error %q should name the kind and say control character", got)
	}
}

func TestAddProject_MultiLineNameIsOneLine(t *testing.T) {
	kb := openTestKB(t)
	want, err := kb.AddProject("first second", "d")
	if err != nil {
		t.Fatal(err)
	}
	got, err := kb.AddProject("first\nsecond", "")
	if err != nil {
		t.Fatalf("AddProject(multi-line): %v", err)
	}
	if got != want {
		t.Errorf("multi-line name gave id %d, want the existing 'first second' id %d", got, want)
	}
	if n := countRows(t, kb, "projects"); n != 1 {
		t.Errorf("projects rows = %d, want 1", n)
	}
	if _, err := kb.AddProject("bad\x1bname", ""); err == nil {
		t.Error("a project name with an ESC character was accepted")
	}
}

func TestAddConcept_MultiLineNameResolvesToTheSingleLineConcept(t *testing.T) {
	kb := openTestKB(t)
	want, err := kb.ResolveConceptName("deterministic output")
	if err != nil {
		t.Fatal(err)
	}
	got, err := kb.ResolveConceptName("deterministic\noutput")
	if err != nil {
		t.Fatalf("ResolveConceptName(wrapped): %v", err)
	}
	if got != want {
		t.Errorf("wrapped name gave concept %d, want %d", got, want)
	}
	if n := countRows(t, kb, "concepts"); n != 1 {
		t.Errorf("concepts rows = %d, want 1", n)
	}
	if _, err := kb.ResolveConceptName("bad\x00name"); err == nil {
		t.Error("a concept name with a NUL was accepted")
	}
}

func TestRenameProjectAndConcept_NewNameIsCleaned(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("old", ""); err != nil {
		t.Fatal(err)
	}
	if err := kb.RenameProject("old", "new\nname"); err != nil {
		t.Fatalf("RenameProject: %v", err)
	}
	if p, _ := kb.ProjectByName("new name"); p == nil {
		t.Error("project not found under the cleaned name 'new name'")
	}
	if _, err := kb.AddConcept("oldc", ""); err != nil {
		t.Fatal(err)
	}
	if err := kb.RenameConcept("oldc", "new\tconcept"); err != nil {
		t.Fatalf("RenameConcept: %v", err)
	}
	if n := countRows(t, kb, "concepts"); n != 1 {
		t.Errorf("concepts = %d, want 1", n)
	}
}

// ─── the route that matters: a hard-wrapped wikilink in a document ───────────

func TestIngestDocument_HardWrappedWikilinkIsTheOrdinaryConcept(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.md", "## S\n\nA paragraph that mentions [[deterministic\noutput]] across a wrap, and [[fine name]].\n")
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	all, _ := kb.Concepts()
	names := map[string]bool{}
	for _, c := range all {
		names[c.Name] = true
		for _, r := range c.Name {
			if r == '\n' || r == '\r' || r == '\t' {
				t.Errorf("concept %q contains a line break or tab", c.Name)
			}
		}
	}
	if !names["deterministic output"] || !names["fine name"] {
		t.Errorf("concepts = %v, want 'deterministic output' and 'fine name'", names)
	}
}

func TestIngestDocument_WrappedAndUnwrappedWikilinkAreOneLink(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.md", "## S\n\nFirst [[deterministic output]] then [[deterministic\noutput]] again.\n")
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	_, secs, _ := ingestSections(t, kb, path)
	cs, err := kb.DocumentSectionConcepts(secs["S"].ID)
	if err != nil || len(cs) != 1 || cs[0].Name != "deterministic output" {
		t.Errorf("concepts = %v, %v; want the one 'deterministic output'", cs, err)
	}
	if n := countRows(t, kb, "concepts"); n != 1 {
		t.Errorf("concepts rows = %d, want 1 (no duplicate for the wrapped spelling)", n)
	}
}

// A wikilink holding a control character is not a tag. It must not fail the
// whole document, and it must not mint anything.
func TestIngestDocument_ControlCharacterWikilinkIsSkippedNotFatal(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.md", "## S\n\nBad [[esc\x1bseq]] and good [[Keeper]].\n")
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument must not fail on one bad wikilink: %v", err)
	}
	all, _ := kb.Concepts()
	if len(all) != 1 || all[0].Name != "Keeper" {
		t.Errorf("concepts = %v, want only Keeper", all)
	}
}
