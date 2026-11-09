package cli

import (
	"io"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["version"] = cmdVersion
	verbs["license"] = cmdLicense
}

/** cmdVersion prints the program name, version and release hash: what
 * kb -version prints, as a verb (the go command's habit). Under --json it is
 * the object {"name", "version", "release_hash"}. It opens no database.
 *
 * Parameters:
 *   _       (*knowledge.KnowledgeBase) — not used; the verb opens no database
 *   _       (*DebugLog)                — not used
 *   jsonOut (bool)                     — whether --json was given
 *   args    ([]string)                 — the arguments after the verb; none are allowed
 *   out     (io.Writer)                — standard output
 *
 * Returns:
 *   error — a usage error for a flag or an argument, else nil
 *
 * Example:
 *   err := cmdVersion(nil, nil, false, nil, os.Stdout) // "kb 0.0.30 5132d92"
 */
func cmdVersion(_ *knowledge.KnowledgeBase, _ *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if err := noArguments("version", args); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			ReleaseHash string `json:"release_hash"`
		}{appName, knowledge.Version, knowledge.ReleaseHash})
	}
	_, err := io.WriteString(out, appName+" "+knowledge.Version+" "+knowledge.ReleaseHash+"\n")
	return err
}

/** cmdLicense prints the license text: what kb -license prints, as a verb.
 * Under --json it is the object {"license": TEXT}. It opens no database.
 *
 * Parameters:
 *   _       (*knowledge.KnowledgeBase) — not used; the verb opens no database
 *   _       (*DebugLog)                — not used
 *   jsonOut (bool)                     — whether --json was given
 *   args    ([]string)                 — the arguments after the verb; none are allowed
 *   out     (io.Writer)                — standard output
 *
 * Returns:
 *   error — a usage error for a flag or an argument, else nil
 *
 * Example:
 *   err := cmdLicense(nil, nil, false, nil, os.Stdout)
 */
func cmdLicense(_ *knowledge.KnowledgeBase, _ *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if err := noArguments("license", args); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			License string `json:"license"`
		}{knowledge.LicenseText})
	}
	_, err := io.WriteString(out, knowledge.LicenseText+"\n")
	return err
}

// noArguments is the usage error for a verb that takes neither a flag nor an
// argument.
func noArguments(verb string, args []string) error {
	positional, err := splitFlags(args, nil, nil)
	if err != nil {
		return err
	}
	if len(positional) != 0 {
		return usageErrorf("usage: %s (it takes no arguments; --json is a global option, as in kb --json %s)", verb, verb)
	}
	return nil
}
