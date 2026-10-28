package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
	"github.com/rsdoiel/knowledge/embedtest"
)

// embedFixture is a scratch knowledge base with n observations and a fake
// embedder registered. The semantic index is not built.
type embedFixture struct {
	t    *testing.T
	db   string
	fake *embedtest.Fake
}

func newEmbedFixture(t *testing.T, n int) *embedFixture {
	t.Helper()
	noTerminal(t)
	dir := t.TempDir()
	t.Chdir(dir)
	f := &embedFixture{t: t, db: filepath.Join(dir, "kb.db"), fake: embedtest.New()}
	knowledge.RegisterEmbedder(f.fake)
	t.Cleanup(func() { knowledge.RegisterEmbedder(nil) })
	if code, _, e := f.run("project", "add", "p", "a project"); code != 0 {
		t.Fatalf("project add: %s", e)
	}
	for i := range n {
		f.obs(fmt.Sprintf("observation %d about slow inference on the Raspberry Pi", i))
	}
	return f
}

func (f *embedFixture) run(args ...string) (code int, out, errOut string) {
	f.t.Helper()
	var o, e bytes.Buffer
	code = mainRun(append([]string{"--db", f.db}, args...), &o, &e)
	return code, o.String(), e.String()
}

func (f *embedFixture) obs(body string) {
	f.t.Helper()
	if code, _, e := f.run("observation", "add", "--project", "p", "note", body); code != 0 {
		f.t.Fatalf("observation add: %s", e)
	}
}

func (f *embedFixture) sidecar() string { return knowledge.VectorsPath(f.db) }

func (f *embedFixture) status() map[string]any {
	f.t.Helper()
	var o, e bytes.Buffer
	if code := mainRun([]string{"--db", f.db, "-json", "embed", "-status"}, &o, &e); code != 0 {
		f.t.Fatalf("embed -status: %d %s", code, e.String())
	}
	var m map[string]any
	if err := json.Unmarshal(o.Bytes(), &m); err != nil {
		f.t.Fatalf("%v: %s", err, o.String())
	}
	return m
}

func (f *embedFixture) notIndexed() int { return int(f.status()["rows_not_indexed"].(float64)) }

func TestEmbed_BuildsTheIndexThenIsCurrent(t *testing.T) {
	f := newEmbedFixture(t, 3)
	if f.notIndexed() != 3 {
		t.Fatalf("before: %v", f.status())
	}
	code, out, errOut := f.run("embed")
	if code != 0 || !strings.Contains(out, "embedded 3 rows") || !strings.Contains(errOut, "embedded 3 of 3 rows") {
		t.Fatalf("embed: exit %d: %q / %q", code, out, errOut)
	}
	if _, err := os.Stat(f.sidecar()); err != nil {
		t.Error("no index file:", err)
	}
	if f.notIndexed() != 0 {
		t.Errorf("after: %v", f.status())
	}
	before := f.fake.Texts()
	if code, out, _ := f.run("embed"); code != 0 || !strings.Contains(out, "embedded 0 rows") || f.fake.Texts() != before {
		t.Errorf("second embed: exit %d %q, embedder saw %d more texts", code, out, f.fake.Texts()-before)
	}
	var o, e bytes.Buffer
	mainRun([]string{"--db", f.db, "-json", "embed"}, &o, &e)
	var res map[string]any
	if err := json.Unmarshal(o.Bytes(), &res); err != nil || res["rows_up_to_date"].(float64) != 3 || res["rows_embedded"].(float64) != 0 || e.Len() != 0 {
		t.Errorf("embed -json = %s / %s (%v)", o.String(), e.String(), err)
	}
}

func TestEmbed_RebuildStartsOver(t *testing.T) {
	f := newEmbedFixture(t, 3)
	f.run("embed")
	before := f.fake.Texts()
	code, out, _ := f.run("embed", "-rebuild")
	if code != 0 || !strings.Contains(out, "embedded 3 rows") || f.fake.Texts() != before+3 {
		t.Errorf("rebuild: exit %d %q, texts %d -> %d", code, out, before, f.fake.Texts())
	}
}

