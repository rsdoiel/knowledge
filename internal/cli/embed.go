package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["embed"] = cmdEmbed
}

// embedContext is the context a kb embed run is cancelled by: an interrupt or
// terminate signal. A test replaces it to cancel a run part way.
var embedContext = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// writeTimeMaxRows is the most rows a write verb embeds on its own account. A
// longer backlog (after a merge or an import, say) is left to kb embed, so a
// one-line change never waits minutes for a model.
const writeTimeMaxRows = 25

const embedUsage = "usage: embed [-status] [-rebuild]"

/** cmdEmbed implements the embed verb: it brings the semantic-search index
 * (the vector sidecar beside the database) in step with the knowledge base,
 * embedding only rows that are new or changed. -rebuild starts the index over;
 * -status reports how far behind it is and changes nothing. A run commits as it
 * goes, so an interrupt loses at most one batch and the next run carries on.
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — the open knowledge base.
 *   dl      (*DebugLog)                — debug log, may be nil.
 *   jsonOut (bool)                     — emit the result as JSON.
 *   args    ([]string)                 — the verb's flags.
 *   out     (io.Writer)                — where the result is written.
 *
 * Returns:
 *   error — a usage error for a bad command line or a build without semantic
 *           search; 66 for a missing model or (for search) a missing index; 65
 *           for an index built by another model; 75 when another run holds the
 *           index or this one was interrupted.
 *
 * Example:
 *   err := cmdEmbed(kb, nil, false, []string{"-status"}, os.Stdout)
 */
func cmdEmbed(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	var status, rebuild bool
	pos, err := splitFlags(args, nil, map[string]*bool{
		"-status": &status, "--status": &status, "-rebuild": &rebuild, "--rebuild": &rebuild,
	})
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return usageErrorf("%s", embedUsage)
	}
	if status && rebuild {
		return usageErrorf("-status changes nothing, so it cannot be combined with -rebuild; %s", embedUsage)
	}
	emb, haveEmbedder := knowledge.RegisteredEmbedder()
	if status {
		return embedStatus(kb, jsonOut, out, emb, haveEmbedder)
	}
	if !haveEmbedder {
		return usageErrorf("this build of kb has no semantic search, so there is nothing to embed with; use the semantic build of kb")
	}
	if c, ok := emb.(knowledge.Checker); ok {
		if err := c.Available(); err != nil {
			return err
		}
	}
	dbPath := kb.Path()
	var v *knowledge.Vectors
	if rebuild {
		// Take the claim before removing the file, so a rebuild cannot pull the
		// index out from under a run that is writing it.
		if cur, err := knowledge.OpenVectors(dbPath); err == nil {
			l, lerr := cur.Lock()
			if lerr != nil {
				cur.Close()
				return lerr
			}
			l.Release()
			cur.Close()
		}
		v, err = knowledge.ResetVectors(dbPath, emb)
	} else {
		v, err = knowledge.CreateVectors(dbPath, emb)
	}
	if err != nil {
		return err
	}
	defer v.Close()
	warnVectorsTracked(dbPath)
	dl.Log("embed", map[string]any{"model": emb.Model(), "rebuild": rebuild})

	ctx, stop := embedContext()
	defer stop()
	start := time.Now()
	opts := knowledge.EmbedOptions{}
	if !jsonOut {
		opts.Progress = func(p knowledge.EmbedProgress) {
			fmt.Fprintf(adviceOut, "embedded %d of %d rows\n", p.Done, p.Total)
		}
	}
	res, err := kb.EmbedIndex(ctx, v, emb, opts)
	took := time.Since(start).Round(time.Millisecond)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return classErrorf(classTempFail, "interrupted after %d rows (%d chunks); the work so far is kept, run kb embed again to carry on", res.RowsEmbedded, res.ChunksEmbedded)
		}
		return err
	}
	if jsonOut {
		return printJSON(out, map[string]any{
			"model": emb.Model(), "rebuild": rebuild,
			"rows_embedded": res.RowsEmbedded, "rows_up_to_date": res.RowsSkipped,
			"rows_removed": res.RowsRemoved, "chunks_embedded": res.ChunksEmbedded,
			"seconds": took.Seconds(),
		})
	}
	fmt.Fprintf(out, "embedded %d rows (%d chunks) with %s in %s; %d already up to date, %d removed\n",
		res.RowsEmbedded, res.ChunksEmbedded, emb.Model(), took, res.RowsSkipped, res.RowsRemoved)
	return nil
}

