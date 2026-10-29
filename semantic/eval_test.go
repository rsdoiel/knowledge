package semantic

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
	"github.com/rsdoiel/knowledge/internal/evalset"
)

// The labelled-query evaluation of semantic plan T9, run with the real model on a
// copy of a real knowledge base. It is skipped without both. It prints the tables
// the tunables are set from; run it with -v:
//
//	KB_EVAL_DB=~/Laboratory/tmp/embed-real/agents/knowledge.db go test ./semantic -run TestEval -v
//
// KB_EVAL_DB is a knowledge.db beside a finished knowledge.vectors.db built by the
// model under test (the default is the dev copy in tmp/embed-real). KB_EVAL_MODEL_DIR
// is the model (default: the dev copy in tmp/xdg). KB_EVAL_CHUNK=N builds a fresh
// index of N-character chunks in the scratch copy first, which takes the 15 minutes
// a first index takes on a Pi; run that one detached.

func evalPaths(t *testing.T) (db, model string) {
	t.Helper()
	home, _ := os.UserHomeDir()
	db = firstExisting(os.Getenv("KB_EVAL_DB"), filepath.Join("..", "..", "tmp", "embed-real", "agents", "knowledge.db"))
	model = firstExisting(os.Getenv("KB_EVAL_MODEL_DIR"),
		filepath.Join(home, "Laboratory", "tmp", "xdg", "kb", "models", "all-MiniLM-L6-v2"))
	if db == "" || model == "" {
		t.Skip("no evaluation database or model: set KB_EVAL_DB and KB_EVAL_MODEL_DIR")
	}
	return db, model
}

func firstExisting(paths ...string) string {
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	in, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
}

// evalFixture opens a scratch copy of the evaluation database with its index.
func evalFixture(t *testing.T) (*knowledge.KnowledgeBase, *knowledge.Vectors, knowledge.Embedder) {
	t.Helper()
	srcDB, model := evalPaths(t)
	dir := t.TempDir()
	db := filepath.Join(dir, "knowledge.db")
	copyFile(t, srcDB, db)
	emb := NewHugot(model)
	kb, err := knowledge.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kb.Close() })

	var v *knowledge.Vectors
	if chunk := os.Getenv("KB_EVAL_CHUNK"); chunk != "" {
		n, err := strconv.Atoi(chunk)
		if err != nil || n < 50 {
			t.Fatalf("KB_EVAL_CHUNK=%q", chunk)
		}
		if v, err = knowledge.CreateVectors(db, emb); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { v.Close() })
		res, err := kb.EmbedIndex(context.Background(), v, emb, knowledge.EmbedOptions{ChunkRunes: n,
			Progress: func(p knowledge.EmbedProgress) {
				if p.Done%100 == 0 {
					t.Logf("embedded %d of %d rows", p.Done, p.Total)
				}
			}})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("built a %d-character index: %d chunks", n, res.ChunksEmbedded)
		return kb, v, emb
	}
	copyFile(t, knowledge.VectorsPath(srcDB), knowledge.VectorsPath(db))
	if v, err = knowledge.OpenVectors(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { v.Close() })
	return kb, v, emb
}

