package knowledge

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Moved from cmd/kb/documentfrontmatter_test.go (library-lift-plan.md L4,
// DR-0035): unit tests for the frontmatter logic, which now lives in this
// package. The git-backed ones exercise GitProvenance against a real
// repository; the rest use a fake Provenance.

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

	author, date, ok := (GitProvenance{}).FirstCommit(filepath.Join(dir, "a.md"))
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
	if _, _, ok := (GitProvenance{}).FirstCommit(path); ok {
		t.Error("expected ok=false outside any git repository")
	}
}

func TestGitLastCommit_ReturnsMostRecentCommitDate(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	gitCommitFile(t, dir, "a.md", "v1\n", "Original Author", "orig@example.com", "2026-01-01T10:00:00-07:00")
	gitCommitFile(t, dir, "a.md", "v2\n", "Later Editor", "later@example.com", "2026-03-15T10:00:00-07:00")

	date, ok := (GitProvenance{}).LastCommit(filepath.Join(dir, "a.md"))
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

	name, ok := (GitProvenance{}).ConfigUserName(dir)
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
	got, err := (GitProvenance{}).FileTime(path)
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

	winner, signals := proposeAuthor(GitProvenance{}, filepath.Join(dir, "a.md"), body)
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

	winner, _ := proposeAuthor(GitProvenance{}, filepath.Join(dir, "a.md"), body)
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

	winner, _ := proposeAuthor(GitProvenance{}, path, body)
	if winner != "Configured Name" {
		t.Errorf("winner = %q, want %q", winner, "Configured Name")
	}
}

