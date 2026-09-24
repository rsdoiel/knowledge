package knowledge

import (
	"os"
	"strings"
	"testing"
)

// L1 (library-lift-plan.md): IngestDocument is the library form of what
// cmd/kb's `document ingest` used to do inline. These tests exercise the
// library directly; cmd/kb/document_test.go still proves the CLI surface is
// unchanged.

const ingestFixture = `## First

Alpha text about chunking.

## Second

Beta text.
`

func ingestSections(t *testing.T, kb *KnowledgeBase, path string) (*Document, map[string]DocumentSection, DocumentSection) {
	t.Helper()
	d, err := kb.DocumentByPath(path)
	if err != nil || d == nil {
		t.Fatalf("DocumentByPath(%q) = %v, %v; want a document", path, d, err)
	}
	secs, err := kb.DocumentSections(d.ID)
	if err != nil {
		t.Fatalf("DocumentSections: %v", err)
	}
	byHeading := map[string]DocumentSection{}
	var gist DocumentSection
	for _, s := range secs {
		if s.Level == "gist" {
			gist = s
			continue
		}
		byHeading[s.Heading] = s
	}
	return d, byHeading, gist
}

func TestIngestDocument_NewDocumentCreatesRowGistAndSections(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.md", ingestFixture)

	res, err := kb.IngestDocument(pid, path, DocumentIngestOptions{})
	if err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	if res.Action != "added" || res.SectionsAdded != 2 || res.Path != path {
		t.Errorf("result = %+v, want Action=added SectionsAdded=2 Path=%q", res, path)
	}
	d, secs, gist := ingestSections(t, kb, path)
	if d.ProjectID != pid {
		t.Errorf("ProjectID = %d, want %d", d.ProjectID, pid)
	}
	if gist.ID == 0 {
		t.Error("no gist row created")
	}
	if len(secs) != 2 || secs["First"].SummaryStatus != "unsummarized" {
		t.Errorf("sections = %+v, want First and Second, unsummarized", secs)
	}
}

