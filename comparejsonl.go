package knowledge

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

/** DBComparison is the result of comparing a live database with a JSONL dump.
 * In every TableDiff the first side ("A") is the database and the second
 * ("B") is the dump: OnlyA is "in this database, not in the JSONL", OnlyB is
 * "in the JSONL, not in this database".
 *
 * Fields:
 *   Tables     ([]TableDiff)        — per-table result, clean tables included.
 *   Collisions ([]IdentityCollision) — entities held under different uuids on the two sides.
 *
 * Example:
 *   c, _ := kb.CompareToJSONL(f)
 *   fmt.Println(c.Recommendation())
 */
type DBComparison struct {
	Tables     []TableDiff
	Collisions []IdentityCollision
}

// InSync reports whether the database and the dump hold the same content.
func (c DBComparison) InSync() bool {
	return c.Recommendation() == "in-sync"
}

/** Recommendation names the action that would bring the two together:
 * "in-sync" when nothing differs; "import" when the dump only adds rows to
 * the database; "export" when the database only adds rows to the dump;
 * "diverged" when both have rows the other lacks, or any row on both sides
 * differs, or an entity carries two uuids (import, then export).
 *
 * Because there are no tombstones, rows only in the dump may be rows deleted
 * here on purpose, and an "import" recommendation brings them back.
 *
 * Returns:
 *   string — one of "in-sync", "import", "export", "diverged".
 *
 * Example:
 *   switch c.Recommendation() { case "import": ... }
 */
func (c DBComparison) Recommendation() string {
	var onlyA, onlyB, different bool
	for _, d := range c.Tables {
		onlyA = onlyA || len(d.OnlyA) > 0
		onlyB = onlyB || len(d.OnlyB) > 0
		different = different || len(d.Different) > 0
	}
	switch {
	case different || len(c.Collisions) > 0 || (onlyA && onlyB):
		return "diverged"
	case onlyB:
		return "import"
	case onlyA:
		return "export"
	}
	return "in-sync"
}

/** CompareToJSONL compares this database with a JSONL dump by full content,
 * never by file time, and changes neither. The database is copied with
 * VACUUM INTO, the dump is imported into a second scratch database, and the
 * two go through the same identity rules merge uses (DiffDatabases,
 * CollisionReport), so this cannot disagree with `kb merge` or `kb import`
 * about what "the same row" means. The library does no file I/O for the dump:
 * the caller passes a reader (DR-0035).
 *
 * Parameters:
 *   r (io.Reader) — a JSONL stream as written by ExportJSONL.
 *
 * Returns:
 *   DBComparison — the per-table differences and a recommendation.
 *   error        — ErrInvalid for a malformed dump, or a database failure.
 *
 * Example:
 *   f, _ := os.Open("agents/knowledge.jsonl")
 *   defer f.Close()
 *   c, err := kb.CompareToJSONL(f)
 */
func (kb *KnowledgeBase) CompareToJSONL(r io.Reader) (DBComparison, error) {
	var c DBComparison
	dir, err := os.MkdirTemp("", "kbcheck-")
	if err != nil {
		return c, err
	}
	defer os.RemoveAll(dir)

	aPath := filepath.Join(dir, "a", "agents", "knowledge.db")
	if err := os.MkdirAll(filepath.Dir(aPath), 0o755); err != nil {
		return c, err
	}
	if _, err := kb.db.Exec(`VACUUM INTO ?`, aPath); err != nil {
		return c, err
	}

	bPath := filepath.Join(dir, "b", "agents", "knowledge.db")
	scratch, err := Open(bPath)
	if err != nil {
		return c, err
	}
	_, err = ImportJSONL(scratch, r)
	if cerr := scratch.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		var syn *json.SyntaxError
		var typ *json.UnmarshalTypeError
		if errors.As(err, &syn) || errors.As(err, &typ) {
			return c, invalidf("%w", err)
		}
		return c, err
	}

	if c.Tables, err = DiffDatabases(aPath, bPath); err != nil {
		return c, err
	}
	c.Collisions, err = CollisionReport(aPath, bPath)
	return c, err
}
