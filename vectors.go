package knowledge

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sqlite "github.com/glebarez/go-sqlite"
)

// The vector sidecar (workspace DR-0088, DR-0089): a SQLite file beside
// knowledge.db that holds the chunks of each embedded row and their vectors. It
// is derived data, rebuilt per machine, and is not part of knowledge.db or
// knowledge.jsonl, so a merge, import or check-db never touches it.

/** VectorsSchemaVersion is the version of the sidecar's tables. A file written
 * by a newer version is refused rather than misread.
 */
const VectorsSchemaVersion = 1

/** ErrNoIndex marks an error raised because the vector index has not been built:
 * there is no sidecar file beside the database. kb exits 66 for it (workspace
 * DR-0003 class "no_input") and the message points to kb embed. Match it with
 * errors.Is.
 *
 * Example:
 *   if _, err := knowledge.OpenVectors(dbPath); errors.Is(err, knowledge.ErrNoIndex) {
 *       // tell the user to run kb embed
 *   }
 */
var ErrNoIndex = errors.New("no vector index")

/** ErrModelMismatch marks an error raised because the vector index was built by
 * a different model (name, dimensions or fingerprint) than the embedder in use.
 * Mixing the two would rank nonsense, so it is refused; the remedy is to rebuild
 * the index. kb exits 65 for it (class "data"). Match it with errors.Is.
 *
 * Example:
 *   if err := v.CheckModel(e); errors.Is(err, knowledge.ErrModelMismatch) {
 *       // tell the user to run kb embed -rebuild
 *   }
 */
var ErrModelMismatch = errors.New("vector index built by another model")

const vectorsSchema = `
CREATE TABLE IF NOT EXISTS vectors_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS vector_chunks (
    source_type  TEXT    NOT NULL,
    source_id    INTEGER NOT NULL,
    chunk_index  INTEGER NOT NULL,
    model        TEXT    NOT NULL,
    content_hash TEXT    NOT NULL,
    text         TEXT    NOT NULL,
    vector       BLOB    NOT NULL,
    PRIMARY KEY (source_type, source_id, chunk_index, model)
);
CREATE TABLE IF NOT EXISTS vectors_lock (
    id           INTEGER PRIMARY KEY CHECK (id = 1),
    owner        TEXT    NOT NULL,
    claimed_at   INTEGER NOT NULL,
    heartbeat_at INTEGER NOT NULL
);
`

/** VectorsMeta is what the sidecar records about how it was built.
 *
 * Fields:
 *   SchemaVersion (int)    — the version of the sidecar's tables
 *   Model         (string) — the embedding model's name
 *   Dimensions    (int)    — the length of each vector
 *   Fingerprint   (string) — identifies the exact model files
 *
 * Example:
 *   m := v.Meta()
 *   fmt.Println(m.Model, m.Dimensions)
 */
type VectorsMeta struct {
	SchemaVersion int
	Model         string
	Dimensions    int
	Fingerprint   string
}

/** StoredChunk is one chunk of a row with its vector.
 *
 * Fields:
 *   SourceType (string)    — kb_fts source_type of the row (set when read back)
 *   SourceID   (int64)     — the row's id (set when read back)
 *   Index      (int)       — the chunk's position in the row, from 0
 *   Hash       (string)    — ChunkHash of Text, so an unchanged chunk is not re-embedded
 *   Text       (string)    — the chunk, kept for the passage line of a hit
 *   Vector     ([]float32) — the embedding, of the sidecar's dimensions
 *
 * Example:
 *   c := knowledge.StoredChunk{Index: 0, Hash: knowledge.ChunkHash(t), Text: t, Vector: vec}
 */
type StoredChunk struct {
	SourceType string
	SourceID   int64
	Index      int
	Hash       string
	Text       string
	Vector     []float32
}

/** RowKey names a row that has chunks in the sidecar.
 *
 * Example:
 *   rows, _ := v.IndexedRows()
 *   fmt.Println(rows[0].SourceType, rows[0].SourceID)
 */
type RowKey struct {
	SourceType string
	SourceID   int64
}

