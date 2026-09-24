package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

// stringSliceFlag implements flag.Value for a repeatable string flag
// (--set FIELD=VALUE, one occurrence per pair) -- no precedent for this in
// cmd/kb yet, so this is knowledge's first repeatable-flag type.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// The report types are the library's (library-lift-plan.md L4), aliased so
// the --json contract has exactly one definition.
type (
	frontmatterFieldReport    = knowledge.FrontmatterFieldReport
	frontmatterKeywordReport  = knowledge.FrontmatterKeywordReport
	documentFrontmatterResult = knowledge.FrontmatterResult
)

// writeFileAtomic writes data to path via a temp file in path's own
// directory and an os.Rename over the original, so a crash mid-write never
// leaves a half-written file. The splice that builds data lives in the
// library (spliceFrontmatter); the write stays here because the caller owns
// it.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".frontmatter-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

/** cmdDocumentFrontmatter implements `kb document frontmatter PATH [--accept
 * FIELD,...] [--accept-keywords NAME,...] [--set FIELD=VALUE] [--dry-run]`
 * (v0.0.11 item 2, frontmatter-generator-plan.md): propose-then-accept
 * title/author/dateCreated/dateModified/keywords for one document, deriving
 * mechanical signals from the document's own prose and git/filesystem
 * provenance -- no model call, no new dependency beyond git itself.
 *
 * title/author/dateCreated are absent-only: never overwritten once present.
 * dateModified is the one exception, always recomputed and rewritten when
 * accepted, since a stale value is worse than a missing one. keywords is a
 * diff against two candidate sets: known concepts already mentioned in the
 * text (plain write on accept) and new candidates distinctive to this
 * document (kb.AddConcept'd before being written, mirroring `kb document
 * tag --concept`'s checked-before-any-write discipline). --set FIELD=VALUE
 * bypasses signal detection and the absent-only rule entirely -- a human's
 * explicit assertion, not a proposal.
 *
 * A bare invocation (no --accept/--accept-keywords/--set) is read-only by
 * construction: nothing is selected, so nothing is written. Editing is
 * surgical, via frontmatterNode/setMappingField/writeDocumentFile -- the
 * document body is never read into anything but raw bytes either side of
 * the frontmatter block.
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — the open knowledge base.
 *   jsonOut (bool)                     — emit results as JSON.
 *   args    ([]string)                 — PATH, --accept, --accept-keywords,
 *                                        --set, --dry-run.
 *   out     (io.Writer)                — where results are written.
 *
 * Returns:
 *   error — on a usage error, an unknown field name, or a failed read/write.
 *
 * Example:
 *   err := cmdDocumentFrontmatter(kb, false, []string{"a.md", "--accept", "title"}, os.Stdout)
 */
func cmdDocumentFrontmatter(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("document frontmatter", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	acceptList := fs.String("accept", "", "comma-separated fields to accept: title,author,dateCreated,dateModified")
	acceptKeywordsList := fs.String("accept-keywords", "", "comma-separated keyword names to accept")
	var setFlags stringSliceFlag
	fs.Var(&setFlags, "set", "FIELD=VALUE, repeatable, bypasses signal detection and the absent-only rule")
	dryRun := fs.Bool("dry-run", false, "preview the write without applying it")
	if len(args) == 0 {
		return fmt.Errorf("usage: document frontmatter PATH [--accept FIELD,...] [--accept-keywords NAME,...] [--set FIELD=VALUE] [--dry-run]")
	}
	// PATH comes first, flags after -- Go's flag package stops parsing at
	// the first non-flag token, so PATH must be peeled off before Parse
	// ever sees it, not left for fs.Args() to return afterward.
	path := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("usage: document frontmatter PATH [--accept FIELD,...] [--accept-keywords NAME,...] [--set FIELD=VALUE] [--dry-run]")
	}

	setValues := map[string]string{}
	for _, kv := range setFlags {
		i := strings.Index(kv, "=")
		if i <= 0 {
			return fmt.Errorf("invalid --set value %q, want FIELD=VALUE", kv)
		}
		field, value := kv[:i], kv[i+1:]
		if !knowledge.IsFrontmatterScalarField(field) {
			return fmt.Errorf("unknown --set field %q", field)
		}
		setValues[field] = value
	}

	var acceptFields []string
	if *acceptList != "" {
		for _, f := range strings.Split(*acceptList, ",") {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			if !knowledge.IsFrontmatterScalarField(f) {
				return fmt.Errorf("unknown --accept field %q", f)
			}
			acceptFields = append(acceptFields, f)
		}
	}

	var acceptKeywordNames []string
	if *acceptKeywordsList != "" {
		for _, n := range strings.Split(*acceptKeywordsList, ",") {
			n = strings.TrimSpace(n)
			if n != "" {
				acceptKeywordNames = append(acceptKeywordNames, n)
			}
		}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	next, result, err := kb.ApplyFrontmatter(path, raw, nil, knowledge.FrontmatterAccept{
		Fields: acceptFields, Keywords: acceptKeywordNames, Set: setValues,
	}, *dryRun)
	if err != nil {
		return err
	}
	if result.Written {
		if err := writeFileAtomic(path, next); err != nil {
			return err
		}
	}

	return reportDocumentFrontmatter(jsonOut, result, out)
}

// reportDocumentFrontmatter renders cmdDocumentFrontmatter's outcome, plain
// text or --json.
func reportDocumentFrontmatter(jsonOut bool, result documentFrontmatterResult, out io.Writer) error {
	if jsonOut {
		return printJSON(out, result)
	}
	for _, f := range result.Fields {
		switch {
		case f.Current != "" && f.Proposed != "":
			// dateModified only: the one field that's always recomputed
			// even when a value already exists (design decision 4), so
			// both need to be shown -- current alone would silently hide
			// the fresh proposal a human needs to decide whether to accept.
			fmt.Fprintf(out, "%s: current: %s. proposing %q.\n", f.Field, f.Current, f.Proposed)
		case f.Current != "":
			fmt.Fprintf(out, "%s: already set: %s\n", f.Field, f.Current)
		case f.Proposed != "":
			if len(f.Signals) > 1 {
				var parts []string
				for _, s := range f.Signals {
					parts = append(parts, fmt.Sprintf("%s: %q", s.Source, s.Value))
				}
				fmt.Fprintf(out, "%s: missing. signals: %s. proposing %q.\n", f.Field, strings.Join(parts, "; "), f.Proposed)
			} else {
				fmt.Fprintf(out, "%s: missing. proposing %q.\n", f.Field, f.Proposed)
			}
		default:
			fmt.Fprintf(out, "%s: no signal available\n", f.Field)
		}
		if f.Accepted {
			fmt.Fprintf(out, "  accepted: %s\n", f.Proposed)
		}
	}
	kw := result.Keywords
	fmt.Fprintf(out, "keywords: current: %s\n", strings.Join(kw.Current, ", "))
	if len(kw.Known) > 0 {
		fmt.Fprintf(out, "  known-concept proposals: %s\n", strings.Join(kw.Known, ", "))
	}
	for _, c := range kw.New {
		fmt.Fprintf(out, "  new candidate: %s (score %.2f)\n", c.Term, c.Score)
	}
	if len(kw.Accepted) > 0 {
		fmt.Fprintf(out, "  accepted: %s\n", strings.Join(kw.Accepted, ", "))
	}
	if result.Written {
		fmt.Fprintln(out, "written.")
	}
	return nil
}
