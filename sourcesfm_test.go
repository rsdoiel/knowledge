package knowledge

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// ─── a `sources:` front matter key links what a file cites (DR-0081 item 4) ──

func TestParseSourceRefs(t *testing.T) {
	refs, warns := ParseSourceRefs([]string{
		"10.1234/sparql", "https://example.org/a", "source:3", "  10.5555/x.y  ",
		"doi:10.1/x", "https://doi.org/10.1/x", "not a source", "source:abc", "source:0", "ftp-ish", "",
	})
	var got []string
	for _, r := range refs {
		got = append(got, r.Kind+"="+r.Value)
	}
	want := "doi=10.1234/sparql,url=https://example.org/a,id=3,doi=10.5555/x.y"
	if strings.Join(got, ",") != want {
		t.Errorf("refs = %v, want %s", got, want)
	}
	// Each malformed entry warns once, naming it; the blank one is silent.
	if len(warns) != 6 {
		t.Errorf("warnings = %q, want 6", warns)
	}
	for _, bad := range []string{"doi:10.1/x", "https://doi.org/10.1/x", "not a source", "source:abc", "source:0", "ftp-ish"} {
		found := false
		for _, w := range warns {
			found = found || strings.Contains(w, bad)
		}
		if !found {
			t.Errorf("no warning names %q: %q", bad, warns)
		}
	}
	// The bare-DOI warning says what to write instead.
	for _, w := range warns {
		if strings.Contains(w, "doi:10.1/x") && !strings.Contains(w, "bare") {
			t.Errorf("a pasted prefix should say to write the bare DOI: %q", w)
		}
	}
}

const docWithSources = "---\ntitle: A Story\nsources:\n  - 10.1234/sparql\n  - https://example.org/a\n  - source:%d\n---\n\n## One\ntext\n"

func linksByIdent(links []SourceLink) map[string]SourceLink {
	out := map[string]SourceLink{}
	for _, l := range links {
		out[l.Source.IdentifierValue] = l
	}
	return out
}

