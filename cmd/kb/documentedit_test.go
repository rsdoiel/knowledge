package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// An interactive frontmatter editor for notes, offered at `kb document ingest`
// (TODO, requested 2026-10-16). A note with no frontmatter block is ingested as
// plain prose with no title, date or concepts; on a terminal the person is
// offered proposals to accept, change or skip, and the file is written only
// after they confirm.

const noteBody = "# Ownership verification\n\nBy Jane Doe\n\nWe checked toast ownership today.\n"

// toastNote mentions toast twice, the threshold at which a known concept is
// proposed as a keyword.
const toastNote = "# Ownership verification\n\nBy Jane Doe\n\nToast was checked. More toast was checked.\n"

func writeNote(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func edit(t *testing.T, kb *knowledge.KnowledgeBase, path, input string) (wrote bool, out string) {
	t.Helper()
	var buf bytes.Buffer
	wrote, err := editFrontmatter(kb, strings.NewReader(input), &buf, path)
	if err != nil {
		t.Fatalf("editFrontmatter: %v", err)
	}
	return wrote, buf.String()
}

func TestEditFrontmatter_EnterAcceptsEveryProposalAndYConfirms(t *testing.T) {
	kb := openTestKB(t)
	path := writeNote(t, "2026-09-29-ownership.md", noteBody)
	// title, author, dateCreated, keywords, confirm
	wrote, out := edit(t, kb, path, "\n\n\n\ny\n")
	if !wrote {
		t.Fatalf("nothing written:\n%s", out)
	}
	got, _ := os.ReadFile(path)
	for _, want := range []string{"title: Ownership verification", "author: Jane Doe", "dateCreated: \"2026-09-29\"", "# Ownership verification", "We checked toast"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("file lacks %q:\n%s", want, got)
		}
	}
	if !strings.HasPrefix(string(got), "---\n") {
		t.Errorf("no frontmatter block at the top:\n%s", got)
	}
	for _, want := range []string{"title [Ownership verification]", "author [Jane Doe]", "dateCreated [2026-09-29]"} {
		if !strings.Contains(out, want) {
			t.Errorf("prompts lack %q:\n%s", want, out)
		}
	}
}

func TestEditFrontmatter_TypedTextReplacesAndDashSkips(t *testing.T) {
	kb := openTestKB(t)
	path := writeNote(t, "2026-09-29-ownership.md", noteBody)
	wrote, out := edit(t, kb, path, "A Better Title\n-\n\n\ny\n")
	if !wrote {
		t.Fatalf("nothing written:\n%s", out)
	}
	got := string(mustRead(t, path))
	if !strings.Contains(got, "title: A Better Title") {
		t.Errorf("typed title not used:\n%s", got)
	}
	if strings.Contains(got, "author:") {
		t.Errorf("a skipped field was written:\n%s", got)
	}
	if !strings.Contains(got, "dateCreated") {
		t.Errorf("an accepted field is missing:\n%s", got)
	}
}

func TestEditFrontmatter_NoOrCancelLeavesTheFileAlone(t *testing.T) {
	for name, input := range map[string]string{
		"n at the confirmation": "\n\n\nn\n",
		"q at a prompt":         "\nq\n",
		"input ends early":      "\n",
		"empty input":           "",
	} {
		t.Run(name, func(t *testing.T) {
			kb := openTestKB(t)
			path := writeNote(t, "2026-09-29-ownership.md", noteBody)
			wrote, out := edit(t, kb, path, input)
			if wrote {
				t.Errorf("wrote the file:\n%s", out)
			}
			if got := string(mustRead(t, path)); got != noteBody {
				t.Errorf("file changed:\n%s", got)
			}
		})
	}
}

func TestEditFrontmatter_OffersKnownConceptsAsKeywords(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("toast", "")
	path := writeNote(t, "2026-09-29-ownership.md", toastNote)
	wrote, out := edit(t, kb, path, "\n\n\n\ny\n")
	if !wrote || !strings.Contains(out, "keywords [toast]") {
		t.Fatalf("keywords not offered:\n%s", out)
	}
	if got := string(mustRead(t, path)); !strings.Contains(got, "toast") || !strings.Contains(got, "keywords") {
		t.Errorf("keyword not written:\n%s", got)
	}
	// Choosing none, and choosing by name.
	path2 := writeNote(t, "2026-09-29-b.md", toastNote)
	edit(t, kb, path2, "\n\n\n-\ny\n")
	if got := string(mustRead(t, path2)); strings.Contains(got, "keywords") {
		t.Errorf("keywords written after '-':\n%s", got)
	}
}

