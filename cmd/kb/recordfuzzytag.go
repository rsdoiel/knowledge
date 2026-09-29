package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

// recordFuzzyRecord is one record's outcome in `record fuzzy-tag`'s report.
type recordFuzzyRecord struct {
	RecordID string                         `json:"record_id"`
	Status   string                         `json:"status"`
	Path     string                         `json:"path"`
	Findings []knowledge.RecordFuzzyFinding `json:"findings"`
	TagsLine string                         `json:"tags_line"`
	Action   string                         `json:"action"`
}

// addedLines returns the lines of next that raw does not have, which is the
// `tags:` line or list items AddRecordTags introduced.
func addedLines(raw, next []byte) string {
	have := map[string]bool{}
	for _, l := range strings.Split(string(raw), "\n") {
		have[strings.TrimRight(l, "\r")] = true
	}
	var added []string
	for _, l := range strings.Split(string(next), "\n") {
		if l = strings.TrimRight(l, "\r"); !have[l] {
			added = append(added, l)
		}
	}
	return strings.Join(added, "\n")
}

// recordFuzzyTag implements `record fuzzy-tag --project NAME [--concept
// NAME,...] [--write] [--dry-run]` (DR-0052). By default it only reports the
// near-miss mentions of concepts a record does not link, and the `tags:` line
// that would link them. --write adds that line to records whose status is
// proposed, editing nothing else in the file; an accepted record is history
// and is never modified, only counted. Writes are both-or-neither: every file
// is read and every edit computed before the first write. The database is not
// touched: `kb ingest` links the new tags, as it does for any edited record.
func recordFuzzyTag(kb *knowledge.KnowledgeBase, jsonOut bool, f recordFlags, out io.Writer) error {
	const usage = "usage: record fuzzy-tag --project NAME [--concept NAME,...] [--write] [--dry-run] [--root DIR]"
	if len(f.args) != 0 || f.project == "" {
		return usageErrorf("%s", usage)
	}
	p, err := kb.ProjectByName(f.project)
	if err != nil {
		return err
	}
	if p == nil {
		return notFoundf("unknown project %q", f.project)
	}
	known, err := kb.Concepts()
	if err != nil {
		return err
	}
	var explicit []string
	for _, raw := range strings.Split(f.concept, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		canon := ""
		for _, c := range known {
			if strings.EqualFold(c.Name, name) {
				canon = c.Name
				break
			}
		}
		if canon == "" {
			return notFoundf("unknown concept %q", name)
		}
		explicit = append(explicit, canon)
	}

	recs, err := kb.ListRecords(knowledge.RecordFilter{Project: f.project})
	if err != nil {
		return err
	}
	root := recordRoot(kb, f)

	type staged struct {
		path      string
		raw, next []byte
	}
	var results []recordFuzzyRecord
	var writes []staged
	skipped := map[string]int{}
	for i := range recs {
		rec := &recs[i]
		rf, raw, err := loadRecordFile(root, rec)
		if err != nil {
			return err
		}
		findings, err := kb.RecordFuzzyReport(rf, explicit)
		if err != nil {
			return err
		}
		if len(findings) == 0 {
			continue
		}
		names := make([]string, len(findings))
		for j, fi := range findings {
			names[j] = fi.Concept
		}
		next, editErr := knowledge.AddRecordTags(raw, names)
		res := recordFuzzyRecord{RecordID: rec.RecordID, Status: rec.Status, Path: rec.Path, Findings: findings, Action: "report"}
		if editErr == nil {
			res.TagsLine = addedLines(raw, next)
		}
		if f.write {
			switch {
			case rec.Status != "proposed":
				res.Action = "skipped-" + rec.Status
				skipped[rec.Status]++
			case editErr != nil:
				return classedAs(classData, fmt.Errorf("DR-%s (%s): %w", rec.RecordID, rec.Path, editErr))
			case f.dryRun:
				res.Action = "would-tag"
			default:
				res.Action = "tagged"
				path, err := resolveWithinRoot(root, rec.Path)
				if err != nil {
					return err
				}
				writes = append(writes, staged{path, raw, next})
			}
		}
		results = append(results, res)
	}

	var written []staged
	for _, w := range writes {
		if err := os.WriteFile(w.path, w.next, 0o644); err != nil {
			for _, done := range written {
				_ = os.WriteFile(done.path, done.raw, 0o644)
			}
			return ioErrorf("writing %s: %v", filepath.Base(w.path), err)
		}
		written = append(written, w)
	}

	skippedOther := 0
	for status, n := range skipped {
		if status != "accepted" {
			skippedOther += n
		}
	}
	if jsonOut {
		if results == nil {
			results = []recordFuzzyRecord{}
		}
		return printJSON(out, map[string]any{
			"dry_run": f.dryRun, "write": f.write, "records": results,
			"tagged": len(writes), "skipped_accepted": skipped["accepted"],
			"skipped_other": skippedOther,
		})
	}
	if len(results) == 0 {
		fmt.Fprintln(out, "no near-miss concept mentions found in records that do not already link them")
		return nil
	}
	for _, r := range results {
		fmt.Fprintf(out, "DR-%s [%s] %s\n", r.RecordID, r.Status, r.Path)
		for _, fi := range r.Findings {
			line := fmt.Sprintf("  %s ~ %s (%d occurrence(s))", fi.Concept, strings.Join(fi.Variants, ", "), fi.Count)
			if fi.ExactMentions > 0 {
				line += fmt.Sprintf("; exact mention, not linked: %d", fi.ExactMentions)
			}
			fmt.Fprintln(out, line)
		}
		switch r.Action {
		case "tagged":
			fmt.Fprintf(out, "  added: %s\n", r.TagsLine)
		case "would-tag":
			fmt.Fprintf(out, "  would add: %s\n", r.TagsLine)
		case "report":
			if r.TagsLine != "" {
				fmt.Fprintf(out, "  add to tags: %s\n", r.TagsLine)
			} else {
				fmt.Fprintln(out, "  add these concepts to tags: by hand (its tags: form is not one this verb edits)")
			}
		default:
			fmt.Fprintf(out, "  not changed: %s records are history, --write only edits proposed ones\n", r.Status)
		}
	}
	switch {
	case f.write && f.dryRun:
		fmt.Fprintf(out, "\nwould tag %d record(s); would skip %d accepted\n", countAction(results, "would-tag"), skipped["accepted"])
	case f.write:
		fmt.Fprintf(out, "\ntagged %d record(s); skipped %d accepted", len(writes), skipped["accepted"])
		if skippedOther > 0 {
			fmt.Fprintf(out, " and %d not proposed", skippedOther)
		}
		fmt.Fprintln(out, "\nrun kb ingest on the records directory to link the new tags")
	default:
		fmt.Fprintf(out, "\n%d record(s) with near-miss mentions; nothing was written (add --write to tag proposed records)\n", len(results))
	}
	return nil
}

func countAction(rs []recordFuzzyRecord, actions ...string) int {
	n := 0
	for _, r := range rs {
		for _, a := range actions {
			if r.Action == a {
				n++
			}
		}
	}
	return n
}
