package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

// The removal commands (DR-0050): project, observation, document and record
// delete, and unlink. Each follows `concept delete` (DR-0039): a refusal says what
// and how many and changes nothing (exit 1), --force where it makes sense, --dry-run
// previews, a target or link that is not there is exit 1, and the output says that a
// delete is local: kb merge and kb import from a database that still has the row bring
// it back.

const localDeleteNote = "kb merge or kb import from a database that still has this brings it back; delete it there as well"

/** parseDeleteArgs splits the arguments of a delete verb into its positionals and
 * its --force and --dry-run flags, in any order. A bare -- ends flag recognition, so
 * a name that begins with a dash can be given.
 *
 * Parameters:
 *   args       ([]string) — the arguments after the subverb.
 *   usage      (string)   — the usage line for an error.
 *   allowForce (bool)     — whether --force is accepted for this verb.
 *
 * Returns:
 *   []string — the positional arguments.
 *   bool     — --force.
 *   bool     — --dry-run.
 *   error    — flag.ErrHelp for a help flag, otherwise a usage error.
 *
 * Example:
 *   pos, force, dry, err := parseDeleteArgs(args, "usage: project delete NAME [--force] [--dry-run]", true)
 */
func parseDeleteArgs(args []string, usage string, allowForce bool) ([]string, bool, bool, error) {
	var force, dryRun bool
	bools := map[string]*bool{"--dry-run": &dryRun}
	if allowForce {
		bools["--force"] = &force
	}
	positional, err := splitFlags(args, nil, bools)
	if errors.Is(err, flag.ErrHelp) {
		return nil, false, false, err
	}
	if err != nil {
		return nil, false, false, usageErrorf("%v; %s", err, usage)
	}
	return positional, force, dryRun, nil
}

// parseID reads a numeric id argument, as a usage error when it is not one.
func parseID(what, arg string) (int64, error) {
	id, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, usageErrorf("invalid %s id %q", what, arg)
	}
	return id, nil
}

// refusal turns the library's *InUseError into the command's refusal: exit 1, what
// blocked it and how many, that nothing was deleted, and what to do about it.
func refusal(err error, subject, hint string) error {
	var inUse *knowledge.InUseError
	if !errors.As(err, &inUse) {
		return err
	}
	var parts []string
	for _, c := range inUse.Counts {
		if c.N > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.N, c.What))
		}
	}
	return negativef("%s still has %s; nothing was deleted (%s)", subject, strings.Join(parts, ", "), hint)
}

func plural(n int, what string) string { return fmt.Sprintf("%d %s", n, what) }

// ─── project delete ──────────────────────────────────────────────────────────

func cmdProjectDelete(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	const usage = "usage: project delete NAME [--force] [--dry-run]"
	pos, force, dryRun, err := parseDeleteArgs(args, usage, true)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usageErrorf("%s", usage)
	}
	name := pos[0]
	subject := fmt.Sprintf("project %q", name)
	hint := "use --force to remove its concept links and delete it, or --dry-run to preview"
	contentHint := "delete or move its observations, records and documents first; --force never removes content"

	u, err := logKBCall(dl, "ProjectUsage", map[string]any{"name": name}, func() (knowledge.ProjectUsage, error) {
		return kb.ProjectUsage(name)
	})
	if err != nil {
		return err
	}
	if dryRun {
		// The same rules, without the deletion: a preview must not promise what a delete would refuse.
		if u.Owned() > 0 {
			return negativef("%s would be refused: it owns %s (%s)", subject, ownedSummary(u), contentHint)
		}
		if u.Concepts > 0 && !force {
			return negativef("%s would be refused: it is linked to %s (%s)", subject, plural(u.Concepts, "concept link(s)"), hint)
		}
	} else {
		u, err = logKBCall(dl, "DeleteProject", map[string]any{"name": name, "force": force}, func() (knowledge.ProjectUsage, error) {
			return kb.DeleteProject(name, force)
		})
		if err != nil {
			var inUse *knowledge.InUseError
			if errors.As(err, &inUse) && u.Owned() > 0 {
				return negativef("%s still owns %s; nothing was deleted (%s)", subject, ownedSummary(u), contentHint)
			}
			return refusal(err, subject, hint)
		}
	}
	notes := []string{localDeleteNote}
	if jsonOut {
		return printJSON(out, struct {
			Project string `json:"project"`
			Deleted bool   `json:"deleted"`
			DryRun  bool   `json:"dry_run"`
			Removed struct {
				ConceptLinks int `json:"concept_links"`
			} `json:"removed"`
			Notes []string `json:"notes,omitempty"`
		}{Project: name, Deleted: !dryRun, DryRun: dryRun, Removed: struct {
			ConceptLinks int `json:"concept_links"`
		}{u.Concepts}, Notes: notes})
	}
	verb := "deleted"
	if dryRun {
		verb = "would delete"
	}
	line := fmt.Sprintf("%s %s", verb, subject)
	if u.Concepts > 0 {
		line += fmt.Sprintf(" and remove %s", plural(u.Concepts, "concept link(s)"))
	}
	fmt.Fprintln(out, line)
	if !dryRun {
		fmt.Fprintln(out, "note:", localDeleteNote)
	}
	return nil
}