func TestProposeAuthor_SignalsListsEveryFiredSignal(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	body := "# Title\n\nBy Byline Author\n\nBody text.\n"
	gitCommitFile(t, dir, "a.md", body, "Git Author", "git@example.com", "2026-01-01T10:00:00-07:00")

	_, signals := proposeAuthor(GitProvenance{}, filepath.Join(dir, "a.md"), body)
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

	value, source := proposeDateCreated(GitProvenance{}, filepath.Join(dir, "a.md"))
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

	value, source := proposeDateModified(GitProvenance{}, filepath.Join(dir, "a.md"))
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
	if _, err := kb.AddRecord(Record{
		RecordID: "0001", ProjectID: pidA, Scope: "project", Path: "alpha/decisions/0001-x.md",
		Title: "x", Date: "2026-01-01", Status: "accepted", Kind: "decision", Body: "alpha record body",
	}); err != nil {
		t.Fatalf("AddRecord alpha: %v", err)
	}
	if _, err := kb.AddRecord(Record{
		RecordID: "0001", ProjectID: pidB, Scope: "project", Path: "beta/decisions/0001-x.md",
		Title: "x", Date: "2026-01-01", Status: "accepted", Kind: "decision", Body: "beta record body",
	}); err != nil {
		t.Fatalf("AddRecord beta: %v", err)
	}
	target, err := kb.AddDocument(Document{ProjectID: pidA, Title: "t", Format: "text", Path: "a.md"})
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
	if _, err := kb.AddRecord(Record{
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

// ─── L4: the exported surface, with a fake Provenance ───────────────────────

type fakeProvenance struct {
	firstAuthor, firstDate string
	firstOK                bool
	lastDate               string
	lastOK                 bool
	config                 string
	configOK               bool
	fileTime               time.Time
	fileErr                error
}

func (f fakeProvenance) FirstCommit(string) (string, string, bool) {
	return f.firstAuthor, f.firstDate, f.firstOK
}
func (f fakeProvenance) LastCommit(string) (string, bool)     { return f.lastDate, f.lastOK }
func (f fakeProvenance) ConfigUserName(string) (string, bool) { return f.config, f.configOK }
func (f fakeProvenance) FileTime(string) (time.Time, error)   { return f.fileTime, f.fileErr }

var gitLikeProvenance = fakeProvenance{
	firstAuthor: "Git Author", firstDate: "2026-01-01T10:00:00-07:00", firstOK: true,
	lastDate: "2026-03-15T10:00:00-07:00", lastOK: true,
	config: "Configured Name", configOK: true,
}

func fieldReport(t *testing.T, r FrontmatterResult, name string) FrontmatterFieldReport {
	t.Helper()
	for _, f := range r.Fields {
		if f.Field == name {
			return f
		}
	}
	t.Fatalf("no %q field in %+v", name, r.Fields)
	return FrontmatterFieldReport{}
}

func TestProposeFrontmatter_ReportsEveryFieldAndWritesNothing(t *testing.T) {
	kb := openTestKB(t)
	raw := []byte("# A Real Title\n\nBody text.\n")
	res, err := kb.ProposeFrontmatter("a.md", raw, gitLikeProvenance)
	if err != nil {
		t.Fatalf("ProposeFrontmatter: %v", err)
	}
	if res.Written || res.DryRun {
		t.Errorf("result = %+v, want a read-only report", res)
	}
	if got := fieldReport(t, res, "title").Proposed; got != "A Real Title" {
		t.Errorf("title proposal = %q, want %q", got, "A Real Title")
	}
	if got := fieldReport(t, res, "author").Proposed; got != "Git Author" {
		t.Errorf("author proposal = %q, want the git first-commit author", got)
	}
	if got := fieldReport(t, res, "dateCreated").Proposed; !strings.HasPrefix(got, "2026-01-01") {
		t.Errorf("dateCreated proposal = %q, want the first commit's date", got)
	}
	if got := fieldReport(t, res, "dateModified").Proposed; !strings.HasPrefix(got, "2026-03-15") {
		t.Errorf("dateModified proposal = %q, want the last commit's date", got)
	}
}

func TestProposeFrontmatter_AuthorSignalsAreLayeredBylineGitConfig(t *testing.T) {
	kb := openTestKB(t)
	raw := []byte("# Title\n\nBy Byline Author\n\nBody.\n")
	res, _ := kb.ProposeFrontmatter("a.md", raw, gitLikeProvenance)
	a := fieldReport(t, res, "author")
	if a.Proposed != "Byline Author" {
		t.Errorf("author = %q, want the byline to win", a.Proposed)
	}
	var sources []string
	for _, s := range a.Signals {
		sources = append(sources, s.Source)
	}
	if strings.Join(sources, ",") != "byline,git,git config" {
		t.Errorf("signal sources = %v, want byline, git, git config in priority order", sources)
	}
}

func TestProposeFrontmatter_FallsBackToFileTimeWhenNoGitHistory(t *testing.T) {
	kb := openTestKB(t)
	ft := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	res, _ := kb.ProposeFrontmatter("a.md", []byte("# T\n\nx\n"), fakeProvenance{fileTime: ft})
	if got := fieldReport(t, res, "dateCreated").Proposed; got != ft.Format(time.RFC3339) {
		t.Errorf("dateCreated = %q, want the file time %q", got, ft.Format(time.RFC3339))
	}
	if got := fieldReport(t, res, "author").Proposed; got != "" {
		t.Errorf("author = %q, want no proposal when every signal is missing", got)
	}
}

func TestProposeFrontmatter_NilProvenanceDefaultsToGit(t *testing.T) {
	kb := openTestKB(t)
	path := filepath.Join(t.TempDir(), "a.md")
	raw := []byte("# T\n\nx\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	res, err := kb.ProposeFrontmatter(path, raw, nil)
	if err != nil {
		t.Fatalf("ProposeFrontmatter(nil provenance): %v", err)
	}
	if got := fieldReport(t, res, "dateCreated").Proposed; got == "" {
		t.Error("dateCreated = \"\", want the filesystem fallback from the default provenance")
	}
}

func TestApplyFrontmatter_AcceptTitlePrependsBlockAndNeverMutatesInput(t *testing.T) {
	kb := openTestKB(t)
	raw := []byte("# A Real Title\n\nBody text.\n")
	orig := string(raw)
	next, res, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance, FrontmatterAccept{Fields: []string{"title"}}, false)
	if err != nil {
		t.Fatalf("ApplyFrontmatter: %v", err)
	}
	want := "---\ntitle: A Real Title\n---\n\n# A Real Title\n\nBody text.\n"
	if string(next) != want {
		t.Errorf("next = %q, want %q", next, want)
	}
	if string(raw) != orig {
		t.Errorf("input bytes changed to %q, want the caller's slice untouched", raw)
	}
	if !res.Written || !fieldReport(t, res, "title").Accepted {
		t.Errorf("result = %+v, want Written and title accepted", res)
	}
}

func TestApplyFrontmatter_BareAcceptIsReadOnlyAndReturnsInputBytes(t *testing.T) {
	kb := openTestKB(t)
	raw := []byte("# T\n\nx\n")
	next, res, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance, FrontmatterAccept{}, false)
	if err != nil || res.Written || string(next) != string(raw) {
		t.Errorf("next=%q res=%+v err=%v, want unchanged bytes and Written=false", next, res, err)
	}
}

func TestApplyFrontmatter_TitleAuthorDateCreatedAreAbsentOnly(t *testing.T) {
	kb := openTestKB(t)
	raw := []byte("---\ntitle: Existing\nauthor: Someone\ndateCreated: \"2020-01-01\"\n---\n\n# Other Title\n")
	next, res, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance,
		FrontmatterAccept{Fields: []string{"title", "author", "dateCreated"}}, false)
	if err != nil {
		t.Fatalf("ApplyFrontmatter: %v", err)
	}
	if res.Written || string(next) != string(raw) {
		t.Errorf("next=%q res=%+v, want nothing overwritten", next, res)
	}
}

func TestApplyFrontmatter_DateModifiedIsAlwaysRefreshed(t *testing.T) {
	kb := openTestKB(t)
	raw := []byte("---\ndateModified: \"2020-01-01\"\n---\n\nBody.\n")
	next, _, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance, FrontmatterAccept{Fields: []string{"dateModified"}}, false)
	if err != nil {
		t.Fatalf("ApplyFrontmatter: %v", err)
	}
	if !strings.Contains(string(next), "2026-03-15") || strings.Contains(string(next), "2020-01-01") {
		t.Errorf("next = %q, want dateModified refreshed to the last commit", next)
	}
}

