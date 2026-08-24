package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

func TestCmdExport_ToStdoutStreamsRawJSONL(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("p", "d"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	var out bytes.Buffer
	if err := cmdExport(kb, nil, false, nil, &out); err != nil {
		t.Fatalf("cmdExport: %v", err)
	}
	if !strings.Contains(out.String(), `"type":"project"`) {
		t.Errorf("stdout = %q, want raw JSONL containing a project record", out.String())
	}
}

func TestCmdExport_ToFileWritesConfirmation(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("p", "d"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	outPath := filepath.Join(t.TempDir(), "export.jsonl")

	var out bytes.Buffer
	if err := cmdExport(kb, nil, false, []string{"-out", outPath}, &out); err != nil {
		t.Fatalf("cmdExport: %v", err)
	}
	if out.String() == "" {
		t.Error("expected a text confirmation on stdout when -out is used")
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), `"type":"project"`) {
		t.Errorf("file contents = %q, want a project record", string(data))
	}
}

func TestCmdExport_JSONModeWithOutEmitsSummary(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("p", "d"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	outPath := filepath.Join(t.TempDir(), "export.jsonl")

	var out bytes.Buffer
	if err := cmdExport(kb, nil, true, []string{"-out", outPath}, &out); err != nil {
		t.Fatalf("cmdExport: %v", err)
	}
	assertValidJSON(t, out.Bytes())
	var got struct {
		LinesWritten int    `json:"lines_written"`
		Path         string `json:"path"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.LinesWritten != 1 || got.Path != outPath {
		t.Errorf("got = %+v, want LinesWritten=1 Path=%q", got, outPath)
	}
}

func TestCmdExport_ScopedByProjectFlag(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("p1", ""); err != nil {
		t.Fatalf("AddProject p1: %v", err)
	}
	if _, err := kb.AddProject("p2", ""); err != nil {
		t.Fatalf("AddProject p2: %v", err)
	}

	var out bytes.Buffer
	if err := cmdExport(kb, nil, false, []string{"-project", "p1"}, &out); err != nil {
		t.Fatalf("cmdExport: %v", err)
	}
	if strings.Count(out.String(), `"type":"project"`) != 1 {
		t.Errorf("stdout = %q, want exactly one project record", out.String())
	}
	if !strings.Contains(out.String(), `"name":"p1"`) {
		t.Errorf("stdout = %q, want the p1 project record", out.String())
	}
}

func TestCmdExport_UnknownProjectErrors(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdExport(kb, nil, false, []string{"-project", "nope"}, &out); err == nil {
		t.Error("expected an error for an unknown project")
	}
}

func TestCmdImport_FromFileTextSummary(t *testing.T) {
	src := openTestKB(t)
	if _, err := src.AddProject("p", "d"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var buf bytes.Buffer
	if err := knowledge.ExportJSONL(src, &buf, ""); err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}
	inPath := filepath.Join(t.TempDir(), "import.jsonl")
	if err := os.WriteFile(inPath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	dst := openTestKB(t)
	var out bytes.Buffer
	if err := cmdImport(dst, nil, false, []string{"-in", inPath}, &out); err != nil {
		t.Fatalf("cmdImport: %v", err)
	}
	if !strings.Contains(out.String(), "project") {
		t.Errorf("stdout = %q, want a per-type summary mentioning \"project\"", out.String())
	}
	projects, err := dst.Projects()
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("dst.Projects() = %d, want 1", len(projects))
	}
}

func TestCmdImport_JSONModeEmitsSummary(t *testing.T) {
	src := openTestKB(t)
	if _, err := src.AddProject("p", "d"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var buf bytes.Buffer
	if err := knowledge.ExportJSONL(src, &buf, ""); err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}
	inPath := filepath.Join(t.TempDir(), "import.jsonl")
	if err := os.WriteFile(inPath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	dst := openTestKB(t)
	var out bytes.Buffer
	if err := cmdImport(dst, nil, true, []string{"-in", inPath}, &out); err != nil {
		t.Fatalf("cmdImport: %v", err)
	}
	assertValidJSON(t, out.Bytes())
	var got []knowledge.ImportTableSummary
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected a non-empty JSON summary")
	}
}

func TestCmdImport_MalformedInputErrors(t *testing.T) {
	inPath := filepath.Join(t.TempDir(), "bad.jsonl")
	if err := os.WriteFile(inPath, []byte("not json\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	dst := openTestKB(t)
	var out bytes.Buffer
	if err := cmdImport(dst, nil, false, []string{"-in", inPath}, &out); err == nil {
		t.Error("expected an error for malformed input")
	}
}

func TestMainRun_ExportImportRoundTripThroughFiles(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	srcDBPath := filepath.Join(dir, "src.db")
	buildTestDB(t, srcDBPath, func(kb *knowledge.KnowledgeBase) {
		kb.AddProject("roundtrip", "via mainRun")
	})

	exportPath := filepath.Join(dir, "export.jsonl")
	var out, errOut bytes.Buffer
	code := mainRun([]string{"--db", srcDBPath, "export", "-out", exportPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("export exit code = %d, errOut=%q", code, errOut.String())
	}

	dstDBPath := filepath.Join(dir, "dst.db")
	out.Reset()
	errOut.Reset()
	code = mainRun([]string{"--db", dstDBPath, "import", "-in", exportPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("import exit code = %d, errOut=%q", code, errOut.String())
	}

	dst, err := knowledge.Open(dstDBPath)
	if err != nil {
		t.Fatalf("Open dst: %v", err)
	}
	defer dst.Close()
	p, err := dst.ProjectByName("roundtrip")
	if err != nil || p == nil {
		t.Fatalf("ProjectByName: %v (p=%v)", err, p)
	}
}
