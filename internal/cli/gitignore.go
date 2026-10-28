package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// derivedPaths are what semantic search writes into a workspace: the downloaded
// models and the vector index. Both are large and rebuilt from the database or
// the network, so neither belongs in a git history (DR-0089 item 6).
var derivedPaths = []string{"agents/models/", "agents/knowledge.vectors.db"}

/** inGitRepo reports whether dir is in a git work tree: it or an ancestor holds
 * a .git entry (a directory, or a file for a linked work tree or a submodule). It
 * runs no program, so it needs no git installed.
 *
 * Parameters:
 *   dir (string) — a directory
 *
 * Returns:
 *   bool — true when a .git entry is found at or above dir
 *
 * Example:
 *   if inGitRepo(root) { ... }
 */
func inGitRepo(dir string) bool {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// ignoreCovers reports whether a .gitignore line already names pattern: the
// line itself, anchored with a leading slash, or (for a directory) without its
// trailing slash. Anything cleverer, such as *.db, is left to git.
func ignoreCovers(lines []string, pattern string) bool {
	bare := strings.TrimSuffix(pattern, "/")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		switch l {
		case pattern, "/" + pattern, bare, "/" + bare:
			return true
		}
	}
	return false
}

/** ensureIgnored adds the lines for models and the vector index to the
 * workspace's .gitignore when the workspace is in a git work tree, and does
 * nothing otherwise: a workspace need not be a repository, and kb makes no git
 * assumption. Lines already there, in either anchored form, are left alone, and
 * a .gitignore that needs nothing is not rewritten.
 *
 * Parameters:
 *   root (string) — the workspace root
 *
 * Returns:
 *   []string — the lines it added (empty when none)
 *   error — the .gitignore could not be read or written
 *
 * Example:
 *   added, err := ensureIgnored("/home/me/Laboratory")
 */
func ensureIgnored(root string) ([]string, error) {
	if !inGitRepo(root) {
		return nil, nil
	}
	path := filepath.Join(root, ".gitignore")
	body, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	lines := strings.Split(string(body), "\n")
	var missing []string
	for _, p := range derivedPaths {
		if !ignoreCovers(lines, p) {
			missing = append(missing, p)
		}
	}
	if len(missing) == 0 {
		return nil, nil
	}
	var b bytes.Buffer
	b.Write(body)
	if len(body) > 0 && !bytes.HasSuffix(body, []byte("\n")) {
		b.WriteByte('\n')
	}
	b.WriteString("# kb: embedding models and the vector index (large, rebuilt)\n")
	for _, m := range missing {
		b.WriteString(m + "\n")
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		return nil, err
	}
	return missing, nil
}

var trackedWarned sync.Map

/** warnIfTracked tells the user, once per path in a process, when git already
 * tracks a derived file or directory in the workspace, since ignoring a tracked
 * path does nothing. It asks git, so it says nothing when the workspace is not a
 * repository or git is not installed.
 *
 * Parameters:
 *   root (string) — the workspace root
 *   path (string) — a path under root in the form derivedPaths uses
 *
 * Example:
 *   warnIfTracked(root, "agents/knowledge.vectors.db")
 */
func warnIfTracked(root, path string) {
	if !inGitRepo(root) {
		return
	}
	if _, loaded := trackedWarned.LoadOrStore(root+"\x00"+path, true); loaded {
		return
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return
	}
	out, err := exec.Command(git, "-C", root, "ls-files", "--", path).Output()
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		return
	}
	advise("git tracks %s; adding it to .gitignore does not stop that. To stop tracking it and keep the file: git rm -r --cached %s", path, path)
}

// reportIgnored says what ensureIgnored changed, on standard output for a
// person. It is silent when nothing changed.
func reportIgnored(out interface{ Write([]byte) (int, error) }, added []string) {
	if len(added) > 0 {
		fmt.Fprintf(out, "added to .gitignore: %s\n", strings.Join(added, ", "))
	}
}
