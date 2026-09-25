package knowledge

import (
	"errors"
	"testing"
)

// X1 of exit-codes-plan.md (knowledge DR-0047 item 4): the library raises two
// kinds of error itself and marks them so a caller can classify with errors.Is
// instead of matching message text. ErrInvalid: a value it was given is not
// acceptable. ErrNotFound: a name it was asked about is not there. Adding the
// marker never changes the message a user reads.

func TestSentinels_AreDistinct(t *testing.T) {
	if errors.Is(ErrInvalid, ErrNotFound) || errors.Is(ErrNotFound, ErrInvalid) {
		t.Error("ErrInvalid and ErrNotFound must be different errors")
	}
}

func TestErrInvalid_ForValuesTheLibraryRejects(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("keep", ""); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"AddProjectWithStatus bad status", func() error { _, err := kb.AddProjectWithStatus("x", "", "bogus"); return err }},
		{"SetProjectStatus bad status", func() error { return kb.SetProjectStatus("keep", "bogus") }},
		{"AddObservation bad kind", func() error { _, err := kb.AddObservation(1, "bogus", "body"); return err }},
		{"AddObservation blank body", func() error { _, err := kb.AddObservation(1, "note", "  "); return err }},
		{"CleanName blank", func() error { _, err := CleanName("project", " "); return err }},
		{"CleanName control character", func() error { _, err := CleanName("concept", "a\x1bb"); return err }},
		{"AddProject blank name", func() error { _, err := kb.AddProject("", ""); return err }},
		{"AddSource blank title", func() error { _, err := kb.AddSource(Source{Title: " "}); return err }},
		{"AddSource bad date", func() error { _, err := kb.AddSource(Source{Title: "T", PublishedDate: "notadate"}); return err }},
		{"AddSource bad url", func() error {
			_, err := kb.AddSource(Source{Title: "T", IdentifierType: "url", IdentifierValue: "x y"})
			return err
		}},
		{"AddSource bad doi", func() error {
			_, err := kb.AddSource(Source{Title: "T", IdentifierType: "doi", IdentifierValue: "zzz"})
			return err
		}},
		{"ApplyFrontmatter unknown --set field", func() error {
			_, _, err := kb.ApplyFrontmatter("a.md", []byte("# T\n"), gitLikeProvenance, FrontmatterAccept{Set: map[string]string{"bogus": "1"}}, true)
			return err
		}},
		{"ApplyFrontmatter unknown --accept field", func() error {
			_, _, err := kb.ApplyFrontmatter("a.md", []byte("# T\n"), gitLikeProvenance, FrontmatterAccept{Fields: []string{"bogus"}}, true)
			return err
		}},
		{"ParseRecord missing required field", func() error {
			_, err := ParseRecord([]byte("---\nid: \"0001\"\n---\nbody\n"), "r.md")
			return err
		}},
		{"ParseRecord no frontmatter", func() error { _, err := ParseRecord([]byte("no frontmatter\n"), "r.md"); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatal("want an error")
			}
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("error %v is not ErrInvalid", err)
			}
			if errors.Is(err, ErrNotFound) {
				t.Errorf("error %v is also ErrNotFound", err)
			}
		})
	}
}

func TestErrNotFound_ForNamesThatAreNotThere(t *testing.T) {
	kb := openTestKB(t)
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"SetProjectStatus", func() error { return kb.SetProjectStatus("nosuch", "active") }},
		{"RenameProject", func() error { return kb.RenameProject("nosuch", "other") }},
		{"RenameConcept", func() error { return kb.RenameConcept("nosuch", "other") }},
		{"DeleteConcept", func() error { _, err := kb.DeleteConcept("nosuch", false); return err }},
		{"SuggestConcepts unknown project", func() error { _, err := kb.SuggestConcepts("nosuch", 5); return err }},
		{"ExportJSONL unknown project", func() error { return ExportJSONL(kb, discard{}, "nosuch") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatal("want an error")
			}
			if !errors.Is(err, ErrNotFound) {
				t.Errorf("error %v is not ErrNotFound", err)
			}
			if errors.Is(err, ErrInvalid) {
				t.Errorf("error %v is also ErrInvalid", err)
			}
		})
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// The marker must not change a word of what a user reads.
func TestSentinelErrors_MessagesAreUnchanged(t *testing.T) {
	kb := openTestKB(t)
	kb.AddProject("keep", "")
	for _, tc := range []struct {
		call func() error
		want string
	}{
		{func() error { return kb.SetProjectStatus("nosuch", "active") }, `knowledge: project "nosuch" not found`},
		{func() error { return kb.SetProjectStatus("keep", "bogus") },
			`knowledge: invalid project status "bogus" (want concept, active, paused, or concluded)`},
		{func() error { _, err := kb.AddObservation(1, "note", " "); return err }, `knowledge: observation body must not be empty`},
		{func() error { _, err := CleanName("project", ""); return err }, `knowledge: project name must not be empty`},
		{func() error { _, err := kb.DeleteConcept("nosuch", false); return err }, `knowledge: concept "nosuch" not found`},
	} {
		if err := tc.call(); err == nil || err.Error() != tc.want {
			t.Errorf("message = %q, want %q", err, tc.want)
		}
	}
}

// A wrapped cause stays reachable through a sentinel error.
func TestSentinelErrors_KeepTheirCause(t *testing.T) {
	cause := errors.New("root")
	err := invalidf("bad thing: %w", cause)
	if !errors.Is(err, ErrInvalid) || !errors.Is(err, cause) {
		t.Errorf("errors.Is(ErrInvalid)=%v, errors.Is(cause)=%v; want both true",
			errors.Is(err, ErrInvalid), errors.Is(err, cause))
	}
	if err.Error() != "bad thing: root" {
		t.Errorf("message = %q", err.Error())
	}
}

// The one existing typed error is not a sentinel: the CLI classifies it by type.
func TestConceptInUseError_IsStillATypedError(t *testing.T) {
	kb := openTestKB(t)
	pid, _ := kb.AddProject("p", "")
	cid, _ := kb.AddConcept("Linked", "")
	oid, _ := kb.AddObservation(pid, "note", "x")
	if err := kb.LinkObservationConcept(oid, cid); err != nil {
		t.Fatal(err)
	}
	_, err := kb.DeleteConcept("Linked", false)
	var inUse *ConceptInUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("error %v is not a *ConceptInUseError", err)
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalid) {
		t.Error("a concept in use is a refusal, neither not-found nor invalid")
	}
}
