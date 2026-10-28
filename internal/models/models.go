// Package models manages the embedding model files kb's semantic search needs
// (DR-0089 item 6): which models kb knows, where installed ones live, and how to
// install one from a directory or archive (offline) or fetch it (online),
// checking every file against a recorded size and SHA-256. It reads and writes
// files only; it does not run a model.
package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"
)

// ManifestName is the file in an installed model directory that records what
// was installed.
const ManifestName = "kb-model.json"

// Errors the caller classifies. Operating-system errors (missing path,
// permission, existing target) are returned as they are.
var (
	// ErrInvalid means the files are not the model they claim to be: one is
	// missing, the wrong size, or fails its checksum.
	ErrInvalid = errors.New("invalid model files")
	// ErrTruncated means a read or write stopped part way.
	ErrTruncated = errors.New("transfer stopped part way")
	// ErrUnavailable means the source could not be reached or refused the request.
	ErrUnavailable = errors.New("model source unavailable")
)

/** File is one file of a model with the size and SHA-256 it must have.
 *
 * Fields:
 *   Name   (string) — the file's name inside the model directory
 *   SHA256 (string) — lower-case hex digest
 *   Size   (int64)  — length in bytes
 */
type File struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

/** Spec describes a model kb knows how to install.
 *
 * Fields:
 *   Name       (string) — the model's name, also its directory name
 *   Dimensions (int)    — length of the vectors it produces
 *   Licence    (string) — the licence its files are published under
 *   Source     (string) — the base URL of its files, pinned to a revision
 *   Files      ([]File) — every file the model needs, with checksums
 */
type Spec struct {
	Name       string `json:"name"`
	Dimensions int    `json:"dimensions"`
	Licence    string `json:"licence"`
	Source     string `json:"source"`
	Files      []File `json:"files"`
}

// miniLM is all-MiniLM-L6-v2, the first and provisional model (DR-0088). The
// ONNX copy is KnightsAnalytics/all-MiniLM-L6-v2, whose model card gives
// Apache-2.0 (checked 2026-10-28); Source is pinned to the revision the sums
// were taken from.
var miniLM = Spec{
	Name:       "all-MiniLM-L6-v2",
	Dimensions: 384,
	Licence:    "Apache-2.0",
	Source:     "https://huggingface.co/KnightsAnalytics/all-MiniLM-L6-v2/resolve/89d6440f2265494cfb4c0c81089de229313d395e",
	Files: []File{
		{"config.json", "953f9c0d463486b10a6871cc2fd59f223b2c70184f49815e7efbcab5d8908b41", 612},
		{"model.onnx", "6fd5d72fe4589f189f8ebc006442dbb529bb7ce38f8082112682524616046452", 90405214},
		{"special_tokens_map.json", "303df45a03609e4ead04bc3dc1536d0ab19b5358db685b6f3da123d05ec200e3", 112},
		{"tokenizer.json", "be50c3628f2bf5bb5e3a7f17b1f74611b2561a3a27eeab05e5aa30f411572037", 466247},
		{"tokenizer_config.json", "acb92769e8195aabd29b7b2137a9e6d6e25c476a4f15aa4355c233426c61576b", 350},
		{"vocab.txt", "07eced375cec144d27c900241f3e339478dec958f92fddbc551f295c992038a3", 231508},
	},
}

// known is the registry; tests may replace it.
var known = []Spec{miniLM}

/** Known returns the models kb can install, default first.
 *
 * Returns:
 *   []Spec — a copy of the registry
 *
 * Example:
 *   for _, s := range models.Known() { fmt.Println(s.Name) }
 */
func Known() []Spec { return append([]Spec(nil), known...) }

/** Find returns the known model called name.
 *
 * Parameters:
 *   name (string) — a model name
 *
 * Returns:
 *   Spec — the model
 *   bool — false when kb does not know the name
 *
 * Example:
 *   spec, ok := models.Find("all-MiniLM-L6-v2")
 */
func Find(name string) (Spec, bool) {
	for _, s := range known {
		if s.Name == name {
			return s, true
		}
	}
	return Spec{}, false
}

/** Register adds a model to the registry, or replaces the one with the same name.
 * kb's own model is built in; this is for tests, and for a program that bundles
 * another model.
 *
 * Parameters:
 *   s (Spec) — the model, with its files' sizes and checksums
 *
 * Example:
 *   models.Register(models.Spec{Name: "tiny", Files: files})
 */