func TestIngestDocument_SourcesKeyLinksTheSources(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	existing, _ := kb.AddSource(Source{Title: "Known paper", IdentifierType: "doi", IdentifierValue: "10.1234/sparql"})
	third, _ := kb.AddSource(Source{Title: "By id"})
	path := writeTempFile(t, "s.md", strings.Replace(docWithSources, "%d", strconv.FormatInt(third, 10), 1))
	res, err := kb.IngestDocument(pid, path, DocumentIngestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.SourcesLinked != 3 || len(res.SourceWarnings) != 0 {
		t.Errorf("result = %+v, want 3 linked and no warnings", res)
	}
	doc, _ := kb.DocumentByPath(path)
	links, _ := kb.DocumentSources(doc.ID)
	by := linksByIdent(links)
	if len(links) != 3 || by["10.1234/sparql"].Source.ID != existing || by["10.1234/sparql"].Source.Title != "Known paper" {
		t.Errorf("an existing identifier must link the existing source: %+v", links)
	}
	if u := by["https://example.org/a"]; u.Source.IdentifierType != "url" || u.Source.Title != "https://example.org/a" || u.Relationship != "cited" {
		t.Errorf("a new url becomes a source titled by its identifier: %+v", u)
	}
	if by[""].Source.ID != third {
		t.Errorf("source:%d did not link: %+v", third, links)
	}
}

func TestIngestDocument_MalformedSourceEntriesWarnAndTheRestLink(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	src := "---\ntitle: S\nsources: [\"10.1/ok\", \"doi:10.2/pasted\", \"source:999\", \"nonsense\"]\n---\n\n## One\ntext\n"
	path := writeTempFile(t, "s.md", src)
	res, err := kb.IngestDocument(pid, path, DocumentIngestOptions{})
	if err != nil {
		t.Fatalf("a bad entry in a file must not fail the ingest: %v", err)
	}
	if res.SourcesLinked != 1 || len(res.SourceWarnings) != 3 {
		t.Errorf("result = %+v, want 1 linked and 3 warnings (pasted prefix, unknown id, nonsense)", res)
	}
}

func TestIngestDocument_SourcesAreRebuiltOnEveryIngestAndRemovedWithTheEntry(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	path := writeTempFile(t, "s.md", "---\ntitle: S\nsources: [\"10.1/a\", \"10.1/b\"]\n---\n\ntext\n")
	kb.IngestDocument(pid, path, DocumentIngestOptions{})
	doc, _ := kb.DocumentByPath(path)
	if l, _ := kb.DocumentSources(doc.ID); len(l) != 2 {
		t.Fatalf("links = %+v", l)
	}

	// A skipped (unchanged) ingest rebuilds a link lost from the database.
	kb.kbExec(t, `DELETE FROM document_sources WHERE document_id = ?`, doc.ID)
	res, _ := kb.IngestDocument(pid, path, DocumentIngestOptions{})
	if res.Action != "skipped" {
		t.Fatalf("action = %q, want the unchanged file skipped", res.Action)
	}
	if l, _ := kb.DocumentSources(doc.ID); len(l) != 2 {
		t.Errorf("a skipped ingest did not rebuild the links: %+v", l)
	}

	// Dropping an entry drops the link it made; the source stays.
	os.WriteFile(path, []byte("---\ntitle: S\nsources: [\"10.1/a\"]\n---\n\ntext\n"), 0o644)
	kb.IngestDocument(pid, path, DocumentIngestOptions{})
	l, _ := kb.DocumentSources(doc.ID)
	if len(l) != 1 || l[0].Source.IdentifierValue != "10.1/a" {
		t.Errorf("links = %+v, want only 10.1/a", l)
	}
	if all, _ := kb.ListSources(); len(all) != 2 {
		t.Errorf("sources = %d, want the dropped source kept", len(all))
	}

	// An empty list is a key too: it clears the file's links.
	os.WriteFile(path, []byte("---\ntitle: S\nsources: []\n---\n\ntext\n"), 0o644)
	kb.IngestDocument(pid, path, DocumentIngestOptions{})
	if l, _ := kb.DocumentSources(doc.ID); len(l) != 0 {
		t.Errorf("sources: [] left %+v", l)
	}
}

func TestIngestDocument_HandLinksSurviveAndAFileWithNoKeyIsLeftAlone(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	path := writeTempFile(t, "s.md", "---\ntitle: S\nsources: [\"10.1/a\"]\n---\n\ntext\n")
	kb.IngestDocument(pid, path, DocumentIngestOptions{})
	doc, _ := kb.DocumentByPath(path)
	hand, _ := kb.AddSource(Source{Title: "added by hand"})
	kb.LinkDocumentSource(doc.ID, hand, "retrieved", "", "")
	os.WriteFile(path, []byte("---\ntitle: S\nsources: []\n---\n\ntext\n"), 0o644)
	kb.IngestDocument(pid, path, DocumentIngestOptions{})
	l, _ := kb.DocumentSources(doc.ID)
	if len(l) != 1 || l[0].Source.ID != hand || l[0].Relationship != "retrieved" {
		t.Errorf("links = %+v, want the hand-added retrieved link kept", l)
	}

	// With no sources key at all, the file manages nothing: a cited link made
	// by hand stays.
	kb.LinkDocumentSource(doc.ID, hand, "cited", "", "")
	os.WriteFile(path, []byte("---\ntitle: S\n---\n\ntext\n"), 0o644)
	kb.IngestDocument(pid, path, DocumentIngestOptions{})
	if l, _ := kb.DocumentSources(doc.ID); len(l) != 1 {
		t.Errorf("a file without the key changed the links: %+v", l)
	}
}

func TestIngestDocument_DryRunWritesNoSourcesAndNoLinks(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	path := writeTempFile(t, "s.md", "---\ntitle: S\nsources: [\"10.1/a\"]\n---\n\ntext\n")
	res, err := kb.IngestDocument(pid, path, DocumentIngestOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if all, _ := kb.ListSources(); len(all) != 0 {
		t.Errorf("a dry run created sources: %+v (result %+v)", all, res)
	}
}

func TestSyncRecordSources_LinksAndRebuilds(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("work", "")
	rec := addTestRecord(t, kb, Record{RecordID: "0001", ProjectID: pid, Title: "x"})
	refs, _ := ParseSourceRefs([]string{"10.1/a", "https://example.org/b"})
	n, warns, err := kb.SyncRecordSources(rec, refs)
	if err != nil || n != 2 || len(warns) != 0 {
		t.Fatalf("SyncRecordSources = %d, %v, %v", n, warns, err)
	}
	hand, _ := kb.AddSource(Source{Title: "hand"})
	kb.LinkRecordSource(rec, hand, "retrieved", "", "")
	refs, _ = ParseSourceRefs([]string{"10.1/a"})
	kb.SyncRecordSources(rec, refs)
	l, _ := kb.RecordSources(rec)
	by := linksByIdent(l)
	if len(l) != 2 || by["10.1/a"].Relationship != "cited" || by[""].Source.ID != hand {
		t.Errorf("links = %+v, want 10.1/a and the retrieved hand link", l)
	}
}

func TestRecordFile_SourcesKeyIsReadKeptAndOptional(t *testing.T) {
	base := "---\nid: \"0001\"\ntitle: \"T\"\ndate: \"2026-10-18\"\nstatus: proposed\nkind: decision\ntrigger: design\nproject: work\nphase: \"\"\nsupersedes: []\nsuperseded_by: []\nrelates_to: []\ninitiative: \"\"\nsession: \"\"\ndecisions: []\ntags: []\n%suuid: \"u\"\norigin_host: \"h\"\n---\nbody\n"
	with := strings.Replace(base, "%s", "sources: [\"10.1/a\", \"https://example.org/b\"]\n", 1)
	rf, err := ParseRecord([]byte(with), "decisions/0001-x.md")
	if err != nil {
		t.Fatal(err)
	}
	if !rf.SourcesPresent || len(rf.Sources) != 2 || rf.Sources[0] != "10.1/a" {
		t.Errorf("Sources = %v present %v", rf.Sources, rf.SourcesPresent)
	}
	for _, w := range rf.Warnings {
		if strings.Contains(w, "sources") {
			t.Errorf("sources is a known field, got warning %q", w)
		}
	}
	out, err := RenderRecordFile(rf)
	if err != nil || !strings.Contains(string(out), "sources: [10.1/a, 'https://example.org/b']") && !strings.Contains(string(out), "sources: [10.1/a, https://example.org/b]") &&
		!strings.Contains(string(out), "sources:") {
		t.Errorf("render dropped the key: %s, %v", out, err)
	}
	again, err := ParseRecord(out, "decisions/0001-x.md")
	if err != nil {
		t.Fatal(err)
	}
	out2, _ := RenderRecordFile(again)
	if string(out) != string(out2) {
		t.Errorf("render is not a fixed point:\n%s\nvs\n%s", out, out2)
	}

	// Without the key nothing changes: no key emitted, not present.
	without := strings.Replace(base, "%s", "", 1)
	rf2, _ := ParseRecord([]byte(without), "decisions/0001-x.md")
	o2, _ := RenderRecordFile(rf2)
	if rf2.SourcesPresent || strings.Contains(string(o2), "sources") {
		t.Errorf("a record with no key must render without one: %s", o2)
	}
	if string(o2) != without {
		t.Errorf("existing records must render byte for byte:\n%s\nvs\n%s", o2, without)
	}
}