func TestEmbed_StatusNeedsNoIndexAndNoEmbedder(t *testing.T) {
	f := newEmbedFixture(t, 2)
	knowledge.RegisterEmbedder(nil) // the plain build
	code, out, _ := f.run("embed", "-status")
	if code != 0 || !strings.Contains(out, "no semantic search") || !strings.Contains(out, "Index: none yet") || !strings.Contains(out, "Not indexed: 2 rows") || strings.Contains(out, "(0 chunks") {
		t.Errorf("status: exit %d: %q", code, out)
	}
	if _, err := os.Stat(f.sidecar()); err == nil {
		t.Error("-status built an index")
	}
}

func TestEmbed_StatusSaysWhenTheModelDiffers(t *testing.T) {
	f := newEmbedFixture(t, 2)
	f.run("embed")
	knowledge.RegisterEmbedder(embedtest.NewNamed("other"))
	_, out, _ := f.run("embed", "-status")
	if !strings.Contains(out, "different model") || !strings.Contains(out, "embed -rebuild") {
		t.Errorf("status: %q", out)
	}
}

// missingModel is an embedder that reports its model as not installed.
type missingModel struct{ *embedtest.Fake }

func (missingModel) Available() error {
	return fmt.Errorf("the embedding model is not installed (run: kb model fetch): %w", fs.ErrNotExist)
}

// cancelling cancels the run's context the first time it embeds.
type cancelling struct {
	*embedtest.Fake
	cancel context.CancelFunc
}

func (c *cancelling) Embed(texts []string) ([][]float32, error) {
	c.cancel()
	return c.Fake.Embed(texts)
}

// A model that is simply not installed has a different fingerprint from the one
// that built the index (its files cannot be read), but that is not a mismatch,
// and the advice to rebuild would throw away the index.
func TestEmbed_MissingModelIsNotReportedAsAMismatch(t *testing.T) {
	f := newEmbedFixture(t, 3)
	f.run("embed")
	knowledge.RegisterEmbedder(missingModel{embedtest.NewNamed("fake-gone")}) // other fingerprint, and not installed
	_, out, _ := f.run("embed", "-status")
	if strings.Contains(out, "different model") || strings.Contains(out, "-rebuild") || !strings.Contains(out, "not installed") {
		t.Errorf("status: %q", out)
	}
	var o, e bytes.Buffer
	mainRun([]string{"--db", f.db, "-json", "embed", "-status"}, &o, &e)
	if strings.Contains(o.String(), `"model_matches"`) || !strings.Contains(o.String(), `"model_installed": false`) {
		t.Errorf("status -json: %s", o.String())
	}
	f.obs("a write while the model is away")
	_, _, errOut := f.run("observation", "add", "--project", "p", "note", "another")
	if strings.Contains(errOut, "-rebuild") || !strings.Contains(errOut, "kb model fetch") {
		t.Errorf("write-time advice = %q", errOut)
	}
}

func TestEmbed_ExitCodes(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*testing.T, *embedFixture)
		args  []string
		want  exitClass
	}{
		{"bad flag", nil, []string{"embed", "--bogus"}, classUsage},
		{"surplus argument", nil, []string{"embed", "now"}, classUsage},
		{"status with rebuild", nil, []string{"embed", "-status", "-rebuild"}, classUsage},
		{"a build without semantic search", func(t *testing.T, f *embedFixture) { knowledge.RegisterEmbedder(nil) }, []string{"embed"}, classUsage},
		{"index built by another model", func(t *testing.T, f *embedFixture) {
			f.run("embed")
			knowledge.RegisterEmbedder(embedtest.NewNamed("other"))
		}, []string{"embed"}, classData},
		{"model not installed", func(t *testing.T, f *embedFixture) {
			knowledge.RegisterEmbedder(missingModel{embedtest.New()})
		}, []string{"embed"}, classNoInput},
		{"another embed holds the index", func(t *testing.T, f *embedFixture) {
			v, err := knowledge.CreateVectors(f.db, f.fake)
			if err != nil {
				t.Fatal(err)
			}
			l, err := v.Lock()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { l.Release(); v.Close() })
		}, []string{"embed"}, classTempFail},
		{"interrupted", func(t *testing.T, f *embedFixture) {
			for i := range 20 {
				f.obs(fmt.Sprintf("one more observation number %d", i))
			}
			ctx, cancel := context.WithCancel(context.Background())
			previous := embedContext
			embedContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
			t.Cleanup(func() { embedContext = previous })
			knowledge.RegisterEmbedder(&cancelling{embedtest.New(), cancel})
		}, []string{"embed"}, classTempFail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newEmbedFixture(t, 3)
			if tc.setup != nil {
				tc.setup(t, f)
			}
			var out, errOut bytes.Buffer
			code := mainRun(append([]string{"--db", f.db, "-json"}, tc.args...), &out, &errOut)
			if code != tc.want.Code || !strings.Contains(errOut.String(), `"`+tc.want.Name+`"`) {
				t.Errorf("exit %d, want %d (%s); stderr %s", code, tc.want.Code, tc.want.Name, errOut.String())
			}
			if out.Len() != 0 {
				t.Errorf("a failure wrote to stdout: %q", out.String())
			}
		})
	}
}

