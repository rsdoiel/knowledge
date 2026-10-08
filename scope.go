package knowledge

import (
	"os"
	"path/filepath"
	"strings"
)

/** ProjectForDir names the project a directory belongs to (DR-0058): the
 * directory sits under agents/projects/<name>/, or under a repository
 * directory <name>/ at the workspace root where agents/projects/<name>/ also
 * exists. The name must be a project the database knows; a directory for which
 * no project row exists, a directory outside the root, and the root, agents/ and
 * agents/decisions/ themselves all give "". Symbolic links are resolved first,
 * so a link into a project counts as the project.
 *
 * Parameters:
 *   root (string) — the workspace root.
 *   dir  (string) — the directory to place, normally the working directory.
 *
 * Returns:
 *   string — the project name, or "" when dir belongs to no known project.
 *   error  — on database failure.
 *
 * Example:
 *   name, err := kb.ProjectForDir("/home/me/Laboratory", "/home/me/Laboratory/harvey/cmd")
 *   // name == "harvey"
 */
func (kb *KnowledgeBase) ProjectForDir(root, dir string) (string, error) {
	rel, ok := relativeUnder(root, dir)
	if !ok || rel == "." {
		return "", nil
	}
	parts := strings.Split(rel, string(filepath.Separator))
	var name string
	switch {
	case parts[0] == "agents":
		if len(parts) < 3 || parts[1] != "projects" {
			return "", nil
		}
		name = parts[2]
	default:
		name = parts[0]
		if fi, err := os.Stat(filepath.Join(root, "agents", "projects", name)); err != nil || !fi.IsDir() {
			return "", nil
		}
	}
	p, err := kb.ProjectByName(name)
	if err != nil || p == nil {
		return "", err
	}
	return p.Name, nil
}

// relativeUnder returns dir relative to root, both with symbolic links
// resolved where they can be, and whether dir is root or beneath it.
func relativeUnder(root, dir string) (string, bool) {
	r, d := resolvedAbs(root), resolvedAbs(dir)
	rel, err := filepath.Rel(r, d)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func resolvedAbs(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}