func TestEditFrontmatter_NeverCreatesAConcept(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("toast", "")
	before, _ := kb.Concepts()
	path := writeNote(t, "2026-09-29-ownership.md", toastNote)
	edit(t, kb, path, "\n\n\n\ny\n")
	after, _ := kb.Concepts()
	if len(after) != len(before) {
		t.Errorf("concepts went from %d to %d: the editor offers known concepts only", len(before), len(after))
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// ─── the hook in `kb document ingest` ──────────────────────────────────────

func asTerminal(t *testing.T, input string) {
	t.Helper()
	oldT, oldIn := atTerminal, promptIn
	atTerminal = func(io.Writer) bool { return true }
	promptIn = strings.NewReader(input)
	t.Cleanup(func() { atTerminal, promptIn = oldT, oldIn })
}

func ingestNote(t *testing.T, kb *knowledge.KnowledgeBase, path string, extra ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := cmdDocumentIngest(kb, false, append([]string{path, "--project", "P"}, extra...), &out)
	return out.String(), err
}

func TestDocumentIngest_OffersTheEditorOnATerminalForANoteWithoutFrontmatter(t *testing.T) {
	kb := openTestKB(t)
	kb.AddProject("P", "")
	path := writeNote(t, "2026-09-29-ownership.md", noteBody)
	asTerminal(t, "\n\n\n\ny\n")
	if _, err := ingestNote(t, kb, path); err != nil {
		t.Fatal(err)
	}
	if got := string(mustRead(t, path)); !strings.HasPrefix(got, "---\n") || !strings.Contains(got, "title: Ownership verification") {
		t.Errorf("the note was not given frontmatter:\n%s", got)
	}
	docs, _ := kb.Documents(0)
	if len(docs) != 1 || docs[0].Title != "Ownership verification" || docs[0].Author != "Jane Doe" {
		t.Errorf("documents = %+v, want the title and author from the new frontmatter", docs)
	}
}

func TestDocumentIngest_NoPromptNonTerminalJSONAndDryRunNeverPrompt(t *testing.T) {
	for name, tc := range map[string]struct {
		term  bool
		json  bool
		extra []string
	}{
		"--no-prompt": {term: true, extra: []string{"--no-prompt"}},
		"no terminal": {term: false},
		"--json":      {term: true, json: true},
		"--dry-run":   {term: true, extra: []string{"--dry-run"}},
	} {
		t.Run(name, func(t *testing.T) {
			kb := openTestKB(t)
			kb.AddProject("P", "")
			path := writeNote(t, "2026-09-29-ownership.md", noteBody)
			if tc.term {
				asTerminal(t, "\n\n\n\ny\n")
			} else {
				noTerminal(t)
			}
			var out bytes.Buffer
			args := append([]string{path, "--project", "P"}, tc.extra...)
			if err := cmdDocumentIngest(kb, tc.json, args, &out); err != nil {
				t.Fatal(err)
			}
			if got := string(mustRead(t, path)); got != noteBody {
				t.Errorf("file changed:\n%s", got)
			}
			if strings.Contains(out.String(), "title [") {
				t.Errorf("prompted:\n%s", out.String())
			}
		})
	}
}

func TestDocumentIngest_AFileWithFrontmatterIsNotPrompted(t *testing.T) {
	kb := openTestKB(t)
	kb.AddProject("P", "")
	body := "---\ntitle: Already\n---\n\n# Heading\n\ntext\n"
	path := writeNote(t, "2026-09-29-x.md", body)
	asTerminal(t, "\n\n\n\ny\n")
	out, err := ingestNote(t, kb, path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "title [") || string(mustRead(t, path)) != body {
		t.Errorf("a file that has frontmatter was edited or prompted:\n%s", out)
	}
}

func TestDocumentIngest_CancellingTheEditorStillIngestsTheNote(t *testing.T) {
	kb := openTestKB(t)
	kb.AddProject("P", "")
	path := writeNote(t, "2026-09-29-ownership.md", noteBody)
	asTerminal(t, "q\n")
	if _, err := ingestNote(t, kb, path); err != nil {
		t.Fatal(err)
	}
	if docs, _ := kb.Documents(0); len(docs) != 1 {
		t.Errorf("documents = %+v, want the note ingested as before", docs)
	}
	if string(mustRead(t, path)) != noteBody {
		t.Error("file changed after cancelling")
	}
}
