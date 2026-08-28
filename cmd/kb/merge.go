package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"

	_ "github.com/glebarez/go-sqlite"
)

func init() {
	verbs["merge"] = cmdMerge
}

// cmdMerge implements the merge verb. Unlike every other verb, it ignores
// kb entirely -- it operates on the two explicit -a/-b source paths and
// writes a fresh -out file, never touching the ambient --db database (see
// the call site in main.go, which skips opening that database for this
// verb specifically).
func cmdMerge(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("merge", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	aPath := fs.String("a", "", "path to the first knowledge.db")
	bPath := fs.String("b", "", "path to the second knowledge.db")
	outPath := fs.String("out", "", "path for the merged knowledge.db (must not exist)")
	force := fs.Bool("force", false, "resolve name/uuid collisions by reconciling b's identity to a's, instead of aborting")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *aPath == "" || *bPath == "" || *outPath == "" {
		return fmt.Errorf("usage: merge -a PATH -b PATH -out PATH [-force]")
	}

	// Progress text (collision-reconciliation notice, per-table summary)
	// is human-readable narration, not the structured result -- suppress
	// it in JSON mode so stdout stays cleanly parseable; the JSON object
	// printed below carries the same information structurally instead.
	progressOut := out
	if jsonOut {
		progressOut = io.Discard
	}
	summary, reconciled, divergences, err := runMerge(dl, *aPath, *bPath, *outPath, *force, progressOut)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			CollisionsReconciled int                           `json:"collisions_reconciled"`
			ContentDivergences   []knowledge.ContentDivergence `json:"content_divergences"`
			Tables               []knowledge.MergeTableSummary `json:"tables"`
		}{CollisionsReconciled: reconciled, ContentDivergences: divergences, Tables: summary})
	}
	return nil
}

// formatDivergences renders the content-divergence report. It is written to
// out on a successful run and folded into the error message on an aborted
// one: a divergence does not block a merge, so when something else does, the
// divergence is still true and still the operator's to reconcile.
func formatDivergences(w io.Writer, divergences []knowledge.ContentDivergence) {
	if len(divergences) == 0 {
		return
	}
	fmt.Fprintf(w, "%d content divergence(s) found — same record, different text; a's copy is kept:\n",
		len(divergences))
	for _, d := range divergences {
		fmt.Fprintf(w, "  %-10s %-30s %s vs %s\n", d.Table, d.Label, d.ChecksumA, d.ChecksumB)
	}
	fmt.Fprintln(w, "reconcile the differing decision record files yourself; the merge does not choose between them.")
}