func Register(s Spec) {
	for i := range known {
		if known[i].Name == s.Name {
			known[i] = s
			return
		}
	}
	known = append(known, s)
}

/** Unregister removes the model called name from the registry, the built-in
 * one included.
 *
 * Parameters:
 *   name (string) — the model's name
 *
 * Example:
 *   defer models.Unregister("tiny")
 */
func Unregister(name string) {
	for i := range known {
		if known[i].Name == name {
			known = append(known[:i:i], known[i+1:]...)
			return
		}
	}
}

/** Root is a directory that holds model directories.
 *
 * Fields:
 *   Path  (string) — the directory
 *   Scope (string) — "override", "workspace" or "user"
 */
type Root struct {
	Path  string
	Scope string
}

/** UserDir returns the per-user model directory: $XDG_DATA_HOME/kb/models, or
 * the platform's equivalent (~/.local/share, ~/Library/Application Support,
 * %LocalAppData%).
 *
 * Returns:
 *   string — the directory (it may not exist yet)
 *   error — the home directory could not be found
 *
 * Example:
 *   dir, err := models.UserDir()
 */
func UserDir() (string, error) {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" && runtime.GOOS != "windows" {
		return filepath.Join(x, "kb", "models"), nil
	}
	switch runtime.GOOS {
	case "windows":
		if d := os.Getenv("LocalAppData"); d != "" {
			return filepath.Join(d, "kb", "models"), nil
		}
	case "darwin":
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, "Library", "Application Support", "kb", "models"), nil
		}
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".local", "share", "kb", "models"), nil
}

/** WorkspaceDir returns the model directory of a workspace.
 *
 * Parameters:
 *   root (string) — the workspace root
 *
 * Returns:
 *   string — root/agents/models
 *
 * Example:
 *   models.WorkspaceDir("/home/me/Laboratory") // /home/me/Laboratory/agents/models
 */
func WorkspaceDir(root string) string { return filepath.Join(root, "agents", "models") }

/** Roots returns the directories searched for an installed model, in order. An
 * override (the -dir flag, else KB_MODEL_DIR) replaces the others; otherwise the
 * workspace's comes first, then the user's.
 *
 * Parameters:
 *   override  (string) — a directory from a flag, or "" for the environment
 *   workspace (string) — the workspace root, or "" when there is none
 *
 * Returns:
 *   []Root — the directories to search
 *
 * Example:
 *   roots := models.Roots("", "/home/me/Laboratory")
 */
func Roots(override, workspace string) []Root {
	if override == "" {
		override = os.Getenv("KB_MODEL_DIR")
	}
	if override != "" {
		return []Root{{override, "override"}}
	}
	var roots []Root
	if workspace != "" {
		roots = append(roots, Root{WorkspaceDir(workspace), "workspace"})
	}
	if u, err := UserDir(); err == nil {
		roots = append(roots, Root{u, "user"})
	}
	return roots
}

/** InstallRoot returns where an install or fetch writes: the override, else the
 * workspace's directory when asked for, else the user's.
 *
 * Parameters:
 *   override  (string) — a directory from a flag, or "" for the environment
 *   workspace (string) — the workspace root when -workspace was given, else ""
 *
 * Returns:
 *   Root — the target directory
 *   error — the user directory could not be worked out
 *
 * Example:
 *   root, err := models.InstallRoot("", "")
 */
func InstallRoot(override, workspace string) (Root, error) {
	if override == "" {
		override = os.Getenv("KB_MODEL_DIR")
	}
	switch {
	case override != "":
		return Root{override, "override"}, nil
	case workspace != "":
		return Root{WorkspaceDir(workspace), "workspace"}, nil
	}
	u, err := UserDir()
	return Root{u, "user"}, err
}

/** Installed is a model found in a root.
 *
 * Fields:
 *   Name     (string)   — the model's name
 *   Dir      (string)   — its directory
 *   Scope    (string)   — the root's scope
 *   Known    (bool)     — kb has a Spec for the name
 *   Complete (bool)     — every file is present at its recorded size
 *   Missing  ([]string) — files absent or the wrong size
 *   Licence  (string)   — from the manifest
 */
type Installed struct {
	Name     string   `json:"name"`
	Dir      string   `json:"dir"`
	Scope    string   `json:"scope"`
	Known    bool     `json:"known"`
	Complete bool     `json:"complete"`
	Missing  []string `json:"missing,omitempty"`
	Licence  string   `json:"licence,omitempty"`
}