func ownedSummary(u knowledge.ProjectUsage) string {
	var parts []string
	for _, c := range []struct {
		n    int
		what string
	}{{u.Observations, "observation(s)"}, {u.Records, "record(s)"}, {u.Documents, "document(s)"}} {
		if c.n > 0 {
			parts = append(parts, plural(c.n, c.what))
		}
	}
	return strings.Join(parts, ", ")
}

// ─── observation delete ──────────────────────────────────────────────────────

func cmdObservationDelete(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	const usage = "usage: observation delete ID [--force] [--dry-run]"
	pos, force, dryRun, err := parseDeleteArgs(args, usage, true)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usageErrorf("%s", usage)
	}
	id, err := parseID("observation", pos[0])
	if err != nil {
		return err
	}
	subject := fmt.Sprintf("observation %d", id)
	hint := "use --force to remove those links and delete it, or --dry-run to preview"

	u, err := logKBCall(dl, "ObservationUsage", map[string]any{"id": id}, func() (knowledge.ObservationUsage, error) {
		return kb.ObservationUsage(id)
	})
	if err != nil {
		return err
	}
	if dryRun {
		if u.Total() > 0 && !force {
			return negativef("%s would be refused: it has %s (%s)", subject, observationLinks(u), hint)
		}
	} else {
		u, err = logKBCall(dl, "DeleteObservation", map[string]any{"id": id, "force": force}, func() (knowledge.ObservationUsage, error) {
			return kb.DeleteObservation(id, force)
		})
		if err != nil {
			return refusal(err, subject, hint)
		}
	}
	if jsonOut {
		return printJSON(out, struct {
			Observation int64 `json:"observation"`
			Deleted     bool  `json:"deleted"`
			DryRun      bool  `json:"dry_run"`
			Removed     struct {
				Concepts      int `json:"concepts"`
				Sources       int `json:"sources"`
				RelationsFrom int `json:"relations_from"`
				RelationsTo   int `json:"relations_to"`
			} `json:"removed"`
			Notes []string `json:"notes,omitempty"`
		}{Observation: id, Deleted: !dryRun, DryRun: dryRun, Removed: struct {
			Concepts      int `json:"concepts"`
			Sources       int `json:"sources"`
			RelationsFrom int `json:"relations_from"`
			RelationsTo   int `json:"relations_to"`
		}{u.Concepts, u.Sources, u.RelationsFrom, u.RelationsTo}, Notes: []string{localDeleteNote}})
	}
	verb := "deleted"
	if dryRun {
		verb = "would delete"
	}
	line := fmt.Sprintf("%s %s", verb, subject)
	if u.Total() > 0 {
		line += fmt.Sprintf(" and remove %s", observationLinks(u))
	}
	fmt.Fprintln(out, line)
	if !dryRun {
		fmt.Fprintln(out, "note:", localDeleteNote)
	}
	return nil
}

func observationLinks(u knowledge.ObservationUsage) string {
	var parts []string
	for _, c := range []struct {
		n    int
		what string
	}{{u.Concepts, "concept link(s)"}, {u.Sources, "source link(s)"},
		{u.RelationsFrom, "supersession(s) of others"}, {u.RelationsTo, "supersession(s) by others"}} {
		if c.n > 0 {
			parts = append(parts, plural(c.n, c.what))
		}
	}
	return strings.Join(parts, ", ")
}

// ─── document delete ─────────────────────────────────────────────────────────