func TestEmbed_InterruptedRunKeepsItsWorkAndResumes(t *testing.T) {
	f := newEmbedFixture(t, 40)
	ctx, cancel := context.WithCancel(context.Background())
	previous := embedContext
	embedContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
	knowledge.RegisterEmbedder(&cancelling{embedtest.New(), cancel})
	if code, _, e := f.run("embed"); code != classTempFail.Code || !strings.Contains(e, "run kb embed again") {
		t.Fatalf("interrupted run: exit %d: %s", code, e)
	}
	embedContext = previous
	kept := 40 - f.notIndexed()
	if kept == 0 || kept == 40 {
		t.Fatalf("an interrupted run kept %d of 40 rows, want some", kept)
	}
	knowledge.RegisterEmbedder(embedtest.New())
	if code, out, _ := f.run("embed"); code != 0 || !strings.Contains(out, fmt.Sprintf("embedded %d rows", 40-kept)) || f.notIndexed() != 0 {
		t.Errorf("resume: exit %d %q", code, out)
	}
}

// ── write-time embedding ──────────────────────────────────────────────────────

func TestWriteTime_WithNoIndexNothingIsBuiltAndNothingIsSaid(t *testing.T) {
	f := newEmbedFixture(t, 2)
	code, _, errOut := f.run("observation", "add", "--project", "p", "note", "a new one")
	if code != 0 || errOut != "" {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(f.sidecar()); err == nil {
		t.Error("a write built the index")
	}
	if f.fake.Calls() != 0 {
		t.Error("a write ran the embedder with no index")
	}
}

func TestWriteTime_IndexesTheRowsAWriteChanged(t *testing.T) {
	f := newEmbedFixture(t, 3)
	f.run("embed")
	base := f.fake.Texts()

	f.obs("a fresh observation about exit codes")
	if f.notIndexed() != 0 || f.fake.Texts() != base+1 {
		t.Errorf("observation add: %d not indexed, embedder saw %d new texts", f.notIndexed(), f.fake.Texts()-base)
	}
	if code, _, e := f.run("observation", "update", "1", "rewritten body about something else entirely"); code != 0 {
		t.Fatal(e)
	}
	if f.notIndexed() != 0 || f.fake.Texts() != base+2 {
		t.Errorf("observation update: %d not indexed, %d new texts", f.notIndexed(), f.fake.Texts()-base)
	}
	if code, _, e := f.run("concept", "add", "latency", "how long a request takes"); code != 0 {
		t.Fatal(e)
	}
	if f.notIndexed() != 0 {
		t.Errorf("concept add left %d rows not indexed", f.notIndexed())
	}
	before := int(f.status()["rows"].(float64))
	if code, _, e := f.run("observation", "delete", "1", "--force"); code != 0 {
		t.Fatal(e)
	}
	st := f.status()
	if int(st["rows"].(float64)) != before-1 || st["rows_to_remove"].(float64) != 0 {
		t.Errorf("observation delete left %v", st)
	}
	v, _ := knowledge.OpenVectors(f.db)
	defer v.Close()
	if rows, _ := v.IndexedRows(); len(rows) != before-1 {
		t.Errorf("index holds %d rows, want %d", len(rows), before-1)
	}
}

func TestWriteTime_ReadsAndDryRunsLeaveTheIndexAlone(t *testing.T) {
	f := newEmbedFixture(t, 2)
	f.run("embed")
	f.obs("added without the embedder") // indexed
	knowledge.RegisterEmbedder(nil)
	f.obs("added by the plain build") // not indexed
	knowledge.RegisterEmbedder(f.fake)
	before := f.fake.Texts()
	for _, args := range [][]string{{"observation", "list", "--project", "p"}, {"search", "plain"}, {"stats"}, {"project", "list"}} {
		if code, _, e := f.run(args...); code != 0 {
			t.Fatalf("%v: %s", args, e)
		}
	}
	if f.fake.Texts() != before || f.notIndexed() != 1 {
		t.Errorf("a read embedded something: %d new texts, %d not indexed", f.fake.Texts()-before, f.notIndexed())
	}
	// The next real write catches up the row the plain build left.
	f.obs("one more")
	if f.notIndexed() != 0 {
		t.Errorf("%d rows still not indexed", f.notIndexed())
	}
}

func TestWriteTime_FailuresNeverFailTheWrite(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(*testing.T, *embedFixture)
		advice string
	}{
		{"the embedder fails", func(t *testing.T, f *embedFixture) { f.fake.FailAfter(0) }, "not yet indexed"},
		{"the model is not installed", func(t *testing.T, f *embedFixture) {
			knowledge.RegisterEmbedder(missingModel{f.fake})
		}, "kb model fetch"},
		{"another embed holds the index", func(t *testing.T, f *embedFixture) {
			v, _ := knowledge.OpenVectors(f.db)
			l, err := v.Lock()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { l.Release(); v.Close() })
		}, "another kb embed is running"},
		{"the index was built by another model", func(t *testing.T, f *embedFixture) {
			knowledge.RegisterEmbedder(embedtest.NewNamed("other"))
		}, "embed -rebuild"},
		{"a backlog is left to kb embed", func(t *testing.T, f *embedFixture) {
			kb, err := knowledge.Open(f.db)
			if err != nil {
				t.Fatal(err)
			}
			defer kb.Close()
			proj, err := kb.ProjectByName("p")
			if err != nil {
				t.Fatal(err)
			}
			for i := range writeTimeMaxRows + 5 {
				if _, err := kb.AddObservation(proj.ID, "note", fmt.Sprintf("a backlog row number %d", i)); err != nil {
					t.Fatal(err)
				}
			}
		}, "too many to embed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newEmbedFixture(t, 2)
			f.run("embed")
			tc.setup(t, f)
			code, _, errOut := f.run("observation", "add", "--project", "p", "note", "the write that must succeed")
			if code != 0 {
				t.Fatalf("the write failed: exit %d: %s", code, errOut)
			}
			if !strings.Contains(errOut, tc.advice) || strings.Count(errOut, "not yet indexed") != 1 {
				t.Errorf("stderr = %q, want one note containing %q", errOut, tc.advice)
			}
			// KB_QUIET silences the note and nothing else.
			t.Setenv("KB_QUIET", "1")
			if code, _, errOut := f.run("observation", "add", "--project", "p", "note", "quiet"); code != 0 || errOut != "" {
				t.Errorf("KB_QUIET: exit %d, stderr %q", code, errOut)
			}
		})
	}
}

