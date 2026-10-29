package cli

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	knowledge "github.com/rsdoiel/knowledge"
	"github.com/rsdoiel/knowledge/embedtest"
)

// T10 of semantic-search-plan.md: a words, meaning or both toggle on the search
// prompt, hidden when the build has no embedder, using the same search as
// kb search -semantic and -hybrid.

var tabKey = tea.KeyPressMsg{Code: tea.KeyTab}

// meaningTUI is a TUI model on a knowledge base with a built semantic index and
// the fake embedder registered.
func meaningTUI(t *testing.T, indexed bool) *tuiModel {
	t.Helper()
	kb := openTestKB(t)
	p, err := kb.AddProject("alpha", "first project")
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		"slow inference on the Raspberry Pi board",
		"the heron stands in the shallow river at dawn",
		"tax forms are due in the spring",
	} {
		if _, err := kb.AddObservation(p, "note", body); err != nil {
			t.Fatal(err)
		}
	}
	fake := embedtest.New()
	knowledge.RegisterEmbedder(fake)
	t.Cleanup(func() { knowledge.RegisterEmbedder(nil) })
	if indexed {
		v, err := knowledge.CreateVectors(kb.Path(), fake)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { v.Close() })
		if _, err := kb.EmbedIndex(context.Background(), v, fake, knowledge.EmbedOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	m, err := newTUIModelAt(kb, nil, tuiStart{state: viewProjects})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return updated.(*tuiModel)
}

func searchPromptText(m *tuiModel) string {
	_, lines := m.screenBody()
	return strings.Join(lines, "\n")
}

func TestTUISearchMode_TabCyclesWordsMeaningBothAndTheLabelSaysWhich(t *testing.T) {
	m := meaningTUI(t, true)
	m, _ = press(t, m, runeKey("S"))
	for i, want := range []string{"(words)", "(meaning)", "(words + meaning)", "(words)"} {
		if got := searchPromptText(m); !strings.Contains(got, want) {
			t.Errorf("step %d: prompt %q does not say %q", i, got, want)
		}
		if !strings.Contains(m.legendFull(), "Tab") {
			t.Errorf("step %d: legend %q does not offer Tab", i, m.legendFull())
		}
		m, _ = press(t, m, tabKey)
	}
}

func TestTUISearchMode_NoEmbedderMeansNoToggleAndTheScreenIsUnchanged(t *testing.T) {
	m := newTestTUIModel(t)
	m, _ = press(t, m, runeKey("S"))
	before, legend := searchPromptText(m), m.legendFull()
	m, _ = press(t, m, tabKey)
	if after := searchPromptText(m); after != before || m.searchMode != searchLexical {
		t.Errorf("Tab changed the prompt: %q -> %q (mode %q)", before, after, m.searchMode)
	}
	if strings.Contains(legend, "Tab") || strings.Contains(before, "(words)") || before != "Search: "+m.searchInput.View() {
		t.Errorf("the plain build's prompt or legend changed: %q / %q", before, legend)
	}
}

func TestTUISearchMode_MeaningFindsWhatWordsCannotAndShowsTheScore(t *testing.T) {
	m := meaningTUI(t, true)
	m, _ = press(t, m, runeKey("S"), tabKey)
	m, _ = press(t, m, typed("slow inference on the Raspberry Pi")...)
	m, _ = press(t, m, key(tea.KeyEnter))
	if m.state != viewSearch || m.searching {
		t.Fatalf("state %s, searching %v, notice %q", viewStateNames[m.state], m.searching, m.notice)
	}
	items := m.searchList.Items()
	if len(items) == 0 {
		t.Fatal("no results")
	}
	first := items[0].(searchResultItem)
	if !strings.Contains(first.r.Snippet, "Raspberry Pi board") || first.score == nil {
		t.Errorf("first result %+v", first)
	}
	if title := first.Title(); !strings.Contains(title, "0.") {
		t.Errorf("the title does not show the score: %q", title)
	}
	if !strings.Contains(m.searchList.Title, "-semantic") {
		t.Errorf("the list title does not name the command: %q", m.searchList.Title)
	}
}

