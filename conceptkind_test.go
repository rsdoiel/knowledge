package knowledge

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
)

// ─── concept kind (DR-0075 item 5, DR-0078 item 2) ───────────────────────────

func conceptKind(t *testing.T, kb *KnowledgeBase, name string) string {
	t.Helper()
	concepts, err := kb.Concepts()
	if err != nil {
		t.Fatalf("Concepts: %v", err)
	}
	for _, c := range concepts {
		if c.Name == name {
			return c.Kind
		}
	}
	t.Fatalf("concept %q not found", name)
	return ""
}

func TestConceptKind_DefaultsToOther(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("toast", ""); err != nil {
		t.Fatal(err)
	}
	if got := conceptKind(t, kb, "toast"); got != "other" {
		t.Errorf("Kind = %q, want other", got)
	}
}

func TestSetConceptKind_SetsAndReAddingDoesNotReset(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("the-long-fire/Anna", "")
	if err := kb.SetConceptKind("the-long-fire/Anna", "character"); err != nil {
		t.Fatalf("SetConceptKind: %v", err)
	}
	// Tagging the concept again from prose or a re-run of concept add must not
	// quietly turn a character back into "other".
	if _, err := kb.AddConcept("the-long-fire/Anna", ""); err != nil {
		t.Fatal(err)
	}
	if got := conceptKind(t, kb, "the-long-fire/Anna"); got != "character" {
		t.Errorf("Kind = %q, want character", got)
	}
}

func TestSetConceptKind_RefusesUnknownKindAndMissingConcept(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("toast", "")
	if err := kb.SetConceptKind("toast", "protagonist"); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown kind: err = %v, want ErrInvalid", err)
	}
	if got := conceptKind(t, kb, "toast"); got != "other" {
		t.Errorf("Kind = %q after a refused change, want other", got)
	}
	if err := kb.SetConceptKind("nothing", "place"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing concept: err = %v, want ErrNotFound", err)
	}
}

func TestSetConceptKind_BumpsUpdatedAt(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("toast", "")
	kb.db.Exec(`UPDATE concepts SET updated_at = '2000-01-01 00:00:00'`)
	if err := kb.SetConceptKind("toast", "theme"); err != nil {
		t.Fatal(err)
	}
	var at string
	kb.db.QueryRow(`SELECT updated_at FROM concepts WHERE name = 'toast'`).Scan(&at)
	if strings.HasPrefix(at, "2000") {
		t.Errorf("updated_at = %q, want it advanced so merge and import see the change", at)
	}
}

func TestConceptsOfKind_FiltersAndRefusesUnknown(t *testing.T) {
	kb := openTestKB(t)
	kb.AddConcept("Anna", "")
	kb.AddConcept("Kitchen", "")
	kb.AddConcept("Grief", "")
	kb.SetConceptKind("Anna", "character")
	kb.SetConceptKind("Kitchen", "place")
	got, err := kb.ConceptsOfKind("character")
	if err != nil || len(got) != 1 || got[0].Name != "Anna" {
		t.Errorf("ConceptsOfKind(character) = %v, %v", got, err)
	}
	other, _ := kb.ConceptsOfKind("other")
	if len(other) != 1 || other[0].Name != "Grief" {
		t.Errorf("ConceptsOfKind(other) = %v", other)
	}
	if _, err := kb.ConceptsOfKind("villain"); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown kind: err = %v, want ErrInvalid", err)
	}
}

func TestOpen_MigratesALegacyConceptsTableToKindOther(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/agents/knowledge.db"
	os.MkdirAll(dir+"/agents", 0o755)
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	raw.Exec(`CREATE TABLE concepts (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE NOT NULL, description TEXT)`)
	raw.Exec(`INSERT INTO concepts (name, description) VALUES ('old', 'before kinds')`)
	raw.Close()

	kb, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open legacy: %v", err)
	}
	defer kb.Close()
	if got := conceptKind(t, kb, "old"); got != "other" {
		t.Errorf("Kind = %q, want other", got)
	}
}

