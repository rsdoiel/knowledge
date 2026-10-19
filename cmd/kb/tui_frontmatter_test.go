package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Documents → Frontmatter… (knowledge DR-0066, DR-0068): ask for the file, show
// what `kb document frontmatter` proposes as a checklist with nothing selected,
// then plan the chosen ones as a dry run and write only on `y`. Nothing is
// written without an explicit selection.

// frontmatterFile writes a document with no frontmatter that mentions a known
// concept, and returns the model, the file's path and the knowledge base.
func frontmatterFile(t *testing.T) (*tuiModel, string) {
	t.Helper()
	m, root, kb := layoutModel(t)
	if _, err := kb.AddConcept("caching", "keep it close"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "post.md")
	body := "# A Post About Caching\n\nBy Pat Example\n\nCaching keeps the answer close, so the second asking is fast.\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return m, path
}

func openFrontmatterPicker(t *testing.T, m *tuiModel, path string) *tuiModel {
	t.Helper()
	m = openLeaf(t, m, "Documents", "Frontmatter…")
	if m.state != viewForm {
		t.Fatalf("state = %s, want the path form\n%s", viewStateNames[m.state], m.View().Content)
	}
	m, _ = press(t, m, typed(path)...)
	m, _ = press(t, m, keyEnter)
	return m
}

func TestFrontmatter_ThePickerListsProposalsWithNothingSelected(t *testing.T) {
	m, path := frontmatterFile(t)
	m = openFrontmatterPicker(t, m, path)
	if m.state != viewPick {
		t.Fatalf("state = %s, want the picker\n%s", viewStateNames[m.state], m.View().Content)
	}
	v := m.View().Content
	for _, want := range []string{"title", "A Post About Caching", "caching", "[ ]", "space toggle"} {
		if !strings.Contains(v, want) {
			t.Errorf("the picker lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "[x]") {
		t.Errorf("nothing may be selected before the person chooses:\n%s", v)
	}
}

func TestFrontmatter_NothingSelectedWritesNothingAndSaysSo(t *testing.T) {
	m, path := frontmatterFile(t)
	before, _ := os.ReadFile(path)
	m = openFrontmatterPicker(t, m, path)
	m, _ = press(t, m, keyEnter)
	if m.state != viewPick || !strings.Contains(m.View().Content, "nothing is selected") {
		t.Errorf("state %s\n%s", viewStateNames[m.state], m.View().Content)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Errorf("the file changed")
	}
}

func TestFrontmatter_SelectThenReviewThenApply(t *testing.T) {
	m, path := frontmatterFile(t)
	m = openFrontmatterPicker(t, m, path)
	// The first selectable row is the title.
	m, _ = press(t, m, ch(' '))
	if !strings.Contains(m.View().Content, "[x]") {
		t.Fatalf("space should toggle the row:\n%s", m.View().Content)
	}
	m, _ = press(t, m, keyEnter)
	if m.state != viewPlan {
		t.Fatalf("state = %s, want the plan\n%s", viewStateNames[m.state], m.View().Content)
	}
	v := m.View().Content
	for _, want := range []string{"accepted", "kb document frontmatter", "--accept", "title"} {
		if !strings.Contains(v, want) {
			t.Errorf("the plan lacks %q:\n%s", want, v)
		}
	}
	// The plan is a dry run: the file is untouched until y.
	if b, _ := os.ReadFile(path); strings.Contains(string(b), "title:") {
		t.Fatalf("the dry run wrote the file:\n%s", b)
	}
	m, _ = press(t, m, ch('y'))
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "title:") || !strings.Contains(string(b), "A Post About Caching") {
		t.Errorf("the title was not written:\n%s", b)
	}
	if strings.Contains(string(b), "keywords:") {
		t.Errorf("an unselected keyword was written:\n%s", b)
	}
}

func TestFrontmatter_AKeywordIsSelectedAndWritten(t *testing.T) {
	m, path := frontmatterFile(t)
	m = openFrontmatterPicker(t, m, path)
	// Walk down to the caching row and select it alone.
	for i := 0; i < 12 && !strings.Contains(currentPickRow(m), "caching"); i++ {
		m, _ = press(t, m, ch('j'))
	}
	if !strings.Contains(currentPickRow(m), "caching") {
		t.Fatalf("could not reach the caching row:\n%s", m.View().Content)
	}
	m, _ = press(t, m, ch(' '))
	m, _ = press(t, m, keyEnter)
	if m.state != viewPlan || !strings.Contains(m.View().Content, "--accept-keywords caching") {
		t.Fatalf("state %s\n%s", viewStateNames[m.state], m.View().Content)
	}
	m, _ = press(t, m, ch('y'))
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "keywords") || !strings.Contains(string(b), "caching") {
		t.Errorf("the keyword was not written:\n%s", b)
	}
	if strings.Contains(string(b), "title:") {
		t.Errorf("an unselected title was written:\n%s", b)
	}
}

func TestFrontmatter_QLeavesWithoutWriting(t *testing.T) {
	m, path := frontmatterFile(t)
	before, _ := os.ReadFile(path)
	m = openFrontmatterPicker(t, m, path)
	m, _ = press(t, m, ch(' '), ch('q'))
	if m.state == viewPick {
		t.Errorf("q did not leave the picker")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Errorf("the file changed")
	}
}

func TestFrontmatter_APathThatIsNotAFileIsRefusedByTheForm(t *testing.T) {
	m, root, _ := layoutModel(t)
	m = openLeaf(t, m, "Documents", "Frontmatter…")
	m, _ = press(t, m, typed(filepath.Join(root, "nosuch.md"))...)
	m, _ = press(t, m, keyEnter)
	if m.state != viewForm {
		t.Errorf("state = %s, want the form still asking", viewStateNames[m.state])
	}
}

func TestFrontmatter_ADocumentWithNothingToProposeSaysSo(t *testing.T) {
	m, root, _ := layoutModel(t)
	path := filepath.Join(root, "done.md")
	// Every scalar field set; no concept is mentioned.
	body := "---\ntitle: Done\nauthor: Pat\ndateCreated: 2026-01-01\ndateModified: 2026-01-01\n---\n\nplain words only\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m = openFrontmatterPicker(t, m, path)
	if m.state == viewPick && strings.Contains(m.View().Content, "[ ]") && !strings.Contains(m.View().Content, "dateModified") {
		t.Errorf("nothing was expected to be selectable:\n%s", m.View().Content)
	}
}

// currentPickRow is the checklist row the cursor is on, as shown.
func currentPickRow(m *tuiModel) string {
	for _, l := range strings.Split(m.View().Content, "\n") {
		if strings.Contains(l, "> [") {
			return l
		}
	}
	return ""
}
