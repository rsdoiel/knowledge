package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	knowledge "github.com/rsdoiel/knowledge"
)

// ─── kb document frontmatter (v0.0.11 item 2), FM1: git/filesystem
// provenance primitives ─────────────────────────────────────────────────────

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

func TestGitFirstCommit_ReturnsOldestAuthorAndDate(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	gitCommitFile(t, dir, "a.md", "v1\n", "Original Author", "orig@example.com", "2026-01-01T10:00:00-07:00")
	gitCommitFile(t, dir, "a.md", "v2\n", "Later Editor", "later@example.com", "2026-02-01T10:00:00-07:00")

	author, date, ok := gitFirstCommit(filepath.Join(dir, "a.md"))
	if !ok {
		t.Fatal("expected ok=true")
	}
	if author != "Original Author" {
		t.Errorf("author = %q, want %q (the oldest commit's author)", author, "Original Author")
	}
	if !strings.HasPrefix(date, "2026-01-01") {
		t.Errorf("date = %q, want it to start with 2026-01-01", date)
	}
}

func TestGitFirstCommit_FalseWhenNotARepo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, _, ok := gitFirstCommit(path); ok {
		t.Error("expected ok=false outside any git repository")
	}
}

func TestGitLastCommit_ReturnsMostRecentCommitDate(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	gitCommitFile(t, dir, "a.md", "v1\n", "Original Author", "orig@example.com", "2026-01-01T10:00:00-07:00")
	gitCommitFile(t, dir, "a.md", "v2\n", "Later Editor", "later@example.com", "2026-03-15T10:00:00-07:00")

	date, ok := gitLastCommit(filepath.Join(dir, "a.md"))
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !strings.HasPrefix(date, "2026-03-15") {
		t.Errorf("date = %q, want it to start with 2026-03-15 (the most recent commit)", date)
	}
}

func TestGitConfigUserName_ReturnsConfiguredName(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	runGitProbe(t, dir, "config", "user.name", "Configured Name")

	name, ok := gitConfigUserName(dir)
	if !ok || name != "Configured Name" {
		t.Errorf("gitConfigUserName = %q, %v, want \"Configured Name\", true", name, ok)
	}
}

func TestFsBirthOrModTime_ReturnsATimeForAnyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := fsBirthOrModTime(path)
	if err != nil {
		t.Fatalf("fsBirthOrModTime: %v", err)
	}
	if got.IsZero() || got.After(time.Now().Add(time.Minute)) {
		t.Errorf("fsBirthOrModTime = %v, want a sane recent time", got)
	}
}

func TestDetectByline_FindsEachLeadInForm(t *testing.T) {
	cases := []struct {
		name, body, wantAuthor string
	}{
		{"By", "# Title\n\nBy Jane Doe\n\nBody text follows.\n", "Jane Doe"},
		{"Author colon", "# Title\n\nAuthor: Jane Doe\n\nBody text follows.\n", "Jane Doe"},
		{"Written by", "# Title\n\nWritten by Jane Doe\n\nBody text follows.\n", "Jane Doe"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			author, ok := detectByline(c.body)
			if !ok || author != c.wantAuthor {
				t.Errorf("detectByline = %q, %v, want %q, true", author, ok, c.wantAuthor)
			}
		})
	}
}

func TestDetectByline_FalseWhenNoLeadInPresent(t *testing.T) {
	body := "# Title\n\nThis document has no byline at all, just prose.\n"
	if _, ok := detectByline(body); ok {
		t.Error("expected ok=false when no byline lead-in is present")
	}
}

// ─── FM2: field proposals (title / author / dateCreated / dateModified) ───

func TestProposeAuthor_PrefersBylineOverGit(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	body := "# Title\n\nBy Byline Author\n\nBody text.\n"
	gitCommitFile(t, dir, "a.md", body, "Git Author", "git@example.com", "2026-01-01T10:00:00-07:00")

	winner, signals := proposeAuthor(filepath.Join(dir, "a.md"), body)
	if winner != "Byline Author" {
		t.Errorf("winner = %q, want %q", winner, "Byline Author")
	}
	if len(signals) < 2 {
		t.Fatalf("signals = %+v, want at least byline and git", signals)
	}
}

