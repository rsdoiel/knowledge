package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// ─── kb document fuzzy-tag (v0.0.11 item 1) ────────────────────────────────

func TestCmdDocumentFuzzyTag_DryRunWritesNothing(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	content := "## Background\n\nthe chunkings happen here.\n"
	path := writeDocFixture(t, root+"/docs", "a.md", content)
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	stdout, err := runDocument(t, kb, false, "fuzzy-tag", "--project", "alpha", "--dry-run")
	if err != nil {
		t.Fatalf("document fuzzy-tag --dry-run: %v", err)
	}
	if !strings.Contains(stdout, "chunking") {
		t.Errorf("dry-run output = %q, want it to mention chunking", stdout)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != content {
		t.Errorf("file content = %q, want unchanged by --dry-run", string(raw))
	}
}

func TestCmdDocumentFuzzyTag_InsertsFootnoteForPluralNearMiss(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	path := writeDocFixture(t, root+"/docs", "a.md", "## Background\n\nthe chunkings happen here.\n")
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if _, err := runDocument(t, kb, false, "fuzzy-tag", "--project", "alpha"); err != nil {
		t.Fatalf("document fuzzy-tag: %v", err)
	}

	raw, _ := os.ReadFile(path)
	got := string(raw)
	if !strings.Contains(got, "chunkings[^1]") {
		t.Errorf("file content = %q, want a [^1] marker after \"chunkings\"", got)
	}
	if !strings.Contains(got, "[^1]: see [[chunking]]") {
		t.Errorf("file content = %q, want a footnote definition linking [[chunking]]", got)
	}
}

func TestCmdDocumentFuzzyTag_PreservesOriginalWordSpelling(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	path := writeDocFixture(t, root+"/docs", "a.md", "## Background\n\nthe chunkings happen here.\n")
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if _, err := runDocument(t, kb, false, "fuzzy-tag", "--project", "alpha"); err != nil {
		t.Fatalf("document fuzzy-tag: %v", err)
	}

	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "the chunkings[^1] happen here") {
		t.Errorf("file content = %q, want \"chunkings\" itself byte-identical, only a marker appended", string(raw))
	}
}