// manifest is what ManifestName holds.
type manifest struct {
	Spec
	Installed string `json:"installed"`
	Origin    string `json:"origin"`
}

/** List returns the models installed in roots, a root's models by name. A
 * missing root is not an error. Completeness is a size check only; Verify does
 * the checksums.
 *
 * Parameters:
 *   roots ([]Root) — the directories to read
 *
 * Returns:
 *   []Installed — the models found
 *   error — a root could not be read
 *
 * Example:
 *   found, err := models.List(models.Roots("", ""))
 */
func List(roots []Root) ([]Installed, error) {
	var out []Installed
	for _, r := range roots {
		ents, err := os.ReadDir(r.Path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, e := range ents {
			if !e.IsDir() || e.Name()[0] == '.' {
				continue
			}
			dir := filepath.Join(r.Path, e.Name())
			spec, ok := Find(e.Name())
			in := Installed{Name: e.Name(), Dir: dir, Scope: r.Scope, Known: ok}
			if ok {
				in.Licence = spec.Licence
				in.Missing = quickCheck(dir, spec)
				in.Complete = len(in.Missing) == 0
			}
			out = append(out, in)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out, nil
}

// quickCheck names the spec's files that are absent or the wrong size.
func quickCheck(dir string, spec Spec) []string {
	var bad []string
	for _, f := range spec.Files {
		fi, err := os.Stat(filepath.Join(dir, f.Name))
		if err != nil || !fi.Mode().IsRegular() || fi.Size() != f.Size {
			bad = append(bad, f.Name)
		}
	}
	return bad
}

/** Lookup finds the installed model called name, searching roots in order.
 *
 * Parameters:
 *   name  (string) — the model's name
 *   roots ([]Root) — where to look
 *
 * Returns:
 *   Installed — the first complete match
 *   bool — false when none is installed and complete
 *
 * Example:
 *   m, ok := models.Lookup("all-MiniLM-L6-v2", models.Roots("", ws))
 */
func Lookup(name string, roots []Root) (Installed, bool) {
	all, err := List(roots)
	if err != nil {
		return Installed{}, false
	}
	for _, in := range all {
		if in.Name == name && in.Complete {
			return in, true
		}
	}
	return Installed{}, false
}

/** Verify checks every file of spec in dir against its recorded size and
 * SHA-256.
 *
 * Parameters:
 *   dir  (string) — a model directory
 *   spec (Spec)   — the model it should be
 *
 * Returns:
 *   error — nil when all match; an error wrapping ErrInvalid naming the first
 *     file that does not; an OS error when a file cannot be read
 *
 * Example:
 *   err := models.Verify(dir, spec)
 */
func Verify(dir string, spec Spec) error {
	for _, f := range spec.Files {
		p := filepath.Join(dir, f.Name)
		fi, err := os.Stat(p)
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s is missing from %s", ErrInvalid, f.Name, dir)
		}
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() || fi.Size() != f.Size {
			return fmt.Errorf("%w: %s is not the expected file (size %d, want %d)", ErrInvalid, p, fi.Size(), f.Size)
		}
		h, err := sumFile(p)
		if err != nil {
			return err
		}
		if h != f.SHA256 {
			return fmt.Errorf("%w: checksum mismatch for %s: got %s, want %s", ErrInvalid, p, h, f.SHA256)
		}
	}
	return nil
}

func sumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// publish moves a finished staging directory to root/spec.Name, writing the
// manifest first. The target must not exist.
func publish(stage, root string, spec Spec, origin string) (Installed, error) {
	m := manifest{Spec: spec, Installed: time.Now().UTC().Format(time.RFC3339), Origin: origin}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Installed{}, err
	}
	if err := os.WriteFile(filepath.Join(stage, ManifestName), append(b, '\n'), 0o644); err != nil {
		return Installed{}, err
	}
	dest := filepath.Join(root, spec.Name)
	if _, err := os.Lstat(dest); err == nil {
		return Installed{}, fmt.Errorf("%s is already installed (remove it to install again): %w", dest, fs.ErrExist)
	}
	if err := os.Rename(stage, dest); err != nil {
		return Installed{}, err
	}
	return Installed{Name: spec.Name, Dir: dest, Known: true, Complete: true, Licence: spec.Licence}, nil
}

// stageDir makes a scratch directory beside the target so the final rename
// stays on one file system.
func stageDir(root string) (string, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(root, ".install-")
}
