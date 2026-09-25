package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["project"] = cmdProject
}

func cmdProject(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return usageErrorf("usage: project <add|list|show|concepts|set-status|set-description|rename|delete> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return cmdProjectAdd(kb, dl, jsonOut, rest, out)
	case "list":
		return cmdProjectList(kb, dl, jsonOut, rest, out)
	case "show":
		return cmdProjectShow(kb, dl, jsonOut, rest, out)
	case "concepts":
		return cmdProjectConcepts(kb, dl, jsonOut, rest, out)
	case "set-status":
		return cmdProjectSetStatus(kb, dl, jsonOut, rest, out)
	case "set-description":
		return cmdProjectSetDescription(kb, dl, jsonOut, rest, out)
	case "rename":
		return cmdProjectRename(kb, dl, jsonOut, rest, out)
	case "delete":
		return cmdProjectDelete(kb, dl, jsonOut, rest, out)
	default:
		return usageErrorf("unknown project subcommand %q", sub)
	}
}

func cmdProjectAdd(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("project add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	status := fs.String("status", "", "concept, active, paused, or concluded (default: active)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return usageErrorf("usage: project add [--status concept|active|paused|concluded] NAME [DESCRIPTION]")
	}
	name, err := knowledge.CleanName("project", rest[0])
	if err != nil {
		return err
	}
	desc := strings.Join(rest[1:], " ")
	var id int64
	if *status != "" {
		id, err = logKBCall(dl, "AddProjectWithStatus", map[string]any{"name": name, "description": desc, "status": *status}, func() (int64, error) {
			return kb.AddProjectWithStatus(name, desc, *status)
		})
	} else {
		id, err = logKBCall(dl, "AddProject", map[string]any{"name": name, "description": desc}, func() (int64, error) {
			return kb.AddProject(name, desc)
		})
	}
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}{ID: id, Name: name})
	}
	fmt.Fprintf(out, "project %q added (id=%d)\n", name, id)
	return nil
}

func cmdProjectSetStatus(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	args, argErr := plainArgs(args, 2, 2, 2, "usage: project set-status NAME STATUS")
	if argErr != nil {
		return argErr
	}
	name, status := args[0], args[1]
	err := logKBCallErr(dl, "SetProjectStatus", map[string]any{"name": name, "status": status}, func() error {
		return kb.SetProjectStatus(name, status)
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}{Name: name, Status: status})
	}
	fmt.Fprintf(out, "project %q status set to %q\n", name, status)
	return nil
}

// Trailing words are joined the way project add joins them, so an unquoted
// description works the same in both places. A bare NAME is a usage error
// rather than a clear: clearing takes an explicit empty string.
func cmdProjectSetDescription(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	args, argErr := plainArgs(args, 2, -1, 1, "usage: project set-description NAME DESCRIPTION")
	if argErr != nil {
		return argErr
	}
	name := args[0]
	desc := strings.Join(args[1:], " ")
	err := logKBCallErr(dl, "SetProjectDescription", map[string]any{"name": name, "description": desc}, func() error {
		return kb.SetProjectDescription(name, desc)
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}{Name: name, Description: desc})
	}
	fmt.Fprintf(out, "project %q description updated\n", name)
	return nil
}