// modelInstalled reports whether an embedder can run: true for one that does not
// say otherwise.
func modelInstalled(e knowledge.Embedder) bool {
	c, ok := e.(knowledge.Checker)
	return !ok || c.Available() == nil
}

// warnVectorsTracked warns once if git tracks the vector index of the workspace
// the database belongs to.
func warnVectorsTracked(dbPath string) {
	dir := filepath.Dir(dbPath)
	if filepath.Base(dir) != "agents" {
		return
	}
	warnIfTracked(filepath.Dir(dir), "agents/knowledge.vectors.db")
}

// embedStatus reports how far the index is behind the knowledge base. It needs
// no embedder and loads no model, so it also works in the plain build.
func embedStatus(kb *knowledge.KnowledgeBase, jsonOut bool, out io.Writer, emb knowledge.Embedder, haveEmbedder bool) error {
	doc := map[string]any{"semantic_build": haveEmbedder}
	if haveEmbedder {
		doc["model"] = emb.Model()
	}
	v, err := knowledge.OpenVectors(kb.Path())
	var st knowledge.EmbedStatus
	switch {
	case errors.Is(err, knowledge.ErrNoIndex):
		corpus, cerr := kb.EmbedCorpus(knowledge.SearchScope{})
		if cerr != nil {
			return cerr
		}
		st = knowledge.EmbedStatus{Rows: len(corpus), RowsToEmbed: len(corpus), RowsByType: map[string]int{}}
		for _, r := range corpus {
			st.RowsByType[r.SourceType]++
			st.ChunksToEmbed += len(knowledge.Chunk(r.Text, knowledge.DefaultChunkRunes))
		}
		doc["index"] = nil
	case err != nil:
		return err
	default:
		defer v.Close()
		if st, err = kb.EmbedStatus(v, 0); err != nil {
			return err
		}
		m := v.Meta()
		doc["index"] = map[string]any{"model": m.Model, "dimensions": m.Dimensions, "fingerprint": m.Fingerprint}
		// A model whose files are missing cannot be compared: its fingerprint is
		// unreadable, which is not a mismatch, and "rebuild" would throw the index away.
		if haveEmbedder {
			if modelInstalled(emb) {
				doc["model_installed"] = true
				doc["model_matches"] = v.CheckModel(emb) == nil
			} else {
				doc["model_installed"] = false
			}
		}
	}
	doc["rows"], doc["rows_by_type"] = st.Rows, st.RowsByType
	doc["rows_up_to_date"], doc["rows_not_indexed"], doc["rows_to_remove"] = st.RowsUpToDate, st.RowsToEmbed, st.RowsToRemove
	doc["chunks_stored"], doc["chunks_to_embed"] = st.ChunksStored, st.ChunksToEmbed
	if jsonOut {
		return printJSON(out, doc)
	}
	if haveEmbedder {
		fmt.Fprintf(out, "This build of kb has semantic search (model %s).\n", emb.Model())
	} else {
		fmt.Fprintln(out, "This build of kb has no semantic search; the counts below are all it can say.")
	}
	var kinds []string
	for k, n := range st.RowsByType {
		kinds = append(kinds, fmt.Sprintf("%s %d", k, n))
	}
	sort.Strings(kinds)
	fmt.Fprintf(out, "Rows: %d (%s)\n", st.Rows, strings.Join(kinds, ", "))
	if idx, ok := doc["index"].(map[string]any); ok {
		fmt.Fprintf(out, "Index: built with %s (%d dimensions), %d chunks stored\n", idx["model"], idx["dimensions"], st.ChunksStored)
		if m, ok := doc["model_matches"].(bool); ok && !m {
			fmt.Fprintln(out, "The index was built by a different model than this build uses: run kb embed -rebuild.")
		}
		if i, ok := doc["model_installed"].(bool); ok && !i {
			fmt.Fprintln(out, "The model is not installed, so the index cannot be updated or searched: run kb model fetch.")
		}
	} else {
		fmt.Fprintln(out, "Index: none yet; kb embed builds it.")
	}
	fmt.Fprintf(out, "Not indexed: %d rows (%d chunks to embed); up to date: %d; to remove: %d\n",
		st.RowsToEmbed, st.ChunksToEmbed, st.RowsUpToDate, st.RowsToRemove)
	return nil
}

