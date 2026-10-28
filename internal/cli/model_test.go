package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsdoiel/knowledge/embedtest"
	"github.com/rsdoiel/knowledge/internal/models"

	knowledge "github.com/rsdoiel/knowledge"
)

// modelEnv gives a test a user model directory of its own, no KB_MODEL_DIR, a
// scratch working directory and a two-file model named "tiny" in the registry.
// It returns the directory holding the model's files.
func modelEnv(t *testing.T) (work, src string) {
	t.Helper()
	noTerminal(t)
	work = t.TempDir()
	t.Chdir(work)
	t.Setenv("XDG_DATA_HOME", filepath.Join(work, "data"))
	t.Setenv("KB_MODEL_DIR", "")
	src = filepath.Join(work, "src")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := models.Spec{Name: "tiny", Dimensions: 4, Licence: "Apache-2.0"}
	for n, body := range map[string]string{"model.onnx": "weights weights", "tokenizer.json": "{}"} {
		sum := sha256.Sum256([]byte(body))
		spec.Files = append(spec.Files, models.File{Name: n, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(body))})
		if err := os.WriteFile(filepath.Join(src, n), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	models.Register(spec)
	t.Cleanup(func() { models.Unregister("tiny") })
	return work, src
}

func runModel(t *testing.T, args ...string) (code int, out, errOut string) {
	t.Helper()
	var o, e bytes.Buffer
	code = mainRun(append([]string{"model"}, args...), &o, &e)
	return code, o.String(), e.String()
}

func TestModel_InstallListAndFetchRoundTrip(t *testing.T) {
	work, src := modelEnv(t)
	code, out, errOut := runModel(t, "install", src)
	if code != 0 {
		t.Fatalf("install: exit %d: %s", code, errOut)
	}
	userDir := filepath.Join(work, "data", "kb", "models", "tiny")
	if !strings.Contains(out, userDir) || !strings.Contains(errOut, "no semantic search") {
		t.Errorf("install said %q / %q", out, errOut)
	}
	if _, err := os.Stat(filepath.Join(userDir, "model.onnx")); err != nil {
		t.Error(err)
	}

	code, out, _ = runModel(t, "list")
	if code != 0 || !strings.Contains(out, "tiny\tuser\tok\t"+userDir) || !strings.Contains(out, "no semantic search") || !strings.Contains(out, "No vector index") {
		t.Errorf("list: exit %d: %q", code, out)
	}

	var o, e bytes.Buffer
	if c := mainRun([]string{"-json", "model", "list"}, &o, &e); c != 0 {
		t.Fatalf("list -json: %d %s", c, e.String())
	}
	var doc struct {
		SemanticBuild bool               `json:"semantic_build"`
		Models        []models.Installed `json:"models"`
		Index         *struct{}          `json:"index"`
	}
	if err := json.Unmarshal(o.Bytes(), &doc); err != nil || doc.SemanticBuild || len(doc.Models) != 1 || !doc.Models[0].Complete || doc.Index != nil {
		t.Errorf("list -json = %s (%v)", o.String(), err)
	}
}

func TestModel_ListSaysWhenTheBuildHasAnEmbedder(t *testing.T) {
	modelEnv(t)
	knowledge.RegisterEmbedder(embedtest.New())
	t.Cleanup(func() { knowledge.RegisterEmbedder(nil) })
	_, out, _ := runModel(t, "list")
	if !strings.Contains(out, "has semantic search (embedder fake)") || !strings.Contains(out, "No models installed") {
		t.Errorf("list: %q", out)
	}
	var o, e bytes.Buffer
	mainRun([]string{"-json", "model", "list"}, &o, &e)
	if !strings.Contains(o.String(), `"models": []`) && !strings.Contains(o.String(), `"models":[]`) {
		t.Errorf("empty list is not []: %s", o.String())
	}
}

func TestModel_WorkspaceInstallAndLookupOrder(t *testing.T) {
	work, src := modelEnv(t)
	ws := filepath.Join(work, "ws")
	os.MkdirAll(filepath.Join(ws, "agents"), 0o755)
	os.WriteFile(filepath.Join(ws, "agents", "knowledge.jsonl"), nil, 0o644)
	t.Chdir(ws)
	if code, _, errOut := runModel(t, "install", src, "-workspace"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(ws, "agents", "models", "tiny", "kb-model.json")); err != nil {
		t.Error(err)
	}
	// The same model at user level too: both are listed, workspace first.
	if code, _, errOut := runModel(t, "install", src); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	_, out, _ := runModel(t, "list")
	w, u := strings.Index(out, "tiny\tworkspace"), strings.Index(out, "tiny\tuser")
	if w < 0 || u < 0 {
		t.Fatalf("list: %q", out)
	}
	// -dir and KB_MODEL_DIR replace both.
	other := filepath.Join(work, "other")
	if code, _, _ := runModel(t, "install", src, "-dir", other); code != 0 {
		t.Fatal("install -dir failed")
	}
	_, out, _ = runModel(t, "list", "-dir", other)
	if strings.Count(out, "tiny\t") != 1 || !strings.Contains(out, "tiny\toverride") {
		t.Errorf("list -dir: %q", out)
	}
	t.Setenv("KB_MODEL_DIR", other)
	_, out, _ = runModel(t, "list")
	if strings.Count(out, "tiny\t") != 1 || !strings.Contains(out, "tiny\toverride") {
		t.Errorf("list with KB_MODEL_DIR: %q", out)
	}
}

func modelServer(t *testing.T, src string, h func(w http.ResponseWriter, r *http.Request, file string)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h(w, r, filepath.Join(src, filepath.Base(r.URL.Path)))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/rev"
}

func TestModel_Fetch(t *testing.T) {
	work, src := modelEnv(t)
	url := modelServer(t, src, func(w http.ResponseWriter, r *http.Request, f string) { http.ServeFile(w, r, f) })
	code, out, errOut := runModel(t, "fetch", "tiny", "-source", url)
	if code != 0 || !strings.Contains(out, filepath.Join(work, "data", "kb", "models", "tiny")) || !strings.Contains(errOut, "fetching model.onnx") {
		t.Errorf("fetch: exit %d: %q / %q", code, out, errOut)
	}
}

// TestModel_ExitCodes gives one failure of each class the verb can produce
// (workspace DR-0003).
func TestModel_ExitCodes(t *testing.T) {
	work, src := modelEnv(t)
	ok := modelServer(t, src, func(w http.ResponseWriter, r *http.Request, f string) { http.ServeFile(w, r, f) })
	notFound := modelServer(t, src, func(w http.ResponseWriter, r *http.Request, f string) { http.NotFound(w, r) })
	down := modelServer(t, src, func(w http.ResponseWriter, r *http.Request, f string) { http.Error(w, "down", 503) })
	truncated := modelServer(t, src, func(w http.ResponseWriter, r *http.Request, f string) {
		b, _ := os.ReadFile(f)
		w.Header().Set("Content-Length", "999")
		w.Write(b[:2])
	})
	wrong := modelServer(t, src, func(w http.ResponseWriter, r *http.Request, f string) {
		b, _ := os.ReadFile(f)
		w.Write(bytes.Repeat([]byte("x"), len(b)))
	})
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()

	bad := filepath.Join(work, "bad")
	os.Mkdir(bad, 0o755)
	os.WriteFile(filepath.Join(bad, "model.onnx"), []byte("not the model"), 0o644)

	cases := []struct {
		name string
		args []string
		want exitClass
	}{
		{"no subverb", nil, classUsage},
		{"unknown subverb", []string{"remove"}, classUsage},
		{"install without PATH", []string{"install"}, classUsage},
		{"install surplus argument", []string{"install", src, "extra"}, classUsage},
		{"install bad flag", []string{"install", src, "--bogus"}, classUsage},
		{"install unknown name", []string{"install", src, "-name", "nonesuch"}, classUsage},
		{"fetch unknown model", []string{"fetch", "nonesuch"}, classUsage},
		{"fetch surplus argument", []string{"fetch", "tiny", "again"}, classUsage},
		{"list surplus argument", []string{"list", "extra"}, classUsage},
		{"flag without value", []string{"install", src, "-dir"}, classUsage},
		{"files are not the model", []string{"install", bad}, classData},
		{"install source missing", []string{"install", filepath.Join(work, "absent")}, classNoInput},
		{"-workspace without a workspace", []string{"install", src, "-workspace"}, classNoInput},
		{"fetch 404", []string{"fetch", "tiny", "-source", notFound}, classNoInput},
		{"fetch source answers 503", []string{"fetch", "tiny", "-source", down}, classUnavailable},
		{"fetch source unreachable", []string{"fetch", "tiny", "-source", deadURL}, classUnavailable},
		{"fetch download stops part way", []string{"fetch", "tiny", "-source", truncated}, classIO},
		{"fetch wrong bytes", []string{"fetch", "tiny", "-source", wrong}, classData},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := mainRun(append([]string{"-json", "model"}, tc.args...), &out, &errOut)
			if code != tc.want.Code {
				t.Fatalf("exit %d, want %d (%s); stderr %s", code, tc.want.Code, tc.want.Name, errOut.String())
			}
			if out.Len() != 0 {
				t.Errorf("a failure wrote to stdout: %q", out.String())
			}
			if !strings.Contains(errOut.String(), `"`+tc.want.Name+`"`) {
				t.Errorf("error does not carry class %q: %s", tc.want.Name, errOut.String())
			}
		})
	}
	// Nothing was left half-installed by any of them.
	if ents, _ := os.ReadDir(filepath.Join(work, "data", "kb", "models")); len(ents) != 0 {
		t.Errorf("failed commands left %d entries", len(ents))
	}

	t.Run("target exists", func(t *testing.T) {
		if code, _, e := runModel(t, "fetch", "tiny", "-source", ok); code != 0 {
			t.Fatalf("first fetch: %d %s", code, e)
		}
		for _, args := range [][]string{{"install", src}, {"fetch", "tiny", "-source", ok}} {
			if code, _, e := runModel(t, args...); code != classCantCreate.Code {
				t.Errorf("%v again: exit %d, want 73 (%s)", args, code, e)
			}
		}
	})

	t.Run("directory refused by the OS", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root is not refused")
		}
		locked := filepath.Join(work, "locked")
		os.Mkdir(locked, 0o500)
		t.Cleanup(func() { os.Chmod(locked, 0o700) })
		if code, _, e := runModel(t, "install", src, "-dir", filepath.Join(locked, "models")); code != classNoPermission.Code {
			t.Errorf("exit %d, want 77: %s", code, e)
		}
	})
}

func TestModel_RefusesDB(t *testing.T) {
	modelEnv(t)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"--db", "x.db", "model", "list"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "does not apply to model") {
		t.Errorf("exit %d: %s", code, errOut.String())
	}
	if _, err := os.Stat("x.db"); err == nil {
		t.Error("--db created a database")
	}
}

func TestModel_BareInstallPointsToFetch(t *testing.T) {
	modelEnv(t)
	_, _, errOut := runModel(t, "install")
	if !strings.Contains(errOut, "kb model fetch") {
		t.Errorf("message does not point to fetch: %q", errOut)
	}
}

func TestModel_NeverOpensADatabase(t *testing.T) {
	work, src := modelEnv(t)
	runModel(t, "install", src)
	runModel(t, "list")
	if _, err := os.Stat(filepath.Join(work, "agents")); err == nil {
		t.Error("model created an agents/ directory")
	}
}
