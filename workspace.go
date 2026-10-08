package knowledge

import (
	"os"
	"path/filepath"
)

/** CeilingEnv names the environment variable listing directories FindWorkspace
 * must not examine, nor anything above them (the PathListSeparator separates
 * entries; blank entries are ignored), as git's GIT_CEILING_DIRECTORIES does.
 * It keeps a scratch directory, or a test, from walking up into a real
 * workspace.
 */
const CeilingEnv = "KB_CEILING_DIRECTORIES"

/** MarkerDB and MarkerJSONL name the two files whose presence under agents/
 * marks a directory as a workspace root. The database is the working copy;
 * the JSONL file is the tracked export a fresh clone has before its database
 * is rebuilt from it (the database is gitignored).
 */
const (
	MarkerDB    = "knowledge.db"
	MarkerJSONL = "knowledge.jsonl"
)

/** FindWorkspace walks up from dir to the nearest ancestor (dir itself
 * included) whose agents/ directory holds a regular file named knowledge.db
 * or knowledge.jsonl, the way git finds .git. Where one directory holds both,
 * the database is reported. An agents/ directory without either file, or a
 * marker that is a directory, does not make a workspace. A directory listed in
 * CeilingEnv ends the walk before it is examined, so a workspace at or above a
 * ceiling is never found.
 *
 * Parameters:
 *   dir (string) — where to start; a relative path is made absolute first.
 *
 * Returns:
 *   root   (string) — the workspace root, or "" when none was found.
 *   marker (string) — MarkerDB or MarkerJSONL, or "" when none was found.
 *   ok     (bool)   — whether a workspace was found.
 *
 * Example:
 *   root, marker, ok := FindWorkspace("/home/me/Laboratory/harvey/cmd")
 *   // root == "/home/me/Laboratory", marker == MarkerDB, ok == true
 */
func FindWorkspace(dir string) (root, marker string, ok bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", false
	}
	ceilings := ceilingSet()
	for cur := abs; ; {
		if ceilings[cur] {
			return "", "", false
		}
		for _, m := range []string{MarkerDB, MarkerJSONL} {
			if fi, err := os.Stat(filepath.Join(cur, "agents", m)); err == nil && fi.Mode().IsRegular() {
				return cur, m, true
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", "", false
		}
		cur = parent
	}
}

// ceilingSet reads CeilingEnv into a set of absolute directories, each both as
// written and with symlinks resolved, so a ceiling matches however the walk
// reached it.
func ceilingSet() map[string]bool {
	set := map[string]bool{}
	for _, entry := range filepath.SplitList(os.Getenv(CeilingEnv)) {
		if entry == "" {
			continue
		}
		abs, err := filepath.Abs(entry)
		if err != nil {
			continue
		}
		set[abs] = true
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			set[resolved] = true
		}
	}
	return set
}