func TestCmdDocumentFuzzyTag_SkipsWhenAlreadyExactlyLinkedElsewhere(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	content := "## Background\n\nsee [[chunking]] here. Also chunkings elsewhere.\n"
	path := writeDocFixture(t, root+"/docs", "a.md", content)
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	stdout, err := runDocument(t, kb, false, "fuzzy-tag", "--project", "alpha")
	if err != nil {
		t.Fatalf("document fuzzy-tag: %v", err)
	}
	if !strings.Contains(stdout, "nothing") {
		t.Errorf("output = %q, want a no-changes report", stdout)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != content {
		t.Errorf("file content = %q, want unchanged -- chunking is already an exact match in this file", string(raw))
	}
}

func TestCmdDocumentFuzzyTag_SecondRunIsIdempotent(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	path := writeDocFixture(t, root+"/docs", "a.md", "## Background\n\nthe chunkings happen here.\n")
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if _, err := runDocument(t, kb, false, "fuzzy-tag", "--project", "alpha"); err != nil {
		t.Fatalf("first document fuzzy-tag: %v", err)
	}
	once, _ := os.ReadFile(path)

	if _, err := runDocument(t, kb, false, "fuzzy-tag", "--project", "alpha"); err != nil {
		t.Fatalf("second document fuzzy-tag: %v", err)
	}
	twice, _ := os.ReadFile(path)

	if string(once) != string(twice) {
		t.Errorf("second run changed the file: %q -> %q", once, twice)
	}
	if strings.Count(string(twice), "[^1]: see [[chunking]]") != 1 {
		t.Errorf("file content = %q, want exactly one footnote definition, not doubled", string(twice))
	}
}

func TestCmdDocumentFuzzyTag_ConceptFlagBypassesDistanceThreshold(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	// "chunkage" is a distance-3 near-miss of "chunking" (8 chars, default
	// threshold 1) -- a real F1 candidate (within the ceiling of 3) that
	// the default per-length threshold rejects, and --concept must force
	// through.
	content := "## Background\n\nsome chunkage happens here.\n"
	path := writeDocFixture(t, root+"/docs", "a.md", content)
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	stdout, err := runDocument(t, kb, false, "fuzzy-tag", "--project", "alpha")
	if err != nil {
		t.Fatalf("document fuzzy-tag: %v", err)
	}
	if strings.Contains(stdout, "chunkage") {
		t.Errorf("output = %q, want \"chunkage\" not footnoted by default (distance 3 > threshold 1)", stdout)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != content {
		t.Errorf("file content = %q, want unchanged without --concept", string(raw))
	}

	if _, err := runDocument(t, kb, false, "fuzzy-tag", "--project", "alpha", "--concept", "chunking"); err != nil {
		t.Fatalf("document fuzzy-tag --concept chunking: %v", err)
	}
	raw, _ = os.ReadFile(path)
	got := string(raw)
	if !strings.Contains(got, "chunkage[^1]") || !strings.Contains(got, "[^1]: see [[chunking]]") {
		t.Errorf("file content = %q, want \"chunkage\" footnoted once --concept chunking bypasses the threshold", got)
	}
}

func TestCmdDocumentFuzzyTag_UnknownConceptFlagErrorsBeforeAnyFileOpens(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeDocFixture(t, root+"/docs", "a.md", "nothing relevant here\n")
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if _, err := runDocument(t, kb, false, "fuzzy-tag", "--project", "alpha", "--concept", "nonexistent"); err == nil {
		t.Error("expected an error for an unknown --concept name")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "nothing relevant here\n" {
		t.Errorf("file content = %q, want unchanged -- the error must be caught before any file is touched", string(raw))
	}
}

func TestCmdDocumentFuzzyTag_RequiresProject(t *testing.T) {
	kb := openTestKB(t)
	if _, err := runDocument(t, kb, false, "fuzzy-tag"); err == nil {
		t.Error("expected an error when --project is omitted")
	}
}

func TestCmdDocumentFuzzyTag_JSONOutputShape(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	path := writeDocFixture(t, root+"/docs", "a.md", "## Background\n\nthe chunkings happen here.\n")
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	stdout, err := runDocument(t, kb, true, "fuzzy-tag", "--project", "alpha")
	if err != nil {
		t.Fatalf("document fuzzy-tag --json: %v", err)
	}
	assertValidJSON(t, []byte(stdout))
	var got struct {
		Files []struct {
			Path      string `json:"path"`
			Footnoted []struct {
				Section  string `json:"section"`
				Text     string `json:"text"`
				Concept  string `json:"concept"`
				Distance int    `json:"distance"`
				Footnote int    `json:"footnote"`
			} `json:"footnoted"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decoding: %v\n%s", err, stdout)
	}
	if len(got.Files) != 1 || len(got.Files[0].Footnoted) != 1 {
		t.Fatalf("got = %+v, want one file with one footnoted entry", got)
	}
	e := got.Files[0].Footnoted[0]
	if e.Concept != "chunking" || e.Text != "chunkings" || e.Footnote != 1 || e.Section != "Background" {
		t.Errorf("footnoted entry = %+v, want concept=chunking text=chunkings footnote=1 section=Background", e)
	}
}

func TestCmdDocumentFuzzyTag_RollsBackAllFilesIfOneWriteFails(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	aContent := "## Background\n\nthe chunkings happen here.\n"
	aPath := writeDocFixture(t, root+"/docs", "a.md", aContent)
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: aPath}); err != nil {
		t.Fatalf("AddDocument a: %v", err)
	}
	bContent := "## Background\n\nthe chunkings happen here too.\n"
	bPath := writeDocFixture(t, root+"/docs", "b.md", bContent)
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "b", Format: "text", Path: bPath}); err != nil {
		t.Fatalf("AddDocument b: %v", err)
	}
	if err := os.Chmod(bPath, 0o444); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(bPath, 0o644) })

	if _, err := runDocument(t, kb, false, "fuzzy-tag", "--project", "alpha"); err == nil {
		t.Fatal("expected an error from the read-only second file's write")
	}

	raw, _ := os.ReadFile(aPath)
	if string(raw) != aContent {
		t.Errorf("a.md = %q, want rolled back to its original content after b.md's write failed", string(raw))
	}
}

func TestCmdDocumentFuzzyTag_NeverTouchesExactMatchInsertion(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	if _, err := kb.AddConcept("prompt", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	content := "## Background\n\nchunking is mentioned here. Also, a promt appears.\n"
	path := writeDocFixture(t, root+"/docs", "a.md", content)
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if _, err := runDocument(t, kb, false, "fuzzy-tag", "--project", "alpha"); err != nil {
		t.Fatalf("document fuzzy-tag: %v", err)
	}

	raw, _ := os.ReadFile(path)
	got := string(raw)
	if strings.Contains(got, "[[chunking]]") || strings.Contains(got, "[[Chunking]]") {
		t.Errorf("file content = %q, want the exact-match concept untouched -- only kb document tag inserts brackets", got)
	}
	if !strings.Contains(got, "promt[^1]") || !strings.Contains(got, "[^1]: see [[prompt]]") {
		t.Errorf("file content = %q, want the fuzzy near-miss \"promt\" footnoted", got)
	}
}

// Two fuzzy near-misses in the same section (no heading between them):
// insertFootnote's own two splices (marker at matchEnd, definition at the
// section boundary -- end of file here, since there's only one section)
// only shift positions *after* the first match's marker insertion point by
// the marker's own length until the section boundary is crossed, not by
// the marker+definition length combined. A flat, single running delta
// applied to every later match overcorrects any later match still in the
// same section, splicing its marker at the wrong byte offset.
func TestCmdDocumentFuzzyTag_HandlesTwoNearMissesInTheSameSection(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	if _, err := kb.AddConcept("prompt", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	content := "## Background\n\nthe chunkibg happens here and the promt appears there.\n"
	path := writeDocFixture(t, root+"/docs", "a.md", content)
	if _, err := kb.AddDocument(knowledge.Document{ProjectID: pid, Title: "a", Format: "text", Path: path}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if _, err := runDocument(t, kb, false, "fuzzy-tag", "--project", "alpha"); err != nil {
		t.Fatalf("document fuzzy-tag: %v", err)
	}

	raw, _ := os.ReadFile(path)
	got := string(raw)
	if !strings.Contains(got, "chunkibg[^1]") {
		t.Errorf("file content = %q, want a marker immediately after \"chunkibg\"", got)
	}
	if !strings.Contains(got, "promt[^2]") {
		t.Errorf("file content = %q, want a marker immediately after \"promt\", not spliced into the wrong byte offset", got)
	}
	if !strings.Contains(got, "[^1]: see [[chunking]]") || !strings.Contains(got, "[^2]: see [[prompt]]") {
		t.Errorf("file content = %q, want both footnote definitions present and correctly labeled", got)
	}
}