func TestJSONL_ConceptKindRoundTrips(t *testing.T) {
	src := openTestKB(t)
	src.AddConcept("Anna", "")
	src.SetConceptKind("Anna", "character")
	var buf bytes.Buffer
	if err := ExportJSONL(src, &buf, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"kind":"character"`) {
		t.Errorf("export lacks the kind: %s", buf.String())
	}
	dst := openTestKB(t)
	if _, err := ImportJSONL(dst, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	if got := conceptKind(t, dst, "Anna"); got != "character" {
		t.Errorf("imported Kind = %q, want character", got)
	}
}

func TestJSONL_ConceptKindNewerIncomingWinsOlderDoesNot(t *testing.T) {
	src := openTestKB(t)
	src.AddConcept("shared", "")
	src.SetConceptKind("shared", "place")
	dst := openTestKB(t)
	dst.AddConcept("shared", "")
	syncConceptUUID(t, src, dst, "shared")

	var buf bytes.Buffer
	ExportJSONL(src, &buf, "")

	dst.db.Exec(`UPDATE concepts SET updated_at = '2000-01-01 00:00:00'`)
	if _, err := ImportJSONL(dst, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	if got := conceptKind(t, dst, "shared"); got != "place" {
		t.Errorf("newer incoming: Kind = %q, want place", got)
	}

	dst.SetConceptKind("shared", "theme") // now newer than the export
	if _, err := ImportJSONL(dst, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	if got := conceptKind(t, dst, "shared"); got != "theme" {
		t.Errorf("older incoming: Kind = %q, want local theme kept", got)
	}
}

func TestJSONL_ConceptWithoutKindFromAnOlderExportKeepsLocalKind(t *testing.T) {
	dst := openTestKB(t)
	dst.AddConcept("Anna", "")
	dst.SetConceptKind("Anna", "character")
	var uuid string
	dst.db.QueryRow(`SELECT uuid FROM concepts WHERE name = 'Anna'`).Scan(&uuid)
	// An export written before kinds existed has no "kind" field at all.
	old := `{"type":"concept","uuid":"` + uuid + `","origin_host":"h","name":"Anna","description":"newer text",` +
		`"identifier_type":"","identifier_value":"","created_at":"2000-01-01 00:00:00","updated_at":"2999-01-01 00:00:00"}` + "\n"
	if _, err := ImportJSONL(dst, strings.NewReader(old)); err != nil {
		t.Fatal(err)
	}
	if got := conceptKind(t, dst, "Anna"); got != "character" {
		t.Errorf("Kind = %q, want character: an export with no kind says nothing about it", got)
	}
	// And a new concept from such an export is "other".
	fresh := strings.Replace(strings.Replace(old, uuid, "01a14a9a-0000-7000-8000-000000000001", 1), `"Anna"`, `"Bram"`, 1)
	if _, err := ImportJSONL(dst, strings.NewReader(fresh)); err != nil {
		t.Fatal(err)
	}
	if got := conceptKind(t, dst, "Bram"); got != "other" {
		t.Errorf("new concept Kind = %q, want other", got)
	}
}

func TestMerge_ConceptKindTravelsBothWays(t *testing.T) {
	a := openTestKB(t)
	b := openTestKB(t)
	a.AddConcept("shared", "")
	b.AddConcept("shared", "")
	syncConceptUUID(t, a, b, "shared")
	a.db.Exec(`UPDATE concepts SET updated_at = '2000-01-01 00:00:00'`)
	b.SetConceptKind("shared", "place") // b is newer: its kind wins on the uuid-conflict update
	b.AddConcept("only-b", "")
	b.SetConceptKind("only-b", "character") // arrives by the insert path

	merged := openMergedTestKB(t, a, b)
	if got := conceptKind(t, merged, "shared"); got != "place" {
		t.Errorf("conflict update: Kind = %q, want place", got)
	}
	if got := conceptKind(t, merged, "only-b"); got != "character" {
		t.Errorf("insert: Kind = %q, want character", got)
	}
}

func TestDiffDatabases_ConceptKindIsADifference(t *testing.T) {
	a := diffFixture(t, "a")
	b := twin(t, a)
	mutate(t, b, func(kb *KnowledgeBase) {
		if err := kb.SetConceptKind("toast", "theme"); err != nil {
			t.Fatal(err)
		}
	})
	d := diffByTable(t, a, b)["concepts"]
	if len(d.Different) != 1 || len(d.OnlyA)+len(d.OnlyB) != 0 {
		t.Errorf("concepts diff = %+v, want exactly one different row", d)
	}
}