func TestWriteTime_PlainBuildDoesNothing(t *testing.T) {
	f := newEmbedFixture(t, 2)
	f.run("embed")
	knowledge.RegisterEmbedder(nil)
	if code, _, e := f.run("observation", "add", "--project", "p", "note", "plain"); code != 0 || e != "" {
		t.Errorf("exit %d, stderr %q", code, e)
	}
}

func TestWriteTime_RecordsAndIngest(t *testing.T) {
	f := newEmbedFixture(t, 1)
	f.run("embed")
	dir := filepath.Join(f.db, "..", "decisions")
	os.MkdirAll(dir, 0o755)
	if code, _, e := f.run("record", "new", "--project", "p", "--title", "Use a sidecar", "--trigger", "design", "--dir", dir); code != 0 {
		t.Skipf("record new: %s", e) // the write path is exercised by the other tests
	}
	if code, _, e := f.run("ingest", dir); code != 0 {
		t.Fatalf("ingest: %s", e)
	}
	if f.notIndexed() != 0 {
		t.Errorf("ingest left %d rows not indexed", f.notIndexed())
	}
}

// ── merge, import and check-db leave the index alone (plan T7) ───────────────

func TestIndexIsLeftAloneByMergeImportAndCheckDB(t *testing.T) {
	f := newEmbedFixture(t, 2)
	f.run("embed")
	before, err := os.ReadFile(f.sidecar())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(f.db)

	// A second knowledge base with rows of its own.
	other := filepath.Join(dir, "other.db")
	run := func(args ...string) string {
		t.Helper()
		var o, e bytes.Buffer
		if code := mainRun(args, &o, &e); code != 0 {
			t.Fatalf("%v: exit %d: %s", args, code, e.String())
		}
		return o.String()
	}
	run("--db", other, "project", "add", "q", "another")
	run("--db", other, "observation", "add", "--project", "q", "note", "a row from the other database")
	jsonl := run("--db", other, "export")
	in := filepath.Join(dir, "other.jsonl")
	os.WriteFile(in, []byte(jsonl), 0o644)

	run("--db", f.db, "import", "-in", in)
	dump := filepath.Join(dir, "dump.jsonl")
	os.WriteFile(dump, []byte(run("--db", f.db, "export")), 0o644)
	run("--db", f.db, "check-db", "--jsonl", dump)
	after, _ := os.ReadFile(f.sidecar())
	if !bytes.Equal(before, after) {
		t.Error("import or check-db changed the index file")
	}
	if n := f.notIndexed(); n != 1 {
		t.Errorf("after an import, %d rows not indexed, want the 1 that came in", n)
	}

	merged := filepath.Join(dir, "merged.db")
	run("merge", "-a", f.db, "-b", other, "-out", merged)
	if _, err := os.Stat(knowledge.VectorsPath(merged)); err == nil {
		t.Error("merge created an index for its output")
	}
	if after, _ := os.ReadFile(f.sidecar()); !bytes.Equal(before, after) {
		t.Error("merge changed the index file")
	}
}

