// kb with semantic search: the normal kb command line with the Hugot embedder
// registered. The plain kb is cmd/kb in the parent module.
package main

import (
	"os"
	"path/filepath"

	knowledge "github.com/rsdoiel/knowledge"
	"github.com/rsdoiel/knowledge/internal/cli"
	"github.com/rsdoiel/knowledge/internal/models"
	"github.com/rsdoiel/knowledge/semantic"
)

// register makes the Hugot embedder the one semantic search uses. The model is
// the default one (kb model fetch) found by the model manager: the workspace's
// agents/models first, then the user directory, or KB_MODEL_DIR. When it is not
// installed the embedder points at where it would be, so the error that follows
// (plan T7) names the place.
func register(workspace string) {
	name := models.Known()[0].Name
	roots := models.Roots("", workspace)
	dir := ""
	if m, ok := models.Lookup(name, roots); ok {
		dir = m.Dir
	} else if len(roots) > 0 {
		dir = filepath.Join(roots[0].Path, name)
	} else {
		dir = name
	}
	knowledge.RegisterEmbedder(semantic.NewHugot(dir))
}

func main() {
	workspace := ""
	if cwd, err := os.Getwd(); err == nil {
		if root, _, ok := knowledge.FindWorkspace(cwd); ok {
			workspace = root
		}
	}
	register(workspace)
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
