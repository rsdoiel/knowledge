package knowledge

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rsdoiel/knowledge/embedtest"
)

// The vector sidecar (semantic-search-plan.md T3.4, workspace DR-0088): a file
// beside the database, built per machine, never part of knowledge.db.

func TestVectorsPath_SitsBesideTheDatabase(t *testing.T) {
	for in, want := range map[string]string{
		"/w/agents/knowledge.db": "/w/agents/knowledge.vectors.db",
		"agents/knowledge.db":    "agents/knowledge.vectors.db",
		"/x/other.db":            "/x/other.vectors.db",
		"/x/noext":               "/x/noext.vectors.db",
	} {
		if got := VectorsPath(in); got != want {
			t.Errorf("VectorsPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOpenVectors_NeverBuiltIsErrNoIndexAndCreatesNothing(t *testing.T) {
	db := filepath.Join(t.TempDir(), "knowledge.db")
	_, err := OpenVectors(db)
	if !errors.Is(err, ErrNoIndex) {
		t.Fatalf("err = %v, want ErrNoIndex", err)
	}
	if _, statErr := os.Stat(VectorsPath(db)); statErr == nil {
		t.Error("OpenVectors created the sidecar")
	}
}

func TestCreateVectors_RecordsTheModelAndReopens(t *testing.T) {
	db := filepath.Join(t.TempDir(), "agents", "knowledge.db")
	e := embedtest.New()
	v, err := CreateVectors(db, e)
	if err != nil {
		t.Fatal(err)
	}
	m := v.Meta()
	if m.Model != "fake" || m.Dimensions != 32 || m.Fingerprint != "fake:fake" || m.SchemaVersion != VectorsSchemaVersion {
		t.Errorf("Meta = %+v", m)
	}
	v.Close()

	v, err = OpenVectors(db)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if v.Meta() != m {
		t.Errorf("reopened Meta = %+v, want %+v", v.Meta(), m)
	}
	if err := v.CheckModel(e); err != nil {
		t.Errorf("CheckModel with the same model: %v", err)
	}
}

func TestCreateVectors_ExistingIndexOfAnotherModelIsErrModelMismatch(t *testing.T) {
	db := filepath.Join(t.TempDir(), "knowledge.db")
	v, err := CreateVectors(db, embedtest.NewNamed("a"))
	if err != nil {
		t.Fatal(err)
	}
	v.Close()
	if _, err := CreateVectors(db, embedtest.NewNamed("b")); !errors.Is(err, ErrModelMismatch) {
		t.Errorf("CreateVectors with another model: err = %v, want ErrModelMismatch", err)
	}
}

func TestCheckModel_DifferentNameOrFingerprintOrDimensionsIsErrModelMismatch(t *testing.T) {
	db := filepath.Join(t.TempDir(), "knowledge.db")
	v, _ := CreateVectors(db, embedtest.New())
	defer v.Close()
	if err := v.CheckModel(embedtest.NewNamed("other")); !errors.Is(err, ErrModelMismatch) {
		t.Errorf("another model: err = %v, want ErrModelMismatch", err)
	}
	if err := v.CheckModel(sameNameOtherFingerprint{}); !errors.Is(err, ErrModelMismatch) {
		t.Errorf("another fingerprint: err = %v, want ErrModelMismatch", err)
	}
	if err := v.CheckModel(sameNameOtherDims{}); !errors.Is(err, ErrModelMismatch) {
		t.Errorf("other dimensions: err = %v, want ErrModelMismatch", err)
	}
	if err := v.CheckModel(embedtest.New()); err != nil {
		t.Errorf("the same model: %v", err)
	}
}

type sameNameOtherFingerprint struct{ *embedtest.Fake }

func (sameNameOtherFingerprint) Model() string       { return "fake" }
func (sameNameOtherFingerprint) Dimensions() int     { return 32 }
func (sameNameOtherFingerprint) Fingerprint() string { return "a different build" }

type sameNameOtherDims struct{ *embedtest.Fake }

func (sameNameOtherDims) Model() string       { return "fake" }
func (sameNameOtherDims) Dimensions() int     { return 64 }
func (sameNameOtherDims) Fingerprint() string { return "fake:fake" }

func newTestVectors(t *testing.T) *Vectors {
	t.Helper()
	v, err := CreateVectors(filepath.Join(t.TempDir(), "knowledge.db"), embedtest.New())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { v.Close() })
	return v
}

func chunksOf(t *testing.T, texts ...string) []StoredChunk {
	t.Helper()
	vecs, err := embedtest.New().Embed(texts)
	if err != nil {
		t.Fatal(err)
	}
	var out []StoredChunk
	for i, x := range texts {
		out = append(out, StoredChunk{Index: i, Hash: ChunkHash(x), Text: x, Vector: vecs[i]})
	}
	return out
}

func TestPutRow_RoundTripsVectorsTextAndHash(t *testing.T) {
	v := newTestVectors(t)
	want := chunksOf(t, "first chunk of text", "second chunk of text")
	if err := v.PutRow("observation", 7, want); err != nil {
		t.Fatal(err)
	}
	got, err := v.RowChunks("observation", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d chunks, want 2", len(got))
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Index != w.Index || g.Hash != w.Hash || g.Text != w.Text || len(g.Vector) != 32 {
			t.Fatalf("chunk %d = %+v, want %+v", i, g, w)
		}
		for j := range w.Vector {
			if g.Vector[j] != w.Vector[j] {
				t.Fatalf("chunk %d vector differs at %d: %v vs %v", i, j, g.Vector[j], w.Vector[j])
			}
		}
	}
}

func TestPutRow_ReplacesTheRowsEarlierChunks(t *testing.T) {
	v := newTestVectors(t)
	v.PutRow("record", 1, chunksOf(t, "one", "two", "three"))
	if err := v.PutRow("record", 1, chunksOf(t, "only one now")); err != nil {
		t.Fatal(err)
	}
	got, _ := v.RowChunks("record", 1)
	if len(got) != 1 || got[0].Text != "only one now" {
		t.Errorf("chunks = %+v, want just the new one", got)
	}
}

func TestPutRow_WrongDimensionsIsInvalidAndWritesNothing(t *testing.T) {
	v := newTestVectors(t)
	bad := []StoredChunk{{Index: 0, Hash: "h", Text: "x", Vector: []float32{1, 2, 3}}}
	if err := v.PutRow("observation", 1, bad); !errors.Is(err, ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
	if rows, _ := v.IndexedRows(); len(rows) != 0 {
		t.Errorf("rows = %v, want none", rows)
	}
}

func TestDeleteRowAndIndexedRows(t *testing.T) {
	v := newTestVectors(t)
	v.PutRow("observation", 1, chunksOf(t, "a"))
	v.PutRow("observation", 2, chunksOf(t, "b", "c"))
	v.PutRow("record", 1, chunksOf(t, "d"))
	rows, err := v.IndexedRows()
	if err != nil || len(rows) != 3 {
		t.Fatalf("IndexedRows = %v, %v; want 3 rows", rows, err)
	}
	if err := v.DeleteRow("observation", 2); err != nil {
		t.Fatal(err)
	}
	if got, _ := v.RowChunks("observation", 2); len(got) != 0 {
		t.Errorf("deleted row still has %d chunks", len(got))
	}
	if rows, _ := v.IndexedRows(); len(rows) != 2 {
		t.Errorf("after delete IndexedRows = %v, want 2", rows)
	}
	if err := v.DeleteRow("observation", 99); err != nil {
		t.Errorf("deleting a row that is not there: %v", err)
	}
}

func TestChunkHash_DependsOnlyOnTheText(t *testing.T) {
	if ChunkHash("a") != ChunkHash("a") || ChunkHash("a") == ChunkHash("b") {
		t.Error("ChunkHash is not a function of the text")
	}
}

func TestOpenVectors_ANewerSchemaIsRefused(t *testing.T) {
	db := filepath.Join(t.TempDir(), "knowledge.db")
	v, _ := CreateVectors(db, embedtest.New())
	if _, err := v.db.Exec(`UPDATE vectors_meta SET value = '999' WHERE key = 'schema_version'`); err != nil {
		t.Fatal(err)
	}
	v.Close()
	if _, err := OpenVectors(db); !errors.Is(err, ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid for a newer schema", err)
	}
}

func TestOpenVectors_NotADatabaseIsInvalid(t *testing.T) {
	db := filepath.Join(t.TempDir(), "knowledge.db")
	if err := os.WriteFile(VectorsPath(db), []byte("this is not a sqlite file at all, just text padding padding"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenVectors(db); !errors.Is(err, ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
}