// cmdProjectRename implements `project rename [--root PATH] [--dry-run] OLD
// NEW`. OLD and NEW are positional, not joined the way set-description's
// trailing words are, since a project name is a single token, not free text.
//
// A project with no records renames via the library's own RenameProject
// (DR-0024). A project that owns records goes through DR-0026's corpus
// rewrite instead of DR-0024's outright refusal: every owned record's file
// gets its project: frontmatter rewritten to NEW, written both-or-neither —
// any write failure rolls back every file already written — and only once
// every file is confirmed on disk does the project row itself rename, via
// RenameProjectRow. No record's database row is touched in the process; the
// checksum mismatch that rewrite leaves behind is exactly what the next
// ordinary `kb ingest` of that corpus is for.
func cmdProjectRename(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("project rename", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", "", "workspace root record paths are relative to (default: inferred from the database path)")
	dryRun := fs.Bool("dry-run", false, "report what a rename would rewrite without writing anything")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 2 {
		return usageErrorf("usage: project rename [--root PATH] [--dry-run] OLD NEW")
	}
	old := rest[0]
	// Cleaned before anything is staged: this verb rewrites every owned
	// record's project: frontmatter before the library sees NEW, so a padded
	// name would reach the files but not the database row (DR-0026).
	new, err := knowledge.CleanName("project", rest[1])
	if err != nil {
		return err
	}

	p, err := kb.ProjectByName(old)
	if err != nil {
		return err
	}
	if p == nil {
		return notFoundf("knowledge: project %q not found", old)
	}
	existing, err := kb.ProjectByName(new)
	if err != nil {
		return err
	}
	if existing != nil {
		return negativef("knowledge: project %q already exists", new)
	}

	records, err := kb.RecordsByProject(p.ID)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		if *dryRun {
			fmt.Fprintf(out, "would rename project %q to %q (owns no records; no corpus rewrite needed)\n", old, new)
			return nil
		}
		err := logKBCallErr(dl, "RenameProject", map[string]any{"old": old, "new": new}, func() error {
			return kb.RenameProject(old, new)
		})
		if err != nil {
			return err
		}
		if jsonOut {
			return printJSON(out, struct {
				Old string `json:"old"`
				New string `json:"new"`
			}{Old: old, New: new})
		}
		fmt.Fprintf(out, "project %q renamed to %q\n", old, new)
		return nil
	}

	rootDir := recordRoot(kb, recordFlags{root: *root})
	type stagedFile struct {
		path     string
		raw      []byte
		rendered []byte
	}
	files := make([]stagedFile, 0, len(records))
	dirs := map[string]bool{}
	for i := range records {
		rec := records[i]
		rf, raw, err := loadRecordFile(rootDir, &rec)
		if err != nil {
			return err
		}
		rf.ProjectName = new
		rendered, err := knowledge.RenderRecordFile(rf)
		if err != nil {
			return fmt.Errorf("rendering DR-%s: %w", rec.RecordID, err)
		}
		path, err := resolveWithinRoot(rootDir, rec.Path)
		if err != nil {
			return err
		}
		files = append(files, stagedFile{path: path, raw: raw, rendered: rendered})
		dirs[filepath.Dir(path)] = true
	}

	if *dryRun {
		paths := make([]string, len(files))
		for i, f := range files {
			paths[i] = f.path
		}
		if jsonOut {
			return printJSON(out, map[string]any{
				"old": old, "new": new, "dry_run": true,
				"records": len(files), "paths": paths,
			})
		}
		fmt.Fprintf(out, "would rewrite %d record(s) and rename project %q to %q:\n", len(files), old, new)
		for _, path := range paths {
			fmt.Fprintf(out, "  %s\n", path)
		}
		return nil
	}

	written := make([]stagedFile, 0, len(files))
	rollback := func() {
		for _, f := range written {
			_ = os.WriteFile(f.path, f.raw, 0o644)
		}
	}
	for _, f := range files {
		if err := os.WriteFile(f.path, f.rendered, 0o644); err != nil {
			rollback()
			return fmt.Errorf("writing %s: %w", f.path, err)
		}
		written = append(written, f)
	}

	if err := logKBCallErr(dl, "RenameProjectRow", map[string]any{"old": old, "new": new}, func() error {
		return kb.RenameProjectRow(old, new)
	}); err != nil {
		rollback()
		return err
	}

	var notes []string
	// Sorted, so the notes come out in the same order on every run (DR-0037).
	sortedDirs := make([]string, 0, len(dirs))
	for dir := range dirs {
		sortedDirs = append(sortedDirs, dir)
	}
	sort.Strings(sortedDirs)
	for _, dir := range sortedDirs {
		if regenErr := regenerateIndexIfPresent(dir); regenErr != nil {
			notes = append(notes, fmt.Sprintf("index.md could not be refreshed in %s: %v", dir, regenErr))
		}
	}

	if jsonOut {
		result := map[string]any{"old": old, "new": new, "records_rewritten": len(files)}
		if len(notes) > 0 {
			result["notes"] = notes
		}
		return printJSON(out, result)
	}
	fmt.Fprintf(out, "project %q renamed to %q (%d record(s) rewritten)\n", old, new, len(files))
	for _, n := range notes {
		fmt.Fprintln(out, n)
	}
	return nil
}

func cmdProjectList(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	args, argErr := plainArgs(args, 0, 0, 0, "usage: project list")
	if argErr != nil {
		return argErr
	}
	projects, err := logKBCall(dl, "Projects", nil, kb.Projects)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, projects)
	}
	if len(projects) == 0 {
		fmt.Fprintln(out, "(no projects)")
		return nil
	}
	for _, p := range projects {
		fmt.Fprintf(out, "%-4d  %-24s  %-10s  %s\n", p.ID, p.Name, p.Status, p.Description)
	}
	return nil
}

func cmdProjectShow(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	args, argErr := plainArgs(args, 1, 1, 1, "usage: project show NAME")
	if argErr != nil {
		return argErr
	}
	name := args[0]
	p, err := logKBCall(dl, "ProjectByName", map[string]any{"name": name}, func() (*knowledge.Project, error) {
		return kb.ProjectByName(name)
	})
	if err != nil {
		return err
	}
	if p == nil {
		return notFoundf("project %q not found", name)
	}
	if jsonOut {
		return printJSON(out, p)
	}
	fmt.Fprintf(out, "%d  %s  [%s]\n", p.ID, p.Name, p.Status)
	if p.Description != "" {
		fmt.Fprintf(out, "  %s\n", p.Description)
	}
	return nil
}

func cmdProjectConcepts(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	args, argErr := plainArgs(args, 1, 1, 1, "usage: project concepts NAME")
	if argErr != nil {
		return argErr
	}
	name := args[0]
	p, err := logKBCall(dl, "ProjectByName", map[string]any{"name": name}, func() (*knowledge.Project, error) {
		return kb.ProjectByName(name)
	})
	if err != nil {
		return err
	}
	if p == nil {
		return notFoundf("project %q not found", name)
	}
	concepts, err := logKBCall(dl, "ProjectConcepts", map[string]any{"project_id": p.ID}, func() ([]knowledge.Concept, error) {
		return kb.ProjectConcepts(p.ID)
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, concepts)
	}
	if len(concepts) == 0 {
		fmt.Fprintln(out, "(no concepts)")
		return nil
	}
	for _, c := range concepts {
		fmt.Fprintf(out, "%-4d  %s\n", c.ID, c.Name)
	}
	return nil
}
