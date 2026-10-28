package models

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tiny swaps the registry for a two-file model and returns its spec and a
// directory holding the right files.
func tiny(t *testing.T) (Spec, string) {
	t.Helper()
	files := map[string]string{"model.onnx": "weights weights weights", "tokenizer.json": "{}"}
	dir := t.TempDir()
	spec := Spec{Name: "tiny", Dimensions: 4, Licence: "Apache-2.0"}
	for _, n := range []string{"model.onnx", "tokenizer.json"} {
		sum := sha256.Sum256([]byte(files[n]))
		spec.Files = append(spec.Files, File{n, hex.EncodeToString(sum[:]), int64(len(files[n]))})
		if err := os.WriteFile(filepath.Join(dir, n), []byte(files[n]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := known
	known = []Spec{spec}
	t.Cleanup(func() { known = old })
	return spec, dir
}

func noStaging(t *testing.T, root string) {
	t.Helper()
	ents, _ := os.ReadDir(root)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			t.Errorf("scratch directory %s left behind", e.Name())
		}
	}
}

func TestRegisteredModelIsComplete(t *testing.T) {
	s, ok := Find("all-MiniLM-L6-v2")
	if !ok || s.Licence == "" || s.Dimensions != 384 || !strings.Contains(s.Source, "/resolve/") {
		t.Fatalf("default model spec incomplete: %+v", s)
	}
	for _, f := range s.Files {
		if len(f.SHA256) != 64 || f.Size <= 0 {
			t.Errorf("%s: bad checksum or size", f.Name)
		}
	}
}

func TestInstallFromDirectory(t *testing.T) {
	spec, src := tiny(t)
	root := filepath.Join(t.TempDir(), "models")
	in, err := Install(src, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if in.Name != "tiny" || in.Dir != filepath.Join(root, "tiny") {
		t.Errorf("installed %+v", in)
	}
	if err := Verify(in.Dir, spec); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(filepath.Join(in.Dir, ManifestName)); err != nil {
		t.Error("no manifest:", err)
	}
	noStaging(t, root)
	if m, ok := Lookup("tiny", []Root{{root, "user"}}); !ok || m.Scope != "user" {
		t.Errorf("Lookup = %+v, %v", m, ok)
	}
	// A second install finds the target.
	if _, err := Install(src, root, "tiny"); !errors.Is(err, fs.ErrExist) {
		t.Errorf("second install: %v, want fs.ErrExist", err)
	}
}

func TestInstallRejectsBadFiles(t *testing.T) {
	_, src := tiny(t)
	root := t.TempDir()
	os.WriteFile(filepath.Join(src, "tokenizer.json"), []byte("{!"), 0o644) // same size, other bytes
	if _, err := Install(src, root, ""); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "checksum") {
		t.Errorf("changed file: %v", err)
	}
	os.Remove(filepath.Join(src, "tokenizer.json"))
	if _, err := Install(src, root, ""); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "missing") {
		t.Errorf("missing file: %v", err)
	}
	if _, err := Install(src, root, "no-such-model"); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown name: %v", err)
	}
	if _, err := Install(filepath.Join(root, "absent"), root, ""); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("absent source: %v", err)
	}
	noStaging(t, root)
	if got, _ := os.ReadDir(root); len(got) != 0 {
		t.Errorf("a failed install left %d entries", len(got))
	}
}

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	for n, b := range files {
		w, _ := zw.Create(n)
		w.Write([]byte(b))
	}
	zw.Close()
	f.Close()
}

func writeTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, _ := os.Create(path)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for n, b := range files {
		tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: int64(len(b)), Typeflag: tar.TypeReg})
		tw.Write([]byte(b))
	}
	tw.Close()
	gz.Close()
	f.Close()
}

func TestInstallFromArchives(t *testing.T) {
	_, src := tiny(t)
	model, _ := os.ReadFile(filepath.Join(src, "model.onnx"))
	tok, _ := os.ReadFile(filepath.Join(src, "tokenizer.json"))
	for _, tc := range []struct {
		name  string
		write func(*testing.T, string, map[string]string)
		files map[string]string
	}{
		{"flat.zip", writeZip, map[string]string{"model.onnx": string(model), "tokenizer.json": string(tok)}},
		{"nested.zip", writeZip, map[string]string{"tiny/model.onnx": string(model), "tiny/tokenizer.json": string(tok)}},
		{"flat.tar.gz", writeTarGz, map[string]string{"model.onnx": string(model), "tokenizer.json": string(tok)}},
		{"nested.tgz", writeTarGz, map[string]string{"tiny/model.onnx": string(model), "tiny/tokenizer.json": string(tok)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			arch := filepath.Join(t.TempDir(), tc.name)
			tc.write(t, arch, tc.files)
			root := t.TempDir()
			in, err := Install(arch, root, "")
			if err != nil {
				t.Fatal(err)
			}
			if !in.Complete {
				t.Errorf("%+v", in)
			}
			noStaging(t, root)
		})
	}
}

func TestInstallRefusesEscapingArchive(t *testing.T) {
	tiny(t)
	arch := filepath.Join(t.TempDir(), "evil.zip")
	writeZip(t, arch, map[string]string{"../escape.txt": "x"})
	root := t.TempDir()
	if _, err := Install(arch, root, ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("escaping entry: %v", err)
	}
	if _, err := Install(filepath.Join(t.TempDir(), "x.rar"), root, ""); err == nil {
		t.Error("unknown archive type accepted")
	}
}

