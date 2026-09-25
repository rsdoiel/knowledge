package knowledge

import "testing"

// A project, concept or observation with an empty or whitespace-only name or
// body used to be stored: `kb project add ''` created a nameless project that
// then showed in every list. The guard lives in the library, not the CLI, so
// every caller (ingest, harvey, a script) gets it.

var blankInputs = []string{"", " ", "  ", "\t", "\n", " \t\n "}

func countRows(t *testing.T, kb *KnowledgeBase, table string) int {
	t.Helper()
	var n int
	if err := kb.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestAddProject_BlankNameRejected(t *testing.T) {
	kb := openTestKB(t)
	before := countRows(t, kb, "projects")
	for _, name := range blankInputs {
		if _, err := kb.AddProject(name, "d"); err == nil {
			t.Errorf("AddProject(%q) succeeded, want an error", name)
		}
	}
	if got := countRows(t, kb, "projects"); got != before {
		t.Errorf("projects rows = %d, want %d (a rejected add must not insert)", got, before)
	}
}

func TestAddProjectWithStatus_BlankNameRejected(t *testing.T) {
	kb := openTestKB(t)
	before := countRows(t, kb, "projects")
	for _, name := range blankInputs {
		if _, err := kb.AddProjectWithStatus(name, "d", "active"); err == nil {
			t.Errorf("AddProjectWithStatus(%q) succeeded, want an error", name)
		}
	}
	if got := countRows(t, kb, "projects"); got != before {
		t.Errorf("projects rows = %d, want %d", got, before)
	}
}

func TestAddProject_TrimsSurroundingWhitespace(t *testing.T) {
	kb := openTestKB(t)
	want, err := kb.AddProject("harvey", "d")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	got, err := kb.AddProject("  harvey\t", "")
	if err != nil {
		t.Fatalf("AddProject(padded): %v", err)
	}
	if got != want {
		t.Errorf("padded name gave id %d, want the existing project's id %d", got, want)
	}
	if n := countRows(t, kb, "projects"); n != 1 {
		t.Errorf("projects rows = %d, want 1 (no padded duplicate)", n)
	}
	id, err := kb.AddProject(" fresh ", "")
	if err != nil {
		t.Fatalf("AddProject(fresh): %v", err)
	}
	p, err := kb.projectByID(id)
	if err != nil {
		t.Fatalf("projectByID: %v", err)
	}
	if p.Name != "fresh" {
		t.Errorf("stored name = %q, want %q", p.Name, "fresh")
	}
}

func TestAddConcept_BlankNameRejected(t *testing.T) {
	kb := openTestKB(t)
	before := countRows(t, kb, "concepts")
	for _, name := range blankInputs {
		if _, err := kb.AddConcept(name, ""); err == nil {
			t.Errorf("AddConcept(%q) succeeded, want an error", name)
		}
		if _, err := kb.AddConceptWithIdentifier(name, "", "", ""); err == nil {
			t.Errorf("AddConceptWithIdentifier(%q) succeeded, want an error", name)
		}
	}
	if got := countRows(t, kb, "concepts"); got != before {
		t.Errorf("concepts rows = %d, want %d", got, before)
	}
}

func TestAddConcept_TrimsSurroundingWhitespace(t *testing.T) {
	kb := openTestKB(t)
	want, err := kb.AddConcept("chunking", "")
	if err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	got, err := kb.AddConcept(" chunking ", "")
	if err != nil {
		t.Fatalf("AddConcept(padded): %v", err)
	}
	if got != want {
		t.Errorf("padded name gave id %d, want %d", got, want)
	}
	if n := countRows(t, kb, "concepts"); n != 1 {
		t.Errorf("concepts rows = %d, want 1", n)
	}
}

func TestResolveConceptName_BlankRejected(t *testing.T) {
	kb := openTestKB(t)
	before := countRows(t, kb, "concepts")
	for _, name := range blankInputs {
		if _, err := kb.ResolveConceptName(name); err == nil {
			t.Errorf("ResolveConceptName(%q) succeeded, want an error", name)
		}
	}
	if got := countRows(t, kb, "concepts"); got != before {
		t.Errorf("concepts rows = %d, want %d", got, before)
	}
}

func TestResolveConceptName_PaddedNameMatchesExisting(t *testing.T) {
	kb := openTestKB(t)
	want, err := kb.AddConcept("Foo", "")
	if err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	got, err := kb.ResolveConceptName("  foo ")
	if err != nil {
		t.Fatalf("ResolveConceptName: %v", err)
	}
	if got != want {
		t.Errorf("padded, differently-cased name gave id %d, want %d", got, want)
	}
	if n := countRows(t, kb, "concepts"); n != 1 {
		t.Errorf("concepts rows = %d, want 1", n)
	}
}

func TestAddObservation_BlankBodyRejected(t *testing.T) {
	kb := openTestKB(t)
	pid, err := kb.AddProject("p", "")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	before := countRows(t, kb, "observations")
	for _, body := range blankInputs {
		if _, err := kb.AddObservation(pid, "note", body); err == nil {
			t.Errorf("AddObservation(%q) succeeded, want an error", body)
		}
		if _, err := kb.AddObservationWithSource(pid, "note", body, ""); err == nil {
			t.Errorf("AddObservationWithSource(%q) succeeded, want an error", body)
		}
	}
	if got := countRows(t, kb, "observations"); got != before {
		t.Errorf("observations rows = %d, want %d", got, before)
	}
}

// Only a blank body is refused. A real body is stored exactly as given,
// surrounding whitespace included: an observation's formatting is its own.
func TestAddObservation_BodyStoredUntrimmed(t *testing.T) {
	kb := openTestKB(t)
	pid, err := kb.AddProject("p", "")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	const body = "  indented first line\nsecond line\n"
	id, err := kb.AddObservation(pid, "note", body)
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}
	var got string
	if err := kb.db.QueryRow(`SELECT body FROM observations WHERE id = ?`, id).Scan(&got); err != nil {
		t.Fatalf("select body: %v", err)
	}
	if got != body {
		t.Errorf("stored body = %q, want %q", got, body)
	}
}

