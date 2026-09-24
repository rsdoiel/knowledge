package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The unit tests for the proposers, provenance primitives, scoring and YAML
// helpers moved to the library with them (library-lift-plan.md L4); these are
// the CLI-level tests that remain.

// runGitProbe runs git with args in dir, failing the test on error -- for
// test setup only, not the primitives under test.
func runGitProbe(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGitProbe(t, dir, "init", "-q", "-b", "main", ".")
}

func gitCommitFile(t *testing.T, dir, relPath, content, authorName, authorEmail, date string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runGitProbe(t, dir, "add", relPath)
	cmd := exec.Command("git",
		"-c", "user.name="+authorName, "-c", "user.email="+authorEmail,
		"commit", "-q", "-m", "commit "+relPath)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

// ─── FM5: cmdDocumentFrontmatter wiring ─────────────────────────────────────

func TestCmdDocumentFrontmatter_BareInvocationIsReadOnly(t *testing.T) {
	kb := openTestKB(t)
	dir := t.TempDir()
	content := "# A Real Title\n\nSome body text.\n"
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stdout, err := runDocument(t, kb, false, "frontmatter", path)
	if err != nil {
		t.Fatalf("document frontmatter: %v", err)
	}
	if !strings.Contains(stdout, "A Real Title") {
		t.Errorf("stdout = %q, want it to mention the proposed title", stdout)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != content {
		t.Errorf("file content = %q, want unchanged by a bare invocation", string(raw))
	}
}

func TestCmdDocumentFrontmatter_AcceptsTitleField(t *testing.T) {
	kb := openTestKB(t)
	dir := t.TempDir()
	content := "# A Real Title\n\nSome body text.\n"
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := runDocument(t, kb, false, "frontmatter", path, "--accept", "title"); err != nil {
		t.Fatalf("document frontmatter --accept title: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "title: A Real Title") {
		t.Errorf("file content = %q, want title: A Real Title written", string(raw))
	}
}

func TestCmdDocumentFrontmatter_NeverOverwritesExistingTitleAuthorOrDateCreated(t *testing.T) {
	kb := openTestKB(t)
	dir := t.TempDir()
	content := "---\ntitle: Existing Title\nauthor: Existing Author\ndateCreated: 2020-01-01T00:00:00Z\n---\n\n# A Different Title\n\nBody.\n"
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := runDocument(t, kb, false, "frontmatter", path, "--accept", "title,author,dateCreated"); err != nil {
		t.Fatalf("document frontmatter: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != content {
		t.Errorf("file content = %q, want unchanged -- all three fields already had values", string(raw))
	}
}

func TestCmdDocumentFrontmatter_DateModifiedAlwaysRefreshedOnAcceptedRun(t *testing.T) {
	kb := openTestKB(t)
	dir := t.TempDir()
	initGitRepo(t, dir)
	content := "---\ndateModified: 2020-01-01T00:00:00Z\n---\n\nBody.\n"
	gitCommitFile(t, dir, "a.md", content, "Author", "a@example.com", "2026-03-15T10:00:00-07:00")

	path := filepath.Join(dir, "a.md")
	if _, err := runDocument(t, kb, false, "frontmatter", path, "--accept", "dateModified"); err != nil {
		t.Fatalf("document frontmatter: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "2020-01-01") {
		t.Errorf("file content = %q, want the stale dateModified refreshed", string(raw))
	}
	if !strings.Contains(string(raw), "2026-03-15") {
		t.Errorf("file content = %q, want dateModified refreshed to the git commit date", string(raw))
	}
}

// dateModified is the one field that's always recomputed regardless of an
// existing value (design decision 4) -- found live, smoke-testing a real
// document: the plain-text report's switch took the "already set" branch
// first whenever Current was non-empty, silently never showing the freshly
// computed Proposed value at all once a dateModified already existed.
func TestCmdDocumentFrontmatter_ReportsProposedDateModifiedEvenWhenCurrentExists(t *testing.T) {
	kb := openTestKB(t)
	dir := t.TempDir()
	initGitRepo(t, dir)
	content := "---\ndateModified: 2020-01-01T00:00:00Z\n---\n\nBody.\n"
	gitCommitFile(t, dir, "a.md", content, "Author", "a@example.com", "2026-03-15T10:00:00-07:00")

	stdout, err := runDocument(t, kb, false, "frontmatter", filepath.Join(dir, "a.md"))
	if err != nil {
		t.Fatalf("document frontmatter: %v", err)
	}
	var dateModifiedLine string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "dateModified:") {
			dateModifiedLine = line
		}
	}
	if !strings.Contains(dateModifiedLine, "2020-01-01") {
		t.Errorf("dateModified line = %q, want it to show the current (stale) value", dateModifiedLine)
	}
	if !strings.Contains(dateModifiedLine, "2026-03-15") {
		t.Errorf("dateModified line = %q, want it to also show the freshly proposed value", dateModifiedLine)
	}
}

// Found reviewing this code before pre-release prep: a frontmatter key
// present with an explicit empty value (e.g. a template's "title: ""')
// made mappingStringValue report ok=true, so the absent-only proposal
// branch never ran -- the field would never get filled in, with no
// diagnostic explaining why.
func TestCmdDocumentFrontmatter_TreatsExplicitEmptyValueAsAbsent(t *testing.T) {
	kb := openTestKB(t)
	dir := t.TempDir()
	content := "---\ntitle: \"\"\n---\n\n# A Real Title\n\nBody.\n"
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := runDocument(t, kb, false, "frontmatter", path, "--accept", "title"); err != nil {
		t.Fatalf("document frontmatter --accept title: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "title: A Real Title") {
		t.Errorf("file content = %q, want the explicitly-empty title filled in from the H1", string(raw))
	}
}

// Found in the same review: --set FIELD= (an explicit empty value) is
// documented as bypassing signal detection -- "a human's explicit
// assertion, not a proposal" -- but silently did nothing at all, with no
// write and no error, because the write-time guard treated an empty value
// as "nothing to write" regardless of whether it came from a genuine
// absent proposal or a deliberate --set.
func TestCmdDocumentFrontmatter_SetEmptyValueWritesRatherThanNoOps(t *testing.T) {
	kb := openTestKB(t)
	dir := t.TempDir()
	content := "---\ntitle: Existing Title\n---\n\nBody.\n"
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := runDocument(t, kb, false, "frontmatter", path, "--set", "title="); err != nil {
		t.Fatalf("document frontmatter --set title=: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), `title: ""`) {
		t.Errorf("file content = %q, want title cleared to an explicit empty string", string(raw))
	}
}

func TestCmdDocumentFrontmatter_SetOverridesBypassesSignalDetection(t *testing.T) {
	kb := openTestKB(t)
	dir := t.TempDir()
	content := "Just a plain document with no byline, no git history.\n"
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := runDocument(t, kb, false, "frontmatter", path, "--set", "author=Explicit Author"); err != nil {
		t.Fatalf("document frontmatter --set: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "author: Explicit Author") {
		t.Errorf("file content = %q, want author: Explicit Author written despite no detected signal", string(raw))
	}
}

func TestCmdDocumentFrontmatter_AcceptKeywordsPlainWriteForKnownConceptMatch(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	before, _ := kb.Concepts()
	dir := t.TempDir()
	content := "# Title\n\nchunking is mentioned twice: chunking again.\n"
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := runDocument(t, kb, false, "frontmatter", path, "--accept-keywords", "chunking"); err != nil {
		t.Fatalf("document frontmatter --accept-keywords: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "chunking") || !strings.Contains(string(raw), "keywords:") {
		t.Errorf("file content = %q, want keywords: [chunking] written", string(raw))
	}
	after, _ := kb.Concepts()
	if len(after) != len(before) {
		t.Errorf("concepts changed from %d to %d, want unchanged -- chunking already existed", len(before), len(after))
	}
}

func TestCmdDocumentFrontmatter_AcceptKeywordsCreatesConceptForNewCandidateBeforeWriting(t *testing.T) {
	kb := openTestKB(t)
	// A term is only *proposed* when it occurs at least twice and is
	// distinctive against the rest of the corpus, so the corpus needs one
	// unrelated document (DR-0036: this test used to pass an unproposed term).
	kb.AddProject("alpha", "")
	other := writeDocFixture(t, t.TempDir(), "other.md", "## S\n\nunrelated words about the weather.\n")
	if _, err := runDocument(t, kb, false, "ingest", other, "--project", "alpha"); err != nil {
		t.Fatalf("seeding comparison document: %v", err)
	}
	before, _ := kb.Concepts()
	dir := t.TempDir()
	content := "# Title\n\ngronkulator appears here, and gronkulator appears again.\n"
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := runDocument(t, kb, false, "frontmatter", path, "--accept-keywords", "gronkulator"); err != nil {
		t.Fatalf("document frontmatter --accept-keywords: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "gronkulator") {
		t.Errorf("file content = %q, want keywords: [gronkulator] written", string(raw))
	}
	after, err := kb.Concepts()
	if err != nil {
		t.Fatalf("Concepts: %v", err)
	}
	if len(after) != len(before)+1 {
		t.Errorf("concepts count = %d, want %d (gronkulator created)", len(after), len(before)+1)
	}
}

func TestCmdDocumentFrontmatter_DryRunWithAcceptPreviewsWithoutWriting(t *testing.T) {
	kb := openTestKB(t)
	dir := t.TempDir()
	content := "# A Real Title\n\nSome body text.\n"
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stdout, err := runDocument(t, kb, false, "frontmatter", path, "--accept", "title", "--dry-run")
	if err != nil {
		t.Fatalf("document frontmatter --dry-run: %v", err)
	}
	if !strings.Contains(stdout, "A Real Title") {
		t.Errorf("stdout = %q, want it to mention the proposed title", stdout)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != content {
		t.Errorf("file content = %q, want unchanged by --dry-run", string(raw))
	}
}

func TestCmdDocumentFrontmatter_BodyNeverModified(t *testing.T) {
	kb := openTestKB(t)
	dir := t.TempDir()
	body := "\n# A Real Title\n\nBody with `code` and [[Wikilink]] untouched.\n"
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := runDocument(t, kb, false, "frontmatter", path, "--accept", "title"); err != nil {
		t.Fatalf("document frontmatter: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.HasSuffix(string(raw), body) {
		t.Errorf("file content = %q, want it to end with the original body bytes unchanged", string(raw))
	}
}

func TestCmdDocumentFrontmatter_JSONOutputShape(t *testing.T) {
	kb := openTestKB(t)
	dir := t.TempDir()
	content := "# A Real Title\n\nSome body text.\n"
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stdout, err := runDocument(t, kb, true, "frontmatter", path, "--accept", "title")
	if err != nil {
		t.Fatalf("document frontmatter --json: %v", err)
	}
	assertValidJSON(t, []byte(stdout))
	var got struct {
		Path   string `json:"path"`
		Fields []struct {
			Field    string `json:"field"`
			Proposed string `json:"proposed"`
			Accepted bool   `json:"accepted"`
		} `json:"fields"`
		Written bool `json:"written"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decoding: %v\n%s", err, stdout)
	}
	if got.Path != path || !got.Written {
		t.Fatalf("got = %+v, want path=%q written=true", got, path)
	}
	found := false
	for _, f := range got.Fields {
		if f.Field == "title" && f.Accepted && f.Proposed == "A Real Title" {
			found = true
		}
	}
	if !found {
		t.Errorf("got.Fields = %+v, want title accepted with proposed=\"A Real Title\"", got.Fields)
	}
}

func TestWriteDocumentFile_AtomicReplaceViaTempFileAndRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	raw := []byte("---\ntitle: Old\n---\n\nBody.\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := writeFileAtomic(path, []byte("---\ntitle: New\n---\n\nBody.\n")); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "a.md" {
		t.Errorf("dir entries = %+v, want only a.md left behind, no leftover temp file", entries)
	}
}

// ─── --accept-keywords only accepts what was proposed (found live 2026-09-23) ─
//
// DR-0033: a keyword is a known concept (plain write) or a *proposed* new
// candidate (concept created first). It never mints a concept for an
// arbitrary string: a probe with a nonsense term wrote it into the file and
// created a real concept in the database.

func TestCmdDocumentFrontmatter_AcceptKeywordsRejectsATermThatWasNeverProposed(t *testing.T) {
	kb := openTestKB(t)
	before, _ := kb.Concepts()
	path := filepath.Join(t.TempDir(), "a.md")
	content := "# Title\n\nSome ordinary body text.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := runDocument(t, kb, false, "frontmatter", path, "--accept-keywords", "zzznotaterm")
	if err == nil {
		t.Fatal("document frontmatter --accept-keywords zzznotaterm = nil error, want a rejection")
	}
	if !strings.Contains(err.Error(), "zzznotaterm") {
		t.Errorf("error = %q, want it to name the rejected term", err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != content {
		t.Errorf("file changed to %q, want it untouched", raw)
	}
	after, _ := kb.Concepts()
	if len(after) != len(before) {
		t.Errorf("concepts changed from %d to %d, want no concept minted", len(before), len(after))
	}
}

func TestCmdDocumentFrontmatter_RejectedKeywordAbortsEveryOtherAcceptedField(t *testing.T) {
	kb := openTestKB(t)
	path := filepath.Join(t.TempDir(), "a.md")
	content := "# A Real Title\n\nSome body text.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := runDocument(t, kb, false, "frontmatter", path, "--accept", "title", "--accept-keywords", "zzznotaterm")
	if err == nil {
		t.Fatal("want a rejection")
	}
	if raw, _ := os.ReadFile(path); string(raw) != content {
		t.Errorf("file changed to %q: a rejected keyword must abort the whole write, as tag --concept does", raw)
	}
}

func TestCmdDocumentFrontmatter_DryRunWithNewCandidateKeywordCreatesNoConcept(t *testing.T) {
	kb := openTestKB(t)
	// A term is only *proposed* when it occurs at least twice and is
	// distinctive against the rest of the corpus, so the corpus needs one
	// unrelated document (DR-0036: this test used to pass an unproposed term).
	kb.AddProject("alpha", "")
	other := writeDocFixture(t, t.TempDir(), "other.md", "## S\n\nunrelated words about the weather.\n")
	if _, err := runDocument(t, kb, false, "ingest", other, "--project", "alpha"); err != nil {
		t.Fatalf("seeding comparison document: %v", err)
	}
	before, _ := kb.Concepts()
	path := filepath.Join(t.TempDir(), "a.md")
	content := "# Title\n\ngronkulator appears here, and gronkulator appears again.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := runDocument(t, kb, false, "frontmatter", path, "--accept-keywords", "gronkulator", "--dry-run"); err != nil {
		t.Fatalf("document frontmatter --dry-run: %v", err)
	}
	if raw, _ := os.ReadFile(path); string(raw) != content {
		t.Errorf("file changed to %q on a dry run", raw)
	}
	after, _ := kb.Concepts()
	if len(after) != len(before) {
		t.Errorf("concepts changed from %d to %d on a dry run, want the database untouched", len(before), len(after))
	}
}
