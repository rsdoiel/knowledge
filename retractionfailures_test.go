package knowledge

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// CheckRetractions printed "[skip]" for a source whose check failed and went on
// as if nothing had happened: the command exited 0 saying it had checked N
// sources, and it stamped last_checked_at on the ones it could not reach. A DOI
// that could not be looked up reads as "not retracted", which is false
// reassurance. It still checks every source (one failure must not stop the rest),
// but now it says so, and only a check that got an answer counts as checked.

func lastChecked(t *testing.T, kb *KnowledgeBase, id int64) string {
	t.Helper()
	var s string
	if err := kb.db.QueryRow(`SELECT IFNULL(last_checked_at, '') FROM sources WHERE id = ?`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCheckRetractions_FailedChecksAreReportedAfterCheckingTheRest(t *testing.T) {
	kb := openTestKB(t)
	idA, _ := kb.AddSource(Source{Title: "Unreachable", IdentifierType: "doi", IdentifierValue: "10.1/a"})
	idB, _ := kb.AddSource(Source{Title: "Fine", IdentifierType: "doi", IdentifierValue: "10.1/b"})
	idC, _ := kb.AddSource(Source{Title: "Retracted", IdentifierType: "doi", IdentifierValue: "10.1/c"})
	down := errors.New("service down")
	checker := func(doi string) (bool, string, error) {
		switch doi {
		case "10.1/a":
			return false, "", down
		case "10.1/c":
			return true, "withdrawn", nil
		}
		return false, "", nil
	}
	beforeA := lastChecked(t, kb, idA)
	beforeB := lastChecked(t, kb, idB)
	var out strings.Builder
	checked, updated, err := kb.CheckRetractions(checker, &out)

	var ce *RetractionCheckError
	if !errors.As(err, &ce) {
		t.Fatalf("error = %v, want a *RetractionCheckError", err)
	}
	if ce.Failed != 1 || !errors.Is(err, down) {
		t.Errorf("Failed=%d, errors.Is(down)=%v; want 1 and true", ce.Failed, errors.Is(err, down))
	}
	if checked != 2 || updated != 1 {
		t.Errorf("checked=%d updated=%d, want the two that got an answer (2) and 1 newly retracted", checked, updated)
	}
	for _, want := range []string{"[skip]", "10.1/a", "[ok]", "10.1/b", "[RETRACTED]", "10.1/c"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output %q should mention %q: every source is still checked", out.String(), want)
		}
	}
	// last_checked_at has a creation default, so compare with the value before:
	// the unreachable source was not stamped, the ones that answered were.
	if got := lastChecked(t, kb, idA); got != beforeA {
		t.Errorf("last_checked_at changed from %q to %q for the source that could not be reached", beforeA, got)
	}
	if got := lastChecked(t, kb, idB); got == beforeB || !strings.Contains(got, "T") {
		t.Errorf("last_checked_at = %q, want it stamped for a source that got an answer (was %q)", got, beforeB)
	}
	if lastChecked(t, kb, idC) == "" {
		t.Error("the retracted source should have last_checked_at set")
	}
}

func TestCheckRetractions_NoFailuresIsNilError(t *testing.T) {
	kb := openTestKB(t)
	kb.AddSource(Source{Title: "T", IdentifierType: "doi", IdentifierValue: "10.1/x"})
	if _, _, err := kb.CheckRetractions(func(string) (bool, string, error) { return false, "", nil }, io.Discard); err != nil {
		t.Errorf("err = %v, want nil when every check got an answer", err)
	}
}

func TestRetractionCheckError_Message(t *testing.T) {
	err := &RetractionCheckError{Checked: 3, Failed: 2, First: errors.New("boom")}
	for _, want := range []string{"2", "boom"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q should mention %q", err.Error(), want)
		}
	}
}
