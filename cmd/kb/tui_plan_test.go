package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	knowledge "github.com/rsdoiel/knowledge"
)

// Plan-then-apply (knowledge DR-0068, v0.0.20 U4). Ingest, record fuzzy-tag and
// the document verbs that edit files each have a dry run. The TUI shows it first,
// scrollable, and only y applies it; n, q and Esc leave with nothing done. The
// plan and the write are the command's own functions.

// planModel is a menu model over a workspace with: a proposed record that mentions
// "streeming" twice, a concept "streaming", and an ingested document that talks
// about streaming. KB_PROJECT=clasm.
func planModel(t *testing.T) (*tuiModel, string, *knowledge.KnowledgeBase) {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("KB_PROJECT", "")
	kb, root := openWorkspaceKB(t)
	dir := filepath.Join(root, "agents", "projects", "clasm", "decisions")
	testRecord{ID: "0001", Project: "clasm", Title: "A decision", Status: "proposed", Trigger: "design",
		Body: "\nWe rely on streeming for the events, and streeming again.\n"}.write(t, dir)
	runIngest(t, kb, dir)
	if _, err := kb.AddConcept("streaming", "SSE"); err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(root, "docs", "note.md")
	if err := os.MkdirAll(filepath.Dir(doc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doc, []byte("# A note\n\nWe talk about streaming here, and streaming again.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdDocument(kb, nil, false, []string{"ingest", doc, "--project", "clasm"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KB_PROJECT", "clasm")
	m, err := newTUIModel(kb, nil)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return next.(*tuiModel), root, kb
}

// ─── ingest ──────────────────────────────────────────────────────────────────

func addRecordFile(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "agents", "projects", "clasm", "decisions")
	testRecord{ID: "0002", Project: "clasm", Title: "A new one", Status: "accepted", Trigger: "design",
		Body: "\nIt uses [[Fresh]] things.\n"}.write(t, dir)
}

func TestPlan_IngestShowsTheDryRunAndOnlyYApplies(t *testing.T) {
	m, root, kb := planModel(t)
	addRecordFile(t, root)
	m = openRow(t, m, "Ingest")
	if m.state != viewPlan {
		t.Fatalf("state = %s, want the plan", viewStateNames[m.state])
	}
	v := m.View()
	for _, want := range []string{"dry run", "1 added", "concept: would create Fresh", "arrived accepted", "y apply"} {
		if !strings.Contains(v, want) {
			t.Errorf("the plan lacks %q:\n%s", want, v)
		}
	}
	if recs, _ := kb.ListRecords(knowledge.RecordFilter{}); len(recs) != 1 {
		t.Fatalf("the plan wrote: %d records in the database", len(recs))
	}
	if ok, _ := kb.HasConcept("Fresh"); ok {
		t.Fatal("the plan created a concept")
	}
	m, _ = press(t, m, ch('y'))
	if recs, _ := kb.ListRecords(knowledge.RecordFilter{}); len(recs) != 2 {
		t.Errorf("after y: %d records, want 2", len(recs))
	}
	if ok, _ := kb.HasConcept("Fresh"); !ok {
		t.Error("the concept was not created")
	}
	if m.state != viewText {
		t.Fatalf("state = %s, want the result", viewStateNames[m.state])
	}
	v = m.View()
	for _, want := range []string{"1 added", "concept: created Fresh", "equivalent:", "kb ingest agents/projects/clasm/decisions"} {
		if !strings.Contains(v, want) {
			t.Errorf("the result lacks %q:\n%s", want, v)
		}
	}
	m, _ = press(t, m, ch('q'))
	if m.state != viewMenu {
		t.Errorf("q from the result gave %s, want the menu", viewStateNames[m.state])
	}
}

func TestPlan_EveryCancelAppliesNothing(t *testing.T) {
	for name, key := range map[string]tea.Msg{"n": ch('n'), "q": ch('q'), "Esc": keyEsc} {
		t.Run(name, func(t *testing.T) {
			m, root, kb := planModel(t)
			addRecordFile(t, root)
			m = openRow(t, m, "Ingest")
			m, cmd := press(t, m, key)
			if isQuit(cmd) || m.state != viewMenu {
				t.Fatalf("state %s (quit %v), want the menu", viewStateNames[m.state], isQuit(cmd))
			}
			if recs, _ := kb.ListRecords(knowledge.RecordFilter{}); len(recs) != 1 {
				t.Errorf("a cancel applied: %d records", len(recs))
			}
			if v := m.View(); !strings.Contains(v, "cancelled") || strings.Contains(v, "equivalent:") {
				t.Errorf("the screen should say cancelled and show no command:\n%s", v)
			}
		})
	}
}

func TestPlan_OnlyYApplies(t *testing.T) {
	m, root, kb := planModel(t)
	addRecordFile(t, root)
	m = openRow(t, m, "Ingest")
	m, _ = press(t, m, keyEnter, ch('j'), ch('a'), tea.KeyMsg{Type: tea.KeySpace}) // nothing here applies
	if m.state != viewPlan {
		t.Fatalf("state = %s; a key that is not y must not leave the plan", viewStateNames[m.state])
	}
	if recs, _ := kb.ListRecords(knowledge.RecordFilter{}); len(recs) != 1 {
		t.Errorf("something applied: %d records", len(recs))
	}
	if _, cmd := press(t, m, keyCtrlC); !isQuit(cmd) {
		t.Error("Ctrl-C did not quit")
	}
}

// ─── record fuzzy-tag ────────────────────────────────────────────────────────

func TestPlan_RecordFuzzyTagReportsThenWrites(t *testing.T) {
	m, root, _ := planModel(t)
	file := filepath.Join(root, "agents", "projects", "clasm", "decisions", "0001-fixture.md")
	before, _ := os.ReadFile(file)
	m = openLeaf(t, m, "Records", "Fuzzy-tag…")
	if m.state != viewPlan {
		t.Fatalf("state = %s, want the plan", viewStateNames[m.state])
	}
	v := m.View()
	for _, want := range []string{"streaming ~ streeming", "nothing was written", "y apply"} {
		if !strings.Contains(v, want) {
			t.Errorf("the plan lacks %q:\n%s", want, v)
		}
	}
	if after, _ := os.ReadFile(file); string(after) != string(before) {
		t.Fatal("the plan wrote")
	}
	m, _ = press(t, m, ch('y'))
	after, _ := os.ReadFile(file)
	if !strings.Contains(string(after), "tags: [streaming]") {
		t.Errorf("the tag was not written:\n%s", after)
	}
	v = m.View()
	for _, want := range []string{"tagged 1 record", "kb ingest", "equivalent:  kb record fuzzy-tag --project clasm --write"} {
		if !strings.Contains(v, want) {
			t.Errorf("the result lacks %q:\n%s", want, v)
		}
	}
}

// With every scope the project is asked first, then the plan.
func TestPlan_WithEveryScopeTheProjectIsAskedFirst(t *testing.T) {
	m, _, _ := planModel(t)
	t.Setenv("KB_PROJECT", "")
	m = openRow(t, m, "Records")
	m, _ = press(t, m, ch('a')) // every scope
	for i := 0; i < 4; i++ {
		m, _ = press(t, m, ch('j'))
	}
	m, _ = press(t, m, keyEnter) // Fuzzy-tag…
	if m.state != viewForm || !strings.Contains(m.View(), "project") {
		t.Fatalf("state %s; the project should be asked:\n%s", viewStateNames[m.state], m.View())
	}
	m, _ = press(t, m, append(typed("nosuch"), keyEnter)...)
	if m.state != viewForm || !strings.Contains(m.View(), "no project") {
		t.Errorf("an unknown project should be refused in the field:\n%s", m.View())
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m, _ = press(t, m, append(typed("clasm"), keyEnter)...)
	if m.state != viewPlan || !strings.Contains(m.View(), "streeming") {
		t.Errorf("state %s; want the plan for clasm:\n%s", viewStateNames[m.state], m.View())
	}
}

// ─── documents ───────────────────────────────────────────────────────────────

func TestPlan_DocumentTagPlansThenEditsTheFile(t *testing.T) {
	m, root, _ := planModel(t)
	doc := filepath.Join(root, "docs", "note.md")
	m = openLeaf(t, m, "Documents", "Tag…")
	if m.state != viewPlan {
		t.Fatalf("state = %s, want the plan", viewStateNames[m.state])
	}
	if v := m.View(); !strings.Contains(v, "would tag") || !strings.Contains(v, "streaming") {
		t.Errorf("the plan lacks the tag it would insert:\n%s", v)
	}
	if b, _ := os.ReadFile(doc); strings.Contains(string(b), "[[streaming]]") {
		t.Fatal("the plan wrote")
	}
	m, _ = press(t, m, ch('y'))
	if b, _ := os.ReadFile(doc); !strings.Contains(string(b), "[[streaming]]") {
		t.Errorf("the wikilink was not inserted:\n%s", b)
	}
	if !strings.Contains(m.View(), "equivalent:  kb document tag --project clasm") {
		t.Errorf("the equivalent command is missing:\n%s", m.View())
	}
}

func TestPlan_DocumentFuzzyTagSaysWhenThereIsNothingToDo(t *testing.T) {
	m, _, _ := planModel(t)
	m = openLeaf(t, m, "Documents", "Fuzzy-tag…")
	if m.state != viewPlan || !strings.Contains(m.View(), "no changes") {
		t.Errorf("state %s; the plan should say there is nothing to do:\n%s", viewStateNames[m.state], m.View())
	}
}

func TestPlan_DocumentIngestIsAFormThenAPlan(t *testing.T) {
	m, root, kb := planModel(t)
	fresh := filepath.Join(root, "docs", "second.md")
	if err := os.WriteFile(fresh, []byte("# Second\n\nA second note about nothing.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = openLeaf(t, m, "Documents", "Ingest a document…")
	if m.state != viewForm || !strings.Contains(m.View(), "path") {
		t.Fatalf("state %s; the path should be asked:\n%s", viewStateNames[m.state], m.View())
	}
	m, _ = press(t, m, append(typed(fresh), keyEnter)...)
	m, _ = press(t, m, keyEnter) // project: the scope's clasm
	m, _ = press(t, m, keyEnter) // title: none
	if m.state != viewPlan || !strings.Contains(m.View(), "added") {
		t.Fatalf("state %s; want the plan:\n%s", viewStateNames[m.state], m.View())
	}
	docs, _ := kb.Documents(0)
	if len(docs) != 1 {
		t.Fatalf("the plan ingested: %d documents", len(docs))
	}
	m, _ = press(t, m, ch('y'))
	if docs, _ = kb.Documents(0); len(docs) != 2 {
		t.Errorf("after y: %d documents, want 2", len(docs))
	}
}

// ─── the table and the legend ────────────────────────────────────────────────

func TestPlan_TheRowsAreBuiltAndFrontmatterIsNot(t *testing.T) {
	built := map[string]bool{}
	for _, it := range groupMenu("document") {
		built[it.Label] = it.Built
	}
	for label, want := range map[string]bool{"Ingest a document…": true, "Tag…": true, "Fuzzy-tag…": true, "Frontmatter…": false} {
		if built[label] != want {
			t.Errorf("Documents → %s built = %v, want %v", label, built[label], want)
		}
	}
	for _, it := range topMenu() {
		if it.Label == "Ingest" && !it.Built {
			t.Error("Ingest is not built")
		}
	}
	for _, it := range groupMenu("record") {
		if it.Label == "Fuzzy-tag…" && !it.Built {
			t.Error("Records → Fuzzy-tag… is not built")
		}
	}
}

// ─── the real binary ─────────────────────────────────────────────────────────

func TestBinary_TUIIngestPlanThenApplyOnATerminal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	builtKB(t)
	_, root, kb := planModel(t)
	addRecordFile(t, root)
	master, screen, wait := startOnATerminal(t, root, "-i")
	screen.waitFor(t, "bring record files into the database")
	master.WriteString("jjjjjjj\r") // Ingest is the eighth row
	screen.waitFor(t, "concept: would create Fresh")
	if ok, _ := kb.HasConcept("Fresh"); ok {
		t.Fatal("the plan created the concept")
	}
	master.WriteString("y")
	screen.waitFor(t, "concept: created Fresh")
	if ok, _ := kb.HasConcept("Fresh"); !ok {
		t.Error("the apply did not create the concept")
	}
	master.WriteString("qq")
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0:\n%s", code, screen.text())
	}
}
