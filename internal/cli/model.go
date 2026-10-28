package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
	"github.com/rsdoiel/knowledge/internal/models"
)

func init() {
	verbs["model"] = cmdModel
}

// modelUsage is the usage line printed for a wrong model command line.
const modelUsage = "usage: model <install|fetch|list> [PATH|NAME] [-workspace] [-dir DIR]"

/** cmdModel implements the model verb: it manages the embedding model files
 * semantic search needs (DR-0089 item 6). It reads and writes files only, never
 * the knowledge base, so it works in the plain build as in the semantic build
 * and is dispatched with a nil kb like init and completion.
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — unused; model is dispatched with nil.
 *   dl      (*DebugLog)                — debug log, may be nil.
 *   jsonOut (bool)                     — emit results as JSON.
 *   args    ([]string)                 — the subverb and its arguments.
 *   out     (io.Writer)                — where results are written.
 *
 * Returns:
 *   error — a usage error for a bad command line; otherwise the class of what
 *           went wrong: 65 for files that are not the model, 66 for a missing
 *           path or workspace, 69 for an unreachable source, 73 for an existing
 *           target, 74 for a transfer that stopped part way, 77 for a refusal.
 *
 * Example:
 *   err := cmdModel(nil, nil, false, []string{"install", "/media/usb/all-MiniLM-L6-v2"}, os.Stdout)
 */
func cmdModel(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return usageErrorf("%s", modelUsage)
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "install":
		return cmdModelInstall(dl, jsonOut, rest, out)
	case "fetch":
		return cmdModelFetch(dl, jsonOut, rest, out)
	case "list":
		return cmdModelList(dl, jsonOut, rest, out)
	}
	return usageErrorf("unknown model subcommand %q; %s", sub, modelUsage)
}

// modelFlags are the options every model subverb takes.
type modelFlags struct {
	workspace bool
	dir       string
	name      string
	source    string
}

// parseModelFlags splits args into positionals and the flags the subverb
// accepts, in either dash form and in any position.
func parseModelFlags(args []string, name, source bool) (modelFlags, []string, error) {
	var f modelFlags
	str := map[string]*string{
		"-dir": &f.dir, "--dir": &f.dir,
	}
	if name {
		for k, v := range map[string]*string{"-name": &f.name, "--name": &f.name} {
			str[k] = v
		}
	}
	if source {
		for k, v := range map[string]*string{"-source": &f.source, "--source": &f.source} {
			str[k] = v
		}
	}
	pos, err := splitFlags(args, str, map[string]*bool{"-workspace": &f.workspace, "--workspace": &f.workspace})
	return f, pos, err
}

// modelInstallRoot works out the directory an install or fetch writes to.
func modelInstallRoot(f modelFlags) (models.Root, error) {
	ws := ""
	if f.workspace {
		root, err := modelWorkspace()
		if err != nil {
			return models.Root{}, err
		}
		ws = root
	}
	root, err := models.InstallRoot(f.dir, ws)
	if err != nil {
		return models.Root{}, noInputf("cannot work out the user data directory: %v", err)
	}
	return root, nil
}

// modelWorkspace returns the workspace root above the working directory.
func modelWorkspace() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, _, ok := knowledge.FindWorkspace(cwd)
	if !ok {
		return "", noInputf("-workspace: no workspace found at or above %s; run kb init, or drop -workspace to use the user directory", cwd)
	}
	return root, nil
}

// modelErr gives the errors the models package defines their exit classes. The
// operating system's errors classify themselves.
func modelErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, models.ErrInvalid):
		return classedAs(classData, err)
	case errors.Is(err, models.ErrUnavailable):
		return classedAs(classUnavailable, err)
	case errors.Is(err, models.ErrTruncated):
		return classedAs(classIO, err)
	}
	return err
}

// modelResult prints what install and fetch did.
func modelResult(out io.Writer, jsonOut bool, verb string, in models.Installed, root models.Root) error {
	_, canUse := knowledge.RegisteredEmbedder()
	if jsonOut {
		return printJSON(out, map[string]any{
			"verb": verb, "name": in.Name, "dir": in.Dir, "scope": root.Scope,
			"licence": in.Licence, "usable_in_this_build": canUse,
		})
	}
	fmt.Fprintf(out, "%s: %s installed in %s (%s, licence %s)\n", verb, in.Name, in.Dir, root.Scope, in.Licence)
	if !canUse {
		advise("this build of kb has no semantic search, so it cannot use the model; the files are in place for the semantic build")
	}
	return nil
}

