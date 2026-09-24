package knowledge

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Moved from cmd/kb/documentfrontmatter.go (library-lift-plan.md L4, DR-0035).
// The git and filesystem primitives are a verbatim move. What is new is the
// Provenance interface in front of them: this module's first os/exec use
// would otherwise be a hidden subprocess for every importer, so a caller
// (harvey, a test) can supply its own source of authorship and dates.

/** Provenance is where ProposeFrontmatter and ApplyFrontmatter learn who
 * wrote a file and when. GitProvenance is the default and shells out to git;
 * a caller that wants no subprocess, or its own idea of authorship, supplies
 * another implementation.
 *
 * Methods:
 *   FirstCommit(path)    — author and RFC 3339 date of the oldest commit
 *                          touching path; ok=false when path has no history.
 *   LastCommit(path)     — RFC 3339 date of the newest commit touching path;
 *                          ok=false when path has no history.
 *   ConfigUserName(dir)  — the configured user.name as seen from dir;
 *                          ok=false when unset.
 *   FileTime(path)       — a filesystem timestamp for path, the fallback when
 *                          there is no history.
 *
 * Example:
 *   var p knowledge.Provenance = knowledge.GitProvenance{}
 */
type Provenance interface {
	FirstCommit(path string) (author, date string, ok bool)
	LastCommit(path string) (date string, ok bool)
	ConfigUserName(dir string) (name string, ok bool)
	FileTime(path string) (time.Time, error)
}

/** GitProvenance is the default Provenance: it runs the git command line
 * from the file's own directory, and reports "no history" (ok=false) for any
 * git failure, whether that is not a repository, no commits yet, or git not
 * being installed. FileTime is the file's modification time; a true birth time is
 * not exposed portably by the standard library.
 *
 * Example:
 *   author, date, ok := knowledge.GitProvenance{}.FirstCommit("notes/a.md")
 */
type GitProvenance struct{}

/** FirstCommit implements Provenance using `git log --follow --reverse`.
 *
 * Parameters:
 *   path (string) — the file to look up.
 *
 * Returns:
 *   string — the oldest commit's author name.
 *   string — that commit's author date, RFC 3339.
 *   bool   — false when git reports no history for path.
 *
 * Example:
 *   author, date, ok := knowledge.GitProvenance{}.FirstCommit("a.md")
 */
func (GitProvenance) FirstCommit(path string) (string, string, bool) { return gitFirstCommit(path) }

/** LastCommit implements Provenance using `git log -1`.
 *
 * Parameters:
 *   path (string) — the file to look up.
 *
 * Returns:
 *   string — the newest commit's author date, RFC 3339.
 *   bool   — false when git reports no history for path.
 *
 * Example:
 *   date, ok := knowledge.GitProvenance{}.LastCommit("a.md")
 */
func (GitProvenance) LastCommit(path string) (string, bool) { return gitLastCommit(path) }

/** ConfigUserName implements Provenance using `git config user.name` run in
 * dir, so a repository-local override is honored.
 *
 * Parameters:
 *   dir (string) — the directory whose git configuration to query.
 *
 * Returns:
 *   string — the configured name.
 *   bool   — false when none is set.
 *
 * Example:
 *   name, ok := knowledge.GitProvenance{}.ConfigUserName(".")
 */
func (GitProvenance) ConfigUserName(dir string) (string, bool) { return gitConfigUserName(dir) }

/** FileTime implements Provenance with the file's modification time.
 *
 * Parameters:
 *   path (string) — the file to stat.
 *
 * Returns:
 *   time.Time — the modification time.
 *   error     — when the file cannot be stat'ed.
 *
 * Example:
 *   t, err := knowledge.GitProvenance{}.FileTime("a.md")
 */
func (GitProvenance) FileTime(path string) (time.Time, error) { return fsBirthOrModTime(path) }

// runGit runs git with args, cwd at dir, returning trimmed stdout. Any
// exec.Command failure (non-repo, no commits yet, git not on PATH) is
// reported as an error -- callers treat that uniformly with "found nothing"
// as provenance-source failure (design decision 3: detected from the
// command's own failure, not a .git pre-check).
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// splitNonEmptyLines splits s on newlines, dropping empty lines -- git log
// output with -z-free formatting.
func splitNonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// gitFirstCommit returns the author and date of the oldest commit touching
// path, or ok=false if path isn't in any git history (not a repo, or
// genuinely untracked). Tries --diff-filter=A first (isolates the add
// commit); if that yields nothing -- e.g. a moved/renamed file, where the
// add may not appear under the current name -- falls back to the oldest
// --follow entry with no filter.
func gitFirstCommit(path string) (author, date string, ok bool) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	line, found := firstGitLogLine(dir, base, true)
	if !found {
		line, found = firstGitLogLine(dir, base, false)
	}
	if !found {
		return "", "", false
	}
	parts := strings.SplitN(line, "\x1f", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// firstGitLogLine runs git log --reverse (oldest first) for base, optionally
// with --diff-filter=A, and returns its first output line.
func firstGitLogLine(dir, base string, addOnly bool) (string, bool) {
	args := []string{"log", "--follow", "--reverse", "--format=%an\x1f%aI"}
	if addOnly {
		args = append(args, "--diff-filter=A")
	}
	args = append(args, "--", base)
	out, err := runGit(dir, args...)
	if err != nil {
		return "", false
	}
	lines := splitNonEmptyLines(out)
	if len(lines) == 0 {
		return "", false
	}
	return lines[0], true
}

// gitLastCommit returns the date of the most recent commit touching path, or
// ok=false if path isn't in any git history.
func gitLastCommit(path string) (date string, ok bool) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	out, err := runGit(dir, "log", "-1", "--format=%aI", "--", base)
	if err != nil || out == "" {
		return "", false
	}
	return out, true
}

// gitConfigUserName returns git's configured user.name, run with cwd at dir
// (so a repo-local override is honored, not just the global config). Not
// the plan's literal zero-argument signature: it structurally needs to know
// which directory's config to query.
func gitConfigUserName(dir string) (string, bool) {
	out, err := runGit(dir, "config", "user.name")
	if err != nil || out == "" {
		return "", false
	}
	return out, true
}

// fsBirthOrModTime returns path's modification time. Real OS-level birth
// time (creation time) isn't exposed portably by Go's standard library
// without per-platform build tags (darwin/windows expose it via extended
// stat fields, Linux only via statx, not through os.FileInfo at all) --
// deliberately not implemented here, mtime is used unconditionally. Worth
// revisiting only if this proves too imprecise in practice.
func fsBirthOrModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
