package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	knowledge "github.com/rsdoiel/knowledge"
)

// The document summary workflow (knowledge DR-0069, v0.0.20 U7). The Review queue
// lists the sections waiting for a person in triage order. Enter on one is a
// two-step unit: the summary is written (in $VISUAL or $EDITOR with the TUI
// suspended, else in a built-in text area), then shown beside its source, and on
// y it is drafted by a person and promoted. A draft already there goes straight
// to the second step. Esc, q and n at any point leave what was approved before.

type editorCall struct {
	calls int
	argv  []string
	file  string // what the editor was given
}

// fakeEditorSeam replaces execEditor. The "editor" runs when the returned command
// does: it turns the file it was given into write(file), as a person saving would.
func fakeEditorSeam(t *testing.T, write func(old string) string) *editorCall {
	t.Helper()
	got := &editorCall{}
	old := execEditor
	execEditor = func(argv []string, path string) tea.Cmd {
		got.calls++
		got.argv = argv
		b, _ := os.ReadFile(path)
		got.file = string(b)
		return func() tea.Msg {
			if write != nil {
				_ = os.WriteFile(path, []byte(write(string(b))), 0o644)
			}
			return editorDoneMsg{}
		}
	}
	t.Cleanup(func() { execEditor = old })
	return got
}

func doneEditor(t *testing.T, m *tuiModel, cmd tea.Cmd) *tuiModel {
	t.Helper()
	if cmd == nil {
		t.Fatal("no editor command was returned")
	}
	next, _ := m.Update(cmd())
	return next.(*tuiModel)
}

// summaryModel is the Review queue open on a workspace whose one document has been
// ingested, with the section under the cursor returned too.
func summaryModel(t *testing.T) (*tuiModel, *knowledge.KnowledgeBase, knowledge.DocumentReviewItem) {
	t.Helper()
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	m, _, kb := planModel(t)
	items, err := kb.DocumentReviewQueue(0, "")
	if err != nil || len(items) == 0 {
		t.Fatalf("the fixture has nothing to review: %v", err)
	}
	m = openLeaf(t, m, "Documents", "Review queue")
	if m.state != viewReviewQueue {
		t.Fatalf("state = %s, want the review queue", viewStateNames[m.state])
	}
	return m, kb, items[0]
}

func sectionStatus(t *testing.T, kb *knowledge.KnowledgeBase, id int64) (status, body, by string) {
	t.Helper()
	items, err := kb.DocumentReviewQueue(0, "reviewed")
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.ID == id {
			return it.SummaryStatus, it.SummaryBody, it.GeneratedBy
		}
	}
	all, _ := kb.DocumentReviewQueue(0, "")
	for _, it := range all {
		if it.ID == id {
			return it.SummaryStatus, it.SummaryBody, it.GeneratedBy
		}
	}
	t.Fatalf("section %d is nowhere", id)
	return
}

func TestSummary_TheQueueShowsTheTriageSignals(t *testing.T) {
	m, _, it := summaryModel(t)
	v := m.View()
	for _, want := range []string{"unsummarized", "A note", "size", "density", "Enter summarise"} {
		if !strings.Contains(v, want) {
			t.Errorf("the queue lacks %q (section %d):\n%s", want, it.ID, v)
		}
	}
	m, _ = press(t, m, ch('q'))
	if m.state != viewGroup {
		t.Errorf("q gave %s, want the Documents menu", viewStateNames[m.state])
	}
}