func cmdModelInstall(dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	f, pos, err := parseModelFlags(args, true, false)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return usageErrorf("usage: model install PATH [-name NAME] [-workspace] [-dir DIR]; PATH is a model directory or archive (.zip, .tar.gz); to download the model instead, run: kb model fetch")
	}
	if len(pos) > 1 {
		return usageErrorf("usage: model install PATH [-name NAME] [-workspace] [-dir DIR]")
	}
	if err := knownModelName(f.name); err != nil {
		return err
	}
	root, err := modelInstallRoot(f)
	if err != nil {
		return err
	}
	dl.Log("model install", map[string]any{"src": pos[0], "root": root.Path})
	in, err := models.Install(pos[0], root.Path, f.name)
	if err != nil {
		return modelErr(err)
	}
	return modelResult(out, jsonOut, "install", in, root)
}

func cmdModelFetch(dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	f, pos, err := parseModelFlags(args, false, true)
	if err != nil {
		return err
	}
	if len(pos) > 1 {
		return usageErrorf("usage: model fetch [NAME] [-source URL] [-workspace] [-dir DIR]")
	}
	name := models.Known()[0].Name
	if len(pos) == 1 {
		name = pos[0]
	}
	if err := knownModelName(name); err != nil {
		return err
	}
	spec, _ := models.Find(name)
	root, err := modelInstallRoot(f)
	if err != nil {
		return err
	}
	dl.Log("model fetch", map[string]any{"name": name, "root": root.Path, "source": f.source})
	var progress io.Writer
	if !jsonOut {
		progress = adviceOut
	}
	in, err := models.Fetch(spec, root.Path, f.source, progress)
	if err != nil {
		return modelErr(err)
	}
	return modelResult(out, jsonOut, "fetch", in, root)
}

// knownModelName is a usage error for a model name kb does not know; "" is fine.
func knownModelName(name string) error {
	if name == "" {
		return nil
	}
	if _, ok := models.Find(name); ok {
		return nil
	}
	var names []string
	for _, s := range models.Known() {
		names = append(names, s.Name)
	}
	return usageErrorf("unknown model %q; kb knows: %s", name, strings.Join(names, ", "))
}

func cmdModelList(dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	f, pos, err := parseModelFlags(args, false, false)
	if err != nil {
		return err
	}
	if len(pos) != 0 {
		return usageErrorf("usage: model list [-workspace] [-dir DIR]")
	}
	ws := ""
	if root, err := modelWorkspace(); err == nil {
		ws = root
	} else if f.workspace {
		return err
	}
	roots := models.Roots(f.dir, ws)
	found, err := models.List(roots)
	if err != nil {
		return err
	}
	emb, canUse := knowledge.RegisteredEmbedder()
	var idx *knowledge.VectorsMeta
	if ws != "" {
		if v, err := knowledge.OpenVectors(knowledge.DefaultPath(ws)); err == nil {
			m := v.Meta()
			idx = &m
			v.Close()
		}
	}
	if jsonOut {
		return printJSON(out, map[string]any{
			"semantic_build": canUse, "models": nonNil(found), "index": idx,
		})
	}
	if canUse {
		fmt.Fprintf(out, "This build of kb has semantic search (embedder %s).\n", emb.Model())
	} else {
		fmt.Fprintln(out, "This build of kb has no semantic search; it can install models but not use them.")
	}
	if len(found) == 0 {
		fmt.Fprintln(out, "No models installed. Run: kb model fetch   (or: kb model install PATH)")
	}
	for _, m := range found {
		state := "ok"
		switch {
		case !m.Known:
			state = "unknown to kb"
		case !m.Complete:
			state = "incomplete: " + strings.Join(m.Missing, ", ")
		}
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", m.Name, m.Scope, state, m.Dir)
	}
	if idx != nil {
		fmt.Fprintf(out, "The vector index was built with %s (%d dimensions).\n", idx.Model, idx.Dimensions)
	} else {
		fmt.Fprintln(out, "No vector index yet.")
	}
	return nil
}

// nonNil keeps an empty list a JSON [] rather than null.
func nonNil(m []models.Installed) []models.Installed {
	if m == nil {
		return []models.Installed{}
	}
	return m
}