// runMerge does the actual merge work. On a collision abort (no -force),
// the collision detail is embedded in the returned error's message rather
// than written to out -- out is reserved for a successful run's progress
// narration, and an error's message is what survives into both the
// text-mode "kb: <message>" line and the JSON-mode {"error": "<message>"}
// envelope (see printError), so this is the one place collision detail
// can go that works correctly in both output modes.
func runMerge(dl *DebugLog, aPath, bPath, outPath string, force bool, out io.Writer) (summary []knowledge.MergeTableSummary, reconciled int, divergences []knowledge.ContentDivergence, err error) {
	scratch, err := os.MkdirTemp("", "kbmerge-")
	if err != nil {
		return nil, 0, nil, err
	}
	defer os.RemoveAll(scratch)

	scratchA := filepath.Join(scratch, "a.db")
	scratchB := filepath.Join(scratch, "b.db")
	if err := checkpointAndCopy(aPath, scratchA); err != nil {
		return nil, 0, nil, fmt.Errorf("checkpoint+copy %s: %w", aPath, err)
	}
	if err := checkpointAndCopy(bPath, scratchB); err != nil {
		return nil, 0, nil, fmt.Errorf("checkpoint+copy %s: %w", bPath, err)
	}

	// Bring both copies up to the current schema before anything ATTACHes
	// them, so a machine whose database predates a table can still be merged.
	// The workspace name comes from the original path, not the copy's -- the
	// copy is under a temp directory and deriving from it would relabel every
	// record that predates the workspace column (DR-0014).
	if err := knowledge.NormalizeForMerge(scratchA, aPath); err != nil {
		return nil, 0, nil, err
	}
	if err := knowledge.NormalizeForMerge(scratchB, bPath); err != nil {
		return nil, 0, nil, err
	}

	collisions, err := knowledge.CollisionReport(scratchA, scratchB)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("collision report: %w", err)
	}
	// Divergences are read before any reconciliation, but the order does not
	// matter: reconciling rewrites uuids, and a divergence is about identity
	// and checksum, neither of which it touches.
	divergences, err = knowledge.DivergenceReport(scratchA, scratchB)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("divergence report: %w", err)
	}
	if len(divergences) > 0 {
		dl.Log("merge_divergence", map[string]any{"a": aPath, "b": bPath, "count": len(divergences)})
	}
	if len(collisions) > 0 {
		dl.Log("merge_collision", map[string]any{"a": aPath, "b": bPath, "count": len(collisions), "force": force})
		if !force {
			var msg strings.Builder
			fmt.Fprintf(&msg, "%d name/uuid collision(s) found:\n", len(collisions))
			for _, c := range collisions {
				fmt.Fprintf(&msg, "  %-10s %-30s %s vs %s\n", c.Table, c.Label, c.UUIDA, c.UUIDB)
			}
			formatDivergences(&msg, divergences)
			msg.WriteString("aborting: resolve collisions or pass -force to reconcile b's identity to a's for each one")
			return nil, 0, divergences, errors.New(msg.String())
		}
		fmt.Fprintf(out, "%d name/uuid collision(s) found:\n", len(collisions))
		for _, c := range collisions {
			fmt.Fprintf(out, "  %-10s %-30s %s vs %s\n", c.Table, c.Label, c.UUIDA, c.UUIDB)
		}
		if err := knowledge.ReconcileCollisions(scratchB, collisions); err != nil {
			return nil, 0, divergences, fmt.Errorf("reconcile collisions: %w", err)
		}
		reconciled = len(collisions)
		dl.Log("merge_reconciled", map[string]any{"count": reconciled})
		fmt.Fprintf(out, "-force set: reconciled %d collision(s) — b's rows now share a's identity, so their observations/links merge normally instead of being dropped\n", reconciled)
	}
	// A divergence never blocks, so it is reported on the way through rather
	// than gating anything.
	formatDivergences(out, divergences)

	summary, err = knowledge.MergeKnowledgeBases(scratchA, scratchB, outPath)
	if err != nil {
		return nil, reconciled, divergences, fmt.Errorf("merge: %w", err)
	}
	dl.Log("merge_summary", map[string]any{"a": aPath, "b": bPath, "out": outPath, "reconciled": reconciled, "tables": summary})

	fmt.Fprintf(out, "%-24s %6s %6s %6s\n", "table", "from_a", "from_b", "merged")
	for _, s := range summary {
		fmt.Fprintf(out, "%-24s %6d %6d %6d\n", s.Table, s.FromA, s.FromB, s.Merged)
	}
	fmt.Fprintf(out, "\nmerged knowledge base written to %s\n", outPath)
	fmt.Fprintln(out, "review it, then copy it into place over each machine's agents/knowledge.db yourself.")
	return summary, reconciled, divergences, nil
}

// checkpointAndCopy checkpoints srcPath's WAL (so all committed data is in
// the main file) and copies it — plus any -wal/-shm sidecars if still
// present — to dstPath.
func checkpointAndCopy(srcPath, dstPath string) error {
	db, err := sql.Open("sqlite", srcPath)
	if err != nil {
		return err
	}
	if _, err := db.Exec(`PRAGMA wal_checkpoint(FULL)`); err != nil {
		db.Close()
		return err
	}
	if err := db.Close(); err != nil {
		return err
	}

	if err := copyFile(srcPath, dstPath); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := copyFile(srcPath+suffix, dstPath+suffix); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}