func TestEval_LabelledQueries(t *testing.T) {
	kb, v, emb := evalFixture(t)
	set, err := evalset.Load(filepath.Join("..", "internal", "evalset", "testdata", "labelled_queries.json"))
	if err != nil {
		t.Fatal(err)
	}
	rel, stale, err := set.Resolve(kb)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range stale {
		t.Errorf("stale label (the evaluation database lacks it): %s", s)
	}
	prep, err := evalset.Prepare(kb, v, emb, set, rel)
	if err != nil {
		t.Fatal(err)
	}

	anyWord, err := evalset.PrepareWith(kb, v, emb, set, rel, evalset.AnyWord)
	if err != nil {
		t.Fatal(err)
	}

	var b strings.Builder
	b.WriteString("\nHybrid with the words half as typed (AND) and as any word (OR), by cutoff:\n")
	for _, c := range []float32{0.30, 0.35, 0.40} {
		and, or := prep.Score(evalset.Params{MinScore: c}), anyWord.Score(evalset.Params{MinScore: c})
		fmt.Fprintf(&b, "cutoff %.2f  semantic MRR %.3f | hybrid AND recall@5 %.3f @10 %.3f MRR %.3f | hybrid OR recall@5 %.3f @10 %.3f MRR %.3f | OR lexical alone MRR %.3f\n",
			c, and.Semantic.MRR, and.Hybrid.Recall5, and.Hybrid.Recall10, and.Hybrid.MRR, or.Hybrid.Recall5, or.Hybrid.Recall10, or.Hybrid.MRR, or.Lexical.MRR)
	}
	b.WriteString("\nHybrid (any word) fusion constant, cutoff 0.35:\n")
	for _, k := range []int{5, 10, 30, 60, 100} {
		r := anyWord.Score(evalset.Params{MinScore: 0.35, FusionK: k})
		fmt.Fprintf(&b, "k %3d  recall@5 %.3f recall@10 %.3f MRR %.3f\n", k, r.Hybrid.Recall5, r.Hybrid.Recall10, r.Hybrid.MRR)
	}
	b.WriteString("\nCutoff sweep (fusion k 60):\n")
	for _, c := range []float32{0.20, 0.25, 0.30, 0.35, 0.40, 0.45, 0.50} {
		fmt.Fprintf(&b, "\n%s", prep.Score(evalset.Params{MinScore: c}))
	}
	b.WriteString("\nFusion constant sweep (cutoff 0.30):\n")
	for _, k := range []int{5, 10, 30, 60, 100} {
		r := prep.Score(evalset.Params{MinScore: 0.30, FusionK: k})
		fmt.Fprintf(&b, "k %3d  hybrid recall@5 %.3f recall@10 %.3f MRR %.3f\n", k, r.Hybrid.Recall5, r.Hybrid.Recall10, r.Hybrid.MRR)
	}
	def := prep.Score(evalset.Params{MinScore: knowledge.DefaultMinScore})
	b.WriteString("\nPer query at the defaults (first relevant rank, rows returned; 0 is not returned):\n")
	for _, q := range def.PerQuery {
		kind := "     "
		if !q.Answerable {
			kind = "none "
		}
		fmt.Fprintf(&b, "%s %s  lexical %2d/%-3d semantic %2d/%-3d hybrid %2d/%-3d\n", q.ID, kind, q.LexRank, q.LexRows, q.SemRank, q.SemRows, q.HybRank, q.HybRows)
	}
	for _, mode := range []string{"lexical", "semantic", "hybrid"} {
		fmt.Fprintf(&b, "\nMisses at the defaults, %s: %s\n", mode, strings.Join(def.Misses(mode), "; "))
	}
	t.Log(b.String())

	// The guard: the numbers measured on 2026-10-29 for all-MiniLM-L6-v2 with
	// 400-character chunks and the default cutoff, less a margin for the corpus
	// growing and the labels being one person's. A chunker, cutoff or model change
	// that costs more than the margin fails here; re-measure and move the floors on
	// purpose. Skipped for other chunk sizes, which this test is also used to compare.
	if os.Getenv("KB_EVAL_CHUNK") != "" {
		return
	}
	if def.Semantic.MRR < 0.75 || def.Semantic.Recall10 < 0.80 || def.Semantic.Recall5 < 0.67 {
		t.Errorf("semantic search got worse than the floors (MRR 0.75, recall@10 0.80, recall@5 0.67):\n%s", def)
	}
	if def.Semantic.Quiet < 3 {
		t.Errorf("semantic search answers the queries that have no answer: quiet %d of 4, floor 3\n%s", def.Semantic.Quiet, def)
	}
}