// embedVerbs are the verbs that change rows semantic search embeds. import,
// merge and check-db are absent on purpose: they leave the index alone, and a
// merge's new rows are simply not yet indexed until kb embed (plan T7).
var embedVerbs = map[string]bool{"observation": true, "concept": true, "record": true, "document": true, "ingest": true, "project": true}

// changesCorpus reports whether a command line is a write to rows that are
// embedded: the verb is one of embedVerbs, the subverb (looked up in the verb
// table) is not a read, and the command is not a dry run.
func changesCorpus(args []string) bool {
	if len(args) == 0 || !embedVerbs[args[0]] {
		return false
	}
	for _, a := range args[1:] {
		if a == "-dry-run" || a == "--dry-run" || isHelpFlag(a) {
			return false
		}
	}
	for _, spec := range verbTable {
		if spec.Name != args[0] {
			continue
		}
		subs := spec.Subverbs
		class := spec.Class
		for i := 1; i < len(args) && len(subs) > 0; i++ {
			var next *subverbSpec
			for j := range subs {
				if subs[j].Name == args[i] {
					next = &subs[j]
					break
				}
			}
			if next == nil {
				return false
			}
			class, subs = next.Class, next.Subverbs
			if len(subs) == 0 {
				break
			}
		}
		return class != classRead && class != classNone
	}
	return false
}

/** embedAfterWrite keeps the semantic index current after a write verb, on a
 * best-effort basis: it embeds the rows the command changed when the index
 * exists and the model is usable, and otherwise says in one line that rows are
 * not yet indexed. It never fails the command and never builds an index: with no
 * index, or a build without an embedder, it does nothing.
 *
 * Parameters:
 *   kb (*knowledge.KnowledgeBase) — the knowledge base the verb wrote to
 *
 * Example:
 *   embedAfterWrite(kb) // after a successful observation add
 */
func embedAfterWrite(kb *knowledge.KnowledgeBase) {
	emb, ok := knowledge.RegisteredEmbedder()
	if !ok {
		return
	}
	v, err := knowledge.OpenVectors(kb.Path())
	if err != nil {
		if !errors.Is(err, knowledge.ErrNoIndex) {
			advise("the semantic index could not be updated: %v", err)
		}
		return
	}
	defer v.Close()
	st, err := kb.EmbedStatus(v, 0)
	if err != nil {
		advise("the semantic index could not be updated: %v", err)
		return
	}
	if st.RowsToEmbed == 0 && st.RowsToRemove == 0 {
		return
	}
	notIndexed := func(why string) {
		advise("%d rows are not yet indexed for semantic search (%s); run kb embed", st.RowsToEmbed, why)
	}
	if !modelInstalled(emb) {
		// Nothing to embed with. Rows that are only to be removed wait too: telling
		// whether the model is the index's own needs its files.
		if st.RowsToEmbed > 0 {
			notIndexed("the model is not installed: kb model fetch")
		}
		return
	}
	if err := v.CheckModel(emb); err != nil {
		notIndexed("the index was built by another model; run kb embed -rebuild")
		return
	}
	if st.RowsToEmbed > writeTimeMaxRows {
		notIndexed("too many to embed with this command")
		return
	}
	if _, err := kb.EmbedIndex(context.Background(), v, emb, knowledge.EmbedOptions{}); err != nil {
		if errors.Is(err, knowledge.ErrLocked) {
			notIndexed("another kb embed is running")
		} else {
			notIndexed(err.Error())
		}
	}
}
