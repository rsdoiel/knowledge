package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// ─── kb document tag (TODO.md's programmatic-corpus-improvement item,
// wikilink-insertion half) ───────────────────────────────────────────────────

func TestCmdDocumentTag_TagsConceptMentionedTwice(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, err := kb.AddProject("alpha", "")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	path := writeDocFixture(t, root+"/docs", "a.md", "Chunking is discussed here. Chunking comes up again later.\n")
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if _, err := runDocument(t, kb, false, "tag", "--project", "alpha"); err != nil {
		t.Fatalf("document tag: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(raw), "[[Chunking]]") {
		t.Errorf("file content = %q, want [[Chunking]] inserted", string(raw))
	}
}

func TestCmdDocumentTag_SkipsSingleMentionWithoutExplicitConcept(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	path := writeDocFixture(t, root+"/docs", "a.md", "Chunking is mentioned just once here.\n")
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if _, err := runDocument(t, kb, false, "tag", "--project", "alpha"); err != nil {
		t.Fatalf("document tag: %v", err)
	}

	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "[[") {
		t.Errorf("file content = %q, want unchanged -- single mention, no --concept override", string(raw))
	}
}

func TestCmdDocumentTag_ExplicitConceptBypassesThreshold(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	path := writeDocFixture(t, root+"/docs", "a.md", "Chunking is mentioned just once here.\n")
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if _, err := runDocument(t, kb, false, "tag", "--project", "alpha", "--concept", "chunking"); err != nil {
		t.Fatalf("document tag: %v", err)
	}

	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "[[Chunking]]") {
		t.Errorf("file content = %q, want [[Chunking]] inserted despite the single mention", string(raw))
	}
}

func TestCmdDocumentTag_UnknownExplicitConceptIsErrorAndTouchesNothing(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.md", "nothing relevant here\n")
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if _, err := runDocument(t, kb, false, "tag", "--project", "alpha", "--concept", "nonexistent"); err == nil {
		t.Error("expected an error for an unknown --concept name")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "nothing relevant here\n" {
		t.Errorf("file content = %q, want unchanged -- the error must be caught before any file is touched", string(raw))
	}
}

func TestCmdDocumentTag_DryRunWritesNothing(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	content := "Chunking here. Chunking again.\n"
	path := writeDocFixture(t, root+"/docs", "a.md", content)
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	stdout, err := runDocument(t, kb, false, "tag", "--project", "alpha", "--dry-run")
	if err != nil {
		t.Fatalf("document tag --dry-run: %v", err)
	}
	if !strings.Contains(stdout, "chunking") {
		t.Errorf("dry-run output = %q, want it to mention chunking", stdout)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != content {
		t.Errorf("file content = %q, want unchanged by --dry-run", string(raw))
	}
}

func TestCmdDocumentTag_IdempotentOnSecondRun(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	path := writeDocFixture(t, root+"/docs", "a.md", "Chunking here. Chunking again.\n")
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if _, err := runDocument(t, kb, false, "tag", "--project", "alpha"); err != nil {
		t.Fatalf("first document tag: %v", err)
	}
	once, _ := os.ReadFile(path)

	if _, err := runDocument(t, kb, false, "tag", "--project", "alpha"); err != nil {
		t.Fatalf("second document tag: %v", err)
	}
	twice, _ := os.ReadFile(path)

	if string(once) != string(twice) {
		t.Errorf("second run changed the file: %q -> %q", once, twice)
	}
	if strings.Count(string(twice), "[[Chunking]]") != 1 {
		t.Errorf("file content = %q, want exactly one wikilink, not doubled", string(twice))
	}
}

func TestCmdDocumentTag_ScopedToProject(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	qid, _ := kb.AddProject("beta", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	content := "Chunking here. Chunking again.\n"
	betaPath := writeDocFixture(t, root+"/beta-docs", "b.md", content)
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: qid, Title: "b", Format: "text", Path: betaPath}); err != nil {
		t.Fatalf("AddDocument beta: %v", err)
	}
	alphaPath := writeDocFixture(t, root+"/alpha-docs", "a.md", "nothing about that here\n")
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: alphaPath}); err != nil {
		t.Fatalf("AddDocument alpha: %v", err)
	}

	if _, err := runDocument(t, kb, false, "tag", "--project", "alpha"); err != nil {
		t.Fatalf("document tag: %v", err)
	}

	raw, _ := os.ReadFile(betaPath)
	if string(raw) != content {
		t.Errorf("beta's document was touched by an --project alpha run: %q", string(raw))
	}
}

func TestCmdDocumentTag_RequiresProject(t *testing.T) {
	kb := openTestKB(t)
	if _, err := runDocument(t, kb, false, "tag"); err == nil {
		t.Error("expected an error when --project is omitted")
	}
}

func TestCmdDocumentTag_UnknownProject(t *testing.T) {
	kb := openTestKB(t)
	if _, err := runDocument(t, kb, false, "tag", "--project", "nonexistent"); err == nil {
		t.Error("expected an error for a nonexistent project")
	}
}

func TestCmdDocumentTag_JSONOutput(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	path := writeDocFixture(t, root+"/docs", "a.md", "Chunking here. Chunking again.\n")
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	stdout, err := runDocument(t, kb, true, "tag", "--project", "alpha")
	if err != nil {
		t.Fatalf("document tag --json: %v", err)
	}
	assertValidJSON(t, []byte(stdout))
	var got struct {
		Files []struct {
			Path     string   `json:"path"`
			Inserted []string `json:"inserted"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decoding: %v\n%s", err, stdout)
	}
	if len(got.Files) != 1 || len(got.Files[0].Inserted) != 1 || got.Files[0].Inserted[0] != "chunking" {
		t.Errorf("got = %+v, want one file with chunking inserted", got)
	}
}

// "tag" alone already appears incidentally (tag_density, concept-tag
// query), so this checks for the subcommand's actual synopsis line rather
// than a substring that would pass without any real documentation.
func TestDocumentHelpText_DocumentsTag(t *testing.T) {
	if !strings.Contains(DocumentHelpText, "document tag --project") {
		t.Error("DocumentHelpText does not document the tag subcommand's synopsis")
	}
}

func TestDocumentHelpText_DocumentsFuzzyTag(t *testing.T) {
	if !strings.Contains(DocumentHelpText, "document fuzzy-tag --project") {
		t.Error("DocumentHelpText does not document the fuzzy-tag subcommand's synopsis")
	}
}

func TestDocumentHelpText_DocumentsFrontmatter(t *testing.T) {
	if !strings.Contains(DocumentHelpText, "document frontmatter PATH") {
		t.Error("DocumentHelpText does not document the frontmatter subcommand's synopsis")
	}
}