func TestSummary_TheEditorWritesTheSummaryAndYApproves(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "myeditor -w")
	ed := fakeEditorSeam(t, func(old string) string {
		return "# a comment the person leaves\n\nStreaming is how events arrive.\n"
	})
	m, kb, it := summaryModel(t)
	t.Setenv("EDITOR", "myeditor -w")
	m, cmd := press(t, m, keyEnter)
	if ed.calls != 1 || len(ed.argv) < 2 || ed.argv[0] != "myeditor" || ed.argv[1] != "-w" {
		t.Fatalf("editor calls %d, argv %q; want myeditor -w FILE", ed.calls, ed.argv)
	}
	if !strings.HasPrefix(ed.file, "#") || !strings.Contains(ed.file, "streaming here") {
		t.Errorf("the buffer should be a # header and the section text:\n%s", ed.file)
	}
	m = doneEditor(t, m, cmd)
	if m.state != viewSummary {
		t.Fatalf("state = %s, want the review of the summary:\n%s", viewStateNames[m.state], m.View())
	}
	v := m.View()
	for _, want := range []string{"Streaming is how events arrive.", "streaming here", "y approve"} {
		if !strings.Contains(v, want) {
			t.Errorf("the review lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "a comment the person leaves") {
		t.Errorf("a # comment line was kept:\n%s", v)
	}
	if st, _, _ := sectionStatus(t, kb, it.ID); st != "unsummarized" {
		t.Fatalf("status %q before y, want unsummarized", st)
	}
	m, _ = press(t, m, ch('y'))
	st, body, by := sectionStatus(t, kb, it.ID)
	if st != "reviewed" || body != "Streaming is how events arrive." || by != "human" {
		t.Errorf("after y: %q, %q, by %q", st, body, by)
	}
	if m.state != viewReviewQueue {
		t.Fatalf("state = %s, want the queue", viewStateNames[m.state])
	}
	v = m.View()
	for _, want := range []string{"kb document draft", "kb document review promote"} {
		if !strings.Contains(v, want) {
			t.Errorf("the notice lacks %q:\n%s", want, v)
		}
	}
}

func TestSummary_AnUnchangedOrEmptyBufferIsRefused(t *testing.T) {
	for name, write := range map[string]func(string) string{
		"unchanged": nil,
		"empty":     func(string) string { return "# only a comment\n" },
	} {
		t.Run(name, func(t *testing.T) {
			fakeEditorSeam(t, write)
			m, kb, it := summaryModel(t)
			t.Setenv("EDITOR", "ed")
			m, cmd := press(t, m, keyEnter)
			m = doneEditor(t, m, cmd)
			if m.state != viewReviewQueue || !strings.Contains(m.View(), name) {
				t.Errorf("state %s; want the queue and a refusal saying %q:\n%s", viewStateNames[m.state], name, m.View())
			}
			if st, _, _ := sectionStatus(t, kb, it.ID); st != "unsummarized" {
				t.Errorf("status %q, want unchanged", st)
			}
		})
	}
}

func TestSummary_WithNoEditorTheBuiltInTextAreaIsUsed(t *testing.T) {
	m, kb, it := summaryModel(t)
	m, cmd := press(t, m, keyEnter)
	if cmd != nil || m.state != viewSummary {
		t.Fatalf("state %s (cmd %v); want the built-in text area", viewStateNames[m.state], cmd != nil)
	}
	if !strings.Contains(m.View(), "streaming here") {
		t.Errorf("the text area should start from the section text:\n%s", m.View())
	}
	// q is text here, and Esc cancels the unit.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m, _ = press(t, m, typed("quiet summary")...)
	if m.state != viewSummary || !strings.Contains(m.View(), "quiet summary") {
		t.Fatalf("typing left the text area (%s):\n%s", viewStateNames[m.state], m.View())
	}
	m, _ = press(t, m, keyEnter)
	if !strings.Contains(m.View(), "y approve") {
		t.Fatalf("Enter should go on to the review:\n%s", m.View())
	}
	m, _ = press(t, m, ch('y'))
	if st, body, _ := sectionStatus(t, kb, it.ID); st != "reviewed" || !strings.HasSuffix(body, "quiet summary") {
		t.Errorf("after y: %q %q", st, body)
	}
}

func TestSummary_EveryCancelWritesNothing(t *testing.T) {
	m, kb, it := summaryModel(t)
	m, _ = press(t, m, keyEnter) // built-in text area
	m, cmd := press(t, m, keyEsc)
	if isQuit(cmd) || m.state != viewReviewQueue {
		t.Fatalf("Esc in the text area gave %s", viewStateNames[m.state])
	}
	for name, key := range map[string]tea.Msg{"n": ch('n'), "q": ch('q'), "Esc": keyEsc} {
		m, _ = press(t, m, keyEnter, tea.KeyMsg{Type: tea.KeyCtrlU})
		m, _ = press(t, m, append(typed("text"), keyEnter)...)
		if m.state != viewSummary {
			t.Fatalf("%s: could not reach the review (%s)", name, viewStateNames[m.state])
		}
		m, _ = press(t, m, key)
		if m.state != viewReviewQueue || !strings.Contains(m.View(), "cancelled") {
			t.Errorf("%s: state %s:\n%s", name, viewStateNames[m.state], m.View())
		}
		if st, _, _ := sectionStatus(t, kb, it.ID); st != "unsummarized" {
			t.Errorf("%s: status %q", name, st)
		}
	}
}

func TestSummary_AnExistingDraftGoesStraightToTheReview(t *testing.T) {
	m, kb, it := summaryModel(t)
	if err := kb.DraftDocumentSummary(it.ID, "A model's draft.", "model", nil); err != nil {
		t.Fatal(err)
	}
	m, _ = press(t, m, ch('q'), ch('q'))
	m = openLeaf(t, m, "Documents", "Review queue")
	m, cmd := press(t, m, keyEnter)
	if cmd != nil || m.state != viewSummary || !strings.Contains(m.View(), "A model's draft.") || !strings.Contains(m.View(), "y approve") {
		t.Fatalf("state %s; a draft should be shown beside its source:\n%s", viewStateNames[m.state], m.View())
	}
	m, _ = press(t, m, ch('y'))
	st, body, by := sectionStatus(t, kb, it.ID)
	if st != "reviewed" || body != "A model's draft." || by != "model" {
		t.Errorf("promoted as is: %q %q by %q; the draft's author must stay", st, body, by)
	}
}

func TestSummary_EditingADraftMakesItTheirs(t *testing.T) {
	fakeEditorSeam(t, func(old string) string { return strings.Replace(old, "A model's draft.", "A better draft.", 1) })
	m, kb, it := summaryModel(t)
	t.Setenv("EDITOR", "ed")
	if err := kb.DraftDocumentSummary(it.ID, "A model's draft.", "model", nil); err != nil {
		t.Fatal(err)
	}
	m, _ = press(t, m, ch('q'), ch('q'))
	m = openLeaf(t, m, "Documents", "Review queue")
	m, _ = press(t, m, keyEnter)
	m, cmd := press(t, m, ch('e'))
	m = doneEditor(t, m, cmd)
	if !strings.Contains(m.View(), "A better draft.") {
		t.Fatalf("the edited text is not shown:\n%s", m.View())
	}
	m, _ = press(t, m, ch('y'))
	if st, body, by := sectionStatus(t, kb, it.ID); st != "reviewed" || body != "A better draft." || by != "human" {
		t.Errorf("after an edit: %q %q by %q", st, body, by)
	}
}

func TestSummary_CtrlCQuitsFromTheUnit(t *testing.T) {
	m, _, _ := summaryModel(t)
	m, _ = press(t, m, keyEnter)
	if _, cmd := press(t, m, keyCtrlC); !isQuit(cmd) {
		t.Error("Ctrl-C did not quit")
	}
}

// ─── the command line ───────────────────────────────────────────────────────

func TestSummary_PromoteTypedAtTheCommandLineIsTheReviewForm(t *testing.T) {
	m, kb, it := summaryModel(t)
	if err := kb.DraftDocumentSummary(it.ID, "A model's draft.", "model", nil); err != nil {
		t.Fatal(err)
	}
	m, _ = press(t, m, ch('q'))
	m = runCommand(t, m, "document review promote "+strconv.FormatInt(it.ID, 10))
	if m.state != viewSummary || !strings.Contains(m.View(), "A model's draft.") {
		t.Fatalf("state %s; want the review form:\n%s", viewStateNames[m.state], m.View())
	}
	m, _ = press(t, m, ch('y'))
	if st, _, _ := sectionStatus(t, kb, it.ID); st != "reviewed" {
		t.Errorf("status %q after y", st)
	}
}

func TestSummary_PromotingWhatHasNoDraftIsRefused(t *testing.T) {
	m, kb, it := summaryModel(t)
	m, _ = press(t, m, ch('q'))
	m = runCommand(t, m, "document review promote "+strconv.FormatInt(it.ID, 10))
	if m.state == viewSummary || !strings.Contains(m.View(), "draft") {
		t.Errorf("state %s; a section with no draft must say so:\n%s", viewStateNames[m.state], m.View())
	}
	if st, _, _ := sectionStatus(t, kb, it.ID); st != "unsummarized" {
		t.Errorf("status %q", st)
	}
}

func TestSummary_DraftTypedAtTheCommandLineAsksFirst(t *testing.T) {
	m, kb, it := summaryModel(t)
	m, _ = press(t, m, ch('q'))
	m = runCommand(t, m, "document draft "+strconv.FormatInt(it.ID, 10)+` "By hand." --by me`)
	if m.state != viewPlan {
		t.Fatalf("state %s; want the confirmation:\n%s", viewStateNames[m.state], m.View())
	}
	m, _ = press(t, m, ch('y'))
	if st, body, by := sectionStatus(t, kb, it.ID); st != "drafted" || body != "By hand." || by != "me" {
		t.Errorf("after y: %q %q by %q", st, body, by)
	}
}

func TestSummary_TheRowIsBuilt(t *testing.T) {
	for _, it := range groupMenu("document") {
		want := it.Label == "Review queue" || it.Label == "Ingest a document…" || it.Label == "Tag…" || it.Label == "Fuzzy-tag…"
		if it.Built != want {
			t.Errorf("%s built = %v, want %v", it.Label, it.Built, want)
		}
	}
}

// ─── the real binary ────────────────────────────────────────────────────────

func TestBinary_TUISummariseWithAnExternalEditor(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	builtKB(t)
	_, root, kb := planModel(t)
	items, _ := kb.DocumentReviewQueue(0, "")
	script := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'Written in the editor.\\n' > \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", script)
	t.Setenv("VISUAL", "")
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	master, screen, wait := startOnATerminal(t, root, "-i")
	screen.waitFor(t, "narratives, their summaries and review")
	master.WriteString("jjjjj\r") // Documents
	screen.waitFor(t, "summaries waiting for a person")
	master.WriteString("jj\r") // Review queue
	screen.waitFor(t, "Enter summarise")
	master.WriteString("\r") // the first section: the editor runs
	screen.waitFor(t, "Written in the editor.")
	master.WriteString("y")
	screen.waitFor(t, "kb document review promote")
	if st, body, by := sectionStatus(t, kb, items[0].ID); st != "reviewed" || body != "Written in the editor." || by != "human" {
		t.Errorf("after y: %q %q by %q", st, body, by)
	}
	master.WriteString("qqq")
	if code := wait(); code != 0 {
		t.Errorf("exit = %d, want 0:\n%s", code, screen.text())
	}
}