func serve(t *testing.T, dir string, h func(http.ResponseWriter, *http.Request, string)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h(w, r, filepath.Join(dir, filepath.Base(r.URL.Path)))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/rev"
}

func TestFetch(t *testing.T) {
	spec, src := tiny(t)
	url := serve(t, src, func(w http.ResponseWriter, r *http.Request, p string) { http.ServeFile(w, r, p) })
	root := t.TempDir()
	var progress strings.Builder
	in, err := Fetch(spec, root, url, &progress)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(in.Dir, spec); err != nil {
		t.Error(err)
	}
	if !strings.Contains(progress.String(), "model.onnx") {
		t.Errorf("progress %q", progress.String())
	}
	noStaging(t, root)
	if _, err := Fetch(spec, root, url, nil); !errors.Is(err, fs.ErrExist) {
		t.Errorf("fetch over an install: %v", err)
	}
}

func TestFetchFailures(t *testing.T) {
	spec, src := tiny(t)
	for _, tc := range []struct {
		name string
		h    func(http.ResponseWriter, *http.Request, string)
		want error
	}{
		{"404", func(w http.ResponseWriter, r *http.Request, p string) { http.NotFound(w, r) }, fs.ErrNotExist},
		{"500", func(w http.ResponseWriter, r *http.Request, p string) { http.Error(w, "down", 500) }, ErrUnavailable},
		{"wrong bytes", func(w http.ResponseWriter, r *http.Request, p string) {
			b, _ := os.ReadFile(p)
			w.Write([]byte(strings.Repeat("x", len(b))))
		}, ErrInvalid},
		{"too long", func(w http.ResponseWriter, r *http.Request, p string) {
			b, _ := os.ReadFile(p)
			w.Write(append(b, '!'))
		}, ErrInvalid},
		{"truncated", func(w http.ResponseWriter, r *http.Request, p string) {
			b, _ := os.ReadFile(p)
			w.Header().Set("Content-Length", "1000")
			w.Write(b[:3])
		}, ErrTruncated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			url := serve(t, src, tc.h)
			root := t.TempDir()
			if _, err := Fetch(spec, root, url, nil); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			noStaging(t, root)
			if got, _ := os.ReadDir(root); len(got) != 0 {
				t.Errorf("left %d entries", len(got))
			}
		})
	}
}

func TestRootsAndListing(t *testing.T) {
	t.Setenv("KB_MODEL_DIR", "")
	t.Setenv("XDG_DATA_HOME", "/data")
	r := Roots("", "/ws")
	if len(r) != 2 || r[0].Scope != "workspace" || r[0].Path != filepath.Join("/ws", "agents", "models") || r[1].Scope != "user" {
		t.Errorf("Roots = %+v", r)
	}
	if got := Roots("", ""); len(got) != 1 || got[0].Scope != "user" {
		t.Errorf("no workspace: %+v", got)
	}
	t.Setenv("KB_MODEL_DIR", "/env")
	if got := Roots("", "/ws"); len(got) != 1 || got[0].Path != "/env" || got[0].Scope != "override" {
		t.Errorf("env override: %+v", got)
	}
	if got := Roots("/flag", "/ws"); got[0].Path != "/flag" {
		t.Errorf("flag override: %+v", got)
	}
	if w, _ := InstallRoot("", "/ws"); w.Path != "/env" {
		t.Errorf("InstallRoot ignored the environment: %+v", w)
	}
	t.Setenv("KB_MODEL_DIR", "")
	if w, _ := InstallRoot("", "/ws"); w.Scope != "workspace" {
		t.Errorf("InstallRoot = %+v", w)
	}
	if w, _ := InstallRoot("", ""); w.Path != filepath.Join("/data", "kb", "models") {
		t.Errorf("InstallRoot user = %+v", w)
	}

	// An incomplete install is listed but not found by Lookup.
	_, src := tiny(t)
	root := t.TempDir()
	if _, err := Install(src, root, ""); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(root, "tiny", "model.onnx"))
	all, err := List([]Root{{root, "user"}, {filepath.Join(root, "absent"), "workspace"}})
	if err != nil || len(all) != 1 || all[0].Complete || len(all[0].Missing) != 1 {
		t.Errorf("List = %+v, %v", all, err)
	}
	if _, ok := Lookup("tiny", []Root{{root, "user"}}); ok {
		t.Error("Lookup found an incomplete model")
	}
}

func TestRegisteredSumsMatchRealFiles(t *testing.T) {
	dir := os.Getenv("KB_TEST_MODEL_DIR")
	if dir == "" {
		dir = "../../../tmp/hugot-spike/models/KnightsAnalytics_all-MiniLM-L6-v2"
	}
	if _, err := os.Stat(filepath.Join(dir, "model.onnx")); err != nil {
		t.Skip("no model directory: set KB_TEST_MODEL_DIR")
	}
	spec, _ := Find("all-MiniLM-L6-v2")
	if err := Verify(dir, spec); err != nil {
		t.Error(err)
	}
}
