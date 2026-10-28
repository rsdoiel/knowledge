// kb with semantic search: the normal kb command line with the Hugot embedder
// registered. The plain kb is cmd/kb in the parent module.
package main

import (
	"os"

	knowledge "github.com/rsdoiel/knowledge"
	"github.com/rsdoiel/knowledge/internal/cli"
	"github.com/rsdoiel/knowledge/semantic"
)

// register makes the Hugot embedder the one semantic search uses. The model
// directory is a placeholder until the model manager exists (plan T5).
func register(modelDir string) {
	knowledge.RegisterEmbedder(semantic.NewHugot(modelDir))
}

func main() {
	register(os.Getenv("KB_MODEL_DIR"))
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