func cmdDocumentDelete(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	const usage = "usage: document delete ID [--force] [--dry-run]"
	pos, force, dryRun, err := parseDeleteArgs(args, usage, true)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usageErrorf("%s", usage)
	}
	id, err := parseID("document", pos[0])
	if err != nil {
		return err
	}
	subject := fmt.Sprintf("document %d", id)
	hint := "a reviewed summary is human-gated data: use --force to delete it anyway, or --dry-run to preview"

	u, err := logKBCall(dl, "DocumentUsage", map[string]any{"id": id}, func() (knowledge.DocumentUsage, error) {
		return kb.DocumentUsage(id)
	})
	if err != nil {
		return err
	}
	if dryRun {
		if u.ReviewedSections > 0 && !force {
			return negativef("%s would be refused: it has %s (%s)", subject, plural(u.ReviewedSections, "reviewed summary(ies)"), hint)
		}
	} else {
		u, err = logKBCall(dl, "DeleteDocument", map[string]any{"id": id, "force": force}, func() (knowledge.DocumentUsage, error) {
			return kb.DeleteDocument(id, force)
		})
		if err != nil {
			return refusal(err, subject, hint)
		}
	}
	note := "a re-ingest of " + u.Path + " recreates the document, without any reviewed summary"
	if jsonOut {
		return printJSON(out, struct {
			Document int64  `json:"document"`
			Path     string `json:"path"`
			Deleted  bool   `json:"deleted"`
			DryRun   bool   `json:"dry_run"`
			Removed  struct {
				Sections         int `json:"sections"`
				ReviewedSections int `json:"reviewed_sections"`
				ConceptLinks     int `json:"concept_links"`
			} `json:"removed"`
			Notes []string `json:"notes,omitempty"`
		}{Document: id, Path: u.Path, Deleted: !dryRun, DryRun: dryRun, Removed: struct {
			Sections         int `json:"sections"`
			ReviewedSections int `json:"reviewed_sections"`
			ConceptLinks     int `json:"concept_links"`
		}{u.Sections, u.ReviewedSections, u.ConceptLinks}, Notes: []string{note, localDeleteNote}})
	}
	verb := "deleted"
	if dryRun {
		verb = "would delete"
	}
	fmt.Fprintf(out, "%s %s (%s): %s, %s\n", verb, subject, u.Path, plural(u.Sections, "section(s)"),
		plural(u.ReviewedSections, "reviewed summary(ies)"))
	fmt.Fprintln(out, "note:", note)
	if !dryRun {
		fmt.Fprintln(out, "note:", localDeleteNote)
	}
	return nil
}

// ─── record delete ───────────────────────────────────────────────────────────

// recordDelete drops the database row of a record whose file is gone. A record's file
// is the source of truth and ingest is additive, so a row whose file vanished stays
// until this removes it. While the file exists it is refused: the next ingest of the
// changed file would only undo the delete, and kb never deletes a decision record from
// disk. The library does no file I/O, so the file check lives here.
func recordDelete(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer) error {
	const usage = "usage: record delete RECORD_ID [--project P | --workspace] [--root DIR] [--dry-run]"
	if len(f.args) != 1 {
		return usageErrorf("%s", usage)
	}
	rec, err := resolveRecord(kb, f.args[0], f)
	if err != nil {
		return err
	}
	root := recordRoot(kb, f)
	path, err := resolveWithinRoot(root, rec.Path)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(path); statErr == nil {
		return negativef("DR-%s's file still exists (%s); nothing was deleted: delete the file first, "+
			"or retire the record with `kb record set-status %s cancelled`", rec.RecordID, rec.Path, rec.RecordID)
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return statErr
	}
	u, err := kb.RecordUsage(rec.ID)
	if err != nil {
		return err
	}
	if !f.dryRun {
		if u, err = kb.DeleteRecord(rec.ID); err != nil {
			return err
		}
	}
	if jsonOut {
		return printJSON(out, struct {
			Record  string `json:"record"`
			Path    string `json:"path"`
			Deleted bool   `json:"deleted"`
			DryRun  bool   `json:"dry_run"`
			Removed struct {
				Concepts      int `json:"concepts"`
				RelationsFrom int `json:"relations_from"`
				RelationsTo   int `json:"relations_to"`
			} `json:"removed"`
			Notes []string `json:"notes,omitempty"`
		}{Record: rec.RecordID, Path: rec.Path, Deleted: !f.dryRun, DryRun: f.dryRun, Removed: struct {
			Concepts      int `json:"concepts"`
			RelationsFrom int `json:"relations_from"`
			RelationsTo   int `json:"relations_to"`
		}{u.Concepts, u.RelationsFrom, u.RelationsTo}, Notes: []string{localDeleteNote}})
	}
	verb := "deleted"
	if f.dryRun {
		verb = "would delete"
	}
	fmt.Fprintf(out, "%s the row for DR-%s (its file %s is gone): %s, %s\n", verb, rec.RecordID, rec.Path,
		plural(u.Concepts, "concept link(s)"), plural(u.RelationsFrom+u.RelationsTo, "relation(s)"))
	if !f.dryRun {
		fmt.Fprintln(out, "note:", localDeleteNote)
	}
	return nil
}
