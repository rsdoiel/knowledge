package knowledge

import (
	"database/sql"
	"fmt"
	"os"
)

/** IdentityCollision is one entity that exists independently in both source
 * databases under two different uuids — almost certainly the same real-world
 * thing, created on two machines before either knew about the other, and now
 * indistinguishable except by what identifies it.
 *
 * What identifies it differs by table. For projects and concepts it is the
 * name, which a UNIQUE constraint already enforces. For records it is the
 * identity DR-0011 defines: workspace, project, scope and record id.
 *
 * Example:
 *   collisions, _ := CollisionReport("a.db", "b.db")
 *   for _, c := range collisions {
 *       fmt.Printf("%s %q: %s vs %s\n", c.Table, c.Label, c.UUIDA, c.UUIDB)
 *   }
 */
type IdentityCollision struct {
	Table string // "projects", "concepts" or "records"
	Label string // the colliding name, or "DR-0007" for a record
	UUIDA string
	UUIDB string
}

// collisionQueries gives, per table, the query that finds rows the two
// databases agree are the same entity but disagree about the uuid of.
//
// The records query is not the identity index restated. That index is
// (workspace, project_id, scope, record_id), and project_id is a local
// autoincrement key: comparing a.records.project_id with b.records.project_id
// compares two unrelated id sequences, so a's harvey and b's antenna would
// match whenever they happened to land on the same number. Across databases
// the project is identified by its name, which is what the projects collision
// machinery already treats as the stable cross-machine key.
//
// Matching on the project's *uuid* would be the other candidate and is worse:
// two machines' independently created projects have different uuids until
// -force reconciles them, so record collisions would appear only after the
// project collision was resolved, and the same pair of databases would report
// different collisions before and after. Name is order-independent.
var collisionQueries = map[string]string{
	"projects": `SELECT main.projects.name, main.projects.uuid, b.projects.uuid
	             FROM main.projects JOIN b.projects USING(name)
	             WHERE main.projects.uuid != b.projects.uuid`,
	"concepts": `SELECT main.concepts.name, main.concepts.uuid, b.concepts.uuid
	             FROM main.concepts JOIN b.concepts USING(name)
	             WHERE main.concepts.uuid != b.concepts.uuid`,
	"records": `SELECT 'DR-' || mr.record_id, mr.uuid, br.uuid
	            FROM main.records mr
	            LEFT JOIN main.projects mp ON mp.id = mr.project_id
	            JOIN b.records br
	              ON br.workspace = mr.workspace
	             AND br.scope     = mr.scope
	             AND br.record_id = mr.record_id
	            LEFT JOIN b.projects bp ON bp.id = br.project_id
	            WHERE IFNULL(mp.name, '') = IFNULL(bp.name, '')
	              AND mr.uuid != br.uuid`,
}

// collisionTables fixes the order collisions are reported in, so output is
// stable rather than map-iteration order.
var collisionTables = []string{"projects", "concepts", "records"}

/** CollisionReport opens aPath and bPath read-only and reports every entity
 * that exists in both under different uuids. Callers should review (and
 * resolve, via ReconcileCollisions) any collisions before calling
 * MergeKnowledgeBases — an unresolved collision is settled "first insert wins"
 * by MergeKnowledgeBases's INSERT OR IGNORE, which drops the losing row and
 * orphans every child row that pointed at it.
 *
 * Both databases must already carry the current schema; see NormalizeForMerge.
 *
 * Parameters:
 *   aPath (string) — path to the first knowledge.db.
 *   bPath (string) — path to the second knowledge.db.
 *
 * Returns:
 *   []IdentityCollision — one entry per colliding entity; empty if none found.
 *   error                — on database failure.
 *
 * Example:
 *   collisions, err := CollisionReport("/machine-a/knowledge.db", "/machine-b/knowledge.db")
 */
