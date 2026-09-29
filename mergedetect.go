package knowledge

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

/** CheckpointAndCopy checkpoints srcPath's WAL, so every committed page is in
 * the main file, and copies it, plus any -wal and -shm sidecars still present,
 * to dstPath. The checkpoint is the one thing it does to the source: it moves
 * committed pages into the main file and changes no row.
 *
 * Parameters:
 *   srcPath (string) — the database to copy.
 *   dstPath (string) — where the copy goes; overwritten if present.
 *
 * Returns:
 *   error — if the source cannot be opened, checkpointed or copied.
 *
 * Example:
 *   err := knowledge.CheckpointAndCopy("agents/knowledge.db", "/tmp/scratch/a.db")
 */
func CheckpointAndCopy(srcPath, dstPath string) error {
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
	if err := copyDatabaseFile(srcPath, dstPath); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := copyDatabaseFile(srcPath+suffix, dstPath+suffix); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func copyDatabaseFile(srcPath, dstPath string) error {
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

/** MergeScratch is a pair of throwaway copies of two knowledge bases, brought
 * up to the current schema, that detection and correction can work on without
 * touching the originals. It is the read-only front half of a merge, shared by
 * `kb merge` and `kb check-db` (DR-0053).
 *
 * Fields:
 *   A (string) — path of the copy of the first database.
 *   B (string) — path of the copy of the second database.
 *
 * Example:
 *   s, err := knowledge.PrepareMergeScratch("a.db", "b.db")
 *   if err != nil { return err }
 *   defer s.Close()
 */
type MergeScratch struct {
	A, B string
	dir  string
}

/** PrepareMergeScratch copies both databases into a fresh temporary directory
 * and brings each copy up to the current schema (NormalizeForMerge), so a
 * database predating a table can still be compared or merged. The originals
 * are opened only to checkpoint and copy them, and their rows are never
 * changed. The workspace name for each copy comes from the original path
 * (DR-0014).
 *
 * Parameters:
 *   aPath (string) — path to the first knowledge.db.
 *   bPath (string) — path to the second knowledge.db.
 *
 * Returns:
 *   *MergeScratch — the copies; call Close to remove them.
 *   error         — if either database cannot be copied or migrated; nothing is left behind.
 *
 * Example:
 *   s, err := knowledge.PrepareMergeScratch("a.db", "b.db")
 */
func PrepareMergeScratch(aPath, bPath string) (*MergeScratch, error) {
	dir, err := os.MkdirTemp("", "kbmerge-")
	if err != nil {
		return nil, err
	}
	s := &MergeScratch{A: filepath.Join(dir, "a.db"), B: filepath.Join(dir, "b.db"), dir: dir}
	for _, c := range []struct{ src, dst string }{{aPath, s.A}, {bPath, s.B}} {
		if err := CheckpointAndCopy(c.src, c.dst); err != nil {
			s.Close()
			return nil, fmt.Errorf("checkpoint+copy %s: %w", c.src, err)
		}
		if err := NormalizeForMerge(c.dst, c.src); err != nil {
			s.Close()
			return nil, err
		}
	}
	return s, nil
}

/** Close removes the scratch directory and everything in it.
 *
 * Returns:
 *   error — if the directory cannot be removed.
 *
 * Example:
 *   defer s.Close()
 */
func (s *MergeScratch) Close() error { return os.RemoveAll(s.dir) }

/** DetectIdentityIssues is the read-only detection half of a merge: the
 * entities both databases hold under different uuids, and the records both
 * hold with different text. It changes nothing; correction is
 * ReconcileCollisions and MergeKnowledgeBases. Both paths must already carry
 * the current schema (see PrepareMergeScratch).
 *
 * Parameters:
 *   aPath (string) — the first database (a scratch copy).
 *   bPath (string) — the second database (a scratch copy).
 *
 * Returns:
 *   []IdentityCollision — CollisionReport's result.
 *   []ContentDivergence — DivergenceReport's result.
 *   error               — on database failure.
 *
 * Example:
 *   collisions, divergences, err := knowledge.DetectIdentityIssues(s.A, s.B)
 */
func DetectIdentityIssues(aPath, bPath string) ([]IdentityCollision, []ContentDivergence, error) {
	collisions, err := CollisionReport(aPath, bPath)
	if err != nil {
		return nil, nil, fmt.Errorf("collision report: %w", err)
	}
	divergences, err := DivergenceReport(aPath, bPath)
	if err != nil {
		return nil, nil, fmt.Errorf("divergence report: %w", err)
	}
	return collisions, divergences, nil
}

/** RowDifference is one row present on both sides whose compared columns differ.
 *
 * Fields:
 *   Label (string) — a short human label for the row.
 *   A     (string) — the compared columns on the first side, joined with " | ", cut to 80 runes.
 *   B     (string) — the same on the second side.
 *
 * Example:
 *   for _, d := range diff.Different { fmt.Println(d.Label, d.A, "vs", d.B) }
 */
type RowDifference struct {
	Label string
	A, B  string
}

/** TableDiff is what differs in one table between two databases.
 *
 * Fields:
 *   Table     (string)          — the table name.
 *   CountA    (int)             — rows on the first side.
 *   CountB    (int)             — rows on the second side.
 *   OnlyA     ([]string)        — labels of rows only on the first side.
 *   OnlyB     ([]string)        — labels of rows only on the second side.
 *   Different ([]RowDifference) — rows on both sides whose compared columns differ.
 *
 * Example:
 *   fmt.Println(d.Table, len(d.OnlyA), len(d.OnlyB), len(d.Different))
 */
type TableDiff struct {
	Table     string
	CountA    int
	CountB    int
	OnlyA     []string
	OnlyB     []string
	Different []RowDifference
}

// Clean reports whether the table has no difference at all.
func (d TableDiff) Clean() bool {
	return len(d.OnlyA)+len(d.OnlyB)+len(d.Different) == 0
}

// rkey is the cross-database identity of a record: workspace, project name,
// scope and record id (DR-0018). project_id is a local autoincrement key and
// never compares across databases.
const rkey = `r.workspace || '/' || IFNULL(p.name, '') || '/' || r.scope || '/' || r.record_id`

// diffQueries gives, per table, one query returning (key, label, fingerprint):
// key is the row's identity across databases, label is for humans, fingerprint
// is the compared columns. Identity follows merge: uuid for observations,
// sources, documents, sections; name for projects and concepts; the record
// identity above for records; the pair of parent identities for links.
var diffQueries = []struct{ table, query string }{
	{"projects", `SELECT name, name, description || char(31) || status FROM projects`},
	{"concepts", `SELECT name, name, description || char(31) || identifier_type || char(31) || identifier_value FROM concepts`},
	{"sources", `SELECT uuid, title, title || char(31) || identifier_type || char(31) || identifier_value || char(31) || retracted FROM sources`},
	{"observations", `SELECT uuid, substr(body, 1, 60), kind || char(31) || body FROM observations`},
	{"records", `SELECT ` + rkey + `, 'DR-' || r.record_id || ' (' || IFNULL(p.name, 'workspace') || ')', r.checksum
		FROM records r LEFT JOIN projects p ON p.id = r.project_id`},
	{"observation_concepts", `SELECT o.uuid || '|' || c.name, substr(o.body, 1, 30) || ' -> ' || c.name, ''
		FROM observation_concepts j JOIN observations o ON o.id = j.observation_id JOIN concepts c ON c.id = j.concept_id`},
	{"project_concepts", `SELECT p.name || '|' || c.name, p.name || ' -> ' || c.name, ''
		FROM project_concepts j JOIN projects p ON p.id = j.project_id JOIN concepts c ON c.id = j.concept_id`},
	{"observation_sources", `SELECT o.uuid || '|' || s.uuid || '|' || j.relationship, substr(o.body, 1, 30) || ' -' || j.relationship || '-> ' || s.title, ''
		FROM observation_sources j JOIN observations o ON o.id = j.observation_id JOIN sources s ON s.id = j.source_id`},
	{"observation_relations", `SELECT f.uuid || '|' || t.uuid || '|' || j.relationship, substr(f.body, 1, 30) || ' -' || j.relationship || '-> ' || substr(t.body, 1, 30), ''
		FROM observation_relations j JOIN observations f ON f.id = j.from_id JOIN observations t ON t.id = j.to_id`},
	{"record_relations", `SELECT fk.k || '|' || tk.k || '|' || j.relationship, 'DR-' || fk.id || ' -' || j.relationship || '-> DR-' || tk.id, ''
		FROM record_relations j
		JOIN (SELECT r.id AS rid, ` + rkey + ` AS k, r.record_id AS id FROM records r LEFT JOIN projects p ON p.id = r.project_id) fk ON fk.rid = j.from_id
		JOIN (SELECT r.id AS rid, ` + rkey + ` AS k, r.record_id AS id FROM records r LEFT JOIN projects p ON p.id = r.project_id) tk ON tk.rid = j.to_id`},
	{"record_concepts", `SELECT ` + rkey + ` || '|' || c.name, 'DR-' || r.record_id || ' -> ' || c.name, ''
		FROM record_concepts j JOIN records r ON r.id = j.record_id LEFT JOIN projects p ON p.id = r.project_id JOIN concepts c ON c.id = j.concept_id`},
	{"documents", `SELECT uuid, title, checksum FROM documents`},
	{"document_sections", `SELECT uuid, heading, heading || char(31) || body || char(31) || summary_body || char(31) || summary_status FROM document_sections`},
	{"document_section_concepts", `SELECT s.uuid || '|' || c.name, s.heading || ' -> ' || c.name, ''
		FROM document_section_concepts j JOIN document_sections s ON s.id = j.section_id JOIN concepts c ON c.id = j.concept_id`},
}

type diffRow struct{ label, fp string }

func readDiffRows(path, query string) (map[string]diffRow, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("knowledge: open %s: %w", path, err)
	}
	defer db.Close()
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]diffRow{}
	for rows.Next() {
		var key, label, fp string
		if err := rows.Scan(&key, &label, &fp); err != nil {
			return nil, err
		}
		out[key] = diffRow{label, fp}
	}
	return out, rows.Err()
}