func TestRenameProject_BlankNewNameRejected(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("old", "d"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	for _, name := range blankInputs {
		if err := kb.RenameProject("old", name); err == nil {
			t.Errorf("RenameProject(old, %q) succeeded, want an error", name)
		}
		if err := kb.RenameProjectRow("old", name); err == nil {
			t.Errorf("RenameProjectRow(old, %q) succeeded, want an error", name)
		}
	}
	p, err := kb.ProjectByName("old")
	if err != nil || p == nil {
		t.Errorf("project \"old\" should be untouched by rejected renames: %v", err)
	}
}

func TestRenameProject_TrimsNewName(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("old", "d"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if err := kb.RenameProject("old", "  new "); err != nil {
		t.Fatalf("RenameProject: %v", err)
	}
	if p, err := kb.ProjectByName("new"); err != nil || p == nil {
		t.Errorf("ProjectByName(new) = %v, %v; want the renamed project under the trimmed name", p, err)
	}
}

func TestRenameConcept_BlankNewNameRejected(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("old", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	for _, name := range blankInputs {
		if err := kb.RenameConcept("old", name); err == nil {
			t.Errorf("RenameConcept(old, %q) succeeded, want an error", name)
		}
	}
	var n int
	if err := kb.db.QueryRow(`SELECT COUNT(*) FROM concepts WHERE name = 'old'`).Scan(&n); err != nil || n != 1 {
		t.Errorf("concept \"old\" count = %d (%v), want 1: rejected renames must not touch it", n, err)
	}
}

func TestRenameConcept_TrimsNewName(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("old", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	if err := kb.RenameConcept("old", " new "); err != nil {
		t.Fatalf("RenameConcept: %v", err)
	}
	var n int
	if err := kb.db.QueryRow(`SELECT COUNT(*) FROM concepts WHERE name = 'new'`).Scan(&n); err != nil || n != 1 {
		t.Errorf("concept \"new\" count = %d (%v), want 1", n, err)
	}
}
