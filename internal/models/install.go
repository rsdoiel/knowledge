package models

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

/** Install copies a model from a directory or an archive (.zip, .tar.gz, .tgz)
 * into root, after checking every file against the recorded size and SHA-256.
 * It uses no network. The model is copied into a scratch directory and moved
 * into place last, so a failure leaves nothing half-installed.
 *
 * Parameters:
 *   src  (string) — a model directory, or an archive holding one
 *   root (string) — the directory to install into
 *   name (string) — the model's name, or "" to recognise it from its files
 *
 * Returns:
 *   Installed — the model now in root
 *   error — an OS error for a missing source, an existing target
 *     (fs.ErrExist), a permission refusal or a copy that failed; ErrInvalid when
 *     the files are not the model
 *
 * Example:
 *   in, err := models.Install("/media/usb/all-MiniLM-L6-v2", dir, "")
 */
func Install(src, root, name string) (Installed, error) {
	fi, err := os.Stat(src)
	if err != nil {
		return Installed{}, err
	}
	srcDir := src
	if !fi.IsDir() {
		tmp, err := os.MkdirTemp("", "kb-model-")
		if err != nil {
			return Installed{}, err
		}
		defer os.RemoveAll(tmp)
		if srcDir, err = extract(src, tmp); err != nil {
			return Installed{}, err
		}
	}
	spec, err := recognise(srcDir, name)
	if err != nil {
		return Installed{}, err
	}
	if dest := filepath.Join(root, spec.Name); exists(dest) {
		return Installed{}, fmt.Errorf("%s is already installed (remove it to install again): %w", dest, fs.ErrExist)
	}
	stage, err := stageDir(root)
	if err != nil {
		return Installed{}, err
	}
	defer os.RemoveAll(stage)
	for _, f := range spec.Files {
		if err := copyChecked(filepath.Join(srcDir, f.Name), filepath.Join(stage, f.Name), f); err != nil {
			return Installed{}, err
		}
	}
	return publish(stage, root, spec, src)
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

// recognise picks the spec the files in dir match: the named one, or the first
// known model whose files all verify. When none does, the error is the named
// model's, or else the first known model's.
func recognise(dir, name string) (Spec, error) {
	if name != "" {
		spec, ok := Find(name)
		if !ok {
			return Spec{}, fmt.Errorf("%w: kb does not know a model called %q", ErrInvalid, name)
		}
		return spec, Verify(dir, spec)
	}
	var first error
	for _, spec := range known {
		err := Verify(dir, spec)
		if err == nil {
			return spec, nil
		}
		if !errors.Is(err, ErrInvalid) {
			return Spec{}, err
		}
		if first == nil {
			first = err
		}
	}
	if first == nil {
		first = fmt.Errorf("%w: kb knows no models", ErrInvalid)
	}
	return Spec{}, first
}

// copyChecked copies src to dst and fails with ErrInvalid if what was copied is
// not the file f describes, so a file changed after the check cannot slip in.
func copyChecked(src, dst string, f File) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, h), in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return &fs.PathError{Op: "copy", Path: dst, Err: err}
	}
	if n != f.Size || hex.EncodeToString(h.Sum(nil)) != f.SHA256 {
		return fmt.Errorf("%w: %s changed while it was copied", ErrInvalid, src)
	}
	return nil
}