func TestApplyFrontmatter_SetBypassesSignalsAndTheAbsentOnlyRule(t *testing.T) {
	kb := openTestKB(t)
	raw := []byte("---\ntitle: Existing\n---\n\n# Other\n")
	next, _, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance,
		FrontmatterAccept{Set: map[string]string{"title": "Forced"}}, false)
	if err != nil || !strings.Contains(string(next), "title: Forced") {
		t.Errorf("next=%q err=%v, want the explicit value written over the existing one", next, err)
	}
}

func TestApplyFrontmatter_SetEmptyValueWritesRatherThanNoOps(t *testing.T) {
	kb := openTestKB(t)
	raw := []byte("---\ntitle: Existing\n---\n\nBody.\n")
	next, res, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance,
		FrontmatterAccept{Set: map[string]string{"title": ""}}, false)
	if err != nil || !res.Written || strings.Contains(string(next), "Existing") {
		t.Errorf("next=%q res=%+v err=%v, want title cleared by an explicit empty --set", next, res, err)
	}
}

func TestApplyFrontmatter_UnknownFieldIsAnErrorBeforeAnything(t *testing.T) {
	kb := openTestKB(t)
	raw := []byte("# T\n\nx\n")
	if _, _, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance, FrontmatterAccept{Fields: []string{"nonesuch"}}, false); err == nil {
		t.Error("unknown accept field: nil error, want one")
	}
	if _, _, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance, FrontmatterAccept{Set: map[string]string{"nonesuch": "x"}}, false); err == nil {
		t.Error("unknown set field: nil error, want one")
	}
}

func TestApplyFrontmatter_RejectsAKeywordThatWasNeverProposedAndMintsNothing(t *testing.T) {
	kb := openTestKB(t)
	before, _ := kb.Concepts()
	raw := []byte("# A Real Title\n\nSome ordinary text.\n")
	next, _, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance,
		FrontmatterAccept{Fields: []string{"title"}, Keywords: []string{"zzznotaterm"}}, false)
	if err == nil || !strings.Contains(err.Error(), "zzznotaterm") {
		t.Fatalf("err = %v, want a rejection naming the term", err)
	}
	if next != nil {
		t.Errorf("next = %q, want no bytes on error", next)
	}
	if after, _ := kb.Concepts(); len(after) != len(before) {
		t.Errorf("concepts %d -> %d, want none minted", len(before), len(after))
	}
}

func seedComparisonRecord(t *testing.T, kb *KnowledgeBase) {
	t.Helper()
	pid, _ := kb.AddProject("alpha", "")
	suggestRecord(t, kb, pid, "0001", "unrelated words about the weather")
}