/** Vectors is an open vector sidecar. Make one with OpenVectors or
 * CreateVectors and close it with Close.
 */
type Vectors struct {
	db   *sql.DB
	path string
	meta VectorsMeta
	now  func() time.Time // the clock the lock reads; tests replace it
}

/** VectorsPath returns where the sidecar for a database lives: beside it, named
 * for it, so --db and KB_DB move both together. agents/knowledge.db gives
 * agents/knowledge.vectors.db.
 *
 * Parameters:
 *   dbPath (string) — the knowledge database's path
 *
 * Returns:
 *   string — the sidecar's path
 *
 * Example:
 *   p := knowledge.VectorsPath("agents/knowledge.db") // agents/knowledge.vectors.db
 */
func VectorsPath(dbPath string) string {
	return strings.TrimSuffix(dbPath, ".db") + ".vectors.db"
}

/** ChunkHash returns the content hash stored with a chunk: the hex SHA-256 of its
 * text. Equal text gives an equal hash, which is how an unchanged chunk is
 * recognised and skipped.
 *
 * Parameters:
 *   text (string) — the chunk text
 *
 * Returns:
 *   string — the hash
 *
 * Example:
 *   h := knowledge.ChunkHash("a chunk")
 */
func ChunkHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

/** OpenVectors opens the sidecar of a database that already has one. It never
 * creates the file, so a search cannot build an index by accident.
 *
 * Parameters:
 *   dbPath (string) — the knowledge database's path (the sidecar's is derived)
 *
 * Returns:
 *   *Vectors — the open sidecar
 *   error — ErrNoIndex when there is no sidecar; ErrInvalid when the file is not
 *     a vector index or was written by a newer schema; an OS error otherwise
 *
 * Example:
 *   v, err := knowledge.OpenVectors("agents/knowledge.db")
 */
func OpenVectors(dbPath string) (*Vectors, error) {
	path := VectorsPath(dbPath)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, markedf(ErrNoIndex, "no vector index at %s: run kb embed to build one", path)
		}
		return nil, fmt.Errorf("knowledge: vector index %s: %w", path, err)
	}
	return openVectorsFile(path, nil)
}

/** CreateVectors opens the sidecar of a database, creating it for the embedder
 * if it does not exist. An existing sidecar must have been built by the same
 * model.
 *
 * Parameters:
 *   dbPath (string) — the knowledge database's path (the sidecar's is derived)
 *   e (Embedder) — the embedder whose vectors the sidecar will hold
 *
 * Returns:
 *   *Vectors — the open sidecar
 *   error — ErrModelMismatch when the existing sidecar was built by another
 *     model; otherwise as OpenVectors
 *
 * Example:
 *   v, err := knowledge.CreateVectors("agents/knowledge.db", embedder)
 */
func CreateVectors(dbPath string, e Embedder) (*Vectors, error) {
	path := VectorsPath(dbPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("knowledge: create directory for %s: %w", path, err)
	}
	return openVectorsFile(path, e)
}