func shortFP(s string) string {
	s = strings.ReplaceAll(s, "\x1f", " | ")
	if r := []rune(s); len(r) > 80 {
		return string(r[:79]) + "…"
	}
	return s
}

/** DiffDatabases compares two knowledge bases table by table, over every table
 * that travels, and reports rows only on one side and rows on both sides whose
 * compared columns differ. It reads and changes nothing. It uses the same
 * notion of "the same row" as merge: name for projects and concepts, uuid for
 * observations, sources, documents and sections, the DR-0018 identity for
 * records, and the pair of parent identities for links.
 *
 * The compared columns are: projects description and status; concepts
 * description and identifier; sources title, identifier and retraction;
 * observations kind and body; records checksum; documents checksum; sections
 * heading, body and summary. Links have identity only.
 *
 * A row deleted on one side is indistinguishable from a row added on the
 * other: there are no tombstones.
 *
 * Both paths must carry the current schema (see PrepareMergeScratch).
 *
 * Parameters:
 *   aPath (string) — the first database.
 *   bPath (string) — the second database.
 *
 * Returns:
 *   []TableDiff — one per table, in a fixed order, clean tables included.
 *   error       — on database failure.
 *
 * Example:
 *   diffs, err := knowledge.DiffDatabases(s.A, s.B)
 */