func TestApplyFrontmatter_NewCandidateKeywordCreatesItsConceptOnARealRun(t *testing.T) {
	kb := openTestKB(t)
	seedComparisonRecord(t, kb)
	before, _ := kb.Concepts()
	raw := []byte("# T\n\ngronkulator appears here, and gronkulator appears again.\n")
	next, res, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance, FrontmatterAccept{Keywords: []string{"gronkulator"}}, false)
	if err != nil {
		t.Fatalf("ApplyFrontmatter: %v", err)
	}
	if !strings.Contains(string(next), "gronkulator") || !res.Written {
		t.Errorf("next=%q res=%+v, want the keyword written", next, res)
	}
	if after, _ := kb.Concepts(); len(after) != len(before)+1 {
		t.Errorf("concepts %d -> %d, want exactly one created", len(before), len(after))
	}
}

func TestApplyFrontmatter_DryRunNeverTouchesTheDatabaseOrReturnsBytesToWrite(t *testing.T) {
	kb := openTestKB(t)
	seedComparisonRecord(t, kb)
	before, _ := kb.Concepts()
	raw := []byte("# T\n\ngronkulator appears here, and gronkulator appears again.\n")
	next, res, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance, FrontmatterAccept{Keywords: []string{"gronkulator"}}, true)
	if err != nil {
		t.Fatalf("ApplyFrontmatter(dry run): %v", err)
	}
	if res.Written || !res.DryRun || string(next) != string(raw) {
		t.Errorf("next=%q res=%+v, want a preview: DryRun set, Written false, input bytes back", next, res)
	}
	if len(res.Keywords.Accepted) != 1 {
		t.Errorf("Accepted = %v, want the keyword reported as accepted", res.Keywords.Accepted)
	}
	if after, _ := kb.Concepts(); len(after) != len(before) {
		t.Errorf("concepts %d -> %d on a dry run, want the database untouched", len(before), len(after))
	}
}

func TestApplyFrontmatter_KnownConceptKeywordIsAPlainWrite(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("chunking", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	before, _ := kb.Concepts()
	raw := []byte("# T\n\nchunking is mentioned twice: chunking again.\n")
	next, _, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance, FrontmatterAccept{Keywords: []string{"chunking"}}, false)
	if err != nil || !strings.Contains(string(next), "keywords:") {
		t.Fatalf("next=%q err=%v, want keywords written", next, err)
	}
	if after, _ := kb.Concepts(); len(after) != len(before) {
		t.Errorf("concepts %d -> %d, want none created for an existing concept", len(before), len(after))
	}
}

func TestSpliceFrontmatter_PrependsABlockWhenNoneExisted(t *testing.T) {
	got := spliceFrontmatter([]byte("# Title\n\nBody text.\n"), false, 0, "title: Foo")
	want := "---\ntitle: Foo\n---\n\n# Title\n\nBody text.\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSpliceFrontmatter_LeavesEveryByteOutsideTheBlockUnchanged(t *testing.T) {
	raw := []byte("---\ntitle: Old\n---\n\nBody with `code` and [[Wikilink]] untouched.\n")
	_, bodyOffset, hadBlock, err := frontmatterNode(raw)
	if err != nil || !hadBlock {
		t.Fatalf("frontmatterNode: hadBlock=%v err=%v", hadBlock, err)
	}
	got := string(spliceFrontmatter(raw, true, bodyOffset, "title: New"))
	if !strings.HasSuffix(got, "\n\nBody with `code` and [[Wikilink]] untouched.\n") || strings.Contains(got, "Old") {
		t.Errorf("got %q, want the body bytes unchanged and the old title gone", got)
	}
}

// ─── determinism (found 2026-09-23 during L4, DR-0037) ──────────────────────
//
// frontmatterRun applied accepted fields by ranging over a map, and
// setMappingField appends each new key as it goes, so the key order written to
// a file followed map iteration order: the same command gave two different
// orders in 8 runs.

func frontmatterKeyOrder(next []byte) []string {
	var keys []string
	for _, line := range strings.Split(string(next), "\n") {
		if line == "---" && len(keys) > 0 {
			break
		}
		if i := strings.Index(line, ":"); i > 0 && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "-") {
			keys = append(keys, line[:i])
		}
	}
	return keys
}

