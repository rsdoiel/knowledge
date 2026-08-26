package main

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	knowledge "github.com/rsdoiel/knowledge"
)

func newTestTUIModel(t *testing.T) *tuiModel {
	t.Helper()
	return newTestTUIModelWithDebugLog(t, nil)
}

func newTestTUIModelWithDebugLog(t *testing.T, dl *DebugLog) *tuiModel {
	t.Helper()
	kb := openTestKB(t)
	pid, err := kb.AddProject("alpha", "first project")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if _, err := kb.AddProject("beta", "second project"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if _, err := kb.AddObservation(pid, "note", "a specific observation body"); err != nil {
		t.Fatalf("AddObservation: %v", err)
	}
	cid, err := kb.AddConcept("streaming", "SSE streaming")
	if err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	if err := kb.LinkProjectConcept(pid, cid); err != nil {
		t.Fatalf("LinkProjectConcept: %v", err)
	}

	m, err := newTUIModel(kb, dl)
	if err != nil {
		t.Fatalf("newTUIModel: %v", err)
	}
	// Real programs receive a WindowSizeMsg before anything else; simulate
	// that so the child lists are sized (SetSize with 0,0 is otherwise
	// harmless, but this matches real usage more closely).
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return updated.(*tuiModel)
}

func key(k tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: k} }

func runeKey(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestTUIModel_StartsOnProjectList(t *testing.T) {
	m := newTestTUIModel(t)
	if m.state != viewProjects {
		t.Errorf("initial state = %v, want viewProjects", m.state)
	}
	if len(m.projectList.Items()) != 2 {
		t.Errorf("project list has %d items, want 2", len(m.projectList.Items()))
	}
}

func TestTUIModel_EnterDrillsIntoProject(t *testing.T) {
	m := newTestTUIModel(t)
	// The list is sorted by insertion in Projects(); "alpha" was added first.
	m.projectList.Select(0)

	updated, _ := m.Update(key(tea.KeyEnter))
	m = updated.(*tuiModel)

	if m.state != viewObservations {
		t.Fatalf("state after Enter = %v, want viewObservations", m.state)
	}
	if m.selectedProject == nil || m.selectedProject.Name != "alpha" {
		t.Fatalf("selectedProject = %+v, want alpha", m.selectedProject)
	}
	if len(m.observationList.Items()) != 1 {
		t.Errorf("observation list has %d items, want 1", len(m.observationList.Items()))
	}
}

func TestTUIModel_ToggleToConceptsAndBack(t *testing.T) {
	m := newTestTUIModel(t)
	m.projectList.Select(0)
	updated, _ := m.Update(key(tea.KeyEnter))
	m = updated.(*tuiModel)

	updated, _ = m.Update(runeKey("c"))
	m = updated.(*tuiModel)
	if m.state != viewConcepts {
		t.Fatalf("state after 'c' = %v, want viewConcepts", m.state)
	}
	if len(m.conceptList.Items()) != 1 {
		t.Errorf("concept list has %d items, want 1", len(m.conceptList.Items()))
	}

	updated, _ = m.Update(runeKey("o"))
	m = updated.(*tuiModel)
	if m.state != viewObservations {
		t.Errorf("state after 'o' = %v, want viewObservations", m.state)
	}
}

func TestTUIModel_EscNavigatesBackToProjects(t *testing.T) {
	m := newTestTUIModel(t)
	m.projectList.Select(0)
	updated, _ := m.Update(key(tea.KeyEnter))
	m = updated.(*tuiModel)

	updated, _ = m.Update(key(tea.KeyEsc))
	m = updated.(*tuiModel)
	if m.state != viewProjects {
		t.Errorf("state after Esc = %v, want viewProjects", m.state)
	}
}

func TestTUIModel_QQuitsFromProjectList(t *testing.T) {
	m := newTestTUIModel(t)
	_, cmd := m.Update(runeKey("q"))
	if cmd == nil {
		t.Fatal("expected a quit command from 'q', got nil")
	}
}

func TestTUIModel_SlashThenEnterRunsSearchAndShowsResults(t *testing.T) {
	m := newTestTUIModel(t)

	updated, _ := m.Update(runeKey("/"))
	m = updated.(*tuiModel)
	if !m.searching {
		t.Fatal("expected searching=true after '/'")
	}

	for _, r := range "specific" {
		updated, _ = m.Update(runeKey(string(r)))
		m = updated.(*tuiModel)
	}

	updated, _ = m.Update(key(tea.KeyEnter))
	m = updated.(*tuiModel)
	if m.searching {
		t.Error("expected searching=false after Enter")
	}
	if m.state != viewSearch {
		t.Fatalf("state after search Enter = %v, want viewSearch", m.state)
	}
	if len(m.searchList.Items()) == 0 {
		t.Error("expected at least one search result for 'specific'")
	}
}

func TestTUIModel_EscCancelsSearchInput(t *testing.T) {
	m := newTestTUIModel(t)
	updated, _ := m.Update(runeKey("/"))
	m = updated.(*tuiModel)

	updated, _ = m.Update(key(tea.KeyEsc))
	m = updated.(*tuiModel)
	if m.searching {
		t.Error("expected searching=false after Esc cancels the search prompt")
	}
	if m.state != viewProjects {
		t.Errorf("state after cancelling search = %v, want viewProjects (unchanged)", m.state)
	}
}

func TestTUIModel_LogsKeyMsgEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.jsonl")
	dl, err := NewDebugLog(path)
	if err != nil {
		t.Fatalf("NewDebugLog: %v", err)
	}
	m := newTestTUIModelWithDebugLog(t, dl)

	updated, _ := m.Update(runeKey("q"))
	_ = updated
	dl.Close()

	records := readJSONLLines(t, path)
	found := false
	for _, r := range records {
		if r["event"] == "tui_msg" && r["key"] == "q" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a tui_msg record with key=q, got %v", records)
	}
}

