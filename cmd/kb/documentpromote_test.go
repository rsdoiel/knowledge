package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// `kb document review promote SECTION_ID` needs an interactive terminal
// (DR-0070, extending DR-0061). Promoting is what makes a summary trusted:
// only a reviewed summary is indexed for search or returned by concept-tag
// retrieval, and models are the main consumers of that retrieval, so a model
// must not be able to promote its own draft. Without a terminal the command
// exits 2 and writes nothing; draft and the read-only verbs are unaffected.
// The rule lives in cmd/kb, not the library, so harvey's LearnSession.Accept
// (which calls PromoteDocumentSummary directly) is untouched.

const promotedBody = "a distinctive summary awaiting a person"

// draftedSection ingests a document and drafts its first section's summary,
// returning the section id. The summary is searchable only once promoted.
func draftedSection(t *testing.T) (kb *knowledge.KnowledgeBase, root string, secID int64) {
	t.Helper()
	kb, root = openWorkspaceKB(t)
	secID = seedIngestedSection(t, kb, root)
	if out, err := runDocument(t, kb, false, "draft", fmt.Sprint(secID), promotedBody, "--by", "model"); err != nil {
		t.Fatalf("document draft = %v (%q)", err, out)
	}
	return kb, root, secID
}

func searchable(t *testing.T, kb *knowledge.KnowledgeBase) bool {
	t.Helper()
	results, err := kb.Search("distinctive summary awaiting")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	return len(results) > 0
}

func TestDocumentPromote_NeedsATerminal(t *testing.T) {
	noTerminal(t)
	kb, _, secID := draftedSection(t)
	out, err := runDocument(t, kb, false, "review", "promote", fmt.Sprint(secID))
	if err == nil {
		t.Fatalf("promote without a terminal succeeded with %q", out)
	}
	if !isUsageError(err) {
		t.Errorf("error %v is not a usage error; the refusal is exit 2", err)
	}
	if !strings.Contains(err.Error(), "person") {
		t.Errorf("error %q should say that a person must promote it", err)
	}
	if searchable(t, kb) {
		t.Error("the summary was promoted despite the refusal")
	}
}

// The check comes before the section is looked up, so what the person sees does
// not depend on whether the id exists.
func TestDocumentPromote_TerminalCheckComesFirst(t *testing.T) {
	noTerminal(t)
	kb, _ := openWorkspaceKB(t)
	_, err := runDocument(t, kb, false, "review", "promote", "9999")
	if err == nil || !isUsageError(err) {
		t.Errorf("err = %v, want the usage error for no terminal", err)
	}
}

func TestDocumentPromote_JSONDoesNotBypass(t *testing.T) {
	noTerminal(t)
	kb, _, secID := draftedSection(t)
	if _, err := runDocument(t, kb, true, "review", "promote", fmt.Sprint(secID)); err == nil || !isUsageError(err) {
		t.Errorf("--json promote without a terminal: err = %v, want a usage error", err)
	}
	if searchable(t, kb) {
		t.Error("the summary was promoted despite the refusal")
	}
}

func TestDocumentPromote_HasNoBypassFlag(t *testing.T) {
	noTerminal(t)
	for _, flag := range []string{"--yes", "-y", "--force", "--accept", "--tty", "--no-tty", "--interactive"} {
		kb, _, secID := draftedSection(t)
		if _, err := runDocument(t, kb, false, "review", "promote", fmt.Sprint(secID), flag); err == nil || !isUsageError(err) {
			t.Errorf("%s: err = %v, want a usage error", flag, err)
		}
		if searchable(t, kb) {
			t.Errorf("%s promoted the summary", flag)
		}
	}
}

// Drafting and reading are not the person's act and need no terminal: models
// legitimately draft, and a script may list the queue.
func TestDocumentOtherVerbsNeedNoTerminal(t *testing.T) {
	noTerminal(t)
	kb, root := openWorkspaceKB(t)
	secID := seedIngestedSection(t, kb, root)
	if out, err := runDocument(t, kb, false, "draft", fmt.Sprint(secID), "a summary", "--by", "model"); err != nil {
		t.Errorf("document draft without a terminal = %v (%q)", err, out)
	}
	if out, err := runDocument(t, kb, false, "review", "list"); err != nil {
		t.Errorf("document review list without a terminal = %v (%q)", err, out)
	}
	if out, err := runDocument(t, kb, false, "list"); err != nil {
		t.Errorf("document list without a terminal = %v (%q)", err, out)
	}
}

// The existing behaviour is kept when a person is there.
func TestDocumentPromote_WithATerminalStillPromotes(t *testing.T) {
	kb, _, secID := draftedSection(t) // TestMain stands in for a terminal
	if out, err := runDocument(t, kb, false, "review", "promote", fmt.Sprint(secID)); err != nil {
		t.Fatalf("promote = %v (%q)", err, out)
	}
	if !searchable(t, kb) {
		t.Error("the promoted summary is not searchable")
	}
}

// ─── the real binary ─────────────────────────────────────────────────────────

func TestBinary_PromoteWithRedirectedStreamsIsExit2(t *testing.T) {
	bin := builtKB(t)
	kb, root, secID := draftedSection(t)
	cmd := exec.Command(bin, "document", "review", "promote", fmt.Sprint(secID))
	cmd.Dir = root
	cmd.Stdin = strings.NewReader("y\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if code := exitCode(t, cmd.Run()); code != 2 {
		t.Errorf("exit = %d, want 2 (stderr %q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "person") {
		t.Errorf("stderr %q should say that a person must promote it", stderr.String())
	}
	if searchable(t, kb) {
		t.Error("the summary was promoted despite the refusal")
	}
}

func TestBinary_PromoteIsReachableFromATerminal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal test is Linux only")
	}
	bin := builtKB(t)
	_, slave := openPtyForTest(t)
	kb, root, secID := draftedSection(t)
	cmd := exec.Command(bin, "document", "review", "promote", fmt.Sprint(secID))
	cmd.Dir = root
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	if code := exitCode(t, cmd.Run()); code != 0 {
		t.Fatalf("exit = %d from a terminal, want 0", code)
	}
	if !searchable(t, kb) {
		t.Error("the summary is not searchable after a promote from a terminal")
	}
}

// harvey promotes through the library, never through this command, and its
// rule ("a model may write but not accept") is enforced by construction there.
// The library must stay free of terminal logic, so this call works whatever
// the terminal state.
func TestLibraryPromoteIsUnaffectedByTheTerminalRule(t *testing.T) {
	noTerminal(t)
	kb, _, secID := draftedSection(t)
	if err := kb.PromoteDocumentSummary(secID); err != nil {
		t.Fatalf("PromoteDocumentSummary = %v", err)
	}
	if !searchable(t, kb) {
		t.Error("the summary is not searchable after a library promote")
	}
}