func TestProposeAuthor_FallsBackToGitFirstCommitAuthor(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	body := "# Title\n\nNo byline here, just prose.\n"
	gitCommitFile(t, dir, "a.md", body, "Git Author", "git@example.com", "2026-01-01T10:00:00-07:00")

	winner, _ := proposeAuthor(filepath.Join(dir, "a.md"), body)
	if winner != "Git Author" {
		t.Errorf("winner = %q, want %q", winner, "Git Author")
	}
}

func TestProposeAuthor_FallsBackToGitConfigWhenUntracked(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	runGitProbe(t, dir, "config", "user.name", "Configured Name")
	body := "# Title\n\nNo byline here, just prose.\n"
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	winner, _ := proposeAuthor(path, body)
	if winner != "Configured Name" {
		t.Errorf("winner = %q, want %q", winner, "Configured Name")
	}
}

func TestProposeAuthor_SignalsListsEveryFiredSignal(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	body := "# Title\n\nBy Byline Author\n\nBody text.\n"
	gitCommitFile(t, dir, "a.md", body, "Git Author", "git@example.com", "2026-01-01T10:00:00-07:00")

	_, signals := proposeAuthor(filepath.Join(dir, "a.md"), body)
	var haveByline, haveGit bool
	for _, s := range signals {
		if s.Value == "Byline Author" {
			haveByline = true
		}
		if s.Value == "Git Author" {
			haveGit = true
		}
	}
	if !haveByline || !haveGit {
		t.Errorf("signals = %+v, want both the byline and git signals listed", signals)
	}
}

func TestProposeDateCreated_PrefersGitOverFilesystem(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	gitCommitFile(t, dir, "a.md", "content\n", "Author", "a@example.com", "2026-01-01T10:00:00-07:00")

	value, source := proposeDateCreated(filepath.Join(dir, "a.md"))
	if !strings.HasPrefix(value, "2026-01-01") {
		t.Errorf("value = %q, want it to start with 2026-01-01", value)
	}
	if source != "git" {
		t.Errorf("source = %q, want %q", source, "git")
	}
}

func TestProposeDateModified_AlwaysComputesRegardlessOfExistingValue(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	gitCommitFile(t, dir, "a.md", "content\n", "Author", "a@example.com", "2026-03-15T10:00:00-07:00")

	value, source := proposeDateModified(filepath.Join(dir, "a.md"))
	if !strings.HasPrefix(value, "2026-03-15") {
		t.Errorf("value = %q, want it to start with 2026-03-15", value)
	}
	if source != "git" {
		t.Errorf("source = %q, want %q", source, "git")
	}
}

// ─── FM3: keyword proposals (DB read-only) ─────────────────────────────────