func CollisionReport(aPath, bPath string) ([]IdentityCollision, error) {
	db, err := sql.Open("sqlite", aPath)
	if err != nil {
		return nil, fmt.Errorf("knowledge: open %s: %w", aPath, err)
	}
	defer db.Close()
	if _, err := db.Exec(`ATTACH DATABASE ? AS b`, bPath); err != nil {
		return nil, fmt.Errorf("knowledge: attach %s: %w", bPath, err)
	}

	var out []IdentityCollision
	for _, table := range collisionTables {
		rows, err := db.Query(collisionQueries[table])
		if err != nil {
			return nil, fmt.Errorf("knowledge: collision query on %s: %w", table, err)
		}
		for rows.Next() {
			c := IdentityCollision{Table: table}
			if err := rows.Scan(&c.Label, &c.UUIDA, &c.UUIDB); err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, c)
		}
		rows.Close()
	}
	return out, nil
}

/** ContentDivergence is one record the two databases agree is the same record
 * — same workspace, project, scope and id — but whose text differs, detected
 * by comparing the checksums ingest stores over the file's bytes.
 *
 * This is reported apart from an IdentityCollision and does not stop a merge.
 * A decision record's text is its artifact in a way a project's description is
 * not, so the operator needs to know which record's prose was dropped in order
 * to reconcile the two Markdown files; but the merge's conflict rule is
 * unchanged, and the row already present still wins. See DR-0013.
 *
 * Example:
 *   divergences, _ := DivergenceReport("a.db", "b.db")
 *   for _, d := range divergences {
 *       fmt.Printf("%s differs: %s vs %s\n", d.Label, d.ChecksumA, d.ChecksumB)
 *   }
 */
type ContentDivergence struct {
	Table     string // always "records" today; only records carry a checksum
	Label     string // "DR-0007"
	ChecksumA string
	ChecksumB string
}

/** DivergenceReport opens aPath and bPath read-only and reports every record
 * the two agree on the identity of but disagree on the content of. Identity is
 * matched exactly as CollisionReport matches it, so the two reports describe
 * the same pairs of rows from different angles: a divergence may or may not
 * also be a collision, and either can occur alone.
 *
 * Parameters:
 *   aPath (string) — path to the first knowledge.db.
 *   bPath (string) — path to the second knowledge.db.
 *
 * Returns:
 *   []ContentDivergence — one entry per diverging record; empty if none.
 *   error                — on database failure.
 *
 * Example:
 *   divergences, err := DivergenceReport("/machine-a/knowledge.db", "/machine-b/knowledge.db")
 */