// extract unpacks an archive into dir and returns the directory holding the
// model: dir itself, or its single subdirectory.
func extract(archive, dir string) (string, error) {
	var err error
	switch l := strings.ToLower(archive); {
	case strings.HasSuffix(l, ".zip"):
		err = extractZip(archive, dir)
	case strings.HasSuffix(l, ".tar.gz"), strings.HasSuffix(l, ".tgz"):
		err = extractTar(archive, dir)
	default:
		return "", fmt.Errorf("%w: %s is not a directory, .zip, .tar.gz or .tgz", ErrInvalid, archive)
	}
	if err != nil {
		return "", err
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	if len(ents) == 1 && ents[0].IsDir() {
		return filepath.Join(dir, ents[0].Name()), nil
	}
	return dir, nil
}

// safeName returns the path inside dir an archive entry maps to, or false for
// one that would escape it.
func safeName(dir, name string) (string, bool) {
	clean := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if clean == "." || path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return filepath.Join(dir, filepath.FromSlash(clean)), true
}

// maxFile bounds one archive entry, so a hostile archive cannot fill the disk.
const maxFile = 1 << 30

func writeEntry(dst string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, io.LimitReader(r, maxFile+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxFile {
		err = fmt.Errorf("%w: archive entry %s is over %d bytes", ErrInvalid, dst, maxFile)
	}
	return err
}

func extractZip(archive, dir string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrInvalid, archive, err)
	}
	defer zr.Close()
	for _, zf := range zr.File {
		dst, ok := safeName(dir, zf.Name)
		if !ok {
			return fmt.Errorf("%w: archive entry %q is outside the archive", ErrInvalid, zf.Name)
		}
		if zf.FileInfo().IsDir() {
			continue
		}
		if !zf.FileInfo().Mode().IsRegular() {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		err = writeEntry(dst, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTar(archive, dir string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrInvalid, archive, err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalid, archive, err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		dst, ok := safeName(dir, h.Name)
		if !ok {
			return fmt.Errorf("%w: archive entry %q is outside the archive", ErrInvalid, h.Name)
		}
		if err := writeEntry(dst, tr); err != nil {
			return err
		}
	}
}

/** Fetch downloads the named model into root. This is the one place kb uses the
 * network, and only when asked. Each file is checked against its recorded size
 * and SHA-256 as it arrives, into a scratch directory moved into place last, so
 * a failed or corrupt download leaves nothing behind.
 *
 * Parameters:
 *   spec     (Spec)      — the model to fetch
 *   root     (string)    — the directory to install into
 *   source   (string)    — a base URL to fetch from instead of spec.Source, or ""
 *   progress (io.Writer) — receives one line per file; may be nil
 *
 * Returns:
 *   Installed — the model now in root
 *   error — fs.ErrExist for an existing target; ErrUnavailable when the source
 *     cannot be reached or refuses; fs.ErrNotExist for a 404; ErrTruncated when
 *     a transfer stops part way; ErrInvalid for a size or checksum mismatch
 *
 * Example:
 *   spec, _ := models.Find("all-MiniLM-L6-v2")
 *   in, err := models.Fetch(spec, dir, "", os.Stderr)
 */
func Fetch(spec Spec, root, source string, progress io.Writer) (Installed, error) {
	if dest := filepath.Join(root, spec.Name); exists(dest) {
		return Installed{}, fmt.Errorf("%s is already installed (remove it to install again): %w", dest, fs.ErrExist)
	}
	if source == "" {
		source = spec.Source
	}
	source = strings.TrimRight(source, "/")
	stage, err := stageDir(root)
	if err != nil {
		return Installed{}, err
	}
	defer os.RemoveAll(stage)
	client := &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment, ResponseHeaderTimeout: 30 * time.Second,
	}}
	for _, f := range spec.Files {
		if progress != nil {
			fmt.Fprintf(progress, "fetching %s (%d bytes)\n", f.Name, f.Size)
		}
		if err := download(client, source+"/"+f.Name, filepath.Join(stage, f.Name), f); err != nil {
			return Installed{}, err
		}
	}
	return publish(stage, root, spec, source)
}

func download(c *http.Client, url, dst string, f File) error {
	resp, err := c.Get(url)
	if err != nil {
		return err // a *url.Error: the caller's classification reads it as unreachable
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return &fs.PathError{Op: "fetch", Path: url, Err: fs.ErrNotExist}
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("%w: %s answered %s", ErrUnavailable, url, resp.Status)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, h), io.LimitReader(resp.Body, f.Size+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	switch {
	case err != nil:
		return fmt.Errorf("%w: %s: %v", ErrTruncated, url, err)
	case n < f.Size:
		return fmt.Errorf("%w: %s ended after %d of %d bytes", ErrTruncated, url, n, f.Size)
	case n > f.Size || hex.EncodeToString(h.Sum(nil)) != f.SHA256:
		return fmt.Errorf("%w: %s is not the expected file (checksum mismatch)", ErrInvalid, url)
	}
	return nil
}