func TestKnownKeywordProposals_ExcludesAlreadyListedKeywords(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	if _, err := kb.AddConcept("workspace", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	text := "chunking is mentioned twice: chunking again. workspace is mentioned twice: workspace again."

	proposals, err := knownKeywordProposals(kb, text, []string{"chunking"})
	if err != nil {
		t.Fatalf("knownKeywordProposals: %v", err)
	}
	if len(proposals) != 1 || !strings.EqualFold(proposals[0], "workspace") {
		t.Errorf("proposals = %+v, want just [workspace] (chunking already listed)", proposals)
	}
}

func TestScoreDocumentCandidateTerms_ExcludingTargetFixesIdfDegeneracy(t *testing.T) {
	target := "gronkulator gronkulator gronkulator appears many times in this document"
	comparison := []string{
		"unrelated text about something else entirely",
		"another unrelated document with different words",
	}
	got := scoreDocumentCandidateTerms(target, comparison, map[string]bool{})
	found := false
	for _, c := range got {
		if c.Term == "gronkulator" {
			found = true
			if c.Score <= 0 {
				t.Errorf("gronkulator score = %v, want positive (idf must not collapse to 0)", c.Score)
			}
		}
	}
	if !found {
		t.Errorf("got = %+v, want gronkulator scored as a candidate", got)
	}
}

func TestScoreDocumentCandidateTerms_TermAbsentFromComparisonScopeStillScores(t *testing.T) {
	target := "distinctiveword distinctiveword shows up twice here"
	comparison := []string{"nothing in common with the target at all"}
	got := scoreDocumentCandidateTerms(target, comparison, map[string]bool{})
	if len(got) == 0 {
		t.Fatal("got no candidates, want distinctiveword scored even though absent from comparison scope")
	}
}

func TestComparisonScope_ScopesToProjectWhenDocumentBelongsToOne(t *testing.T) {
	kb := openTestKB(t)
	pidA, _ := kb.AddProject("alpha", "")
	pidB, _ := kb.AddProject("beta", "")
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0001", ProjectID: pidA, Scope: "project", Path: "alpha/decisions/0001-x.md",
		Title: "x", Date: "2026-01-01", Status: "accepted", Kind: "decision", Body: "alpha record body",
	}); err != nil {
		t.Fatalf("AddRecord alpha: %v", err)
	}
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0001", ProjectID: pidB, Scope: "project", Path: "beta/decisions/0001-x.md",
		Title: "x", Date: "2026-01-01", Status: "accepted", Kind: "decision", Body: "beta record body",
	}); err != nil {
		t.Fatalf("AddRecord beta: %v", err)
	}
	target, err := kb.AddDocument(knowledge.Document{ProjectID: pidA, Title: "t", Format: "text", Path: "a.md"})
	if err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	doc, err := kb.DocumentByPath("a.md")
	if err != nil {
		t.Fatalf("DocumentByPath: %v", err)
	}
	_ = target

	items, err := comparisonScope(kb, doc)
	if err != nil {
		t.Fatalf("comparisonScope: %v", err)
	}
	for _, item := range items {
		if strings.Contains(item, "beta record body") {
			t.Errorf("items = %+v, want beta's record excluded (different project)", items)
		}
	}
}

func TestComparisonScope_FallsBackToWholeCorpusWhenNoProject(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddRecord(knowledge.Record{
		RecordID: "0001", ProjectID: 0, Scope: "workspace", Path: "decisions/0001-x.md",
		Title: "x", Date: "2026-01-01", Status: "accepted", Kind: "decision", Body: "workspace record body",
	}); err != nil {
		t.Fatalf("AddRecord: %v", err)
	}

	items, err := comparisonScope(kb, nil)
	if err != nil {
		t.Fatalf("comparisonScope: %v", err)
	}
	found := false
	for _, item := range items {
		if strings.Contains(item, "workspace record body") {
			found = true
		}
	}
	if !found {
		t.Errorf("items = %+v, want the workspace record included when there's no target project", items)
	}
}

// ─── FM4: yaml.Node surgical frontmatter writer ────────────────────────────

func TestFrontmatterNode_ParsesExistingBlock(t *testing.T) {
	raw := []byte("---\ntitle: Foo\nauthor: Bar\n---\n\nBody text.\n")
	node, bodyOffset, hadBlock, err := frontmatterNode(raw)
	if err != nil {
		t.Fatalf("frontmatterNode: %v", err)
	}
	if !hadBlock {
		t.Fatal("hadBlock = false, want true")
	}
	if string(raw[bodyOffset:]) != "\n\nBody text.\n" {
		t.Errorf("raw[bodyOffset:] = %q, want %q", string(raw[bodyOffset:]), "\n\nBody text.\n")
	}
	found := false
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "title" && node.Content[i+1].Value == "Foo" {
			found = true
		}
	}
	if !found {
		t.Errorf("node.Content = %+v, want title: Foo present", node.Content)
	}
}

func TestFrontmatterNode_NoBlockReturnsEmptyMappingHadBlockFalse(t *testing.T) {
	raw := []byte("Just a plain document, no frontmatter.\n")
	node, _, hadBlock, err := frontmatterNode(raw)
	if err != nil {
		t.Fatalf("frontmatterNode: %v", err)
	}
	if hadBlock {
		t.Error("hadBlock = true, want false")
	}
	if len(node.Content) != 0 {
		t.Errorf("node.Content = %+v, want empty", node.Content)
	}
}