func DivergenceReport(aPath, bPath string) ([]ContentDivergence, error) {
	db, err := sql.Open("sqlite", aPath)
	if err != nil {
		return nil, fmt.Errorf("knowledge: open %s: %w", aPath, err)
	}
	defer db.Close()
	if _, err := db.Exec(`ATTACH DATABASE ? AS b`, bPath); err != nil {
		return nil, fmt.Errorf("knowledge: attach %s: %w", bPath, err)
	}

	rows, err := db.Query(`
		SELECT 'DR-' || mr.record_id, mr.checksum, br.checksum
		FROM main.records mr
		LEFT JOIN main.projects mp ON mp.id = mr.project_id
		JOIN b.records br
		  ON br.workspace = mr.workspace
		 AND br.scope     = mr.scope
		 AND br.record_id = mr.record_id
		LEFT JOIN b.projects bp ON bp.id = br.project_id
		WHERE IFNULL(mp.name, '') = IFNULL(bp.name, '')
		  AND mr.checksum != br.checksum`)
	if err != nil {
		return nil, fmt.Errorf("knowledge: divergence query: %w", err)
	}
	defer rows.Close()

	var out []ContentDivergence
	for rows.Next() {
		d := ContentDivergence{Table: "records"}
		if err := rows.Scan(&d.Label, &d.ChecksumA, &d.ChecksumB); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

/** ReconcileCollisions rewrites, in bPath, the uuid of every row named in
 * collisions to match its counterpart's uuid (UUIDA) — "a" wins. This must
 * be called (and MergeKnowledgeBases must be given the now-rewritten bPath)
 * before merging past a CollisionReport hit: without it, MergeKnowledgeBases
 * resolves a name collision by keeping whichever row it inserts first and
 * silently dropping the other via the uuid/name UNIQUE constraints — which
 * also orphans and drops every child row (observations, links) that pointed
 * at the dropped row through its uuid. Reconciling first means both sides'
 * child rows correctly attach to the single surviving merged parent instead.
 *
 * Parameters:
 *   bPath      (string)              — path to the knowledge.db whose colliding rows will be rewritten.
 *   collisions ([]IdentityCollision) — the result of CollisionReport(aPath, bPath).
 *
 * Returns:
 *   error — on database failure.
 *
 * Example:
 *   collisions, _ := CollisionReport(aPath, bPath)
 *   if len(collisions) > 0 {
 *       _ = ReconcileCollisions(bPath, collisions)
 *   }
 *   summary, _ := MergeKnowledgeBases(aPath, bPath, mergedPath)
 */
func ReconcileCollisions(bPath string, collisions []IdentityCollision) error {
	if len(collisions) == 0 {
		return nil
	}
	db, err := sql.Open("sqlite", bPath)
	if err != nil {
		return fmt.Errorf("knowledge: open %s: %w", bPath, err)
	}
	defer db.Close()
	for _, c := range collisions {
		// The uuid alone locates the row: every table carrying one has a
		// UNIQUE index over it. That is what lets this stay one statement as
		// identity grows from a name to a four-column tuple -- what identifies
		// the entity across the two databases is CollisionReport's problem,
		// and what identifies the row within b is always its uuid.
		if _, err := db.Exec(
			`UPDATE `+c.Table+` SET uuid = ? WHERE uuid = ?`,
			c.UUIDA, c.UUIDB,
		); err != nil {
			return fmt.Errorf("knowledge: reconcile %s %q: %w", c.Table, c.Label, err)
		}
	}
	return nil
}

/** NormalizeForMerge brings a merge scratch copy up to the current schema
 * before it is ATTACHed, so that a database predating a table can still be
 * merged instead of failing the query that names it. Merge copies its inputs
 * with a plain file copy rather than through Open, so nothing has ever
 * migrated them; `records` is the first table for which that matters, since a
 * database written before decision records existed has no such table and
 * SELECT ... FROM b.records against it is an error, not an empty result.
 *
 * The workspace name is derived from originalPath, never from scratchPath.
 * A record's workspace is the base name of its database's root (DR-0011), and
 * the scratch copy is the one place where the file is deliberately not where
 * it came from — deriving from the copy would backfill a temp directory's name
 * over every record that predates the workspace column. See DR-0014.
 *
 * Parameters:
 *   scratchPath  (string) — path to the throwaway copy to migrate in place.
 *   originalPath (string) — path the copy was taken from; supplies the
 *                           workspace name and is not opened or modified.
 *
 * Returns:
 *   error — if the copy cannot be opened or migrated.
 *
 * Example:
 *   _ = checkpointAndCopy(aPath, scratchA)
 *   if err := knowledge.NormalizeForMerge(scratchA, aPath); err != nil {
 *       return err
 *   }
 */
func NormalizeForMerge(scratchPath, originalPath string) error {
	kb, err := openWithWorkspace(scratchPath, workspaceFromDBPath(originalPath))
	if err != nil {
		return fmt.Errorf("knowledge: normalize %s for merge: %w", scratchPath, err)
	}
	return kb.Close()
}

/** MergeTableSummary reports, per table, how many rows came from each
 * source and how many rows the merged table ended up with (less than
 * FromA+FromB when uuid or name collisions caused an intentional drop).
 */
type MergeTableSummary struct {
	Table  string
	FromA  int
	FromB  int
	Merged int
}

/** MergeKnowledgeBases creates a fresh knowledge base at mergedPath (which
 * must not already exist) containing the set union of aPath and bPath,
 * deduped by uuid (and, for projects/concepts, by the pre-existing name
 * UNIQUE constraint, and for records by their four-column identity). aPath and
 * bPath are opened read-only via ATTACH; neither is modified.
 *
 * All nine tables that travel are carried: projects, concepts, sources,
 * observations and records, plus the four join tables. Every one appears in
 * the returned summary, so a table that loses rows says so — records were once
 * absent from both the merge and the summary, and a merge that dropped them
 * reported success (DR-0013).
 *
 * Both sources must already carry the current schema. This function names
 * every table directly, so a database predating one of them fails the query
 * rather than contributing nothing; callers copying a live database should run
 * NormalizeForMerge on the copy first, as cmd/kb's merge verb does (DR-0014).
 *
 * Parameters:
 *   aPath      (string) — path to the first source knowledge.db.
 *   bPath      (string) — path to the second source knowledge.db.
 *   mergedPath (string) — path for the new merged knowledge.db; must not exist.
 *
 * Returns:
 *   []MergeTableSummary — one entry per table, with each side's count and the merged count.
 *   error                — on database failure, or if mergedPath already exists.
 *
 * Example:
 *   summary, err := MergeKnowledgeBases("/machine-a/knowledge.db", "/machine-b/knowledge.db", "/tmp/merged.db")
 */
func MergeKnowledgeBases(aPath, bPath, mergedPath string) ([]MergeTableSummary, error) {
	if _, err := os.Stat(mergedPath); err == nil {
		return nil, fmt.Errorf("knowledge: merge target %s already exists", mergedPath)
	}

	kb, err := Open(mergedPath)
	if err != nil {
		return nil, err
	}
	kb.Close()

	db, err := sql.Open("sqlite", mergedPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if _, err := db.Exec(`ATTACH DATABASE ? AS a`, aPath); err != nil {
		return nil, fmt.Errorf("knowledge: attach %s: %w", aPath, err)
	}
	if _, err := db.Exec(`ATTACH DATABASE ? AS b`, bPath); err != nil {
		return nil, fmt.Errorf("knowledge: attach %s: %w", bPath, err)
	}

	parentCols := map[string]string{
		"projects": "name, description, status, created_at, updated_at, uuid, origin_host",
		"concepts": "name, description, created_at, identifier_type, identifier_value, uuid, origin_host",
		"sources":  "title, identifier_type, identifier_value, authors, published_date, publisher, rights, version, retracted, retraction_note, first_seen_at, last_checked_at, uuid, origin_host",
	}
	for _, table := range []string{"projects", "concepts", "sources"} {
		cols := parentCols[table]
		for _, src := range []string{"a", "b"} {
			if _, err := db.Exec(fmt.Sprintf(
				`INSERT OR IGNORE INTO %s (%s) SELECT %s FROM %s.%s`,
				table, cols, cols, src, table,
			)); err != nil {
				return nil, fmt.Errorf("knowledge: merge %s from %s: %w", table, src, err)
			}
		}
	}

	const obsCols = "kind, body, created_at, source_doi, uuid, origin_host"
	for _, src := range []string{"a", "b"} {
		if _, err := db.Exec(fmt.Sprintf(`
			INSERT OR IGNORE INTO observations (project_id, %s)
			SELECT mp.id, o.kind, o.body, o.created_at, o.source_doi, o.uuid, o.origin_host
			FROM %s.observations o
			JOIN %s.projects sp ON sp.id = o.project_id
			JOIN projects mp ON mp.uuid = sp.uuid`,
			obsCols, src, src,
		)); err != nil {
			return nil, fmt.Errorf("knowledge: merge observations from %s: %w", src, err)
		}
	}

	// Records hang off projects the way observations do, with one difference
	// that decides the join: records.project_id is nullable, because a
	// workspace-tier record has no project (DR-0011). The INNER JOIN the
	// observations pass uses would drop every one of them and leave a
	// plausible-looking count behind, so this is a LEFT JOIN. The WHERE clause
	// keeps that leniency from going too far: a record whose project_id is set
	// but unresolvable is skipped, as an observation would be, rather than
	// silently arriving as a workspace-tier record.
	const recordCols = `record_id, scope, path, title, date, status, kind,
		"trigger", phase, initiative, session, body, checksum, ingested_at,
		uuid, origin_host, workspace`
	for _, src := range []string{"a", "b"} {
		if _, err := db.Exec(fmt.Sprintf(`
			INSERT OR IGNORE INTO records (project_id, %s)
			SELECT mp.id, r.record_id, r.scope, r.path, r.title, r.date,
			       r.status, r.kind, r."trigger", r.phase, r.initiative,
			       r.session, r.body, r.checksum, r.ingested_at, r.uuid,
			       r.origin_host, r.workspace
			FROM %s.records r
			LEFT JOIN %s.projects sp ON sp.id = r.project_id
			LEFT JOIN projects mp ON mp.uuid = sp.uuid
			WHERE r.project_id IS NULL OR mp.id IS NOT NULL`,
			recordCols, src, src,
		)); err != nil {
			return nil, fmt.Errorf("knowledge: merge records from %s: %w", src, err)
		}
	}

	for _, src := range []string{"a", "b"} {
		if _, err := db.Exec(fmt.Sprintf(`
			INSERT OR IGNORE INTO record_relations (from_id, to_id, relationship)
			SELECT mf.id, mt.id, j.relationship
			FROM %s.record_relations j
			JOIN %s.records sf ON sf.id = j.from_id
			JOIN %s.records st ON st.id = j.to_id
			JOIN records mf ON mf.uuid = sf.uuid
			JOIN records mt ON mt.uuid = st.uuid`,
			src, src, src,
		)); err != nil {
			return nil, fmt.Errorf("knowledge: merge record_relations from %s: %w", src, err)
		}
	}

	for _, src := range []string{"a", "b"} {
		if _, err := db.Exec(fmt.Sprintf(`
			INSERT OR IGNORE INTO observation_concepts (observation_id, concept_id)
			SELECT mo.id, mc.id
			FROM %s.observation_concepts j
			JOIN %s.observations so ON so.id = j.observation_id
			JOIN %s.concepts     sc ON sc.id = j.concept_id
			JOIN observations mo ON mo.uuid = so.uuid
			JOIN concepts     mc ON mc.uuid = sc.uuid`,
			src, src, src,
		)); err != nil {
			return nil, fmt.Errorf("knowledge: merge observation_concepts from %s: %w", src, err)
		}

		if _, err := db.Exec(fmt.Sprintf(`
			INSERT OR IGNORE INTO project_concepts (project_id, concept_id)
			SELECT mp.id, mc.id
			FROM %s.project_concepts j
			JOIN %s.projects sp ON sp.id = j.project_id
			JOIN %s.concepts sc ON sc.id = j.concept_id
			JOIN projects mp ON mp.uuid = sp.uuid
			JOIN concepts mc ON mc.uuid = sc.uuid`,
			src, src, src,
		)); err != nil {
			return nil, fmt.Errorf("knowledge: merge project_concepts from %s: %w", src, err)
		}

		if _, err := db.Exec(fmt.Sprintf(`
			INSERT OR IGNORE INTO observation_sources (observation_id, source_id, relationship)
			SELECT mo.id, ms.id, j.relationship
			FROM %s.observation_sources j
			JOIN %s.observations so ON so.id = j.observation_id
			JOIN %s.sources      ss ON ss.id = j.source_id
			JOIN observations mo ON mo.uuid = so.uuid
			JOIN sources      ms ON ms.uuid = ss.uuid`,
			src, src, src,
		)); err != nil {
			return nil, fmt.Errorf("knowledge: merge observation_sources from %s: %w", src, err)
		}
	}

	// Every table that travels appears here. A table missing from this list
	// is a table whose loss is never reported: records were absent from both
	// the merge and this summary, so a merge that dropped them said nothing
	// about it (DR-0013).
	allTables := []string{
		"projects", "concepts", "sources", "observations", "records",
		"observation_concepts", "project_concepts", "observation_sources",
		"record_relations",
	}
	var summary []MergeTableSummary
	for _, table := range allTables {
		s := MergeTableSummary{Table: table}
		if err := db.QueryRow(`SELECT COUNT(*) FROM a.` + table).Scan(&s.FromA); err != nil {
			return nil, fmt.Errorf("knowledge: count a.%s: %w", table, err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM b.` + table).Scan(&s.FromB); err != nil {
			return nil, fmt.Errorf("knowledge: count b.%s: %w", table, err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&s.Merged); err != nil {
			return nil, fmt.Errorf("knowledge: count merged %s: %w", table, err)
		}
		summary = append(summary, s)
	}

	if _, err := db.Exec(`DETACH DATABASE a`); err != nil {
		return nil, err
	}
	if _, err := db.Exec(`DETACH DATABASE b`); err != nil {
		return nil, err
	}
	db.Close()

	kb2, err := Open(mergedPath)
	if err != nil {
		return nil, err
	}
	defer kb2.Close()

	return summary, nil
}