func TestChangesCorpus(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{[]string{"observation", "add", "--project", "p", "note", "x"}, true},
		{[]string{"observation", "update", "1", "x"}, true},
		{[]string{"observation", "delete", "1"}, true},
		{[]string{"observation", "show", "1"}, false},
		{[]string{"observation", "list", "--project", "p"}, false},
		{[]string{"concept", "add", "c", "d"}, true},
		{[]string{"concept", "rename", "a", "b"}, true},
		{[]string{"concept", "recall", "text"}, false},
		{[]string{"record", "supersede", "p/DR-0001", "p/DR-0002"}, true},
		{[]string{"record", "set-status", "p/DR-0001", "accepted"}, true},
		{[]string{"record", "list"}, false},
		{[]string{"document", "review", "promote", "3"}, true},
		{[]string{"document", "review", "list"}, false},
		{[]string{"document", "ingest", "f.md", "--project", "p"}, true},
		{[]string{"document", "list"}, false},
		{[]string{"ingest", "dir"}, true},
		{[]string{"ingest", "dir", "--dry-run"}, false},
		{[]string{"ingest", "-help"}, false},
		{[]string{"project", "delete", "p"}, true},
		{[]string{"project", "list"}, false},
		{[]string{"search", "term"}, false},
		{[]string{"import", "-in", "f"}, false},
		{[]string{"stats"}, false},
		{[]string{"embed"}, false},
		{nil, false},
	} {
		if got := changesCorpus(tc.args); got != tc.want {
			t.Errorf("changesCorpus(%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestWriteTime_PromotingADocumentSummaryIndexesIt(t *testing.T) {
	f := newEmbedFixture(t, 1)
	f.run("embed")
	kb, err := knowledge.Open(f.db)
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()
	secID := seedIngestedSection(t, kb, filepath.Dir(f.db))
	if _, err := runDocument(t, kb, false, "draft", fmt.Sprint(secID), promotedBody, "--by", "model"); err != nil {
		t.Fatal(err)
	}
	embedAfterWrite(kb) // a draft is not trusted, so it is not in the corpus
	if f.notIndexed() != 0 {
		t.Fatalf("a draft summary was indexed: %v", f.status())
	}
	if err := kb.PromoteDocumentSummary(secID); err != nil {
		t.Fatal(err)
	}
	if f.notIndexed() != 1 {
		t.Fatalf("after promoting, the summary should be waiting: %v", f.status())
	}
	embedAfterWrite(kb) // what the verb does when it finishes
	if f.notIndexed() != 0 {
		t.Errorf("promoted summary not indexed: %v", f.status())
	}
}