func TestSetMappingField_AppendsNewKey(t *testing.T) {
	node, _, _, _ := frontmatterNode([]byte("---\ntitle: Foo\n---\n"))
	if err := setMappingField(node, "author", "Jane Doe"); err != nil {
		t.Fatalf("setMappingField: %v", err)
	}
	rendered, err := renderFrontmatter(node)
	if err != nil {
		t.Fatalf("renderFrontmatter: %v", err)
	}
	if !strings.Contains(rendered, "author: Jane Doe") {
		t.Errorf("rendered = %q, want it to contain \"author: Jane Doe\"", rendered)
	}
	if !strings.Contains(rendered, "title: Foo") {
		t.Errorf("rendered = %q, want the existing title untouched", rendered)
	}
}

func TestSetMappingField_OverwritesExistingKeyInPlace(t *testing.T) {
	node, _, _, _ := frontmatterNode([]byte("---\ntitle: Old Title\nauthor: Jane\n---\n"))
	if err := setMappingField(node, "title", "New Title"); err != nil {
		t.Fatalf("setMappingField: %v", err)
	}
	rendered, err := renderFrontmatter(node)
	if err != nil {
		t.Fatalf("renderFrontmatter: %v", err)
	}
	if strings.Contains(rendered, "Old Title") {
		t.Errorf("rendered = %q, want the old title gone", rendered)
	}
	if !strings.Contains(rendered, "title: New Title") {
		t.Errorf("rendered = %q, want the new title", rendered)
	}
	if !strings.Contains(rendered, "author: Jane") {
		t.Errorf("rendered = %q, want author untouched", rendered)
	}
}

func TestSetMappingField_LeavesUnknownKeysAndCommentsUntouched(t *testing.T) {
	raw := []byte("---\ntitle: Foo\ncustomField: keep-me\n---\n")
	node, _, _, err := frontmatterNode(raw)
	if err != nil {
		t.Fatalf("frontmatterNode: %v", err)
	}
	if err := setMappingField(node, "author", "Jane Doe"); err != nil {
		t.Fatalf("setMappingField: %v", err)
	}
	rendered, err := renderFrontmatter(node)
	if err != nil {
		t.Fatalf("renderFrontmatter: %v", err)
	}
	if !strings.Contains(rendered, "customField: keep-me") {
		t.Errorf("rendered = %q, want the unknown field preserved -- this is the data-loss regression FM4 exists to prevent", rendered)
	}
}

func TestWriteDocumentFile_PrependsBlockWhenNoneExisted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	raw := []byte("# Title\n\nBody text.\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := writeDocumentFile(path, raw, false, 0, "title: Foo"); err != nil {
		t.Fatalf("writeDocumentFile: %v", err)
	}
	got, _ := os.ReadFile(path)
	want := "---\ntitle: Foo\n---\n\n# Title\n\nBody text.\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", string(got), want)
	}
}

func TestWriteDocumentFile_BodyBytesUnchangedOutsideFrontmatterBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	raw := []byte("---\ntitle: Old\n---\n\nBody with `code` and [[Wikilink]] untouched.\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, bodyOffset, hadBlock, err := frontmatterNode(raw)
	if err != nil || !hadBlock {
		t.Fatalf("frontmatterNode: hadBlock=%v err=%v", hadBlock, err)
	}
	if err := writeDocumentFile(path, raw, true, bodyOffset, "title: New"); err != nil {
		t.Fatalf("writeDocumentFile: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "Body with `code` and [[Wikilink]] untouched.\n") {
		t.Errorf("got %q, want the body bytes unchanged", string(got))
	}
	if strings.Contains(string(got), "Old") {
		t.Errorf("got %q, want the old title gone", string(got))
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
	before, _ := kb.Concepts()
	dir := t.TempDir()
	content := "# Title\n\ngronkulator appears here as a brand new term.\n"
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
	if err := writeDocumentFile(path, raw, true, len("---\ntitle: Old\n---"), "title: New"); err != nil {
		t.Fatalf("writeDocumentFile: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "a.md" {
		t.Errorf("dir entries = %+v, want only a.md left behind, no leftover temp file", entries)
	}
}