func openVectorsFile(path string, create Embedder) (*Vectors, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("knowledge: open %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	v := &Vectors{db: db, path: path, now: time.Now}
	fail := func(err error) (*Vectors, error) {
		db.Close()
		var se *sqlite.Error
		if errors.As(err, &se) && (se.Code()&0xff == 26 || se.Code()&0xff == 11) { // NOTADB, CORRUPT
			return nil, invalidf("knowledge: %s is not a vector index: %v", path, err)
		}
		return nil, fmt.Errorf("knowledge: vector index %s: %w", path, refusedByOS(path, err))
	}
	// A second kb opening the file while one is writing waits briefly rather than failing.
	_, _ = db.Exec(`PRAGMA busy_timeout = 5000`)
	if _, err := db.Exec(vectorsSchema); err != nil {
		return fail(err)
	}
	meta, found, err := v.readMeta()
	if err != nil {
		return fail(err)
	}
	if found && meta.SchemaVersion > VectorsSchemaVersion {
		db.Close()
		return nil, invalidf("knowledge: %s has schema version %d; this kb reads up to %d", path, meta.SchemaVersion, VectorsSchemaVersion)
	}
	switch {
	case found:
		v.meta = meta
		if create != nil {
			if err := v.CheckModel(create); err != nil {
				db.Close()
				return nil, err
			}
		}
	case create != nil:
		v.meta = VectorsMeta{SchemaVersion: VectorsSchemaVersion, Model: create.Model(), Dimensions: create.Dimensions(), Fingerprint: create.Fingerprint()}
		if err := v.writeMeta(); err != nil {
			return fail(err)
		}
	default:
		db.Close()
		return nil, markedf(ErrNoIndex, "no vector index in %s: run kb embed to build one", path)
	}
	return v, nil
}

func (v *Vectors) readMeta() (VectorsMeta, bool, error) {
	rows, err := v.db.Query(`SELECT key, value FROM vectors_meta`)
	if err != nil {
		return VectorsMeta{}, false, err
	}
	defer rows.Close()
	var m VectorsMeta
	n := 0
	for rows.Next() {
		var k, val string
		if err := rows.Scan(&k, &val); err != nil {
			return VectorsMeta{}, false, err
		}
		n++
		switch k {
		case "schema_version":
			m.SchemaVersion, _ = strconv.Atoi(val)
		case "model":
			m.Model = val
		case "dimensions":
			m.Dimensions, _ = strconv.Atoi(val)
		case "fingerprint":
			m.Fingerprint = val
		}
	}
	return m, n > 0, rows.Err()
}

func (v *Vectors) writeMeta() error {
	tx, err := v.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, val := range map[string]string{
		"schema_version": strconv.Itoa(v.meta.SchemaVersion),
		"model":          v.meta.Model,
		"dimensions":     strconv.Itoa(v.meta.Dimensions),
		"fingerprint":    v.meta.Fingerprint,
	} {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO vectors_meta(key, value) VALUES (?, ?)`, k, val); err != nil {
			return err
		}
	}
	return tx.Commit()
}

/** Meta returns what the sidecar recorded about how it was built.
 *
 * Returns:
 *   VectorsMeta — the schema version, model, dimensions and fingerprint
 */
func (v *Vectors) Meta() VectorsMeta { return v.meta }

/** Path returns the sidecar file's path. */
func (v *Vectors) Path() string { return v.path }

/** Close closes the sidecar. */
func (v *Vectors) Close() error { return v.db.Close() }

/** CheckModel reports whether e is the model the sidecar was built with: same
 * name, dimensions and fingerprint.
 *
 * Parameters:
 *   e (Embedder) — the embedder about to use the sidecar
 *
 * Returns:
 *   error — nil, or ErrModelMismatch naming both models and the remedy
 *
 * Example:
 *   if err := v.CheckModel(embedder); err != nil { return err }
 */
func (v *Vectors) CheckModel(e Embedder) error {
	if v.meta.Model == e.Model() && v.meta.Dimensions == e.Dimensions() && v.meta.Fingerprint == e.Fingerprint() {
		return nil
	}
	return markedf(ErrModelMismatch,
		"the vector index %s was built with model %q (%d dimensions, fingerprint %q), not %q (%d, %q): run kb embed -rebuild",
		v.path, v.meta.Model, v.meta.Dimensions, v.meta.Fingerprint, e.Model(), e.Dimensions(), e.Fingerprint())
}

/** PutRow stores the chunks of one row, replacing any it had, in one
 * transaction: after it returns the row has exactly these chunks, and if it
 * fails the row is as it was.
 *
 * Parameters:
 *   sourceType (string) — kb_fts source_type of the row
 *   sourceID (int64) — the row's id
 *   chunks ([]StoredChunk) — the row's chunks; each vector must have the sidecar's dimensions
 *
 * Returns:
 *   error — ErrInvalid for a vector of the wrong length; a database error otherwise
 *
 * Example:
 *   err := v.PutRow("observation", 12, chunks)
 */
func (v *Vectors) PutRow(sourceType string, sourceID int64, chunks []StoredChunk) error {
	for _, c := range chunks {
		if len(c.Vector) != v.meta.Dimensions {
			return invalidf("knowledge: chunk %d of %s:%d has %d dimensions, the index has %d",
				c.Index, sourceType, sourceID, len(c.Vector), v.meta.Dimensions)
		}
	}
	tx, err := v.db.Begin()
	if err != nil {
		return fmt.Errorf("knowledge: vector index: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM vector_chunks WHERE source_type = ? AND source_id = ?`, sourceType, sourceID); err != nil {
		return fmt.Errorf("knowledge: vector index: %w", err)
	}
	for _, c := range chunks {
		if _, err := tx.Exec(`INSERT INTO vector_chunks(source_type, source_id, chunk_index, model, content_hash, text, vector)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, sourceType, sourceID, c.Index, v.meta.Model, c.Hash, c.Text, encodeVector(c.Vector)); err != nil {
			return fmt.Errorf("knowledge: vector index: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("knowledge: vector index: %w", err)
	}
	return nil
}

/** RowChunks returns the chunks stored for a row, in chunk order.
 *
 * Parameters:
 *   sourceType (string) — kb_fts source_type of the row
 *   sourceID (int64) — the row's id
 *
 * Returns:
 *   []StoredChunk — the chunks, none if the row is not indexed
 *   error — a database error
 *
 * Example:
 *   chunks, err := v.RowChunks("record", 3)
 */
func (v *Vectors) RowChunks(sourceType string, sourceID int64) ([]StoredChunk, error) {
	rows, err := v.db.Query(`SELECT chunk_index, content_hash, text, vector FROM vector_chunks
		WHERE source_type = ? AND source_id = ? ORDER BY chunk_index`, sourceType, sourceID)
	if err != nil {
		return nil, fmt.Errorf("knowledge: vector index: %w", err)
	}
	defer rows.Close()
	var out []StoredChunk
	for rows.Next() {
		c := StoredChunk{SourceType: sourceType, SourceID: sourceID}
		var blob []byte
		if err := rows.Scan(&c.Index, &c.Hash, &c.Text, &blob); err != nil {
			return nil, fmt.Errorf("knowledge: vector index: %w", err)
		}
		if c.Vector, err = decodeVector(blob, v.meta.Dimensions); err != nil {
			return nil, fmt.Errorf("knowledge: vector index %s:%d chunk %d: %w", sourceType, sourceID, c.Index, err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

/** DeleteRow removes every chunk of a row. A row with none is not an error.
 *
 * Parameters:
 *   sourceType (string) — kb_fts source_type of the row
 *   sourceID (int64) — the row's id
 *
 * Returns:
 *   error — a database error
 *
 * Example:
 *   err := v.DeleteRow("observation", 12)
 */
func (v *Vectors) DeleteRow(sourceType string, sourceID int64) error {
	if _, err := v.db.Exec(`DELETE FROM vector_chunks WHERE source_type = ? AND source_id = ?`, sourceType, sourceID); err != nil {
		return fmt.Errorf("knowledge: vector index: %w", err)
	}
	return nil
}

/** IndexedRows lists the rows that have chunks in the sidecar.
 *
 * Returns:
 *   []RowKey — the rows, ordered by type then id
 *   error — a database error
 *
 * Example:
 *   rows, err := v.IndexedRows()
 */
func (v *Vectors) IndexedRows() ([]RowKey, error) {
	rows, err := v.db.Query(`SELECT DISTINCT source_type, source_id FROM vector_chunks ORDER BY source_type, source_id`)
	if err != nil {
		return nil, fmt.Errorf("knowledge: vector index: %w", err)
	}
	defer rows.Close()
	var out []RowKey
	for rows.Next() {
		var k RowKey
		if err := rows.Scan(&k.SourceType, &k.SourceID); err != nil {
			return nil, fmt.Errorf("knowledge: vector index: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// encodeVector writes a vector as little-endian float32s.
func encodeVector(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(x))
	}
	return b
}

// decodeVector reads what encodeVector wrote and checks its length.
func decodeVector(b []byte, dims int) ([]float32, error) {
	if len(b) != 4*dims {
		return nil, invalidf("stored vector is %d bytes, want %d", len(b), 4*dims)
	}
	v := make([]float32, dims)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return v, nil
}