func TestApplyFrontmatter_NewKeysAreWrittenInCanonicalOrderEveryTime(t *testing.T) {
	kb := openTestKB(t)
	raw := []byte("# A Real Title\n\nBody text.\n")
	want := "title,author,dateCreated,dateModified"
	for i := 0; i < 25; i++ {
		next, _, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance,
			FrontmatterAccept{Fields: []string{"dateModified", "title", "dateCreated", "author"}}, false)
		if err != nil {
			t.Fatalf("ApplyFrontmatter: %v", err)
		}
		if got := strings.Join(frontmatterKeyOrder(next), ","); got != want {
			t.Fatalf("run %d: key order = %s, want %s regardless of the order accepted", i, got, want)
		}
	}
}

func TestApplyFrontmatter_SetKeysAreWrittenInCanonicalOrderEveryTime(t *testing.T) {
	kb := openTestKB(t)
	raw := []byte("Body only.\n")
	acc := FrontmatterAccept{Set: map[string]string{"dateModified": "d2", "author": "A", "title": "T", "dateCreated": "d1"}}
	for i := 0; i < 25; i++ {
		next, _, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance, acc, false)
		if err != nil {
			t.Fatalf("ApplyFrontmatter: %v", err)
		}
		if got := strings.Join(frontmatterKeyOrder(next), ","); got != "title,author,dateCreated,dateModified" {
			t.Fatalf("run %d: key order = %s", i, got)
		}
	}
}

func TestApplyFrontmatter_MixedSetAndAcceptStayInCanonicalOrder(t *testing.T) {
	kb := openTestKB(t)
	raw := []byte("# A Real Title\n\nBody text.\n")
	for i := 0; i < 25; i++ {
		next, _, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance,
			FrontmatterAccept{Fields: []string{"title", "dateModified"}, Set: map[string]string{"author": "Forced"}}, false)
		if err != nil {
			t.Fatalf("ApplyFrontmatter: %v", err)
		}
		if got := strings.Join(frontmatterKeyOrder(next), ","); got != "title,author,dateModified" {
			t.Fatalf("run %d: key order = %s, want canonical order across --set and --accept together", i, got)
		}
	}
}

func TestApplyFrontmatter_ExistingKeysKeepTheirPositionWhenOverwritten(t *testing.T) {
	kb := openTestKB(t)
	raw := []byte("---\nzeta: 1\ndateModified: \"2020-01-01\"\nalpha: 2\n---\n\nBody.\n")
	next, _, err := kb.ApplyFrontmatter("a.md", raw, gitLikeProvenance, FrontmatterAccept{Fields: []string{"dateModified", "title"}}, false)
	if err != nil {
		t.Fatalf("ApplyFrontmatter: %v", err)
	}
	if got := strings.Join(frontmatterKeyOrder(next), ","); got != "zeta,dateModified,alpha" {
		t.Errorf("key order = %s, want the existing keys in place (title has no proposal here)", got)
	}
}

func TestApplyFrontmatter_UnknownSetFieldErrorIsDeterministic(t *testing.T) {
	kb := openTestKB(t)
	for i := 0; i < 25; i++ {
		_, _, err := kb.ApplyFrontmatter("a.md", []byte("x\n"), gitLikeProvenance,
			FrontmatterAccept{Set: map[string]string{"zzz": "1", "aaa": "2", "mmm": "3"}}, false)
		if err == nil || !strings.Contains(err.Error(), `"aaa"`) {
			t.Fatalf("run %d: err = %v, want it to name the alphabetically first unknown field", i, err)
		}
	}
}

func TestFrontmatterFieldOrderCoversExactlyTheScalarFields(t *testing.T) {
	// ApplyFrontmatter walks frontmatterFieldOrder, so a scalar field missing
	// from it would be silently ignored, and one that is not a field would be
	// applied as one.
	inOrder := map[string]bool{}
	for _, f := range frontmatterFieldOrder {
		inOrder[f] = true
		if !frontmatterScalarFields[f] {
			t.Errorf("%q is in frontmatterFieldOrder but is not a scalar field", f)
		}
	}
	for f := range frontmatterScalarFields {
		if !inOrder[f] {
			t.Errorf("%q is a scalar field but is missing from frontmatterFieldOrder", f)
		}
	}
	if len(frontmatterFieldOrder) != len(inOrder) {
		t.Errorf("frontmatterFieldOrder has duplicates: %v", frontmatterFieldOrder)
	}
}