func TestIngestDocument_TitlePrecedence(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")

	// Explicit option beats frontmatter.
	p1 := writeTempFile(t, "one.md", "---\ntitle: \"From Frontmatter\"\n---\n## S\n\nx\n")
	if _, err := kb.IngestDocument(pid, p1, DocumentIngestOptions{Title: "Override"}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	if d, _, _ := ingestSections(t, kb, p1); d.Title != "Override" {
		t.Errorf("Title = %q, want Override", d.Title)
	}

	// Frontmatter beats the filename.
	p2 := writeTempFile(t, "two.md", "---\ntitle: \"From Frontmatter\"\n---\n## S\n\nx\n")
	if _, err := kb.IngestDocument(pid, p2, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	if d, _, _ := ingestSections(t, kb, p2); d.Title != "From Frontmatter" {
		t.Errorf("Title = %q, want From Frontmatter", d.Title)
	}

	// With neither, the file's base name is the last resort.
	p3 := writeTempFile(t, "three.txt", "plain notes\n")
	if _, err := kb.IngestDocument(pid, p3, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	if d, _, _ := ingestSections(t, kb, p3); d.Title != "three.txt" {
		t.Errorf("Title = %q, want three.txt", d.Title)
	}
}

func TestIngestDocument_UnchangedFileIsSkipped(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.md", ingestFixture)
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("first IngestDocument: %v", err)
	}
	res, err := kb.IngestDocument(pid, path, DocumentIngestOptions{})
	if err != nil {
		t.Fatalf("second IngestDocument: %v", err)
	}
	if res.Action != "skipped" {
		t.Errorf("Action = %q, want skipped", res.Action)
	}
}

func TestIngestDocument_ChangedBodyKeepsSummaryAndSetsStale(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.md", ingestFixture)
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	_, secs, _ := ingestSections(t, kb, path)
	first := secs["First"]
	if err := kb.DraftDocumentSummary(first.ID, "A kept summary.", "human", nil); err != nil {
		t.Fatalf("DraftDocumentSummary: %v", err)
	}
	if err := kb.PromoteDocumentSummary(first.ID); err != nil {
		t.Fatalf("PromoteDocumentSummary: %v", err)
	}

	changed := strings.Replace(ingestFixture, "Alpha text about chunking.", "Alpha text, rewritten.", 1)
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	res, err := kb.IngestDocument(pid, path, DocumentIngestOptions{})
	if err != nil {
		t.Fatalf("re-IngestDocument: %v", err)
	}
	if res.Action != "updated" || res.SectionsUpdated != 1 || res.SectionsStale != 1 {
		t.Errorf("result = %+v, want updated, 1 updated, 1 stale", res)
	}
	_, secs, _ = ingestSections(t, kb, path)
	got := secs["First"]
	if !got.SummaryStale || got.SummaryBody != "A kept summary." || got.SummaryStatus != "reviewed" {
		t.Errorf("First = %+v, want stale, summary and reviewed status kept", got)
	}
	if secs["Second"].SummaryStale {
		t.Error("Second (unchanged) must not be flagged stale")
	}
}

func TestIngestDocument_RemovedHeadingReportedNotDeleted(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.md", ingestFixture)
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	if err := os.WriteFile(path, []byte("## First\n\nAlpha text about chunking.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	res, err := kb.IngestDocument(pid, path, DocumentIngestOptions{})
	if err != nil {
		t.Fatalf("re-IngestDocument: %v", err)
	}
	if len(res.RemovedHeadings) != 1 || res.RemovedHeadings[0] != "Second" {
		t.Errorf("RemovedHeadings = %v, want [Second]", res.RemovedHeadings)
	}
	if _, secs, _ := ingestSections(t, kb, path); len(secs) != 2 {
		t.Errorf("sections = %d, want 2: a removed heading is reported, never deleted", len(secs))
	}
}

func TestIngestDocument_NewHeadingInsertedUnsummarized(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.md", ingestFixture)
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	if err := os.WriteFile(path, []byte(ingestFixture+"\n## Third\n\nGamma.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	res, err := kb.IngestDocument(pid, path, DocumentIngestOptions{})
	if err != nil {
		t.Fatalf("re-IngestDocument: %v", err)
	}
	if res.SectionsAdded != 1 {
		t.Errorf("SectionsAdded = %d, want 1", res.SectionsAdded)
	}
	if _, secs, _ := ingestSections(t, kb, path); secs["Third"].SummaryStatus != "unsummarized" {
		t.Errorf("Third = %+v, want unsummarized", secs["Third"])
	}
}

func TestIngestDocument_DryRunWritesNothing(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.md", ingestFixture)
	res, err := kb.IngestDocument(pid, path, DocumentIngestOptions{DryRun: true})
	if err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	if !res.DryRun || res.Action != "added" || res.SectionsAdded != 2 {
		t.Errorf("result = %+v, want a dry-run report of 2 added", res)
	}
	if d, _ := kb.DocumentByPath(path); d != nil {
		t.Errorf("DocumentByPath = %+v, want nil after a dry run", d)
	}
}

func TestIngestDocument_RejectsPDF(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.pdf", "%PDF-1.4\n")
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err == nil {
		t.Error("IngestDocument(.pdf) = nil error, want an explicit unsupported-format error")
	}
}

func TestIngestDocument_MissingFileErrors(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.IngestDocument(pid, "/nonexistent/nope.md", DocumentIngestOptions{}); err == nil {
		t.Error("IngestDocument(missing) = nil error, want a read error")
	}
}

func TestIngestDocument_ParserWarningIsCarriedInResult(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	// An unterminated frontmatter block makes the parser warn rather than fail.
	path := writeTempFile(t, "a.md", "---\ntitle: broken\n## S\n\nx\n")
	pd, err := ParseDocumentFile(path, "")
	if err != nil {
		t.Fatalf("ParseDocumentFile: %v", err)
	}
	res, err := kb.IngestDocument(pid, path, DocumentIngestOptions{})
	if err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	if res.Warning != pd.Warning {
		t.Errorf("Warning = %q, want the parser's %q", res.Warning, pd.Warning)
	}
}

// ─── tagging and density, moved with the ingest logic ───────────────────────

func TestIngestDocument_WikilinkBecomesSectionConcept(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.md", "## S\n\nAbout [[Chunking]] here.\n")
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	_, secs, _ := ingestSections(t, kb, path)
	cs, err := kb.DocumentSectionConcepts(secs["S"].ID)
	if err != nil || len(cs) != 1 || cs[0].Name != "Chunking" {
		t.Errorf("concepts = %v, %v; want [Chunking]", cs, err)
	}
}

func TestIngestDocument_RemovedWikilinkIsPrunedOnReingest(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.md", "## S\n\nAbout [[Chunking]] here.\n")
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	if err := os.WriteFile(path, []byte("## S\n\nAbout nothing here.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("re-IngestDocument: %v", err)
	}
	_, secs, _ := ingestSections(t, kb, path)
	if cs, _ := kb.DocumentSectionConcepts(secs["S"].ID); len(cs) != 0 {
		t.Errorf("concepts = %v, want the dropped wikilink pruned", cs)
	}
}

func TestIngestDocument_FrontmatterKeywordsLinkToGist(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.md", "---\nkeywords: [\"chunking\"]\n---\n## S\n\nBody.\n")
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	_, _, gist := ingestSections(t, kb, path)
	cs, err := kb.DocumentSectionConcepts(gist.ID)
	if err != nil || len(cs) != 1 || cs[0].Name != "chunking" {
		t.Errorf("gist concepts = %v, %v; want [chunking]", cs, err)
	}
}

func TestIngestDocument_DensityLinkingThresholdAndCodeExclusion(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	if _, err := kb.AddConcept("rename", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	twice := writeTempFile(t, "twice.md", "## S\n\nA rename, then another rename.\n")
	once := writeTempFile(t, "once.md", "## S\n\nA single rename only.\n")
	code := writeTempFile(t, "code.md", "## S\n\nUse `rename` and `rename` here.\n")
	for _, p := range []string{twice, once, code} {
		if _, err := kb.IngestDocument(pid, p, DocumentIngestOptions{}); err != nil {
			t.Fatalf("IngestDocument(%s): %v", p, err)
		}
	}
	want := map[string]int{twice: 1, once: 0, code: 0}
	for p, n := range want {
		_, secs, _ := ingestSections(t, kb, p)
		cs, _ := kb.DocumentSectionConcepts(secs["S"].ID)
		if len(cs) != n {
			t.Errorf("%s: %d concept links, want %d (>1 mention outside code spans links)", p, len(cs), n)
		}
	}
}

func TestIngestDocument_TagDensityCountsKnownConcepts(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	for _, c := range []string{"chunking", "retrieval"} {
		if _, err := kb.AddConcept(c, ""); err != nil {
			t.Fatalf("AddConcept: %v", err)
		}
	}
	path := writeTempFile(t, "a.md", "## S\n\nWe discuss chunking and retrieval.\n")
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	_, secs, gist := ingestSections(t, kb, path)
	if secs["S"].TagDensity != 2 || gist.TagDensity != 2 {
		t.Errorf("TagDensity section=%d gist=%d, want 2 and 2", secs["S"].TagDensity, gist.TagDensity)
	}
}

func TestStripCodeSpans_RemovesFencedAndInlineKeepsWordBoundaries(t *testing.T) {
	got := StripCodeSpans("a`x`b\n```\nfenced\n```\nc")
	if strings.Contains(got, "x") || strings.Contains(got, "fenced") {
		t.Errorf("StripCodeSpans left code behind: %q", got)
	}
	if strings.Contains(got, "ab") {
		t.Errorf("StripCodeSpans fused neighbours across a removed span: %q", got)
	}
}

// ─── determinism (found 2026-09-23 auditing map iteration, DR-0037) ─────────
//
// reingestChangedDocument built RemovedHeadings by ranging over a map, so when
// a re-ingest dropped two or more headings they came back in a different order
// on every run, in both the text and the --json output.

func TestIngestDocument_RemovedHeadingsAreReportedInDocumentOrderEveryTime(t *testing.T) {
	const before = "## Alpha\n\na\n\n## Bravo\n\nb\n\n## Charlie\n\nc\n\n## Delta\n\nd\n\n## Echo\n\ne\n"
	for i := 0; i < 20; i++ {
		kb := openTestKB(t)
		pid, _ := kb.AddProject("alpha", "")
		path := writeTempFile(t, "a.md", before)
		if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
			t.Fatalf("IngestDocument: %v", err)
		}
		if err := os.WriteFile(path, []byte("## Foxtrot\n\nnew\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		res, err := kb.IngestDocument(pid, path, DocumentIngestOptions{})
		if err != nil {
			t.Fatalf("re-IngestDocument: %v", err)
		}
		if got := strings.Join(res.RemovedHeadings, ","); got != "Alpha,Bravo,Charlie,Delta,Echo" {
			t.Fatalf("run %d: RemovedHeadings = %s, want the old document's own order", i, got)
		}
	}
}

func TestIngestDocument_RemovedHeadingsListsARepeatedHeadingOnce(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.md", "## Same\n\none\n\n## Same\n\ntwo\n\n## Other\n\nx\n")
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	if err := os.WriteFile(path, []byte("## Fresh\n\nnew\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	res, _ := kb.IngestDocument(pid, path, DocumentIngestOptions{})
	seen := map[string]int{}
	for _, h := range res.RemovedHeadings {
		seen[h]++
	}
	if seen["Same"] != 1 || seen["Other"] != 1 {
		t.Errorf("RemovedHeadings = %v, want each removed heading exactly once", res.RemovedHeadings)
	}
}

// ─── a wikilink inside code is not a tag (found 2026-09-23, DR-0037) ─────────

func TestIngestDocument_WikilinkInsideCodeDoesNotMintOrLinkAConcept(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	body := "## S\n\nThe log line is `[[recall: something]]` in inline code.\n\n```\n[[tool: fenced example]]\n```\n\nA real tag: [[RealOne]].\n"
	path := writeTempFile(t, "a.md", body)
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	_, secs, gist := ingestSections(t, kb, path)
	for _, id := range []int64{secs["S"].ID, gist.ID} {
		cs, _ := kb.DocumentSectionConcepts(id)
		if len(cs) != 1 || cs[0].Name != "RealOne" {
			t.Errorf("section %d concepts = %v, want only RealOne", id, cs)
		}
	}
	all, _ := kb.Concepts()
	for _, c := range all {
		if strings.Contains(c.Name, "recall") || strings.Contains(c.Name, "tool") {
			t.Errorf("concept %q was minted from a wikilink inside code", c.Name)
		}
	}
}

func TestIngestDocument_WikilinkBothInAndOutOfCodeStillLinks(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, "a.md", "## S\n\nUse `[[Both]]` like [[Both]] here.\n")
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	_, secs, _ := ingestSections(t, kb, path)
	if cs, _ := kb.DocumentSectionConcepts(secs["S"].ID); len(cs) != 1 || cs[0].Name != "Both" {
		t.Errorf("concepts = %v, want Both linked by its real mention", cs)
	}
}
