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
func cmdMerge(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
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
	summary, reconciled, err := runMerge(*aPath, *bPath, *outPath, *force, progressOut)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			CollisionsReconciled int                           `json:"collisions_reconciled"`
			Tables               []knowledge.MergeTableSummary `json:"tables"`
		}{CollisionsReconciled: reconciled, Tables: summary})
	}
	return nil
}

// runMerge does the actual merge work. On a collision abort (no -force),
// the collision detail is embedded in the returned error's message rather
// than written to out -- out is reserved for a successful run's progress
// narration, and an error's message is what survives into both the
// text-mode "kb: <message>" line and the JSON-mode {"error": "<message>"}
// envelope (see printError), so this is the one place collision detail
// can go that works correctly in both output modes.
func runMerge(aPath, bPath, outPath string, force bool, out io.Writer) (summary []knowledge.MergeTableSummary, reconciled int, err error) {
	scratch, err := os.MkdirTemp("", "kbmerge-")
	if err != nil {
		return nil, 0, err
	}
	defer os.RemoveAll(scratch)

	scratchA := filepath.Join(scratch, "a.db")
	scratchB := filepath.Join(scratch, "b.db")
	if err := checkpointAndCopy(aPath, scratchA); err != nil {
		return nil, 0, fmt.Errorf("checkpoint+copy %s: %w", aPath, err)
	}
	if err := checkpointAndCopy(bPath, scratchB); err != nil {
		return nil, 0, fmt.Errorf("checkpoint+copy %s: %w", bPath, err)
	}

	collisions, err := knowledge.CollisionReport(scratchA, scratchB)
	if err != nil {
		return nil, 0, fmt.Errorf("collision report: %w", err)
	}
	if len(collisions) > 0 {
		if !force {
			var msg strings.Builder
			fmt.Fprintf(&msg, "%d name/uuid collision(s) found:\n", len(collisions))
			for _, c := range collisions {
				fmt.Fprintf(&msg, "  %-10s %-30s %s vs %s\n", c.Table, c.Name, c.UUIDA, c.UUIDB)
			}
			msg.WriteString("aborting: resolve collisions or pass -force to reconcile b's identity to a's for each one")
			return nil, 0, errors.New(msg.String())
		}
		fmt.Fprintf(out, "%d name/uuid collision(s) found:\n", len(collisions))
		for _, c := range collisions {
			fmt.Fprintf(out, "  %-10s %-30s %s vs %s\n", c.Table, c.Name, c.UUIDA, c.UUIDB)
		}
		if err := knowledge.ReconcileCollisions(scratchB, collisions); err != nil {
			return nil, 0, fmt.Errorf("reconcile collisions: %w", err)
		}
		reconciled = len(collisions)
		fmt.Fprintf(out, "-force set: reconciled %d collision(s) — b's rows now share a's identity, so their observations/links merge normally instead of being dropped\n", reconciled)
	}

	summary, err = knowledge.MergeKnowledgeBases(scratchA, scratchB, outPath)
	if err != nil {
		return nil, reconciled, fmt.Errorf("merge: %w", err)
	}

	fmt.Fprintf(out, "%-24s %6s %6s %6s\n", "table", "from_a", "from_b", "merged")
	for _, s := range summary {
		fmt.Fprintf(out, "%-24s %6d %6d %6d\n", s.Table, s.FromA, s.FromB, s.Merged)
	}
	fmt.Fprintf(out, "\nmerged knowledge base written to %s\n", outPath)
	fmt.Fprintln(out, "review it, then copy it into place over each machine's agents/knowledge.db yourself.")
	return summary, reconciled, nil
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