func TestTUISearchMode_BothSaysWhichRankingFoundARow(t *testing.T) {
	m := meaningTUI(t, true)
	m, _ = press(t, m, runeKey("S"), tabKey, tabKey)
	m, _ = press(t, m, typed("slow")...)
	m, _ = press(t, m, key(tea.KeyEnter))
	items := m.searchList.Items()
	if m.state != viewSearch || len(items) == 0 {
		t.Fatalf("state %s, notice %q", viewStateNames[m.state], m.notice)
	}
	if first := items[0].(searchResultItem); first.via == "" || !strings.Contains(first.Title(), "via "+first.via) {
		t.Errorf("hybrid result lacks its via: %+v / %q", first, first.Title())
	}
}

func TestTUISearchMode_AScopeAndTermWorkAndANonEmbeddedKindIsAMessageNotACrash(t *testing.T) {
	m := meaningTUI(t, true)
	m, _ = press(t, m, runeKey("S"), tabKey)
	m, _ = press(t, m, typed("alpha/ slow inference on the Raspberry Pi")...)
	m, _ = press(t, m, key(tea.KeyEnter))
	if m.state != viewSearch || len(m.searchList.Items()) == 0 {
		t.Fatalf("scoped meaning search: state %s, notice %q", viewStateNames[m.state], m.notice)
	}
	m, _ = press(t, m, runeKey("S"))
	m.searchInput.SetValue("/sources slow")
	m, _ = press(t, m, key(tea.KeyEnter))
	if !m.searching || !strings.Contains(m.notice, "not embedded") {
		t.Errorf("searching %v, notice %q: want the prompt kept open with the reason", m.searching, m.notice)
	}
}

func TestTUISearchMode_NoIndexKeepsThePromptOpenAndSaysWhatToRun(t *testing.T) {
	m := meaningTUI(t, false)
	m, _ = press(t, m, runeKey("S"), tabKey)
	m, _ = press(t, m, typed("slow inference")...)
	m, _ = press(t, m, key(tea.KeyEnter))
	if !m.searching || m.state == viewSearch || !strings.Contains(m.notice, "kb embed") {
		t.Errorf("searching %v, state %s, notice %q", m.searching, viewStateNames[m.state], m.notice)
	}
	// Back to words, the same line runs as a plain search.
	m, _ = press(t, m, tabKey, tabKey)
	m, _ = press(t, m, key(tea.KeyEnter))
	if m.searchMode != searchLexical || m.searching {
		t.Errorf("mode %q, searching %v, notice %q", m.searchMode, m.searching, m.notice)
	}
}

// The TUI and kb search -semantic rank the same rows in the same order: both go
// through one function.
func TestTUISearchMode_AgreesWithTheCommandLine(t *testing.T) {
	m := meaningTUI(t, true)
	m, _ = press(t, m, runeKey("S"), tabKey)
	m, _ = press(t, m, typed("river heron dawn")...)
	m, _ = press(t, m, key(tea.KeyEnter))
	var tui []int64
	for _, it := range m.searchList.Items() {
		tui = append(tui, it.(searchResultItem).r.SourceID)
	}
	res, err := meaningSearch(m.kb, nil, searchSemantic, nil, knowledge.SearchScope{}, "everywhere", "river heron dawn")
	if err != nil {
		t.Fatal(err)
	}
	var cli []int64
	for _, h := range res.Hits {
		cli = append(cli, h.R.SourceID)
	}
	if len(tui) == 0 || len(tui) != len(cli) {
		t.Fatalf("tui %v cli %v", tui, cli)
	}
	for i := range tui {
		if tui[i] != cli[i] {
			t.Errorf("row %d: tui %d cli %d", i, tui[i], cli[i])
		}
	}
}