func DiffDatabases(aPath, bPath string) ([]TableDiff, error) {
	var out []TableDiff
	for _, q := range diffQueries {
		a, err := readDiffRows(aPath, q.query)
		if err != nil {
			return nil, fmt.Errorf("knowledge: read %s from %s: %w", q.table, aPath, err)
		}
		b, err := readDiffRows(bPath, q.query)
		if err != nil {
			return nil, fmt.Errorf("knowledge: read %s from %s: %w", q.table, bPath, err)
		}
		d := TableDiff{Table: q.table, CountA: len(a), CountB: len(b)}
		for k, ra := range a {
			rb, ok := b[k]
			switch {
			case !ok:
				d.OnlyA = append(d.OnlyA, ra.label)
			case ra.fp != rb.fp:
				d.Different = append(d.Different, RowDifference{Label: ra.label, A: shortFP(ra.fp), B: shortFP(rb.fp)})
			}
		}
		for k, rb := range b {
			if _, ok := a[k]; !ok {
				d.OnlyB = append(d.OnlyB, rb.label)
			}
		}
		sort.Strings(d.OnlyA)
		sort.Strings(d.OnlyB)
		sort.Slice(d.Different, func(i, j int) bool { return d.Different[i].Label < d.Different[j].Label })
		out = append(out, d)
	}
	return out, nil
}