func TestTUIModel_LogsStateTransitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.jsonl")
	dl, err := NewDebugLog(path)
	if err != nil {
		t.Fatalf("NewDebugLog: %v", err)
	}
	m := newTestTUIModelWithDebugLog(t, dl)
	m.projectList.Select(0)

	updated, _ := m.Update(key(tea.KeyEnter))
	_ = updated
	dl.Close()

	records := readJSONLLines(t, path)
	found := false
	for _, r := range records {
		if r["event"] == "tui_state_change" && r["from"] == "viewProjects" && r["to"] == "viewObservations" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a tui_state_change record from=viewProjects to=viewObservations, got %v", records)
	}
}

func TestTUIModel_LogsKBCallsFromSearch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.jsonl")
	dl, err := NewDebugLog(path)
	if err != nil {
		t.Fatalf("NewDebugLog: %v", err)
	}
	m := newTestTUIModelWithDebugLog(t, dl)

	updated, _ := m.Update(runeKey("/"))
	m = updated.(*tuiModel)
	for _, r := range "specific" {
		updated, _ = m.Update(runeKey(string(r)))
		m = updated.(*tuiModel)
	}
	updated, _ = m.Update(key(tea.KeyEnter))
	_ = updated
	dl.Close()

	records := readJSONLLines(t, path)
	found := false
	for _, r := range records {
		if r["event"] == "kb_call" && r["method"] == "Search" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a kb_call record for Search, got %v", records)
	}
}

// newTestTUIModelWithRecords is newTestTUIModel plus two decision records on
// the "alpha" project, for exercising the records view.
func newTestTUIModelWithRecords(t *testing.T) *tuiModel {
	t.Helper()
	m := newTestTUIModel(t)
	p, err := m.kb.ProjectByName("alpha")
	if err != nil || p == nil {
		t.Fatalf("ProjectByName: %v", err)
	}
	for _, r := range []knowledge.Record{
		{RecordID: "0002", ProjectID: p.ID, Scope: "project", Date: "2026-08-19",
			Path: "alpha/decisions/0002-x.md", Title: "The newer decision",
			Status: "accepted", Kind: "decision", Body: "body"},
		{RecordID: "0001", ProjectID: p.ID, Scope: "project", Date: "2026-08-01",
			Path: "alpha/decisions/0001-x.md", Title: "The older decision",
			Status: "superseded", Kind: "correction", Body: "body"},
	} {
		if _, err := m.kb.AddRecord(r); err != nil {
			t.Fatalf("AddRecord: %v", err)
		}
	}
	return m
}

func TestTUIModel_BrowseRecordsUnderAProject(t *testing.T) {
	m := newTestTUIModelWithRecords(t)
	m.projectList.Select(0)
	updated, _ := m.Update(key(tea.KeyEnter))
	m = updated.(*tuiModel)

	updated, _ = m.Update(runeKey("r"))
	m = updated.(*tuiModel)

	if m.state != viewRecords {
		t.Fatalf("state after 'r' = %v, want viewRecords", m.state)
	}
	if len(m.recordList.Items()) != 2 {
		t.Fatalf("record list has %d items, want 2", len(m.recordList.Items()))
	}
	item, ok := m.recordList.Items()[0].(recordItem)
	if !ok {
		t.Fatalf("item 0 is %T, want recordItem", m.recordList.Items()[0])
	}
	if item.r.RecordID != "0002" {
		t.Errorf("first item = DR-%s, want DR-0002 (newest first)", item.r.RecordID)
	}
	if !strings.Contains(item.Title(), "DR-0002") {
		t.Errorf("Title() = %q, want it to name the record", item.Title())
	}
}

func TestTUIModel_RecordsNavigateBackAndAcross(t *testing.T) {
	m := newTestTUIModelWithRecords(t)
	m.projectList.Select(0)
	updated, _ := m.Update(key(tea.KeyEnter))
	m = updated.(*tuiModel)
	updated, _ = m.Update(runeKey("r"))
	m = updated.(*tuiModel)

	updated, _ = m.Update(runeKey("o"))
	m = updated.(*tuiModel)
	if m.state != viewObservations {
		t.Errorf("state after 'o' = %v, want viewObservations", m.state)
	}

	updated, _ = m.Update(runeKey("r"))
	m = updated.(*tuiModel)
	updated, _ = m.Update(key(tea.KeyEsc))
	m = updated.(*tuiModel)
	if m.state != viewProjects {
		t.Errorf("state after esc = %v, want viewProjects", m.state)
	}
}

// The TUI is read-mostly by design: no key in the records view writes.
func TestTUIModel_RecordsViewRendersAndQuits(t *testing.T) {
	m := newTestTUIModelWithRecords(t)
	m.projectList.Select(0)
	updated, _ := m.Update(key(tea.KeyEnter))
	m = updated.(*tuiModel)
	updated, _ = m.Update(runeKey("r"))
	m = updated.(*tuiModel)

	if view := m.View(); !strings.Contains(view, "Records") {
		t.Errorf("View() = %q, want the records list rendered", view)
	}
	if _, cmd := m.Update(runeKey("q")); cmd == nil {
		t.Error("q did not quit from the records view")
	}
}
