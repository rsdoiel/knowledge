package knowledge

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

/** observationReadSites lists every non-test Go file that reads the
 * observations table (FROM observations / JOIN observations), with the
 * reason it does and what DR-0090 (draft-then-promote) requires of it.
 * The table is the one in design/observation-draft-then-promote-design.md.
 *
 * A new reader of observations must decide what it does with a drafted
 * row, then be added here with that decision; TestObservationReadSitesAreMapped
 * fails until it is.
 *
 * Example:
 *   reason, ok := observationReadSites["retrieval.go"]
 */
var observationReadSites = map[string]string{
	"knowledge.go":         "list, show, summary, rebuildFTS, retraction propagation: status filter; rebuildFTS indexes reviewed only",
	"conceptrecall.go":     "recall rows: reviewed only (recall feeds harvey's prompt)",
	"retrieval.go":         "recall rows: reviewed only",
	"conceptdetail.go":     "a concept's observations and counts: reviewed only",
	"scopedsearch.go":      "project-scoped search sets: reviewed only",
	"stats.go":             "counts: reviewed as now, drafts counted separately",
	"removal.go":           "usage, delete: a draft counts as content for a project delete",
	"jsonl.go":             "export (drafts as observation_draft), import (promote-on-import), link exports",
	"knowledge_merge.go":   "merge: reviewed wins, drafts keep the one present",
	"observationreview.go": "the review queue and promote: they are the only readers of drafted rows by design",
	"mergedetect.go":       "row identity for drift detection: status is part of identity",
}

var observationReadRE = regexp.MustCompile(`(?i)\b(FROM|JOIN)\s+observations\b`)

/** TestObservationReadSitesAreMapped fails when a non-test Go file reads the
 * observations table and is not in observationReadSites, or when a mapped
 * file no longer reads it. It is the guard K0 of the draft-then-promote plan
 * asks for: it keeps a later change from adding a reader that shows a
 * drafted observation without anyone deciding that it should.
 *
 * Parameters:
 *   t (*testing.T) — the test handle
 *
 * Example:
 *   go test -run TestObservationReadSitesAreMapped
 */
func TestObservationReadSitesAreMapped(t *testing.T) {
	found := map[string]bool{}
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "testdata", "tmp", "Reference", "Forks":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if observationReadRE.Match(b) {
			found[filepath.ToSlash(p)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var unmapped, stale []string
	for p := range found {
		if _, ok := observationReadSites[p]; !ok {
			unmapped = append(unmapped, p)
		}
	}
	for p := range observationReadSites {
		if !found[p] {
			stale = append(stale, p)
		}
	}
	sort.Strings(unmapped)
	sort.Strings(stale)
	if len(unmapped) > 0 {
		t.Errorf("files read the observations table with no draft-status decision (add to observationReadSites): %v", unmapped)
	}
	if len(stale) > 0 {
		t.Errorf("observationReadSites lists files that no longer read observations: %v", stale)
	}
}
