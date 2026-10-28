package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The whole provenance path through the real command line (mainRun): a document
// and a record that list their sources, an observation linked by hand, a
// retraction, an export from one database and an import into another, then
// search and recall (DR-0080, DR-0081).
func TestProvenance_EndToEndThroughTheCommandLine(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	run := func(db string, args ...string) (string, string, int) {
		t.Helper()
		var out, errOut bytes.Buffer
		code := mainRun(append([]string{"--db", filepath.Join(dir, db)}, args...), &out, &errOut)
		return out.String(), errOut.String(), code
	}
	must := func(db string, args ...string) string {
		t.Helper()
		out, errOut, code := run(db, args...)
		if code != 0 {
			t.Fatalf("kb %v: exit %d: %s", args, code, errOut)
		}
		return out
	}

	must("a.db", "project", "add", "work")
	must("a.db", "source", "add", "SPARQL 1.1 Query Language", "--doi", "10.1234/sparql")
	// A document and a record each list what they cite.
	doc := filepath.Join(dir, "story.md")
	os.WriteFile(doc, []byte("---\ntitle: A Story\nsources: [\"10.1234/sparql\", \"https://example.org/page\"]\n---\n\n## One\nhedgehog text\n"), 0o644)
	must("a.db", "document", "ingest", doc, "--project", "work")
	recDir := filepath.Join(dir, "work", "decisions")
	os.MkdirAll(recDir, 0o755)
	os.WriteFile(filepath.Join(recDir, "0001-x.md"), []byte("---\nid: \"0001\"\ntitle: \"Use SPARQL for marmalade queries\"\ndate: \"2026-10-18\"\nstatus: accepted\nkind: decision\ntrigger: design\nproject: work\nphase: \"\"\nsupersedes: []\nsuperseded_by: []\nrelates_to: []\ninitiative: \"\"\nsession: \"\"\ndecisions: []\ntags: []\nsources: [\"10.1234/sparql\"]\nuuid: \"01a15008-0000-7000-8000-000000000001\"\norigin_host: \"h\"\n---\n\n**Decision.** Use SPARQL for marmalade queries.\n"), 0o644)
	must("a.db", "ingest", filepath.Join(dir, "work"))
	must("a.db", "observation", "add", "--project", "work", "finding", "marmalade result checked against the paper")
	must("a.db", "source", "link", "1", "1", "--version", "1.1", "--hash", "sha256:aa")
	must("a.db", "source", "retract", "1", "withdrawn by the publisher")

	// Counts of what cites the source.
	show := must("a.db", "source", "show", "1")
	if !strings.Contains(show, "cited by 1 observation(s), 1 document(s), 1 record(s)") {
		t.Errorf("source show:\n%s", show)
	}

	// Search says what supports each hit and marks the retraction.
	text := must("a.db", "search", "marmalade")
	if strings.Count(text, "[retracted source: SPARQL 1.1 Query Language]") != 2 {
		t.Errorf("search should mark the observation and the record:\n%s", text)
	}
	js := must("a.db", "search", "work/", "marmalade")
	if js == "" {
		t.Fatal("no output")
	}

	// Export, import elsewhere, and the same answers come back.
	dump := filepath.Join(dir, "dump.jsonl")
	must("a.db", "export", "-out", dump)
	must("b.db", "import", "-in", dump)
	again := must("b.db", "search", "marmalade")
	if strings.Count(again, "[retracted source: SPARQL 1.1 Query Language]") != 2 {
		t.Errorf("after import:\n%s", again)
	}
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := mainRun([]string{"--json", "--db", filepath.Join(dir, "b.db"), "search", "work/", "marmalade"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("scoped json search: %d %s", code, errOut.String())
	}
	var doc2 struct {
		Results []struct {
			Type      string `json:"source_type"`
			Retracted bool   `json:"retracted"`
			Sources   []struct {
				Title         string `json:"title"`
				SourceVersion string `json:"source_version"`
				SourceHash    string `json:"source_hash"`
			} `json:"sources"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc2); err != nil {
		t.Fatal(err)
	}
	seenObs := false
	for _, r := range doc2.Results {
		if r.Type == "observation" {
			seenObs = true
			if !r.Retracted || len(r.Sources) != 1 || r.Sources[0].SourceVersion != "1.1" || r.Sources[0].SourceHash != "sha256:aa" {
				t.Errorf("the observation's provenance did not survive the import: %+v", r)
			}
		}
	}
	if !seenObs {
		t.Errorf("no observation hit: %s", out.String())
	}

	// The sources kind finds the source and the projects that cite it.
	srcs := must("b.db", "search", "/sources", "sparql")
	if !strings.Contains(srcs, "SPARQL 1.1 Query Language") || !strings.Contains(srcs, "[work]") {
		t.Errorf("sources search:\n%s", srcs)
	}
	// And check-db agrees the two databases are in step.
	if out, _, code := run("b.db", "check-db", "--jsonl", dump); code != 0 {
		t.Errorf("check-db after the import: exit %d\n%s", code, out)
	}
}
